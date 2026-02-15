package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// MemoryCommand handles memory-related CLI commands.
// It provides operations to list, get, search, delete cells, list scenes, prune expired cells,
// and admin operations (stats, clear-user, export, import, rebuild-fts).
type MemoryCommand struct {
	cellRepo  memoryv2.MemoryCellRepository
	sceneRepo memoryv2.MemorySceneRepository
	admin     MemoryAdmin // Optional admin operations
	output    io.Writer
}

// NewMemoryCommand creates a new memory command handler.
func NewMemoryCommand(
	cellRepo memoryv2.MemoryCellRepository,
	sceneRepo memoryv2.MemorySceneRepository,
	output io.Writer,
) *MemoryCommand {
	return &MemoryCommand{
		cellRepo:  cellRepo,
		sceneRepo: sceneRepo,
		output:    output,
	}
}

// List displays memory cells matching the given filter.
func (c *MemoryCommand) List(ctx context.Context, filter memoryv2.MemoryCellFilter, format string) error {
	cells, err := c.cellRepo.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list memory cells: %w", err)
	}

	if len(cells) == 0 {
		c.writef("No memory cells found.\n")
		return nil
	}

	return c.renderCells(cells, format)
}

// Get displays a single memory cell by ID.
func (c *MemoryCommand) Get(ctx context.Context, id string, format string) error {
	cell, err := c.cellRepo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get memory cell: %w", err)
	}

	return c.renderCell(cell, format)
}

// Search performs full-text search across memory cells.
func (c *MemoryCommand) Search(ctx context.Context, query string, limit int, format string) error {
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	if limit <= 0 {
		limit = 20
	}

	cells, err := c.cellRepo.SearchFTS(ctx, query, limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(cells) == 0 {
		c.writef("No results found for query: %q\n", query)
		return nil
	}

	c.writef("Search results for %q (%d matches):\n\n", query, len(cells))
	return c.renderCells(cells, format)
}

// Delete removes a memory cell by ID.
func (c *MemoryCommand) Delete(ctx context.Context, id string) error {
	if err := c.cellRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete memory cell: %w", err)
	}

	c.writef("Memory cell %s deleted successfully.\n", id)
	return nil
}

// Scenes lists all memory scenes.
func (c *MemoryCommand) Scenes(ctx context.Context, format string) error {
	scenes, err := c.sceneRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list scenes: %w", err)
	}

	if len(scenes) == 0 {
		c.writef("No scenes found.\n")
		return nil
	}

	return c.renderScenes(scenes, format)
}

// Prune deletes all expired memory cells.
func (c *MemoryCommand) Prune(ctx context.Context) error {
	count, err := c.cellRepo.DeleteExpired(ctx)
	if err != nil {
		return fmt.Errorf("prune failed: %w", err)
	}

	if count == 0 {
		c.writef("No expired cells found.\n")
		return nil
	}

	c.writef("Pruned %d expired memory cell(s).\n", count)
	return nil
}

// writef writes formatted output, ignoring write errors (CLI output).
func (c *MemoryCommand) writef(format string, args ...interface{}) {
	fmt.Fprintf(c.output, format, args...) //nolint:errcheck // CLI output
}

// --- Rendering helpers ---

func (c *MemoryCommand) renderCells(cells []*memoryv2.MemoryCell, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return c.renderCellsJSON(cells)
	default:
		return c.renderCellsTable(cells)
	}
}

func (c *MemoryCommand) renderCell(cell *memoryv2.MemoryCell, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return c.renderCellJSON(cell)
	default:
		return c.renderCellDetail(cell)
	}
}

func (c *MemoryCommand) renderScenes(scenes []*memoryv2.MemoryScene, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return c.renderScenesJSON(scenes)
	default:
		return c.renderScenesTable(scenes)
	}
}

