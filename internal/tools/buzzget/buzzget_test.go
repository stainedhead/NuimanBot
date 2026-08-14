package buzzget_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/tools/buzzget"
)

func TestBuzzGet_Name(t *testing.T) {
	tool := buzzget.New()
	if tool.Name() != "buzz_get_messages" {
		t.Errorf("expected name 'buzz_get_messages', got %q", tool.Name())
	}
}

func TestBuzzGet_RequiredPermissions_None(t *testing.T) {
	// Deliberately empty, matching buzzsend -- any Buzz contact who can
	// reach Iman at all should be able to prompt it to recall context,
	// including a first-message Guest.
	tool := buzzget.New()
	perms := tool.RequiredPermissions()
	if len(perms) != 0 {
		t.Errorf("expected no required permissions, got %v", perms)
	}
}

func TestBuzzGet_Execute_MissingChannel(t *testing.T) {
	tool := buzzget.New()
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected an error result for a missing channel parameter")
	}
}

// fakeBuzzScript writes an executable shell script named "buzz" to a temp
// directory and prepends that directory to PATH for the duration of the
// test, standing in for the real CLI.
func fakeBuzzScript(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake buzz script is a shell script; skip on windows")
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "buzz")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake buzz script: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBuzzGet_Execute_Success(t *testing.T) {
	capturedArgsFile := filepath.Join(t.TempDir(), "captured-args")
	fakeBuzzScript(t, fmt.Sprintf(`echo "$@" > %q
echo '{"messages":[{"content":"hi"}]}'
exit 0`, capturedArgsFile))

	tool := buzzget.New()
	result, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "messages") {
		t.Errorf("expected CLI output to be surfaced, got %q", result.Output)
	}

	captured, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatalf("failed to read captured args: %v", err)
	}
	args := string(captured)
	if !strings.Contains(args, "messages get") {
		t.Errorf("expected 'messages get' subcommand, got args: %q", args)
	}
	if !strings.Contains(args, "--channel chan-uuid-1") {
		t.Errorf("expected --channel chan-uuid-1, got args: %q", args)
	}
}

func TestBuzzGet_Execute_OptionalParams(t *testing.T) {
	capturedArgsFile := filepath.Join(t.TempDir(), "captured-args")
	fakeBuzzScript(t, fmt.Sprintf(`echo "$@" > %q
exit 0`, capturedArgsFile))

	tool := buzzget.New()
	_, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
		"limit":   float64(50),
		"before":  float64(1700000000),
		"since":   float64(1600000000),
		"kinds":   "1,1984",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	captured, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatalf("failed to read captured args: %v", err)
	}
	args := string(captured)
	for _, want := range []string{
		"--limit 50",
		"--before 1700000000",
		"--since 1600000000",
		"--kinds 1,1984",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("expected args to contain %q, got: %q", want, args)
		}
	}
}

func TestBuzzGet_Execute_CLIFailureSurfacesError(t *testing.T) {
	fakeBuzzScript(t, `echo '{"error":"channel_not_found"}' >&2
exit 3`)

	tool := buzzget.New()
	result, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
	})
	if err != nil {
		t.Fatalf("unexpected Go error (CLI failures should surface via ExecutionResult.Error): %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected ExecutionResult.Error to be set for a nonzero CLI exit")
	}
	if !strings.Contains(result.Error, "channel_not_found") {
		t.Errorf("expected the CLI's stderr surfaced in the error, got %q", result.Error)
	}
}

// Compile-time interface check.
var _ domain.Tool = (*buzzget.Tool)(nil)
