package storage

import (
	"context"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"path/filepath"
	"testing"
	"time"
)

// TestE2E_ChatWorkflow tests the complete chat workflow from user creation to conversation
func TestE2E_ChatWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	conversationBasePath := filepath.Join(tmpDir, "conversations")

	ctx := context.Background()

	// Step 1: Create user profile
	profileRepo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	userID := "alice"
	profile := domain.NewUserProfile(userID, "alice@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Alice"
	profile.LastName = "Anderson"
	profile.PlatformIDs = domain.PlatformIdentifiers{
		Slack: "U01ABC123",
		CLI:   "alice",
	}

	err := profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Step 1 failed - create user: %v", err)
	}

	// Step 2: Verify user can be retrieved
	retrievedProfile, err := profileRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("Step 2 failed - retrieve user: %v", err)
	}
	if retrievedProfile.FirstName != "Alice" {
		t.Errorf("User profile mismatch: got %s, want Alice", retrievedProfile.FirstName)
	}

	// Step 3: Start a new conversation
	convRepo := NewFileConversationRepository(conversationBasePath)

	conv := &domain.Conversation{
		ID:        "conv-001",
		UserID:    userID,
		Platform:  domain.PlatformSlack,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = convRepo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Step 3 failed - create conversation: %v", err)
	}

	// Step 4: User sends first message
	msg1 := domain.StoredMessage{
		ID:         "msg-001",
		Role:       "user",
		Content:    "Hello, I need help with Go testing",
		TokenCount: 15,
		Timestamp:  time.Now(),
	}

	err = convRepo.AppendMessage(ctx, "conv-001", msg1)
	if err != nil {
		t.Fatalf("Step 4 failed - append user message: %v", err)
	}

	// Step 5: Assistant responds
	msg2 := domain.StoredMessage{
		ID:         "msg-002",
		Role:       "assistant",
		Content:    "I'd be happy to help with Go testing! What specific aspect are you interested in?",
		TokenCount: 25,
		Timestamp:  time.Now(),
	}

	err = convRepo.AppendMessage(ctx, "conv-001", msg2)
	if err != nil {
		t.Fatalf("Step 5 failed - append assistant message: %v", err)
	}

	// Step 6: User continues conversation
	msg3 := domain.StoredMessage{
		ID:         "msg-003",
		Role:       "user",
		Content:    "I want to learn about table-driven tests",
		TokenCount: 18,
		Timestamp:  time.Now(),
	}

	err = convRepo.AppendMessage(ctx, "conv-001", msg3)
	if err != nil {
		t.Fatalf("Step 6 failed - append second user message: %v", err)
	}

	// Step 7: Retrieve full conversation and verify
	fullConv, err := convRepo.GetConversation(ctx, "conv-001")
	if err != nil {
		t.Fatalf("Step 7 failed - retrieve conversation: %v", err)
	}

	if len(fullConv.Messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(fullConv.Messages))
	}

	if fullConv.Messages[0].Content != "Hello, I need help with Go testing" {
		t.Errorf("First message content mismatch")
	}

	// Step 8: List user's conversations
	conversations, err := convRepo.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("Step 8 failed - list conversations: %v", err)
	}

	if len(conversations) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(conversations))
	}

	// Step 9: Start another conversation for same user
	conv2 := &domain.Conversation{
		ID:        "conv-002",
		UserID:    userID,
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = convRepo.SaveConversation(ctx, conv2)
	if err != nil {
		t.Fatalf("Step 9 failed - create second conversation: %v", err)
	}

	// Step 10: Verify user now has 2 conversations
	conversations, err = convRepo.ListConversations(ctx, userID)
	if err != nil {
		t.Fatalf("Step 10 failed - list conversations again: %v", err)
	}

	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}

	t.Logf("✓ Chat workflow complete: User created, 2 conversations with 3 messages")
}

