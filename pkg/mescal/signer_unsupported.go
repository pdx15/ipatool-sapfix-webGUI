//go:build !darwin || !cgo

package mescal

import "fmt"

// Available reports whether this platform can create the binary SAP
// signature Apple expects for protected Store actions.
func Available() bool {
	return false
}

// Sign creates the binary SAP signature Apple expects for protected Store
// actions.
func Sign(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("%w: macOS with cgo enabled is required", ErrUnavailable)
}
