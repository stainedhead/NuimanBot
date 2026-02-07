# NuimanBot Initial MVP Data Dictionary

This document defines the key data structures, types, and schemas used within the NuimanBot system, primarily extracted from the `PRODUCT_REQUIREMENT_DOC.md` (`specs/initial-mvp-spec/spec.md`). This serves as a central reference for all sub-agents to ensure consistent data representation and interface design.

## 1. User Management

### `Role` (string enum)

```go
type Role string

const (
    RoleAdmin Role = "admin"
    RoleUser  Role = "user"
)
```

### `User` (struct)

Represents a user of the NuimanBot system.

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

## 2. Messaging Gateways

### `Platform` (string enum)

```go
type Platform string

const (
    PlatformTelegram Platform = "telegram"
    PlatformSlack    Platform = "slack"
    PlatformCLI      Platform = "cli"
)
```

### `Gateway` (interface)

Defines the contract for interacting with messaging platforms.

```go
type Gateway interface {
    Platform() Platform
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Send(ctx context.Context, msg OutgoingMessage) error
    OnMessage(handler MessageHandler)
}
```

### `MessageHandler` (function signature)

```go
type MessageHandler func(ctx context.Context, msg IncomingMessage) error
```

### `IncomingMessage` (struct)

Represents a message received from a platform.

```go
type IncomingMessage struct {
    ID          string
    Platform    Platform
    PlatformUID string    // Platform-specific user ID
    Text        string
    Timestamp   time.Time
    Metadata    map[string]any
}
```

### `OutgoingMessage` (struct)

Represents a message to be sent to a platform.

```go
type OutgoingMessage struct {
    RecipientID string
    Text        string
    Format      string  // "text", "markdown"
    Metadata    map[string]any
}
```

## 3. LLM Provider Abstraction

### `LLMProvider` (string enum)

```go
type LLMProvider string

const (
    ProviderAnthropic LLMProvider = "anthropic"
    ProviderOpenAI    LLMProvider = "openai"
    ProviderOllama    LLMProvider = "ollama"
)
```

### `LLMService` (interface)

Defines the contract for interacting with LLM providers.

```go
type LLMService interface {
    Complete(ctx context.Context, provider LLMProvider, req LLMRequest) (*LLMResponse, error)
    Stream(ctx context.Context, provider LLMProvider, req LLMRequest) (<-chan StreamChunk, error)
    ListModels(ctx context.Context, provider LLMProvider) ([]ModelInfo, error) // ModelInfo is implicit
}
```

### `LLMRequest` (struct)

Represents a request to an LLM.

```go
type LLMRequest struct {
    Model       string
    Messages    []Message // Message is an implicit struct (likely from domain/message.go or similar)
    MaxTokens   int
    Temperature float64
    Tools       []ToolDefinition  // For function calling
    SystemPrompt string
}
```

### `LLMResponse` (struct)

Represents a response from an LLM.

```go
type LLMResponse struct {
    Content     string
    ToolCalls   []ToolCall
    Usage       TokenUsage // TokenUsage is an implicit struct
    FinishReason string
}
```

### `StreamChunk` (struct)

Represents a chunk of a streaming LLM response.

```go
type StreamChunk struct {
    Delta       string
    ToolCall    *ToolCall
    Done        bool
    Error       error
}
```

### Implicit Types

*   **`Message`**: Used within `LLMRequest`, likely a struct defining `Role` and `Content`.
*   **`ToolDefinition`**: Defines the schema for tools an LLM can use.
*   **`ToolCall`**: Represents an LLM's call to a tool.
*   **`TokenUsage`**: Contains information about token usage in an LLM interaction.
*   **`ModelInfo`**: Provides details about an available LLM model.

## 4. Skills System

### `Permission` (string enum)

```go
type Permission string

const (
    PermissionRead      Permission = "read"
    PermissionWrite     Permission = "write"
    PermissionNetwork   Permission = "network"
    PermissionShell     Permission = "shell"
)
```

### `Skill` (interface)

Defines the contract for a NuimanBot skill.

```go
type Skill interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, params map[string]any) (*SkillResult, error)
    RequiredPermissions() []Permission
    Config() SkillConfig
}
```

