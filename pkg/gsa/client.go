package gsa

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/majd/ipatool/v2/pkg/anisette"
	"github.com/majd/ipatool/v2/pkg/appleca"
	"howett.net/plist"
)

const (
	// DefaultGSAURL is Apple's GrandSlam Authentication endpoint.
	DefaultGSAURL = "https://gsa.apple.com/grandslam/GsService2"
	// DefaultValidateURL is the 2FA code validation endpoint.
	DefaultValidateURL = "https://gsa.apple.com/grandslam/GsService2/validate"
	// DefaultUserAgent pairs with locally generated Windows anisette data.
	DefaultUserAgent = "iTunes/12.13.10 (Windows; Microsoft Windows 10 x64 Professional Edition (Build 19045); x64) AppleWebKit/7613.3.9.0.2"
)

var (
	// ErrAuthCodeRequired is returned when the account requires a two-factor
	// authentication code that was not provided.
	ErrAuthCodeRequired = errors.New("auth code is required")
	// ErrInvalidAuthCode is returned when a supplied 2FA code is rejected.
	ErrInvalidAuthCode = errors.New("invalid authentication code")
	// ErrBadCredentials is returned when Apple rejects the credentials.
	ErrBadCredentials = errors.New("bad Apple ID or password")
)

// Account carries the tokens issued by Apple during a GSA login.
type Account struct {
	Email               string
	Name                string
	DirectoryServicesID string // DsPrsId / dsPersonId
	AdsID               string // adsid (for X-Apple-Identity-Token)
	GsIDMSToken         string // GsIdmsToken from GSA
	PETToken            string // com.apple.gs.idms.pet
	HBToken             string // com.apple.gs.idms.hb
	PasswordToken       string // iTunes Store token (after ItunesAuthenticate)
	StoreFront          string
	Pod                 string
}

// Client performs the GSA login handshake and the subsequent iTunes Store
// authenticate exchange. It owns its http.Client (configured with the shared
// session cookie jar and a redirect policy that returns 3xx responses
// verbatim), so that store cookies are shared with the rest of the
// application while redirects are handled explicitly.
type Client struct {
	HTTP          *http.Client
	GSAURL        string
	ValidateURL   string
	iTunesAuthURL func(pod string) string
}

// NewClient returns a Client with the default endpoints and the given cookie
// jar. The jar is shared with the rest of the application so that the store
// session cookies produced by ItunesAuthenticate persist for later requests.
func NewClient(jar http.CookieJar) *Client {
	transport, err := appleca.Transport()
	if err != nil {
		// Fall back to the default transport; the login flow will surface the
		// TLS error with full context if Apple's roots cannot be set up.
		transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	return &Client{
		HTTP: &http.Client{
			Jar:       jar,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// Return 3xx responses verbatim; redirects (e.g. the pod
				// assignment 302 from the authenticate endpoint) are handled
				// explicitly.
				return http.ErrUseLastResponse
			},
		},
		GSAURL:        DefaultGSAURL,
		ValidateURL:   DefaultValidateURL,
		iTunesAuthURL: defaultiTunesAuthURL,
	}
}

func defaultiTunesAuthURL(pod string) string {
	return fmt.Sprintf(
		"https://p%s-buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/authenticate?Pod=%s&PRH=%s",
		pod, pod, pod)
}

// twoFADone is an internal marker for the recursion that re-runs the SRP
// exchange after a successful 2FA code validation.
const twoFADone = "__2fa_done__"

// Login runs the SRP-6a GSA handshake and returns the account tokens issued by
// gsa.apple.com (PET/adsid/GsIdmsToken). It does not perform the iTunes Store
// authenticate exchange; call ItunesAuthenticate afterwards.
func (c *Client) Login(email, password string, a anisette.Data, authCode string) (Account, error) {
	return c.login(email, password, a, authCode)
}

