package ratelimit_test

import (
	"testing"
	"time"

	"nuimanbot/internal/infrastructure/ratelimit"
)

func TestTokenBucket_Allow_Success(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(5, 1*time.Second) // 5 tokens, 1 per second

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		if !bucket.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if bucket.Allow() {
		t.Error("Request 6 should be denied (bucket empty)")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(2, 50*time.Millisecond) // 2 tokens, refill every 50ms

	// Consume both tokens
	bucket.Allow()
	bucket.Allow()

	// Should be denied immediately
	if bucket.Allow() {
		t.Error("Should be denied when bucket is empty")
	}

	// Wait for refill
	time.Sleep(100 * time.Millisecond)

	// Should allow after refill
	if !bucket.Allow() {
		t.Error("Should be allowed after refill")
	}
}

func TestTokenBucket_MaxCapacity(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(3, 10*time.Millisecond)

	// Wait for potential overfill
	time.Sleep(200 * time.Millisecond)

	// Should only allow up to capacity (3 tokens)
	allowed := 0
	for i := 0; i < 10; i++ {
		if bucket.Allow() {
			allowed++
		}
	}

	if allowed != 3 {
		t.Errorf("Expected 3 allowed requests (max capacity), got %d", allowed)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	bucket := ratelimit.NewTokenBucket(100, 10*time.Millisecond)

	done := make(chan bool)
	allowed := make(chan bool, 200)

	// Concurrent requests
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				allowed <- bucket.Allow()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	close(allowed)

	// Count allowed requests
	count := 0
	for a := range allowed {
		if a {
			count++
		}
	}

	// Should allow up to capacity (100)
	if count > 100 {
		t.Errorf("Allowed %d requests, expected <= 100 (bucket capacity)", count)
	}
}

func TestRateLimiter_PerUserLimit(t *testing.T) {
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"default": {Requests: 3, Window: 1 * time.Second},
	})

	// User 1 should get 3 requests
	for i := 0; i < 3; i++ {
		if !limiter.Allow("user1", "default") {
			t.Errorf("User1 request %d should be allowed", i+1)
		}
	}

	// User 1's 4th request denied
	if limiter.Allow("user1", "default") {
		t.Error("User1's 4th request should be denied")
	}

	// User 2 should still have capacity
	if !limiter.Allow("user2", "default") {
		t.Error("User2's first request should be allowed")
	}
}

func TestRateLimiter_PerActionLimit(t *testing.T) {
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"search":     {Requests: 2, Window: 1 * time.Second},
		"calculator": {Requests: 10, Window: 1 * time.Second},
	})

	// Search limited to 2
	limiter.Allow("user1", "search")
	limiter.Allow("user1", "search")
	if limiter.Allow("user1", "search") {
		t.Error("Search should be limited to 2 requests")
	}

	// Calculator has separate limit
	if !limiter.Allow("user1", "calculator") {
		t.Error("Calculator should have separate limit")
	}
}

func TestRateLimiter_DefaultLimit(t *testing.T) {
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"default": {Requests: 5, Window: 1 * time.Second},
	})

	// Unknown action should use default
	for i := 0; i < 5; i++ {
		if !limiter.Allow("user1", "unknown_action") {
			t.Errorf("Request %d should use default limit", i+1)
		}
	}

	if limiter.Allow("user1", "unknown_action") {
		t.Error("Should apply default limit to unknown actions")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"default": {Requests: 2, Window: 100 * time.Millisecond},
	})

	// Consume limit
	limiter.Allow("user1", "default")
	limiter.Allow("user1", "default")

	// Should be denied
	if limiter.Allow("user1", "default") {
		t.Error("Should be denied when limit reached")
	}

	// Wait for window reset
	time.Sleep(150 * time.Millisecond)

	// Should be allowed after reset
	if !limiter.Allow("user1", "default") {
		t.Error("Should be allowed after window reset")
	}
}
