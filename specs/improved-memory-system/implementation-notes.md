# Implementation Notes: Improved Memory System

**Date:** 2026-03-22
*(Updated progressively during implementation)*

---

## Architecture Decisions

### ADR-1: Hand-roll MCP JSON-RPC Client (not mark3labs/mcp-go)

**Decision:** Implement a minimal JSON-RPC 2.0 client for MCP from scratch.

**Rationale:**
- Only 3 methods needed: `initialize`, `tools/list`, `tools/call`
- Estimated implementation: ~300 lines
- Supply-chain risk: avoiding an external dependency is consistent with the project's security-first stance (AGENTS.md)
- `mark3labs/mcp-go` is well-maintained but adds a transitive dependency chain

**Consequence:** We own the implementation. Must handle protocol version negotiation (`protocolVersion: "2024-11-05"`). If MCP protocol evolves, we update our client.

---

### ADR-2: Ingatan as Storage Backend (not search index)

**Decision:** Use Ingatan as a full storage + search backend, not just as a search index alongside the file-based storage.

**Rationale:**
- Keeping cells in two stores creates consistency problems
- Ingatan is the source of truth when configured; file-based is the source of truth when not configured
- The `MemoryCellRepository` interface wraps the entire CRUD surface — a clean boundary

**Consequence:** When `memory.backend: ingatan`, the file-based repositories are not instantiated. No dual-write.

---

### ADR-3: SHA256 Store Name Derivation

**Decision:** Ingatan store name = `{prefix}_{sha256(user_id)[:16]}`

**Rationale:**
- Ingatan store names must be lowercase alphanumeric + hyphens
- NuimanBot user IDs can be arbitrary (Telegram chat IDs, Slack IDs, usernames with mixed case)
- 16 hex chars = 64 bits of entropy — collision probability negligible for personal use

**Consequence:** Store names are opaque in Ingatan's list view. Store metadata field `nuiman_user_id` = original user ID for debugging. Operators can look up a user's store by computing the hash.

---

### ADR-4: IngatanMemoryCellRepository.DeleteExpired is a No-Op

**Decision:** `DeleteExpired` on the Ingatan adapter always returns `(0, nil)` without making any Ingatan API call.

**Rationale:**
- Ingatan has no TTL delete endpoint equivalent to `DELETE /stores/{store}/memories?expired=true`
- Memory expiry in NuimanBot is a niche feature (rarely used based on codebase review — `ExpiresAt` field is usually nil)
- Ingatan's store-level permissions and the fact that `ExpiresAt` is stored in metadata make client-side expiry enforcement impractical at recall time

**Consequence:** Expired cells stored in Ingatan persist until manually deleted. At recall time, `MemoryRecallService` calls `IsExpired()` on returned cells and skips them. This is already the correct behaviour since `SearchFTS` does not filter by `ExpiresAt` anyway. Document this in a code comment.

---

### ADR-5: MCP Tool Permission Model

**Decision:** MCP tools inherit the same RBAC permission model as built-in tools. No special MCP-specific permissions.

**Rationale:**
- NuimanBot's tool permission model (`domain.Permission`) is already in place and tested
- MCP tools are no more trusted than built-in tools — they run external code
- Simpler than adding a new permission namespace

**Consequence:** Each `MCPToolAdapter` is assigned a default permission of `domain.PermissionNetwork` (since all MCP calls go over the network). Admin-only MCP tools must be manually configured in the tool allowlist.

---

## Known Limitations

### L-1: Ingatan store auto-creation

The Ingatan bridge assumes the user's store already exists. If it doesn't exist, the first `Create` call will receive a 404. Mitigation: auto-create the store on first `Create` call if the response is 404 (attempt `POST /api/v1/stores` and retry the original request).

**Status:** To be implemented in Phase 1 Task 1.3 (handle 404 on Create → auto-create store → retry).

### L-2: Scene memory update race condition

`IngatanMemorySceneRepository.Upsert` needs to look up the existing scene memory ID before updating. Under concurrent writes to the same scene, two agents could both decide the scene doesn't exist and both attempt to create it, resulting in two scene memories for the same scene. Mitigation: use Ingatan's tag-based list as the deduplication mechanism — on retrieval, always use the most recently updated scene memory.

**Status:** Document in code comment; accept the limitation for initial implementation. A proper solution requires Ingatan to support conditional writes.

### L-3: stdio transport goroutine lifecycle

The stdio MCP transport starts a subprocess and communicates via stdin/stdout. If the NuimanBot process exits without calling `transport.Close()`, the subprocess becomes an orphan. Mitigation: register a `defer transport.Close()` in the startup wiring, and use `context.WithCancel` to signal shutdown.

---

## Open Issues

*(Populated during implementation)*

---

## Gotchas

*(Populated during implementation)*

---

## Test Patterns

### Mock Ingatan HTTP Server Pattern

```go
func newMockIngatan(t *testing.T) (*httptest.Server, *IngatanHTTPClient) {
    mux := http.NewServeMux()
    srv := httptest.NewServer(mux)
    t.Cleanup(srv.Close)

    mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
        exp := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
        json.NewEncoder(w).Encode(map[string]string{
            "token": "test-jwt-token", "expires_at": exp,
        })
    })

    cfg := IngatanConfig{
        URL:         srv.URL,
        APIKey:      "test-key",
        StorePrefix: "test",
    }
    client := NewIngatanHTTPClient(cfg)
    return srv, client
}
```

### MCP Mock Server Pattern

```go
func newMockMCPServer(t *testing.T, tools []MCPTool) *httptest.Server {
    mux := http.NewServeMux()
    srv := httptest.NewServer(mux)
    t.Cleanup(srv.Close)

    mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
        var req jsonRPCRequest
        json.NewDecoder(r.Body).Decode(&req)
        switch req.Method {
        case "initialize":
            json.NewEncoder(w).Encode(jsonRPCResponse{
                ID: req.ID,
                Result: map[string]any{
                    "protocolVersion": "2024-11-05",
                    "capabilities": map[string]any{"tools": map[string]any{}},
                    "serverInfo": map[string]string{"name": "test"},
                },
            })
        case "tools/list":
            json.NewEncoder(w).Encode(jsonRPCResponse{
                ID: req.ID,
                Result: map[string]any{"tools": tools},
            })
        }
    })

    return srv
}
```

### Integration Test Build Tag

All integration tests must use:
```go
//go:build integration

package integration_test
```

Run with: `go test -tags integration ./test/integration/...`
