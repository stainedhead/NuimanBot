package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// Metadata key constants used when mapping MemoryCell fields to Ingatan metadata.
const (
	metaKeyScene        = "scene"
	metaKeyCellType     = "cell_type"
	metaKeySalience     = "salience"
	metaKeySourceMsgIDs = "source_message_ids"
	metaKeyCellID       = "nuiman_cell_id"
	metaKeyExpiresAt    = "expires_at"
)

// Ingatan scene metadata key constants.
const (
	metaKeySceneName   = "scene_name"
	metaKeyTokenCount  = "token_count"
	metaKeyNuimanScene = "nuiman_scene_id"
)

// ingatanSaveRequest is the body sent to POST /api/v1/stores/{store}/memories.
type ingatanSaveRequest struct {
	Title     string                 `json:"title,omitempty"`
	Content   string                 `json:"content"`
	Tags      []string               `json:"tags,omitempty"`
	Source    string                 `json:"source"`
	SourceRef string                 `json:"source_ref,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ingatanMemory is the memory record returned by the Ingatan REST API.
type ingatanMemory struct {
	ID        string                 `json:"id"`
	Store     string                 `json:"store"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Tags      []string               `json:"tags"`
	Source    string                 `json:"source"`
	SourceRef string                 `json:"source_ref"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// ingatanSearchRequest is the body sent to POST /api/v1/stores/{store}/memories/search.
type ingatanSearchRequest struct {
	Query string   `json:"query"`
	Mode  string   `json:"mode"`
	TopK  int      `json:"top_k,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// ingatanSearchResult is a single result entry from a search response.
type ingatanSearchResult struct {
	Memory ingatanMemory `json:"memory"`
	Score  float64       `json:"score"`
}

// ingatanSearchResponse is the top-level search response body.
type ingatanSearchResponse struct {
	Results []ingatanSearchResult `json:"results"`
}

// ingatanListResponse is the top-level list response body.
type ingatanListResponse struct {
	Memories []ingatanMemory `json:"memories"`
}

// cellToSaveRequest converts a MemoryCell to an Ingatan save request.
func cellToSaveRequest(cell *memoryv2.MemoryCell) ingatanSaveRequest {
	meta := map[string]interface{}{
		metaKeyScene:        cell.Scene,
		metaKeyCellType:     cell.CellType.String(),
		metaKeySalience:     cell.Salience,
		metaKeySourceMsgIDs: cell.Source,
		metaKeyCellID:       cell.ID,
	}
	if cell.ExpiresAt != nil {
		meta[metaKeyExpiresAt] = cell.ExpiresAt.UTC().Format(time.RFC3339)
	}

	return ingatanSaveRequest{
		Content:   cell.Content,
		Tags:      []string{cell.Scene},
		Source:    "conversation",
		SourceRef: cell.ConversationID,
		Metadata:  meta,
	}
}

// memoryToCell converts an Ingatan memory record to a MemoryCell.
func memoryToCell(m ingatanMemory) (*memoryv2.MemoryCell, error) {
	cell := &memoryv2.MemoryCell{
		Content:        m.Content,
		ConversationID: m.SourceRef,
	}

	// Parse metadata fields.
	if scene, ok := m.Metadata[metaKeyScene].(string); ok {
		cell.Scene = scene
	}
	if cellTypeStr, ok := m.Metadata[metaKeyCellType].(string); ok {
		ct, err := memoryv2.ParseCellType(cellTypeStr)
		if err != nil {
			return nil, fmt.Errorf("ingatan: mapping: parse cell_type %q: %w", cellTypeStr, err)
		}
		cell.CellType = ct
	}
	cell.Salience = metaFloat64(m.Metadata, metaKeySalience)
	if src, ok := m.Metadata[metaKeySourceMsgIDs].(string); ok {
		cell.Source = src
	}
	if id, ok := m.Metadata[metaKeyCellID].(string); ok {
		cell.ID = id
	}
	if expiresStr, ok := m.Metadata[metaKeyExpiresAt].(string); ok && expiresStr != "" {
		t, err := time.Parse(time.RFC3339, expiresStr)
		if err == nil {
			cell.ExpiresAt = &t
		}
	}

	// Parse timestamps.
	if m.CreatedAt != "" {
		ts, err := time.Parse(time.RFC3339, m.CreatedAt)
		if err == nil {
			cell.CreatedAt = ts
		}
	}
	if m.UpdatedAt != "" {
		ts, err := time.Parse(time.RFC3339, m.UpdatedAt)
		if err == nil {
			cell.UpdatedAt = ts
		}
	}

	return cell, nil
}

// sceneSaveRequest converts a MemoryScene to an Ingatan save request.
func sceneSaveRequest(scene *memoryv2.MemoryScene) ingatanSaveRequest {
	return ingatanSaveRequest{
		Content: scene.Summary,
		Tags:    []string{"_scene", scene.Scene},
		Source:  "manual",
		Metadata: map[string]interface{}{
			metaKeySceneName:   scene.Scene,
			metaKeyTokenCount:  scene.TokenCount,
			metaKeyNuimanScene: scene.Scene,
		},
	}
}

// memoryToScene converts an Ingatan memory record to a MemoryScene.
func memoryToScene(m ingatanMemory) (*memoryv2.MemoryScene, error) {
	scene := &memoryv2.MemoryScene{
		Summary: m.Content,
	}
	if name, ok := m.Metadata[metaKeySceneName].(string); ok {
		scene.Scene = name
	}
	scene.TokenCount = metaInt(m.Metadata, metaKeyTokenCount)
	if m.UpdatedAt != "" {
		ts, err := time.Parse(time.RFC3339, m.UpdatedAt)
		if err == nil {
			scene.UpdatedAt = ts
		}
	}
	return scene, nil
}

// metaFloat64 extracts a float64 from a metadata map, handling both float64 and string types.
func metaFloat64(meta map[string]interface{}, key string) float64 {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// metaInt extracts an int from a metadata map, handling float64, int, and string types.
func metaInt(meta map[string]interface{}, key string) int {
	v, ok := meta[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case json.Number:
		i, _ := val.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	default:
		return 0
	}
}
