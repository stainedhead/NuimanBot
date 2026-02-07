package preprocess

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// CommandSandbox executes preprocessing commands in a sandboxed environment
type CommandSandbox struct {
	// workingDir is the default working directory
	workingDir string
}

// NewCommandSandbox creates a new command sandbox
func NewCommandSandbox() *CommandSandbox {
	return &CommandSandbox{
		workingDir: ".",
	}
}

// Execute runs a preprocessing command with security constraints
func (s *CommandSandbox) Execute(ctx context.Context, cmd domain.PreprocessCommand) (*domain.CommandResult, error) {
	// Validate command first
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Set working directory
	workingDir := s.workingDir
	if cmd.WorkingDir != "" {
		workingDir = cmd.WorkingDir
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, cmd.Timeout)
	defer cancel()

	// Parse command into parts (first word is command, rest are args)
	parts := strings.Fields(cmd.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	baseCmd := parts[0]
	args := parts[1:]

	// Create command
	execCmd := exec.CommandContext(execCtx, baseCmd, args...)
	execCmd.Dir = workingDir

	// Capture output
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Execute
	start := time.Now()
	err := execCmd.Run()
	executionTime := time.Since(start)

	// Build result
	result := &domain.CommandResult{
		ExecutionTime: executionTime,
	}

	// Capture output (limit to max size)
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if len(output) > domain.MaxCommandOutputSize {
		result.Output = output[:domain.MaxCommandOutputSize]
		result.Truncated = true
	} else {
		result.Output = output
	}

	// Determine exit code and error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Error = "command timed out"
		} else if execCtx.Err() == context.Canceled {
			result.ExitCode = -1
			result.Error = "command cancelled"
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = err.Error()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

// SetWorkingDir sets the default working directory for commands
func (s *CommandSandbox) SetWorkingDir(dir string) {
	s.workingDir = dir
}
