package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClaims_SubjectIsStable verifies newClaims produces the same subject for the same key.
func TestNewClaims_SubjectIsStable(t *testing.T) {
	c1 := newClaims("my-api-key")
	c2 := newClaims("my-api-key")

	sub1, err := c1.GetSubject()
	require.NoError(t, err)
	sub2, err := c2.GetSubject()
	require.NoError(t, err)

	assert.Equal(t, sub1, sub2, "same key should produce same subject")
	assert.NotEmpty(t, sub1)
}

// TestNewClaims_DifferentKeysDifferentSubjects verifies distinct API keys produce different subjects.
func TestNewClaims_DifferentKeysDifferentSubjects(t *testing.T) {
	c1 := newClaims("key-alpha")
	c2 := newClaims("key-beta")

	sub1, _ := c1.GetSubject()
	sub2, _ := c2.GetSubject()

	assert.NotEqual(t, sub1, sub2)
}

// TestSignToken_ValidSecretProducesToken verifies signToken returns a non-empty token.
func TestSignToken_ValidSecretProducesToken(t *testing.T) {
	claims := newClaims("some-key")
	token, err := signToken(claims, "a-valid-secret-for-signing-here!")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

// TestSignToken_TokenHasThreeParts verifies the JWT has header.payload.signature format.
func TestSignToken_TokenHasThreeParts(t *testing.T) {
	claims := newClaims("some-key")
	token, err := signToken(claims, "a-valid-secret-for-signing-here!")
	require.NoError(t, err)

	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	assert.Equal(t, 2, parts, "JWT must have exactly 2 dots (header.payload.signature)")
}

// TestPrincipalID_IsHexPrefixOfSHA256 verifies principalID returns a 16-char hex string.
func TestPrincipalID_IsHexPrefixOfSHA256(t *testing.T) {
	id := principalID("test-key")
	assert.Len(t, id, 16, "principalID should be a 16-char hex string")

	// All characters must be valid hex.
	for _, c := range id {
		if !isHexChar(c) {
			t.Errorf("character %q is not a hex digit", c)
		}
	}
}

// TestPrincipalID_RawKeyNotExposed verifies the raw key doesn't appear in the principal ID.
func TestPrincipalID_RawKeyNotExposed(t *testing.T) {
	key := "my-secret-api-key"
	id := principalID(key)
	assert.NotContains(t, id, key, "raw key must not appear in principal ID")
}

// isHexChar reports whether r is a valid lowercase hex character.
func isHexChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
}
