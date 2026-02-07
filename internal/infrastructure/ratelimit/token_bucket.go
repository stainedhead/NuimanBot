package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter.
// Tokens are added at a fixed rate up to a maximum capacity.
type TokenBucket struct {
	capacity       int        // Maximum number of tokens
	tokens         float64    // Current token count
	refillRate     float64    // Tokens added per second
	lastRefillTime time.Time  // Last time tokens were refilled
	mu             sync.Mutex // Protects concurrent access
}

// NewTokenBucket creates a new token bucket with the given capacity and refill rate.
// capacity: maximum number of tokens (burst size)
// refillInterval: time between adding 1 token
func NewTokenBucket(capacity int, refillInterval time.Duration) *TokenBucket {
	refillRate := 1.0 / refillInterval.Seconds() // tokens per second

	return &TokenBucket{
		capacity:       capacity,
		tokens:         float64(capacity), // Start with full bucket
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// Allow attempts to consume one token from the bucket.
// Returns true if a token was available, false otherwise.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()
	tb.tokens += elapsed * tb.refillRate

	// Cap at maximum capacity
	if tb.tokens > float64(tb.capacity) {
		tb.tokens = float64(tb.capacity)
	}

	tb.lastRefillTime = now

	// Try to consume a token
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	return false
}

// RateLimit defines rate limiting parameters for an action.
type RateLimit struct {
	Requests int           // Number of requests allowed
	Window   time.Duration // Time window for the limit
}

// RateLimiter manages rate limits for multiple users and actions.
type RateLimiter struct {
	limits  map[string]RateLimit    // Action -> rate limit config
	buckets map[string]*TokenBucket // Key (user:action) -> token bucket
	mu      sync.RWMutex            // Protects buckets map
}

// NewRateLimiter creates a new rate limiter with the given limits.
// limits: map of action name to rate limit configuration
func NewRateLimiter(limits map[string]RateLimit) *RateLimiter {
	return &RateLimiter{
		limits:  limits,
		buckets: make(map[string]*TokenBucket),
	}
}

// Allow checks if a request from the given user for the given action is allowed.
// Returns true if allowed, false if rate limit exceeded.
func (rl *RateLimiter) Allow(userID, action string) bool {
	// Get rate limit for this action (or default)
	limit, exists := rl.limits[action]
	if !exists {
		limit = rl.limits["default"]
	}

	// Get or create bucket for this user+action
	key := userID + ":" + action
	bucket := rl.getBucket(key, limit)

	return bucket.Allow()
}

// getBucket retrieves or creates a token bucket for the given key.
func (rl *RateLimiter) getBucket(key string, limit RateLimit) *TokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	// Create new bucket
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists := rl.buckets[key]; exists {
		return bucket
	}

	// Calculate refill interval: window / requests
	// Example: 10 requests per 1 second = refill every 100ms
	refillInterval := limit.Window / time.Duration(limit.Requests)

	bucket = NewTokenBucket(limit.Requests, refillInterval)
	rl.buckets[key] = bucket

	return bucket
}

// Reset removes all rate limit state for a specific user.
// Useful for testing or administrative reset.
func (rl *RateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove all buckets for this user
	for key := range rl.buckets {
		if len(key) > len(userID) && key[:len(userID)] == userID {
			delete(rl.buckets, key)
		}
	}
}
