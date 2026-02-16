package config

import (
	"testing"

	"nuimanbot/internal/config"
)

func TestViperConfigLoaderAdapter_Load(t *testing.T) {
	t.Run("returns error when load fails", func(t *testing.T) {
		loadCalled := false
		adapter := &ViperConfigLoaderAdapter{
			LoadFunc: func() (*config.NuimanBotConfig, error) {
				loadCalled = true
				return nil, errLoadFailed
			},
		}

		cfg, err := adapter.Load()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if cfg != nil {
			t.Fatal("expected nil config on error")
		}
		if !loadCalled {
			t.Fatal("expected LoadFunc to be called")
		}
	})

	t.Run("returns config on success", func(t *testing.T) {
		expected := &config.NuimanBotConfig{
			Server: config.ServerConfig{LogLevel: "debug"},
		}
		adapter := &ViperConfigLoaderAdapter{
			LoadFunc: func() (*config.NuimanBotConfig, error) {
				return expected, nil
			},
		}

		cfg, err := adapter.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.LogLevel != "debug" {
			t.Errorf("expected LogLevel=debug, got %s", cfg.Server.LogLevel)
		}
	})
}

func TestViperConfigLoaderAdapter_Validate(t *testing.T) {
	t.Run("returns error when validation fails", func(t *testing.T) {
		validateCalled := false
		adapter := &ViperConfigLoaderAdapter{
			ValidateFunc: func(cfg *config.NuimanBotConfig) error {
				validateCalled = true
				return errValidationFailed
			},
		}

		err := adapter.Validate(&config.NuimanBotConfig{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !validateCalled {
			t.Fatal("expected ValidateFunc to be called")
		}
	})

	t.Run("returns nil on valid config", func(t *testing.T) {
		adapter := &ViperConfigLoaderAdapter{
			ValidateFunc: func(cfg *config.NuimanBotConfig) error {
				return nil
			},
		}

		err := adapter.Validate(&config.NuimanBotConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