### `SkillConfig` (struct)

Configuration for an individual skill.

```go
type SkillConfig struct {
    Enabled bool
    APIKey  SecureString
    Env     map[string]string
    Params  map[string]interface{}
}
```

### `SkillResult` (struct)

Result of a skill execution.

```go
type SkillResult struct {
    Output   string
    Metadata map[string]any
    Error    string
}
```

### `SkillRegistry` (interface)

Manages skill registration and retrieval.

```go
type SkillRegistry interface {
    Register(skill Skill) error
    Get(name string) (Skill, error)
    List() []Skill
    ListForUser(ctx context.Context, userID string) ([]Skill, error)
}
```

## 5. Security Layer

### `SecurityService` (interface)

Defines core security operations.

```go
type SecurityService interface {
    Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error)
    Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error)
    ValidateInput(ctx context.Context, input string) (string, error)
    Audit(ctx context.Context, event AuditEvent) error
}
```

### `CredentialVault` (interface)

Defines secure credential storage operations.

```go
type CredentialVault interface {
    Store(ctx context.Context, key string, value SecureString) error
    Retrieve(ctx context.Context, key string) (SecureString, error)
    Delete(ctx context.Context, key string) error
    RotateKey(ctx context.Context) error
    List(ctx context.Context) ([]string, error)
}
```

### `SecureString` (struct)

Wrapper for sensitive string data.

```go
type SecureString struct {
    value []byte
}

func (s *SecureString) Value() string { return string(s.value) }
func (s *SecureString) Zero()         { /* zero out memory */ }
```

### `AuditEvent` (struct)

Structure for security audit logs.

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

## 6. Memory and Context

### `MemoryRepository` (interface)

Defines the contract for conversation memory persistence.

```go
type MemoryRepository interface {
    SaveMessage(ctx context.Context, convID string, msg StoredMessage) error
    GetConversation(ctx context.Context, convID string) (*Conversation, error)
    GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]StoredMessage, error)
    DeleteConversation(ctx context.Context, convID string) error
    ListConversations(ctx context.Context, userID string) ([]ConversationSummary, error) // ConversationSummary is implicit
}
```

### `MemoryBackend` (string enum)

```go
type MemoryBackend string
const (
    MemoryBackendBuiltin MemoryBackend = "builtin"
    MemoryBackendQMD     MemoryBackend = "qmd"
)
```

### `MemoryCitationsMode` (string enum)

```go
type MemoryCitationsMode string
const (
    MemoryCitationsModeAuto MemoryCitationsMode = "auto"
    MemoryCitationsModeOn   MemoryCitationsMode = "on"
    MemoryCitationsModeOff  MemoryCitationsMode = "off"
)
```

### `MemoryQMDIndexPath` (struct)

Defines a path to a memory document or directory for QMD.

```go
type MemoryQMDIndexPath struct {
    Path    string // File path or directory
    Name    string // Optional name for the document
    Pattern string // Optional glob pattern for files in a directory
}
```

### `StoredMessage` (struct)

Represents a message stored in memory.

```go
type StoredMessage struct {
    ID           string
    Role         string
    Content      string
    ToolCalls    []ToolCall
    ToolResults  []ToolResult // ToolResult is an implicit struct
    TokenCount   int
    Timestamp    time.Time
}
```

### `Conversation` (struct)

Represents a conversation in memory.

