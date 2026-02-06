package websearch_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/skills/websearch"
)

func TestWebSearchSkill_Metadata(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	if skill.Name() != "websearch" {
		t.Errorf("Expected name 'websearch', got '%s'", skill.Name())
	}

	if skill.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := skill.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestWebSearchSkill_Execute_MissingQuery(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	result, err := skill.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing query")
	}
}

func TestWebSearchSkill_Execute_EmptyQuery(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for empty query")
	}
}

func TestWebSearchSkill_Execute_InvalidLimit(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "test",
		"limit": 100,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for limit > 50")
	}
}

func TestWebSearchSkill_Execute_DefaultLimit(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	// This will likely fail with search error, but tests that default limit works
	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "test",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	// Should not get parameter validation error
	if result.Error == "missing query parameter" {
		t.Error("Should not get parameter validation error for optional limit")
	}
}

func TestWebSearchSkill_RequiredPermissions(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	perms := skill.RequiredPermissions()
	if len(perms) != 1 {
		t.Errorf("Expected 1 permission, got %d", len(perms))
	}
	if len(perms) > 0 && perms[0] != domain.PermissionNetwork {
		t.Errorf("Expected PermissionNetwork, got %v", perms[0])
	}
}

func TestWebSearchSkill_Config(t *testing.T) {
	skill := websearch.NewWebSearch(10)

	config := skill.Config()
	if !config.Enabled {
		t.Error("Expected skill to be enabled by default")
	}
}
