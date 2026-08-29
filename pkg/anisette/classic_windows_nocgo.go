//go:build windows && !cgo

package anisette

import "errors"

// fetchClassic requires cgo to call the Objective-C runtime in objc.dll via
// the cdecl calling convention used by 32-bit classic iCloud.
func fetchClassic() (Data, error) {
	return Data{}, errors.New(
		"classic iCloud anisette requires a cgo-enabled Windows build; " +
			"build with CGO_ENABLED=1 (a C compiler such as mingw-w64 is required)")
}
