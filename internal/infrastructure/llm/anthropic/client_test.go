package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// TestComplete_Success tests successful completion without tools
func TestComplete_Success(t *testing.T) {
	// Create mock Anthropic API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Check API key header
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-api-key" {
			t.Errorf("Expected x-api-key header with test-api-key, got %s", apiKey)
		}

		// Return mock response
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "Hello! How can I help you today?",
				},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with mock server
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test Complete()
	req := &domain.LLMRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: 0.7,
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() failed: %v", err)
	}

	// Verify response
	if response.Content != "Hello! How can I help you today?" {
		t.Errorf("Expected content 'Hello! How can I help you today?', got %s", response.Content)
	}
	if response.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 20 {
		t.Errorf("Expected 20 completion tokens, got %d", response.Usage.CompletionTokens)
	}
	if response.FinishReason != "end_turn" {
		t.Errorf("Expected finish reason 'end_turn', got %s", response.FinishReason)
	}
}

// TestComplete_WithSystemPrompt tests completion with system prompt
func TestComplete_WithSystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to verify system prompt
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Check if system prompt is present
		system, ok := reqBody["system"]
		if !ok {
			t.Error("Expected system field in request")
		}

		// Verify system prompt format
		if systemSlice, ok := system.([]interface{}); ok {
			if len(systemSlice) > 0 {
				if block, ok := systemSlice[0].(map[string]interface{}); ok {
					if block["type"] != "text" {
						t.Errorf("Expected system type 'text', got %v", block["type"])
					}
					if block["text"] != "You are a helpful assistant." {
						t.Errorf("Expected system text 'You are a helpful assistant.', got %v", block["text"])
					}
				}
			}
		}

		// Return mock response
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{"type": "text", "text": "I'm here to help!"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  15,
				"output_tokens": 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:        "claude-3-sonnet-20240229",
		MaxTokens:    1024,
		Temperature:  0.7,
		SystemPrompt: "You are a helpful assistant.",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() with system prompt failed: %v", err)
	}

	if response.Content != "I'm here to help!" {
		t.Errorf("Expected content 'I'm here to help!', got %s", response.Content)
	}
}

// TestComplete_WithTools tests completion with tool definitions
func TestComplete_WithTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to verify tools
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Check if tools are present
		tools, ok := reqBody["tools"]
		if !ok {
			t.Error("Expected tools field in request")
		}

		toolsSlice, ok := tools.([]interface{})
		if !ok || len(toolsSlice) == 0 {
			t.Error("Expected non-empty tools array")
		}

		// Return response with tool use
		response := map[string]interface{}{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-sonnet-20240229",
			"content": []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   "tool_123",
					"name": "calculator",
					"input": map[string]interface{}{
						"operation": "add",
						"a":         5,
						"b":         3,
					},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  20,
				"output_tokens": 15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-api-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   1024,
		Temperature: 0.7,
		Messages: []domain.Message{
			{Role: "user", Content: "What is 5 + 3?"},
		},
		Tools: []domain.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Performs basic arithmetic operations",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"operation": map[string]any{"type": "string"},
						"a":         map[string]any{"type": "number"},
						"b":         map[string]any{"type": "number"},
					},
				},
			},
		},
	}

	response, err := client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err != nil {
		t.Fatalf("Complete() with tools failed: %v", err)
	}

	// Verify tool call
	if len(response.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}
	if response.ToolCalls[0].ToolName != "calculator" {
		t.Errorf("Expected tool name 'calculator', got %s", response.ToolCalls[0].ToolName)
	}
	if response.FinishReason != "tool_use" {
		t.Errorf("Expected finish reason 'tool_use', got %s", response.FinishReason)
	}
}

// TestComplete_APIError tests error handling
func TestComplete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("invalid-key"),
	}
	client, err := NewClientWithBaseURL(cfg, server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	_, err = client.Complete(context.Background(), domain.LLMProviderAnthropic, req)
	if err == nil {
		t.Error("Expected error for invalid API key, got nil")
	}
}

// TestComplete_InvalidProvider tests provider validation
func TestComplete_InvalidProvider(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:     "gpt-4",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	// Try with wrong provider
	_, err = client.Complete(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error for wrong provider, got nil")
	}
}

// TestListModels_Success tests model listing
func TestListModels_Success(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	models, err := client.ListModels(context.Background(), domain.LLMProviderAnthropic)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	// Should return at least Claude models
	if len(models) == 0 {
		t.Error("Expected at least one model, got none")
	}

	// Check for expected models
	foundSonnet := false
	for _, model := range models {
		if model.ID == "claude-3-5-sonnet-20241022" || model.ID == "claude-3-sonnet-20240229" {
			foundSonnet = true
			if model.Provider != "anthropic" {
				t.Errorf("Expected provider 'anthropic', got %s", model.Provider)
			}
			if model.ContextWindow <= 0 {
				t.Error("Expected positive context window")
			}
		}
	}
	if !foundSonnet {
		t.Error("Expected to find Claude Sonnet model in list")
	}
}

