package nostr

import "encoding/json"

const (
	// KindChannelMessage is the NIP-29 group chat message kind Buzz uses for
	// channel messages. Resolved against github.com/block/buzz's
	// crates/buzz-core/src/kind.rs (KIND_STREAM_MESSAGE = 9) — see
	// research.md Q5 / tasks.md P0.2.
	KindChannelMessage = 9

	// ChannelTagName is the NIP-01 tag name Buzz uses to scope an event to a
	// channel (a NIP-29 group tag, value = channel UUID). Buzz's relay
	// rejects kind:9 messages without this tag. Resolved against
	// github.com/block/buzz's NOSTR.md — see research.md Q5 / tasks.md P0.2.
	ChannelTagName = "h"

	// KindAgentProfile is Buzz's KIND_AGENT_PROFILE
	// (crates/buzz-core/src/kind.rs): a replaceable, pubkey-scoped event.
	// Verified 2026-08-02 against github.com/block/buzz's
	// buzz-relay/src/handlers/side_effects.rs (handle_agent_profile) and
	// NOSTR.md during the P2.2 spike: the relay's actual content schema is
	// {"channel_add_policy": "anyone"|"owner_only"|"nobody"} — not the
	// "agent metadata + owner reference" free-form profile kind.rs's doc
	// comment describes. It carries no channel (#h) tag — it is global,
	// keyed by pubkey, not channel-scoped. See tasks.md P2.2 and
	// implementation-notes.md for the full finding.
	KindAgentProfile = 10100

	// KindChannelMembership is Buzz's KIND_NIP29_PUT_USER
	// (crates/buzz-core/src/kind.rs = 9000), a NIP-29 channel-membership
	// admin event. Confirmed against
	// buzz-relay/src/handlers/side_effects.rs's handle_put_user: carries a
	// required #h channel tag, a required PubkeyTagName ("p") target-member
	// tag, and an optional RoleTagName ("role") tag (absent = no role
	// change — the relay preserves the member's current role rather than
	// defaulting it). See research.md Q2/Q5 and tasks.md P2.3.
	KindChannelMembership = 9000

	// PubkeyTagName is the NIP-01 "p" tag Buzz uses on a kind:9000 event to
	// name the target member pubkey.
	PubkeyTagName = "p"

	// RoleTagName is the tag Buzz uses on a kind:9000 event to set a
	// member's channel role.
	RoleTagName = "role"

	// RoleBot is MemberRole::Bot's canonical string
	// (crates/buzz-core/src/channel.rs) — Buzz's channel-membership-based
	// agent designation, a separate, non-hierarchical role (not just "low
	// permission").
	RoleBot = "bot"
)

// Filter represents a NIP-01 subscription filter (the object sent as part of
// a REQ message).
type Filter struct {
	Kinds      []int
	ChannelIDs []string
	// Since is left unset by the New*Filter constructors below (initial
	// connect gets the full backlog). Client populates it per relay on
	// reconnect, from that relay's own high-water mark, for backfill of
	// events missed during a disconnect (FR-010; see client.go's Client doc
	// comment for the full per-relay rationale).
	Since *int64
}

// NewChannelFilter builds a Filter matching Buzz channel messages
// (KindChannelMessage) scoped to the given channel IDs via ChannelTagName
// (FR-002).
func NewChannelFilter(channelIDs []string) Filter {
	return Filter{
		Kinds:      []int{KindChannelMessage},
		ChannelIDs: channelIDs,
	}
}

// NewMembershipFilter builds a Filter matching Buzz channel-membership admin
// events (KindChannelMembership) scoped to the given channel IDs via
// ChannelTagName (P2.3).
func NewMembershipFilter(channelIDs []string) Filter {
	return Filter{
		Kinds:      []int{KindChannelMembership},
		ChannelIDs: channelIDs,
	}
}

// NewAgentProfileFilter builds a Filter matching Buzz agent-profile events
// (KindAgentProfile). Unscoped by channel — kind:10100 is a global,
// pubkey-keyed event, not a channel-tagged one (P2.3).
func NewAgentProfileFilter() Filter {
	return Filter{Kinds: []int{KindAgentProfile}}
}

// MarshalJSON serializes Filter to the NIP-01 filter object shape:
// {"kinds":[...], "#h":[...], "since":...}.
func (f Filter) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, 3)
	if len(f.Kinds) > 0 {
		m["kinds"] = f.Kinds
	}
	if len(f.ChannelIDs) > 0 {
		m["#"+ChannelTagName] = f.ChannelIDs
	}
	if f.Since != nil {
		m["since"] = *f.Since
	}
	return json.Marshal(m)
}

// NewSubscriptionRequest builds a NIP-01 REQ message frame:
// ["REQ", <subscription id>, <filter1>, <filter2>, ...]. Multiple filters
// under one subscription id are OR'd together per NIP-01 — used to combine
// distinct kind/tag-shaped filters (e.g. channel messages' #h-scoped kind:9
// alongside kind:10100's global, untagged filter) that can't be expressed as
// a single filter object.
func NewSubscriptionRequest(subscriptionID string, filters ...Filter) ([]byte, error) {
	frame := make([]any, 0, len(filters)+2)
	frame = append(frame, "REQ", subscriptionID)
	for _, f := range filters {
		frame = append(frame, f)
	}
	return json.Marshal(frame)
}
