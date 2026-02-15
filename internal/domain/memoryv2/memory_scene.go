package memoryv2

import (
	"fmt"
	"time"
)

// Validation limits for MemoryScene.
const (
	MaxSummaryLength  = 10000
	MaxSummaryTokens  = 2000
)

// MemoryScene represents a topic with consolidated summary.
type MemoryScene struct {
	// Scene is the scene name (primary key).
	Scene string

	// Summary is the consolidated summary of all cells in this scene.
	Summary string

	// TokenCount is the token count of the summary.
	TokenCount int

	// UpdatedAt is the last update timestamp.
	UpdatedAt time.Time
}

// Validate checks if the scene is valid according to business rules.
func (m *MemoryScene) Validate() error {
	if err := m.validateScene(); err != nil {
		return err
	}
	if err := m.validateSummary(); err != nil {
		return err
	}
	if err := m.validateTokenCount(); err != nil {
		return err
	}
	if err := m.validateTimestamp(); err != nil {
		return err
	}
	return nil
}

func (m *MemoryScene) validateScene() error {
	if m.Scene == "" {
		return fmt.Errorf("%w: scene cannot be empty", ErrInvalidInput)
	}
	if !scenePattern.MatchString(m.Scene) {
		return fmt.Errorf("%w: scene must be 3-64 lowercase chars, numbers, or dashes", ErrInvalidInput)
	}
	return nil
}

func (m *MemoryScene) validateSummary() error {
	if m.Summary == "" {
		return fmt.Errorf("%w: summary cannot be empty", ErrInvalidInput)
	}
	if len(m.Summary) > MaxSummaryLength {
		return fmt.Errorf("%w: summary exceeds %d characters", ErrInvalidInput, MaxSummaryLength)
	}
	return nil
}

func (m *MemoryScene) validateTokenCount() error {
	if m.TokenCount <= 0 {
		return fmt.Errorf("%w: token_count must be greater than 0", ErrInvalidInput)
	}
	if m.TokenCount > MaxSummaryTokens {
		return fmt.Errorf("%w: token_count exceeds %d", ErrInvalidInput, MaxSummaryTokens)
	}
	return nil
}

func (m *MemoryScene) validateTimestamp() error {
	if m.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at cannot be zero", ErrInvalidInput)
	}
	return nil
}

// String returns a human-readable representation of the memory scene.
func (m *MemoryScene) String() string {
	return fmt.Sprintf("MemoryScene{Scene: %s, TokenCount: %d, UpdatedAt: %s}",
		m.Scene, m.TokenCount, m.UpdatedAt.Format(time.RFC3339))
}
