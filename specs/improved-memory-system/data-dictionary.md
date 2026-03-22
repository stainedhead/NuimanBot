# Data Dictionary: Improved Memory System

**Date:** 2026-03-22

---

## New Domain Entities

### `IngatanConfig` (config.go)

```go
type IngatanConfig struct {
    URL               string              `yaml:"url"`              // e.g. "https://localhost:8443"
    APIKey            domain.SecureString `yaml:"api_key"`          // stored in vault, redacted in logs
    StorePrefix       string              `yaml:"store_prefix"`     // e.g. "nuiman"
    TLSSkipVerify     bool                `yaml:"tls_skip_verify"`  // dev only; warn in prod
    TokenTTL          string              `yaml:"token_ttl"`        // e.g. "23h" — how long before JWT refresh
    FallbackToBuiltin bool                `yaml:"fallback_to_builtin"` // default: true
}
```

### Updated `MemoryConfig` (config.go)

```go
type MemoryConfig struct {
    Backend   MemoryBackend       `yaml:"backend"`   // "builtin" | "ingatan"
    Citations MemoryCitationsMode `yaml:"citations"`
    QMD       MemoryQMDConfig     `yaml:"qmd"`
    Ingatan   IngatanConfig       `yaml:"ingatan"`   // NEW
}

// New constant
const MemoryBackendIngatan MemoryBackend = "ingatan"
```

### `TLSConfig` (config.go)

```go
type TLSConfig struct {
    Enabled      bool     `yaml:"enabled"`
    AutoGenerate bool     `yaml:"auto_generate"` // default: true
    CertFile     string   `yaml:"cert_file"`     // e.g. "data/certs/server.crt"
    KeyFile      string   `yaml:"key_file"`      // e.g. "data/certs/server.key"
    Hosts        []string `yaml:"hosts"`         // default: ["localhost"]
}
```

---

## New Infrastructure Types

### `IngatanHTTPClient` (`internal/infrastructure/storage/ingatan_client.go`)

```go
type IngatanHTTPClient struct {
    baseURL       string
    httpClient    *http.Client    // pre-configured with TLS
    tokenCache    *TokenCache
    apiKey        string
}

type TokenCache struct {
    mu        sync.RWMutex
    token     string
    expiresAt time.Time
}
```

Key methods:
- `Do(ctx, method, path, body) (*http.Response, error)` — injects `Authorization: Bearer` header, refreshes token if needed
- `Refresh(ctx) error` — exchanges API key for JWT, updates cache

### `IngatanMemoryCellRepository` (`internal/infrastructure/storage/ingatan_memory_cell_repository.go`)

Implements `domain/memoryv2.MemoryCellRepository`.

```go
type IngatanMemoryCellRepository struct {
    client      *IngatanHTTPClient
    storePrefix string
}
```

Store name derivation:
```go
func (r *IngatanMemoryCellRepository) storeFor(conversationID string) string {
    hash := sha256.Sum256([]byte(conversationID))
    return r.storePrefix + "_" + hex.EncodeToString(hash[:])[:16]
}
```

### `IngatanMemorySceneRepository` (`internal/infrastructure/storage/ingatan_memory_scene_repository.go`)

Implements `domain/memoryv2.MemorySceneRepository`.

Scene memories are stored with:
- `Tags: ["_scene", scene_name]`
- `Metadata["scene_name"]`: scene name for reverse lookup
- `Metadata["token_count"]`: integer
- `Metadata["nuiman_scene_id"]`: scene name (used as stable ID)

---

## New Crypto Types

### `LoadOrGenerateCert` (`internal/infrastructure/crypto/cert.go`)

```go
// LoadOrGenerateCert loads a TLS certificate from certPath/keyPath.
// If the files do not exist, generates a self-signed ECDSA P-256 certificate
// valid for 365 days for the given hosts, writes to certPath/keyPath, and returns it.
func LoadOrGenerateCert(certPath, keyPath string, hosts []string) (tls.Certificate, error)
```

---

## New MCP Types

