package appstore

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/schollz/progressbar/v3"
	"howett.net/plist"
)

var (
	ErrLicenseRequired = errors.New("license is required")
)

type DownloadInput struct {
	Account           Account
	App               App
	OutputPath        string
	Progress          *progressbar.ProgressBar
	ExternalVersionID string
	Platform          Platform
	// ProgressWriter, when set, receives the raw downloaded bytes so callers
	// (e.g. the GUI progress tracker) can show byte/percentage progress. This is
	// separate from Progress, which is the CLI progress-bar renderer.
	ProgressWriter io.Writer
	// OnTotalBytes, when set, is invoked once the total size of the package
	// (in bytes) is known so callers (e.g. the GUI progress tracker) can show
	// a percentage and weight while the download is in progress.
	OnTotalBytes func(total int64)
}

// DownloadOutput describes the outcome of a completed download.
type DownloadOutput struct {
	DestinationPath string
	Sinfs           []Sinf
}

// checkDownloadResult is the outcome of one direct-download probe: identical to
// a real download request through the App Store download endpoint, but without
// transferring the package itself. The probe is used to distinguish apps whose
// license the account already holds (downloadable) from apps that have never
// been installed by this account (ErrLicenseRequired).
type CheckDownloadInput struct {
	Account  Account
	App      App
	Platform Platform
}

type CheckDownloadOutput struct {
	Version                    string
	ExternalVersionIdentifiers []string
	LatestExternalVersionID    string
}

// CheckDownload performs the same request Download sends (same endpoint, same
// payload, same failure classification) and stops once the response has been
// validated. It returns ErrLicenseRequired when the account has no license for
// the app, and nil with the response metadata when the app is downloadable.
func (t *appstore) CheckDownload(input CheckDownloadInput) (CheckDownloadOutput, error) {
	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return CheckDownloadOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	externalVersionID := ""
	if input.Platform == PlatformAppleTV {
		externalVersionID, err = t.lookupLatestExternalVersionID(input.Account, input.App, input.Platform)
		if err != nil {
			return CheckDownloadOutput{}, fmt.Errorf("failed to resolve platform version: %w", err)
		}
	}

	req := t.downloadRequest(input.Account, input.App, guid, externalVersionID)

	res, err := t.downloadClient.Send(req)
	if err != nil {
		return CheckDownloadOutput{}, fmt.Errorf("failed to send http request: %w", err)
	}

	item, err := classifyDownloadResponse(res)
	if err != nil {
		return CheckDownloadOutput{}, err
	}

	output := CheckDownloadOutput{
		Version: metadataString(item.Metadata, "bundleShortVersionString"),
	}

	rawIdentifiers, ok := item.Metadata["softwareVersionExternalIdentifiers"].([]interface{})
	if ok {
		output.ExternalVersionIdentifiers = make([]string, len(rawIdentifiers))
		for i, val := range rawIdentifiers {
			output.ExternalVersionIdentifiers[i] = fmt.Sprintf("%v", val)
		}
	}

	if latest, ok := item.Metadata["softwareVersionExternalIdentifier"]; ok && latest != nil {
		output.LatestExternalVersionID = fmt.Sprintf("%v", latest)
	}

	return output, nil
}

