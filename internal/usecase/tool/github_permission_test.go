package tool_test

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	. "nuimanbot/internal/usecase/tool"
	"nuimanbot/internal/usecase/tool/github"
)

// newGitHubMockService builds a Service backed by a mock registry that
// serves a single tool named "github" (Execute always succeeds), so tests
// here exercise ExecuteWithUser's permission check in isolation without
// depending on the real GitHubSkill's gh-CLI execution path.
func newGitHubMockService(t *testing.T, cfg *config.ToolsSystemConfig) *Service {
	t.Helper()

	mockTool := &MockTool{
		NameFunc:        func() string { return "github" },
		DescriptionFunc: func() string { return "GitHub operations via gh CLI" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "ok"}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}

	registry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	if cfg == nil {
		cfg = &config.ToolsSystemConfig{}
	}

	return NewService(cfg, registry, &MockSecurityService{})
}

func TestExecuteWithUser_GitHubActionAware_ReadActionsAllowedForRoleUser(t *testing.T) {
	svc := newGitHubMockService(t, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	readActions := []string{
		github.ActionIssueList,
		github.ActionIssueView,
		github.ActionPRList,
		github.ActionPRView,
		github.ActionRepoView,
	}

	for _, action := range readActions {
		if _, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{"action": action}); err != nil {
			t.Errorf("action %q: expected RoleUser to be allowed, got error: %v", action, err)
		}
	}
}

func TestExecuteWithUser_GitHubActionAware_WriteActionsDeniedForRoleUser(t *testing.T) {
	svc := newGitHubMockService(t, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	writeActions := []string{
		github.ActionIssueCreate,
		github.ActionIssueComment,
		github.ActionIssueClose,
		github.ActionPRCreate,
		github.ActionPRReview,
		github.ActionPRMerge,
		github.ActionWorkflowRun,
	}

	for _, action := range writeActions {
		_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{"action": action})
		if !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Errorf("action %q: expected ErrInsufficientPermissions for RoleUser, got: %v", action, err)
		}
	}
}

func TestExecuteWithUser_GitHubActionAware_WriteAndReadActionsAllowedForRoleAdmin(t *testing.T) {
	svc := newGitHubMockService(t, nil)
	admin := &domain.User{ID: "a1", Role: domain.RoleAdmin}

	allActions := []string{
		github.ActionIssueList, github.ActionIssueView, github.ActionPRList,
		github.ActionPRView, github.ActionRepoView,
		github.ActionIssueCreate, github.ActionIssueComment, github.ActionIssueClose,
		github.ActionPRCreate, github.ActionPRReview, github.ActionPRMerge, github.ActionWorkflowRun,
	}

	for _, action := range allActions {
		if _, err := svc.ExecuteWithUser(context.Background(), admin, "conv1", "github", map[string]any{"action": action}); err != nil {
			t.Errorf("action %q: expected RoleAdmin to be allowed, got error: %v", action, err)
		}
	}
}

func TestExecuteWithUser_GitHubActionAware_UnrecognizedActionFailsClosedForRoleUser(t *testing.T) {
	svc := newGitHubMockService(t, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{"action": "totally_unknown"})
	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected fail-closed ErrInsufficientPermissions for an unrecognized action, got: %v", err)
	}
}

func TestExecuteWithUser_GitHubActionAware_MissingActionFailsClosedForRoleUser(t *testing.T) {
	svc := newGitHubMockService(t, nil)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{})
	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected fail-closed ErrInsufficientPermissions when params carries no action, got: %v", err)
	}
}

// TestExecuteWithUser_ToolsPermissionsConfigOverride_RestoresPermissiveGitHub
// covers P3.4: a `tools.permissions` config override applies uniformly to a
// tool, taking precedence over both the action-aware github split and the
// static admin-only ToolPermissions entry.
func TestExecuteWithUser_ToolsPermissionsConfigOverride_RestoresPermissiveGitHub(t *testing.T) {
	cfg := &config.ToolsSystemConfig{Permissions: map[string]string{"github": "user"}}
	svc := newGitHubMockService(t, cfg)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	// Without the override, pr_merge would be admin-only (see
	// TestExecuteWithUser_GitHubActionAware_WriteActionsDeniedForRoleUser).
	if _, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{"action": github.ActionPRMerge}); err != nil {
		t.Errorf("expected tools.permissions override to allow RoleUser pr_merge, got error: %v", err)
	}
}

// TestExecuteWithUser_ToolsPermissionsConfigOverride_TightensCalculator
// covers the inverse direction: an operator can also raise a tool's default
// (e.g. calculator, normally RoleGuest) via the same config path.
func TestExecuteWithUser_ToolsPermissionsConfigOverride_TightensCalculator(t *testing.T) {
	mockTool := &MockTool{
		NameFunc:        func() string { return "calculator" },
		DescriptionFunc: func() string { return "calculator" },
		InputSchemaFunc: func() map[string]any { return nil },
		ExecuteFunc: func(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
			return &domain.ExecutionResult{Output: "4"}, nil
		},
		RequiredPermissionsFunc: func() []domain.Permission { return nil },
		ConfigFunc:              func() domain.ToolConfig { return domain.ToolConfig{Enabled: true} },
	}
	registry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}
	cfg := &config.ToolsSystemConfig{Permissions: map[string]string{"calculator": "admin"}}
	svc := NewService(cfg, registry, &MockSecurityService{})

	guest := &domain.User{ID: "g1", Role: domain.RoleGuest}
	if _, err := svc.ExecuteWithUser(context.Background(), guest, "conv1", "calculator", nil); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected tools.permissions override to deny RoleGuest, got: %v", err)
	}

	admin := &domain.User{ID: "a1", Role: domain.RoleAdmin}
	if _, err := svc.ExecuteWithUser(context.Background(), admin, "conv1", "calculator", nil); err != nil {
		t.Errorf("expected RoleAdmin to still be allowed after override, got error: %v", err)
	}
}

// TestExecuteWithUser_ToolsPermissionsConfigOverride_UnrecognizedValueIgnored
// covers the fail-safe: an unparseable override value doesn't grant or deny
// access on its own — it falls through to the next precedence level (here,
// the static admin-only "github" entry, since the action isn't a recognized
// read action either).
func TestExecuteWithUser_ToolsPermissionsConfigOverride_UnrecognizedValueIgnored(t *testing.T) {
	cfg := &config.ToolsSystemConfig{Permissions: map[string]string{"github": "not-a-real-role"}}
	svc := newGitHubMockService(t, cfg)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "github", map[string]any{"action": github.ActionPRMerge})
	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected unrecognized override value to fall through to admin-only default, got: %v", err)
	}
}
