package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAtomicFileWriter_Write tests atomic file writing
func TestAtomicFileWriter_Write(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.json")

	content := []byte(`{"test": "data"}`)

	// Write file atomically
	writer := NewAtomicFileWriter()
	err := writer.Write(targetPath, content, 0644)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}

	// Verify file content
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("content mismatch: got %s, want %s", data, content)
	}
}

// TestAtomicFileWriter_WriteCreatesDirectory tests that write creates parent directories
func TestAtomicFileWriter_WriteCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "subdir", "test.json")

	content := []byte(`{"test": "data"}`)

	writer := NewAtomicFileWriter()
	err := writer.Write(targetPath, content, 0644)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}
}

// TestAtomicFileWriter_WriteOverwrite tests overwriting an existing file
func TestAtomicFileWriter_WriteOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.json")

	// Write initial content
	initialContent := []byte(`{"test": "old"}`)
	err := os.WriteFile(targetPath, initialContent, 0644)
	if err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	// Overwrite with new content
	newContent := []byte(`{"test": "new"}`)
	writer := NewAtomicFileWriter()
	err = writer.Write(targetPath, newContent, 0644)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify new content
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(newContent) {
		t.Errorf("content mismatch: got %s, want %s", data, newContent)
	}
}

// TestFileLock tests file locking mechanism
func TestFileLock_LockUnlock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	locker := NewFileLock(lockPath)

	// Lock file
	err := locker.Lock()
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	// Unlock file
	err = locker.Unlock()
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

// TestFileLock_TryLock tests non-blocking lock attempt
func TestFileLock_TryLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	locker := NewFileLock(lockPath)

	// TryLock should succeed
	acquired, err := locker.TryLock()
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}

	// Clean up
	err = locker.Unlock()
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}
