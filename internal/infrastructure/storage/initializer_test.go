package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/infrastructure/storage"
)

// TestInitialize_CreatesRequiredDirectories verifies that Initialize creates
// the required directory structure
func TestInitialize_CreatesRequiredDirectories(t *testing.T) {
	// Arrange
	tempDir := t.TempDir() // Automatically cleaned up after test

	// Act
	err := storage.Initialize(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Verify directories exist
	expectedDirs := []string{
		filepath.Join(tempDir, "users"),
		filepath.Join(tempDir, "system"),
	}

	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			t.Errorf("Expected directory %s does not exist", dir)
			continue
		}
		if err != nil {
			t.Errorf("Failed to stat directory %s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Path %s is not a directory", dir)
		}
	}
}

// TestInitialize_IsIdempotent verifies that Initialize can be called multiple
// times safely without errors
func TestInitialize_IsIdempotent(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Act - call Initialize twice
	err1 := storage.Initialize(tempDir)
	err2 := storage.Initialize(tempDir)

	// Assert
	if err1 != nil {
		t.Fatalf("First Initialize() failed: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Second Initialize() failed: %v", err2)
	}
}

// TestInitialize_HandlesPermissionErrors verifies graceful error handling
// when directories cannot be created due to permission issues
func TestInitialize_HandlesPermissionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// Arrange - create a directory with no write permissions
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("Failed to create readonly directory: %v", err)
	}

	// Act
	err := storage.Initialize(readOnlyDir)

	// Assert - should return an error
	if err == nil {
		t.Error("Expected Initialize() to fail with permission error, but it succeeded")
	}
}

// TestInitialize_CreatesDefaultAdmin verifies that the default admin user
// is created on first initialization
func TestInitialize_CreatesDefaultAdmin(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Act
	err := storage.Initialize(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Verify default admin user file exists
	adminFile := filepath.Join(tempDir, "users.json")
	if _, err := os.Stat(adminFile); os.IsNotExist(err) {
		t.Error("Expected users.json file to be created, but it doesn't exist")
	}
}

// TestInitialize_DoesNotRecreateAdminIfExists verifies that Initialize
// does not recreate the admin user if it already exists
func TestInitialize_DoesNotRecreateAdminIfExists(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Create users.json with existing data
	usersFile := filepath.Join(tempDir, "users.json")
	existingData := `{
		"version": "1.0",
		"lastUpdated": "2026-02-16T12:00:00Z",
		"users": [{"userID": "test-user", "username": "testuser"}],
		"indexes": {
			"byUsername": {"testuser": "test-user"},
			"byEmail": {},
			"byAPIKey": {},
			"byPlatform": {"slack": {}, "telegram": {}, "cli": {}}
		}
	}`
	if err := os.WriteFile(usersFile, []byte(existingData), 0644); err != nil {
		t.Fatalf("Failed to create test users.json: %v", err)
	}

	// Act
	err := storage.Initialize(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Verify existing data is preserved
	data, err := os.ReadFile(usersFile)
	if err != nil {
		t.Fatalf("Failed to read users.json: %v", err)
	}

	// Should still contain the test user
	if !contains(string(data), "test-user") {
		t.Error("Expected Initialize() to preserve existing users, but data was overwritten")
	}
}

// TestEnsureDirectories_CreatesAllRequiredDirs verifies directory creation
func TestEnsureDirectories_CreatesAllRequiredDirs(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Act
	err := storage.EnsureDirectories(tempDir)

	// Assert
	if err != nil {
		t.Fatalf("EnsureDirectories() failed: %v", err)
	}

	// Verify directories exist with correct permissions
	dirs := []string{"users", "system"}
	for _, dir := range dirs {
		path := filepath.Join(tempDir, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Directory %s does not exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Path %s is not a directory", dir)
		}

		// Check permissions (0755)
		mode := info.Mode().Perm()
		expected := os.FileMode(0755)
		if mode != expected {
			t.Errorf("Directory %s has permissions %o, expected %o", dir, mode, expected)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestInitialize_InvalidBasePath verifies error handling for invalid paths
func TestInitialize_InvalidBasePath(t *testing.T) {
	// Act
	err := storage.Initialize("")

	// Assert
	if err == nil {
		t.Error("Expected Initialize() to fail with empty base path, but it succeeded")
	}
}
