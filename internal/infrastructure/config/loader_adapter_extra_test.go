package config

import (
	"testing"
)

// TestNewViperConfigLoaderAdapter tests the constructor.
// Note: The actual config.LoadConfig will likely fail without a config file,
// so we just verify the constructor creates a functional adapter.
func TestNewViperConfigLoaderAdapter_Constructor(t *testing.T) {
	adapter := NewViperConfigLoaderAdapter()
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}
	if adapter.LoadFunc == nil {
		t.Fatal("Expected non-nil LoadFunc")
	}
	if adapter.ValidateFunc == nil {
		t.Fatal("Expected non-nil ValidateFunc")
	}
}

// TestNewViperConfigLoaderAdapter_Load tests that Load is callable.
func TestNewViperConfigLoaderAdapter_Load(t *testing.T) {
	adapter := NewViperConfigLoaderAdapter()

	// Load will likely return an error since there's no config file,
	// but it should not panic
	_, _ = adapter.Load()
}
