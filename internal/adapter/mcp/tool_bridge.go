// Package mcp provides adapters that bridge MCP (Model Context Protocol) tools
// into NuimanBot's domain tool registry.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	infra "nuimanbot/internal/infrastructure/mcp"
	"nuimanbot/internal/usecase/tool/common"
)

// MCPToolAdapter wraps an infra.MCPClient tool invocation and implements
// domain.Tool so MCP tools can participate in the standard tool registry.
type MCPToolAdapter struct {
	client     *infra.MCPClient
	toolDef    infra.MCPTool
	serverName string
	sanitizer  *common.OutputSanitizer
}

// NewMCPToolAdapter constructs an MCPToolAdapter for the given client and tool.
// serverName is used in the tool name prefix and error messages.
func NewMCPToolAdapter(client *infra.MCPClient, toolDef infra.MCPTool, serverName string) *MCPToolAdapter {
	return &MCPToolAdapter{
		client:     client,
		toolDef:    toolDef,
		serverName: serverName,
		sanitizer:  common.NewOutputSanitizer(),
	}
}

// Name returns the namespaced tool identifier: "mcp:<serverName>:<toolName>".
func (a *MCPToolAdapter) Name() string {
	return "mcp:" + a.serverName + ":" + a.toolDef.Name
}

// Description returns the MCP tool's description.
func (a *MCPToolAdapter) Description() string {
	return a.toolDef.Description
}

// InputSchema returns a JSON-object schema derived from the MCP tool's inputSchema.
// If the MCP tool has no inputSchema, an empty object schema is returned.
func (a *MCPToolAdapter) InputSchema() map[string]any {
	if a.toolDef.InputSchema == nil {
		return map[string]any{"type": "object"}
	}
	var schema map[string]any
	if err := json.Unmarshal(a.toolDef.InputSchema, &schema); err != nil {
		return map[string]any{"type": "object"}
	}
	return schema
}

// RequiredPermissions returns [domain.PermissionNetwork] because all MCP tool
// invocations involve a network call to the MCP server (ADR-5).
func (a *MCPToolAdapter) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the tool's configuration.  MCP tools have no local config.
func (a *MCPToolAdapter) Config() domain.ToolConfig {
	return domain.ToolConfig{Enabled: true}
}

// Execute calls the MCP tool via tools/call and returns the concatenated text
// content.  Output is sanitized via OutputSanitizer before being returned to
// prevent prompt-injection from untrusted MCP servers.
func (a *MCPToolAdapter) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	result, err := a.client.CallTool(ctx, a.toolDef.Name, params)
	if err != nil {
		return nil, fmt.Errorf("mcp: %s: %s: %w", a.serverName, a.toolDef.Name, err)
	}

	if result.IsError {
		errText := concatenateTextContent(result.Content)
		return nil, fmt.Errorf("mcp: %s: %s: tool error: %s", a.serverName, a.toolDef.Name, errText)
	}

	output := a.sanitizer.SanitizeOutput(concatenateTextContent(result.Content))
	return &domain.ExecutionResult{Output: output}, nil
}

// concatenateTextContent joins all text-typed content items with a newline separator.
func concatenateTextContent(contents []infra.MCPContent) string {
	var parts []string
	for _, c := range contents {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}
