package buzzsend_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/tools/buzzsend"
)

func TestBuzzSend_Name(t *testing.T) {
	tool := buzzsend.New()
	if tool.Name() != "buzz_send_message" {
		t.Errorf("expected name 'buzz_send_message', got %q", tool.Name())
	}
}

func TestBuzzSend_RequiredPermissions_None(t *testing.T) {
	// Deliberately empty -- see the tool's doc comment. A non-empty result
	// here would silently break replies for every non-admin Buzz contact.
	tool := buzzsend.New()
	perms := tool.RequiredPermissions()
	if len(perms) != 0 {
		t.Errorf("expected no required permissions, got %v", perms)
	}
}

func TestBuzzSend_Execute_MissingChannel(t *testing.T) {
	tool := buzzsend.New()
	result, err := tool.Execute(context.Background(), map[string]any{"content": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected an error result for a missing channel parameter")
	}
}

func TestBuzzSend_Execute_MissingContent(t *testing.T) {
	tool := buzzsend.New()
	result, err := tool.Execute(context.Background(), map[string]any{"channel": "abc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected an error result for a missing content parameter")
	}
}

// fakeBuzzScript writes an executable shell script named "buzz" to a temp
// directory and prepends that directory to PATH for the duration of the
// test, standing in for the real CLI. body is the script's shell body.
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

func TestBuzzSend_Execute_Success(t *testing.T) {
	var capturedArgsFile = filepath.Join(t.TempDir(), "captured-args")
	fakeBuzzScript(t, fmt.Sprintf(`echo "$@" > %q
echo '{"event_id":"abc","accepted":true}'
exit 0`, capturedArgsFile))

	tool := buzzsend.New()
	result, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
		"content": "hello there",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "accepted") {
		t.Errorf("expected CLI output to be surfaced, got %q", result.Output)
	}

	captured, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatalf("failed to read captured args: %v", err)
	}
	args := string(captured)
	if !strings.Contains(args, "messages send") {
		t.Errorf("expected 'messages send' subcommand, got args: %q", args)
	}
	if !strings.Contains(args, "--channel chan-uuid-1") {
		t.Errorf("expected --channel chan-uuid-1, got args: %q", args)
	}
	if !strings.Contains(args, "--content hello there") {
		t.Errorf("expected single-line content passed directly, got args: %q", args)
	}
}

func TestBuzzSend_Execute_MultilineContentUsesStdin(t *testing.T) {
	fakeBuzzScript(t, `
content="$(cat)"
echo "STDIN:$content"
exit 0`)

	tool := buzzsend.New()
	result, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
		"content": "line one\nline two",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "STDIN:line one\nline two") {
		t.Errorf("expected multiline content delivered via stdin verbatim, got %q", result.Output)
	}
}

func TestBuzzSend_Execute_ReplyToAndMentions(t *testing.T) {
	var capturedArgsFile = filepath.Join(t.TempDir(), "captured-args")
	fakeBuzzScript(t, fmt.Sprintf(`echo "$@" > %q
exit 0`, capturedArgsFile))

	tool := buzzsend.New()
	_, err := tool.Execute(context.Background(), map[string]any{
		"channel":  "chan-uuid-1",
		"content":  "hi",
		"reply_to": "event-abc",
		"mentions": []any{"npub1xyz", "pubkeyhex"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	captured, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatalf("failed to read captured args: %v", err)
	}
	args := string(captured)
	if !strings.Contains(args, "--reply-to event-abc") {
		t.Errorf("expected --reply-to event-abc, got args: %q", args)
	}
	if !strings.Contains(args, "--mention npub1xyz") || !strings.Contains(args, "--mention pubkeyhex") {
		t.Errorf("expected both --mention flags, got args: %q", args)
	}
}

func TestBuzzSend_Execute_CLIFailureSurfacesError(t *testing.T) {
	fakeBuzzScript(t, `echo '{"error":"auth_error"}' >&2
exit 3`)

	tool := buzzsend.New()
	result, err := tool.Execute(context.Background(), map[string]any{
		"channel": "chan-uuid-1",
		"content": "hi",
	})
	if err != nil {
		t.Fatalf("unexpected Go error (CLI failures should surface via ExecutionResult.Error): %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected ExecutionResult.Error to be set for a nonzero CLI exit")
	}
	if !strings.Contains(result.Error, "auth_error") {
		t.Errorf("expected the CLI's stderr surfaced in the error, got %q", result.Error)
	}
}

// Compile-time interface check.
var _ domain.Tool = (*buzzsend.Tool)(nil)
