package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func buildRESTConfig(apiKey string) config.ExternalAPIRestConfig {
	return config.ExternalAPIRestConfig{
		Enabled: true,
		Port:    0,
		APIKey:  domain.NewSecureStringFromString(apiKey),
	}
}

func TestAuthHandler_ValidAPIKey_Returns200WithJWT(t *testing.T) {
	cfg := buildRESTConfig("test-secret-key")
	jwtSecret := "jwt-signing-secret"

	handler := api.NewAuthHandler(cfg, jwtSecret)

	body, _ := json.Marshal(map[string]string{"api_key": "test-secret-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	tokenStr, ok := resp["token"].(string)
	require.True(t, ok, "response should contain a token string")
	require.NotEmpty(t, tokenStr)

	// Parse and verify claims
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid)

	claims, ok := token.Claims.(jwt.MapClaims)
	require.True(t, ok)

	// Verify required claims
	assert.NotEmpty(t, claims["sub"])
	assert.Equal(t, "nuimanbot", claims["iss"])

	// exp should be ~24h from now
	expFloat, ok := claims["exp"].(float64)
	require.True(t, ok)
	expTime := time.Unix(int64(expFloat), 0)
	assert.True(t, expTime.After(time.Now().Add(23*time.Hour)), "exp should be ~24h from now")
	assert.True(t, expTime.Before(time.Now().Add(25*time.Hour)), "exp should not be more than 25h from now")
}

func TestAuthHandler_InvalidAPIKey_Returns401(t *testing.T) {
	cfg := buildRESTConfig("correct-key")
	handler := api.NewAuthHandler(cfg, "jwt-secret")

	body, _ := json.Marshal(map[string]string{"api_key": "wrong-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	var resp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "unauthorized", resp["error"])
}

func TestAuthHandler_MissingBody_Returns400(t *testing.T) {
	cfg := buildRESTConfig("key")
	handler := api.NewAuthHandler(cfg, "jwt-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_MalformedJSON_Returns400(t *testing.T) {
	cfg := buildRESTConfig("key")
	handler := api.NewAuthHandler(cfg, "jwt-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_EmptyAPIKey_Returns400(t *testing.T) {
	cfg := buildRESTConfig("key")
	handler := api.NewAuthHandler(cfg, "jwt-secret")

	body, _ := json.Marshal(map[string]string{"api_key": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAuthHandler_OnlyAcceptsPOST(t *testing.T) {
	cfg := buildRESTConfig("key")
	handler := api.NewAuthHandler(cfg, "jwt-secret")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/token", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
