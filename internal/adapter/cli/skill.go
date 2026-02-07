package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

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

// SkillCommand handles skill-related CLI commands.
// It provides operations to execute, list, and describe skills.
type SkillCommand struct {
	registry SkillRegistry
	renderer SkillRenderer
	output   io.Writer
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

// Execute executes a skill by name with arguments.
// Returns rendered prompt and allowed tools.
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

	return rendered, nil
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
