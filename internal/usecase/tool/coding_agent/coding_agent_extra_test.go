package coding_agent

import (
	"context"
	"fmt"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/executor"
	"nuimanbot/internal/usecase/tool/testutil"
)

func TestCodingAgentSkill_Config(t *testing.T) {
	cfg := domain.ToolConfig{Enabled: true}
	skill := NewCodingAgentSkill(cfg, nil, nil)
	got := skill.Config()
	if !got.Enabled {
		t.Error("Config() should return the stored config")
	}
}

func TestCodingAgentSkill_GetToolCommand(t *testing.T) {
	skill := &CodingAgentSkill{}
	tests := []struct {
		tool string
		want string
	}{
		{ToolCodex, "codex"},
		{ToolClaudeCode, "claude-code"},
		{ToolOpenCode, "opencode"},
		{ToolGemini, "gemini"},
		{ToolCopilot, "copilot"},
		{"custom-tool", "custom-tool"},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := skill.getToolCommand(tt.tool)
			if got != tt.want {
				t.Errorf("getToolCommand(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestCodingAgentSkill_BuildToolArgs_ClaudeCode(t *testing.T) {
	skill := &CodingAgentSkill{}

	// Claude Code without auto
	args := skill.buildToolArgs(ToolClaudeCode, "fix the bug", ModeInteractive)
	if len(args) == 0 {
		t.Error("buildToolArgs should return non-empty args")
	}
	if args[0] != "fix the bug" {
		t.Errorf("First arg should be task, got %q", args[0])
	}

	// Claude Code with auto mode
	argsAuto := skill.buildToolArgs(ToolClaudeCode, "fix the bug", ModeAuto)
	hasAutoApprove := false
	for _, arg := range argsAuto {
		if arg == "--auto-approve" {
			hasAutoApprove = true
		}
	}
	if !hasAutoApprove {
		t.Errorf("Auto mode should include --auto-approve, got %v", argsAuto)
	}
}

func TestCodingAgentSkill_BuildToolArgs_Codex(t *testing.T) {
	skill := &CodingAgentSkill{}

	// Codex non-interactive
	args := skill.buildToolArgs(ToolCodex, "write tests", ModeAuto)
	hasNonInteractive := false
	for _, arg := range args {
		if arg == "--non-interactive" {
			hasNonInteractive = true
		}
	}
	if !hasNonInteractive {
		t.Errorf("Non-interactive mode should include --non-interactive, got %v", args)
	}

	// Codex interactive (should not have --non-interactive)
	argsInteractive := skill.buildToolArgs(ToolCodex, "write tests", ModeInteractive)
	for _, arg := range argsInteractive {
		if arg == "--non-interactive" {
			t.Error("Interactive mode should not have --non-interactive flag")
		}
	}
}

func TestCodingAgentSkill_BuildToolArgs_GenericTool(t *testing.T) {
	skill := &CodingAgentSkill{}
	args := skill.buildToolArgs("custom-tool", "do something", ModeAuto)
	if len(args) == 0 {
		t.Error("buildToolArgs should return non-empty args for generic tool")
	}
	if args[0] != "do something" {
		t.Errorf("First arg should be task, got %q", args[0])
	}
}

func TestCodingAgentSkill_Execute_Success(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			ExitCode: 0,
			Stdout:   "Task completed successfully",
		}, nil
	}

	skill := NewCodingAgentSkill(domain.ToolConfig{}, mockExec, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool": ToolClaudeCode,
		"task": "add unit tests",
		"mode": ModeAuto,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}
	if result.Output == "" {
		t.Error("Output should not be empty")
	}
}

func TestCodingAgentSkill_Execute_ToolError(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return nil, fmt.Errorf("tool not found")
	}

	skill := NewCodingAgentSkill(domain.ToolConfig{}, mockExec, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"tool": ToolCodex,
		"task": "add tests",
	})
	if err == nil {
		t.Error("expected error when tool execution fails")
	}
}

func TestCodingAgentSkill_Execute_NonZeroExit(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			ExitCode: 1,
			Stderr:   "error occurred",
		}, nil
	}

	skill := NewCodingAgentSkill(domain.ToolConfig{}, mockExec, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"tool": ToolClaudeCode,
		"task": "do something",
	})
	if err == nil {
		t.Error("expected error for non-zero exit code")
	}
}

func TestCodingAgentSkill_GetTimeout(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
	}{
		{"no timeout", map[string]any{}},
		{"int timeout", map[string]any{"timeout_seconds": 60}},
		{"float64 timeout", map[string]any{"timeout_seconds": float64(120)}},
	}

	skill := &CodingAgentSkill{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := skill.getTimeout(tt.params)
			if timeout <= 0 {
				t.Error("timeout should be positive")
			}
		})
	}
}

func TestCodingAgentSkill_FormatOutput(t *testing.T) {
	skill := &CodingAgentSkill{}
	execResult := &executor.ExecutionResult{
		Stdout: "Generated code here",
	}

	output := skill.formatOutput(execResult, 1.5, ModeAuto)
	if output == "" {
		t.Error("formatOutput should return non-empty JSON")
	}
	if output[0] != '{' {
		t.Error("formatOutput should return valid JSON")
	}
}
