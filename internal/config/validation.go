package config

import (
	"errors"
	"fmt"
	"strings"
)

// Validate performs comprehensive validation of the entire configuration.
// It checks all required fields, validates ranges, and enforces environment-specific rules.
// Returns an error that combines all validation failures.
func Validate(cfg *NuimanBotConfig) error {
	if cfg == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	var errs []error

	// Validate Server config
	if err := validateServer(&cfg.Server); err != nil {
		errs = append(errs, err)
	}

	// Validate Security config
	if err := validateSecurity(&cfg.Security, cfg.Server.Environment); err != nil {
		errs = append(errs, err)
	}

	// Validate Storage config
	if err := validateStorage(&cfg.Storage); err != nil {
		errs = append(errs, err)
	}

	// If there are errors, combine them
	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed: %w", errors.Join(errs...))
	}

	return nil
}

// validateServer validates server configuration.
func validateServer(cfg *ServerConfig) error {
	var errs []error

	// Validate log level (empty is allowed, will use environment default)
	if cfg.LogLevel != "" {
		validLogLevels := []string{"debug", "info", "warn", "error"}
		if !isOneOf(cfg.LogLevel, validLogLevels) {
			errs = append(errs, fmt.Errorf("server.log_level must be one of: %s", strings.Join(validLogLevels, ", ")))
		}
	}

	return joinErrors(errs)
}

// validateSecurity validates security configuration.
func validateSecurity(cfg *SecurityConfig, env Environment) error {
	var errs []error

	// Validate encryption key
	if cfg.EncryptionKey == "" {
		errs = append(errs, fmt.Errorf("security.encryption_key is required"))
	} else if env.IsProduction() && len(cfg.EncryptionKey) < 32 {
		errs = append(errs, fmt.Errorf("security.encryption_key must be at least 32 characters in production"))
	}

	// Validate input max length (0 means use environment default)
	switch {
	case cfg.InputMaxLength < 0:
		errs = append(errs, fmt.Errorf("security.input_max_length cannot be negative"))
	case cfg.InputMaxLength > 0 && cfg.InputMaxLength < 100:
		errs = append(errs, fmt.Errorf("security.input_max_length must be at least 100 bytes (or 0 for default)"))
	case cfg.InputMaxLength > 1048576:
		errs = append(errs, fmt.Errorf("security.input_max_length cannot exceed 1MB (1048576 bytes)"))
	}

	return joinErrors(errs)
}

// validateStorage validates storage configuration.
func validateStorage(cfg *StorageConfig) error {
	var errs []error

	// Validate storage type
	validTypes := []string{"sqlite", "postgres", "memory"}
	if cfg.Type == "" {
		errs = append(errs, fmt.Errorf("storage.type is required"))
	} else if !isOneOf(cfg.Type, validTypes) {
		errs = append(errs, fmt.Errorf("storage.type must be one of: %s", strings.Join(validTypes, ", ")))
	}

	// Validate storage path or DSN (required for database storage types)
	requiresPath := cfg.Type == "sqlite" || cfg.Type == "postgres"
	if requiresPath && cfg.Path == "" && cfg.DSN == "" {
		errs = append(errs, fmt.Errorf("storage.path or storage.dsn is required for %s storage", cfg.Type))
	}

	return joinErrors(errs)
}

// Helper functions

// isOneOf checks if a value is in a list of valid values.
func isOneOf(value string, validValues []string) bool {
	for _, v := range validValues {
		if value == v {
			return true
		}
	}
	return false
}

// joinErrors combines multiple errors into one, or returns nil if no errors.
func joinErrors(errs []error) error {
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
