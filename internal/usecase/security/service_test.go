package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	usecaseSecurity "nuimanbot/internal/usecase/security" // Alias to avoid conflict with test package
)

// MockCredentialVault is a mock implementation of domain.CredentialVault for testing.
type MockCredentialVault struct {
	StoreFunc     func(ctx context.Context, key string, value domain.SecureString) error
	RetrieveFunc  func(ctx context.Context, key string) (domain.SecureString, error)
	DeleteFunc    func(ctx context.Context, key string) error
	RotateKeyFunc func(ctx context.Context) error
	ListFunc      func(ctx context.Context) ([]string, error)
}

func (m *MockCredentialVault) Store(ctx context.Context, key string, value domain.SecureString) error {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, key, value)
	}
	return nil
}

func (m *MockCredentialVault) Retrieve(ctx context.Context, key string) (domain.SecureString, error) {
	if m.RetrieveFunc != nil {
		return m.RetrieveFunc(ctx, key)
	}
	return domain.SecureString{}, nil
}

func (m *MockCredentialVault) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}

func (m *MockCredentialVault) RotateKey(ctx context.Context) error {
	if m.RotateKeyFunc != nil {
		return m.RotateKeyFunc(ctx)
	}
	return nil
}

func (m *MockCredentialVault) List(ctx context.Context) ([]string, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockInputValidator is a mock implementation of usecaseSecurity.InputValidator.
type MockInputValidator struct {
	ValidateInputFunc func(ctx context.Context, input string, maxLength int) (string, error)
}

func (m *MockInputValidator) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	if m.ValidateInputFunc != nil {
		return m.ValidateInputFunc(ctx, input, maxLength)
	}
	return input, nil // Default: just return input
}

// MockAuditor is a mock implementation of usecaseSecurity.Auditor.
type MockAuditor struct {
	AuditFunc func(ctx context.Context, event *domain.AuditEvent) error
	Events    []domain.AuditEvent // To capture audited events
}

func (m *MockAuditor) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.AuditFunc != nil {
		return m.AuditFunc(ctx, event)
	}
	m.Events = append(m.Events, *event) // Store a copy of the event
	return nil
}

// Helper to create a new SecurityService for tests
func newTestSecurityService() (*usecaseSecurity.Service, *MockCredentialVault, *MockInputValidator, *MockAuditor) {
	mockVault := &MockCredentialVault{}
	mockInputValidator := &MockInputValidator{}
	mockAuditor := &MockAuditor{}
	service := usecaseSecurity.NewService(mockVault, mockInputValidator, mockAuditor)
	return service, mockVault, mockInputValidator, mockAuditor
}

func TestSecurityService_EncryptDecrypt(t *testing.T) {
	service, _, _, _ := newTestSecurityService()
	ctx := context.Background()

	_, err := service.Encrypt(ctx, "user1", []byte("data"))
	if err == nil || err.Error() != "user-specific data encryption/decryption not yet implemented in SecurityService" {
		t.Errorf("Encrypt: expected 'not yet implemented' error, got %v", err)
	}

	_, err = service.Decrypt(ctx, "user1", []byte("data"))
	if err == nil || err.Error() != "user-specific data encryption/decryption not yet implemented in SecurityService" {
		t.Errorf("Decrypt: expected 'not yet implemented' error, got %v", err)
	}
}

