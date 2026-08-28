//go:build darwin && cgo

package mescal

import (
	"os"
	"testing"
)

func TestAvailable(t *testing.T) {
	if !Available() {
		t.Fatal("Available() returned false on darwin with cgo enabled")
	}
}

func TestSignIntegration(t *testing.T) {
	if os.Getenv("IPATOOL_TEST_APPLE_SAP") != "1" {
		t.Skip("set IPATOOL_TEST_APPLE_SAP=1 to test macOS CommerceKit signing")
	}

	signature, err := Sign([]byte("ipatool SAP integration test"))
	if err != nil {
		t.Fatalf("Sign() returned an error: %v", err)
	}

	if len(signature) == 0 {
		t.Fatal("Sign() returned an empty signature")
	}
}
