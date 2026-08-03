package crypto_test

import (
	"context"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
	"nuimanbot/internal/infrastructure/nostr"
)

func newTestVault(t *testing.T) domain.CredentialVault {
	t.Helper()
	tempDir := t.TempDir()
	key := make([]byte, 32)
	vault, err := crypto.NewFileCredentialVault(filepath.Join(tempDir, "vault.enc"), key)
	if err != nil {
		t.Fatalf("NewFileCredentialVault() error = %v", err)
	}
	return vault
}

func TestEnsureBuzzKeypair_GeneratesAndPersistsWhenAbsent(t *testing.T) {
	vault := newTestVault(t)
	ctx := context.Background()

	got, err := crypto.EnsureBuzzKeypair(ctx, vault, "buzz_private_key", domain.SecureString{})
	if err != nil {
		t.Fatalf("EnsureBuzzKeypair() error = %v", err)
	}
	if got.Value() == "" {
		t.Fatal("expected a generated private key, got empty string")
	}

	stored, err := vault.Retrieve(ctx, "buzz_private_key")
	if err != nil {
		t.Fatalf("vault.Retrieve() error = %v", err)
	}
	if stored.Value() != got.Value() {
		t.Errorf("stored key %q != returned key %q", stored.Value(), got.Value())
	}
}

func TestEnsureBuzzKeypair_UsesExistingConfiguredKeyUnchanged(t *testing.T) {
	vault := newTestVault(t)
	ctx := context.Background()

	existingKey, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	configured := domain.NewSecureStringFromString(existingKey)

	got, err := crypto.EnsureBuzzKeypair(ctx, vault, "buzz_private_key", configured)
	if err != nil {
		t.Fatalf("EnsureBuzzKeypair() error = %v", err)
	}
	if got.Value() != existingKey {
		t.Errorf("EnsureBuzzKeypair() = %q, want unchanged configured key %q", got.Value(), existingKey)
	}

	// Must not have written anything to the vault — the configured key takes
	// precedence and no generation should have occurred.
	if _, err := vault.Retrieve(ctx, "buzz_private_key"); err == nil {
		t.Error("expected no vault entry to be created when a private key is already configured")
	}
}

func TestEnsureBuzzKeypair_RestartReusesPersistedKey(t *testing.T) {
	vault := newTestVault(t)
	ctx := context.Background()

	first, err := crypto.EnsureBuzzKeypair(ctx, vault, "buzz_private_key", domain.SecureString{})
	if err != nil {
		t.Fatalf("EnsureBuzzKeypair() first call error = %v", err)
	}

	// Simulate a restart: config still has no private key configured, but
	// the vault now holds one from the prior run.
	second, err := crypto.EnsureBuzzKeypair(ctx, vault, "buzz_private_key", domain.SecureString{})
	if err != nil {
		t.Fatalf("EnsureBuzzKeypair() second call error = %v", err)
	}

	if second.Value() != first.Value() {
		t.Errorf("restart generated a new key: first=%q second=%q, want identical", first.Value(), second.Value())
	}
}

func TestEnsureBuzzKeypair_PublicKeyIsDerivable(t *testing.T) {
	vault := newTestVault(t)
	ctx := context.Background()

	priv, err := crypto.EnsureBuzzKeypair(ctx, vault, "buzz_private_key", domain.SecureString{})
	if err != nil {
		t.Fatalf("EnsureBuzzKeypair() error = %v", err)
	}

	pub, err := nostr.PublicKeyFromPrivateKey(priv.Value())
	if err != nil {
		t.Fatalf("PublicKeyFromPrivateKey() error = %v", err)
	}
	if len(pub) != 64 {
		t.Errorf("derived public key hex length = %d, want 64", len(pub))
	}
}
