# User Customization - Data Dictionary

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Draft

---

## Overview

This document defines all data structures, types, interfaces, and constants for the User Customization implementation (SOUL.md, USER.md, RULES.md). Organized by Clean Architecture layers.

**Purpose:**
- Single source of truth for all data types
- Ensure consistency across layers
- Document validation rules and constraints
- Specify database schemas

---

## Table of Contents

1. [Domain Layer](#domain-layer)
2. [Use Case Layer](#use-case-layer)
3. [Infrastructure Layer](#infrastructure-layer)
4. [Configuration](#configuration)
5. [Type Aliases & Enums](#type-aliases--enums)

---

## Domain Layer

Location: `internal/domain/`

### 1. PersonaFile (Entity)

**File:** `personafile.go`

```go
// PersonaFile represents a user's persona configuration file
type PersonaFile struct {
    // User ID this file belongs to
    UserID string

    // File type: "SOUL", "USER", "RULES"
    Type PersonaFileType

    // Absolute file path
    Path string

    // File content (Markdown)
    Content string

    // Last modified timestamp
    ModifiedAt time.Time

    // File size in bytes
    SizeBytes int64
}
```

**Methods:**
```go
// Validate checks if persona file is valid
func (p *PersonaFile) Validate() error

// IsEmpty returns true if content is empty or whitespace only
func (p *PersonaFile) IsEmpty() bool

// TokenCount estimates token count for this file
func (p *PersonaFile) TokenCount() int
```

**Validation Rules:**
- `UserID` must be non-empty, <= 64 chars
- `Type` must be valid PersonaFileType
- `Path` must be absolute path within user's data directory
- `Content` must be valid UTF-8, <= 100KB
- `ModifiedAt` must not be zero value

---

### 2. RulesConfig (Value Object)

**File:** `rulesconfig.go`

```go
// RulesConfig represents parsed YAML frontmatter from RULES.md
type RulesConfig struct {
    // Actions requiring user confirmation
    RequiresConfirmation []string `yaml:"requires_confirmation"`

    // Blocked tools/actions (cannot execute)
    BlockedTools []string `yaml:"blocked_tools"`

    // Privacy settings
    Privacy PrivacyConfig `yaml:"privacy"`

    // Raw frontmatter (for validation)
    RawYAML string
}

// PrivacyConfig defines privacy-related rules
type PrivacyConfig struct {
    // Data that should never be stored
    NeverStore []string `yaml:"never_store"`
}
```

**Methods:**
```go
// Validate checks if rules config is valid
func (r *RulesConfig) Validate() error

// RequiresConfirmation checks if action requires confirmation
func (r *RulesConfig) RequiresConfirmationFor(action string) bool

// IsToolBlocked checks if tool is blocked
func (r *RulesConfig) IsToolBlocked(tool string) bool
```

**Validation Rules:**
- All lists must contain valid identifiers (alphanumeric + underscore)
- No duplicate entries in lists
- Privacy.NeverStore must include standard sensitive patterns

---

### 3. MemoryAction (Entity)

**File:** `memoryaction.go`

```go
// MemoryAction represents an explicit memory write operation
type MemoryAction struct {
    // Unique action ID
    ID string

    // User ID
    UserID string

    // Action type: "write_file", "persona_update"
    Type MemoryActionType

    // Target file path
    FilePath string

    // Operation: "append", "replace", "insert"
    Operation string

    // Content to write
    Content string

    // Confirmation required
    RequiresConfirmation bool

    // Confirmation status
    Confirmed bool

    // Timestamp
    CreatedAt time.Time
}
```

**Methods:**
```go
// Validate checks if memory action is valid
func (m *MemoryAction) Validate() error

// AwaitingConfirmation returns true if not yet confirmed
func (m *MemoryAction) AwaitingConfirmation() bool
```

---

### 4. PersonaFileRepository (Interface)

**File:** `personafile.go`

```go
// PersonaFileRepository defines operations for persona file storage
type PersonaFileRepository interface {
    // Get retrieves a persona file by user ID and type
    Get(ctx context.Context, userID string, fileType PersonaFileType) (*PersonaFile, error)

    // Save saves a persona file
    Save(ctx context.Context, file *PersonaFile) error

    // Delete deletes a persona file
    Delete(ctx context.Context, userID string, fileType PersonaFileType) error

    // List lists all persona files for a user
    List(ctx context.Context, userID string) ([]*PersonaFile, error)
}
```

**Expected Behavior:**
- `Get`: Returns ErrNotFound if file doesn't exist
- `Save`: Creates file if doesn't exist, updates if exists
- `Delete`: Returns nil if file doesn't exist (idempotent)
- `List`: Returns empty slice if no files found

---

## Use Case Layer

Location: `internal/usecase/persona/`

### 1. PromptComposer (Service)

**File:** `promptcomposer.go`

```go
// PromptComposer builds system prompts from persona files
type PromptComposer struct {
    repo          domain.PersonaFileRepository
    tokenBudget   TokenBudget
    globalPolicy  string
}

// ComposerInput represents input for prompt composition
type ComposerInput struct {
    UserID            string
    Platform          string // "slack", "telegram", "cli"
    AgentPreferences  AgentPreferences
}

// ComposerOutput represents composed system prompt
type ComposerOutput struct {
    SystemPrompt string
    TokensUsed   int
    Truncated    bool
    TruncatedFiles []string
}
```

**Methods:**
```go
// Compose builds system prompt from persona files
func (c *PromptComposer) Compose(ctx context.Context, input ComposerInput) (*ComposerOutput, error)
```

---

### 2. RulesEnforcer (Service)

**File:** `rulesenforcer.go`

```go
// RulesEnforcer enforces RULES.md hard rules
type RulesEnforcer struct {
    repo domain.PersonaFileRepository
}

// EnforcerInput represents input for rule enforcement
type EnforcerInput struct {
    UserID string
    Action string
    Tool   string
}

// EnforcerOutput represents enforcement result
type EnforcerOutput struct {
    Allowed bool
    Reason  string  // Human-readable reason if not allowed
}
```

---

### 3. MemoryWriter (Service)

**File:** `memorywriter.go`

```go
// MemoryWriter handles explicit memory write operations
type MemoryWriter struct {
    repo      domain.PersonaFileRepository
    auditor   AuditLogger
    enforcer  *RulesEnforcer
}

// WriteInput represents input for memory write
type WriteInput struct {
    UserID    string
    FilePath  string
    Content   string
    Operation string  // "append", "replace"
}

// WriteOutput represents write result
type WriteOutput struct {
    Success          bool
    RequiresConfirmation bool
    ConfirmationID   string  // ID to confirm later
}
```

---

## Infrastructure Layer

Location: `internal/infrastructure/persona/`

### 1. FileRepository (Implementation)

**File:** `filerepository.go`

```go
// FileRepository implements PersonaFileRepository using filesystem
type FileRepository struct {
    basePath string
    cache    *FileCache
}

// FileCache caches file content to reduce disk I/O
type FileCache struct {
    ttl time.Duration
    entries map[string]*CacheEntry
    mu sync.RWMutex
}

// CacheEntry represents cached file
type CacheEntry struct {
    Content    string
    ExpiresAt  time.Time
}
```

---

## Configuration

Location: `internal/config/`

### Feature Configuration

**YAML:**
```yaml
persona:
  enabled: true
  default_directory: "{{user_data_dir}}/persona"
  files:
    - name: "SOUL.md"
      template: "templates/SOUL.md"
    - name: "USER.md"
      template: "templates/USER.md"
    - name: "RULES.md"
      template: "templates/RULES.md"
  token_limits:
    max_total: 4000
    max_per_file: 1500
  cache_ttl: 900  # 15 minutes
  onboarding:
    enabled: true
    auto_trigger: true
```

**Go Struct:**
```go
type PersonaConfig struct {
    Enabled          bool              `yaml:"enabled"`
    DefaultDirectory string            `yaml:"default_directory"`
    Files            []PersonaFileSpec `yaml:"files"`
    TokenLimits      TokenBudget       `yaml:"token_limits"`
    CacheTTL         int               `yaml:"cache_ttl"`
    Onboarding       OnboardingConfig  `yaml:"onboarding"`
}

type PersonaFileSpec struct {
    Name     string `yaml:"name"`
    Template string `yaml:"template"`
}

type TokenBudget struct {
    MaxTotal   int            `yaml:"max_total"`
    MaxPerFile int            `yaml:"max_per_file"`
}

type OnboardingConfig struct {
    Enabled     bool `yaml:"enabled"`
    AutoTrigger bool `yaml:"auto_trigger"`
}
```

---

## Type Aliases & Enums

### Enumerations

#### PersonaFileType
```go
// PersonaFileType represents the type of persona file
type PersonaFileType int

const (
    PersonaFileSOUL PersonaFileType = iota  // SOUL.md
    PersonaFileUSER                         // USER.md
    PersonaFileRULES                        // RULES.md
)

// String returns the string representation
func (t PersonaFileType) String() string {
    return [...]string{"SOUL", "USER", "RULES"}[t]
}

// IsValid checks if enum value is valid
func (t PersonaFileType) IsValid() bool {
    return t >= PersonaFileSOUL && t <= PersonaFileRULES
}
```

#### MemoryActionType
```go
// MemoryActionType represents the type of memory action
type MemoryActionType int

const (
    MemoryActionWriteFile MemoryActionType = iota
    MemoryActionPersonaUpdate
)

// String returns the string representation
func (t MemoryActionType) String() string {
    return [...]string{"write_file", "persona_update"}[t]
}
```

---

## Error Types

```go
var (
    ErrPersonaFileNotFound      = errors.New("persona file not found")
    ErrInvalidPersonaFileType   = errors.New("invalid persona file type")
    ErrPathTraversal            = errors.New("path traversal detected")
    ErrRuleBlocked              = errors.New("action blocked by rules")
    ErrConfirmationRequired     = errors.New("confirmation required")
    ErrInvalidYAMLFrontmatter   = errors.New("invalid YAML frontmatter")
    ErrTokenBudgetExceeded      = errors.New("token budget exceeded")
)
```

---

## Constants

### File Paths
```go
const (
    PersonaFilenameSOUL  = "SOUL.md"
    PersonaFilenameUSER  = "USER.md"
    PersonaFilenameRULES = "RULES.md"
)
```

### Limits
```go
const (
    MaxPersonaFileSize = 100 * 1024  // 100KB
    MaxTokensTotal     = 4000
    MaxTokensPerFile   = 1500
    CacheTTLSeconds    = 900  // 15 minutes
)
```

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-15 | Initial version - domain entities defined |

---

**Next:** Fill in detailed validation rules, add implementation examples, define all service methods.
