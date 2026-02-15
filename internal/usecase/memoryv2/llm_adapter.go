package memoryv2

import (
	"context"
	"encoding/json"
	"fmt"

	"nuimanbot/internal/domain"
)

// LLMServiceAdapter adapts domain.LLMService to the LLMClient interface
type LLMServiceAdapter struct {
	service  domain.LLMService
	provider domain.LLMProvider
	model    string
}

// NewLLMServiceAdapter creates a new LLM service adapter
func NewLLMServiceAdapter(service domain.LLMService, provider domain.LLMProvider, model string) *LLMServiceAdapter {
	return &LLMServiceAdapter{
		service:  service,
		provider: provider,
		model:    model,
	}
}

// GenerateJSON calls the LLM and expects a JSON response
func (a *LLMServiceAdapter) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, responseSchema interface{}) (string, error) {
	// Build request
	req := &domain.LLMRequest{
		Model:        a.model,
		SystemPrompt: systemPrompt,
		Messages: []domain.Message{
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	// Add JSON schema hint to system prompt
	schemaJSON, err := json.MarshalIndent(responseSchema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response schema: %w", err)
	}

	req.SystemPrompt = systemPrompt + fmt.Sprintf("\n\nYou must respond with valid JSON matching this schema:\n```json\n%s\n```", string(schemaJSON))

	// Call LLM
	resp, err := a.service.Complete(ctx, a.provider, req)
	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	// Extract text content
	content := resp.Content
	if content == "" {
		return "", fmt.Errorf("empty response from LLM")
	}

	// Validate it's valid JSON
	var testParse interface{}
	if err := json.Unmarshal([]byte(content), &testParse); err != nil {
		return "", fmt.Errorf("LLM returned invalid JSON: %w", err)
	}

	return content, nil
}
