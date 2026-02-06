package domain

import (
	"context"
)

// Permission defines a capability required to execute certain actions or skills.
type Permission string

const (
	PermissionRead    Permission = "read"    // Read data
	PermissionWrite   Permission = "write"   // Write data
	PermissionNetwork Permission = "network" // Make network requests
	PermissionShell   Permission = "shell"   // Execute shell commands (admin only)
)

// SkillConfig defines configuration parameters for a specific skill.
type SkillConfig struct {
	Enabled bool
	APIKey  SecureString // Now directly referencing SecureString, as it's in the same domain package
	Env     map[string]string
	Params  map[string]interface{}
}

// SkillResult encapsulates the output and metadata from a skill execution.
type SkillResult struct {
	Output   string
	Metadata map[string]any
	Error    string // Empty if successful
}

// Skill interface defines the contract for any skill in the NuimanBot system.
type Skill interface {
	// Name returns the unique skill identifier.
	Name() string

	// Description returns a human-readable description of the skill.
	Description() string

	// InputSchema returns the JSON schema for the parameters the skill accepts.
	InputSchema() map[string]any

	// Execute runs the skill with given parameters. Skill-specific configuration
	// and other runtime context can be retrieved from the `ctx`.
	Execute(ctx context.Context, params map[string]any) (*SkillResult, error)

	// RequiredPermissions returns a list of permissions needed to use this skill.
	RequiredPermissions() []Permission

	// Config returns the skill's specific configuration.
	Config() SkillConfig
}
