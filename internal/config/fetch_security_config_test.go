package config_test

import (
	"testing"

	"nuimanbot/internal/config"
)

func TestFetchSecurityConfig_SSRFProtectionEnabled_DefaultsTrueWhenUnset(t *testing.T) {
	cfg := config.FetchSecurityConfig{}
	if !cfg.SSRFProtectionEnabled() {
		t.Error("expected SSRFProtectionEnabled() to default to true when unset (nil)")
	}
}

func TestFetchSecurityConfig_SSRFProtectionEnabled_ExplicitFalse(t *testing.T) {
	disabled := false
	cfg := config.FetchSecurityConfig{SSRFProtection: &disabled}
	if cfg.SSRFProtectionEnabled() {
		t.Error("expected SSRFProtectionEnabled() to be false when explicitly disabled")
	}
}

func TestFetchSecurityConfig_SSRFProtectionEnabled_ExplicitTrue(t *testing.T) {
	enabled := true
	cfg := config.FetchSecurityConfig{SSRFProtection: &enabled}
	if !cfg.SSRFProtectionEnabled() {
		t.Error("expected SSRFProtectionEnabled() to be true when explicitly enabled")
	}
}

func TestFetchSecurityConfig_FollowRedirectsEnabled_DefaultsTrueWhenUnset(t *testing.T) {
	cfg := config.FetchSecurityConfig{}
	if !cfg.FollowRedirectsEnabled() {
		t.Error("expected FollowRedirectsEnabled() to default to true when unset (nil)")
	}
}

func TestFetchSecurityConfig_FollowRedirectsEnabled_ExplicitFalse(t *testing.T) {
	disabled := false
	cfg := config.FetchSecurityConfig{FollowRedirects: &disabled}
	if cfg.FollowRedirectsEnabled() {
		t.Error("expected FollowRedirectsEnabled() to be false when explicitly disabled")
	}
}

func TestFetchSecurityConfig_FollowRedirectsEnabled_ExplicitTrue(t *testing.T) {
	enabled := true
	cfg := config.FetchSecurityConfig{FollowRedirects: &enabled}
	if !cfg.FollowRedirectsEnabled() {
		t.Error("expected FollowRedirectsEnabled() to be true when explicitly enabled")
	}
}

func TestSecurityConfig_HasFetchField(t *testing.T) {
	ssrf := false
	redirects := false
	cfg := config.SecurityConfig{
		Fetch: config.FetchSecurityConfig{
			SSRFProtection:  &ssrf,
			FollowRedirects: &redirects,
		},
	}
	if cfg.Fetch.SSRFProtectionEnabled() {
		t.Error("expected explicitly disabled SSRFProtection to report SSRFProtectionEnabled()=false")
	}
	if cfg.Fetch.FollowRedirectsEnabled() {
		t.Error("expected explicitly disabled FollowRedirects to report FollowRedirectsEnabled()=false")
	}
}
