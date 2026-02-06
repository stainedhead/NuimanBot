package security

import (
	"context"
	"errors"
	"log/slog" // Structured logging
	"time"

	"nuimanbot/internal/domain"
)

// ensure that Service implements domain.SecurityService
var _ domain.SecurityService = (*Service)(nil)

// InputValidator defines the interface for validating and sanitizing user input.
// This is an internal interface for the security usecase, not exported.
type InputValidator interface {
	ValidateInput(ctx context.Context, input string, maxLength int) (string, error)
}

// Auditor defines the interface for logging security-relevant events.
// This is an internal interface for the security usecase, not exported.
type Auditor interface {
	Audit(ctx context.Context, event *domain.AuditEvent) error
}

// Service implements the domain.SecurityService interface.
type Service struct {
	vault          domain.CredentialVault // Now using domain.CredentialVault
	inputValidator InputValidator
	auditor        Auditor
}

// NewService creates a new security service.
func NewService(vault domain.CredentialVault, inputValidator InputValidator, auditor Auditor) *Service {
	// Provide default implementations if not explicitly given
	if inputValidator == nil {
		inputValidator = NewDefaultInputValidator() // Assuming NewDefaultInputValidator exists
	}
	if auditor == nil {
		auditor = NewNoOpAuditor() // Assuming NewNoOpAuditor exists
	}

	return &Service{
		vault:          vault,
		inputValidator: inputValidator,
		auditor:        auditor,
	}
}

// Encrypt encrypts data for a specific user context.
func (s *Service) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return nil, errors.New("user-specific data encryption/decryption not yet implemented in SecurityService")
}

// Decrypt decrypts user-specific data.
func (s *Service) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return nil, errors.New("user-specific data encryption/decryption not yet implemented in SecurityService")
}

// ValidateInput sanitizes and validates user input.
func (s *Service) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	if s.inputValidator == nil {
		return "", errors.New("input validator not configured for security service")
	}
	return s.inputValidator.ValidateInput(ctx, input, maxLength)
}

// Audit logs a security-relevant event.
func (s *Service) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if s.auditor == nil {
		return errors.New("auditor not configured for security service")
	}
	return s.auditor.Audit(ctx, event)
}

// StoreCredential uses the vault to securely store a credential.
func (s *Service) StoreCredential(ctx context.Context, key string, value domain.SecureString) error {
	return s.vault.Store(ctx, key, value)
}

// RetrieveCredential uses the vault to retrieve a credential.
func (s *Service) RetrieveCredential(ctx context.Context, key string) (domain.SecureString, error) {
	return s.vault.Retrieve(ctx, key)
}

// DeleteCredential uses the vault to delete a credential.
func (s *Service) DeleteCredential(ctx context.Context, key string) error {
	return s.vault.Delete(ctx, key)
}

// ListCredentials uses the vault to list credential keys.
func (s *Service) ListCredentials(ctx context.Context) ([]string, error) {
	return s.vault.List(ctx)
}

// RotateMasterKey uses the vault to rotate the master encryption key.
func (s *Service) RotateMasterKey(ctx context.Context) error {
	return s.vault.RotateKey(ctx)
}

// NoOpAuditor is a placeholder implementation for Auditor for MVP.
type NoOpAuditor struct{}

// NewNoOpAuditor creates a new instance of NoOpAuditor.
func NewNoOpAuditor() *NoOpAuditor {
	return &NoOpAuditor{}
}

func (n *NoOpAuditor) Audit(ctx context.Context, event *domain.AuditEvent) error {
	slog.Info("AUDIT",
		"timestamp", event.Timestamp.Format(time.RFC3339),
		"user_id", event.UserID,
		"action", event.Action,
		"resource", event.Resource,
		"outcome", event.Outcome,
		"details", event.Details,
	)
	return nil
}
