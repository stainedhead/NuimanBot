package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// AtomicFileWriter provides atomic file write operations using temp file + rename pattern.
// This ensures that file reads never see partially written content.
type AtomicFileWriter struct{}

// NewAtomicFileWriter creates a new atomic file writer.
func NewAtomicFileWriter() *AtomicFileWriter {
	return &AtomicFileWriter{}
}

// Write writes content to targetPath atomically.
// It creates a temporary file, writes content, then renames to target.
// Parent directories are created if they don't exist.
func (w *AtomicFileWriter) Write(targetPath string, content []byte, perm os.FileMode) error {
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create temporary file in the same directory
	// This ensures rename is atomic (same filesystem)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	// Write content to temp file
	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	// Atomically rename temp file to target
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Clear tmpPath so defer doesn't remove the renamed file
	tmpPath = ""

	return nil
}

// FileLock provides file-based locking mechanism using flock.
// Used to prevent concurrent modifications to critical files like users.json and bots.json.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock.
func NewFileLock(path string) *FileLock {
	return &FileLock{
		path: path,
	}
}

// Lock acquires an exclusive lock on the file.
// Blocks until the lock is acquired.
func (l *FileLock) Lock() error {
	// Create parent directory if needed
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create lock file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Acquire exclusive lock (blocking)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	l.file = file
	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns true if lock was acquired, false if already locked.
func (l *FileLock) TryLock() (bool, error) {
	// Create parent directory if needed
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Open or create lock file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil // Lock already held by another process
		}
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	l.file = file
	return true, nil
}

// Unlock releases the file lock.
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil // Not locked
	}

	// Release lock
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// Close file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	l.file = nil
	return nil
}
