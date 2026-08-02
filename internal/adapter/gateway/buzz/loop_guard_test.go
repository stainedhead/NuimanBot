package buzz

import (
	"testing"
	"time"
)

func TestLoopGuard_AllowsHumanMessagesAlways(t *testing.T) {
	g := newLoopGuard(5, 30*time.Second)
	now := time.Now()
	for i := 0; i < 20; i++ {
		if !g.Allow("chan-1", false, now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatalf("Allow() returned false for human message #%d, want always true", i)
		}
	}
}

func TestLoopGuard_AllowsSingleAgentReply(t *testing.T) {
	g := newLoopGuard(5, 30*time.Second)
	if !g.Allow("chan-1", true, time.Now()) {
		t.Error("Allow() = false for a single agent message, want true (under threshold)")
	}
}

func TestLoopGuard_SuppressesRunawayAgentChain(t *testing.T) {
	g := newLoopGuard(5, 30*time.Second)
	now := time.Now()

	var allowed int
	const messageCount = 50
	for i := 0; i < messageCount; i++ {
		if g.Allow("chan-1", true, now.Add(time.Duration(i)*time.Millisecond)) {
			allowed++
		}
	}

	if allowed != 5 {
		t.Errorf("allowed %d of %d runaway agent-to-agent messages, want exactly 5 (the threshold) — guard must terminate the chain, not run indefinitely", allowed, messageCount)
	}
}

func TestLoopGuard_HumanMessageResetsStreak(t *testing.T) {
	g := newLoopGuard(3, 30*time.Second)
	now := time.Now()

	for i := 0; i < 3; i++ {
		if !g.Allow("chan-1", true, now.Add(time.Duration(i)*time.Millisecond)) {
			t.Fatalf("agent message #%d suppressed before reaching threshold", i)
		}
	}
	if g.Allow("chan-1", true, now.Add(3*time.Millisecond)) {
		t.Fatal("4th consecutive agent message allowed, want suppressed (threshold=3)")
	}

	// A human message should reset the streak.
	if !g.Allow("chan-1", false, now.Add(4*time.Millisecond)) {
		t.Fatal("human message suppressed, want always allowed")
	}

	if !g.Allow("chan-1", true, now.Add(5*time.Millisecond)) {
		t.Error("agent message after a human reset was suppressed, want allowed (fresh streak)")
	}
}

func TestLoopGuard_WindowExpiryResetsStreak(t *testing.T) {
	g := newLoopGuard(2, 10*time.Millisecond)
	now := time.Now()

	if !g.Allow("chan-1", true, now) {
		t.Fatal("1st agent message suppressed")
	}
	if !g.Allow("chan-1", true, now.Add(5*time.Millisecond)) {
		t.Fatal("2nd agent message suppressed")
	}
	if g.Allow("chan-1", true, now.Add(6*time.Millisecond)) {
		t.Fatal("3rd agent message within window allowed, want suppressed (threshold=2)")
	}

	// Beyond the window, the streak should restart rather than staying tripped.
	if !g.Allow("chan-1", true, now.Add(20*time.Millisecond)) {
		t.Error("agent message after window expiry was suppressed, want allowed (fresh streak)")
	}
}

func TestLoopGuard_ChannelsAreIndependent(t *testing.T) {
	g := newLoopGuard(1, 30*time.Second)
	now := time.Now()

	if !g.Allow("chan-1", true, now) {
		t.Fatal("1st message in chan-1 suppressed")
	}
	if g.Allow("chan-1", true, now.Add(time.Millisecond)) {
		t.Fatal("2nd message in chan-1 allowed, want suppressed (threshold=1)")
	}
	if !g.Allow("chan-2", true, now.Add(2*time.Millisecond)) {
		t.Error("1st message in a different channel (chan-2) suppressed, want allowed — channels must be independent")
	}
}
