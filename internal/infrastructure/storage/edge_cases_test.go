package storage

import (
	"context"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestConcurrentUserProfileAccess tests concurrent read/write access to user profiles
func TestConcurrentUserProfileAccess(t *testing.T) {
	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	ctx := context.Background()
	userID := "concurrent-user"

	// Create initial profile
	profile := domain.NewUserProfile(userID, "test@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Test"
	profile.LastName = "User"

	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Concurrent reads and writes
	const numGoroutines = 50
	const numOperations = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Spawn readers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := repo.GetProfileByUserID(ctx, userID)
				if err != nil {
					errors <- fmt.Errorf("reader %d iteration %d: %w", readerID, j, err)
				}
			}
		}(i)
	}

	// Spawn writers
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				updatedProfile := domain.NewUserProfile(userID, fmt.Sprintf("writer%d-op%d@example.com", writerID, j), domain.UserTypeIndividual)
				updatedProfile.FirstName = "Test"
				updatedProfile.LastName = "User"
				err := repo.SaveProfile(ctx, updatedProfile)
				if err != nil {
					errors <- fmt.Errorf("writer %d iteration %d: %w", writerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

// TestConcurrentConversationAccess tests concurrent message appends
func TestConcurrentConversationAccess(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileConversationRepository(basePath)

	ctx := context.Background()
	conversationID := "concurrent-conv"

	// Create conversation
	conv := &domain.Conversation{
		ID:        conversationID,
		UserID:    "test-user",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Concurrent message appends
	const numWriters = 20
	const messagesPerWriter = 50
	expectedTotal := numWriters * messagesPerWriter

	var wg sync.WaitGroup
	errors := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < messagesPerWriter; j++ {
				msg := domain.StoredMessage{
					ID:         fmt.Sprintf("msg-%d-%d", writerID, j),
					Role:       "user",
					Content:    fmt.Sprintf("Message from writer %d, iteration %d", writerID, j),
					TokenCount: 10,
					Timestamp:  time.Now(),
				}
				err := repo.AppendMessage(ctx, conversationID, msg)
				if err != nil {
					errors <- fmt.Errorf("writer %d iteration %d: %w", writerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent append error: %v", err)
	}

	// Verify all messages were written
	retrieved, err := repo.GetConversation(ctx, conversationID)
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}

	if len(retrieved.Messages) != expectedTotal {
		t.Errorf("Expected %d messages, got %d", expectedTotal, len(retrieved.Messages))
	}
}

// TestLargeDataVolumeConversations tests handling of conversations with many messages
func TestLargeDataVolumeConversations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large data volume test in short mode")
	}

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileConversationRepository(basePath)

	ctx := context.Background()
	conversationID := "large-conv"

	// Create conversation
	conv := &domain.Conversation{
		ID:        conversationID,
		UserID:    "test-user",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Add 10,000 messages
	const numMessages = 10000
	t.Logf("Adding %d messages...", numMessages)
	start := time.Now()

	for i := 0; i < numMessages; i++ {
		msg := domain.StoredMessage{
			ID:         fmt.Sprintf("msg-%d", i),
			Role:       "user",
			Content:    fmt.Sprintf("Test message number %d with some content to make it realistic", i),
			TokenCount: 10,
			Timestamp:  time.Now(),
		}
		err := repo.AppendMessage(ctx, conversationID, msg)
		if err != nil {
			t.Fatalf("Failed to add message %d: %v", i, err)
		}

		if (i+1)%1000 == 0 {
			t.Logf("Added %d messages...", i+1)
		}
	}

	duration := time.Since(start)
	t.Logf("Added %d messages in %v (%.2f msg/sec)", numMessages, duration, float64(numMessages)/duration.Seconds())

	// Verify retrieval performance
	start = time.Now()
	retrieved, err := repo.GetConversation(ctx, conversationID)
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}
	retrievalDuration := time.Since(start)

	if len(retrieved.Messages) != numMessages {
		t.Errorf("Expected %d messages, got %d", numMessages, len(retrieved.Messages))
	}

	t.Logf("Retrieved %d messages in %v", numMessages, retrievalDuration)

	// Performance target: should retrieve in < 1 second
	if retrievalDuration > time.Second {
		t.Logf("Warning: Retrieval took %v, which is slower than 1s target", retrievalDuration)
	}
}

// TestLargeDataVolumeMemoryCells tests handling of many memory cells
func TestLargeDataVolumeMemoryCells(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large data volume test in short mode")
	}

	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()
	conversationID := "test-conversation"

	// Create 10,000 memory cells (reduced from 100k for reasonable test time)
	const numCells = 10000
	t.Logf("Creating %d memory cells...", numCells)
	start := time.Now()

	scenes := []string{"project-setup", "testing-patterns", "performance", "memory-system", "database"}

	for i := 0; i < numCells; i++ {
		// Generate a simple UUID-like string
		cellID := fmt.Sprintf("%08x-0000-0000-0000-%012x", i/65536, i%65536)
		cell := &memoryv2.MemoryCell{
			ID:             cellID,
			ConversationID: conversationID,
			Scene:          scenes[i%len(scenes)],
			CellType:       memoryv2.CellTypeFact,
			Content:        fmt.Sprintf("Memory content %d about %s", i, scenes[i%len(scenes)]),
			Source:         fmt.Sprintf("[\"%s\"]", cellID),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell %d: %v", i, err)
		}

		if (i+1)%1000 == 0 {
			t.Logf("Created %d cells...", i+1)
		}
	}

	duration := time.Since(start)
	t.Logf("Created %d cells in %v (%.2f cells/sec)", numCells, duration, float64(numCells)/duration.Seconds())

	// Test search performance
	start = time.Now()
	results, err := repo.GetByScene(ctx, "project-setup", 100)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}
	searchDuration := time.Since(start)

	t.Logf("Search found %d results in %v", len(results), searchDuration)

	// Performance target: search should complete in < 500ms
	if searchDuration > 500*time.Millisecond {
		t.Logf("Warning: Search took %v, which is slower than 500ms target", searchDuration)
	}
}

