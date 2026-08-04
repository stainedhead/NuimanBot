package tool_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
	. "nuimanbot/internal/usecase/tool" // Import skill package
)

// MockTool implements domain.Tool for testing purposes.
type MockTool struct {
	NameFunc                func() string
	DescriptionFunc         func() string
	InputSchemaFunc         func() map[string]any
	ExecuteFunc             func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error)
	RequiredPermissionsFunc func() []domain.Permission
	ConfigFunc              func() domain.ToolConfig
}

func (m *MockTool) Name() string                { return m.NameFunc() }
func (m *MockTool) Description() string         { return m.DescriptionFunc() }
func (m *MockTool) InputSchema() map[string]any { return m.InputSchemaFunc() }
func (m *MockTool) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	return m.ExecuteFunc(ctx, params)
}
func (m *MockTool) RequiredPermissions() []domain.Permission { return m.RequiredPermissionsFunc() }
func (m *MockTool) Config() domain.ToolConfig                { return m.ConfigFunc() }

// MockToolRegistry implements the SkillRegistry interface.
type MockToolRegistry struct {
	GetFunc         func(name string) (domain.Tool, error)
	ListFunc        func() []domain.Tool
	ListForUserFunc func(ctx context.Context, userID string) ([]domain.Tool, error)
}

func (m *MockToolRegistry) Register(tool domain.Tool) error { return nil } // Not used in these tests
func (m *MockToolRegistry) Get(name string) (domain.Tool, error) {
	return m.GetFunc(name)
}
func (m *MockToolRegistry) List() []domain.Tool { return m.ListFunc() }
func (m *MockToolRegistry) ListForUser(ctx context.Context, userID string) ([]domain.Tool, error) {
	if m.ListForUserFunc != nil {
		return m.ListForUserFunc(ctx, userID)
	}
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

func (m *MockSecurityService) GenerateAPIKey(ctx context.Context) (string, error) {
	return "mock-api-key-12345678", nil
}

func TestNewService(t *testing.T) {
	mockCfg := &config.ToolsSystemConfig{}
	mockRegistry := &MockToolRegistry{}
	mockSecurity := &MockSecurityService{}

	svc := NewService(mockCfg, mockRegistry, mockSecurity)

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestExecute_SkillNotFound(t *testing.T) {
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) {
			return nil, domain.ErrNotFound // Simulate skill not found
		},
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	_, err := svc.Execute(ctx, "nonexistent", nil)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestExecute_SkillExecutionFailure(t *testing.T) {
	mockError := errors.New("skill failed")
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return nil, mockError
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event // Capture audit event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

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
	mockResult := &domain.ExecutionResult{Output: "skill output"}
	mockTool := &MockTool{
		NameFunc: func() string { return "testskill" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return mockResult, nil
		},
	}
	auditEvents := make(chan domain.AuditEvent, 2)
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event // Capture audit event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

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
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "skill1" }},
		&MockTool{NameFunc: func() string { return "skill2" }},
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	// "skill1"/"skill2" aren't in ToolPermissions, so they resolve to
	// DefaultToolPermission (RoleUser) — a RoleUser is sufficient to see both.
	listedSkills, err := svc.ListTools(ctx, &domain.User{ID: "user1", Role: domain.RoleUser})
	if err != nil {
		t.Errorf("ListSkills returned an unexpected error: %v", err)
	}
	if len(listedSkills) != len(mockTools) {
		t.Errorf("Expected %d skills, got %d", len(mockTools), len(listedSkills))
	}
}

func TestListSkillsForUser(t *testing.T) {
	// For now, MockToolRegistry.ListForUser simply calls List().
	// This test just ensures the method is callable and returns expected results from List().
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "skill1" }},
		&MockTool{NameFunc: func() string { return "skill2" }},
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	ctx := context.Background()
	listedSkills, err := svc.ListTools(ctx, &domain.User{ID: "user1", Role: domain.RoleUser})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(listedSkills) != len(mockTools) {
		t.Errorf("Expected %d skills, got %d", len(mockTools), len(listedSkills))
	}
}

