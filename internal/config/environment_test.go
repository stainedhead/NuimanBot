package config_test

import (
	"os"
	"testing"

	"nuimanbot/internal/config"
)

func TestEnvironment_Development(t *testing.T) {
	env := config.EnvironmentDevelopment

	if !env.IsDevelopment() {
		t.Error("Expected IsDevelopment() to be true")
	}

	if env.IsProduction() {
		t.Error("Expected IsProduction() to be false")
	}

	if env.IsStaging() {
		t.Error("Expected IsStaging() to be false")
	}
}

func TestEnvironment_Production(t *testing.T) {
	env := config.EnvironmentProduction

	if !env.IsProduction() {
		t.Error("Expected IsProduction() to be true")
	}

	if env.IsDevelopment() {
		t.Error("Expected IsDevelopment() to be false")
	}
}

func TestEnvironment_Staging(t *testing.T) {
	env := config.EnvironmentStaging

	if !env.IsStaging() {
		t.Error("Expected IsStaging() to be true")
	}

	if env.IsProduction() {
		t.Error("Expected IsProduction() to be false")
	}
}

func TestEnvironment_String(t *testing.T) {
	tests := []struct {
		env  config.Environment
		want string
	}{
		{config.EnvironmentDevelopment, "development"},
		{config.EnvironmentStaging, "staging"},
		{config.EnvironmentProduction, "production"},
	}

	for _, tt := range tests {
		if got := tt.env.String(); got != tt.want {
			t.Errorf("Environment.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseEnvironment(t *testing.T) {
	tests := []struct {
		input string
		want  config.Environment
	}{
		{"development", config.EnvironmentDevelopment},
		{"dev", config.EnvironmentDevelopment},
		{"staging", config.EnvironmentStaging},
		{"stage", config.EnvironmentStaging},
		{"production", config.EnvironmentProduction},
		{"prod", config.EnvironmentProduction},
		{"", config.EnvironmentDevelopment},        // default
		{"invalid", config.EnvironmentDevelopment}, // default
	}

	for _, tt := range tests {
		got := config.ParseEnvironment(tt.input)
		if got != tt.want {
			t.Errorf("ParseEnvironment(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestEnvironmentFromEnv(t *testing.T) {
	// Save original value
	original := os.Getenv("ENVIRONMENT")
	defer func() {
		if original != "" {
			os.Setenv("ENVIRONMENT", original)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}
	}()

	tests := []struct {
		envVar string
		want   config.Environment
	}{
		{"production", config.EnvironmentProduction},
		{"staging", config.EnvironmentStaging},
		{"development", config.EnvironmentDevelopment},
		{"", config.EnvironmentDevelopment}, // default when not set
	}

	for _, tt := range tests {
		if tt.envVar != "" {
			os.Setenv("ENVIRONMENT", tt.envVar)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}

		got := config.EnvironmentFromEnv()
		if got != tt.want {
			t.Errorf("EnvironmentFromEnv() with ENVIRONMENT=%q = %v, want %v", tt.envVar, got, tt.want)
		}
	}
}

func TestApplyEnvironmentDefaults_Development(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
		},
	}

	config.ApplyEnvironmentDefaults(cfg)

	// Note: Debug is not forced in development to allow env var overrides
	// It defaults to false (Go zero value)

	if cfg.Server.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", cfg.Server.LogLevel)
	}

	if cfg.Security.InputMaxLength != 8192 {
		t.Errorf("Expected InputMaxLength 8192, got %d", cfg.Security.InputMaxLength)
	}
}

func TestApplyEnvironmentDefaults_Production(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentProduction,
		},
	}

	config.ApplyEnvironmentDefaults(cfg)

	if cfg.Server.Debug != false {
		t.Error("Expected Debug to be false in production")
	}

	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got '%s'", cfg.Server.LogLevel)
	}

	if cfg.Security.InputMaxLength != 4096 {
		t.Errorf("Expected InputMaxLength 4096, got %d", cfg.Security.InputMaxLength)
	}
}

func TestApplyEnvironmentDefaults_Staging(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentStaging,
		},
	}

	config.ApplyEnvironmentDefaults(cfg)

	if cfg.Server.Debug != false {
		t.Error("Expected Debug to be false in staging")
	}

	if cfg.Server.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got '%s'", cfg.Server.LogLevel)
	}

	if cfg.Security.InputMaxLength != 4096 {
		t.Errorf("Expected InputMaxLength 4096, got %d", cfg.Security.InputMaxLength)
	}
}

func TestValidateProductionConfig_Valid(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentProduction,
			Debug:       false,
			LogLevel:    "info",
		},
		Security: config.SecurityConfig{
			EncryptionKey: "test-key-with-sufficient-length-123456",
		},
	}

	if err := config.ValidateProductionConfig(cfg); err != nil {
		t.Errorf("Expected no error for valid production config, got: %v", err)
	}
}

func TestValidateProductionConfig_DebugEnabled(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentProduction,
			Debug:       true, // Should fail in production
		},
	}

	if err := config.ValidateProductionConfig(cfg); err == nil {
		t.Error("Expected error when debug is enabled in production")
	}
}

func TestValidateProductionConfig_DebugLogLevel(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentProduction,
			Debug:       false,
			LogLevel:    "debug", // Should warn in production
		},
		Security: config.SecurityConfig{
			EncryptionKey: "test-key-with-sufficient-length-12345678", // Must be 32+ chars
		},
	}

	// Should not error, but would log warning in real implementation
	if err := config.ValidateProductionConfig(cfg); err != nil {
		t.Errorf("Expected no error for debug log level (should only warn), got: %v", err)
	}
}

func TestValidateProductionConfig_WeakEncryptionKey(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentProduction,
			Debug:       false,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "short", // Too short
		},
	}

	if err := config.ValidateProductionConfig(cfg); err == nil {
		t.Error("Expected error for weak encryption key in production")
	}
}
