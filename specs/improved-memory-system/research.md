# Research: Improved Memory System

**Date:** 2026-03-22

---

## 1. Existing NuimanBot Memory System (v2)

### Domain Layer (`internal/domain/memoryv2/`)

**`MemoryCell`** — core knowledge unit:
- Fields: `ID` (UUID), `ConversationID`, `Scene` (lowercase 3-64 chars), `CellType`, `Salience` (0.0-1.0), `Content` (max 2000 chars), `Source` (JSON array of msg IDs), `CreatedAt`, `UpdatedAt`, `ExpiresAt`
- Validation: full business rule validation via `Validate()`
- `IsExpired()` helper

**`CellType` enum:**
- `fact`, `decision`, `task`, `preference`, `plan`, `risk`

**`MemoryScene`** — per-topic consolidated summary:
- Fields: `Scene`, `Summary` (max 10000 chars), `TokenCount` (max 2000), `UpdatedAt`

**Repository interfaces (defined in use case, implemented in infra):**
- `MemoryCellRepository`: `Create`, `Get`, `List`, `Delete`, `SearchFTS`, `GetByScene`, `GetHighSalience`, `DeleteExpired`
- `MemorySceneRepository`: `Upsert`, `Get`, `List`, `Delete`

### Use Case Layer (`internal/usecase/memoryv2/`)

**`MemoryCuratorService`:**
- `ExtractCells(ctx, InteractionContext)` — calls LLM to extract typed cells from a conversation turn; persists cells; triggers `ConsolidateScene` for touched scenes
- `ConsolidateScene(ctx, sceneName)` — fetches all cells for scene, calls LLM to generate/update summary, upserts `MemoryScene`
- Integrated with `tracing`, `metrics`, and `alerting` packages

**`MemoryRecallService`:**
- `RecallMemory(ctx, RecallRequest)` — FTS → fallback to high-salience; applies token budget; returns `RecallResponse`
- `FormatMemoryForInjection(RecallResponse)` — formats for LLM context injection

**`InteractionContext`:** `ConversationID`, `UserMessage`, `AssistantReply`, `ToolOutputs`, `MessageIDs`, `Timestamp`

### Infrastructure Layer (`internal/infrastructure/storage/`)

**`FileMemoryCellRepository`** — JSON file per `ConversationID`, in-memory index, BM25-style keyword search
**`FileMemorySceneRepository`** — JSON file per scene name

### Config (`internal/config/config.go`)

Current `MemoryConfig`:
```go
type MemoryConfig struct {
    Backend   MemoryBackend       // "builtin" | "qmd"
    Citations MemoryCitationsMode // "auto" | "on" | "off"
    QMD       MemoryQMDConfig
}
```
No `IngatanConfig` yet. `MemoryBackendIngatan` constant not defined.

---

## 2. Ingatan API Surface

### Authentication

`POST /auth/token` (unauthenticated):
- Request: `{"api_key": "..."}`
- Response: `{"token": "eyJ...", "expires_at": "2026-03-23T10:00:00Z"}`
- Token type: HS256 JWT, 24h TTL (configurable)
- Claims: `sub` (principal ID), `name`, `type`, `role`, `iat`, `exp`, `iss: "ingatan"`

All `/api/v1/*` routes require: `Authorization: Bearer <jwt>`

### Memory CRUD

```
POST   /api/v1/stores/{store}/memories
GET    /api/v1/stores/{store}/memories
GET    /api/v1/stores/{store}/memories/{memoryID}
PUT    /api/v1/stores/{store}/memories/{memoryID}
DELETE /api/v1/stores/{store}/memories/{memoryID}
```

**Save request body:**
```json
{
  "title": "string",
  "content": "string (required)",
  "tags": ["string"],
  "source": "manual|conversation|import|agent|file|url",
  "source_ref": "string",
  "metadata": {}
}
```

**Memory response:**
```json
{
  "id": "uuid",
  "store": "string",
  "title": "string",
  "content": "string",
  "tags": [],
  "source": "string",
  "source_ref": "string",
  "metadata": {},
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

### Search

`POST /api/v1/stores/{store}/memories/search`:
```json
{
  "query": "string (required)",
  "mode": "hybrid|semantic|keyword",
  "top_k": 10,
  "tags": []
}
```

Response: `{"results": [{"memory": {...}, "score": 0.95, "score_components": {"semantic": 0.9, "keyword": 0.8, "rrf": 0.95}}]}`

`GET /api/v1/stores/{store}/memories/{memoryID}/similar?top_k=10`

### Ingest

`POST /api/v1/stores/{store}/ingest/url` — `{"url": "...", "tags": [], "metadata": {}}`
`POST /api/v1/stores/{store}/ingest/file` — multipart form data

### Store Management

```
POST   /api/v1/stores
GET    /api/v1/stores
GET    /api/v1/stores/{store}
DELETE /api/v1/stores/{store}
POST   /api/v1/stores/{store}/members
DELETE /api/v1/stores/{store}/members/{principalID}
```

Store roles: `owner`, `writer`, `reader`

### MCP Tools (from `internal/adapter/mcp/`)

Ingatan exposes these MCP tools:
- `memory_save` — save a memory
- `memory_get` — get by ID
- `memory_search` — hybrid search
- `memory_delete` — delete
- `ingest_url` — fetch and ingest URL
- `ingest_file` — ingest local file
- `store_list` — list accessible stores

MCP transport: HTTP (URL: `https://localhost:8443/mcp`)

