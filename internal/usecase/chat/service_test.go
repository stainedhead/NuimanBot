package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// Mock implementations for testing

type mockLLMService struct {
	completeFunc func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error)
}

func (m *mockLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, provider, req)
	}
	return &domain.LLMResponse{Content: "mock response"}, nil
}

func (m *mockLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func (m *mockLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	return nil, errors.New("not implemented")
}

type mockMemoryRepository struct {
	saveMessageFunc       func(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error
	getConversationFunc   func(ctx context.Context, convID string) (*domain.Conversation, error)
	getRecentMessagesFunc func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error)
}

func (m *mockMemoryRepository) SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error {
	if m.saveMessageFunc != nil {
		return m.saveMessageFunc(ctx, convID, userID, platform, msg)
	}
	return nil
}

func (m *mockMemoryRepository) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	if m.getConversationFunc != nil {
		return m.getConversationFunc(ctx, convID)
	}
	return &domain.Conversation{}, nil
}

func (m *mockMemoryRepository) GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
	if m.getRecentMessagesFunc != nil {
		return m.getRecentMessagesFunc(ctx, convID, maxTokens)
	}
	return []domain.StoredMessage{}, nil
}

func (m *mockMemoryRepository) DeleteConversation(ctx context.Context, convID string) error {
	return nil
}

func (m *mockMemoryRepository) ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error) {
	return nil, errors.New("not implemented")
}

type mockSkillExecutionService struct {
	executeFunc    func(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error)
	listSkillsFunc func(ctx context.Context, userID string) ([]domain.Skill, error)
}

func (m *mockSkillExecutionService) Execute(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, skillName, params)
	}
	return &domain.SkillResult{Output: "mock skill result"}, nil
}

func (m *mockSkillExecutionService) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	if m.listSkillsFunc != nil {
		return m.listSkillsFunc(ctx, userID)
	}
	return []domain.Skill{}, nil
}

type mockSecurityService struct {
	validateInputFunc func(ctx context.Context, input string, maxLength int) (string, error)
	auditFunc         func(ctx context.Context, event *domain.AuditEvent) error
}

func (m *mockSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	if m.validateInputFunc != nil {
		return m.validateInputFunc(ctx, input, maxLength)
	}
	return input, nil
}

func (m *mockSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	if m.auditFunc != nil {
		return m.auditFunc(ctx, event)
	}
	return nil
}

type mockSkill struct {
	name        string
	description string
	inputSchema map[string]any
}

func (m *mockSkill) Name() string {
	return m.name
}

func (m *mockSkill) Description() string {
	return m.description
}

func (m *mockSkill) InputSchema() map[string]any {
	return m.inputSchema
}

func (m *mockSkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
	return &domain.SkillResult{Output: "mock result"}, nil
}

func (m *mockSkill) RequiredPermissions() []domain.Permission {
	return []domain.Permission{}
}

func (m *mockSkill) Config() domain.SkillConfig {
	return domain.SkillConfig{Enabled: true}
}

// Test helper to create a service with mocks
func createTestService(
	llmService LLMService,
	memoryRepo MemoryRepository,
	skillExecService SkillExecutionService,
	securityService SecurityService,
) *Service {
	return NewService(llmService, memoryRepo, skillExecService, securityService)
}

// TestProcessMessage_NoToolCalls tests basic message processing without tool calls
func TestProcessMessage_NoToolCalls(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{
				Content:      "Hello! How can I help you?",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage: domain.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{}, nil
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-1",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Hello",
		Timestamp:   time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if outgoingMsg.Content != "Hello! How can I help you?" {
		t.Errorf("Expected content 'Hello! How can I help you?', got %s", outgoingMsg.Content)
	}

	if outgoingMsg.RecipientID != "platform-user-1" {
		t.Errorf("Expected recipient 'platform-user-1', got %s", outgoingMsg.RecipientID)
	}
}

// TestProcessMessage_WithToolCalls tests message processing with a single tool call
func TestProcessMessage_WithToolCalls(t *testing.T) {
	callCount := 0

	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				// First call - LLM wants to use a tool
				return &domain.LLMResponse{
					Content: "I'll calculate that for you.",
					ToolCalls: []domain.ToolCall{
						{
							ToolName: "calculator",
							Arguments: map[string]any{
								"operation": "add",
								"a":         5.0,
								"b":         3.0,
							},
						},
					},
					FinishReason: "tool_use",
					Usage: domain.TokenUsage{
						PromptTokens:     10,
						CompletionTokens: 15,
						TotalTokens:      25,
					},
				}, nil
			}
			// Second call - LLM responds with final answer
			return &domain.LLMResponse{
				Content:      "The result is 8.",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage: domain.TokenUsage{
					PromptTokens:     30,
					CompletionTokens: 10,
					TotalTokens:      40,
				},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{
				&mockSkill{
					name:        "calculator",
					description: "Performs arithmetic operations",
					inputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"operation": map[string]any{"type": "string"},
							"a":         map[string]any{"type": "number"},
							"b":         map[string]any{"type": "number"},
						},
					},
				},
			}, nil
		},
		executeFunc: func(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
			if skillName != "calculator" {
				return nil, errors.New("unknown skill")
			}
			return &domain.SkillResult{
				Output: "8",
			}, nil
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-2",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "What is 5 + 3?",
		Timestamp:   time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if outgoingMsg.Content != "The result is 8." {
		t.Errorf("Expected content 'The result is 8.', got %s", outgoingMsg.Content)
	}

	// Verify LLM was called twice (once for tool use, once for final response)
	if callCount != 2 {
		t.Errorf("Expected 2 LLM calls, got %d", callCount)
	}
}

