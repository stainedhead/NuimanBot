package health_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/health"
)

func TestDefaultHealthChecker_CheckDatabase_Success(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	checker := health.NewHealthChecker(db, nil, "")

	if !checker.CheckDatabase() {
		t.Error("Expected database check to pass with valid connection")
	}
}

func TestDefaultHealthChecker_CheckDatabase_Nil(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")

	if checker.CheckDatabase() {
		t.Error("Expected database check to fail with nil connection")
	}
}

func TestDefaultHealthChecker_CheckDatabase_Closed(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	db.Close() // Close immediately

	checker := health.NewHealthChecker(db, nil, "")

	if checker.CheckDatabase() {
		t.Error("Expected database check to fail with closed connection")
	}
}

func TestDefaultHealthChecker_CheckLLM_Success(t *testing.T) {
	// Mock LLM service
	mockLLM := &mockLLMService{}

	checker := health.NewHealthChecker(nil, mockLLM, "")

	if !checker.CheckLLM() {
		t.Error("Expected LLM check to pass with valid service")
	}
}

func TestDefaultHealthChecker_CheckLLM_Nil(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")

	if checker.CheckLLM() {
		t.Error("Expected LLM check to fail with nil service")
	}
}

func TestDefaultHealthChecker_CheckVault_FileExists(t *testing.T) {
	// Create temporary vault file
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "vault.enc")

	if err := os.WriteFile(vaultPath, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create test vault file: %v", err)
	}

	checker := health.NewHealthChecker(nil, nil, vaultPath)

	if !checker.CheckVault() {
		t.Error("Expected vault check to pass with existing file")
	}
}

func TestDefaultHealthChecker_CheckVault_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "nonexistent.enc")

	checker := health.NewHealthChecker(nil, nil, vaultPath)

	// Non-existent file is OK for initial setup
	if !checker.CheckVault() {
		t.Error("Expected vault check to pass for non-existent file (initial setup)")
	}
}

func TestDefaultHealthChecker_CheckVault_EmptyPath(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")

	if checker.CheckVault() {
		t.Error("Expected vault check to fail with empty path")
	}
}

func TestDefaultHealthChecker_CheckVault_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	checker := health.NewHealthChecker(nil, nil, tmpDir)

	if checker.CheckVault() {
		t.Error("Expected vault check to fail when path is a directory")
	}
}

func TestDefaultHealthChecker_CheckVault_NotReadable(t *testing.T) {
	// Create temporary vault file with no read permissions
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "vault.enc")

	if err := os.WriteFile(vaultPath, []byte("test"), 0000); err != nil {
		t.Fatalf("Failed to create test vault file: %v", err)
	}
	defer os.Chmod(vaultPath, 0600) // Restore for cleanup

	checker := health.NewHealthChecker(nil, nil, vaultPath)

	if checker.CheckVault() {
		t.Error("Expected vault check to fail with unreadable file")
	}
}

func TestMockHealthChecks(t *testing.T) {
	mock := &health.MockHealthChecks{
		DatabaseHealthy: true,
		LLMHealthy:      false,
		VaultHealthy:    true,
	}

	if !mock.CheckDatabase() {
		t.Error("Expected mock database check to return true")
	}

	if mock.CheckLLM() {
		t.Error("Expected mock LLM check to return false")
	}

	if !mock.CheckVault() {
		t.Error("Expected mock vault check to return true")
	}
}

// mockLLMService is a minimal mock for testing
type mockLLMService struct{}

func (m *mockLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	return &domain.LLMResponse{}, nil
}

func (m *mockLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{}, nil
}
