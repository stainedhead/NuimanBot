package domain

import (
	"fmt"
	"time"
)

// MemoryScope defines the scope of skill memory
type MemoryScope string

const (
	MemoryScopeSkill   MemoryScope = "skill"   // Per-skill memory
	MemoryScopeUser    MemoryScope = "user"    // Per-user memory
	MemoryScopeGlobal  MemoryScope = "global"  // Global memory
	MemoryScopeSession MemoryScope = "session" // Per-session memory
)

// SkillMemory represents persistent memory for a skill
type SkillMemory struct {
	// ID is the unique identifier
	ID string

	// SkillName is the skill this memory belongs to
	SkillName string

	// Scope is the memory scope
	Scope MemoryScope

	// Key is the memory key
	Key string

	// Value is the stored value (JSON serializable)
	Value string

	// CreatedAt is when the memory was created
	CreatedAt time.Time

	// UpdatedAt is when the memory was last updated
	UpdatedAt time.Time

	// ExpiresAt is when the memory expires (optional)
	ExpiresAt *time.Time

	// Metadata for additional context
	Metadata map[string]string
}

// Validate checks if the memory entry is valid
func (m *SkillMemory) Validate() error {
	if m.SkillName == "" {
		return fmt.Errorf("skill name is required")
	}

	if m.Key == "" {
		return fmt.Errorf("key is required")
	}

	if m.Scope == "" {
		m.Scope = MemoryScopeSkill
	}

	validScopes := map[MemoryScope]bool{
		MemoryScopeSkill:   true,
		MemoryScopeUser:    true,
		MemoryScopeGlobal:  true,
		MemoryScopeSession: true,
	}

	if !validScopes[m.Scope] {
		return fmt.Errorf("invalid scope: %s", m.Scope)
	}

	return nil
}

// IsExpired checks if the memory has expired
func (m *SkillMemory) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*m.ExpiresAt)
}

// MemoryQuery defines interface for querying skill memory
type MemoryQuery interface {
	// Get retrieves a memory value
	Get(skillName, key string, scope MemoryScope) (*SkillMemory, error)

	// Set stores a memory value
	Set(memory *SkillMemory) error

	// Delete removes a memory value
	Delete(skillName, key string, scope MemoryScope) error

	// List lists all memory for a skill
	List(skillName string, scope MemoryScope) ([]*SkillMemory, error)

	// Cleanup removes expired memory
	Cleanup() error
}