func TestSecurityService_ValidateInput(t *testing.T) {
	service, _, mockInputValidator, _ := newTestSecurityService()
	ctx := context.Background()

	tests := []struct {
		name        string
		input       string
		maxLength   int
		mockOut     string
		mockErr     error
		expectedOut string
		expectedErr error
	}{
		{
			name:        "Valid input, mock returns success",
			input:       "hello world",
			maxLength:   100,
			mockOut:     "hello world",
			mockErr:     nil,
			expectedOut: "hello world",
			expectedErr: nil,
		},
		{
			name:        "Invalid input, mock returns error",
			input:       "bad input",
			maxLength:   100,
			mockOut:     "",
			mockErr:     errors.New("mock validation failed: bad word"),
			expectedOut: "",
			expectedErr: errors.New("mock validation failed: bad word"),
		},
		{
			name:        "Empty input, mock returns empty",
			input:       "",
			maxLength:   100,
			mockOut:     "",
			mockErr:     nil,
			expectedOut: "",
			expectedErr: nil,
		},
		{
			name:        "Whitespace input, mock trims",
			input:       "   trim me   ",
			maxLength:   100,
			mockOut:     "trim me",
			mockErr:     nil,
			expectedOut: "trim me",
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Configure the mock for this specific test case
			mockInputValidator.ValidateInputFunc = func(ctx context.Context, input string, maxLength int) (string, error) {
				if input != tt.input {
					t.Fatalf("Mock received unexpected input: got %q, want %q", input, tt.input)
				}
				if maxLength != tt.maxLength {
					t.Fatalf("Mock received unexpected maxLength: got %d, want %d", maxLength, tt.maxLength)
				}
				return tt.mockOut, tt.mockErr
			}

			got, err := service.ValidateInput(ctx, tt.input, tt.maxLength)

			if !errors.Is(err, tt.expectedErr) && (err == nil || tt.expectedErr == nil || err.Error() != tt.expectedErr.Error()) {
				t.Errorf("ValidateInput() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if tt.expectedErr == nil && got != tt.expectedOut {
				t.Errorf("ValidateInput() got = %q, expected %q", got, tt.expectedOut)
			}
		})
	}
}

func TestSecurityService_Audit(t *testing.T) {
	service, _, _, mockAuditor := newTestSecurityService()
	ctx := context.Background()

	event := domain.AuditEvent{
		UserID:    "testuser",
		Action:    "login",
		Resource:  "system",
		Outcome:   "success",
		Timestamp: time.Now(), // Set timestamp for auditor
	}

	err := service.Audit(ctx, &event)
	if err != nil {
		t.Errorf("Audit failed unexpectedly: %v", err)
	}

	if len(mockAuditor.Events) != 1 {
		t.Fatalf("Expected 1 audit event, got %d", len(mockAuditor.Events))
	}
	if mockAuditor.Events[0].Action != "login" {
		t.Errorf("Expected audited action 'login', got '%s'", mockAuditor.Events[0].Action)
	}
}

func TestSecurityService_CredentialVaultOperations(t *testing.T) {
	service, mockVault, _, _ := newTestSecurityService()
	ctx := context.Background()

	// Test StoreCredential
	mockVault.StoreFunc = func(ctx context.Context, key string, value domain.SecureString) error {
		if key != "test_key" || value.Value() != "test_value" {
			return errors.New("store func received wrong arguments")
		}
		return nil
	}
	err := service.StoreCredential(ctx, "test_key", domain.NewSecureStringFromString("test_value"))
	if err != nil {
		t.Errorf("StoreCredential failed: %v", err)
	}

	// Test RetrieveCredential
	mockVault.RetrieveFunc = func(ctx context.Context, key string) (domain.SecureString, error) {
		if key != "test_key" {
			return domain.SecureString{}, errors.New("retrieve func received wrong key")
		}
		return domain.NewSecureStringFromString("retrieved_value"), nil
	}
	val, err := service.RetrieveCredential(ctx, "test_key")
	if err != nil {
		t.Errorf("RetrieveCredential failed: %v", err)
	}
	if val.Value() != "retrieved_value" {
		t.Errorf("RetrieveCredential got %s, want retrieved_value", val.Value())
	}

	// Test DeleteCredential
	mockVault.DeleteFunc = func(ctx context.Context, key string) error {
		if key != "test_key" {
			return errors.New("delete func received wrong key")
		}
		return nil
	}
	err = service.DeleteCredential(ctx, "test_key")
	if err != nil {
		t.Errorf("DeleteCredential failed: %v", err)
	}

	// Test ListCredentials
	mockVault.ListFunc = func(ctx context.Context) ([]string, error) {
		return []string{"key1", "key2"}, nil
	}
	keys, err := service.ListCredentials(ctx)
	if err != nil {
		t.Errorf("ListCredentials failed: %v", err)
	}
	if len(keys) != 2 || keys[0] != "key1" || keys[1] != "key2" {
		t.Errorf("ListCredentials got %v, want [key1 key2]", keys)
	}

	// Test RotateMasterKey
	mockVault.RotateKeyFunc = func(ctx context.Context) error {
		return nil
	}
	err = service.RotateMasterKey(ctx)
	if err != nil {
		t.Errorf("RotateMasterKey failed: %v", err)
	}
}