func (c *Client) login(email, password string, a anisette.Data, authCode string) (Account, error) {
	a = a.WithDefaults()
	if a.ClientTime == "" {
		a.ClientTime = iso8601Now()
	}

	spd, result, err := c.exchange(email, password, a)
	if err != nil {
		return Account{}, err
	}

	// Legacy "outer Result" 2FA trigger: Apple reports the need for a second
	// factor directly in the complete response Result field.
	if (result == "TwoFactorAuthentication" || result == "TwoStepVerification") && authCode == "" {
		return Account{}, ErrAuthCodeRequired
	}

	// Modern 2FA trigger: status-code 409 inside the decrypted spd payload.
	if intVal(spd, "status-code") == 409 {
		if authCode == twoFADone {
			return Account{}, errors.New("GSA 2FA: still receiving 409 after validation")
		}

		dsid := strVal(spd, "adsid")
		idms := strVal(spd, "GsIdmsToken")

		if authCode == "" {
			if sm := strVal(spd, "sm"); sm != "" {
				return Account{}, fmt.Errorf("%w: %s", ErrAuthCodeRequired, sm)
			}

			return Account{}, ErrAuthCodeRequired
		}

		ok, err := c.validate2FA(dsid, idms, authCode, a)
		if err != nil {
			return Account{}, err
		}

		if !ok {
			return Account{}, ErrInvalidAuthCode
		}

		return c.login(email, password, a, twoFADone)
	}

	acc := buildAccount(email, spd)
	if acc.DirectoryServicesID == "" || acc.GsIDMSToken == "" {
		return Account{}, errors.New("GSA: adsid/token absent from decrypted spd")
	}

	return acc, nil
}

// exchange runs the two-step SRP-6a exchange (init + complete) and returns the
// decrypted spd payload together with the complete Result field.
func (c *Client) exchange(email, password string, a anisette.Data) (map[string]interface{}, string, error) {
	aBytes, err := randomPrivate()
	if err != nil {
		return nil, "", err
	}

	N := GroupN()
	G := GroupG()
	nPadded := padTo(N, GroupLength)
	gPad := gPadded(GroupLength)
	A := computeA(aBytes, N, G, GroupLength)

	cpd := buildCPD(a)
	headers := buildHeaders(a)

	// Step 1: init
	initBody := map[string]interface{}{
		"Header": map[string]interface{}{"Version": "1.0.1"},
		"Request": map[string]interface{}{
			"A2k": A,
			"cpd": cpd,
			"ps":  []string{"s2k", "s2k_fo"},
			"u":   email,
			"o":   "init",
		},
	}

	initResp, err := c.postPlist(c.GSAURL, initBody, headers)
	if err != nil {
		return nil, "", err
	}

	initMap, err := decodePlistResponse(initResp)
	if err != nil {
		return nil, "", err
	}

	if err := checkStatus(initMap, "init"); err != nil {
		return nil, "", err
	}

	resp := responseDict(initMap)

	salt, _ := dataVal(resp, "s")
	bRaw, _ := dataVal(resp, "B")
	challenge := strVal(resp, "c")
	iters := intVal(resp, "i")
	if iters <= 0 {
		iters = 20000
	}

	protocol := strVal(resp, "sp")
	if protocol == "" {
		protocol = "s2k"
	}

	if len(bRaw) == 0 || len(salt) == 0 || challenge == "" {
		return nil, "", errors.New("GSA init: incomplete server response")
	}

	// SRP math
	bPadded := padTo(new(big.Int).SetBytes(bRaw), GroupLength)
	k := computeK(nPadded, gPad)
	u := computeU(A, bPadded)
	x := AppleS2K(password, salt, int(iters), protocol == "s2k_fo")
	S := computePremasterSecret(bPadded, k, G.Bytes(), x, nPadded, aBytes, u, GroupLength)
	K := sessionKey(S)
	M1 := computeM1(nPadded, gPad, []byte(strings.ToLower(email)), salt, A, bPadded, K)

	// Step 2: complete
	completeBody := map[string]interface{}{
		"Header": map[string]interface{}{"Version": "1.0.1"},
		"Request": map[string]interface{}{
			"M1":  M1,
			"c":   challenge,
			"cpd": cpd,
			"o":   "complete",
			"u":   email,
		},
	}

	completeResp, err := c.postPlist(c.GSAURL, completeBody, headers)
	if err != nil {
		return nil, "", err
	}

	completeMap, err := decodePlistResponse(completeResp)
	if err != nil {
		return nil, "", err
	}

	if err := checkStatus(completeMap, "complete"); err != nil {
		return nil, "", err
	}

	resp = responseDict(completeMap)
	result := strVal(resp, "Result")

	switch result {
	case "RepairRequired":
		return nil, "", errors.New("Apple ID needs repair: visit appleid.apple.com")
	case "", "Allow", "TwoFactorAuthentication", "TwoStepVerification":
		// continue to spd decryption
	default:
		return nil, "", fmt.Errorf("GSA: unexpected Result = %q", result)
	}

	spdEnc, _ := dataVal(resp, "spd")
	if len(spdEnc) == 0 {
		return nil, "", errors.New("GSA: spd missing — possible SRP key mismatch")
	}

	key, iv := SPDKeyIV(K)
	spdPlain, err := cbcDecrypt(key, iv, spdEnc)
	if err != nil {
		return nil, "", errors.New("GSA: AES-CBC decrypt failed — wrong session key?")
	}

	var spd map[string]interface{}
	if _, err := plist.Unmarshal(spdPlain, &spd); err != nil {
		return nil, "", fmt.Errorf("GSA: failed to parse spd: %w", err)
	}

	return spd, result, nil
}

