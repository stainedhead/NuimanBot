# Tasks: Improved Memory System

**Date:** 2026-03-22
**Methodology:** TDD — Red → Green → Refactor (all three phases mandatory)
**Model:** All worker agents are claude-sonnet-4-6

---

## How to Execute a Phase

Each phase is run by spawning an agent team in an isolated git worktree. The orchestrator:

1. Creates the worktree branch: `git worktree add -b feat/ims-phase-N-<name> ../ims-phase-N`
2. Spawns the Implementer agent(s) with `isolation: worktree` pointing at that branch
3. After Green: spawns the Refactor agent on the same worktree
4. After Refactor: spawns the Code Review agent — must return zero must-fix issues before continuing
5. Spawns the Quality Gate agent — must return all gates green
6. Merges the worktree branch into main
7. Updates `status.md`

**Agent spawn parameters (all worker agents):**
```
model: sonnet      (claude-sonnet-4-6)
isolation: worktree
mode: acceptEdits
```

---

## Phase 1 — Ingatan Bridge

### Task 1.1 — IngatanConfig + TLSConfig in config.go

**Agent role:** Implementer-1
**TDD cycle:**

- **Red:**
  - `internal/config/config_test.go` — add test: YAML with `memory.backend: ingatan` and `memory.ingatan.*` fields unmarshals into `IngatanConfig` correctly
  - Test `IngatanConfig.APIKey` is a `domain.SecureString` and masked in `%v` formatting
  - Test `MemoryBackendIngatan` constant equals `"ingatan"`

- **Green:**
  - Add `IngatanConfig` struct to `internal/config/config.go`
  - Add `TLSConfig` struct to `internal/config/config.go`
  - Add `MemoryBackendIngatan MemoryBackend = "ingatan"` constant
  - Update `MemoryConfig` to include `Ingatan IngatanConfig`

- **Refactor:**
  - Ensure all new fields have YAML tags and doc comments
  - Verify `SecureString` redaction works in log output

**Done when:** Tests pass, quality gates pass.

---

### Task 1.2 — IngatanHTTPClient + TokenCache

**Agent role:** Implementer-2
**TDD cycle:**

- **Red:** `internal/infrastructure/storage/ingatan_client_test.go`:
  - Test token exchange: mock HTTP server returns `{"token": "abc", "expires_at": "..."}` → client stores token, subsequent `Do` calls include `Authorization: Bearer abc`
  - Test token auto-refresh: mock server; set `expiresAt` to past → `Do` triggers refresh before request
  - Test token exchange failure: mock returns 401 → `Do` returns error
  - Test `TLSSkipVerify: true` creates client that accepts self-signed cert
  - Test request context cancellation propagates to HTTP call

- **Green:**
  - `internal/infrastructure/storage/ingatan_client.go`
  - `IngatanHTTPClient`, `TokenCache`, `NewIngatanHTTPClient(cfg IngatanConfig)`
  - `Do(ctx, method, path string, body io.Reader) (*http.Response, error)`

- **Refactor:**
  - `TokenCache.needsRefresh()` extracted as pure method (testable)
  - Buffer constant `tokenRefreshBuffer = 5 * time.Minute` (refresh 5 min before expiry)
  - Error messages consistently prefixed: `"ingatan: token exchange: ..."`

**Done when:** Tests pass with `-race` flag, quality gates pass.

---

### Task 1.3 — IngatanMemoryCellRepository

**Agent role:** Implementer-3
**TDD cycle:**

- **Red:** `internal/infrastructure/storage/ingatan_memory_cell_repository_test.go`:
  - For each method: `Create`, `Get`, `List`, `Delete`, `SearchFTS`, `GetByScene`, `GetHighSalience`, `DeleteExpired` — use `httptest.NewServer` to mock Ingatan
  - `Create`: verify correct JSON body sent; 201 → nil error; 409 → `ErrAlreadyExists`; 400 → `ErrInvalidInput`
  - `Get`: 200 → correctly mapped `MemoryCell`; 404 → `ErrNotFound`
  - `SearchFTS`: sends `mode: hybrid`; response mapped to `[]MemoryCell`
  - `GetHighSalience`: fetches by source=conversation, filters by salience
  - `DeleteExpired`: returns 0, nil (no-op in Ingatan adapter)
  - Mapping round-trip: create a `MemoryCell`, convert to save request, convert response back — assert fields equal

