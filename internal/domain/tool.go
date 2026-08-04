package domain

import (
	"context"
)

// Permission defines a capability required to execute certain actions or tools.
type Permission string

const (
	PermissionRead    Permission = "read"    // Read data
	PermissionWrite   Permission = "write"   // Write data
	PermissionNetwork Permission = "network" // Make network requests
	PermissionShell   Permission = "shell"   // Execute shell commands (admin only)
)

// ToolConfig defines configuration parameters for a specific tool.
type ToolConfig struct {
	Enabled bool
	APIKey  SecureString // Now directly referencing SecureString, as it's in the same domain package
	Env     map[string]string
	Params  map[string]interface{}
}

// ExecutionStatus describes the outcome of a tool execution beyond simple
// success/error, currently used to represent a call that has been paused
// pending human confirmation rather than executed or rejected
// (specs/260802-improve-nuimanbot-security, Part C / FR-010).
type ExecutionStatus string

const (
	// StatusPendingConfirmation means the tool/action was flagged as
	// requiring confirmation (per persona RULES.md and/or
	// security.confirmation.default_required_actions) and has been recorded
	// in a ConfirmationStore rather than executed. ConfirmationID and
	// Summary are populated; Output is not. The zero value of
	// ExecutionStatus ("") is distinct from this and from any other defined
	// status — existing tools that never set Status are unaffected and
	// should be treated as a normal completed result by callers that only
	// check for StatusPendingConfirmation.
	StatusPendingConfirmation ExecutionStatus = "pending_confirmation"
)

// ExecutionResult encapsulates the output and metadata from a tool execution.
type ExecutionResult struct {
	Output   string
	Metadata map[string]any
	Error    string // Empty if successful

	// Status, ConfirmationID, and Summary are set only when a tool/action
	// call is paused pending confirmation (Status == StatusPendingConfirmation)
	// rather than executed. See internal/usecase/tool.Service.ExecuteWithUser
	// and internal/usecase/security.ConfirmationStore.
	Status         ExecutionStatus
	ConfirmationID string
	Summary        string
}

// Tool interface defines the contract for any tool in the NuimanBot system.
type Tool interface {
	// Name returns the unique tool identifier.
	Name() string

	// Description returns a human-readable description of the tool.
	Description() string

	// InputSchema returns the JSON schema for the parameters the tool accepts.
	InputSchema() map[string]any

	// Execute runs the tool with given parameters. Tool-specific configuration
	// and other runtime context can be retrieved from the `ctx`.
	Execute(ctx context.Context, params map[string]any) (*ExecutionResult, error)

	// RequiredPermissions returns a list of permissions needed to use this tool.
	RequiredPermissions() []Permission

	// Config returns the tool's specific configuration.
	Config() ToolConfig
}
