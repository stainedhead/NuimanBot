# NuimanBot Product Requirements Document

> Security-hardened, Golang personal AI agent with multi-platform support

**Version:** 1.1
**Last Updated:** 2026-02-06
**Status:** Production-Ready MVP (75% Complete)
**Implementation Progress:** 33/44 planned features complete

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Goals and Non-Goals](#2-goals-and-non-goals)
3. [User Roles and Permissions](#3-user-roles-and-permissions)
4. [System Architecture](#4-system-architecture)
5. [Security Layer](#5-security-layer)
6. [MCP Integration](#6-mcp-integration)
7. [Messaging Gateways](#7-messaging-gateways)
8. [LLM Provider Abstraction](#8-llm-provider-abstraction)
9. [Skills System](#9-skills-system)
10. [Memory and Context](#10-memory-and-context)
11. [MVP Phases](#11-mvp-phases)
12. [Verification Strategy](#12-verification_strategy)

---

## 1. Executive Summary

NuimanBot is a security-hardened personal AI agent built in Go, designed as a secure alternative to existing AI agent frameworks. It addresses critical security vulnerabilities found in similar platforms while providing:

- **Multi-user server deployment** with role-based access control (RBAC)
- **Multi-platform support**: Telegram, Slack, CLI
- **Multi-provider LLM integration**: Anthropic Claude, OpenAI, Ollama (local)
- **MCP dual capability**: Functions as both Server and Client per the November 2025 specification
- **Custom skills only**: No external skill imports—maximum security posture

### Key Differentiator

Research shows that 26% of community skills in similar platforms contain security vulnerabilities including credential leakage, prompt injection enabling RCE, and supply chain attacks. NuimanBot eliminates these attack vectors through:

- Zero external skill imports
- AES-256-GCM encrypted credential storage
- Input sanitization and output sandboxing
- Comprehensive audit logging

---

## 2. Goals and Non-Goals

### Goals

| Goal | Metric |
|------|--------|
| Zero credential leakage | AES-256-GCM encryption at rest; no plaintext secrets |
| 100% skill security | Custom skills only; sandboxed execution |
| Multi-provider LLM support | Anthropic, OpenAI, Ollama all functional |
| MCP compliance | Full 2025-11-25 specification support |
| Test coverage | ≥80% overall; domain layer ≥90% |
| Clean Architecture | Strict layer dependencies per AGENTS.md |

### Non-Goals

| Non-Goal | Rationale |
|----------|-----------|
| External skill marketplace | Security risk—attack surface |
| WhatsApp/iMessage support | Phase 2+ consideration |
| Web UI admin panel | CLI-first approach |
| Self-updating agent | Security risk—supply chain |
| Persistent learning across users | Privacy isolation required |

---

## 3. User Roles and Permissions

### Role Definitions

```go
type Role string

const (
    RoleAdmin Role = "admin"
    RoleUser  Role = "user"
)
```

### Permission Matrix

| Capability | Admin | User |
|------------|:-----:|:----:|
| Manage own settings | ✓ | ✓ |
| Use allowed skills | ✓ | ✓ |
| Use all skills | ✓ | ✗ |
| Add/remove users | ✓ | ✗ |
| Configure LLM providers | ✓ | ✗ |
| Manage skills globally | ✓ | ✗ |
| Access MCP administration | ✓ | ✗ |
| View audit logs | ✓ | ✗ |
| Set user skill allowlists | ✓ | ✗ |

### User Entity

```go
type User struct {
    ID            string
    Username      string
    Role          Role
    PlatformIDs   map[Platform]string  // Telegram ID, Slack ID, etc.
    AllowedSkills []string             // Empty = all (admin only)
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

---

## 4. System Architecture

NuimanBot follows Clean Architecture principles with strict dependency rules. Dependencies flow inward only.

### Layer Structure

```
cmd/nuimanbot/main.go              # Entry point, dependency injection

internal/
├── domain/                        # Layer 1: Entities (innermost)
│   ├── user.go                    # User, Role entities
│   ├── message.go                 # Message, Conversation entities
│   ├── skill.go                   # Skill definitions
│   ├── permission.go              # Permission entities
│   └── errors.go                  # Domain errors
│
├── usecase/                       # Layer 2: Application logic
│   ├── auth/                      # Authentication use cases
│   │   ├── service.go
│   │   └── service_test.go
│   ├── chat/                      # Conversation orchestration
│   │   ├── service.go
│   │   └── service_test.go
│   ├── skill/                     # Skill execution
│   │   ├── service.go
│   │   └── service_test.go
│   └── mcp/                       # MCP session management
│       ├── service.go
│       └── service_test.go
│
├── adapter/                       # Layer 3: Interface adapters
│   ├── gateway/                   # Messaging platform adapters
│   │   ├── telegram/
│   │   │   ├── gateway.go
│   │   │   └── gateway_test.go
│   │   ├── slack/
│   │   │   ├── gateway.go
│   │   │   └── gateway_test.go
│   │   └── cli/
│   │       ├── gateway.go
│   │       └── gateway_test.go
│   └── repository/                # Data persistence adapters
│       ├── sqlite/
│       │   ├── user.go
│       │   ├── message.go
│       │   └── *_test.go
│       └── postgres/
│           ├── user.go
│           ├── message.go
│           └── *_test.go
│
└── infrastructure/                # Layer 4: External services (outermost)
    ├── llm/                       # LLM provider clients
    │   ├── anthropic/
    │   │   ├── client.go
    │   │   └── client_test.go
    │   ├── openai/
    │   │   ├── client.go
    │   │   └── client_test.go
    │   └── ollama/
    │       ├── client.go
    │       └── client_test.go
    ├── mcp/                       # MCP server/client
    │   ├── server.go
    │   ├── client.go
    │   └── *_test.go
    └── crypto/                    # Encryption services
        ├── vault.go
        ├── aes.go
        └── *_test.go
```

### Dependency Rules

1. **Domain Layer**: Pure business entities and interfaces. No external imports except stdlib.
2. **Use Case Layer**: Orchestrates domain entities. Defines repository/service interfaces.
3. **Adapter Layer**: Implements interfaces defined in use case layer. Converts external data to domain models.
4. **Infrastructure Layer**: Concrete implementations for external services, databases, APIs.

### Configuration Structure

All NuimanBot configuration will be loaded into a single, comprehensive `NuimanBotConfig` struct at application startup. This struct will serve as the central source of truth for all configurable aspects of the agent.

```go
// NuimanBotConfig encapsulates the entire application configuration.
type NuimanBotConfig struct {
    Server struct {
        LogLevel string
        Debug    bool
    }
    Security struct {
        InputMaxLength     int
        TokenRotationHours int
    }
    LLM      LLMConfig
    Gateways struct {
        Telegram TelegramConfig
        Slack    SlackConfig
        CLI      CLIConfig
    }
    MCP      MCPConfig
    Storage  struct {
        Type string
        Path string
    }
    Skills   SkillsSystemConfig
    Memory   MemoryConfig
    ExternalAPI struct {
        OpenAI struct {
            Enabled    bool
            Port       int
            APIKey     SecureString
            DefaultModel string
        }
        REST struct {
            Enabled    bool
            Port       int
            APIKey     SecureString
        }
    }
    Tools struct {
        WebSearch struct {
            APIKey    string
            MaxResults int
        }
        Exec struct {
            Timeout            int
            RestrictToWorkspace bool
        }
    }
}
```


---

## 5. Security Layer

### Threat Model

| Threat | Impact in Other Platforms | NuimanBot Mitigation |
|--------|---------------------------|----------------------|
| Credential leakage | Plaintext API keys exposed | AES-256-GCM encryption at rest |
| Prompt injection | RCE via crafted input | Input sanitization, output sandboxing |
| Malicious skills | Data exfiltration, backdoors | Custom skills only—no external imports |
| Session hijacking | Token leakage, impersonation | Token rotation, secure WebSocket |
| Privilege escalation | Unauthorized admin access | Strict RBAC enforcement |
| Supply chain attacks | Compromised dependencies | Minimal deps, audit logging |

### Security Service Interface

```go
type SecurityService interface {
    // Encrypt encrypts data for a specific user context
    Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error)

    // Decrypt decrypts user-specific data
    Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error)

    // ValidateInput sanitizes and validates user input
    ValidateInput(ctx context.Context, input string) (string, error)

    // Audit logs a security-relevant event
    Audit(ctx context.Context, event AuditEvent) error
}
```

### Credential Vault Interface

```go
type CredentialVault interface {
    // Store securely stores a credential
    Store(ctx context.Context, key string, value SecureString) error

    // Retrieve retrieves a credential
    Retrieve(ctx context.Context, key string) (SecureString, error)

    // Delete removes a credential
    Delete(ctx context.Context, key string) error

    // RotateKey rotates the master encryption key
    RotateKey(ctx context.Context) error

    // List returns all stored credential keys (not values)
    List(ctx context.Context) ([]string, error)
}

// SecureString wraps sensitive data with automatic zeroing
type SecureString struct {
    value []byte
}

func (s *SecureString) Value() string { return string(s.value) }
func (s *SecureString) Zero()         { /* zero out memory */ }
```

### Audit Event Structure

```go
type AuditEvent struct {
    Timestamp   time.Time
    UserID      string
    Action      string
    Resource    string
    Outcome     string  // "success", "failure", "denied"
    Details     map[string]any
    SourceIP    string
    Platform    Platform
}
```

### Input Validation Rules

- Maximum input length: 32KB
- No null bytes
- UTF-8 validation
- Prompt injection pattern detection
- Command injection pattern detection

---

## 6. MCP Integration

NuimanBot implements both MCP Server and Client modes per the [2025-11-25 specification](https://modelcontextprotocol.io/specification/2025-11-25).

### MCP Server Mode

Exposes NuimanBot skills as MCP tools for external consumption.

```go
type MCPServer interface {
    // Start starts the MCP server
    Start(ctx context.Context) error

    // Stop gracefully stops the server
    Stop(ctx context.Context) error

    // RegisterSkill registers a skill as an MCP tool
    RegisterSkill(skill Skill) error

    // UnregisterSkill removes a skill
    UnregisterSkill(name string) error
}
```

**Tool Registration:**

```go
func (s *mcpServer) registerSkillAsTool(skill Skill) error {
    tool := &mcp.Tool{
        Name:        skill.Name(),
        Description: skill.Description(),
        InputSchema: skill.InputSchema(),
    }

    handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // Validate caller permissions
        if err := s.authorize(ctx, skill); err != nil {
            return nil, err
        }

        // Execute skill
        result, err := skill.Execute(ctx, req.Params)
        if err != nil {
            return nil, err
        }

        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: result.Output}},
        }, nil
    }

    return mcp.AddTool(s.server, tool, handler)
}
```

### MCP Client Mode

Connects to external MCP servers to consume their tools.

```go
type MCPClient interface {
    // Connect establishes connection to an MCP server
    Connect(ctx context.Context, serverURL string) error

    // Disconnect closes the connection
    Disconnect(ctx context.Context) error

    // ListTools returns available tools from connected server
    ListTools(ctx context.Context) ([]ToolInfo, error)

    // CallTool invokes a tool on the connected server
    CallTool(ctx context.Context, name string, params map[string]any) (*ToolResult, error)
}
```

**Security Controls for MCP Client:**

- Allowlist-only server connections (admin-configured)
- Per-tool authorization checks before invocation
- Audit logging for all MCP calls
- Timeout enforcement (30s default)
- Response size limits (1MB default)

### MCP Configuration

```go
type MCPConfig struct {
    // Server configuration
    Server struct {
        Enabled bool
        Port    int
        TLS     bool
    }

    // Client configuration
    Client struct {
        AllowedServers []string
        Timeout        time.Duration
        MaxRetries     int
    }
}
```

---

## 7. Messaging Gateways

### Gateway Interface

```go
type Gateway interface {
    // Platform returns the platform identifier
    Platform() Platform

    // Start begins listening for messages
    Start(ctx context.Context) error

    // Stop gracefully shuts down the gateway
    Stop(ctx context.Context) error

    // Send sends a message to a user
    Send(ctx context.Context, msg OutgoingMessage) error

    // OnMessage registers a handler for incoming messages
    OnMessage(handler MessageHandler)
}

type Platform string

const (
    PlatformTelegram Platform = "telegram"
    PlatformSlack    Platform = "slack"
    PlatformCLI      Platform = "cli"
)

type MessageHandler func(ctx context.Context, msg IncomingMessage) error

type IncomingMessage struct {
    ID          string
    Platform    Platform
    PlatformUID string    // Platform-specific user ID
    Text        string
    Timestamp   time.Time
    Metadata    map[string]any
}

type OutgoingMessage struct {
    RecipientID string
    Text        string
    Format      string  // "text", "markdown"
    Metadata    map[string]any
}
```

### Telegram Gateway

**Library:** `github.com/go-telegram/bot`

**Features:**
- User whitelist by Telegram ID
- Webhook and long-polling support
- Markdown message formatting
- Rate limiting (30 msg/sec global)

**Configuration:**

```go
type DMPolicy string
const (
    DMPolicyPairing   DMPolicy = "pairing"   // Default: only allow if previously paired or admin approves
    DMPolicyAllowlist DMPolicy = "allowlist" // Only allow if sender is in AllowedIDs
    DMPolicyOpen      DMPolicy = "open"      // Allow all direct messages
)

type TelegramConfig struct {
    Token       SecureString
    WebhookURL  string         // Empty = use long polling
    AllowedIDs  []int64        // Empty = deny all (admin must configure)
    DMPolicy    DMPolicy       // "pairing", "allowlist", or "open"
}
```

### Slack Gateway

**Library:** `github.com/slack-go/slack`

**Features:**
- Socket Mode (no public endpoint required)
- Workspace allowlist
- Thread support
- Slash command integration

**Configuration:**

```go
type SlackConfig struct {
    BotToken    SecureString
    AppToken    SecureString   // For Socket Mode
    WorkspaceID string
}
```

### CLI Gateway

**Features:**
- Interactive REPL for local testing
- Command history
- Debug mode with verbose output
- Useful for development and admin tasks

**Configuration:**

```go
type CLIConfig struct {
    HistoryFile string
    DebugMode   bool
}
```

---

## 8. LLM Provider Abstraction

### LLM Service Interface

```go
type LLMService interface {
    // Complete performs a completion request
    Complete(ctx context.Context, provider LLMProvider, req LLMRequest) (*LLMResponse, error)

    // Stream performs a streaming completion
    Stream(ctx context.Context, provider LLMProvider, req LLMRequest) (<-chan StreamChunk, error)

    // ListModels returns available models for a provider
    ListModels(ctx context.Context, provider LLMProvider) ([]ModelInfo, error)
}

type LLMProvider string

const (
    ProviderAnthropic LLMProvider = "anthropic"
    ProviderOpenAI    LLMProvider = "openai"
    ProviderOllama    LLMProvider = "ollama"
)

type LLMRequest struct {
    Model       string
    Messages    []Message
    MaxTokens   int
    Temperature float64
    Tools       []ToolDefinition  // For function calling
    SystemPrompt string
}

type LLMResponse struct {
    Content     string
    ToolCalls   []ToolCall
    Usage       TokenUsage
    FinishReason string
}

type StreamChunk struct {
    Delta       string
    ToolCall    *ToolCall
    Done        bool
    Error       error
}
```

### Provider Implementations

| Provider | Library | Features |
|----------|---------|----------|
| Anthropic | `github.com/anthropics/anthropic-sdk-go` | Claude models, tool use, vision |
| OpenAI | `github.com/openai/openai-go` | GPT models, function calling, vision |
| Ollama | HTTP API (stdlib `net/http`) | Local models, no API key required |
| Bedrock | `github.com/aws/aws-sdk-go` | AWS-managed models, secure integration |

### Provider Configuration

```go
type LLMProviderType string
const (
    LLMProviderTypeAnthropic LLMProviderType = "anthropic"
    LLMProviderTypeOpenAI    LLMProviderType = "openai"
    LLMProviderTypeOllama    LLMProviderType = "ollama"
    LLMProviderTypeBedrock   LLMProviderType = "bedrock" // New entry
    // Add other providers as needed
)

type LLMProviderConfig struct {
    ID      string          // Unique identifier for the provider configuration. This is a custom ID (e.g., "openai-byok-azure") and not necessarily the canonical provider name.
    Type    LLMProviderType // Type of the LLM provider (e.g., "anthropic", "openai")
    APIKey  SecureString    // API key for the provider
    BaseURL string          // Base URL for the provider API (optional)
    Name    string          // User-friendly name for the provider
}

type LLMConfig struct {
    // Default model and an ordered list of fallback models (provider/model reference)
    DefaultModel struct {
        Primary   string   // e.g., "anthropic/claude-sonnet-4-20250514"
        Fallbacks []string // e.g., ["openai/gpt-4o", "ollama/llama3.2"]
    }

    // Per-model configuration, including aliases and provider-specific parameters
    // Alias values must be globally unique.
    Models map[string]struct { // Key: provider/model reference (e.g., "openai/gpt-4o")
        Alias            string                 // Optional shorter alias (e.g., "gpt4"). Must be globally unique.
        ProviderConfigID string                 // ID from LLMConfig.Providers to use for this model's API key/base URL.
        Params           map[string]interface{} // Provider-specific parameters (e.g., "temperature": 0.7)
    }

    // Providers is a list of LLM provider configurations, allowing BYOK scenarios.
    Providers []LLMProviderConfig

    // Individual provider configurations (legacy, or for specific settings not covered by Providers array)
    Anthropic struct {
        APIKey SecureString
    }

    OpenAI struct {
        APIKey  SecureString
        BaseURL string // For Azure/proxies
    }

    Ollama struct {
        BaseURL string // e.g., "http://localhost:11434"
    }

    Bedrock struct { // New entry for AWS Bedrock configuration
        AWSRegion  string // AWS region, e.g., "us-east-1"
        AWSProfile string // AWS profile name (optional, uses default if empty)
    }
}
```

---

## 9. Skills System

### Skill Interface

```go
type SkillConfig struct {
    Enabled bool                   // Enable/disable the skill
    APIKey  SecureString           // API key for the skill (managed via vault/env)
    Env     map[string]string      // Environment variables for the skill
    Params  map[string]interface{} // Custom parameters for the skill
}

// SkillsSystemConfig defines global settings for the skill system.
type SkillsSystemConfig struct {
    Entries map[string]SkillConfig // Individual skill configurations by ID
    Load    struct {
        ExtraDirs []string // Additional directories to scan for skills
        Watch     bool     // Watch skill folders for changes and refresh the skills snapshot
    }
}

type Skill interface {
    // Name returns the unique skill identifier
    Name() string

    // Description returns a human-readable description
    Description() string

    // InputSchema returns the JSON schema for parameters
    InputSchema() map[string]any

    // Execute runs the skill with given parameters. Skill-specific configuration
    // and other runtime context can be retrieved from the `ctx`.
    Execute(ctx context.Context, params map[string]any) (*SkillResult, error)

    // RequiredPermissions returns permissions needed to use this skill
    RequiredPermissions() []Permission

    // Config returns the skill's specific configuration
    Config() SkillConfig
}

type SkillResult struct {
    Output   string
    Metadata map[string]any
    Error    string  // Empty if successful
}

type Permission string

const (
    PermissionRead      Permission = "read"       // Read data
    PermissionWrite     Permission = "write"      // Write data
    PermissionNetwork   Permission = "network"    // Make network requests
    PermissionShell     Permission = "shell"      // Execute shell commands (admin only)
)

#### Dynamic Skill Loading

NuimanBot will leverage Go's plugin system (`plugin` package) to dynamically load skills from compiled shared objects (`.so` files). This allows for extending `NuimanBot`'s capabilities without recompiling the main application. Each loaded plugin must export a symbol that returns an instance of the `Skill` interface, ensuring type safety.

```

### Security Controls

| Control | Implementation |
|---------|----------------|
| No external imports | Skills defined in `internal/` only |
| No shell by default | `shell` permission requires admin role |
| Allowlisted commands | Shell skills can only run pre-approved commands |
| Capability-based | Each skill declares required permissions |
| Rate limiting | Per-user, per-skill limits |
| Timeout enforcement | Default 30s, configurable per skill |
| Output sanitization | Prevent prompt injection in results |
| Workspace restriction | Shell skill commands are restricted to the agent's workspace |

### Built-in Skills (MVP)

| Skill | Description | Permissions |
|-------|-------------|-------------|
| `calculator` | Evaluate mathematical expressions | none |
| `datetime` | Get current date/time, timezone conversion | none |
| `weather` | Get weather for a location | network |
| `web_search` | Search the web | network |
| `reminder` | Set reminders | write |
| `notes` | Create/read/update notes | read, write |

### Skill Registry

```go
type SkillRegistry interface {
    // Register adds a skill to the registry
    Register(skill Skill) error

    // Get retrieves a skill by name
    Get(name string) (Skill, error)

    // List returns all registered skills
    List() []Skill

    // ListForUser returns skills available to a specific user
    ListForUser(ctx context.Context, userID string) ([]Skill, error)
}
```

---

## 10. Memory and Context

### Memory Repository Interface

```go
// MemoryConfig defines the configuration for the agent's long-term memory.
type MemoryConfig struct {
    Backend   MemoryBackend       // Which memory backend to use ("builtin", "qmd")
    Citations MemoryCitationsMode // How citations are handled ("auto", "on", "off")
    QMD       MemoryQMDConfig     // Configuration for the Queryable Memory Document (QMD) backend
}

type MemoryBackend string
const (
    MemoryBackendBuiltin MemoryBackend = "builtin"
    MemoryBackendQMD     MemoryBackend = "qmd"
)

type MemoryCitationsMode string
const (
    MemoryCitationsModeAuto MemoryCitationsMode = "auto"
    MemoryCitationsModeOn   MemoryCitationsMode = "on"
    MemoryCitationsModeOff  MemoryCitationsMode = "off"
)

// MemoryQMDConfig defines configuration for the Queryable Memory Document (QMD) backend.
type MemoryQMDConfig struct {
    Command            string             // Command to execute for QMD (e.g., embedding model). This command will be executed via NuimanBot's internal LLMService, which will be extended to support dedicated embedding calls.
    IncludeDefaultMemory bool             // Include default memory (e.g., BOOTSTRAP.md)
    Paths              []MemoryQMDIndexPath // Paths to directories/files for QMD
    Sessions           struct {
        Enabled      bool // Enable session-specific memory export
        ExportDir    string
        RetentionDays int
    }
    Update struct {
        Interval  string // e.g., "1h", "30m"
        DebounceMs int
        OnBoot     bool
    }
    Limits struct {
        MaxResults      int
        MaxSnippetChars int
        MaxInjectedChars int
        TimeoutMs       int
    }
}

// MemoryQMDIndexPath defines a path to a memory document or directory.
type MemoryQMDIndexPath struct {
    Path    string // File path or directory
    Name    string // Optional name for the document
    Pattern string // Optional glob pattern for files in a directory
}

type MemoryRepository interface {
    // SaveMessage persists a message in a conversation
    SaveMessage(ctx context.Context, convID string, msg StoredMessage) error

    // GetConversation retrieves a full conversation
    GetConversation(ctx context.Context, convID string) (*Conversation, error)

    // GetRecentMessages retrieves messages up to a token limit
    GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]StoredMessage, error)

    // DeleteConversation removes a conversation
    DeleteConversation(ctx context.Context, convID string) error

    // ListConversations returns conversations for a user
    ListConversations(ctx context.Context, userID string) ([]ConversationSummary, error)
}

type StoredMessage struct {
    ID           string
    Role         string  // "user", "assistant", "system"
    Content      string
    ToolCalls    []ToolCall
    ToolResults  []ToolResult
    TokenCount   int
    Timestamp    time.Time
}

type Conversation struct {
    ID        string
    UserID    string
    Platform  Platform
    Messages  []StoredMessage
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Storage Backends

| Backend | Use Case |
|---------|----------|
| SQLite | Development, single-server deployment |
| PostgreSQL | Production, multi-server deployment |

### Context Window Management

- Automatic token counting per message
- Sliding window with summarization for long conversations
- Priority retention for system prompts and recent messages
- Per-provider token limit awareness

### Queryable Memory Documents (QMD)

Inspired by OpenClaw, NuimanBot can integrate a system for Queryable Memory Documents (QMD). QMD allows the agent to index and query external documents or structured data as part of its long-term memory. This enhances the agent's ability to retrieve relevant information beyond the current conversation context, effectively expanding its "knowledge base" for specific work items.

Configurable aspects include:
- Paths to directories containing memory documents (e.g., Markdown, text files). These paths can include placeholders like `{userID}` or `{agentID}` which will be dynamically resolved at runtime based on the current context.
- Patterns for filtering document types.
- Update intervals for re-indexing memory.
- Session-specific memory export.
- Limits on results and snippet sizes during queries.


---

## 11. MVP Phases

### Phase 1: Foundation

| Task | Description | Status |
|------|-------------|--------|
| Project setup | go.mod, directory structure, CI | ☐ |
| Domain entities | User, Message, Permission, Skill | ☐ |
| Security core | AES-256-GCM encryption, input validation, audit | ☐ |
| CLI gateway | Interactive REPL | ☐ |
| Anthropic provider | Claude API integration | ☐ |
| Basic skills | calculator, datetime | ☐ |
| SQLite storage | User and message persistence | ☐ |
| Quality gates | Linting, testing, coverage | ☐ |

### Phase 2: Multi-Platform

| Task | Description | Status |
|------|-------------|--------|
| Telegram gateway | go-telegram/bot integration | ☐ |
| Slack gateway | slack-go/slack Socket Mode | ☐ |
| OpenAI provider | GPT models integration | ☐ |
| Ollama provider | Local model support | ☐ |
| RBAC enforcement | Permission checks throughout | ☐ |
| User management | Admin commands for user CRUD | ☐ |
| Additional skills | weather, web_search, notes | ☐ |

### Phase 3: MCP Integration

| Task | Description | Status |
|------|-------------|--------|
| MCP Server | Expose skills as MCP tools | ☐ |
| MCP Client | Consume external MCP servers | ☐ |
| Tool authorization | Per-tool permission checks | ☐ |
| MCP admin commands | Server management CLI | ☐ |
| Integration testing | End-to-end MCP flows | ☐ |

### Phase 4: Production Hardening

| Task | Description | Status |
|------|-------------|--------|
| PostgreSQL support | Production database backend | ☐ |
| Monitoring/metrics | Prometheus/OpenTelemetry | ☐ |
| Security audit | Third-party review | ☐ |
| Performance optimization | Profiling, bottleneck removal | ☐ |
| Documentation | User guide, API docs | ☐ |
| Deployment automation | Docker, Kubernetes manifests | ☐ |

---

## 12. Verification Strategy

---

## 13. External API Interfaces

To enable flexible integration with external clients and management applications, NuimanBot will expose several RESTful API interfaces. These interfaces adhere to industry standards where applicable (e.g., OpenAI API compatibility) and provide robust authentication and authorization mechanisms.

### OpenAI-Compatible Chat Completions API

NuimanBot will expose an OpenAI-compatible API endpoint, allowing clients to interact with the agent as if it were an OpenAI service. This is particularly useful for integrating with existing OpenAI-compatible libraries and tools, as well as for CLI management applications to make LLM calls.

**Endpoint:** `/v1/chat/completions`

**Description:** Performs a chat completion request with the NuimanBot agent. The agent will process the request through its internal LLM pipeline, including memory retrieval, tool execution, and response generation, before formatting the result into an OpenAI-compatible response.

**Key Features:**
-   **Authentication:** Requires API Key authentication (e.g., `Authorization: Bearer <API_KEY>`). A single global API key (`external_api.openai.api_key`) is used for all clients.
-   **Conversation Management:** Supports continuation of conversations via `conversation_id` in the request metadata.
-   **Tool Calling:** Supports OpenAI-style tool calls within the chat completions flow.
-   **Model Mapping:** The client's provided model name must exactly match an internal `provider/model` string (e.g., `openai/gpt-4o`) or a globally unique alias defined in `LLMConfig.Models`. If no model is specified, `external_api.openai.default_model` is used.
-   **Streaming:** Supports server-sent events (SSE) for streaming responses.

**Configuration:**

```yaml
external_api:
  openai:
    enabled: true
    port: 8081 # Port for the OpenAI-compatible API server
    api_key: <secure-api-key> # API Key for clients to authenticate with NuimanBot
    default_model: anthropic/claude-sonnet-4-20250514 # Default model if client doesn't specify
```

---

### CLI Management REST API

NuimanBot will expose a dedicated REST API for programmatic management via CLI and other management applications. These endpoints will allow querying status, managing configurations, controlling skills, and overseeing sessions.

**Base Path:** `/api/v1`

**Authentication:** Requires API Key authentication (e.g., `Authorization: Bearer <API_KEY>`). Authorization for sensitive operations (e.g., skill management, config changes) will further rely on `NuimanBot`'s internal `User Roles and Permissions` (Section 3). The `UserID` associated with the API call (e.g., via a dedicated header) will determine access rights.

**Key Endpoints:**

#### Agent Management

-   **`GET /api/v1/agent/status`**
    -   **Description:** Retrieves the current operational status of the NuimanBot agent.
    -   **Response:** Agent ID, name, active channels, uptime, health indicators.
-   **`POST /api/v1/agent/reset`**
    -   **Description:** Resets the agent's current primary session and memory.
    -   **Request Body:** `{"session_id": "string?"}` (Optional: specific session to reset)
-   **`POST /api/v1/agent/onboard`**
    -   **Description:** Triggers or continues the agent's onboarding process.

#### Skill Management

-   **`GET /api/v1/skills`**
    -   **Description:** Lists all configured skills, including their enabled status, descriptions, and required permissions.
-   **`GET /api/v1/skills/{skill_name}`**
    -   **Description:** Retrieves detailed information about a specific skill, including its `InputSchema` and current configuration.
-   **`POST /api/v1/skills/{skill_name}/enable`**
    -   **Description:** Enables a specific skill.
-   **`POST /api/v1/skills/{skill_name}/disable`**
    -   **Description:** Disables a specific skill.

#### Configuration Management

-   **`GET /api/v1/config`**
    -   **Description:** Retrieves the current effective configuration of NuimanBot (sensitive information redacted).
-   **`GET /api/v1/config/{path}`**
    -   **Description:** Retrieves a specific configuration value by its dot-separated path (e.g., `llm.default_model.primary`).
-   **`PATCH /api/v1/config`**
    -   **Description:** Applies partial updates to the NuimanBot configuration.
    -   **Request Body:** JSON object representing the configuration patch.
-   **`POST /api/v1/config/reload`**
    -   **Description:** Triggers a graceful reload of the agent's configuration from its source files.

#### Session Management

-   **`GET /api/v1/sessions`**
    -   **Description:** Lists all active or recently active conversational sessions.
-   **`GET /api/v1/sessions/{session_id}`**
    -   **Description:** Retrieves detailed information about a specific conversational session (e.g., last activity, current model, token usage).
-   **`POST /api/v1/sessions/{session_id}/reset`**
    -   **Description:** Resets a specific conversational session, clearing its memory and context.

#### Job Management

-   **`POST /api/v1/jobs/dispatch`**
    -   **Description:** Dispatches an internal job (e.g., a background task, cron job trigger, or webhook event) to NuimanBot's job queue.
    -   **Request Body:** `{"type": "string", "context": "object", ...}` (Matches internal `JobRequest` structure)

**Configuration:**

```yaml
external_api:
  rest:
    enabled: true
    port: 8082
    api_key: <secure-rest-api-key>
```


### Quality Gates

All gates must pass before completing any development task:

```bash
# 1. Format code
go fmt ./...

# 2. Tidy dependencies
go mod tidy

# 3. Vet for suspicious constructs
go vet ./...

# 4. Run linter
golangci-lint run

# 5. Run all tests
go test ./...

# 6. Build executable
go build -o bin/nuimanbot ./cmd/nuimanbot

# 7. Verify it runs
./bin/nuimanbot --help
```

### Quick Validation

```bash
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

### Test Coverage Requirements

| Layer | Minimum Coverage |
|-------|------------------|
| `internal/domain/` | 90% |
| `internal/usecase/` | 85% |
| `internal/adapter/` | 80% |
| `internal/infrastructure/` | 75% |
| **Overall** | **80%** |

### Testing Strategy

- **Unit tests**: All business logic in domain and usecase layers
- **Integration tests**: Repository implementations, API clients
- **E2E tests**: Full message flow from gateway through LLM and back
- **Security tests**: Input validation, encryption, access control

---

## Appendix: Configuration Reference

### Environment Variables

```bash
# Required
NUIMANBOT_ENCRYPTION_KEY=<base64-encoded-32-byte-key>

# Anthropic (if enabled)
ANTHROPIC_API_KEY=<api-key>

# OpenAI (if enabled)
OPENAI_API_KEY=<api-key>

# Telegram (if enabled)
TELEGRAM_BOT_TOKEN=<bot-token>

# Slack (if enabled)
SLACK_BOT_TOKEN=<bot-token>
SLACK_APP_TOKEN=<app-token>

# Database
DATABASE_URL=sqlite://data/nuimanbot.db
# or
DATABASE_URL=postgres://user:pass@host:5432/nuimanbot

# MCP
MCP_SERVER_ENABLED=true
MCP_SERVER_PORT=8080
```

### Configuration File (config.yaml)

```yaml
server:
  log_level: info
  debug: false

security:
  input_max_length: 32768
  token_rotation_hours: 24

llm:
  # Default model and an ordered list of fallback models (provider/model reference)
  default_model:
    primary: anthropic/claude-sonnet-4-20250514
    fallbacks:
      - openai/gpt-4o
      - ollama/llama3.2
  # Per-model configuration, including aliases and provider-specific parameters
  models:
    anthropic/claude-sonnet-4-20250514:
      alias: sonnet
      params: # Model-specific parameters (e.g., max_tokens, temperature)
        max_tokens: 4096
    openai/gpt-4o:
      alias: gpt4
      params:
        temperature: 0.7
        max_tokens: 4096
    ollama/llama3.2:
      params:
        max_tokens: 4096
  # Providers is a list of LLM provider configurations, allowing BYOK scenarios.
  providers:
    - id: anthropic-byok # Unique ID for this provider configuration
      type: anthropic
      name: lc-anthropic # User-friendly name
      api_key: sk-ant-YOUR-ANTHROPIC-KEY # BYOK API Key
    - id: openai-byok # Unique ID for this provider configuration
      type: openai
      name: lc-openai # User-friendly name
      api_key: sk-YOUR-OPENAI-KEY # BYOK API Key
      base_url: https://api.openai.com/v1 # Optional base URL
    - id: ollama-local
      type: ollama
      name: Local Ollama
      base_url: http://localhost:11434
      # APIKey is not needed for local Ollama setup
skills: # Skill system configuration
  load:
    extra_dirs:
      - ./internal/skills/custom # Additional directories to scan for custom skills
    watch: true # Watch skill folders for changes
  entries: # Individual skill configurations by ID
    calculator:
      enabled: true
    web_search:
      enabled: true
      api_key: <brave-search-api-key> # API key for the web_search skill
      params:
        safesearch: moderate # Custom parameter for web_search
    custom_skill_name:
      enabled: false # Example of disabling a custom skill
      env:
        CUSTOM_ENV_VAR: "value" # Environment variables for the skill

tools:
  web_search:
    api_key: <brave-search-api-key> # Brave Search API key for web_search skill
    max_results: 5
  exec: # Shell execution tool configuration
    timeout: 60 # Default timeout for shell commands in seconds
    restrict_to_workspace: true # If true, block commands accessing paths outside workspace

gateways:
  telegram:
    enabled: true
    allowed_ids: []  # Admin must populate
    dm_policy: pairing # 'pairing', 'allowlist', or 'open'. Default: pairing
  slack:
    enabled: false
  cli:
    enabled: true
    debug_mode: false

mcp:
  server:
    enabled: true
    port: 8080
    tls: true
  client:
    allowed_servers: []
    timeout: 30s

external_api:
  openai:
    enabled: true
    port: 8081
    api_key: <secure-openai-api-key> # API Key for clients to authenticate with NuimanBot
    default_model: anthropic/claude-sonnet-4-20250514
  rest:
    enabled: true
    port: 8082
    api_key: <secure-rest-api-key> # API Key for clients to authenticate with NuimanBot

storage:
  type: sqlite  # or postgres
  path: data/nuimanbot.db

memory:
  backend: qmd # Use Queryable Memory Documents backend
  citations: auto # Automatically include citations in responses
  qmd: # QMD-specific configuration
    command: "ollama/llama3.2" # Embedding model to use for QMD (e.g., ollama/llama3.2)
    include_default_memory: true # Include BOOTSTRAP.md in QMD
    paths: # Paths to memory documents or directories
      - path: ./docs/memory
        pattern: "*.md"
      - path: ./users/shared_notes.txt
        name: "Shared Notes"
    sessions: # Session-specific memory export
      enabled: true
      export_dir: ./data/session_memory
      retention_days: 30
    update: # QMD update configuration
      interval: "24h" # Re-index memory every 24 hours
      debounce_ms: 1000
      on_boot: true # Re-index on bot boot
    limits: # QMD query limits
      max_results: 5
      max_snippet_chars: 200
      max_injected_chars: 1000
      timeout_ms: 5000