// ItunesAuthenticate exchanges the short-lived PET for an iTunes Store
// passwordToken and persists the store session cookies.
func (c *Client) ItunesAuthenticate(acc Account, a anisette.Data, guid string) (Account, error) {
	if acc.PETToken == "" {
		return acc, errors.New("no PET token available")
	}

	payload, headers := c.itunesAuthPayload(acc, a, guid)

	pods := uniquePods(acc.Pod)
	var lastErr error

	for _, pod := range pods {
		url := c.iTunesAuthURL(pod)

		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Second)
			}

			resp, err := c.postPlist(url, payload, headers)
			if err != nil {
				lastErr = err
				break
			}

			handled := false

			switch resp.StatusCode {
			case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect:
				location := resp.Header.Get("Location")
				drainAndClose(resp)

				if location == "" {
					lastErr = fmt.Errorf("HTTP %d without Location (pod %s)", resp.StatusCode, pod)
					handled = true
					break
				}

				resp2, err := c.postPlist(location, payload, headers)
				if err != nil {
					lastErr = err
					handled = true
					break
				}

				if resp2.StatusCode == http.StatusOK || resp2.StatusCode == http.StatusCreated {
					return parseItunesAuthResponse(resp2, acc)
				}

				drainAndClose(resp2)
				lastErr = fmt.Errorf("HTTP %d after redirect (pod %s)", resp2.StatusCode, pod)
				handled = true
			case http.StatusOK, http.StatusCreated:
				return parseItunesAuthResponse(resp, acc)
			case http.StatusNoContent:
				drainAndClose(resp)
				lastErr = fmt.Errorf("HTTP 204 (pod %s)", pod)
				continue
			case http.StatusNotFound:
				drainAndClose(resp)
				lastErr = fmt.Errorf("HTTP 404 (pod %s)", pod)
				handled = true
			default:
				drainAndClose(resp)
				lastErr = fmt.Errorf("HTTP %d (pod %s)", resp.StatusCode, pod)
				handled = true
			}

			if handled {
				break
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no pods attempted")
	}

	return acc, fmt.Errorf("iTunes auth: all attempts exhausted: %w", lastErr)
}

