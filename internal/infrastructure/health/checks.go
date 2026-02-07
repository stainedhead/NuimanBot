package health

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"nuimanbot/internal/domain"
)

// HealthChecker provides methods to check health of various dependencies.
type HealthChecker interface {
	CheckDatabase() bool
	CheckLLM() bool
	CheckVault() bool
}

// DefaultHealthChecker implements HealthChecker with actual dependency checks.
type DefaultHealthChecker struct {
	db         *sql.DB
	llmService domain.LLMService
	vaultPath  string
}

// NewHealthChecker creates a new health checker with actual dependencies.
func NewHealthChecker(db *sql.DB, llmService domain.LLMService, vaultPath string) *DefaultHealthChecker {
	return &DefaultHealthChecker{
		db:         db,
		llmService: llmService,
		vaultPath:  vaultPath,
	}
}

// CheckDatabase verifies database connectivity.
func (h *DefaultHealthChecker) CheckDatabase() bool {
	if h.db == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		slog.Warn("database health check failed", "error", err)
		return false
	}

	return true
}

// CheckLLM verifies LLM service availability.
func (h *DefaultHealthChecker) CheckLLM() bool {
	if h.llmService == nil {
		return false
	}

	// For now, just check if service exists
	// Could enhance with actual provider ping
	return true
}

// CheckVault verifies credential vault file exists and is accessible.
func (h *DefaultHealthChecker) CheckVault() bool {
	if h.vaultPath == "" {
		return false
	}

	info, err := os.Stat(h.vaultPath)
	if err != nil {
		// File doesn't exist yet - that's ok for initial setup
		if os.IsNotExist(err) {
			return true
		}
		slog.Warn("vault health check failed", "error", err)
		return false
	}

	// Check if it's a file and readable
	if info.IsDir() {
		return false
	}

	// Try to open for reading
	f, err := os.Open(h.vaultPath)
	if err != nil {
		slog.Warn("vault file not readable", "error", err)
		return false
	}
	_ = f.Close() //nolint:errcheck // Best effort cleanup in health check

	return true
}

// MockHealthChecks provides a mock implementation for testing.
type MockHealthChecks struct {
	DatabaseHealthy bool
	LLMHealthy      bool
	VaultHealthy    bool
}

// CheckDatabase returns the mocked database health status.
func (m *MockHealthChecks) CheckDatabase() bool {
	return m.DatabaseHealthy
}

// CheckLLM returns the mocked LLM health status.
func (m *MockHealthChecks) CheckLLM() bool {
	return m.LLMHealthy
}

// CheckVault returns the mocked vault health status.
func (m *MockHealthChecks) CheckVault() bool {
	return m.VaultHealthy
}
