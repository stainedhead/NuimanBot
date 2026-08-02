package nostr

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Verify checks that e's signature is a valid BIP-340 Schnorr signature by
// e.PubKey over e.ID, and that e.ID matches the canonical serialization of
// e's other fields (detecting tampered content). It is a pure function with
// no shared state, safe to call concurrently and off the relay read loop
// (NFR: signature verification must not block the read loop for other
// events). Malformed events (bad hex, wrong-length keys/signatures) return
// (false, error) rather than panicking.
func Verify(e Event) (bool, error) {
	if e.ID == "" || e.PubKey == "" || e.Sig == "" {
		return false, fmt.Errorf("event missing id, pubkey, or sig")
	}

	if ComputeID(e) != e.ID {
		return false, nil
	}

	pubBytes, err := hex.DecodeString(e.PubKey)
	if err != nil {
		return false, fmt.Errorf("invalid pubkey hex: %w", err)
	}
	pubKey, err := schnorr.ParsePubKey(pubBytes)
	if err != nil {
		return false, fmt.Errorf("invalid pubkey: %w", err)
	}

	sigBytes, err := hex.DecodeString(e.Sig)
	if err != nil {
		return false, fmt.Errorf("invalid signature hex: %w", err)
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return false, fmt.Errorf("invalid signature: %w", err)
	}

	idBytes, err := hex.DecodeString(e.ID)
	if err != nil {
		return false, fmt.Errorf("invalid id hex: %w", err)
	}

	return sig.Verify(idBytes, pubKey), nil
}
