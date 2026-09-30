package appstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	gohttp "net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/majd/ipatool/v2/pkg/gsa"
	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/util"
)

var (
	ErrAuthCodeRequired = errors.New("auth code is required")
)

const legacyAuthenticateEndpoint = "https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/authenticate"

const (
	maxAuthenticationRequestAttempts = 3
	authenticationRetryDelay         = 10 * time.Second
	maxAuthenticationRetryDelay      = 30 * time.Second
)

type LoginInput struct {
	Email    string
	Password string
	AuthCode string
	Endpoint string
}

type LoginOutput struct {
	Account Account
}

func (t *appstore) Login(input LoginInput) (LoginOutput, error) {
	authCode, err := normalizeAuthCode(input.AuthCode)
	if err != nil {
		return LoginOutput{}, err
	}
	input.AuthCode = authCode

	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return LoginOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	// On macOS and Windows, skip the GSA (SRP-6a) flow entirely. GSA requires
	// anisette device-attestation headers which on non-Windows platforms come
	// from public anisette servers (e.g. ani.sidestore.io). Those servers
	// return the same device identity for all users (a fake "MacBookPro15,1"),
	// causing Apple to register that fake device in every user's trusted
	// devices list. Instead, use the legacy authenticate flow signed via the
	// platform's SAP action signer: sapsigner.exe on Windows, CommerceKit on
	// macOS. This matches the upstream ipatool behaviour on macOS.
	//
	// On Linux there is no SAP signing service available, so the legacy flow
	// cannot be used. Fall through to GSA for Linux only.
	if runtime.GOOS == "windows" {
		acc, err := t.login(input.Email, input.Password, input.AuthCode, guid, legacyAuthenticateEndpoint)
		if err != nil {
			return LoginOutput{}, err
		}

		return LoginOutput{Account: acc}, nil
	}

	if runtime.GOOS == "darwin" {
		// macOS: use the legacy authenticate flow with SAP signing via
		// CommerceKit. This avoids registering a fake device in the user's
		// trusted devices list (which would happen if GSA used public anisette
		// servers).
		acc, err := t.login(input.Email, input.Password, input.AuthCode, guid, input.Endpoint)
		if err != nil {
			return LoginOutput{}, err
		}

		return LoginOutput{Account: acc}, nil
	}

	// Linux: no SAP signing service is available, so GSA is the only option.
	if t.gsa != nil && t.anisette != nil {
		acc, gsaErr := t.loginWithGSA(input, guid)
		if gsaErr != nil {
			if errors.Is(gsaErr, gsa.ErrAuthCodeRequired) {
				return LoginOutput{}, ErrAuthCodeRequired
			}
			if errors.Is(gsaErr, gsa.ErrBadCredentials) || errors.Is(gsaErr, gsa.ErrInvalidAuthCode) {
				return LoginOutput{}, gsaErr
			}
			return LoginOutput{}, gsaErr
		}
		return LoginOutput{Account: acc}, nil
	}

	acc, err := t.login(input.Email, input.Password, input.AuthCode, guid, input.Endpoint)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{
		Account: acc,
	}, nil
}

// LoginMZFinance authenticates with the App Store using the stable legacy
// MZFinance authenticate flow. It runs the GSA (SRP-6a) handshake first so the
// public anisette and two-factor verification are handled reliably, then
// completes authentication directly against the legacy MZFinance authenticate
// endpoint (the same stable path used on Windows), bypassing the glitchy
// native/fast endpoint that Login may fall back to on macOS.
func (t *appstore) LoginMZFinance(input LoginInput) (LoginOutput, error) {
	authCode, err := normalizeAuthCode(input.AuthCode)
	if err != nil {
		return LoginOutput{}, err
	}
	input.AuthCode = authCode

	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return LoginOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	acc, err := t.loginWithGSAThenLegacy(input, guid)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{Account: acc}, nil
}

