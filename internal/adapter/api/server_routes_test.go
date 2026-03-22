package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func buildSrvConfig(apiKey string) config.ExternalAPIRestConfig {
	return config.ExternalAPIRestConfig{
		Enabled: true,
		Port:    0,
		APIKey:  domain.NewSecureStringFromString(apiKey),
	}
}

// TestNewServer_HealthEndpoint exercises the health route registered in NewServer.
func TestNewServer_HealthEndpoint(t *testing.T) {
	srv, err := NewServer(buildSrvConfig("key"), strings.Repeat("x", 32))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestNewServer_AuthEndpointRegistered exercises the auth token route.
func TestNewServer_AuthEndpointRegistered(t *testing.T) {
	const apiKey = "my-api-key-for-test"
	srv, err := NewServer(buildSrvConfig(apiKey), strings.Repeat("y", 32))
	require.NoError(t, err)

	body := `{"api_key":"` + apiKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestNewServer_ProtectedRouteWithoutJWT_Returns401 exercises the protected route middleware chain.
func TestNewServer_ProtectedRouteWithoutJWT_Returns401(t *testing.T) {
	srv, err := NewServer(buildSrvConfig("key"), strings.Repeat("z", 32))
	require.NoError(t, err)

	// Accessing a protected route without a JWT should return 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
