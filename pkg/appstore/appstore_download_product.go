package appstore

import (
	"errors"
	"fmt"
	gohttp "net/http"
	"net/url"
	"strings"

	"github.com/majd/ipatool/v2/pkg/http"
)

const (
	downloadDispatchDomain = "downloaddispatch." + iTunesAPIDomain
	updateProductPath      = "/up/updateProduct"
)

// isEmptyRedownloadError reports a redownload endpoint failure that carries no
// payload at all: an empty HTTP 500 body.
func isEmptyRedownloadError(err error) bool {
	var unexpected *http.UnexpectedResponseError

	return errors.As(err, &unexpected) &&
		unexpected.StatusCode == gohttp.StatusInternalServerError && unexpected.Snippet == ""
}

// Limit recovery to the observed availability response; other customer messages
// and structured failures must retain their normal error handling.
func isUnavailableDownloadProductResponse(res http.Result[downloadResult]) bool {
	message := strings.ToLower(strings.TrimSpace(res.Data.CustomerMessage))

	return res.StatusCode == gohttp.StatusOK &&
		res.Data.FailureType == "" && len(res.Data.Items) == 0 &&
		(message == "no longer available" || strings.HasSuffix(message, " no longer available"))
}

// sendUpdateProduct asks the bag's updateProduct endpoint for a pinned
// version. It can serve pinned iOS, macOS, and tvOS versions when redownload
// returns an empty HTTP 500 or a message-only availability error. The request
// keeps the same session and version selection as the redownload attempt.
func (t *appstore) sendUpdateProduct(endpoint string, acc Account, app App, guid, externalVersionID string) (http.Result[downloadResult], error) {
	update, err := newDownloadEndpoint(endpoint, updateProductPath)
	if err != nil {
		return http.Result[downloadResult]{}, err
	}

	payload := map[string]interface{}{
		"creditDisplay": "",
		"guid":          guid,
		"salableAdamId": app.ID,
		"serialNumber":  "0",
		"appExtVrsId":   externalVersionID,
	}

	res, err := t.downloadClient.Send(http.Request{
		URL:            fmt.Sprintf("%s?guid=%s", update.baseURL, guid),
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
	})
	if err != nil {
		return res, fmt.Errorf("failed to send update request: %w", err)
	}

	if res.Data.FailureType != "" {
		return res, nil
	}

	if res.Data.CustomerMessage != "" {
		return res, NewErrorWithMetadata(fmt.Errorf("received update error: %s", res.Data.CustomerMessage), res)
	}

	if res.StatusCode != gohttp.StatusOK {
		return res, fmt.Errorf("received unexpected update status code: %d", res.StatusCode)
	}

	if len(res.Data.Items) != 1 {
		return res, errors.New("update response must contain exactly one item")
	}

	metadata := res.Data.Items[0].Metadata
	if fmt.Sprint(metadata["itemId"]) != fmt.Sprint(app.ID) ||
		fmt.Sprint(metadata["softwareVersionExternalIdentifier"]) != externalVersionID {
		return res, errors.New("update response does not match the requested app or version")
	}

	bundleID, ok := metadata["softwareVersionBundleId"].(string)
	if !ok || bundleID == "" || (app.BundleID != "" && bundleID != app.BundleID) {
		return res, errors.New("update response does not match the requested bundle identifier")
	}

	return res, nil
}

type downloadProductEndpoint struct {
	baseURL string
}

// newDownloadEndpoint validates a bag-provided download endpoint. Only the
// trusted download dispatch domain is accepted.
func newDownloadEndpoint(endpoint, path string) (downloadProductEndpoint, error) {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host != downloadDispatchDomain ||
		parsed.Path != path || parsed.RawPath != "" || parsed.RawQuery != "" ||
		parsed.ForceQuery || parsed.Fragment != "" || parsed.User != nil {
		return downloadProductEndpoint{}, errors.New("invalid download endpoint in bag")
	}

	return downloadProductEndpoint{
		baseURL: endpoint,
	}, nil
}
