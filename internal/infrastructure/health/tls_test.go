package health_test

import (
	"net"
	"strings"
	"testing"

	"nuimanbot/internal/infrastructure/health"
)

// TestStartTLS_AlreadyBoundPort verifies that StartTLS returns a non-nil error
// when the target address is already in use, surfacing the bind failure to the
// caller rather than silently swallowing it inside a goroutine.
func TestStartTLS_AlreadyBoundPort(t *testing.T) {
	// Bind a real TCP listener to a random free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to acquire a free port: %v", err)
	}
	t.Cleanup(func() {
		if err := ln.Close(); err != nil {
			t.Logf("cleanup: close listener: %v", err)
		}
	})

	addr := ln.Addr().String()

	s := health.NewServer(nil, nil, "")

	// Attempt to start the TLS health server on the already-bound port.
	// We pass empty cert/key paths intentionally: the bind failure must
	// occur (and be returned) before any TLS handshake takes place.
	//
	// If the implementation still fires ListenAndServeTLS in a goroutine
	// and returns nil, this assertion will catch the regression.
	gotErr := s.StartTLS(addr, "", "")

	if gotErr == nil {
		t.Fatal("StartTLS: expected a non-nil error when port is already in use, got nil")
	}

	errMsg := gotErr.Error()
	if !strings.Contains(errMsg, "listen") &&
		!strings.Contains(errMsg, "bind") &&
		!strings.Contains(errMsg, "address already in use") &&
		!strings.Contains(errMsg, "in use") {
		t.Errorf("StartTLS: error %q does not contain expected keyword (listen/bind/address already in use)", errMsg)
	}
}
