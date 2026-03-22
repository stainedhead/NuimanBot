# Summary: Feature Gaps to Consider for Next Phase

Identified during architecture review (2026-03-07). These are either config scaffolding with no implementation, or missing security controls worth addressing.

---

## 1. MCP Client (mcp.json config + MCP server tool calls)

**Current state:** Config scaffolding only (`MCPConfig`, `MCPClientConfig`, `MCPServerConfig` in `internal/config/config.go`). No implementation exists.

**What needs to be built:**
- `mcp.json` loader/parser
- MCP transport layer (stdio, SSE, or HTTP)
- JSON-RPC protocol handling (`initialize`, `tools/list`, `tools/call`)
- No MCP Go SDK dependency in `go.mod` — needs to be added (e.g. `github.com/mark3labs/mcp-go` or implement from spec)
- Adapter from MCP server tools → internal `domain.Tool` interface so discovered tools register in the tool registry

---

## 2. HTTPS/TLS with Self-Signed Certificates

**Current state:** Both HTTP servers use plain `ListenAndServe`. No TLS code exists. `MCPServerConfig.TLS bool` field is a placeholder never consumed.

**What needs to be built:**
- Self-signed cert generator in `internal/infrastructure/crypto/` using stdlib (`crypto/tls`, `crypto/x509`, `crypto/ecdsa`)
- `LoadOrGenerateCert(certPath, keyPath string, hosts []string) (tls.Certificate, error)` — auto-generates on first run if files are absent
- Shared `TLSConfig` struct added to `config.go` (fields: `Enabled`, `AutoGenerate`, `CertFile`, `KeyFile`, `Hosts`)
- `health.Server` — add `StartTLS` path using `ListenAndServeTLS`
- `web.Server` — add `StartTLS` path; flip session cookie `Secure: true` when TLS is active
- No new external dependencies required

---

## 3. REST API (used by CLI and external clients)

**Current state:** `ExternalAPIConfig` in `config.go` has `OpenAI`-compatible and `REST` sub-configs (port, API key) but there is no server implementation. The CLI communicates entirely in-process, not over HTTP.

**What needs to be built:**
- REST API server in `internal/adapter/web/` or a new `internal/adapter/api/` package
- OpenAI-compatible endpoint (`POST /v1/chat/completions`) if desired
- CLI client that calls the REST API over HTTP (replaces in-process wiring if remote deployment is the goal)
- Security controls (see item 4 below)

**Decision needed:** Is the REST API intended for local in-process use, or for remote/network clients? This shapes the security model significantly.

---

## 4. REST API Security Controls (missing)

**Current state of web admin UI (the only HTTP surface today):**

| Control | Status | Notes |
|---|---|---|
| Session auth (bcrypt + random session IDs) | Done | `internal/adapter/web/auth.go` |
| CSRF tokens (single-use) | Done | `internal/adapter/web/auth.go` |
| Role checks on all admin routes | Partial | `handleUsers` only checks `user == nil`, not role |
| Transport encryption (TLS) | Missing | `Secure: false` on session cookie, plain HTTP |
| Login rate limiting | Missing | Brute force possible |
| Input sanitization on form values | Missing | Values passed directly to service layer |
| Default credential enforcement | Missing | Hardcoded `admin/admin` added in `main.go:725`, no forced rotation |

**For the REST API, additionally needed:**
- API key or JWT bearer token authentication
- Per-client rate limiting
- Request body size limits
- Input validation at HTTP boundary

---

## 5. Ingatan Memory Backend Integration

**Context:** Ingatan (`~/Code/stainedhead/Golang/ingatan-product/ingatan-app`) is a purpose-built second-brain service with REST + MCP APIs, hybrid search, and multi-user RBAC. NuimanBot has its own memory system (v2) with different strengths.

**Where NuimanBot memory v2 wins:**
- LLM auto-extraction: curator watches every conversation turn and extracts typed, salience-scored `MemoryCell` records
- Typed cells (`CellType` enum) + salience scoring (0–1) enable structured, ranked recall
- Scene consolidation: LLM-generated per-topic summaries injected alongside cells

**Where ingatan wins:**
- Search quality: hybrid HNSW semantic + BM25 keyword with RRF fusion (vs FTS-only in NuimanBot)
- Ingest breadth: URL, file, PDF, bulk import (NuimanBot only extracts from conversations)
- Multi-user RBAC: per-store Owner/Writer/Reader roles (NuimanBot has only `conversation_id` scoping)
- REST + MCP already implemented and tested behind JWT + TLS — directly closes items 1, 3, and 4 above
- HTTPS already live (`ListenAndServeTLS :8443`)

**Recommended approach: dual backend with bridge pattern**

Keep NuimanBot's curator (auto-extraction stays unchanged). Add an ingatan repository adapter alongside the existing file-based one. Selection via config:

```yaml
memory:
  backend: ingatan  # or "builtin"
  ingatan:
    url: https://localhost:8443
    api_key: <user-provided>
    store: <username>   # ingatan personal store convention: store name == user ID
```

Bridge: curator extracts cells → maps to ingatan `Memory` (salience + CellType go into `metadata`) → `POST /stores/{store}/memories`. Recall maps ingatan search results back to `MemoryCell` structs for injection. Users without ingatan get the native backend with no degradation.

**What needs to be built:**
- `internal/infrastructure/storage/ingatan_memory_cell_repository.go` — implements `memoryv2.MemoryCellRepository` via ingatan REST API
- `internal/infrastructure/storage/ingatan_memory_scene_repository.go` — implements `memoryv2.MemorySceneRepository` (maps to ingatan conversation summaries or a dedicated store)
- Config wiring in `main.go` to select backend at startup
- `IngatanConfig` struct in `config.go` (URL, APIKey, Store, TLSSkipVerify for self-signed certs)

**Open question before implementing:** Ingatan's auth requires a JWT Bearer token. Decide whether NuimanBot holds a long-lived JWT per user, or whether ingatan needs a token-exchange endpoint (API key → JWT). This shapes the login-as-user flow.

**Also note:** Using ingatan as the MCP target for item 1 (MCP Client) is a concrete, tested integration path rather than building a generic MCP client from scratch.
