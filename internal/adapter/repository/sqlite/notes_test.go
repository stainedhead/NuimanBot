package sqlite_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/domain"
)

func setupNotesTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create users table (notes depend on users)
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL,
			platform_uid TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Insert test user
	_, err = db.Exec(`INSERT INTO users (id, platform, platform_uid, role) VALUES ('user1', 'cli', 'testuser', 'user')`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Create notes table
	_, err = db.Exec(`
		CREATE TABLE notes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create notes table: %v", err)
	}

	return db
}

func TestNotesRepository_Create(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := sqlite.NewNotesRepository(db)
	ctx := context.Background()

	note := &domain.Note{
		ID:        "note1",
		UserID:    "user1",
		Title:     "Test Note",
		Content:   "This is a test note",
		Tags:      []string{"test", "sample"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify note was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM notes WHERE id = ?", note.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query notes: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 note, got %d", count)
	}
}

func TestNotesRepository_GetByID(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := sqlite.NewNotesRepository(db)
	ctx := context.Background()

	// Insert test note
	note := &domain.Note{
		ID:        "note1",
		UserID:    "user1",
		Title:     "Test Note",
		Content:   "This is a test note",
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Retrieve note
	retrieved, err := repo.GetByID(ctx, "note1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if retrieved.Title != note.Title {
		t.Errorf("Expected title '%s', got '%s'", note.Title, retrieved.Title)
	}
	if retrieved.Content != note.Content {
		t.Errorf("Expected content '%s', got '%s'", note.Content, retrieved.Content)
	}
}

func TestNotesRepository_List(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := sqlite.NewNotesRepository(db)
	ctx := context.Background()

	// Create multiple notes
	for i := 1; i <= 3; i++ {
		note := &domain.Note{
			ID:        string(rune('0' + i)),
			UserID:    "user1",
			Title:     "Note " + string(rune('0'+i)),
			Content:   "Content " + string(rune('0'+i)),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.Create(ctx, note)
		if err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}
	}

	// List notes
	notes, err := repo.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(notes) != 3 {
		t.Errorf("Expected 3 notes, got %d", len(notes))
	}
}

func TestNotesRepository_Update(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := sqlite.NewNotesRepository(db)
	ctx := context.Background()

	// Create note
	note := &domain.Note{
		ID:        "note1",
		UserID:    "user1",
		Title:     "Original Title",
		Content:   "Original Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Update note
	note.Title = "Updated Title"
	note.Content = "Updated Content"
	err = repo.Update(ctx, note)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	updated, err := repo.GetByID(ctx, "note1")
	if err != nil {
		t.Fatalf("Failed to get updated note: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", updated.Title)
	}
}

func TestNotesRepository_Delete(t *testing.T) {
	db := setupNotesTestDB(t)
	defer db.Close()

	repo := sqlite.NewNotesRepository(db)
	ctx := context.Background()

	// Create note
	note := &domain.Note{
		ID:        "note1",
		UserID:    "user1",
		Title:     "Test Note",
		Content:   "Test Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Delete note
	err = repo.Delete(ctx, "note1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, "note1")
	if err == nil {
		t.Error("Expected error when getting deleted note")
	}
}
