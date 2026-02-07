package plugin

import (
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// PluginSecurity handles plugin security validation
type PluginSecurity struct {
	// Add security policies here
}

// NewPluginSecurity creates a new security validator
func NewPluginSecurity() *PluginSecurity {
	return &PluginSecurity{}
}

// ValidateManifest checks if manifest is safe
func (s *PluginSecurity) ValidateManifest(manifest *domain.PluginManifest) error {
	// Check namespace doesn't contain reserved words
	reserved := []string{"system", "core", "internal", "admin"}
	ns := strings.ToLower(manifest.Namespace)

	for _, word := range reserved {
		if strings.Contains(ns, word) {
			return fmt.Errorf("namespace cannot contain reserved word: %s", word)
		}
	}

	// Validate dependencies don't create cycles (simplified check)
	if len(manifest.Dependencies) > 50 {
		return fmt.Errorf("too many dependencies (max 50)")
	}

	return nil
}

// CheckPermissions validates plugin permissions (placeholder)
func (s *PluginSecurity) CheckPermissions(plugin *domain.Plugin) error {
	// Placeholder for future permission checks
	return nil
}
