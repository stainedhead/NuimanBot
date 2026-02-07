package chat

import (
	"context"
	"strings"

	"nuimanbot/internal/domain"
)

// convertSkillsToTools converts a list of skills to LLM tool definitions
func convertSkillsToTools(skills []domain.Tool) []domain.ToolDefinition {
	tools := make([]domain.ToolDefinition, 0, len(skills))

	for _, skill := range skills {
		tool := domain.ToolDefinition{
			Name:        skill.Name(),
			Description: skill.Description(),
			InputSchema: skill.InputSchema(),
		}
		tools = append(tools, tool)
	}

	return tools
}

// executeToolCalls executes a list of tool calls and returns their results
func (s *Service) executeToolCalls(ctx context.Context, toolCalls []domain.ToolCall) []domain.ToolResult {
	results := make([]domain.ToolResult, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		result, err := s.toolExecService.Execute(ctx, toolCall.ToolName, toolCall.Arguments)

		toolResult := domain.ToolResult{
			ToolName: toolCall.ToolName,
		}

		if err != nil {
			toolResult.Error = err.Error()
		} else if result.Error != "" {
			// Skill returned an error in the result
			toolResult.Error = result.Error
		} else {
			toolResult.Output = result.Output
			toolResult.Metadata = result.Metadata
		}

		results = append(results, toolResult)
	}

	return results
}

// formatToolResults formats tool results into a text representation for the LLM
func formatToolResults(results []domain.ToolResult) string {
	if len(results) == 0 {
		return "No tool results."
	}

	var formatted string
	for i, result := range results {
		if i > 0 {
			formatted += "\n\n"
		}

		formatted += "Tool: " + result.ToolName + "\n"

		if result.Error != "" {
			formatted += "Error: " + result.Error
		} else {
			formatted += "Result: " + result.Output
		}
	}

	return formatted
}

// buildCacheKey creates a stable cache key from conversation messages.
// The key is a concatenation of all message roles and content.
func buildCacheKey(messages []domain.Message) string {
	var builder strings.Builder
	for i, msg := range messages {
		if i > 0 {
			builder.WriteString("\n---\n")
		}
		builder.WriteString(msg.Role)
		builder.WriteString(": ")
		builder.WriteString(msg.Content)
	}
	return builder.String()
}
