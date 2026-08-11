package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/usecase/memories"
)

// MemoriesCommandHandler handles the Memories environment's CLI commands
// (FR-037-FR-038). Unlike Projects/Jobs/Chores/History, Memories' per-item
// "chat" sub-command IS in scope here: memories.Service.AskAboutCell is a
// real backing method (the other four environments' chat sub-commands are
// DEFERRED — see project_commands.go's HandleProjectCommand default case
// for why).
type MemoriesCommandHandler struct {
	service *memories.Service
}

// NewMemoriesCommandHandler creates a new Memories environment command handler.
func NewMemoriesCommandHandler(service *memories.Service) *MemoriesCommandHandler {
	return &MemoriesCommandHandler{service: service}
}

// Handle satisfies EnvCommandHandler, delegating to HandleMemoriesCommand.
func (h *MemoriesCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleMemoriesCommand(ctx, currentUser, ownerUserID, input)
}

// HandleMemoriesCommand processes a "/memories ..." command and returns the
// formatted terminal response. ownerUserID (AD-5: the authenticated
// session's Username) scopes every call into the Memories service — never
// currentUser.ID. Memories is available to any authenticated user; no
// admin-role gating is applied here.
func (h *MemoriesCommandHandler) HandleMemoriesCommand(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	subcommand := parts[1]
	args := parts[2:]

	switch subcommand {
	case "browse":
		return h.browse(ctx, ownerUserID, args)
	case "chat":
		return h.chat(ctx, ownerUserID, args)
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown memories command: %s\nUse '/memories help' for usage information.", subcommand), nil
	}
}

// browse lists ownerUserID's memory cells, optionally narrowed by a
// free-text query (FR-037). memoryv2.MemoryCellFilter has no free-text
// query field (only exact Scene / MinSalience filters — matching what
// internal/adapter/web/memories_handler.go's handleMemories exposes), so a
// query is applied client-side as a case-insensitive substring match over
// each cell's Scene and Content, after fetching all of ownerUserID's cells.
// Service.ListCells already enforces ownership (filter.ConversationID is
// always overridden from ownerUserID), so this never risks leaking another
// user's cells.
func (h *MemoriesCommandHandler) browse(ctx context.Context, ownerUserID string, args []string) (string, error) {
	query := strings.Join(args, " ")

	cells, err := h.service.ListCells(ctx, ownerUserID, memoryv2.MemoryCellFilter{})
	if err != nil {
		return "", fmt.Errorf("failed to browse memory cells: %w", err)
	}

	if query != "" {
		cells = filterCellsByQuery(cells, query)
	}

	if len(cells) == 0 {
		if query != "" {
			return fmt.Sprintf("No memory cells found matching %q.", query), nil
		}
		return "No memory cells found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d memory cell(s):\n\n", len(cells)))
	for i, c := range cells {
		result.WriteString(fmt.Sprintf("%d. Scene: %s (ID: %s)\n", i+1, c.Scene, c.ID))
		result.WriteString(fmt.Sprintf("   Type: %s\n", c.CellType.String()))
		result.WriteString(fmt.Sprintf("   Salience: %.2f\n", c.Salience))
		result.WriteString(fmt.Sprintf("   Content: %s\n", c.Content))
		result.WriteString(fmt.Sprintf("   Created: %s\n", c.CreatedAt.Format("2006-01-02 15:04:05")))
	}
	return result.String(), nil
}

// filterCellsByQuery returns cells whose Scene or Content contains query,
// case-insensitively.
func filterCellsByQuery(cells []*memoryv2.MemoryCell, query string) []*memoryv2.MemoryCell {
	needle := strings.ToLower(query)
	matched := make([]*memoryv2.MemoryCell, 0, len(cells))
	for _, c := range cells {
		if strings.Contains(strings.ToLower(c.Scene), needle) || strings.Contains(strings.ToLower(c.Content), needle) {
			matched = append(matched, c)
		}
	}
	return matched
}

// chat asks a single-turn question about one memory cell, grounded only in
// that cell's own content (FR-038). This never edits the cell — the agent
// remains the sole writer of memory content, same as web (AskAboutCell
// makes an outbound LLM call but never mutates the memory-cell store).
// Usage: /memories chat <cell-id> <message...>
func (h *MemoriesCommandHandler) chat(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /memories chat <cell-id> <message>", nil
	}

	cellID := args[0]
	question := strings.Join(args[1:], " ")

	answer, err := h.service.AskAboutCell(ctx, ownerUserID, cellID, question)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, memoryv2.ErrNotFound) {
			return fmt.Sprintf("Memory cell not found: %s", cellID), nil
		}
		return "", fmt.Errorf("failed to get an answer: %w", err)
	}

	return fmt.Sprintf("Q: %s\nA: %s", question, answer), nil
}

// showHelp returns help text for Memories environment commands.
func (h *MemoriesCommandHandler) showHelp() string {
	return `Memories Commands:

  /memories browse [query]
    Search/browse your stored memory cells. Omit query to list all.

  /memories chat <cell-id> <message>
    Ask a question about a specific memory cell; the answer is grounded
    only in that cell's own content. This does not edit the cell — the
    agent remains the sole writer of memory content, same as web.

  /memories help
    Show this help message
`
}
