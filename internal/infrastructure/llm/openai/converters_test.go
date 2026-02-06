package openai

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func TestConvertRequest(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test"),
		DefaultModel: "gpt-4o",
	}
	client := New(cfg)

	tests := []struct {
		name             string
		req              *domain.LLMRequest
		wantSystemMsg    bool
		wantModel        string
		wantMaxTokens    int
		wantTemperature  float32
		wantMessageCount int
		wantTools        bool
	}{
		{
			name: "basic request",
			req: &domain.LLMRequest{
				Model: "gpt-4",
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantModel:        "gpt-4",
			wantMessageCount: 1,
		},
		{
			name: "with system prompt",
			req: &domain.LLMRequest{
				Model:        "gpt-4",
				SystemPrompt: "You are helpful",
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantSystemMsg:    true,
			wantModel:        "gpt-4",
			wantMessageCount: 2,
		},
		{
			name: "with max tokens and temperature",
			req: &domain.LLMRequest{
				Model: "gpt-4",
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
				MaxTokens:   500,
				Temperature: 0.8,
			},
			wantModel:        "gpt-4",
			wantMaxTokens:    500,
			wantTemperature:  0.8,
			wantMessageCount: 1,
		},
		{
			name: "with tools",
			req: &domain.LLMRequest{
				Model: "gpt-4",
				Messages: []domain.Message{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []domain.ToolDefinition{
					{
						Name:        "get_weather",
						Description: "Get weather information",
						InputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			wantModel:        "gpt-4",
			wantMessageCount: 1,
			wantTools:        true,
		},
		{
			name: "use default model",
			req: &domain.LLMRequest{
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantModel:        "gpt-4o", // Default from config
			wantMessageCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.convertRequest(tt.req)

			if result.Model != tt.wantModel {
				t.Errorf("Model = %s, want %s", result.Model, tt.wantModel)
			}

			if len(result.Messages) != tt.wantMessageCount {
				t.Errorf("Message count = %d, want %d", len(result.Messages), tt.wantMessageCount)
			}

			if tt.wantSystemMsg && result.Messages[0].Role != openai.ChatMessageRoleSystem {
				t.Error("Expected first message to be system message")
			}

			if tt.wantMaxTokens > 0 && result.MaxTokens != tt.wantMaxTokens {
				t.Errorf("MaxTokens = %d, want %d", result.MaxTokens, tt.wantMaxTokens)
			}

			if tt.wantTemperature > 0 && result.Temperature != tt.wantTemperature {
				t.Errorf("Temperature = %f, want %f", result.Temperature, tt.wantTemperature)
			}

			if tt.wantTools && len(result.Tools) == 0 {
				t.Error("Expected tools to be present")
			}
		})
	}
}

func TestConvertTools(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test"),
		DefaultModel: "gpt-4o",
	}
	client := New(cfg)

	tools := []domain.ToolDefinition{
		{
			Name:        "calculator",
			Description: "Perform calculations",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"expression": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		},
	}

	result := client.convertTools(tools)

	if len(result) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(result))
	}

	if result[0].Type != openai.ToolTypeFunction {
		t.Error("Expected tool type to be function")
	}

	if result[0].Function.Name != "calculator" {
		t.Errorf("Tool 0 name = %s, want calculator", result[0].Function.Name)
	}

	if result[1].Function.Name != "search" {
		t.Errorf("Tool 1 name = %s, want search", result[1].Function.Name)
	}
}

func TestConvertResponse(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test"),
		DefaultModel: "gpt-4o",
	}
	client := New(cfg)

	tests := []struct {
		name             string
		resp             *openai.ChatCompletionResponse
		wantContent      string
		wantFinishReason string
		wantToolCalls    int
		wantPromptTokens int
		wantCompTokens   int
	}{
		{
			name: "basic response",
			resp: &openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "Hello! How can I help?",
						},
						FinishReason: openai.FinishReasonStop,
					},
				},
				Usage: openai.Usage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			wantContent:      "Hello! How can I help?",
			wantFinishReason: "stop",
			wantPromptTokens: 10,
			wantCompTokens:   5,
		},
		{
			name: "with tool calls",
			resp: &openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "",
							ToolCalls: []openai.ToolCall{
								{
									Function: openai.FunctionCall{
										Name:      "get_weather",
										Arguments: `{"location": "Tokyo"}`,
									},
								},
							},
						},
						FinishReason: openai.FinishReasonToolCalls,
					},
				},
				Usage: openai.Usage{
					PromptTokens:     20,
					CompletionTokens: 15,
					TotalTokens:      35,
				},
			},
			wantContent:      "",
			wantFinishReason: "tool_calls",
			wantToolCalls:    1,
			wantPromptTokens: 20,
			wantCompTokens:   15,
		},
		{
			name: "empty choices",
			resp: &openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{},
				Usage: openai.Usage{
					PromptTokens:     5,
					CompletionTokens: 0,
					TotalTokens:      5,
				},
			},
			wantContent:      "",
			wantPromptTokens: 5,
			wantCompTokens:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.convertResponse(tt.resp)

			if result.Content != tt.wantContent {
				t.Errorf("Content = %s, want %s", result.Content, tt.wantContent)
			}

			if result.FinishReason != tt.wantFinishReason {
				t.Errorf("FinishReason = %s, want %s", result.FinishReason, tt.wantFinishReason)
			}

			if len(result.ToolCalls) != tt.wantToolCalls {
				t.Errorf("ToolCalls count = %d, want %d", len(result.ToolCalls), tt.wantToolCalls)
			}

			if result.Usage.PromptTokens != tt.wantPromptTokens {
				t.Errorf("PromptTokens = %d, want %d", result.Usage.PromptTokens, tt.wantPromptTokens)
			}

			if result.Usage.CompletionTokens != tt.wantCompTokens {
				t.Errorf("CompletionTokens = %d, want %d", result.Usage.CompletionTokens, tt.wantCompTokens)
			}
		})
	}
}

