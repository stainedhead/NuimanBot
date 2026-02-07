package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/skill/common"
	"nuimanbot/internal/usecase/skill/executor"
)

const (
	defaultTimeout   = 30 * time.Second
	ghCommand        = "gh"
	defaultRateLimit = "30/minute"
)

// GitHubSkill provides GitHub operations via gh CLI
type GitHubSkill struct {
	config      domain.SkillConfig
	executor    executor.ExecutorService
	rateLimiter *common.RateLimiter
	sanitizer   *common.OutputSanitizer
}

// Action types supported by GitHubSkill
const (
	ActionIssueCreate  = "issue_create"
	ActionIssueList    = "issue_list"
	ActionIssueView    = "issue_view"
	ActionIssueComment = "issue_comment"
	ActionIssueClose   = "issue_close"
	ActionPRCreate     = "pr_create"
	ActionPRList       = "pr_list"
	ActionPRView       = "pr_view"
	ActionPRReview     = "pr_review"
	ActionPRMerge      = "pr_merge"
	ActionRepoView     = "repo_view"
	ActionWorkflowRun  = "workflow_run"
)

// NewGitHubSkill creates a new GitHubSkill instance
func NewGitHubSkill(
	config domain.SkillConfig,
	executor executor.ExecutorService,
	rateLimiter *common.RateLimiter,
	sanitizer *common.OutputSanitizer,
) *GitHubSkill {
	return &GitHubSkill{
		config:      config,
		executor:    executor,
		rateLimiter: rateLimiter,
		sanitizer:   sanitizer,
	}
}

// Name returns the skill identifier
func (s *GitHubSkill) Name() string {
	return "github"
}

// Description returns a human-readable description
func (s *GitHubSkill) Description() string {
	return "GitHub operations via gh CLI: manage issues, PRs, repos, and workflows"
}

// RequiredPermissions returns the permissions needed
func (s *GitHubSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{
		domain.PermissionNetwork,
		domain.PermissionShell,
	}
}

// Config returns the skill configuration
func (s *GitHubSkill) Config() domain.SkillConfig {
	return s.config
}

// InputSchema returns the JSON schema for parameters
func (s *GitHubSkill) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{
					ActionIssueCreate, ActionIssueList, ActionIssueView,
					ActionIssueComment, ActionIssueClose,
					ActionPRCreate, ActionPRList, ActionPRView,
					ActionPRReview, ActionPRMerge,
					ActionRepoView, ActionWorkflowRun,
				},
				"description": "GitHub action to perform",
			},
			"repo": map[string]any{
				"type":        "string",
				"description": "Repository in format 'owner/repo'",
			},
			"params": map[string]any{
				"type":        "object",
				"description": "Action-specific parameters",
			},
		},
		"required": []string{"action"},
	}
}

// Execute runs the GitHub operation
func (s *GitHubSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	action, err := s.validateAction(params)
	if err != nil {
		return nil, err
	}

	repo := s.getRepo(params)
	actionParams := s.getActionParams(params)

	args, err := s.buildGHArgs(action, repo, actionParams)
	if err != nil {
		return nil, err
	}

	execResult, err := s.executeGH(ctx, args)
	if err != nil {
		return nil, err
	}

	output := s.formatOutput(execResult.Stdout)

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"action":    action,
			"repo":      repo,
			"exit_code": execResult.ExitCode,
		},
	}, nil
}

// validateAction validates the action parameter
func (s *GitHubSkill) validateAction(params map[string]any) (string, error) {
	action, ok := params["action"].(string)
	if !ok || action == "" {
		return "", fmt.Errorf("action is required")
	}

	// Validate action is supported
	validActions := []string{
		ActionIssueCreate, ActionIssueList, ActionIssueView,
		ActionIssueComment, ActionIssueClose,
		ActionPRCreate, ActionPRList, ActionPRView,
		ActionPRReview, ActionPRMerge,
		ActionRepoView, ActionWorkflowRun,
	}

	for _, valid := range validActions {
		if action == valid {
			return action, nil
		}
	}

	return "", fmt.Errorf("unsupported action: %s", action)
}

// getRepo extracts the repo parameter
func (s *GitHubSkill) getRepo(params map[string]any) string {
	if repo, ok := params["repo"].(string); ok {
		return repo
	}

	// Check for default_repo in config
	if defaultRepo, ok := s.config.Params["default_repo"].(string); ok {
		return defaultRepo
	}

	return ""
}

// getActionParams extracts action-specific parameters
func (s *GitHubSkill) getActionParams(params map[string]any) map[string]any {
	if actionParams, ok := params["params"].(map[string]any); ok {
		return actionParams
	}
	return make(map[string]any)
}

// buildGHArgs constructs gh CLI arguments based on action
func (s *GitHubSkill) buildGHArgs(action, repo string, actionParams map[string]any) ([]string, error) {
	var args []string

	switch action {
	case ActionIssueList:
		args = s.buildIssueListArgs(repo)
	case ActionIssueCreate:
		args = s.buildIssueCreateArgs(repo, actionParams)
	case ActionIssueView:
		args = s.buildIssueViewArgs(repo, actionParams)
	case ActionIssueComment:
		args = s.buildIssueCommentArgs(repo, actionParams)
	case ActionIssueClose:
		args = s.buildIssueCloseArgs(repo, actionParams)
	case ActionPRList:
		args = s.buildPRListArgs(repo)
	case ActionPRCreate:
		args = s.buildPRCreateArgs(repo, actionParams)
	case ActionPRView:
		args = s.buildPRViewArgs(repo, actionParams)
	case ActionPRReview:
		args = s.buildPRReviewArgs(repo, actionParams)
	case ActionPRMerge:
		args = s.buildPRMergeArgs(repo, actionParams)
	case ActionRepoView:
		args = s.buildRepoViewArgs(repo)
	case ActionWorkflowRun:
		args = s.buildWorkflowRunArgs(repo, actionParams)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}

	return args, nil
}

