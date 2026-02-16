package audit_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nuimanbot/internal/infrastructure/audit"
)

func TestNewAuditLogger_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}
	defer logger.Close()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected audit log file to be created")
	}
}

func TestNewAuditLogger_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "dir", "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}
	defer logger.Close()

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected audit log file to be created in nested directory")
	}
}

func TestNewAuditLogger_EmptyPath(t *testing.T) {
	_, err := audit.NewAuditLogger("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestAuditLogger_Log_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	event := audit.FileAuditEvent{
		ID:        "evt-001",
		UserID:    "user123",
		Action:    "file_write",
		FilePath:  "/data/users/user123/SOUL.md",
		Timestamp: time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC),
		Details: map[string]any{
			"file_type": "SOUL",
			"size":      1024,
		},
	}

	err = logger.Log(event)
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Read and verify JSON
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var decoded audit.FileAuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if decoded.ID != "evt-001" {
		t.Errorf("ID = %q, want %q", decoded.ID, "evt-001")
	}
	if decoded.UserID != "user123" {
		t.Errorf("UserID = %q, want %q", decoded.UserID, "user123")
	}
	if decoded.Action != "file_write" {
		t.Errorf("Action = %q, want %q", decoded.Action, "file_write")
	}
	if decoded.FilePath != "/data/users/user123/SOUL.md" {
		t.Errorf("FilePath = %q, want %q", decoded.FilePath, "/data/users/user123/SOUL.md")
	}
	if decoded.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if decoded.Details["file_type"] != "SOUL" {
		t.Errorf("Details[file_type] = %v, want %q", decoded.Details["file_type"], "SOUL")
	}
}

func TestAuditLogger_Log_MultipleEvents_AppendOnly(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	events := []audit.FileAuditEvent{
		{ID: "evt-001", UserID: "user1", Action: "file_write", FilePath: "/a.md", Timestamp: time.Now()},
		{ID: "evt-002", UserID: "user2", Action: "file_delete", FilePath: "/b.md", Timestamp: time.Now()},
		{ID: "evt-003", UserID: "user1", Action: "file_write", FilePath: "/c.md", Timestamp: time.Now()},
	}

	for _, event := range events {
		if err := logger.Log(event); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	err = logger.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify each line is valid JSON
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		var decoded audit.FileAuditEvent
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Errorf("line %d is not valid JSON: %v", lineCount+1, err)
		}
		lineCount++
	}

	if lineCount != 3 {
		t.Errorf("expected 3 lines, got %d", lineCount)
	}
}

func TestAuditLogger_Log_AppendAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	// First logger writes one event
	logger1, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	err = logger1.Log(audit.FileAuditEvent{
		ID: "evt-001", UserID: "user1", Action: "file_write", FilePath: "/a.md", Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	logger1.Close()

	// Second logger opens the same file and appends
	logger2, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	err = logger2.Log(audit.FileAuditEvent{
		ID: "evt-002", UserID: "user2", Action: "file_delete", FilePath: "/b.md", Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	logger2.Close()

	// Verify both entries exist
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	ids := []string{}
	for scanner.Scan() {
		var decoded audit.FileAuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
			t.Errorf("line %d is not valid JSON: %v", lineCount+1, err)
		}
		ids = append(ids, decoded.ID)
		lineCount++
	}

	if lineCount != 2 {
		t.Errorf("expected 2 lines, got %d", lineCount)
	}
	if len(ids) >= 2 {
		if ids[0] != "evt-001" || ids[1] != "evt-002" {
			t.Errorf("expected IDs [evt-001, evt-002], got %v", ids)
		}
	}
}

func TestAuditLogger_Log_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			event := audit.FileAuditEvent{
				ID:        fmt.Sprintf("evt-%03d", idx),
				UserID:    "user1",
				Action:    "file_write",
				FilePath:  "/data/test.md",
				Timestamp: time.Now(),
			}
			if err := logger.Log(event); err != nil {
				t.Errorf("Log() error in goroutine %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
	logger.Close()

	// Verify all lines are valid JSON
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		var decoded audit.FileAuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
			t.Errorf("line %d is not valid JSON: %v", lineCount+1, err)
		}
		lineCount++
	}

	if lineCount != numGoroutines {
		t.Errorf("expected %d lines, got %d", numGoroutines, lineCount)
	}
}

func TestAuditLogger_Log_NilDetails(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}
	defer logger.Close()

	event := audit.FileAuditEvent{
		ID:        "evt-001",
		UserID:    "user1",
		Action:    "file_write",
		FilePath:  "/test.md",
		Timestamp: time.Now(),
		Details:   nil,
	}

	err = logger.Log(event)
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
}

func TestAuditLogger_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	// Close multiple times should not panic or error
	if err := logger.Close(); err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestAuditLogger_Log_AfterClose(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}

	logger.Close()

	err = logger.Log(audit.FileAuditEvent{
		ID:        "evt-001",
		UserID:    "user1",
		Action:    "file_write",
		FilePath:  "/test.md",
		Timestamp: time.Now(),
	})
	if err == nil {
		t.Error("expected error when logging after close")
	}
}

func TestNewAuditLogger_InvalidPath(t *testing.T) {
	// Use a path that cannot be created (file as directory)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")

	// Create a regular file where we need a directory
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	logPath := filepath.Join(blocker, "subdir", "audit.jsonl")
	_, err := audit.NewAuditLogger(logPath)
	if err == nil {
		t.Error("expected error for invalid nested path")
	}
}

func TestAuditLogger_Log_UnmarshalableDetails(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")

	logger, err := audit.NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditLogger() error: %v", err)
	}
	defer logger.Close()

	// A channel cannot be marshaled to JSON
	event := audit.FileAuditEvent{
		ID:        "evt-bad",
		UserID:    "user1",
		Action:    "file_write",
		FilePath:  "/test.md",
		Timestamp: time.Now(),
		Details: map[string]any{
			"bad_value": make(chan int),
		},
	}

	err = logger.Log(event)
	if err == nil {
		t.Error("expected error for unmarshalable event")
	}
}

func TestFileAuditEvent_JSONRoundTrip(t *testing.T) {
	original := audit.FileAuditEvent{
		ID:        "evt-roundtrip",
		UserID:    "user42",
		Action:    "file_delete",
		FilePath:  "/data/users/user42/RULES.md",
		Timestamp: time.Date(2026, 2, 15, 14, 0, 0, 0, time.UTC),
		Details: map[string]any{
			"reason":    "user_request",
			"file_type": "RULES",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded audit.FileAuditEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.UserID != original.UserID {
		t.Errorf("UserID = %q, want %q", decoded.UserID, original.UserID)
	}
	if decoded.Action != original.Action {
		t.Errorf("Action = %q, want %q", decoded.Action, original.Action)
	}
	if decoded.FilePath != original.FilePath {
		t.Errorf("FilePath = %q, want %q", decoded.FilePath, original.FilePath)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, original.Timestamp)
	}
}
