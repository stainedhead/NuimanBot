package buzz

import (
	"sync"
	"time"
)

const (
	// buzzLoopGuardMaxConsecutiveAgent is the number of consecutive
	// agent-authored messages (per channel, with no intervening human
	// message) this gateway will forward to the message handler before
	// suppressing further ones. Chosen to allow a normal short agent-to-
	// agent back-and-forth (e.g. a handful of turns resolving a request)
	// while still bounding a runaway exchange to a small, fixed number of
	// turns rather than letting it run indefinitely.
	buzzLoopGuardMaxConsecutiveAgent = 5

	// buzzLoopGuardWindow bounds how long a "consecutive" streak is tracked
	// before it ages out and restarts. Without this, a channel that
	// legitimately sees occasional agent traffic over hours/days would
	// eventually accumulate enough messages to trip the guard even though
	// no single exchange was ever a tight runaway loop.
	buzzLoopGuardWindow = 30 * time.Second
)

// loopGuard prevents runaway agent-to-agent reply chains (FR-009) by
// tracking, per channel, a rolling count of consecutive agent-authored
// messages received with no intervening human message. Once the count
// exceeds maxConsecutive within window, further agent-authored messages in
// that channel are suppressed (Allow returns false) until a human message
// resets the streak or the streak ages out past window.
//
// Time-window + consecutive-count, not reply-chain/event-tag based: Buzz's
// kind:9 channel messages carry no reliable "in-reply-to" tag for this
// gateway to follow (see research.md Q2 — the reply-loop heuristic itself
// was left as an implementation-time decision), so a chain can't be
// distinguished from independent traffic by tag alone. A consecutive-count
// heuristic is a direct, easily-reasoned-about bound on runaway auto-reply
// depth. It consults agentCache (P2.3) rather than a per-message field,
// consistent with Buzz having no per-message agent-identity marker.
type loopGuard struct {
	mu             sync.Mutex
	maxConsecutive int
	window         time.Duration
	channels       map[string]*channelStreak
}

type channelStreak struct {
	consecutiveAgent int
	firstAt          time.Time
}

func newLoopGuard(maxConsecutive int, window time.Duration) *loopGuard {
	return &loopGuard{
		maxConsecutive: maxConsecutive,
		window:         window,
		channels:       make(map[string]*channelStreak),
	}
}

// Allow reports whether a message from channelID, authored by an agent
// (isAgent) at receivedAt, should be forwarded to the message handler. Human
// messages always reset the channel's streak and are always allowed.
func (g *loopGuard) Allow(channelID string, isAgent bool, receivedAt time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !isAgent {
		delete(g.channels, channelID)
		return true
	}

	s, ok := g.channels[channelID]
	if !ok || receivedAt.Sub(s.firstAt) > g.window {
		s = &channelStreak{firstAt: receivedAt}
		g.channels[channelID] = s
	}
	s.consecutiveAgent++
	return s.consecutiveAgent <= g.maxConsecutive
}
