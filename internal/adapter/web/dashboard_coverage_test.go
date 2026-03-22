package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHandleReloadConfig_RequiresAuth verifies redirect when unauthenticated.
func TestHandleReloadConfig_RequiresAuth(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/reload", nil)
	w := httptest.NewRecorder()

	server.handleReloadConfig(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect (302), got %d", w.Code)
	}
}

// TestHandleReloadConfig_NonAdminForbidden verifies 403 for non-admin users.
func TestHandleReloadConfig_NonAdminForbidden(t *testing.T) {
	server := NewServer(":0")
	auth := server.auth

	if err := auth.AddUser("regular", "pass", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	sessionID := auth.CreateSession("regular", "user")

	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/reload", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleReloadConfig(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestHandleReloadConfig_Success verifies admin can reload config.
func TestHandleReloadConfig_Success(t *testing.T) {
	server, sessionID := newAuthenticatedServer(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/reload", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	w := httptest.NewRecorder()

	server.handleReloadConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "success") {
		t.Error("expected body to contain 'success'")
	}
}

// TestFormatDuration_Seconds verifies seconds-only formatting.
func TestFormatDuration_Seconds(t *testing.T) {
	result := formatDuration(45 * time.Second)
	if result != "45s" {
		t.Errorf("expected '45s', got %q", result)
	}
}

// TestFormatDuration_Minutes verifies minutes+seconds formatting.
func TestFormatDuration_Minutes(t *testing.T) {
	result := formatDuration(5*time.Minute + 30*time.Second)
	if result != "5m 30s" {
		t.Errorf("expected '5m 30s', got %q", result)
	}
}

// TestFormatDuration_Hours verifies hours formatting.
func TestFormatDuration_Hours(t *testing.T) {
	result := formatDuration(2*time.Hour + 15*time.Minute + 10*time.Second)
	if result != "2h 15m 10s" {
		t.Errorf("expected '2h 15m 10s', got %q", result)
	}
}

// TestFormatDuration_Days verifies days formatting.
func TestFormatDuration_Days(t *testing.T) {
	result := formatDuration(25*time.Hour + 30*time.Minute)
	if !strings.Contains(result, "d") {
		t.Errorf("expected days in result, got %q", result)
	}
}

// TestFormatBytes_Bytes verifies byte formatting below 1 KiB.
func TestFormatBytes_Bytes(t *testing.T) {
	result := formatBytes(512)
	if result != "512 B" {
		t.Errorf("expected '512 B', got %q", result)
	}
}

// TestFormatBytes_Kilobytes verifies KiB formatting.
func TestFormatBytes_Kilobytes(t *testing.T) {
	result := formatBytes(2048)
	if !strings.Contains(result, "KiB") {
		t.Errorf("expected KiB in result, got %q", result)
	}
}

// TestFormatBytes_Megabytes verifies MiB formatting.
func TestFormatBytes_Megabytes(t *testing.T) {
	result := formatBytes(3 * 1024 * 1024)
	if !strings.Contains(result, "MiB") {
		t.Errorf("expected MiB in result, got %q", result)
	}
}
