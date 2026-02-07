package common

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-skill, per-user rate limiting
type RateLimiter struct {
	limiters       map[string]*rate.Limiter
	lastAccessTime map[string]time.Time
	mu             sync.Mutex
}

// NewRateLimiter creates a new RateLimiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters:       make(map[string]*rate.Limiter),
		lastAccessTime: make(map[string]time.Time),
	}
}

// Allow checks if an operation is allowed under the rate limit
// Format: "30/minute" or "20/hour"
func (r *RateLimiter) Allow(skillName, userID, limitSpec string) (bool, error) {
	key := fmt.Sprintf("%s:%s", skillName, userID)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create limiter for this key
	limiter, exists := r.limiters[key]
	if !exists {
		// Parse limit specification
		rateLimit, burst, err := parseLimitSpec(limitSpec)
		if err != nil {
			return false, err
		}

		limiter = rate.NewLimiter(rateLimit, burst)
		r.limiters[key] = limiter
	}

	// Update last access time
	r.lastAccessTime[key] = time.Now()

	// Check if allowed
	return limiter.Allow(), nil
}

// Cleanup removes stale limiters (not accessed in the last hour)
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)

	for key, lastAccess := range r.lastAccessTime {
		if lastAccess.Before(cutoff) {
			delete(r.limiters, key)
			delete(r.lastAccessTime, key)
		}
	}
}

// parseLimitSpec parses a limit specification like "30/minute" or "20/hour"
func parseLimitSpec(spec string) (rate.Limit, int, error) {
	// Parse format: "N/period" where period is "second", "minute", or "hour"
	var count int
	var period string

	_, err := fmt.Sscanf(spec, "%d/%s", &count, &period)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid limit spec format: %s (expected 'N/period')", spec)
	}

	if count <= 0 {
		return 0, 0, fmt.Errorf("invalid limit count: %d (must be > 0)", count)
	}

	var duration time.Duration
	switch period {
	case "second":
		duration = time.Second
	case "minute":
		duration = time.Minute
	case "hour":
		duration = time.Hour
	default:
		return 0, 0, fmt.Errorf("invalid period: %s (must be 'second', 'minute', or 'hour')", period)
	}

	// Calculate rate: operations per second
	rateLimit := rate.Limit(float64(count) / duration.Seconds())

	// Burst allows initial burst of operations
	burst := count

	return rateLimit, burst, nil
}
