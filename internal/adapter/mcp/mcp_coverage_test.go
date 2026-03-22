package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/domain"
	infra "nuimanbot/internal/infrastructure/mcp"
	"nuimanbot/internal/usecase/tool"
)

// makeInitTransport returns a transport that handles only initialize.
func makeInitTransport() *mockTransport {
	return &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			return nil, errors.New("unexpected method: " + method)
		},
	}
}

// TestMCPToolAdapter_Config verifies Config returns Enabled=true.
func TestMCPToolAdapter_Config(t *testing.T) {
	client := newInitializedClient(t, makeInitTransport())
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "tool"}, "server")

	cfg := adapter.Config()
	assert.True(t, cfg.Enabled, "expected Config.Enabled=true for MCP tools")
}

// TestMCPToolAdapter_InputSchema_NilSchema verifies default empty schema is returned.
func TestMCPToolAdapter_InputSchema_NilSchema(t *testing.T) {
	client := newInitializedClient(t, makeInitTransport())
	toolDef := infra.MCPTool{Name: "tool", InputSchema: nil}

	adapter := NewMCPToolAdapter(client, toolDef, "server")
	schema := adapter.InputSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])
}

// TestMCPToolAdapter_InputSchema_InvalidJSON verifies fallback schema on bad JSON.
func TestMCPToolAdapter_InputSchema_InvalidJSON(t *testing.T) {
	client := newInitializedClient(t, makeInitTransport())
	toolDef := infra.MCPTool{Name: "tool", InputSchema: json.RawMessage(`not valid json`)}

	adapter := NewMCPToolAdapter(client, toolDef, "server")
	schema := adapter.InputSchema()

	assert.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"], "should return default empty schema for invalid JSON")
}

// TestMCPToolAdapter_InputSchema_ValidJSON verifies real schema is returned.
func TestMCPToolAdapter_InputSchema_ValidJSON(t *testing.T) {
	rawSchema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)
	client := newInitializedClient(t, makeInitTransport())
	toolDef := infra.MCPTool{Name: "tool", InputSchema: rawSchema}

	adapter := NewMCPToolAdapter(client, toolDef, "server")
	schema := adapter.InputSchema()

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema["type"])
	assert.NotNil(t, schema["properties"])
}

// TestBuildTransport_Unsupported verifies error for unknown transport type.
func TestBuildTransport_Unsupported(t *testing.T) {
	entry := infra.MCPServerEntry{
		Name:      "test-server",
		Transport: "grpc", // not supported
	}
	_, err := buildTransport(entry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported transport")
}

// TestBuildTransport_HTTP verifies HTTP transport is created without error.
func TestBuildTransport_HTTP(t *testing.T) {
	entry := infra.MCPServerEntry{
		Name:      "test-server",
		Transport: "http",
		URL:       "http://localhost:9999/mcp",
	}
	transport, err := buildTransport(entry)
	require.NoError(t, err)
	assert.NotNil(t, transport)
}

// TestBuildTransport_Stdio verifies stdio transport is created with a valid command.
func TestBuildTransport_Stdio(t *testing.T) {
	entry := infra.MCPServerEntry{
		Name:      "test-server",
		Transport: "stdio",
		Command:   "echo",
		Args:      []string{},
	}
	transport, err := buildTransport(entry)
	require.NoError(t, err)
	assert.NotNil(t, transport)
	_ = transport.Close() // clean up the spawned process
}

// TestConcatenateTextContent_NonTextIgnored verifies non-text content items are skipped.
func TestConcatenateTextContent_NonTextIgnored(t *testing.T) {
	contents := []infra.MCPContent{
		{Type: "image", Text: "ignored"},
		{Type: "text", Text: "hello"},
		{Type: "text", Text: ""}, // empty text is also ignored
		{Type: "text", Text: "world"},
	}
	result := concatenateTextContent(contents)
	assert.Equal(t, "hello\nworld", result)
}

// TestConcatenateTextContent_Empty verifies empty content returns empty string.
func TestConcatenateTextContent_Empty(t *testing.T) {
	result := concatenateTextContent(nil)
	assert.Equal(t, "", result)
}

// TestBuildMCPTools_ListToolsFailure verifies that a server failing tools/list is skipped.
func TestBuildMCPTools_ListToolsFailure(t *testing.T) {
	// Create a mock server that succeeds on initialize but fails on tools/list.
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]string{"name": "test"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case "tools/list":
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := infra.MCPConfig{
		Servers: []infra.MCPServerEntry{
			{Name: "fails-list", Transport: "http", URL: srv.URL + "/mcp"},
		},
	}

	registry := tool.NewInMemoryRegistry()
	err := BuildMCPTools(context.Background(), cfg, registry)
	// BuildMCPTools should not return an error — it logs and skips failing servers.
	require.NoError(t, err)
	// No tools should be registered since tools/list failed.
	assert.Empty(t, registry.List())
}

// TestWithToolTimeout verifies the timeout option overrides the default.
func TestWithToolTimeout(t *testing.T) {
	client := newInitializedClient(t, makeInitTransport())
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "t"}, "srv", WithToolTimeout(42))
	assert.Equal(t, int64(42), int64(adapter.timeout))
}

// TestMCPToolAdapter_RequiredPermissions_ContainsNetwork verifies network permission is included.
func TestMCPToolAdapter_RequiredPermissions_ContainsNetwork(t *testing.T) {
	client := newInitializedClient(t, makeInitTransport())
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "tool"}, "server")

	perms := adapter.RequiredPermissions()
	assert.Contains(t, perms, domain.PermissionNetwork)
}