// TestListTools_FiltersAdminOnlyToolsForNonAdmin is the FR-002 regression
// test: ListTools must exclude admin-only tools ("github", "coding_agent")
// for a non-admin user, and include them for an admin — the same RBAC
// resolveRequiredRole/checkPermission applies to Execute/ExecuteWithUser.
func TestListTools_FiltersAdminOnlyToolsForNonAdmin(t *testing.T) {
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "calculator" }},   // RoleGuest
		&MockTool{NameFunc: func() string { return "weather" }},      // RoleUser
		&MockTool{NameFunc: func() string { return "github" }},       // RoleAdmin (ceiling)
		&MockTool{NameFunc: func() string { return "coding_agent" }}, // RoleAdmin
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	ctx := context.Background()

	nonAdmin := &domain.User{ID: "user1", Role: domain.RoleUser}
	listed, err := svc.ListTools(ctx, nonAdmin)
	if err != nil {
		t.Fatalf("ListTools returned unexpected error: %v", err)
	}
	names := toolNames(listed)
	for _, forbidden := range []string{"github", "coding_agent"} {
		if names[forbidden] {
			t.Errorf("expected non-admin ListTools to exclude %q, got %v", forbidden, names)
		}
	}
	if !names["calculator"] || !names["weather"] {
		t.Errorf("expected non-admin ListTools to include calculator/weather, got %v", names)
	}

	admin := &domain.User{ID: "admin1", Role: domain.RoleAdmin}
	listedAdmin, err := svc.ListTools(ctx, admin)
	if err != nil {
		t.Fatalf("ListTools returned unexpected error: %v", err)
	}
	adminNames := toolNames(listedAdmin)
	for _, expected := range []string{"calculator", "weather", "github", "coding_agent"} {
		if !adminNames[expected] {
			t.Errorf("expected admin ListTools to include %q, got %v", expected, adminNames)
		}
	}
}

// TestListTools_NilUserFailsClosedToGuest verifies a nil user is treated as
// the lowest-privilege RoleGuest identity, not as unrestricted access.
func TestListTools_NilUserFailsClosedToGuest(t *testing.T) {
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "calculator" }}, // RoleGuest
		&MockTool{NameFunc: func() string { return "weather" }},    // RoleUser
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	listed, err := svc.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned unexpected error: %v", err)
	}
	names := toolNames(listed)
	if !names["calculator"] {
		t.Errorf("expected nil user (guest) to still see guest-level tools, got %v", names)
	}
	if names["weather"] {
		t.Errorf("expected nil user (guest) to be denied user-level tools, got %v", names)
	}
}

func toolNames(tools []domain.Tool) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, t := range tools {
		names[t.Name()] = true
	}
	return names
}

// TestListTools_RoleFiltering verifies ListTools filters the registry's tools
// by the caller's role (FR-012), using the same permission rule ExecuteWithUser
// enforces — a tool a role can't execute must not be listed as available.
func TestListTools_RoleFiltering(t *testing.T) {
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "calculator" }}, // RoleGuest
		&MockTool{NameFunc: func() string { return "weather" }},    // RoleUser
		&MockTool{NameFunc: func() string { return "admin.user" }}, // RoleAdmin
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	ctx := context.Background()

	guestUser := &domain.User{ID: "guest1", Role: domain.RoleGuest}
	guestTools, err := svc.ListTools(ctx, guestUser)
	if err != nil {
		t.Fatalf("ListTools for guest returned an unexpected error: %v", err)
	}
	if len(guestTools) != 1 || guestTools[0].Name() != "calculator" {
		t.Errorf("Expected guest to see only [calculator], got: %+v", guestTools)
	}

	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}
	adminTools, err := svc.ListTools(ctx, adminUser)
	if err != nil {
		t.Fatalf("ListTools for admin returned an unexpected error: %v", err)
	}
	if len(adminTools) != len(mockTools) {
		t.Errorf("Expected admin to see all %d tools, got %d: %+v", len(mockTools), len(adminTools), adminTools)
	}
}

