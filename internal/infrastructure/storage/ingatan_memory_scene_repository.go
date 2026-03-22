package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"nuimanbot/internal/domain/memoryv2"
)

// IngatanMemorySceneRepository implements memoryv2.MemorySceneRepository using the Ingatan REST API.
// Scene memories are stored with tags ["_scene", scene_name] and metadata containing the scene name,
// token count, and a nuiman_scene_id for stable identity.
//
// Note on race condition (L-2 from implementation-notes): Under concurrent Upsert calls for the
// same scene, two agents may both decide the scene doesn't exist and create duplicates. At Get/List
// time the most recent entry is used. A proper fix requires Ingatan conditional writes.
type IngatanMemorySceneRepository struct {
	client      *IngatanHTTPClient
	storePrefix string
}

// NewIngatanMemorySceneRepository creates a new IngatanMemorySceneRepository.
func NewIngatanMemorySceneRepository(client *IngatanHTTPClient, storePrefix string) *IngatanMemorySceneRepository {
	if storePrefix == "" {
		storePrefix = defaultStorePrefix
	}
	return &IngatanMemorySceneRepository{
		client:      client,
		storePrefix: storePrefix,
	}
}

// sceneStore returns the Ingatan store name used for all scenes.
func (r *IngatanMemorySceneRepository) sceneStore() string {
	return r.storePrefix + "_scenes"
}

// Upsert creates a scene if it doesn't exist, or updates it if it does.
// It searches by tags=["_scene", scene_name] to find an existing scene memory.
func (r *IngatanMemorySceneRepository) Upsert(ctx context.Context, scene *memoryv2.MemoryScene) error {
	store := r.sceneStore()
	existingID, err := r.findSceneIngatanID(ctx, store, scene.Scene)
	if err != nil {
		return fmt.Errorf("ingatan: upsert scene: find: %w", err)
	}

	payload, err := json.Marshal(sceneSaveRequest(scene))
	if err != nil {
		return fmt.Errorf("ingatan: upsert scene: marshal: %w", err)
	}

	if existingID == "" {
		// Create new scene memory.
		resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("ingatan: upsert scene: create: %w", err)
		}
		defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("ingatan: upsert scene: create: unexpected status %d", resp.StatusCode)
		}
		return nil
	}

	// Update existing scene memory.
	resp, err := r.client.Do(ctx, http.MethodPut, "/api/v1/stores/"+store+"/memories/"+existingID, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ingatan: upsert scene: update: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("ingatan: upsert scene: update: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// Get retrieves a scene by name.
// Returns ErrNotFound if no scene with that name exists.
func (r *IngatanMemorySceneRepository) Get(ctx context.Context, sceneName string) (*memoryv2.MemoryScene, error) {
	store := r.sceneStore()
	results, err := r.searchSceneByName(ctx, store, sceneName)
	if err != nil {
		return nil, fmt.Errorf("ingatan: get scene: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("ingatan: get scene: %w", memoryv2.ErrNotFound)
	}
	return results[0], nil
}

// List retrieves all scenes.
func (r *IngatanMemorySceneRepository) List(ctx context.Context) ([]*memoryv2.MemoryScene, error) {
	store := r.sceneStore()
	payload, err := json.Marshal(ingatanSearchRequest{
		Query: "_scene",
		Mode:  "hybrid",
		TopK:  1000,
		Tags:  []string{"_scene"},
	})
	if err != nil {
		return nil, fmt.Errorf("ingatan: list scenes: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories/search", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ingatan: list scenes: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return []*memoryv2.MemoryScene{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingatan: list scenes: unexpected status %d", resp.StatusCode)
	}

	var searchResp ingatanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("ingatan: list scenes: decode: %w", err)
	}

	scenes := make([]*memoryv2.MemoryScene, 0, len(searchResp.Results))
	for i := range searchResp.Results {
		mem := searchResp.Results[i].Memory
		if !isSceneMemory(mem) {
			continue
		}
		scene, err := memoryToScene(mem)
		if err != nil {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

// Delete removes a scene by name.
// Returns ErrNotFound if no scene with that name exists.
func (r *IngatanMemorySceneRepository) Delete(ctx context.Context, sceneName string) error {
	store := r.sceneStore()
	ingatanID, err := r.findSceneIngatanID(ctx, store, sceneName)
	if err != nil {
		return fmt.Errorf("ingatan: delete scene: find: %w", err)
	}
	if ingatanID == "" {
		return fmt.Errorf("ingatan: delete scene: %w", memoryv2.ErrNotFound)
	}

	resp, err := r.client.Do(ctx, http.MethodDelete, "/api/v1/stores/"+store+"/memories/"+ingatanID, nil)
	if err != nil {
		return fmt.Errorf("ingatan: delete scene: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("ingatan: delete scene: %w", memoryv2.ErrNotFound)
	default:
		return fmt.Errorf("ingatan: delete scene: unexpected status %d", resp.StatusCode)
	}
}

// findSceneIngatanID returns Ingatan's internal UUID for a scene, or "" if not found.
func (r *IngatanMemorySceneRepository) findSceneIngatanID(ctx context.Context, store, sceneName string) (string, error) {
	results, err := r.searchSceneByName(ctx, store, sceneName)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	// Return the Ingatan ID stored in nuiman_scene_id (which is the scene name),
	// but we need the Ingatan UUID. Search again to get the raw memory.
	memories, err := r.searchRawSceneByName(ctx, store, sceneName)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "", nil
	}
	return memories[0].ID, nil
}

// searchSceneByName returns MemoryScene objects matching the given scene name.
func (r *IngatanMemorySceneRepository) searchSceneByName(ctx context.Context, store, sceneName string) ([]*memoryv2.MemoryScene, error) {
	memories, err := r.searchRawSceneByName(ctx, store, sceneName)
	if err != nil {
		return nil, err
	}
	scenes := make([]*memoryv2.MemoryScene, 0, len(memories))
	for i := range memories {
		scene, err := memoryToScene(memories[i])
		if err != nil {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

// searchRawSceneByName returns raw Ingatan memory records for a scene name.
func (r *IngatanMemorySceneRepository) searchRawSceneByName(ctx context.Context, store, sceneName string) ([]ingatanMemory, error) {
	payload, err := json.Marshal(ingatanSearchRequest{
		Query: sceneName,
		Mode:  "hybrid",
		TopK:  10,
		Tags:  []string{"_scene", sceneName},
	})
	if err != nil {
		return nil, fmt.Errorf("search raw scene: marshal: %w", err)
	}

	resp, err := r.client.Do(ctx, http.MethodPost, "/api/v1/stores/"+store+"/memories/search", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("search raw scene: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close is idiomatic Go

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search raw scene: unexpected status %d", resp.StatusCode)
	}

	var searchResp ingatanSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("search raw scene: decode: %w", err)
	}

	var memories []ingatanMemory
	for _, result := range searchResp.Results {
		if name, ok := result.Memory.Metadata[metaKeySceneName].(string); ok && name == sceneName {
			memories = append(memories, result.Memory)
		}
	}
	return memories, nil
}

// isSceneMemory returns true if the memory has the _scene tag.
func isSceneMemory(mem ingatanMemory) bool {
	for _, tag := range mem.Tags {
		if tag == "_scene" {
			return true
		}
	}
	return false
}