type cellView struct {
	ID             string  `json:"id"`
	Scene          string  `json:"scene"`
	Type           string  `json:"type"`
	Salience       float64 `json:"salience"`
	Content        string  `json:"content"`
	ConversationID string  `json:"conversation_id"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	ExpiresAt      string  `json:"expires_at,omitempty"`
}

func toCellView(cell *memoryv2.MemoryCell) cellView {
	v := cellView{
		ID:             cell.ID,
		Scene:          cell.Scene,
		Type:           cell.CellType.String(),
		Salience:       cell.Salience,
		Content:        cell.Content,
		ConversationID: cell.ConversationID,
		CreatedAt:      cell.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      cell.UpdatedAt.Format(time.RFC3339),
	}
	if cell.ExpiresAt != nil {
		v.ExpiresAt = cell.ExpiresAt.Format(time.RFC3339)
	}
	return v
}

func (c *MemoryCommand) renderCellsJSON(cells []*memoryv2.MemoryCell) error {
	views := make([]cellView, len(cells))
	for i, cell := range cells {
		views[i] = toCellView(cell)
	}
	return json.NewEncoder(c.output).Encode(views)
}

func (c *MemoryCommand) renderCellJSON(cell *memoryv2.MemoryCell) error {
	return json.NewEncoder(c.output).Encode(toCellView(cell))
}

func (c *MemoryCommand) renderCellsTable(cells []*memoryv2.MemoryCell) error {
	c.writef("%-8s  %-20s  %-12s  %-8s  %s\n", "ID", "SCENE", "TYPE", "SALIENCE", "CONTENT")
	c.writef("%s\n", strings.Repeat("-", 80))

	for _, cell := range cells {
		shortID := cell.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		content := cell.Content
		if len(content) > 40 {
			content = content[:37] + "..."
		}
		c.writef("%-8s  %-20s  %-12s  %-8.2f  %s\n",
			shortID, cell.Scene, cell.CellType.String(), cell.Salience, content)
	}

	c.writef("\n%d cell(s) found.\n", len(cells))
	return nil
}

func (c *MemoryCommand) renderCellDetail(cell *memoryv2.MemoryCell) error {
	c.writef("ID:              %s\n", cell.ID)
	c.writef("Scene:           %s\n", cell.Scene)
	c.writef("Type:            %s\n", cell.CellType.String())
	c.writef("Salience:        %.2f\n", cell.Salience)
	c.writef("Content:         %s\n", cell.Content)
	c.writef("Conversation ID: %s\n", cell.ConversationID)
	c.writef("Source:          %s\n", cell.Source)
	c.writef("Created At:      %s\n", cell.CreatedAt.Format(time.RFC3339))
	c.writef("Updated At:      %s\n", cell.UpdatedAt.Format(time.RFC3339))
	if cell.ExpiresAt != nil {
		c.writef("Expires At:      %s\n", cell.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}

type sceneView struct {
	Scene      string `json:"scene"`
	Summary    string `json:"summary"`
	TokenCount int    `json:"token_count"`
	UpdatedAt  string `json:"updated_at"`
}

func (c *MemoryCommand) renderScenesJSON(scenes []*memoryv2.MemoryScene) error {
	views := make([]sceneView, len(scenes))
	for i, s := range scenes {
		views[i] = sceneView{
			Scene:      s.Scene,
			Summary:    s.Summary,
			TokenCount: s.TokenCount,
			UpdatedAt:  s.UpdatedAt.Format(time.RFC3339),
		}
	}
	return json.NewEncoder(c.output).Encode(views)
}

func (c *MemoryCommand) renderScenesTable(scenes []*memoryv2.MemoryScene) error {
	c.writef("%-25s  %-8s  %-20s  %s\n", "SCENE", "TOKENS", "UPDATED", "SUMMARY")
	c.writef("%s\n", strings.Repeat("-", 90))

	for _, s := range scenes {
		summary := s.Summary
		if len(summary) > 40 {
			summary = summary[:37] + "..."
		}
		c.writef("%-25s  %-8d  %-20s  %s\n",
			s.Scene, s.TokenCount, s.UpdatedAt.Format("2006-01-02 15:04:05"), summary)
	}

	c.writef("\n%d scene(s) found.\n", len(scenes))
	return nil
}
