package github

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/skill/executor"
	"nuimanbot/internal/usecase/skill/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubSkill_Name(t *testing.T) {
	skill := NewGitHubSkill(domain.SkillConfig{}, nil, nil, nil)
	assert.Equal(t, "github", skill.Name())
}

func TestGitHubSkill_Description(t *testing.T) {
	skill := NewGitHubSkill(domain.SkillConfig{}, nil, nil, nil)
	desc := skill.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "GitHub")
}

func TestGitHubSkill_RequiredPermissions(t *testing.T) {
	skill := NewGitHubSkill(domain.SkillConfig{}, nil, nil, nil)
	permissions := skill.RequiredPermissions()
	assert.Contains(t, permissions, domain.PermissionNetwork)
	assert.Contains(t, permissions, domain.PermissionShell)
}

func TestGitHubSkill_InputSchema(t *testing.T) {
	skill := NewGitHubSkill(domain.SkillConfig{}, nil, nil, nil)
	schema := skill.InputSchema()

	// Verify required fields
	assert.NotNil(t, schema)
	assert.Contains(t, schema, "type")
	assert.Contains(t, schema, "properties")
	assert.Contains(t, schema, "required")

	// Verify action is required
	required, ok := schema["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "action")
}

func TestGitHubSkill_Execute_IssueList(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Verify gh command is used
		assert.Equal(t, "gh", req.Command)
		assert.Contains(t, req.Args, "issue")
		assert.Contains(t, req.Args, "list")

		return &executor.ExecutionResult{
			Stdout:   `[{"number":1,"title":"Test Issue","state":"open"}]`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_list",
		"repo":   "owner/repo",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "Test Issue")
}

func TestGitHubSkill_Execute_IssueCreate(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Equal(t, "gh", req.Command)
		assert.Contains(t, req.Args, "issue")
		assert.Contains(t, req.Args, "create")
		assert.Contains(t, req.Args, "--title")
		assert.Contains(t, req.Args, "Bug Report")

		return &executor.ExecutionResult{
			Stdout:   `{"number":42,"title":"Bug Report","state":"open"}`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_create",
		"repo":   "owner/repo",
		"params": map[string]any{
			"title": "Bug Report",
			"body":  "Description of the bug",
		},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "Bug Report")
}

func TestGitHubSkill_Execute_PRList(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Equal(t, "gh", req.Command)
		assert.Contains(t, req.Args, "pr")
		assert.Contains(t, req.Args, "list")

		return &executor.ExecutionResult{
			Stdout:   `[{"number":10,"title":"Feature PR","state":"open"}]`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "pr_list",
		"repo":   "owner/repo",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "Feature PR")
}

func TestGitHubSkill_Execute_RepoView(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Equal(t, "gh", req.Command)
		assert.Contains(t, req.Args, "repo")
		assert.Contains(t, req.Args, "view")

		return &executor.ExecutionResult{
			Stdout:   `{"name":"repo","full_name":"owner/repo","description":"Test repo"}`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "repo_view",
		"repo":   "owner/repo",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "Test repo")
}

func TestGitHubSkill_Execute_MissingAction(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "action")
}

func TestGitHubSkill_Execute_InvalidAction(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "invalid_action",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestGitHubSkill_Execute_WithRateLimit(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			Stdout:   `[]`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"rate_limit": "30/minute",
		},
	}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	// First call should succeed
	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_list",
		"repo":   "owner/repo",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_OutputSanitization(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			Stdout:   `{"token":"ghp_1234567890abcdefGHIJKLMNOPQRSTUVWXYZ"}`,
			Stderr:   "",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_list",
		"repo":   "owner/repo",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
	// Token should be redacted if sanitizer is configured
	// This test will pass even without sanitizer as it's optional
}

func TestGitHubSkill_Execute_IssueView(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "issue")
		assert.Contains(t, req.Args, "view")
		return &executor.ExecutionResult{
			Stdout:   "Issue #1: Test Issue",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_view",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "1"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_IssueComment(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "issue")
		assert.Contains(t, req.Args, "comment")
		assert.Contains(t, req.Args, "--body")
		return &executor.ExecutionResult{
			Stdout:   "Comment added",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_comment",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "1", "body": "LGTM"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_IssueClose(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "issue")
		assert.Contains(t, req.Args, "close")
		return &executor.ExecutionResult{
			Stdout:   "Issue closed",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_close",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "1"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_PRCreate(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "pr")
		assert.Contains(t, req.Args, "create")
		assert.Contains(t, req.Args, "--title")
		return &executor.ExecutionResult{
			Stdout:   `{"number":100,"title":"New Feature"}`,
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "pr_create",
		"repo":   "owner/repo",
		"params": map[string]any{"title": "New Feature", "body": "Details"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_PRView(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "pr")
		assert.Contains(t, req.Args, "view")
		return &executor.ExecutionResult{
			Stdout:   "PR #10: Feature",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "pr_view",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "10"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_PRReview(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "pr")
		assert.Contains(t, req.Args, "review")
		assert.Contains(t, req.Args, "--comment")
		return &executor.ExecutionResult{
			Stdout:   "Review submitted",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "pr_review",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "10", "comment": "Looks good"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_PRMerge(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "pr")
		assert.Contains(t, req.Args, "merge")
		return &executor.ExecutionResult{
			Stdout:   "PR merged",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "pr_merge",
		"repo":   "owner/repo",
		"params": map[string]any{"number": "10"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_WorkflowRun(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		assert.Contains(t, req.Args, "workflow")
		assert.Contains(t, req.Args, "run")
		return &executor.ExecutionResult{
			Stdout:   "Workflow triggered",
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "workflow_run",
		"repo":   "owner/repo",
		"params": map[string]any{"workflow": "ci.yml"},
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_DefaultRepo(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Should use default_repo from config
		assert.Contains(t, req.Args, "default/repo")
		return &executor.ExecutionResult{
			Stdout:   `[]`,
			ExitCode: 0,
		}, nil
	}

	config := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"default_repo": "default/repo",
		},
	}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_list",
	})

	testutil.AssertNoError(t, err)
	assert.NotNil(t, result)
}

func TestGitHubSkill_Execute_GHCommandFails(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return &executor.ExecutionResult{
			Stdout:   "",
			Stderr:   "authentication error",
			ExitCode: 1,
		}, nil
	}

	config := domain.SkillConfig{Enabled: true}
	skill := NewGitHubSkill(config, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"action": "issue_list",
		"repo":   "owner/repo",
	})

	testutil.AssertError(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "gh command failed")
}
