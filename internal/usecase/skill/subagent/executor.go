package subagent

import (
	"context"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"time"
)

// LLMService defines the interface for LLM interactions
type LLMService interface {
	Chat(ctx context.Context, req domain.LLMRequest) (domain.LLMResponse, error)
}

// ToolExecutor defines the interface for executing tools
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
}

// SubagentExecutor implements autonomous multi-step subagent execution
type SubagentExecutor struct {
	llmService    LLMService
	toolExecutor  ToolExecutor
	maxIterations int
}

// NewSubagentExecutor creates a new SubagentExecutor
func NewSubagentExecutor(llmService LLMService, toolExecutor ToolExecutor) *SubagentExecutor {
	return &SubagentExecutor{
		llmService:    llmService,
		toolExecutor:  toolExecutor,
		maxIterations: 50, // Default maximum iterations to prevent runaway
	}
}

// finalizeResult sets the final fields on a result before returning
func (e *SubagentExecutor) finalizeResult(result *domain.SubagentResult, status domain.SubagentStatus, errorMsg string, tokensUsed, toolCallsMade int, startTime time.Time) *domain.SubagentResult {
	result.Status = status
	result.ErrorMessage = errorMsg
	result.ExecutionTime = time.Since(startTime)
	result.CompletedAt = time.Now()
	result.TokensUsed = tokensUsed
	result.ToolCallsMade = toolCallsMade
	return result
}

// Execute runs a subagent in an isolated context with autonomous multi-step execution
func (e *SubagentExecutor) Execute(ctx context.Context, subagentCtx domain.SubagentContext) (*domain.SubagentResult, error) {
	startTime := time.Now()

	// Initialize result
	result := &domain.SubagentResult{
		SubagentID:  subagentCtx.ID,
		Status:      domain.SubagentStatusRunning,
		StepResults: []domain.SubagentStepResult{},
		Metadata:    make(map[string]interface{}),
	}

	// Initialize conversation with subagent history
	conversation := make([]domain.Message, len(subagentCtx.ConversationHistory))
	copy(conversation, subagentCtx.ConversationHistory)

	// Execution loop
	var tokensUsed int
	var toolCallsMade int
	stepNumber := 1

	for i := 0; i < e.maxIterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return e.finalizeResult(result, domain.SubagentStatusCancelled, "execution cancelled", tokensUsed, toolCallsMade, startTime), nil
		default:
		}

		// Check resource limits before making LLM call
		elapsed := time.Since(startTime)
		if !subagentCtx.ResourceLimits.IsWithinLimits(tokensUsed, toolCallsMade, elapsed) {
			return e.finalizeResult(result, domain.SubagentStatusTimeout, "resource limits exceeded", tokensUsed, toolCallsMade, startTime), nil
		}

		// Make LLM call
		stepStartTime := time.Now()
		req := domain.LLMRequest{
			Messages: conversation,
		}

		resp, err := e.llmService.Chat(ctx, req)
		if err != nil {
			return e.finalizeResult(result, domain.SubagentStatusError, fmt.Sprintf("LLM error: %v", err), tokensUsed, toolCallsMade, startTime), nil
		}

		// Track token usage
		tokensUsed += resp.Usage.TotalTokens

		// Check token limit after response
		if subagentCtx.ResourceLimits.MaxTokens > 0 && tokensUsed > subagentCtx.ResourceLimits.MaxTokens {
			errorMsg := fmt.Sprintf("token limit exceeded: %d > %d", tokensUsed, subagentCtx.ResourceLimits.MaxTokens)
			return e.finalizeResult(result, domain.SubagentStatusError, errorMsg, tokensUsed, toolCallsMade, startTime), nil
		}

		// Add assistant response to conversation
		conversation = append(conversation, domain.Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		// Record step result (LLM call only, not individual tool executions)
		stepResult := domain.SubagentStepResult{
			StepNumber: stepNumber,
			Action:     fmt.Sprintf("LLM call (finish: %s, tools: %d)", resp.FinishReason, len(resp.ToolCalls)),
			Result:     resp.Content,
			TokensUsed: resp.Usage.TotalTokens,
			Duration:   time.Since(stepStartTime),
		}
		result.StepResults = append(result.StepResults, stepResult)
		stepNumber++

		// Check if we're done (no tool calls)
		if resp.FinishReason == "end_turn" || len(resp.ToolCalls) == 0 {
			result.Output = resp.Content
			return e.finalizeResult(result, domain.SubagentStatusComplete, "", tokensUsed, toolCallsMade, startTime), nil
		}

		// Handle tool calls
		for _, toolCall := range resp.ToolCalls {
			// Check tool call limit BEFORE making the call
			if subagentCtx.ResourceLimits.MaxToolCalls > 0 && toolCallsMade >= subagentCtx.ResourceLimits.MaxToolCalls {
				errorMsg := fmt.Sprintf("tool call limit exceeded: %d >= %d", toolCallsMade, subagentCtx.ResourceLimits.MaxToolCalls)
				return e.finalizeResult(result, domain.SubagentStatusError, errorMsg, tokensUsed, toolCallsMade, startTime), nil
			}

			// Check tool restriction
			if !e.isToolAllowed(toolCall.ToolName, subagentCtx.AllowedTools) {
				errorMsg := fmt.Sprintf("tool '%s' not allowed (allowed: %v)", toolCall.ToolName, subagentCtx.AllowedTools)
				return e.finalizeResult(result, domain.SubagentStatusError, errorMsg, tokensUsed, toolCallsMade, startTime), nil
			}

			// Execute tool
			toolResult, err := e.toolExecutor.Execute(ctx, toolCall.ToolName, toolCall.Arguments)
			if err != nil {
				errorMsg := fmt.Sprintf("tool execution error: %v", err)
				return e.finalizeResult(result, domain.SubagentStatusError, errorMsg, tokensUsed, toolCallsMade, startTime), nil
			}

			toolCallsMade++

			// Add tool result to conversation
			conversation = append(conversation, domain.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool result from %s: %s", toolCall.ToolName, toolResult),
			})
		}

		// Check resource limits after tool calls
		elapsed = time.Since(startTime)
		if !subagentCtx.ResourceLimits.IsWithinLimits(tokensUsed, toolCallsMade, elapsed) {
			return e.finalizeResult(result, domain.SubagentStatusError, "resource limits exceeded after tool calls", tokensUsed, toolCallsMade, startTime), nil
		}
	}

	// Max iterations reached
	errorMsg := fmt.Sprintf("max iterations (%d) reached", e.maxIterations)
	return e.finalizeResult(result, domain.SubagentStatusError, errorMsg, tokensUsed, toolCallsMade, startTime), nil
}

// Cancel terminates a running subagent (stub for now, will be implemented in lifecycle management)
func (e *SubagentExecutor) Cancel(ctx context.Context, subagentID string) error {
	return errors.New("not implemented: lifecycle management in P3A.4")
}

// GetStatus retrieves the current status of a subagent (stub for now, will be implemented in lifecycle management)
func (e *SubagentExecutor) GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	return nil, errors.New("not implemented: lifecycle management in P3A.4")
}

// isToolAllowed checks if a tool is allowed for this subagent
func (e *SubagentExecutor) isToolAllowed(toolName string, allowedTools []string) bool {
	// nil means all tools allowed
	if allowedTools == nil {
		return true
	}

	// Empty slice means no tools allowed
	if len(allowedTools) == 0 {
		return false
	}

	// Check if tool is in allowed list
	for _, allowed := range allowedTools {
		if allowed == toolName {
			return true
		}
	}

	return false
}