// classifyDownloadResponse applies the exact failure-type classification used
// by Download to a response of the App Store download endpoint.
func classifyDownloadResponse(res http.Result[downloadResult]) (downloadItemResult, error) {
	if res.Data.FailureType == FailureTypePasswordTokenExpired ||
		res.Data.FailureType == FailureTypeSignInRequired ||
		res.Data.FailureType == FailureTypeDeviceVerificationFailed {
		return downloadItemResult{}, ErrPasswordTokenExpired
	}

	// FailureType 5002 (LicenseAlreadyExists) was previously mapped to
	// ErrPasswordTokenExpired, but this caused re-authentication loops.
	// With serialNumber: "0" in the payload, this error should not occur
	// for valid downloads. If it does occur, it indicates a real problem
	// that should be surfaced to the user.
	if res.Data.FailureType == FailureTypeLicenseAlreadyExists {
		message := "license already exists"
		if res.Data.CustomerMessage != "" {
			message = res.Data.CustomerMessage
		}
		return downloadItemResult{}, NewErrorWithMetadata(errors.New(message), res)
	}

	if res.Data.FailureType == FailureTypeLicenseNotFound {
		return downloadItemResult{}, ErrLicenseRequired
	}

	if res.Data.FailureType != "" && res.Data.CustomerMessage != "" {
		return downloadItemResult{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.CustomerMessage), res)
	}

	if res.Data.FailureType != "" {
		return downloadItemResult{}, NewErrorWithMetadata(fmt.Errorf("received error: %s", res.Data.FailureType), res)
	}

	if len(res.Data.Items) == 0 {
		// Log full response for debugging
		errMsg := "invalid response"
		if res.Data.CustomerMessage != "" {
			errMsg = fmt.Sprintf("invalid response: %s", res.Data.CustomerMessage)
		}
		if res.Data.FailureType != "" {
			errMsg = fmt.Sprintf("invalid response: failure type %s", res.Data.FailureType)
		}
		return downloadItemResult{}, NewErrorWithMetadata(errors.New(errMsg), res)
	}

	return res.Data.Items[0], nil
}

// metadataString reads a string-formatted value from the response metadata.
func metadataString(metadata map[string]interface{}, key string) string {
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}

	return fmt.Sprintf("%v", raw)
}

func (t *appstore) Download(input DownloadInput) (DownloadOutput, error) {
	macAddr, err := t.machine.MacAddress()
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to get mac address: %w", err)
	}

	guid := strings.ReplaceAll(strings.ToUpper(macAddr), ":", "")

	externalVersionID := input.ExternalVersionID
	if externalVersionID == "" && input.Platform == PlatformAppleTV {
		externalVersionID, err = t.lookupLatestExternalVersionID(input.Account, input.App, input.Platform)
		if err != nil {
			return DownloadOutput{}, fmt.Errorf("failed to resolve platform version: %w", err)
		}
	}

	req := t.downloadRequest(input.Account, input.App, guid, externalVersionID)

	res, err := t.downloadClient.Send(req)
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to send http request: %w", err)
	}

	item, err := classifyDownloadResponse(res)
	if err != nil {
		// If volumeStoreDownloadProduct returned empty Items[], retry primary once first
		if isEmptyResponseError(err) {
			res, err = t.downloadClient.Send(req)
			if err != nil {
				return DownloadOutput{}, fmt.Errorf("failed to retry http request: %w", err)
			}
			item, err = classifyDownloadResponse(res)
		}

		// If primary still fails with empty Items[], try redownload endpoint
		if isEmptyResponseError(err) {
			redownloadReq := t.redownloadRequest(input.Account, input.App, guid, externalVersionID)
			redownloadRes, redownloadErr := t.downloadClient.Send(redownloadReq)
			if redownloadErr != nil {
				return DownloadOutput{}, fmt.Errorf("failed to send redownload request: %w", redownloadErr)
			}
			item, err = classifyDownloadResponse(redownloadRes)
			if err != nil {
				return DownloadOutput{}, fmt.Errorf("both download endpoints failed: %w", err)
			}
		} else if err != nil {
			return DownloadOutput{}, err
		}
	}

	version := "unknown"

	// Read the version from the item metadata
	if itemVersion, ok := item.Metadata["bundleShortVersionString"]; ok {
		version = fmt.Sprintf("%v", itemVersion)
	}

	// Read the minimum iOS version from the item metadata
	iosVersion := metadataString(item.Metadata, "minimumOsVersion")

	destination, err := t.resolveDestinationPath(input.App, version, iosVersion, input.Account.Email, input.OutputPath)
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to resolve destination path: %w", err)
	}

	tmpPath := fmt.Sprintf("%s.tmp", destination)

	err = t.downloadFile(item.URL, tmpPath, input.Progress, input.ProgressWriter, input.OnTotalBytes)
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to download file: %w", err)
	}

	err = t.applyPatches(item, input.Account, tmpPath, destination)
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to apply patches: %w", err)
	}

	err = t.validatePackagePlatform(destination, input.Platform)
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to validate package platform: %w", err)
	}

	// If iOS version was not available from API metadata, try to read it from the IPA's Info.plist
	originalDestination := destination
	if iosVersion == "" {
		if actualIOSVersion, readErr := t.readMinimumOSVersionFromIPA(destination); readErr == nil && actualIOSVersion != "" {
			iosVersion = actualIOSVersion
			// Rename file with the correct iOS version
			newDestination, err := t.resolveDestinationPath(input.App, version, iosVersion, input.Account.Email, input.OutputPath)
			if err == nil && newDestination != destination {
				if renameErr := os.Rename(destination, newDestination); renameErr == nil {
					destination = newDestination
				}
			}
		}
	}

	// Extract app name from Info.plist and rename file
	if appName, readErr := t.readAppNameFromIPA(destination); readErr == nil && appName != "" {
		if newDestination, renameErr := t.renameWithAppName(destination, appName, version, iosVersion, input.Account.Email); renameErr == nil {
			destination = newDestination
		}
	}

	err = t.os.Remove(fmt.Sprintf("%s.tmp", originalDestination))
	if err != nil {
		return DownloadOutput{}, fmt.Errorf("failed to remove file: %w", err)
	}

	return DownloadOutput{
		DestinationPath: destination,
		Sinfs:           item.Sinfs,
	}, nil
}

