//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adaptermcp "nuimanbot/internal/adapter/mcp"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/mcp"
)

// TestMCPInitialize connects to the Ingatan MCP server and verifies the protocol version.
func TestMCPInitialize(t *testing.T) {
	skipIfNoMCPServer(t)

	transport := mcp.NewHTTPTransport(mcp.HTTPTransportConfig{
		URL: mcpURL(),
	})
	client := mcp.NewClient(transport, "ingatan")

	ctx := context.Background()
	err := client.Initialize(ctx)
	require.NoError(t, err, "Initialize should succeed against Ingatan MCP server")
}

// TestMCPToolsListAndCall verifies tool discovery and invocation.
func TestMCPToolsListAndCall(t *testing.T) {
	skipIfNoMCPServer(t)

	transport := mcp.NewHTTPTransport(mcp.HTTPTransportConfig{
		URL: mcpURL(),
	})
	client := mcp.NewClient(transport, "ingatan")

	ctx := context.Background()
	require.NoError(t, client.Initialize(ctx))

	tools, err := client.ListTools(ctx)
	require.NoError(t, err)

	// Verify memory_search tool is present.
	var found bool
	for _, tool := range tools {
		if tool.Name == "memory_search" {
			found = true
			break
		}
	}
	assert.True(t, found, "memory_search tool should be listed by Ingatan MCP server")

	// Call memory_search with a test query.
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

	cfg := &mcp.Config{
		Servers: []mcp.ServerConfig{
			{
				Name:      "ingatan",
				Transport: mcp.TransportHTTP,
				URL:       mcpURL(),
			},
		},
	}

	registry := &mockToolRegistry{}
	ctx := context.Background()

	err := adaptermcp.BuildMCPTools(ctx, cfg, registry)
	require.NoError(t, err)

	// All registered tools should have "mcp:" prefix.
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

// Register adds a tool to the registry.
func (r *mockToolRegistry) Register(tool domain.Tool) error {
	r.tools = append(r.tools, tool)
	return nil
}
