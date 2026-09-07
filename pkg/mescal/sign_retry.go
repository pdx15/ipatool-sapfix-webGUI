package mescal

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Apple's SAP service is intermittently unreachable, and the signing helpers
// treat any failure as fatal (the Windows helper panics on a TLS handshake
// timeout). Signing is a pure, idempotent operation — the same input bytes
// always produce the same signature — so transient signer failures are
// retried transparently.
const (
	maxSignAttempts    = 3
	signRetryDelayBase = 250 * time.Millisecond
)

// signerSleep is the backoff between signing attempts. Tests replace it
// with a no-op.
var signerSleep = time.Sleep

// signWithRetry runs sign up to maxSignAttempts times, retrying only
// failures that look transient (a network hiccup while the helper talks to
// Apple, or an explicit service-unavailable from the macOS signer).
func signWithRetry(sign func() ([]byte, error)) ([]byte, error) {
	attempts := 0
	var lastErr error

	for attempt := 1; attempt <= maxSignAttempts; attempt++ {
		attempts = attempt

		signature, err := sign()
		if err == nil {
			return signature, nil
		}

		lastErr = err

		if !isTransientSignerError(err) {
			break
		}

		signerSleep(time.Duration(attempt) * signRetryDelayBase)
	}

	if attempts == maxSignAttempts {
		return nil, fmt.Errorf("signing failed after %d attempts: %w", attempts, lastErr)
	}

	return nil, lastErr
}

// isTransientSignerError reports whether a signer failure looks transient
// and worth retrying. The Windows helper panics on the first network error,
// so its stderr text is the only signal available there.
func isTransientSignerError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrUnavailable) {
		return true
	}

	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"timeout",
		"tls handshake",
		"connection reset",
		"connection refused",
		"unexpected eof",
		"temporarily unavailable",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}

	return false
}