type platformPackageInfo struct {
	SupportedPlatforms []string `plist:"CFBundleSupportedPlatforms,omitempty"`
}

func (*appstore) validatePackagePlatform(path string, platform Platform) error {
	if platform != PlatformAppleTV {
		return nil
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "Payload/") || !strings.HasSuffix(file.Name, ".app/Info.plist") {
			continue
		}

		infoFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open info plist: %w", err)
		}

		data, readErr := io.ReadAll(infoFile)
		closeErr := infoFile.Close()

		if readErr != nil {
			return fmt.Errorf("failed to read info plist: %w", readErr)
		}

		if closeErr != nil {
			return fmt.Errorf("failed to close info plist: %w", closeErr)
		}

		var info platformPackageInfo

		_, err = plist.Unmarshal(data, &info)
		if err != nil {
			return fmt.Errorf("failed to decode info plist: %w", err)
		}

		for _, supportedPlatform := range info.SupportedPlatforms {
			if supportedPlatform == "AppleTVOS" {
				return nil
			}
		}
	}

	return errors.New("downloaded package does not declare AppleTVOS support")
}

type downloadItemResult struct {
	HashMD5  string                 `plist:"md5,omitempty"`
	URL      string                 `plist:"URL,omitempty"`
	Sinfs    []Sinf                 `plist:"sinfs,omitempty"`
	Metadata map[string]interface{} `plist:"metadata,omitempty"`
}

type downloadResult struct {
	FailureType     string               `plist:"failureType,omitempty"`
	CustomerMessage string               `plist:"customerMessage,omitempty"`
	Items           []downloadItemResult `plist:"songList,omitempty"`
}