- **Green:**
  - `internal/infrastructure/storage/ingatan_memory_cell_repository.go`
  - `internal/infrastructure/storage/ingatan_mapping.go` — all mapping functions

- **Refactor:**
  - All mapping functions in `ingatan_mapping.go` with unit tests
  - No magic strings — use constants for metadata keys (`metaKeyScene = "scene"`, etc.)
  - `storeFor(conversationID)` extracted as package-level pure function

**Done when:** Tests pass, quality gates pass.

---

### Task 1.4 — IngatanMemorySceneRepository

**Agent role:** Implementer-3 (continued)
**TDD cycle:**

- **Red:** `internal/infrastructure/storage/ingatan_memory_scene_repository_test.go`:
  - `Upsert` (new scene): `POST /stores/{store}/memories` called with correct body; 201 → nil
  - `Upsert` (existing scene): scene memory already exists → `PUT /stores/{store}/memories/{id}` called
  - `Get`: searches by `tags=_scene,{scene_name}`; maps first result to `MemoryScene`; no result → `ErrNotFound`
  - `List`: fetches all memories tagged `_scene`; maps to `[]MemoryScene`
  - `Delete`: finds scene memory by tag, deletes by ID

- **Green:**
  - `internal/infrastructure/storage/ingatan_memory_scene_repository.go`

- **Refactor:**
  - Scene ID caching (to avoid repeated list+filter on Upsert): store `nuiman_scene_id` = scene_name; use as lookup key
  - Shared mapping extracted to `ingatan_mapping.go`

**Done when:** Tests pass, quality gates pass.

---

### Task 1.5 — Backend Selector (main.go)

**Agent role:** Implementer-1 (continued)
**TDD cycle:**

- **Red:** Test in `cmd/nuimanbot/` (or integration test):
  - `cfg.Memory.Backend == "ingatan"` → `IngatanMemoryCellRepository` and `IngatanMemorySceneRepository` returned
  - `cfg.Memory.Backend == "builtin"` (or empty) → file-based repositories returned
  - `cfg.Memory.Backend == "ingatan"` but `IngatanConfig.URL` empty → returns error at startup

- **Green:**
  - Update `cmd/nuimanbot/main.go` — add `buildMemoryRepositories(cfg Config) (MemoryCellRepository, MemorySceneRepository, error)` function
  - Wire into dependency injection

- **Refactor:**
  - Extract to `internal/adapter/factory/memory_factory.go` if main.go grows too large
  - Factory function has clear error messages for misconfiguration

**Done when:** Tests pass, quality gates pass.

---

### Task 1.6 — Graceful Degradation

**Agent role:** Implementer-2 (continued)
**TDD cycle:**

- **Red:**
  - Test: when Ingatan is unreachable at startup and `fallback_to_builtin: true`, built-in repositories are used and a warning is logged
  - Test: when `fallback_to_builtin: false` and Ingatan unreachable, startup returns error

- **Green:**
  - Add startup health probe to `IngatanHTTPClient` (`GET /api/v1/health`)
  - Implement fallback logic in `buildMemoryRepositories`

- **Refactor:**
  - Health probe timeout configurable (default 5s)
  - Alert sent via `alerting.SendAlert` when Ingatan is unavailable and fallback is used

**Done when:** Tests pass, quality gates pass.

---

### Task 1.7 — Phase 1 Refactor Agent Review

**Agent role:** Refactor Agent
**Tasks:**
- Review all Phase 1 code across all new files
- Check for: duplication in mapping functions, inconsistent error wrapping, naming that doesn't match Go conventions, functions doing more than one thing
- Must-fix: any function > 40 lines without a clear reason; any duplicated mapping logic; any exported type without doc comment
- Run `go test ./... -race` after each refactor step to ensure tests stay green
- Update implementation-notes.md with any significant decisions made

