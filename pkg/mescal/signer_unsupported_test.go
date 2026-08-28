//go:build !darwin || !cgo

package mescal

import (
	"errors"
	"testing"
)

func TestAvailable(t *testing.T) {
	if Available() {
		t.Fatal("Available() returned true on an unsupported platform")
	}
}

func TestSignUnsupported(t *testing.T) {
	_, err := Sign([]byte("ipatool SAP test"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Sign() error = %v, want ErrUnavailable", err)
	}
}