// loginWithGSAThenLegacy runs the GSA (SRP-6a) handshake to verify the
// credentials (and a two-factor code when supplied), then completes the login
// through the stable legacy MZFinance authenticate endpoint. This is an
// independent path from Login: it never touches the native/fast endpoint and
// does not change Login's behaviour.
func (t *appstore) loginWithGSAThenLegacy(input LoginInput, guid string) (Account, error) {
	if t.gsa != nil && t.anisette != nil {
		ani, err := t.anisette.Fetch(context.Background())
		if err != nil {
			return Account{}, fmt.Errorf("failed to fetch anisette data: %w", err)
		}

		_, gsaErr := t.gsa.Login(input.Email, input.Password, ani, input.AuthCode)
		switch {
		case gsaErr == nil:
			// GSA verified the credentials (and 2FA code, if provided).
		case errors.Is(gsaErr, gsa.ErrAuthCodeRequired):
			return Account{}, ErrAuthCodeRequired
		case errors.Is(gsaErr, gsa.ErrBadCredentials), errors.Is(gsaErr, gsa.ErrInvalidAuthCode):
			return Account{}, gsaErr
		default:
			// GSA could not be used (e.g. a transient server failure); still
			// attempt the stable legacy flow below.
		}
	}

	return t.login(input.Email, input.Password, input.AuthCode, guid, legacyAuthenticateEndpoint)
}

// loginWithGSA runs the SRP-6a GSA handshake, exchanges the PET for an iTunes
// Store password token, and persists the resulting session.
func (t *appstore) loginWithGSA(input LoginInput, guid string) (Account, error) {
	ani, err := t.anisette.Fetch(context.Background())
	if err != nil {
		return Account{}, fmt.Errorf("failed to fetch anisette data: %w", err)
	}

	acc, err := t.gsa.Login(input.Email, input.Password, ani, input.AuthCode)
	if err != nil {
		return Account{}, err
	}

	acc, err = t.gsa.ItunesAuthenticate(acc, ani, guid)
	if err != nil {
		return Account{}, err
	}

	if t.cookieJar != nil {
		if err := t.cookieJar.Save(); err != nil {
			return Account{}, fmt.Errorf("failed to save cookies: %w", err)
		}
	}

	out := Account{
		Name:                acc.Name,
		Email:               acc.Email,
		PasswordToken:       acc.PasswordToken,
		DirectoryServicesID: acc.DirectoryServicesID,
		StoreFront:          acc.StoreFront,
		Password:            input.Password,
		Pod:                 acc.Pod,
	}

	data, err := json.Marshal(out)
	if err != nil {
		return Account{}, fmt.Errorf("failed to marshal json: %w", err)
	}

	if err := t.keychain.Set("account", data); err != nil {
		return Account{}, fmt.Errorf("failed to save account in keychain: %w", err)
	}

	return out, nil
}

func normalizeAuthCode(code string) (string, error) {
	if code == "" {
		return "", nil
	}

	// Terminals may wrap pasted input in bracketed-paste markers. Strip only
	// a matched outer pair; other escape sequences are invalid input.
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "\x1b[200~") && strings.HasSuffix(code, "\x1b[201~") {
		code = strings.TrimSuffix(strings.TrimPrefix(code, "\x1b[200~"), "\x1b[201~")
	}

	code = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}, code)
	if len(code) != 6 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
		return "", errors.New("2FA code must contain exactly six digits")
	}

	return code, nil
}

type loginAddressResult struct {
	FirstName string `plist:"firstName,omitempty"`
	LastName  string `plist:"lastName,omitempty"`
}

type loginAccountResult struct {
	Email   string             `plist:"appleId,omitempty"`
	Address loginAddressResult `plist:"address,omitempty"`
}

type loginResult struct {
	FailureType         string             `plist:"failureType,omitempty"`
	CustomerMessage     string             `plist:"customerMessage,omitempty"`
	Account             loginAccountResult `plist:"accountInfo,omitempty"`
	DirectoryServicesID string             `plist:"dsPersonId,omitempty"`
	PasswordToken       string             `plist:"passwordToken,omitempty"`
}

