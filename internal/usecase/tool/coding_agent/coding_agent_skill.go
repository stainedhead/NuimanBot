package coding_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/skill/common"
	"nuimanbot/internal/usecase/skill/executor"
)

const (
	defaultTimeout = 300 * time.Second
)

// CodingAgentSkill orchestrates external coding CLI tools
type CodingAgentSkill struct {
	config   domain.SkillConfig
	executor executor.ExecutorService
	pathVal  *common.PathValidator
}

// Tool constants
const (
	ToolCodex      = "codex"
	ToolClaudeCode = "claude_code"
	ToolOpenCode   = "opencode"
	ToolGemini     = "gemini"
	ToolCopilot    = "copilot"
)

// Mode constants
const (
	ModeInteractive = "interactive"
	ModeAuto        = "auto"
	ModeYOLO        = "yolo"
)

// CodingAgentOutput represents the structured output
type CodingAgentOutput struct {
	Status             string   `json:"status"`
	Output             string   `json:"output"`
	FilesModified      []string `json:"files_modified,omitempty"`
	SessionID          string   `json:"session_id,omitempty"`
	Duration           float64  `json:"duration"`
	ApprovalsRequested int      `json:"approvals_requested,omitempty"`
	ApprovalsGranted   int      `json:"approvals_granted,omitempty"`
}

// NewCodingAgentSkill creates a new CodingAgentSkill instance
func NewCodingAgentSkill(
	config domain.SkillConfig,
	executor executor.ExecutorService,
	pathVal *common.PathValidator,
) *CodingAgentSkill {
	return &CodingAgentSkill{
		config:   config,
		executor: executor,
		pathVal:  pathVal,
	}
}

// Name returns the skill identifier
func (s *CodingAgentSkill) Name() string {
	return "coding_agent"
}

// Description returns a human-readable description
func (s *CodingAgentSkill) Description() string {
	return "Orchestrate external coding CLI tools (Codex, Claude Code, OpenCode, Gemini, Copilot) for code generation and refactoring"
}

// RequiredPermissions returns the permissions needed
func (s *CodingAgentSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionShell}
}

// Config returns the skill configuration
func (s *CodingAgentSkill) Config() domain.SkillConfig {
	return s.config
}

// InputSchema returns the JSON schema for parameters
func (s *CodingAgentSkill) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tool": map[string]any{
				"type": "string",
				"enum": []string{
					ToolCodex, ToolClaudeCode, ToolOpenCode,
					ToolGemini, ToolCopilot,
				},
				"description": "Coding CLI tool to use",
			},
			"task": map[string]any{
				"type":        "string",
				"description": "Task description for the coding agent",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{ModeInteractive, ModeAuto, ModeYOLO},
				"default":     ModeInteractive,
				"description": "Execution mode",
			},
			"workspace": map[string]any{
				"type":        "string",
				"description": "Working directory (default: current workspace)",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"default":     300,
				"description": "Task timeout in seconds",
			},
		},
		"required": []string{"tool", "task"},
	}
}

// Execute runs the coding agent task
func (s *CodingAgentSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	tool, task, err := s.validateParams(params)
	if err != nil {
		return nil, err
	}

	workspace, err := s.validateWorkspace(params)
	if err != nil {
		return nil, err
	}

	timeout := s.getTimeout(params)
	mode := s.getMode(params)

	startTime := time.Now()

	execResult, err := s.executeTool(ctx, tool, task, workspace, timeout, mode)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	duration := time.Since(startTime).Seconds()

	output := s.formatOutput(execResult, duration, mode)

	return &domain.SkillResult{
		Output: output,
		Metadata: map[string]any{
			"tool":      tool,
			"mode":      mode,
			"workspace": workspace,
			"duration":  duration,
		},
	}, nil
}

