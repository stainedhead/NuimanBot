# PRD: Improved Memory System — NuimanBot v2

**Status:** Draft
**Date:** 2026-03-22
**Scope:** Memory backend upgrade, Ingatan bridge, MCP client integration, REST API & TLS hardening

---

## 1. Executive Summary

NuimanBot v1 ships a capable built-in memory system (v2) with LLM-driven extraction, typed `MemoryCell` records, salience scoring, scene consolidation, and file-based storage. It works well for a single-user, in-process deployment. However, several gaps identified during the architecture review limit its usefulness as the agent scales:

1. **Search quality** — The current `SearchFTS` path uses simple keyword matching. There is no semantic/vector search, so queries that do not share exact tokens with stored cells yield poor recall.
2. **Storage isolation** — Memory is scoped only by `conversation_id`, with no per-user RBAC. In a multi-user deployment every user's memory is visible to any other user's query path.
3. **Ingest breadth** — The curator only watches conversation turns. Users cannot manually import external knowledge (URLs, files, PDFs) that the bot should remember.
4. **Security gaps** — HTTP servers run in plain-text mode; the session cookie `Secure` flag is `false`; the web admin UI has missing role checks and no rate limiting; there are no security controls at all on the planned REST API surface.
5. **MCP client is absent** — Config scaffolding exists (`MCPConfig`, `MCPClientConfig`) but no transport, protocol handling, or tool-registration bridge is implemented.

NuimanBot's sister project **Ingatan** (`~/Code/stainedhead/Golang/ingatan-product/ingatan-app`) already solves items 1, 2, 3, and the REST/TLS portions of item 4. It provides hybrid HNSW semantic + BM25 keyword search with RRF fusion, per-store Owner/Writer/Reader RBAC, REST + MCP APIs, JWT auth, TLS, and ingest pipelines for URLs and files.

This PRD proposes a **three-part upgrade**:

| Part | What | Why |
|------|------|-----|
| **A — Ingatan Bridge** | Add an Ingatan repository adapter alongside the existing file-based backend. Config-selectable. | Gets hybrid search, multi-user RBAC, and rich ingest with zero changes to the curator pipeline. |
| **B — TLS + REST API Hardening** | Implement self-signed cert generation; flip all HTTP surfaces to TLS; add missing security controls to the web admin UI and the planned REST API. | Closes the most critical security gaps identified in the architecture review. |
| **C — MCP Client** | Implement the MCP transport + protocol layer; bridge discovered MCP tools into the domain `Tool` registry. Use Ingatan's MCP server as the first concrete target. | Delivers the only fully unimplemented feature area; using Ingatan as the target provides a tested integration path. |

Each part is independently deployable and can be specced as a separate feature.

---

## 2. Problem Statements

### 2.1 Weak Memory Recall

**Current behaviour:** `MemoryRecallService.RecallMemory` calls `SearchFTS` which does BM25-style keyword matching against stored `MemoryCell.Content`. If the user says "remind me about the deployment pipeline" but the stored cell says "CI/CD workflow uses GitHub Actions", the query finds nothing and falls back to high-salience cells — an expensive fallback that injects unrelated memory.

**Desired behaviour:** A semantic embedding search that can match "deployment pipeline" to "CI/CD workflow" across the lexical gap. Ingatan's hybrid search (HNSW + BM25 + RRF) already solves this.

**Impact:** Missed memory recall degrades agent quality; false-negative fallback injects irrelevant context, increasing token cost and hallucination risk.

---

### 2.2 No Multi-User Memory Isolation

**Current behaviour:** `MemoryCellRepository` is scoped by `conversation_id`. Nothing prevents the query path from surfacing cells belonging to a different user's conversation if their IDs happen to share a scene.

**Desired behaviour:** Each user owns a dedicated memory store. Queries are always scoped to the requesting user's store. Sharing is explicit and role-controlled.

**Impact:** Privacy violation risk; a prerequisite for any team/org deployment.

---

### 2.3 Manual Ingest Is Impossible

**Current behaviour:** Memory cells are created exclusively by the `MemoryCuratorService` watching conversation turns. There is no way for a user to tell the bot "remember this article" or "import this document".

**Desired behaviour:** Users can save a URL or file path; the bot ingests, chunks, embeds, and stores it in their memory — retrievable by future queries.

