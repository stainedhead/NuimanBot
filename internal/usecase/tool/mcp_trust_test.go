package tool_test

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	. "nuimanbot/internal/usecase/tool"
)

// mcpTrustMockTool is a minimal domain.Tool + TrustClassifiedTool test double
// for exercising the "mcp:<server>:<tool>" namespace's dynamic trust-based
// RBAC (P6.3) and confirmation (P6.4) resolution, without depending on the
// real internal/adapter/mcp.MCPToolAdapter (which requires a live/faked MCP
// transport just to construct). name is expected to already carry the
// "mcp:<server>:<tool>" prefix real bridged tools use.
type mcpTrustMockTool struct {
	name  string
	trust string
}

func (m *mcpTrustMockTool) Name() string        { return m.name }
func (m *mcpTrustMockTool) Description() string { return "mock MCP tool" }
func (m *mcpTrustMockTool) InputSchema() map[string]any {
	return map[string]any{"type": "object"}
}
func (m *mcpTrustMockTool) Execute(_ context.Context, _ map[string]any) (*domain.ExecutionResult, error) {
	return &domain.ExecutionResult{Output: "ok"}, nil
}
func (m *mcpTrustMockTool) RequiredPermissions() []domain.Permission { return nil }
func (m *mcpTrustMockTool) Config() domain.ToolConfig                { return domain.ToolConfig{Enabled: true} }

// TrustLevel implements TrustClassifiedTool.
func (m *mcpTrustMockTool) TrustLevel() string { return m.trust }

// newMCPTrustMockService builds a Service backed by a mock registry serving a
// single MCP-namespaced tool with the given trust level.
func newMCPTrustMockService(t *testing.T, toolName, trust string) *Service {
	t.Helper()

	mockTool := &mcpTrustMockTool{name: toolName, trust: trust}
	registry := &MockToolRegistry{
		GetFunc:  func(name string) (domain.Tool, error) { return mockTool, nil },
		ListFunc: func() []domain.Tool { return []domain.Tool{mockTool} },
	}

	return NewService(&config.ToolsSystemConfig{}, registry, &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	})
}

// --- P6.3: RBAC ---

func TestExecuteWithUser_MCPTool_ReadOnlyTrust_AllowedForRoleUser(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:github-mcp:issue_list", TrustReadOnly)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:issue_list", nil)

	if err != nil {
		t.Errorf("expected read_only-trust MCP tool to be allowed for RoleUser, got error: %v", err)
	}
}

func TestExecuteWithUser_MCPTool_WriteTrust_DeniedForRoleUser(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:github-mcp:pr_merge", TrustWrite)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected write-trust MCP tool to be denied for RoleUser, got: %v", err)
	}
}

func TestExecuteWithUser_MCPTool_WriteTrust_AllowedForRoleAdmin(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:github-mcp:pr_merge", TrustWrite)
	svc.SetConfirmationStore(newStubConfirmationStore())
	user := &domain.User{ID: "u1", Role: domain.RoleAdmin}

	result, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if err != nil {
		t.Fatalf("expected write-trust MCP tool to be allowed for RoleAdmin, got error: %v", err)
	}
	// Write trust also means P6.4's confirmation gate kicks in — RoleAdmin
	// is permitted to REQUEST the action, but it still pauses pending
	// confirmation rather than executing immediately.
	if result.Status != domain.StatusPendingConfirmation {
		t.Errorf("expected StatusPendingConfirmation for a write-trust MCP tool, got %v", result.Status)
	}
}

func TestExecuteWithUser_MCPTool_UnknownTrust_DeniedForRoleUser(t *testing.T) {
	// Unknown trust (e.g. omitted from mcp.json) is treated identically to
	// "write" for RBAC purposes (FR-022's fail-closed default).
	svc := newMCPTrustMockService(t, "mcp:some-server:some-tool", TrustUnknown)
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:some-server:some-tool", nil)

	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected unknown-trust MCP tool to be denied for RoleUser, got: %v", err)
	}
}

func TestExecuteWithUser_MCPTool_UnrecognizedTrustString_TreatedAsUnknown_DeniedForRoleUser(t *testing.T) {
	// A tool whose TrustLevel() returns something other than the three
	// recognized values (defensive: should never happen given
	// infra.normalizeTrustLevel's fail-closed normalization, but the
	// permission layer must not silently trust an unrecognized string as
	// read-only).
	svc := newMCPTrustMockService(t, "mcp:some-server:some-tool", "not-a-real-trust-value")
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:some-server:some-tool", nil)

	if !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("expected an unrecognized trust value to fail closed (denied) for RoleUser, got: %v", err)
	}
}

