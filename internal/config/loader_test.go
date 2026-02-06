package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/config"
)

func TestLoadConfig_FromFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: debug
  debug: true
security:
  input_max_length: 1024
llm:
  default_model:
    primary: anthropic/claude-sonnet
  providers:
    - id: test-anthropic
      type: anthropic
      api_key: sk-test-anthropic-key-file
    - id: test-openai
      type: openai
      api_key: sk-test-openai-key-file
gateways:
  cli:
    debug_mode: true
mcp:
  client:
    timeout: 30s
`
	err := os.WriteFile(configFilePath, []byte(configContent), 0o644) // Fixed octalLiteral
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	// Set required environment variable for SecureString handling (even if not directly used in this test case)
	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected Server.LogLevel 'debug', got '%s'", cfg.Server.LogLevel)
	}
	if !cfg.Server.Debug {
		t.Errorf("Expected Server.Debug true, got false")
	}
	if cfg.Security.InputMaxLength != 1024 {
		t.Errorf("Expected Security.InputMaxLength 1024, got %d", cfg.Security.InputMaxLength)
	}
	if cfg.LLM.DefaultModel.Primary != "anthropic/claude-sonnet" {
		t.Errorf("Expected LLM.DefaultModel.Primary 'anthropic/claude-sonnet', got '%s'", cfg.LLM.DefaultModel.Primary)
	}
	if !cfg.Gateways.CLI.DebugMode {
		t.Errorf("Expected Gateways.CLI.DebugMode true, got false")
	}
	if cfg.MCP.Client.Timeout != "30s" {
		t.Errorf("Expected MCP.Client.Timeout '30s', got '%s'", cfg.MCP.Client.Timeout)
	}
	if len(cfg.LLM.Providers) != 2 {
		t.Fatalf("Expected 2 LLM providers, got %d", len(cfg.LLM.Providers))
	}
	if cfg.LLM.Providers[0].APIKey.Value() != "sk-test-anthropic-key-file" {
		t.Errorf("Expected provider API key 'sk-test-anthropic-key-file', got '%s'", cfg.LLM.Providers[0].APIKey.Value())
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Unset all relevant env vars first to ensure clean state
	envVars := []string{
		"NUIMANBOT_SERVER_LOGLEVEL",
		"NUIMANBOT_SERVER_DEBUG",
		"NUIMANBOT_SECURITY_INPUTMAXLENGTH",
		"NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY",
		"NUIMANBOT_GATEWAYS_CLI_DEBUGMODE",
		"NUIMANBOT_ENCRYPTION_KEY",
		"NUIMANBOT_LLM_PROVIDERS_0_APIKEY",           // For array testing
		"NUIMANBOT_LLM_PROVIDERS_1_APIKEY",           // For array testing
		"NUIMANBOT_SKILLS_ENTRIES_CALCULATOR_APIKEY", // For map testing
	}
	for _, ev := range envVars {
		if err := os.Unsetenv(ev); err != nil {
			t.Fatalf("Failed to unset env var %s: %v", ev, err)
		}
	}
	t.Cleanup(func() { // Restore env vars after test
		for _, ev := range envVars {
			if err := os.Unsetenv(ev); err != nil {
				t.Logf("Failed to unset env var %s during cleanup: %v", ev, err)
			}
		}
	})

	if err := os.Setenv("NUIMANBOT_SERVER_LOGLEVEL", "info"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_SERVER_DEBUG", "false"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_SECURITY_INPUTMAXLENGTH", "2048"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY", "openai/gpt4"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_GATEWAYS_CLI_DEBUGMODE", "false"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_LLM_PROVIDERS_0_ID", "env-anthropic"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_LLM_PROVIDERS_0_TYPE", "anthropic"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_LLM_PROVIDERS_0_APIKEY", "sk-env-anthropic-key"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}
	if err := os.Setenv("NUIMANBOT_SKILLS_ENTRIES_CALCULATOR_APIKEY", "skill-calc-key"); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	fmt.Println("Environment variables for test:", os.Environ()) // Debug print

	cfg, err := config.LoadConfig() // No path, so relies on env vars primarily
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected Server.LogLevel 'info', got '%s'", cfg.Server.LogLevel)
	}
	if cfg.Server.Debug {
		t.Errorf("Expected Server.Debug false, got true")
	}
	if cfg.Security.InputMaxLength != 2048 {
		t.Errorf("Expected Security.InputMaxLength 2048, got %d", cfg.Security.InputMaxLength)
	}
	if cfg.LLM.DefaultModel.Primary != "openai/gpt4" {
		t.Errorf("Expected LLM.DefaultModel.Primary 'openai/gpt4', got '%s'", cfg.LLM.DefaultModel.Primary)
	}
	if cfg.Gateways.CLI.DebugMode {
		t.Errorf("Expected Gateways.CLI.DebugMode false, got true")
	}

	// Test SecureString handling from env
	if len(cfg.LLM.Providers) > 0 && cfg.LLM.Providers[0].ID == "env-anthropic" {
		if cfg.LLM.Providers[0].APIKey.Value() != "sk-env-anthropic-key" {
			t.Errorf("Expected env provider API key 'sk-env-anthropic-key', got '%s'", cfg.LLM.Providers[0].APIKey.Value())
		}
	} else {
		t.Errorf("LLM Provider from env not loaded correctly or APIKey not set")
	}

	if val, ok := cfg.Skills.Entries["calculator"]; ok {
		if val.APIKey.Value() != "skill-calc-key" {
			t.Errorf("Expected skill API key 'skill-calc-key', got '%s'", val.APIKey.Value())
		}
	} else {
		t.Errorf("Calculator skill config not loaded from env")
	}
}

func TestLoadConfig_MissingEncryptionKey(t *testing.T) {

	if err := os.Unsetenv("NUIMANBOT_ENCRYPTION_KEY"); err != nil {

		t.Fatalf("Failed to unset env var: %v", err)

	}

	t.Cleanup(func() {

		if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "dummykey"); err != nil { // Re-set for other tests

			t.Logf("Failed to set env var during cleanup: %v", err)

		}

	})

	_, err := config.LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "NUIMANBOT_ENCRYPTION_KEY is not set") {

		t.Errorf("Expected 'encryption key not set' error, got: %v", err)

	}

}

func TestLoadConfig_OpenAIConfig(t *testing.T) {
	tempDir := t.TempDir()
	configFilePath := filepath.Join(tempDir, "config.yaml")
	configContent := `
