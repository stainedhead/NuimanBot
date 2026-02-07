package common

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow_UnderLimit(t *testing.T) {
	limiter := NewRateLimiter()

	// 10 requests per second should all be allowed initially
	for i := 0; i < 10; i++ {
		allowed, err := limiter.Allow("test_skill", "user1", "10/second")
		require.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}
}

func TestRateLimiter_Allow_OverLimit(t *testing.T) {
	limiter := NewRateLimiter()

	// First 5 requests allowed (burst)
	for i := 0; i < 5; i++ {
		allowed, err := limiter.Allow("test_skill", "user1", "5/second")
		require.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	// 6th request should be denied (over burst limit)
	allowed, err := limiter.Allow("test_skill", "user1", "5/second")
	require.NoError(t, err)
	assert.False(t, allowed, "6th request should be denied")
}

func TestRateLimiter_Allow_ConcurrentRequests(t *testing.T) {
	limiter := NewRateLimiter()

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0

	// Make 20 concurrent requests with limit of 10/second
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, err := limiter.Allow("test_skill", "user1", "10/second")
			require.NoError(t, err)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// At most 10 should be allowed (burst limit)
	assert.LessOrEqual(t, allowedCount, 10, "Should not exceed burst limit")
	assert.Greater(t, allowedCount, 0, "Some requests should be allowed")
}

func TestRateLimiter_Allow_DifferentUsers(t *testing.T) {
	limiter := NewRateLimiter()

	// Each user gets their own limit
	for i := 0; i < 5; i++ {
		allowed1, err := limiter.Allow("test_skill", "user1", "5/second")
		require.NoError(t, err)
		assert.True(t, allowed1, "User1 request %d should be allowed", i+1)

		allowed2, err := limiter.Allow("test_skill", "user2", "5/second")
		require.NoError(t, err)
		assert.True(t, allowed2, "User2 request %d should be allowed", i+1)
	}

	// Both users should be denied now (hit their individual limits)
	allowed1, err := limiter.Allow("test_skill", "user1", "5/second")
	require.NoError(t, err)
	assert.False(t, allowed1, "User1 should be denied")

	allowed2, err := limiter.Allow("test_skill", "user2", "5/second")
	require.NoError(t, err)
	assert.False(t, allowed2, "User2 should be denied")
}

func TestRateLimiter_Allow_DifferentSkills(t *testing.T) {
	limiter := NewRateLimiter()

	// Each skill gets its own limit per user
	for i := 0; i < 5; i++ {
		allowed1, err := limiter.Allow("skill1", "user1", "5/second")
		require.NoError(t, err)
		assert.True(t, allowed1, "Skill1 request %d should be allowed", i+1)

		allowed2, err := limiter.Allow("skill2", "user1", "5/second")
		require.NoError(t, err)
		assert.True(t, allowed2, "Skill2 request %d should be allowed", i+1)
	}

	// Both skills should be denied now (hit their individual limits)
	allowed1, err := limiter.Allow("skill1", "user1", "5/second")
	require.NoError(t, err)
	assert.False(t, allowed1, "Skill1 should be denied")

	allowed2, err := limiter.Allow("skill2", "user1", "5/second")
	require.NoError(t, err)
	assert.False(t, allowed2, "Skill2 should be denied")
}

func TestRateLimiter_Allow_MinuteLimit(t *testing.T) {
	limiter := NewRateLimiter()

	// 60 requests per minute = 1 per second
	// Burst allows initial burst, then rate-limited
	allowed1, err := limiter.Allow("test_skill", "user1", "60/minute")
	require.NoError(t, err)
	assert.True(t, allowed1)
}

func TestRateLimiter_Allow_HourLimit(t *testing.T) {
	limiter := NewRateLimiter()

	// 3600 requests per hour = 1 per second
	allowed1, err := limiter.Allow("test_skill", "user1", "3600/hour")
	require.NoError(t, err)
	assert.True(t, allowed1)
}

func TestRateLimiter_Allow_InvalidFormat(t *testing.T) {
	limiter := NewRateLimiter()

	allowed, err := limiter.Allow("test_skill", "user1", "invalid")
	require.Error(t, err)
	assert.False(t, allowed)
	assert.Contains(t, err.Error(), "invalid limit spec format")
}

func TestRateLimiter_Allow_InvalidPeriod(t *testing.T) {
	limiter := NewRateLimiter()

	allowed, err := limiter.Allow("test_skill", "user1", "10/day")
	require.Error(t, err)
	assert.False(t, allowed)
	assert.Contains(t, err.Error(), "invalid period")
}

func TestRateLimiter_Allow_InvalidCount(t *testing.T) {
	limiter := NewRateLimiter()

	allowed, err := limiter.Allow("test_skill", "user1", "0/second")
	require.Error(t, err)
	assert.False(t, allowed)
	assert.Contains(t, err.Error(), "invalid limit count")
}

func TestRateLimiter_Cleanup_StaleLimiters(t *testing.T) {
	limiter := NewRateLimiter()

	// Make some requests
	_, _ = limiter.Allow("test_skill", "user1", "10/second")
	_, _ = limiter.Allow("test_skill", "user2", "10/second")

	// Manually set last access time to 2 hours ago
	limiter.mu.Lock()
	key1 := "test_skill:user1"
	limiter.lastAccessTime[key1] = time.Now().Add(-2 * time.Hour)
	limiter.mu.Unlock()

	// Run cleanup
	limiter.Cleanup()

	// Check that stale limiter was removed
	limiter.mu.Lock()
	_, exists1 := limiter.limiters[key1]
	_, exists2 := limiter.limiters["test_skill:user2"]
	limiter.mu.Unlock()

	assert.False(t, exists1, "Stale limiter should be removed")
	assert.True(t, exists2, "Recent limiter should be kept")
}

func TestRateLimiter_Cleanup_NoStale(t *testing.T) {
	limiter := NewRateLimiter()

	// Make some requests
	_, _ = limiter.Allow("test_skill", "user1", "10/second")
	_, _ = limiter.Allow("test_skill", "user2", "10/second")

	// Run cleanup (should not remove anything)
	limiter.Cleanup()

	// Check that all limiters are still there
	limiter.mu.Lock()
	count := len(limiter.limiters)
	limiter.mu.Unlock()

	assert.Equal(t, 2, count, "All limiters should be kept")
}
