package repo_search

import (
	"context"
	"fmt"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/executor"
	"nuimanbot/internal/usecase/tool/testutil"
)

func TestRepoSearchSkill_Config(t *testing.T) {
	cfg := domain.ToolConfig{Enabled: true}
	skill := NewRepoSearchSkill(cfg, nil, nil, nil)
	got := skill.Config()
	if !got.Enabled {
		t.Error("Config() should return the configured value")
	}
}

func TestRepoSearchSkill_Execute_EmptyResults(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// ripgrep exit code 1 = no matches
		return &executor.ExecutionResult{ExitCode: 1, Stdout: ""}, nil
	}

	skill := NewRepoSearchSkill(domain.ToolConfig{}, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query": "some_pattern",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}
	// Empty results JSON should contain empty results array
	if result.Output == "" {
		t.Error("Output should not be empty even with no results")
	}
}

func TestRepoSearchSkill_Execute_RipgrepError(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		return nil, fmt.Errorf("rg not found")
	}

	skill := NewRepoSearchSkill(domain.ToolConfig{}, mockExec, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"query": "some_pattern",
	})
	if err == nil {
		t.Error("expected error when ripgrep fails")
	}
}

func TestRepoSearchSkill_Execute_RipgrepNonZeroExit(t *testing.T) {
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		// Exit code 2 = real error
		return &executor.ExecutionResult{ExitCode: 2, Stderr: "rg fatal error"}, nil
	}

	skill := NewRepoSearchSkill(domain.ToolConfig{}, mockExec, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"query": "some_pattern",
	})
	if err == nil {
		t.Error("expected error for ripgrep exit code 2")
	}
}

func TestRepoSearchSkill_ParseLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantFile  string
		wantLine  int
		wantMatch string
	}{
		{
			name:      "valid line",
			line:      "main.go:42:func main() {",
			wantOK:    true,
			wantFile:  "main.go",
			wantLine:  42,
			wantMatch: "func main() {",
		},
		{
			name:   "empty line",
			line:   "",
			wantOK: false,
		},
		{
			name:   "missing line number",
			line:   "main.go:notanum:content",
			wantOK: false,
		},
		{
			name:   "only two parts",
			line:   "main.go:42",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := &RepoSearchSkill{}
			result, ok := skill.parseLine(tt.line)
			if ok != tt.wantOK {
				t.Errorf("parseLine(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if ok {
				if result.File != tt.wantFile {
					t.Errorf("File = %q, want %q", result.File, tt.wantFile)
				}
				if result.Line != tt.wantLine {
					t.Errorf("Line = %d, want %d", result.Line, tt.wantLine)
				}
				if result.Match != tt.wantMatch {
					t.Errorf("Match = %q, want %q", result.Match, tt.wantMatch)
				}
			}
		})
	}
}

func TestRepoSearchSkill_EmptyResults(t *testing.T) {
	skill := &RepoSearchSkill{}
	result := skill.emptyResults()
	if result == "" {
		t.Error("emptyResults() should return non-empty JSON")
	}
	if result != `{"results":[],"total_matches":0,"truncated":false}` {
		t.Errorf("emptyResults() = %q, want empty JSON", result)
	}
}

func TestRepoSearchSkill_GetIntParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		key    string
		want   int
	}{
		{"int value", map[string]any{"key": 42}, "key", 42},
		{"float64 value", map[string]any{"key": float64(42)}, "key", 42},
		{"int64 value", map[string]any{"key": int64(42)}, "key", 42},
		{"missing key", map[string]any{}, "key", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIntParam(tt.params, tt.key)
			if got != tt.want {
				t.Errorf("getIntParam() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRepoSearchSkill_Execute_WithFileTypeAndContextLines(t *testing.T) {
	var capturedArgs []string
	mockExec := testutil.NewMockExecutor()
	mockExec.ExecuteFunc = func(ctx context.Context, req executor.ExecutionRequest) (*executor.ExecutionResult, error) {
		capturedArgs = req.Args
		return &executor.ExecutionResult{
			ExitCode: 0,
			Stdout:   "main.go:1:package main",
		}, nil
	}

	skill := NewRepoSearchSkill(domain.ToolConfig{}, mockExec, nil, nil)

	result, err := skill.Execute(context.Background(), map[string]any{
		"query":         "main",
		"file_type":     "go",
		"context_lines": 3,
		"max_results":   10,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil")
	}

	// Check that file type was passed as arg
	foundType := false
	for _, arg := range capturedArgs {
		if arg == "go" {
			foundType = true
			break
		}
	}
	if !foundType {
		t.Errorf("Expected file type 'go' in args: %v", capturedArgs)
	}
}

func TestRepoSearchSkill_MarshalResults(t *testing.T) {
	skill := &RepoSearchSkill{}
	output := SearchOutput{
		Results:      []SearchResult{{File: "main.go", Line: 1, Match: "package main"}},
		TotalMatches: 1,
		Truncated:    false,
	}
	result := skill.marshalResults(output)
	if result == "" {
		t.Error("marshalResults should return non-empty string")
	}
	if result[0] != '{' {
		t.Error("marshalResults should return valid JSON")
	}
}
