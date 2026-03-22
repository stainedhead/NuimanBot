# Status: Improved Memory System

**Last Updated:** 2026-03-22
**Overall Progress:** 14% (1/7 phases complete)

---

## Phase Overview

| Phase | Scope | Status | Progress |
|-------|-------|--------|----------|
| Phase 1 | Ingatan Bridge — Config + Auth + Repositories | Not Started | 0% |
| Phase 2 | TLS Auto-Generation + Server Upgrades | Not Started | 0% |
| Phase 3 | Web Admin Security Gap Fixes | Not Started | 0% |
| Phase 4 | REST API Security Controls | Not Started | 0% |
| Phase 5 | MCP Client — Transport + Protocol | Complete | 100% |
| Phase 6 | MCP Client — Tool Bridge + Startup Integration | Not Started | 0% |
| Phase 7 | Integration Testing (Ingatan end-to-end) | Not Started | 0% |

---

## Current Phase: Phase 5 Complete — Phase 6 Ready

**Next Action:** Begin Phase 6 — MCP Tool Bridge + Startup Integration

---

## Phase Detail

### Phase 1 — Ingatan Bridge (Config + Auth + Repositories)
- [ ] Task 1.1 — Add `IngatanConfig` to config.go + YAML wiring
- [ ] Task 1.2 — Ingatan HTTP client + JWT token exchange + cache
- [ ] Task 1.3 — `IngatanMemoryCellRepository` implementation
- [ ] Task 1.4 — `IngatanMemorySceneRepository` implementation
- [ ] Task 1.5 — Backend selector in `main.go`
- [ ] Task 1.6 — Graceful degradation (fallback + health check)
- [ ] Task 1.7 — Code review + refactor (agent-driven)
- [ ] Task 1.8 — Quality gate validation

**Status:** Not Started | **Progress:** 0%

### Phase 2 — TLS Auto-Generation + Server Upgrades
- [ ] Task 2.1 — `LoadOrGenerateCert` in `internal/infrastructure/crypto/`
- [ ] Task 2.2 — `TLSConfig` struct in config.go
- [ ] Task 2.3 — `health.Server` TLS upgrade
- [ ] Task 2.4 — `web.Server` TLS upgrade + Secure cookie flag
- [ ] Task 2.5 — Code review + refactor (agent-driven)
- [ ] Task 2.6 — Quality gate validation

**Status:** Not Started | **Progress:** 0%

### Phase 3 — Web Admin Security Gap Fixes
- [ ] Task 3.1 — `requireRole` middleware for all admin routes
- [ ] Task 3.2 — Login rate limiter (token bucket per IP)
- [ ] Task 3.3 — Input sanitization on form values
- [ ] Task 3.4 — Default credential forced rotation on first login
- [ ] Task 3.5 — Code review + refactor (agent-driven)
- [ ] Task 3.6 — Quality gate validation

**Status:** Not Started | **Progress:** 0%

### Phase 4 — REST API Security Controls
- [ ] Task 4.1 — `POST /api/v1/auth/token` (API key → JWT exchange)
- [ ] Task 4.2 — JWT middleware for all REST API routes
- [ ] Task 4.3 — Per-client rate limiting middleware
- [ ] Task 4.4 — Body-size limit middleware (1 MiB)
- [ ] Task 4.5 — Input validation at handler boundary
- [ ] Task 4.6 — Code review + refactor (agent-driven)
- [ ] Task 4.7 — Quality gate validation

**Status:** Not Started | **Progress:** 0%

### Phase 5 — MCP Client Transport + Protocol
- [x] Task 5.1 — `mcp.json` loader + env substitution
- [x] Task 5.2 — `Transport` interface
- [x] Task 5.3 — HTTP transport implementation
- [x] Task 5.4 — stdio transport implementation
- [x] Task 5.5 — JSON-RPC client (`initialize`, `tools/list`, `tools/call`)
- [x] Task 5.6 — Code review + refactor (full TDD cycle with Red/Green/Refactor)
- [x] Task 5.7 — Quality gate validation (0 lint issues, all 41 tests pass with -race)

**Status:** Complete | **Progress:** 100%

**Files Created:**
- `internal/infrastructure/mcp/jsonrpc.go` — JSON-RPC 2.0 request/response types
- `internal/infrastructure/mcp/transport.go` — Transport interface
- `internal/infrastructure/mcp/config_loader.go` — mcp.json loader + env substitution
- `internal/infrastructure/mcp/config_loader_test.go` — 11 tests
- `internal/infrastructure/mcp/http_transport.go` — HTTP transport implementation
- `internal/infrastructure/mcp/http_transport_test.go` — 9 tests
- `internal/infrastructure/mcp/stdio_transport.go` — stdio transport implementation
- `internal/infrastructure/mcp/stdio_transport_test.go` — 7 tests
- `internal/infrastructure/mcp/client.go` — MCPClient with Initialize/ListTools/CallTool
- `internal/infrastructure/mcp/client_test.go` — 12 tests (using mockTransport)

**Notes:**
- Request IDs use `sync/atomic` for thread safety in HTTP transport
- stdio transport serializes calls with `sync.Mutex` (one request at a time)
- Context cancellation propagated to both transports
- JSON-RPC error responses mapped to Go errors
- `initialized` guard enforced before ListTools/CallTool
- Pre-existing lint issues in other packages are not related to Phase 5

### Phase 6 — MCP Tool Bridge + Startup Integration
- [ ] Task 6.1 — `MCPToolAdapter` implementing `domain.Tool`
- [ ] Task 6.2 — Tool namespace registration (`mcp:<server>:<tool>`)
- [ ] Task 6.3 — Startup wiring in `main.go` (connect → initialize → register)
- [ ] Task 6.4 — Bad server skip with logged error
- [ ] Task 6.5 — `/help` output updated with MCP tools
- [ ] Task 6.6 — Code review + refactor (agent-driven)
- [ ] Task 6.7 — Quality gate validation

**Status:** Not Started | **Progress:** 0%

### Phase 7 — Integration Testing (Ingatan end-to-end)
- [ ] Task 7.1 — Integration test suite: Ingatan memory write + hybrid recall
- [ ] Task 7.2 — Integration test suite: MCP tool call via Ingatan MCP server
- [ ] Task 7.3 — Cross-user isolation test
- [ ] Task 7.4 — TLS handshake test (self-signed cert)
- [ ] Task 7.5 — Full quality gate run on all phases
- [ ] Task 7.6 — Final code review (orchestrator agent)

**Status:** Not Started | **Progress:** 0%

---

## Blockers

None currently.

---

## Notes

- Phase 1 is a prerequisite for Phase 7 integration tests.
- Phases 2, 3, 4 can proceed in parallel with Phase 1 once config changes land.
- Phases 5 and 6 can proceed in parallel with Phases 2-4.
- Each phase ends with a mandatory code review + refactor step before quality gate validation.
