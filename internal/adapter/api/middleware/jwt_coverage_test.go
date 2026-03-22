package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api/middleware"
)

// makeTokenWithNoneAlgorithm creates a JWT using the "none" algorithm pattern
// by crafting a header with a non-HMAC alg to trigger the signing method check.
// Since jwt/v5 won't sign with an unsupported method, we craft a tampered header manually.
func makeTokenWithTamperedAlg(t *testing.T) string {
	t.Helper()
	// Build a base64url-encoded header for "RS256" alg, then attach a real HS256 payload
	// and signature. This token's header will claim RS256, causing the middleware's
	// method-type assertion to fail.
	import_ := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"RS256","typ":"JWT"}
	// Minimal payload with future exp.
	payload := "eyJzdWIiOiJ1c2VyMTIzIiwiZXhwIjo5OTk5OTk5OTk5fQ" // {"sub":"user123","exp":9999999999}
	// Signature (invalid, doesn't matter — middleware rejects at alg check).
	sig := "invalidsig"
	return import_ + "." + payload + "." + sig
}

// TestJWTMiddleware_WrongSigningAlgorithm_Returns401 verifies that tokens claiming
// a non-HMAC signing algorithm are rejected.
func TestJWTMiddleware_WrongSigningAlgorithm_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeTokenWithTamperedAlg(t))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	// Should reject because the token either fails alg check or signature validation.
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestJWTMiddleware_EmptyBearerToken_Returns401 verifies empty bearer token is rejected.
func TestJWTMiddleware_EmptyBearerToken_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestJWTMiddleware_TokenWithEmptySubject_Returns401 verifies token with empty sub is rejected.
func TestJWTMiddleware_TokenWithEmptySubject_Returns401(t *testing.T) {
	// Create token with empty subject claim.
	claims := jwt.MapClaims{
		"sub": "",
		"iss": "nuimanbot",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
