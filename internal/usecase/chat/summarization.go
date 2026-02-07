package chat

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// SummarizeConversation creates a summary of conversation messages.
// This is useful for compressing old messages when the context window is exceeded.
func (s *Service) SummarizeConversation(ctx context.Context, conversationID string, maxTokens int) (string, error) {
	// Get messages to summarize
	messages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, maxTokens)
	if err != nil {
		return "", fmt.Errorf("failed to get messages for summarization: %w", err)
	}

	if len(messages) == 0 {
		return "", fmt.Errorf("no messages to summarize")
	}

	// Build summarization prompt
	prompt := buildSummarizationPrompt(messages)

	// Request summary from LLM
	llmRequest := &domain.LLMRequest{
		Model:       "claude-3-haiku-20240307", // Use cheaper model for summarization
		Messages:    []domain.Message{{Role: "user", Content: prompt}},
		MaxTokens:   500, // Limit summary length
		Temperature: 0.3, // Lower temperature for consistent summaries
		SystemPrompt: `You are a conversation summarizer. Create concise summaries that preserve:
- Key facts, dates, numbers, and names
- Important decisions or agreements
- Action items or requests
- Overall conversation context
Be specific and factual. Avoid generic statements.`,
	}

	response, err := s.llmService.Complete(ctx, domain.LLMProviderAnthropic, llmRequest)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	return response.Content, nil
}

// buildSummarizationPrompt constructs the prompt for conversation summarization
func buildSummarizationPrompt(messages []domain.StoredMessage) string {
	var builder strings.Builder
	builder.WriteString("Please summarize the following conversation:\n\n")

	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	builder.WriteString("\nProvide a concise summary that captures the key points, facts, and context.")
	return builder.String()
}
