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

const testJWTSecret = "test-jwt-secret-32-bytes-long!!"

func makeValidToken(t *testing.T, principalID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": principalID,
		"iss": "nuimanbot",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

func makeValidTokenWithRole(t *testing.T, principalID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  principalID,
		"role": role,
		"iss":  "nuimanbot",
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

func makeExpiredToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": "user123",
		"iss": "nuimanbot",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return signed
}

func TestJWTMiddleware_MissingAuthorizationHeader_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")
}

func TestJWTMiddleware_MalformedToken_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_ExpiredToken_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeExpiredToken(t))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTMiddleware_ValidToken_StoresPrincipalAndCallsNext(t *testing.T) {
	const principalID = "user-abc-123"
	var capturedPrincipal string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPrincipal = middleware.PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeValidToken(t, principalID))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, principalID, capturedPrincipal)
}

func TestJWTMiddleware_ValidTokenWithRole_StoresRole(t *testing.T) {
	var capturedRole string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRole = middleware.RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeValidTokenWithRole(t, "user-abc", middleware.RoleAdmin))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, middleware.RoleAdmin, capturedRole)
}

func TestJWTMiddleware_ValidTokenWithoutRole_RoleIsEmpty(t *testing.T) {
	var capturedRole string
	roleSeen := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRole = middleware.RoleFromContext(r.Context())
		roleSeen = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+makeValidToken(t, "user-xyz"))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, roleSeen)
	assert.Empty(t, capturedRole, "a token without a role claim must not be treated as admin")
}

func TestJWTMiddleware_BearerPrefixRequired(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.JWT(testJWTSecret)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Token without "Bearer " prefix
	req.Header.Set("Authorization", makeValidToken(t, "user123"))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
