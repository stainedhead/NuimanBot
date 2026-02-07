package e2e

import (
	"context"
	"io"
	"testing"

	"nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain"
	skillinfra "nuimanbot/internal/infrastructure/skill"
	skillusecase "nuimanbot/internal/usecase/skill"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSkillArgumentSubstitution verifies that skills properly substitute arguments
func TestSkillArgumentSubstitution(t *testing.T) {
	ctx := context.Background()

	// Set up skill infrastructure
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	// Initialize registry with shared skills
	roots := []domain.SkillRoot{
		{Path: "../data/skills/shared", Scope: domain.ScopeProject},
	}
	err := registry.Initialize(ctx, roots)
	require.NoError(t, err, "Failed to initialize registry")

	// Set up CLI skill command
	skillCmd := cli.NewSkillCommand(registry, renderer, io.Discard)

	tests := []struct {
		name           string
		skillName      string
		args           []string
		expectedInBody string
	}{
		{
			name:           "code-review with file path",
			skillName:      "code-review",
			args:           []string{"my/file.go"},
			expectedInBody: "Perform a comprehensive code review of the following: my/file.go",
		},
		{
			name:           "debugging with description",
			skillName:      "debugging",
			args:           []string{"null", "pointer", "error"},
			expectedInBody: "Help debug the following issue: null pointer error",
		},
		{
			name:           "testing with multiple args",
			skillName:      "testing",
			args:           []string{"user", "authentication", "service"},
			expectedInBody: "Help write comprehensive tests for: user authentication service",
		},
		{
			name:           "refactoring with single arg",
			skillName:      "refactoring",
			args:           []string{"OrderProcessor"},
			expectedInBody: "Suggest refactoring improvements for: OrderProcessor",
		},
		{
			name:           "api-docs with endpoint",
			skillName:      "api-docs",
			args:           []string{"GET", "/api/users"},
			expectedInBody: "Generate API documentation for: GET /api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute skill
			rendered, err := skillCmd.Execute(ctx, tt.skillName, tt.args)
			require.NoError(t, err, "Failed to execute skill")
			require.NotNil(t, rendered, "Rendered skill should not be nil")

			// Verify argument substitution
			assert.Contains(t, rendered.Prompt, tt.expectedInBody,
				"Skill body should contain substituted arguments")

			// Verify skill name
			assert.Equal(t, tt.skillName, rendered.SkillName, "Skill name should match")
		})
	}
}

// TestSkillWithNoArguments verifies skills work with no arguments (empty substitution)
func TestSkillWithNoArguments(t *testing.T) {
	ctx := context.Background()

	// Set up skill infrastructure
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	// Initialize registry with shared skills
	roots := []domain.SkillRoot{
		{Path: "../data/skills/shared", Scope: domain.ScopeProject},
	}
	err := registry.Initialize(ctx, roots)
	require.NoError(t, err, "Failed to initialize registry")

	// Set up CLI skill command
	skillCmd := cli.NewSkillCommand(registry, renderer, io.Discard)

	// Execute code-review with no arguments
	rendered, err := skillCmd.Execute(ctx, "code-review", []string{})
	require.NoError(t, err, "Failed to execute skill")
	require.NotNil(t, rendered, "Rendered skill should not be nil")

	// Should have empty string substitution
	assert.Contains(t, rendered.Prompt, "Perform a comprehensive code review of the following: \n",
		"Should substitute empty string when no arguments provided")
}

// TestSkillAllowedTools verifies that allowed-tools are properly parsed
func TestSkillAllowedTools(t *testing.T) {
	ctx := context.Background()

	// Set up skill infrastructure
	repo := skillinfra.NewFilesystemSkillRepository()
	registry := skillusecase.NewInMemorySkillRegistry(repo)
	renderer := skillusecase.NewDefaultSkillRenderer()

	// Initialize registry with shared skills
	roots := []domain.SkillRoot{
		{Path: "../data/skills/shared", Scope: domain.ScopeProject},
	}
	err := registry.Initialize(ctx, roots)
	require.NoError(t, err, "Failed to initialize registry")

	// Set up CLI skill command
	skillCmd := cli.NewSkillCommand(registry, renderer, io.Discard)

	tests := []struct {
		name          string
		skillName     string
		expectedTools []string
	}{
		{
			name:          "code-review has repo_search and github",
			skillName:     "code-review",
			expectedTools: []string{"repo_search", "github"},
		},
		{
			name:          "debugging has repo_search and github",
			skillName:     "debugging",
			expectedTools: []string{"repo_search", "github"},
		},
		{
			name:          "testing has repo_search and github",
			skillName:     "testing",
			expectedTools: []string{"repo_search", "github"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute skill
			rendered, err := skillCmd.Execute(ctx, tt.skillName, []string{"test"})
			require.NoError(t, err, "Failed to execute skill")
			require.NotNil(t, rendered, "Rendered skill should not be nil")

			// Verify allowed tools
			assert.Equal(t, tt.expectedTools, rendered.AllowedTools,
				"Allowed tools should match frontmatter")
		})
	}
}
