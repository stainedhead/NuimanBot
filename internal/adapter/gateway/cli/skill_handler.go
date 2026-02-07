package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// SkillExecutor defines the interface for executing skills.
type SkillExecutor interface {
	Execute(ctx context.Context, skillName string, args []string) (*domain.RenderedSkill, error)
	List(ctx context.Context) error
	Describe(ctx context.Context, skillName string) error
}

// SkillHandler wraps a SkillExecutor to implement SkillCommandHandler.
// It handles the rendering and display of skill execution results.
// Phase 7: Integrated with message handler for E2E chat flow.
type SkillHandler struct {
	executor       SkillExecutor
	output         io.Writer
	messageHandler domain.MessageHandler // Phase 7: Added for chat integration
	platform       domain.Platform       // Phase 7: Added for message context
	platformUID    string                // Phase 7: Added for user identification
}

// NewSkillHandler creates a new skill handler wrapper.
func NewSkillHandler(executor SkillExecutor, output io.Writer) *SkillHandler {
	return &SkillHandler{
		executor: executor,
		output:   output,
	}
}

// SetMessageHandler sets the message handler for chat integration.
// This enables skills to pass rendered prompts through the chat service.
func (h *SkillHandler) SetMessageHandler(handler domain.MessageHandler, platform domain.Platform, platformUID string) {
	h.messageHandler = handler
	h.platform = platform
	h.platformUID = platformUID
}

// Execute executes a skill and processes it through the chat service.
// Phase 7: Integrated with chat orchestrator for full E2E functionality.
func (h *SkillHandler) Execute(ctx context.Context, skillName string, args []string) error {
	rendered, err := h.executor.Execute(ctx, skillName, args)
	if err != nil {
		return err
	}

	// Display skill activation
	fmt.Fprintf(h.output, "[Skill activated: %s]\n", skillName)

	// Phase 7: If message handler is available, process through chat service
	if h.messageHandler != nil {
		// Create incoming message with rendered prompt
		skillMessage := domain.IncomingMessage{
			ID:          fmt.Sprintf("skill-%s-%d", skillName, time.Now().UnixNano()),
			Platform:    h.platform,
			PlatformUID: h.platformUID,
			Text:        rendered.Prompt,
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"skill_name":      skillName,
				"skill_args":      args,
				"allowed_tools":   rendered.AllowedTools,
				"is_skill_invoke": true,
			},
		}

		// Process through chat service
		if err := h.messageHandler(ctx, skillMessage); err != nil {
			return fmt.Errorf("failed to process skill through chat service: %w", err)
		}

		return nil
	}

	// Phase 5 fallback: Display only (for testing without chat service)
	fmt.Fprintf(h.output, "\nPrompt:\n%s\n", rendered.Prompt)

	if len(rendered.AllowedTools) > 0 {
		fmt.Fprintf(h.output, "\nAllowed tools: %s\n", strings.Join(rendered.AllowedTools, ", "))
	} else {
		fmt.Fprintf(h.output, "\nAllowed tools: all\n")
	}

	fmt.Fprintf(h.output, "\n[Note: Chat integration not configured. Skill prompt displayed above.]\n")

	return nil
}

// List delegates to the wrapped executor's List method.
func (h *SkillHandler) List(ctx context.Context) error {
	return h.executor.List(ctx)
}

// Describe delegates to the wrapped executor's Describe method.
func (h *SkillHandler) Describe(ctx context.Context, skillName string) error {
	return h.executor.Describe(ctx, skillName)
}
