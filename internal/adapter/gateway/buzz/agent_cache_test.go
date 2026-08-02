package buzz

import (
	"sync"
	"testing"
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
