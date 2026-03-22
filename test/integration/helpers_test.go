//go:build integration

package integration_test

import (
	"os"
	"testing"
)

// skipIfNoIngatan skips the test if the INGATAN_URL environment variable is not set.
// This prevents integration tests from failing in CI environments without Ingatan.
func skipIfNoIngatan(t *testing.T) {
	t.Helper()
	if os.Getenv("INGATAN_URL") == "" {
		t.Skip("ingatan not running (set INGATAN_URL to enable)")
	}
}

// skipIfNoMCPServer skips the test if the MCP_URL environment variable is not set.
// This prevents integration tests from failing in CI environments without an MCP server.
func skipIfNoMCPServer(t *testing.T) {
	t.Helper()
	if os.Getenv("MCP_URL") == "" {
		t.Skip("MCP server not running (set MCP_URL to enable)")
	}
}

// ingatanURL returns the Ingatan base URL from the environment, defaulting to localhost.
func ingatanURL() string {
	if u := os.Getenv("INGATAN_URL"); u != "" {
		return u
	}
	return "http://localhost:8443"
}

// mcpURL returns the MCP endpoint URL from the environment, defaulting to localhost.
func mcpURL() string {
	if u := os.Getenv("MCP_URL"); u != "" {
		return u
	}
	return "http://localhost:8443/mcp"
}
