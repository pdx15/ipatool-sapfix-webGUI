//go:build windows && !amd64

package anisette

import "errors"

// fetchMSStore is only implemented for 64-bit Windows: the Microsoft Store
// iCloud package is 64-bit, and the offset-based AOSKit calls rely on the
// uniform x64 calling convention.
func fetchMSStore() (Data, error) {
	return Data{}, errors.New("Microsoft Store iCloud anisette is only supported on 64-bit Windows")
}
