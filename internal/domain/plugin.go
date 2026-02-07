package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PluginNamespace represents a plugin namespace in org/skill-name format
type PluginNamespace string

// Validate checks if the namespace follows org/skill-name format
func (n PluginNamespace) Validate() error {
	s := string(n)

	// Must contain exactly one slash
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return fmt.Errorf("namespace must be in org/skill-name format")
	}

	org, skill := parts[0], parts[1]

	// Both parts must be non-empty
	if org == "" || skill == "" {
		return fmt.Errorf("org and skill-name cannot be empty")
	}

	// Must be lowercase alphanumeric with dashes/underscores
	validPattern := regexp.MustCompile(`^[a-z0-9_-]+$`)

	if !validPattern.MatchString(org) {
		return fmt.Errorf("org must be lowercase alphanumeric with dashes/underscores")
	}

	if !validPattern.MatchString(skill) {
		return fmt.Errorf("skill-name must be lowercase alphanumeric with dashes/underscores")
	}

	return nil
}

// Org returns the organization part of the namespace
func (n PluginNamespace) Org() string {
	parts := strings.Split(string(n), "/")
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// SkillName returns the skill name part of the namespace
func (n PluginNamespace) SkillName() string {
	parts := strings.Split(string(n), "/")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// String returns the namespace as a string
func (n PluginNamespace) String() string {
	return string(n)
}

// PluginState represents the state of a plugin
type PluginState string

const (
	PluginStateInstalled PluginState = "installed"
	PluginStateDisabled  PluginState = "disabled"
	PluginStateUpdating  PluginState = "updating"
)

// PluginManifest represents the plugin.yaml manifest
type PluginManifest struct {
	// Namespace is the unique identifier (org/skill-name)
	Namespace string `yaml:"namespace"`

	// Version is the semver version
	Version string `yaml:"version"`

	// Description is the plugin description
	Description string `yaml:"description"`

	// Author is the plugin author
	Author string `yaml:"author,omitempty"`

	// Homepage is the plugin homepage URL
	Homepage string `yaml:"homepage,omitempty"`

	// Repository is the source repository URL
	Repository string `yaml:"repository,omitempty"`

	// License is the license identifier (e.g., MIT, Apache-2.0)
	License string `yaml:"license,omitempty"`

	// Skills is the list of skill names provided by this plugin
	Skills []string `yaml:"skills,omitempty"`

	// Dependencies maps plugin namespaces to version constraints
	Dependencies map[string]string `yaml:"dependencies,omitempty"`

	// MinNuimanBotVersion is the minimum required NuimanBot version
	MinNuimanBotVersion string `yaml:"min-nuimanbot-version,omitempty"`
}

// Validate checks if the manifest is valid
func (m *PluginManifest) Validate() error {
	if m.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	ns := PluginNamespace(m.Namespace)
	if err := ns.Validate(); err != nil {
		return fmt.Errorf("invalid namespace: %w", err)
	}

	if m.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Basic semver validation (x.y.z format)
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	if !semverPattern.MatchString(m.Version) {
		return fmt.Errorf("version must be valid semver (e.g., 1.0.0)")
	}

	return nil
}

// Plugin represents an installed plugin
type Plugin struct {
	// Namespace is the unique identifier
	Namespace PluginNamespace

	// Manifest is the parsed plugin.yaml
	Manifest PluginManifest

	// State is the current plugin state
	State PluginState

	// InstalledAt is when the plugin was installed
	InstalledAt time.Time

	// UpdatedAt is when the plugin was last updated
	UpdatedAt time.Time

	// Path is the local filesystem path to the plugin
	Path string

	// Source is the original source (e.g., GitHub URL)
	Source string
}

// IsInstalled returns true if the plugin is installed and enabled
func (p *Plugin) IsInstalled() bool {
	return p.State == PluginStateInstalled
}

// IsDisabled returns true if the plugin is disabled
func (p *Plugin) IsDisabled() bool {
	return p.State == PluginStateDisabled
}

// GetDependencies returns the plugin's dependencies
func (p *Plugin) GetDependencies() map[string]string {
	if p.Manifest.Dependencies == nil {
		return make(map[string]string)
	}
	return p.Manifest.Dependencies
}

// PluginRegistry defines the interface for plugin management
type PluginRegistry interface {
	// Install installs a plugin from a source (e.g., GitHub URL, local path)
	Install(namespace PluginNamespace, source string) error

	// Uninstall removes a plugin
	Uninstall(namespace PluginNamespace) error

	// Get retrieves a plugin by namespace
	Get(namespace PluginNamespace) (*Plugin, error)

	// List returns all installed plugins
	List() ([]*Plugin, error)

	// Enable enables a disabled plugin
	Enable(namespace PluginNamespace) error

	// Disable disables a plugin without uninstalling
	Disable(namespace PluginNamespace) error
}
