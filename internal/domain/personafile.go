package domain

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// PersonaFileType represents the type of persona file.
type PersonaFileType int

const (
	// PersonaFileSOUL represents a SOUL.md persona file.
	PersonaFileSOUL PersonaFileType = iota
	// PersonaFileUSER represents a USER.md persona file.
	PersonaFileUSER
	// PersonaFileRULES represents a RULES.md persona file.
	PersonaFileRULES
)

// personaFileTypeNames maps PersonaFileType values to their string representations.
var personaFileTypeNames = [...]string{"SOUL", "USER", "RULES"}

// personaFileTypeFilenames maps PersonaFileType values to their filenames.
var personaFileTypeFilenames = [...]string{"SOUL.md", "USER.md", "RULES.md"}

// String returns the string representation of a PersonaFileType.
func (t PersonaFileType) String() string {
	if !t.IsValid() {
		return fmt.Sprintf("PersonaFileType(%d)", int(t))
	}
	return personaFileTypeNames[t]
}

// IsValid checks if the PersonaFileType value is valid.
func (t PersonaFileType) IsValid() bool {
	return t >= PersonaFileSOUL && t <= PersonaFileRULES
}

// Filename returns the canonical filename for a PersonaFileType.
func (t PersonaFileType) Filename() string {
	if !t.IsValid() {
		return ""
	}
	return personaFileTypeFilenames[t]
}

// Persona file constants.
const (
	// MaxPersonaFileSize is the maximum allowed content size (100KB).
	MaxPersonaFileSize = 100 * 1024

	// PersonaFilenameSOUL is the filename for SOUL persona files.
	PersonaFilenameSOUL = "SOUL.md"
	// PersonaFilenameUSER is the filename for USER persona files.
	PersonaFilenameUSER = "USER.md"
	// PersonaFilenameRULES is the filename for RULES persona files.
	PersonaFilenameRULES = "RULES.md"
)

// Persona file errors.
var (
	// ErrPersonaFileNotFound is returned when a persona file does not exist.
	ErrPersonaFileNotFound = errors.New("persona file not found")
	// ErrInvalidPersonaFileType is returned when a persona file type is invalid.
	ErrInvalidPersonaFileType = errors.New("invalid persona file type")
	// ErrPathTraversal is returned when a path traversal attempt is detected.
	ErrPathTraversal = errors.New("path traversal detected")
)

// PersonaFile represents a user's persona configuration file.
type PersonaFile struct {
	// UserID this file belongs to.
	UserID string

	// Type is the persona file type: SOUL, USER, or RULES.
	Type PersonaFileType

	// Path is the absolute file path.
	Path string

	// Content is the file content (Markdown).
	Content string

	// ModifiedAt is the last modified timestamp.
	ModifiedAt time.Time

	// SizeBytes is the file size in bytes.
	SizeBytes int64
}

// Validate checks if the persona file is valid.
func (p *PersonaFile) Validate() error {
	if p.UserID == "" {
		return errors.New("userID is required")
	}
	if len(p.UserID) > 64 {
		return errors.New("userID must be <= 64 characters")
	}
	if !p.Type.IsValid() {
		return errors.New("invalid persona file type")
	}
	if p.Path == "" {
		return errors.New("path is required")
	}
	if !filepath.IsAbs(p.Path) {
		return errors.New("path must be absolute")
	}
	if !utf8.ValidString(p.Content) {
		return errors.New("content must be valid UTF-8")
	}
	if len(p.Content) > MaxPersonaFileSize {
		return errors.New("content must be <= 100KB")
	}
	if p.ModifiedAt.IsZero() {
		return errors.New("modifiedAt is required")
	}
	return nil
}

// IsEmpty returns true if the content is empty or whitespace only.
func (p *PersonaFile) IsEmpty() bool {
	return strings.TrimSpace(p.Content) == ""
}

// TokenCount estimates the token count for this file's content.
// Uses a simple heuristic of ~4 characters per token, which is a
// common approximation for English text with LLMs.
func (p *PersonaFile) TokenCount() int {
	trimmed := strings.TrimSpace(p.Content)
	if trimmed == "" {
		return 0
	}
	return (len(trimmed) + 3) / 4
}

// PersonaFileRepository defines operations for persona file storage.
type PersonaFileRepository interface {
	// Get retrieves a persona file by user ID and type.
	Get(ctx context.Context, userID string, fileType PersonaFileType) (*PersonaFile, error)

	// Save creates or updates a persona file.
	Save(ctx context.Context, file *PersonaFile) error

	// Delete removes a persona file. Returns nil if file doesn't exist (idempotent).
	Delete(ctx context.Context, userID string, fileType PersonaFileType) error

	// List returns all persona files for a user. Returns empty slice if none found.
	List(ctx context.Context, userID string) ([]*PersonaFile, error)
}
