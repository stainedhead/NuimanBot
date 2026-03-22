package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/health"
)

// mockLLMSvc is a minimal domain.LLMService implementation for testing.
type mockLLMSvc struct{}

func (m *mockLLMSvc) Complete(_ context.Context, _ domain.LLMProvider, _ *domain.LLMRequest) (*domain.LLMResponse, error) {
	return &domain.LLMResponse{}, nil
}

func (m *mockLLMSvc) Stream(_ context.Context, _ domain.LLMProvider, _ *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockLLMSvc) ListModels(_ context.Context, _ domain.LLMProvider) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{}, nil
}

// --- DefaultHealthChecker: CheckDatabase ---

func TestDefaultHealthChecker_CheckDatabase_Nil(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")
	if checker.CheckDatabase() {
		t.Error("expected false for nil db")
	}
}

// --- DefaultHealthChecker: CheckLLM ---

func TestDefaultHealthChecker_CheckLLM_NilService(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")
	if checker.CheckLLM() {
		t.Error("expected false for nil LLM service")
	}
}

func TestDefaultHealthChecker_CheckLLM_NonNilService(t *testing.T) {
	checker := health.NewHealthChecker(nil, &mockLLMSvc{}, "")
	if !checker.CheckLLM() {
		t.Error("expected true for non-nil LLM service")
	}
}

// --- DefaultHealthChecker: CheckVault ---

func TestDefaultHealthChecker_CheckVault_EmptyPath(t *testing.T) {
	checker := health.NewHealthChecker(nil, nil, "")
	if checker.CheckVault() {
		t.Error("expected false for empty vault path")
	}
}

func TestDefaultHealthChecker_CheckVault_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "nonexistent.enc")
	checker := health.NewHealthChecker(nil, nil, vaultPath)
	if !checker.CheckVault() {
		t.Error("expected true for non-existent file (initial setup)")
	}
}

func TestDefaultHealthChecker_CheckVault_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	vaultPath := filepath.Join(tmpDir, "vault.enc")
	if err := os.WriteFile(vaultPath, []byte("data"), 0o600); err != nil {
		t.Fatalf("write vault file: %v", err)
	}
	checker := health.NewHealthChecker(nil, nil, vaultPath)
	if !checker.CheckVault() {
		t.Error("expected true for existing readable vault file")
	}
}

func TestDefaultHealthChecker_CheckVault_IsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	checker := health.NewHealthChecker(nil, nil, tmpDir)
	if checker.CheckVault() {
		t.Error("expected false when vault path is a directory")
	}
}

// --- MockHealthChecks ---

func TestMockHealthChecks_AllTrue(t *testing.T) {
	m := &health.MockHealthChecks{
		DatabaseHealthy: true,
		LLMHealthy:      true,
		VaultHealthy:    true,
	}
	if !m.CheckDatabase() {
		t.Error("expected CheckDatabase true")
	}
	if !m.CheckLLM() {
		t.Error("expected CheckLLM true")
	}
	if !m.CheckVault() {
		t.Error("expected CheckVault true")
	}
}

func TestMockHealthChecks_AllFalse(t *testing.T) {
	m := &health.MockHealthChecks{
		DatabaseHealthy: false,
		LLMHealthy:      false,
		VaultHealthy:    false,
	}
	if m.CheckDatabase() {
		t.Error("expected CheckDatabase false")
	}
	if m.CheckLLM() {
		t.Error("expected CheckLLM false")
	}
	if m.CheckVault() {
		t.Error("expected CheckVault false")
	}
}

// --- Server.Start / Stop ---

func TestServer_StartAndStop(t *testing.T) {
	// Find a free port.
	port := freePort(t)
	addr := ":" + itoa(port)

	s := health.NewServer(nil, nil, "")
	if err := s.Start(addr); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give the server a small amount of time to bind.
	// We test by stopping it immediately; Stop should not error.
	if err := s.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestServer_StopWithoutStart(t *testing.T) {
	s := health.NewServer(nil, nil, "")
	// Stop on a server that was never started should not error.
	if err := s.Stop(); err != nil {
		t.Errorf("Stop without Start returned error: %v", err)
	}
}

// --- Server.NewServer with deps ---

func TestNewServer_WithDeps(t *testing.T) {
	// Providing non-nil deps should create a real DefaultHealthChecker.
	s := health.NewServer(nil, &mockLLMSvc{}, "")
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithVaultPath(t *testing.T) {
	s := health.NewServer(nil, nil, "/tmp/vault.enc")
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

// --- checkStatus (via Liveness with checker) ---

func TestLiveness_WithChecks_AllHealthy(t *testing.T) {
	checks := &health.MockHealthChecks{
		DatabaseHealthy: true,
		LLMHealthy:      true,
		VaultHealthy:    true,
	}
	s := health.NewServer(nil, nil, "")
	s.SetHealthChecks(checks)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Liveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLiveness_WithChecks_Unhealthy(t *testing.T) {
	checks := &health.MockHealthChecks{
		DatabaseHealthy: false,
		LLMHealthy:      false,
		VaultHealthy:    false,
	}
	s := health.NewServer(nil, nil, "")
	s.SetHealthChecks(checks)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Liveness(w, req)

	// Liveness always returns 200 (the server is alive even if deps are unhealthy)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from Liveness regardless of check status, got %d", w.Code)
	}
}

// --- Server.addStorageMetrics via Liveness with invalid dataDir ---

func TestLiveness_BadDataDir_DoesNotPanic(t *testing.T) {
	// A data directory that exists but has no valid storage structure causes
	// GetStorageMetrics to fail — addStorageMetrics should log a warning and
	// continue, so Liveness still returns 200.
	s := health.NewServer(nil, nil, "")
	s.SetDataDirectory(t.TempDir()) // empty dir → metrics collection will error

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Liveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even with bad data dir, got %d", w.Code)
	}
}

// --- Version with default (no SetVersion call) ---

func TestVersion_DefaultVersion(t *testing.T) {
	s := health.NewServer(nil, nil, "")
	// Do not call SetVersion — exercises the default "1.0.0" branch.
	req := httptest.NewRequest(http.MethodGet, "/health/version", nil)
	w := httptest.NewRecorder()
	s.Version(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// itoa converts an int to a string without importing strconv at package level.
func itoa(n int) string {
	return intToStr(n)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
