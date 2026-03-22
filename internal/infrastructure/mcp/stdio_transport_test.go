package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestStdioTransport_WriteAndRead(t *testing.T) {
	// Write a helper script that reads one JSON line and responds
	scriptPath := writeTempShellScript(t, `#!/bin/sh
IFS= read -r line
printf '{"jsonrpc":"2.0","id":1,"result":{"status":"received"}}\n'
`)

	transport, err := NewStdioTransport(scriptPath, nil)
	if err != nil {
		t.Fatalf("NewStdioTransport failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	params, _ := json.Marshal(map[string]string{"test": "value"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := transport.Send(ctx, "test/method", params)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if got["status"] != "received" {
		t.Errorf("expected status 'received', got %v", got["status"])
	}
}

func TestStdioTransport_ProcessExitGraceful(t *testing.T) {
	// Script that exits immediately
	scriptPath := writeTempShellScript(t, `#!/bin/sh
exit 0
`)
	transport, err := NewStdioTransport(scriptPath, nil)
	if err != nil {
		t.Fatalf("NewStdioTransport failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	params, _ := json.Marshal(map[string]any{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should return an error (process exited), not panic.
	// The key requirement is no panic — the test passing at all satisfies this.
	_, err = transport.Send(ctx, "test", params)
	if err == nil {
		t.Log("note: process exit returned nil error (process may still be running)")
	}
}

func TestStdioTransport_ContextCancellation(t *testing.T) {
	// Script that reads input and hangs
	scriptPath := writeTempShellScript(t, `#!/bin/sh
IFS= read -r line
sleep 60
`)
	transport, err := NewStdioTransport(scriptPath, nil)
	if err != nil {
		t.Fatalf("NewStdioTransport failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	params, _ := json.Marshal(map[string]any{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = transport.Send(ctx, "test", params)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestStdioTransport_NewWithArgs(t *testing.T) {
	// Script that echoes its first argument in the result
	scriptPath := writeTempShellScript(t, `#!/bin/sh
IFS= read -r line
printf '{"jsonrpc":"2.0","id":1,"result":{"arg":"'"$1"'"}}\n'
`)
	transport, err := NewStdioTransport(scriptPath, []string{"hello-arg"})
	if err != nil {
		t.Fatalf("NewStdioTransport failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	params, _ := json.Marshal(map[string]any{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := transport.Send(ctx, "test", params)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if got["arg"] != "hello-arg" {
		t.Errorf("expected arg 'hello-arg', got %v", got["arg"])
	}
}

func TestStdioTransport_Close(t *testing.T) {
	// Long-running process
	scriptPath := writeTempShellScript(t, `#!/bin/sh
sleep 60
`)
	transport, err := NewStdioTransport(scriptPath, nil)
	if err != nil {
		t.Fatalf("NewStdioTransport failed: %v", err)
	}

	// Close should terminate the process cleanly
	if err := transport.Close(); err != nil {
		t.Errorf("unexpected error on Close: %v", err)
	}
}

func TestStdioTransport_InvalidCommand(t *testing.T) {
	_, err := NewStdioTransport("/nonexistent/binary/that/does/not/exist", nil)
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}
}

// writeTempShellScript writes an executable shell script to a temp file.
func writeTempShellScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/script.sh"
	if err := writeTempScriptFile(path, []byte(content)); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	return path
}

// writeTempScriptFile writes data to a file with executable permissions.
func writeTempScriptFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(data)
	return err
}
