package bedrock

import (
	"testing"

	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// TestTranslateModelID tests model ID translation
func TestTranslateModelID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Claude 3.5 Sonnet latest",
			input:    "claude-3-5-sonnet-20241022",
			expected: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
		},
		{
			name:     "Claude 3.5 Sonnet v1",
			input:    "claude-3-5-sonnet-20240620",
			expected: "anthropic.claude-3-5-sonnet-20240620-v1:0",
		},
		{
			name:     "Claude 3 Opus",
			input:    "claude-3-opus-20240229",
			expected: "anthropic.claude-3-opus-20240229-v1:0",
		},
		{
			name:     "Claude 3 Sonnet",
			input:    "claude-3-sonnet-20240229",
			expected: "anthropic.claude-3-sonnet-20240229-v1:0",
		},
		{
			name:     "Claude 3 Haiku",
			input:    "claude-3-haiku-20240307",
			expected: "anthropic.claude-3-haiku-20240307-v1:0",
		},
		{
			name:     "Already Bedrock ID",
			input:    "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
			expected: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
		},
		{
			name:     "Unknown model",
			input:    "unknown-model",
			expected: "unknown-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateModelID(tt.input)
			if result != tt.expected {
				t.Errorf("translateModelID(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertMessages_Basic tests basic message conversion
func TestConvertMessages_Basic(t *testing.T) {
	messages := []domain.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
		{Role: "user", Content: "How are you?"},
	}

	result, system := convertMessages(messages, "")

	if len(result) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(result))
	}

	if len(system) != 0 {
		t.Fatalf("Expected no system blocks, got %d", len(system))
	}

	// Check first message
	if result[0].Role != types.ConversationRoleUser {
		t.Errorf("Expected user role for first message")
	}

	// Check second message
	if result[1].Role != types.ConversationRoleAssistant {
		t.Errorf("Expected assistant role for second message")
	}
}

// TestConvertMessages_WithSystemPrompt tests conversion with system prompt
func TestConvertMessages_WithSystemPrompt(t *testing.T) {
	messages := []domain.Message{
		{Role: "user", Content: "Hello"},
	}

	systemPrompt := "You are a helpful assistant."

	result, system := convertMessages(messages, systemPrompt)

	if len(result) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(result))
	}

	if len(system) != 1 {
		t.Fatalf("Expected 1 system block, got %d", len(system))
	}

	// Verify system block
	textBlock, ok := system[0].(*types.SystemContentBlockMemberText)
	if !ok {
		t.Fatalf("Expected text system block")
	}

	if textBlock.Value != systemPrompt {
		t.Errorf("Expected system prompt %q, got %q", systemPrompt, textBlock.Value)
	}
}

// TestConvertTools tests tool conversion
func TestConvertTools(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get the current weather",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"location": map[string]any{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	result := convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result))
	}

	toolMember, ok := result[0].(*types.ToolMemberToolSpec)
	if !ok {
		t.Fatal("Expected ToolMemberToolSpec")
	}

	if *toolMember.Value.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %q", *toolMember.Value.Name)
	}

	if *toolMember.Value.Description != "Get the current weather" {
		t.Errorf("Expected description 'Get the current weather', got %q", *toolMember.Value.Description)
	}
}

// TestConvertResponse tests response conversion
func TestConvertResponse(t *testing.T) {
	inputTokens := int32(10)
	outputTokens := int32(20)
	totalTokens := int32(30)

	output := &bedrockruntime.ConverseOutput{
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: "Hello! How can I help you?",
					},
				},
			},
		},
		StopReason: types.StopReasonEndTurn,
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
		},
	}

	result := convertResponse(output)

	if result.Content != "Hello! How can I help you?" {
		t.Errorf("Expected content 'Hello! How can I help you?', got %q", result.Content)
	}

	if result.FinishReason != "end_turn" {
		t.Errorf("Expected finish reason 'end_turn', got %q", result.FinishReason)
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", result.Usage.PromptTokens)
	}

	if result.Usage.CompletionTokens != 20 {
		t.Errorf("Expected 20 completion tokens, got %d", result.Usage.CompletionTokens)
	}

	if result.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", result.Usage.TotalTokens)
	}
}

// TestConvertResponse_WithToolCall tests response with tool call
func TestConvertResponse_WithToolCall(t *testing.T) {
	toolUseID := "tool_123"
	toolName := "get_weather"
	inputTokens := int32(15)
	outputTokens := int32(25)
	totalTokens := int32(40)

	// Create tool input as document
	toolInput := map[string]interface{}{
		"location": "San Francisco",
	}
	inputDoc := document.NewLazyDocument(toolInput)

	output := &bedrockruntime.ConverseOutput{
		Output: &types.ConverseOutputMemberMessage{
			Value: types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							ToolUseId: &toolUseID,
							Name:      &toolName,
							Input:     inputDoc,
						},
					},
				},
			},
		},
		StopReason: types.StopReasonToolUse,
		Usage: &types.TokenUsage{
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
		},
	}

	result := convertResponse(output)

	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	toolCall := result.ToolCalls[0]
	if toolCall.ToolName != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %q", toolCall.ToolName)
	}

	if len(toolCall.Arguments) == 0 {
		t.Fatalf("Expected arguments, got empty map")
	}

	location, ok := toolCall.Arguments["location"].(string)
	if !ok {
		t.Fatalf("Expected location to be string, got type %T with value %v", toolCall.Arguments["location"], toolCall.Arguments["location"])
	}

	if location != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got %q", location)
	}
}
