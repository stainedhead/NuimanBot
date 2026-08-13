package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestInitialize_CustomOutput verifies that a caller-supplied Output writer
// receives log records instead of the os.Stdout default — required by the
// ACP entrypoint (cmd/nuimanbot/acp.go), which must never let log lines land
// on stdout alongside the ACP JSON-RPC stream.
func TestInitialize_CustomOutput(t *testing.T) {
	var buf bytes.Buffer
	Initialize(Config{Level: LogLevelInfo, Format: "json", Output: &buf})

	slog.Info("test message", "key", "value")

	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log output to contain %q, got %q", "test message", buf.String())
	}
}

// TestInitialize_DefaultOutput verifies that a nil Output preserves the
// existing default (os.Stdout) behavior for every other caller of
// Initialize — this test only checks that Initialize doesn't panic/error
// with a zero-value Output; the stdout default itself isn't independently
// observable from within a test process without redirecting os.Stdout.
func TestInitialize_DefaultOutput(t *testing.T) {
	Initialize(Config{Level: LogLevelInfo, Format: "json"})
	slog.Info("default output smoke test")
}
