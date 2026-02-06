package domain

import "context"

// LLMProvider identifies different LLM provider types.
type LLMProvider string

const (
	LLMProviderAnthropic LLMProvider = "anthropic"
	LLMProviderOpenAI    LLMProvider = "openai"
	LLMProviderOllama    LLMProvider = "ollama"
	LLMProviderBedrock   LLMProvider = "bedrock"
)

// Message represents a generic chat message, used primarily within LLM contexts.
type Message struct {
	Role    string `json:"role"`    // e.g., "user", "assistant", "system"
	Content string `json:"content"` // The message content
}

// LLMRequest represents a request to an LLM.
type LLMRequest struct {
	Model        string
	Messages     []Message
	MaxTokens    int
	Temperature  float64
	Tools        []ToolDefinition // For function calling
	SystemPrompt string
}

// ToolDefinition defines the schema for a tool that an LLM can use.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolCall represents an LLM's request to call a tool.
type ToolCall struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	ToolName string         `json:"tool_name"`
	Output   string         `json:"output"`
	Error    string         `json:"error,omitempty"`    // Optional: If the tool call resulted in an error
	Metadata map[string]any `json:"metadata,omitempty"` // Optional: Additional metadata from the tool
}

// TokenUsage provides information about token usage in an LLM interaction.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LLMResponse represents a response from an LLM.
type LLMResponse struct {
	Content      string
	ToolCalls    []ToolCall
	Usage        TokenUsage
	FinishReason string
}

// StreamChunk represents a chunk of a streaming LLM response.
type StreamChunk struct {
	Delta    string
	ToolCall *ToolCall
	Done     bool
	Error    error
}

// ModelInfo provides details about an available LLM model.
type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	ContextWindow int    `json:"context_window"`
}

// LLMService defines the contract for interacting with LLM providers.
type LLMService interface {
	// Complete performs a completion request.
	Complete(ctx context.Context, provider LLMProvider, req *LLMRequest) (*LLMResponse, error)

	// Stream performs a streaming completion.
	Stream(ctx context.Context, provider LLMProvider, req *LLMRequest) (<-chan StreamChunk, error)

	// ListModels returns available models for a provider.
	ListModels(ctx context.Context, provider LLMProvider) ([]ModelInfo, error)
}
