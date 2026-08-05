package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nuimanbot/internal/domain"
)

func TestNetworkAllowlistMiddleware_UnconfiguredAllowsEverything(t *testing.T) {
	server := NewServer(":0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.9:1234" // not loopback
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected an unconfigured server to allow all requests, got status %d", w.Code)
	}
}

func TestNetworkAllowlistMiddleware_LocalhostOnly_RejectsRemote(t *testing.T) {
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{Mode: domain.AccessModeLocalhostOnly})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback source in localhost-only mode, got %d", w.Code)
	}
}

func TestNetworkAllowlistMiddleware_LocalhostOnly_AllowsLoopback(t *testing.T) {
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{Mode: domain.AccessModeLocalhostOnly})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected loopback source to be allowed in localhost-only mode, got %d", w.Code)
	}
}

func TestNetworkAllowlistMiddleware_Remote_EmptyAllowlistDeniesAll(t *testing.T) {
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{Mode: domain.AccessModeRemote, Allowlist: []string{}})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an explicitly empty allowlist (fail-closed), got %d", w.Code)
	}
}

func TestNetworkAllowlistMiddleware_Remote_PopulatedAllowlist(t *testing.T) {
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{
		Mode:      domain.AccessModeRemote,
		Allowlist: []string{"203.0.113.9"},
	})

	allowedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	allowedReq.RemoteAddr = "203.0.113.9:1234"
	w1 := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w1, allowedReq)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected allowlisted source to be allowed, got %d", w1.Code)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	deniedReq.RemoteAddr = "198.51.100.1:1234"
	w2 := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(w2, deniedReq)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected non-allowlisted source to be denied, got %d", w2.Code)
	}
}

func TestNetworkAllowlistMiddleware_GatesBeforeAuth(t *testing.T) {
	// The Security NFR requires allowlist enforcement to happen before
	// authentication — verified here by confirming a denied source never
	// reaches even the public /health endpoint (no auth required at all).
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{Mode: domain.AccessModeRemote, Allowlist: []string{}})

	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected the login page itself to be gated by the allowlist, got %d", w.Code)
	}
}
