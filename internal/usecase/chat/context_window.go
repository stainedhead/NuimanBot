package chat

import (
	"context"

	"nuimanbot/internal/domain"
)

// Provider token limits (maximum context window sizes)
const (
	AnthropicTokenLimit = 200000 // Claude 3 Opus/Sonnet: 200k tokens
	OpenAITokenLimit    = 128000 // GPT-4 Turbo: 128k tokens
	OllamaTokenLimit    = 32000  // Default for local models: 32k tokens
	ReservedTokens      = 2000   // Reserve tokens for response generation
	MemoryTokenBudget   = 2000   // Token budget for recalled long-term memories
)

// BuildContextWindow constructs a context window from conversation history
// that fits within the provider's token limit.
// When a query is provided and a MemoryRecaller is configured, recalled memories
// are prepended as a system message. Memory recall failures degrade gracefully.
// Returns messages (with optional memory prefix) and total token count.
func (s *Service) BuildContextWindow(ctx context.Context, conversationID string, provider domain.LLMProvider, maxTokens int, query string) ([]domain.Message, int) {
	// Get provider-specific token limit if maxTokens is 0 or exceeds limit
	providerLimit := getProviderTokenLimit(provider)
	if maxTokens == 0 || maxTokens > providerLimit {
		maxTokens = providerLimit
	}

	// Reserve tokens for the response (only if we're using provider limits)
	reservedTokens := 0
	if maxTokens == providerLimit {
		reservedTokens = ReservedTokens
	}

	availableTokens := maxTokens - reservedTokens

	// Recall long-term memories if recaller is configured and query is provided
	var memoryContent string
	var memoryTokens int
	if s.memoryRecaller != nil && query != "" {
		budget := MemoryTokenBudget
		if budget > availableTokens/4 {
			budget = availableTokens / 4 // Cap at 25% of available tokens
		}
		recalled, err := s.memoryRecaller.RecallAndFormat(ctx, conversationID, query, budget)
		if err == nil && recalled != "" {
			memoryContent = recalled
			memoryTokens = estimateTokens(memoryContent)
			availableTokens -= memoryTokens
		}
		// On error, silently degrade - continue without memories
	}

	// Get recent messages up to the available token limit
	recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, availableTokens)
	if err != nil || len(recentMessages) == 0 {
		if memoryContent != "" {
			// Even with no conversation messages, return memory if available
			return []domain.Message{{Role: "system", Content: memoryContent}}, memoryTokens
		}
		return []domain.Message{}, 0
	}

	// Convert stored messages to domain messages
	// Add from newest to oldest until we hit the token limit
	messages := make([]domain.Message, 0, len(recentMessages)+1)
	totalTokens := 0

	// Iterate from newest to oldest
	for i := len(recentMessages) - 1; i >= 0; i-- {
		msg := recentMessages[i]

		// Check if adding this message would exceed limit
		if totalTokens+msg.TokenCount > availableTokens {
			break
		}

		// Prepend message (maintain chronological order)
		messages = append([]domain.Message{{
			Role:    msg.Role,
			Content: msg.Content,
		}}, messages...)

		totalTokens += msg.TokenCount
	}

	// Prepend memory as system message if available
	if memoryContent != "" {
		messages = append([]domain.Message{{
			Role:    "system",
			Content: memoryContent,
		}}, messages...)
		totalTokens += memoryTokens
	}

	return messages, totalTokens
}

// estimateTokens estimates the token count for text using a simple heuristic.
// Approximation: ~4 characters per token.
func estimateTokens(text string) int {
	return (len(text) + 3) / 4
}

// getProviderTokenLimit returns the maximum context window size for the given provider.
func getProviderTokenLimit(provider domain.LLMProvider) int {
	switch provider {
	case domain.LLMProviderAnthropic:
		return AnthropicTokenLimit
	case domain.LLMProviderOpenAI:
		return OpenAITokenLimit
	case domain.LLMProviderOllama:
		return OllamaTokenLimit
	default:
		return OllamaTokenLimit // Default to smallest limit for safety
	}
}
