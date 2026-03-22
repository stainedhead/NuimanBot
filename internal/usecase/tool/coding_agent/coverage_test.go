package coding_agent

import (
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/executor"

	"github.com/stretchr/testify/assert"
)

// TestGetToolCommand covers all branches
func TestGetToolCommand(t *testing.T) {
	t.Parallel()
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	tests := []struct {
		tool string
		want string
	}{
		{ToolCodex, "codex"},
		{ToolClaudeCode, "claude-code"},
		{ToolOpenCode, "opencode"},
		{ToolGemini, "gemini"},
		{ToolCopilot, "copilot"},
		{"unknown-tool", "unknown-tool"}, // default case
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.tool, func(t *testing.T) {
			t.Parallel()
			got := skill.getToolCommand(tt.tool)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBuildToolArgs covers all branches
func TestBuildToolArgs(t *testing.T) {
	t.Parallel()
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	t.Run("claude_code_auto_mode", func(t *testing.T) {
		t.Parallel()
		args := skill.buildToolArgs(ToolClaudeCode, "fix bug", ModeAuto)
		assert.Contains(t, args, "fix bug")
		assert.Contains(t, args, "--auto-approve")
	})

	t.Run("claude_code_interactive_mode", func(t *testing.T) {
		t.Parallel()
		args := skill.buildToolArgs(ToolClaudeCode, "fix bug", ModeInteractive)
		assert.Contains(t, args, "fix bug")
		assert.NotContains(t, args, "--auto-approve")
	})

	t.Run("codex_auto_mode", func(t *testing.T) {
		t.Parallel()
		args := skill.buildToolArgs(ToolCodex, "generate code", ModeAuto)
		assert.Contains(t, args, "--task")
		assert.Contains(t, args, "--non-interactive")
	})

	t.Run("codex_interactive_mode", func(t *testing.T) {
		t.Parallel()
		args := skill.buildToolArgs(ToolCodex, "generate code", ModeInteractive)
		assert.Contains(t, args, "--task")
		assert.NotContains(t, args, "--non-interactive")
	})

	t.Run("generic_tool", func(t *testing.T) {
		t.Parallel()
		args := skill.buildToolArgs(ToolGemini, "analyze code", ModeInteractive)
		assert.Contains(t, args, "analyze code")
	})
}

// TestGetTimeout covers all branches
func TestGetTimeout(t *testing.T) {
	t.Parallel()
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	t.Run("default_timeout", func(t *testing.T) {
		t.Parallel()
		timeout := skill.getTimeout(map[string]any{})
		assert.Equal(t, defaultTimeout, timeout)
	})

	t.Run("int_timeout", func(t *testing.T) {
		t.Parallel()
		timeout := skill.getTimeout(map[string]any{"timeout": 120})
		assert.Equal(t, 120*time.Second, timeout)
	})

	t.Run("float64_timeout", func(t *testing.T) {
		t.Parallel()
		timeout := skill.getTimeout(map[string]any{"timeout": float64(60)})
		assert.Equal(t, 60*time.Second, timeout)
	})
}

// TestFormatOutput covers the formatOutput function
func TestFormatOutput(t *testing.T) {
	t.Parallel()
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	execResult := &executor.ExecutionResult{
		Stdout:   "Generated code output",
		ExitCode: 0,
	}

	output := skill.formatOutput(execResult, 1.5, ModeInteractive)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "completed")
}
