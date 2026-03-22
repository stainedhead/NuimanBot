package health_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/infrastructure/health"
)

// TestHealthServer_StartTLS_AcceptsHTTPS verifies that StartTLS starts an HTTPS server
// and that an HTTPS client with InsecureSkipVerify=true receives a 200 response.
func TestHealthServer_StartTLS_AcceptsHTTPS(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := config.TLSConfig{
		Enabled:      true,
		AutoGenerate: true,
		CertFile:     filepath.Join(dir, "server.crt"),
		KeyFile:      filepath.Join(dir, "server.key"),
		Hosts:        []string{"localhost", "127.0.0.1"},
	}

	server := health.NewServer(nil, nil, "")

	if err := server.StartTLS(addr, cfg); err != nil {
		t.Fatalf("StartTLS failed: %v", err)
	}
	defer server.Stop() //nolint:errcheck

	// Give the server a moment to start listening.
	time.Sleep(20 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test only
		},
	}

	resp, err := client.Get("https://" + addr + "/health")
	if err != nil {
		t.Fatalf("HTTPS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestHealthServer_StartTLS_RejectsPlainHTTP verifies that a plain HTTP client
// fails to communicate with a TLS-enabled health server.
func TestHealthServer_StartTLS_RejectsPlainHTTP(t *testing.T) {
	dir := t.TempDir()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	cfg := config.TLSConfig{
		Enabled:      true,
		AutoGenerate: true,
		CertFile:     filepath.Join(dir, "server.crt"),
		KeyFile:      filepath.Join(dir, "server.key"),
		Hosts:        []string{"localhost"},
	}

	server := health.NewServer(nil, nil, "")

	if err := server.StartTLS(addr, cfg); err != nil {
		t.Fatalf("StartTLS failed: %v", err)
	}
	defer server.Stop() //nolint:errcheck

	time.Sleep(20 * time.Millisecond)

	// Plain HTTP client should fail to communicate meaningfully with a TLS server.
	// The TLS server may return a garbled response or close the connection —
	// either an error or a non-2xx status is acceptable.
	resp, err := http.Get("http://" + addr + "/health")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			t.Errorf("plain HTTP to TLS server should not return a 2xx status, got %d", resp.StatusCode)
		}
	}
	// An error is also acceptable (connection reset, EOF, etc.)
}

// freePort finds an available TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