// TestListTools_AllowedToolsWhitelist verifies a user's AllowedTools whitelist
// further restricts ListTools even when their role would otherwise permit a tool.
func TestListTools_AllowedToolsWhitelist(t *testing.T) {
	mockTools := []domain.Tool{
		&MockTool{NameFunc: func() string { return "calculator" }},
		&MockTool{NameFunc: func() string { return "datetime" }},
	}
	mockRegistry := &MockToolRegistry{
		ListFunc: func() []domain.Tool { return mockTools },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	restrictedUser := &domain.User{
		ID:           "guest1",
		Role:         domain.RoleGuest,
		AllowedTools: []string{"calculator"},
	}

	ctx := context.Background()
	listedTools, err := svc.ListTools(ctx, restrictedUser)
	if err != nil {
		t.Fatalf("ListTools returned an unexpected error: %v", err)
	}
	if len(listedTools) != 1 || listedTools[0].Name() != "calculator" {
		t.Errorf("Expected whitelist to restrict listing to [calculator], got: %+v", listedTools)
	}
}

// RBAC Permission Tests

func TestExecuteWithUser_AdminCanExecuteAllSkills(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "admin.user" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "admin command executed"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	adminUser := &domain.User{
		ID:   "admin1",
		Role: domain.RoleAdmin,
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, adminUser, "conv1", "admin.user", nil)

	if err != nil {
		t.Errorf("Admin should be able to execute admin skills, got error: %v", err)
	}
	if result == nil || result.Output != "admin command executed" {
		t.Errorf("Expected admin command result, got: %v", result)
	}
}

func TestExecuteWithUser_UserCannotExecuteAdminSkills(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "admin.user" },
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	auditEvents := make(chan domain.AuditEvent, 1)
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	regularUser := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	_, err := svc.ExecuteWithUser(ctx, regularUser, "conv1", "admin.user", nil)

	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("Expected ErrInsufficientPermissions, got: %v", err)
	}

	// Verify audit event for permission denial
	select {
	case event := <-auditEvents:
		if event.Action != "tool_execution_denied" {
			t.Errorf("Expected 'tool_execution_denied' action, got %s", event.Action)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for audit event")
	}
}

func TestExecuteWithUser_UserCanExecuteUserSkills(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "weather" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "sunny"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	regularUser := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, regularUser, "conv1", "weather", nil)

	if err != nil {
		t.Errorf("User should be able to execute user-level skills, got error: %v", err)
	}
	if result == nil || result.Output != "sunny" {
		t.Errorf("Expected weather result, got: %v", result)
	}
}

func TestExecuteWithUser_GuestCanOnlyExecuteGuestSkills(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "calculator" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "5"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	guestUser := &domain.User{
		ID:   "guest1",
		Role: domain.RoleGuest,
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, guestUser, "conv1", "calculator", nil)

	if err != nil {
		t.Errorf("Guest should be able to execute guest-level skills, got error: %v", err)
	}
	if result == nil || result.Output != "5" {
		t.Errorf("Expected calculator result, got: %v", result)
	}
}

func TestExecuteWithUser_GuestCannotExecuteUserSkills(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "weather" },
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	guestUser := &domain.User{
		ID:   "guest1",
		Role: domain.RoleGuest,
	}

	ctx := context.Background()
	_, err := svc.ExecuteWithUser(ctx, guestUser, "conv1", "weather", nil)

	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("Expected ErrInsufficientPermissions, got: %v", err)
	}
}

func TestExecuteWithUser_AllowedSkillsWhitelist(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "calculator" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "5"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// User with AllowedTools whitelist - only calculator and datetime allowed
	restrictedUser := &domain.User{
		ID:           "user1",
		Role:         domain.RoleUser,
		AllowedTools: []string{"calculator", "datetime"},
	}

	ctx := context.Background()

	// Should be able to execute calculator (in whitelist)
	result, err := svc.ExecuteWithUser(ctx, restrictedUser, "conv1", "calculator", nil)
	if err != nil {
		t.Errorf("Should be able to execute whitelisted skill, got error: %v", err)
	}
	if result == nil {
		t.Error("Expected result for whitelisted skill")
	}
}

func TestExecuteWithUser_AllowedSkillsBlocksNonWhitelisted(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "weather" },
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// User with AllowedTools whitelist - only calculator and datetime allowed
	restrictedUser := &domain.User{
		ID:           "user1",
		Role:         domain.RoleUser,
		AllowedTools: []string{"calculator", "datetime"},
	}

	ctx := context.Background()

	// Should NOT be able to execute weather (not in whitelist)
	_, err := svc.ExecuteWithUser(ctx, restrictedUser, "conv1", "weather", nil)
	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("Expected ErrInsufficientPermissions for non-whitelisted skill, got: %v", err)
	}
}

