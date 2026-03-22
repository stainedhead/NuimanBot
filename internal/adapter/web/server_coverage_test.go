package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetAddr verifies GetAddr returns the configured address.
func TestGetAddr(t *testing.T) {
	server := NewServer(":9876")
	if got := server.GetAddr(); got != ":9876" {
		t.Errorf("GetAddr() = %q, want %q", got, ":9876")
	}
}

// TestGetAuth verifies GetAuth returns the auth service.
func TestGetAuth(t *testing.T) {
	server := NewServer(":0")
	auth := server.GetAuth()
	if auth == nil {
		t.Error("GetAuth() returned nil, expected non-nil AuthService")
	}
}

// TestHandleRootRedirect_NotFound verifies non-root paths return 404.
func TestHandleRootRedirect_NotFound(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/notroot", nil)
	w := httptest.NewRecorder()

	server.handleRootRedirect(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-root path, got %d", w.Code)
	}
}

// TestHandleRootRedirect_Root verifies / redirects.
func TestHandleRootRedirect_Root(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleRootRedirect(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
}

// TestHandleHealth_ResponseContent verifies the health endpoint body includes status.
func TestHandleHealth_ResponseContent(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty health response body")
	}
}

// TestError404 verifies Error404 writes 404 status.
func TestError404(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()

	server.Error404(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestError500 verifies Error500 writes 500 status.
func TestError500(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	w := httptest.NewRecorder()

	server.Error500(w, req, nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestWithFlashSuccess verifies WithFlashSuccess sets the flash message.
func TestWithFlashSuccess(t *testing.T) {
	bd := NewBaseData("Test", "test")
	bd = bd.WithFlashSuccess("all good")
	if bd.FlashSuccess != "all good" {
		t.Errorf("expected FlashSuccess='all good', got %q", bd.FlashSuccess)
	}
}

// TestWithFlashError verifies WithFlashError sets the flash message.
func TestWithFlashError(t *testing.T) {
	bd := NewBaseData("Test", "test")
	bd = bd.WithFlashError("something failed")
	if bd.FlashError != "something failed" {
		t.Errorf("expected FlashError='something failed', got %q", bd.FlashError)
	}
}

// TestNewBaseData verifies NewBaseData sets defaults.
func TestNewBaseData(t *testing.T) {
	bd := NewBaseData("My Title", "home")
	if bd.Title != "My Title" {
		t.Errorf("expected title 'My Title', got %q", bd.Title)
	}
	if bd.ActivePage != "home" {
		t.Errorf("expected activePage 'home', got %q", bd.ActivePage)
	}
	if bd.IsAuthenticated {
		t.Error("expected IsAuthenticated=false by default")
	}
	if bd.Version != defaultVersion {
		t.Errorf("expected version %q, got %q", defaultVersion, bd.Version)
	}
}

// TestWithUser verifies WithUser marks as authenticated.
func TestWithUser(t *testing.T) {
	bd := NewBaseData("Test", "test")
	u := &User{ID: "1", Username: "bob", Role: "admin"}
	bd = bd.WithUser(u)
	if !bd.IsAuthenticated {
		t.Error("expected IsAuthenticated=true after WithUser")
	}
	if bd.CurrentUser != u {
		t.Error("expected CurrentUser to be set")
	}
}