// TestNewClient_InvalidConfig tests client creation validation
func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.LLMProviderConfig
		wantErr bool
	}{
		{
			name: "missing API key",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderAnthropic,
				APIKey: domain.NewSecureStringFromString(""),
			},
			wantErr: true,
		},
		{
			name: "wrong provider type",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderOpenAI,
				APIKey: domain.NewSecureStringFromString("test-key"),
			},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: &config.LLMProviderConfig{
				Type:   domain.LLMProviderAnthropic,
				APIKey: domain.NewSecureStringFromString("test-key"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStream_InvalidProvider tests Stream with invalid provider
func TestStream_InvalidProvider(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &domain.LLMRequest{
		Model:     "gpt-4",
		MaxTokens: 1024,
		Messages:  []domain.Message{{Role: "user", Content: "Hello"}},
	}

	// Try with wrong provider
	_, err = client.Stream(context.Background(), domain.LLMProviderOpenAI, req)
	if err == nil {
		t.Error("Expected error for wrong provider in Stream, got nil")
	}
}

// TestStream_ParameterConstruction tests that Stream properly constructs parameters
// Note: Full streaming behavior testing requires complex SDK mocking and is deferred
func TestStream_ParameterConstruction(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name string
		req  *domain.LLMRequest
	}{
		{
			name: "basic request",
			req: &domain.LLMRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
		},
		{
			name: "with temperature",
			req: &domain.LLMRequest{
				Model:       "claude-3-sonnet-20240229",
				MaxTokens:   1024,
				Temperature: 0.7,
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
		},
		{
			name: "with system prompt",
			req: &domain.LLMRequest{
				Model:        "claude-3-sonnet-20240229",
				MaxTokens:    1024,
				SystemPrompt: "You are helpful",
				Messages: []domain.Message{
					{Role: "user", Content: "Hello"},
				},
			},
		},
		{
			name: "with tools",
			req: &domain.LLMRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				Messages: []domain.Message{
					{Role: "user", Content: "Calculate 5+3"},
				},
				Tools: []domain.ToolDefinition{
					{
						Name:        "calculator",
						Description: "Math tool",
						InputSchema: map[string]any{
							"type":       "object",
							"properties": map[string]any{"a": map[string]any{"type": "number"}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Stream() should return a channel without error for valid Anthropic requests
			// The actual streaming behavior would require mocking the SDK's streaming API
			ch, err := client.Stream(context.Background(), domain.LLMProviderAnthropic, tt.req)
			if err != nil {
				t.Errorf("Stream() error = %v, want nil", err)
			}
			if ch == nil {
				t.Error("Stream() returned nil channel")
			}

			// Clean up by reading any immediate errors and closing context
			// In a real scenario, the SDK would be streaming, but without API key/network,
			// we just verify the channel was created
			select {
			case chunk, ok := <-ch:
				if ok && chunk.Error != nil {
					// Expected - no valid API connection
					t.Logf("Expected error from stream: %v", chunk.Error)
				}
			default:
				// Channel not ready yet - that's fine
			}
		})
	}
}

// TestListModels_InvalidProvider tests ListModels with invalid provider
func TestListModels_InvalidProvider(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.ListModels(context.Background(), domain.LLMProviderOpenAI)
	if err == nil {
		t.Error("Expected error for wrong provider in ListModels, got nil")
	}
}

// TestConvertInputSchema tests schema conversion
func TestConvertInputSchema(t *testing.T) {
	tests := []struct {
		name           string
		schema         map[string]any
		wantProperties bool
	}{
		{
			name: "schema with required fields as []string",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "number"},
				},
				"required": []string{"name"},
			},
			wantProperties: true,
		},
		{
			name: "schema with required fields as []interface{}",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
				"required": []interface{}{"city"},
			},
			wantProperties: true,
		},
		{
			name: "schema without required fields",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"optional": map[string]any{"type": "string"},
				},
			},
			wantProperties: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInputSchema(tt.schema)
			if tt.wantProperties && result.Properties == nil {
				t.Error("convertInputSchema() properties should not be nil")
			}
		})
	}
}

// TestCreateToolParam tests tool parameter creation
func TestCreateToolParam(t *testing.T) {
	schema := anthropicsdk.ToolInputSchemaParam{
		Type: "object",
		Properties: map[string]any{
			"query": map[string]any{"type": "string"},
		},
	}

	tests := []struct {
		name        string
		toolName    string
		description string
	}{
		{
			name:        "tool with description",
			toolName:    "search",
			description: "Search the web",
		},
		{
			name:        "tool without description",
			toolName:    "calculator",
			description: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createToolParam(tt.toolName, tt.description, schema)
			if result.OfTool.Name != tt.toolName {
				t.Errorf("createToolParam() name = %v, want %v", result.OfTool.Name, tt.toolName)
			}
			// Tool param created successfully - verify it has correct structure
			if result.OfTool.InputSchema.Type != "object" {
				t.Errorf("createToolParam() schema type = %v, want object", result.OfTool.InputSchema.Type)
			}
		})
	}
}

