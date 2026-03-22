package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api/middleware"
)

const oneMiB = 1 << 20 // 1048576 bytes

func TestBodyLimitMiddleware_ExactlyOneMiB_Passes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body fully to trigger limit check
		buf := make([]byte, oneMiB+100)
		n, _ := r.Body.Read(buf)
		_ = n
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.BodyLimit(oneMiB)(next)

	body := bytes.Repeat([]byte("x"), oneMiB)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodyLimitMiddleware_OneMiBPlusOneByte_Returns413(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body to trigger limit check
		buf := make([]byte, oneMiB+100)
		_, _ = r.Body.Read(buf)
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.BodyLimit(oneMiB)(next)

	body := bytes.Repeat([]byte("x"), oneMiB+1)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}

func TestBodyLimitMiddleware_413ErrorDoesNotLeakBodyContents(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, oneMiB+100)
		_, _ = r.Body.Read(buf)
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.BodyLimit(oneMiB)(next)

	// Body containing a secret value
	secret := "super-secret-value-12345"
	body := strings.Repeat("x", oneMiB) + secret
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.NotContains(t, rr.Body.String(), secret, "error response must not contain body contents")
}

func TestBodyLimitMiddleware_SmallBody_Passes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.BodyLimit(oneMiB)(next)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"key":"value"}`))
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
