package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

// TestPluginDiscovery_Scan tests scanning plugin directories
func TestPluginDiscovery_Scan(t *testing.T) {
	// Create temp plugin directory
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "acme-hello")

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}

	// Create plugin.yaml
	manifest := `namespace: acme/hello
version: 1.0.0
description: A test plugin
skills:
  - hello
  - world
`

	manifestFile := filepath.Join(pluginDir, "plugin.yaml")
	if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	// Scan
	discovery := NewPluginDiscovery()
	plugins, err := discovery.Scan(context.Background(), tmpDir)

	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(plugins) != 1 {
		t.Fatalf("Scan() found %d plugins, want 1", len(plugins))
	}

	plugin := plugins[0]
	if plugin.Namespace != "acme/hello" {
		t.Errorf("Namespace = %q, want 'acme/hello'", plugin.Namespace)
	}

	if plugin.Manifest.Version != "1.0.0" {
		t.Errorf("Version = %q, want '1.0.0'", plugin.Manifest.Version)
	}
}

// TestPluginDiscovery_MultiplePlugins tests scanning multiple plugins
func TestPluginDiscovery_MultiplePlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two plugins
	plugins := []struct {
		dir       string
		namespace string
	}{
		{"plugin1", "acme/plugin1"},
		{"plugin2", "acme/plugin2"},
	}

	for _, p := range plugins {
		pluginDir := filepath.Join(tmpDir, p.dir)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		manifest := "namespace: " + p.namespace + "\nversion: 1.0.0\ndescription: Test\n"
		manifestFile := filepath.Join(pluginDir, "plugin.yaml")
		if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
			t.Fatalf("Failed to write manifest: %v", err)
		}
	}

	discovery := NewPluginDiscovery()
	found, err := discovery.Scan(context.Background(), tmpDir)

	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(found) != 2 {
		t.Fatalf("Scan() found %d plugins, want 2", len(found))
	}
}

// TestPluginDiscovery_NamespaceCollision tests collision detection
func TestPluginDiscovery_NamespaceCollision(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two plugins with same namespace
	for i := 1; i <= 2; i++ {
		pluginDir := filepath.Join(tmpDir, "plugin"+string(rune('0'+i)))
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		manifest := "namespace: acme/duplicate\nversion: 1.0.0\ndescription: Test\n"
		manifestFile := filepath.Join(pluginDir, "plugin.yaml")
		if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
			t.Fatalf("Failed to write manifest: %v", err)
		}
	}

	discovery := NewPluginDiscovery()
	_, err := discovery.Scan(context.Background(), tmpDir)

	// Should detect collision
	if err == nil {
		t.Error("Scan() should error on namespace collision")
	}
}

// TestPluginDiscovery_InvalidManifest tests handling invalid manifests
func TestPluginDiscovery_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "invalid")

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Invalid YAML
	manifest := "invalid: yaml: syntax: ["
	manifestFile := filepath.Join(pluginDir, "plugin.yaml")
	if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	discovery := NewPluginDiscovery()
	plugins, err := discovery.Scan(context.Background(), tmpDir)

	// Should skip invalid plugins
	if err != nil {
		t.Fatalf("Scan() should skip invalid manifests, got error: %v", err)
	}

	if len(plugins) != 0 {
		t.Errorf("Scan() should skip invalid plugins, got %d", len(plugins))
	}
}

// TestPluginDiscovery_NoManifest tests directories without plugin.yaml
func TestPluginDiscovery_NoManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dir without manifest
	pluginDir := filepath.Join(tmpDir, "not-a-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	discovery := NewPluginDiscovery()
	plugins, err := discovery.Scan(context.Background(), tmpDir)

	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Should find no plugins
	if len(plugins) != 0 {
		t.Errorf("Scan() found %d plugins, want 0", len(plugins))
	}
}

// TestPluginDiscovery_ParseManifest tests manifest parsing
func TestPluginDiscovery_ParseManifest(t *testing.T) {
	manifest := `namespace: acme/hello-world
version: 1.2.3
description: A comprehensive plugin
author: ACME Corporation
homepage: https://acme.com/plugins/hello
repository: https://github.com/acme/hello-world-plugin
license: MIT
skills:
  - hello
  - world
  - greet
dependencies:
  acme/core: ^1.0.0
  acme/utils: ~2.1.0
min-nuimanbot-version: 0.5.0
`

	tmpFile := filepath.Join(t.TempDir(), "plugin.yaml")
	if err := os.WriteFile(tmpFile, []byte(manifest), 0644); err != nil {
		t.Fatalf("Failed to write manifest: %v", err)
	}

	discovery := NewPluginDiscovery()
	parsed, err := discovery.ParseManifest(tmpFile)

	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	if parsed.Namespace != "acme/hello-world" {
		t.Errorf("Namespace = %q, want 'acme/hello-world'", parsed.Namespace)
	}

	if parsed.Version != "1.2.3" {
		t.Errorf("Version = %q, want '1.2.3'", parsed.Version)
	}

	if len(parsed.Skills) != 3 {
		t.Errorf("Skills count = %d, want 3", len(parsed.Skills))
	}

	if len(parsed.Dependencies) != 2 {
		t.Errorf("Dependencies count = %d, want 2", len(parsed.Dependencies))
	}
}

// TestPluginDiscovery_BuildCatalog tests catalog building
func TestPluginDiscovery_BuildCatalog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple plugins
	for i := 1; i <= 3; i++ {
		pluginDir := filepath.Join(tmpDir, "plugin"+string(rune('0'+i)))
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		manifest := "namespace: acme/plugin" + string(rune('0'+i)) + "\nversion: 1.0.0\ndescription: Plugin " + string(rune('0'+i)) + "\n"
		manifestFile := filepath.Join(pluginDir, "plugin.yaml")
		if err := os.WriteFile(manifestFile, []byte(manifest), 0644); err != nil {
			t.Fatalf("Failed to write manifest: %v", err)
		}
	}

	discovery := NewPluginDiscovery()
	catalog, err := discovery.BuildCatalog(context.Background(), tmpDir)

	if err != nil {
		t.Fatalf("BuildCatalog() error = %v", err)
	}

	if len(catalog) != 3 {
		t.Fatalf("Catalog size = %d, want 3", len(catalog))
	}

	// Verify catalog contains all namespaces
	for i := 1; i <= 3; i++ {
		ns := domain.PluginNamespace("acme/plugin" + string(rune('0'+i)))
		if _, ok := catalog[ns]; !ok {
			t.Errorf("Catalog missing namespace %s", ns)
		}
	}
}
