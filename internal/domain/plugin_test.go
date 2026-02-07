package domain

import (
	"testing"
)

// TestPluginNamespace_Validation tests namespace format validation
func TestPluginNamespace_Validation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{"valid org/skill", "acme/hello-world", false},
		{"valid with dash", "my-org/my-skill", false},
		{"valid with underscore", "my_org/my_skill", false},
		{"invalid no slash", "acme-hello-world", true},
		{"invalid multiple slashes", "acme/sub/hello-world", true},
		{"invalid empty org", "/hello-world", true},
		{"invalid empty skill", "acme/", true},
		{"invalid spaces", "acme corp/hello world", true},
		{"invalid uppercase", "ACME/HelloWorld", true},
		{"invalid special chars", "acme!/hello@world", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := PluginNamespace(tt.namespace)
			err := ns.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPluginNamespace_Parts tests org/skill extraction
func TestPluginNamespace_Parts(t *testing.T) {
	ns := PluginNamespace("acme/hello-world")

	org := ns.Org()
	if org != "acme" {
		t.Errorf("Org() = %q, want 'acme'", org)
	}

	skill := ns.SkillName()
	if skill != "hello-world" {
		t.Errorf("SkillName() = %q, want 'hello-world'", skill)
	}
}

// TestPluginManifest_Validation tests manifest validation
func TestPluginManifest_Validation(t *testing.T) {
	tests := []struct {
		name     string
		manifest PluginManifest
		wantErr  bool
	}{
		{
			name: "valid manifest",
			manifest: PluginManifest{
				Namespace:   "acme/hello-world",
				Version:     "1.0.0",
				Description: "A test plugin",
				Author:      "ACME Corp",
				Skills:      []string{"hello", "world"},
			},
			wantErr: false,
		},
		{
			name: "missing namespace",
			manifest: PluginManifest{
				Version:     "1.0.0",
				Description: "Test",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			manifest: PluginManifest{
				Namespace:   "acme/hello",
				Description: "Test",
			},
			wantErr: true,
		},
		{
			name: "invalid namespace",
			manifest: PluginManifest{
				Namespace: "invalid-namespace",
				Version:   "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			manifest: PluginManifest{
				Namespace: "acme/hello",
				Version:   "not-semver",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPlugin_State tests plugin state management
func TestPlugin_State(t *testing.T) {
	plugin := &Plugin{
		Namespace: "acme/hello",
		State:     PluginStateInstalled,
	}

	if !plugin.IsInstalled() {
		t.Error("IsInstalled() should return true")
	}

	if plugin.IsDisabled() {
		t.Error("IsDisabled() should return false")
	}

	plugin.State = PluginStateDisabled

	if plugin.IsInstalled() {
		t.Error("IsInstalled() should return false")
	}

	if !plugin.IsDisabled() {
		t.Error("IsDisabled() should return true")
	}
}

// TestPlugin_Dependencies tests dependency tracking
func TestPlugin_Dependencies(t *testing.T) {
	plugin := &Plugin{
		Namespace: "acme/hello",
		Manifest: PluginManifest{
			Dependencies: map[string]string{
				"acme/utils": "^1.0.0",
				"acme/core":  "~2.1.0",
			},
		},
	}

	deps := plugin.GetDependencies()
	if len(deps) != 2 {
		t.Errorf("GetDependencies() count = %d, want 2", len(deps))
	}

	if deps["acme/utils"] != "^1.0.0" {
		t.Error("Dependency version mismatch")
	}
}

// TestPluginRegistry_Interface tests registry interface definition
func TestPluginRegistry_Interface(t *testing.T) {
	// This test just verifies the interface compiles
	var _ PluginRegistry = (*mockPluginRegistry)(nil)
}

type mockPluginRegistry struct{}

func (m *mockPluginRegistry) Install(namespace PluginNamespace, source string) error {
	return nil
}

func (m *mockPluginRegistry) Uninstall(namespace PluginNamespace) error {
	return nil
}

func (m *mockPluginRegistry) Get(namespace PluginNamespace) (*Plugin, error) {
	return nil, nil
}

func (m *mockPluginRegistry) List() ([]*Plugin, error) {
	return nil, nil
}

func (m *mockPluginRegistry) Enable(namespace PluginNamespace) error {
	return nil
}

func (m *mockPluginRegistry) Disable(namespace PluginNamespace) error {
	return nil
}
