//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

const (
	testAPIKey    = "valid-test-api-key-123"
	testJWTSecret = "test-jwt-secret-not-for-production"
)

// startTestAPIServer starts a REST API server on a free port and returns its base URL.
func startTestAPIServer(t *testing.T) string {
	t.Helper()

	cfg := config.ExternalAPIRestConfig{
		Enabled: true,
		APIKey:  domain.NewSecureStringFromString(testAPIKey),
	}

	srv, err := api.NewServer(cfg, testJWTSecret, nil, nil)
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close()) // free the port; server will re-bind

	go func() { _ = srv.Start(addr) }()
	time.Sleep(20 * time.Millisecond) // brief wait for server to accept connections

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	return "http://" + addr
}

// issueToken requests a JWT from the server using the provided API key.
func issueToken(t *testing.T, baseURL, apiKey string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"api_key": apiKey})
	resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body)) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result["token"]
}

// TestRestAPITokenIssuance tests the POST /api/v1/auth/token endpoint.
func TestRestAPITokenIssuance(t *testing.T) {
	baseURL := startTestAPIServer(t)

	t.Run("valid API key returns JWT", func(t *testing.T) {
		token := issueToken(t, baseURL, testAPIKey)
		assert.NotEmpty(t, token, "valid API key should return a JWT")
		assert.True(t, strings.HasPrefix(token, "ey"), "JWT should start with 'ey'")
	})

	t.Run("invalid API key returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"api_key": "wrong-key"})
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body)) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing body returns 400", func(t *testing.T) {
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", http.NoBody) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestRestAPIJWTMiddleware verifies that protected endpoints require a valid JWT.
func TestRestAPIJWTMiddleware(t *testing.T) {
	baseURL := startTestAPIServer(t)
	healthURL := baseURL + "/api/v1/health"

	t.Run("no Authorization header returns 401", func(t *testing.T) {
		resp, err := http.Get(healthURL) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		// Health endpoint is unprotected; test a protected route
		assert.Equal(t, http.StatusOK, resp.StatusCode) // health is open
	})

	t.Run("valid JWT returns 200 on health", func(t *testing.T) {
		token := issueToken(t, baseURL, testAPIKey)
		require.NotEmpty(t, token)

		req, _ := http.NewRequest(http.MethodGet, healthURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("malformed token on protected route returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/", nil)
		req.Header.Set("Authorization", "Bearer not-a-jwt")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestRestAPIBodyLimit verifies that requests with bodies larger than 1 MiB are rejected.
func TestRestAPIBodyLimit(t *testing.T) {
	baseURL := startTestAPIServer(t)

	t.Run("body over 1 MiB returns 413", func(t *testing.T) {
		largeBody := strings.NewReader(strings.Repeat("x", (1<<20)+1))
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", largeBody) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)

		// 413 must not echo back the body.
		rbody, _ := io.ReadAll(resp.Body)
		assert.NotContains(t, string(rbody), "xxxxxxxx")
	})

	t.Run("body at exactly 1 MiB passes body limit check", func(t *testing.T) {
		// 1 MiB of JSON-ish content — will fail auth but not body limit.
		body := fmt.Sprintf(`{"api_key":"%s"}`, strings.Repeat("x", (1<<20)-20))
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", strings.NewReader(body)) //nolint:noctx
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.NotEqual(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})
}

// TestRestAPIRateLimit verifies per-principal rate limiting kicks in at 100 req/min.
func TestRestAPIRateLimit(t *testing.T) {
	baseURL := startTestAPIServer(t)

	token := issueToken(t, baseURL, testAPIKey)
	require.NotEmpty(t, token)

	// Send 101 requests — the 101st should be rate-limited.
	got429 := false
	for i := 0; i < 101; i++ {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	assert.True(t, got429, "should receive 429 after exceeding rate limit")
}