func (t *appstore) login(email, password, authCode, guid, endpoint string) (Account, error) {
	redirect := ""

	var (
		err error
		res http.Result[loginResult]
	)

	retry := true

	for attempt := 1; retry && attempt <= 4; attempt++ {
		requestAttempt := attempt
		if redirect != "" {
			// The pod redirect is part of the same authentication attempt. Apple
			// expects the original XML plist body, including its attempt value.
			requestAttempt = 1
		}

		request := t.loginRequest(email, password, authCode, guid, endpoint, requestAttempt)
		request.URL, _ = util.IfEmpty(redirect, request.URL), ""
		res, err = t.sendAuthenticationRequest(request)

		if err != nil {
			if shouldRetryWithLegacyAuthenticate(endpoint, err) {
				return t.login(email, password, authCode, guid, legacyAuthenticateEndpoint)
			}

			stage := "sign-in"
			if authCode != "" {
				stage = "2FA verification"
			}

			if redirect != "" {
				stage += " at Store pod"
			}

			return Account{}, fmt.Errorf("%s request failed: %w", stage, err)
		}

		if retry, redirect, err = t.parseLoginResponse(&res, authCode); err != nil {
			return Account{}, err
		}
	}

	if retry {
		return Account{}, NewErrorWithMetadata(errors.New("too many attempts"), res)
	}

	sf, err := res.GetHeader(HTTPHeaderStoreFront)
	if err != nil {
		return Account{}, NewErrorWithMetadata(fmt.Errorf("failed to get storefront header: %w", err), res)
	}

	pod, err := res.GetHeader(HTTPHeaderPod)
	if err != nil && !errors.Is(err, http.ErrHeaderNotFound) {
		return Account{}, NewErrorWithMetadata(fmt.Errorf("failed to get pod header: %w", err), res)
	}

	addr := res.Data.Account.Address
	acc := Account{
		Name:                strings.Join([]string{addr.FirstName, addr.LastName}, " "),
		Email:               res.Data.Account.Email,
		PasswordToken:       res.Data.PasswordToken,
		DirectoryServicesID: res.Data.DirectoryServicesID,
		StoreFront:          sf,
		Password:            password,
		Pod:                 pod,
	}

	data, err := json.Marshal(acc)
	if err != nil {
		return Account{}, fmt.Errorf("failed to marshal json: %w", err)
	}

	err = t.keychain.Set("account", data)
	if err != nil {
		return Account{}, fmt.Errorf("failed to save account in keychain: %w", err)
	}

	return acc, nil
}

// sendAuthenticationRequest repeats an authentication request while Apple
// answers with a transient failure (empty 204, 404, 429 or a 5xx gateway
// error), honoring a Retry-After header when Apple sends one.
func (t *appstore) sendAuthenticationRequest(request http.Request) (http.Result[loginResult], error) {
	statuses := make([]string, 0, maxAuthenticationRequestAttempts)

	sleep := t.authRetrySleep
	if sleep == nil {
		sleep = time.Sleep
	}

	for attempt := 1; ; attempt++ {
		result, err := t.loginClient.Send(request)

		status, retry := retryableAuthenticationError(err)
		if !retry {
			if err != nil {
				return result, authenticationRequestError(err)
			}

			return result, nil
		}

		statuses = append(statuses, strconv.Itoa(status))

		if attempt == maxAuthenticationRequestAttempts {
			return result, fmt.Errorf(
				"authentication request failed after %d attempts (HTTP %s): %w",
				maxAuthenticationRequestAttempts, strings.Join(statuses, ", "), authenticationRequestError(err),
			)
		}

		delay := min(authenticationRetryDelay<<(attempt-1), maxAuthenticationRetryDelay)

		var responseErr *http.UnexpectedResponseError
		if errors.As(err, &responseErr) {
			if requested, ok := authenticationRetryAfter(responseErr.RetryAfter, time.Now()); ok {
				if requested > maxAuthenticationRetryDelay {
					return result, fmt.Errorf("apple requested a wait longer than %s; try again later: %w", maxAuthenticationRetryDelay, err)
				}

				// Retry-After takes precedence over the fallback backoff.
				delay = max(requested, time.Second)
			}
		}

		sleep(delay)
	}
}

// retryableAuthenticationError reports whether the authentication failure is
// transient enough to be worth another attempt.
func retryableAuthenticationError(err error) (int, bool) {
	var responseErr *http.UnexpectedResponseError
	if !errors.As(err, &responseErr) {
		return 0, false
	}

	status := responseErr.StatusCode
	retry := status == gohttp.StatusNoContent ||
		status == gohttp.StatusNotFound ||
		status == gohttp.StatusTooManyRequests ||
		status/100 == 5

	return status, retry
}

