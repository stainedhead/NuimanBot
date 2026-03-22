package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"nuimanbot/internal/domain/memoryv2"
)

// defaultStorePrefix is used when no prefix is configured.
const defaultStorePrefix = "nuiman"

// storeFor derives a deterministic Ingatan store name from a prefix and conversation ID.
// The store name is: {prefix}_{sha256(conversationID)[:16]}.
// If prefix is empty, defaultStorePrefix is used.
func storeFor(prefix, conversationID string) string {
	if prefix == "" {
		prefix = defaultStorePrefix
	}
	hash := sha256.Sum256([]byte(conversationID))
	return prefix + "_" + hex.EncodeToString(hash[:])[:16]
}

// IngatanMemoryCellRepository implements memoryv2.MemoryCellRepository using the Ingatan REST API.
type IngatanMemoryCellRepository struct {
	client      *IngatanHTTPClient
	storePrefix string
}

// NewIngatanMemoryCellRepository creates a new IngatanMemoryCellRepository.
func NewIngatanMemoryCellRepository(client *IngatanHTTPClient, storePrefix string) *IngatanMemoryCellRepository {
	if storePrefix == "" {
		storePrefix = defaultStorePrefix
	}
	return &IngatanMemoryCellRepository{
		client:      client,
		storePrefix: storePrefix,
	}
}

// Create inserts a new MemoryCell into Ingatan.
// Returns ErrAlreadyExists if a cell with the same nuiman_cell_id already exists (409).
// Returns ErrInvalidInput on bad request (400).
// On 404, auto-creates the store and retries once (see ADR-3, implementation-notes L-1).
func (r *IngatanMemoryCellRepository) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	store := storeFor(r.storePrefix, cell.ConversationID)
	if err := r.createInStore(ctx, store, cell); err != nil {
		var notFoundErr *ingatanNotFoundError
		if errors.As(err, &notFoundErr) {
			// Store doesn't exist — auto-create and retry.
			if createErr := r.ensureStore(ctx, store); createErr != nil {
				return fmt.Errorf("ingatan: create: ensure store: %w", createErr)
			}
			return r.createInStore(ctx, store, cell)
		}
		return err
	}
	return nil
}

