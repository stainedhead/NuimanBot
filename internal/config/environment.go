package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Environment represents the deployment environment.
type Environment string

const (
	// EnvironmentDevelopment is the development environment.
	EnvironmentDevelopment Environment = "development"
	// EnvironmentStaging is the staging environment.
	EnvironmentStaging Environment = "staging"
	// EnvironmentProduction is the production environment.
	EnvironmentProduction Environment = "production"
)

// String returns the string representation of the environment.
func (e Environment) String() string {
	return string(e)
}

// IsDevelopment returns true if the environment is development.
func (e Environment) IsDevelopment() bool {
	return e == EnvironmentDevelopment
}

// IsStaging returns true if the environment is staging.
func (e Environment) IsStaging() bool {
	return e == EnvironmentStaging
}

// IsProduction returns true if the environment is production.
func (e Environment) IsProduction() bool {
	return e == EnvironmentProduction
}

// ParseEnvironment parses a string into an Environment.
// Returns EnvironmentDevelopment if the input is invalid.
func ParseEnvironment(s string) Environment {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "production", "prod":
		return EnvironmentProduction
	case "staging", "stage":
		return EnvironmentStaging
	case "development", "dev", "":
		return EnvironmentDevelopment
	default:
		slog.Warn("Unknown environment, defaulting to development", "input", s)
		return EnvironmentDevelopment
	}
}

// EnvironmentFromEnv reads the environment from the ENVIRONMENT env var.
// Defaults to development if not set.
func EnvironmentFromEnv() Environment {
	return ParseEnvironment(os.Getenv("ENVIRONMENT"))
}

// ApplyEnvironmentDefaults applies environment-specific defaults to the configuration.
func ApplyEnvironmentDefaults(cfg *NuimanBotConfig) {
	env := cfg.Server.Environment

	switch env {
	case EnvironmentDevelopment:
		// Development defaults: verbose logging, relaxed limits
		if cfg.Server.LogLevel == "" {
			cfg.Server.LogLevel = "debug"
		}
		// Note: Debug defaults to false (Go zero value), only set explicitly if needed
		// Don't force Debug=true in development to allow env var override
		if cfg.Security.InputMaxLength == 0 {
			cfg.Security.InputMaxLength = 8192
		}

	case EnvironmentStaging:
		// Staging defaults: production-like but with more logging
		if cfg.Server.LogLevel == "" {
			cfg.Server.LogLevel = "info"
		}
		// Debug should be explicitly disabled in staging/production
		if cfg.Security.InputMaxLength == 0 {
			cfg.Security.InputMaxLength = 4096
		}

	case EnvironmentProduction:
		// Production defaults: minimal logging, strict limits
		if cfg.Server.LogLevel == "" {
			cfg.Server.LogLevel = "info"
		}
		// Debug should be explicitly disabled in production
		if cfg.Security.InputMaxLength == 0 {
			cfg.Security.InputMaxLength = 4096
		}
	}

	slog.Debug("Applied environment defaults",
		"environment", env,
		"log_level", cfg.Server.LogLevel,
		"debug", cfg.Server.Debug,
	)
}

// ValidateProductionConfig validates that production settings are appropriate.
// Returns an error if the configuration is unsafe for production.
func ValidateProductionConfig(cfg *NuimanBotConfig) error {
	if !cfg.Server.Environment.IsProduction() {
		return nil // Only validate production configs
	}

	// Check for debug mode in production
	if cfg.Server.Debug {
		return fmt.Errorf("debug mode must be disabled in production")
	}

	// Warn about debug log level
	if cfg.Server.LogLevel == "debug" {
		slog.Warn("Debug log level in production may expose sensitive information")
	}

	// Check encryption key strength
	if len(cfg.Security.EncryptionKey) < 32 {
		return fmt.Errorf("encryption key must be at least 32 characters in production (got %d)", len(cfg.Security.EncryptionKey))
	}

	// Validate input limits
	if cfg.Security.InputMaxLength > 8192 {
		slog.Warn("Input max length is unusually high for production",
			"value", cfg.Security.InputMaxLength,
		)
	}

	slog.Info("Production configuration validated successfully")
	return nil
}
