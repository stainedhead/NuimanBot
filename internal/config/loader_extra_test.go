package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/config"
)

func TestLoadConfig_EnvOverrides(t *testing.T) {
	// Create minimal config file
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
  debug: false
security:
  input_max_length: 512
  encryption_key: "short"
llm:
  default_model:
    primary: anthropic/claude-sonnet
`
	if err := os.WriteFile(configFilePath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test env overrides
	t.Setenv("NUIMANBOT_SERVER_LOGLEVEL", "warn")
	t.Setenv("NUIMANBOT_SERVER_DEBUG", "true")
	t.Setenv("NUIMANBOT_SECURITY_INPUTMAXLENGTH", "2048")
	t.Setenv("NUIMANBOT_ENCRYPTION_KEY", "test-key-longer-than-32-chars-ok")
	t.Setenv("NUIMANBOT_GATEWAYS_CLI_DEBUGMODE", "true")
	t.Setenv("NUIMANBOT_MCP_CLIENT_TIMEOUT", "60s")
	t.Setenv("NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY", "openai/gpt-4")

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Server.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.Server.LogLevel, "warn")
	}
	if !cfg.Server.Debug {
		t.Error("Debug should be true from env override")
	}
	if cfg.Security.InputMaxLength != 2048 {
		t.Errorf("InputMaxLength = %d, want 2048", cfg.Security.InputMaxLength)
	}
	if cfg.Gateways.CLI.DebugMode != true {
		t.Error("CLI DebugMode should be true from env")
	}
	if cfg.MCP.Client.Timeout != "60s" {
		t.Errorf("MCP timeout = %q, want %q", cfg.MCP.Client.Timeout, "60s")
	}
	if cfg.LLM.DefaultModel.Primary != "openai/gpt-4" {
		t.Errorf("DefaultModel = %q, want %q", cfg.LLM.DefaultModel.Primary, "openai/gpt-4")
	}
}

func TestLoadConfig_BedrockEnvOverrides(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFilePath, []byte("server:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_PROFILE", "test-profile")
	t.Setenv("BEDROCK_DEFAULT_MODEL", "anthropic.claude-v2")
	t.Setenv("BEDROCK_MAX_RETRIES", "5")
	t.Setenv("BEDROCK_REQUEST_TIMEOUT", "120")
	t.Setenv("NUIMANBOT_ENCRYPTION_KEY", "test-key-longer-than-32-chars-ok")

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.LLM.Bedrock.AWSRegion != "us-west-2" {
		t.Errorf("AWSRegion = %q, want %q", cfg.LLM.Bedrock.AWSRegion, "us-west-2")
	}
	if cfg.LLM.Bedrock.AWSProfile != "test-profile" {
		t.Errorf("AWSProfile = %q, want %q", cfg.LLM.Bedrock.AWSProfile, "test-profile")
	}
	if cfg.LLM.Bedrock.DefaultModel != "anthropic.claude-v2" {
		t.Errorf("DefaultModel = %q, want %q", cfg.LLM.Bedrock.DefaultModel, "anthropic.claude-v2")
	}
	if cfg.LLM.Bedrock.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.LLM.Bedrock.MaxRetries)
	}
	if cfg.LLM.Bedrock.RequestTimeout != 120 {
		t.Errorf("RequestTimeout = %d, want 120", cfg.LLM.Bedrock.RequestTimeout)
	}
}

func TestLoadConfig_ProvidersFromEnv(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFilePath, []byte("server:\n  log_level: info\n"), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	t.Setenv("NUIMANBOT_ENCRYPTION_KEY", "test-key-longer-than-32-chars-ok")
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_ID", "anthropic-env")
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_TYPE", "anthropic")
	t.Setenv("NUIMANBOT_LLM_PROVIDERS_0_APIKEY", "sk-test-key")

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Check that the provider was loaded from env
	found := false
	for _, p := range cfg.LLM.Providers {
		if p.ID == "anthropic-env" {
			found = true
			if p.Type != "anthropic" {
				t.Errorf("Provider type = %q, want %q", p.Type, "anthropic")
			}
		}
	}
	if !found {
		t.Error("Expected provider from env vars to be loaded")
	}
}

func TestValidateProductionConfig_DebugMode(t *testing.T) {
	cfg := &config.NuimanBotConfig{}
	cfg.Server.Environment = config.ParseEnvironment("production")
	cfg.Server.Debug = true
	cfg.Security.EncryptionKey = "a-valid-32-char-encryption-key-ok"

	err := config.ValidateProductionConfig(cfg)
	if err == nil {
		t.Error("expected error for debug mode in production")
	}
}

func TestValidateProductionConfig_WeakKey(t *testing.T) {
	cfg := &config.NuimanBotConfig{}
	cfg.Server.Environment = config.ParseEnvironment("production")
	cfg.Server.Debug = false
	cfg.Security.EncryptionKey = "short-key"

	err := config.ValidateProductionConfig(cfg)
	if err == nil {
		t.Error("expected error for short encryption key in production")
	}
}

func TestValidateProductionConfig_ValidConfig(t *testing.T) {
	cfg := &config.NuimanBotConfig{}
	cfg.Server.Environment = config.ParseEnvironment("production")
	cfg.Server.Debug = false
	cfg.Security.EncryptionKey = "a-very-valid-32-char-or-longer-encryption-key"

	err := config.ValidateProductionConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error for valid production config: %v", err)
	}
}

func TestValidateProductionConfig_NonProductionEnv(t *testing.T) {
	cfg := &config.NuimanBotConfig{}
	cfg.Server.Environment = config.ParseEnvironment("development")
	cfg.Server.Debug = true
	cfg.Security.EncryptionKey = "short"

	// Should not error for non-production
	err := config.ValidateProductionConfig(cfg)
	if err != nil {
		t.Errorf("expected no error for non-production: %v", err)
	}
}