---

### Task 1.8 — Phase 1 Code Review

**Agent role:** Code Review Agent
**Checklist:**
- [ ] `tls_skip_verify: true` logs a startup warning
- [ ] No raw credentials appear in any log line (search for API key values in test logs)
- [ ] `TokenCache` has no race condition under `go test -race`
- [ ] `DeleteExpired` no-op is documented in code comment
- [ ] All exported functions and types have doc comments starting with the name
- [ ] HTTP client reuses `http.Transport` (not creating new transport per request)
- [ ] `storeFor()` produces the same hash for the same input (deterministic)
- [ ] Store prefix defaults to `"nuiman"` if not configured
- [ ] Integration test tags (`//go:build integration`) prevent running in normal CI

**Must-fix issues must be resolved before Phase 1 quality gate.**

---

### Task 1.9 — Phase 1 Quality Gate

**Agent role:** Quality Gate Agent
```bash
go fmt ./...
go mod tidy
go vet ./...
golangci-lint run
go test ./...
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help
```
All must pass with zero errors. Update `status.md`.

---

## Phase 2 — TLS Auto-Generation + Server Upgrades

### Task 2.1 — LoadOrGenerateCert

**Agent role:** Implementer-1
**TDD cycle:**

- **Red:** `internal/infrastructure/crypto/cert_test.go`:
  - Generates cert when files absent; cert + key files created at specified paths
  - Reuses existing files on second call (no regeneration; verify by mtime)
  - Generated cert valid for `localhost` (TLS dial to test server succeeds)
  - Generated cert written with correct permissions (key: 0600, cert: 0644)
  - `hosts` parameter appears in cert's `DNSNames`

- **Green:** `internal/infrastructure/crypto/cert.go` — stdlib only

- **Refactor:** Extract `generateSelfSigned`, `writeCertFiles`, `loadFromFiles`

---

### Task 2.2 — TLSConfig + Server TLS Upgrades

**Agent role:** Implementer-2
**TDD cycle:**

- **Red:**
  - `health.Server` test: `StartTLS` starts HTTPS; plain HTTP request rejected
  - `web.Server` test: `StartTLS` sets `session.Options.Secure = true`; confirms via test cookie inspection

- **Green:** Add `StartTLS` to both servers; build TLS config from `LoadOrGenerateCert`

- **Refactor:** Shared `buildTLSConfig` helper

---

### Task 2.3 — Phase 2 Refactor + Code Review + Quality Gate

- **Refactor Agent:** No duplication between servers; cert files gitignored
- **Code Review Agent:** Key file permissions, no cert regeneration on restart, session cookie Secure flag
- **Quality Gate Agent:** Full gate run

---

## Phase 3 — Web Admin Security Gap Fixes

### Task 3.1 — requireRole Middleware

**Agent role:** Implementer
**TDD cycle:**

- **Red:** Tests: non-admin session → 403; admin session → passes through; unauthenticated → 401
- **Green:** `requireRole` middleware applied to all admin route groups
- **Refactor:** Clean middleware chain

---

### Task 3.2 — Login Rate Limiter

**Agent role:** Implementer
**TDD cycle:**

- **Red:** Tests: 5 failed logins → 429; 6th attempt from different IP → not throttled; success resets bucket
- **Green:** Per-IP token bucket on `POST /login`
- **Refactor:** Rate limiter setup extracted from handler

---

### Task 3.3 — Input Sanitization

**Agent role:** Implementer
**TDD cycle:**

- **Red:** Test injection patterns in form values are sanitized
- **Green:** Pipe through `security.SanitizeInput()`
- **Refactor:** Typed helper `sanitizedFormValue`

---

### Task 3.4 — Default Credential Rotation

**Agent role:** Implementer
**TDD cycle:**

- **Red:** First `admin/admin` login → redirect to `/admin/change-password`; other admin routes blocked until changed
- **Green:** `forcePasswordChange` session flag + middleware
- **Refactor:** Flag check extracted to helper

