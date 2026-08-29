//go:build windows && amd64

package anisette

import (
	"os"
	"path/filepath"
	"strings"
)

// icloudClassicDownloadURL and icloudStoreDownloadURL are the official places
// to install iCloud for Windows.
const (
	icloudClassicDownloadURL = "https://support.apple.com/en-us/103121"
	icloudStoreDownloadURL   = "https://apps.microsoft.com/detail/9PKTQ5699M62"
)

func checkICloud() ICloudStatus {
	// Microsoft Store iCloud: newest package under WindowsApps that ships
	// AOSKit.dll. We prefer it first because it is the current supported build.
	if dir, err := findICloudDir(); err == nil {
		version := storeVersionFromPath(dir)
		return ICloudStatus{
			Installed:   true,
			Variant:     "microsoft-store",
			Version:     version,
			DownloadURL: icloudStoreDownloadURL,
		}
	}

	// Classic iCloud: Common Files\Apple\Internet Services\AOSKit.dll in either
	// the 64-bit or 32-bit Program Files location.
	if services := findClassicServicesDir(); services != "" {
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

// findClassicServicesDir returns the path of the classic iCloud "Internet
// Services" directory that ships AOSKit.dll, or "" if none is found.
func findClassicServicesDir() string {
	for _, root := range programFilesCommonDirs() {
		services := filepath.Join(root, "Apple", "Internet Services")
		if _, err := os.Stat(filepath.Join(services, "AOSKit.dll")); err == nil {
			return services
		}
	}
	return ""
}

// programFilesCommonDirs returns the "Common Files" roots (64-bit then 32-bit)
// for the current machine.
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

// storeVersionFromPath extracts the version from a WindowsApps package path,
// e.g. "...\AppleInc.iCloud_15.3.80.0_x64__...\iCloud" -> "15.3.80.0".
func storeVersionFromPath(icloudDir string) string {
	parent := filepath.Base(filepath.Dir(icloudDir))
	parts := strings.Split(parent, "_")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}