// authenticationRequestError turns a raw unexpected-response failure into a
// message that tells the user what to do instead of leaking Apple's HTML.
func authenticationRequestError(err error) error {
	var responseErr *http.UnexpectedResponseError
	if !errors.As(err, &responseErr) {
		return err
	}

	if responseErr.StatusCode == gohttp.StatusTooManyRequests {
		return fmt.Errorf("apple rate limited authentication; try again later: %w", err)
	}

	return fmt.Errorf("apple returned no usable authentication response; try again later or from another network: %w", err)
}

// authenticationRetryAfter parses a Retry-After header value (delta-seconds
// or HTTP-date) into a wait duration.
func authenticationRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		// Saturate before converting to Duration to avoid overflow. A wait over
		// the budget ends this login rather than retrying before Apple's deadline.
		if seconds > uint64(maxAuthenticationRetryDelay/time.Second) {
			return maxAuthenticationRetryDelay + time.Second, true
		}

		return time.Duration(seconds) * time.Second, true
	}

	if date, err := gohttp.ParseTime(value); err == nil {
		return max(time.Duration(0), date.Sub(now)), true
	}

	return 0, false
}

func shouldRetryWithLegacyAuthenticate(endpoint string, err error) bool {
	if !strings.Contains(endpoint, "/native/") {
		return false
	}

	var responseErr *http.UnexpectedResponseError
	if !errors.As(err, &responseErr) {
		return false
	}

	switch responseErr.StatusCode {
	case gohttp.StatusNoContent, gohttp.StatusForbidden, gohttp.StatusNotFound, gohttp.StatusServiceUnavailable:
		return true
	default:
		return false
	}
}

func (t *appstore) parseLoginResponse(res *http.Result[loginResult], authCode string) (bool, string, error) {
	var (
		retry    bool
		redirect string
		err      error
	)

	if res.StatusCode == gohttp.StatusFound {
		if redirect, err = res.GetHeader("location"); err != nil {
			err = fmt.Errorf("failed to retrieve redirect location: %w", err)
		} else {
			retry = true
		}
	} else if res.Data.FailureType == "" && res.Data.CustomerMessage == CustomerMessageBadLogin {
		if authCode == "" {
			err = ErrAuthCodeRequired
		} else {
			err = errors.New("apple did not complete verification; try a fresh 2FA code")
		}
	} else if res.Data.FailureType == "" && res.Data.CustomerMessage == CustomerMessageAccountDisabled {
		err = NewErrorWithMetadata(errors.New("account is disabled"), res)
	} else if res.Data.FailureType != "" {
		if res.Data.CustomerMessage != "" {
			err = NewErrorWithMetadata(errors.New(res.Data.CustomerMessage), res)
		} else {
			err = NewErrorWithMetadata(fmt.Errorf("apple returned no usable authentication response (HTTP %d): missing account credentials or unexpected status; try again later or from another network", res.StatusCode), res)
		}
	} else if res.StatusCode != gohttp.StatusOK || res.Data.PasswordToken == "" || res.Data.DirectoryServicesID == "" {
		err = NewErrorWithMetadata(errors.New("something went wrong"), res)
	}

	return retry, redirect, err
}

func (t *appstore) loginRequest(email, password, authCode, guid, endpoint string, attempt int) http.Request {
	return http.Request{
		Method:         http.MethodPOST,
		URL:            authenticateURL(endpoint),
		ResponseFormat: http.ResponseFormatXML,
		SignAction:     true,
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Payload: &http.XMLPayload{
			Content: map[string]interface{}{
				"appleId":  email,
				"attempt":  strconv.Itoa(attempt),
				"guid":     guid,
				"password": fmt.Sprintf("%s%s", password, strings.ReplaceAll(authCode, " ", "")),
				"rmp":      "0",
				"why":      "signIn",
			},
		},
	}
}

// authenticateURL normalizes the bag-provided authentication endpoint. Apple's
// current endpoint (https://auth.itunes.apple.com/auth/v1/native/fast) only
// responds correctly when the path has a trailing slash; without it the request
// is redirected/dropped and the login silently fails. The legacy MZFinance
// authenticate endpoint is left untouched.
func authenticateURL(endpoint string) string {
	if endpoint == "" {
		return endpoint
	}

	if strings.Contains(endpoint, "/native/") && !strings.HasSuffix(endpoint, "/") {
		return endpoint + "/"
	}

	return endpoint
}
