package notes_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/skills/notes"
)

// mockNotesRepo is a mock notes repository for testing
type mockNotesRepo struct {
	notes map[string]*domain.Note
}

func newMockNotesRepo() *mockNotesRepo {
	return &mockNotesRepo{notes: make(map[string]*domain.Note)}
}

func (m *mockNotesRepo) Create(ctx context.Context, note *domain.Note) error {
	m.notes[note.ID] = note
	return nil
}

func (m *mockNotesRepo) GetByID(ctx context.Context, noteID string) (*domain.Note, error) {
	note, ok := m.notes[noteID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return note, nil
}

func (m *mockNotesRepo) List(ctx context.Context, userID string) ([]*domain.Note, error) {
	var result []*domain.Note
	for _, note := range m.notes {
		if note.UserID == userID {
			result = append(result, note)
		}
	}
	return result, nil
}

func (m *mockNotesRepo) Update(ctx context.Context, note *domain.Note) error {
	if _, ok := m.notes[note.ID]; !ok {
		return domain.ErrNotFound
	}
	m.notes[note.ID] = note
	return nil
}

func (m *mockNotesRepo) Delete(ctx context.Context, noteID string) error {
	if _, ok := m.notes[noteID]; !ok {
		return domain.ErrNotFound
	}
	delete(m.notes, noteID)
	return nil
}

func TestNotesSkill_Metadata(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	if skill.Name() != "notes" {
		t.Errorf("Expected name 'notes', got '%s'", skill.Name())
	}

	if skill.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := skill.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestNotesSkill_Execute_Create(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	// Add user_id to context
	ctx := context.WithValue(context.Background(), "user_id", "user1")

	result, err := skill.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Test Note",
		"content":   "Test content",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestNotesSkill_Execute_MissingOperation(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1")

	result, err := skill.Execute(ctx, map[string]any{
		"title": "Test Note",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing operation")
	}
}

func TestNotesSkill_Execute_InvalidOperation(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1")

	result, err := skill.Execute(ctx, map[string]any{
		"operation": "invalid",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for invalid operation")
	}
}

func TestNotesSkill_RequiredPermissions(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	perms := skill.RequiredPermissions()
	if len(perms) != 1 {
		t.Errorf("Expected 1 permission, got %d", len(perms))
	}
	if len(perms) > 0 && perms[0] != domain.PermissionWrite {
		t.Errorf("Expected PermissionWrite, got %v", perms[0])
	}
}

func TestNotesSkill_Config(t *testing.T) {
	repo := newMockNotesRepo()
	skill := notes.NewNotes(repo)

	config := skill.Config()
	if !config.Enabled {
		t.Error("Expected skill to be enabled by default")
	}
}
