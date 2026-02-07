package subagent

import (
	"context"
	"errors"
	"nuimanbot/internal/domain"
	"testing"
	"time"
)

// Mock LLM service for testing
type mockLLMService struct {
	responses []domain.LLMResponse
	callCount int
	err       error
}

func (m *mockLLMService) Chat(ctx context.Context, req domain.LLMRequest) (domain.LLMResponse, error) {
	if m.err != nil {
		return domain.LLMResponse{}, m.err
	}
	if m.callCount >= len(m.responses) {
		return domain.LLMResponse{}, errors.New("no more mock responses")
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

// Mock tool executor for testing
type mockToolExecutor struct {
	results map[string]string
	callLog []string
	err     error
}

func (m *mockToolExecutor) Execute(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.callLog = append(m.callLog, toolName)
	if result, ok := m.results[toolName]; ok {
		return result, nil
	}
	return "mock result", nil
}

// TestSubagentExecutor_Execute_SingleStep tests single-step execution
func TestSubagentExecutor_Execute_SingleStep(t *testing.T) {
	mockLLM := &mockLLMService{
		responses: []domain.LLMResponse{
			{
				Content:      "Task completed successfully",
				FinishReason: "end_turn",
				Usage: domain.TokenUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			},
		},
	}

	executor := NewSubagentExecutor(mockLLM, nil)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-1",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Do a simple task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	// Validate result
	if result.SubagentID != "test-subagent-1" {
		t.Errorf("SubagentID = %v, want test-subagent-1", result.SubagentID)
	}

	if result.Status != domain.SubagentStatusComplete {
		t.Errorf("Status = %v, want %v", result.Status, domain.SubagentStatusComplete)
	}

	if result.Output != "Task completed successfully" {
		t.Errorf("Output = %v, want 'Task completed successfully'", result.Output)
	}

	if result.TokensUsed != 150 {
		t.Errorf("TokensUsed = %v, want 150", result.TokensUsed)
	}

	if result.ToolCallsMade != 0 {
		t.Errorf("ToolCallsMade = %v, want 0", result.ToolCallsMade)
	}

	if len(result.StepResults) != 1 {
		t.Errorf("StepResults length = %v, want 1", len(result.StepResults))
	}
}

// TestSubagentExecutor_Execute_MultiStep tests multi-step execution with tool calls
func TestSubagentExecutor_Execute_MultiStep(t *testing.T) {
	mockTools := &mockToolExecutor{
		results: map[string]string{
			"read_file": "file contents here",
			"grep":      "search results here",
		},
		callLog: []string{},
	}

	mockLLM := &mockLLMService{
		responses: []domain.LLMResponse{
			{
				Content:      "",
				FinishReason: "tool_use",
				ToolCalls: []domain.ToolCall{
					{ToolName: "read_file", Arguments: map[string]interface{}{"path": "test.go"}},
				},
				Usage: domain.TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
			},
			{
				Content:      "",
				FinishReason: "tool_use",
				ToolCalls: []domain.ToolCall{
					{ToolName: "grep", Arguments: map[string]interface{}{"pattern": "test"}},
				},
				Usage: domain.TokenUsage{PromptTokens: 120, CompletionTokens: 60, TotalTokens: 180},
			},
			{
				Content:      "Analysis complete: Found 3 test cases",
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 150, CompletionTokens: 70, TotalTokens: 220},
			},
		},
	}

	executor := NewSubagentExecutor(mockLLM, mockTools)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-multi",
		ParentContextID: "parent-1",
		SkillName:       "analysis-skill",
		AllowedTools:    []string{"read_file", "grep"},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Analyze the test file"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Validate multi-step execution
	if result.Status != domain.SubagentStatusComplete {
		t.Errorf("Status = %v, want %v", result.Status, domain.SubagentStatusComplete)
	}

	if result.ToolCallsMade != 2 {
		t.Errorf("ToolCallsMade = %v, want 2", result.ToolCallsMade)
	}

	if result.TokensUsed != 550 { // 100+50+120+60+150+70
		t.Errorf("TokensUsed = %v, want 550", result.TokensUsed)
	}

	if len(result.StepResults) != 3 {
		t.Errorf("StepResults length = %v, want 3", len(result.StepResults))
	}

	// Verify tool calls were made
	if len(mockTools.callLog) != 2 {
		t.Errorf("Tool calls made = %v, want 2", len(mockTools.callLog))
	}
}

// TestSubagentExecutor_Execute_ToolRestriction tests tool restriction enforcement
func TestSubagentExecutor_Execute_ToolRestriction(t *testing.T) {
	mockTools := &mockToolExecutor{
		results: map[string]string{
			"write_file": "file written",
		},
		callLog: []string{},
	}

	mockLLM := &mockLLMService{
		responses: []domain.LLMResponse{
			{
				Content:      "",
				FinishReason: "tool_use",
				ToolCalls: []domain.ToolCall{
					{ToolName: "write_file", Arguments: map[string]interface{}{"path": "test.go"}},
				},
				Usage: domain.TokenUsage{PromptTokens: 100, CompletionTokens: 50},
			},
		},
	}

	executor := NewSubagentExecutor(mockLLM, mockTools)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-restricted",
		ParentContextID: "parent-1",
		SkillName:       "restricted-skill",
		AllowedTools:    []string{"read_file", "grep"}, // write_file NOT allowed
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Do something"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should fail because write_file is not allowed
	if result.Status != domain.SubagentStatusError {
		t.Errorf("Status = %v, want %v (tool restriction should fail)", result.Status, domain.SubagentStatusError)
	}

	if result.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty when tool is restricted")
	}

	// Tool should NOT have been called
	if len(mockTools.callLog) != 0 {
		t.Errorf("Tool should not have been called, but got %v calls", len(mockTools.callLog))
	}
}