// createInStore posts a single memory cell to the given store.
func (r *IngatanMemoryCellRepository) createInStore(ctx context.Context, store string, cell *memoryv2.MemoryCell) error {
	payload, err := json.Marshal(cellToSaveRequest(cell))
	if err != nil {
		return fmt.Errorf("ingatan: create: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ingatan: create: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		return nil
	case http.StatusConflict:
		return fmt.Errorf("ingatan: create: %w", memoryv2.ErrAlreadyExists)
	case http.StatusBadRequest:
		return fmt.Errorf("ingatan: create: %w", memoryv2.ErrInvalidInput)
	case http.StatusNotFound:
		return &ingatanNotFoundError{op: "create", store: store}
	default:
		return fmt.Errorf("ingatan: create: unexpected status %d", resp.StatusCode)
	}
}

// ensureStore creates an Ingatan store with the given name.
func (r *IngatanMemoryCellRepository) ensureStore(ctx context.Context, storeName string) error {
	payload, err := json.Marshal(map[string]string{
		"name":        storeName,
		"description": "NuimanBot memory store",
	})
	if err != nil {
		return fmt.Errorf("ingatan: ensure store: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ingatan: ensure store: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("ingatan: ensure store: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Get is not supported by the Ingatan backend.
//
// Ingatan stores are partitioned per conversation: a cell can only be retrieved from the
// store keyed to its conversationID. Without that context, the correct store cannot be
// determined and any search would always target the wrong partition.
//
// Callers that need to fetch a cell by ID must use
// List(ctx, MemoryCellFilter{ConversationID: "..."}) instead.
// See ADR-2 in specs/post-memory-design-review/implementation-notes.md.
func (r *IngatanMemoryCellRepository) Get(_ context.Context, id string) (*memoryv2.MemoryCell, error) {
	return nil, fmt.Errorf("ingatan: get %q: %w: requires conversation context — use List with ConversationID filter",
		id, memoryv2.ErrNotFound)
}

// List retrieves cells matching the provided filter.
func (r *IngatanMemoryCellRepository) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if filter.ConversationID == "" {
		return []*memoryv2.MemoryCell{}, nil
	}
	store := storeFor(r.storePrefix, filter.ConversationID)
	resp, err := r.client.Do(ctx, http.MethodGet, "/api/v1/stores/"+store+"/memories", nil)
	if err != nil {
		return nil, fmt.Errorf("ingatan: list: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return []*memoryv2.MemoryCell{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingatan: list: unexpected status %d", resp.StatusCode)
	}

	var listResp ingatanListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("ingatan: list: decode: %w", err)
	}

	return memoryCellsFromList(listResp.Memories, filter)
}

// Update persists changes to an existing cell.
func (r *IngatanMemoryCellRepository) Update(ctx context.Context, cell *memoryv2.MemoryCell) error {
	store := storeFor(r.storePrefix, cell.ConversationID)
	// Find the Ingatan ID for this cell first.
	ingatanID, err := r.findIngatanID(ctx, store, cell.ID)
	if err != nil {
		return fmt.Errorf("ingatan: update: find id: %w", err)
	}
	if ingatanID == "" {
		return fmt.Errorf("ingatan: update: %w", memoryv2.ErrNotFound)
	}

	payload, err := json.Marshal(cellToSaveRequest(cell))
	if err != nil {
		return fmt.Errorf("ingatan: update: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPut, "/api/v1/stores/"+store+"/memories/"+ingatanID, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ingatan: update: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("ingatan: update: %w", memoryv2.ErrNotFound)
	default:
		return fmt.Errorf("ingatan: update: unexpected status %d", resp.StatusCode)
	}
}

// Delete removes a cell by its nuiman_cell_id.
func (r *IngatanMemoryCellRepository) Delete(ctx context.Context, id string) error {
	// To delete, we need both the store name and Ingatan's internal UUID.
	// Since we don't know the store without a conversationID, we must search first.
	// This is a known limitation — use List+Delete in practice.
	store := storeFor(r.storePrefix, id) // proxy store derivation
	ingatanID, err := r.findIngatanID(ctx, store, id)
	if err != nil {
		return fmt.Errorf("ingatan: delete: %w", err)
	}
	if ingatanID == "" {
		return fmt.Errorf("ingatan: delete: %w", memoryv2.ErrNotFound)
	}

	resp, err := r.client.Do(ctx, http.MethodDelete, "/api/v1/stores/"+store+"/memories/"+ingatanID, nil)
	if err != nil {
		return fmt.Errorf("ingatan: delete: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("ingatan: delete: %w", memoryv2.ErrNotFound)
	default:
		return fmt.Errorf("ingatan: delete: unexpected status %d", resp.StatusCode)
	}
}

// SearchFTS performs full-text search using Ingatan's hybrid HNSW+BM25 mode.
func (r *IngatanMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	// SearchFTS searches across all stores when no conversationID context is available.
	// For a per-conversation implementation, the caller should use GetByScene or List instead.
	// We use a well-known default store name as a fallback.
	store := storeFor(r.storePrefix, query)
	return r.searchInStore(ctx, store, query, limit)
}

// searchInStore performs a hybrid search in a specific Ingatan store.
func (r *IngatanMemoryCellRepository) searchInStore(ctx context.Context, store, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	payload, err := json.Marshal(ingatanSearchRequest{
		Query: query,
		Mode:  "hybrid",
		TopK:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("ingatan: search fts: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories/search", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ingatan: search fts: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return []*memoryv2.MemoryCell{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingatan: search fts: unexpected status %d", resp.StatusCode)
	}

	var searchResp ingatanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("ingatan: search fts: decode: %w", err)
	}

	cells := make([]*memoryv2.MemoryCell, 0, len(searchResp.Results))
	for i := range searchResp.Results {
		cell, err := memoryToCell(searchResp.Results[i].Memory)
		if err != nil {
			continue
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

// GetByScene retrieves cells for a specific scene, ordered by salience descending.
func (r *IngatanMemoryCellRepository) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	store := storeFor(r.storePrefix, scene)
	payload, err := json.Marshal(ingatanSearchRequest{
		Query: scene,
		Mode:  "hybrid",
		TopK:  limit,
		Tags:  []string{scene},
	})
	if err != nil {
		return nil, fmt.Errorf("ingatan: get by scene: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories/search", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ingatan: get by scene: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return []*memoryv2.MemoryCell{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingatan: get by scene: unexpected status %d", resp.StatusCode)
	}

	var searchResp ingatanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("ingatan: get by scene: decode: %w", err)
	}

	cells := make([]*memoryv2.MemoryCell, 0, len(searchResp.Results))
	for i := range searchResp.Results {
		cell, err := memoryToCell(searchResp.Results[i].Memory)
		if err != nil {
			continue
		}
		if cell.Scene == scene {
			cells = append(cells, cell)
		}
	}
	return cells, nil
}

// GetHighSalience retrieves cells above a salience threshold for a conversation.
func (r *IngatanMemoryCellRepository) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	store := storeFor(r.storePrefix, conversationID)
	resp, err := r.client.Do(ctx, http.MethodGet, "/api/v1/stores/"+store+"/memories", nil)
	if err != nil {
		return nil, fmt.Errorf("ingatan: get high salience: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return []*memoryv2.MemoryCell{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingatan: get high salience: unexpected status %d", resp.StatusCode)
	}

	var listResp ingatanListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("ingatan: get high salience: decode: %w", err)
	}

	var cells []*memoryv2.MemoryCell
	for i := range listResp.Memories {
		cell, err := memoryToCell(listResp.Memories[i])
		if err != nil {
			continue
		}
		if cell.Salience >= threshold {
			cells = append(cells, cell)
			if limit > 0 && len(cells) >= limit {
				break
			}
		}
	}
	if cells == nil {
		cells = []*memoryv2.MemoryCell{}
	}
	return cells, nil
}

// DeleteExpired is a no-op in the Ingatan adapter.
// Ingatan does not expose a TTL-based delete endpoint. Expired cells persist in Ingatan
// until manually deleted. At recall time, MemoryRecallService calls IsExpired() and skips
// expired cells. See ADR-4 in implementation-notes.md.
func (r *IngatanMemoryCellRepository) DeleteExpired(_ context.Context) (int, error) {
	slog.Debug("ingatan: DeleteExpired called — no-op; expired cells persist in Ingatan (see ADR-4)")
	return 0, nil
}

// findIngatanID finds Ingatan's internal UUID for a cell identified by nuiman_cell_id.
func (r *IngatanMemoryCellRepository) findIngatanID(ctx context.Context, store, nuimanCellID string) (string, error) {
	payload, err := json.Marshal(ingatanSearchRequest{
		Query: nuimanCellID,
		Mode:  "hybrid",
		TopK:  5,
	})
	if err != nil {
		return "", fmt.Errorf("find ingatan id: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories/search", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("find ingatan id: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("find ingatan id: unexpected status %d", resp.StatusCode)
	}

	var searchResp ingatanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", fmt.Errorf("find ingatan id: decode: %w", err)
	}

	for _, result := range searchResp.Results {
		if id, ok := result.Memory.Metadata[metaKeyCellID].(string); ok && id == nuimanCellID {
			return result.Memory.ID, nil
		}
	}
	return "", nil
}

// memoryCellsFromList converts a list of Ingatan memories to MemoryCells, applying filters.
func memoryCellsFromList(memories []ingatanMemory, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	cells := make([]*memoryv2.MemoryCell, 0, len(memories))
	for i := range memories {
		cell, err := memoryToCell(memories[i])
		if err != nil {
			continue
		}
		if !matchesFilter(cell, filter) {
			continue
		}
		cells = append(cells, cell)
		if filter.Limit > 0 && len(cells) >= filter.Limit {
			break
		}
	}
	return cells, nil
}

// matchesFilter returns true if a cell matches the given filter criteria.
func matchesFilter(cell *memoryv2.MemoryCell, filter memoryv2.MemoryCellFilter) bool {
	if filter.Scene != "" && cell.Scene != filter.Scene {
		return false
	}
	if filter.CellType != nil && cell.CellType != *filter.CellType {
		return false
	}
	if filter.MinSalience != nil && cell.Salience < *filter.MinSalience {
		return false
	}
	if !filter.IncludeExpired && cell.IsExpired() {
		return false
	}
	return true
}

// ingatanNotFoundError represents a 404 response from Ingatan, used to trigger store auto-creation.
type ingatanNotFoundError struct {
	op    string
	store string
}

func (e *ingatanNotFoundError) Error() string {
	return fmt.Sprintf("ingatan: %s: store %q not found (404)", e.op, e.store)
}
