package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// SkillRegistry defines the interface for skill registry operations.
// This interface is satisfied by the usecase layer implementation.
type SkillRegistry interface {
	Get(name string) (*domain.Skill, error)
	UserInvocableCatalog() []domain.SkillCatalogEntry
}

// SkillRenderer defines the interface for skill rendering operations.
// This interface is satisfied by the usecase layer implementation.
type SkillRenderer interface {
	Render(skill *domain.Skill, args []string) (*domain.RenderedSkill, error)
}

// LifecycleManager defines the interface for subagent lifecycle management.
type LifecycleManager interface {
	Start(ctx context.Context, subagentCtx domain.SubagentContext) error
	Cancel(ctx context.Context, subagentID string) error
	GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error)
	ListRunning(ctx context.Context) []string
}

// SkillCommand handles skill-related CLI commands.
// It provides operations to execute, list, and describe skills.
type SkillCommand struct {
	registry  SkillRegistry
	renderer  SkillRenderer
	output    io.Writer
	lifecycle LifecycleManager
}

// NewSkillCommand creates a new skill command handler.
func NewSkillCommand(
	registry SkillRegistry,
	renderer SkillRenderer,
	output io.Writer,
) *SkillCommand {
	return &SkillCommand{
		registry: registry,
		renderer: renderer,
		output:   output,
	}
}

// SetLifecycleManager sets the lifecycle manager for subagent execution.
func (c *SkillCommand) SetLifecycleManager(lifecycle LifecycleManager) {
	c.lifecycle = lifecycle
}

// Execute executes a skill by name with arguments.
// Returns rendered prompt and allowed tools.
// For skills with context: fork, starts a subagent and returns immediately.
//
// Errors:
//   - ErrSkillNotFound if skill does not exist
//   - Error if skill is not user-invocable
//   - Error if rendering fails
func (c *SkillCommand) Execute(ctx context.Context, skillName string, args []string) (*domain.RenderedSkill, error) {
	// Get skill from registry
	skill, err := c.registry.Get(skillName)
	if err != nil {
		return nil, err
	}

	// Check if user can invoke this skill
	if !skill.CanBeInvokedByUser() {
		return nil, fmt.Errorf("skill %s is not user-invocable", skillName)
	}

	// Render skill with arguments
	rendered, err := c.renderer.Render(skill, args)
	if err != nil {
		return nil, fmt.Errorf("failed to render skill: %w", err)
	}

	// Check if skill should fork
	if skill.ShouldFork() && c.lifecycle != nil {
		return c.executeAsSubagent(ctx, skill, rendered)
	}

	return rendered, nil
}

// executeAsSubagent starts a skill as a forked subagent
func (c *SkillCommand) executeAsSubagent(ctx context.Context, skill *domain.Skill, rendered *domain.RenderedSkill) (*domain.RenderedSkill, error) {
	// Create subagent context
	subagentCtx := domain.SubagentContext{
		ID:              fmt.Sprintf("subagent-%s-%d", skill.Name, time.Now().UnixNano()),
		ParentContextID: "cli-parent",
		SkillName:       skill.Name,
		AllowedTools:    rendered.AllowedTools,
		ResourceLimits:  domain.DefaultResourceLimits(),
		ConversationHistory: []domain.Message{
			{Role: "user", Content: rendered.Prompt},
		},
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Start subagent
	if err := c.lifecycle.Start(ctx, subagentCtx); err != nil {
		return nil, fmt.Errorf("failed to start subagent: %w", err)
	}

	// Display notification
	fmt.Fprintf(c.output, "Started subagent: %s (ID: %s)\n", skill.Name, subagentCtx.ID)
	fmt.Fprintf(c.output, "Use /subagent-status %s to check progress\n", subagentCtx.ID)

	return rendered, nil
}

// GetSubagentStatus retrieves the status of a running subagent
func (c *SkillCommand) GetSubagentStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	if c.lifecycle == nil {
		return nil, fmt.Errorf("lifecycle manager not configured")
	}

	return c.lifecycle.GetStatus(ctx, subagentID)
}

// ListRunningSubagents lists all currently running subagents
func (c *SkillCommand) ListRunningSubagents(ctx context.Context) error {
	if c.lifecycle == nil {
		return fmt.Errorf("lifecycle manager not configured")
	}

	running := c.lifecycle.ListRunning(ctx)

	if len(running) == 0 {
		fmt.Fprintln(c.output, "No running subagents.")
		return nil
	}

	fmt.Fprintf(c.output, "Running subagents (%d):\n", len(running))
	for _, id := range running {
		status, err := c.lifecycle.GetStatus(ctx, id)
		if err != nil {
			fmt.Fprintf(c.output, "  %s (error getting status)\n", id)
			continue
		}
		fmt.Fprintf(c.output, "  %s - %s\n", id, status.Status)
	}

	return nil
}

// List lists all available user-invocable skills.
// Displays a formatted list of skills with their descriptions.
func (c *SkillCommand) List(ctx context.Context) error {
	catalog := c.registry.UserInvocableCatalog()

	if len(catalog) == 0 {
		fmt.Fprintln(c.output, "No user-invocable skills found.")
		return nil
	}

	fmt.Fprintln(c.output, "Available skills:")
	for _, entry := range catalog {
		fmt.Fprintf(c.output, "  /%s - %s\n", entry.Name, entry.Description)
	}

	return nil
}

// Describe shows detailed information about a skill.
// Displays skill metadata, permissions, and full body content.
func (c *SkillCommand) Describe(ctx context.Context, skillName string) error {
	skill, err := c.registry.Get(skillName)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.output, "Skill: %s\n", skill.Name)
	fmt.Fprintf(c.output, "Description: %s\n", skill.Description)
	fmt.Fprintf(c.output, "Scope: %s\n", skill.Scope)
	fmt.Fprintf(c.output, "User-invocable: %t\n", skill.CanBeInvokedByUser())
	fmt.Fprintf(c.output, "Model-invocable: %t\n", skill.CanBeSelectedByModel())

	if len(skill.AllowedTools()) > 0 {
		fmt.Fprintf(c.output, "Allowed tools: %s\n", strings.Join(skill.AllowedTools(), ", "))
	} else {
		fmt.Fprintln(c.output, "Allowed tools: all")
	}

	fmt.Fprintf(c.output, "\nBody:\n%s\n", skill.BodyMD)

	return nil
}