server:
  log_level: info
security:
  input_max_length: 1024
llm:
  openai:
    api_key: sk-test-openai-key
    base_url: https://api.openai.com/v1
    default_model: gpt-4o
    organization: org-test123
  ollama:
    base_url: http://localhost:11434
    default_model: llama2
`
	err := os.WriteFile(configFilePath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD="); err != nil {
		t.Fatalf("Failed to set env var: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test OpenAI config
	if cfg.LLM.OpenAI.APIKey.Value() != "sk-test-openai-key" {
		t.Errorf("Expected OpenAI APIKey 'sk-test-openai-key', got '%s'", cfg.LLM.OpenAI.APIKey.Value())
	}
	if cfg.LLM.OpenAI.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected OpenAI BaseURL 'https://api.openai.com/v1', got '%s'", cfg.LLM.OpenAI.BaseURL)
	}
	if cfg.LLM.OpenAI.DefaultModel != "gpt-4o" {
		t.Errorf("Expected OpenAI DefaultModel 'gpt-4o', got '%s'", cfg.LLM.OpenAI.DefaultModel)
	}
	if cfg.LLM.OpenAI.Organization != "org-test123" {
		t.Errorf("Expected OpenAI Organization 'org-test123', got '%s'", cfg.LLM.OpenAI.Organization)
	}

	// Test Ollama config
	if cfg.LLM.Ollama.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected Ollama BaseURL 'http://localhost:11434', got '%s'", cfg.LLM.Ollama.BaseURL)
	}
	if cfg.LLM.Ollama.DefaultModel != "llama2" {
		t.Errorf("Expected Ollama DefaultModel 'llama2', got '%s'", cfg.LLM.Ollama.DefaultModel)
	}
}

func TestLoadConfig_MixedSources(t *testing.T) {

	tempDir := t.TempDir()

	configFilePath := filepath.Join(tempDir, "config.yaml")

	configContent := `

server:

  log_level: debug

  debug: true

security:

  input_max_length: 512

llm:

  default_model:

    primary: anthropic/claude-sonnet-file

`

	err := os.WriteFile(configFilePath, []byte(configContent), 0o644) // Fixed octalLiteral

	if err != nil {

		t.Fatalf("Failed to write temp config file: %v", err)

	}

	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC="); err != nil {

		t.Fatalf("Failed to set env var: %v", err)

	}

	if err := os.Setenv("NUIMANBOT_SERVER_LOGLEVEL", "override_info"); err != nil { // Env var should override file

		t.Fatalf("Failed to set env var: %v", err)

	}

	if err := os.Setenv("NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY", "openai/gpt-mixed"); err != nil { // Env var should override file

		t.Fatalf("Failed to set env var: %v", err)

	}

	t.Cleanup(func() {

		if err := os.Unsetenv("NUIMANBOT_SERVER_LOGLEVEL"); err != nil {

			t.Logf("Failed to unset env var during cleanup: %v", err)

		}

		if err := os.Unsetenv("NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY"); err != nil {

			t.Logf("Failed to unset env var during cleanup: %v", err)

		}

	})

	cfg, err := config.LoadConfig(tempDir)

	if err != nil {

		t.Fatalf("LoadConfig failed: %v", err)

	}

	// Environment variable should take precedence
	if cfg.Server.LogLevel != "override_info" {
		t.Errorf("Expected Server.LogLevel 'override_info' from env, got '%s'", cfg.Server.LogLevel)
	}
	if cfg.LLM.DefaultModel.Primary != "openai/gpt-mixed" {
		t.Errorf("Expected LLM.DefaultModel.Primary 'openai/gpt-mixed' from env, got '%s'", cfg.LLM.DefaultModel.Primary)
	}

	// File config should still be present where not overridden
	if cfg.Security.InputMaxLength != 512 {
		t.Errorf("Expected Security.InputMaxLength 512 from file, got %d", cfg.Security.InputMaxLength)
	}
}
