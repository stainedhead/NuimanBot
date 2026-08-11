package domain

// AccessMode selects whether the web admin binds to localhost only or
// accepts remote connections (FR-005/FR-006).
type AccessMode string

const (
	// AccessModeLocalhostOnly binds 127.0.0.1 only; no remote connections
	// are possible regardless of Allowlist.
	AccessModeLocalhostOnly AccessMode = "localhost_only"
	// AccessModeRemote binds a configured interface; Allowlist (if any)
	// governs which remote sources may connect.
	AccessModeRemote AccessMode = "remote"
)

// NetworkAccessConfig configures the web admin's network exposure
// (FR-005–FR-008).
//
// Allowlist has a deliberate three-state representation, preserved through
// config parsing rather than collapsed (spec.md Edge Case #11):
//   - Allowlist == nil: no allowlist configured — allow all sources (open
//     remote access; an explicit admin choice).
//   - Allowlist != nil && len(Allowlist) == 0: an explicitly empty
//     allowlist — deny all sources (fail-closed).
//   - Allowlist != nil && len(Allowlist) > 0: only listed entries allowed.
type NetworkAccessConfig struct {
	Mode        AccessMode
	BindAddress string
	Allowlist   []string
}

// localhostHosts are the hosts always treated as "local" regardless of
// Allowlist, used to enforce AccessModeLocalhostOnly.
var localhostHosts = map[string]bool{
	"127.0.0.1": true,
	"::1":       true,
	"localhost": true,
}

// IsAllowed reports whether host is permitted to connect under this config.
//
// In AccessModeLocalhostOnly, only loopback hosts are permitted — this is a
// defense-in-depth check; the primary enforcement point is that the
// listener itself binds only to 127.0.0.1 (see internal/config /
// cmd/nuimanbot wiring), so a non-loopback request should never reach this
// check in practice.
//
// In AccessModeRemote, the three-state Allowlist semantics documented on
// the struct apply.
func (c NetworkAccessConfig) IsAllowed(host string) bool {
	if c.Mode == AccessModeLocalhostOnly {
		return localhostHosts[host]
	}

	if c.Allowlist == nil {
		return true
	}

	for _, entry := range c.Allowlist {
		if entry == host {
			return true
		}
	}
	return false
}

// HasAllowlist reports whether an allowlist is configured at all (nil vs.
// non-nil), independent of whether it is empty. Useful for config-validation
// warnings/UI messaging that need to distinguish "no allowlist" from "deny
// all" without duplicating the nil check inline.
func (c NetworkAccessConfig) HasAllowlist() bool {
	return c.Allowlist != nil
}
