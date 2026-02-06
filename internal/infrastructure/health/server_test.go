package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/infrastructure/health"
)

func TestLiveness(t *testing.T) {
	server := health.NewServer(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.Liveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
}

func TestReadiness_AllHealthy(t *testing.T) {
	checks := &health.MockHealthChecks{
		DatabaseHealthy: true,
		LLMHealthy:      true,
		VaultHealthy:    true,
	}

	server := health.NewServer(nil, nil, "")
	server.SetHealthChecks(checks)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	server.Readiness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ready" {
		t.Errorf("Expected status 'ready', got '%v'", response["status"])
	}

	checksMap, ok := response["checks"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected checks map in response")
	}

	if checksMap["database"] != true {
		t.Error("Expected database check to be true")
	}
	if checksMap["llm"] != true {
		t.Error("Expected llm check to be true")
	}
	if checksMap["vault"] != true {
		t.Error("Expected vault check to be true")
	}
}

func TestReadiness_DatabaseUnhealthy(t *testing.T) {
	checks := &health.MockHealthChecks{
		DatabaseHealthy: false,
		LLMHealthy:      true,
		VaultHealthy:    true,
	}

	server := health.NewServer(nil, nil, "")
	server.SetHealthChecks(checks)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	server.Readiness(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "not_ready" {
		t.Errorf("Expected status 'not_ready', got '%v'", response["status"])
	}
}

func TestReadiness_MultipleUnhealthy(t *testing.T) {
	checks := &health.MockHealthChecks{
		DatabaseHealthy: false,
		LLMHealthy:      false,
		VaultHealthy:    true,
	}

	server := health.NewServer(nil, nil, "")
	server.SetHealthChecks(checks)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	server.Readiness(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	checksMap, ok := response["checks"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected checks map in response")
	}

	if checksMap["database"] != false {
		t.Error("Expected database check to be false")
	}
	if checksMap["llm"] != false {
		t.Error("Expected llm check to be false")
	}
}

func TestVersion(t *testing.T) {
	version := "1.0.0"
	server := health.NewServer(nil, nil, "")
	server.SetVersion(version)

	req := httptest.NewRequest(http.MethodGet, "/health/version", nil)
	w := httptest.NewRecorder()

	server.Version(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["version"] != version {
		t.Errorf("Expected version '%s', got '%s'", version, response["version"])
	}
}

func TestServer_Start(t *testing.T) {
	server := health.NewServer(nil, nil, "")

	// Test that server can be created and routes registered
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Test liveness route
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Liveness route failed: expected 200, got %d", w.Code)
	}

	// Test readiness route
	req = httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Should work even without health checks set (defaults to unhealthy)
	if w.Code == 0 {
		t.Error("Readiness route not registered")
	}

	// Test version route
	req = httptest.NewRequest(http.MethodGet, "/health/version", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Version route failed: expected 200, got %d", w.Code)
	}
}

func TestReadiness_NoChecksSet(t *testing.T) {
	server := health.NewServer(nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	server.Readiness(w, req)

	// Should return not_ready when no checks are set
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 when no checks set, got %d", w.Code)
	}
}
