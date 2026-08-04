package config_test

import (
	"testing"

	"nuimanbot/internal/config"
)

func TestToolOutputValidationConfig_IsEnabled_DefaultsTrueWhenUnset(t *testing.T) {
	cfg := config.ToolOutputValidationConfig{}
	if !cfg.IsEnabled() {
		t.Error("expected IsEnabled() to default to true when Enabled is unset (nil)")
	}
}

func TestToolOutputValidationConfig_IsEnabled_ExplicitFalse(t *testing.T) {
	disabled := false
	cfg := config.ToolOutputValidationConfig{Enabled: &disabled}
	if cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be false when explicitly disabled")
	}
}

func TestToolOutputValidationConfig_IsEnabled_ExplicitTrue(t *testing.T) {
	enabled := true
	cfg := config.ToolOutputValidationConfig{Enabled: &enabled}
	if !cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be true when explicitly enabled")
	}
}

func TestToolOutputValidationConfig_ResolvedAction_DefaultsToReject(t *testing.T) {
	cfg := config.ToolOutputValidationConfig{}
	if got := cfg.ResolvedAction(); got != "reject" {
		t.Errorf("expected default action %q, got %q", "reject", got)
	}
}

func TestToolOutputValidationConfig_ResolvedAction_UnrecognizedValueDefaultsToReject(t *testing.T) {
	cfg := config.ToolOutputValidationConfig{Action: "something-else"}
	if got := cfg.ResolvedAction(); got != "reject" {
		t.Errorf("expected unrecognized action to fail closed to %q, got %q", "reject", got)
	}
}

func TestToolOutputValidationConfig_ResolvedAction_Annotate(t *testing.T) {
	cfg := config.ToolOutputValidationConfig{Action: "annotate"}
	if got := cfg.ResolvedAction(); got != "annotate" {
		t.Errorf("expected action %q, got %q", "annotate", got)
	}
}

func TestSecurityConfig_HasToolOutputValidationField(t *testing.T) {
	action := "annotate"
	enabled := false
	cfg := config.SecurityConfig{
		ToolOutputValidation: config.ToolOutputValidationConfig{
			Enabled: &enabled,
			Action:  action,
		},
	}
	if cfg.ToolOutputValidation.IsEnabled() {
		t.Error("expected explicitly disabled config to report IsEnabled()=false")
	}
	if cfg.ToolOutputValidation.ResolvedAction() != "annotate" {
		t.Errorf("expected ResolvedAction()=%q, got %q", "annotate", cfg.ToolOutputValidation.ResolvedAction())
	}
}
