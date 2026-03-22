package storage

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// sampleScene returns a valid MemoryScene for use in tests.
func sampleScene(t *testing.T) *memoryv2.MemoryScene {
	t.Helper()
	return &memoryv2.MemoryScene{
		Scene:      "test-scene",
		Summary:    "This is a test scene summary",
		TokenCount: 42,
		UpdatedAt:  time.Now().UTC().Truncate(time.Second),
	}
}

// ingatanSceneMemoryResponse builds a fake Ingatan memory response for a scene.
func ingatanSceneMemoryResponse(scene *memoryv2.MemoryScene) map[string]interface{} {
	return map[string]interface{}{
		"id":         "ingatan-scene-id-001",
		"store":      "test_scene_store",
		"content":    scene.Summary,
		"tags":       []string{"_scene", scene.Scene},
		"source":     "manual",
		"source_ref": "",
		"metadata": map[string]interface{}{
			metaKeySceneName:   scene.Scene,
			metaKeyTokenCount:  float64(scene.TokenCount),
			metaKeyNuimanScene: scene.Scene,
		},
		"created_at": scene.UpdatedAt.Format(time.RFC3339),
		"updated_at": scene.UpdatedAt.Format(time.RFC3339),
	}
}

// --- Upsert (new scene) ---

func TestIngatanMemorySceneRepository_Upsert_New(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	var capturedBody map[string]interface{}
	postCalled := false

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search"):
			// No existing scene found.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories"):
			postCalled = true
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &capturedBody)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ingatanSceneMemoryResponse(scene))
		default:
			http.NotFound(w, r)
		}
	})

	if err := repo.Upsert(context.Background(), scene); err != nil {
		t.Fatalf("Upsert (new) failed: %v", err)
	}
	if !postCalled {
		t.Error("Expected POST to /memories for new scene, got none")
	}
	if capturedBody["content"] != scene.Summary {
		t.Errorf("Expected body content %q, got %v", scene.Summary, capturedBody["content"])
	}
	// Verify tags include _scene and scene name.
	tags, _ := capturedBody["tags"].([]interface{})
	hasSceneTag := false
	hasNameTag := false
	for _, tag := range tags {
		if tag == "_scene" {
			hasSceneTag = true
		}
		if tag == scene.Scene {
			hasNameTag = true
		}
	}
	if !hasSceneTag {
		t.Error("Expected '_scene' tag in POST body")
	}
	if !hasNameTag {
		t.Errorf("Expected scene name tag %q in POST body", scene.Scene)
	}
}

// --- Upsert (existing scene) ---

func TestIngatanMemorySceneRepository_Upsert_Existing(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	putCalled := false
	var putPath string

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search"):
			// Return existing scene.
			result := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  0.99,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(result)
		case r.Method == http.MethodPut:
			putCalled = true
			putPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ingatanSceneMemoryResponse(scene))
		default:
			http.NotFound(w, r)
		}
	})

	if err := repo.Upsert(context.Background(), scene); err != nil {
		t.Fatalf("Upsert (existing) failed: %v", err)
	}
	if !putCalled {
		t.Error("Expected PUT for existing scene, got none")
	}
	if !strings.Contains(putPath, "ingatan-scene-id-001") {
		t.Errorf("Expected PUT to contain Ingatan ID, got %q", putPath)
	}
}

// --- Get ---

func TestIngatanMemorySceneRepository_Get_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			result := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  0.99,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		http.NotFound(w, r)
	})

	got, err := repo.Get(context.Background(), scene.Scene)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Scene != scene.Scene {
		t.Errorf("Expected Scene %q, got %q", scene.Scene, got.Scene)
	}
	if got.Summary != scene.Summary {
		t.Errorf("Expected Summary %q, got %q", scene.Summary, got.Summary)
	}
	if got.TokenCount != scene.TokenCount {
		t.Errorf("Expected TokenCount %d, got %d", scene.TokenCount, got.TokenCount)
	}
}

func TestIngatanMemorySceneRepository_Get_NotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
			return
		}
		http.NotFound(w, r)
	})

	_, err := repo.Get(context.Background(), "nonexistent-scene")
	if err == nil {
		t.Fatal("Expected ErrNotFound, got nil")
	}
	if !strings.Contains(err.Error(), memoryv2.ErrNotFound.Error()) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

// --- List ---

func TestIngatanMemorySceneRepository_List(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			result := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  0.99,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		http.NotFound(w, r)
	})

	scenes, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(scenes) != 1 {
		t.Fatalf("Expected 1 scene, got %d", len(scenes))
	}
	if scenes[0].Scene != scene.Scene {
		t.Errorf("Expected Scene %q, got %q", scene.Scene, scenes[0].Scene)
	}
}

