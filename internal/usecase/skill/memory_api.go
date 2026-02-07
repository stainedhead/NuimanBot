package skill

import (
	"encoding/json"
	"fmt"

	"nuimanbot/internal/domain"
)

// MemoryAPI provides memory access for skills
type MemoryAPI struct {
	storage domain.MemoryQuery
}

// NewMemoryAPI creates a new memory API
func NewMemoryAPI(storage domain.MemoryQuery) *MemoryAPI {
	return &MemoryAPI{storage: storage}
}

// Remember stores a value in skill memory
func (api *MemoryAPI) Remember(skillName, key string, value interface{}, scope domain.MemoryScope) error {
	// Serialize value to JSON
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	memory := &domain.SkillMemory{
		SkillName: skillName,
		Key:       key,
		Value:     string(valueJSON),
		Scope:     scope,
	}

	return api.storage.Set(memory)
}

// Recall retrieves a value from skill memory
func (api *MemoryAPI) Recall(skillName, key string, scope domain.MemoryScope, dest interface{}) error {
	memory, err := api.storage.Get(skillName, key, scope)
	if err != nil {
		return err
	}

	if memory.IsExpired() {
		api.storage.Delete(skillName, key, scope)
		return fmt.Errorf("memory expired")
	}

	// Deserialize JSON to dest
	if err := json.Unmarshal([]byte(memory.Value), dest); err != nil {
		return fmt.Errorf("failed to deserialize value: %w", err)
	}

	return nil
}

// Forget removes a value from skill memory
func (api *MemoryAPI) Forget(skillName, key string, scope domain.MemoryScope) error {
	return api.storage.Delete(skillName, key, scope)
}

// ListMemories lists all memory for a skill
func (api *MemoryAPI) ListMemories(skillName string, scope domain.MemoryScope) ([]*domain.SkillMemory, error) {
	memories, err := api.storage.List(skillName, scope)
	if err != nil {
		return nil, err
	}

	// Filter out expired memories
	var valid []*domain.SkillMemory
	for _, m := range memories {
		if !m.IsExpired() {
			valid = append(valid, m)
		}
	}

	return valid, nil
}
