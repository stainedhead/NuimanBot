package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// MemoryAdmin defines admin-specific operations for memory management.
// Implemented by infrastructure layer (e.g., SQLiteMemoryAdmin).
type MemoryAdmin interface {
	// Stats returns overall memory statistics.
	Stats(ctx context.Context) (*MemoryStats, error)
	// CountCellsByConversation counts cells for a specific conversation.
	CountCellsByConversation(ctx context.Context, conversationID string) (int, error)
	// DeleteCellsByConversation removes all cells for a conversation.
	DeleteCellsByConversation(ctx context.Context, conversationID string) (int, error)
	// RebuildFTSIndex rebuilds the full-text search index.
	RebuildFTSIndex(ctx context.Context) error
}

// MemoryStats holds memory system statistics.
type MemoryStats struct {
	CellCount   int
	SceneCount  int
	DBSizeBytes int64
}

// SetAdmin sets the admin repository for admin operations (optional).
func (c *MemoryCommand) SetAdmin(admin MemoryAdmin) {
	c.admin = admin
}

// Stats displays memory system statistics.
func (c *MemoryCommand) Stats(ctx context.Context) error {
	if c.admin == nil {
		return fmt.Errorf("admin operations not available")
	}

	stats, err := c.admin.Stats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	c.writef("Memory Statistics\n")
	c.writef("%s\n", strings.Repeat("-", 30))
	c.writef("Cells:         %d\n", stats.CellCount)
	c.writef("Scenes:        %d\n", stats.SceneCount)
	c.writef("Database Size: %s\n", formatBytes(stats.DBSizeBytes))
	return nil
}

// ClearUser deletes all memories for a conversation/user.
// If confirmed is false, displays what would be deleted without actually deleting.
func (c *MemoryCommand) ClearUser(ctx context.Context, conversationID string, confirmed bool) error {
	if c.admin == nil {
		return fmt.Errorf("admin operations not available")
	}
	if conversationID == "" {
		return fmt.Errorf("conversation ID cannot be empty")
	}

	if !confirmed {
		count, err := c.admin.CountCellsByConversation(ctx, conversationID)
		if err != nil {
			return fmt.Errorf("failed to count cells: %w", err)
		}
		if count == 0 {
			c.writef("No memory cells found for conversation %q.\n", conversationID)
			return nil
		}
		c.writef("Would delete %d memory cell(s) for conversation %q.\n", count, conversationID)
		c.writef("Use --confirm to proceed.\n")
		return nil
	}

	count, err := c.admin.DeleteCellsByConversation(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("failed to clear user data: %w", err)
	}

	if count == 0 {
		c.writef("No memory cells found for conversation %q.\n", conversationID)
		return nil
	}

	c.writef("Deleted %d memory cell(s) for conversation %q.\n", count, conversationID)
	return nil
}

