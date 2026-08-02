package crypto

import (
	"context"
	"fmt"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/nostr"
)

// EnsureBuzzKeypair returns the Buzz agent's secp256k1 private key
// (hex-encoded, matching the format nostr.Sign/nostr.Verify expect), per
// FR-007's generate-if-absent requirement.
//
// If configured already holds a key, it is returned unchanged — an operator-
// supplied key always takes precedence and is never regenerated. Otherwise
// the vault is checked under vaultKey for a key persisted by a previous run
// (so a restart reuses it rather than generating a new one each time); if
// neither is present, a new keypair is generated via nostr.GenerateKeypair
// and the private key is persisted to vault before being returned.
func EnsureBuzzKeypair(ctx context.Context, vault domain.CredentialVault, vaultKey string, configured domain.SecureString) (domain.SecureString, error) {
	if configured.Value() != "" {
		return configured, nil
	}

	if existing, err := vault.Retrieve(ctx, vaultKey); err == nil && existing.Value() != "" {
		return existing, nil
	}

	privKeyHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		return domain.SecureString{}, fmt.Errorf("failed to generate Buzz keypair: %w", err)
	}

	generated := domain.NewSecureStringFromString(privKeyHex)
	if err := vault.Store(ctx, vaultKey, generated); err != nil {
		return domain.SecureString{}, fmt.Errorf("failed to persist generated Buzz keypair: %w", err)
	}

	return generated, nil
}