func TestConvertToolCalls(t *testing.T) {
	cfg := &config.OpenAIProviderConfig{
		APIKey:       domain.NewSecureStringFromString("sk-test"),
		DefaultModel: "gpt-4o",
	}
	client := New(cfg)

	tests := []struct {
		name         string
		toolCalls    []openai.ToolCall
		wantCount    int
		wantToolName string
		wantArgsKey  string
		wantParseErr bool
	}{
		{
			name: "valid tool call",
			toolCalls: []openai.ToolCall{
				{
					Function: openai.FunctionCall{
						Name:      "calculator",
						Arguments: `{"expression": "2+2"}`,
					},
				},
			},
			wantCount:    1,
			wantToolName: "calculator",
			wantArgsKey:  "expression",
		},
		{
			name: "multiple tool calls",
			toolCalls: []openai.ToolCall{
				{
					Function: openai.FunctionCall{
						Name:      "weather",
						Arguments: `{"location": "Tokyo"}`,
					},
				},
				{
					Function: openai.FunctionCall{
						Name:      "time",
						Arguments: `{"timezone": "JST"}`,
					},
				},
			},
			wantCount:    2,
			wantToolName: "weather",
			wantArgsKey:  "location",
		},
		{
			name: "invalid JSON arguments",
			toolCalls: []openai.ToolCall{
				{
					Function: openai.FunctionCall{
						Name:      "broken",
						Arguments: `{invalid json`,
					},
				},
			},
			wantCount:    1,
			wantToolName: "broken",
			wantParseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.convertToolCalls(tt.toolCalls)

			if len(result) != tt.wantCount {
				t.Fatalf("ToolCall count = %d, want %d", len(result), tt.wantCount)
			}

			if result[0].ToolName != tt.wantToolName {
				t.Errorf("ToolName = %s, want %s", result[0].ToolName, tt.wantToolName)
			}

			if tt.wantParseErr {
				if _, ok := result[0].Arguments["_parse_error"]; !ok {
					t.Error("Expected _parse_error in arguments for invalid JSON")
				}
			} else {
				if _, ok := result[0].Arguments[tt.wantArgsKey]; !ok {
					t.Errorf("Expected key %s in arguments", tt.wantArgsKey)
				}
			}
		})
	}
}
