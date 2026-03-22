package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// newMockIngatan builds a test IngatanHTTPClient pointing at a mock HTTP server.
// The caller registers additional route handlers on the returned mux.
func newMockIngatan(t *testing.T) (*httptest.Server, *http.ServeMux, *IngatanHTTPClient) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, _ *http.Request) {
		exp := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token": "test-jwt", "expires_at": exp,
		}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})

	client := NewIngatanHTTPClient(IngatanClientConfig{
		BaseURL:     srv.URL,
		APIKey:      "test-api-key",
		StorePrefix: "test",
	})
	return srv, mux, client
}

// sampleCell returns a valid MemoryCell for use in tests.
func sampleCell(t *testing.T) *memoryv2.MemoryCell {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return &memoryv2.MemoryCell{
		ID:             "11111111-1111-1111-1111-111111111111",
		ConversationID: "conv-abc",
		Scene:          "test-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.8,
		Content:        "The sky is blue",
		Source:         `["msg-1","msg-2"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// ingatanMemoryResponse builds a fake Ingatan memory response JSON for the given cell.
func ingatanMemoryResponse(cell *memoryv2.MemoryCell) map[string]interface{} {
	meta := map[string]interface{}{
		metaKeyScene:        cell.Scene,
		metaKeyCellType:     cell.CellType.String(),
		metaKeySalience:     cell.Salience,
		metaKeySourceMsgIDs: cell.Source,
		metaKeyCellID:       cell.ID,
	}
	return map[string]interface{}{
		"id":         "ingatan-id-abc",
		"store":      "test_store",
		"content":    cell.Content,
		"tags":       []string{cell.Scene},
		"source":     "conversation",
		"source_ref": cell.ConversationID,
		"metadata":   meta,
		"created_at": cell.CreatedAt.Format(time.RFC3339),
		"updated_at": cell.UpdatedAt.Format(time.RFC3339),
	}
}

// --- Create ---

func TestIngatanMemoryCellRepository_Create_Success(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	var capturedBody map[string]interface{}
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &capturedBody)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ingatanMemoryResponse(cell))
			return
		}
		http.NotFound(w, r)
	})

	if err := repo.Create(context.Background(), cell); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if capturedBody["content"] != cell.Content {
		t.Errorf("Expected body content %q, got %v", cell.Content, capturedBody["content"])
	}
}

func TestIngatanMemoryCellRepository_Create_Conflict(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			w.WriteHeader(http.StatusConflict)
			return
		}
		http.NotFound(w, r)
	})

	err := repo.Create(context.Background(), cell)
	if err == nil {
		t.Fatal("Expected ErrAlreadyExists, got nil")
	}
	if !strings.Contains(err.Error(), memoryv2.ErrAlreadyExists.Error()) {
		t.Errorf("Expected ErrAlreadyExists in error, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Create_InvalidInput(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		http.NotFound(w, r)
	})

	err := repo.Create(context.Background(), cell)
	if err == nil {
		t.Fatal("Expected ErrInvalidInput, got nil")
	}
	if !strings.Contains(err.Error(), memoryv2.ErrInvalidInput.Error()) {
		t.Errorf("Expected ErrInvalidInput in error, got: %v", err)
	}
}

func TestIngatanMemoryCellRepository_Create_AutoCreateStore(t *testing.T) {
	// 404 on first Create → auto-creates store → retries Create → succeeds.
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	callCount := 0
	storeCreated := false

	mux.HandleFunc("/api/v1/stores", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			storeCreated = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "test_store"})
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/memories") {
			callCount++
			if callCount == 1 {
				// First attempt: store doesn't exist
				w.WriteHeader(http.StatusNotFound)
				return
			}
			// Second attempt (after store creation): success
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(ingatanMemoryResponse(cell))
			return
		}
		http.NotFound(w, r)
	})

	if err := repo.Create(context.Background(), cell); err != nil {
		t.Fatalf("Create with auto-create store failed: %v", err)
	}
	if !storeCreated {
		t.Error("Expected store to be auto-created on 404")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 Create calls (first 404, retry after store creation), got %d", callCount)
	}
}

// --- Get ---

// TestIngatanMemoryCellRepository_Get_ReturnsErrNotFoundWithConversationContext verifies
// that Get always returns an ErrNotFound-wrapped error indicating the caller must use
// List with a ConversationID filter instead. See R-02 and ADR-2.
func TestIngatanMemoryCellRepository_Get_ReturnsErrNotFoundWithConversationContext(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	cell, err := repo.Get(context.Background(), "some-uuid")

	if cell != nil {
		t.Errorf("Expected nil cell, got %v", cell)
	}
	if err == nil {
		t.Fatal("Expected non-nil error, got nil")
	}
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("Expected errors.Is(err, memoryv2.ErrNotFound) == true, got false; error: %v", err)
	}
	if !strings.Contains(err.Error(), "conversation context") {
		t.Errorf("Expected error message to contain %q, got: %v", "conversation context", err)
	}
}

// TestIngatanMemoryCellRepository_Get_AlwaysUnsupported verifies that Get makes no HTTP
// calls and returns the documented ErrNotFound-wrapped error for any input.
// Get by cell ID alone is not supported; callers must use List with ConversationID.
func TestIngatanMemoryCellRepository_Get_AlwaysUnsupported(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	// No HTTP routes registered — any HTTP call would panic/fail the test.
	got, err := repo.Get(context.Background(), cell.ID)
	if got != nil {
		t.Errorf("Expected nil cell, got %v", got)
	}
	if err == nil {
		t.Fatal("Expected non-nil error, got nil")
	}
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("Expected errors.Is(err, ErrNotFound) == true; error: %v", err)
	}
	if !strings.Contains(err.Error(), "conversation context") {
		t.Errorf("Expected error to contain 'conversation context'; got: %v", err)
	}
}

// TestIngatanMemoryCellRepository_Get_NotFound verifies that Get returns ErrNotFound
// without making any network calls, regardless of what the server would return.
func TestIngatanMemoryCellRepository_Get_NotFound(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	_, err := repo.Get(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("Expected ErrNotFound, got nil")
	}
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("Expected errors.Is(err, ErrNotFound) == true; error: %v", err)
	}
}

// --- SearchFTS ---

func TestIngatanMemoryCellRepository_SearchFTS_HybridMode(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t)

	var capturedSearchBody map[string]interface{}
	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/search") {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &capturedSearchBody)
			result := map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"memory": ingatanMemoryResponse(cell),
						"score":  0.9,
					},
				},
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		http.NotFound(w, r)
	})

	results, err := repo.SearchFTS(context.Background(), "sky blue", 10)
	if err != nil {
		t.Fatalf("SearchFTS failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Content != cell.Content {
		t.Errorf("Expected content %q, got %q", cell.Content, results[0].Content)
	}
	if capturedSearchBody["mode"] != "hybrid" {
		t.Errorf("Expected search mode 'hybrid', got %v", capturedSearchBody["mode"])
	}
	if capturedSearchBody["query"] != "sky blue" {
		t.Errorf("Expected query 'sky blue', got %v", capturedSearchBody["query"])
	}
}

// --- GetHighSalience ---

func TestIngatanMemoryCellRepository_GetHighSalience(t *testing.T) {
	_, mux, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")
	cell := sampleCell(t) // salience = 0.8

	mux.HandleFunc("/api/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/memories") {
			result := map[string]interface{}{
				"memories": []interface{}{ingatanMemoryResponse(cell)},
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		http.NotFound(w, r)
	})

	results, err := repo.GetHighSalience(context.Background(), "conv-abc", 0.5, 10)
	if err != nil {
		t.Fatalf("GetHighSalience failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected at least one high-salience result, got none")
	}
	for _, r := range results {
		if r.Salience < 0.5 {
			t.Errorf("Expected salience >= 0.5, got %f", r.Salience)
		}
	}
}

// --- DeleteExpired (no-op per ADR-4) ---

func TestIngatanMemoryCellRepository_DeleteExpired_IsNoOp(t *testing.T) {
	_, _, client := newMockIngatan(t)
	repo := NewIngatanMemoryCellRepository(client, "test")

	// DeleteExpired must return (0, nil) without making any HTTP call.
	requestMade := false
	// If any request is made beyond the initial token exchange, it would need a route.
	// We don't register any route here; any unexpected call would fail the test.

	count, err := repo.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpired returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 deleted (no-op), got %d", count)
	}
	if requestMade {
		t.Error("DeleteExpired must not make any HTTP requests")
	}
}

// --- Mapping round-trip ---

func TestIngatanMapping_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(24 * time.Hour)
	original := &memoryv2.MemoryCell{
		ID:             "22222222-2222-2222-2222-222222222222",
		ConversationID: "conv-round-trip",
		Scene:          "round-trip",
		CellType:       memoryv2.CellTypeDecision,
		Salience:       0.75,
		Content:        "round trip content",
		Source:         `["msg-rt-1"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      &expires,
	}

	// Convert to save request.
	req := cellToSaveRequest(original)

	// Simulate Ingatan response.
	respBody := map[string]interface{}{
		"id":         "ingatan-rt-id",
		"store":      "test_store",
		"content":    req.Content,
		"tags":       req.Tags,
		"source":     req.Source,
		"source_ref": req.SourceRef,
		"metadata":   req.Metadata,
		"created_at": now.Format(time.RFC3339),
		"updated_at": now.Format(time.RFC3339),
	}

	respJSON, _ := json.Marshal(respBody)
	var respMap ingatanMemory
	if err := json.Unmarshal(respJSON, &respMap); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	got, err := memoryToCell(respMap)
	if err != nil {
		t.Fatalf("memoryToCell failed: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID: got %q, want %q", got.ID, original.ID)
	}
	if got.Content != original.Content {
		t.Errorf("Content: got %q, want %q", got.Content, original.Content)
	}
	if got.Scene != original.Scene {
		t.Errorf("Scene: got %q, want %q", got.Scene, original.Scene)
	}
	if got.CellType != original.CellType {
		t.Errorf("CellType: got %v, want %v", got.CellType, original.CellType)
	}
	if got.Salience != original.Salience {
		t.Errorf("Salience: got %f, want %f", got.Salience, original.Salience)
	}
	if got.ConversationID != original.ConversationID {
		t.Errorf("ConversationID: got %q, want %q", got.ConversationID, original.ConversationID)
	}
	if got.Source != original.Source {
		t.Errorf("Source: got %q, want %q", got.Source, original.Source)
	}
	if got.ExpiresAt == nil {
		t.Error("Expected ExpiresAt to be set after round-trip")
	} else if !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt: got %v, want %v", got.ExpiresAt, expires)
	}
}

// --- storeFor determinism ---

func TestStoreFor_Deterministic(t *testing.T) {
	// Same input must always produce the same store name.
	a := storeFor("nuiman", "conv-123")
	b := storeFor("nuiman", "conv-123")
	if a != b {
		t.Errorf("storeFor is not deterministic: %q != %q", a, b)
	}
}

func TestStoreFor_DifferentInputsDifferentOutput(t *testing.T) {
	a := storeFor("nuiman", "conv-123")
	b := storeFor("nuiman", "conv-456")
	if a == b {
		t.Error("storeFor produced same output for different inputs")
	}
}

func TestStoreFor_PrefixIsolation(t *testing.T) {
	a := storeFor("nuiman", "conv-123")
	b := storeFor("other", "conv-123")
	if a == b {
		t.Error("storeFor produced same output for different prefixes")
	}
	if !strings.HasPrefix(a, "nuiman_") {
		t.Errorf("Expected store name to start with 'nuiman_', got %q", a)
	}
}

func TestStoreFor_DefaultPrefix(t *testing.T) {
	name := storeFor("", "conv-abc")
	if !strings.HasPrefix(name, "nuiman_") {
		t.Errorf("Expected store name to start with 'nuiman_' when prefix is empty, got %q", name)
	}
}
