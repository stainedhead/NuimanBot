package persona

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"nuimanbot/internal/domain"
)

const actionMemoryWrite = "memory_write"

// validWriteOperations defines the allowed write operations.
var validWriteOperations = map[string]bool{
	"append":  true,
	"replace": true,
}

// AuditLogger logs audit events for memory operations.
type AuditLogger interface {
	Log(entry AuditEntry) error
}

// AuditEntry represents an auditable memory operation event.
type AuditEntry struct {
	ID        string
	UserID    string
	Action    string
	FilePath  string
	Timestamp time.Time
	Details   map[string]any
}

// RulesChecker checks whether actions are permitted by user rules.
type RulesChecker interface {
	// IsAllowed checks if an action is allowed for the user.
	// Returns false and a reason string if blocked.
	IsAllowed(ctx context.Context, userID, action string) (bool, string, error)
	// NeedsConfirmation checks if an action requires user confirmation.
	NeedsConfirmation(ctx context.Context, userID, action string) (bool, error)
}

// MemoryWriter handles explicit memory write operations to persona files.
type MemoryWriter struct {
	repo     domain.PersonaFileRepository
	auditor  AuditLogger
	enforcer RulesChecker
}

// WriteInput represents input for a memory write operation.
type WriteInput struct {
	UserID    string
	FilePath  string // Persona filename: "SOUL.md", "USER.md", or "RULES.md"
	Content   string
	Operation string // "append" or "replace"
}

// WriteOutput represents the result of a memory write operation.
type WriteOutput struct {
	Success              bool
	RequiresConfirmation bool
	ConfirmationID       string
}

// NewMemoryWriter creates a new MemoryWriter service.
func NewMemoryWriter(repo domain.PersonaFileRepository, auditor AuditLogger, enforcer RulesChecker) *MemoryWriter {
	return &MemoryWriter{
		repo:     repo,
		auditor:  auditor,
		enforcer: enforcer,
	}
}

// Write performs a memory write operation to a persona file.
func (w *MemoryWriter) Write(ctx context.Context, input WriteInput) (*WriteOutput, error) {
	if err := validateWriteInput(input); err != nil {
		return nil, fmt.Errorf("invalid write input: %w", err)
	}

	fileType, err := resolveFileType(input.FilePath)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	allowed, reason, err := w.enforcer.IsAllowed(ctx, input.UserID, actionMemoryWrite)
	if err != nil {
		return nil, fmt.Errorf("rules check failed: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("action blocked by rules: %s", reason)
	}

	needsConfirm, err := w.enforcer.NeedsConfirmation(ctx, input.UserID, actionMemoryWrite)
	if err != nil {
		return nil, fmt.Errorf("confirmation check failed: %w", err)
	}
	if needsConfirm {
		confirmID := fmt.Sprintf("confirm_%s_%s_%d", input.UserID, fileType.String(), time.Now().UnixNano())
		return &WriteOutput{
			RequiresConfirmation: true,
			ConfirmationID:       confirmID,
		}, nil
	}

	content, err := w.buildContent(ctx, input, fileType)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	file := &domain.PersonaFile{
		UserID:     input.UserID,
		Type:       fileType,
		Path:       input.FilePath,
		Content:    content,
		ModifiedAt: now,
		SizeBytes:  int64(len(content)),
	}
	if err := w.repo.Save(ctx, file); err != nil {
		return nil, fmt.Errorf("failed to save persona file: %w", err)
	}

	// Audit logging is best-effort; failures do not block the write.
	_ = w.auditor.Log(AuditEntry{
		UserID:    input.UserID,
		Action:    "persona_" + input.Operation,
		FilePath:  input.FilePath,
		Timestamp: now,
		Details: map[string]any{
			"operation": input.Operation,
			"file_type": fileType.String(),
			"size":      len(content),
		},
	})

	return &WriteOutput{Success: true}, nil
}

// validateWriteInput checks that all required fields are present and valid.
func validateWriteInput(input WriteInput) error {
	if input.UserID == "" {
		return errors.New("userID is required")
	}
	if input.FilePath == "" {
		return errors.New("filePath is required")
	}
	if input.Content == "" {
		return errors.New("content is required")
	}
	if !validWriteOperations[input.Operation] {
		return errors.New("operation must be 'append' or 'replace'")
	}
	return nil
}

// resolveFileType maps a filename to a PersonaFileType.
func resolveFileType(filePath string) (domain.PersonaFileType, error) {
	filename := filepath.Base(filePath)
	switch filename {
	case domain.PersonaFilenameSOUL:
		return domain.PersonaFileSOUL, nil
	case domain.PersonaFilenameUSER:
		return domain.PersonaFileUSER, nil
	case domain.PersonaFilenameRULES:
		return domain.PersonaFileRULES, nil
	default:
		return 0, fmt.Errorf("unsupported persona file: %s", filename)
	}
}

// buildContent constructs the final file content based on the operation type.
func (w *MemoryWriter) buildContent(ctx context.Context, input WriteInput, fileType domain.PersonaFileType) (string, error) {
	if input.Operation == "replace" {
		return input.Content, nil
	}

	existing, err := w.repo.Get(ctx, input.UserID, fileType)
	if err != nil {
		if errors.Is(err, domain.ErrPersonaFileNotFound) {
			return input.Content, nil
		}
		return "", fmt.Errorf("failed to read existing file: %w", err)
	}
	if existing.IsEmpty() {
		return input.Content, nil
	}
	return existing.Content + "\n" + input.Content, nil
}