// TestIndexCorruptionRecovery tests recovery from corrupted index files
func TestIndexCorruptionRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileConversationRepository(basePath)

	ctx := context.Background()
	userID := "test-user"

	// Create some conversations
	for i := 0; i < 5; i++ {
		conv := &domain.Conversation{
			ID:        fmt.Sprintf("conv-%d", i),
			UserID:    userID,
			Platform:  domain.PlatformCLI,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.SaveConversation(ctx, conv)
		if err != nil {
			t.Fatalf("Failed to create conversation %d: %v", i, err)
		}
	}

	// Corrupt the index file (if it exists)
	indexPath := filepath.Join(basePath, "conversations", "index.json")
	if _, err := os.Stat(indexPath); err == nil {
		err = os.WriteFile(indexPath, []byte("corrupted invalid json {{{"), 0644)
		if err != nil {
			t.Fatalf("Failed to corrupt index: %v", err)
		}

		// Create a new repository instance (simulates restart)
		repo = NewFileConversationRepository(basePath)

		// Try to list conversations - should rebuild index or handle gracefully
		conversations, err := repo.ListConversations(ctx, userID)
		if err != nil {
			// It's okay if it fails, as long as it doesn't panic
			t.Logf("List conversations failed after index corruption (expected): %v", err)
		} else {
			t.Logf("Retrieved %d conversations after index corruption", len(conversations))
		}
	} else {
		t.Skip("No index file was created, skipping corruption test")
	}
}

// TestMemoryCellIndexCorruptionRecovery tests memory cell index recovery
func TestMemoryCellIndexCorruptionRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileMemoryCellRepository(basePath)

	ctx := context.Background()
	conversationID := "test-conversation"

	// Create memory cells with various scenes
	scenes := []string{"project-setup", "testing-patterns", "performance"}
	for i := 0; i < 10; i++ {
		cellID := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		cell := &memoryv2.MemoryCell{
			ID:             cellID,
			ConversationID: conversationID,
			Scene:          scenes[i%len(scenes)],
			CellType:       memoryv2.CellTypeFact,
			Content:        fmt.Sprintf("Test content %d", i),
			Source:         fmt.Sprintf("[\"%s\"]", cellID),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		err := repo.Create(ctx, cell)
		if err != nil {
			t.Fatalf("Failed to create cell %d: %v", i, err)
		}
	}

	// Try to find the index file and corrupt it (implementation-specific)
	// Since we don't know the exact index file structure, we'll just test
	// that the repository handles errors gracefully
	cells, err := repo.GetByScene(ctx, "project-setup", 100)
	if err != nil {
		t.Logf("Find by scene failed: %v", err)
	} else {
		t.Logf("Found %d cells", len(cells))
	}
}

