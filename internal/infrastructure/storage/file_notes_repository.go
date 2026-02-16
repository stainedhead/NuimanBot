package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NotesIndex represents the index structure for notes
type NotesIndex struct {
	Version     string              `json:"version"`
	LastUpdated string              `json:"lastUpdated"`
	ByUser      map[string][]string `json:"byUser"` // userID -> []noteID
	ByTag       map[string][]string `json:"byTag"`  // tag -> []noteID
}

// FileNotesRepository implements NotesRepository using file storage
type FileNotesRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileNotesRepository creates a new file-based notes repository
func NewFileNotesRepository(basePath string) *FileNotesRepository {
	return &FileNotesRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

// getNotesDir returns the path to a user's notes directory
func (r *FileNotesRepository) getNotesDir(userID string) string {
	return filepath.Join(r.basePath, "users", userID, "notes")
}

// getNoteFile returns the path to a note's JSON file
func (r *FileNotesRepository) getNoteFile(userID, noteID string) string {
	return filepath.Join(r.getNotesDir(userID), noteID+".json")
}

// getIndexFile returns the path to a user's notes index file
func (r *FileNotesRepository) getIndexFile(userID string) string {
	return filepath.Join(r.getNotesDir(userID), "index.json")
}

// loadIndex loads the notes index for a user
func (r *FileNotesRepository) loadIndex(userID string) (*NotesIndex, error) {
	indexPath := r.getIndexFile(userID)

	// Check if file exists
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		// Return empty index
		return &NotesIndex{
			Version: "1.0",
			ByUser:  make(map[string][]string),
			ByTag:   make(map[string][]string),
		}, nil
	}

	// Read file
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file: %w", err)
	}

	// Parse JSON
	var index NotesIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index file: %w", err)
	}

	// Initialize maps if nil
	if index.ByUser == nil {
		index.ByUser = make(map[string][]string)
	}
	if index.ByTag == nil {
		index.ByTag = make(map[string][]string)
	}

	return &index, nil
}

// saveIndex saves the notes index for a user
func (r *FileNotesRepository) saveIndex(userID string, index *NotesIndex) error {
	indexPath := r.getIndexFile(userID)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Update timestamp
	index.LastUpdated = time.Now().Format(time.RFC3339)

	// Marshal to JSON
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write atomically
	if err := r.writer.Write(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// addToIndex adds a note to the index
func (r *FileNotesRepository) addToIndex(index *NotesIndex, note *domain.Note) {
	// By user
	index.ByUser[note.UserID] = appendUnique(index.ByUser[note.UserID], note.ID)

	// By tags
	for _, tag := range note.Tags {
		index.ByTag[tag] = appendUnique(index.ByTag[tag], note.ID)
	}
}

// removeFromIndex removes a note from the index
func (r *FileNotesRepository) removeFromIndex(index *NotesIndex, note *domain.Note) {
	// By user
	index.ByUser[note.UserID] = removeString(index.ByUser[note.UserID], note.ID)

	// By tags
	for _, tag := range note.Tags {
		index.ByTag[tag] = removeString(index.ByTag[tag], note.ID)
	}
}

// Create inserts a new note
func (r *FileNotesRepository) Create(ctx context.Context, note *domain.Note) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate note
	if err := note.Validate(); err != nil {
		return fmt.Errorf("note validation failed: %w", err)
	}

	// Ensure notes directory exists
	notesDir := r.getNotesDir(note.UserID)
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return fmt.Errorf("failed to create notes directory: %w", err)
	}

	// Check if note already exists
	notePath := r.getNoteFile(note.UserID, note.ID)
	if _, err := os.Stat(notePath); err == nil {
		return errors.New("note already exists")
	}

	// Write note to file
	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal note: %w", err)
	}

	if err := r.writer.Write(notePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write note file: %w", err)
	}

	// Update index
	index, err := r.loadIndex(note.UserID)
	if err != nil {
		return err
	}

	r.addToIndex(index, note)

	return r.saveIndex(note.UserID, index)
}

