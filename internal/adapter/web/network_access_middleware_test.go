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

func TestNetworkAllowlistMiddleware_LocalhostOnly_AllowsIPv6Loopback(t *testing.T) {
	// Regression test: on many systems "localhost" resolves to ::1 before
	// 127.0.0.1, and Go formats that RemoteAddr as "[::1]:port" (bracketed).
	// A naive last-colon port strip returns "[::1]" (brackets still
	// attached), which never matches localhostHosts's "::1" entry —
	// silently rejecting legitimate IPv6 loopback traffic. Caught via
	// manual end-to-end verification (curl to "localhost" was denied) and
	// fixed by switching extractRemoteIP to net.SplitHostPort.
	server := NewServer(":0")
	server.SetNetworkAccessConfig(domain.NetworkAccessConfig{Mode: domain.AccessModeLocalhostOnly})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "[::1]:54321"
	w := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected IPv6 loopback ([::1]) to be allowed in localhost-only mode, got %d", w.Code)
	}
}

func TestExtractRemoteIP_IPv4AndIPv6(t *testing.T) {
	cases := []struct {
		remoteAddr string
		want       string
	}{
		{"127.0.0.1:1234", "127.0.0.1"},
		{"203.0.113.9:8081", "203.0.113.9"},
		{"[::1]:54321", "::1"},
		{"[2001:db8::1]:443", "2001:db8::1"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tc.remoteAddr
		if got := extractRemoteIP(req); got != tc.want {
			t.Errorf("RemoteAddr %q: expected %q, got %q", tc.remoteAddr, tc.want, got)
		}
	}
}

func TestExtractRemoteIP_NoPortIsDefensive(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-a-valid-addr"
	if got := extractRemoteIP(req); got != "not-a-valid-addr" {
		t.Fatalf("expected the raw RemoteAddr as a defensive fallback, got %q", got)
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
