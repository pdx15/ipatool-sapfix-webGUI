package appstore

import (
	"errors"
	"fmt"
	gohttp "net/http"
	"strings"

	"github.com/majd/ipatool/v2/pkg/http"
)

type ListVersionsInput struct {
	Account Account
	App     App
}

type ListVersionsOutput struct {
	ExternalVersionIdentifiers []string
	LatestExternalVersionID    string
}

func (t *appstore) ListVersions(input ListVersionsInput) (ListVersionsOutput, error) {
	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return ListVersionsOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	req := t.listVersionsRequest(input.Account, input.App, guid)
	res, err := t.downloadClient.Send(req)

	if err != nil {
		return ListVersionsOutput{}, fmt.Errorf("failed to send http request: %w", err)
	}

	if res.Data.FailureType == FailureTypePasswordTokenExpired || res.Data.FailureType == FailureTypeSignInRequired {
		return ListVersionsOutput{}, ErrPasswordTokenExpired
	}

	if res.Data.FailureType == FailureTypeLicenseNotFound {
		return ListVersionsOutput{}, ErrLicenseRequired
	}

	if res.Data.FailureType != "" && res.Data.CustomerMessage != "" {
		return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.CustomerMessage), res)
	}

	if res.Data.FailureType != "" {
		return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.FailureType), res)
	}

	if len(res.Data.Items) == 0 {
		// Retry primary endpoint once first (some apps like Ozon need this)
		res, err = t.downloadClient.Send(req)
		if err != nil {
			return ListVersionsOutput{}, fmt.Errorf("failed to retry http request: %w", err)
		}
		if res.Data.FailureType == FailureTypePasswordTokenExpired || res.Data.FailureType == FailureTypeSignInRequired {
			return ListVersionsOutput{}, ErrPasswordTokenExpired
		}
		if res.Data.FailureType == FailureTypeLicenseNotFound {
			return ListVersionsOutput{}, ErrLicenseRequired
		}
		if res.Data.FailureType != "" && res.Data.CustomerMessage != "" {
			return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.CustomerMessage), res)
		}
		if res.Data.FailureType != "" {
			return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.FailureType), res)
		}
	}

	if len(res.Data.Items) == 0 {
		// Try redownload endpoint as fallback
		redownloadReq := t.redownloadRequest(input.Account, input.App, guid, "")
		redownloadRes, redownloadErr := t.downloadClient.Send(redownloadReq)
		if redownloadErr != nil {
			var responseErr *http.UnexpectedResponseError
			if errors.As(redownloadErr, &responseErr) &&
				responseErr.StatusCode == gohttp.StatusInternalServerError && responseErr.Snippet == "" {
				// The unpinned redownload request can fail even when Apple's catalog
				// advertises a downloadable iOS build (issue #547). Retry that exact
				// build once with the latest catalog version.
				versionID, lookupErr := t.lookupLatestExternalVersionID(input.Account, input.App, PlatformIPhone)
				if lookupErr != nil {
					return ListVersionsOutput{}, fmt.Errorf("failed to resolve latest version for redownload: %w (original error: %w)", lookupErr, redownloadErr)
				}

				pinnedReq := t.redownloadRequest(input.Account, input.App, guid, versionID)
				pinnedRes, pinnedErr := t.downloadClient.Send(pinnedReq)
				if pinnedErr != nil {
					return ListVersionsOutput{}, fmt.Errorf("failed to send version-pinned redownload request: %w", pinnedErr)
				}
				if len(pinnedRes.Data.Items) == 0 {
					errMsg := "invalid response"
					if pinnedRes.Data.CustomerMessage != "" {
						errMsg = fmt.Sprintf("invalid response: %s", pinnedRes.Data.CustomerMessage)
					}
					return ListVersionsOutput{}, NewErrorWithMetadata(errors.New(errMsg), pinnedRes)
				}
				res = pinnedRes
			} else {
				return ListVersionsOutput{}, fmt.Errorf("both endpoints failed: primary=invalid response, redownload=%w", redownloadErr)
			}
		} else {
			if len(redownloadRes.Data.Items) == 0 {
				errMsg := "invalid response"
				if redownloadRes.Data.CustomerMessage != "" {
					errMsg = fmt.Sprintf("invalid response: %s", redownloadRes.Data.CustomerMessage)
				}
				return ListVersionsOutput{}, NewErrorWithMetadata(errors.New(errMsg), redownloadRes)
			}
			res = redownloadRes
		}
	}

	item := res.Data.Items[0]

	rawIdentifiers, ok := item.Metadata["softwareVersionExternalIdentifiers"].([]interface{})
	if !ok {
		return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("failed to get version identifiers from item metadata"), item.Metadata)
	}

	externalVersionIdentifiers := make([]string, len(rawIdentifiers))
	for i, val := range rawIdentifiers {
		externalVersionIdentifiers[i] = fmt.Sprintf("%v", val)
	}

	latestExternalVersionID := item.Metadata["softwareVersionExternalIdentifier"]
	if latestExternalVersionID == nil {
		return ListVersionsOutput{}, NewErrorWithMetadata(fmt.Errorf("failed to get latest version from item metadata"), item.Metadata)
	}

	return ListVersionsOutput{
		ExternalVersionIdentifiers: externalVersionIdentifiers,
		LatestExternalVersionID:    fmt.Sprintf("%v", latestExternalVersionID),
	}, nil
}

func (t *appstore) listVersionsRequest(acc Account, app App, guid string) http.Request {
	payload := map[string]interface{}{
		"creditDisplay": "",
		"guid":          guid,
		"salableAdamId": app.ID,
		"serialNumber":  "0",
	}

	podPrefix := ""
	if acc.Pod != "" {
		podPrefix = "p" + acc.Pod + "-"
	}

	return http.Request{
		URL:            fmt.Sprintf("https://%s%s%s?guid=%s", podPrefix, PrivateAppStoreAPIDomain, PrivateAppStoreAPIPathDownload, guid),
		Method:         http.MethodPOST,
		ResponseFormat: http.ResponseFormatXML,
		Headers: map[string]string{
			"Content-Type": "application/x-apple-plist",
			"iCloud-DSID":  acc.DirectoryServicesID,
			"X-Dsid":       acc.DirectoryServicesID,
		},
		Payload: &http.XMLPayload{
			Content: payload,
		},
	}
}
