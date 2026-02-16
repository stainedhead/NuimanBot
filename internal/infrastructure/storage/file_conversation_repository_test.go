package storage

import (
	"context"
	"nuimanbot/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestFileConversationRepository_SaveConversation(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	conv := &domain.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Verify can retrieve conversation
	retrieved, err := repo.GetConversation(ctx, "conv-123")
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if retrieved.ID != "conv-123" {
		t.Errorf("expected ID conv-123, got %s", retrieved.ID)
	}
	if retrieved.UserID != "user-456" {
		t.Errorf("expected UserID user-456, got %s", retrieved.UserID)
	}
}

func TestFileConversationRepository_AppendMessage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	conv := &domain.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Append a message
	msg := domain.StoredMessage{
		ID:         "msg-1",
		Role:       "user",
		Content:    "Hello",
		TokenCount: 10,
		Timestamp:  time.Now(),
	}

	err = repo.AppendMessage(ctx, "conv-123", msg)
	if err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}

	// Verify message was appended
	retrieved, err := repo.GetConversation(ctx, "conv-123")
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(retrieved.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(retrieved.Messages))
	}
	if retrieved.Messages[0].Content != "Hello" {
		t.Errorf("expected content 'Hello', got %s", retrieved.Messages[0].Content)
	}
}

func TestFileConversationRepository_ListConversations(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	ctx := context.Background()

	// Create multiple conversations for same user
	for i := 0; i < 3; i++ {
		conv := &domain.Conversation{
			ID:        string(rune('a'+i)) + "-conv",
			UserID:    "user-456",
			Platform:  domain.PlatformCLI,
			Messages:  []domain.StoredMessage{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := repo.SaveConversation(ctx, conv)
		if err != nil {
			t.Fatalf("SaveConversation failed: %v", err)
		}
	}

	// List conversations
	summaries, err := repo.ListConversations(ctx, "user-456")
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(summaries))
	}
}

func TestFileConversationRepository_DeleteConversation(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	conv := &domain.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Delete conversation
	err = repo.DeleteConversation(ctx, "conv-123")
	if err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}

	// Verify deletion
	_, err = repo.GetConversation(ctx, "conv-123")
	if err == nil {
		t.Error("expected error when getting deleted conversation")
	}
}

func TestFileConversationRepository_CountMessages(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	conv := &domain.Conversation{
		ID:        "conv-123",
		UserID:    "user-456",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Append multiple messages
	for i := 0; i < 5; i++ {
		msg := domain.StoredMessage{
			ID:         string(rune('a' + i)),
			Role:       "user",
			Content:    "Message " + string(rune('0'+i)),
			TokenCount: 10,
			Timestamp:  time.Now(),
		}
		err = repo.AppendMessage(ctx, "conv-123", msg)
		if err != nil {
			t.Fatalf("AppendMessage failed: %v", err)
		}
	}

	// Count messages
	count, err := repo.CountMessages(ctx, "conv-123")
	if err != nil {
		t.Fatalf("CountMessages failed: %v", err)
	}

	if count != 5 {
		t.Errorf("expected 5 messages, got %d", count)
	}
}

func TestFileConversationRepository_LargeConversation(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "data")

	repo := NewFileConversationRepository(basePath)

	conv := &domain.Conversation{
		ID:        "conv-large",
		UserID:    "user-456",
		Platform:  domain.PlatformCLI,
		Messages:  []domain.StoredMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	err := repo.SaveConversation(ctx, conv)
	if err != nil {
		t.Fatalf("SaveConversation failed: %v", err)
	}

	// Append 1000 messages (testing scalability)
	for i := 0; i < 1000; i++ {
		msg := domain.StoredMessage{
			ID:         string(rune(i)),
			Role:       "user",
			Content:    "Message content here with some text to make it realistic",
			TokenCount: 15,
			Timestamp:  time.Now(),
		}
		err = repo.AppendMessage(ctx, "conv-large", msg)
		if err != nil {
			t.Fatalf("AppendMessage %d failed: %v", i, err)
		}
	}

	// Retrieve and verify
	start := time.Now()
	retrieved, err := repo.GetConversation(ctx, "conv-large")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}

	if len(retrieved.Messages) != 1000 {
		t.Errorf("expected 1000 messages, got %d", len(retrieved.Messages))
	}

	// Performance check: should load < 100ms (p90 target)
	if duration > 150*time.Millisecond {
		t.Logf("Warning: Load time %v exceeds target of 100ms", duration)
	}
}
