package security_test

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	usecaseSecurity "nuimanbot/internal/usecase/security"
)

// TestNewService_NilDependencies verifies defaults are used when nil interfaces are passed
func TestNewService_NilDependencies(t *testing.T) {
	t.Parallel()
	mockVault := &MockCredentialVault{}

	// Pass nil for both inputValidator and auditor — should use defaults (no panic)
	service := usecaseSecurity.NewService(mockVault, nil, nil)
	if service == nil {
		t.Fatal("NewService returned nil with nil dependencies")
	}

	ctx := context.Background()

	// Should not error — defaults were used
	_, err := service.ValidateInput(ctx, "hello", 100)
	if err != nil {
		t.Errorf("ValidateInput with default validator failed: %v", err)
	}
}

// TestNewNoOpAuditor verifies the NoOpAuditor works correctly
func TestNewNoOpAuditor(t *testing.T) {
	t.Parallel()
	auditor := usecaseSecurity.NewNoOpAuditor()
	if auditor == nil {
		t.Fatal("NewNoOpAuditor returned nil")
	}

	ctx := context.Background()
	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "test-user",
		Action:    "test-action",
		Resource:  "test-resource",
		Outcome:   "success",
		Details:   map[string]any{"key": "value"},
	}

	err := auditor.Audit(ctx, event)
	if err != nil {
		t.Errorf("NoOpAuditor.Audit failed: %v", err)
	}
}

// TestGenerateAPIKey verifies key generation works
func TestGenerateAPIKey(t *testing.T) {
	t.Parallel()
	service, _, _, _ := newTestSecurityService()

	ctx := context.Background()
	key1, err := service.GenerateAPIKey(ctx)
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if len(key1) == 0 {
		t.Error("Generated API key should not be empty")
	}

	// Keys should be unique
	key2, err := service.GenerateAPIKey(ctx)
	if err != nil {
		t.Fatalf("Second GenerateAPIKey failed: %v", err)
	}

	if key1 == key2 {
		t.Error("Generated keys should be unique, got identical keys")
	}
}

// TestValidateInput_NilValidator tests the branch where inputValidator is nil
// This is exercised by directly constructing a service without passing a validator
// and then setting it to nil (not directly possible via public API since NewService
// substitutes defaults — so this tests the default path).
func TestValidateInput_DefaultValidator(t *testing.T) {
	t.Parallel()
	mockVault := &MockCredentialVault{}

	// nil inputValidator triggers default
	service := usecaseSecurity.NewService(mockVault, nil, nil)

	ctx := context.Background()
	// Default validator should pass through simple inputs
	result, err := service.ValidateInput(ctx, "simple input", 100)
	if err != nil {
		t.Fatalf("ValidateInput with default validator: %v", err)
	}
	if result == "" {
		t.Error("Expected non-empty result from default validator")
	}
}

// TestAudit_NilAuditor tests the path where auditor is nil
// NewService substitutes a default, so this tests the default NoOp path.
func TestAudit_DefaultAuditor(t *testing.T) {
	t.Parallel()
	mockVault := &MockCredentialVault{}

	// nil auditor triggers default NoOpAuditor
	service := usecaseSecurity.NewService(mockVault, nil, nil)

	ctx := context.Background()
	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Action:    "login",
		Resource:  "system",
		Outcome:   "success",
	}

	err := service.Audit(ctx, event)
	if err != nil {
		t.Fatalf("Audit with default auditor failed: %v", err)
	}
}
