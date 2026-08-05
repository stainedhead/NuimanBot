package config

import (
	"time"

	"nuimanbot/internal/domain"
)

// NetworkAccessConfig is the config.yaml/env-loadable form of
// domain.NetworkAccessConfig (FR-005–FR-008).
//
// Allowlist preserves the same nil-vs-empty distinction as its domain
// counterpart (spec.md Edge Case #11): an absent `allowlist` key in
// config.yaml decodes to a nil slice ("allow all"); an explicitly empty
// `allowlist: []` decodes to a non-nil, zero-length slice ("deny all",
// fail-closed). This works because internal/config/loader.go's mapstructure
// decoder is fed viper's AllSettings() map — a key that never appears in
// that map leaves the target field's Go zero value (nil) untouched, while a
// key present with an empty YAML list value decodes to an actual empty
// slice. See network_access_config_test.go for a decode-path test proving
// this holds through the real loader, not just at the domain-type level.
type NetworkAccessConfig struct {
	// Mode is "localhost_only" or "remote". Empty/unrecognized values must
	// be treated as "localhost_only" by ToDomain (fail-safe default: an
	// unrecognized mode must never fail open into remote access).
	Mode string `yaml:"mode"`
	// BindAddress is the interface to bind when Mode == "remote" (e.g.
	// "0.0.0.0:8443"). Ignored in localhost_only mode.
	BindAddress string `yaml:"bind_address"`
	// Allowlist is nil ("allow all") vs. explicitly empty ("deny all") —
	// see the type doc comment above.
	Allowlist []string `yaml:"allowlist"`
}

// ToDomain converts this config-layer struct to its domain.NetworkAccessConfig
// equivalent. An unrecognized/empty Mode defaults to AccessModeLocalhostOnly
// (fail-safe: a config typo must never silently open remote access).
func (c NetworkAccessConfig) ToDomain() domain.NetworkAccessConfig {
	mode := domain.AccessModeLocalhostOnly
	if c.Mode == string(domain.AccessModeRemote) {
		mode = domain.AccessModeRemote
	}
	return domain.NetworkAccessConfig{
		Mode:        mode,
		BindAddress: c.BindAddress,
		Allowlist:   c.Allowlist,
	}
}

// DefaultNetworkAccessConfig returns the safe-by-default config (Breaking
// Changes section, spec.md): localhost-only, no allowlist, so existing
// single-machine deployments are unaffected by upgrade without manual
// config edits.
func DefaultNetworkAccessConfig() NetworkAccessConfig {
	return NetworkAccessConfig{
		Mode: string(domain.AccessModeLocalhostOnly),
	}
}

// WorkerPoolConfig is the config.yaml/env-loadable form of
// domain.WorkerPoolConfig (FR-004, FR-039).
type WorkerPoolConfig struct {
	MaxConcurrentWorkers int `yaml:"max_concurrent_workers"`
}

// ToDomain converts to domain.WorkerPoolConfig. A non-positive
// MaxConcurrentWorkers (unset, or an invalid config value) resolves to
// DefaultWorkerPoolSize rather than a value domain.WorkerPoolConfig.Validate
// would reject, so a missing/blank config section doesn't stall the whole
// worker pool at startup.
func (c WorkerPoolConfig) ToDomain() domain.WorkerPoolConfig {
	n := c.MaxConcurrentWorkers
	if n <= 0 {
		n = DefaultWorkerPoolSize
	}
	return domain.WorkerPoolConfig{MaxConcurrentWorkers: n}
}

// DefaultWorkerPoolSize is plan.md Phase 4's pinned default: 3 concurrent
// workers (safe for a single-machine deployment without operator tuning;
// bounds concurrent LLM-API call exposure while avoiding one long Chore
// starving the Job queue).
const DefaultWorkerPoolSize = 3

// RetentionDefaultsConfig holds the per-user default retention windows
// (FR-003) an admin can override in Settings. A zero/absent value for any
// field means "Never" (see domain.RetentionPolicy), consistent with that
// value object's own zero-value semantics — NOT "expire immediately".
type RetentionDefaultsConfig struct {
	// ChatDays is the default Chat retention window in days (FR-014).
	// 0 means "Never". plan.md Phase 4 default: 90.
	ChatDays int `yaml:"chat_days"`
	// ProjectDays is the default Project retention window in days
	// (FR-023). 0 means "Never". plan.md Phase 4 default: 180.
	ProjectDays int `yaml:"project_days"`
	// HistoryDays is the default History (Job/Chore run) retention window
	// in days (FR-043). 0 means "Never". plan.md Phase 4 default: 90.
	HistoryDays int `yaml:"history_days"`
}

// plan.md Phase 4's pinned default retention windows, in days.
const (
	DefaultChatRetentionDays    = 90
	DefaultProjectRetentionDays = 180
	DefaultHistoryRetentionDays = 90
)

// DefaultRetentionDefaultsConfig returns plan.md Phase 4's pinned defaults.
func DefaultRetentionDefaultsConfig() RetentionDefaultsConfig {
	return RetentionDefaultsConfig{
		ChatDays:    DefaultChatRetentionDays,
		ProjectDays: DefaultProjectRetentionDays,
		HistoryDays: DefaultHistoryRetentionDays,
	}
}

// ToDomainRetentionPolicy converts a days value (0 == Never) to a
// domain.RetentionPolicy, using the given default (in days) when days == 0
// AND useDefaultForZero is true — callers distinguish "unset, use the
// system default" from "explicitly set to Never" at a higher layer (the
// per-user Settings usecase), since this config struct alone cannot tell
// the two apart once decoded (a config file that never mentions the key and
// one that sets it to 0 are indistinguishable in a plain int field).
func ToDomainRetentionPolicy(days int) domain.RetentionPolicy {
	if days <= 0 {
		return domain.NeverExpire()
	}
	return domain.NewRetentionPolicy(daysToDuration(days))
}

// daysToDuration converts a whole number of days to a time.Duration.
func daysToDuration(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}
