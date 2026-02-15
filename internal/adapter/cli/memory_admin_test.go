package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"
)

// --- Mock MemoryAdmin ---

type mockMemoryAdmin struct {
	stats        *MemoryStats
	statsErr     error
	cellCount    int
	countErr     error
	deletedCount int
	deleteErr    error
	rebuildErr   error
}

func (m *mockMemoryAdmin) Stats(_ context.Context) (*MemoryStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	return m.stats, nil
}

func (m *mockMemoryAdmin) CountCellsByConversation(_ context.Context, _ string) (int, error) {
	return m.cellCount, m.countErr
}

func (m *mockMemoryAdmin) DeleteCellsByConversation(_ context.Context, _ string) (int, error) {
	return m.deletedCount, m.deleteErr
}

func (m *mockMemoryAdmin) RebuildFTSIndex(_ context.Context) error {
	return m.rebuildErr
}

// --- Stats Tests ---

func TestMemoryCommand_Stats_Success(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{
		stats: &MemoryStats{CellCount: 42, SceneCount: 5, DBSizeBytes: 1048576},
	})

	err := cmd.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "42") {
		t.Errorf("Expected cell count 42, got: %s", output)
	}
	if !strings.Contains(output, "5") {
		t.Errorf("Expected scene count 5, got: %s", output)
	}
	if !strings.Contains(output, "1.0 MB") {
		t.Errorf("Expected '1.0 MB', got: %s", output)
	}
}

func TestMemoryCommand_Stats_AdminNotSet(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.Stats(context.Background())
	if err == nil {
		t.Fatal("Stats() should error when admin not set")
	}
	if !strings.Contains(err.Error(), "admin operations not available") {
		t.Errorf("Expected admin not available error, got: %v", err)
	}
}

func TestMemoryCommand_Stats_Error(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{
		statsErr: errors.New("db error"),
	})

	err := cmd.Stats(context.Background())
	if err == nil {
		t.Fatal("Stats() should return error on failure")
	}
	if !strings.Contains(err.Error(), "db error") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

func TestMemoryCommand_Stats_SmallDBSize(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{
		stats: &MemoryStats{CellCount: 0, SceneCount: 0, DBSizeBytes: 512},
	})

	err := cmd.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	if !strings.Contains(buf.String(), "512 B") {
		t.Errorf("Expected '512 B', got: %s", buf.String())
	}
}

// --- ClearUser Tests ---

func TestMemoryCommand_ClearUser_Confirmed(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{deletedCount: 15})

	err := cmd.ClearUser(context.Background(), "telegram:user123", true)
	if err != nil {
		t.Fatalf("ClearUser() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Deleted") {
		t.Errorf("Expected 'Deleted' in output, got: %s", output)
	}
	if !strings.Contains(output, "15") {
		t.Errorf("Expected count 15, got: %s", output)
	}
	if !strings.Contains(output, "telegram:user123") {
		t.Errorf("Expected conversation ID, got: %s", output)
	}
}

func TestMemoryCommand_ClearUser_DryRun(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{cellCount: 15})

	err := cmd.ClearUser(context.Background(), "telegram:user123", false)
	if err != nil {
		t.Fatalf("ClearUser() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Would delete") {
		t.Errorf("Expected 'Would delete', got: %s", output)
	}
	if !strings.Contains(output, "15") {
		t.Errorf("Expected count 15, got: %s", output)
	}
	if !strings.Contains(output, "--confirm") {
		t.Errorf("Expected '--confirm' instruction, got: %s", output)
	}
}

func TestMemoryCommand_ClearUser_DryRunNothingToDelete(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{cellCount: 0})

	err := cmd.ClearUser(context.Background(), "telegram:user123", false)
	if err != nil {
		t.Fatalf("ClearUser() error = %v", err)
	}

	if !strings.Contains(buf.String(), "No memory cells found") {
		t.Errorf("Expected 'No memory cells found', got: %s", buf.String())
	}
}

