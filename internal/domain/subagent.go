package domain

import (
	"context"
	"errors"
	"time"
)

// SubagentStatus represents the execution status of a subagent
type SubagentStatus string

const (
	// SubagentStatusPending indicates the subagent is waiting to start
	SubagentStatusPending SubagentStatus = "pending"

	// SubagentStatusRunning indicates the subagent is currently executing
	SubagentStatusRunning SubagentStatus = "running"

	// SubagentStatusComplete indicates the subagent finished successfully
	SubagentStatusComplete SubagentStatus = "complete"

	// SubagentStatusError indicates the subagent failed with an error
	SubagentStatusError SubagentStatus = "error"

	// SubagentStatusTimeout indicates the subagent exceeded time limit
	SubagentStatusTimeout SubagentStatus = "timeout"

	// SubagentStatusCancelled indicates the subagent was cancelled by user
	SubagentStatusCancelled SubagentStatus = "cancelled"
)

// IsTerminal returns true if the status represents a terminal state
func (s SubagentStatus) IsTerminal() bool {
	return s == SubagentStatusComplete ||
		s == SubagentStatusError ||
		s == SubagentStatusTimeout ||
		s == SubagentStatusCancelled
}

// IsValid returns true if the status is a recognized value
func (s SubagentStatus) IsValid() bool {
	switch s {
	case SubagentStatusPending,
		SubagentStatusRunning,
		SubagentStatusComplete,
		SubagentStatusError,
		SubagentStatusTimeout,
		SubagentStatusCancelled:
		return true
	default:
		return false
	}
}

// ResourceLimits defines resource constraints for subagent execution
type ResourceLimits struct {
	// MaxTokens is the maximum number of tokens the subagent can consume
	MaxTokens int

	// MaxToolCalls is the maximum number of tool calls allowed
	MaxToolCalls int

	// Timeout is the maximum execution time
	Timeout time.Duration
}

// DefaultResourceLimits returns sensible default resource limits
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxTokens:    100000, // 100k tokens
		MaxToolCalls: 50,     // 50 tool calls
		Timeout:      5 * time.Minute,
	}
}

// IsWithinLimits checks if current resource usage is within limits
func (r ResourceLimits) IsWithinLimits(tokensUsed int, toolCallsMade int, elapsed time.Duration) bool {
	if r.MaxTokens > 0 && tokensUsed > r.MaxTokens {
		return false
	}
	if r.MaxToolCalls > 0 && toolCallsMade > r.MaxToolCalls {
		return false
	}
	if r.Timeout > 0 && elapsed > r.Timeout {
		return false
	}
	return true
}

// SubagentContext represents an isolated execution context for a subagent
type SubagentContext struct {
	// ID is the unique identifier for this subagent
	ID string

	// ParentContextID is the ID of the parent conversation context
	ParentContextID string

	// SkillName is the name of the skill being executed
	SkillName string

	// AllowedTools is the list of tools the subagent can use
	// Empty list means no tools allowed, nil means all tools allowed
	AllowedTools []string

	// ResourceLimits defines the resource constraints
	ResourceLimits ResourceLimits

	// ConversationHistory is the forked conversation history
	ConversationHistory []Message

	// CreatedAt is when the subagent was created
	CreatedAt time.Time

	// Metadata for additional context
	Metadata map[string]interface{}
}

// Validate checks if the SubagentContext is valid
func (c *SubagentContext) Validate() error {
	if c.ID == "" {
		return errors.New("subagent context ID is required")
	}
	if c.ParentContextID == "" {
		return errors.New("parent context ID is required")
	}
	if c.SkillName == "" {
		return errors.New("skill name is required")
	}
	if c.ResourceLimits.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if c.ResourceLimits.MaxTokens < 0 {
		return errors.New("max tokens must be non-negative")
	}
	if c.ResourceLimits.MaxToolCalls < 0 {
		return errors.New("max tool calls must be non-negative")
	}
	return nil
}

// SubagentResult represents the outcome of subagent execution
type SubagentResult struct {
	// SubagentID is the ID of the subagent that produced this result
	SubagentID string

	// Status is the final execution status
	Status SubagentStatus

	// Output is the aggregated result from the subagent
	Output string

	// ErrorMessage contains error details if Status is SubagentStatusError
	ErrorMessage string

	// TokensUsed is the total tokens consumed
	TokensUsed int

	// ToolCallsMade is the total number of tool calls executed
	ToolCallsMade int

	// ExecutionTime is how long the subagent ran
	ExecutionTime time.Duration

	// CompletedAt is when the subagent finished
	CompletedAt time.Time

	// StepResults contains results from individual execution steps
	StepResults []SubagentStepResult

	// Metadata for additional result data
	Metadata map[string]interface{}
}

// Validate checks if the SubagentResult is valid
func (r *SubagentResult) Validate() error {
	if r.SubagentID == "" {
		return errors.New("subagent ID is required")
	}

	if !r.Status.IsValid() {
		return errors.New("invalid subagent status")
	}

	// If status is error, error message is required
	if r.Status == SubagentStatusError && r.ErrorMessage == "" {
		return errors.New("error message required when status is error")
	}

	return nil
}

// SubagentStepResult represents the result of a single execution step
type SubagentStepResult struct {
	// StepNumber is the sequential step number (1-indexed)
	StepNumber int

	// Action describes what the subagent did in this step
	Action string

	// Result is the outcome of this step
	Result string

	// TokensUsed in this step
	TokensUsed int

	// Duration of this step
	Duration time.Duration
}

// SubagentExecutor defines the interface for executing subagents
type SubagentExecutor interface {
	// Execute runs a subagent in an isolated context
	Execute(ctx context.Context, subagentCtx SubagentContext) (*SubagentResult, error)

	// Cancel terminates a running subagent
	Cancel(ctx context.Context, subagentID string) error

	// GetStatus retrieves the current status of a subagent
	GetStatus(ctx context.Context, subagentID string) (*SubagentResult, error)
}