**Impact:** Significantly narrows the bot's knowledge base; blocks use cases like "I've read this RFC, keep it in mind for our discussions".

---

### 2.4 Plain-Text HTTP / Missing Security Controls

**Current state (from architecture review):**

| Control | Web Admin UI | Planned REST API |
|---------|-------------|-----------------|
| TLS | ❌ Plain HTTP | ❌ |
| Session cookie `Secure` flag | ❌ `false` | N/A |
| Role check on all admin routes | ⚠️ Partial (`handleUsers` only) | ❌ |
| Login rate limiting | ❌ | ❌ |
| Input sanitization on form values | ❌ | ❌ |
| Default credential rotation | ❌ Hardcoded `admin/admin` | N/A |
| API key / JWT auth | N/A | ❌ |
| Per-client rate limiting | N/A | ❌ |
| Request body size limits | N/A | ❌ |

**Desired behaviour:** All HTTP surfaces run over TLS. Admin routes enforce role checks. Login is rate-limited. The REST API requires JWT bearer tokens, enforces rate limits per client, and validates input at the boundary.

---

### 2.5 MCP Client Is Unimplemented

**Current state:** `MCPConfig`, `MCPClientConfig`, and `MCPServerConfig` exist in `config.go` as scaffolding. There is no transport layer, no JSON-RPC protocol handling, and no `mcp.json` loader. The `domain.Tool` registry has no MCP-sourced entries. `go.mod` has no MCP dependency.

**Desired behaviour:** NuimanBot can connect to an MCP server (e.g. Ingatan), call `initialize` + `tools/list` at startup, and register the discovered tools in the `domain.Tool` registry so they are available to users in chat.

---

## 3. Goals and Non-Goals

### Goals

- **G1** — Improve memory recall quality by bridging to Ingatan's hybrid search backend (configurable fallback to built-in).
- **G2** — Enforce per-user memory isolation using Ingatan's per-store RBAC.
- **G3** — Enable manual memory ingest from URLs and local files through Ingatan's ingest pipeline.
- **G4** — Harden all HTTP surfaces to TLS using auto-generated self-signed certificates.
- **G5** — Close identified web admin UI security gaps (role enforcement, rate limiting, sanitization, credential rotation).
- **G6** — Implement REST API security controls (JWT auth, rate limiting, body-size limits, input validation).
- **G7** — Implement an MCP client that connects to any MCP server and bridges discovered tools into the tool registry.
- **G8** — Use Ingatan's MCP server as the first validated MCP target, delivering concrete memory/search tools over MCP.

### Non-Goals

- Replacing the built-in memory backend — it remains the default; Ingatan is opt-in.
- Rewriting the curator service — it continues to extract `MemoryCell` records; the bridge maps them to Ingatan's `Memory` model.
- Kubernetes deployment — remains on hold.
- Changing the Ingatan codebase — NuimanBot is a consumer of Ingatan's existing REST + MCP APIs.

---

## 4. User Stories

### Part A — Ingatan Bridge

| ID | Story |
|----|-------|
| US-A1 | As a **user**, when I ask the bot about something discussed weeks ago, it retrieves the right memory even when my wording differs from what was stored. |
| US-A2 | As an **admin**, I can configure `memory.backend: ingatan` and the bot uses Ingatan for all memory read/write without code changes. |
| US-A3 | As a **user**, I can tell the bot "remember this URL" and it fetches, chunks, and stores the content for future recall. |
| US-A4 | As a **user**, I can tell the bot "remember this file" and it reads, chunks, and stores the content for future recall. |
| US-A5 | As an **operator**, each user's memory is isolated in their own Ingatan store; no cross-user leakage is possible. |
| US-A6 | As an **admin**, if Ingatan is unavailable, the bot degrades gracefully to the built-in backend (configurable). |

### Part B — TLS + Security Hardening

| ID | Story |
|----|-------|
| US-B1 | As an **operator**, all HTTP endpoints (web admin, REST API, health) are served over HTTPS without requiring a CA-issued certificate. |
| US-B2 | As an **admin**, I am forced to change the default password on first login; the system refuses to accept `admin/admin`. |
| US-B3 | As an **attacker**, I cannot brute-force the admin login because it rate-limits failed attempts. |
| US-B4 | As a **developer**, I can call the REST API using a Bearer JWT obtained by exchanging an API key at `POST /auth/token`. |
| US-B5 | As an **operator**, all admin routes require the caller to hold the `admin` role; a regular-user JWT cannot access them. |

