package skill_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	. "nuimanbot/internal/usecase/skill" // Import skill package
)

// MockSkill implements domain.Skill for testing purposes.
type MockSkill struct {
	NameFunc                func() string
	DescriptionFunc         func() string
	InputSchemaFunc         func() map[string]any
	ExecuteFunc             func(ctx context.Context, params map[string]any) (*domain.SkillResult, error)
	RequiredPermissionsFunc func() []domain.Permission
	ConfigFunc              func() domain.SkillConfig
}

func (m *MockSkill) Name() string                { return m.NameFunc() }
func (m *MockSkill) Description() string         { return m.DescriptionFunc() }
func (m *MockSkill) InputSchema() map[string]any { return m.InputSchemaFunc() }
func (m *MockSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	return m.ExecuteFunc(ctx, params)
}
func (m *MockSkill) RequiredPermissions() []domain.Permission { return m.RequiredPermissionsFunc() }
func (m *MockSkill) Config() domain.SkillConfig               { return m.ConfigFunc() }

// MockSkillRegistry implements the SkillRegistry interface.
type MockSkillRegistry struct {
	GetFunc  func(name string) (domain.Skill, error)
	ListFunc func() []domain.Skill
}

func (m *MockSkillRegistry) Register(skill domain.Skill) error { return nil } // Not used in these tests
func (m *MockSkillRegistry) Get(name string) (domain.Skill, error) {
	return m.GetFunc(name)
}
func (m *MockSkillRegistry) List() []domain.Skill { return m.ListFunc() }
func (m *MockSkillRegistry) ListForUser(ctx context.Context, userID string) ([]domain.Skill, error) {
	return m.ListFunc(), nil
}

// MockSecurityService implements the domain.SecurityService interface.
type MockSecurityService struct {
	EncryptFunc       func(ctx context.Context, userID string, plaintext []byte) ([]byte, error)
	DecryptFunc       func(ctx context.Context, userID string, ciphertext []byte) ([]byte, error)
	ValidateInputFunc func(ctx context.Context, input string, maxLength int) (string, error)
	AuditFunc         func(ctx context.Context, event *domain.AuditEvent) error
}

func (m *MockSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	if m.EncryptFunc != nil {
		return m.EncryptFunc(ctx, userID, plaintext)
	}
	return nil, errors.New("Encrypt not implemented in mock")
}

func (m *MockSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	if m.DecryptFunc != nil {
		return m.DecryptFunc(ctx, userID, ciphertext)
	}
	return nil, errors.New("Decrypt not implemented in mock")
}

func (m *MockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	if m.ValidateInputFunc != nil {
		return m.ValidateInputFunc(ctx, input, maxLength)
	}
	return input, nil // Default: just return input
}

func (m *MockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.AuditFunc != nil {
		return m.AuditFunc(ctx, event)
	}
	return nil
}

func TestNewService(t *testing.T) {
	mockCfg := &config.SkillsSystemConfig{}
	mockRegistry := &MockSkillRegistry{}
	mockSecurity := &MockSecurityService{}

	svc := NewService(mockCfg, mockRegistry, mockSecurity)

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestExecute_SkillNotFound(t *testing.T) {
	mockRegistry := &MockSkillRegistry{
		GetFunc: func(name string) (domain.Skill, error) {
			return nil, domain.ErrNotFound // Simulate skill not found
		},
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.SkillsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "nonexistent", nil)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestExecute_SkillExecutionFailure(t *testing.T) {
	mockError := errors.New("skill failed")
	mockSkill := &MockSkill{
		NameFunc:    func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.SkillResult, error) { return nil, mockError },
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockSkillRegistry{
		GetFunc: func(name string) (domain.Skill, error) { return mockSkill, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event // Capture audit event
			return nil
		},
	}
	svc := NewService(&config.SkillsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "testskill", map[string]any{"param": "value"})

	if !errors.Is(err, mockError) {
		t.Errorf("Expected skill execution error, got: %v", err)
	}

	// Verify audit events
	select {
	case attemptEvent := <-auditEvents:
		if attemptEvent.Outcome != "attempt" {
			t.Errorf("Expected 'attempt' outcome, got %s", attemptEvent.Outcome)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for attempt audit event")
	}

	select {
	case failureEvent := <-auditEvents:
		if failureEvent.Outcome != "failure" {
			t.Errorf("Expected 'failure' outcome, got %s", failureEvent.Outcome)
		}
		if failureEvent.Details["error"] != mockError.Error() {
			t.Errorf("Expected error detail '%s', got '%s'", mockError.Error(), failureEvent.Details["error"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for failure audit event")
	}
}

func TestExecute_SkillExecutionSuccess(t *testing.T) {
	mockResult := &domain.SkillResult{Output: "skill output"}
	mockSkill := &MockSkill{
		NameFunc:    func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.SkillResult, error) { return mockResult, nil },
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockSkillRegistry{
		GetFunc: func(name string) (domain.Skill, error) { return mockSkill, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event // Capture audit event
			return nil
		},
	}
	svc := NewService(&config.SkillsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	result, err := svc.Execute(ctx, "testskill", map[string]any{"param": "value"})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Output != mockResult.Output {
		t.Errorf("Expected skill output '%s', got '%s'", mockResult.Output, result.Output)
	}

	// Verify audit events
	select {
	case attemptEvent := <-auditEvents:
		if attemptEvent.Outcome != "attempt" {
			t.Errorf("Expected 'attempt' outcome, got %s", attemptEvent.Outcome)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for attempt audit event")
	}

	select {
	case successEvent := <-auditEvents:
		if successEvent.Outcome != "success" {
			t.Errorf("Expected 'success' outcome, got %s", successEvent.Outcome)
		}
		if successEvent.Details["output_summary"] != mockResult.Output {
			t.Errorf("Expected output summary '%s', got '%s'", mockResult.Output, successEvent.Details["output_summary"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for success audit event")
	}
}

func TestListSkills(t *testing.T) {
	mockSkills := []domain.Skill{
		&MockSkill{NameFunc: func() string { return "skill1" }},
		&MockSkill{NameFunc: func() string { return "skill2" }},
	}
	mockRegistry := &MockSkillRegistry{
		ListFunc: func() []domain.Skill { return mockSkills },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.SkillsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	listedSkills, err := svc.ListSkills(ctx, "user1") // userID is ignored in mock for now
	if err != nil {
		t.Errorf("ListSkills returned an unexpected error: %v", err)
	}
	if len(listedSkills) != len(mockSkills) {
		t.Errorf("Expected %d skills, got %d", len(mockSkills), len(listedSkills))
	}
	if listedSkills[0].Name() != "skill1" || listedSkills[1].Name() != "skill2" {
		t.Errorf("Listed skills mismatch: got %+v", listedSkills)
	}
}

func TestListSkillsForUser(t *testing.T) {
	// For now, MockSkillRegistry.ListForUser simply calls List().
	// This test just ensures the method is callable and returns expected results from List().
	mockSkills := []domain.Skill{
		&MockSkill{NameFunc: func() string { return "skill1" }},
		&MockSkill{NameFunc: func() string { return "skill2" }},
	}
	mockRegistry := &MockSkillRegistry{
		ListFunc: func() []domain.Skill { return mockSkills },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.SkillsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	listedSkills, err := svc.ListSkills(ctx, "user1")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(listedSkills) != len(mockSkills) {
		t.Errorf("Expected %d skills, got %d", len(mockSkills), len(listedSkills))
	}
	if listedSkills[0].Name() != "skill1" || listedSkills[1].Name() != "skill2" {
		t.Errorf("Listed skills mismatch: got %+v", listedSkills)
	}
}
