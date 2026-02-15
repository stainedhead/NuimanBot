package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// --- Mock Repositories ---

type mockMemoryCellRepository struct {
	cells         map[string]*memoryv2.MemoryCell
	searchResults []*memoryv2.MemoryCell
	listResults   []*memoryv2.MemoryCell
	deleteErr     error
	deletedCount  int
}

func newMockCellRepo() *mockMemoryCellRepository {
	return &mockMemoryCellRepository{
		cells: make(map[string]*memoryv2.MemoryCell),
	}
}

func (m *mockMemoryCellRepository) Create(_ context.Context, cell *memoryv2.MemoryCell) error {
	if _, exists := m.cells[cell.ID]; exists {
		return memoryv2.ErrAlreadyExists
	}
	m.cells[cell.ID] = cell
	return nil
}

func (m *mockMemoryCellRepository) Get(_ context.Context, id string) (*memoryv2.MemoryCell, error) {
	cell, ok := m.cells[id]
	if !ok {
		return nil, memoryv2.ErrNotFound
	}
	return cell, nil
}

func (m *mockMemoryCellRepository) List(_ context.Context, _ memoryv2.MemoryCellFilter) ([]*memoryv2.MemoryCell, error) {
	if m.listResults != nil {
		return m.listResults, nil
	}
	result := make([]*memoryv2.MemoryCell, 0, len(m.cells))
	for _, c := range m.cells {
		result = append(result, c)
	}
	return result, nil
}

func (m *mockMemoryCellRepository) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.cells[id]; !ok {
		return memoryv2.ErrNotFound
	}
	delete(m.cells, id)
	return nil
}

func (m *mockMemoryCellRepository) SearchFTS(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return m.searchResults, nil
}

func (m *mockMemoryCellRepository) GetByScene(_ context.Context, _ string, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *mockMemoryCellRepository) GetHighSalience(_ context.Context, _ string, _ float64, _ int) ([]*memoryv2.MemoryCell, error) {
	return nil, nil
}

func (m *mockMemoryCellRepository) DeleteExpired(_ context.Context) (int, error) {
	return m.deletedCount, nil
}

type mockMemorySceneRepository struct {
	scenes map[string]*memoryv2.MemoryScene
}

func newMockSceneRepo() *mockMemorySceneRepository {
	return &mockMemorySceneRepository{
		scenes: make(map[string]*memoryv2.MemoryScene),
	}
}

func (m *mockMemorySceneRepository) Upsert(_ context.Context, scene *memoryv2.MemoryScene) error {
	m.scenes[scene.Scene] = scene
	return nil
}

func (m *mockMemorySceneRepository) Get(_ context.Context, scene string) (*memoryv2.MemoryScene, error) {
	s, ok := m.scenes[scene]
	if !ok {
		return nil, memoryv2.ErrNotFound
	}
	return s, nil
}