func (c *Client) itunesAuthPayload(acc Account, a anisette.Data, guid string) (map[string]interface{}, map[string]string) {
	identity := base64.StdEncoding.EncodeToString([]byte(acc.AdsID + ":" + acc.GsIDMSToken))

	payload := map[string]interface{}{
		"appleId":       acc.Email,
		"attempt":       "1",
		"createSession": "true",
		"guid":          guid,
		"password":      acc.PETToken, // PET as password
		"rmp":           "0",
		"why":           "signIn",
	}

	headers := map[string]string{
		"Content-Type":           "application/x-apple-plist",
		"User-Agent":             userAgentOr(a),
		"X-Apple-I-MD":           a.OTP,
		"X-Apple-I-MD-M":         a.MachineID,
		"X-Apple-I-MD-RINFO":     a.RoutingInfo,
		"X-Apple-I-MD-LU":        a.LocalUserUUID,
		"X-Mme-Device-Id":        a.DeviceID,
		"X-Apple-I-Client-Time":  a.ClientTime,
		"X-Apple-I-TimeZone":     a.Timezone,
		"X-Apple-Identity-Token": identity,
	}

	return payload, headers
}

func parseItunesAuthResponse(resp *http.Response, acc Account) (Account, error) {
	defer drainAndClose(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return acc, err
	}

	var data map[string]interface{}
	if _, err := plist.Unmarshal(body, &data); err != nil {
		return acc, fmt.Errorf("failed to parse authenticate response: %w", err)
	}

	token := strVal(data, "passwordToken")
	if token == "" {
		msg := strVal(data, "customerMessage")
		if msg == "" {
			msg = "unknown error"
		}

		return acc, fmt.Errorf("iTunes auth: no passwordToken — %s", msg)
	}

	acc.PasswordToken = token
	if dsp := strVal(data, "dsPersonId"); dsp != "" {
		acc.DirectoryServicesID = dsp
	}

	if pod := resp.Header.Get("pod"); pod != "" {
		acc.Pod = pod
	} else if pod := resp.Header.Get("itspod"); pod != "" {
		acc.Pod = pod
	}

	if sf := resp.Header.Get("x-set-apple-store-front"); sf != "" {
		acc.StoreFront = sf
	}

	return acc, nil
}

func (c *Client) validate2FA(dsid, idmsToken, code string, a anisette.Data) (bool, error) {
	identity := base64.StdEncoding.EncodeToString([]byte(dsid + ":" + idmsToken))

	headers := map[string]string{
		"Accept":                 "text/x-xml-plist",
		"Content-Type":           "text/x-xml-plist",
		"User-Agent":             "Xcode",
		"Accept-Language":        "en-us",
		"X-Apple-App-Info":       "com.apple.gs.xcode.auth",
		"X-Xcode-Version":        "11.2 (11B41)",
		"X-Apple-Identity-Token": identity,
		"X-Apple-I-MD":           a.OTP,
		"X-Apple-I-MD-M":         a.MachineID,
		"X-Apple-I-MD-LU":        a.LocalUserUUID,
		"X-Apple-I-Client-Time":  a.ClientTime,
		"X-Apple-Locale":         a.Locale,
		"X-Apple-I-TimeZone":     a.Timezone,
		"security-code":          code,
	}

	if a.RoutingInfo != "" {
		headers["X-Apple-I-MD-RINFO"] = a.RoutingInfo
	}

	if a.ClientInfo != "" {
		headers["X-MMe-Client-Info"] = a.ClientInfo
	}

	if a.DeviceID != "" {
		headers["X-Mme-Device-Id"] = a.DeviceID
	}

	resp, err := c.getPlist(c.ValidateURL, headers)
	if err != nil {
		return false, err
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return true, nil
	}

	var data map[string]interface{}
	if _, err := plist.Unmarshal(body, &data); err != nil {
		// Apple returns HTTP 200 on validation; an unparseable body is treated
		// leniently, matching the reference implementation.
		return true, nil
	}

	if ec := coerceInt(data["ec"]); ec != 0 {
		return false, nil
	}

	return true, nil
}