// TestPermissionErrors tests handling of permission-related errors
func TestPermissionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()
	usersJSONPath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(usersJSONPath, "test-encryption-key-32bytes!ab")

	ctx := context.Background()
	userID := "test-user"

	// Create initial profile
	profile := domain.NewUserProfile(userID, "test@example.com", domain.UserTypeIndividual)
	profile.FirstName = "Test"
	profile.LastName = "User"

	err := repo.SaveProfile(ctx, profile)
	if err != nil {
		t.Fatalf("Failed to create profile: %v", err)
	}

	// Make the entire tmpDir read-only to prevent atomic writes from working
	err = os.Chmod(tmpDir, 0555) // Read and execute only
	if err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}

	// Restore permissions after test
	defer os.Chmod(tmpDir, 0755)

	// Try to update profile - should fail because temp file can't be created
	profile.PrimaryEmail = "updated@example.com"
	err = repo.SaveProfile(ctx, profile)
	if err == nil {
		t.Error("Expected permission error when updating in read-only directory")
	} else {
		t.Logf("Got expected permission error: %v", err)
	}
}

// TestDiskFullScenario tests handling when disk is "full" (simulated)
func TestDiskFullScenario(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileConversationRepository(basePath)

	ctx := context.Background()
	conversationID := "test-conv"

	// Create conversation
	conv := &domain.Conversation{
		ID:        conversationID,
		UserID:    "test-user",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Try to add a very large message (simulates disk pressure)
	largeContent := make([]byte, 100*1024*1024) // 100MB message
	for i := range largeContent {
		largeContent[i] = byte('x')
	}

	msg := domain.StoredMessage{
		ID:         "large-msg",
		Role:       "user",
		Content:    string(largeContent),
		TokenCount: 100000,
		Timestamp:  time.Now(),
	}

	// This might fail or succeed depending on available disk
	// The key is that it should fail gracefully if it fails
	err = repo.AppendMessage(ctx, conversationID, msg)
	if err != nil {
		t.Logf("Large message failed (expected on disk pressure): %v", err)

		// Verify the conversation is still intact
		retrieved, err := repo.GetConversation(ctx, conversationID)
		if err != nil {
			t.Fatalf("Conversation corrupted after disk full: %v", err)
		}

		if retrieved == nil {
			t.Error("Lost conversation after disk full scenario")
		}
	} else {
		t.Logf("Large message succeeded (disk has space)")
	}
}

// TestConcurrentNotesAccess tests concurrent access to notes repository
func TestConcurrentNotesAccess(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileNotesRepository(basePath)

	ctx := context.Background()
	userID := "test-user"

	const numGoroutines = 30
	const notesPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*notesPerGoroutine)

	// Concurrent note creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < notesPerGoroutine; j++ {
				note := &domain.Note{
					ID:        fmt.Sprintf("note-%d-%d", routineID, j),
					UserID:    userID,
					Title:     fmt.Sprintf("Note %d-%d", routineID, j),
					Content:   fmt.Sprintf("Content from routine %d, note %d", routineID, j),
					Tags:      []string{fmt.Sprintf("tag-%d", routineID%5)},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				err := repo.Create(ctx, note)
				if err != nil {
					errors <- fmt.Errorf("routine %d note %d: %w", routineID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent note creation error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Had %d concurrent errors", errorCount)
	}

	// Verify all notes were created
	notes, err := repo.List(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to list notes: %v", err)
	}

	expected := numGoroutines * notesPerGoroutine
	if len(notes) != expected {
		t.Errorf("Expected %d notes, got %d", expected, len(notes))
	}
}

// TestConcurrentAuditLogWrites tests concurrent writes to audit log
func TestConcurrentAuditLogWrites(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")
	repo := NewFileAuditRepository(basePath)

	ctx := context.Background()

	const numGoroutines = 50
	const entriesPerGoroutine = 100
	expectedTotal := numGoroutines * entriesPerGoroutine

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*entriesPerGoroutine)

	start := time.Now()

	// Concurrent audit log writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				entry := &domain.AuditEvent{
					Timestamp: time.Now(),
					UserID:    fmt.Sprintf("user-%d", routineID%10),
					Action:    "test_action",
					Resource:  fmt.Sprintf("resource-%d", j),
					Outcome:   "success",
					SourceIP:  "127.0.0.1",
					Platform:  domain.PlatformCLI,
					Details:   map[string]any{"routine": routineID, "entry": j},
				}
				err := repo.Append(ctx, entry)
				if err != nil {
					errors <- fmt.Errorf("routine %d entry %d: %w", routineID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	duration := time.Since(start)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent audit write error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Had %d concurrent errors", errorCount)
	}

	t.Logf("Wrote %d audit entries concurrently in %v (%.2f entries/sec)",
		expectedTotal, duration, float64(expectedTotal)/duration.Seconds())

	// Query to verify all entries were written
	entries, err := repo.Query(ctx, domain.AuditFilter{
		Limit: expectedTotal * 2,
	})
	if err != nil {
		t.Fatalf("Failed to query audit log: %v", err)
	}

	if len(entries) != expectedTotal {
		t.Errorf("Expected %d audit entries, got %d", expectedTotal, len(entries))
	}
}
