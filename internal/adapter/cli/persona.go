package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"nuimanbot/internal/domain"
)

// TemplateLoader loads persona file templates.
type TemplateLoader interface {
	Load(fileType domain.PersonaFileType) (string, error)
}

// PersonaCommand handles persona-related CLI commands.
type PersonaCommand struct {
	repo   domain.PersonaFileRepository
	loader TemplateLoader
	output io.Writer
}

// NewPersonaCommand creates a new persona command handler.
func NewPersonaCommand(
	repo domain.PersonaFileRepository,
	loader TemplateLoader,
	output io.Writer,
) *PersonaCommand {
	return &PersonaCommand{
		repo:   repo,
		loader: loader,
		output: output,
	}
}

// Init initializes persona files (SOUL.md, USER.md, RULES.md) for a user.
// Creates files from templates if they don't already exist.
func (c *PersonaCommand) Init(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	// Check if files already exist
	fileTypes := []domain.PersonaFileType{
		domain.PersonaFileSOUL,
		domain.PersonaFileUSER,
		domain.PersonaFileRULES,
	}

	for _, fileType := range fileTypes {
		existing, err := c.repo.Get(ctx, userID, fileType)
		if err != nil && !errors.Is(err, domain.ErrPersonaFileNotFound) {
			return fmt.Errorf("failed to check existing file %s: %w", fileType.String(), err)
		}
		if existing != nil {
			return fmt.Errorf("persona file %s already exists for user %s", fileType.String(), userID)
		}
	}

	// Load templates and save files
	for _, fileType := range fileTypes {
		template, err := c.loader.Load(fileType)
		if err != nil {
			return fmt.Errorf("failed to load template for %s: %w", fileType.String(), err)
		}

		file := &domain.PersonaFile{
			UserID:  userID,
			Type:    fileType,
			Content: template,
		}

		if err := c.repo.Save(ctx, file); err != nil {
			return fmt.Errorf("failed to save %s: %w", fileType.String(), err)
		}
	}

	// Write success message
	fmt.Fprintf(c.output, "✓ Persona files initialized successfully for user: %s\n", userID)
	fmt.Fprintf(c.output, "  - SOUL.md: Agent persona and communication style\n")
	fmt.Fprintf(c.output, "  - USER.md: Your preferences and context\n")
	fmt.Fprintf(c.output, "  - RULES.md: Hard rules and restrictions\n")
	fmt.Fprintf(c.output, "\nYou can now customize these files to personalize your AI assistant.\n")

	return nil
}
