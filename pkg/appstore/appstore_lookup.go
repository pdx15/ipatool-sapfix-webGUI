package appstore

import (
	"errors"
	"fmt"
	gohttp "net/http"
	"net/url"

	"github.com/majd/ipatool/v2/pkg/http"
)

type LookupInput struct {
	Account  Account
	BundleID string
	AppID    int64
	Platform Platform
}

type LookupOutput struct {
	App App
}

func (t *appstore) Lookup(input LookupInput) (LookupOutput, error) {
	countryCode, err := countryCodeFromStoreFront(input.Account.StoreFront)
	if err != nil {
		return LookupOutput{}, fmt.Errorf("failed to resolve the country code: %w", err)
	}

	request, err := t.lookupRequest(input.BundleID, input.AppID, countryCode, input.Platform)
	if err != nil {
		return LookupOutput{}, fmt.Errorf("failed to create lookup request: %w", err)
	}

	res, err := t.searchClient.Send(request)
	if err != nil {
		return LookupOutput{}, fmt.Errorf("request failed: %w", err)
	}

	if res.StatusCode != gohttp.StatusOK {
		return LookupOutput{}, NewErrorWithMetadata(errors.New("invalid response"), res)
	}

	if len(res.Data.Results) == 0 {
		return LookupOutput{}, errors.New("app not found")
	}

	return LookupOutput{
		App: res.Data.Results[0],
	}, nil
}

func (t *appstore) lookupRequest(bundleID string, appID int64, countryCode string, platform Platform) (http.Request, error) {
	url, err := t.lookupURL(bundleID, appID, countryCode, platform)
	if err != nil {
		return http.Request{}, err
	}

	return http.Request{
		URL:            url,
		Method:         http.MethodGET,
		ResponseFormat: http.ResponseFormatJSON,
	}, nil
}

func (t *appstore) lookupURL(bundleID string, appID int64, countryCode string, platform Platform) (string, error) {
	entity, err := platform.lookupEntity()
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Add("entity", entity)
	params.Add("limit", "1")
	params.Add("media", "software")
	params.Add("country", countryCode)

	// Support lookup by AppID if BundleID is empty
	if bundleID != "" {
		params.Add("bundleId", bundleID)
	} else if appID > 0 {
		params.Add("id", fmt.Sprintf("%d", appID))
	} else {
		return "", errors.New("either BundleID or AppID must be provided")
	}

	return fmt.Sprintf("https://%s%s?%s", iTunesAPIDomain, iTunesAPIPathLookup, params.Encode()), nil
}
