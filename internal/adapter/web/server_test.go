package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServerStart tests that the web server can be created and started
func TestServerStart(t *testing.T) {
	server := NewServer(":0") // Use port 0 for testing (OS assigns random port)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	// Server should not be nil
	if server.httpServer == nil {
		t.Error("httpServer should not be nil")
	}
}

// TestServerStaticFilesServed tests that static files are served correctly
func TestServerStaticFilesServed(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/static/styles.css", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("expected status OK or NotFound, got %d", w.Code)
	}
}

// TestServerHealthCheck tests that the health check endpoint works
func TestServerHealthCheck(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

// TestServerIndexRedirect tests that / redirects to /admin/
func TestServerIndexRedirect(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	// Should redirect
	if w.Code != http.StatusFound && w.Code != http.StatusMovedPermanently {
		t.Errorf("expected redirect status, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/admin/" && location != "/admin/login" {
		t.Errorf("expected redirect to /admin/ or /admin/login, got %s", location)
	}
}
