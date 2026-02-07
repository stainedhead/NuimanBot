package executor

import (
	"context"
	"time"
)

// ExecutorService handles external command execution with timeout and PTY support
type ExecutorService interface {
	// Execute runs a command with timeout and returns output
	Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)

	// ExecuteBackground runs a command in the background
	ExecuteBackground(ctx context.Context, req ExecutionRequest) (*BackgroundSession, error)

	// GetSessionStatus returns the status of a background session
	GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error)

	// GetSessionOutput retrieves the output of a background session
	GetSessionOutput(ctx context.Context, sessionID string) (string, error)

	// CancelSession cancels a running background session
	CancelSession(ctx context.Context, sessionID string) error
}

// ExecutionRequest represents a command execution request
type ExecutionRequest struct {
	Command    string            // Command to execute
	Args       []string          // Command arguments
	WorkingDir string            // Working directory (default: current)
	Env        map[string]string // Environment variables
	Timeout    time.Duration     // Execution timeout
	PTYMode    bool              // Use PTY mode for interactive CLIs
}

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
	Stdout   string        // Standard output
	Stderr   string        // Standard error
	ExitCode int           // Exit code
	Duration time.Duration // Execution duration
}

// BackgroundSession represents a long-running background session
type BackgroundSession struct {
	ID        string    // Unique session ID
	StartedAt time.Time // Session start time
}

// SessionStatus represents the status of a background session
type SessionStatus struct {
	ID          string       // Session ID
	Status      SessionState // Current status
	StartedAt   time.Time    // Start time
	CompletedAt *time.Time   // Completion time (nil if running)
	ExitCode    *int         // Exit code (nil if running)
	Error       string       // Error message (empty if no error)
}

// SessionState represents the state of a background session
type SessionState string

const (
	SessionStateRunning   SessionState = "running"
	SessionStateCompleted SessionState = "completed"
	SessionStateFailed    SessionState = "failed"
	SessionStateCancelled SessionState = "cancelled"
)
