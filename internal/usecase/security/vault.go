package security

import (
	"context"
	"nuimanbot/internal/domain" // Import domain for SecureString
)

// CredentialVault defines the contract for securely storing and retrieving credentials.
type CredentialVault interface {
	// Store securely stores a credential associated with a key.
	Store(ctx context.Context, key string, value domain.SecureString) error

	// Retrieve retrieves a credential associated with a key.
	Retrieve(ctx context.Context, key string) (domain.SecureString, error)

	// Delete removes a credential associated with a key.
	Delete(ctx context.Context, key string) error

	// RotateKey rotates the master encryption key used by the vault.
	RotateKey(ctx context.Context) error

	// List returns all stored credential keys (not their values).
	List(ctx context.Context) ([]string, error)
}
