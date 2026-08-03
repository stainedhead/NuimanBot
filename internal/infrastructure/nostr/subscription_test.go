package nostr_test

import (
	"encoding/json"
	"testing"

	"nuimanbot/internal/infrastructure/nostr"
)

func TestKindChannelMessage_IsNine(t *testing.T) {
	// NIP-29 group chat message kind, per Buzz's kind.rs (KIND_STREAM_MESSAGE = 9),
	// resolved in research.md Q5 / tasks.md P0.2.
	if nostr.KindChannelMessage != 9 {
		t.Errorf("KindChannelMessage = %d, want 9", nostr.KindChannelMessage)
	}
}

func TestChannelTagName_IsH(t *testing.T) {
	// Buzz's channel-ID tag, per NOSTR.md: "kind:9 requires #h tag."
	if nostr.ChannelTagName != "h" {
		t.Errorf("ChannelTagName = %q, want %q", nostr.ChannelTagName, "h")
	}
}

func TestNewChannelFilter_CoversAllConfiguredChannelIDs(t *testing.T) {
	channelIDs := []string{"channel-uuid-1", "channel-uuid-2", "channel-uuid-3"}
	filter := nostr.NewChannelFilter(channelIDs)

	if len(filter.Kinds) != 1 || filter.Kinds[0] != nostr.KindChannelMessage {
		t.Errorf("filter.Kinds = %v, want [%d]", filter.Kinds, nostr.KindChannelMessage)
	}
	if len(filter.ChannelIDs) != len(channelIDs) {
		t.Fatalf("filter.ChannelIDs = %v, want %v", filter.ChannelIDs, channelIDs)
	}
	for i, id := range channelIDs {
		if filter.ChannelIDs[i] != id {
			t.Errorf("filter.ChannelIDs[%d] = %q, want %q", i, filter.ChannelIDs[i], id)
		}
	}
}

func TestFilter_MarshalJSON_MatchesNIP01Shape(t *testing.T) {
	filter := nostr.NewChannelFilter([]string{"channel-uuid-1", "channel-uuid-2"})

	data, err := json.Marshal(filter)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal filter JSON: %v", err)
	}

	kinds, ok := decoded["kinds"].([]any)
	if !ok || len(kinds) != 1 || kinds[0] != float64(9) {
		t.Errorf("decoded[\"kinds\"] = %v, want [9]", decoded["kinds"])
	}

	tag, ok := decoded["#h"].([]any)
	if !ok || len(tag) != 2 || tag[0] != "channel-uuid-1" || tag[1] != "channel-uuid-2" {
		t.Errorf("decoded[\"#h\"] = %v, want [channel-uuid-1 channel-uuid-2]", decoded["#h"])
	}
}

func TestNewSubscriptionRequest_BuildsREQFrame(t *testing.T) {
	filter := nostr.NewChannelFilter([]string{"channel-uuid-1"})

	frame, err := nostr.NewSubscriptionRequest("sub-1", filter)
	if err != nil {
		t.Fatalf("NewSubscriptionRequest() error = %v", err)
	}

	var decoded []any
	if err := json.Unmarshal(frame, &decoded); err != nil {
		t.Fatalf("failed to unmarshal REQ frame: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("REQ frame has %d elements, want 3 (\"REQ\", subscription id, filter)", len(decoded))
	}
	if decoded[0] != "REQ" {
		t.Errorf("frame[0] = %v, want \"REQ\"", decoded[0])
	}
	if decoded[1] != "sub-1" {
		t.Errorf("frame[1] = %v, want \"sub-1\"", decoded[1])
	}
	filterObj, ok := decoded[2].(map[string]any)
	if !ok {
		t.Fatalf("frame[2] is not an object: %v", decoded[2])
	}
	if _, ok := filterObj["#h"]; !ok {
		t.Errorf("frame[2] filter missing #h tag: %v", filterObj)
	}
}
