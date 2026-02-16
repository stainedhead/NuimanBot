package storage

import (
	"context"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestIntegration_NewUserOnboarding tests the complete new user workflow
func TestIntegration_NewUserOnboarding(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	// Initialize all repositories
	profileRepo := NewFileUserProfileRepository(filepath.Join(basePath, "users.json"), "test-key")
	convRepo := NewFileConversationRepository(basePath)
	notesRepo := NewFileNotesRepository(basePath)
	auditRepo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	// 1. Create new user profile
	profile := domain.NewUserProfile("user-new", "newuser@example.com", domain.UserTypeIndividual)
	profile.FirstName = "New"
	profile.LastName = "User"
	profile.Moniker = "newuser"

	err := profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to save profile: %v", err)
	}

	// 2. Log audit event for user creation
	auditEvent := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    profile.UserID,
		Action:    "user_created",
		Resource:  "user_profile",
		Outcome:   "success",
		Details:   map[string]any{"email": profile.PrimaryEmail},
		SourceIP:  "127.0.0.1",
		Platform:  domain.PlatformCLI,
	}
	err = auditRepo.Append(ctx, auditEvent)
	if err != nil {
		t.Fatalf("Failed to log audit event: %v", err)
	}

	// 3. Create first conversation
	conv := &domain.Conversation{
		ID:        "conv-first",
		UserID:    profile.UserID,
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = convRepo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to save conversation: %v", err)
	}

	// 4. Create welcome note
	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    profile.UserID,
		Title:     "Welcome to NuimanBot",
		Content:   "Getting started guide and tips",
		Tags:      []string{"welcome", "guide"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = notesRepo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Verify all data persisted correctly
	retrievedProfile, err := profileRepo.GetProfileByUserID(ctx, profile.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve profile: %v", err)
	}
	if retrievedProfile.FirstName != "New" {
		t.Errorf("Profile not persisted correctly")
	}

	conversations, err := convRepo.ListConversations(ctx, profile.UserID)
	if err != nil || len(conversations) != 1 {
		t.Errorf("Conversation not persisted correctly")
	}

	notes, err := notesRepo.List(ctx, profile.UserID)
	if err != nil || len(notes) != 1 {
		t.Errorf("Note not persisted correctly")
	}

	auditEvents, err := auditRepo.Query(ctx, domain.AuditFilter{UserID: profile.UserID})
	if err != nil || len(auditEvents) != 1 {
		t.Errorf("Audit event not persisted correctly")
	}
}

// TestIntegration_ConversationWorkflow tests a complete conversation lifecycle
func TestIntegration_ConversationWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	convRepo := NewFileConversationRepository(basePath)
	auditRepo := NewFileAuditRepository(basePath)

	ctx := context.Background()
	userID := "user-conv-test"

	// Create conversation
	conv := &domain.Conversation{
		ID:        "conv-workflow",
		UserID:    userID,
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := convRepo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to save conversation: %v", err)
	}

	// Append multiple messages
	messages := []domain.StoredMessage{
		{
			ID:         "msg-1",
			Role:       "user",
			Content:    "Hello, how are you?",
			TokenCount: 10,
			Timestamp:  time.Now(),
		},
		{
			ID:         "msg-2",
			Role:       "assistant",
			Content:    "I'm doing well, thank you! How can I help you today?",
			TokenCount: 20,
			Timestamp:  time.Now(),
		},
		{
			ID:         "msg-3",
			Role:       "user",
			Content:    "I need help with Go programming",
			TokenCount: 15,
			Timestamp:  time.Now(),
		},
	}

	for _, msg := range messages {
		err = convRepo.AppendMessage(ctx, conv.ID, msg)
		if err != nil {
			t.Fatalf("Failed to append message: %v", err)
		}

		// Log audit event for each message
		auditEvent := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    userID,
			Action:    "message_sent",
			Resource:  conv.ID,
			Outcome:   "success",
			Details:   map[string]any{"role": msg.Role},
			SourceIP:  "127.0.0.1",
			Platform:  domain.PlatformCLI,
		}
		err = auditRepo.Append(ctx, auditEvent)
		if err != nil {
			t.Fatalf("Failed to log audit event: %v", err)
		}
	}

	// Verify conversation state
	retrieved, err := convRepo.GetConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve conversation: %v", err)
	}

	if len(retrieved.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(retrieved.Messages))
	}

	count, err := convRepo.CountMessages(ctx, conv.ID)
	if err != nil || count != 3 {
		t.Errorf("Message count incorrect: %v", err)
	}

	// Verify audit trail
	auditEvents, err := auditRepo.Query(ctx, domain.AuditFilter{
		UserID: userID,
		Action: "message_sent",
	})
	if err != nil || len(auditEvents) != 3 {
		t.Errorf("Expected 3 audit events, got %d", len(auditEvents))
	}
}

