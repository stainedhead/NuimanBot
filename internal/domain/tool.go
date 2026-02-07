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

// ExecutionResult encapsulates the output and metadata from a tool execution.
type ExecutionResult struct {
	Output   string
	Metadata map[string]any
	Error    string // Empty if successful
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
