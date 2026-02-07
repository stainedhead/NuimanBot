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

func createTestRepository(t *testing.T) (*sqlite.MessageRepository, func()) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign key constraints
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Create repository and initialize schema
	repo := sqlite.NewMessageRepository(db)
	if err := repo.Init(context.Background()); err != nil {
		db.Close()
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return repo, cleanup
}

func TestMessageBatcher_Add(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	batcher := sqlite.NewMessageBatcher(repo, 10, 100*time.Millisecond)
	defer batcher.Stop()

	// Add a message
	msg := domain.StoredMessage{
		ID:      "msg-1",
		Role:    "user",
		Content: "test message",
	}

	err := batcher.Add(ctx, "conv-1", "user-1", domain.PlatformCLI, msg)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Wait for flush
	time.Sleep(150 * time.Millisecond)

	// Verify message was saved
	conv, err := repo.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}

	if len(conv.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conv.Messages))
	}

	if conv.Messages[0].ID != "msg-1" {
		t.Errorf("Expected message ID 'msg-1', got %q", conv.Messages[0].ID)
	}
}

func TestMessageBatcher_SizeBasedFlush(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	// Set batch size to 3
	batcher := sqlite.NewMessageBatcher(repo, 3, 1*time.Second)
	defer batcher.Stop()

	// Add 3 messages (should trigger flush)
	for i := 1; i <= 3; i++ {
		msg := domain.StoredMessage{
			ID:      "msg-" + string(rune('0'+i)),
			Role:    "user",
			Content: "test message",
		}
		err := batcher.Add(ctx, "conv-1", "user-1", domain.PlatformCLI, msg)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Give a moment for flush to complete
	time.Sleep(50 * time.Millisecond)

	// Verify all messages were saved
	conv, err := repo.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}

	if len(conv.Messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(conv.Messages))
	}
}

func TestMessageBatcher_TimeBasedFlush(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	// Short flush interval
	batcher := sqlite.NewMessageBatcher(repo, 100, 100*time.Millisecond)
	defer batcher.Stop()

	// Add 2 messages (below batch size)
	for i := 1; i <= 2; i++ {
		msg := domain.StoredMessage{
			ID:      "msg-" + string(rune('0'+i)),
			Role:    "user",
			Content: "test message",
		}
		err := batcher.Add(ctx, "conv-1", "user-1", domain.PlatformCLI, msg)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Wait for time-based flush
	time.Sleep(150 * time.Millisecond)

	// Verify messages were saved
	conv, err := repo.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}

	if len(conv.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(conv.Messages))
	}
}

func TestMessageBatcher_Flush(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	// Long flush interval so we can test manual flush
	batcher := sqlite.NewMessageBatcher(repo, 100, 10*time.Second)
	defer batcher.Stop()

	// Add messages
	for i := 1; i <= 5; i++ {
		msg := domain.StoredMessage{
			ID:      "msg-" + string(rune('0'+i)),
			Role:    "user",
			Content: "test message",
		}
		err := batcher.Add(ctx, "conv-1", "user-1", domain.PlatformCLI, msg)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Manually flush
	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	// Verify all messages were saved
	conv, err := repo.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}

	if len(conv.Messages) != 5 {
		t.Fatalf("Expected 5 messages, got %d", len(conv.Messages))
	}
}

func TestMessageBatcher_Stop(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	batcher := sqlite.NewMessageBatcher(repo, 100, 1*time.Second)

	// Add messages
	for i := 1; i <= 3; i++ {
		msg := domain.StoredMessage{
			ID:      "msg-" + string(rune('0'+i)),
			Role:    "user",
			Content: "test message",
		}
		err := batcher.Add(ctx, "conv-1", "user-1", domain.PlatformCLI, msg)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Stop (should flush remaining messages)
	batcher.Stop()

	// Verify messages were flushed on stop
	conv, err := repo.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation() error = %v", err)
	}

	if len(conv.Messages) != 3 {
		t.Fatalf("Expected 3 messages after stop, got %d", len(conv.Messages))
	}
}

func TestMessageBatcher_MultipleConversations(t *testing.T) {
	repo, cleanup := createTestRepository(t)
	defer cleanup()

	ctx := context.Background()
	batcher := sqlite.NewMessageBatcher(repo, 10, 100*time.Millisecond)
	defer batcher.Stop()

	// Add messages to different conversations
	for i := 1; i <= 3; i++ {
		convID := "conv-" + string(rune('0'+i))
		msg := domain.StoredMessage{
			ID:      "msg-" + string(rune('0'+i)),
			Role:    "user",
			Content: "test message",
		}
		err := batcher.Add(ctx, convID, "user-1", domain.PlatformCLI, msg)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Wait for flush
	time.Sleep(150 * time.Millisecond)

	// Verify each conversation has its message
	for i := 1; i <= 3; i++ {
		convID := "conv-" + string(rune('0'+i))
		conv, err := repo.GetConversation(ctx, convID)
		if err != nil {
			t.Fatalf("GetConversation(%s) error = %v", convID, err)
		}

		if len(conv.Messages) != 1 {
			t.Errorf("Expected 1 message in %s, got %d", convID, len(conv.Messages))
		}
	}
}