// GetByID retrieves a note by ID
func (r *FileNotesRepository) GetByID(ctx context.Context, noteID string) (*domain.Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// We need to find which user owns this note
	// Search through all users
	usersDir := filepath.Join(r.basePath, "users")
	users, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("note not found")
		}
		return nil, fmt.Errorf("failed to read users directory: %w", err)
	}

	// Search through user indexes
	for _, user := range users {
		if !user.IsDir() {
			continue
		}
		userID := user.Name()
		index, err := r.loadIndex(userID)
		if err != nil {
			continue
		}

		// Check if note belongs to this user
		found := false
		for _, id := range index.ByUser[userID] {
			if id == noteID {
				found = true
				break
			}
		}

		if found {
			// Load note
			notePath := r.getNoteFile(userID, noteID)
			data, err := os.ReadFile(notePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read note file: %w", err)
			}

			var note domain.Note
			if err := json.Unmarshal(data, &note); err != nil {
				return nil, fmt.Errorf("failed to parse note file: %w", err)
			}

			return &note, nil
		}
	}

	return nil, errors.New("note not found")
}

// List retrieves notes for a user
func (r *FileNotesRepository) List(ctx context.Context, userID string) ([]*domain.Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	index, err := r.loadIndex(userID)
	if err != nil {
		return nil, err
	}

	noteIDs := index.ByUser[userID]

	// Load notes
	notes := make([]*domain.Note, 0, len(noteIDs))
	for _, noteID := range noteIDs {
		notePath := r.getNoteFile(userID, noteID)
		data, err := os.ReadFile(notePath)
		if err != nil {
			continue // Skip notes that can't be read
		}

		var note domain.Note
		if err := json.Unmarshal(data, &note); err != nil {
			continue // Skip notes that can't be parsed
		}

		notes = append(notes, &note)
	}

	return notes, nil
}

// Update updates an existing note
func (r *FileNotesRepository) Update(ctx context.Context, note *domain.Note) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate note
	if err := note.Validate(); err != nil {
		return fmt.Errorf("note validation failed: %w", err)
	}

	notePath := r.getNoteFile(note.UserID, note.ID)

	// Check if note exists
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		return errors.New("note not found")
	}

	// Load old note to update index
	oldData, err := os.ReadFile(notePath)
	if err != nil {
		return fmt.Errorf("failed to read old note: %w", err)
	}

	var oldNote domain.Note
	if err := json.Unmarshal(oldData, &oldNote); err != nil {
		return fmt.Errorf("failed to parse old note: %w", err)
	}

	// Write updated note to file
	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal note: %w", err)
	}

	if err := r.writer.Write(notePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write note file: %w", err)
	}

	// Update index if tags changed
	if !tagsEqual(oldNote.Tags, note.Tags) {
		index, err := r.loadIndex(note.UserID)
		if err != nil {
			return err
		}

		r.removeFromIndex(index, &oldNote)
		r.addToIndex(index, note)

		return r.saveIndex(note.UserID, index)
	}

	return nil
}

// Delete removes a note by ID
func (r *FileNotesRepository) Delete(ctx context.Context, noteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the user who owns this note
	usersDir := filepath.Join(r.basePath, "users")
	users, err := os.ReadDir(usersDir)
	if err != nil {
		return errors.New("note not found")
	}

	for _, user := range users {
		if !user.IsDir() {
			continue
		}
		userID := user.Name()
		index, err := r.loadIndex(userID)
		if err != nil {
			continue
		}

		// Check if note belongs to this user
		found := false
		for _, id := range index.ByUser[userID] {
			if id == noteID {
				found = true
				break
			}
		}

		if found {
			// Load note to update index
			notePath := r.getNoteFile(userID, noteID)
			data, err := os.ReadFile(notePath)
			if err != nil {
				return fmt.Errorf("failed to read note file: %w", err)
			}

			var note domain.Note
			if err := json.Unmarshal(data, &note); err != nil {
				return fmt.Errorf("failed to parse note file: %w", err)
			}

			// Delete file
			if err := os.Remove(notePath); err != nil {
				return fmt.Errorf("failed to delete note file: %w", err)
			}

			// Update index
			r.removeFromIndex(index, &note)
			return r.saveIndex(userID, index)
		}
	}

	return errors.New("note not found")
}

// tagsEqual checks if two tag slices are equal
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
