package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
)

func TestIngatanConfig_YAMLUnmarshal(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
memory:
  backend: ingatan
  ingatan:
    url: "https://localhost:8443"
    api_key: "secret-api-key"
    store_prefix: "nuiman"
    tls_skip_verify: true
    token_ttl: "23h"
    fallback_to_builtin: true
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Memory.Backend != config.MemoryBackendIngatan {
		t.Errorf("Expected Memory.Backend %q, got %q", config.MemoryBackendIngatan, cfg.Memory.Backend)
	}
	if cfg.Memory.Ingatan.URL != "https://localhost:8443" {
		t.Errorf("Expected Ingatan.URL %q, got %q", "https://localhost:8443", cfg.Memory.Ingatan.URL)
	}
	if cfg.Memory.Ingatan.APIKey.Value() != "secret-api-key" {
		t.Errorf("Expected Ingatan.APIKey 'secret-api-key', got %q", cfg.Memory.Ingatan.APIKey.Value())
	}
	if cfg.Memory.Ingatan.StorePrefix != "nuiman" {
		t.Errorf("Expected Ingatan.StorePrefix %q, got %q", "nuiman", cfg.Memory.Ingatan.StorePrefix)
	}
	if !cfg.Memory.Ingatan.TLSSkipVerify {
		t.Error("Expected Ingatan.TLSSkipVerify true, got false")
	}
	if cfg.Memory.Ingatan.TokenTTL != "23h" {
		t.Errorf("Expected Ingatan.TokenTTL %q, got %q", "23h", cfg.Memory.Ingatan.TokenTTL)
	}
	if !cfg.Memory.Ingatan.FallbackToBuiltin {
		t.Error("Expected Ingatan.FallbackToBuiltin true, got false")
	}
}

func TestMemoryBackendIngatan_Constant(t *testing.T) {
	if config.MemoryBackendIngatan != "ingatan" {
		t.Errorf("Expected MemoryBackendIngatan == %q, got %q", "ingatan", config.MemoryBackendIngatan)
	}
}

func TestIngatanConfig_APIKeyIsSecureString(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
memory:
  backend: ingatan
  ingatan:
    url: "https://localhost:8443"
    api_key: "my-secret-key"
    store_prefix: "nuiman"
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// The SecureString %v format must not expose the raw value.
	formatted := fmt.Sprintf("%v", cfg.Memory.Ingatan.APIKey)
	if formatted == "my-secret-key" {
		t.Errorf("SecureString %%v format must not expose the raw API key value, got: %q", formatted)
	}

	// Value() must return the actual key.
	if cfg.Memory.Ingatan.APIKey.Value() != "my-secret-key" {
		t.Errorf("APIKey.Value() = %q, want %q", cfg.Memory.Ingatan.APIKey.Value(), "my-secret-key")
	}
}

func TestTLSConfig_YAMLUnmarshal(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
tls:
  enabled: true
  auto_generate: true
  cert_file: "data/certs/server.crt"
  key_file: "data/certs/server.key"
  hosts:
    - "localhost"
    - "nuimanbot.internal"
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.TLS.Enabled {
		t.Error("Expected TLS.Enabled true, got false")
	}
	if !cfg.TLS.AutoGenerate {
		t.Error("Expected TLS.AutoGenerate true, got false")
	}
	if cfg.TLS.CertFile != "data/certs/server.crt" {
		t.Errorf("Expected TLS.CertFile %q, got %q", "data/certs/server.crt", cfg.TLS.CertFile)
	}
	if cfg.TLS.KeyFile != "data/certs/server.key" {
		t.Errorf("Expected TLS.KeyFile %q, got %q", "data/certs/server.key", cfg.TLS.KeyFile)
	}
	if len(cfg.TLS.Hosts) != 2 || cfg.TLS.Hosts[0] != "localhost" || cfg.TLS.Hosts[1] != "nuimanbot.internal" {
		t.Errorf("Expected TLS.Hosts [localhost, nuimanbot.internal], got %v", cfg.TLS.Hosts)
	}
}

func TestMCPClientConfig_ConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
mcp:
  client:
    config_file: "mcp.json"
    enabled: true
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.MCP.Client.ConfigFile != "mcp.json" {
		t.Errorf("Expected MCP.Client.ConfigFile %q, got %q", "mcp.json", cfg.MCP.Client.ConfigFile)
	}
	if !cfg.MCP.Client.Enabled {
		t.Error("Expected MCP.Client.Enabled true, got false")
	}
}
