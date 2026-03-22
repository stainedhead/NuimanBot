package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
)

func TestLoadConfig_ProvidersEnvOverride_ExistingProvider(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
llm:
  providers:
    - id: existing-provider
      type: anthropic
      api_key: old-key
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("NUIMANBOT_ENCRYPTION_KEY", "test-key-longer-than-32-chars-ok")
	// Override the existing provider
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_ID", "existing-provider")
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_TYPE", "openai")
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_APIKEY", "new-key")

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	found := false
	for _, p := range cfg.LLM.Providers {
		if p.ID == "existing-provider" {
			found = true
			if string(p.Type) != "openai" {
				t.Errorf("Provider type = %q, want %q", p.Type, "openai")
			}
		}
	}
	if !found {
		t.Error("Expected existing-provider to be in providers list")
	}
}

func TestLoadConfig_SecurityTokenRotationHours(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFilePath, []byte("server:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("NUIMANBOT_ENCRYPTION_KEY", "test-key-longer-than-32-chars-ok")
	t.Setenv("NUIMANBOT_SECURITY_TOKENROTATIONHOURS", "48")

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Security.TokenRotationHours != 48 {
		t.Errorf("TokenRotationHours = %d, want 48", cfg.Security.TokenRotationHours)
	}
}
