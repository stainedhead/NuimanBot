package persona

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// mockFrontmatterParser implements FrontmatterParser for testing.
type mockFrontmatterParser struct {
	config domain.RulesConfig
	body   string
	err    error
}

func (m *mockFrontmatterParser) ParseMarkdownWithFrontmatter(_ string) (domain.RulesConfig, string, error) {
	return m.config, m.body, m.err
}

// --- NewRulesEnforcer tests ---

func TestNewRulesEnforcer(t *testing.T) {
	repo := newMockRepo()
	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	if enforcer == nil {
		t.Fatal("NewRulesEnforcer() returned nil")
	}
}

// --- Enforce: blocked tools ---

func TestEnforce_ToolBlocked(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\nblocked_tools:\n  - shell_exec\n---\n")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools: []string{"shell_exec"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("Enforce() Allowed = true, want false for blocked tool")
	}
	if out.Reason == "" {
		t.Error("Enforce() Reason should explain why tool is blocked")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation should be false for blocked tool")
	}
}

func TestEnforce_ToolNotBlocked(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\nblocked_tools:\n  - shell_exec\n---\n")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools: []string{"shell_exec"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "read_file",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true for non-blocked tool")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation should be false")
	}
}

// --- Enforce: requires confirmation ---

func TestEnforce_ActionRequiresConfirmation(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\nrequires_confirmation:\n  - delete_file\n---\n")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			RequiresConfirmation: []string{"delete_file"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Action: "delete_file",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true (confirmation != block)")
	}
	if !out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation = false, want true")
	}
	if out.Reason == "" {
		t.Error("Enforce() Reason should explain confirmation requirement")
	}
}

func TestEnforce_ActionNoConfirmation(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\nrequires_confirmation:\n  - delete_file\n---\n")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			RequiresConfirmation: []string{"delete_file"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Action: "read_file",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation = true, want false")
	}
}

// --- Enforce: precedence ---

func TestEnforce_BlockedTakesPrecedence(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "content")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools:         []string{"shell_exec"},
			RequiresConfirmation: []string{"shell_exec"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
		Action: "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("Enforce() Allowed = true, want false (blocked takes precedence)")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation should be false when blocked")
	}
}

// --- Enforce: graceful degradation ---

func TestEnforce_NoRulesFile(t *testing.T) {
	repo := newMockRepo() // no files set
	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
		Action: "delete_file",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true when no RULES.md exists")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation should be false when no rules")
	}
}

func TestEnforce_EmptyRulesFile(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "")
	parser := &mockFrontmatterParser{config: domain.RulesConfig{}}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true for empty rules")
	}
}

func TestEnforce_NoFrontmatter(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "# Rules\n\nJust markdown, no frontmatter.")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{},
		body:   "# Rules\n\nJust markdown, no frontmatter.",
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true for rules without frontmatter")
	}
}

// --- Enforce: admin policy ---

func TestEnforce_AdminPolicyBlocksTool(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\n---\n# Empty rules")
	parser := &mockFrontmatterParser{config: domain.RulesConfig{}}

	adminPolicy := &domain.RulesConfig{
		BlockedTools: []string{"dangerous_tool"},
	}
	enforcer := NewRulesEnforcer(repo, parser, adminPolicy)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "dangerous_tool",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("Enforce() Allowed = true, want false (admin policy blocks tool)")
	}
	if out.Reason == "" {
		t.Error("Enforce() Reason should explain admin policy block")
	}
}

func TestEnforce_AdminPolicyRequiresConfirmation(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\n---\n# Empty rules")
	parser := &mockFrontmatterParser{config: domain.RulesConfig{}}

	adminPolicy := &domain.RulesConfig{
		RequiresConfirmation: []string{"admin_action"},
	}
	enforcer := NewRulesEnforcer(repo, parser, adminPolicy)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Action: "admin_action",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true (confirmation != block)")
	}
	if !out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation = false, want true (admin policy)")
	}
}

