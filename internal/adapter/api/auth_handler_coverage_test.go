package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func buildConfig(apiKey string) config.ExternalAPIRestConfig {
	return config.ExternalAPIRestConfig{
		Enabled: true,
		Port:    0,
		APIKey:  domain.NewSecureStringFromString(apiKey),
	}
}

// TestValidAPIKey_EmptyConfiguredKey returns false when no key is configured.
func TestValidAPIKey_EmptyConfiguredKey(t *testing.T) {
	h := &AuthHandler{
		cfg:       buildConfig(""),
		jwtSecret: "test",
	}
	assert.False(t, h.validAPIKey("any-key"), "empty configured key should always reject")
}

// TestValidAPIKey_MatchingKey returns true for a matching key.
func TestValidAPIKey_MatchingKey(t *testing.T) {
	h := &AuthHandler{
		cfg:       buildConfig("correct-key"),
		jwtSecret: "test",
	}
	assert.True(t, h.validAPIKey("correct-key"))
}

// TestValidAPIKey_MismatchedKey returns false for wrong key.
func TestValidAPIKey_MismatchedKey(t *testing.T) {
	h := &AuthHandler{
		cfg:       buildConfig("correct-key"),
		jwtSecret: "test",
	}
	assert.False(t, h.validAPIKey("wrong-key"))
}

// TestServeHTTP_ValidKey_Returns200 verifies the full success path via ServeHTTP.
func TestServeHTTP_ValidKey_Returns200(t *testing.T) {
	h := NewAuthHandler(buildConfig("my-key"), "jwt-secret-32-bytes-long-valid!!!")

	body, _ := json.Marshal(map[string]string{"api_key": "my-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
}