// Export exports memories for a conversation to JSON written to the command's output.
func (c *MemoryCommand) Export(ctx context.Context, conversationID string) error {
	if conversationID == "" {
		return fmt.Errorf("conversation ID cannot be empty")
	}

	cells, err := c.cellRepo.List(ctx, memoryv2.MemoryCellFilter{
		ConversationID: conversationID,
		IncludeExpired: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list cells for export: %w", err)
	}

	if len(cells) == 0 {
		c.writef("No memory cells found for conversation %q.\n", conversationID)
		return nil
	}

	// Collect unique scene names from cells
	sceneNames := make(map[string]bool)
	for _, cell := range cells {
		sceneNames[cell.Scene] = true
	}

	// Fetch scene data (graceful on missing scenes)
	var scenes []*memoryv2.MemoryScene
	for name := range sceneNames {
		scene, getErr := c.sceneRepo.Get(ctx, name)
		if getErr != nil {
			continue
		}
		scenes = append(scenes, scene)
	}

	// Build export data
	data := memoryExportData{
		Version:        1,
		ConversationID: conversationID,
		ExportedAt:     time.Now().Format(time.RFC3339),
		CellCount:      len(cells),
		SceneCount:     len(scenes),
		Cells:          make([]exportCell, len(cells)),
		Scenes:         make([]exportScene, len(scenes)),
	}

	for i, cell := range cells {
		data.Cells[i] = toExportCell(cell)
	}
	for i, scene := range scenes {
		data.Scenes[i] = toExportScene(scene)
	}

	enc := json.NewEncoder(c.output)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// Import imports memories from JSON read from the given reader.
func (c *MemoryCommand) Import(ctx context.Context, reader io.Reader) error {
	var data memoryExportData
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return fmt.Errorf("invalid import data: %w", err)
	}

	if data.Version != 1 {
		return fmt.Errorf("unsupported export version: %d", data.Version)
	}

	// Import scenes first (upsert - safe to re-import)
	scenesImported := 0
	for _, es := range data.Scenes {
		scene, err := fromExportScene(es)
		if err != nil {
			c.writef("Warning: skipping invalid scene %q: %v\n", es.Scene, err)
			continue
		}
		if err := c.sceneRepo.Upsert(ctx, scene); err != nil {
			c.writef("Warning: failed to import scene %q: %v\n", es.Scene, err)
			continue
		}
		scenesImported++
	}

	// Import cells (skip duplicates)
	cellsImported := 0
	cellsSkipped := 0
	for _, ec := range data.Cells {
		cell, err := fromExportCell(ec)
		if err != nil {
			c.writef("Warning: skipping invalid cell %q: %v\n", ec.ID, err)
			cellsSkipped++
			continue
		}
		if err := c.cellRepo.Create(ctx, cell); err != nil {
			if errors.Is(err, memoryv2.ErrAlreadyExists) {
				cellsSkipped++
				continue
			}
			c.writef("Warning: failed to import cell %q: %v\n", ec.ID, err)
			cellsSkipped++
			continue
		}
		cellsImported++
	}

	c.writef("Imported %d cell(s) and %d scene(s).\n", cellsImported, scenesImported)
	if cellsSkipped > 0 {
		c.writef("Skipped %d cell(s) (duplicates or errors).\n", cellsSkipped)
	}
	return nil
}

// RebuildFTS rebuilds the FTS5 full-text search index.
func (c *MemoryCommand) RebuildFTS(ctx context.Context) error {
	if c.admin == nil {
		return fmt.Errorf("admin operations not available")
	}

	if err := c.admin.RebuildFTSIndex(ctx); err != nil {
		return fmt.Errorf("failed to rebuild FTS index: %w", err)
	}

	c.writef("FTS index rebuilt successfully.\n")
	return nil
}

// --- Export/Import data types ---

type memoryExportData struct {
	Version        int           `json:"version"`
	ConversationID string        `json:"conversation_id"`
	ExportedAt     string        `json:"exported_at"`
	CellCount      int           `json:"cell_count"`
	SceneCount     int           `json:"scene_count"`
	Cells          []exportCell  `json:"cells"`
	Scenes         []exportScene `json:"scenes"`
}

type exportCell struct {
	ID             string  `json:"id"`
	ConversationID string  `json:"conversation_id"`
	Scene          string  `json:"scene"`
	Type           string  `json:"type"`
	Salience       float64 `json:"salience"`
	Content        string  `json:"content"`
	Source         string  `json:"source"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	ExpiresAt      string  `json:"expires_at,omitempty"`
}

type exportScene struct {
	Scene      string `json:"scene"`
	Summary    string `json:"summary"`
	TokenCount int    `json:"token_count"`
	UpdatedAt  string `json:"updated_at"`
}

// --- Conversion helpers ---

func toExportCell(cell *memoryv2.MemoryCell) exportCell {
	ec := exportCell{
		ID:             cell.ID,
		ConversationID: cell.ConversationID,
		Scene:          cell.Scene,
		Type:           cell.CellType.String(),
		Salience:       cell.Salience,
		Content:        cell.Content,
		Source:         cell.Source,
		CreatedAt:      cell.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      cell.UpdatedAt.Format(time.RFC3339),
	}
	if cell.ExpiresAt != nil {
		ec.ExpiresAt = cell.ExpiresAt.Format(time.RFC3339)
	}
	return ec
}

func fromExportCell(ec exportCell) (*memoryv2.MemoryCell, error) {
	cellType, err := memoryv2.ParseCellType(ec.Type)
	if err != nil {
		return nil, err
	}

	createdAt, err := time.Parse(time.RFC3339, ec.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at: %w", err)
	}

	updatedAt, err := time.Parse(time.RFC3339, ec.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at: %w", err)
	}

	cell := &memoryv2.MemoryCell{
		ID:             ec.ID,
		ConversationID: ec.ConversationID,
		Scene:          ec.Scene,
		CellType:       cellType,
		Salience:       ec.Salience,
		Content:        ec.Content,
		Source:         ec.Source,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	if ec.ExpiresAt != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, ec.ExpiresAt)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid expires_at: %w", parseErr)
		}
		cell.ExpiresAt = &expiresAt
	}

	return cell, nil
}

func toExportScene(scene *memoryv2.MemoryScene) exportScene {
	return exportScene{
		Scene:      scene.Scene,
		Summary:    scene.Summary,
		TokenCount: scene.TokenCount,
		UpdatedAt:  scene.UpdatedAt.Format(time.RFC3339),
	}
}

func fromExportScene(es exportScene) (*memoryv2.MemoryScene, error) {
	updatedAt, err := time.Parse(time.RFC3339, es.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at: %w", err)
	}

	return &memoryv2.MemoryScene{
		Scene:      es.Scene,
		Summary:    es.Summary,
		TokenCount: es.TokenCount,
		UpdatedAt:  updatedAt,
	}, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