---

### Task 3.5 — Phase 3 Refactor + Code Review + Quality Gate

- **Refactor Agent:** Sanitization consistency, rate limiter setup
- **Code Review Agent:** No role leakage in 403, constant-time credential comparison, X-Forwarded-For decision documented
- **Quality Gate Agent:** Full gate run

---

## Phase 4 — REST API Security Controls

### Task 4.1 — Auth Token Endpoint

**Agent role:** Implementer-1
**TDD cycle:**

- **Red:** Valid API key → 200 + JWT; invalid key → 401; missing key → 400
- **Green:** `internal/adapter/api/auth_handler.go`
- **Refactor:** Claims struct extracted

---

### Task 4.2 — JWT Middleware

**Agent role:** Implementer-2
**TDD cycle:**

- **Red:** Missing/malformed/expired JWT → 401; valid JWT → principal in context
- **Green:** `internal/adapter/api/middleware/jwt.go`
- **Refactor:** Single responsibility

---

### Task 4.3 — Rate Limit + Body Limit + Validation Middleware

**Agent role:** Implementer-2 (continued)
**TDD cycle:**

- **Red (rate limit):** 100+ req/min from same principal → 429
- **Green:** Per-principal token bucket
- **Red (body limit):** 1MiB+1 → 413
- **Green:** `http.MaxBytesReader`
- **Red (validation):** Injection patterns in body fields → sanitized/rejected
- **Green:** Validation middleware
- **Refactor:** Middleware ordering documented

---

### Task 4.4 — Phase 4 Refactor + Code Review + Quality Gate

- **Refactor Agent:** Middleware chain ordering, DRY
- **Code Review Agent:** JWT secret source, rate limiter keyed on principal ID, 413 doesn't leak body, structured JSON errors
- **Quality Gate Agent:** Full gate run

---

## Phase 5 — MCP Client Transport + Protocol

### Task 5.1 — mcp.json Loader + HTTP Transport

**Agent role:** Implementer-1
**TDD cycle:**

- **Red (loader):** Parse http+stdio entries; env substitution; error on missing required fields
- **Green:** `internal/infrastructure/mcp/config_loader.go`
- **Red (HTTP transport):** Correct JSON-RPC envelope; 200 response parsed; non-200 → error
- **Green:** `internal/infrastructure/mcp/http_transport.go`
- **Refactor:** Env substitution as pure function

---

### Task 5.2 — stdio Transport + MCP Client Protocol

**Agent role:** Implementer-2
**TDD cycle:**

- **Red (stdio):** Writes to stdin; reads from stdout; handles process exit
- **Green:** `internal/infrastructure/mcp/stdio_transport.go`
- **Red (initialize):** Correct request; stores capabilities; errors if called twice
- **Green:** `MCPClient.Initialize`
- **Red (tools/list):** Returns parsed tools; errors if not initialized
- **Green:** `MCPClient.ListTools`
- **Red (tools/call):** Correct request; `isError: true` → Go error
- **Green:** `MCPClient.CallTool`
- **Refactor:** JSON-RPC types in `jsonrpc.go`; atomic ID counter

---

### Task 5.3 — Phase 5 Refactor + Code Review + Quality Gate

- **Refactor Agent:** Thread safety, context propagation, goroutine leaks
- **Code Review Agent:** `initialize` guard, stdio crash handling, HTTP timeout, error response mapping
- **Quality Gate Agent:** Full gate run

---

## Phase 6 — MCP Tool Bridge + Startup Integration

### Task 6.1 — MCPToolAdapter + Startup Wiring

**Agent role:** Implementer
**TDD cycle:**

- **Red (adapter):** `Name()` format; `Description()`; `Execute()` calls `CallTool`; `isError` propagated; output sanitized
- **Green:** `internal/adapter/mcp/tool_bridge.go`
- **Red (startup):** Valid server → tools registered; failing server → skipped; `/help` shows `mcp:` tools
- **Green:** `buildMCPTools` in `main.go` or factory
- **Refactor:** `buildMCPTools` extracted, testable

---

