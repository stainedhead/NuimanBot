package config_test

import (
	"strings"
	"testing"

	"nuimanbot/internal/config"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "info",
		},
		Security: config.SecurityConfig{
			InputMaxLength: 4096,
			VaultPath:      "/tmp/vault.db",
			EncryptionKey:  "test-key-12345678901234567890123456",
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "/tmp/storage.db",
		},
	}

	if err := config.Validate(cfg); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "invalid-level",
		},
		Security: config.SecurityConfig{
			EncryptionKey: "test-key-12345678901234567890123456",
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "/tmp/test.db",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid log level")
	}

	if !strings.Contains(err.Error(), "log_level") {
		t.Errorf("Expected error about log_level, got: %v", err)
	}
}

func TestValidate_MissingEncryptionKey(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "info",
		},
		Security: config.SecurityConfig{
			EncryptionKey: "",
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "/tmp/test.db",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Expected error for missing encryption key")
	}

	if !strings.Contains(err.Error(), "encryption_key") {
		t.Errorf("Expected error about encryption_key, got: %v", err)
	}
}

func TestValidate_InvalidInputMaxLength(t *testing.T) {
	tests := []struct {
		name      string
		maxLength int
		wantError bool
	}{
		{"zero length (use default)", 0, false},
		{"negative length", -1, true},
		{"too small", 99, true},
		{"minimum valid", 100, false},
		{"normal", 4096, false},
		{"large", 65536, false},
		{"too large", 1048577, true}, // > 1MB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NuimanBotConfig{
				Server: config.ServerConfig{
					Environment: config.EnvironmentDevelopment,
					LogLevel:    "info",
				},
				Security: config.SecurityConfig{
					InputMaxLength: tt.maxLength,
					EncryptionKey:  "test-key-12345678901234567890123456",
				},
				Storage: config.StorageConfig{
					Type: "sqlite",
					Path: "/tmp/test.db",
				},
			}

			err := config.Validate(cfg)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && !strings.Contains(err.Error(), "input_max_length") {
				t.Errorf("Expected error about input_max_length, got: %v", err)
			}
		})
	}
}

func TestValidate_InvalidStorageType(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "info",
		},
		Security: config.SecurityConfig{
			EncryptionKey: "test-key-12345678901234567890123456",
		},
		Storage: config.StorageConfig{
			Type: "invalid-storage",
			Path: "/tmp/test.db",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid storage type")
	}

	if !strings.Contains(err.Error(), "storage.type") {
		t.Errorf("Expected error about storage.type, got: %v", err)
	}
}

func TestValidate_MissingStoragePath(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "info",
		},
		Security: config.SecurityConfig{
			EncryptionKey: "test-key-12345678901234567890123456",
		},
		Storage: config.StorageConfig{
			Type: "sqlite",
			Path: "",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Expected error for missing storage path")
	}

	if !strings.Contains(err.Error(), "storage.path") {
		t.Errorf("Expected error about storage.path, got: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Server: config.ServerConfig{
			Environment: config.EnvironmentDevelopment,
			LogLevel:    "invalid",
		},
		Security: config.SecurityConfig{
			InputMaxLength: -1,
			EncryptionKey:  "",
		},
		Storage: config.StorageConfig{
			Type: "invalid",
			Path: "",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("Expected errors for multiple validation failures")
	}

	errStr := err.Error()

	// Should contain all error details
	expectedErrors := []string{
		"log_level",
		"input_max_length",
		"encryption_key",
		"storage.type",
	}

	for _, expected := range expectedErrors {
		if !strings.Contains(errStr, expected) {
			t.Errorf("Expected error string to contain %q, got: %v", expected, errStr)
		}
	}

	// Note: storage.path error is not checked because when storage.type is invalid,
	// the path validation may not run or may not be meaningful
}

func TestValidate_ProductionEncryptionKeyLength(t *testing.T) {
	tests := []struct {
		name      string
		keyLength int
		wantError bool
	}{
		{"too short for production", 16, true},
		{"barely too short", 31, true},
		{"minimum valid", 32, false},
		{"longer than minimum", 64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NuimanBotConfig{
				Server: config.ServerConfig{
					Environment: config.EnvironmentProduction,
					LogLevel:    "info",
				},
				Security: config.SecurityConfig{
					EncryptionKey: strings.Repeat("a", tt.keyLength),
				},
				Storage: config.StorageConfig{
					Type: "sqlite",
					Path: "/tmp/test.db",
				},
			}

			err := config.Validate(cfg)
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidate_NilConfig(t *testing.T) {
	err := config.Validate(nil)
	if err == nil {
		t.Fatal("Expected error for nil config")
	}

	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("Expected error about nil config, got: %v", err)
	}
}
