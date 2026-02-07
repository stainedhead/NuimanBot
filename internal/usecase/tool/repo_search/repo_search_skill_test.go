package repo_search

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

func TestRepoSearchSkill_Name(t *testing.T) {
	skill := NewRepoSearchSkill(domain.ToolConfig{}, nil, nil, nil)
	assert.Equal(t, "repo_search", skill.Name())
}

func TestRepoSearchSkill_Description(t *testing.T) {
	skill := NewRepoSearchSkill(domain.ToolConfig{}, nil, nil, nil)
	desc := skill.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "codebase")
}

func TestRepoSearchSkill_RequiredPermissions(t *testing.T) {
	skill := NewRepoSearchSkill(domain.ToolConfig{}, nil, nil, nil)
	permissions := skill.RequiredPermissions()
	assert.Contains(t, permissions, domain.PermissionRead)
}

func TestRepoSearchSkill_InputSchema(t *testing.T) {
	skill := NewRepoSearchSkill(domain.ToolConfig{}, nil, nil, nil)
	schema := skill.InputSchema()

	// Verify required fields are present
	assert.NotNil(t, schema)
	assert.Contains(t, schema, "type")
	assert.Contains(t, schema, "properties")
	assert.Contains(t, schema, "required")

	// Verify query is a required field
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "query")
}

func TestRepoSearchSkill_Execute_Success(t *testing.T) {
	// Create mock executor that returns ripgrep output
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			Stdout:   "internal/domain/user.go:42:type User struct {",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": []interface{}{"/tmp/workspace"},
		},
	}

	skill := NewRepoSearchSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "User struct",
		"path":  "/tmp/workspace",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Output)
}

func TestRepoSearchSkill_Execute_MissingQuery(t *testing.T) {
	mockExec := testutil.NewMockExecutor()

	config := domain.ToolConfig{
		Enabled: true,
	}

	skill := NewRepoSearchSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query")
}

func TestRepoSearchSkill_Execute_PathTraversal(t *testing.T) {
	mockExec := testutil.NewMockExecutor()

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": []interface{}{"/tmp/workspace"},
		},
	}

	// Create path validator with allowed directories
	pathVal := common.NewPathValidator([]string{"/tmp/workspace"})

	skill := NewRepoSearchSkill(config, mockExec, pathVal, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "test",
		"path":  "/tmp/workspace/../etc/passwd",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestRepoSearchSkill_Execute_OutsideWorkspace(t *testing.T) {
	mockExec := testutil.NewMockExecutor()

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": []interface{}{"/tmp/workspace"},
		},
	}

	// Create path validator with allowed directories
	pathVal := common.NewPathValidator([]string{"/tmp/workspace"})

	skill := NewRepoSearchSkill(config, mockExec, pathVal, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "test",
		"path":  "/etc",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "outside allowed workspace")
}

func TestRepoSearchSkill_Execute_WithFileType(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Verify file type filter was applied
		assert.Contains(t, req.Args, "--type")
		assert.Contains(t, req.Args, "go")

		return &executor.ExecutionResult{
			Stdout:   "internal/domain/user.go:42:type User struct {",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": []interface{}{"/tmp/workspace"},
		},
	}

	skill := NewRepoSearchSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query":     "User struct",
		"path":      "/tmp/workspace",
		"file_type": "go",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestRepoSearchSkill_Execute_WithMaxResults(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Verify max results limit was applied
		assert.Contains(t, req.Args, "--max-count")
		assert.Contains(t, req.Args, "10")

		return &executor.ExecutionResult{
			Stdout:   "file1.go:1:match",
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": []interface{}{"/tmp/workspace"},
		},
	}

	skill := NewRepoSearchSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query":       "test",
		"path":        "/tmp/workspace",
		"max_results": 10,
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}