### Task 6.2 — Phase 6 Refactor + Code Review + Quality Gate

- **Refactor Agent:** Permission model consistency, collision handling
- **Code Review Agent:** `SanitizeOutput` applied, namespace collision handled, `/help` format consistent, error includes server name
- **Quality Gate Agent:** Full gate run

---

## Phase 7 — Integration Testing

### Task 7.1 — Integration Test Suite

**Agent role:** Implementer
**TDD cycle:**

- Write `//go:build integration` tagged tests
- `ingatan_memory_test.go`: hybrid search semantic match; cross-user isolation
- `mcp_ingatan_test.go`: tool registration + call via Ingatan MCP
- `tls_test.go`: self-signed cert TLS verification
- All tests skip gracefully if Ingatan not running

---

### Task 7.2 — Final Code Review + Quality Gate

- **Code Review Agent:** Integration tests use `:0` ports; helpers extracted; `t.Skip` for missing Ingatan
- **Quality Gate Agent:** Full gate run across entire codebase including all phases

---

## Task Summary

| Phase | Tasks | New Files | Tests |
|-------|-------|-----------|-------|
| 1 — Ingatan Bridge | 9 | ~10 | ~40 |
| 2 — TLS | 3 | ~3 | ~10 |
| 3 — Web Security | 5 | ~2 | ~15 |
| 4 — REST API | 4 | ~6 | ~20 |
| 5 — MCP Transport | 3 | ~6 | ~25 |
| 6 — MCP Bridge | 2 | ~2 | ~10 |
| 7 — Integration | 2 | ~4 | ~12 |
| **Total** | **28** | **~33** | **~132** |

---

## Agent Spawn Reference

When spawning phase agents, use these prompt templates:

### Implementer Agent Prompt Template
```
You are implementing Phase N (description) of the improved-memory-system feature for NuimanBot.

Read the following spec files before starting:
- specs/improved-memory-system/spec.md
- specs/improved-memory-system/plan.md (Phase N section)
- specs/improved-memory-system/data-dictionary.md
- specs/improved-memory-system/research.md
- AGENTS.md (project rules)

Your task: [specific task description]

MANDATORY TDD cycle:
1. Write failing tests first (Red). Run `go test ./...` and confirm they fail.
2. Write minimal implementation to make tests pass (Green). Run `go test ./...`.
3. Refactor for quality while keeping tests green (Refactor). MANDATORY — do not skip.
4. Run `go test ./... -race` after refactor.

When done, update specs/improved-memory-system/status.md with task completion.
```

### Refactor Agent Prompt Template
```
You are the Refactor agent for Phase N of the improved-memory-system feature.

Review all new/modified files in this phase for:
- Duplication (DRY violations)
- Naming clarity (idiomatic Go: MixedCaps, no underscores, acronyms uppercase)
- Function length (>40 lines is a smell unless justified)
- Single responsibility
- Error wrapping consistency ("ingatan: <op>: %w" or "mcp: <op>: %w")
- All exported types/functions have doc comments starting with the name

After each change, run `go test ./...` to confirm tests stay green.
Update specs/improved-memory-system/implementation-notes.md with decisions made.
```

### Code Review Agent Prompt Template
```
You are the Code Review agent for Phase N of the improved-memory-system feature.

Perform a thorough code review of all files changed in this phase.
Check against the checklist in specs/improved-memory-system/tasks.md (Phase N code review section).

Return a report with:
- MUST-FIX issues (block merge)
- SHOULD-FIX issues (fix if trivial, document if complex)
- LOOKS GOOD items

Do not proceed with merge until all MUST-FIX issues are resolved.
```

### Quality Gate Agent Prompt Template
```
You are the Quality Gate agent for Phase N.

Run these commands in order. All must pass with zero errors:
1. go fmt ./...
2. go mod tidy
3. go vet ./...
4. golangci-lint run
5. go test ./...
6. go build -o bin/nuimanbot ./cmd/nuimanbot
7. ./bin/nuimanbot --help

Report any failures back for fixing. When all pass, update status.md marking Phase N complete.
```