// TestProcessMessage_MultipleToolIterations tests multiple tool calling iterations
func TestProcessMessage_MultipleToolIterations(t *testing.T) {
	callCount := 0

	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount < 3 {
				// First two calls - LLM wants to use tools
				return &domain.LLMResponse{
					Content: "Using tool...",
					ToolCalls: []domain.ToolCall{
						{
							ToolName:  "calculator",
							Arguments: map[string]any{"operation": "add"},
						},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			// Third call - final response
			return &domain.LLMResponse{
				Content:      "Done!",
				ToolCalls:    []domain.ToolCall{},
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{
				&mockSkill{name: "calculator", description: "Calculator", inputSchema: map[string]any{}},
			}, nil
		},
		executeFunc: func(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
			return &domain.SkillResult{Output: "result"}, nil
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-3",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Complex calculation",
		Timestamp:   time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if outgoingMsg.Content != "Done!" {
		t.Errorf("Expected content 'Done!', got %s", outgoingMsg.Content)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 LLM calls, got %d", callCount)
	}
}

// TestProcessMessage_MaxIterationsExceeded tests max tool calling iterations limit
func TestProcessMessage_MaxIterationsExceeded(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			// Always return tool calls (never finish)
			return &domain.LLMResponse{
				Content: "Using tool...",
				ToolCalls: []domain.ToolCall{
					{ToolName: "calculator", Arguments: map[string]any{}},
				},
				FinishReason: "tool_use",
				Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
			}, nil
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{
				&mockSkill{name: "calculator", description: "Calculator", inputSchema: map[string]any{}},
			}, nil
		},
		executeFunc: func(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
			return &domain.SkillResult{Output: "result"}, nil
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-4",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Infinite loop test",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err == nil {
		t.Fatal("Expected error for max iterations exceeded, got nil")
	}

	if err.Error() != "max tool calling iterations exceeded" {
		t.Errorf("Expected 'max tool calling iterations exceeded' error, got: %v", err)
	}
}

// TestProcessMessage_ToolExecutionError tests handling of tool execution errors
func TestProcessMessage_ToolExecutionError(t *testing.T) {
	callCount := 0

	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			callCount++
			if callCount == 1 {
				return &domain.LLMResponse{
					Content: "Using tool...",
					ToolCalls: []domain.ToolCall{
						{ToolName: "calculator", Arguments: map[string]any{}},
					},
					FinishReason: "tool_use",
					Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20},
				}, nil
			}
			// After tool error, LLM responds with error message
			return &domain.LLMResponse{
				Content:      "I encountered an error executing the tool.",
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

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{
				&mockSkill{name: "calculator", description: "Calculator", inputSchema: map[string]any{}},
			}, nil
		},
		executeFunc: func(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
			return nil, errors.New("tool execution failed")
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-5",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Test error handling",
		Timestamp:   time.Now(),
	}

	outgoingMsg, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	// Should still get a response even with tool error
	if outgoingMsg.Content != "I encountered an error executing the tool." {
		t.Errorf("Expected error message, got: %s", outgoingMsg.Content)
	}
}

// TestProcessMessage_InputValidationError tests input validation failure
func TestProcessMessage_InputValidationError(t *testing.T) {
	llmService := &mockLLMService{}
	memoryRepo := &mockMemoryRepository{}
	skillExecService := &mockSkillExecutionService{}

	securityService := &mockSecurityService{
		validateInputFunc: func(ctx context.Context, input string, maxLength int) (string, error) {
			return "", errors.New("input validation failed: contains malicious content")
		},
	}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-6",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "<script>alert('xss')</script>",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err == nil {
		t.Fatal("Expected input validation error, got nil")
	}

	if err.Error() != "input validation failed: input validation failed: contains malicious content" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestProcessMessage_SkillListingError tests skill listing failure
func TestProcessMessage_SkillListingError(t *testing.T) {
	llmService := &mockLLMService{}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return nil, errors.New("skill service unavailable")
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-7",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Test",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err == nil {
		t.Fatal("Expected skill listing error, got nil")
	}

	if err.Error() != "failed to list skills: skill service unavailable" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestProcessMessage_LLMCompletionError tests LLM completion failure
func TestProcessMessage_LLMCompletionError(t *testing.T) {
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return nil, errors.New("LLM service unavailable")
		},
	}

	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			return []domain.StoredMessage{}, nil
		},
	}

	skillExecService := &mockSkillExecutionService{
		listSkillsFunc: func(ctx context.Context, userID string) ([]domain.Skill, error) {
			return []domain.Skill{}, nil
		},
	}

	securityService := &mockSecurityService{}

	service := createTestService(llmService, memoryRepo, skillExecService, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "test-msg-8",
		Platform:    domain.PlatformCLI,
		PlatformUID: "platform-user-1",
		Text:        "Test",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessage(context.Background(), incomingMsg)
	if err == nil {
		t.Fatal("Expected LLM completion error, got nil")
	}

	if err.Error() != "LLM completion failed: LLM service unavailable" {
		t.Errorf("Unexpected error message: %v", err)
	}
}