// buildAccount extracts the account identity from the decrypted spd payload.
func buildAccount(email string, spd map[string]interface{}) Account {
	acc := Account{Email: email}

	acc.DirectoryServicesID = strVal(spd, "DsPrsId")
	if acc.DirectoryServicesID == "" {
		acc.DirectoryServicesID = strVal(spd, "adsid")
	}

	acc.GsIDMSToken = strVal(spd, "GsIdmsToken")
	if acc.GsIDMSToken == "" {
		acc.GsIDMSToken = strVal(spd, "idms_token")
	}

	if acc.GsIDMSToken == "" {
		acc.GsIDMSToken = strVal(spd, "acntToken")
	}

	acc.AdsID = strVal(spd, "adsid")

	firstName := strVal(spd, "fn")
	lastName := strVal(spd, "ln")

	if firstName == "" && lastName == "" {
		if acctInfo, ok := spd["acctInfo"].(map[string]interface{}); ok {
			firstName = strVal(acctInfo, "firstName")
			lastName = strVal(acctInfo, "lastName")
		}
	}

	acc.Name = strings.TrimSpace(firstName + " " + lastName)
	if acc.Name == "" {
		acc.Name = email
	}

	if tokens, ok := spd["t"].(map[string]interface{}); ok {
		acc.PETToken = tokenFromDict(tokens, "com.apple.gs.idms.pet")
		acc.HBToken = tokenFromDict(tokens, "com.apple.gs.idms.hb")
	}

	return acc
}

func tokenFromDict(tokens map[string]interface{}, key string) string {
	entry, ok := tokens[key].(map[string]interface{})
	if !ok {
		return ""
	}

	return strVal(entry, "token")
}

// buildCPD builds the "cpd" dictionary embedded in the GSA request body.
func buildCPD(a anisette.Data) map[string]interface{} {
	cpd := map[string]interface{}{
		"bootstrap": true,
		"icscrec":   true,
		"pbe":       false,
		"prkgen":    true,
		"svct":      "iCloud",

		"X-Apple-I-Device-Configuration-Mode": "Default",
		"X-Apple-I-ReAuth":                    false,
		"X-Apple-I-Request-UUID":              newRequestUUID(),

		"loc":  a.Locale,
		"cou":  countryCode(a.Locale),
		"dc":   "PC",
		"dec":  true,
		"capp": "com.apple.gs.xcode.auth",
		"ptkn": "",
		"prtn": "R1",
		"at":   "",

		"X-Apple-I-MD":          a.OTP,
		"X-Apple-I-MD-M":        a.MachineID,
		"X-Apple-I-MD-LU":       a.LocalUserUUID,
		"X-Apple-I-MD-RINFO":    routingInfoInt(a.RoutingInfo),
		"X-Apple-I-SRL-NO":      a.SerialNo,
		"X-Apple-I-Client-Time": a.ClientTime,
		"X-Apple-Locale":        a.Locale,
		"X-Apple-I-TimeZone":    a.Timezone,
	}

	if a.ClientInfo != "" {
		cpd["X-MMe-Client-Info"] = a.ClientInfo
	}

	if a.DeviceID != "" {
		cpd["X-Mme-Device-Id"] = a.DeviceID
	}

	return cpd
}

// buildHeaders builds the HTTP headers for the init/complete requests. Most
// anisette fields live in the cpd body; only the machine ID (and optionally
// client info/device id) stay in the headers.
func buildHeaders(a anisette.Data) map[string]string {
	headers := map[string]string{
		"Content-Type":    "text/x-xml-plist",
		"Accept":          "*/*",
		"User-Agent":      userAgentOr(a),
		"Accept-Language": "en-US,en;q=0.9",
		"X-Apple-I-MD-M":  a.MachineID,
	}

	if a.ClientInfo != "" {
		headers["X-MMe-Client-Info"] = a.ClientInfo
	}

	if a.DeviceID != "" {
		headers["X-Mme-Device-Id"] = a.DeviceID
	}

	return headers
}

