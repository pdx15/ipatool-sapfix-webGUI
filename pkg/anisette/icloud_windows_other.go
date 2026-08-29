//go:build windows && !amd64

package anisette

import (
	"os"
	"path/filepath"
)

const (
	icloudClassicDownloadURL = "https://support.apple.com/en-us/103121"
	icloudStoreDownloadURL   = "https://apps.microsoft.com/detail/9PKT5699M62"
)

// checkICloud on 32-bit Windows only probes the classic iCloud install,
// because the Microsoft Store package layout check lives in an amd64-only file.
func checkICloud() ICloudStatus {
	if findClassicServicesDir() != "" {
		return ICloudStatus{
			Installed:   true,
			Variant:     "classic",
			DownloadURL: icloudClassicDownloadURL,
		}
	}
	return ICloudStatus{
		DownloadURL: icloudStoreDownloadURL,
	}
}

func findClassicServicesDir() string {
	for _, root := range programFilesCommonDirs() {
		services := filepath.Join(root, "Apple", "Internet Services")
		if _, err := os.Stat(filepath.Join(services, "AOSKit.dll")); err == nil {
			return services
		}
	}
	return ""
}

func programFilesCommonDirs() []string {
	var roots []string
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		roots = append(roots, filepath.Join(pf, "Common Files"))
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		roots = append(roots, filepath.Join(pf86, "Common Files"))
	}
	return roots
}