### `MCPClientConfig` — extends existing struct

The existing `MCPClientConfig` in `config.go` is extended to include `ConfigFile`:
```go
type MCPClientConfig struct {
    ConfigFile string `yaml:"config_file"` // path to mcp.json; default: "mcp.json"
    Enabled    bool   `yaml:"enabled"`
}
```

### `MCPServerEntry` (`internal/infrastructure/mcp/config_loader.go`)

```go
type MCPServerEntry struct {
    Name      string            `json:"name"`
    Transport string            `json:"transport"` // "http" | "stdio"
    URL       string            `json:"url"`       // for http transport
    Command   string            `json:"command"`   // for stdio transport
    Args      []string          `json:"args"`      // for stdio transport
    Headers   map[string]string `json:"headers"`   // optional extra headers
}

type MCPConfig struct {
    Servers []MCPServerEntry `json:"servers"`
}
```

### `Transport` interface (`internal/infrastructure/mcp/transport.go`)

```go
type Transport interface {
    Send(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
    Close() error
}
```

### `MCPClient` (`internal/infrastructure/mcp/client.go`)

```go
type MCPClient struct {
    transport    Transport
    serverName   string
    capabilities ServerCapabilities
    mu           sync.RWMutex
    initialized  bool
}

type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

type MCPToolResult struct {
    Content []MCPContent `json:"content"`
    IsError bool         `json:"isError"`
}

type MCPContent struct {
    Type string `json:"type"` // "text" | "image" | "resource"
    Text string `json:"text"`
}

type ServerCapabilities struct {
    Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
    ListChanged bool `json:"listChanged,omitempty"`
}
```

### `MCPToolAdapter` (`internal/adapter/mcp/tool_bridge.go`)

Implements `domain.Tool`:

```go
type MCPToolAdapter struct {
    client      *mcp.MCPClient
    toolDef     mcp.MCPTool
    serverName  string
    permissions domain.Permission
}

// Name returns "mcp:<serverName>:<toolName>"
func (a *MCPToolAdapter) Name() string

// Description returns the MCP tool description
func (a *MCPToolAdapter) Description() string

// Execute calls tools/call on the MCP server and returns the text result
func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (string, error)
```

---

## Mapping Reference

### MemoryCell → Ingatan saveMemoryBody

```
MemoryCell.Content         → SaveRequest.Content
MemoryCell.Scene           → SaveRequest.Tags[0], Metadata["scene"]
MemoryCell.CellType        → Metadata["cell_type"]
MemoryCell.Salience        → Metadata["salience"]
MemoryCell.ConversationID  → SaveRequest.SourceRef, Source="conversation"
MemoryCell.Source (JSON)   → Metadata["source_message_ids"]
MemoryCell.ID              → Metadata["nuiman_cell_id"]
MemoryCell.ExpiresAt       → Metadata["expires_at"] (RFC3339 or absent)
```

### Ingatan SearchResult → MemoryCell

```
Memory.Content              → MemoryCell.Content
Metadata["scene"]           → MemoryCell.Scene
Metadata["cell_type"]       → MemoryCell.CellType (via ParseCellType)
Metadata["salience"]        → MemoryCell.Salience (float64)
Memory.SourceRef            → MemoryCell.ConversationID
Metadata["source_message_ids"] → MemoryCell.Source
Metadata["nuiman_cell_id"]  → MemoryCell.ID
Memory.CreatedAt            → MemoryCell.CreatedAt
Memory.UpdatedAt            → MemoryCell.UpdatedAt
Metadata["expires_at"]      → MemoryCell.ExpiresAt (parse RFC3339)
```

---

## Error Types

No new domain errors are required. Existing `domain/memoryv2` errors are reused:
- `ErrNotFound` — when Ingatan returns 404
- `ErrAlreadyExists` — when Ingatan returns 409
- `ErrInvalidInput` — when mapping fails validation
- New: HTTP errors from Ingatan are wrapped with context: `fmt.Errorf("ingatan: %s: %w", operation, err)`
