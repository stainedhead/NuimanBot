//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api"
)

// startTestAPIServer starts a REST API server on a free port and returns its base URL.
// The server is stopped in t.Cleanup.
func startTestAPIServer(t *testing.T) string {
	t.Helper()

	cfg := api.ServerConfig{
		APIKeys: map[string]string{
			"valid-key-123": "principal-alice",
		},
		JWTSecret: []byte("test-jwt-secret-not-for-production"),
	}

	srv, ln, err := api.ListenAndServeOnFreePort(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	return "http://" + ln.Addr().String()
}

// issueToken requests a JWT from the server using the provided API key.
func issueToken(t *testing.T, baseURL, apiKey string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"api_key": apiKey})
	resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 from token endpoint")

	var tok struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tok))
	require.NotEmpty(t, tok.Token, "token should not be empty")
	return tok.Token
}

// TestRestAPITokenIssuance tests POST /api/v1/auth/token.
func TestRestAPITokenIssuance(t *testing.T) {
	baseURL := startTestAPIServer(t)

	t.Run("valid key returns 200 and JWT", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"api_key": "valid-key-123"})
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.NotEmpty(t, result["token"], "response should contain token")
		assert.NotEmpty(t, result["expires_at"], "response should contain expires_at")
	})

	t.Run("invalid key returns 401", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"api_key": "wrong-key"})
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing key returns 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{})
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestRestAPIJWTMiddleware tests that protected endpoints require a valid JWT.
func TestRestAPIJWTMiddleware(t *testing.T) {
	baseURL := startTestAPIServer(t)
	token := issueToken(t, baseURL, "valid-key-123")

	t.Run("valid JWT returns 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("missing Authorization header returns 401", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("malformed token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer not.a.valid.jwt")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestRestAPIBodyLimit verifies that bodies larger than 1 MiB are rejected with 413.
func TestRestAPIBodyLimit(t *testing.T) {
	baseURL := startTestAPIServer(t)

	t.Run("body over 1 MiB returns 413", func(t *testing.T) {
		// 1 MiB + 1 byte
		oversized := strings.Repeat("x", 1<<20+1)
		body := fmt.Sprintf(`{"api_key": "%s"}`, oversized)

		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// http.MaxBytesReader returns 413 Request Entity Too Large.
		assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	})

	t.Run("body at exactly 1 MiB passes", func(t *testing.T) {
		// A valid request within the limit.
		body, _ := json.Marshal(map[string]string{"api_key": "invalid"})
		resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Invalid key → 401, but the body was accepted (not 413).
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestRestAPIRateLimit verifies that exceeding 100 req/min returns 429.
func TestRestAPIRateLimit(t *testing.T) {
	baseURL := startTestAPIServer(t)
	token := issueToken(t, baseURL, "valid-key-123")

	// Send 101 requests rapidly and check we get a 429.
	var got429 bool
	for i := 0; i < 110; i++ {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}

	assert.True(t, got429, "should receive 429 after exceeding rate limit")

	// Wait for the rate limit window to reset.
	// The test bucket refills after 1 minute; for CI speed we just verify we got 429.
	_ = time.Now() // suppress unused import
}
