// Package nostr implements NIP-01 Nostr protocol concerns (event
// construction, signing, verification, subscriptions, and relay transport)
// as an infrastructure-layer dependency. No domain or config types are
// imported here — this package is a pure, swappable Nostr client.
package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Event represents a NIP-01 Nostr event.
type Event struct {
	ID        string
	PubKey    string
	CreatedAt int64
	Kind      int
	Tags      [][]string
	Content   string
	Sig       string
}

// ComputeID computes the NIP-01 event ID: the hex-encoded SHA-256 hash of the
// event's canonical serialization ([0, pubkey, created_at, kind, tags, content]).
func ComputeID(e Event) string {
	sum := sha256.Sum256(canonicalSerialize(e))
	return hex.EncodeToString(sum[:])
}

// canonicalSerialize builds the NIP-01 ID-computation byte string. It is
// hand-rolled rather than built with encoding/json because Go's JSON
// marshaler escapes additional characters (<, >, &, U+2028, U+2029) beyond
// the seven NIP-01 mandates, which would produce a different event ID than
// other Nostr clients for the same event content.
func canonicalSerialize(e Event) []byte {
	var b strings.Builder
	b.WriteString("[0,")
	writeCanonicalString(&b, e.PubKey)
	b.WriteByte(',')
	b.WriteString(strconv.FormatInt(e.CreatedAt, 10))
	b.WriteByte(',')
	b.WriteString(strconv.Itoa(e.Kind))
	b.WriteString(",[")
	for i, tag := range e.Tags {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('[')
		for j, t := range tag {
			if j > 0 {
				b.WriteByte(',')
			}
			writeCanonicalString(&b, t)
		}
		b.WriteByte(']')
	}
	b.WriteString("],")
	writeCanonicalString(&b, e.Content)
	b.WriteByte(']')
	return []byte(b.String())
}

// writeCanonicalString writes s as a JSON string literal, escaping only the
// seven characters NIP-01 requires (quote, backslash, and the C0 control
// codes \n \r \t \b \f); all other runes, including non-ASCII UTF-8, are
// written verbatim per NIP-01.
func writeCanonicalString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}

// GenerateKeypair generates a new secp256k1 keypair, returning the
// hex-encoded private key and hex-encoded x-only (BIP-340) public key that
// NIP-01 requires.
func GenerateKeypair() (privKeyHex, pubKeyHex string, err error) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}
	return hex.EncodeToString(priv.Serialize()), hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey())), nil
}

// PublicKeyFromPrivateKey derives the hex-encoded x-only public key for a
// hex-encoded secp256k1 private key.
func PublicKeyFromPrivateKey(privKeyHex string) (string, error) {
	priv, err := parsePrivateKey(privKeyHex)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey())), nil
}

// Sign computes the event ID and a BIP-340 Schnorr signature over it using
// the given hex-encoded private key, populating e.PubKey, e.ID, and e.Sig.
func Sign(e *Event, privKeyHex string) error {
	priv, err := parsePrivateKey(privKeyHex)
	if err != nil {
		return err
	}

	e.PubKey = hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	e.ID = ComputeID(*e)

	idBytes, err := hex.DecodeString(e.ID)
	if err != nil {
		return fmt.Errorf("invalid computed event id: %w", err)
	}

	sig, err := schnorr.Sign(priv, idBytes)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}
	e.Sig = hex.EncodeToString(sig.Serialize())
	return nil
}

func parsePrivateKey(privKeyHex string) (*btcec.PrivateKey, error) {
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: got %d bytes, want 32", len(privBytes))
	}
	priv, _ := btcec.PrivKeyFromBytes(privBytes)
	return priv, nil
}
