package persona

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

// BenchmarkRulesEnforcer_AllowedTool benchmarks enforcement for allowed tools (fast path).
func BenchmarkRulesEnforcer_AllowedTool(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, `---
blocked_tools:
  - dangerous_tool
requires_confirmation:
  - external_api
---
# Rules
Be safe.`)

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)
	input := EnforcerInput{
		UserID: "bench-user",
		Tool:   "calculator", // Not in any list - allowed
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enforcer.Enforce(ctx, input)
		if err != nil {
			b.Fatalf("Enforce failed: %v", err)
		}
	}
}

// BenchmarkRulesEnforcer_BlockedTool benchmarks enforcement for blocked tools.
func BenchmarkRulesEnforcer_BlockedTool(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, `---
blocked_tools:
  - dangerous_tool
  - filesystem_delete
  - external_api
requires_confirmation: []
---
# Rules
Be safe.`)

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)
	input := EnforcerInput{
		UserID: "bench-user",
		Tool:   "dangerous_tool", // Blocked
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enforcer.Enforce(ctx, input)
		if err != nil {
			b.Fatalf("Enforce failed: %v", err)
		}
	}
}

// BenchmarkRulesEnforcer_ConfirmationRequired benchmarks enforcement for confirmation-required tools.
func BenchmarkRulesEnforcer_ConfirmationRequired(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, `---
blocked_tools: []
requires_confirmation:
  - external_api
  - credential_use
  - destructive_action
---
# Rules
Be safe.`)

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)
	input := EnforcerInput{
		UserID: "bench-user",
		Tool:   "external_api", // Requires confirmation
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enforcer.Enforce(ctx, input)
		if err != nil {
			b.Fatalf("Enforce failed: %v", err)
		}
	}
}

// BenchmarkRulesEnforcer_NoRulesFile benchmarks enforcement when RULES.md doesn't exist (graceful degradation).
func BenchmarkRulesEnforcer_NoRulesFile(b *testing.B) {
	repo := newMockRepo() // Empty repo

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)
	input := EnforcerInput{
		UserID: "nonexistent-user",
		Tool:   "calculator",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enforcer.Enforce(ctx, input)
		if err != nil {
			b.Fatalf("Enforce failed: %v", err)
		}
	}
}

// BenchmarkRulesEnforcer_WithAdminPolicy benchmarks enforcement with admin policy merging.
func BenchmarkRulesEnforcer_WithAdminPolicy(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, `---
blocked_tools:
  - user_blocked_tool
requires_confirmation:
  - user_confirm_tool
---
# Rules
User rules.`)

	adminPolicy := &domain.RulesConfig{
		BlockedTools: []string{"admin_blocked_tool"},
		RequiresConfirmation: []string{"admin_confirm_tool"},
	}

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, adminPolicy)
	input := EnforcerInput{
		UserID: "bench-user",
		Tool:   "calculator", // Not blocked by either
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := enforcer.Enforce(ctx, input)
		if err != nil {
			b.Fatalf("Enforce failed: %v", err)
		}
	}
}

// BenchmarkRulesEnforcer_Parallel benchmarks concurrent enforcement requests.
func BenchmarkRulesEnforcer_Parallel(b *testing.B) {
	repo := newMockRepo()
	repo.addFile("bench-user", domain.PersonaFileRULES, `---
blocked_tools:
  - dangerous_tool
requires_confirmation:
  - external_api
---
# Rules
Be safe.`)

	parser := &mockFrontmatterParser{}
	enforcer := NewRulesEnforcer(repo, parser, nil)
	input := EnforcerInput{
		UserID: "bench-user",
		Tool:   "calculator",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, err := enforcer.Enforce(ctx, input)
			if err != nil {
				b.Fatalf("Enforce failed: %v", err)
			}
		}
	})
}
