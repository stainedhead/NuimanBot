package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// ExportFormat defines the format for conversation export.
type ExportFormat string

const (
	ExportFormatJSON     ExportFormat = "json"
	ExportFormatMarkdown ExportFormat = "markdown"
)

// ConversationExport represents an exported conversation.
type ConversationExport struct {
	ConversationID string                 `json:"conversation_id"`
	UserID         string                 `json:"user_id"`
	Platform       string                 `json:"platform"`
	ExportedAt     time.Time              `json:"exported_at"`
	MessageCount   int                    `json:"message_count"`
	Messages       []domain.StoredMessage `json:"messages"`
}

// ExportConversation exports a conversation in the specified format.
func (s *Service) ExportConversation(ctx context.Context, conversationID string, format ExportFormat) (string, error) {
	// Get conversation metadata
	conversation, err := s.memoryRepo.GetConversation(ctx, conversationID)
	if err != nil {
		return "", fmt.Errorf("failed to get conversation: %w", err)
	}

	// Get all messages (use large token limit to get everything)
	messages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 1000000)
	if err != nil {
		return "", fmt.Errorf("failed to get messages: %w", err)
	}

	// Create export structure
	export := ConversationExport{
		ConversationID: conversationID,
		UserID:         conversation.UserID,
		Platform:       string(conversation.Platform),
		ExportedAt:     time.Now(),
		MessageCount:   len(messages),
		Messages:       messages,
	}

	// Format based on requested type
	switch format {
	case ExportFormatJSON:
		return exportJSON(export)
	case ExportFormatMarkdown:
		return exportMarkdown(export)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportJSON exports conversation as JSON.
func exportJSON(export ConversationExport) (string, error) {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// exportMarkdown exports conversation as Markdown.
func exportMarkdown(export ConversationExport) (string, error) {
	var builder strings.Builder

	// Header
	builder.WriteString("# Conversation Export\n\n")
	builder.WriteString(fmt.Sprintf("**Conversation ID:** %s\n", export.ConversationID))
	builder.WriteString(fmt.Sprintf("**User ID:** %s\n", export.UserID))
	builder.WriteString(fmt.Sprintf("**Platform:** %s\n", export.Platform))
	builder.WriteString(fmt.Sprintf("**Exported:** %s\n", export.ExportedAt.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("**Message Count:** %d\n\n", export.MessageCount))
	builder.WriteString("---\n\n")

	// Messages
	builder.WriteString("## Messages\n\n")
	for i, msg := range export.Messages {
		// Message header
		builder.WriteString(fmt.Sprintf("### Message %d\n\n", i+1))
		builder.WriteString(fmt.Sprintf("**Role:** %s\n", msg.Role))
		builder.WriteString(fmt.Sprintf("**Timestamp:** %s\n", msg.Timestamp.Format(time.RFC3339)))
		if msg.TokenCount > 0 {
			builder.WriteString(fmt.Sprintf("**Tokens:** %d\n", msg.TokenCount))
		}
		builder.WriteString("\n")

		// Message content
		builder.WriteString("**Content:**\n\n")
		builder.WriteString("```\n")
		builder.WriteString(msg.Content)
		builder.WriteString("\n```\n\n")
		builder.WriteString("---\n\n")
	}

	return builder.String(), nil
}
