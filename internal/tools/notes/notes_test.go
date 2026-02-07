package notes_test

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/tools/notes"
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
	tool := notes.NewNotes(repo)

	if tool.Name() != "notes" {
		t.Errorf("Expected name 'notes', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestNotesSkill_Execute_Create(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	// Add user_id to context
	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
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
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
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
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
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
	tool := notes.NewNotes(repo)

	perms := tool.RequiredPermissions()
	if len(perms) != 1 {
		t.Errorf("Expected 1 permission, got %d", len(perms))
	}
	if len(perms) > 0 && perms[0] != domain.PermissionWrite {
		t.Errorf("Expected PermissionWrite, got %v", perms[0])
	}
}

func TestNotesSkill_Config(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	config := tool.Config()
	if !config.Enabled {
		t.Error("Expected tool to be enabled by default")
	}
}

func TestNotesSkill_Execute_MissingUserID(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	// Context without user_id
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Test",
		"content":   "Test content",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing user_id")
	}
}

func TestNotesSkill_Execute_CreateWithTags(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Tagged Note",
		"content":   "Content with tags",
		"tags":      []interface{}{"work", "important"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
	if result.Metadata["note_id"] == nil {
		t.Error("Expected note_id in metadata")
	}
}

func TestNotesSkill_Execute_CreateMissingTitle(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"content":   "Content without title",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing title")
	}
}

func TestNotesSkill_Execute_CreateMissingContent(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Title without content",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing content")
	}
}

func TestNotesSkill_Execute_Read(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create a note first
	createResult, _ := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Test Note",
		"content":   "Test content",
	})
	noteID := createResult.Metadata["note_id"].(string)

	// Read the note
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "read",
		"id":        noteID,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("Expected output from read operation")
	}
}

func TestNotesSkill_Execute_ReadMissingID(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "read",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing id")
	}
}

func TestNotesSkill_Execute_ReadNotFound(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "read",
		"id":        "nonexistent",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for nonexistent note")
	}
}

func TestNotesSkill_Execute_Update(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create a note first
	createResult, _ := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Original Title",
		"content":   "Original content",
	})
	noteID := createResult.Metadata["note_id"].(string)

	// Update the note
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "update",
		"id":        noteID,
		"title":     "Updated Title",
		"content":   "Updated content",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestNotesSkill_Execute_UpdatePartial(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create a note first
	createResult, _ := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Original Title",
		"content":   "Original content",
	})
	noteID := createResult.Metadata["note_id"].(string)

	// Update only the title
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "update",
		"id":        noteID,
		"title":     "New Title Only",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestNotesSkill_Execute_UpdateWithTags(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create a note first
	createResult, _ := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Test",
		"content":   "Test",
	})
	noteID := createResult.Metadata["note_id"].(string)

	// Update with tags
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "update",
		"id":        noteID,
		"tags":      []interface{}{"updated", "tagged"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestNotesSkill_Execute_UpdateMissingID(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "update",
		"title":     "New Title",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing id")
	}
}

func TestNotesSkill_Execute_UpdateNotFound(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "update",
		"id":        "nonexistent",
		"title":     "New Title",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for nonexistent note")
	}
}

func TestNotesSkill_Execute_Delete(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create a note first
	createResult, _ := tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "To Delete",
		"content":   "Will be deleted",
	})
	noteID := createResult.Metadata["note_id"].(string)

	// Delete the note
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "delete",
		"id":        noteID,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
}

func TestNotesSkill_Execute_DeleteMissingID(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "delete",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for missing id")
	}
}

func TestNotesSkill_Execute_DeleteNotFound(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "delete",
		"id":        "nonexistent",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("Expected error for nonexistent note")
	}
}

func TestNotesSkill_Execute_List(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	// Create multiple notes
	tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Note 1",
		"content":   "Content 1",
		"tags":      []interface{}{"tag1"},
	})
	tool.Execute(ctx, map[string]any{
		"operation": "create",
		"title":     "Note 2",
		"content":   "Content 2",
	})

	// List notes
	result, err := tool.Execute(ctx, map[string]any{
		"operation": "list",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("Expected output from list operation")
	}
	if result.Metadata["count"] != 2 {
		t.Errorf("Expected count 2, got %v", result.Metadata["count"])
	}
}

func TestNotesSkill_Execute_ListEmpty(t *testing.T) {
	repo := newMockNotesRepo()
	tool := notes.NewNotes(repo)

	ctx := context.WithValue(context.Background(), "user_id", "user1") //nolint:staticcheck // Test uses string key for simplicity

	result, err := tool.Execute(ctx, map[string]any{
		"operation": "list",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Errorf("Expected no error, got: %s", result.Error)
	}
	if result.Output != "No notes found" {
		t.Errorf("Expected 'No notes found', got: %s", result.Output)
	}
}