func userAgentOr(a anisette.Data) string {
	if a.UserAgent != "" {
		return a.UserAgent
	}

	return DefaultUserAgent
}

// checkStatus inspects the Status dict of a GSA response and maps known error
// codes to sentinel errors.
func checkStatus(m map[string]interface{}, step string) error {
	lookup := m
	if resp, ok := m["Response"].(map[string]interface{}); ok {
		lookup = resp
	}

	status, ok := lookup["Status"].(map[string]interface{})
	if !ok {
		return nil
	}

	ec := coerceInt(status["ec"])
	if ec == 0 {
		return nil
	}

	em := coerceStr(status["em"])

	switch ec {
	case -22421:
		return ErrBadCredentials
	case -22020:
		return ErrAuthCodeRequired
	default:
		return fmt.Errorf("GSA %s error %d: %s", step, ec, em)
	}
}

// responseDict returns the "Response" sub-dict of a wrapped GSA response, or
// the top-level dict for the legacy flat format.
func responseDict(m map[string]interface{}) map[string]interface{} {
	if resp, ok := m["Response"].(map[string]interface{}); ok {
		return resp
	}

	return m
}

func (c *Client) postPlist(url string, body interface{}, headers map[string]string) (*http.Response, error) {
	data, err := encodePlist(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/x-xml-plist")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) getPlist(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func decodePlistResponse(resp *http.Response) (map[string]interface{}, error) {
	defer drainAndClose(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GSA: HTTP %d: %s", resp.StatusCode, bodySnippet(body))
	}

	var m map[string]interface{}
	if _, err := plist.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return m, nil
}

func encodePlist(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := plist.NewEncoder(&buf).Encode(v); err != nil {
		return nil, fmt.Errorf("failed to encode plist: %w", err)
	}

	return buf.Bytes(), nil
}

func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func bodySnippet(body []byte) string {
	snippet := strings.Join(strings.Fields(string(body)), " ")
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}

	return snippet
}

func countryCode(locale string) string {
	for _, sep := range []string{"_", "-"} {
		if i := strings.Index(locale, sep); i >= 0 && i+1 < len(locale) {
			return locale[i+1:]
		}
	}

	return "US"
}

func routingInfoInt(routingInfo string) int64 {
	if routingInfo == "" {
		return 0
	}

	n, err := strconv.ParseInt(routingInfo, 10, 64)
	if err != nil {
		return 0
	}

	return n
}

func iso8601Now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func newRequestUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}

	b[6] = (b[6] & 0x0F) | 0x40 // version 4
	b[8] = (b[8] & 0x3F) | 0x80 // variant

	s := strings.ToUpper(hex.EncodeToString(b))
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

func uniquePods(pod string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 3)

	for _, p := range []string{pod, "25", "6"} {
		if p == "" || seen[p] {
			continue
		}

		seen[p] = true
		out = append(out, p)
	}

	return out
}

// coerceStr converts a plist value to its string form, handling the numeric
// encodings Apple uses for fields like DsPrsId.
func coerceStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case uint64:
		return strconv.FormatUint(t, 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	}

	return ""
}

func coerceInt(v interface{}) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case uint64:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n
		}
	}

	return 0
}

func strVal(m map[string]interface{}, key string) string {
	return coerceStr(m[key])
}

func intVal(m map[string]interface{}, key string) int64 {
	return coerceInt(m[key])
}

func dataVal(m map[string]interface{}, key string) ([]byte, bool) {
	switch t := m[key].(type) {
	case []byte:
		return t, true
	case string:
		b, err := base64.StdEncoding.DecodeString(t)
		return b, err == nil
	}

	return nil, false
}