func TestExecuteWithUser_EmptyAllowedSkillsAllowsAllForRole(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "weather" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "sunny"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// User with empty AllowedTools - should allow all skills for their role
	unrestrictedUser := &domain.User{
		ID:           "user1",
		Role:         domain.RoleUser,
		AllowedTools: []string{}, // Empty = all allowed
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, unrestrictedUser, "conv1", "weather", nil)

	if err != nil {
		t.Errorf("Empty AllowedTools should allow all role skills, got error: %v", err)
	}
	if result == nil || result.Output != "sunny" {
		t.Errorf("Expected weather result, got: %v", result)
	}
}

// RulesEnforcer Integration Tests

// MockRulesEnforcer implements a mock for persona RulesEnforcer.
// Note: EnforcerInput and EnforcerOutput are defined in the main package (service.go)
type MockRulesEnforcer struct {
	EnforceFunc func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error)
}

func (m *MockRulesEnforcer) Enforce(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
	if m.EnforceFunc != nil {
		return m.EnforceFunc(ctx, input)
	}
	return &EnforcerOutput{Allowed: true}, nil
}

// stubConfirmationStore is a minimal in-memory security.ConfirmationStore
// used by P5.3 tests. It is not concurrency-hardened (unlike
// internal/infrastructure/security.FileConfirmationStore, which has its own
// dedicated concurrency test suite) — it exists purely to exercise
// tool.Service's ExecuteWithUser/Execute confirmation-gate wiring.
type stubConfirmationStore struct {
	byID      map[string]security.ConfirmationRequest
	openByKey map[string]string
	nextID    int
}

func newStubConfirmationStore() *stubConfirmationStore {
	return &stubConfirmationStore{
		byID:      make(map[string]security.ConfirmationRequest),
		openByKey: make(map[string]string),
	}
}

func (s *stubConfirmationStore) key(userID, conversationID string) string {
	return userID + "\x00" + conversationID
}

func (s *stubConfirmationStore) Create(ctx context.Context, req security.ConfirmationRequest) (string, error) {
	key := s.key(req.UserID, req.ConversationID)
	if _, open := s.openByKey[key]; open {
		return "", security.ErrConfirmationAlreadyOpen
	}
	s.nextID++
	id := fmt.Sprintf("conf-%d", s.nextID)
	req.ID = id
	req.Status = security.ConfirmationStatusPending
	req.CreatedAt = time.Now()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = req.CreatedAt.Add(5 * time.Minute)
	}
	s.byID[id] = req
	s.openByKey[key] = id
	return id, nil
}

func (s *stubConfirmationStore) Resolve(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
	req, ok := s.byID[id]
	if !ok {
		return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
	}
	if req.Status != security.ConfirmationStatusPending {
		return security.ConfirmationRequest{}, security.ErrConfirmationAlreadyResolved
	}
	if approved {
		req.Status = security.ConfirmationStatusApproved
	} else {
		req.Status = security.ConfirmationStatusDenied
	}
	s.byID[id] = req
	delete(s.openByKey, s.key(req.UserID, req.ConversationID))
	return req, nil
}

func (s *stubConfirmationStore) Get(ctx context.Context, id string) (security.ConfirmationRequest, error) {
	req, ok := s.byID[id]
	if !ok {
		return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
	}
	return req, nil
}

func (s *stubConfirmationStore) GetOpenByKey(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
	id, ok := s.openByKey[s.key(userID, conversationID)]
	if !ok {
		return security.ConfirmationRequest{}, false, nil
	}
	return s.byID[id], true, nil
}

func (s *stubConfirmationStore) ExpireStale(ctx context.Context) error {
	now := time.Now()
	for id, req := range s.byID {
		if req.Status == security.ConfirmationStatusPending && now.After(req.ExpiresAt) {
			req.Status = security.ConfirmationStatusExpired
			s.byID[id] = req
			delete(s.openByKey, s.key(req.UserID, req.ConversationID))
		}
	}
	return nil
}

