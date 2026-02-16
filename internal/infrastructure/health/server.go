package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// Server provides HTTP endpoints for health checks.
type Server struct {
	server      *http.Server
	checker     HealthChecker
	version     string
	dataDir     string
	startTime   time.Time
	mu          sync.RWMutex
	shutdownCtx context.Context
	cancel      context.CancelFunc
}

// NewServer creates a new health check server.
func NewServer(db *sql.DB, llmService domain.LLMService, vaultPath string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		version:     "1.0.0", // Default version
		startTime:   time.Now(),
		shutdownCtx: ctx,
		cancel:      cancel,
	}

	// Create default health checker if dependencies provided
	if db != nil || llmService != nil || vaultPath != "" {
		s.checker = NewHealthChecker(db, llmService, vaultPath)
	}

	return s
}

// SetHealthChecks sets a custom health checker (useful for testing).
func (s *Server) SetHealthChecks(checker HealthChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checker = checker
}

// SetVersion sets the version string returned by the version endpoint.
func (s *Server) SetVersion(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
}

// SetDataDirectory sets the data directory path for storage metrics.
func (s *Server) SetDataDirectory(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataDir = dataDir
}

// Liveness returns 200 OK if the server is running (Kubernetes liveness probe).
// Enhanced to include storage metrics, version, and uptime information.
func (s *Server) Liveness(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	version := s.version
	dataDir := s.dataDir
	checker := s.checker
	startTime := s.startTime
	s.mu.RUnlock()

	response := s.buildHealthResponse(version, dataDir, checker, startTime)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode health response", "error", err)
	}
}

// buildHealthResponse constructs the comprehensive health response.
func (s *Server) buildHealthResponse(version, dataDir string, checker HealthChecker, startTime time.Time) map[string]interface{} {
	response := map[string]interface{}{
		"status":         "healthy",
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"version":        version,
		"uptime_seconds": int64(time.Since(startTime).Seconds()),
	}

	s.addStorageMetrics(response, dataDir)
	s.addHealthChecks(response, checker)

	return response
}

// addStorageMetrics adds storage metrics to the health response if available.
func (s *Server) addStorageMetrics(response map[string]interface{}, dataDir string) {
	if dataDir == "" {
		return
	}

	metrics, err := storage.GetStorageMetrics(dataDir)
	if err != nil {
		slog.Warn("Failed to collect storage metrics", "error", err)
		return
	}

	response["storage"] = metrics
	response["users"] = metrics.Users
	response["data"] = metrics.Data
}

// addHealthChecks adds health check results to the response.
func (s *Server) addHealthChecks(response map[string]interface{}, checker HealthChecker) {
	checks := []map[string]interface{}{}

	if checker != nil {
		checks = append(checks,
			map[string]interface{}{
				"name":   "database",
				"status": checkStatus(checker.CheckDatabase()),
			},
			map[string]interface{}{
				"name":   "llm",
				"status": checkStatus(checker.CheckLLM()),
			},
			map[string]interface{}{
				"name":   "vault",
				"status": checkStatus(checker.CheckVault()),
			},
		)
	}

	response["checks"] = checks
}

// checkStatus converts boolean health check result to status string.
func checkStatus(healthy bool) string {
	if healthy {
		return "ok"
	}
	return "unhealthy"
}

// Readiness checks if all dependencies are healthy (Kubernetes readiness probe).
func (s *Server) Readiness(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checker := s.checker
	s.mu.RUnlock()

	checks := map[string]bool{
		"database": false,
		"llm":      false,
		"vault":    false,
	}

	// If no checker is set, return not ready
	if checker != nil {
		checks["database"] = checker.CheckDatabase()
		checks["llm"] = checker.CheckLLM()
		checks["vault"] = checker.CheckVault()
	}

	allReady := true
	for _, ready := range checks {
		if !ready {
			allReady = false
			break
		}
	}

	status := "ready"
	statusCode := http.StatusOK
	if !allReady {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"checks": checks,
	})
}

// Version returns version information about the application.
func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	version := s.version
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"version": version,
	})
}

// RegisterRoutes registers health check and metrics routes on the provided mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.Liveness)
	mux.HandleFunc("/health/ready", s.Readiness)
	mux.HandleFunc("/health/version", s.Version)
	mux.Handle("/metrics", promhttp.Handler()) // Prometheus metrics endpoint
}

// Start starts the health check HTTP server on the specified port.
func (s *Server) Start(port string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	s.server = &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Starting health check server", "port", port)

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health check server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the health check server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	slog.Info("Stopping health check server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}

	s.cancel()
	return nil
}
