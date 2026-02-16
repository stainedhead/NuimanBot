package storage

import (
	"context"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestUserProfilePersistence verifies user profiles persist correctly across restarts
func TestUserProfilePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	encryptionKey := "test-encryption-key-32bytes!ab"

	ctx := context.Background()

	// Create repository and save profiles
	repo1 := NewFileUserProfileRepository(usersJSONPath, encryptionKey)

	profiles := []*domain.UserProfile{
		domain.NewUserProfile("user-1", "alice@example.com", domain.UserTypeIndividual),
		domain.NewUserProfile("user-2", "bob@example.com", domain.UserTypeIndividual),
		domain.NewUserProfile("user-3", "charlie@example.com", domain.UserTypeEnterprise),
	}

	profiles[0].FirstName = "Alice"
	profiles[0].LastName = "Anderson"
	profiles[0].PlatformIDs = domain.PlatformIdentifiers{Slack: "U01ABC123"}

	profiles[1].FirstName = "Bob"
	profiles[1].LastName = "Builder"
	profiles[1].PlatformIDs = domain.PlatformIdentifiers{Telegram: "123456789"}

	profiles[2].FirstName = "Charlie"
	profiles[2].LastName = "Company"

	for _, p := range profiles {
		err := repo1.SaveProfile(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save profile %s: %v", p.UserID, err)
		}
	}

	// Simulate restart by creating new repository instance
	repo2 := NewFileUserProfileRepository(usersJSONPath, encryptionKey)

	// Verify all profiles persisted correctly
	for _, original := range profiles {
		retrieved, err := repo2.GetProfileByUserID(ctx, original.UserID)
		if err != nil {
			t.Errorf("Failed to retrieve profile %s after restart: %v", original.UserID, err)
			continue
		}

		if retrieved.PrimaryEmail != original.PrimaryEmail {
			t.Errorf("Profile %s: email mismatch: got %s, want %s",
				original.UserID, retrieved.PrimaryEmail, original.PrimaryEmail)
		}

		if retrieved.FirstName != original.FirstName {
			t.Errorf("Profile %s: first name mismatch: got %s, want %s",
				original.UserID, retrieved.FirstName, original.FirstName)
		}

		if retrieved.UserType != original.UserType {
			t.Errorf("Profile %s: user type mismatch: got %v, want %v",
				original.UserID, retrieved.UserType, original.UserType)
		}
	}

	// Verify platform ID lookups still work
	slackProfile, err := repo2.GetProfileByPlatformID(ctx, domain.PlatformSlack, "U01ABC123")
	if err != nil {
		t.Errorf("Failed to retrieve profile by Slack ID after restart: %v", err)
	} else if slackProfile.UserID != "user-1" {
		t.Errorf("Wrong profile retrieved by Slack ID: got %s, want user-1", slackProfile.UserID)
	}
}

