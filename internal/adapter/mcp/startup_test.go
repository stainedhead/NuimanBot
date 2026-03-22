package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/domain"
	infra "nuimanbot/internal/infrastructure/mcp"
	"nuimanbot/internal/usecase/tool"
)

// newMockMCPHTTPServer creates a test HTTP server acting as a minimal MCP server.
// tools are advertised on tools/list.  failInit makes initialize return HTTP 500.
func newMockMCPHTTPServer(t *testing.T, tools []infra.MCPTool, failInit bool) *httptest.Server {
	t.Helper()
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
			if failInit {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
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
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result":  map[string]any{"tools": tools},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- BuildMCPTools: valid server registers tools ---

func TestBuildMCPTools_ValidServer_RegistersTools(t *testing.T) {
	tools := []infra.MCPTool{
		{Name: "memory_search", Description: "Search memory"},
		{Name: "memory_store", Description: "Store memory"},
	}
	srv := newMockMCPHTTPServer(t, tools, false)

	cfg := infra.MCPConfig{
		Servers: []infra.MCPServerEntry{
			{Name: "ingatan", Transport: "http", URL: srv.URL + "/mcp"},
		},
	}

	registry := tool.NewInMemoryRegistry()
	err := BuildMCPTools(context.Background(), cfg, registry)
	require.NoError(t, err)

	all := registry.List()
	assert.Len(t, all, 2)

	names := make([]string, 0, len(all))
	for _, t := range all {
		names = append(names, t.Name())
	}
	assert.Contains(t, names, "mcp:ingatan:memory_search")
	assert.Contains(t, names, "mcp:ingatan:memory_store")
}

// --- BuildMCPTools: failing server is skipped, others proceed ---

func TestBuildMCPTools_FailingServer_Skipped(t *testing.T) {
	good := []infra.MCPTool{{Name: "good_tool", Description: "Works fine"}}
	goodSrv := newMockMCPHTTPServer(t, good, false)
	badSrv := newMockMCPHTTPServer(t, nil, true) // returns 500 on initialize

	cfg := infra.MCPConfig{
		Servers: []infra.MCPServerEntry{
			{Name: "bad", Transport: "http", URL: badSrv.URL + "/mcp"},
			{Name: "good", Transport: "http", URL: goodSrv.URL + "/mcp"},
		},
	}

	registry := tool.NewInMemoryRegistry()
	err := BuildMCPTools(context.Background(), cfg, registry)
	// BuildMCPTools must not return an error when only some servers fail.
	require.NoError(t, err)

	all := registry.List()
	assert.Len(t, all, 1)
	assert.Equal(t, "mcp:good:good_tool", all[0].Name())
}

// --- BuildMCPTools: empty config registers nothing ---

func TestBuildMCPTools_EmptyConfig_NoTools(t *testing.T) {
	cfg := infra.MCPConfig{Servers: nil}
	registry := tool.NewInMemoryRegistry()

	err := BuildMCPTools(context.Background(), cfg, registry)
	require.NoError(t, err)

	assert.Empty(t, registry.List())
}

// --- BuildMCPTools: tool name collision logged and skipped (no panic) ---

func TestBuildMCPTools_ExistingCollision_NoPanic(t *testing.T) {
	tools := []infra.MCPTool{{Name: "search", Description: "desc"}}
	srv := newMockMCPHTTPServer(t, tools, false)

	cfg := infra.MCPConfig{
		Servers: []infra.MCPServerEntry{
			{Name: "srv", Transport: "http", URL: srv.URL + "/mcp"},
		},
	}

	registry := tool.NewInMemoryRegistry()
	// Pre-register the colliding name.
	collision := &stubTool{name: "mcp:srv:search"}
	require.NoError(t, registry.Register(collision))

	// Must not panic; duplicate is logged and skipped.
	assert.NotPanics(t, func() {
		_ = BuildMCPTools(context.Background(), cfg, registry)
	})

	// Only the pre-registered tool should exist.
	all := registry.List()
	assert.Len(t, all, 1)
}

// stubTool satisfies domain.Tool for collision tests.
type stubTool struct {
	name string
}

func (s *stubTool) Name() string                             { return s.name }
func (s *stubTool) Description() string                      { return "stub" }
func (s *stubTool) InputSchema() map[string]any              { return map[string]any{} }
func (s *stubTool) RequiredPermissions() []domain.Permission { return nil }
func (s *stubTool) Config() domain.ToolConfig                { return domain.ToolConfig{} }
func (s *stubTool) Execute(_ context.Context, _ map[string]any) (*domain.ExecutionResult, error) {
	return nil, nil
}