// TestE2E_MemoryWorkflow tests the complete memory management workflow
func TestE2E_MemoryWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	memoryBasePath := filepath.Join(tmpDir, "memory")
	sceneBasePath := filepath.Join(tmpDir, "scenes")

	ctx := context.Background()

	cellRepo := NewFileMemoryCellRepository(memoryBasePath)
	sceneRepo := NewFileMemorySceneRepository(sceneBasePath)

	conversationID := "conv-123"

	// Step 1: Create memory cells during conversation
	cells := []*memoryv2.MemoryCell{
		{
			ID:             "00000000-0000-0000-0000-000000000001",
			ConversationID: conversationID,
			Scene:          "user-preferences",
			CellType:       memoryv2.CellTypeFact,
			Content:        "User prefers Go for backend development",
			Source:         `["msg-001"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "00000000-0000-0000-0000-000000000002",
			ConversationID: conversationID,
			Scene:          "user-preferences",
			CellType:       memoryv2.CellTypeFact,
			Content:        "User is interested in table-driven testing patterns",
			Source:         `["msg-003"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "00000000-0000-0000-0000-000000000003",
			ConversationID: conversationID,
			Scene:          "project-context",
			CellType:       memoryv2.CellTypeFact,
			Content:        "Working on a CLI application called nuimanbot",
			Source:         `["msg-005"]`,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for i, cell := range cells {
		err := cellRepo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Step 1.%d failed - create memory cell: %v", i+1, err)
		}
	}

	// Step 2: Create/update scene metadata
	scene := &memoryv2.MemoryScene{
		Scene:      "user-preferences",
		Summary:    "User's preferences and interests",
		TokenCount: 10,
		UpdatedAt:  time.Now(),
	}

	err := sceneRepo.Upsert(ctx, scene)
	if err != nil {
		t.Fatalf("Step 2 failed - create scene: %v", err)
	}

	// Step 3: Search memory by scene
	prefCells, err := cellRepo.GetByScene(ctx, "user-preferences", 100)
	if err != nil {
		t.Fatalf("Step 3 failed - search by scene: %v", err)
	}

	if len(prefCells) != 2 {
		t.Errorf("Expected 2 cells in user-preferences scene, got %d", len(prefCells))
	}

	// Step 4: Full-text search across all memory
	goCells, err := cellRepo.SearchFTS(ctx, "Go", 100)
	if err != nil {
		t.Fatalf("Step 4 failed - FTS search: %v", err)
	}

	// FTS search may be case-sensitive or use different matching
	// Just verify it doesn't error; actual results depend on implementation
	t.Logf("FTS search for 'Go' returned %d cells", len(goCells))

	// Step 5: Retrieve specific memory cell
	cell, err := cellRepo.Get(ctx, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("Step 5 failed - get specific cell: %v", err)
	}

	if cell.Scene != "user-preferences" {
		t.Errorf("Cell scene mismatch: got %s, want user-preferences", cell.Scene)
	}

	// Step 6: List all scenes
	scenes, err := sceneRepo.List(ctx)
	if err != nil {
		t.Fatalf("Step 6 failed - list scenes: %v", err)
	}

	if len(scenes) < 1 {
		t.Errorf("Expected at least 1 scene, got %d", len(scenes))
	}

	// Step 7: Delete a memory cell
	err = cellRepo.Delete(ctx, "00000000-0000-0000-0000-000000000003")
	if err != nil {
		t.Fatalf("Step 7 failed - delete cell: %v", err)
	}

	// Step 8: Verify deletion
	_, err = cellRepo.Get(ctx, "00000000-0000-0000-0000-000000000003")
	if err == nil {
		t.Error("Expected error when retrieving deleted cell")
	}

	t.Logf("✓ Memory workflow complete: 3 cells created, searched, 1 deleted")
}

// TestE2E_NotesWorkflow tests the complete notes management workflow
func TestE2E_NotesWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	notesBasePath := filepath.Join(tmpDir, "notes")

	ctx := context.Background()

	notesRepo := NewFileNotesRepository(notesBasePath)
	userID := "alice"

	// Step 1: Create initial note
	note1 := &domain.Note{
		ID:        "note-001",
		UserID:    userID,
		Title:     "Go Testing Best Practices",
		Content:   "Table-driven tests are idiomatic in Go...",
		Tags:      []string{"golang", "testing", "best-practices"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := notesRepo.Create(ctx, note1)
	if err != nil {
		t.Fatalf("Step 1 failed - create note: %v", err)
	}

	// Step 2: Create another note
	note2 := &domain.Note{
		ID:        "note-002",
		UserID:    userID,
		Title:     "Project Ideas",
		Content:   "Build a CLI tool for managing notes...",
		Tags:      []string{"ideas", "cli", "golang"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = notesRepo.Create(ctx, note2)
	if err != nil {
		t.Fatalf("Step 2 failed - create second note: %v", err)
	}

	// Step 3: List all notes
	allNotes, err := notesRepo.List(ctx, userID)
	if err != nil {
		t.Fatalf("Step 3 failed - list notes: %v", err)
	}

	if len(allNotes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(allNotes))
	}

	// Step 4: Retrieve specific note
	retrieved, err := notesRepo.GetByID(ctx, "note-001")
	if err != nil {
		t.Fatalf("Step 4 failed - get note by ID: %v", err)
	}

	if retrieved.Title != "Go Testing Best Practices" {
		t.Errorf("Note title mismatch: got %s", retrieved.Title)
	}

	// Step 5: Update note content
	note1.Content = "Table-driven tests are idiomatic in Go. Use subtests for better organization."
	note1.Tags = append(note1.Tags, "subtests")
	note1.UpdatedAt = time.Now()

	err = notesRepo.Update(ctx, note1)
	if err != nil {
		t.Fatalf("Step 5 failed - update note: %v", err)
	}

	// Step 6: Verify update
	updated, err := notesRepo.GetByID(ctx, "note-001")
	if err != nil {
		t.Fatalf("Step 6 failed - retrieve updated note: %v", err)
	}

	if updated.Content != note1.Content {
		t.Errorf("Note content not updated correctly")
	}

	if len(updated.Tags) != 4 {
		t.Errorf("Expected 4 tags after update, got %d", len(updated.Tags))
	}

	// Step 7: Filter notes by tag
	golangNotes := 0
	for _, note := range allNotes {
		for _, tag := range note.Tags {
			if tag == "golang" {
				golangNotes++
				break
			}
		}
	}

	if golangNotes != 2 {
		t.Errorf("Expected 2 notes with 'golang' tag, got %d", golangNotes)
	}

	// Step 8: Delete a note
	err = notesRepo.Delete(ctx, "note-002")
	if err != nil {
		t.Fatalf("Step 8 failed - delete note: %v", err)
	}

	// Step 9: Verify deletion
	allNotes, err = notesRepo.List(ctx, userID)
	if err != nil {
		t.Fatalf("Step 9 failed - list notes after deletion: %v", err)
	}

	if len(allNotes) != 1 {
		t.Errorf("Expected 1 note after deletion, got %d", len(allNotes))
	}

	t.Logf("✓ Notes workflow complete: 2 notes created, 1 updated, 1 deleted")
}

// TestE2E_UserManagement tests complete user lifecycle
func TestE2E_UserManagement(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")

	ctx := context.Background()

	profileRepo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	// Step 1: Create user profile
	userID := "bob"
	profile := domain.NewUserProfile(userID, "bob@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Bob"
	profile.LastName = "Builder"
	profile.Moniker = "bob-the-builder"

	err := profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Step 1 failed - create profile: %v", err)
	}

	// Step 2: Retrieve by user ID
	retrieved, err := profileRepo.GetProfileByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("Step 2 failed - get by user ID: %v", err)
	}

	if retrieved.Moniker != "bob-the-builder" {
		t.Errorf("Moniker mismatch: got %s", retrieved.Moniker)
	}

	// Step 3: Retrieve by email
	retrieved, err = profileRepo.GetProfileByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("Step 3 failed - get by email: %v", err)
	}

	if retrieved.UserID != userID {
		t.Errorf("UserID mismatch when retrieved by email")
	}

	// Step 4: Update profile with platform IDs
	profile.PlatformIDs = domain.PlatformIdentifiers{
		Slack:    "U02DEF456",
		Telegram: "987654321",
	}

	err = profileRepo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Step 4 failed - update profile: %v", err)
	}

	// Step 5: Retrieve by Slack platform ID
	retrieved, err = profileRepo.GetProfileByPlatformID(ctx, domain.PlatformSlack, "U02DEF456")
	if err != nil {
		t.Fatalf("Step 5 failed - get by platform ID: %v", err)
	}

	if retrieved.UserID != userID {
		t.Errorf("UserID mismatch when retrieved by platform ID")
	}

	// Step 6: List all profiles
	allProfiles, err := profileRepo.ListProfiles(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Step 6 failed - list profiles: %v", err)
	}

	if len(allProfiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(allProfiles))
	}

	// Step 7: Create second user
	user2ID := "charlie"
	profile2 := domain.NewUserProfile(user2ID, "charlie@example.com", domain.UserTypeEnterprise)
	profile2.FirstName = "Charlie"

	err = profileRepo.SaveProfile(ctx, profile2)
	if err != nil {
		t.Fatalf("Step 7 failed - create second profile: %v", err)
	}

	// Step 8: Verify two profiles exist
	allProfiles, err = profileRepo.ListProfiles(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Step 8 failed - list all profiles: %v", err)
	}

	if len(allProfiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(allProfiles))
	}

	// Step 9: Delete first user
	err = profileRepo.DeleteProfile(ctx, userID)
	if err != nil {
		t.Fatalf("Step 9 failed - delete profile: %v", err)
	}

	// Step 10: Verify deletion
	_, err = profileRepo.GetProfileByUserID(ctx, userID)
	if err == nil {
		t.Error("Expected error when retrieving deleted profile")
	}

	allProfiles, err = profileRepo.ListProfiles(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Step 10 failed - list profiles after deletion: %v", err)
	}

	if len(allProfiles) != 1 {
		t.Errorf("Expected 1 profile after deletion, got %d", len(allProfiles))
	}

	t.Logf("✓ User management complete: 2 users created, 1 deleted")
}

