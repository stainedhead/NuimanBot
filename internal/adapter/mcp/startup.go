package mcp

import (
	"context"
	"fmt"
	"log/slog"

	infra "nuimanbot/internal/infrastructure/mcp"
	"nuimanbot/internal/usecase/tool"
)

// BuildMCPTools connects to every MCP server defined in cfg, retrieves its tool
// list, and registers each tool as an MCPToolAdapter in registry.
//
// Servers that fail to initialize are logged and skipped; the function proceeds
// with the remaining servers and returns nil unless every server fails in a way
// that should abort startup (currently none do — failures are always skipped).
//
// Tool name collisions (an "mcp:<server>:<tool>" name already registered) are
// logged as warnings and the duplicate tool is not re-registered.
func BuildMCPTools(ctx context.Context, cfg infra.MCPConfig, registry tool.ToolRegistry) error {
	for _, entry := range cfg.Servers {
		if err := connectAndRegisterServer(ctx, entry, registry); err != nil {
			// Log but do not abort — misconfigured servers must not prevent startup.
			slog.Warn("mcp: skipping server due to error",
				"server", entry.Name,
				"error", err,
			)
		}
	}
	return nil
}

// connectAndRegisterServer initializes a single MCP server and registers its
// tools.  It returns an error when the server cannot be initialized or its tool
// list cannot be fetched.
func connectAndRegisterServer(ctx context.Context, entry infra.MCPServerEntry, registry tool.ToolRegistry) error {
	transport, err := buildTransport(entry)
	if err != nil {
		return err
	}

	client := infra.NewMCPClient(transport, entry.Name)
	if err := client.Initialize(ctx); err != nil {
		_ = transport.Close()
		return err
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		_ = transport.Close()
		return err
	}

	registered := 0
	for _, t := range tools {
		adapter := NewMCPToolAdapter(client, t, entry.Name)
		if regErr := registry.Register(adapter); regErr != nil {
			slog.Warn("mcp: skipping tool due to registration conflict",
				"server", entry.Name,
				"tool", t.Name,
				"error", regErr,
			)
			continue
		}
		slog.Info("mcp: tool registered",
			"server", entry.Name,
			"tool", t.Name,
			"full_name", adapter.Name(),
		)
		registered++
	}

	slog.Info("mcp: server connected",
		"server", entry.Name,
		"transport", entry.Transport,
		"tools_registered", registered,
	)
	return nil
}

// buildTransport constructs the appropriate Transport for a server entry.
func buildTransport(entry infra.MCPServerEntry) (infra.Transport, error) {
	switch entry.Transport {
	case "http":
		return infra.NewHTTPTransport(entry.URL, entry.Headers), nil
	case "stdio":
		return infra.NewStdioTransport(entry.Command, entry.Args)
	default:
		// Config validation should have caught this, but guard defensively.
		return nil, fmt.Errorf("mcp: %s: unsupported transport %q", entry.Name, entry.Transport)
	}
}
