package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"

	"nuimanbot/internal/domain"
)

// decodeNetworkAccessOnly mimics loader.go's real decode path (same
// mapstructure.DecoderConfig shape: TagName "yaml", the same DecodeHooks,
// WeaklyTypedInput true) but scoped to a minimal wrapper struct, so this
// test proves the nil-vs-empty Allowlist distinction (spec.md Edge Case
// #11) survives the actual decode mechanism used in production — not just
// the domain-type logic in isolation.
func decodeNetworkAccessOnly(t *testing.T, yamlContent string) NetworkAccessConfig {
	t.Helper()

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBufferString(yamlContent)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	var wrapper struct {
		NetworkAccess NetworkAccessConfig `yaml:"network_access"`
	}

	decoderConfig := &mapstructure.DecoderConfig{
		Result:  &wrapper,
		TagName: "yaml",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		WeaklyTypedInput: true,
	}
	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		t.Fatalf("NewDecoder: %v", err)
	}
	if err := decoder.Decode(v.AllSettings()); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return wrapper.NetworkAccess
}

func TestNetworkAccessConfig_Decode_AbsentAllowlistIsNil(t *testing.T) {
	cfg := decodeNetworkAccessOnly(t, `
network_access:
  mode: remote
  bind_address: "0.0.0.0:8443"
`)
	if cfg.Allowlist != nil {
		t.Fatalf("expected nil Allowlist for an absent key through the real decode path, got %#v", cfg.Allowlist)
	}
	if !cfg.ToDomain().IsAllowed("203.0.113.9") {
		t.Fatal("expected absent allowlist to allow all sources (via ToDomain)")
	}
}

func TestNetworkAccessConfig_Decode_EmptyAllowlistIsNonNilEmpty(t *testing.T) {
	cfg := decodeNetworkAccessOnly(t, `
network_access:
  mode: remote
  allowlist: []
`)
	if cfg.Allowlist == nil {
		t.Fatal("expected non-nil (empty) Allowlist for an explicitly empty list through the real decode path")
	}
	if len(cfg.Allowlist) != 0 {
		t.Fatalf("expected zero-length Allowlist, got %v", cfg.Allowlist)
	}
	if cfg.ToDomain().IsAllowed("203.0.113.9") {
		t.Fatal("expected explicitly empty allowlist to deny all sources (fail-closed, via ToDomain)")
	}
}

func TestNetworkAccessConfig_Decode_PopulatedAllowlist(t *testing.T) {
	cfg := decodeNetworkAccessOnly(t, `
network_access:
  mode: remote
  allowlist:
    - "203.0.113.9"
    - "trusted.example.com"
`)
	if len(cfg.Allowlist) != 2 {
		t.Fatalf("expected 2 allowlist entries, got %v", cfg.Allowlist)
	}
	d := cfg.ToDomain()
	if !d.IsAllowed("203.0.113.9") || d.IsAllowed("evil.example.com") {
		t.Fatalf("unexpected allowlist evaluation: %+v", d)
	}
}

func TestNetworkAccessConfig_ToDomain_ModeDefaultsToLocalhostOnly(t *testing.T) {
	cases := []string{"", "bogus", "REMOTE", "Remote"}
	for _, mode := range cases {
		c := NetworkAccessConfig{Mode: mode}
		d := c.ToDomain()
		if d.Mode != domain.AccessModeLocalhostOnly {
			t.Errorf("mode %q: expected fail-safe default to AccessModeLocalhostOnly, got %v", mode, d.Mode)
		}
	}
}

func TestNetworkAccessConfig_ToDomain_RemoteModeRecognized(t *testing.T) {
	c := NetworkAccessConfig{Mode: "remote"}
	if c.ToDomain().Mode != domain.AccessModeRemote {
		t.Fatal("expected exact string 'remote' to map to AccessModeRemote")
	}
}

func TestDefaultNetworkAccessConfig_IsLocalhostOnlyNoAllowlist(t *testing.T) {
	c := DefaultNetworkAccessConfig()
	d := c.ToDomain()
	if d.Mode != domain.AccessModeLocalhostOnly {
		t.Fatalf("expected default mode to be localhost-only, got %v", d.Mode)
	}
	if d.Allowlist != nil {
		t.Fatalf("expected default allowlist to be nil, got %v", d.Allowlist)
	}
}

func TestWorkerPoolConfig_ToDomain_DefaultsWhenUnset(t *testing.T) {
	cases := []int{0, -5}
	for _, n := range cases {
		c := WorkerPoolConfig{MaxConcurrentWorkers: n}
		d := c.ToDomain()
		if d.MaxConcurrentWorkers != DefaultWorkerPoolSize {
			t.Errorf("n=%d: expected default %d, got %d", n, DefaultWorkerPoolSize, d.MaxConcurrentWorkers)
		}
		if err := d.Validate(); err != nil {
			t.Errorf("n=%d: expected default to be valid, got %v", n, err)
		}
	}
}

func TestWorkerPoolConfig_ToDomain_PositiveValuePassedThrough(t *testing.T) {
	c := WorkerPoolConfig{MaxConcurrentWorkers: 7}
	if got := c.ToDomain().MaxConcurrentWorkers; got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestDefaultRetentionDefaultsConfig_MatchesPlanPhase4(t *testing.T) {
	c := DefaultRetentionDefaultsConfig()
	if c.ChatDays != 90 || c.ProjectDays != 180 || c.HistoryDays != 90 {
		t.Fatalf("expected plan.md Phase 4 defaults (90/180/90), got %+v", c)
	}
}

func TestToDomainRetentionPolicy(t *testing.T) {
	if p := ToDomainRetentionPolicy(0); !p.IsNever() {
		t.Fatal("expected 0 days to mean Never")
	}
	p := ToDomainRetentionPolicy(90)
	if p.IsNever() {
		t.Fatal("expected 90 days to not be Never")
	}
	if *p.Period != 90*24*time.Hour {
		t.Fatalf("expected 90 days as a duration, got %v", *p.Period)
	}
}
