//go:build !windows

package anisette

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Public servers (SideStore-style) used when no local iCloud install is
// available. Public servers are often flaky, so each is tried in turn across
// two passes before giving up.
var publicServers = []string{
	"https://ani.sidestore.io",
	"https://ani.f1sh.me",
	"https://ani.npeg.us",
	"https://ani.sidestore.app",
	"https://ani.846969.xyz",
	"https://anisette.wedotstud.io",
	"https://ani.neoarz.com",
	"https://ani3server.fly.dev",
	"https://ani.jaydenha.uk",
	"https://anisette.crystall1ne.dev",
}

// PublicServerProvider fetches anisette data from public servers.
type PublicServerProvider struct {
	HTTPClient *http.Client
}

// NewProvider returns the anisette provider for this platform. On non-Windows
// systems there is no local iCloud install to extract anisette from, so a
// public server is used.
func NewProvider(httpClient *http.Client) Provider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &PublicServerProvider{HTTPClient: httpClient}
}

// Fetch queries public anisette servers until one responds with a complete
// data set.
func (p *PublicServerProvider) Fetch(ctx context.Context) (Data, error) {
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	var lastErr error

	// Two passes: public servers frequently 5xx transiently.
	for pass := 0; pass < 2; pass++ {
		for _, server := range publicServers {
			data, err := fetchFromServer(ctx, client, server)
			if err != nil {
				lastErr = err
				continue
			}

			if data.Complete() {
				return data, nil
			}

			lastErr = fmt.Errorf("anisette server %s returned incomplete data", server)
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no anisette servers configured")
	}

	return Data{}, fmt.Errorf("failed to fetch anisette data from public servers: %w", lastErr)
}

func fetchFromServer(ctx context.Context, client *http.Client, server string) (Data, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server, nil)
	if err != nil {
		return Data{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return Data{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Data{}, fmt.Errorf("anisette server %s returned HTTP %d", server, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Data{}, err
	}

	return ParseJSON(body), nil
}
