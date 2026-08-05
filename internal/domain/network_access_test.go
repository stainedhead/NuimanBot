package domain

import "testing"

func TestNetworkAccessConfig_LocalhostOnly(t *testing.T) {
	c := NetworkAccessConfig{Mode: AccessModeLocalhostOnly}
	allowed := []string{"127.0.0.1", "::1", "localhost"}
	for _, h := range allowed {
		if !c.IsAllowed(h) {
			t.Errorf("expected %q to be allowed in localhost-only mode", h)
		}
	}
	denied := []string{"10.0.0.5", "203.0.113.9", "evil.example.com"}
	for _, h := range denied {
		if c.IsAllowed(h) {
			t.Errorf("expected %q to be denied in localhost-only mode", h)
		}
	}
}

func TestNetworkAccessConfig_Remote_NilAllowlistAllowsAll(t *testing.T) {
	c := NetworkAccessConfig{Mode: AccessModeRemote, Allowlist: nil}
	if !c.IsAllowed("203.0.113.9") {
		t.Fatal("expected absent (nil) allowlist to allow all sources")
	}
	if c.HasAllowlist() {
		t.Fatal("expected HasAllowlist() == false for nil allowlist")
	}
}

func TestNetworkAccessConfig_Remote_EmptyAllowlistDeniesAll(t *testing.T) {
	c := NetworkAccessConfig{Mode: AccessModeRemote, Allowlist: []string{}}
	if c.IsAllowed("203.0.113.9") {
		t.Fatal("expected explicitly empty allowlist to deny all sources (fail-closed)")
	}
	if !c.HasAllowlist() {
		t.Fatal("expected HasAllowlist() == true for an explicitly empty (non-nil) allowlist")
	}
}

func TestNetworkAccessConfig_Remote_PopulatedAllowlist(t *testing.T) {
	c := NetworkAccessConfig{Mode: AccessModeRemote, Allowlist: []string{"203.0.113.9", "trusted.example.com"}}
	if !c.IsAllowed("203.0.113.9") {
		t.Fatal("expected allowlisted IP to be allowed")
	}
	if !c.IsAllowed("trusted.example.com") {
		t.Fatal("expected allowlisted hostname to be allowed")
	}
	if c.IsAllowed("203.0.113.10") {
		t.Fatal("expected non-allowlisted IP to be denied")
	}
}