func (m *mockMemorySceneRepository) List(_ context.Context) ([]*memoryv2.MemoryScene, error) {
	result := make([]*memoryv2.MemoryScene, 0, len(m.scenes))
	for _, s := range m.scenes {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockMemorySceneRepository) Delete(_ context.Context, scene string) error {
	if _, ok := m.scenes[scene]; !ok {
		return memoryv2.ErrNotFound
	}
	delete(m.scenes, scene)
	return nil
}

// --- Helpers ---

func testCell(id, scene, content string, salience float64, cellType memoryv2.CellType) *memoryv2.MemoryCell {
	now := time.Now()
	return &memoryv2.MemoryCell{
		ID:             id,
		ConversationID: "conv-1",
		Scene:          scene,
		CellType:       cellType,
		Salience:       salience,
		Content:        content,
		Source:         `["msg-1"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func testScene(name, summary string, tokenCount int) *memoryv2.MemoryScene {
	return &memoryv2.MemoryScene{
		Scene:      name,
		Summary:    summary,
		TokenCount: tokenCount,
		UpdatedAt:  time.Now(),
	}
}

func newTestMemoryCommand() (*MemoryCommand, *mockMemoryCellRepository, *mockMemorySceneRepository, *bytes.Buffer) {
	cellRepo := newMockCellRepo()
	sceneRepo := newMockSceneRepo()
	buf := &bytes.Buffer{}
	cmd := NewMemoryCommand(cellRepo, sceneRepo, buf)
	return cmd, cellRepo, sceneRepo, buf
}

// --- Test: List ---

func TestMemoryCommand_List_Empty(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()

	err := cmd.List(context.Background(), memoryv2.MemoryCellFilter{}, "table")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("No memory cells found")) {
		t.Errorf("Expected 'No memory cells found', got: %s", buf.String())
	}
}

func TestMemoryCommand_List_WithCells(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell1 := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cell2 := testCell("11111111-2222-3333-4444-555555555555", "user-prefs", "Prefers dark mode", 0.70, memoryv2.CellTypePreference)
	cellRepo.cells[cell1.ID] = cell1
	cellRepo.cells[cell2.ID] = cell2

	err := cmd.List(context.Background(), memoryv2.MemoryCellFilter{}, "table")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("project-setup")) {
		t.Errorf("Expected scene 'project-setup' in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("user-prefs")) {
		t.Errorf("Expected scene 'user-prefs' in output, got: %s", output)
	}
}

func TestMemoryCommand_List_JSONFormat(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cellRepo.cells[cell.ID] = cell

	err := cmd.List(context.Background(), memoryv2.MemoryCellFilter{}, "json")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should be valid JSON
	var result []interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Errorf("Expected valid JSON output, got error: %v\nOutput: %s", jsonErr, buf.String())
	}
}

// --- Test: Get ---

func TestMemoryCommand_Get_Found(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cellRepo.cells[cell.ID] = cell

	err := cmd.Get(context.Background(), cell.ID, "table")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte(cell.ID)) {
		t.Errorf("Expected cell ID in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Go project initialized")) {
		t.Errorf("Expected content in output, got: %s", output)
	}
}

func TestMemoryCommand_Get_NotFound(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.Get(context.Background(), "nonexistent-id", "table")
	if err == nil {
		t.Fatal("Get() should return error for nonexistent cell")
	}
}

func TestMemoryCommand_Get_JSONFormat(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cellRepo.cells[cell.ID] = cell

	err := cmd.Get(context.Background(), cell.ID, "json")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var result map[string]interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Errorf("Expected valid JSON, got error: %v\nOutput: %s", jsonErr, buf.String())
	}
}

// --- Test: Search ---

func TestMemoryCommand_Search_WithResults(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cellRepo.searchResults = []*memoryv2.MemoryCell{cell}

	err := cmd.Search(context.Background(), "project", 10, "table")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Go project initialized")) {
		t.Errorf("Expected content in search results, got: %s", output)
	}
}

func TestMemoryCommand_Search_NoResults(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()
	cellRepo.searchResults = []*memoryv2.MemoryCell{}

	err := cmd.Search(context.Background(), "nonexistent", 10, "table")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("No results found")) {
		t.Errorf("Expected 'No results found', got: %s", buf.String())
	}
}

func TestMemoryCommand_Search_EmptyQuery(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.Search(context.Background(), "", 10, "table")
	if err == nil {
		t.Fatal("Search() should return error for empty query")
	}
}

// --- Test: Delete ---

func TestMemoryCommand_Delete_Success(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project initialized", 0.85, memoryv2.CellTypeFact)
	cellRepo.cells[cell.ID] = cell

	err := cmd.Delete(context.Background(), cell.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("deleted")) {
		t.Errorf("Expected deletion confirmation, got: %s", buf.String())
	}

	// Verify cell is gone
	if _, exists := cellRepo.cells[cell.ID]; exists {
		t.Error("Cell should have been deleted from repository")
	}
}

func TestMemoryCommand_Delete_NotFound(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.Delete(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("Delete() should return error for nonexistent cell")
	}
}

// --- Test: Scenes ---

func TestMemoryCommand_Scenes_Empty(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()

	err := cmd.Scenes(context.Background(), "table")
	if err != nil {
		t.Fatalf("Scenes() error = %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("No scenes found")) {
		t.Errorf("Expected 'No scenes found', got: %s", buf.String())
	}
}

func TestMemoryCommand_Scenes_WithScenes(t *testing.T) {
	cmd, _, sceneRepo, buf := newTestMemoryCommand()

	scene1 := testScene("project-setup", "Project setup involves Go and SQLite", 15)
	scene2 := testScene("user-prefs", "User prefers dark mode and vim", 12)
	sceneRepo.scenes[scene1.Scene] = scene1
	sceneRepo.scenes[scene2.Scene] = scene2

	err := cmd.Scenes(context.Background(), "table")
	if err != nil {
		t.Fatalf("Scenes() error = %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("project-setup")) {
		t.Errorf("Expected 'project-setup' in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("user-prefs")) {
		t.Errorf("Expected 'user-prefs' in output, got: %s", output)
	}
}

func TestMemoryCommand_Scenes_JSONFormat(t *testing.T) {
	cmd, _, sceneRepo, buf := newTestMemoryCommand()

	scene := testScene("project-setup", "Project setup involves Go and SQLite", 15)
	sceneRepo.scenes[scene.Scene] = scene

	err := cmd.Scenes(context.Background(), "json")
	if err != nil {
		t.Fatalf("Scenes() error = %v", err)
	}

	var result []interface{}
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Errorf("Expected valid JSON, got error: %v\nOutput: %s", jsonErr, buf.String())
	}
}

// --- Test: Prune ---

func TestMemoryCommand_Prune_WithExpired(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()
	cellRepo.deletedCount = 5

	err := cmd.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("5")) {
		t.Errorf("Expected count of 5 in output, got: %s", output)
	}
}

func TestMemoryCommand_Prune_NoneExpired(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()
	cellRepo.deletedCount = 0

	err := cmd.Prune(context.Background())
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if !bytes.Contains(buf.Bytes(), []byte("No expired")) {
		t.Errorf("Expected 'No expired' message, got: %s", buf.String())
	}
}
