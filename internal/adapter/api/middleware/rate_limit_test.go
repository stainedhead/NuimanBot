package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api/middleware"
)

// buildRateLimitedHandler creates a handler that injects principalID into the
// context and then passes through the shared rateLimitMW.
func buildRateLimitedHandler(next http.Handler, principalID string, rateLimitMW func(http.Handler) http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.ContextWithPrincipal(r.Context(), principalID)
		rateLimitMW(next).ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRateLimitMiddleware_AllowsUpToLimit(t *testing.T) {
	const limit = 5
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// Create the middleware once so the registry is shared.
	rateLimitMW := middleware.RateLimit(limit, time.Minute)
	handler := buildRateLimitedHandler(next, "principal-a", rateLimitMW)

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i+1)
	}
}

func TestRateLimitMiddleware_BlocksAfterLimit(t *testing.T) {
	const limit = 5
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	rateLimitMW := middleware.RateLimit(limit, time.Minute)
	handler := buildRateLimitedHandler(next, "principal-b", rateLimitMW)

	// Exhaust limit.
	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	// Next request should be blocked.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimitMiddleware_DifferentPrincipalsIndependent(t *testing.T) {
	const limit = 3
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// One shared middleware instance — each principal has its own bucket.
	rateLimitMW := middleware.RateLimit(limit, time.Minute)

	handlerA := buildRateLimitedHandler(next, "principal-x", rateLimitMW)
	handlerB := buildRateLimitedHandler(next, "principal-y", rateLimitMW)

	// Exhaust principal-x.
	for i := 0; i < limit; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handlerA.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}
	// principal-x is now throttled.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handlerA.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// principal-y should still be allowed.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rr2 := httptest.NewRecorder()
	handlerB.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimitMiddleware_NoPrincipal_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := middleware.RateLimit(10, time.Minute)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No principal in context.
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