---

## 3. MemoryCell → Ingatan Mapping

| `MemoryCell` Field | Ingatan `Memory` Field | Strategy |
|-------------------|------------------------|----------|
| `Content` | `Content` | Direct |
| `Scene` | `Tags[0]`, `Metadata["scene"]` | Tag for BM25 filter; metadata for structured access |
| `CellType.String()` | `Metadata["cell_type"]` | Stored as string |
| `Salience` | `Metadata["salience"]` | float64 JSON value |
| `ConversationID` | `SourceRef` | When `Source == "conversation"` |
| `Source` (JSON array) | `Metadata["source_message_ids"]` | Round-trip as JSON string |
| `ID` | `Metadata["nuiman_cell_id"]` | For Get/Delete by NuimanBot ID |
| `ExpiresAt` | `Metadata["expires_at"]` | RFC3339 if non-nil |

**MemoryScene → Ingatan Memory:**
- `Source: "manual"`
- `Tags: ["_scene", scene_name]`
- `Content: summary`
- `Metadata["scene_name"]`, `Metadata["token_count"]`, `Metadata["nuiman_scene_id"]`

---

## 4. Store Naming Strategy

Ingatan store names must be alphanumeric + hyphens, lowercase. NuimanBot user IDs can be arbitrary strings (Telegram chat IDs, Slack user IDs, CLI usernames).

**Approach:** `{store_prefix}_{sha256(user_id)[:16]}`

Example: `store_prefix=nuiman`, `user_id="slack-U12345"` → `nuiman_8f3a9bc14e7d5021`

Store metadata stores `nuiman_user_id` for reverse lookup and debugging.

---

## 5. MCP Protocol (JSON-RPC 2.0 subset)

Required methods:

**`initialize`** (must be first call):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "clientInfo": {"name": "nuimanbot", "version": "1.0"},
    "capabilities": {}
  }
}
```
Response: `{"result": {"protocolVersion": "...", "capabilities": {}, "serverInfo": {...}}}`

**`tools/list`**:
```json
{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}
```
Response: `{"result": {"tools": [{"name": "...", "description": "...", "inputSchema": {...}}]}}`

**`tools/call`**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {"name": "memory_search", "arguments": {"store": "nuiman_abc123", "query": "..."}
  }
}
```
Response: `{"result": {"content": [{"type": "text", "text": "..."}], "isError": false}}`

**Transport options:**
- HTTP: POST to URL, `Content-Type: application/json`
- stdio: write to stdin, read from stdout (newline-delimited)

---

## 6. TLS / Crypto Research

**stdlib-only cert generation** (no external deps):
```go
// Key generation
key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

// Cert template
template := x509.Certificate{
    SerialNumber:          bigint,
    Subject:               pkix.Name{CommonName: "localhost"},
    NotBefore:             now,
    NotAfter:              now.Add(365 * 24 * time.Hour),
    KeyUsage:              x509.KeyUsageDigitalSignature,
    ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    BasicConstraintsValid: true,
    DNSNames:              hosts,
    IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
}

// Self-sign
certDER, _ := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
```
Write `certDER` as PEM to `certFile`, marshal `key` as PKCS8 PEM to `keyFile`.

---

## 7. Existing Security Infrastructure

- `internal/usecase/security/input_validation.go` — `ValidateInput()`, `SanitizeInput()`
- `internal/infrastructure/ratelimit/token_bucket.go` — token bucket rate limiter (reusable for login and REST API)
- `internal/adapter/web/auth.go` — bcrypt + session IDs + CSRF tokens (existing)
- `internal/infrastructure/crypto/vault.go` — AES-256-GCM credential vault (reuse for API key storage)

---

## 8. JWT Library

`github.com/golang-jwt/jwt/v5` — check if already in `go.mod`:
```bash
grep golang-jwt go.mod
```
If not present, add. This is the same library Ingatan uses.

---

## 9. Open Questions Resolved

- **OQ-1 MCP library**: Hand-roll JSON-RPC client. Surface is small (3 methods, ~300 lines). Avoids supply-chain risk.
- **OQ-2 Ingatan auth**: Hold one API key per user in vault; exchange at startup for 24h JWT; refresh transparently.
- **OQ-3 Store naming**: SHA256 of user_id, first 16 hex chars as suffix.
- **OQ-4 MCP transport**: HTTP first (Ingatan uses HTTP); stdio added later.
