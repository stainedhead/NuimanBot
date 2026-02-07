package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

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
type SkillHandler struct {
	executor SkillExecutor
	output   io.Writer
}

// NewSkillHandler creates a new skill handler wrapper.
func NewSkillHandler(executor SkillExecutor, output io.Writer) *SkillHandler {
	return &SkillHandler{
		executor: executor,
		output:   output,
	}
}

// Execute executes a skill and displays the result.
// This is a wrapper that adapts the SkillExecutor.Execute() to the
// SkillCommandHandler.Execute() interface.
func (h *SkillHandler) Execute(ctx context.Context, skillName string, args []string) error {
	rendered, err := h.executor.Execute(ctx, skillName, args)
	if err != nil {
		return err
	}

	// Display skill activation (Phase 5: display only, Phase 7: integrate with chat)
	fmt.Fprintf(h.output, "[Skill activated: %s]\n", skillName)
	fmt.Fprintf(h.output, "\nPrompt:\n%s\n", rendered.Prompt)

	if len(rendered.AllowedTools) > 0 {
		fmt.Fprintf(h.output, "\nAllowed tools: %s\n", strings.Join(rendered.AllowedTools, ", "))
	} else {
		fmt.Fprintf(h.output, "\nAllowed tools: all\n")
	}

	// TODO Phase 7: Integrate with chat orchestrator
	// - Pass rendered.Prompt to chat service
	// - Apply tool restrictions from rendered.AllowedTools
	// - Process the skill prompt through the LLM

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
