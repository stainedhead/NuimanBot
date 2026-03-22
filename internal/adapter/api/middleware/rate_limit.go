package middleware

import (
	"net/http"
	"sync"
	"time"

	"nuimanbot/internal/infrastructure/ratelimit"
)

// rateLimiterRegistry holds per-principal token buckets.
// A separate registry is created for each RateLimit middleware instance.
type rateLimiterRegistry struct {
	mu       sync.RWMutex
	buckets  map[string]*ratelimit.TokenBucket
	requests int
	window   time.Duration
}

func newRateLimiterRegistry(requests int, window time.Duration) *rateLimiterRegistry {
	return &rateLimiterRegistry{
		buckets:  make(map[string]*ratelimit.TokenBucket),
		requests: requests,
		window:   window,
	}
}

func (r *rateLimiterRegistry) allow(principalID string) bool {
	return r.getOrCreate(principalID).Allow()
}

func (r *rateLimiterRegistry) getOrCreate(principalID string) *ratelimit.TokenBucket {
	r.mu.RLock()
	bucket, ok := r.buckets[principalID]
	r.mu.RUnlock()
	if ok {
		return bucket
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after acquiring the write lock.
	if bucket, ok = r.buckets[principalID]; ok {
		return bucket
	}
	refillInterval := r.window / time.Duration(r.requests)
	bucket = ratelimit.NewTokenBucket(r.requests, refillInterval)
	r.buckets[principalID] = bucket
	return bucket
}

// RateLimit returns a middleware that enforces per-principal rate limiting.
// requests defines the token bucket capacity (burst) and window defines the refill window.
// The principal ID is read from the request context (set by the JWT middleware).
// Returns 401 if no principal is in the context; 429 if the rate limit is exceeded.
func RateLimit(requests int, window time.Duration) func(http.Handler) http.Handler {
	registry := newRateLimiterRegistry(requests, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principalID := PrincipalFromContext(r.Context())
			if principalID == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "no authenticated principal")
				return
			}

			if !registry.allow(principalID) {
				writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
