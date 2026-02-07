package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

func TestExportConversation_JSON(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getConversationFunc: func(ctx context.Context, convID string) (*domain.Conversation, error) {
			return &domain.Conversation{
				ID:       convID,
				UserID:   "user123",
				Platform: domain.PlatformCLI,
			}, nil
		},
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", Timestamp: time.Now(), TokenCount: 10},
				{ID: "msg2", Role: "assistant", Content: "Hi there!", Timestamp: time.Now(), TokenCount: 15},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	exported, err := service.ExportConversation(context.Background(), "conv-123", ExportFormatJSON)
	if err != nil {
		t.Fatalf("ExportConversation failed: %v", err)
	}

	// Verify it's valid JSON
	var export ConversationExport
	if err := json.Unmarshal([]byte(exported), &export); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if export.ConversationID != "conv-123" {
		t.Errorf("Expected conversation_id 'conv-123', got %s", export.ConversationID)
	}

	if export.MessageCount != 2 {
		t.Errorf("Expected 2 messages, got %d", export.MessageCount)
	}
}

func TestExportConversation_Markdown(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getConversationFunc: func(ctx context.Context, convID string) (*domain.Conversation, error) {
			return &domain.Conversation{
				ID:       convID,
				UserID:   "user123",
				Platform: domain.PlatformCLI,
			}, nil
		},
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{
				{ID: "msg1", Role: "user", Content: "Hello", Timestamp: time.Now(), TokenCount: 10},
				{ID: "msg2", Role: "assistant", Content: "Hi there!", Timestamp: time.Now(), TokenCount: 15},
			}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	exported, err := service.ExportConversation(context.Background(), "conv-123", ExportFormatMarkdown)
	if err != nil {
		t.Fatalf("ExportConversation failed: %v", err)
	}

	// Verify markdown structure
	if !strings.Contains(exported, "# Conversation Export") {
		t.Error("Expected markdown header")
	}

	if !strings.Contains(exported, "**Conversation ID:** conv-123") {
		t.Error("Expected conversation ID in markdown")
	}

	if !strings.Contains(exported, "### Message 1") {
		t.Error("Expected message headers in markdown")
	}

	if !strings.Contains(exported, "Hello") && !strings.Contains(exported, "Hi there!") {
		t.Error("Expected message content in markdown")
	}
}

func TestExportConversation_UnsupportedFormat(t *testing.T) {
	memoryRepo := &mockMemoryRepository{
		getConversationFunc: func(ctx context.Context, convID string) (*domain.Conversation, error) {
			return &domain.Conversation{ID: convID}, nil
		},
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	service := createTestService(&mockLLMService{}, memoryRepo, &mockToolExecutionService{}, &mockSecurityService{})

	_, err := service.ExportConversation(context.Background(), "conv-123", ExportFormat("xml"))
	if err == nil {
		t.Fatal("Expected error for unsupported format")
	}

	if !strings.Contains(err.Error(), "unsupported export format") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