// buildIssueListArgs builds args for issue list
func (s *GitHubSkill) buildIssueListArgs(repo string) []string {
	return s.appendRepoFlag([]string{"issue", "list", "--json", "number,title,state,labels"}, repo)
}

// buildIssueCreateArgs builds args for issue create
func (s *GitHubSkill) buildIssueCreateArgs(repo string, params map[string]any) []string {
	args := s.appendRepoFlag([]string{"issue", "create"}, repo)
	args = s.appendStringParam(args, params, "title", "--title")
	args = s.appendStringParam(args, params, "body", "--body")
	return args
}

// buildIssueViewArgs builds args for issue view
func (s *GitHubSkill) buildIssueViewArgs(repo string, params map[string]any) []string {
	args := []string{"issue", "view"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	return s.appendRepoFlag(args, repo)
}

// buildIssueCommentArgs builds args for issue comment
func (s *GitHubSkill) buildIssueCommentArgs(repo string, params map[string]any) []string {
	args := []string{"issue", "comment"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	args = s.appendStringParam(args, params, "body", "--body")
	return s.appendRepoFlag(args, repo)
}

// buildIssueCloseArgs builds args for issue close
func (s *GitHubSkill) buildIssueCloseArgs(repo string, params map[string]any) []string {
	args := []string{"issue", "close"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	return s.appendRepoFlag(args, repo)
}

// buildPRListArgs builds args for PR list
func (s *GitHubSkill) buildPRListArgs(repo string) []string {
	return s.appendRepoFlag([]string{"pr", "list", "--json", "number,title,state,author"}, repo)
}

// buildPRCreateArgs builds args for PR create
func (s *GitHubSkill) buildPRCreateArgs(repo string, params map[string]any) []string {
	args := s.appendRepoFlag([]string{"pr", "create"}, repo)
	args = s.appendStringParam(args, params, "title", "--title")
	args = s.appendStringParam(args, params, "body", "--body")
	return args
}

// buildPRViewArgs builds args for PR view
func (s *GitHubSkill) buildPRViewArgs(repo string, params map[string]any) []string {
	args := []string{"pr", "view"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	return s.appendRepoFlag(args, repo)
}

// buildPRReviewArgs builds args for PR review
func (s *GitHubSkill) buildPRReviewArgs(repo string, params map[string]any) []string {
	args := []string{"pr", "review"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	if comment, ok := params["comment"].(string); ok {
		args = append(args, "--comment", "--body", comment)
	}
	return s.appendRepoFlag(args, repo)
}

// buildPRMergeArgs builds args for PR merge
func (s *GitHubSkill) buildPRMergeArgs(repo string, params map[string]any) []string {
	args := []string{"pr", "merge"}
	if number, ok := params["number"].(string); ok {
		args = append(args, number)
	}
	return s.appendRepoFlag(args, repo)
}

// buildRepoViewArgs builds args for repo view
func (s *GitHubSkill) buildRepoViewArgs(repo string) []string {
	args := []string{"repo", "view", "--json", "name,description,owner"}
	if repo != "" {
		args = append(args, repo)
	}
	return args
}

// buildWorkflowRunArgs builds args for workflow run
func (s *GitHubSkill) buildWorkflowRunArgs(repo string, params map[string]any) []string {
	args := []string{"workflow", "run"}
	if workflow, ok := params["workflow"].(string); ok {
		args = append(args, workflow)
	}
	return s.appendRepoFlag(args, repo)
}

// Helper methods to reduce duplication

// appendRepoFlag appends --repo flag if repo is specified
func (s *GitHubSkill) appendRepoFlag(args []string, repo string) []string {
	if repo != "" {
		return append(args, "--repo", repo)
	}
	return args
}

// appendStringParam appends a string parameter with its flag if present
func (s *GitHubSkill) appendStringParam(args []string, params map[string]any, key, flag string) []string {
	if value, ok := params[key].(string); ok && value != "" {
		return append(args, flag, value)
	}
	return args
}

// executeGH runs the gh CLI command
func (s *GitHubSkill) executeGH(ctx context.Context, args []string) (*executor.ExecutionResult, error) {
	execReq := executor.ExecutionRequest{
		Command: ghCommand,
		Args:    args,
		Timeout: defaultTimeout,
	}

	execResult, err := s.executor.Execute(ctx, execReq)
	if err != nil {
		return nil, fmt.Errorf("gh execution failed: %w", err)
	}

	if execResult.ExitCode != 0 {
		stderr := execResult.Stderr
		if s.sanitizer != nil {
			stderr = s.sanitizer.SanitizeOutput(stderr)
		}
		return nil, fmt.Errorf("gh command failed (exit %d): %s", execResult.ExitCode, stderr)
	}

	return execResult, nil
}

// formatOutput formats and sanitizes the output
func (s *GitHubSkill) formatOutput(rawOutput string) string {
	output := strings.TrimSpace(rawOutput)

	if s.sanitizer != nil {
		output = s.sanitizer.SanitizeOutput(output)
	}

	return output
}
