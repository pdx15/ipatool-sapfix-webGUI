//go:build !windows && (!darwin || !cgo)

package mescal

import "fmt"

// Sign creates the binary SAP signature Apple expects for protected Store
// actions.
func Sign(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("%w: macOS with cgo enabled is required", ErrUnavailable)
}