func (t *appstore) downloadFile(src, dst string, progress *progressbar.ProgressBar, progressWriter io.Writer, onTotal func(int64)) error {
	req, err := t.httpClient.NewRequest("GET", src, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	file, err := t.os.OpenFile(dst, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	defer file.Close()

	stat, err := t.os.Stat(dst)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if req != nil && stat != nil {
		req.Header.Add("range", fmt.Sprintf("bytes=%d-", stat.Size()))
	}

	res, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	total := res.ContentLength + stat.Size()
	if onTotal != nil {
		onTotal(total)
	}

	if progress != nil {
		progress.ChangeMax64(total)
		err = progress.Set64(stat.Size())
		if err != nil {
			return fmt.Errorf("can not set bar progress: %w", err)
		}
	}

	// Seek to the end so a resumed download appends after the already-downloaded
	// range (for a fresh file this is a no-op at offset 0).
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("can not seek file: %w", err)
	}

	// The CLI progress bar (progress) and any raw-bytes tracker (progressWriter,
	// e.g. the GUI progress tracker) each receive the raw downloaded bytes.
	writers := []io.Writer{file}
	if progress != nil {
		writers = append(writers, progress)
	}
	if progressWriter != nil {
		writers = append(writers, progressWriter)
	}
	_, err = io.Copy(io.MultiWriter(writers...), res.Body)

	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (*appstore) downloadRequest(acc Account, app App, guid string, externalVersionID string) http.Request {
	payload := map[string]interface{}{
		"creditDisplay": "",
		"guid":          guid,
		"salableAdamId": app.ID,
		"serialNumber":  "0",
	}

	if externalVersionID != "" {
		payload["externalVersionId"] = externalVersionID
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

// redownloadRequest creates a request to the redownload endpoint as a fallback
// when volumeStoreDownloadProduct returns empty Items[] for certain apps
// (e.g. Instagram, Microsoft Teams). The redownload endpoint uses appExtVrsId
// instead of externalVersionId for version pinning.
func (*appstore) redownloadRequest(acc Account, app App, guid string, externalVersionID string) http.Request {
	payload := map[string]interface{}{
		"creditDisplay": "",
		"guid":          guid,
		"salableAdamId": app.ID,
		"serialNumber":  "0",
	}

	if externalVersionID != "" {
		payload["appExtVrsId"] = externalVersionID
	}

	// Note: redownload endpoint does not use pod prefix
	return http.Request{
		URL:            fmt.Sprintf("https://downloaddispatch.itunes.apple.com/r/redownload?guid=%s", guid),
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

// isEmptyResponseError checks if the error indicates an empty Items[] response
// that could be retried with the redownload endpoint.
func isEmptyResponseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "invalid response") && !strings.Contains(errStr, "received error")
}
// accountUsername returns the local part of an email address (everything
// before the '@'). When the address has no '@', the full string is returned.
func accountUsername(email string) string {
	if idx := strings.Index(email, "@"); idx >= 0 {
		return email[:idx]
	}
	return email
}

func fileName(app App, version string, iosVersion string, accountEmail string) string {
	var parts []string

	if app.BundleID != "" {
		parts = append(parts, app.BundleID)
	}

	if app.ID != 0 {
		parts = append(parts, strconv.FormatInt(app.ID, 10))
	}

	if version != "" {
		parts = append(parts, version)
	}

	if iosVersion != "" {
		parts = append(parts, fmt.Sprintf("iOS%s", iosVersion))
	}

	if accountEmail != "" {
		parts = append(parts, accountUsername(accountEmail))
	}

	return fmt.Sprintf("%s.ipa", strings.Join(parts, "_"))
}

func (t *appstore) resolveDestinationPath(app App, version string, iosVersion string, accountEmail string, path string) (string, error) {
	file := fileName(app, version, iosVersion, accountEmail)

	if path == "" {
		workdir, err := t.os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}

		return fmt.Sprintf("%s/%s", workdir, file), nil
	}

	isDir, err := t.isDirectory(path)
	if err != nil {
		return "", fmt.Errorf("failed to determine whether path is a directory: %w", err)
	}

	if isDir {
		return fmt.Sprintf("%s/%s", path, file), nil
	}

	return path, nil
}

func (t *appstore) isDirectory(path string) (bool, error) {
	info, err := t.os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read file metadata: %w", err)
	}

	if info == nil {
		return false, nil
	}

	return info.IsDir(), nil
}

func (t *appstore) applyPatches(item downloadItemResult, acc Account, src, dst string) error {
	srcZip, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip reader: %w", err)
	}
	defer srcZip.Close()

	dstFile, err := t.os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer dstFile.Close()

	dstZip := zip.NewWriter(dstFile)
	defer dstZip.Close()

	err = t.replicateZip(srcZip, dstZip)
	if err != nil {
		return fmt.Errorf("failed to replicate zip: %w", err)
	}

	err = t.writeMetadata(item.Metadata, acc, dstZip)
	if err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// readMinimumOSVersionFromIPA reads the MinimumOSVersion from the Info.plist inside the IPA file.
func (t *appstore) readMinimumOSVersionFromIPA(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open zip reader: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "Payload/") || !strings.HasSuffix(file.Name, ".app/Info.plist") {
			continue
		}

		infoFile, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open info plist: %w", err)
		}

		data, readErr := io.ReadAll(infoFile)
		closeErr := infoFile.Close()

		if readErr != nil {
			return "", fmt.Errorf("failed to read info plist: %w", readErr)
		}

		if closeErr != nil {
			return "", fmt.Errorf("failed to close info plist: %w", closeErr)
		}

		var info map[string]interface{}
		_, err = plist.Unmarshal(data, &info)
		if err != nil {
			return "", fmt.Errorf("failed to decode info plist: %w", err)
		}

		return readMinimumOSVersionFromMetadata(info), nil
	}

	return "", errors.New("Info.plist not found in IPA")
}

