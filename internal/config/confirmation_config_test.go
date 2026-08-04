package config_test

import (
	"testing"
	"time"

	"nuimanbot/internal/config"
)

func TestConfirmationConfig_IsEnabled_DefaultsTrueWhenUnset(t *testing.T) {
	cfg := config.ConfirmationConfig{}
	if !cfg.IsEnabled() {
		t.Error("expected IsEnabled() to default to true when unset (nil)")
	}
}

func TestConfirmationConfig_IsEnabled_ExplicitFalse(t *testing.T) {
	disabled := false
	cfg := config.ConfirmationConfig{Enabled: &disabled}
	if cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be false when explicitly disabled")
	}
}

func TestConfirmationConfig_IsEnabled_ExplicitTrue(t *testing.T) {
	enabled := true
	cfg := config.ConfirmationConfig{Enabled: &enabled}
	if !cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be true when explicitly enabled")
	}
}

func TestConfirmationConfig_ResolvedTimeout_DefaultsWhenEmpty(t *testing.T) {
	cfg := config.ConfirmationConfig{}
	if got := cfg.ResolvedTimeout(); got != config.DefaultConfirmationTimeout {
		t.Errorf("expected default timeout %v, got %v", config.DefaultConfirmationTimeout, got)
	}
}

func TestConfirmationConfig_ResolvedTimeout_DefaultsWhenUnparseable(t *testing.T) {
	cfg := config.ConfirmationConfig{Timeout: "not-a-duration"}
	if got := cfg.ResolvedTimeout(); got != config.DefaultConfirmationTimeout {
		t.Errorf("expected default timeout for unparseable value, got %v", got)
	}
}

func TestConfirmationConfig_ResolvedTimeout_ExplicitValue(t *testing.T) {
	cfg := config.ConfirmationConfig{Timeout: "10m"}
	if got := cfg.ResolvedTimeout(); got != 10*time.Minute {
		t.Errorf("expected 10m, got %v", got)
	}
}

func TestConfirmationConfig_ResolvedTimeout_DefaultsWhenNonPositive(t *testing.T) {
	cfg := config.ConfirmationConfig{Timeout: "0s"}
	if got := cfg.ResolvedTimeout(); got != config.DefaultConfirmationTimeout {
		t.Errorf("expected default timeout for zero duration, got %v", got)
	}
}

func TestConfirmationConfig_RequiresConfirmationByDefault_ToolAction(t *testing.T) {
	cfg := config.ConfirmationConfig{
		DefaultRequiredActions: []string{"github.pr_merge", "github.issue_close", "coding_agent.yolo_mode"},
	}

	cases := []struct {
		tool, action string
		want         bool
	}{
		{"github", "pr_merge", true},
		{"github", "issue_close", true},
		{"coding_agent", "yolo_mode", true},
		{"github", "issue_list", false},
		{"calculator", "", false},
	}

	for _, tc := range cases {
		if got := cfg.RequiresConfirmationByDefault(tc.tool, tc.action); got != tc.want {
			t.Errorf("RequiresConfirmationByDefault(%q, %q) = %v, want %v", tc.tool, tc.action, got, tc.want)
		}
	}
}

func TestConfirmationConfig_RequiresConfirmationByDefault_CaseInsensitive(t *testing.T) {
	cfg := config.ConfirmationConfig{DefaultRequiredActions: []string{"GitHub.PR_Merge"}}
	if !cfg.RequiresConfirmationByDefault("github", "pr_merge") {
		t.Error("expected case-insensitive match")
	}
}

func TestConfirmationConfig_RequiresConfirmationByDefault_WholeToolEntry(t *testing.T) {
	cfg := config.ConfirmationConfig{DefaultRequiredActions: []string{"coding_agent"}}
	if !cfg.RequiresConfirmationByDefault("coding_agent", "") {
		t.Error("expected bare tool-name entry to match when action is empty")
	}
	if cfg.RequiresConfirmationByDefault("coding_agent", "yolo_mode") {
		t.Error("a bare tool-name entry should not match a specific action (use \"coding_agent.yolo_mode\" for that)")
	}
}

func TestConfirmationConfig_RequiresConfirmationByDefault_DisabledAlwaysFalse(t *testing.T) {
	disabled := false
	cfg := config.ConfirmationConfig{
		Enabled:                &disabled,
		DefaultRequiredActions: []string{"github.pr_merge"},
	}
	if cfg.RequiresConfirmationByDefault("github", "pr_merge") {
		t.Error("expected RequiresConfirmationByDefault to be false when the subsystem is disabled")
	}
}

func TestSecurityConfig_HasConfirmationField(t *testing.T) {
	cfg := config.SecurityConfig{
		Confirmation: config.ConfirmationConfig{
			Timeout:                "5m",
			DefaultRequiredActions: []string{"github.pr_merge"},
		},
	}
	if !cfg.Confirmation.RequiresConfirmationByDefault("github", "pr_merge") {
		t.Error("expected SecurityConfig.Confirmation to be wired and functional")
	}
}
