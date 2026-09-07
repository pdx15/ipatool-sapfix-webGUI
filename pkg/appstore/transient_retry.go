package appstore

import (
	"errors"
	"fmt"
	gohttp "net/http"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/majd/ipatool/v2/pkg/http"
)

// Apple's edge infrastructure intermittently answers descriptor, catalog and
// purchase-history requests with gateway errors (most visibly an HTTP 504
// with an HTML error page) that clear on the very next attempt. The read-only
// clients created in NewAppStore are wrapped with transientRetryClient so
// these responses are retried transparently (mirrors the upstream
// authentication retry pattern).
const (
	maxTransientAttempts    = 3
	transientRetryDelayBase = 250 * time.Millisecond
)

// retryableTransientError reports whether err is a transient Apple response
// worth retrying. HTTP 500 is deliberately excluded: an empty 500 from the
// redownload endpoint is a signal, not a failure — the issue #547 recovery
// (catalog lookup + version-pinned retry) relies on seeing it.
func retryableTransientError(err error) (int, bool) {
	var responseErr *http.UnexpectedResponseError
	if !errors.As(err, &responseErr) {
		return 0, false
	}

	status := responseErr.StatusCode

	retry := status == gohttp.StatusNoContent ||
		status == gohttp.StatusNotFound ||
		status == gohttp.StatusTooManyRequests ||
		status == gohttp.StatusBadGateway ||
		status == gohttp.StatusServiceUnavailable ||
		status == gohttp.StatusGatewayTimeout

	return status, retry
}

// transientRetryClient wraps a read-only http.Client and transparently
// retries Send on transient Apple responses (204, 404, 429, 502, 503, 504)
// with a short linear backoff.
type transientRetryClient[R interface{}] struct {
	inner http.Client[R]
	sleep func(time.Duration)
}

func newTransientRetryClient[R interface{}](inner http.Client[R]) http.Client[R] {
	return transientRetryClient[R]{inner: inner, sleep: time.Sleep}
}

func (c transientRetryClient[R]) Send(request http.Request) (http.Result[R], error) {
	sleep := c.sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	var statuses []string

	for attempt := 1; ; attempt++ {
		result, err := c.inner.Send(request)

		status, retry := retryableTransientError(err)
		if !retry {
			return result, err
		}

		statuses = append(statuses, strconv.Itoa(status))

		if attempt == maxTransientAttempts {
			return result, fmt.Errorf(
				"request failed after %d attempts (HTTP %s): %w",
				maxTransientAttempts, strings.Join(statuses, ", "), err,
			)
		}

		sleep(time.Duration(attempt) * transientRetryDelayBase)
	}
}

func (c transientRetryClient[R]) Do(req *gohttp.Request) (*gohttp.Response, error) {
	return c.inner.Do(req)
}

func (c transientRetryClient[R]) NewRequest(method, url string, body io.Reader) (*gohttp.Request, error) {
	return c.inner.NewRequest(method, url, body)
}
