package memoryv2

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Validation limits for MemoryCell.
const (
	MaxContentLength       = 2000
	MaxConversationIDLen   = 128
	MinSceneNameLength     = 3
	MaxSceneNameLength     = 64
)

// scenePattern validates scene names: lowercase letters, numbers, and dashes.
var scenePattern = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

// MemoryCell represents a structured knowledge unit extracted from conversations.
type MemoryCell struct {
	// ID is the unique identifier (UUID).
	ID string

	// ConversationID is the conversation or user ID this cell belongs to.
	ConversationID string

	// Scene is the topic/scene name (e.g., "project-setup", "user-preferences").
	Scene string

	// CellType is the type of knowledge.
	CellType CellType

	// Salience is the importance score (0.0-1.0).
	Salience float64

	// Content is the structured content (JSON or text).
	Content string

	// Source contains the source message IDs (JSON array).
	Source string

	// CreatedAt is the creation timestamp.
	CreatedAt time.Time

	// UpdatedAt is the last update timestamp.
	UpdatedAt time.Time

	// ExpiresAt is the optional expiration time.
	ExpiresAt *time.Time
}

// Validate checks if the memory cell is valid according to business rules.
func (m *MemoryCell) Validate() error {
	if err := m.validateID(); err != nil {
		return err
	}
	if err := m.validateConversationID(); err != nil {
		return err
	}
	if err := m.validateScene(); err != nil {
		return err
	}
	if err := m.validateCellType(); err != nil {
		return err
	}
	if err := m.validateSalience(); err != nil {
		return err
	}
	if err := m.validateContent(); err != nil {
		return err
	}
	if err := m.validateSource(); err != nil {
		return err
	}
	if err := m.validateTimestamps(); err != nil {
		return err
	}
	return nil
}

func (m *MemoryCell) validateID() error {
	if m.ID == "" {
		return fmt.Errorf("%w: id cannot be empty", ErrInvalidInput)
	}
	if _, err := uuid.Parse(m.ID); err != nil {
		return fmt.Errorf("%w: id must be a valid UUID", ErrInvalidInput)
	}
	return nil
}

func (m *MemoryCell) validateConversationID() error {
	if m.ConversationID == "" {
		return fmt.Errorf("%w: conversation_id cannot be empty", ErrInvalidInput)
	}
	if len(m.ConversationID) > MaxConversationIDLen {
		return fmt.Errorf("%w: conversation_id exceeds %d characters", ErrInvalidInput, MaxConversationIDLen)
	}
	return nil
}

func (m *MemoryCell) validateScene() error {
	if m.Scene == "" {
		return fmt.Errorf("%w: scene cannot be empty", ErrInvalidInput)
	}
	if !scenePattern.MatchString(m.Scene) {
		return fmt.Errorf("%w: scene must be 3-64 lowercase chars, numbers, or dashes", ErrInvalidInput)
	}
	return nil
}

func (m *MemoryCell) validateCellType() error {
	if !m.CellType.IsValid() {
		return fmt.Errorf("%w: invalid cell type: %d", ErrInvalidInput, m.CellType)
	}
	return nil
}

func (m *MemoryCell) validateSalience() error {
	if m.Salience < 0.0 || m.Salience > 1.0 {
		return fmt.Errorf("%w: salience must be between 0.0 and 1.0", ErrInvalidInput)
	}
	return nil
}

func (m *MemoryCell) validateContent() error {
	if m.Content == "" {
		return fmt.Errorf("%w: content cannot be empty", ErrInvalidInput)
	}
	if len(m.Content) > MaxContentLength {
		return fmt.Errorf("%w: content exceeds %d characters", ErrInvalidInput, MaxContentLength)
	}
	return nil
}

func (m *MemoryCell) validateSource() error {
	if m.Source == "" {
		return fmt.Errorf("%w: source cannot be empty", ErrInvalidInput)
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(m.Source), &arr); err != nil {
		return fmt.Errorf("%w: source must be a valid JSON array", ErrInvalidInput)
	}
	return nil
}

func (m *MemoryCell) validateTimestamps() error {
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at cannot be zero", ErrInvalidInput)
	}
	if m.UpdatedAt.Before(m.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidInput)
	}
	if m.ExpiresAt != nil && !m.ExpiresAt.After(m.CreatedAt) {
		return fmt.Errorf("%w: expires_at must be after created_at", ErrInvalidInput)
	}
	return nil
}

// IsExpired checks if the cell has expired.
func (m *MemoryCell) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// String returns a human-readable representation of the memory cell.
func (m *MemoryCell) String() string {
	return fmt.Sprintf("MemoryCell{ID: %s, Scene: %s, Type: %s, Salience: %.2f}",
		m.ID, m.Scene, m.CellType.String(), m.Salience)
}
