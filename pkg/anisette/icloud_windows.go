//go:build windows

package anisette

import (
	"os"
	"path/filepath"
)

// icloudClassicDownloadURL is the official Apple installer for the classic
// (non-Microsoft Store) iCloud for Windows build that ships AOSKit.dll.
const icloudClassicDownloadURL = "https://updates.cdn-apple.com/2020/windows/001-39935-20200911-1A70AA56-F448-11EA-8CC0-99D41950005E/iCloudSetup.exe"

func checkICloud() ICloudStatus {
	// The classic iCloud install is the supported anisette source on Windows:
	// Common Files\Apple\Internet Services\AOSKit.dll in either the 64-bit or
	// 32-bit Program Files location. The Microsoft Store build is NOT used.
	if services := findClassicServicesDir(); services != "" {
		return ICloudStatus{
			Installed:   true,
			Variant:     "classic",
			DownloadURL: icloudClassicDownloadURL,
		}
	}

	return ICloudStatus{
		DownloadURL: icloudClassicDownloadURL,
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
