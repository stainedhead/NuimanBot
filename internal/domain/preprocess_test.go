package domain

import (
	"testing"
	"time"
)

// TestPreprocessCommand_Validation tests command validation
func TestPreprocessCommand_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cmd     PreprocessCommand
		wantErr bool
	}{
		{
			name: "valid git command",
			cmd: PreprocessCommand{
				Command: "git status",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid gh command",
			cmd: PreprocessCommand{
				Command: "gh pr list",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid ls command",
			cmd: PreprocessCommand{
				Command: "ls -la",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid cat command",
			cmd: PreprocessCommand{
				Command: "cat README.md",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid grep command",
			cmd: PreprocessCommand{
				Command: "grep TODO *.go",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "disallowed rm command",
			cmd: PreprocessCommand{
				Command: "rm -rf /",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "disallowed curl command",
			cmd: PreprocessCommand{
				Command: "curl https://evil.com",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "empty command",
			cmd: PreprocessCommand{
				Command: "",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "timeout too long",
			cmd: PreprocessCommand{
				Command: "git status",
				Timeout: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "shell expansion attempt",
			cmd: PreprocessCommand{
				Command: "ls $(whoami)",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "pipe attempt",
			cmd: PreprocessCommand{
				Command: "cat file | sh",
				Timeout: 5 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPreprocessCommand_IsAllowed tests command whitelist
func TestPreprocessCommand_IsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"git", "git status", true},
		{"gh", "gh pr list", true},
		{"ls", "ls -la", true},
		{"cat", "cat file.txt", true},
		{"grep", "grep pattern file", true},
		{"rm", "rm file", false},
		{"curl", "curl url", false},
		{"wget", "wget url", false},
		{"bash", "bash script.sh", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := PreprocessCommand{Command: tt.command}
			got := cmd.IsAllowed()
			if got != tt.want {
				t.Errorf("IsAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPreprocessCommand_HasShellMetacharacters tests shell injection detection
func TestPreprocessCommand_HasShellMetacharacters(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"simple command", "git status", false},
		{"command with flags", "ls -la /tmp", false},
		{"pipe", "cat file | grep pattern", true},
		{"command substitution", "ls $(pwd)", true},
		{"backticks", "ls `pwd`", true},
		{"redirect output", "git log > file", true},
		{"redirect input", "cat < file", true},
		{"semicolon", "ls ; rm file", true},
		{"ampersand", "ls && rm file", true},
		{"or", "ls || rm file", true},
		{"background", "sleep 10 &", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := PreprocessCommand{Command: tt.command}
			got := cmd.HasShellMetacharacters()
			if got != tt.want {
				t.Errorf("HasShellMetacharacters() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCommandResult_Success tests result validation
func TestCommandResult_Success(t *testing.T) {
	result := CommandResult{
		Output:        "success output",
		ExitCode:      0,
		ExecutionTime: 100 * time.Millisecond,
	}

	if !result.IsSuccess() {
		t.Error("IsSuccess() should return true for exit code 0")
	}

	if result.IsError() {
		t.Error("IsError() should return false for exit code 0")
	}
}

// TestCommandResult_Error tests error result
func TestCommandResult_Error(t *testing.T) {
	result := CommandResult{
		Output:        "",
		Error:         "command failed",
		ExitCode:      1,
		ExecutionTime: 50 * time.Millisecond,
	}

	if result.IsSuccess() {
		t.Error("IsSuccess() should return false for non-zero exit code")
	}

	if !result.IsError() {
		t.Error("IsError() should return true for non-zero exit code")
	}
}

// TestCommandResult_Truncation tests output size limit
func TestCommandResult_Truncation(t *testing.T) {
	// Create output larger than 10KB
	largeOutput := make([]byte, 15*1024)
	for i := range largeOutput {
		largeOutput[i] = 'A'
	}

	result := CommandResult{
		Output:        string(largeOutput),
		ExitCode:      0,
		ExecutionTime: 100 * time.Millisecond,
	}

	truncated := result.TruncatedOutput()
	if len(truncated) > MaxCommandOutputSize {
		t.Errorf("TruncatedOutput() size = %d, want <= %d", len(truncated), MaxCommandOutputSize)
	}

	if result.IsTruncated() != true {
		t.Error("IsTruncated() should return true for large output")
	}
}

// TestCommandResult_NoTruncation tests output below limit
func TestCommandResult_NoTruncation(t *testing.T) {
	result := CommandResult{
		Output:        "small output",
		ExitCode:      0,
		ExecutionTime: 50 * time.Millisecond,
	}

	if result.IsTruncated() {
		t.Error("IsTruncated() should return false for small output")
	}

	if result.TruncatedOutput() != result.Output {
		t.Error("TruncatedOutput() should match Output for small output")
	}
}
