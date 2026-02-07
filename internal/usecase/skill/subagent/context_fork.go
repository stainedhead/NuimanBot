package subagent

import (
	"context"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"time"

	"github.com/google/uuid"
)

// ContextForker handles forking conversation contexts for subagent execution
type ContextForker struct{}

// NewContextForker creates a new ContextForker instance
func NewContextForker() *ContextForker {
	return &ContextForker{}
}

// Fork creates an isolated subagent context from a parent conversation
func (f *ContextForker) Fork(
	ctx context.Context,
	parentCtxID string,
	parentHistory []domain.Message,
	skillName string,
	allowedTools []string,
	resourceLimits domain.ResourceLimits,
) (*domain.SubagentContext, error) {
	// Validate inputs
	if parentCtxID == "" {
		return nil, errors.New("parent context ID is required")
	}
	if skillName == "" {
		return nil, errors.New("skill name is required")
	}

	// Generate unique subagent ID
	subagentID := fmt.Sprintf("subagent-%s", uuid.New().String())

	// Deep copy conversation history to ensure isolation
	copiedHistory := make([]domain.Message, len(parentHistory))
	for i, msg := range parentHistory {
		copiedHistory[i] = domain.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Deep copy allowed tools to ensure isolation
	var copiedTools []string
	if allowedTools != nil {
		copiedTools = make([]string, len(allowedTools))
		copy(copiedTools, allowedTools)
	}

	// Create subagent context
	subagentCtx := &domain.SubagentContext{
		ID:                  subagentID,
		ParentContextID:     parentCtxID,
		SkillName:           skillName,
		AllowedTools:        copiedTools,
		ResourceLimits:      resourceLimits,
		ConversationHistory: copiedHistory,
		CreatedAt:           time.Now(),
		Metadata:            make(map[string]interface{}),
	}

	// Validate the created context
	if err := subagentCtx.Validate(); err != nil {
		return nil, fmt.Errorf("failed to create valid subagent context: %w", err)
	}

	return subagentCtx, nil
}
