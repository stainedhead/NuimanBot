// Package config provides infrastructure adapters for configuration loading.
package config

import (
	"errors"

	"nuimanbot/internal/config"
)

// Sentinel errors for testing.
var (
	errLoadFailed       = errors.New("config load failed")
	errValidationFailed = errors.New("config validation failed")
)

// ViperConfigLoaderAdapter adapts the config.LoadConfig and config.Validate
// functions to the ConfigLoader interface expected by ConfigManager.
//
// This adapter allows dependency injection of the load and validate functions,
// making it easy to test and swap implementations.
type ViperConfigLoaderAdapter struct {
	// LoadFunc loads the configuration. Defaults to config.LoadConfig if nil.
	LoadFunc func() (*config.NuimanBotConfig, error)

	// ValidateFunc validates the configuration. Defaults to config.Validate if nil.
	ValidateFunc func(*config.NuimanBotConfig) error
}

// NewViperConfigLoaderAdapter creates a ViperConfigLoaderAdapter that uses
// the standard config.LoadConfig and config.Validate functions.
func NewViperConfigLoaderAdapter() *ViperConfigLoaderAdapter {
	return &ViperConfigLoaderAdapter{
		LoadFunc: func() (*config.NuimanBotConfig, error) {
			return config.LoadConfig()
		},
		ValidateFunc: config.Validate,
	}
}

// Load loads the configuration using the configured LoadFunc.
func (a *ViperConfigLoaderAdapter) Load() (*config.NuimanBotConfig, error) {
	return a.LoadFunc()
}

// Validate validates the configuration using the configured ValidateFunc.
func (a *ViperConfigLoaderAdapter) Validate(cfg *config.NuimanBotConfig) error {
	return a.ValidateFunc(cfg)
}