### Part C — MCP Client

| ID | Story |
|----|-------|
| US-C1 | As a **user**, I can use tools exposed by an MCP server (e.g. Ingatan's `memory_search`) in my chat session without any extra configuration beyond pointing NuimanBot at the server. |
| US-C2 | As an **admin**, I configure MCP servers in `mcp.json` and they are loaded at startup. |
| US-C3 | As a **developer**, newly discovered MCP tools appear alongside built-in tools in `/help`. |

---

## 5. Architecture

### 5.1 Part A — Ingatan Bridge

#### 5.1.1 Dual-Backend Repository Pattern

The existing `domain/memoryv2` package defines two repository interfaces:

```
MemoryCellRepository  — Create, Get, List, Delete, SearchFTS, GetByScene, GetHighSalience, DeleteExpired
MemorySceneRepository — Upsert, Get, List, Delete
```

The file-based implementations live in `internal/infrastructure/storage/`. The Ingatan bridge adds two new implementations in the same package:

```
internal/infrastructure/storage/
├── file_memory_cell_repository.go        (existing)
├── file_memory_scene_repository.go       (existing)
├── ingatan_memory_cell_repository.go     (NEW)
└── ingatan_memory_scene_repository.go    (NEW)
```

Selection is driven by the updated `MemoryConfig`:

```yaml
memory:
  backend: ingatan     # or "builtin" (default)
  ingatan:
    url: https://localhost:8443
    api_key: <user-provided>
    store_prefix: nuiman   # store name = store_prefix + "_" + user_id
    tls_skip_verify: false  # true for dev with self-signed certs
    token_ttl: 23h          # how long before re-exchanging the API key for a JWT
```

#### 5.1.2 Ingatan Auth Flow

Ingatan uses API key → JWT exchange (`POST /auth/token`). NuimanBot holds a per-user API key (stored in the credential vault). On startup (or when the cached JWT is near expiry), it exchanges the key for a JWT and caches it. All subsequent Ingatan calls use `Authorization: Bearer <jwt>`.

```
NuimanBot startup
  └── IngatanTokenCache.Refresh(apiKey)
        └── POST https://ingatan/auth/token {"api_key": "..."}
              → {"token": "eyJ...", "expires_at": "..."}
        └── cache JWT until expires_at - token_ttl_buffer
```

#### 5.1.3 MemoryCell → Ingatan Memory Mapping

| NuimanBot `MemoryCell` field | Ingatan `Memory` field | Notes |
|------------------------------|------------------------|-------|
| `Content` | `Content` | Direct |
| `Scene` | `Tags[0]` + `Metadata["scene"]` | Tag enables BM25 filter; metadata preserves structured value |
| `CellType.String()` | `Metadata["cell_type"]` | |
| `Salience` | `Metadata["salience"]` | float64 JSON |
| `ConversationID` | `SourceRef` (when Source=conversation) | |
| `Source` (JSON array of msg IDs) | `Metadata["source_message_ids"]` | |
| `ID` | `Metadata["nuiman_cell_id"]` | Round-trip identity for Get/Delete |

`MemoryScene` maps to a separate Ingatan `Memory` with `Source=manual`, `Tags=["_scene"]`, and `Content=Summary`. This allows scene summaries to be retrieved and updated independently.

#### 5.1.4 Search Mapping

`SearchFTS(query, limit)` → `POST /api/v1/stores/{store}/memories/search` with `mode: hybrid, top_k: limit`. The Ingatan response `SearchResult` objects are mapped back to `MemoryCell` via the `metadata` fields preserved during save.

`GetHighSalience(conversationID, threshold, limit)` → `GET /api/v1/stores/{store}/memories?source=conversation` filtered client-side by `metadata["salience"] >= threshold`.

#### 5.1.5 Config Changes

Add `IngatanConfig` struct to `config.go`:

```go
type IngatanConfig struct {
    URL           string              `yaml:"url"`
    APIKey        domain.SecureString `yaml:"api_key"`
    StorePrefix   string              `yaml:"store_prefix"`
    TLSSkipVerify bool                `yaml:"tls_skip_verify"`
    TokenTTL      string              `yaml:"token_ttl"`
}

// MemoryBackend constants
const (
    MemoryBackendBuiltin  MemoryBackend = "builtin"
    MemoryBackendIngatan  MemoryBackend = "ingatan"
)

type MemoryConfig struct {
    Backend  MemoryBackend  `yaml:"backend"`
    Citations MemoryCitationsMode `yaml:"citations"`
    QMD      MemoryQMDConfig `yaml:"qmd"`
    Ingatan  IngatanConfig   `yaml:"ingatan"`
}
```

#### 5.1.6 Graceful Degradation

If `backend: ingatan` is configured but Ingatan is unreachable at startup, NuimanBot logs a warning and falls back to the built-in backend (if `fallback_to_builtin: true` in config, which is the default). A health check tracks Ingatan reachability. On degraded mode, an alert fires.

---

### 5.2 Part B — TLS + Security Hardening

#### 5.2.1 Certificate Auto-Generation

New package `internal/infrastructure/crypto/` (extends existing):

```go
// LoadOrGenerateCert checks for certFile/keyFile.
// If absent, generates a self-signed ECDSA P-256 cert valid for 365 days
// for the given hosts, writes it to certFile/keyFile, and returns the tls.Certificate.
func LoadOrGenerateCert(certPath, keyPath string, hosts []string) (tls.Certificate, error)
```

Uses only stdlib: `crypto/tls`, `crypto/x509`, `crypto/ecdsa`, `crypto/elliptic`, `encoding/pem`.

#### 5.2.2 Shared TLS Config Struct

Added to `config.go`:

```go
type TLSConfig struct {
    Enabled      bool     `yaml:"enabled"`
    AutoGenerate bool     `yaml:"auto_generate"`  // default: true
    CertFile     string   `yaml:"cert_file"`
    KeyFile      string   `yaml:"key_file"`
    Hosts        []string `yaml:"hosts"`           // default: ["localhost"]
}
```

#### 5.2.3 Server TLS Upgrades

- **`health.Server`**: Add `StartTLS(tlsCfg TLSConfig)` path that calls `http.ListenAndServeTLS`.
- **`web.Server`**: Add `StartTLS(tlsCfg TLSConfig)` path; flip `session.Options.Secure = true` when TLS is active.
- **REST API Server** (Part B + C): Start with TLS from the outset.

#### 5.2.4 Web Admin UI Security Gaps

| Gap | Fix |
|-----|-----|
| `handleUsers` only checks `user == nil`, not role | Add `requireRole(admin)` middleware applied to all admin routes |
| No login rate limiting | Token bucket rate limiter per remote IP on `POST /login` (5 req/min, burst 3) — reuse existing `internal/infrastructure/ratelimit/token_bucket.go` |
| Input not sanitized on form values | Pass form values through the existing `security.SanitizeInput()` before passing to service layer |
| Default `admin/admin` never rotated | On first successful `admin` login, if password matches default, redirect to `/admin/change-password` and block all other admin routes until changed |

#### 5.2.5 REST API Security Controls

The REST API server (new `internal/adapter/api/`) will use:

- **JWT middleware** — `Authorization: Bearer <jwt>`, HS256, 24h TTL. `POST /api/v1/auth/token` exchanges an API key for a JWT.
- **Per-client rate limiting** — Token bucket per authenticated principal ID (100 req/min, burst 20), using the existing rate limiter.
- **Body size limit** — `http.MaxBytesReader(w, r.Body, 1<<20)` (1 MiB) on all POST/PUT routes.
- **Input validation** — All string fields validated through `security.ValidateInput()` at the handler boundary before reaching the service layer.

---

### 5.3 Part C — MCP Client

#### 5.3.1 mcp.json Loader

New file `internal/infrastructure/mcp/config_loader.go`. Reads `mcp.json` from the config directory:

```json
{
  "servers": [
    {
      "name": "ingatan",
      "transport": "http",
      "url": "https://localhost:8443/mcp",
      "api_key": "${INGATAN_API_KEY}"
    },
    {
      "name": "local-tool",
      "transport": "stdio",
      "command": "/usr/local/bin/my-mcp-tool"
    }
  ]
}
```

Environment variable substitution (`${VAR}`) is performed at load time.

#### 5.3.2 Transport Layer

New package `internal/infrastructure/mcp/`:

```
internal/infrastructure/mcp/
├── config_loader.go        — mcp.json loader
├── transport.go            — Transport interface
├── http_transport.go       — SSE or HTTP transport
├── stdio_transport.go      — stdio transport
└── client.go               — JSON-RPC client over Transport
```

`Transport` interface:

```go
type Transport interface {
    // Send sends a JSON-RPC request and returns the response body.
    Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
    Close() error
}
```

#### 5.3.3 JSON-RPC Protocol

`client.go` implements:

- `Initialize(ctx, clientInfo)` → `initialize` method, stores `serverCapabilities`.
- `ListTools(ctx)` → `tools/list` method, returns `[]MCPTool`.
- `CallTool(ctx, name, args)` → `tools/call` method, returns `MCPToolResult`.

```go
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema
}

type MCPToolResult struct {
    Content []MCPContent `json:"content"`
    IsError bool         `json:"isError"`
}
```

#### 5.3.4 Domain Tool Bridge

New `internal/adapter/mcp/tool_bridge.go`:

- Converts each `MCPTool` into a `domain.Tool` implementation.
- `Execute(ctx, args)` calls `client.CallTool(ctx, tool.Name, args)` and returns the result as a string.
- Tools are registered in the `domain.Tool` registry under the namespace `mcp:<server-name>:<tool-name>` (e.g. `mcp:ingatan:memory_search`).

#### 5.3.5 Startup Integration

In `cmd/nuimanbot/main.go`, after the tool registry is built:

```
for each server in mcp.json:
    connect transport
    initialize client
    list tools
    for each discovered tool:
        wrap in domain.Tool adapter
        register in tool registry
```

Failures per server are logged and skipped — one bad MCP server does not prevent startup.

#### 5.3.6 Ingatan as First MCP Target

Ingatan's MCP server exposes (from `internal/adapter/mcp/`):

- `memory_save` — save a memory to a store
- `memory_get` — retrieve a memory by ID
- `memory_search` — hybrid search across a store
- `memory_delete` — delete a memory
- `ingest_url` — fetch and ingest a URL
- `ingest_file` — read and ingest a file
- `store_list` — list accessible stores

With the MCP client live, NuimanBot users gain direct access to these tools in chat without any custom adapter code.

---

## 6. Data Flow

### 6.1 Memory Write (Ingatan backend)

```
User message
  └── ChatService.ProcessMessage
        └── MemoryCuratorService.ExtractCells
              └── LLM extraction (unchanged)
              └── for each MemoryCell:
                    IngatanMemoryCellRepository.Create
                      └── map MemoryCell → ingatan.SaveRequest
                      └── POST https://ingatan/api/v1/stores/{store}/memories
              └── IngatanMemoryCuratorService.ConsolidateScene
                    └── GET /stores/{store}/memories?tags=scene-name
                    └── LLM consolidation (unchanged)
                    └── PUT /stores/{store}/memories/{scene-memory-id}
```

### 6.2 Memory Recall (Ingatan backend)

```
User query arrives
  └── MemoryRecallService.RecallMemory
        └── IngatanMemoryCellRepository.SearchFTS(query, limit)
              └── POST /stores/{store}/memories/search {"query": q, "mode": "hybrid", "top_k": limit}
              └── map SearchResult → MemoryCell (via metadata)
        └── if no results: GetHighSalience (fallback, same as built-in)
        └── applyBudgetWithScenes (unchanged)
        └── FormatMemoryForInjection (unchanged)
```

### 6.3 MCP Tool Call

```
User: "search my memory for deployment pipelines"
  └── ChatService detects tool call: mcp:ingatan:memory_search
        └── MCPToolAdapter.Execute(ctx, {"store": userStore, "query": "deployment pipelines"})
              └── MCPClient.CallTool("memory_search", args)
                    └── HTTP POST https://ingatan/mcp (JSON-RPC tools/call)
              └── format result string
        └── inject result into LLM context
```

---

## 7. Configuration Reference

### Full memory config example (Ingatan backend)

```yaml
memory:
  backend: ingatan
  citations: auto
  ingatan:
    url: https://localhost:8443
    api_key: ${INGATAN_API_KEY}
    store_prefix: nuiman
    tls_skip_verify: true      # for self-signed dev certs
    token_ttl: 23h
    fallback_to_builtin: true  # degrade gracefully if ingatan is down

tls:
  enabled: true
  auto_generate: true
  cert_file: data/certs/server.crt
  key_file: data/certs/server.key
  hosts:
    - localhost
    - nuimanbot.internal

mcp:
  client:
    config_file: mcp.json
  server:
    enabled: false
    port: 8080
    tls: true
```

---

## 8. Security Considerations

### 8.1 API Key Storage

Ingatan API keys are stored in NuimanBot's encrypted credential vault (AES-256-GCM), never in plaintext config files. The `domain.SecureString` type ensures they are redacted from logs.

### 8.2 JWT Caching

Exchanged JWTs are held in memory only, never written to disk. On process restart they are re-exchanged. Token expiry is tracked; refresh happens before expiry using a configurable buffer (`token_ttl`).

### 8.3 TLS Verification

`tls_skip_verify: true` is available for development with self-signed certs but must never be set in production. Linting should warn if this is `true` in a non-dev environment. The auto-generated cert is written to `data/certs/` which is gitignored.

### 8.4 MCP Trust Boundary

MCP tools execute with the same RBAC and tool-permission constraints as built-in tools. A user cannot call an MCP tool unless the tool is in their allowlist (same permission model as existing tools). Admin-only MCP tools are flagged with a permission requirement in the bridge adapter.

### 8.5 Input Sanitization on MCP Results

MCP tool results pass through the same `SanitizeOutput` pipeline as built-in tool results before being injected into the LLM context, preventing prompt injection via malicious MCP server responses.

---

## 9. Testing Strategy

Following the project's TDD methodology (Red-Green-Refactor):

### Part A

| Test | Type | Scope |
|------|------|-------|
| `IngatanMemoryCellRepository` CRUD + Search | Unit (mock HTTP) | `internal/infrastructure/storage/` |
| Token exchange + cache refresh | Unit | `internal/infrastructure/mcp/` |
| MemoryCell ↔ ingatan.Memory mapping round-trip | Unit | mapping functions |
| Fallback to built-in when Ingatan is down | Unit | config wiring in `main.go` |
| Full recall flow with Ingatan backend | Integration (Ingatan running) | `usecase/memoryv2/` |

### Part B

| Test | Type | Scope |
|------|------|-------|
| `LoadOrGenerateCert` creates valid cert on first call, reuses on second | Unit | `internal/infrastructure/crypto/` |
| Web admin role check blocks non-admin | Unit | `internal/adapter/web/` |
| Login rate limiter returns 429 after threshold | Unit | `internal/adapter/web/` |
| REST API rejects requests without Bearer token | Unit | `internal/adapter/api/` |
| REST API enforces body-size limit | Unit | `internal/adapter/api/` |
| Default password change redirect on first login | Unit | `internal/adapter/web/` |

### Part C

| Test | Type | Scope |
|------|------|-------|
| `mcp.json` loader parses all transport types | Unit | `internal/infrastructure/mcp/` |
| HTTP transport sends correct JSON-RPC framing | Unit (mock server) | `internal/infrastructure/mcp/` |
| `initialize` + `tools/list` populates registry | Unit | `internal/adapter/mcp/` |
| Discovered tool executes via `tools/call` | Unit | `internal/adapter/mcp/` |
| Bad MCP server skipped at startup | Unit | `cmd/nuimanbot/main.go` |
| End-to-end: Ingatan MCP tool called from chat | Integration | |

---

## 10. Acceptance Criteria

### Part A — Ingatan Bridge

- [ ] `memory.backend: ingatan` routes all cell reads/writes through Ingatan REST API.
- [ ] Hybrid search returns semantically relevant results even when query tokens don't match stored content exactly (validated by integration test with a pre-seeded Ingatan store).
- [ ] Each user's memory is isolated in `{store_prefix}_{user_id}` store; no cross-user data is accessible.
- [ ] `POST /stores/{store}/memories` is called with correct metadata mapping (verified in unit test).
- [ ] When Ingatan is unreachable and `fallback_to_builtin: true`, bot continues using built-in memory with a logged warning.
- [ ] All quality gates pass: `go fmt`, `go vet`, `golangci-lint`, `go test ./...`, `go build -o bin/nuimanbot`.

### Part B — TLS + Security Hardening

- [ ] `./bin/nuimanbot` starts HTTPS on the web admin port when `tls.enabled: true`.
- [ ] Self-signed cert is auto-generated to `data/certs/` if files are absent; reused on restart.
- [ ] `Secure: true` is set on the session cookie when TLS is active.
- [ ] All admin routes return 403 for a non-admin authenticated user.
- [ ] Login rate limit returns HTTP 429 after 5 failed attempts within 1 minute.
- [ ] `POST /api/v1/auth/token` returns a JWT; subsequent calls with that JWT succeed; calls without a token return 401.
- [ ] POST requests with body > 1 MiB return 413.
- [ ] First login with default `admin/admin` redirects to the password-change page.

### Part C — MCP Client

- [ ] `mcp.json` with at least one `http` transport entry loads without error.
- [ ] Discovered tools appear in `/help` output under `mcp:<server>:` prefix.
- [ ] A chat message that triggers an MCP tool call returns a response derived from the MCP server's output.
- [ ] A misconfigured MCP server in `mcp.json` is skipped with a logged error; bot starts normally.
- [ ] Ingatan's `memory_search` tool is callable from chat when Ingatan MCP is configured.

---

## 11. Dependencies and Open Questions

### Dependencies

| Dependency | Status | Notes |
|-----------|--------|-------|
| Ingatan running at `localhost:8443` | External | Required for Ingatan backend + MCP integration tests |
| `github.com/mark3labs/mcp-go` or hand-rolled JSON-RPC | Decision needed | See OQ-1 |
| `github.com/golang-jwt/jwt/v5` | Likely already in `go.mod` via Ingatan review | Confirm before adding |

### Open Questions

**OQ-1: MCP Go library vs. hand-rolled JSON-RPC**
The summary notes `github.com/mark3labs/mcp-go` as a candidate. Evaluate against hand-rolling. The protocol surface needed (initialize, tools/list, tools/call) is small. A dependency adds supply-chain risk; hand-rolling adds maintenance. **Recommendation: hand-roll the client.** The protocol is stable enough and the required surface is < 300 lines.

**OQ-2: Ingatan token exchange — long-lived API key or per-session JWT?**
NuimanBot can hold one API key per Ingatan user (stored in vault) and exchange it at startup. Alternatively, Ingatan could issue a long-lived token. The current `POST /auth/token` in Ingatan issues a 24h JWT. NuimanBot should cache and refresh it transparently (see §5.1.2). **No Ingatan changes required.**

**OQ-3: Store naming when user ID is a platform-specific string (e.g. Telegram chat ID)**
Ingatan store names must be lowercase alphanumeric. NuimanBot user IDs may contain colons, hyphens, or uppercase. **Recommendation: SHA256-hash the user ID and use the first 16 hex chars as the store suffix, with `store_prefix_` as the namespace. Store a `nuiman_user_id` field in the store metadata for reverse lookup.**

**OQ-4: Should the MCP client use SSE or HTTP transport for Ingatan?**
Ingatan's MCP server likely uses HTTP transport (based on the `url` field in the MCP server config). Confirm from Ingatan's `cmd/ingatan/main.go`. Implement HTTP first; add SSE when needed.

---

## 12. Implementation Phases (Suggested)

| Phase | Scope | Estimate |
|-------|-------|---------|
| **Phase 1** | Part A — Ingatan bridge (repositories, auth, config, mapping) | 3–4 days |
| **Phase 2** | Part B — TLS auto-generation + server upgrades | 1–2 days |
| **Phase 3** | Part B — Web admin security gap fixes | 1 day |
| **Phase 4** | Part B — REST API security controls | 1–2 days |
| **Phase 5** | Part C — MCP client transport + protocol | 2 days |
| **Phase 6** | Part C — Domain tool bridge + startup integration | 1 day |
| **Phase 7** | Integration testing: Ingatan MCP as first target | 1 day |

All phases follow the project's TDD methodology and must pass all quality gates before completion.

---

## 13. Out-of-Scope (Future Consideration)

- Vector/semantic search in the built-in backend (would require an embedding provider dependency — deferred in favour of the Ingatan bridge approach).
- Multi-tenancy in the built-in backend (deferred; Ingatan handles this).
- Docker/Kubernetes deployment (remains on hold per existing plan).
- Comprehensive linting cleanup (remains on hold).
- MCP server implementation for NuimanBot (not needed; Ingatan already provides the target).

---

*Prepared from architecture review findings in `summary-todo-items.md`, analysis of `internal/domain/memoryv2/`, `internal/usecase/memoryv2/`, `internal/config/config.go`, and the Ingatan codebase at `~/Code/stainedhead/Golang/ingatan-product/ingatan-app/`.*
