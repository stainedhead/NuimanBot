package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

// executorService implements ExecutorService
type executorService struct {
	sessions map[string]*sessionInfo
	mu       sync.RWMutex
}

// sessionInfo holds information about a background session
type sessionInfo struct {
	ID          string
	Cmd         *exec.Cmd
	Cancel      context.CancelFunc
	StartedAt   time.Time
	CompletedAt *time.Time
	ExitCode    *int
	Output      *bytes.Buffer
	Status      SessionState
	Error       string
}

// NewExecutorService creates a new ExecutorService
func NewExecutorService() ExecutorService {
	return &executorService{
		sessions: make(map[string]*sessionInfo),
	}
}

// Execute runs a command with timeout and returns output
func (s *executorService) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	// Create context with timeout
	execCtx, cancel := s.createExecutionContext(ctx, req.Timeout)
	if cancel != nil {
		defer cancel()
	}

	// Create and configure command
	cmd := s.createCommand(execCtx, req)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command and measure duration
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	// Handle execution result
	exitCode, err := s.handleExecutionError(execCtx, err)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}

	return result, nil
}

// ExecuteBackground runs a command in the background
func (s *executorService) ExecuteBackground(ctx context.Context, req ExecutionRequest) (*BackgroundSession, error) {
	// Generate unique session ID
	sessionID := uuid.New().String()

	// Create context with timeout or cancellation
	execCtx, cancel := s.createExecutionContext(ctx, req.Timeout)

	// Create and configure command
	cmd := s.createCommand(execCtx, req)

	// Capture output
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Create session info
	info := &sessionInfo{
		ID:        sessionID,
		Cmd:       cmd,
		Cancel:    cancel,
		StartedAt: time.Now(),
		Output:    &output,
		Status:    SessionStateRunning,
	}

	// Store session
	s.mu.Lock()
	s.sessions[sessionID] = info
	s.mu.Unlock()

	// Start command in background
	if err := cmd.Start(); err != nil {
		cancel()
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return nil, fmt.Errorf("failed to start background command: %w", err)
	}

	// Monitor command completion in goroutine
	go s.monitorSession(sessionID, cmd, cancel)

	session := &BackgroundSession{
		ID:        sessionID,
		StartedAt: info.StartedAt,
	}

	return session, nil
}

// monitorSession monitors a background session until completion
func (s *executorService) monitorSession(sessionID string, cmd *exec.Cmd, cancel context.CancelFunc) {
	defer cancel()

	err := cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return
	}

	completedAt := time.Now()
	info.CompletedAt = &completedAt

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			info.ExitCode = &exitCode
			info.Status = SessionStateCompleted
		} else {
			info.Status = SessionStateFailed
			info.Error = err.Error()
		}
	} else {
		exitCode := 0
		info.ExitCode = &exitCode
		info.Status = SessionStateCompleted
	}
}

// GetSessionStatus returns the status of a background session
func (s *executorService) GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	status := &SessionStatus{
		ID:          info.ID,
		Status:      info.Status,
		StartedAt:   info.StartedAt,
		CompletedAt: info.CompletedAt,
		ExitCode:    info.ExitCode,
		Error:       info.Error,
	}

	return status, nil
}

// GetSessionOutput retrieves the output of a background session
func (s *executorService) GetSessionOutput(ctx context.Context, sessionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	return info.Output.String(), nil
}

// CancelSession cancels a running background session
func (s *executorService) CancelSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Cancel the session
	if info.Cancel != nil {
		info.Cancel()
	}

	// Update status
	completedAt := time.Now()
	info.CompletedAt = &completedAt
	info.Status = SessionStateCancelled

	return nil
}

// createExecutionContext creates a context with timeout or cancellation
func (s *executorService) createExecutionContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

// createCommand creates and configures an exec.Cmd from ExecutionRequest
func (s *executorService) createCommand(ctx context.Context, req ExecutionRequest) *exec.Cmd {
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)

	// Set working directory
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Set environment variables
	if len(req.Env) > 0 {
		cmd.Env = make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return cmd
}

// handleExecutionError processes command execution errors and returns exit code
func (s *executorService) handleExecutionError(ctx context.Context, err error) (int, error) {
	if err == nil {
		return 0, nil
	}

	// Check if error is due to context cancellation (timeout)
	if ctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("command execution timeout: %w", ctx.Err())
	}

	// Check if error is exec.ExitError (non-zero exit code)
	if exitErr, ok := err.(*exec.ExitError); ok {
		// Non-zero exit code is not an error for us, just captured in result
		return exitErr.ExitCode(), nil
	}

	// Other errors (e.g., command not found)
	return 0, fmt.Errorf("command execution failed: %w", err)
}
