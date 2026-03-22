package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/domain"
	infra "nuimanbot/internal/infrastructure/mcp"
)

// mockTransport is a test double for infra.Transport.
type mockTransport struct {
	sendFn func(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
}

func (m *mockTransport) Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	return m.sendFn(ctx, method, params)
}

func (m *mockTransport) Close() error { return nil }

// newInitializedClient creates an MCPClient that has already been initialized
// using the provided transport.  The transport must handle the "initialize" call.
func newInitializedClient(t *testing.T, transport infra.Transport) *infra.MCPClient {
	t.Helper()
	client := infra.NewMCPClient(transport, "test-server")
	err := client.Initialize(context.Background())
	require.NoError(t, err)
	return client
}

// initResponse returns a mock initialize response payload.
func initResponse() json.RawMessage {
	return mustMarshal(map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]string{"name": "test"},
	})
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// --- Name ---

func TestMCPToolAdapter_Name(t *testing.T) {
	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	toolDef := infra.MCPTool{Name: "search", Description: "Search memory"}

	adapter := NewMCPToolAdapter(client, toolDef, "my-server")

	assert.Equal(t, "mcp:my-server:search", adapter.Name())
}

// --- Description ---

func TestMCPToolAdapter_Description(t *testing.T) {
	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	toolDef := infra.MCPTool{Name: "search", Description: "Searches memory stores"}

	adapter := NewMCPToolAdapter(client, toolDef, "my-server")

	assert.Equal(t, "Searches memory stores", adapter.Description())
}

// --- Execute: success path ---

func TestMCPToolAdapter_Execute_Success(t *testing.T) {
	callResult := mustMarshal(infra.MCPToolResult{
		Content: []infra.MCPContent{
			{Type: "text", Text: "result one"},
			{Type: "text", Text: "result two"},
		},
		IsError: false,
	})

	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
			switch method {
			case "initialize":
				return initResponse(), nil
			case "tools/call":
				// Verify args are forwarded correctly.
				var req struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				}
				require.NoError(t, json.Unmarshal(params, &req))
				assert.Equal(t, "search", req.Name)
				assert.Equal(t, "cats", req.Arguments["query"])
				return callResult, nil
			}
			return nil, errors.New("unexpected method: " + method)
		},
	}
	client := newInitializedClient(t, transport)
	toolDef := infra.MCPTool{Name: "search", Description: "Search"}

	adapter := NewMCPToolAdapter(client, toolDef, "my-server")
	result, err := adapter.Execute(context.Background(), map[string]any{"query": "cats"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Output, "result one")
	assert.Contains(t, result.Output, "result two")
}

// --- Execute: MCP isError propagation ---

func TestMCPToolAdapter_Execute_MCPError(t *testing.T) {
	callResult := mustMarshal(infra.MCPToolResult{
		Content: []infra.MCPContent{{Type: "text", Text: "tool blew up"}},
		IsError: true,
	})

	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			switch method {
			case "initialize":
				return initResponse(), nil
			case "tools/call":
				return callResult, nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "search"}, "my-server")

	result, err := adapter.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "my-server")
}

// --- Execute: transport error ---

func TestMCPToolAdapter_Execute_TransportError(t *testing.T) {
	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			switch method {
			case "initialize":
				return initResponse(), nil
			case "tools/call":
				return nil, errors.New("connection refused")
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "search"}, "my-server")

	result, err := adapter.Execute(context.Background(), nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "my-server")
}

// --- Execute: output sanitization ---

func TestMCPToolAdapter_Execute_SanitizesOutput(t *testing.T) {
	// Output contains a GitHub token that should be redacted.
	sensitiveOutput := "Token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"

	callResult := mustMarshal(infra.MCPToolResult{
		Content: []infra.MCPContent{{Type: "text", Text: sensitiveOutput}},
		IsError: false,
	})

	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			switch method {
			case "initialize":
				return initResponse(), nil
			case "tools/call":
				return callResult, nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "search"}, "my-server")

	result, err := adapter.Execute(context.Background(), nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	// The raw token must not appear in the output.
	assert.NotContains(t, result.Output, "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij")
	assert.Contains(t, result.Output, "[REDACTED]")
}

// --- RequiredPermissions ---

func TestMCPToolAdapter_RequiredPermissions(t *testing.T) {
	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "search"}, "my-server")

	perms := adapter.RequiredPermissions()

	assert.Contains(t, perms, domain.PermissionNetwork)
}

// --- Execute: per-tool timeout ---

// TestMCPToolAdapter_Execute_Timeout verifies that Execute cancels the MCP call
// when the tool takes longer than the configured per-tool timeout.  The mock
// transport blocks for 5 seconds, but the adapter is configured with a 100ms
// timeout so the call must return well before 1 second.
func TestMCPToolAdapter_Execute_Timeout(t *testing.T) {
	transport := &mockTransport{
		sendFn: func(ctx context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			// Simulate a hanging MCP server: block until the context is cancelled.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return mustMarshal(infra.MCPToolResult{}), nil
			}
		},
	}
	client := newInitializedClient(t, transport)
	adapter := NewMCPToolAdapter(client, infra.MCPTool{Name: "slow-tool"}, "test-server",
		WithToolTimeout(100*time.Millisecond))

	start := time.Now()
	result, err := adapter.Execute(context.Background(), nil)
	elapsed := time.Since(start)

	// Must return quickly — well under 1 second.
	assert.Less(t, elapsed, 1*time.Second, "Execute should time out before 1 second")

	require.Error(t, err)
	assert.Nil(t, result)
	// Error must mention the server name, tool name, and indicate a timeout.
	assert.Contains(t, err.Error(), "test-server")
	assert.Contains(t, err.Error(), "slow-tool")
	assert.Contains(t, err.Error(), "timed out")
}

// --- InputSchema ---

func TestMCPToolAdapter_InputSchema(t *testing.T) {
	rawSchema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)

	transport := &mockTransport{
		sendFn: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "initialize" {
				return initResponse(), nil
			}
			return nil, errors.New("unexpected")
		},
	}
	client := newInitializedClient(t, transport)
	toolDef := infra.MCPTool{Name: "search", InputSchema: rawSchema}

	adapter := NewMCPToolAdapter(client, toolDef, "my-server")
	schema := adapter.InputSchema()

	assert.NotNil(t, schema)
}