func TestMemoryCommand_ClearUser_NothingToDelete(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{deletedCount: 0})

	err := cmd.ClearUser(context.Background(), "telegram:user123", true)
	if err != nil {
		t.Fatalf("ClearUser() error = %v", err)
	}

	if !strings.Contains(buf.String(), "No memory cells found") {
		t.Errorf("Expected 'No memory cells found', got: %s", buf.String())
	}
}

func TestMemoryCommand_ClearUser_EmptyConversationID(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{})

	err := cmd.ClearUser(context.Background(), "", true)
	if err == nil {
		t.Fatal("ClearUser() should error for empty conversation ID")
	}
}

func TestMemoryCommand_ClearUser_AdminNotSet(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.ClearUser(context.Background(), "telegram:user123", true)
	if err == nil {
		t.Fatal("ClearUser() should error when admin not set")
	}
}

func TestMemoryCommand_ClearUser_DeleteError(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{
		deleteErr: errors.New("permission denied"),
	})

	err := cmd.ClearUser(context.Background(), "telegram:user123", true)
	if err == nil {
		t.Fatal("ClearUser() should propagate delete error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

// --- Export Tests ---

func TestMemoryCommand_Export_Success(t *testing.T) {
	cmd, cellRepo, sceneRepo, buf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project", 0.85, memoryv2.CellTypeFact)
	cell.ConversationID = "telegram:user123"
	cellRepo.listResults = []*memoryv2.MemoryCell{cell}

	scene := testScene("project-setup", "Project setup summary", 10)
	sceneRepo.scenes["project-setup"] = scene

	err := cmd.Export(context.Background(), "telegram:user123")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify valid JSON
	var export memoryExportData
	if err := json.Unmarshal(buf.Bytes(), &export); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v\nOutput: %s", err, buf.String())
	}

	if export.Version != 1 {
		t.Errorf("Expected version 1, got %d", export.Version)
	}
	if export.ConversationID != "telegram:user123" {
		t.Errorf("Expected conversation ID 'telegram:user123', got %q", export.ConversationID)
	}
	if export.CellCount != 1 {
		t.Errorf("Expected 1 cell, got %d", export.CellCount)
	}
	if len(export.Cells) != 1 {
		t.Errorf("Expected 1 cell in data, got %d", len(export.Cells))
	}
	if len(export.Scenes) != 1 {
		t.Errorf("Expected 1 scene in data, got %d", len(export.Scenes))
	}
}

func TestMemoryCommand_Export_NoData(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()
	cellRepo.listResults = []*memoryv2.MemoryCell{}

	err := cmd.Export(context.Background(), "telegram:user123")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if !strings.Contains(buf.String(), "No memory cells found") {
		t.Errorf("Expected 'No memory cells found', got: %s", buf.String())
	}
}

func TestMemoryCommand_Export_EmptyConversationID(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.Export(context.Background(), "")
	if err == nil {
		t.Fatal("Export() should error for empty conversation ID")
	}
}

func TestMemoryCommand_Export_CellWithoutScene(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	// Cell references scene that doesn't exist in scene repo
	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "missing-scene", "Content", 0.5, memoryv2.CellTypeFact)
	cell.ConversationID = "telegram:user123"
	cellRepo.listResults = []*memoryv2.MemoryCell{cell}

	err := cmd.Export(context.Background(), "telegram:user123")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Should still produce valid JSON with cell but no scenes
	var export memoryExportData
	if err := json.Unmarshal(buf.Bytes(), &export); err != nil {
		t.Fatalf("Expected valid JSON, got error: %v", err)
	}
	if len(export.Cells) != 1 {
		t.Errorf("Expected 1 cell, got %d", len(export.Cells))
	}
	if len(export.Scenes) != 0 {
		t.Errorf("Expected 0 scenes, got %d", len(export.Scenes))
	}
}

