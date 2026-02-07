package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutorService_Execute_Success(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "echo",
		Args:    []string{"hello", "world"},
		Timeout: 5 * time.Second,
	}

	result, err := svc.Execute(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "hello world")
	assert.Empty(t, result.Stderr)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestExecutorService_Execute_Timeout(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sleep",
		Args:    []string{"10"},
		Timeout: 100 * time.Millisecond,
	}

	result, err := svc.Execute(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestExecutorService_Execute_CommandNotFound(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "nonexistentcommand12345",
		Args:    []string{},
		Timeout: 5 * time.Second,
	}

	result, err := svc.Execute(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "executable file not found")
}

func TestExecutorService_Execute_NonZeroExit(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sh",
		Args:    []string{"-c", "exit 42"},
		Timeout: 5 * time.Second,
	}

	result, err := svc.Execute(ctx, req)

	require.NoError(t, err) // Exit code != 0 is not an error, just captured in result
	assert.Equal(t, 42, result.ExitCode)
}

func TestExecutorService_Execute_WorkingDirectory(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command:    "pwd",
		Args:       []string{},
		WorkingDir: "/tmp",
		Timeout:    5 * time.Second,
	}

	result, err := svc.Execute(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, strings.TrimSpace(result.Stdout), "/tmp")
}

func TestExecutorService_Execute_Environment(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sh",
		Args:    []string{"-c", "echo $TEST_VAR"},
		Env:     map[string]string{"TEST_VAR": "test_value"},
		Timeout: 5 * time.Second,
	}

	result, err := svc.Execute(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "test_value")
}

func TestExecutorService_ExecuteBackground_CreateSession(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sleep",
		Args:    []string{"1"},
		Timeout: 10 * time.Second,
	}

	session, err := svc.ExecuteBackground(ctx, req)

	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.False(t, session.StartedAt.IsZero())
}

func TestExecutorService_GetSessionStatus_Running(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sleep",
		Args:    []string{"2"},
		Timeout: 10 * time.Second,
	}

	session, err := svc.ExecuteBackground(ctx, req)
	require.NoError(t, err)

	status, err := svc.GetSessionStatus(ctx, session.ID)

	require.NoError(t, err)
	assert.Equal(t, session.ID, status.ID)
	assert.Equal(t, SessionStateRunning, status.Status)
	assert.Nil(t, status.CompletedAt)
	assert.Nil(t, status.ExitCode)
}

func TestExecutorService_GetSessionStatus_Completed(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "echo",
		Args:    []string{"done"},
		Timeout: 5 * time.Second,
	}

	session, err := svc.ExecuteBackground(ctx, req)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	status, err := svc.GetSessionStatus(ctx, session.ID)

	require.NoError(t, err)
	assert.Equal(t, SessionStateCompleted, status.Status)
	assert.NotNil(t, status.CompletedAt)
	assert.NotNil(t, status.ExitCode)
	assert.Equal(t, 0, *status.ExitCode)
}

func TestExecutorService_GetSessionOutput(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "echo",
		Args:    []string{"background output"},
		Timeout: 5 * time.Second,
	}

	session, err := svc.ExecuteBackground(ctx, req)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(500 * time.Millisecond)

	output, err := svc.GetSessionOutput(ctx, session.ID)

	require.NoError(t, err)
	assert.Contains(t, output, "background output")
}

func TestExecutorService_CancelSession(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	req := ExecutionRequest{
		Command: "sleep",
		Args:    []string{"30"},
		Timeout: 60 * time.Second,
	}

	session, err := svc.ExecuteBackground(ctx, req)
	require.NoError(t, err)

	// Cancel immediately
	err = svc.CancelSession(ctx, session.ID)
	require.NoError(t, err)

	// Check status
	status, err := svc.GetSessionStatus(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, SessionStateCancelled, status.Status)
}

func TestExecutorService_GetSessionStatus_NotFound(t *testing.T) {
	svc := NewExecutorService()
	ctx := context.Background()

	status, err := svc.GetSessionStatus(ctx, "nonexistent-session-id")

	require.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "session not found")
}
