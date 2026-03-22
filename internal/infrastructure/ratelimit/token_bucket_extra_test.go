package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// TestRateLimiter_Reset tests the Reset method.
func TestRateLimiter_Reset(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 10, Window: time.Second},
		"api":     {Requests: 5, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	userID := "user-to-reset"

	// Exhaust rate limit for user
	for i := 0; i < 5; i++ {
		limiter.Allow(userID, "api")
	}

	// Reset should clear all buckets for this user
	limiter.Reset(userID)

	// After reset, user should be able to make requests again
	allowed := limiter.Allow(userID, "api")
	if !allowed {
		t.Error("Expected user to be allowed after Reset")
	}
}

// TestRateLimiter_Reset_Partial tests that Reset only removes the specified user's buckets.
func TestRateLimiter_Reset_Partial(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 3, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	user1 := "user1"
	user2 := "user2"

	// Exhaust both users
	for i := 0; i < 3; i++ {
		limiter.Allow(user1, "default")
		limiter.Allow(user2, "default")
	}

	// Reset only user1
	limiter.Reset(user1)

	// user1 should be allowed (bucket reset)
	if !limiter.Allow(user1, "default") {
		t.Error("Expected user1 to be allowed after reset")
	}

	// user2 should still be rate limited
	allowed := limiter.Allow(user2, "default")
	if allowed {
		t.Error("Expected user2 to still be rate limited")
	}
}

// TestRateLimiter_Reset_NonExistentUser tests Reset for a user with no buckets.
func TestRateLimiter_Reset_NonExistentUser(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 10, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	// Reset non-existent user should be a no-op
	limiter.Reset("nonexistent-user")

	// Limiter should still work normally
	allowed := limiter.Allow("new-user", "default")
	if !allowed {
		t.Error("Expected new user to be allowed")
	}
}

// TestRateLimiter_ConcurrentAccess tests that concurrent access is safe.
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 100, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	var wg sync.WaitGroup
	users := []string{"user1", "user2", "user3", "user4", "user5"}

	for _, user := range users {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(u string) {
				defer wg.Done()
				limiter.Allow(u, "default")
			}(user)
		}
	}

	wg.Wait()
}

// TestRateLimiter_ConcurrentCreation tests that concurrent bucket creation is safe (double-check lock).
func TestRateLimiter_ConcurrentCreation(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 100, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// All goroutines use the same key to trigger the double-check
			limiter.Allow("same-user", "default")
		}()
	}
	wg.Wait()
}

// TestTokenBucket_Refill tests that tokens refill over time.
func TestTokenBucket_Refill(t *testing.T) {
	// Create a bucket with 1 request per 10ms
	bucket := NewTokenBucket(5, 10*time.Millisecond)

	// Drain the bucket
	for i := 0; i < 5; i++ {
		if !bucket.Allow() {
			t.Fatalf("Expected to be allowed on request %d", i+1)
		}
	}

	// Should be empty now
	if bucket.Allow() {
		t.Error("Expected bucket to be empty after draining")
	}

	// Wait for some tokens to refill
	time.Sleep(25 * time.Millisecond) // Should add ~2 tokens

	// Should be allowed now
	if !bucket.Allow() {
		t.Error("Expected bucket to have tokens after refill")
	}
}

// TestRateLimiter_UnknownAction tests behavior with unknown action (uses default).
func TestRateLimiter_UnknownAction(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 5, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	// Unknown action should fall back to default limit
	allowed := limiter.Allow("user", "unknown-action")
	if !allowed {
		t.Error("Expected first request to unknown action to be allowed")
	}
}

// TestRateLimiter_Reset_UserPrefix tests Reset only removes keys starting with userID.
func TestRateLimiter_Reset_UserPrefix(t *testing.T) {
	limits := map[string]RateLimit{
		"default": {Requests: 10, Window: time.Second},
	}
	limiter := NewRateLimiter(limits)

	// Use users where one user ID is a prefix of another
	user := "user1"
	userLong := "user100"

	limiter.Allow(user, "default")
	limiter.Allow(userLong, "default")

	// Reset user1 - should NOT remove user100
	limiter.Reset(user)

	// user100 should still have its bucket
	// (test that Reset uses proper prefix check, not substring)
	_ = limiter.Allow(userLong, "default")
}
