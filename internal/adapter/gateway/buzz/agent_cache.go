package buzz

import "sync"

// agentCache tracks which Buzz pubkeys are known to be agents, per FR-009's
// need for a pubkey→is_agent lookup at message-receive time rather than a
// per-message field (see research.md Q2/Q5: Buzz has no per-message
// agent-identity tag). Populated from two signals, both handled by
// handleAgentStatusEvent (P2.3): a channel-membership kind:9000 event
// carrying role "bot" (nostr.RoleBot), or the mere presence of a kind:10100
// agent-profile event for a pubkey (see implementation-notes.md P2.2 for why
// presence, not content, is the signal for that kind).
//
// Safe for concurrent use: the relay read loop and any future
// membership/profile update paths may call Set/IsAgent concurrently.
type agentCache struct {
	mu      sync.RWMutex
	isAgent map[string]bool
}

func newAgentCache() *agentCache {
	return &agentCache{isAgent: make(map[string]bool)}
}

// Set records whether pubkey is known to be an agent.
func (c *agentCache) Set(pubkey string, isAgent bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isAgent[pubkey] = isAgent
}

// IsAgent reports whether pubkey is known to be an agent. Unknown pubkeys
// default to false.
func (c *agentCache) IsAgent(pubkey string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isAgent[pubkey]
}
