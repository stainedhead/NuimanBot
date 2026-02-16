package domain

import (
	"errors"
	"fmt"
	"time"
)

// MemoryActionType represents the type of memory action.
type MemoryActionType int

const (
	// MemoryActionWriteFile represents a file write operation.
	MemoryActionWriteFile MemoryActionType = iota
	// MemoryActionPersonaUpdate represents a persona file update operation.
	MemoryActionPersonaUpdate
)

// memoryActionTypeNames maps MemoryActionType values to their string representations.
var memoryActionTypeNames = [...]string{"write_file", "persona_update"}

// String returns the string representation of MemoryActionType.
func (t MemoryActionType) String() string {
	if !t.IsValid() {
		return fmt.Sprintf("MemoryActionType(%d)", int(t))
	}
	return memoryActionTypeNames[t]
}

// IsValid checks if the MemoryActionType value is valid.
func (t MemoryActionType) IsValid() bool {
	return t >= MemoryActionWriteFile && t <= MemoryActionPersonaUpdate
}

// Valid operations for a MemoryAction.
var validOperations = map[string]bool{
	"append":  true,
	"replace": true,
	"insert":  true,
}

// MemoryAction represents an explicit memory write operation.
type MemoryAction struct {
	// Unique action ID
	ID string

	// User ID
	UserID string

	// Action type: "write_file", "persona_update"
	Type MemoryActionType

	// Target file path
	FilePath string

	// Operation: "append", "replace", "insert"
	Operation string

	// Content to write
	Content string

	// Whether this action requires user confirmation before executing
	RequiresConfirmation bool

	// Whether the user has confirmed this action
	Confirmed bool

	// Timestamp when the action was created
	CreatedAt time.Time
}

// Validate checks if the memory action is valid.
func (m *MemoryAction) Validate() error {
	if m.ID == "" {
		return errors.New("ID is required")
	}
	if len(m.ID) > 128 {
		return errors.New("ID must be <= 128 characters")
	}

	if m.UserID == "" {
		return errors.New("userID is required")
	}
	if len(m.UserID) > 64 {
		return errors.New("userID must be <= 64 characters")
	}

	if !m.Type.IsValid() {
		return errors.New("invalid memory action type")
	}

	if m.FilePath == "" {
		return errors.New("filePath is required")
	}

	if m.Operation == "" {
		return errors.New("operation is required")
	}
	if !validOperations[m.Operation] {
		return errors.New("operation must be one of: append, replace, insert")
	}

	if m.Content == "" {
		return errors.New("content is required")
	}

	if m.CreatedAt.IsZero() {
		return errors.New("createdAt is required")
	}

	return nil
}

// AwaitingConfirmation returns true if confirmation is required but not yet given.
func (m *MemoryAction) AwaitingConfirmation() bool {
	return m.RequiresConfirmation && !m.Confirmed
}