// --- Delete ---

func TestIngatanMemorySceneRepository_Delete_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	scene := sampleScene(t)

	deleteCalled := false

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search"):
			result := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanSceneMemoryResponse(scene),
						"score":  0.99,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(result)
		case r.Method == http.MethodDelete:
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})

	if err := repo.Delete(context.Background(), scene.Scene); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleteCalled {
		t.Error("Expected DELETE call, got none")
	}
}

func TestIngatanMemorySceneRepository_Delete_NotFound(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
			return
		}
		http.NotFound(w, r)
	})

	err := repo.Delete(context.Background(), "nonexistent-scene")
	if err == nil {
		t.Fatal("Expected ErrNotFound, got nil")
	}
	if !strings.Contains(err.Error(), memoryv2.ErrNotFound.Error()) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

// makeNSceneResults returns a search response containing n scene results.
func makeNSceneResults(n int) map[string]interface{} {
	results := make([]interface{}, n)
	for i := range results {
		results[i] = map[string]interface{}{
			"memory": map[string]interface{}{
				"id":         "ingatan-scene-id-000",
				"store":      "test_scenes",
				"content":    "Scene summary",
				"tags":       []string{"_scene", "scene-name"},
				"source":     "manual",
				"source_ref": "",
				"metadata": map[string]interface{}{
					metaKeySceneName:   "scene-name",
					metaKeyTokenCount:  float64(10),
					metaKeyNuimanScene: "scene-name",
				},
				"created_at": time.Now().UTC().Format(time.RFC3339),
				"updated_at": time.Now().UTC().Format(time.RFC3339),
			},
			"score": 0.99,
		}
	}
	return map[string]interface{}{"results": results}
}

// captureSceneListLogs replaces the default slog handler with a capturing handler for the
// duration of the test and returns the handler so callers can inspect logged records.
func captureSceneListLogs(t *testing.T) *testLogHandler {
	t.Helper()
	handler := &testLogHandler{}
	original := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(original) })
	return handler
}

// TestIngatanMemorySceneRepository_List_TruncationWarning_ExactLimit verifies that
// List() emits a slog.Warn containing "truncated" or "limit" when the number of returned
// scenes equals maxSceneListLimit (R-11).
func TestIngatanMemorySceneRepository_List_TruncationWarning_ExactLimit(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	handler := captureSceneListLogs(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(makeNSceneResults(maxSceneListLimit))
			return
		}
		http.NotFound(w, r)
	})

	scenes, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(scenes) != maxSceneListLimit {
		t.Fatalf("Expected %d scenes, got %d", maxSceneListLimit, len(scenes))
	}

	found := false
	for _, rec := range handler.records {
		if rec.Level == slog.LevelWarn &&
			(strings.Contains(rec.Message, "truncated") || strings.Contains(rec.Message, "limit")) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected List() to emit a slog.Warn containing 'truncated' or 'limit' when result count equals maxSceneListLimit, but none was found")
	}
}

// TestIngatanMemorySceneRepository_List_NoWarning_BelowLimit verifies that
// List() does NOT emit a truncation warning when the result count is below maxSceneListLimit (R-11).
func TestIngatanMemorySceneRepository_List_NoWarning_BelowLimit(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	handler := captureSceneListLogs(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(makeNSceneResults(500))
			return
		}
		http.NotFound(w, r)
	})

	if _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, rec := range handler.records {
		if rec.Level == slog.LevelWarn &&
			(strings.Contains(rec.Message, "truncated") || strings.Contains(rec.Message, "limit")) {
			t.Errorf("Expected no truncation warning for 500 scenes, but got: %s", rec.Message)
		}
	}
}

// TestIngatanMemorySceneRepository_List_NoWarning_Empty verifies that
// List() does NOT emit a truncation warning when the result count is 0 (R-11).
func TestIngatanMemorySceneRepository_List_NoWarning_Empty(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemorySceneRepository(client, "test")
	handler := captureSceneListLogs(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories/search") {
			_ = json.NewEncoder(w).Encode(makeNSceneResults(0))
			return
		}
		http.NotFound(w, r)
	})

	if _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, rec := range handler.records {
		if rec.Level == slog.LevelWarn &&
			(strings.Contains(rec.Message, "truncated") || strings.Contains(rec.Message, "limit")) {
			t.Errorf("Expected no truncation warning for 0 scenes, but got: %s", rec.Message)
		}
	}
}
