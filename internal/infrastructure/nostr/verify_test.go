package nostr_test

import (
	"testing"

	"nuimanbot/internal/infrastructure/nostr"
)

func signedTestEvent(t *testing.T) (nostr.Event, string) {
	t.Helper()
	privHex, _, err := nostr.GenerateKeypair()
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
	return e, privHex
}

func TestVerify_ValidSignatureAccepted(t *testing.T) {
	e, _ := signedTestEvent(t)

	valid, err := nostr.Verify(e)
	if err != nil {
		t.Fatalf("Verify() unexpected error = %v", err)
	}
	if !valid {
		t.Error("Verify() = false, want true for a validly signed event")
	}
}

func TestVerify_TamperedContentRejected(t *testing.T) {
	e, _ := signedTestEvent(t)
	e.Content = "tampered content" // ID/Sig no longer match the content

	valid, err := nostr.Verify(e)
	if err != nil {
		t.Fatalf("Verify() unexpected error = %v", err)
	}
	if valid {
		t.Error("Verify() = true for tampered content, want false")
	}
}

func TestVerify_WrongPubKeyRejected(t *testing.T) {
	e, _ := signedTestEvent(t)

	_, otherPub, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	e.PubKey = otherPub
	// Recompute ID to match the swapped pubkey field (as an attacker forging
	// authorship would), leaving Sig as the original signer's signature —
	// the signature must fail to verify against the new pubkey.
	e.ID = nostr.ComputeID(e)

	valid, err := nostr.Verify(e)
	if err != nil {
		t.Fatalf("Verify() unexpected error = %v", err)
	}
	if valid {
		t.Error("Verify() = true for a signature that doesn't match the claimed pubkey, want false")
	}
}

func TestVerify_MalformedEventRejectedWithoutPanic(t *testing.T) {
	// withValidID recomputes ID from e's other fields so the malformed-field
	// cases below reach signature/pubkey parsing instead of short-circuiting
	// on Verify's earlier "ID doesn't match content" tamper check.
	withValidID := func(e nostr.Event) nostr.Event {
		e.ID = nostr.ComputeID(e)
		return e
	}

	tests := []struct {
		name  string
		event nostr.Event
	}{
		{
			name:  "missing id/pubkey/sig",
			event: nostr.Event{Content: "no signature fields"},
		},
		{
			name: "non-hex pubkey",
			event: withValidID(nostr.Event{
				PubKey: "not-hex!!",
				Sig:    "bb",
			}),
		},
		{
			name: "wrong-length pubkey",
			event: withValidID(nostr.Event{
				PubKey: "abcd",
				Sig:    "bb",
			}),
		},
		{
			name: "non-hex signature",
			event: withValidID(nostr.Event{
				PubKey: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
				Sig:    "not-hex!!",
			}),
		},
		{
			name: "wrong-length signature",
			event: withValidID(nostr.Event{
				PubKey: "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
				Sig:    "abcd",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Verify() panicked on malformed input: %v", r)
				}
			}()

			valid, err := nostr.Verify(tt.event)
			if valid {
				t.Error("Verify() = true for malformed event, want false")
			}
			if err == nil {
				t.Error("Verify() error = nil, want a descriptive error for malformed input")
			}
		})
	}
}

func TestVerify_ConcurrentCallsAreSafe(t *testing.T) {
	e, _ := signedTestEvent(t)

	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func() {
			_, _ = nostr.Verify(e)
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
