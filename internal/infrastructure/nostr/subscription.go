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
)

// Filter represents a NIP-01 subscription filter (the object sent as part of
// a REQ message).
type Filter struct {
	Kinds      []int
	ChannelIDs []string
	Since      *int64
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
// ["REQ", <subscription id>, <filter>].
func NewSubscriptionRequest(subscriptionID string, filter Filter) ([]byte, error) {
	return json.Marshal([]any{"REQ", subscriptionID, filter})
}
