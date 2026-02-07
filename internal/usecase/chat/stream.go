package chat

import (
	"context"
	"fmt"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/requestid"
)

// ProcessMessageStream processes a message and returns streaming chunks.
// Note: Streaming currently does NOT support tool calls. If tool calls are detected,
// the stream will fall back to non-streaming mode (Complete) and return a single chunk.
func (s *Service) ProcessMessageStream(ctx context.Context, incomingMsg *domain.IncomingMessage) (<-chan domain.StreamChunk, error) {
	ctx, reqID := requestid.MustFromContext(ctx)
	logger := requestid.Logger(ctx)

	logger.Info("Processing message (streaming)",
		"platform", incomingMsg.Platform,
		"user", incomingMsg.PlatformUID,
		"request_id", reqID,
	)

	// Output channel
	outCh := make(chan domain.StreamChunk, 10)

	// Process in background goroutine
	go func() {
		defer close(outCh)

		// 1. Validate Input
		validatedInput, err := s.securityService.ValidateInput(ctx, incomingMsg.Text, 32768)
		if err != nil {
			outCh <- domain.StreamChunk{Error: fmt.Errorf("input validation failed: %w", err)}
			return
		}
		incomingMsg.Text = validatedInput

		// Generate conversation ID
		conversationID := getConversationID(incomingMsg.Platform, incomingMsg.PlatformUID)

		// 2. Load conversation history
		recentMessages, err := s.memoryRepo.GetRecentMessages(ctx, conversationID, 4096)
		if err != nil {
			outCh <- domain.StreamChunk{Error: fmt.Errorf("failed to get recent messages: %w", err)}
			return
		}

		// 3. Get available skills and convert to tools
		skills, err := s.skillExecService.ListSkills(ctx, incomingMsg.PlatformUID)
		if err != nil {
			outCh <- domain.StreamChunk{Error: fmt.Errorf("failed to list skills: %w", err)}
			return
		}

		tools := convertSkillsToTools(skills)

		// 4. Build LLM messages
		llmMessages := []domain.Message{}
		// Add history
		for i := range recentMessages {
			llmMessages = append(llmMessages, domain.Message{
				Role:    recentMessages[i].Role,
				Content: recentMessages[i].Content,
			})
		}
		// Add current message
		llmMessages = append(llmMessages, domain.Message{
			Role:    "user",
			Content: validatedInput,
		})

		// 5. Stream LLM response
		streamCh, err := s.llmService.Stream(ctx, domain.LLMProviderAnthropic, &domain.LLMRequest{
			Model:       "claude-3-5-sonnet-20241022",
			Messages:    llmMessages,
			MaxTokens:   4096,
			Temperature: 0.7,
			Tools:       tools,
		})
		if err != nil {
			outCh <- domain.StreamChunk{Error: fmt.Errorf("failed to start LLM stream: %w", err)}
			return
		}

		// 6. Forward stream chunks and handle tool calls
		var fullContent string
		var toolCalls []domain.ToolCall

		for chunk := range streamCh {
			// Check for errors
			if chunk.Error != nil {
				outCh <- chunk
				return
			}

			// Accumulate content
			if chunk.Delta != "" {
				fullContent += chunk.Delta
				// Forward delta to output
				outCh <- domain.StreamChunk{Delta: chunk.Delta}
			}

			// Collect tool calls
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, *chunk.ToolCall)
			}

			// Check if done
			if chunk.Done {
				break
			}
		}

		// 7. Handle tool calls if present
		if len(toolCalls) > 0 {
			logger.Info("Tool calls detected in stream - falling back to non-streaming mode")
			outCh <- domain.StreamChunk{Error: fmt.Errorf("streaming with tool calls not yet supported - use ProcessMessage instead")}
			return
		}

		// 8. Save assistant response
		s.memoryRepo.SaveMessage(ctx, conversationID, incomingMsg.PlatformUID, incomingMsg.Platform, domain.StoredMessage{
			Role:       "assistant",
			Content:    fullContent,
			TokenCount: len(fullContent) / 4, // Rough estimate
		})

		// Send final done marker
		outCh <- domain.StreamChunk{Done: true}

		logger.Info("Streaming response completed",
			"request_id", reqID,
			"content_length", len(fullContent),
		)
	}()

	return outCh, nil
}
