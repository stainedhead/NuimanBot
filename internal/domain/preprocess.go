package domain

import (
	"fmt"
	"strings"
	"time"
)

// MaxCommandTimeout is the maximum allowed timeout for preprocessing commands
const MaxCommandTimeout = 5 * time.Second

// MaxCommandOutputSize is the maximum output size (10KB)
const MaxCommandOutputSize = 10 * 1024

// AllowedCommands is the whitelist of commands allowed in preprocessing
var AllowedCommands = []string{
	"git",
	"gh",
	"ls",
	"cat",
	"grep",
}

// PreprocessCommand represents a command to execute during skill preprocessing
type PreprocessCommand struct {
	// Command is the shell command to execute
	Command string

	// Timeout is the maximum execution time
	Timeout time.Duration

	// WorkingDir is the directory to execute the command in (optional)
	WorkingDir string
}

// Validate checks if the command is safe to execute
func (c *PreprocessCommand) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if c.Timeout > MaxCommandTimeout {
		return fmt.Errorf("timeout %v exceeds maximum %v", c.Timeout, MaxCommandTimeout)
	}

	if !c.IsAllowed() {
		return fmt.Errorf("command not in whitelist: %s", c.Command)
	}

	if c.HasShellMetacharacters() {
		return fmt.Errorf("command contains shell metacharacters: %s", c.Command)
	}

	return nil
}

// IsAllowed checks if the command is in the whitelist
func (c *PreprocessCommand) IsAllowed() bool {
	if c.Command == "" {
		return false
	}

	// Extract the base command (first word)
	parts := strings.Fields(c.Command)
	if len(parts) == 0 {
		return false
	}

	baseCommand := parts[0]

	for _, allowed := range AllowedCommands {
		if baseCommand == allowed {
			return true
		}
	}

	return false
}

// HasShellMetacharacters checks if the command contains dangerous shell metacharacters
func (c *PreprocessCommand) HasShellMetacharacters() bool {
	// Dangerous characters that could enable command injection
	dangerousChars := []string{
		"|",  // pipe
		";",  // command separator
		"&",  // background/and
		"$",  // variable expansion
		"`",  // command substitution
		">",  // redirect output
		"<",  // redirect input
		"||", // or
		"&&", // and
	}

	for _, char := range dangerousChars {
		if strings.Contains(c.Command, char) {
			return true
		}
	}

	return false
}

// CommandResult represents the result of executing a preprocessing command
type CommandResult struct {
	// Output is the stdout from the command
	Output string

	// Error is the error message if execution failed
	Error string

	// ExitCode is the command exit code
	ExitCode int

	// ExecutionTime is how long the command took to execute
	ExecutionTime time.Duration

	// Truncated indicates if the output was truncated
	Truncated bool
}

// IsSuccess returns true if the command completed successfully
func (r *CommandResult) IsSuccess() bool {
	return r.ExitCode == 0
}

// IsError returns true if the command failed
func (r *CommandResult) IsError() bool {
	return r.ExitCode != 0
}

// IsTruncated returns true if the output exceeded the size limit
func (r *CommandResult) IsTruncated() bool {
	return len(r.Output) > MaxCommandOutputSize
}

// TruncatedOutput returns the output truncated to the maximum size
func (r *CommandResult) TruncatedOutput() string {
	if len(r.Output) <= MaxCommandOutputSize {
		return r.Output
	}

	// Reserve space for truncation message
	truncationMsg := "\n... (output truncated)"
	maxContentSize := MaxCommandOutputSize - len(truncationMsg)

	return r.Output[:maxContentSize] + truncationMsg
}
