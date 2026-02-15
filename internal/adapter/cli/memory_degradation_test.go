package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// --- Failing Repository Mocks for CLI ---

type failingCellRepo struct {
	listErr          error
	getErr           error
	searchErr        error
	deleteErr        error
	deleteExpiredErr error
}

func (r *failingCellRepo) Create(_ context.Context, _ *memoryv2.MemoryCell) error { return nil }

func (r *failingCellRepo) Get(_ context.Context, _ string) (*memoryv2.MemoryCell, error) {
	return nil, r.getErr
}

func (r *failingCellRepo) List(_ context.Context, _ memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	return nil, r.listErr
}

func (r *failingCellRepo) Delete(_ context.Context, _ string) error {
	return r.deleteErr
}

func (r *failingCellRepo) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, r.searchErr
}

func (r *failingCellRepo) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (r *failingCellRepo) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (r *failingCellRepo) DeleteExpired(_ context.Context) (int, error) {
	return 0, r.deleteExpiredErr
}

type failingSceneRepo struct {
	listErr error
}

func (r *failingSceneRepo) Upsert(_ context.Context, _ *memoryv2.MemoryScene) error { return nil }

func (r *failingSceneRepo) Get(_ context.Context, _ string) (*memoryv2.MemoryScene, error) {
	return nil, memoryv2.ErrNotFound
}

func (r *failingSceneRepo) List(_ context.Context) ([]*memoryv2.MemoryScene, error) {
	return nil, r.listErr
}

func (r *failingSceneRepo) Delete(_ context.Context, _ string) error { return nil }

// --- CLI Degradation Tests ---

func TestMemoryCommand_Degradation_ListFails(t *testing.T) {
	repo := &failingCellRepo{listErr: errors.New("database locked")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.List(context.Background(), memoryv2.MemoryCellFilter{}, "table")

	if err == nil {
		t.Fatal("Expected error when List fails")
	}
	if !strings.Contains(err.Error(), "database locked") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_GetFails(t *testing.T) {
	repo := &failingCellRepo{getErr: errors.New("connection reset")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.Get(context.Background(), "some-id", "table")

	if err == nil {
		t.Fatal("Expected error when Get fails")
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_SearchFails(t *testing.T) {
	repo := &failingCellRepo{searchErr: errors.New("FTS index corrupted")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.Search(context.Background(), "query", 10, "table")

	if err == nil {
		t.Fatal("Expected error when Search fails")
	}
	if !strings.Contains(err.Error(), "FTS index corrupted") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_DeleteFails(t *testing.T) {
	repo := &failingCellRepo{deleteErr: errors.New("permission denied")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.Delete(context.Background(), "some-id")

	if err == nil {
		t.Fatal("Expected error when Delete fails")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_ScenesFails(t *testing.T) {
	sceneRepo := &failingSceneRepo{listErr: errors.New("disk full")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(newMockCellRepo(), sceneRepo, buf)

	err := cmd.Scenes(context.Background(), "table")

	if err == nil {
		t.Fatal("Expected error when Scenes fails")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_PruneFails(t *testing.T) {
	repo := &failingCellRepo{deleteExpiredErr: errors.New("table locked")}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.Prune(context.Background())

	if err == nil {
		t.Fatal("Expected error when Prune fails")
	}
	if !strings.Contains(err.Error(), "table locked") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_ErrorsWrapped(t *testing.T) {
	// All errors should be wrapped with context for debugging.
	dbErr := errors.New("underlying db error")
	repo := &failingCellRepo{
		listErr:          dbErr,
		getErr:           dbErr,
		searchErr:        dbErr,
		deleteErr:        dbErr,
		deleteExpiredErr: dbErr,
	}
	sceneRepo := &failingSceneRepo{listErr: dbErr}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, sceneRepo, buf)

	tests := []struct {
		name   string
		fn     func() error
		prefix string
	}{
		{"List", func() error { return cmd.List(context.Background(), memoryv2.MemoryCellFilter{}, "table") }, "failed to list"},
		{"Get", func() error { return cmd.Get(context.Background(), "id", "table") }, "failed to get"},
		{"Search", func() error { return cmd.Search(context.Background(), "q", 10, "table") }, "search failed"},
		{"Delete", func() error { return cmd.Delete(context.Background(), "id") }, "failed to delete"},
		{"Scenes", func() error { return cmd.Scenes(context.Background(), "table") }, "failed to list scenes"},
		{"Prune", func() error { return cmd.Prune(context.Background()) }, "prune failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			err := tt.fn()
			if err == nil {
				t.Fatalf("%s should return error", tt.name)
			}
			if !strings.Contains(err.Error(), tt.prefix) {
				t.Errorf("Expected error prefix %q, got: %v", tt.prefix, err)
			}
			// Verify the underlying error is wrapped
			if !errors.Is(err, dbErr) {
				t.Errorf("Expected wrapped error with errors.Is, got: %v", err)
			}
		})
	}
}

func TestMemoryCommand_Degradation_GetNotFound(t *testing.T) {
	// ErrNotFound should be wrapped properly.
	repo := &failingCellRepo{getErr: memoryv2.ErrNotFound}
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(repo, newMockSceneRepo(), buf)

	err := cmd.Get(context.Background(), "nonexistent", "table")

	if err == nil {
		t.Fatal("Expected error for not found")
	}
	if !errors.Is(err, memoryv2.ErrNotFound) {
		t.Errorf("Expected ErrNotFound to be wrapped, got: %v", err)
	}
}

func TestMemoryCommand_Degradation_RenderCellWithExpiry(t *testing.T) {
	// Cells with expiry dates should render without panic or error.
	cellRepo := newMockCellRepo()
	now := time.Now()
	expiry := now.Add(24 * time.Hour)
	cell := &memoryv2.MemoryCell{
		ID:             "cell-with-expiry",
		ConversationID: "conv-1",
		Scene:          "test-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.5,
		Content:        "Expires soon",
		Source:         `["msg-1"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      &expiry,
	}
	cellRepo.cells[cell.ID] = cell

	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(cellRepo, newMockSceneRepo(), buf)

	err := cmd.Get(context.Background(), cell.ID, "table")
	if err != nil {
		t.Fatalf("Get with expiry should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "Expires At") {
		t.Error("Expected 'Expires At' in output for cell with expiry")
	}
}
