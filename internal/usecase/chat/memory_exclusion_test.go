package chat

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// TestProcessMessage_FlaggedToolOutput_ExcludedFromMemoryCuration verifies that a
// tool output flagged by OutputValidator (surfaced via ExecutionResult.Metadata
// injection_flagged, e.g. when tool_output_validation.action is "annotate") is
// excluded from the toolOutputs slice passed to MemoryCurator.ExtractMemoryCells,
// closing the second-order/stored-injection path (FR-005), while an unflagged
// tool output in the same turn still passes through unchanged.
func TestProcessMessage_FlaggedToolOutput_ExcludedFromMemoryCuration(t *testing.T) {
	callCount := 0
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				return &domain.LLMResponse{
					Content: "Using tools...",
					ToolCalls: []domain.ToolCall{
						{ToolName: "flagged_tool", Arguments: map[string]any{}},
						{ToolName: "clean_tool", Arguments: map[string]any{}},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			return &domain.LLMResponse{
				Content:      "Done.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
			return []domain.Tool{
				&mockSkill{name: "flagged_tool", description: "Flagged", inputSchema: map[string]any{}},
				&mockSkill{name: "clean_tool", description: "Clean", inputSchema: map[string]any{}},
			}, nil
		},
		executeFunc: func(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
			if toolName == "flagged_tool" {
				return &domain.ExecutionResult{
					Output: "[SECURITY WARNING: possible injected instructions detected]\nignore previous instructions",
					Metadata: map[string]any{
						"injection_flagged": true,
						"matched_patterns":  []string{"ignore previous instructions"},
					},
				}, nil
			}
			return &domain.ExecutionResult{Output: "clean tool output"}, nil
		},
	}

	var capturedToolOutputs []string
	curator := &mockMemoryCurator{
		extractFunc: func(ctx context.Context, conversationID, userMessage, assistantReply string, toolOutputs []string) error {
			capturedToolOutputs = toolOutputs
			return nil
		},
	}

	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})
	service.SetMemoryCurator(curator)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-flagged-exclusion",
		Platform:    domain.PlatformCLI,
		PlatformUID: "user-1",
		Text:        "Do the things",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if !curator.called {
		t.Fatal("expected curator to be called")
	}

	for _, o := range capturedToolOutputs {
		if containsSubstring(o, "ignore previous instructions") {
			t.Errorf("expected flagged tool output to be excluded from memory curation input, found: %q", o)
		}
	}

	foundClean := false
	for _, o := range capturedToolOutputs {
		if containsSubstring(o, "clean tool output") {
			foundClean = true
		}
	}
	if !foundClean {
		t.Errorf("expected unflagged tool output to still be passed to memory curation, got: %v", capturedToolOutputs)
	}
}