func TestExecuteWithUser_MCPTool_ToolsPermissionsConfigOverride_TakesPrecedenceOverTrust(t *testing.T) {
	// The tools.permissions config override (Phase 3 / FR-018a) is generic —
	// it applies to any registered tool name, including the dynamic
	// "mcp:<server>:<tool>" namespace — and takes precedence over trust-based
	// resolution, same as it does over the static ToolPermissions map and
	// github's action-aware split.
	mockTool := &mcpTrustMockTool{name: "mcp:github-mcp:pr_merge", trust: TrustWrite}
	registry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	svc := NewService(&config.ToolsSystemConfig{
		Permissions: map[string]string{"mcp:github-mcp:pr_merge": "user"},
	}, registry, &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	})
	// The tool is still write-trust, so P6.4's confirmation gate still
	// applies independently of the RBAC override under test here — a store
	// must be configured or the call fails closed for that separate reason.
	svc.SetConfirmationStore(newStubConfirmationStore())
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if err != nil {
		t.Errorf("expected tools.permissions override to allow RoleUser past the RBAC check despite write trust, got error: %v", err)
	}
}

// --- P6.4: confirmation-required set ---

func TestExecuteWithUser_MCPTool_WriteTrust_TriggersConfirmation(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:github-mcp:pr_merge", TrustWrite)
	svc.SetConfirmationStore(newStubConfirmationStore())
	user := &domain.User{ID: "u1", Role: domain.RoleAdmin}

	result, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.StatusPendingConfirmation {
		t.Errorf("expected StatusPendingConfirmation, got %v", result.Status)
	}
	if result.ConfirmationID == "" {
		t.Error("expected a non-empty ConfirmationID")
	}
}

func TestExecuteWithUser_MCPTool_UnknownTrust_TriggersConfirmation(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:some-server:some-tool", TrustUnknown)
	svc.SetConfirmationStore(newStubConfirmationStore())
	user := &domain.User{ID: "u1", Role: domain.RoleAdmin}

	result, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:some-server:some-tool", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.StatusPendingConfirmation {
		t.Errorf("expected unknown-trust MCP tool to require confirmation, got %v", result.Status)
	}
}

func TestExecuteWithUser_MCPTool_ReadOnlyTrust_DoesNotTriggerConfirmation(t *testing.T) {
	svc := newMCPTrustMockService(t, "mcp:github-mcp:issue_list", TrustReadOnly)
	svc.SetConfirmationStore(newStubConfirmationStore())
	user := &domain.User{ID: "u1", Role: domain.RoleUser}

	result, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:issue_list", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status == domain.StatusPendingConfirmation {
		t.Error("expected read_only-trust MCP tool to execute directly, not require confirmation")
	}
	if result.Output != "ok" {
		t.Errorf("expected the tool to actually execute and return its output, got %q", result.Output)
	}
}

func TestExecuteWithUser_MCPTool_WriteTrust_NoStoreConfigured_FailsClosed(t *testing.T) {
	// Mirrors TestExecuteWithUser_RulesEnforcerRequiresConfirmation_NoStoreConfigured:
	// a write-trust MCP tool that would require confirmation, but with no
	// ConfirmationStore configured, must be denied outright rather than
	// executed unconfirmed.
	svc := newMCPTrustMockService(t, "mcp:github-mcp:pr_merge", TrustWrite)
	user := &domain.User{ID: "u1", Role: domain.RoleAdmin}

	_, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if err == nil {
		t.Fatal("expected an error (fail closed) when no ConfirmationStore is configured")
	}
}

func TestExecuteWithUser_MCPTool_WriteTrust_ConfirmationDisabled_ExecutesDirectly(t *testing.T) {
	// When security.confirmation.enabled is explicitly false, the automatic
	// MCP-trust-driven confirmation requirement is inert too (consistent with
	// config.ConfirmationConfig.RequiresConfirmationByDefault's own
	// IsEnabled() gate) — it is not a separate, always-on gate that ignores
	// the subsystem's enabled switch.
	mockTool := &mcpTrustMockTool{name: "mcp:github-mcp:pr_merge", trust: TrustWrite}
	registry := &MockToolRegistry{
		GetFunc: func(name string) (domain.Tool, error) { return mockTool, nil },
	}
	svc := NewService(&config.ToolsSystemConfig{}, registry, &MockSecurityService{
		AuditFunc: func(ctx context.Context, event *domain.AuditEvent) error { return nil },
	})
	disabled := false
	svc.SetConfirmationConfig(config.ConfirmationConfig{Enabled: &disabled})
	user := &domain.User{ID: "u1", Role: domain.RoleAdmin}

	result, err := svc.ExecuteWithUser(context.Background(), user, "conv1", "mcp:github-mcp:pr_merge", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status == domain.StatusPendingConfirmation {
		t.Error("expected confirmation gate to be inert when security.confirmation.enabled is false")
	}
}