// TestConvertToolResults tests tool results conversion
func TestConvertToolResults(t *testing.T) {
	tests := []struct {
		name        string
		toolResults []domain.ToolResult
		toolCallID  string
	}{
		{
			name: "successful tool result",
			toolResults: []domain.ToolResult{
				{
					ToolName: "calculator",
					Output:   "8",
					Error:    "",
				},
			},
			toolCallID: "tool_123",
		},
		{
			name: "tool result with error",
			toolResults: []domain.ToolResult{
				{
					ToolName: "search",
					Output:   "",
					Error:    "network timeout",
				},
			},
			toolCallID: "tool_456",
		},
		{
			name: "multiple tool results",
			toolResults: []domain.ToolResult{
				{
					ToolName: "calculator",
					Output:   "42",
					Error:    "",
				},
				{
					ToolName: "weather",
					Output:   "sunny",
					Error:    "",
				},
			},
			toolCallID: "tool_789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToolResults(tt.toolResults, tt.toolCallID)
			if result.Role != "user" {
				t.Errorf("convertToolResults() role = %v, want user", result.Role)
			}
			// Verify content blocks were created
			// The implementation creates content blocks for each result
		})
	}
}

// TestConvertMessages_BasicMessages tests basic message conversion
func TestConvertMessages_BasicMessages(t *testing.T) {
	messages := []domain.Message{
		{
			Role:    "user",
			Content: "Hello, how are you?",
		},
		{
			Role:    "assistant",
			Content: "I'm doing well, thank you!",
		},
		{
			Role:    "user",
			Content: "That's great!",
		},
	}

	result := convertMessages(messages)
	if len(result) != len(messages) {
		t.Errorf("convertMessages() returned %d messages, want %d", len(result), len(messages))
	}

	// Verify roles are preserved by comparing string values
	for i, msg := range result {
		if string(msg.Role) != messages[i].Role {
			t.Errorf("convertMessages() message %d role = %v, want %v", i, msg.Role, messages[i].Role)
		}
	}
}

// TestConvertTools_ComplexSchema tests tool conversion with complex schemas
func TestConvertTools_ComplexSchema(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "complex_tool",
			Description: "A tool with complex schema",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nested": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"field": map[string]any{"type": "string"},
						},
					},
					"array": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
				},
				"required": []string{"nested"},
			},
		},
	}

	result := convertTools(tools)
	if len(result) == 0 {
		t.Error("convertTools() returned empty slice")
	}

	if result[0].OfTool.Name != "complex_tool" {
		t.Errorf("convertTools() tool name = %v, want complex_tool", result[0].OfTool.Name)
	}
}

// TestConvertTools_MultipleTools tests converting multiple tools
func TestConvertTools_MultipleTools(t *testing.T) {
	tools := []domain.ToolDefinition{
		{
			Name:        "calculator",
			Description: "Performs math",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"a": map[string]any{"type": "number"}},
			},
		},
		{
			Name:        "search",
			Description: "Searches the web",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"query": map[string]any{"type": "string"}},
			},
		},
		{
			Name:        "weather",
			Description: "Gets weather",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"location": map[string]any{"type": "string"}},
			},
		},
	}

	result := convertTools(tools)
	if len(result) != 3 {
		t.Errorf("convertTools() returned %d tools, want 3", len(result))
	}

	// Verify all tool names are present
	names := make(map[string]bool)
	for _, tool := range result {
		names[tool.OfTool.Name] = true
	}

	expectedNames := []string{"calculator", "search", "weather"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("convertTools() missing tool %s", name)
		}
	}
}

// TestNewClientWithBaseURL_InvalidURL tests error handling for invalid URLs
func TestNewClientWithBaseURL_InvalidURL(t *testing.T) {
	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString("test-key"),
	}

	// Empty base URL should still create client (SDK handles it)
	_, err := NewClientWithBaseURL(cfg, "")
	if err != nil {
		t.Errorf("NewClientWithBaseURL() with empty URL unexpectedly failed: %v", err)
	}
}

// TestConvertMessages_EmptyContent tests handling of empty messages
func TestConvertMessages_EmptyContent(t *testing.T) {
	messages := []domain.Message{
		{
			Role:    "user",
			Content: "",
		},
		{
			Role:    "assistant",
			Content: "Response",
		},
	}

	result := convertMessages(messages)
	if len(result) != 2 {
		t.Errorf("convertMessages() returned %d messages, want 2", len(result))
	}
}

// TestConvertInputSchema_WithComplexRequired tests complex required field handling
func TestConvertInputSchema_WithComplexRequired(t *testing.T) {
	// Test with mixed required types
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field1": map[string]any{"type": "string"},
			"field2": map[string]any{"type": "number"},
			"field3": map[string]any{"type": "boolean"},
		},
		"required": []interface{}{"field1", "field2"},
	}

	result := convertInputSchema(schema)
	if len(result.Required) != 2 {
		t.Errorf("convertInputSchema() required length = %d, want 2", len(result.Required))
	}
}
