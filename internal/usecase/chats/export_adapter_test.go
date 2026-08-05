package chats

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// TestConversationExportAdapter_FullInterface exercises every
// conversationExportAdapter method directly. ExportConversation itself only
// calls GetConversation/GetRecentMessages, so the remaining three methods
// (present only to satisfy chat.MemoryRepository's shape) are otherwise
// untested by the Service-level tests in service_test.go.
func TestConversationExportAdapter_FullInterface(t *testing.T) {
	repo := storage.NewFileConversationRepository(t.TempDir())
	adapter := &conversationExportAdapter{repo: repo}
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:        "conv-1",
		UserID:    "user-a",
		Platform:  domain.PlatformWeb,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	if err := adapter.SaveMessage(ctx, "conv-1", "user-a", domain.PlatformWeb, domain.StoredMessage{ID: "m1", Role: "user", Content: "hi", Timestamp: time.Now()}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	got, err := adapter.GetConversation(ctx, "conv-1")
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message after SaveMessage, got %d", len(got.Messages))
	}

	msgs, err := adapter.GetRecentMessages(ctx, "conv-1", 1000)
	if err != nil {
		t.Fatalf("GetRecentMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 recent message, got %d", len(msgs))
	}

	summaries, err := adapter.ListConversations(ctx, "user-a")
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != "conv-1" {
		t.Fatalf("expected 1 summary for conv-1, got %+v", summaries)
	}

	if err := adapter.DeleteConversation(ctx, "conv-1"); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	if _, err := adapter.GetConversation(ctx, "conv-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after DeleteConversation, got %v", err)
	}
}

func TestConversationExportAdapter_GetRecentMessages_NotFound(t *testing.T) {
	repo := storage.NewFileConversationRepository(t.TempDir())
	adapter := &conversationExportAdapter{repo: repo}
	_, err := adapter.GetRecentMessages(context.Background(), "missing", 100)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
