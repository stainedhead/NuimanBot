package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"nuimanbot/internal/adapter/api/middleware"
)

// TestBodyLimitMiddleware_NilBody_PassesThrough verifies that a nil body request
// passes straight through without error.
func TestBodyLimitMiddleware_NilBody_PassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.BodyLimit(oneMiB)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// httptest.NewRequest with nil body sets Body to http.NoBody, but the
	// BodyLimit middleware checks for Body == nil specifically. We set it explicitly.
	req.Body = nil
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	assert.True(t, called, "next handler should be called when body is nil")
	assert.Equal(t, http.StatusOK, rr.Code)
}
