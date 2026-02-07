package cli

import (
	"context"
	"fmt"
	"io"

	"nuimanbot/internal/domain"
)

// PluginCommand handles plugin CLI commands
type PluginCommand struct {
	registry domain.PluginRegistry
	output   io.Writer
}

// NewPluginCommand creates a new plugin command handler
func NewPluginCommand(registry domain.PluginRegistry, output io.Writer) *PluginCommand {
	return &PluginCommand{
		registry: registry,
		output:   output,
	}
}

// Install installs a plugin
func (c *PluginCommand) Install(ctx context.Context, namespace string, source string) error {
	ns := domain.PluginNamespace(namespace)

	if err := ns.Validate(); err != nil {
		return fmt.Errorf("invalid namespace: %w", err)
	}

	if err := c.registry.Install(ns, source); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Fprintf(c.output, "Successfully installed plugin: %s\n", namespace)
	return nil
}

// List lists all installed plugins
func (c *PluginCommand) List(ctx context.Context) error {
	plugins, err := c.registry.List()
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Fprintln(c.output, "No plugins installed.")
		return nil
	}

	fmt.Fprintf(c.output, "Installed plugins (%d):\n", len(plugins))
	for _, p := range plugins {
		state := "enabled"
		if p.IsDisabled() {
			state = "disabled"
		}
		fmt.Fprintf(c.output, "  %s (v%s) - %s [%s]\n",
			p.Namespace, p.Manifest.Version, p.Manifest.Description, state)
	}

	return nil
}

// Uninstall removes a plugin
func (c *PluginCommand) Uninstall(ctx context.Context, namespace string) error {
	ns := domain.PluginNamespace(namespace)

	if err := c.registry.Uninstall(ns); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	fmt.Fprintf(c.output, "Successfully uninstalled plugin: %s\n", namespace)
	return nil
}

// Enable enables a plugin
func (c *PluginCommand) Enable(ctx context.Context, namespace string) error {
	ns := domain.PluginNamespace(namespace)

	if err := c.registry.Enable(ns); err != nil {
		return fmt.Errorf("enable failed: %w", err)
	}

	fmt.Fprintf(c.output, "Successfully enabled plugin: %s\n", namespace)
	return nil
}

// Disable disables a plugin
func (c *PluginCommand) Disable(ctx context.Context, namespace string) error {
	ns := domain.PluginNamespace(namespace)

	if err := c.registry.Disable(ns); err != nil {
		return fmt.Errorf("disable failed: %w", err)
	}

	fmt.Fprintf(c.output, "Successfully disabled plugin: %s\n", namespace)
	return nil
}
