package bedrock

import (
	"strings"

	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Model ID translation map: Anthropic names -> Bedrock IDs
var modelIDMap = map[string]string{
	"claude-3-5-sonnet-20241022": "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-opus-20240229":     "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-sonnet-20240229":   "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-haiku-20240307":    "anthropic.claude-3-haiku-20240307-v1:0",
}

// translateModelID converts Anthropic model names to Bedrock model IDs.
// If the input is already a Bedrock ID or unknown, it returns the input unchanged.
func translateModelID(modelName string) string {
	if bedrockID, ok := modelIDMap[modelName]; ok {
		return bedrockID
	}
	return modelName
}

// convertMessages converts domain messages to Bedrock format.
// System messages are extracted into a separate system content block array.
func convertMessages(messages []domain.Message, systemPrompt string) ([]types.Message, []types.SystemContentBlock) {
	var bedrockMessages []types.Message
	var systemBlocks []types.SystemContentBlock

	// Add system prompt if provided
	if systemPrompt != "" {
		systemBlocks = append(systemBlocks, &types.SystemContentBlockMemberText{
			Value: systemPrompt,
		})
	}

	// Convert messages
	for _, msg := range messages {
		// Skip system messages - already handled above
		if msg.Role == "system" {
			continue
		}

		// Create text content block
		contentBlock := &types.ContentBlockMemberText{
			Value: msg.Content,
		}

		// Determine role
		var role types.ConversationRole
		if msg.Role == "user" {
			role = types.ConversationRoleUser
		} else {
			role = types.ConversationRoleAssistant
		}

		// Create message
		bedrockMsg := types.Message{
			Role:    role,
			Content: []types.ContentBlock{contentBlock},
		}

		bedrockMessages = append(bedrockMessages, bedrockMsg)
	}

	return bedrockMessages, systemBlocks
}

// convertTools converts domain tool definitions to Bedrock format.
func convertTools(tools []domain.ToolDefinition) []types.Tool {
	result := make([]types.Tool, 0, len(tools))

	for _, tool := range tools {
		// Convert input schema to document
		inputSchemaDoc := document.NewLazyDocument(tool.InputSchema)

		// Create tool specification
		toolSpec := types.ToolSpecification{
			Name:        aws.String(tool.Name),
			Description: aws.String(tool.Description),
			InputSchema: &types.ToolInputSchemaMemberJson{
				Value: inputSchemaDoc,
			},
		}

		result = append(result, &types.ToolMemberToolSpec{
			Value: toolSpec,
		})
	}

	return result
}

// convertResponse converts Bedrock ConverseOutput to domain.LLMResponse.
func convertResponse(output *bedrockruntime.ConverseOutput) *domain.LLMResponse {
	result := &domain.LLMResponse{
		Content:      "",
		ToolCalls:    []domain.ToolCall{},
		FinishReason: normalizeStopReason(output.StopReason),
		Usage:        convertUsage(output.Usage),
	}

	// Extract message from Output interface
	if msgOutput, ok := output.Output.(*types.ConverseOutputMemberMessage); ok {
		for _, content := range msgOutput.Value.Content {
			switch block := content.(type) {
			case *types.ContentBlockMemberText:
				result.Content += block.Value

			case *types.ContentBlockMemberToolUse:
				toolCall := parseToolUseBlock(block.Value)
				result.ToolCalls = append(result.ToolCalls, toolCall)
			}
		}
	}

	return result
}

// convertUsage converts Bedrock TokenUsage to domain.TokenUsage.
func convertUsage(usage *types.TokenUsage) domain.TokenUsage {
	if usage == nil {
		return domain.TokenUsage{}
	}

	inputTokens := 0
	outputTokens := 0
	totalTokens := 0

	if usage.InputTokens != nil {
		inputTokens = int(*usage.InputTokens)
	}
	if usage.OutputTokens != nil {
		outputTokens = int(*usage.OutputTokens)
	}
	if usage.TotalTokens != nil {
		totalTokens = int(*usage.TotalTokens)
	}

	return domain.TokenUsage{
		PromptTokens:     inputTokens,
		CompletionTokens: outputTokens,
		TotalTokens:      totalTokens,
	}
}

// normalizeStopReason converts Bedrock stop reason to a normalized string.
func normalizeStopReason(reason types.StopReason) string {
	// Convert enum to string and lowercase with underscores
	s := string(reason)
	return strings.ToLower(strings.ReplaceAll(s, " ", "_"))
}

// parseToolUseBlock extracts a tool call from a Bedrock ToolUseBlock.
func parseToolUseBlock(block types.ToolUseBlock) domain.ToolCall {
	toolCall := domain.ToolCall{
		ToolName:  "",
		Arguments: make(map[string]any),
	}

	if block.Name != nil {
		toolCall.ToolName = *block.Name
	}

	if block.Input != nil {
		// Unmarshal document.Interface to map
		var inputMap map[string]interface{}
		err := block.Input.UnmarshalSmithyDocument(&inputMap)
		// The SDK returns an error about type but still populates the map
		// So we use the map regardless of error
		if len(inputMap) > 0 {
			toolCall.Arguments = inputMap
		}
		_ = err // Ignore error as SDK populates map despite reporting type error
	}

	return toolCall
}