func (s *stubConfirmationStore) ListPending(ctx context.Context) ([]security.ConfirmationRequest, error) {
	pending := make([]security.ConfirmationRequest, 0, len(s.byID))
	for _, req := range s.byID {
		if req.Status == security.ConfirmationStatusPending {
			pending = append(pending, req)
		}
	}
	return pending, nil
}

func TestExecuteWithUser_RulesEnforcerBlocksTool(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "filesystem.delete" },
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	auditEvents := make(chan domain.AuditEvent, 1)
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}

	// RulesEnforcer that blocks filesystem.delete
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			if input.Tool == "filesystem.delete" {
				return &EnforcerOutput{
					Allowed: false,
					Reason:  "tool \"filesystem.delete\" is blocked by rules",
				}, nil
			}
			return &EnforcerOutput{Allowed: true}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)

	user := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	_, err := svc.ExecuteWithUser(ctx, user, "conv1", "filesystem.delete", nil)

	// Should be blocked by RulesEnforcer
	if err == nil {
		t.Fatal("Expected error when tool is blocked by rules, got nil")
	}

	// Verify audit event for rules enforcement denial
	select {
	case event := <-auditEvents:
		if event.Action != "tool_rules_denied" {
			t.Errorf("Expected 'tool_rules_denied' action, got %s", event.Action)
		}
		if event.Outcome != "denied" {
			t.Errorf("Expected 'denied' outcome, got %s", event.Outcome)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for audit event")
	}
}

func TestExecuteWithUser_RulesEnforcerAllowsTool(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "calculator" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "42"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}

	// RulesEnforcer that allows all tools
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			return &EnforcerOutput{Allowed: true}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)

	user := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, user, "conv1", "calculator", nil)

	if err != nil {
		t.Errorf("Tool should be allowed by rules, got error: %v", err)
	}
	if result == nil || result.Output != "42" {
		t.Errorf("Expected calculator result, got: %v", result)
	}
}

// TestExecuteWithUser_RulesEnforcerRequiresConfirmation_NoStoreConfigured
// covers Part C's fail-closed behavior (specs/260802-improve-nuimanbot-security,
// §8.3): when RulesEnforcer flags an action as requiring confirmation but no
// ConfirmationStore has been configured via SetConfirmationStore, the call
// must still be denied outright — a missing store is a configuration/infra
// fault, not license to execute unconfirmed.
func TestExecuteWithUser_RulesEnforcerRequiresConfirmation_NoStoreConfigured(t *testing.T) {
	mockTool := &MockTool{
		NameFunc: func() string { return "external.api.call" },
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	auditEvents := make(chan domain.AuditEvent, 1)
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditEvents <- *event
			return nil
		},
	}

	// RulesEnforcer that requires confirmation
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			if input.Tool == "external.api.call" {
				return &EnforcerOutput{
					Allowed:              true,
					RequiresConfirmation: true,
					Reason:               "tool \"external.api.call\" requires user confirmation",
				}, nil
			}
			return &EnforcerOutput{Allowed: true}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)
	// Deliberately NOT calling SetConfirmationStore.

	user := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	_, err := svc.ExecuteWithUser(ctx, user, "conv1", "external.api.call", nil)

	if err == nil {
		t.Fatal("Expected error when tool requires confirmation but no store is configured, got nil")
	}

	// Verify audit event for confirmation requirement
	select {
	case event := <-auditEvents:
		if event.Action != "tool_confirmation_required" {
			t.Errorf("Expected 'tool_confirmation_required' action, got %s", event.Action)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for audit event")
	}
}

