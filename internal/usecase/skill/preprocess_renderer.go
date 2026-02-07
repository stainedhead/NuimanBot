package skill

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// CommandExecutor defines the interface for executing preprocessing commands
type CommandExecutor interface {
	Execute(ctx context.Context, cmd domain.PreprocessCommand) (*domain.CommandResult, error)
}

// PreprocessRenderer renders skills with preprocessing command substitution
type PreprocessRenderer struct {
	executor    CommandExecutor
	argRenderer *DefaultSkillRenderer
}

// NewPreprocessRenderer creates a new preprocessing renderer
func NewPreprocessRenderer(executor CommandExecutor) *PreprocessRenderer {
	return &PreprocessRenderer{
		executor:    executor,
		argRenderer: NewDefaultSkillRenderer(),
	}
}

// Render renders a skill with preprocessing and argument substitution
func (r *PreprocessRenderer) Render(ctx context.Context, skill *domain.Skill, args []string) (*domain.RenderedSkill, error) {
	// Start with the skill body
	content := skill.BodyMD

	// Step 1: Execute preprocessing commands
	processedContent, err := r.executePreprocessing(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("preprocessing failed: %w", err)
	}

	// Step 2: Apply argument substitution using the default renderer
	// Create a temporary skill with the processed content
	processedSkill := &domain.Skill{
		Name:        skill.Name,
		Description: skill.Description,
		Frontmatter: skill.Frontmatter,
		BodyMD:      processedContent,
	}

	return r.argRenderer.Render(processedSkill, args)
}

// executePreprocessing finds and executes all !command blocks
func (r *PreprocessRenderer) executePreprocessing(ctx context.Context, content string) (string, error) {
	// Parse content for !command blocks
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	inCommandBlock := false
	var commandLines []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "!command" {
			// Start of command block
			inCommandBlock = true
			commandLines = []string{}
			continue
		}

		if inCommandBlock {
			// Empty line ends the command block
			if strings.TrimSpace(line) == "" {
				// Execute the command
				commandStr := strings.Join(commandLines, "\n")
				output := r.executeCommand(ctx, commandStr)

				// Add output to result (with code block formatting)
				result = append(result, "```")
				result = append(result, output)
				result = append(result, "```")
				result = append(result, "") // Empty line after code block

				inCommandBlock = false
				commandLines = []string{}
				continue
			}

			// Add line to command
			commandLines = append(commandLines, line)
			continue
		}

		// Regular line
		result = append(result, line)
	}

	// Handle command block at end of file
	if inCommandBlock && len(commandLines) > 0 {
		commandStr := strings.Join(commandLines, "\n")
		output := r.executeCommand(ctx, commandStr)

		result = append(result, "```")
		result = append(result, output)
		result = append(result, "```")
	}

	return strings.Join(result, "\n"), nil
}

// executeCommand executes a single command and returns the output or error message
func (r *PreprocessRenderer) executeCommand(ctx context.Context, commandStr string) string {
	cmd := domain.PreprocessCommand{
		Command: strings.TrimSpace(commandStr),
		Timeout: domain.MaxCommandTimeout,
	}

	result, err := r.executor.Execute(ctx, cmd)

	if err != nil {
		return fmt.Sprintf("ERROR: Failed to execute command: %v", err)
	}

	if result.IsError() {
		return fmt.Sprintf("ERROR: Command failed with exit code %d\n%s\n%s",
			result.ExitCode, result.Output, result.Error)
	}

	return result.TruncatedOutput()
}