// validateParams validates required parameters
func (s *CodingAgentSkill) validateParams(params map[string]any) (string, string, error) {
	tool, ok := params["tool"].(string)
	if !ok || tool == "" {
		return "", "", fmt.Errorf("tool is required")
	}

	task, ok := params["task"].(string)
	if !ok || task == "" {
		return "", "", fmt.Errorf("task is required")
	}

	// Validate tool is supported
	validTools := []string{ToolCodex, ToolClaudeCode, ToolOpenCode, ToolGemini, ToolCopilot}
	supported := false
	for _, valid := range validTools {
		if tool == valid {
			supported = true
			break
		}
	}

	if !supported {
		return "", "", fmt.Errorf("unsupported tool: %s", tool)
	}

	return tool, task, nil
}

// validateWorkspace validates workspace path
func (s *CodingAgentSkill) validateWorkspace(params map[string]any) (string, error) {
	workspace := "."
	if ws, ok := params["workspace"].(string); ok && ws != "" {
		workspace = ws
	}

	// Validate path if pathValidator is configured
	if s.pathVal != nil {
		if err := s.pathVal.ValidatePath(workspace); err != nil {
			return "", fmt.Errorf("workspace validation failed: %w", err)
		}
	}

	return workspace, nil
}

// getTimeout extracts timeout parameter
func (s *CodingAgentSkill) getTimeout(params map[string]any) time.Duration {
	if timeout, ok := params["timeout"].(int); ok {
		return time.Duration(timeout) * time.Second
	}
	if timeout, ok := params["timeout"].(float64); ok {
		return time.Duration(timeout) * time.Second
	}
	return defaultTimeout
}

// getMode extracts mode parameter
func (s *CodingAgentSkill) getMode(params map[string]any) string {
	if mode, ok := params["mode"].(string); ok {
		return mode
	}
	return ModeInteractive
}

// executeTool executes the coding tool CLI
func (s *CodingAgentSkill) executeTool(ctx context.Context, tool, task, workspace string, timeout time.Duration, mode string) (*executor.ExecutionResult, error) {
	command := s.getToolCommand(tool)
	args := s.buildToolArgs(tool, task, mode)

	execReq := executor.ExecutionRequest{
		Command:    command,
		Args:       args,
		WorkingDir: workspace,
		Timeout:    timeout,
		PTYMode:    true, // Enable PTY for interactive CLIs
	}

	execResult, err := s.executor.Execute(ctx, execReq)
	if err != nil {
		return nil, err
	}

	if execResult.ExitCode != 0 {
		return nil, fmt.Errorf("tool exited with code %d: %s", execResult.ExitCode, execResult.Stderr)
	}

	return execResult, nil
}

// getToolCommand returns the CLI command for the tool
func (s *CodingAgentSkill) getToolCommand(tool string) string {
	switch tool {
	case ToolCodex:
		return "codex"
	case ToolClaudeCode:
		return "claude-code"
	case ToolOpenCode:
		return "opencode"
	case ToolGemini:
		return "gemini"
	case ToolCopilot:
		return "copilot"
	default:
		return tool
	}
}

// buildToolArgs builds command arguments based on tool and mode
func (s *CodingAgentSkill) buildToolArgs(tool, task, mode string) []string {
	args := []string{}

	switch tool {
	case ToolClaudeCode:
		// Claude Code expects task as argument
		args = append(args, task)
		if mode == ModeAuto {
			args = append(args, "--auto-approve")
		}
	case ToolCodex:
		args = append(args, "--task", task)
		if mode != ModeInteractive {
			args = append(args, "--non-interactive")
		}
	default:
		// Generic: pass task as argument
		args = append(args, task)
	}

	return args
}

// formatOutput formats the execution result as JSON
func (s *CodingAgentSkill) formatOutput(execResult *executor.ExecutionResult, duration float64, mode string) string {
	output := CodingAgentOutput{
		Status:   "completed",
		Output:   execResult.Stdout,
		Duration: duration,
	}

	// Parse output for additional metadata if available
	// For now, keep it simple

	jsonOutput, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to format output: %s"}`, err.Error())
	}

	return string(jsonOutput)
}
