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

	"github.com/majd/ipatool/v2/pkg/gsa"
	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/util"
)

var (
	ErrAuthCodeRequired = errors.New("auth code is required")
)

const legacyAuthenticateEndpoint = "https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/authenticate"

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
	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return LoginOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	// On Windows the GSA (SRP-6a) flow is consistently rejected by Apple with
	// a machine-provisioning error (-22410) before authentication can proceed,
	// regardless of iCloud state. Skip it entirely and go straight to the
	// legacy authenticate flow, which our Windows build signs with the bundled
	// "sapsigner.exe" helper (mirroring the macOS CommerceKit service).
	if runtime.GOOS == "windows" {
		acc, err := t.login(input.Email, input.Password, input.AuthCode, guid, legacyAuthenticateEndpoint)
		if err != nil {
			return LoginOutput{}, err
		}

		return LoginOutput{Account: acc}, nil
	}

	// Prefer the GSA (SRP-6a) flow. It does not rely on the deprecated native
	// auth endpoint and handles two-factor authentication properly.
	if t.gsa != nil && t.anisette != nil {
		acc, gsaErr := t.loginWithGSA(input, guid)
		switch {
		case gsaErr == nil:
			return LoginOutput{Account: acc}, nil
		case errors.Is(gsaErr, gsa.ErrAuthCodeRequired):
			return LoginOutput{}, ErrAuthCodeRequired
		case errors.Is(gsaErr, gsa.ErrBadCredentials), errors.Is(gsaErr, gsa.ErrInvalidAuthCode):
			return LoginOutput{}, gsaErr
		case runtime.GOOS != "darwin":
			// Other non-macOS platforms (Linux) have no SAP signing service
			// (neither CommerceKit nor sapsigner.exe), so the legacy flow
			// cannot help. Surface the real GSA failure.
			return LoginOutput{}, gsaErr
		default:
			// macOS: GSA could not be used (e.g. no anisette available or a
			// transient server failure). Fall through to the legacy flow, which
			// can sign the request via the macOS CommerceKit service.
		}
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
		res, err = t.loginClient.Send(request)

		if err != nil {
			if shouldRetryWithLegacyAuthenticate(endpoint, err) {
				return t.login(email, password, authCode, guid, legacyAuthenticateEndpoint)
			}

			return Account{}, fmt.Errorf("request failed: %w", err)
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
	} else if res.Data.FailureType == "" && authCode == "" && res.Data.CustomerMessage == CustomerMessageBadLogin {
		err = ErrAuthCodeRequired
	} else if res.Data.FailureType == "" && res.Data.CustomerMessage == CustomerMessageAccountDisabled {
		err = NewErrorWithMetadata(errors.New("account is disabled"), res)
	} else if res.Data.FailureType != "" {
		if res.Data.CustomerMessage != "" {
			err = NewErrorWithMetadata(errors.New(res.Data.CustomerMessage), res)
		} else {
			err = NewErrorWithMetadata(fmt.Errorf("something went wrong (failure type %s)", res.Data.FailureType), res)
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