func TestEnforce_AdminPolicyOverridesUser(t *testing.T) {
	repo := newMockRepo()
	// User has no blocked tools, but admin blocks it
	repo.addFile("user1", domain.PersonaFileRULES, "---\n---\n# No user blocks")
	parser := &mockFrontmatterParser{config: domain.RulesConfig{}}

	adminPolicy := &domain.RulesConfig{
		BlockedTools: []string{"shell_exec"},
	}
	enforcer := NewRulesEnforcer(repo, parser, adminPolicy)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("Enforce() Allowed = true, want false (admin override)")
	}
}

// --- Enforce: error propagation ---

func TestEnforce_RepoErrorPropagates(t *testing.T) {
	repo := newMockRepo()
	repo.err = errors.New("database connection failed")
	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	_, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err == nil {
		t.Fatal("Enforce() expected error, got nil")
	}
}

func TestEnforce_ParserErrorPropagates(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "---\ninvalid: [yaml\n---\n")
	parser := &mockFrontmatterParser{
		err: errors.New("invalid YAML frontmatter"),
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	_, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err == nil {
		t.Fatal("Enforce() expected error for invalid YAML, got nil")
	}
}

// --- Enforce: edge cases ---

func TestEnforce_EmptyInput(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "content")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools:         []string{"shell_exec"},
			RequiresConfirmation: []string{"delete_file"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		// No Tool or Action specified
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("Enforce() Allowed = false, want true for empty tool/action")
	}
	if out.RequiresConfirmation {
		t.Error("Enforce() RequiresConfirmation = true, want false for empty input")
	}
}

func TestEnforce_EmptyUserID(t *testing.T) {
	repo := newMockRepo()
	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	_, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "",
		Tool:   "shell_exec",
	})
	if err == nil {
		t.Fatal("Enforce() expected error for empty UserID, got nil")
	}
}

func TestEnforce_CombinedUserAndAdminRules(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "content")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools:         []string{"user_blocked"},
			RequiresConfirmation: []string{"user_confirm"},
		},
	}
	adminPolicy := &domain.RulesConfig{
		BlockedTools:         []string{"admin_blocked"},
		RequiresConfirmation: []string{"admin_confirm"},
	}
	enforcer := NewRulesEnforcer(repo, parser, adminPolicy)

	// User-blocked tool
	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "user_blocked",
	})
	if err != nil {
		t.Fatalf("user_blocked: unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("user_blocked: Allowed = true, want false")
	}

	// Admin-blocked tool
	out, err = enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "admin_blocked",
	})
	if err != nil {
		t.Fatalf("admin_blocked: unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("admin_blocked: Allowed = true, want false")
	}

	// User-confirm action
	out, err = enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Action: "user_confirm",
	})
	if err != nil {
		t.Fatalf("user_confirm: unexpected error: %v", err)
	}
	if !out.RequiresConfirmation {
		t.Error("user_confirm: RequiresConfirmation = false, want true")
	}

	// Admin-confirm action
	out, err = enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Action: "admin_confirm",
	})
	if err != nil {
		t.Fatalf("admin_confirm: unexpected error: %v", err)
	}
	if !out.RequiresConfirmation {
		t.Error("admin_confirm: RequiresConfirmation = false, want true")
	}

	// Allowed tool
	out, err = enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "safe_tool",
	})
	if err != nil {
		t.Fatalf("safe_tool: unexpected error: %v", err)
	}
	if !out.Allowed {
		t.Error("safe_tool: Allowed = false, want true")
	}
}

func TestEnforce_NilAdminPolicy(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "content")
	parser := &mockFrontmatterParser{
		config: domain.RulesConfig{
			BlockedTools: []string{"shell_exec"},
		},
	}
	enforcer := NewRulesEnforcer(repo, parser, nil)

	out, err := enforcer.Enforce(context.Background(), EnforcerInput{
		UserID: "user1",
		Tool:   "shell_exec",
	})
	if err != nil {
		t.Fatalf("Enforce() unexpected error: %v", err)
	}
	if out.Allowed {
		t.Error("Enforce() Allowed = true, want false")
	}
}
