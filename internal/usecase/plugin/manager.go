package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"nuimanbot/internal/domain"
)

// PluginDiscoverer defines discovery interface
type PluginDiscoverer interface {
	Scan(ctx context.Context, dir string) ([]*domain.Plugin, error)
	ParseManifest(path string) (*domain.PluginManifest, error)
}

// PluginManager manages plugin installation and lifecycle
type PluginManager struct {
	pluginDir  string
	discoverer PluginDiscoverer
	plugins    map[domain.PluginNamespace]*domain.Plugin
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(pluginDir string, discoverer PluginDiscoverer) *PluginManager {
	return &PluginManager{
		pluginDir:  pluginDir,
		discoverer: discoverer,
		plugins:    make(map[domain.PluginNamespace]*domain.Plugin),
	}
}

// Install installs a plugin from source (simplified - just copies from local path)
func (m *PluginManager) Install(ctx context.Context, namespace domain.PluginNamespace, source string) error {
	// Check if already installed
	if _, exists := m.plugins[namespace]; exists {
		return fmt.Errorf("plugin %s already installed", namespace)
	}

	// Parse manifest from source
	manifestPath := filepath.Join(source, "plugin.yaml")
	manifest, err := m.discoverer.ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Validate namespace matches
	if domain.PluginNamespace(manifest.Namespace) != namespace {
		return fmt.Errorf("namespace mismatch: manifest has %s, requested %s", manifest.Namespace, namespace)
	}

	// Create plugin directory
	destDir := filepath.Join(m.pluginDir, namespace.Org()+"-"+namespace.SkillName())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy plugin.yaml (simplified - in real impl would copy all files)
	if err := m.copyFile(manifestPath, filepath.Join(destDir, "plugin.yaml")); err != nil {
		return fmt.Errorf("failed to copy manifest: %w", err)
	}

	// Register plugin
	plugin := &domain.Plugin{
		Namespace: namespace,
		Manifest:  *manifest,
		State:     domain.PluginStateInstalled,
		Path:      destDir,
		Source:    source,
	}
	m.plugins[namespace] = plugin

	return nil
}

// Uninstall removes a plugin
func (m *PluginManager) Uninstall(ctx context.Context, namespace domain.PluginNamespace) error {
	plugin, exists := m.plugins[namespace]
	if !exists {
		return fmt.Errorf("plugin %s not installed", namespace)
	}

	// Remove directory
	if err := os.RemoveAll(plugin.Path); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Unregister
	delete(m.plugins, namespace)

	return nil
}

// Get retrieves a plugin
func (m *PluginManager) Get(namespace domain.PluginNamespace) (*domain.Plugin, error) {
	plugin, exists := m.plugins[namespace]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", namespace)
	}
	return plugin, nil
}

// List returns all plugins
func (m *PluginManager) List() ([]*domain.Plugin, error) {
	plugins := make([]*domain.Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins, nil
}

// Enable enables a plugin
func (m *PluginManager) Enable(namespace domain.PluginNamespace) error {
	plugin, exists := m.plugins[namespace]
	if !exists {
		return fmt.Errorf("plugin %s not found", namespace)
	}
	plugin.State = domain.PluginStateInstalled
	return nil
}

// Disable disables a plugin
func (m *PluginManager) Disable(namespace domain.PluginNamespace) error {
	plugin, exists := m.plugins[namespace]
	if !exists {
		return fmt.Errorf("plugin %s not found", namespace)
	}
	plugin.State = domain.PluginStateDisabled
	return nil
}

// Initialize loads existing plugins
func (m *PluginManager) Initialize(ctx context.Context) error {
	plugins, err := m.discoverer.Scan(ctx, m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to scan plugins: %w", err)
	}

	for _, p := range plugins {
		m.plugins[p.Namespace] = p
	}

	return nil
}

// copyFile is a helper to copy a file (simplified implementation)
func (m *PluginManager) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
