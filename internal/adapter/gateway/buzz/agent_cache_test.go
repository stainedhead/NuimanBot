package buzz

import (
	"sync"
	"testing"
	"time"
)

func TestAgentCache_UnknownPubkey_DefaultsFalse(t *testing.T) {
	c := newAgentCache()
	if c.IsAgent("unknown-pubkey") {
		t.Error("IsAgent() for an unknown pubkey = true, want false")
	}
}

func TestAgentCache_SetTrue_IsAgentReturnsTrue(t *testing.T) {
	c := newAgentCache()
	c.Set("pk-1", true)
	if !c.IsAgent("pk-1") {
		t.Error("IsAgent() = false after Set(pk-1, true), want true")
	}
}

func TestAgentCache_SetFalse_OverridesPreviousTrue(t *testing.T) {
	c := newAgentCache()
	c.Set("pk-1", true)
	c.Set("pk-1", false)
	if c.IsAgent("pk-1") {
		t.Error("IsAgent() = true after Set(pk-1, false), want false")
	}
}

func TestAgentCache_ConcurrentAccess_NoRace(t *testing.T) {
	c := newAgentCache()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Set("pk", i%2 == 0)
		}(i)
		go func() {
			defer wg.Done()
			c.IsAgent("pk")
		}()
	}
	wg.Wait()
}

// FR-009: agentCache must not grow unbounded for process lifetime — entries
// are evicted once they exceed a TTL (time since last Set/IsAgent touch) or
// once the cache exceeds a hard capacity bound.

func TestAgentCache_TTLExpiry_StaleEntryEvictedAndTreatedAsUnknown(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := newAgentCacheWithOptions(clock, 1*time.Hour, 100)

	c.Set("pk-stale", true)
	if !c.IsAgent("pk-stale") {
		t.Fatal("IsAgent() = false immediately after Set(true), want true")
	}

	now = now.Add(2 * time.Hour) // advance past the 1h TTL without touching pk-stale

	if c.IsAgent("pk-stale") {
		t.Error("IsAgent() = true for an entry past its TTL, want false (unknown)")
	}

	// A subsequent Set (which sweeps expired entries) should have physically
	// removed the stale entry, not just masked it via the TTL check.
	c.Set("pk-fresh", true)
	if c.size() != 1 {
		t.Errorf("cache size = %d after TTL sweep, want 1 (only pk-fresh should remain)", c.size())
	}
}

func TestAgentCache_TTLIsSliding_RecentIsAgentTouchExtendsLife(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := newAgentCacheWithOptions(clock, 1*time.Hour, 100)

	c.Set("pk-1", true)

	now = now.Add(45 * time.Minute) // within TTL
	if !c.IsAgent("pk-1") {
		t.Fatal("IsAgent() = false within TTL, want true")
	}

	now = now.Add(45 * time.Minute) // 90m since Set, but only 45m since last IsAgent touch
	if !c.IsAgent("pk-1") {
		t.Error("IsAgent() = false after a sliding-TTL touch kept the entry alive, want true")
	}
}

func TestAgentCache_CapacityBound_EvictsOldestWhenFull(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := newAgentCacheWithOptions(clock, 24*time.Hour, 3)

	c.Set("pk-1", true)
	now = now.Add(time.Second)
	c.Set("pk-2", true)
	now = now.Add(time.Second)
	c.Set("pk-3", true)

	if c.size() != 3 {
		t.Fatalf("cache size = %d after 3 Sets at capacity 3, want 3", c.size())
	}

	now = now.Add(time.Second)
	c.Set("pk-4", true) // over capacity — should evict the oldest (pk-1)

	if c.size() > 3 {
		t.Errorf("cache size = %d after exceeding capacity, want <= 3", c.size())
	}
	if c.IsAgent("pk-1") {
		t.Error("IsAgent(pk-1) = true, want false — oldest entry should have been evicted to make room")
	}
	if !c.IsAgent("pk-4") {
		t.Error("IsAgent(pk-4) = false, want true — newest entry should be present")
	}
}
