package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/infrastructure/storage"
)

// TestGetStorageMetrics_EmptyDirectory verifies metrics collection on
// a freshly initialized (empty) data directory
func TestGetStorageMetrics_EmptyDirectory(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", metrics.Status)
	}

	if metrics.DataDirectory != tempDir {
		t.Errorf("Expected data_directory '%s', got '%s'", tempDir, metrics.DataDirectory)
	}

	if metrics.TotalSizeMB < 0 {
		t.Errorf("Expected non-negative total_size_mb, got %f", metrics.TotalSizeMB)
	}

	if metrics.DiskAvailableGB <= 0 {
		t.Errorf("Expected positive disk_available_gb, got %f", metrics.DiskAvailableGB)
	}

	if metrics.DiskUsedPercent < 0 || metrics.DiskUsedPercent > 100 {
		t.Errorf("Expected disk_used_percent in [0, 100], got %f", metrics.DiskUsedPercent)
	}

	if !metrics.Writable {
		t.Error("Expected writable to be true for temp directory")
	}
}

// TestGetStorageMetrics_CountsUsers verifies that user count is
// correctly derived from users.json
func TestGetStorageMetrics_CountsUsers(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Act - default admin user was created by Initialize
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Users.TotalCount != 1 {
		t.Errorf("Expected 1 user (default admin), got %d", metrics.Users.TotalCount)
	}

	if metrics.Users.ActiveCount != 1 {
		t.Errorf("Expected 1 active user (default admin), got %d", metrics.Users.ActiveCount)
	}
}

// TestGetStorageMetrics_CountsMultipleUsers verifies accurate user counting
// with multiple users in users.json
func TestGetStorageMetrics_CountsMultipleUsers(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Add more users to users.json
	usersFile := filepath.Join(tempDir, "users.json")
	usersData := `{
		"version": "1.0",
		"lastUpdated": "2026-02-16T12:00:00Z",
		"users": [
			{"userID": "admin", "moniker": "Admin", "primaryEmail": "admin@localhost", "role": "admin", "userType": "admin", "enabled": true},
			{"userID": "user1", "moniker": "User1", "primaryEmail": "user1@test.com", "role": "user", "userType": "standard", "enabled": true},
			{"userID": "user2", "moniker": "User2", "primaryEmail": "user2@test.com", "role": "user", "userType": "standard", "enabled": false}
		],
		"indexes": {
			"byUsername": {},
			"byEmail": {},
			"byAPIKey": {},
			"byPlatform": {"slack": {}, "telegram": {}, "cli": {}}
		}
	}`
	if err := os.WriteFile(usersFile, []byte(usersData), 0644); err != nil {
		t.Fatalf("Failed to write users.json: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Users.TotalCount != 3 {
		t.Errorf("Expected 3 total users, got %d", metrics.Users.TotalCount)
	}

	if metrics.Users.ActiveCount != 2 {
		t.Errorf("Expected 2 active users (admin + user1), got %d", metrics.Users.ActiveCount)
	}
}

// TestGetStorageMetrics_CountsConversations verifies conversation counting
// by scanning user conversation directories
func TestGetStorageMetrics_CountsConversations(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Create conversation directories for admin user
	convDir := filepath.Join(tempDir, "users", "admin", "conversations")
	if err := os.MkdirAll(convDir, 0755); err != nil {
		t.Fatalf("Failed to create conversations dir: %v", err)
	}

	// Create index file with conversations
	indexData := `{
		"version": "1.0",
		"lastUpdated": "2026-02-16T12:00:00Z",
		"conversations": {
			"conv1": {"id": "conv1", "userID": "admin"},
			"conv2": {"id": "conv2", "userID": "admin"}
		}
	}`
	if err := os.WriteFile(filepath.Join(convDir, "index.json"), []byte(indexData), 0644); err != nil {
		t.Fatalf("Failed to write conversation index: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Data.Conversations != 2 {
		t.Errorf("Expected 2 conversations, got %d", metrics.Data.Conversations)
	}
}

// TestGetStorageMetrics_CountsNotes verifies note counting
func TestGetStorageMetrics_CountsNotes(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Create notes directory for admin user
	notesDir := filepath.Join(tempDir, "users", "admin", "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatalf("Failed to create notes dir: %v", err)
	}

	// Create note files
	for _, noteID := range []string{"note1", "note2", "note3"} {
		noteFile := filepath.Join(notesDir, noteID+".json")
		if err := os.WriteFile(noteFile, []byte(`{"id":"`+noteID+`"}`), 0644); err != nil {
			t.Fatalf("Failed to create note file: %v", err)
		}
	}

	// Create index.json (should NOT be counted as a note)
	indexFile := filepath.Join(notesDir, "index.json")
	if err := os.WriteFile(indexFile, []byte(`{}`), 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Data.Notes != 3 {
		t.Errorf("Expected 3 notes, got %d", metrics.Data.Notes)
	}
}

// TestGetStorageMetrics_CountsMemoryCells verifies memory cell counting
func TestGetStorageMetrics_CountsMemoryCells(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Create memory cells directory
	cellsDir := filepath.Join(tempDir, "memory", "cells")
	if err := os.MkdirAll(cellsDir, 0755); err != nil {
		t.Fatalf("Failed to create cells dir: %v", err)
	}

	// Create cell files
	for _, cellID := range []string{"cell1", "cell2"} {
		cellFile := filepath.Join(cellsDir, cellID+".json")
		if err := os.WriteFile(cellFile, []byte(`{"id":"`+cellID+`"}`), 0644); err != nil {
			t.Fatalf("Failed to create cell file: %v", err)
		}
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Data.MemoryCells != 2 {
		t.Errorf("Expected 2 memory cells, got %d", metrics.Data.MemoryCells)
	}
}

// TestGetStorageMetrics_MissingDirectory verifies error handling for
// a non-existent data directory
func TestGetStorageMetrics_MissingDirectory(t *testing.T) {
	// Act
	metrics, err := storage.GetStorageMetrics("/nonexistent/path/that/does/not/exist")

	// Assert
	if err != nil {
		t.Fatalf("Expected no error for missing directory, got: %v", err)
	}

	if metrics.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy' for missing directory, got '%s'", metrics.Status)
	}

	if metrics.Writable {
		t.Error("Expected writable to be false for missing directory")
	}
}

// TestGetStorageMetrics_EmptyBaseDir verifies error handling for empty base dir
func TestGetStorageMetrics_EmptyBaseDir(t *testing.T) {
	// Act
	_, err := storage.GetStorageMetrics("")

	// Assert
	if err == nil {
		t.Error("Expected error for empty base directory, got nil")
	}
}

// TestGetStorageMetrics_JSONSerializable verifies that StorageMetrics
// can be serialized to the expected JSON structure
func TestGetStorageMetrics_JSONSerializable(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	if err := storage.Initialize(tempDir); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	// Serialize to JSON
	data, err := json.Marshal(metrics)
	if err != nil {
		t.Fatalf("Failed to marshal metrics to JSON: %v", err)
	}

	// Deserialize and verify structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal metrics JSON: %v", err)
	}

	// Verify top-level fields exist
	requiredFields := []string{"status", "data_directory", "total_size_mb", "disk_available_gb", "disk_used_percent", "writable"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field '%s' in JSON output", field)
		}
	}

	// Verify nested fields
	users, ok := result["users"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'users' to be an object")
	}
	if _, ok := users["total_count"]; !ok {
		t.Error("Expected 'total_count' in users object")
	}
	if _, ok := users["active_count"]; !ok {
		t.Error("Expected 'active_count' in users object")
	}

	dataObj, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'data' to be an object")
	}
	if _, ok := dataObj["conversations"]; !ok {
		t.Error("Expected 'conversations' in data object")
	}
	if _, ok := dataObj["notes"]; !ok {
		t.Error("Expected 'notes' in data object")
	}
	if _, ok := dataObj["memory_cells"]; !ok {
		t.Error("Expected 'memory_cells' in data object")
	}
}