// TestIntegration_MemoryWorkflow tests memory cell and scene management
func TestIntegration_MemoryWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	cellRepo := NewFileMemoryCellRepository(basePath)
	sceneRepo := NewFileMemorySceneRepository(basePath)
	auditRepo := NewFileAuditRepository(basePath)

	ctx := context.Background()
	userID := "user-memory-test"
	convID := "conv-memory"

	// Create scene
	scene := &memoryv2.MemoryScene{
		Scene:      "project-setup",
		Summary:    "User is setting up a new Go project with file-based storage",
		TokenCount: 50,
		UpdatedAt:  time.Now(),
	}
	err := sceneRepo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Failed to create scene: %v", err)
	}

	// Create memory cells for the scene
	cells := []*memoryv2.MemoryCell{
		{
			ID:             uuid.New().String(),
			ConversationID: convID,
			Scene:          "project-setup",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.9,
			Content:        "User prefers file-based storage over SQLite",
			Source:         `["msg-1"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: convID,
			Scene:          "project-setup",
			CellType:       memoryv2.CellTypePreference,
			Salience:       0.8,
			Content:        "User wants to use JSON format for structured data",
			Source:         `["msg-2"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: convID,
			Scene:          "project-setup",
			CellType:       memoryv2.CellTypeDecision,
			Salience:       0.7,
			Content:        "Decided to use JSONL for append-only logs",
			Source:         `["msg-3"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, cell := range cells {
		err = cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create memory cell: %v", err)
		}

		// Log audit event
		auditEvent := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    userID,
			Action:    "memory_created",
			Resource:  cell.Scene,
			Outcome:   "success",
			Details:   map[string]any{"cell_type": cell.CellType.String()},
			SourceIP:  "127.0.0.1",
			Platform:  domain.PlatformCLI,
		}
		err = auditRepo.Append(ctx, auditEvent)
		if err != nil {
			t.Fatalf("Failed to log audit event: %v", err)
		}
	}

	// Verify scene exists
	retrievedScene, err := sceneRepo.Get(ctx, "project-setup")
	if err != nil {
		t.Fatalf("Failed to retrieve scene: %v", err)
	}
	if retrievedScene.TokenCount != 50 {
		t.Errorf("Scene not persisted correctly")
	}

	// Verify cells by scene
	sceneCells, err := cellRepo.GetByScene(ctx, "project-setup", 10)
	if err != nil || len(sceneCells) != 3 {
		t.Errorf("Expected 3 cells in scene, got %d", len(sceneCells))
	}

	// Verify cells by salience
	highSalienceCells, err := cellRepo.GetHighSalience(ctx, convID, 0.75, 10)
	if err != nil || len(highSalienceCells) != 2 {
		t.Errorf("Expected 2 high-salience cells, got %d", len(highSalienceCells))
	}

	// Verify search
	searchResults, err := cellRepo.SearchFTS(ctx, "file-based storage", 10)
	if err != nil || len(searchResults) == 0 {
		t.Errorf("Search should find cells")
	}
}

// TestIntegration_NotesWorkflow tests note management
func TestIntegration_NotesWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	notesRepo := NewFileNotesRepository(basePath)
	auditRepo := NewFileAuditRepository(basePath)

	ctx := context.Background()
	userID := "user-notes-test"

	// Create multiple notes
	notes := []*domain.Note{
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Project Ideas",
			Content:   "Ideas for future projects",
			Tags:      []string{"ideas", "projects"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Meeting Notes - 2026-02-16",
			Content:   "Discussed file-based storage implementation",
			Tags:      []string{"meeting", "work"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			UserID:    userID,
			Title:     "Learning Resources",
			Content:   "Useful links and tutorials for Go programming",
			Tags:      []string{"learning", "go"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, note := range notes {
		err := notesRepo.Create(ctx, note)
		if err != nil {
			t.Fatalf("Failed to create note: %v", err)
		}

		// Log audit event
		auditEvent := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    userID,
			Action:    "note_created",
			Resource:  note.ID,
			Outcome:   "success",
			Details:   map[string]any{"title": note.Title},
			SourceIP:  "127.0.0.1",
			Platform:  domain.PlatformCLI,
		}
		err = auditRepo.Append(ctx, auditEvent)
		if err != nil {
			t.Fatalf("Failed to log audit event: %v", err)
		}
	}

	// Update a note
	notes[0].Content = "Updated content with more ideas"
	notes[0].Tags = append(notes[0].Tags, "updated")
	notes[0].UpdatedAt = time.Now()

	err := notesRepo.Update(ctx, notes[0])
	if err != nil {
		t.Fatalf("Failed to update note: %v", err)
	}

	// Delete a note
	err = notesRepo.Delete(ctx, notes[2].ID)
	if err != nil {
		t.Fatalf("Failed to delete note: %v", err)
	}

	// Verify final state
	remainingNotes, err := notesRepo.List(ctx, userID)
	if err != nil || len(remainingNotes) != 2 {
		t.Errorf("Expected 2 remaining notes, got %d", len(remainingNotes))
	}

	// Verify updated note
	retrievedNote, err := notesRepo.GetByID(ctx, notes[0].ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated note: %v", err)
	}
	if len(retrievedNote.Tags) != 3 {
		t.Errorf("Note tags not updated correctly")
	}
}

// TestIntegration_ConcurrentAccess tests concurrent operations across repositories
func TestIntegration_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	convRepo := NewFileConversationRepository(basePath)
	auditRepo := NewFileAuditRepository(basePath)

	ctx := context.Background()
	convID := "conv-concurrent"
	userID := "user-concurrent"

	// Create conversation
	conv := &domain.Conversation{
		ID:        convID,
		UserID:    userID,
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := convRepo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to save conversation: %v", err)
	}

	// Concurrently append messages and log audit events
	numWorkers := 5
	messagesPerWorker := 10

	errChan := make(chan error, numWorkers*2)
	doneChan := make(chan bool, numWorkers*2)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for j := 0; j < messagesPerWorker; j++ {
				msg := domain.StoredMessage{
					ID:         uuid.New().String(),
					Role:       "user",
					Content:    "Concurrent message",
					TokenCount: 10,
					Timestamp:  time.Now(),
				}
				if err := convRepo.AppendMessage(ctx, convID, msg); err != nil {
					errChan <- err
					return
				}
			}
			doneChan <- true
		}(i)

		go func(workerID int) {
			for j := 0; j < messagesPerWorker; j++ {
				event := &domain.AuditEvent{
					Timestamp: time.Now(),
					UserID:    userID,
					Action:    "concurrent_test",
					Resource:  convID,
					Outcome:   "success",
					Details:   map[string]any{"worker": workerID, "msg": j},
					SourceIP:  "127.0.0.1",
					Platform:  domain.PlatformCLI,
				}
				if err := auditRepo.Append(ctx, event); err != nil {
					errChan <- err
					return
				}
			}
			doneChan <- true
		}(i)
	}

	// Wait for all workers
	for i := 0; i < numWorkers*2; i++ {
		select {
		case err := <-errChan:
			t.Fatalf("Concurrent operation failed: %v", err)
		case <-doneChan:
			// Success
		}
	}

	// Verify all messages were written
	count, err := convRepo.CountMessages(ctx, convID)
	if err != nil {
		t.Fatalf("Failed to count messages: %v", err)
	}

	expectedMessages := numWorkers * messagesPerWorker
	if count != expectedMessages {
		t.Errorf("Expected %d messages, got %d", expectedMessages, count)
	}

	// Verify all audit events were written
	events, err := auditRepo.Query(ctx, domain.AuditFilter{
		Action: "concurrent_test",
	})
	if err != nil {
		t.Fatalf("Failed to query audit events: %v", err)
	}

	expectedEvents := numWorkers * messagesPerWorker
	if len(events) != expectedEvents {
		t.Errorf("Expected %d audit events, got %d", expectedEvents, len(events))
	}
}

// TestIntegration_DataIntegrity tests that data remains consistent across operations
func TestIntegration_DataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	profileRepo := NewFileUserProfileRepository(filepath.Join(basePath, "users.json"), "test-key")
	cellRepo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()

	// Create user
	profile := domain.NewUserProfile("user-integrity", "integrity@example.com", domain.UserTypeIndividual)
	err := profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to save profile: %v", err)
	}

	// Create memory cell
	cell := &memoryv2.MemoryCell{
		ID:             uuid.New().String(),
		ConversationID: "conv-test",
		Scene:          "test-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "Test content for integrity check",
		Source:         `[]`,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err = cellRepo.Create(ctx, cell)
	if err != nil {
		t.Fatalf("Failed to create cell: %v", err)
	}

	// Retrieve and verify data hasn't been corrupted
	retrievedProfile, err := profileRepo.GetProfileByUserID(ctx, profile.UserID)
	if err != nil {
		t.Fatalf("Failed to retrieve profile: %v", err)
	}

	if retrievedProfile.PrimaryEmail != "integrity@example.com" {
		t.Error("Profile data corrupted")
	}

	retrievedCell, err := cellRepo.Get(ctx, cell.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve cell: %v", err)
	}

	if retrievedCell.Content != "Test content for integrity check" {
		t.Error("Cell data corrupted")
	}

	// Verify timestamps are preserved
	if retrievedCell.CreatedAt.IsZero() {
		t.Error("Timestamp not preserved")
	}
}
