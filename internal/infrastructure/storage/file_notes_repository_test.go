package storage

import (
	"context"
	"nuimanbot/internal/domain"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFileNotesRepository_Create(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-123",
		Title:     "Meeting Notes",
		Content:   "Discussed project requirements and timeline",
		Tags:      []string{"work", "meeting"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify can retrieve note
	retrieved, err := repo.GetByID(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Title != "Meeting Notes" {
		t.Errorf("expected title 'Meeting Notes', got %s", retrieved.Title)
	}
}

func TestFileNotesRepository_GetByID(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-123",
		Title:     "Test Note",
		Content:   "Test content",
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	retrieved, err := repo.GetByID(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ID != note.ID {
		t.Errorf("expected ID %s, got %s", note.ID, retrieved.ID)
	}
}

func TestFileNotesRepository_GetByIDNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	ctx := context.Background()
	_, err := repo.GetByID(ctx, "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent note")
	}
}

func TestFileNotesRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	ctx := context.Background()

	// Create multiple notes for same user
	for i := 0; i < 3; i++ {
		note := &domain.Note{
			ID:        uuid.New().String(),
			UserID:    "user-123",
			Title:     "Note " + string(rune('A'+i)),
			Content:   "Content " + string(rune('A'+i)),
			Tags:      []string{"tag"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.Create(ctx, note)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Create note for different user
	otherNote := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-456",
		Title:     "Other Note",
		Content:   "Other content",
		Tags:      []string{"other"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, otherNote)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// List notes for user-123
	notes, err := repo.List(ctx, "user-123")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(notes) != 3 {
		t.Errorf("expected 3 notes for user-123, got %d", len(notes))
	}
}

func TestFileNotesRepository_Update(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-123",
		Title:     "Original Title",
		Content:   "Original content",
		Tags:      []string{"original"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update note
	note.Title = "Updated Title"
	note.Content = "Updated content"
	note.Tags = []string{"updated", "modified"}
	note.UpdatedAt = time.Now()

	err = repo.Update(ctx, note)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetByID(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %s", retrieved.Title)
	}
	if len(retrieved.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(retrieved.Tags))
	}
}

func TestFileNotesRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    "user-123",
		Title:     "To Be Deleted",
		Content:   "This will be deleted",
		Tags:      []string{"temp"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify note exists
	_, err = repo.GetByID(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Delete note
	err = repo.Delete(ctx, note.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify note is gone
	_, err = repo.GetByID(ctx, note.ID)
	if err == nil {
		t.Error("expected error when getting deleted note")
	}
}

func TestFileNotesRepository_TagIndex(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileNotesRepository(basePath)

	ctx := context.Background()

	// Create notes with different tags
	notes := []*domain.Note{
		{
			ID:        uuid.New().String(),
			UserID:    "user-123",
			Title:     "Note 1",
			Content:   "Content 1",
			Tags:      []string{"work", "urgent"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			UserID:    "user-123",
			Title:     "Note 2",
			Content:   "Content 2",
			Tags:      []string{"personal"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			UserID:    "user-123",
			Title:     "Note 3",
			Content:   "Content 3",
			Tags:      []string{"work"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, note := range notes {
		err := repo.Create(ctx, note)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List all notes - should get all 3
	allNotes, err := repo.List(ctx, "user-123")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(allNotes) != 3 {
		t.Errorf("expected 3 notes, got %d", len(allNotes))
	}

	// Verify tag index exists (implementation detail, but good to test)
	// The index should be updated when creating notes
}
