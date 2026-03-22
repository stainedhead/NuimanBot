//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adaptermcp "nuimanbot/internal/adapter/mcp"
	"nuimanbot/internal/domain"
	inframcp "nuimanbot/internal/infrastructure/mcp"
	tooltool "nuimanbot/internal/usecase/tool"
)

// TestMCPInitialize connects to the Ingatan MCP server and verifies the protocol version.
func TestMCPInitialize(t *testing.T) {
	skipIfNoMCPServer(t)

	transport := inframcp.NewHTTPTransport(mcpURL(), nil)
	client := inframcp.NewMCPClient(transport, "ingatan")

	ctx := context.Background()
	err := client.Initialize(ctx)
	require.NoError(t, err, "Initialize should succeed against Ingatan MCP server")
}

// TestMCPToolsListAndCall verifies tool discovery and invocation.
func TestMCPToolsListAndCall(t *testing.T) {
	skipIfNoMCPServer(t)

	transport := inframcp.NewHTTPTransport(mcpURL(), nil)
	client := inframcp.NewMCPClient(transport, "ingatan")

	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	var found bool
	for _, tool := range tools {
		if tool.Name == "memory_search" {
			found = true
			break
		}
	}
	assert.True(t, found, "memory_search tool should be listed by Ingatan MCP server")

	if found {
		result, err := client.CallTool(ctx, "memory_search", map[string]any{
			"query": "test query",
			"limit": 5,
		})
		require.NoError(t, err)
		assert.NotNil(t, result)
	}
}

// TestMCPToolRegistration verifies that BuildMCPTools registers tools with the "mcp:" prefix.
func TestMCPToolRegistration(t *testing.T) {
	skipIfNoMCPServer(t)

	cfg := inframcp.MCPConfig{
		Servers: []inframcp.MCPServerEntry{
			{
				Name:      "ingatan",
				Transport: "http",
				URL:       mcpURL(),
			},
		},
	}

	registry := &mockToolRegistry{}
	ctx := context.Background()

	err := adaptermcp.BuildMCPTools(ctx, cfg, registry)
	require.NoError(t, err)

	assert.NotEmpty(t, registry.tools, "at least one tool should be registered")
	for _, tool := range registry.tools {
		assert.True(t,
			len(tool.Name()) >= 4 && tool.Name()[:4] == "mcp:",
			"tool %q should have mcp: prefix", tool.Name())
	}
}

// mockToolRegistry is a simple in-memory tool registry for testing.
type mockToolRegistry struct {
	tools []domain.Tool
}

// Register adds a tool to the registry, satisfying tool.ToolRegistry.
func (r *mockToolRegistry) Register(t domain.Tool) error {
	r.tools = append(r.tools, t)
	return nil
}

// Get retrieves a tool by name.
func (r *mockToolRegistry) Get(name string) (domain.Tool, error) {
	for _, t := range r.tools {
		if t.Name() == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("tool %q not found", name)
}

// List returns all registered tools.
func (r *mockToolRegistry) List() []domain.Tool {
	return r.tools
}

// ListForUser returns all tools (no permission filtering in mock).
func (r *mockToolRegistry) ListForUser(_ context.Context, _ string) ([]domain.Tool, error) {
	return r.tools, nil
}

// compile-time assertion that mockToolRegistry satisfies the ToolRegistry interface.
var _ tooltool.ToolRegistry = (*mockToolRegistry)(nil)
