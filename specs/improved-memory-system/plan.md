# Implementation Plan: Improved Memory System

**Date:** 2026-03-22

---

## Guiding Principles

1. **TDD first** — every task starts with failing tests (Red), minimal implementation to pass (Green), then mandatory refactor (Refactor). No shortcuts.
2. **Clean Architecture** — dependencies flow inward only. Ingatan HTTP client lives in `infrastructure/storage/`; domain interfaces stay unchanged.
3. **Agent teams** — each phase runs as a team of specialized agents in isolated git worktrees. A code-review agent runs after Green and again after Refactor before quality gates.
4. **No regressions** — all quality gates (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint`, `go test ./...`, `go build`, `./bin/nuimanbot --help`) must pass before a phase is marked complete.

---

## Agent Team Architecture

### Orchestrator Agent (main thread)
- Spawns phase teams, monitors completion, merges worktrees
- Runs final quality gate sweep before committing each phase
- Model: inherits current (claude-sonnet-4-6)

### Per-Phase Agent Roles

Each phase team is composed of:

| Role | Count | Responsibility |
|------|-------|----------------|
| **Implementer** | 1 per task group | Red → Green cycle. Writes failing tests first, then minimal implementation. |
| **Refactor Agent** | 1 per phase | Runs after all tasks are Green. Reviews for duplication, naming, complexity, DRY. |
| **Code Review Agent** | 1 per phase | Independent review after Refactor. Checks correctness, security, idiomatic Go, test quality. Issues must-fix and should-fix comments. |
| **Quality Gate Agent** | 1 per phase | Runs all quality gate commands. Reports failures back to Implementer if any. |

All worker agents: `model: sonnet` (claude-sonnet-4-6), `isolation: worktree`

### Worktree Strategy

Each phase gets its own git worktree branch:
```
main (production)
├── feat/ims-phase-1-ingatan-bridge    (Phase 1 worktree)
├── feat/ims-phase-2-tls               (Phase 2 worktree)
├── feat/ims-phase-3-web-security      (Phase 3 worktree)
├── feat/ims-phase-4-rest-api          (Phase 4 worktree)
├── feat/ims-phase-5-mcp-transport     (Phase 5 worktree)
├── feat/ims-phase-6-mcp-bridge        (Phase 6 worktree)
└── feat/ims-phase-7-integration       (Phase 7 worktree)
```

**Parallelism:**
- Phase 1 starts first (config changes needed by later phases)
- Phases 2, 3, 4 can start after Phase 1's config changes are merged
- Phases 5, 6 can run in parallel with Phases 2-4
- Phase 7 requires all previous phases merged

**Merge order:** 1 → (2, 3, 4 in any order) → (5, 6 in any order) → 7

---

## Phase-by-Phase Plan

---

### Phase 1 — Ingatan Bridge (Config + Auth + Repositories)

**Worktree:** `feat/ims-phase-1-ingatan-bridge`
**Agents:** 3 implementers (tasks split), 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (new)
- `internal/config/config.go` — add `IngatanConfig`, `TLSConfig`, `MemoryBackendIngatan`
- `internal/infrastructure/storage/ingatan_client.go` — HTTP client + JWT token cache
- `internal/infrastructure/storage/ingatan_memory_cell_repository.go`
- `internal/infrastructure/storage/ingatan_memory_scene_repository.go`
- `internal/infrastructure/storage/ingatan_client_test.go`
- `internal/infrastructure/storage/ingatan_memory_cell_repository_test.go`
- `internal/infrastructure/storage/ingatan_memory_scene_repository_test.go`
- `cmd/nuimanbot/main.go` — backend selector

#### Task Sequence

**Task Group A (Implementer-1): Config**
1. Red: write test for `IngatanConfig` YAML unmarshal, including `SecureString` redaction
2. Green: add `IngatanConfig`, `TLSConfig`, `MemoryBackendIngatan` to `config.go`
3. Refactor: ensure field naming is idiomatic; verify `SecureString` behaviour

**Task Group B (Implementer-2): HTTP Client + Token Cache**
1. Red: write tests for `IngatanHTTPClient` — token exchange (mock HTTP server), token cache refresh, expired token auto-refresh, TLS skip-verify mode
2. Green: implement `IngatanHTTPClient` with `TokenCache`
3. Refactor: extract retry logic, clean up error wrapping

**Task Group C (Implementer-3): Repositories**
1. Red: write tests for `IngatanMemoryCellRepository` — `Create`, `Get`, `List`, `Delete`, `SearchFTS`, `GetByScene`, `GetHighSalience`, `DeleteExpired` (all using mock HTTP server via `httptest.NewServer`)
2. Green: implement `IngatanMemoryCellRepository`
3. Red: write tests for `IngatanMemorySceneRepository` — `Upsert`, `Get`, `List`, `Delete`
4. Green: implement `IngatanMemorySceneRepository`
5. Refactor: extract mapping functions (`cellToSaveRequest`, `searchResultToCell`, etc.) into a separate `ingatan_mapping.go`

**Task Group D (main.go): Backend Selector**
1. Red: write integration test for backend selection logic (config-driven)
2. Green: update `cmd/nuimanbot/main.go` to select Ingatan or built-in repositories based on `memory.backend`
3. Refactor: extract `buildMemoryRepositories(cfg)` function

**Refactor Agent:** Review all Phase 1 code for:
- Consistent error wrapping with `"ingatan: <op>: %w"` prefix
- No HTTP client creation in hot paths (client must be reused)
- Store name derivation must be pure function (no side effects)
- Mapping functions have 100% test coverage

**Code Review Agent:** Check:
- `tls_skip_verify: true` logs a warning at startup
- No raw credentials in any log line (verify `SecureString` usage)
- Token cache uses `sync.RWMutex` correctly (no data races under `-race`)
- `DeleteExpired` is a no-op in Ingatan adapter (Ingatan does not have TTL delete endpoint) — must document in implementation-notes
- All exported functions have doc comments

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 2 — TLS Auto-Generation + Server Upgrades

**Worktree:** `feat/ims-phase-2-tls`
**Agents:** 2 implementers, 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (new/modified)
- `internal/infrastructure/crypto/cert.go` — `LoadOrGenerateCert`
- `internal/infrastructure/crypto/cert_test.go`
- `internal/config/config.go` — add `TLSConfig` (coordinate with Phase 1 if merged)
- `internal/infrastructure/health/checks.go` — `StartTLS` path
- `internal/adapter/web/server.go` (or equivalent) — `StartTLS` path + Secure cookie

#### Task Sequence

**Task Group A (Implementer-1): Cert Generation**
1. Red: test `LoadOrGenerateCert` — generates new cert when files absent; reuses on second call; generated cert passes TLS verification for `localhost`; cert files created with restricted permissions (0600)
2. Green: implement using stdlib only (`crypto/tls`, `crypto/x509`, `crypto/ecdsa`, `crypto/elliptic`, `encoding/pem`, `crypto/rand`)
3. Refactor: extract `generateSelfSigned`, `writeCertFiles`, `loadFromFiles` as separate functions

**Task Group B (Implementer-2): Server TLS Upgrades**
1. Red: test `health.Server` starts on HTTPS when TLS config provided (use `httptest.NewTLSServer` pattern)
2. Green: add `StartTLS(TLSConfig)` to `health.Server`
3. Red: test `web.Server` sets `Secure: true` on session cookie when TLS active
4. Green: add `StartTLS(TLSConfig)` to `web.Server`; set `session.Options.Secure = tlsEnabled`
5. Refactor: shared `buildTLSConfig(cfg TLSConfig)` helper to avoid duplication

**Refactor Agent:** Verify no duplication between health and web server TLS setup.

**Code Review Agent:** Check:
- Key file written with `0600` permissions (private key must not be world-readable)
- Cert file written with `0644` permissions
- `data/certs/` is already gitignored (verify `.gitignore`)
- `StartTLS` and `Start` have consistent shutdown semantics
- No cert regeneration on every restart (reuse check)

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 3 — Web Admin Security Gap Fixes

**Worktree:** `feat/ims-phase-3-web-security`
**Agents:** 1 implementer, 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (modified)
- `internal/adapter/web/auth.go` — rate limiter on login, default credential rotation
- `internal/adapter/web/handlers.go` (or admin routes file) — `requireRole` middleware
- `internal/adapter/web/middleware.go` (new or existing) — `requireRole`

#### Task Sequence

**Task Group A: Role Enforcement**
1. Red: test all admin routes return 403 when called with a valid non-admin session; test admin routes succeed with admin session
2. Green: implement `requireRole(role domain.Role) http.HandlerFunc` middleware; apply to all admin route groups
3. Refactor: ensure middleware chains are readable; extract route grouping

**Task Group B: Login Rate Limiting**
1. Red: test `POST /login` returns 429 after 5 failed attempts in 1 minute from same IP; test successful login clears the bucket; test different IPs have independent buckets
2. Green: add per-IP token bucket using `internal/infrastructure/ratelimit/token_bucket.go` on the login handler
3. Refactor: rate limiter setup extracted to a constructor, not inline in handler

**Task Group C: Input Sanitization**
1. Red: test that form values containing injection patterns are sanitized before reaching the service layer (use existing attack patterns from `security.SanitizeInput`)
2. Green: pipe all form values through `security.SanitizeInput()` before service calls
3. Refactor: extract form extraction + sanitization into a typed helper `sanitizedFormValue(r, key)`

**Task Group D: Default Credential Rotation**
1. Red: test that first login with `admin/admin` results in HTTP redirect to `/admin/change-password`; test that all admin routes (except change-password) return 302 redirect while default password is active
2. Green: add a `forcePasswordChange` flag to session on first `admin/admin` login; middleware checks flag and redirects
3. Refactor: flag storage and check extracted to a helper

**Refactor Agent:** Check consistency of sanitization across all form-reading endpoints.

**Code Review Agent:** Check:
- Rate limiter uses remote IP correctly (considers X-Forwarded-For header with caution — document decision)
- `requireRole` does not leak which roles exist in the 403 response body
- Default credential detection uses constant-time comparison
- No new hardcoded credentials introduced

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 4 — REST API Security Controls

**Worktree:** `feat/ims-phase-4-rest-api`
**Agents:** 2 implementers, 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (new)
- `internal/adapter/api/server.go` — REST API server setup
- `internal/adapter/api/auth_handler.go` — `POST /api/v1/auth/token`
- `internal/adapter/api/middleware/jwt.go` — JWT validation middleware
- `internal/adapter/api/middleware/rate_limit.go` — per-client rate limiting
- `internal/adapter/api/middleware/body_limit.go` — 1 MiB body size limit
- `internal/adapter/api/middleware/validate.go` — input validation at boundary
- `internal/adapter/api/auth_handler_test.go`
- `internal/adapter/api/middleware/*_test.go`
- `cmd/nuimanbot/main.go` — wire REST API server

#### Task Sequence

**Task Group A (Implementer-1): Auth Token Exchange**
1. Red: test `POST /api/v1/auth/token` — valid API key returns JWT with correct claims; invalid key returns 401; missing key returns 400; expired/malformed body returns 400
2. Green: implement `auth_handler.go`; reuse JWT library (`golang-jwt/jwt/v5`); sign with HS256
3. Refactor: claims struct extracted to `internal/adapter/api/claims.go`

**Task Group B (Implementer-2): Middleware Stack**
1. Red: test JWT middleware — missing header → 401; malformed token → 401; valid token → `principal` in context; expired token → 401
2. Green: implement `jwt.go` middleware
3. Red: test rate limit middleware — principal A hitting 100 req/min limit gets 429; principal B unaffected
4. Green: implement per-principal rate limiting using existing token bucket
5. Red: test body limit middleware — 1 MiB+1 body returns 413; 1 MiB body passes
6. Green: implement using `http.MaxBytesReader`
7. Red: test input validation — string fields with injection patterns sanitized or rejected per security policy
8. Green: implement validation middleware
9. Refactor: middleware chain ordering documented; each middleware single-responsibility

**Refactor Agent:** Ensure middleware stack is assembled in one place with clear ordering comments.

**Code Review Agent:** Check:
- JWT secret loaded from config (encrypted vault), never from env var unprotected
- Rate limiter keyed on principal ID (from JWT), not IP (after auth)
- 413 response does not leak body contents in error message
- Validation middleware returns structured JSON errors consistent with the rest of the API

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 5 — MCP Client Transport + Protocol

**Worktree:** `feat/ims-phase-5-mcp-transport`
**Agents:** 2 implementers, 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (new)
- `internal/infrastructure/mcp/config_loader.go`
- `internal/infrastructure/mcp/config_loader_test.go`
- `internal/infrastructure/mcp/transport.go`
- `internal/infrastructure/mcp/http_transport.go`
- `internal/infrastructure/mcp/http_transport_test.go`
- `internal/infrastructure/mcp/stdio_transport.go`
- `internal/infrastructure/mcp/stdio_transport_test.go`
- `internal/infrastructure/mcp/client.go`
- `internal/infrastructure/mcp/client_test.go`

#### Task Sequence

**Task Group A (Implementer-1): Config Loader + HTTP Transport**
1. Red: test `mcp.json` loader — parses `http` and `stdio` entries; resolves `${ENV_VAR}` substitution; returns error on missing required fields; handles empty `servers` array
2. Green: implement `config_loader.go`
3. Red: test HTTP transport — sends correct JSON-RPC envelope; handles 200 response; handles non-200 (returns error); handles malformed JSON response; uses provided headers
4. Green: implement `http_transport.go`
5. Refactor: env substitution extracted to pure function; transport construction clean

**Task Group B (Implementer-2): stdio Transport + MCP Client**
1. Red: test stdio transport — writes newline-terminated JSON to stdin; reads response from stdout; handles process exit
2. Green: implement `stdio_transport.go`
3. Red: test `MCPClient.Initialize` — sends correct `initialize` request; stores `serverCapabilities`; errors if called twice; errors if server returns non-2024-11-05 protocol
4. Green: implement `Initialize`
5. Red: test `MCPClient.ListTools` — returns parsed `[]MCPTool`; errors if not initialized; handles empty tools list
6. Green: implement `ListTools`
7. Red: test `MCPClient.CallTool` — sends correct `tools/call`; returns `MCPToolResult`; propagates `isError: true` as Go error; handles unknown tool name
8. Green: implement `CallTool`
9. Refactor: JSON-RPC request/response types extracted to `jsonrpc.go`; ID counter is atomic

**Refactor Agent:** Review for:
- Request IDs use `sync/atomic` (thread-safe increment)
- No goroutine leaks in stdio transport
- Context cancellation propagated to in-flight requests

**Code Review Agent:** Check:
- `initialize` is always called before `tools/list` or `tools/call` (enforced by `initialized` guard)
- stdio transport handles process crash gracefully (no panic on closed pipe)
- HTTP transport has configurable timeout (defaults to 30s)
- JSON-RPC error response (`{"error": {...}}`) is mapped to a Go error

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 6 — MCP Tool Bridge + Startup Integration

**Worktree:** `feat/ims-phase-6-mcp-bridge`
**Agents:** 1 implementer, 1 refactor agent, 1 code review agent, 1 quality gate agent

#### Affected Files (new/modified)
- `internal/adapter/mcp/tool_bridge.go`
- `internal/adapter/mcp/tool_bridge_test.go`
- `cmd/nuimanbot/main.go` — startup wiring

#### Task Sequence

**Task Group A: MCPToolAdapter**
1. Red: test `MCPToolAdapter.Name()` returns `"mcp:<server>:<tool>"` format; test `Description()` returns MCP tool description; test `Execute()` calls `CallTool` and returns text content; test `Execute()` returns error when `isError: true`; test `Execute()` sanitizes output through `SanitizeOutput`
2. Green: implement `MCPToolAdapter`
3. Red: test startup wiring — given `mcp.json` with valid server, tools registered in registry; given server that fails `initialize`, that server is skipped and others proceed; `/help` output includes `mcp:` prefixed tools
4. Green: implement startup loop in `main.go` (or extracted `mcp_startup.go`)
5. Refactor: startup wiring extracted to `buildMCPTools(ctx, mcpCfg, registry)` function; error handling consistent

**Refactor Agent:** Verify:
- MCP tool execution respects the same permission model as built-in tools
- Tool registration does not panic if registry already has a tool with same name (log warning, skip)
- `buildMCPTools` is testable in isolation

**Code Review Agent:** Check:
- MCP tool output passes through `SanitizeOutput` before being returned to LLM context (prompt injection protection)
- Tool namespace collision (`mcp:server:name` vs built-in name) handled gracefully
- `/help` output format for MCP tools is consistent with built-in tool display
- Error from `CallTool` includes server name for debugging

**Quality Gate Agent:** Run full quality gate sequence.

---

### Phase 7 — Integration Testing

**Worktree:** `feat/ims-phase-7-integration`
**Agents:** 1 implementer, 1 code review agent, 1 quality gate agent

#### Affected Files (new)
- `test/integration/ingatan_memory_test.go`
- `test/integration/mcp_ingatan_test.go`
- `test/integration/tls_test.go`
- `test/integration/helpers_ingatan_test.go`

#### Task Sequence

**Integration Test Suite**
1. Write `ingatan_memory_test.go`:
   - Pre-seed Ingatan store with known cells
   - Run `SearchFTS` via Ingatan adapter — verify hybrid search returns semantically similar results
   - Verify cross-user isolation: user A cells not returned for user B query
   - Verify `DeleteExpired` is a no-op and logs appropriately
2. Write `mcp_ingatan_test.go`:
   - Connect NuimanBot MCP client to Ingatan MCP server (running in test)
   - Verify `memory_search` tool appears in registry as `mcp:ingatan:memory_search`
   - Verify calling the tool returns results
3. Write `tls_test.go`:
   - Verify `LoadOrGenerateCert` cert passes TLS verification
   - Verify web server rejects plain HTTP when TLS enabled
4. Final quality gate run across all packages: `go test ./...`, `golangci-lint run`, `go build`

**Code Review Agent:** Final end-to-end review:
- All integration tests use `t.Skip` if Ingatan not available (CI-safe)
- No hardcoded ports in tests (use `net.Listen(":0")` pattern)
- Test helpers extracted to `helpers_ingatan_test.go`

**Quality Gate Agent:** Run full quality gate sequence across entire codebase.

---

## Merge Plan

```
Phase 1 complete → merge feat/ims-phase-1-ingatan-bridge → main
Phases 2, 3, 4 complete (independent) → merge in order 2, 3, 4 → main
Phases 5, 6 complete (independent) → merge in order 5, 6 → main
Phase 7 complete → merge feat/ims-phase-7-integration → main
```

Each merge: resolve conflicts, re-run quality gates on merged state.

---

## Dependency Graph

```
Phase 1 (config + ingatan repos)
    │
    ├── Phase 2 (TLS)          ─── independent of Phase 1 after config struct lands
    ├── Phase 3 (web security) ─── independent of Phase 1
    └── Phase 4 (REST API)     ─── needs TLSConfig from Phase 1/2

Phase 5 (MCP transport)        ─── independent of all above
    │
    └── Phase 6 (MCP bridge)   ─── depends on Phase 5

Phase 7 (integration tests)    ─── depends on Phases 1-6 all merged
```

---

## Quality Gate Reminder

Every phase must pass before merge:

```bash
go fmt ./...
go mod tidy
go vet ./...
golangci-lint run
go test ./...
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help
```
