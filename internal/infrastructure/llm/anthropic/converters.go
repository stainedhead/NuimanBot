package anthropic

import (
	"encoding/json"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"

	"nuimanbot/internal/domain"
)

const (
	// Content block types
	contentTypeText    = "text"
	contentTypeToolUse = "tool_use"

	// Message roles
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"

	// Schema type
	schemaTypeObject = "object"
)

// convertMessages converts domain.Message slice to Anthropic SDK format
func convertMessages(messages []domain.Message) []anthropicsdk.MessageParam {
	result := make([]anthropicsdk.MessageParam, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages - they're handled separately in SystemPrompt
		if msg.Role == roleSystem {
			continue
		}

		// Create text content block
		contentBlock := anthropicsdk.NewTextBlock(msg.Content)

		// Create message based on role
		msgParam := anthropicsdk.MessageParam{
			Role:    anthropicsdk.MessageParamRole(msg.Role),
			Content: []anthropicsdk.ContentBlockParamUnion{contentBlock},
		}

		result = append(result, msgParam)
	}

	return result
}

// convertTools converts domain.ToolDefinition slice to Anthropic SDK format
func convertTools(tools []domain.ToolDefinition) []anthropicsdk.ToolUnionParam {
	result := make([]anthropicsdk.ToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		// Convert input schema map to ToolInputSchemaParam
		inputSchema := anthropicsdk.ToolInputSchemaParam{
			Type:       "object",
			Properties: tool.InputSchema["properties"],
		}

		// Extract required fields if present
		if req, ok := tool.InputSchema["required"].([]string); ok {
			inputSchema.Required = req
		} else if reqI, ok := tool.InputSchema["required"].([]interface{}); ok {
			// Convert []interface{} to []string
			required := make([]string, 0, len(reqI))
			for _, r := range reqI {
				if rStr, ok := r.(string); ok {
					required = append(required, rStr)
				}
			}
			inputSchema.Required = required
		}

		// Use helper function to create ToolUnionParam
		toolParam := anthropicsdk.ToolUnionParamOfTool(
			inputSchema,
			tool.Name,
		)

		// Set description if provided
		if tool.Description != "" {
			toolParam.OfTool.Description = anthropicsdk.String(tool.Description)
		}

		result = append(result, toolParam)
	}

	return result
}

// convertInputSchema converts a map-based schema to ToolInputSchemaParam
func convertInputSchema(schema map[string]any) anthropicsdk.ToolInputSchemaParam {
	inputSchema := anthropicsdk.ToolInputSchemaParam{
		Type:       schemaTypeObject,
		Properties: schema["properties"],
	}

	// Extract required fields if present
	if req, ok := schema["required"].([]string); ok {
		inputSchema.Required = req
	} else if reqI, ok := schema["required"].([]interface{}); ok {
		// Convert []interface{} to []string
		required := make([]string, 0, len(reqI))
		for _, r := range reqI {
			if rStr, ok := r.(string); ok {
				required = append(required, rStr)
			}
		}
		inputSchema.Required = required
	}

	return inputSchema
}

// createToolParam creates a ToolUnionParam from name, description, and schema
func createToolParam(name, description string, schema anthropicsdk.ToolInputSchemaParam) anthropicsdk.ToolUnionParam {
	toolParam := anthropicsdk.ToolUnionParamOfTool(schema, name)

	// Set description if provided
	if description != "" {
		toolParam.OfTool.Description = anthropicsdk.String(description)
	}

	return toolParam
}

// convertResponse converts Anthropic SDK response to domain.LLMResponse
func convertResponse(response *anthropicsdk.Message) *domain.LLMResponse {
	result := &domain.LLMResponse{
		Content:      "",
		ToolCalls:    []domain.ToolCall{},
		FinishReason: string(response.StopReason),
		Usage: domain.TokenUsage{
			PromptTokens:     int(response.Usage.InputTokens),
			CompletionTokens: int(response.Usage.OutputTokens),
			TotalTokens:      int(response.Usage.InputTokens + response.Usage.OutputTokens),
		},
	}

	// Extract content and tool calls from response
	for _, content := range response.Content {
		switch content.Type {
		case contentTypeText:
			result.Content += content.Text

		case contentTypeToolUse:
			toolCall := parseToolCall(content)
			result.ToolCalls = append(result.ToolCalls, toolCall)
		}
	}

	return result
}

// parseToolCall extracts a tool call from a content block
func parseToolCall(content anthropicsdk.ContentBlockUnion) domain.ToolCall {
	toolCall := domain.ToolCall{
		ToolName:  content.Name,
		Arguments: make(map[string]any),
	}

	// Unmarshal JSON input to map
	if len(content.Input) > 0 {
		var inputMap map[string]interface{}
		if err := json.Unmarshal(content.Input, &inputMap); err == nil {
			toolCall.Arguments = inputMap
		}
	}

	return toolCall
}

// convertToolResults converts domain.ToolResult slice to Anthropic content blocks
func convertToolResults(toolResults []domain.ToolResult, toolCallID string) anthropicsdk.MessageParam {
	contentBlocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(toolResults))

	for _, result := range toolResults {
		// Create tool result content block
		isError := result.Error != ""
		content := result.Output
		if isError {
			content = fmt.Sprintf("Error: %s", result.Error)
		}

		toolResultBlock := anthropicsdk.NewToolResultBlock(
			toolCallID,
			content,
			isError,
		)

		contentBlocks = append(contentBlocks, toolResultBlock)
	}

	return anthropicsdk.MessageParam{
		Role:    anthropicsdk.MessageParamRoleUser,
		Content: contentBlocks,
	}
}
