package cli

import (
	"context"
	"testing"

	cliadapter "nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain/memoryv2"
)

// discardWriter discards all output for testing
type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) { return len(p), nil }

// mockCellRepo implements memoryv2.MemoryCellRepository for testing.
type mockCellRepo struct {
	cells     []*memoryv2.MemoryCell
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	searchErr error
}

func (m *mockCellRepo) Create(ctx context.Context, cell *memoryv2.MemoryCell) error {
	return m.createErr
}
func (m *mockCellRepo) Get(ctx context.Context, id string) (*memoryv2.MemoryCell, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, c := range m.cells {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, memoryv2.ErrNotFound
}
func (m *mockCellRepo) List(ctx context.Context, filter memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	return m.cells, nil
}
func (m *mockCellRepo) Update(ctx context.Context, cell *memoryv2.MemoryCell) error {
	return m.updateErr
}
func (m *mockCellRepo) Delete(ctx context.Context, id string) error {
	return m.deleteErr
}
func (m *mockCellRepo) SearchFTS(ctx context.Context, query string, limit int) ([]*memoryv2.MemoryCell, error) {
	return m.cells, m.searchErr
}
func (m *mockCellRepo) GetByScene(ctx context.Context, scene string, limit int) ([]*memoryv2.MemoryCell, error) {
	return m.cells, nil
}
func (m *mockCellRepo) GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*memoryv2.MemoryCell, error) {
	return m.cells, nil
}
func (m *mockCellRepo) DeleteExpired(ctx context.Context) (int, error) {
	return 0, nil
}

// mockSceneRepo implements memoryv2.MemorySceneRepository for testing.
type mockSceneRepo struct {
	scenes    []*memoryv2.MemoryScene
	upsertErr error
	getErr    error
	deleteErr error
}

func (m *mockSceneRepo) Upsert(ctx context.Context, scene *memoryv2.MemoryScene) error {
	return m.upsertErr
}
func (m *mockSceneRepo) Get(ctx context.Context, name string) (*memoryv2.MemoryScene, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, s := range m.scenes {
		if s.Scene == name {
			return s, nil
		}
	}
	return nil, memoryv2.ErrNotFound
}
func (m *mockSceneRepo) List(ctx context.Context) ([]*memoryv2.MemoryScene, error) {
	return m.scenes, nil
}
func (m *mockSceneRepo) Delete(ctx context.Context, name string) error {
	return m.deleteErr
}

func newTestMemoryHandler() *MemoryCommandHandler {
	cellRepo := &mockCellRepo{}
	sceneRepo := &mockSceneRepo{}
	w := &discardWriter{}
	cmd := cliadapter.NewMemoryCommand(cellRepo, sceneRepo, w)
	return NewMemoryCommandHandler(cmd)
}

func TestIsMemoryCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/memory list", true},
		{"/memory get abc", true},
		{"/memory ", true},
		{"/admin bot list", false},
		{"memory list", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsMemoryCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsMemoryCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_ShowHelp(t *testing.T) {
	h := newTestMemoryHandler()

	// Short input - show help
	if err := h.HandleMemoryCommand(context.Background(), "/memory"); err != nil {
		t.Errorf("HandleMemoryCommand('/memory') returned error: %v", err)
	}

	// Help subcommand
	if err := h.HandleMemoryCommand(context.Background(), "/memory help"); err != nil {
		t.Errorf("HandleMemoryCommand('/memory help') returned error: %v", err)
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_UnknownSubcommand(t *testing.T) {
	h := newTestMemoryHandler()

	err := h.HandleMemoryCommand(context.Background(), "/memory unknown-cmd")
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_List(t *testing.T) {
	h := newTestMemoryHandler()

	if err := h.HandleMemoryCommand(context.Background(), "/memory list"); err != nil {
		t.Errorf("HandleMemoryCommand('/memory list') returned error: %v", err)
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_ListWithArgs(t *testing.T) {
	h := newTestMemoryHandler()
	ctx := context.Background()

	cases := []string{
		"/memory list --scene myscene",
		"/memory list --type fact",
		"/memory list --limit 5",
		"/memory list --format json",
		"/memory list --conversation conv-1",
	}

	for _, cmd := range cases {
		t.Run(cmd, func(t *testing.T) {
			if err := h.HandleMemoryCommand(ctx, cmd); err != nil {
				t.Errorf("HandleMemoryCommand(%q) returned error: %v", cmd, err)
			}
		})
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_ListInvalidLimit(t *testing.T) {
	h := newTestMemoryHandler()

	err := h.HandleMemoryCommand(context.Background(), "/memory list --limit notanumber")
	if err == nil {
		t.Error("expected error for invalid limit")
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_ListInvalidType(t *testing.T) {
	h := newTestMemoryHandler()

	err := h.HandleMemoryCommand(context.Background(), "/memory list --type invalidcelltype")
	if err == nil {
		t.Error("expected error for invalid cell type")
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_Get(t *testing.T) {
	h := newTestMemoryHandler()
	ctx := context.Background()

	// Get without ID - error
	err := h.HandleMemoryCommand(ctx, "/memory get")
	if err == nil {
		t.Error("expected error when no ID provided for get")
	}

	// Get with ID (will error as "not found", but should not panic)
	_ = h.HandleMemoryCommand(ctx, "/memory get some-id")

	// Get with format flag
	_ = h.HandleMemoryCommand(ctx, "/memory get some-id --format json")
}

func TestMemoryCommandHandler_HandleMemoryCommand_Search(t *testing.T) {
	h := newTestMemoryHandler()
	ctx := context.Background()

	// Search without query - error
	err := h.HandleMemoryCommand(ctx, "/memory search")
	if err == nil {
		t.Error("expected error when no query provided for search")
	}

	// Search with query
	_ = h.HandleMemoryCommand(ctx, "/memory search hello world")

	// Search with limit
	_ = h.HandleMemoryCommand(ctx, "/memory search hello --limit 5")

	// Search with format
	_ = h.HandleMemoryCommand(ctx, "/memory search hello --format json")
}

func TestMemoryCommandHandler_HandleMemoryCommand_SearchInvalidLimit(t *testing.T) {
	h := newTestMemoryHandler()

	err := h.HandleMemoryCommand(context.Background(), "/memory search hello --limit abc")
	if err == nil {
		t.Error("expected error for invalid limit in search")
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_SearchEmptyQuery(t *testing.T) {
	h := newTestMemoryHandler()

	// Only flags, no query words
	err := h.HandleMemoryCommand(context.Background(), "/memory search --limit 5")
	if err == nil {
		t.Error("expected error when search query is empty")
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_Delete(t *testing.T) {
	h := newTestMemoryHandler()
	ctx := context.Background()

	// Delete without ID - error
	err := h.HandleMemoryCommand(ctx, "/memory delete")
	if err == nil {
		t.Error("expected error when no ID provided for delete")
	}

	// Delete with ID
	_ = h.HandleMemoryCommand(ctx, "/memory delete some-id")
}

func TestMemoryCommandHandler_HandleMemoryCommand_Scenes(t *testing.T) {
	h := newTestMemoryHandler()
	ctx := context.Background()

	if err := h.HandleMemoryCommand(ctx, "/memory scenes"); err != nil {
		t.Errorf("HandleMemoryCommand('/memory scenes') returned error: %v", err)
	}

	if err := h.HandleMemoryCommand(ctx, "/memory scenes --format json"); err != nil {
		t.Errorf("HandleMemoryCommand('/memory scenes --format json') returned error: %v", err)
	}
}

func TestMemoryCommandHandler_HandleMemoryCommand_Prune(t *testing.T) {
	h := newTestMemoryHandler()
	// prune may fail depending on implementation, just verify no panic
	_ = h.HandleMemoryCommand(context.Background(), "/memory prune")
}
