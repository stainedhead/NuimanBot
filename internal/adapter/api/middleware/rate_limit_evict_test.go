package middleware

import (
	"runtime"
	"testing"
	"time"
)

// TestRateLimiterRegistryEvictIdleZeroDuration verifies that evictIdle(0) empties the buckets map.
func TestRateLimiterRegistryEvictIdleZeroDuration(t *testing.T) {
	r := newRateLimiterRegistry(10, time.Minute)

	// Populate some entries.
	r.allow("principal-1")
	r.allow("principal-2")
	r.allow("principal-3")

	r.mu.RLock()
	count := len(r.buckets)
	r.mu.RUnlock()
	if count != 3 {
		t.Fatalf("expected 3 entries before eviction, got %d", count)
	}

	// evictIdle(0) should evict everything because all entries were last used before now+0.
	r.evictIdle(0)

	r.mu.RLock()
	count = len(r.buckets)
	r.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 entries after evictIdle(0), got %d", count)
	}
}

// TestRateLimiterRegistryEvictIdleRetainsRecent verifies that evictIdle keeps recently-used
// principals and removes those not used within the idle window.
func TestRateLimiterRegistryEvictIdleRetainsRecent(t *testing.T) {
	r := newRateLimiterRegistry(10, time.Minute)

	// Insert an "old" entry by directly manipulating lastUsed.
	r.allow("old-principal")
	r.mu.Lock()
	r.buckets["old-principal"].lastUsed = time.Now().Add(-2 * time.Hour)
	r.mu.Unlock()

	// Insert a recent entry.
	r.allow("new-principal")

	// Evict entries idle for more than 1 hour.
	r.evictIdle(1 * time.Hour)

	r.mu.RLock()
	_, oldExists := r.buckets["old-principal"]
	_, newExists := r.buckets["new-principal"]
	r.mu.RUnlock()

	if oldExists {
		t.Error("expected old-principal (last used 2h ago) to be evicted, but it still exists")
	}
	if !newExists {
		t.Error("expected new-principal (recently used) to survive eviction, but it was removed")
	}
}

// TestRateLimiterRegistryBackgroundGoroutineStarted verifies that newRateLimiterRegistry
// starts exactly one background eviction goroutine.
func TestRateLimiterRegistryBackgroundGoroutineStarted(t *testing.T) {
	before := runtime.NumGoroutine()
	_ = newRateLimiterRegistry(10, time.Minute)

	// Poll for up to 200ms for the background goroutine to be scheduled.
	// The goroutine is started with `go func()`, so it may not appear instantly.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() > before {
			return // goroutine started — test passes
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Errorf("expected goroutine count to increase by at least 1 after newRateLimiterRegistry(), before=%d after=%d",
		before, runtime.NumGoroutine())
}

// TestRateLimiterRegistryAllowBlockBehaviorUnchanged verifies that the existing
// allow/block behavior is unchanged after adding principalBucket.
func TestRateLimiterRegistryAllowBlockBehaviorUnchanged(t *testing.T) {
	const limit = 3
	r := newRateLimiterRegistry(limit, time.Minute)
	principal := "test-principal"

	// Should allow up to limit requests.
	for i := 0; i < limit; i++ {
		if !r.allow(principal) {
			t.Fatalf("expected allow=true on request %d", i+1)
		}
	}

	// The next request should be blocked.
	if r.allow(principal) {
		t.Error("expected allow=false after exhausting the rate limit capacity")
	}
}