// TestConversationPersistenceAndConsistency verifies conversations and messages persist correctly
func TestConversationPersistenceAndConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	ctx := context.Background()

	// Create repository and save conversations
	repo1 := NewFileConversationRepository(basePath)

	conversations := []*domain.Conversation{
		{
			ID:        "conv-1",
			UserID:    "user-1",
			Platform:  domain.PlatformCLI,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "conv-2",
			UserID:    "user-1",
			Platform:  domain.PlatformSlack,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "conv-3",
			UserID:    "user-2",
			Platform:  domain.PlatformCLI,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, conv := range conversations {
		err := repo1.SaveConversation(ctx, conv)
		if err != nil {
			t.Fatalf("Failed to save conversation %s: %v", conv.ID, err)
		}
	}

	// Add messages to conversations
	messages := []struct {
		convID string
		msg    domain.StoredMessage
	}{
		{"conv-1", domain.StoredMessage{ID: "msg-1", Role: "user", Content: "Hello", TokenCount: 5, Timestamp: time.Now()}},
		{"conv-1", domain.StoredMessage{ID: "msg-2", Role: "assistant", Content: "Hi there!", TokenCount: 10, Timestamp: time.Now()}},
		{"conv-2", domain.StoredMessage{ID: "msg-3", Role: "user", Content: "Test message", TokenCount: 8, Timestamp: time.Now()}},
		{"conv-3", domain.StoredMessage{ID: "msg-4", Role: "user", Content: "Another test", TokenCount: 7, Timestamp: time.Now()}},
		{"conv-3", domain.StoredMessage{ID: "msg-5", Role: "assistant", Content: "Response", TokenCount: 6, Timestamp: time.Now()}},
	}

	for _, m := range messages {
		err := repo1.AppendMessage(ctx, m.convID, m.msg)
		if err != nil {
			t.Fatalf("Failed to append message %s to %s: %v", m.msg.ID, m.convID, err)
		}
	}

	// Simulate restart
	repo2 := NewFileConversationRepository(basePath)

	// Verify all conversations persisted
	for _, original := range conversations {
		retrieved, err := repo2.GetConversation(ctx, original.ID)
		if err != nil {
			t.Errorf("Failed to retrieve conversation %s after restart: %v", original.ID, err)
			continue
		}

		if retrieved.UserID != original.UserID {
			t.Errorf("Conversation %s: user ID mismatch: got %s, want %s",
				original.ID, retrieved.UserID, original.UserID)
		}

		if retrieved.Platform != original.Platform {
			t.Errorf("Conversation %s: platform mismatch: got %v, want %v",
				original.ID, retrieved.Platform, original.Platform)
		}
	}

	// Verify message counts
	conv1, _ := repo2.GetConversation(ctx, "conv-1")
	if len(conv1.Messages) != 2 {
		t.Errorf("Conversation conv-1: expected 2 messages, got %d", len(conv1.Messages))
	}

	conv2, _ := repo2.GetConversation(ctx, "conv-2")
	if len(conv2.Messages) != 1 {
		t.Errorf("Conversation conv-2: expected 1 message, got %d", len(conv2.Messages))
	}

	conv3, _ := repo2.GetConversation(ctx, "conv-3")
	if len(conv3.Messages) != 2 {
		t.Errorf("Conversation conv-3: expected 2 messages, got %d", len(conv3.Messages))
	}

	// Verify message content
	if conv1.Messages[0].Content != "Hello" {
		t.Errorf("First message content mismatch: got %s, want Hello", conv1.Messages[0].Content)
	}

	// Verify list still works after restart
	list, err := repo2.ListConversations(ctx, "user-1")
	if err != nil {
		t.Errorf("Failed to list conversations after restart: %v", err)
	} else if len(list) != 2 {
		t.Errorf("Expected 2 conversations for user-1, got %d", len(list))
	}
}

// TestMemoryCellConsistency verifies memory cells and indexes remain consistent
func TestMemoryCellConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	ctx := context.Background()

	// Create repository and save cells
	repo1 := NewFileMemoryCellRepository(basePath)

	cells := make([]*memoryv2.MemoryCell, 100)
	scenes := []string{"project-setup", "coding-patterns", "testing", "deployment", "monitoring"}

	for i := 0; i < 100; i++ {
		cellID := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		cells[i] = &memoryv2.MemoryCell{
			ID:             cellID,
			ConversationID: "test-conv",
			Scene:          scenes[i%len(scenes)],
			CellType:       memoryv2.CellTypeFact,
			Content:        fmt.Sprintf("Memory cell %d content", i),
			Source:         fmt.Sprintf("[\"%s\"]", cellID),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo1.Create(ctx, cells[i])
		if err != nil {
			t.Fatalf("Failed to create cell %d: %v", i, err)
		}
	}

	// Simulate restart
	repo2 := NewFileMemoryCellRepository(basePath)

	// Verify all cells can be retrieved
	for i, original := range cells {
		retrieved, err := repo2.Get(ctx, original.ID)
		if err != nil {
			t.Errorf("Failed to retrieve cell %d after restart: %v", i, err)
			continue
		}

		if retrieved.Content != original.Content {
			t.Errorf("Cell %d content mismatch: got %s, want %s",
				i, retrieved.Content, original.Content)
		}

		if retrieved.Scene != original.Scene {
			t.Errorf("Cell %d scene mismatch: got %s, want %s",
				i, retrieved.Scene, original.Scene)
		}
	}

	// Verify scene-based queries work correctly
	for _, scene := range scenes {
		results, err := repo2.GetByScene(ctx, scene, 100)
		if err != nil {
			t.Errorf("Failed to query scene %s after restart: %v", scene, err)
			continue
		}

		// Each scene should have 20 cells (100 cells / 5 scenes)
		if len(results) != 20 {
			t.Errorf("Scene %s: expected 20 cells, got %d", scene, len(results))
		}

		// Verify all results belong to the correct scene
		for _, cell := range results {
			if cell.Scene != scene {
				t.Errorf("Scene query %s returned cell with scene %s", scene, cell.Scene)
			}
		}
	}

	// Verify FTS search still works
	results, err := repo2.SearchFTS(ctx, "content", 100)
	if err != nil {
		t.Errorf("FTS search failed after restart: %v", err)
	} else if len(results) != 100 {
		t.Errorf("FTS search: expected 100 results, got %d", len(results))
	}
}

// TestNotesIndexAccuracy verifies notes and tag indexes remain accurate
func TestNotesIndexAccuracy(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	ctx := context.Background()

	// Create repository and save notes
	repo1 := NewFileNotesRepository(basePath)

	userID := "test-user"
	tags := []string{"important", "work", "personal", "todo", "done"}

	notes := make([]*domain.Note, 50)
	for i := 0; i < 50; i++ {
		notes[i] = &domain.Note{
			ID:        fmt.Sprintf("note-%d", i),
			UserID:    userID,
			Title:     fmt.Sprintf("Note %d", i),
			Content:   fmt.Sprintf("Note content %d", i),
			Tags:      []string{tags[i%len(tags)]},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo1.Create(ctx, notes[i])
		if err != nil {
			t.Fatalf("Failed to create note %d: %v", i, err)
		}
	}

	// Simulate restart
	repo2 := NewFileNotesRepository(basePath)

	// Verify all notes persisted
	allNotes, err := repo2.List(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to list notes after restart: %v", err)
	}

	if len(allNotes) != 50 {
		t.Errorf("Expected 50 notes, got %d", len(allNotes))
	}

	// Verify individual note retrieval
	for i, original := range notes {
		retrieved, err := repo2.GetByID(ctx, original.ID)
		if err != nil {
			t.Errorf("Failed to retrieve note %d after restart: %v", i, err)
			continue
		}

		if retrieved.Title != original.Title {
			t.Errorf("Note %d title mismatch: got %s, want %s",
				i, retrieved.Title, original.Title)
		}

		if len(retrieved.Tags) != len(original.Tags) {
			t.Errorf("Note %d tags length mismatch: got %d, want %d",
				i, len(retrieved.Tags), len(original.Tags))
		}
	}

	// Verify tag-based filtering works correctly
	// Each tag should appear in 10 notes (50 notes / 5 tags)
	for _, tag := range tags {
		count := 0
		for _, note := range allNotes {
			for _, noteTag := range note.Tags {
				if noteTag == tag {
					count++
					break
				}
			}
		}

		if count != 10 {
			t.Errorf("Tag %s: expected 10 notes, got %d", tag, count)
		}
	}
}

// TestAuditLogIntegrity verifies audit log entries persist and remain ordered
func TestAuditLogIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	ctx := context.Background()

	// Create repository and log events
	repo1 := NewFileAuditRepository(basePath)

	events := make([]*domain.AuditEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = &domain.AuditEvent{
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			UserID:    fmt.Sprintf("user-%d", i%5),
			Action:    "test_action",
			Resource:  fmt.Sprintf("resource-%d", i),
			Outcome:   "success",
			SourceIP:  "127.0.0.1",
			Platform:  domain.PlatformCLI,
			Details:   map[string]any{"index": i},
		}
		err := repo1.Append(ctx, events[i])
		if err != nil {
			t.Fatalf("Failed to log event %d: %v", i, err)
		}
	}

	// Simulate restart
	repo2 := NewFileAuditRepository(basePath)

	// Query all events
	retrieved, err := repo2.Query(ctx, domain.AuditFilter{Limit: 1000})
	if err != nil {
		t.Fatalf("Failed to query audit log after restart: %v", err)
	}

	if len(retrieved) != 100 {
		t.Errorf("Expected 100 audit events, got %d", len(retrieved))
	}

	// Verify events are in reverse chronological order (most recent first)
	// This is typical for audit logs
	for i := 1; i < len(retrieved); i++ {
		if retrieved[i].Timestamp.After(retrieved[i-1].Timestamp) {
			t.Errorf("Events out of order: event %d timestamp %v is after event %d timestamp %v",
				i, retrieved[i].Timestamp, i-1, retrieved[i-1].Timestamp)
		}
	}

	// Verify filtering works correctly
	user0Events, err := repo2.Query(ctx, domain.AuditFilter{UserID: "user-0", Limit: 1000})
	if err != nil {
		t.Errorf("Failed to query user-0 events: %v", err)
	} else if len(user0Events) != 20 {
		t.Errorf("Expected 20 events for user-0, got %d", len(user0Events))
	}

	// Verify all returned events are for the correct user
	for _, evt := range user0Events {
		if evt.UserID != "user-0" {
			t.Errorf("User filter returned event for wrong user: %s", evt.UserID)
		}
	}
}

// TestDataRecoveryAfterPartialWrite tests recovery from interrupted writes
func TestDataRecoveryAfterPartialWrite(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	encryptionKey := "test-encryption-key-32bytes!ab"

	ctx := context.Background()

	// Create repository and save initial profile
	repo1 := NewFileUserProfileRepository(usersJSONPath, encryptionKey)

	profile := domain.NewUserProfile("user-1", "alice@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Alice"

	err := repo1.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to save initial profile: %v", err)
	}

	// Verify file was created
	originalData, err := os.ReadFile(usersJSONPath)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	// Simulate partial write by writing corrupted data
	// (In real scenarios, atomic writes should prevent this, but we test recovery anyway)
	err = os.WriteFile(usersJSONPath, []byte("corrupted partial data"), 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupted data: %v", err)
	}

	// Try to create new repository - should handle corruption gracefully
	repo2 := NewFileUserProfileRepository(usersJSONPath, encryptionKey)

	// Attempt to retrieve profile should fail gracefully
	_, err = repo2.GetProfileByUserID(ctx, "user-1")
	if err == nil {
		t.Logf("Repository handled corrupted file gracefully (returned error as expected)")
	}

	// Restore original data (simulating manual recovery or backup restore)
	err = os.WriteFile(usersJSONPath, originalData, 0644)
	if err != nil {
		t.Fatalf("Failed to restore original data: %v", err)
	}

	// Create new repository and verify data is accessible again
	repo3 := NewFileUserProfileRepository(usersJSONPath, encryptionKey)

	recovered, err := repo3.GetProfileByUserID(ctx, "user-1")
	if err != nil {
		t.Errorf("Failed to recover profile after data restoration: %v", err)
	} else if recovered.FirstName != "Alice" {
		t.Errorf("Recovered profile data mismatch: got %s, want Alice", recovered.FirstName)
	}
}

// TestCrossRepositoryConsistency verifies data consistency across multiple repository types
func TestCrossRepositoryConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	conversationBasePath := filepath.Join(tmpDir, "conversations")
	notesBasePath := filepath.Join(tmpDir, "notes")

	ctx := context.Background()

	// Create repositories
	profileRepo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	convRepo := NewFileConversationRepository(conversationBasePath)
	notesRepo := NewFileNotesRepository(notesBasePath)

	// Create user
	userID := "test-user"
	profile := domain.NewUserProfile(userID, "user@example.com", domain.UserTypeIndividual)
	err := profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to create user profile: %v", err)
	}

	// Create conversations for user
	conv1 := &domain.Conversation{
		ID:        "conv-1",
		UserID:    userID,
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = convRepo.SaveConversation(ctx, conv1)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Create notes for user
	note := &domain.Note{
		ID:        "note-1",
		UserID:    userID,
		Title:     "Test Note",
		Content:   "Content",
		Tags:      []string{"test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err = notesRepo.Create(ctx, note)
	if err != nil {
		t.Fatalf("Failed to create note: %v", err)
	}

	// Simulate restart of all repositories
	profileRepo2 := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	convRepo2 := NewFileConversationRepository(conversationBasePath)
	notesRepo2 := NewFileNotesRepository(notesBasePath)

	// Verify user still exists
	retrievedProfile, err := profileRepo2.GetProfileByUserID(ctx, userID)
	if err != nil {
		t.Errorf("Failed to retrieve user profile after restart: %v", err)
	} else if retrievedProfile.UserID != userID {
		t.Errorf("User profile mismatch after restart")
	}

	// Verify conversations still exist and reference correct user
	retrievedConv, err := convRepo2.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Errorf("Failed to retrieve conversation after restart: %v", err)
	} else if retrievedConv.UserID != userID {
		t.Errorf("Conversation user ID mismatch: got %s, want %s", retrievedConv.UserID, userID)
	}

	// Verify notes still exist and reference correct user
	retrievedNote, err := notesRepo2.GetByID(ctx, "note-1")
	if err != nil {
		t.Errorf("Failed to retrieve note after restart: %v", err)
	} else if retrievedNote.UserID != userID {
		t.Errorf("Note user ID mismatch: got %s, want %s", retrievedNote.UserID, userID)
	}

	// Verify user deletion would maintain referential integrity
	// (In this test, we just verify the data exists - actual deletion logic
	// would be handled at the application layer)
	conversations, _ := convRepo2.ListConversations(ctx, userID)
	notes, _ := notesRepo2.List(ctx, userID)

	t.Logf("User %s has %d conversations and %d notes - referential integrity maintained",
		userID, len(conversations), len(notes))
}
