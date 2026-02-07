package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"nuimanbot/internal/domain"
)

// PluginDiscovery scans and catalogs plugins
type PluginDiscovery struct{}

// NewPluginDiscovery creates a new plugin discovery
func NewPluginDiscovery() *PluginDiscovery {
	return &PluginDiscovery{}
}

// Scan scans a directory for plugins
func (d *PluginDiscovery) Scan(ctx context.Context, pluginDir string) ([]*domain.Plugin, error) {
	var plugins []*domain.Plugin
	seen := make(map[domain.PluginNamespace]bool)

	// Read directory entries
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Look for plugin.yaml
		manifestPath := filepath.Join(pluginDir, entry.Name(), "plugin.yaml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			// Skip directories without plugin.yaml
			continue
		}

		// Parse manifest
		manifest, err := d.ParseManifest(manifestPath)
		if err != nil {
			// Skip invalid manifests
			continue
		}

		// Validate manifest
		if err := manifest.Validate(); err != nil {
			// Skip invalid manifests
			continue
		}

		// Check for namespace collision
		ns := domain.PluginNamespace(manifest.Namespace)
		if seen[ns] {
			return nil, fmt.Errorf("namespace collision detected: %s", ns)
		}
		seen[ns] = true

		// Create plugin
		plugin := &domain.Plugin{
			Namespace: ns,
			Manifest:  *manifest,
			State:     domain.PluginStateInstalled,
			Path:      filepath.Join(pluginDir, entry.Name()),
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// ParseManifest parses a plugin.yaml file
func (d *PluginDiscovery) ParseManifest(path string) (*domain.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest domain.PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// BuildCatalog builds a catalog of all plugins in a directory
func (d *PluginDiscovery) BuildCatalog(ctx context.Context, pluginDir string) (map[domain.PluginNamespace]*domain.Plugin, error) {
	plugins, err := d.Scan(ctx, pluginDir)
	if err != nil {
		return nil, err
	}

	catalog := make(map[domain.PluginNamespace]*domain.Plugin)
	for _, plugin := range plugins {
		catalog[plugin.Namespace] = plugin
	}

	return catalog, nil
}
