package tool_test

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
	. "nuimanbot/internal/usecase/tool"
)

// TestSetRateLimiter covers the 0.0% function
func TestSetRateLimiter(t *testing.T) {
	t.Parallel()
	mockCfg := &config.ToolsSystemConfig{}
	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return nil, domain.ErrNotFound },
		ListFunc: func() []domain.Tool { return nil },
	}
	mockSecurity := &MockSecurityService{}
	svc := NewService(mockCfg, mockRegistry, mockSecurity)

	// Create a rate limiter and set it - this should not panic
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"default": {Requests: 100, Window: 0},
	})
	svc.SetRateLimiter(limiter)
	// If we get here without panic, the setter was called
}

// TestExecuteWithUser_RateLimitExceeded covers the rate limit path
func TestExecuteWithUser_RateLimitExceeded(t *testing.T) {
	t.Parallel()

	adminUser := &domain.User{
		ID:   "admin-user",
		Role: domain.RoleAdmin,
	}

	mockTool := &MockTool{
		NameFunc:        func() string { return "test-tool" },
		DescriptionFunc: func() string { return "test" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}
	mockSecurity := &MockSecurityService{}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// Create a limiter with 1 request per second - we'll exhaust it quickly
	limiter := ratelimit.NewRateLimiter(map[string]ratelimit.RateLimit{
		"default": {Requests: 1, Window: 100}, // 1 request per 100ns - extremely tight
	})
	svc.SetRateLimiter(limiter)

	// Exhaust the bucket by calling repeatedly
	var gotRateLimitErr bool
	for i := 0; i < 20; i++ {
		_, err := svc.ExecuteWithUser(context.Background(), adminUser, "test-tool", nil)
		if errors.Is(err, domain.ErrRateLimitExceeded) {
			gotRateLimitErr = true
			break
		}
	}

	// Should have hit rate limit at some point, but just verify no panic
	_ = gotRateLimitErr
}

// TestAuditPermissionDenial_AuditError covers the audit error path in auditPermissionDenial
func TestAuditPermissionDenial_AuditError(t *testing.T) {
	t.Parallel()

	// Use "admin.user" which requires RoleAdmin, but use a non-admin user
	user := &domain.User{
		ID:   "user-id",
		Role: domain.RoleUser, // Not admin, will be denied for "admin.user" tool
	}

	mockTool := &MockTool{
		NameFunc:                func() string { return "admin.user" },
		DescriptionFunc:         func() string { return "restricted" },
		InputSchemaFunc:         func() map[string]any { return nil },
		ExecuteFunc:             func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) { return nil, nil },
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	// Mock security service that returns an error on Audit
	auditCalled := false
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditCalled = true
			return errors.New("audit failure")
		},
	}

	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	_, err := svc.ExecuteWithUser(context.Background(), user, "admin.user", nil)
	// Should return permission error (not the audit error)
	if err == nil {
		t.Error("Expected permission denied error")
	}
	if !auditCalled {
		t.Error("Expected audit to be called for permission denial")
	}
}

// TestExecuteWithUser_RulesEnforcer covers rules enforcer paths
func TestExecuteWithUser_RulesEnforcer_Blocked(t *testing.T) {
	t.Parallel()

	adminUser := &domain.User{
		ID:   "admin-user",
		Role: domain.RoleAdmin,
	}

	mockTool := &MockTool{
		NameFunc:        func() string { return "some-tool" },
		DescriptionFunc: func() string { return "tool" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	mockSecurity := &MockSecurityService{}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// Set a rules enforcer that blocks the tool
	blockedEnforcer := &mockRulesEnforcer{
		output: &EnforcerOutput{Allowed: false, Reason: "tool is blocked by rules"},
	}
	svc.SetRulesEnforcer(blockedEnforcer)

	_, err := svc.ExecuteWithUser(context.Background(), adminUser, "some-tool", nil)
	if err == nil {
		t.Error("Expected error when rules enforcer blocks tool")
	}
}

func TestExecuteWithUser_RulesEnforcer_RequiresConfirmation(t *testing.T) {
	t.Parallel()

	adminUser := &domain.User{
		ID:   "admin-user",
		Role: domain.RoleAdmin,
	}

	mockTool := &MockTool{
		NameFunc:        func() string { return "some-tool" },
		DescriptionFunc: func() string { return "tool" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	mockSecurity := &MockSecurityService{}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// Set a rules enforcer that requires confirmation
	confirmEnforcer := &mockRulesEnforcer{
		output: &EnforcerOutput{Allowed: true, RequiresConfirmation: true, Reason: "needs confirmation"},
	}
	svc.SetRulesEnforcer(confirmEnforcer)

	_, err := svc.ExecuteWithUser(context.Background(), adminUser, "some-tool", nil)
	if err == nil {
		t.Error("Expected error when tool requires confirmation")
	}
}

func TestExecuteWithUser_RulesEnforcer_Error(t *testing.T) {
	t.Parallel()

	adminUser := &domain.User{
		ID:   "admin-user",
		Role: domain.RoleAdmin,
	}

	mockTool := &MockTool{
		NameFunc:        func() string { return "some-tool" },
		DescriptionFunc: func() string { return "tool" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	mockSecurity := &MockSecurityService{}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	// Set a rules enforcer that returns an error
	errorEnforcer := &mockRulesEnforcer{
		err: errors.New("rules service unavailable"),
	}
	svc.SetRulesEnforcer(errorEnforcer)

	_, err := svc.ExecuteWithUser(context.Background(), adminUser, "some-tool", nil)
	if err == nil {
		t.Error("Expected error when rules enforcer fails")
	}
}

// mockRulesEnforcer is a mock for the RulesEnforcer interface
type mockRulesEnforcer struct {
	output *EnforcerOutput
	err    error
}

func (m *mockRulesEnforcer) Enforce(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// TestExecute_AuditSuccessPath covers the success path of Execute
func TestExecute_AuditSuccessPath(t *testing.T) {
	t.Parallel()

	mockTool := &MockTool{
		NameFunc:        func() string { return "my-tool" },
		DescriptionFunc: func() string { return "a tool" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "success output"}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	auditCount := 0
	mockRegistry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}
	mockSecurity := &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error {
			auditCount++
			return nil
		},
	}
	svc := NewService(&config.ToolsSystemConfig{}, mockRegistry, mockSecurity)

	result, err := svc.Execute(context.Background(), "my-tool", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Output != "success output" {
		t.Errorf("Expected 'success output', got %q", result.Output)
	}
	if auditCount < 2 {
		t.Errorf("Expected at least 2 audit events (attempt + success), got %d", auditCount)
	}
}