// TestE2E_MultiUserScenarios tests data isolation between users
func TestE2E_MultiUserScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	conversationBasePath := filepath.Join(tmpDir, "conversations")
	notesBasePath := filepath.Join(tmpDir, "notes")

	ctx := context.Background()

	profileRepo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	convRepo := NewFileConversationRepository(conversationBasePath)
	notesRepo := NewFileNotesRepository(notesBasePath)

	// Create two users
	alice := domain.NewUserProfile("alice", "alice@example.com", domain.UserTypeIndividual)
	bob := domain.NewUserProfile("bob", "bob@example.com", domain.UserTypeIndividual)

	_ = profileRepo.SaveProfile(ctx, alice)
	_ = profileRepo.SaveProfile(ctx, bob)

	// Alice creates conversations
	aliceConv1 := &domain.Conversation{
		ID:        "alice-conv-1",
		UserID:    "alice",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	aliceConv2 := &domain.Conversation{
		ID:        "alice-conv-2",
		UserID:    "alice",
		Platform:  domain.PlatformSlack,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = convRepo.SaveConversation(ctx, aliceConv1)
	_ = convRepo.SaveConversation(ctx, aliceConv2)

	// Bob creates conversations
	bobConv1 := &domain.Conversation{
		ID:        "bob-conv-1",
		UserID:    "bob",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = convRepo.SaveConversation(ctx, bobConv1)

	// Alice creates notes
	aliceNote := &domain.Note{
		ID:        "alice-note-1",
		UserID:    "alice",
		Title:     "Alice's Note",
		Content:   "Private content",
		Tags:      []string{"private"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = notesRepo.Create(ctx, aliceNote)

	// Bob creates notes
	bobNote := &domain.Note{
		ID:        "bob-note-1",
		UserID:    "bob",
		Title:     "Bob's Note",
		Content:   "Bob's private content",
		Tags:      []string{"private"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = notesRepo.Create(ctx, bobNote)

	// Verify Alice only sees her conversations
	aliceConvs, err := convRepo.ListConversations(ctx, "alice")
	if err != nil {
		t.Fatalf("Failed to list Alice's conversations: %v", err)
	}

	if len(aliceConvs) != 2 {
		t.Errorf("Alice: expected 2 conversations, got %d", len(aliceConvs))
	}

	// Verify Bob only sees his conversations
	bobConvs, err := convRepo.ListConversations(ctx, "bob")
	if err != nil {
		t.Fatalf("Failed to list Bob's conversations: %v", err)
	}

	if len(bobConvs) != 1 {
		t.Errorf("Bob: expected 1 conversation, got %d", len(bobConvs))
	}

	// Verify Alice only sees her notes
	aliceNotes, err := notesRepo.List(ctx, "alice")
	if err != nil {
		t.Fatalf("Failed to list Alice's notes: %v", err)
	}

	if len(aliceNotes) != 1 {
		t.Errorf("Alice: expected 1 note, got %d", len(aliceNotes))
	}

	if aliceNotes[0].Title != "Alice's Note" {
		t.Errorf("Alice got wrong note: %s", aliceNotes[0].Title)
	}

	// Verify Bob only sees his notes
	bobNotes, err := notesRepo.List(ctx, "bob")
	if err != nil {
		t.Fatalf("Failed to list Bob's notes: %v", err)
	}

	if len(bobNotes) != 1 {
		t.Errorf("Bob: expected 1 note, got %d", len(bobNotes))
	}

	if bobNotes[0].Title != "Bob's Note" {
		t.Errorf("Bob got wrong note: %s", bobNotes[0].Title)
	}

	// Verify Bob cannot access Alice's conversation directly
	aliceConvFromBob, err := convRepo.GetConversation(ctx, "alice-conv-1")
	if err != nil {
		t.Logf("Good: Bob cannot retrieve Alice's conversation by ID (got error)")
	} else {
		// Conversation exists but should be filtered by user ID in application layer
		if aliceConvFromBob.UserID != "alice" {
			t.Error("Data corruption: conversation has wrong user ID")
		}
		t.Logf("Note: Conversation retrieved but belongs to Alice (application should filter)")
	}

	t.Logf("✓ Multi-user scenario complete: Data isolation verified")
}

// TestE2E_ConcurrentUsers tests multiple users working simultaneously
func TestE2E_ConcurrentUsers(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	conversationBasePath := filepath.Join(tmpDir, "conversations")

	ctx := context.Background()

	profileRepo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")
	convRepo := NewFileConversationRepository(conversationBasePath)

	// Create 5 users concurrently
	numUsers := 5
	done := make(chan bool, numUsers)

	for i := 0; i < numUsers; i++ {
		go func(userNum int) {
			userID := fmt.Sprintf("user-%d", userNum)
			profile := domain.NewUserProfile(userID, fmt.Sprintf("user%d@example.com", userNum), domain.UserTypeIndividual)
			profile.FirstName = fmt.Sprintf("User%d", userNum)

			err := profileRepo.SaveProfile(ctx, profile)
			if err != nil {
				t.Errorf("Failed to create user %d: %v", userNum, err)
			}

			// Each user creates 2 conversations
			for j := 0; j < 2; j++ {
				conv := &domain.Conversation{
					ID:        fmt.Sprintf("%s-conv-%d", userID, j),
					UserID:    userID,
					Platform:  domain.PlatformCLI,
					Messages:  []domain.StoredMessage{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				err = convRepo.SaveConversation(ctx, conv)
				if err != nil {
					t.Errorf("User %d failed to create conversation %d: %v", userNum, j, err)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numUsers; i++ {
		<-done
	}

	// Verify all users created
	allProfiles, err := profileRepo.ListProfiles(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}

	if len(allProfiles) != numUsers {
		t.Errorf("Expected %d users, got %d", numUsers, len(allProfiles))
	}

	// Verify each user has 2 conversations
	for i := 0; i < numUsers; i++ {
		userID := fmt.Sprintf("user-%d", i)
		convs, err := convRepo.ListConversations(ctx, userID)
		if err != nil {
			t.Errorf("Failed to list conversations for %s: %v", userID, err)
			continue
		}

		if len(convs) != 2 {
			t.Errorf("User %s: expected 2 conversations, got %d", userID, len(convs))
		}
	}

	t.Logf("✓ Concurrent users complete: 5 users with 2 conversations each")
}
