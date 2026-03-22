package middleware

import (
	"net/http"
	"sync"
	"time"

	"nuimanbot/internal/infrastructure/ratelimit"
)

const (
	// principalBucketEvictInterval is how often the background eviction goroutine runs.
	principalBucketEvictInterval = 10 * time.Minute

	// principalBucketIdleTimeout is the maximum time a principal bucket can be idle before eviction.
	principalBucketIdleTimeout = 1 * time.Hour
)

// principalBucket pairs a token bucket with the last time it was accessed.
type principalBucket struct {
	bucket   *ratelimit.TokenBucket
	lastUsed time.Time
}

// rateLimiterRegistry holds per-principal token buckets.
// A separate registry is created for each RateLimit middleware instance.
type rateLimiterRegistry struct {
	mu       sync.RWMutex
	buckets  map[string]*principalBucket
	requests int
	window   time.Duration
}

// newRateLimiterRegistry creates a registry and starts a background goroutine that
// periodically evicts idle buckets to prevent unbounded memory growth from principals
// that send requests infrequently (e.g., one-off API consumers or scanners).
func newRateLimiterRegistry(requests int, window time.Duration) *rateLimiterRegistry {
	r := &rateLimiterRegistry{
		buckets:  make(map[string]*principalBucket),
		requests: requests,
		window:   window,
	}
	// Run eviction on a fixed interval rather than per-request to keep the hot
	// path lock-free and to amortize the O(n) eviction cost over many requests.
	go func() {
		ticker := time.NewTicker(principalBucketEvictInterval)
		defer ticker.Stop()
		for range ticker.C {
			r.evictIdle(principalBucketIdleTimeout)
		}
	}()
	return r
}

func (r *rateLimiterRegistry) allow(principalID string) bool {
	return r.getOrCreate(principalID).bucket.Allow()
}

// getOrCreate retrieves the bucket for principalID, creating it if absent, and
// stamps lastUsed so the eviction goroutine can identify idle entries.
func (r *rateLimiterRegistry) getOrCreate(principalID string) *principalBucket {
	now := time.Now()

	r.mu.RLock()
	entry, ok := r.buckets[principalID]
	r.mu.RUnlock()
	if ok {
		r.mu.Lock()
		entry.lastUsed = now
		r.mu.Unlock()
		return entry
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after acquiring the write lock.
	if entry, ok = r.buckets[principalID]; ok {
		entry.lastUsed = now
		return entry
	}
	refillInterval := r.window / time.Duration(r.requests)
	entry = &principalBucket{
		bucket:   ratelimit.NewTokenBucket(r.requests, refillInterval),
		lastUsed: now,
	}
	r.buckets[principalID] = entry
	return entry
}

// evictIdle removes buckets that have not been used within maxIdleTime.
// This prevents unbounded map growth from principals that make a single request
// and never return.
func (r *rateLimiterRegistry) evictIdle(maxIdleTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-maxIdleTime)
	for id, entry := range r.buckets {
		if entry.lastUsed.Before(cutoff) {
			delete(r.buckets, id)
		}
	}
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