// readAppNameFromIPA extracts the app name from Info.plist inside the IPA file.
// It tries CFBundleDisplayName first, then CFBundleName as fallback.
func (t *appstore) readAppNameFromIPA(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open zip reader: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "Payload/") || !strings.HasSuffix(file.Name, ".app/Info.plist") {
			continue
		}

		infoFile, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open info plist: %w", err)
		}

		data, readErr := io.ReadAll(infoFile)
		closeErr := infoFile.Close()

		if readErr != nil {
			return "", fmt.Errorf("failed to read info plist: %w", readErr)
		}

		if closeErr != nil {
			return "", fmt.Errorf("failed to close info plist: %w", closeErr)
		}

		var info map[string]interface{}
		_, err = plist.Unmarshal(data, &info)
		if err != nil {
			return "", fmt.Errorf("failed to decode info plist: %w", err)
		}

		// Try CFBundleDisplayName first, then CFBundleName
		if name, ok := info["CFBundleDisplayName"].(string); ok && name != "" {
			return name, nil
		}
		if name, ok := info["CFBundleName"].(string); ok && name != "" {
			return name, nil
		}

		return "", errors.New("app name not found in Info.plist")
	}

	return "", errors.New("Info.plist not found in IPA")
}

// renameWithAppName renames the IPA file to use the app name instead of bundle ID.
// Format: AppName_Version_iOS_Account.ipa
func (t *appstore) renameWithAppName(oldPath, appName, version, iosVersion, accountEmail string) (string, error) {
	var parts []string

	if appName != "" {
		parts = append(parts, appName)
	}

	if version != "" {
		parts = append(parts, version)
	}

	if iosVersion != "" {
		parts = append(parts, fmt.Sprintf("iOS%s", iosVersion))
	}

	if accountEmail != "" {
		parts = append(parts, accountUsername(accountEmail))
	}

	newFileName := fmt.Sprintf("%s.ipa", strings.Join(parts, "_"))
	
	// Get directory from old path
	dir := ""
	if idx := strings.LastIndex(oldPath, "/"); idx >= 0 {
		dir = oldPath[:idx+1]
	} else if idx := strings.LastIndex(oldPath, "\\"); idx >= 0 {
		dir = oldPath[:idx+1]
	}
	
	newPath := dir + newFileName

	err := os.Rename(oldPath, newPath)
	if err != nil {
		return oldPath, fmt.Errorf("failed to rename file: %w", err)
	}

	return newPath, nil
}

func (t *appstore) writeMetadata(metadata map[string]interface{}, acc Account, zip *zip.Writer) error {
	metadata["apple-id"] = acc.Email
	metadata["userName"] = acc.Email

	metadataFile, err := zip.Create("iTunesMetadata.plist")
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	data, err := plist.Marshal(metadata, plist.BinaryFormat)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	_, err = metadataFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}