```go
type Conversation struct {
    ID        string
    UserID    string
    Platform  Platform
    Messages  []StoredMessage
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Implicit Types

*   **`ConversationSummary`**: Used in `MemoryRepository.ListConversations`.
*   **`ToolResult`**: Used in `StoredMessage`.

## 7. MCP Integration

### `MCPServer` (interface)

Defines the contract for the MCP Server.

```go
type MCPServer interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    RegisterSkill(skill Skill) error
    UnregisterSkill(name string) error
}
```

### `MCPClient` (interface)

Defines the contract for the MCP Client.

```go
type MCPClient interface {
    Connect(ctx context.Context, serverURL string) error
    Disconnect(ctx context.Context) error
    ListTools(ctx context.Context) ([]ToolInfo, error) // ToolInfo is implicit
    CallTool(ctx context.Context, name string, params map[string]any) (*ToolResult, error) // ToolResult is implicit
}
```

### Implicit Types

*   **`ToolInfo`**: Information about an MCP tool.

## 8. Global Configuration Structures (`NuimanBotConfig` and Sub-Configs)

### `NuimanBotConfig` (top-level struct)

Encapsulates the entire application configuration.

```go
type NuimanBotConfig struct {
    Server struct {
        LogLevel string
        Debug    bool
    }
    Security struct {
        InputMaxLength     int
        TokenRotationHours int
    }
    LLM      LLMConfig           // Defined below
    Gateways struct {
        Telegram TelegramConfig    // Defined below
        Slack    SlackConfig       // Defined below
        CLI      CLIConfig         // Defined below
    }
    MCP      MCPConfig           // Defined below
    Storage  struct {
        Type string
        Path string
    }
    Skills   SkillsSystemConfig  // Defined below
    Memory   MemoryConfig        // Defined below
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

### `LLMProviderType` (string enum)

```go
type LLMProviderType string
const (
    LLMProviderTypeAnthropic LLMProviderType = "anthropic"
    LLMProviderTypeOpenAI    LLMProviderType = "openai"
    LLMProviderTypeOllama    LLMProviderType = "ollama"
    LLMProviderTypeBedrock   LLMProviderType = "bedrock"
)
```

### `LLMProviderConfig` (struct)

Configuration for a specific LLM provider instance.

```go
type LLMProviderConfig struct {
    ID      string
    Type    LLMProviderType
    APIKey  SecureString
    BaseURL string
    Name    string
}
```

### `LLMConfig` (struct)

Overall LLM system configuration.

```go
type LLMConfig struct {
    DefaultModel struct {
        Primary   string
        Fallbacks []string
    }
    Models map[string]struct { // Key: provider/model reference
        Alias            string
        ProviderConfigID string
        Params           map[string]interface{}
    }
    Providers []LLMProviderConfig
    Anthropic struct { APIKey SecureString }
    OpenAI struct { APIKey  SecureString; BaseURL string }
    Ollama struct { BaseURL string }
    Bedrock struct { AWSRegion  string; AWSProfile string }
}
```

### `DMPolicy` (string enum)

Telegram DM policy.

```go
type DMPolicy string
const (
    DMPolicyPairing   DMPolicy = "pairing"
    DMPolicyAllowlist DMPolicy = "allowlist"
    DMPolicyOpen      DMPolicy = "open"
)
```

### `TelegramConfig` (struct)

Telegram Gateway configuration.

```go
type TelegramConfig struct {
    Token       SecureString
    WebhookURL  string
    AllowedIDs  []int64
    DMPolicy    DMPolicy
}
```

### `SlackConfig` (struct)

Slack Gateway configuration.

```go
type SlackConfig struct {
    BotToken    SecureString
    AppToken    SecureString
    WorkspaceID string
}
```

### `CLIConfig` (struct)

CLI Gateway configuration.

```go
type CLIConfig struct {
    HistoryFile string
    DebugMode   bool
}
```

### `MCPConfig` (struct)

MCP Server and Client configuration.

```go
type MCPConfig struct {
    Server struct {
        Enabled bool
        Port    int
        TLS     bool
    }
    Client struct {
        AllowedServers []string
        Timeout        time.Duration
        MaxRetries     int
    }
}
```

### `SkillsSystemConfig` (struct)

Global settings for the skill system.

```go
type SkillsSystemConfig struct {
    Entries map[string]SkillConfig
    Load    struct {
        ExtraDirs []string
        Watch     bool
    }
}
```

### `MemoryConfig` (struct)

Configuration for the agent's long-term memory.

```go
type MemoryConfig struct {
    Backend   MemoryBackend
    Citations MemoryCitationsMode
    QMD       MemoryQMDConfig
}
```

### `MemoryQMDConfig` (struct)

Configuration for the Queryable Memory Document (QMD) backend.

```go
type MemoryQMDConfig struct {
    Command            string
    IncludeDefaultMemory bool
    Paths              []MemoryQMDIndexPath
    Sessions           struct {
        Enabled      bool
        ExportDir    string
        RetentionDays int
    }
    Update struct {
        Interval  string
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
```