// --- Import Tests ---

func TestMemoryCommand_Import_Success(t *testing.T) {
	cmd, cellRepo, sceneRepo, buf := newTestMemoryCommand()

	now := time.Now().Format(time.RFC3339)
	data := memoryExportData{
		Version:        1,
		ConversationID: "telegram:user123",
		ExportedAt:     now,
		CellCount:      1,
		SceneCount:     1,
		Cells: []exportCell{
			{
				ID:             "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				ConversationID: "telegram:user123",
				Scene:          "project-setup",
				Type:           "fact",
				Salience:       0.85,
				Content:        "Go project",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
		Scenes: []exportScene{
			{
				Scene:      "project-setup",
				Summary:    "Project setup summary",
				TokenCount: 10,
				UpdatedAt:  now,
			},
		},
	}

	jsonData, _ := json.Marshal(data)
	reader := bytes.NewReader(jsonData)

	err := cmd.Import(context.Background(), reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Imported 1 cell(s) and 1 scene(s)") {
		t.Errorf("Expected import summary, got: %s", output)
	}

	// Verify cell was created
	if len(cellRepo.cells) != 1 {
		t.Errorf("Expected 1 cell in repo, got %d", len(cellRepo.cells))
	}

	// Verify scene was upserted
	if len(sceneRepo.scenes) != 1 {
		t.Errorf("Expected 1 scene in repo, got %d", len(sceneRepo.scenes))
	}
}

func TestMemoryCommand_Import_InvalidJSON(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	reader := bytes.NewReader([]byte("not json"))
	err := cmd.Import(context.Background(), reader)
	if err == nil {
		t.Fatal("Import() should error on invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid import data") {
		t.Errorf("Expected 'invalid import data' error, got: %v", err)
	}
}

func TestMemoryCommand_Import_UnsupportedVersion(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	data := memoryExportData{Version: 99}
	jsonData, _ := json.Marshal(data)
	reader := bytes.NewReader(jsonData)

	err := cmd.Import(context.Background(), reader)
	if err == nil {
		t.Fatal("Import() should error on unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported export version") {
		t.Errorf("Expected version error, got: %v", err)
	}
}

func TestMemoryCommand_Import_SkipsDuplicates(t *testing.T) {
	cmd, cellRepo, _, buf := newTestMemoryCommand()

	// Pre-populate a cell to trigger duplicate detection
	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project", 0.85, memoryv2.CellTypeFact)
	cellRepo.cells[cell.ID] = cell

	now := time.Now().Format(time.RFC3339)
	data := memoryExportData{
		Version:        1,
		ConversationID: "telegram:user123",
		ExportedAt:     now,
		CellCount:      1,
		Cells: []exportCell{
			{
				ID:             "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				ConversationID: "telegram:user123",
				Scene:          "project-setup",
				Type:           "fact",
				Salience:       0.85,
				Content:        "Go project",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}

	jsonData, _ := json.Marshal(data)
	reader := bytes.NewReader(jsonData)

	err := cmd.Import(context.Background(), reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if !strings.Contains(buf.String(), "Skipped 1") {
		t.Errorf("Expected 'Skipped 1' message, got: %s", buf.String())
	}
}

func TestMemoryCommand_Import_InvalidCellType(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()

	now := time.Now().Format(time.RFC3339)
	data := memoryExportData{
		Version:        1,
		ConversationID: "telegram:user123",
		ExportedAt:     now,
		CellCount:      1,
		Cells: []exportCell{
			{
				ID:             "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				ConversationID: "telegram:user123",
				Scene:          "project-setup",
				Type:           "invalid_type",
				Salience:       0.85,
				Content:        "Go project",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}

	jsonData, _ := json.Marshal(data)
	reader := bytes.NewReader(jsonData)

	err := cmd.Import(context.Background(), reader)
	if err != nil {
		t.Fatalf("Import() should not return error for invalid cells, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Warning") {
		t.Errorf("Expected warning for invalid cell, got: %s", output)
	}
	if !strings.Contains(output, "Skipped 1") {
		t.Errorf("Expected 'Skipped 1', got: %s", output)
	}
}

// --- RebuildFTS Tests ---

func TestMemoryCommand_RebuildFTS_Success(t *testing.T) {
	cmd, _, _, buf := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{})

	err := cmd.RebuildFTS(context.Background())
	if err != nil {
		t.Fatalf("RebuildFTS() error = %v", err)
	}

	if !strings.Contains(buf.String(), "rebuilt successfully") {
		t.Errorf("Expected success message, got: %s", buf.String())
	}
}

func TestMemoryCommand_RebuildFTS_AdminNotSet(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()

	err := cmd.RebuildFTS(context.Background())
	if err == nil {
		t.Fatal("RebuildFTS() should error when admin not set")
	}
}

func TestMemoryCommand_RebuildFTS_Error(t *testing.T) {
	cmd, _, _, _ := newTestMemoryCommand()
	cmd.SetAdmin(&mockMemoryAdmin{
		rebuildErr: errors.New("FTS table corrupted"),
	})

	err := cmd.RebuildFTS(context.Background())
	if err == nil {
		t.Fatal("RebuildFTS() should return error on failure")
	}
	if !strings.Contains(err.Error(), "FTS table corrupted") {
		t.Errorf("Expected wrapped error, got: %v", err)
	}
}

// --- Export/Import Round-Trip Test ---

func TestMemoryCommand_ExportImportRoundTrip(t *testing.T) {
	// Step 1: Export
	exportCmd, exportCellRepo, exportSceneRepo, exportBuf := newTestMemoryCommand()

	cell := testCell("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "project-setup", "Go project", 0.85, memoryv2.CellTypeFact)
	cell.ConversationID = "telegram:user123"
	cell.Source = `["msg-1"]`
	exportCellRepo.listResults = []*memoryv2.MemoryCell{cell}

	scene := testScene("project-setup", "Project setup summary", 10)
	exportSceneRepo.scenes["project-setup"] = scene

	err := exportCmd.Export(context.Background(), "telegram:user123")
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Step 2: Import into a fresh repository
	importCmd, importCellRepo, importSceneRepo, importBuf := newTestMemoryCommand()
	reader := bytes.NewReader(exportBuf.Bytes())

	err = importCmd.Import(context.Background(), reader)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	// Verify cell was imported
	if len(importCellRepo.cells) != 1 {
		t.Fatalf("Expected 1 imported cell, got %d", len(importCellRepo.cells))
	}

	importedCell, exists := importCellRepo.cells["aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"]
	if !exists {
		t.Fatal("Expected cell to be imported with correct ID")
	}

	if importedCell.Scene != "project-setup" {
		t.Errorf("Expected scene 'project-setup', got %q", importedCell.Scene)
	}
	if importedCell.Content != "Go project" {
		t.Errorf("Expected content 'Go project', got %q", importedCell.Content)
	}
	if importedCell.CellType != memoryv2.CellTypeFact {
		t.Errorf("Expected cell type 'fact', got %q", importedCell.CellType.String())
	}

	// Verify scene was imported
	if len(importSceneRepo.scenes) != 1 {
		t.Fatalf("Expected 1 imported scene, got %d", len(importSceneRepo.scenes))
	}

	importedScene, exists := importSceneRepo.scenes["project-setup"]
	if !exists {
		t.Fatal("Expected scene to be imported")
	}
	if importedScene.Summary != "Project setup summary" {
		t.Errorf("Expected summary, got %q", importedScene.Summary)
	}

	if !strings.Contains(importBuf.String(), "Imported 1 cell(s) and 1 scene(s)") {
		t.Errorf("Expected import summary, got: %s", importBuf.String())
	}
}

// --- formatBytes Tests ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
		}
	}
}
