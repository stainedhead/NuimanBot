package nostr_test

import (
	"encoding/json"
	"testing"

	"nuimanbot/internal/infrastructure/nostr"
)

// TestComputeID_MatchesKnownVector checks event ID computation against an
// independently-computed reference (Python hashlib/json, not this package),
// per NIP-01's canonical serialization: sha256(JSON([0,pubkey,created_at,kind,tags,content])).
func TestComputeID_MatchesKnownVector(t *testing.T) {
	tests := []struct {
		name   string
		event  nostr.Event
		wantID string
	}{
		{
			name: "mixed content with escapes and tags",
			event: nostr.Event{
				PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
				CreatedAt: 1700000000,
				Kind:      1,
				Tags: [][]string{
					{"e", "abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"},
					{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459"},
				},
				Content: "Hello, Nostr!\nSecond line with \"quotes\" and \\backslash\\ and\ttab.",
			},
			wantID: "421fba733bbcdacf55fd21c0f23d2e7a09bcce2f1e143419352e8482ed81f504",
		},
		{
			name: "empty tags and content",
			event: nostr.Event{
				PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
				CreatedAt: 0,
				Kind:      0,
				Tags:      [][]string{},
				Content:   "",
			},
			wantID: "ea0a5edc700281045fa6f7d1d9242dfbac9c05c51bc5a8b5e4e2608f73923523",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nostr.ComputeID(tt.event)
			if got != tt.wantID {
				t.Errorf("ComputeID() = %q, want %q", got, tt.wantID)
			}
			if len(got) != 64 {
				t.Errorf("ComputeID() length = %d, want 64 (hex-encoded SHA-256)", len(got))
			}
		})
	}
}

func TestComputeID_Deterministic(t *testing.T) {
	e := nostr.Event{
		PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
		CreatedAt: 1700000000,
		Kind:      9,
		Tags:      [][]string{{"h", "channel-uuid-1"}},
		Content:   "hello channel",
	}
	id1 := nostr.ComputeID(e)
	id2 := nostr.ComputeID(e)
	if id1 != id2 {
		t.Errorf("ComputeID() not deterministic: %q != %q", id1, id2)
	}
}

func TestGenerateKeypair_ProducesValidHexKeys(t *testing.T) {
	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	if len(privHex) != 64 {
		t.Errorf("private key hex length = %d, want 64", len(privHex))
	}
	if len(pubHex) != 64 {
		t.Errorf("public key hex length = %d, want 64 (x-only per BIP-340)", len(pubHex))
	}

	derivedPub, err := nostr.PublicKeyFromPrivateKey(privHex)
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey() error = %v", err)
	}
	if derivedPub != pubHex {
		t.Errorf("derived pubkey %q != generated pubkey %q", derivedPub, pubHex)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	e := nostr.Event{
		CreatedAt: 1700000000,
		Kind:      9,
		Tags:      [][]string{{"h", "channel-uuid-1"}},
		Content:   "hello channel",
	}

	if err := nostr.Sign(&e, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if e.PubKey != pubHex {
		t.Errorf("Sign() did not set PubKey correctly: got %q, want %q", e.PubKey, pubHex)
	}
	if e.ID == "" {
		t.Fatal("Sign() did not set ID")
	}
	if e.Sig == "" {
		t.Fatal("Sign() did not set Sig")
	}

	valid, err := nostr.Verify(e)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !valid {
		t.Error("Verify() = false for a validly signed event, want true")
	}
}

// TestEvent_MarshalJSON_MatchesNIP01WireShape checks that json.Marshal(Event)
// produces the exact NIP-01 wire field names (id/pubkey/created_at/kind/
// tags/content/sig) — Client.Publish (P2.1) relies on this to build an
// outgoing ["EVENT", event] frame other relays/clients can parse.
func TestEvent_MarshalJSON_MatchesNIP01WireShape(t *testing.T) {
	e := nostr.Event{
		ID:        "abc123",
		PubKey:    "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
		CreatedAt: 1700000000,
		Kind:      9,
		Tags:      [][]string{{"h", "channel-uuid-1"}},
		Content:   "hello",
		Sig:       "deadbeef",
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event JSON: %v", err)
	}

	for _, key := range []string{"id", "pubkey", "created_at", "kind", "tags", "content", "sig"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("marshaled event missing wire field %q: %v", key, decoded)
		}
	}
}

func TestEvent_Tag_ReturnsFirstMatchingTagValue(t *testing.T) {
	e := nostr.Event{Tags: [][]string{{"h", "channel-uuid-1"}, {"p", "some-pubkey"}, {"role", "bot"}}}

	if v, ok := e.Tag("h"); !ok || v != "channel-uuid-1" {
		t.Errorf("Tag(\"h\") = (%q, %v), want (\"channel-uuid-1\", true)", v, ok)
	}
	if v, ok := e.Tag("role"); !ok || v != "bot" {
		t.Errorf("Tag(\"role\") = (%q, %v), want (\"bot\", true)", v, ok)
	}
	if _, ok := e.Tag("missing"); ok {
		t.Error("Tag(\"missing\") returned ok=true, want false")
	}
}

func TestEvent_Tag_SkipsShortTags(t *testing.T) {
	e := nostr.Event{Tags: [][]string{{"solo"}}}
	if _, ok := e.Tag("solo"); ok {
		t.Error("Tag() matched a single-element tag, want it skipped (no value)")
	}
}

func TestSign_DifferentKeysProduceDifferentSignatures(t *testing.T) {
	priv1, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	priv2, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	base := nostr.Event{CreatedAt: 1700000000, Kind: 9, Tags: [][]string{}, Content: "same content"}

	e1 := base
	if err := nostr.Sign(&e1, priv1); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	e2 := base
	if err := nostr.Sign(&e2, priv2); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if e1.PubKey == e2.PubKey {
		t.Fatal("expected different pubkeys for different private keys")
	}
	if e1.Sig == e2.Sig {
		t.Error("expected different signatures for events signed by different keys")
	}
}
