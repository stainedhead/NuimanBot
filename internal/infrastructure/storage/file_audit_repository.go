package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileAuditRepository implements AuditRepository using JSONL file storage
type FileAuditRepository struct {
	basePath string
	mu       sync.Mutex // Protects concurrent writes
}

// NewFileAuditRepository creates a new file-based audit repository
func NewFileAuditRepository(basePath string) *FileAuditRepository {
	return &FileAuditRepository{
		basePath: basePath,
	}
}

// getAuditFile returns the path to the audit log file
func (r *FileAuditRepository) getAuditFile() string {
	return filepath.Join(r.basePath, "audit", "audit.jsonl")
}

// Append adds a new audit entry (append-only, concurrent-safe)
func (r *FileAuditRepository) Append(ctx context.Context, entry *domain.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	auditPath := r.getAuditFile()

	// Ensure audit directory exists
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		return fmt.Errorf("failed to create audit directory: %w", err)
	}

	// Open file for append (create if doesn't exist)
	file, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write entry + newline
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return nil
}

// Query retrieves audit entries matching filter
func (r *FileAuditRepository) Query(ctx context.Context, filter domain.AuditFilter) ([]*domain.AuditEvent, error) {
	auditPath := r.getAuditFile()

	// Check if file exists
	if _, err := os.Stat(auditPath); os.IsNotExist(err) {
		// Return empty slice
		return []*domain.AuditEvent{}, nil
	}

	// Open file for reading
	file, err := os.Open(auditPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read all entries (streaming approach)
	var events []*domain.AuditEvent
	scanner := bufio.NewScanner(file)

	// Set a larger buffer for scanner to handle large lines
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		var event domain.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			// Skip malformed entries
			continue
		}

		// Apply filters
		if filter.UserID != "" && event.UserID != filter.UserID {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		if filter.Outcome != "" && event.Outcome != filter.Outcome {
			continue
		}

		events = append(events, &event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read audit file: %w", err)
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(events) {
			return []*domain.AuditEvent{}, nil
		}
		events = events[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(events) {
		events = events[:filter.Limit]
	}

	return events, nil
}
