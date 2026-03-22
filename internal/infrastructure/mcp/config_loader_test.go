package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMCPConfig_HTTPEntry(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "my-server",
				"transport": "http",
				"url": "https://example.com/mcp",
				"headers": {"Authorization": "Bearer token123"}
			}
		]
	}`
	path := writeTempFile(t, data)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
	}
	s := cfg.Servers[0]
	if s.Name != "my-server" {
		t.Errorf("expected name 'my-server', got %q", s.Name)
	}
	if s.Transport != "http" {
		t.Errorf("expected transport 'http', got %q", s.Transport)
	}
	if s.URL != "https://example.com/mcp" {
		t.Errorf("unexpected URL: %q", s.URL)
	}
	if s.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("unexpected header: %q", s.Headers["Authorization"])
	}
}

func TestLoadMCPConfig_StdioEntry(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "stdio-server",
				"transport": "stdio",
				"command": "/usr/bin/mcp-server",
				"args": ["--port", "8080"]
			}
		]
	}`
	path := writeTempFile(t, data)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := cfg.Servers[0]
	if s.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", s.Transport)
	}
	if s.Command != "/usr/bin/mcp-server" {
		t.Errorf("unexpected command: %q", s.Command)
	}
	if len(s.Args) != 2 || s.Args[0] != "--port" || s.Args[1] != "8080" {
		t.Errorf("unexpected args: %v", s.Args)
	}
}

func TestLoadMCPConfig_EnvSubstitution(t *testing.T) {
	t.Setenv("MCP_TEST_TOKEN", "secret-token-value")

	data := `{
		"servers": [
			{
				"name": "env-server",
				"transport": "http",
				"url": "https://example.com/mcp",
				"headers": {"Authorization": "Bearer ${MCP_TEST_TOKEN}"}
			}
		]
	}`
	path := writeTempFile(t, data)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	auth := cfg.Servers[0].Headers["Authorization"]
	if auth != "Bearer secret-token-value" {
		t.Errorf("env substitution failed; got %q", auth)
	}
}

func TestLoadMCPConfig_EnvSubstitutionInURL(t *testing.T) {
	t.Setenv("MCP_HOST", "myhost.example.com")

	data := `{
		"servers": [
			{
				"name": "env-url-server",
				"transport": "http",
				"url": "https://${MCP_HOST}/mcp"
			}
		]
	}`
	path := writeTempFile(t, data)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Servers[0].URL != "https://myhost.example.com/mcp" {
		t.Errorf("env substitution in URL failed; got %q", cfg.Servers[0].URL)
	}
}

func TestLoadMCPConfig_MissingName(t *testing.T) {
	data := `{
		"servers": [
			{
				"transport": "http",
				"url": "https://example.com/mcp"
			}
		]
	}`
	path := writeTempFile(t, data)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoadMCPConfig_MissingTransport(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "no-transport",
				"url": "https://example.com/mcp"
			}
		]
	}`
	path := writeTempFile(t, data)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected error for missing transport, got nil")
	}
}

func TestLoadMCPConfig_HTTPMissingURL(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "no-url",
				"transport": "http"
			}
		]
	}`
	path := writeTempFile(t, data)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected error for missing url, got nil")
	}
}

func TestLoadMCPConfig_StdioMissingCommand(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "no-command",
				"transport": "stdio"
			}
		]
	}`
	path := writeTempFile(t, data)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected error for missing command, got nil")
	}
}

func TestLoadMCPConfig_UnknownTransport(t *testing.T) {
	data := `{
		"servers": [
			{
				"name": "unknown-server",
				"transport": "websocket",
				"url": "wss://example.com"
			}
		]
	}`
	path := writeTempFile(t, data)

	_, err := LoadMCPConfig(path)
	if err == nil {
		t.Fatal("expected error for unknown transport, got nil")
	}
}

func TestLoadMCPConfig_EmptyServers(t *testing.T) {
	data := `{"servers": []}`
	path := writeTempFile(t, data)

	cfg, err := LoadMCPConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected empty servers, got %d", len(cfg.Servers))
	}
}

func TestLoadMCPConfig_FileNotFound(t *testing.T) {
	_, err := LoadMCPConfig("/nonexistent/path/mcp.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestSubstituteEnv_NoVars(t *testing.T) {
	result := substituteEnv("hello world")
	if result != "hello world" {
		t.Errorf("expected unchanged string, got %q", result)
	}
}

func TestSubstituteEnv_MissingVar(t *testing.T) {
	_ = os.Unsetenv("MCP_MISSING_VAR_XYZ")
	// Missing vars should be left as empty string (os.Getenv returns "")
	result := substituteEnv("prefix_${MCP_MISSING_VAR_XYZ}_suffix")
	if result != "prefix__suffix" {
		t.Errorf("expected empty substitution, got %q", result)
	}
}

// writeTempFile creates a temporary JSON file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
