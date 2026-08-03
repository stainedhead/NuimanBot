package buzz

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// agentCacheTTL bounds how long a cached pubkey→is_agent entry survives
	// without being touched (by Set or IsAgent) before it's treated as
	// unknown and becomes eligible for eviction. TTL is sliding, refreshed
	// on every IsAgent lookup, not just on Set: loopGuard (FR-009's other
	// consumer of agentCache) relies on IsAgent staying accurate for the
	// lifetime of an active agent even during stretches where its
	// kind:9000/10100 identity events don't recur. A short, write-only TTL
	// would let a quiet-but-still-active agent silently fall back to
	// "unknown" (=false, which bypasses loop-guard suppression) after a busy
	// conversation goes quiet for a while. 24h comfortably outlives any
	// single conversation while still bounding indefinite growth from
	// pubkeys that genuinely stop appearing (FR-009).
	agentCacheTTL = 24 * time.Hour

	// agentCacheMaxSize hard-bounds the cache's memory footprint regardless
	// of TTL, in case a busy/public relay observes more distinct pubkeys
	// within one TTL window than a single long-running process should
	// reasonably keep resident. When full, the single least-recently-touched
	// entry is evicted to make room for a new pubkey.
	agentCacheMaxSize = 5000
)

// cacheEntry is a single agentCache record. Both fields are atomics — not
// just lastSeen — because IsAgent reads them after releasing
// agentCache.mu's read lock (to slide the TTL forward without holding the
// map lock across the whole call), so a concurrent Set holding the write
// lock could otherwise race with that unguarded read.
type cacheEntry struct {
	isAgent  atomic.Bool
	lastSeen atomic.Int64
}

// agentCache tracks which Buzz pubkeys are known to be agents, per FR-009's
// need for a pubkey→is_agent lookup at message-receive time rather than a
// per-message field (see research.md Q2/Q5: Buzz has no per-message
// agent-identity tag). Populated from two signals, both handled by
// handleAgentStatusEvent (P2.3): a channel-membership kind:9000 event
// carrying role "bot" (nostr.RoleBot), or the mere presence of a kind:10100
// agent-profile event for a pubkey (see implementation-notes.md P2.2 for why
// presence, not content, is the signal for that kind).
//
// Bounded by TTL + capacity (FR-009): entries are evicted once they exceed
// agentCacheTTL since last touch, or once the cache exceeds agentCacheMaxSize
// (oldest entry evicted first). See implementation-notes.md for the
// TTL/capacity rationale — there's no prior cache-eviction pattern elsewhere
// in this codebase to match, so this was a fresh design call.
//
// Safe for concurrent use: the relay read loop and any future
// membership/profile update paths may call Set/IsAgent concurrently.
type agentCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	now     func() time.Time
	ttl     time.Duration
	maxSize int
}

func newAgentCache() *agentCache {
	return newAgentCacheWithOptions(time.Now, agentCacheTTL, agentCacheMaxSize)
}

// newAgentCacheWithOptions builds an agentCache with an overridable clock,
// TTL, and capacity — used by tests to exercise eviction deterministically
// and without waiting on real time or inserting thousands of entries.
func newAgentCacheWithOptions(now func() time.Time, ttl time.Duration, maxSize int) *agentCache {
	return &agentCache{
		entries: make(map[string]*cacheEntry),
		now:     now,
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Set records whether pubkey is known to be an agent.
func (c *agentCache) Set(pubkey string, isAgent bool) {
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpiredLocked(now)

	if e, ok := c.entries[pubkey]; ok {
		e.isAgent.Store(isAgent)
		e.lastSeen.Store(now.UnixNano())
		return
	}

	if len(c.entries) >= c.maxSize {
		c.evictOldestLocked()
	}

	e := &cacheEntry{}
	e.isAgent.Store(isAgent)
	e.lastSeen.Store(now.UnixNano())
	c.entries[pubkey] = e
}

// IsAgent reports whether pubkey is known to be an agent. Unknown or
// TTL-expired pubkeys default to false. A successful lookup slides the
// entry's TTL forward (see agentCacheTTL's doc comment for why).
func (c *agentCache) IsAgent(pubkey string) bool {
	c.mu.RLock()
	e, ok := c.entries[pubkey]
	c.mu.RUnlock()
	if !ok {
		return false
	}

	now := c.now()
	if now.Sub(time.Unix(0, e.lastSeen.Load())) > c.ttl {
		return false
	}
	e.lastSeen.Store(now.UnixNano())
	return e.isAgent.Load()
}

// size reports the current number of entries, including any not yet swept
// past their TTL. Test-only helper.
func (c *agentCache) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictExpiredLocked removes all entries whose TTL has elapsed as of now.
// Caller must hold c.mu for writing.
func (c *agentCache) evictExpiredLocked(now time.Time) {
	for pubkey, e := range c.entries {
		if now.Sub(time.Unix(0, e.lastSeen.Load())) > c.ttl {
			delete(c.entries, pubkey)
		}
	}
}

// evictOldestLocked removes the single least-recently-touched entry. Caller
// must hold c.mu for writing.
func (c *agentCache) evictOldestLocked() {
	var oldestKey string
	var oldestAt int64
	first := true
	for pubkey, e := range c.entries {
		seen := e.lastSeen.Load()
		if first || seen < oldestAt {
			oldestAt = seen
			oldestKey = pubkey
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}
