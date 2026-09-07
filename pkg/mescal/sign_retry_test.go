package mescal

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func withNoopSignerSleep(t *testing.T) {
	t.Helper()

	oldSleep := signerSleep
	signerSleep = func(time.Duration) {}
	t.Cleanup(func() { signerSleep = oldSleep })
}

func TestSignWithRetrySucceedsAfterTransientFailure(t *testing.T) {
	withNoopSignerSleep(t)

	calls := 0
	sign := func() ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("sapsigner failed: exit status 2: panic: Post: net/http: TLS handshake timeout")
		}

		return []byte("sig"), nil
	}

	signature, err := signWithRetry(sign)
	if err != nil {
		t.Fatalf("signWithRetry() returned error: %v", err)
	}

	if string(signature) != "sig" {
		t.Fatalf("signWithRetry() = %q, want %q", signature, "sig")
	}

	if calls != 2 {
		t.Fatalf("sign called %d times, want 2", calls)
	}
}

func TestSignWithRetryGivesUpAfterMaxAttempts(t *testing.T) {
	withNoopSignerSleep(t)

	original := errors.New("sapsigner failed: exit status 2: panic: Post: net/http: TLS handshake timeout")

	calls := 0
	sign := func() ([]byte, error) {
		calls++
		return nil, original
	}

	_, err := signWithRetry(sign)
	if !errors.Is(err, original) {
		t.Fatalf("signWithRetry() error does not wrap the original error: %v", err)
	}

	if calls != maxSignAttempts {
		t.Fatalf("sign called %d times, want %d", calls, maxSignAttempts)
	}

	want := fmt.Sprintf("after %d attempts", maxSignAttempts)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("signWithRetry() error %q, want it to contain %q", err, want)
	}
}

func TestSignWithRetryDoesNotRetryPermanentFailure(t *testing.T) {
	withNoopSignerSleep(t)

	original := errors.New("sapsigner failed: exit status 2: sap framework not found")

	calls := 0
	sign := func() ([]byte, error) {
		calls++
		return nil, original
	}

	_, err := signWithRetry(sign)
	if !errors.Is(err, original) {
		t.Fatalf("signWithRetry() error does not wrap the original error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("sign called %d times, want 1", calls)
	}
}

func TestSignWithRetryReturnsSignatureFromFinalAttempt(t *testing.T) {
	withNoopSignerSleep(t)

	calls := 0
	sign := func() ([]byte, error) {
		calls++
		if calls < maxSignAttempts {
			return nil, fmt.Errorf("connection reset by peer")
		}

		return []byte("sig"), nil
	}

	signature, err := signWithRetry(sign)
	if err != nil {
		t.Fatalf("signWithRetry() returned error: %v", err)
	}

	if string(signature) != "sig" {
		t.Fatalf("signWithRetry() = %q, want %q", signature, "sig")
	}
}

func TestIsTransientSignerError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unavailable sentinel", err: ErrUnavailable, want: true},
		{name: "wrapped unavailable sentinel", err: fmt.Errorf("commerce kit: %w", ErrUnavailable), want: true},
		{
			name: "sapsigner tls handshake timeout panic",
			err:  errors.New("sapsigner failed: exit status 2: panic: Post \"https://play.itunes.apple.com/WebObjects/MZPlay.woa/wa/signSapSetup\": net/http: TLS handshake timeout"),
			want: true,
		},
		{name: "timeout", err: errors.New("request timeout after 30s"), want: true},
		{name: "connection reset", err: errors.New("read tcp: connection reset by peer"), want: true},
		{name: "missing framework", err: errors.New("sapsigner failed: exit status 2: sap framework not found"), want: false},
		{name: "empty signature", err: errors.New("sapsigner returned an empty SAP signature"), want: false},
		{name: "helper not found", err: errors.New("the Apple SAP signing helper (sapsigner.exe) was not found"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientSignerError(tc.err); got != tc.want {
				t.Fatalf("isTransientSignerError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
