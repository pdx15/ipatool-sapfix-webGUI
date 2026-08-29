//go:build windows

package anisette

import (
	"context"
	"fmt"
	"net/http"
)

// cString converts a fixed-size byte buffer containing a NUL-terminated C
// string into a Go string.
func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}

	return string(b)
}

// windowsProvider extracts anisette data from a locally installed iCloud.
// The classic "2020" iCloud build is preferred (P1); the Microsoft Store
// build is used as a fallback.
type windowsProvider struct{}

// NewProvider returns the anisette provider for Windows. The http client is
// unused on this platform and may be nil.
func NewProvider(_ *http.Client) Provider {
	return &windowsProvider{}
}

// Fetch tries classic iCloud first, then falls back to Microsoft Store iCloud.
func (p *windowsProvider) Fetch(_ context.Context) (Data, error) {
	data, err := fetchClassic()
	if err == nil && data.Complete() {
		return data.WithDefaults(), nil
	}

	classicErr := err

	data, err = fetchMSStore()
	if err == nil && data.Complete() {
		return data.WithDefaults(), nil
	}

	return Data{}, fmt.Errorf(
		"anisette: failed to extract anisette data: "+
			"classic iCloud: %v; Microsoft Store iCloud: %v",
		classicErr, err)
}
