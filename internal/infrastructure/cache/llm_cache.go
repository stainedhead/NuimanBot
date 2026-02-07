package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/metrics"
)

// cacheEntry represents a cached LLM response with expiration.
type cacheEntry struct {
	response  *domain.LLMResponse
	expiresAt time.Time
}

// LLMCache is an in-memory cache for LLM responses.
// It uses prompt hashes as keys and supports TTL-based expiration.
type LLMCache struct {
	entries   map[string]*cacheEntry
	maxSize   int
	ttl       time.Duration
	mu        sync.RWMutex
	hits      uint64
	misses    uint64
	evictions uint64
}

// CacheStats represents cache performance statistics.
type CacheStats struct {
	Size      int     // Current number of entries
	Hits      uint64  // Number of cache hits
	Misses    uint64  // Number of cache misses
	Evictions uint64  // Number of evictions due to size limit
	HitRate   float64 // Hit rate (hits / (hits + misses))
}

// NewLLMCache creates a new LLM response cache.
// maxSize is the maximum number of entries to cache.
// ttl is the time-to-live for cached entries.
func NewLLMCache(maxSize int, ttl time.Duration) *LLMCache {
	return &LLMCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Set stores an LLM response in the cache.
func (c *LLMCache) Set(ctx context.Context, prompt string, response *domain.LLMResponse) {
	key := c.normalizeAndHash(prompt)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entry if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
		metrics.CacheEvictionsTotal.WithLabelValues("llm").Inc()
	}

	c.entries[key] = &cacheEntry{
		response:  response,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Get retrieves an LLM response from the cache.
// Returns the response and true if found and not expired, nil and false otherwise.
func (c *LLMCache) Get(ctx context.Context, prompt string) (*domain.LLMResponse, bool) {
	key := c.normalizeAndHash(prompt)

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.recordMiss()
		return nil, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		// Remove expired entry
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()

		c.recordMiss()
		return nil, false
	}

	c.recordHit()
	return entry.response, true
}

// Delete removes an entry from the cache.
func (c *LLMCache) Delete(ctx context.Context, prompt string) {
	key := c.normalizeAndHash(prompt)

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear removes all entries from the cache.
func (c *LLMCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Stats returns cache performance statistics.
func (c *LLMCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return CacheStats{
		Size:      len(c.entries),
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		HitRate:   hitRate,
	}
}

// normalizeAndHash normalizes the prompt and returns a hash key.
// Normalization removes leading/trailing whitespace and lowercases the text.
func (c *LLMCache) normalizeAndHash(prompt string) string {
	// Normalize: trim whitespace
	normalized := strings.TrimSpace(prompt)

	// Hash the normalized prompt
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// evictOldest evicts the oldest entry based on expiration time.
// Must be called with lock held.
func (c *LLMCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	// Find the entry with the earliest expiration time
	first := true
	for key, entry := range c.entries {
		if first || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.evictions++
	}
}

// recordHit increments the hit counter.
func (c *LLMCache) recordHit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits++
	metrics.CacheHitsTotal.WithLabelValues("llm").Inc()
}

// recordMiss increments the miss counter.
func (c *LLMCache) recordMiss() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.misses++
	metrics.CacheMissesTotal.WithLabelValues("llm").Inc()
}
