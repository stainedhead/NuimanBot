package notes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
	notesRepo "nuimanbot/internal/usecase/notes"
)

// Notes implements the domain.Tool interface for note management.
type Notes struct {
	repo   notesRepo.Repository
	config domain.ToolConfig
}

// NewNotes creates a new Notes tool.
func NewNotes(repo notesRepo.Repository) *Notes {
	return &Notes{
		repo: repo,
		config: domain.ToolConfig{
			Enabled: true,
		},
	}
}

// Name returns the tool name.
func (n *Notes) Name() string {
	return "notes"
}

// Description returns the tool description.
func (n *Notes) Description() string {
	return "Create, read, update, delete, and list user notes"
}

// InputSchema returns the JSON schema for the tool's input parameters.
func (n *Notes) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "Operation to perform: 'create', 'read', 'update', 'delete', or 'list'",
				"enum":        []string{"create", "read", "update", "delete", "list"},
			},
			"id": map[string]any{
				"type":        "string",
				"description": "Note ID (required for read, update, delete)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Note title (required for create, optional for update)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Note content (required for create, optional for update)",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Note tags (optional)",
			},
		},
		"required": []string{"operation"},
	}
}

// Execute performs the notes operation.
func (n *Notes) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	// Extract user ID from context
	userID, ok := ctx.Value("user_id").(string)
	if !ok || userID == "" {
		return &domain.ExecutionResult{
			Error: "user_id not found in context",
		}, nil
	}

	// Extract operation
	operation, ok := params["operation"].(string)
	if !ok || operation == "" {
		return &domain.ExecutionResult{
			Error: "missing operation parameter",
		}, nil
	}

	// Execute operation
	switch operation {
	case "create":
		return n.createNote(ctx, userID, params)
	case "read":
		return n.readNote(ctx, params)
	case "update":
		return n.updateNote(ctx, params)
	case "delete":
		return n.deleteNote(ctx, params)
	case "list":
		return n.listNotes(ctx, userID)
	default:
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("invalid operation: %s", operation),
		}, nil
	}
}

// createNote creates a new note.
func (n *Notes) createNote(ctx context.Context, userID string, params map[string]any) (*domain.ExecutionResult, error) {
	title, ok := params["title"].(string)
	if !ok || title == "" {
		return &domain.ExecutionResult{
			Error: "missing title parameter",
		}, nil
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return &domain.ExecutionResult{
			Error: "missing content parameter",
		}, nil
	}

	// Extract tags (optional)
	var tags []string
	if tagsParam, ok := params["tags"].([]interface{}); ok {
		for _, tag := range tagsParam {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// Create note
	note := &domain.Note{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := n.repo.Create(ctx, note); err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to create note: %v", err),
		}, nil
	}

	return &domain.ExecutionResult{
		Output: fmt.Sprintf("Note created successfully with ID: %s", note.ID),
		Metadata: map[string]any{
			"note_id": note.ID,
		},
	}, nil
}

// readNote retrieves a note by ID.
func (n *Notes) readNote(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	noteID, ok := params["id"].(string)
	if !ok || noteID == "" {
		return &domain.ExecutionResult{
			Error: "missing id parameter",
		}, nil
	}

	note, err := n.repo.GetByID(ctx, noteID)
	if err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to read note: %v", err),
		}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Title: %s\n", note.Title))
	output.WriteString(fmt.Sprintf("Content: %s\n", note.Content))
	if len(note.Tags) > 0 {
		output.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(note.Tags, ", ")))
	}
	output.WriteString(fmt.Sprintf("Created: %s\n", note.CreatedAt.Format("2006-01-02 15:04:05")))
	output.WriteString(fmt.Sprintf("Updated: %s", note.UpdatedAt.Format("2006-01-02 15:04:05")))

	return &domain.ExecutionResult{
		Output: output.String(),
		Metadata: map[string]any{
			"note": map[string]any{
				"id":      note.ID,
				"title":   note.Title,
				"content": note.Content,
				"tags":    note.Tags,
			},
		},
	}, nil
}

// updateNote updates an existing note.
func (n *Notes) updateNote(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	noteID, ok := params["id"].(string)
	if !ok || noteID == "" {
		return &domain.ExecutionResult{
			Error: "missing id parameter",
		}, nil
	}

	// Get existing note
	note, err := n.repo.GetByID(ctx, noteID)
	if err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to find note: %v", err),
		}, nil
	}

	// Update fields if provided
	if title, ok := params["title"].(string); ok && title != "" {
		note.Title = title
	}
	if content, ok := params["content"].(string); ok && content != "" {
		note.Content = content
	}
	if tagsParam, ok := params["tags"].([]interface{}); ok {
		var tags []string
		for _, tag := range tagsParam {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
		note.Tags = tags
	}

	if err := n.repo.Update(ctx, note); err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to update note: %v", err),
		}, nil
	}

	return &domain.ExecutionResult{
		Output: "Note updated successfully",
	}, nil
}

// deleteNote deletes a note by ID.
func (n *Notes) deleteNote(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	noteID, ok := params["id"].(string)
	if !ok || noteID == "" {
		return &domain.ExecutionResult{
			Error: "missing id parameter",
		}, nil
	}

	if err := n.repo.Delete(ctx, noteID); err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to delete note: %v", err),
		}, nil
	}

	return &domain.ExecutionResult{
		Output: "Note deleted successfully",
	}, nil
}

// listNotes lists all notes for a user.
func (n *Notes) listNotes(ctx context.Context, userID string) (*domain.ExecutionResult, error) {
	notes, err := n.repo.List(ctx, userID)
	if err != nil {
		return &domain.ExecutionResult{
			Error: fmt.Sprintf("failed to list notes: %v", err),
		}, nil
	}

	if len(notes) == 0 {
		return &domain.ExecutionResult{
			Output: "No notes found",
		}, nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d notes:\n\n", len(notes)))

	for i, note := range notes {
		output.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", i+1, note.Title, note.ID))
		if len(note.Tags) > 0 {
			output.WriteString(fmt.Sprintf("   Tags: %s\n", strings.Join(note.Tags, ", ")))
		}
		output.WriteString(fmt.Sprintf("   Created: %s\n\n", note.CreatedAt.Format("2006-01-02 15:04:05")))
	}

	// Convert notes for metadata
	notesData := make([]map[string]any, len(notes))
	for i, note := range notes {
		notesData[i] = map[string]any{
			"id":    note.ID,
			"title": note.Title,
			"tags":  note.Tags,
		}
	}

	return &domain.ExecutionResult{
		Output: output.String(),
		Metadata: map[string]any{
			"count": len(notes),
			"notes": notesData,
		},
	}, nil
}

// RequiredPermissions returns the permissions required for this tool.
func (n *Notes) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionWrite}
}

// Config returns the tool's configuration.
func (n *Notes) Config() domain.ToolConfig {
	return n.config
}