// TestExecuteWithUser_RulesEnforcerRequiresConfirmation_WithStore_ReturnsPending
// is P5.3's core acceptance criterion: with a ConfirmationStore configured, a
// flagged action returns a StatusPendingConfirmation ExecutionResult with a
// populated ConfirmationID/Summary — not an error, and the underlying tool is
// never executed.
func TestExecuteWithUser_RulesEnforcerRequiresConfirmation_WithStore_ReturnsPending(t *testing.T) {
	executed := false
	mockTool := &MockTool{
		NameFunc: func() string { return "external.api.call" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			executed = true
			return &domain.ExecutionResult{Output: "should not run"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			return &EnforcerOutput{
				Allowed:              true,
				RequiresConfirmation: true,
				Reason:               "tool \"external.api.call\" requires user confirmation",
			}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)
	svc.SetConfirmationStore(newStubConfirmationStore())

	user := &domain.User{ID: "user1", Role: domain.RoleUser}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, user, "conv1", "external.api.call", map[string]any{"foo": "bar"})
	if err != nil {
		t.Fatalf("expected no error when a confirmation store is configured, got %v", err)
	}
	if result.Status != domain.StatusPendingConfirmation {
		t.Errorf("expected StatusPendingConfirmation, got %v", result.Status)
	}
	if result.ConfirmationID == "" {
		t.Error("expected a populated ConfirmationID")
	}
	if result.Summary == "" {
		t.Error("expected a populated Summary")
	}
	if executed {
		t.Error("expected the underlying tool NOT to execute while confirmation is pending")
	}
}

// TestExecuteWithUser_ConfirmationAlreadyOpen_FailsClosed covers FR-014: a
// second side-effecting call for a conversation with an already-open
// confirmation is denied (not silently executed, not silently queued).
func TestExecuteWithUser_ConfirmationAlreadyOpen_FailsClosed(t *testing.T) {
	mockTool := &MockTool{NameFunc: func() string { return "github" }}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			return &EnforcerOutput{Allowed: true, RequiresConfirmation: true, Reason: "requires confirmation"}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)
	svc.SetConfirmationStore(newStubConfirmationStore())

	user := &domain.User{ID: "user1", Role: domain.RoleAdmin}
	ctx := context.Background()

	first, err := svc.ExecuteWithUser(ctx, user, "conv1", "github", map[string]any{"action": "pr_merge"})
	if err != nil || first.Status != domain.StatusPendingConfirmation {
		t.Fatalf("expected first call to return pending confirmation, got result=%v err=%v", first, err)
	}

	_, err = svc.ExecuteWithUser(ctx, user, "conv1", "github", map[string]any{"action": "issue_close"})
	if err == nil {
		t.Fatal("expected the second call to fail closed while a confirmation is already open")
	}
}

// TestExecuteWithUser_ConfigDefaultRequiredActions_UnionsWithRulesEnforcer
// covers P5.6/FR-012: even when RulesEnforcer says no confirmation is
// required, a security.confirmation.default_required_actions match still
// triggers the confirmation gate.
func TestExecuteWithUser_ConfigDefaultRequiredActions_UnionsWithRulesEnforcer(t *testing.T) {
	mockTool := &MockTool{NameFunc: func() string { return "github" }}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}
	// RulesEnforcer explicitly says this tool does NOT require confirmation;
	// the config-level default list must still gate it (union, not override).
	mockEnforcer := &MockRulesEnforcer{
		EnforceFunc: func(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
			return &EnforcerOutput{Allowed: true, RequiresConfirmation: false}, nil
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	svc.SetRulesEnforcer(mockEnforcer)
	svc.SetConfirmationStore(newStubConfirmationStore())
	svc.SetConfirmationConfig(config.ConfirmationConfig{
		DefaultRequiredActions: []string{"github.pr_merge"},
	})

	user := &domain.User{ID: "user1", Role: domain.RoleAdmin}
	ctx := context.Background()

	result, err := svc.ExecuteWithUser(ctx, user, "conv1", "github", map[string]any{"action": "pr_merge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.StatusPendingConfirmation {
		t.Errorf("expected StatusPendingConfirmation from the config-level default list, got %v", result.Status)
	}
}

func TestExecuteWithUser_NoRulesEnforcerSet(t *testing.T) {
	// Test that tool execution works normally when RulesEnforcer is not set
	mockTool := &MockTool{
		NameFunc: func() string { return "calculator" },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "42"}, nil
		},
	}
	mockRegistry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)
	// Note: NOT calling SetRulesEnforcer - should work without it

	user := &domain.User{
		ID:   "user1",
		Role: domain.RoleUser,
	}

	ctx := context.Background()
	result, err := svc.ExecuteWithUser(ctx, user, "conv1", "calculator", nil)

	if err != nil {
		t.Errorf("Tool execution should work without RulesEnforcer, got error: %v", err)
	}
	if result == nil || result.Output != "42" {
		t.Errorf("Expected calculator result, got: %v", result)
	}
}
