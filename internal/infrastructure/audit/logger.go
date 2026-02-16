package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileAuditEvent represents an auditable file operation event.
type FileAuditEvent struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Action    string         `json:"action"`
	FilePath  string         `json:"file_path"`
	Timestamp time.Time      `json:"timestamp"`
	Details   map[string]any `json:"details,omitempty"`
}

// AuditLogger writes file operation audit events as JSON lines to an append-only log file.
type AuditLogger struct {
	mu     sync.Mutex
	file   *os.File
	closed bool
}

// NewAuditLogger creates an AuditLogger that appends JSON-line entries to the given path.
// Parent directories are created if they do not exist.
func NewAuditLogger(path string) (*AuditLogger, error) {
	if path == "" {
		return nil, fmt.Errorf("audit log path cannot be empty")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &AuditLogger{file: file}, nil
}

// Log appends a FileAuditEvent as a single JSON line to the audit log.
func (l *AuditLogger) Log(event FileAuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return fmt.Errorf("audit logger is closed")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	data = append(data, '\n')

	if _, err := l.file.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// Close flushes and closes the underlying log file. Close is idempotent.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	return l.file.Close()
}
