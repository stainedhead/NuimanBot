package config_test

import (
	"testing"

	"nuimanbot/internal/config"
)

// TestToolsSystemConfig_HasPermissionsField covers P3.4: the `tools.permissions`
// config section (map of tool name -> role string) that
// internal/usecase/tool.Service.resolveRequiredRole layers as an override on
// top of the code-level internal/usecase/tool.ToolPermissions defaults.
func TestToolsSystemConfig_HasPermissionsField(t *testing.T) {
	cfg := config.ToolsSystemConfig{
		Permissions: map[string]string{
			"github":       "user",
			"coding_agent": "admin",
		},
	}

	if got, want := cfg.Permissions["github"], "user"; got != want {
		t.Errorf("cfg.Permissions[%q] = %q, want %q", "github", got, want)
	}
	if got, want := cfg.Permissions["coding_agent"], "admin"; got != want {
		t.Errorf("cfg.Permissions[%q] = %q, want %q", "coding_agent", got, want)
	}
}

// TestToolsSystemConfig_PermissionsFieldDefaultsToNil ensures an unset
// `tools.permissions` section doesn't panic on lookup and behaves as "no
// override configured" (nil map reads return the zero value, ok=false).
func TestToolsSystemConfig_PermissionsFieldDefaultsToNil(t *testing.T) {
	cfg := config.ToolsSystemConfig{}

	if cfg.Permissions != nil {
		t.Errorf("expected Permissions to default to nil, got %#v", cfg.Permissions)
	}
	if _, ok := cfg.Permissions["github"]; ok {
		t.Error("expected lookup on nil Permissions map to report ok=false")
	}
}
