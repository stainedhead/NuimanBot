// Package storage provides infrastructure-layer repository implementations.
package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// tokenRefreshBuffer is the duration before JWT expiry at which a proactive refresh is triggered.
const tokenRefreshBuffer = 5 * time.Minute

// IngatanClientConfig holds the construction parameters for IngatanHTTPClient.
type IngatanClientConfig struct {
	// BaseURL is the root URL of the Ingatan server (e.g. "https://localhost:8443").
	BaseURL string
	// APIKey is the Ingatan API key used to exchange for a JWT.
	APIKey string
	// StorePrefix is the prefix for store names (default: "nuiman").
	StorePrefix string
	// TLSSkipVerify disables TLS certificate verification. For development only.
	TLSSkipVerify bool
}

// TokenCache caches an Ingatan JWT and its expiry time.
// It is safe for concurrent use via sync.RWMutex.
type TokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

// needsRefresh reports true if the token is absent or will expire within tokenRefreshBuffer.
func (c *TokenCache) needsRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.token == "" {
		return true
	}
	return time.Until(c.expiresAt) < tokenRefreshBuffer
}

// IngatanHTTPClient is an HTTP client for the Ingatan REST API.
// It automatically exchanges an API key for a JWT and transparently refreshes
// the token before expiry.
type IngatanHTTPClient struct {
	baseURL    string
	httpClient *http.Client
	tokenCache *TokenCache
	// refreshMu serializes concurrent refresh attempts to prevent redundant token exchanges.
	refreshMu   sync.Mutex
	apiKey      string
	storePrefix string
}

// NewIngatanHTTPClient creates a new IngatanHTTPClient from the provided configuration.
// If TLSSkipVerify is true, a warning is logged and TLS certificate verification is disabled.
func NewIngatanHTTPClient(cfg IngatanClientConfig) *IngatanHTTPClient {
	transport := &http.Transport{}
	if cfg.TLSSkipVerify {
		slog.Warn("ingatan: TLSSkipVerify is enabled — TLS certificate verification is disabled; do not use in production")
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // intentional: dev-only skip
	}

	return &IngatanHTTPClient{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		tokenCache:  &TokenCache{},
		apiKey:      cfg.APIKey,
		storePrefix: cfg.StorePrefix,
	}
}

// Do executes an authenticated HTTP request against the Ingatan API.
// It transparently refreshes the JWT when needed before making the request.
// Double-checked locking ensures that only one goroutine performs the token
// exchange even when multiple goroutines call Do concurrently with an expired token.
func (c *IngatanHTTPClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if c.tokenCache.needsRefresh() {
		c.refreshMu.Lock()
		// Re-check after acquiring the lock — another goroutine may have already
		// refreshed the token while this goroutine was waiting.
		var refreshErr error
		if c.tokenCache.needsRefresh() {
			refreshErr = c.refresh(ctx)
		}
		c.refreshMu.Unlock()
		if refreshErr != nil {
			return nil, refreshErr
		}
	}

	c.tokenCache.mu.RLock()
	token := c.tokenCache.token
	c.tokenCache.mu.RUnlock()

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("ingatan: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ingatan: do request: %w", err)
	}
	return resp, nil
}

// Ping checks whether the Ingatan server is reachable by calling GET /api/v1/health.
// It does not require a valid JWT — the health endpoint is unauthenticated.
func (c *IngatanHTTPClient) Ping(ctx context.Context) error {
	url := c.baseURL + "/api/v1/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("ingatan: ping: build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ingatan: ping: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go
	if resp.StatusCode >= 500 {
		return fmt.Errorf("ingatan: ping: server returned %d", resp.StatusCode)
	}
	return nil
}

// tokenExchangeRequest is the body sent to POST /auth/token.
type tokenExchangeRequest struct {
	APIKey string `json:"api_key"`
}

// tokenExchangeResponse is the body returned by POST /auth/token.
type tokenExchangeResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// validateTokenResponse validates a tokenExchangeResponse from the Ingatan auth endpoint.
// It returns the parsed expiry time and an error if the response is unusable.
// All errors are prefixed with "ingatan: token exchange: ".
func validateTokenResponse(resp tokenExchangeResponse) (time.Time, error) {
	expiresAt, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("ingatan: token exchange: parse expires_at %q: %w", resp.ExpiresAt, err)
	}
	if resp.Token == "" {
		return time.Time{}, fmt.Errorf("ingatan: token exchange: server returned empty token")
	}
	if !expiresAt.After(time.Now()) {
		return time.Time{}, fmt.Errorf("ingatan: token exchange: server returned already-expired token (expires_at=%s)", resp.ExpiresAt)
	}
	return expiresAt, nil
}

// refresh exchanges the API key for a fresh JWT and stores it in the token cache.
func (c *IngatanHTTPClient) refresh(ctx context.Context) error {
	payload, err := json.Marshal(tokenExchangeRequest{APIKey: c.apiKey})
	if err != nil {
		return fmt.Errorf("ingatan: token exchange: marshal request: %w", err)
	}

	url := c.baseURL + "/auth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ingatan: token exchange: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ingatan: token exchange: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ingatan: token exchange: unexpected status %d", resp.StatusCode)
	}

	var tokenResp tokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("ingatan: token exchange: decode response: %w", err)
	}

	expiresAt, err := validateTokenResponse(tokenResp)
	if err != nil {
		return err
	}

	c.tokenCache.mu.Lock()
	c.tokenCache.token = tokenResp.Token
	c.tokenCache.expiresAt = expiresAt
	c.tokenCache.mu.Unlock()

	return nil
}
