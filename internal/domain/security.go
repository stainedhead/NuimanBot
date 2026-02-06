package domain

import (
	"context"
	"time"
)

// SecureString wraps sensitive data with automatic zeroing.
// In a real implementation, this would involve memory locking and manual clearing.
type SecureString struct {
	value []byte
}

// NewSecureString creates a new SecureString from a byte slice.
func NewSecureString(val []byte) SecureString {
	return SecureString{value: val}
}

// NewSecureStringFromString creates a new SecureString from a string.
func NewSecureStringFromString(s string) SecureString {
	return NewSecureString([]byte(s))
}

// Value returns the string value of the SecureString.
// USE WITH CAUTION: The returned string might be copied and persist longer than desired.
func (s SecureString) Value() string {
	return string(s.value)
}

// Zero attempts to zero out the memory holding the sensitive data.
func (s *SecureString) Zero() {
	if s.value != nil {
		for i := range s.value {
			s.value[i] = 0
		}
		s.value = nil
	}
}

// AuditEvent represents a security-relevant event for auditing purposes.
type AuditEvent struct {
	Timestamp time.Time
	UserID    string         // User who performed the action
	Action    string         // e.g., "login", "skill_execution", "credential_access"
	Resource  string         // e.g., "user_settings", "calculator_skill", "ANTHROPIC_API_KEY"
	Outcome   string         // "success", "failure", "denied"
	Details   map[string]any // Additional context about the event
	SourceIP  string
	Platform  Platform // Now directly referencing Platform, as it's in the same domain package
}

// SecurityService defines the contract for security operations.
type SecurityService interface {
	// Encrypt encrypts data for a specific user context.
	Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error)

	// Decrypt decrypts user-specific data.
	Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error)

	// ValidateInput sanitizes and validates user input.
	ValidateInput(ctx context.Context, input string, maxLength int) (string, error)

	// Audit logs a security-relevant event.
	Audit(ctx context.Context, event *AuditEvent) error
}

// CredentialVault defines the contract for securely storing and retrieving credentials.
type CredentialVault interface {
	// Store securely stores a credential associated with a key.
	Store(ctx context.Context, key string, value SecureString) error

	// Retrieve retrieves a credential associated with a key.
	Retrieve(ctx context.Context, key string) (SecureString, error)

	// Delete removes a credential associated with a key.
	Delete(ctx context.Context, key string) error

	// RotateKey rotates the master encryption key used by the vault.
	RotateKey(ctx context.Context) error

	// List returns all stored credential keys (not their values).
	List(ctx context.Context) ([]string, error)
}