// TestGetStorageMetrics_NoUsersFile verifies graceful handling when
// users.json is missing
func TestGetStorageMetrics_NoUsersFile(t *testing.T) {
	// Arrange - create directory structure without users.json
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "users"), 0755); err != nil {
		t.Fatalf("Failed to create users dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "system"), 0755); err != nil {
		t.Fatalf("Failed to create system dir: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Users.TotalCount != 0 {
		t.Errorf("Expected 0 users when users.json is missing, got %d", metrics.Users.TotalCount)
	}

	if metrics.Users.ActiveCount != 0 {
		t.Errorf("Expected 0 active users when users.json is missing, got %d", metrics.Users.ActiveCount)
	}
}

// TestGetStorageMetrics_ZeroCounts verifies that counts are zero when
// no data exists
func TestGetStorageMetrics_ZeroCounts(t *testing.T) {
	// Arrange - create directory structure without any data
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "users"), 0755); err != nil {
		t.Fatalf("Failed to create users dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "system"), 0755); err != nil {
		t.Fatalf("Failed to create system dir: %v", err)
	}

	// Act
	metrics, err := storage.GetStorageMetrics(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("GetStorageMetrics() failed: %v", err)
	}

	if metrics.Data.Conversations != 0 {
		t.Errorf("Expected 0 conversations, got %d", metrics.Data.Conversations)
	}
	if metrics.Data.Notes != 0 {
		t.Errorf("Expected 0 notes, got %d", metrics.Data.Notes)
	}
	if metrics.Data.MemoryCells != 0 {
		t.Errorf("Expected 0 memory cells, got %d", metrics.Data.MemoryCells)
	}
}