// TestSubagentExecutor_Execute_TokenLimit tests token limit enforcement
func TestSubagentExecutor_Execute_TokenLimit(t *testing.T) {
	mockLLM := &mockLLMService{
		responses: []domain.LLMResponse{
			{
				Content:      "Step 1",
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 500, CompletionTokens: 600, TotalTokens: 1100},
			},
		},
	}

	executor := NewSubagentExecutor(mockLLM, nil)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-limit",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits: domain.ResourceLimits{
			MaxTokens:    1000, // Limit is 1000, usage will be 1100
			MaxToolCalls: 50,
			Timeout:      5 * time.Minute,
		},
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should timeout/error due to token limit
	if result.Status != domain.SubagentStatusError && result.Status != domain.SubagentStatusTimeout {
		t.Errorf("Status = %v, want error or timeout (token limit exceeded)", result.Status)
	}

	if result.TokensUsed <= 0 {
		t.Errorf("TokensUsed = %v, should be > 0", result.TokensUsed)
	}
}

// TestSubagentExecutor_Execute_ToolCallLimit tests tool call limit enforcement
func TestSubagentExecutor_Execute_ToolCallLimit(t *testing.T) {
	mockTools := &mockToolExecutor{
		results: map[string]string{
			"read_file": "content",
		},
		callLog: []string{},
	}

	// Create 6 responses, each with a tool call
	responses := make([]domain.LLMResponse, 6)
	for i := 0; i < 6; i++ {
		responses[i] = domain.LLMResponse{
			Content:      "",
			FinishReason: "tool_use",
			ToolCalls: []domain.ToolCall{
				{ToolName: "read_file", Arguments: map[string]interface{}{"path": "test.go"}},
			},
			Usage: domain.TokenUsage{PromptTokens: 50, CompletionTokens: 30},
		}
	}

	mockLLM := &mockLLMService{
		responses: responses,
	}

	executor := NewSubagentExecutor(mockLLM, mockTools)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-tool-limit",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{"read_file"},
		ResourceLimits: domain.ResourceLimits{
			MaxTokens:    100000,
			MaxToolCalls: 5, // Limit to 5 tool calls
			Timeout:      5 * time.Minute,
		},
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should stop due to tool call limit
	if result.Status != domain.SubagentStatusError && result.Status != domain.SubagentStatusTimeout {
		t.Errorf("Status = %v, want error or timeout (tool call limit exceeded)", result.Status)
	}

	if result.ToolCallsMade > 5 {
		t.Errorf("ToolCallsMade = %v, should not exceed 5", result.ToolCallsMade)
	}
}

// TestSubagentExecutor_Execute_ContextCancellation tests context cancellation
func TestSubagentExecutor_Execute_ContextCancellation(t *testing.T) {
	mockLLM := &mockLLMService{
		responses: []domain.LLMResponse{
			{
				Content:      "Never reached",
				FinishReason: "end_turn",
				Usage:        domain.TokenUsage{PromptTokens: 100, CompletionTokens: 50},
			},
		},
	}

	executor := NewSubagentExecutor(mockLLM, nil)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-cancel",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := executor.Execute(ctx, subagentCtx)

	// Should handle cancellation gracefully
	if err == nil && (result == nil || result.Status == domain.SubagentStatusComplete) {
		t.Error("Execute() should handle context cancellation")
	}
}

// TestSubagentExecutor_Execute_LLMError tests LLM error handling
func TestSubagentExecutor_Execute_LLMError(t *testing.T) {
	mockLLM := &mockLLMService{
		err: errors.New("LLM service unavailable"),
	}

	executor := NewSubagentExecutor(mockLLM, nil)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-error",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Status != domain.SubagentStatusError {
		t.Errorf("Status = %v, want %v", result.Status, domain.SubagentStatusError)
	}

	if result.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty when LLM fails")
	}
}

// TestSubagentExecutor_Execute_MaxIterations tests maximum iteration limit
func TestSubagentExecutor_Execute_MaxIterations(t *testing.T) {
	// Create many responses that all request more tool calls
	responses := make([]domain.LLMResponse, 100)
	for i := 0; i < 100; i++ {
		responses[i] = domain.LLMResponse{
			Content:      "",
			FinishReason: "tool_use",
			ToolCalls: []domain.ToolCall{
				{ToolName: "read_file", Arguments: map[string]interface{}{"path": "test.go"}},
			},
			Usage: domain.TokenUsage{PromptTokens: 10, CompletionTokens: 10},
		}
	}

	mockTools := &mockToolExecutor{
		results: map[string]string{
			"read_file": "content",
		},
	}

	mockLLM := &mockLLMService{
		responses: responses,
	}

	executor := NewSubagentExecutor(mockLLM, mockTools)

	subagentCtx := domain.SubagentContext{
		ID:              "test-subagent-iterations",
		ParentContextID: "parent-1",
		SkillName:       "test-skill",
		AllowedTools:    []string{"read_file"},
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: "Task"},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, subagentCtx)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Should stop due to max iterations or resource limits
	if result.Status == domain.SubagentStatusRunning || result.Status == domain.SubagentStatusPending {
		t.Errorf("Status = %v, should be terminal", result.Status)
	}

	// Should have made some progress but stopped
	if len(result.StepResults) == 0 {
		t.Error("Should have some step results")
	}
}
