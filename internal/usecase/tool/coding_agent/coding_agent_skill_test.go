package coding_agent

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/common"
	"nuimanbot/internal/usecase/tool/executor"
	"nuimanbot/internal/usecase/tool/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodingAgentSkill_Name(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)
	assert.Equal(t, "coding_agent", skill.Name())
}

func TestCodingAgentSkill_Description(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)
	desc := skill.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "coding")
}

func TestCodingAgentSkill_RequiredPermissions(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)
	permissions := skill.RequiredPermissions()
	assert.Contains(t, permissions, domain.PermissionShell)
}

func TestCodingAgentSkill_InputSchema(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)
	schema := skill.InputSchema()

	assert.NotNil(t, schema)
	assert.Contains(t, schema, "type")
	assert.Contains(t, schema, "properties")
	assert.Contains(t, schema, "required")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "tool")
	assert.Contains(t, required, "task")
}

func TestCodingAgentSkill_Execute_MissingTool(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"task": "Create a function",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tool")
}

func TestCodingAgentSkill_Execute_MissingTask(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool": "claude_code",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "task")
}

func TestCodingAgentSkill_Execute_InvalidTool(t *testing.T) {
	skill := NewCodingAgentSkill(domain.ToolConfig{}, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool": "unknown_tool",
		"task": "Create a function",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestCodingAgentSkill_Execute_ClaudeCode(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Equal(t, "claude-code", req.Command)
		assert.True(t, req.PTYMode) // Should use PTY mode
		return &executor.ExecutionResult{
			Stdout:   "Task completed successfully",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"claude_code"},
		},
	}

	skill := NewCodingAgentSkill(config, mockExec, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool": "claude_code",
		"task": "Create a calculateTotal function",
		"mode": "auto",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "completed")
}

func TestCodingAgentSkill_Execute_WithWorkspace(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Equal(t, "/tmp/workspace", req.WorkingDir)
		return &executor.ExecutionResult{
			Stdout:   "Task completed",
			ExitCode: 0,
		}, nil
	}

	pathVal := common.NewPathValidator([]string{"/tmp/workspace"})

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"codex"},
		},
	}

	skill := NewCodingAgentSkill(config, mockExec, pathVal)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool":      "codex",
		"task":      "Add error handling",
		"workspace": "/tmp/workspace",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestCodingAgentSkill_Execute_WorkspacePathTraversal(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	pathVal := common.NewPathValidator([]string{"/tmp/workspace"})

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"codex"},
		},
	}

	skill := NewCodingAgentSkill(config, mockExec, pathVal)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool":      "codex",
		"task":      "Add error handling",
		"workspace": "/tmp/workspace/../etc",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestCodingAgentSkill_Execute_WithTimeout(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Verify custom timeout is set
		assert.NotNil(t, req.Timeout)
		return &executor.ExecutionResult{
			Stdout:   "Task completed",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"claude_code"},
		},
	}

	skill := NewCodingAgentSkill(config, mockExec, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool":    "claude_code",
		"task":    "Refactor code",
		"timeout": 600,
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestCodingAgentSkill_Execute_InteractiveMode(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.True(t, req.PTYMode)
		return &executor.ExecutionResult{
			Stdout:   "Awaiting approval...",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"claude_code"},
		},
	}

	skill := NewCodingAgentSkill(config, mockExec, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"tool": "claude_code",
		"task": "Add tests",
		"mode": "interactive",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}
