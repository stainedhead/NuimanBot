# Improved Admin Features - Data Dictionary

**Created:** 2026-02-08
**Version:** 1.0
**Status:** Complete

---

## Overview

This document defines all data structures, types, interfaces, and constants for the Improved Admin Features implementation. Organized by Clean Architecture layers.

**Purpose:**
- Single source of truth for all data types
- Ensure consistency across layers
- Document validation rules and constraints
- Specify file storage schemas (JSON)
- Define API request/response types
- Document configuration structures

---

## Table of Contents

1. [Domain Layer](#domain-layer)
2. [Use Case Layer](#use-case-layer)
3. [Infrastructure Layer](#infrastructure-layer)
4. [Adapter Layer - REST API](#adapter-layer---rest-api)
5. [Adapter Layer - Web UI](#adapter-layer---web-ui)
6. [Adapter Layer - CLI](#adapter-layer---cli)
7. [Configuration](#configuration)
8. [Type Aliases & Enums](#type-aliases--enums)
9. [File Storage Schema](#file-storage-schema)
10. [Constants](#constants)
11. [Type Mapping Reference](#type-mapping-reference)

---

## Domain Layer

Location: `internal/domain/`

### 1. UserProfile (Entity)

**File:** `user_profile.go`

```go
// UserProfile represents comprehensive user identity and preferences
// beyond authentication. Contains personal information, platform integration,
// agent customization, and organizational context.
type UserProfile struct {
    // Core Identity
    UserID    string    `json:"userID"`    // References User.ID (primary key)
    Moniker   string    `json:"moniker"`   // Display name or handle
    FirstName string    `json:"firstName"` // Given name
    LastName  string    `json:"lastName"`  // Family name
    NickName  string    `json:"nickName"`  // Preferred informal name

    // Contact Information
    PrimaryEmail string `json:"primaryEmail"` // Primary contact email
    BackupEmail  string `json:"backupEmail"`  // Secondary contact email
    MobilePhone  string `json:"mobilePhone"`  // Mobile phone number (E.164 format)

    // Localization
    PrimaryLanguage   string `json:"primaryLanguage"`   // ISO 639-1 code (e.g., "en", "es")
    SecondaryLanguage string `json:"secondaryLanguage"` // ISO 639-1 code for fallback
    Timezone          string `json:"timezone"`          // IANA timezone (e.g., "America/New_York")
    PrimaryLocation   string `json:"primaryLocation"`   // Geographic location or timezone identifier

    // Organizational Context
    JobRole  string   `json:"jobRole"`  // User's organizational role
    UserType UserType `json:"userType"` // Individual, Enterprise, Developer, Admin

    // Multi-Platform Integration
    PlatformIDs PlatformIdentifiers `json:"platformIDs"` // Slack, Telegram, CLI identifiers

    // Personalization
    AgentPreferences  AgentPreferences  `json:"agentPreferences"`  // Agent behavior customization
    NotesInformation  NotesInformation  `json:"notesInformation"`  // Admin notes and flags
    ProfileInfo       string            `json:"profileInfo"`       // Freeform biographical info (max 2000 chars)

    // Metadata
    Enabled      bool      `json:"enabled"`          // Account enabled/disabled
    DataDirectory string   `json:"dataDirectory"`    // Path to user's data directory
    CreatedAt    time.Time `json:"createdAt"`        // Profile creation time
    UpdatedAt    time.Time `json:"updatedAt"`        // Last modification time
    LastVerified time.Time `json:"lastVerified"`     // Last time user verified their info
}
```

**Methods:**
```go
// Validate checks if profile is valid according to business rules
func (up *UserProfile) Validate() error

// GetDisplayName returns the best available display name
// Priority: NickName > Moniker > FirstName > UserID
func (up *UserProfile) GetDisplayName() string

// GetFullName returns "FirstName LastName", or empty string if both unset
func (up *UserProfile) GetFullName() string

// GetPreferredLanguage returns PrimaryLanguage or "en" if not set
func (up *UserProfile) GetPreferredLanguage() string

// GetTimezone returns Timezone or "UTC" if not set
func (up *UserProfile) GetTimezone() string
```

**Validation Rules:**
- `UserID`: Required, must be valid UUID format, <= 64 chars
- `Moniker`: Optional, max 50 chars, alphanumeric + hyphens/underscores
- `FirstName`: Optional, max 100 chars
- `LastName`: Optional, max 100 chars
- `NickName`: Optional, max 50 chars
- `PrimaryEmail`: Required, valid email format, max 254 chars (RFC 5321)
- `BackupEmail`: Optional, valid email format if present
- `MobilePhone`: Optional, E.164 format if present (e.g., "+1234567890")
- `PrimaryLanguage`: Required, valid ISO 639-1 code, default "en"
- `SecondaryLanguage`: Optional, valid ISO 639-1 code if present
- `Timezone`: Required, valid IANA timezone, default "UTC"
- `PrimaryLocation`: Optional, max 100 chars
- `JobRole`: Optional, max 100 chars
- `UserType`: Required, must be valid UserType enum
- `ProfileInfo`: Optional, max 2000 chars
- `DataDirectory`: Required, must be valid path

**Business Rules:**
- UserID must reference existing User.ID
- PrimaryEmail must be unique across all profiles
- BackupEmail must be different from PrimaryEmail if set
- Platform IDs must be unique per platform (enforced by index)
- DataDirectory must exist and be writable
- Enabled=false prevents user login but preserves data

**Example:**
```go
profile := &UserProfile{
    UserID:           "550e8400-e29b-41d4-a716-446655440000",
    Moniker:          "alice_admin",
    FirstName:        "Alice",
    LastName:         "Anderson",
    NickName:         "Ally",
    PrimaryEmail:     "alice@example.com",
    BackupEmail:      "alice.anderson@personal.com",
    MobilePhone:      "+14155552671",
    PrimaryLanguage:  "en",
    Timezone:         "America/Los_Angeles",
    PrimaryLocation:  "San Francisco, CA",
    JobRole:          "Engineering Manager",
    UserType:         UserTypeEnterprise,
    PlatformIDs: PlatformIdentifiers{
        Slack:    "U01ABC123",
        Telegram: "123456789",
        CLI:      "alice",
    },
    AgentPreferences: DefaultAgentPreferences(),
    NotesInformation: NotesInformation{},
    Enabled:          true,
    DataDirectory:    "users/550e8400-e29b-41d4-a716-446655440000",
    CreatedAt:        time.Now(),
    UpdatedAt:        time.Now(),
    LastVerified:     time.Now(),
}

if err := profile.Validate(); err != nil {
    // Handle validation error
}
```

---

### 2. PlatformIdentifiers (Value Object)

**File:** `user_profile.go`

```go
// PlatformIdentifiers stores user IDs for different messaging platforms.
// Used for routing messages from platforms to correct user profile.
type PlatformIdentifiers struct {
    CLI      string `json:"cli"`      // CLI username
    Slack    string `json:"slack"`    // Slack user ID (e.g., "U01ABC123")
    Telegram string `json:"telegram"` // Telegram user ID (numeric string, e.g., "123456789")
}
```

**Validation Rules:**
- `CLI`: Optional, must match User.Username if set
- `Slack`: Optional, must match pattern `^[UW][A-Z0-9]{8,10}$` (Slack user/workspace ID format)
- `Telegram`: Optional, must be numeric string if set (Telegram user ID is int64)
- At least one platform ID must be set (user must access from somewhere)
- Each platform ID must be unique across all users

**Immutability:** Not immutable. Updates require atomic write to users.json with index updates.

**Example:**
```go
platformIDs := PlatformIdentifiers{
    CLI:      "alice",
    Slack:    "U01ABC123DEF",
    Telegram: "1234567890",
}
```

---

### 3. AgentPreferences (Value Object)

**File:** `user_profile.go`

```go
// AgentPreferences stores user-specific preferences for agent behavior.
// Controls communication style, verbosity, formatting, and feature defaults.
type AgentPreferences struct {
    // Communication Style
    CommunicationStyle CommunicationStyle `json:"communicationStyle"` // professional, casual, technical, friendly
    Verbosity          Verbosity          `json:"verbosity"`          // concise, moderate, detailed
    ResponseFormat     ResponseFormat     `json:"responseFormat"`     // markdown, plain, structured

    // Content Preferences
    CodeExamplesPreferred bool `json:"codeExamplesPreferred"` // Include code examples in responses
    ExplainDecisions      bool `json:"explainDecisions"`      // Explain reasoning behind answers
    ProactiveMode         bool `json:"proactiveMode"`         // Offer suggestions proactively

    // Skill Defaults
    SkillDefaults map[string]SkillConfig `json:"skillDefaults"` // Per-skill default settings

    // Notification Preferences
    NotificationPreferences NotificationPreferences `json:"notificationPreferences"` // When to notify user
}

// SkillConfig stores default configuration for a specific skill
type SkillConfig struct {
    AutoExecute bool              `json:"autoExecute"` // Execute without confirmation
    Options     map[string]any    `json:"options"`     // Skill-specific options
}

// NotificationPreferences controls when user receives notifications
type NotificationPreferences struct {
    TaskCompletion  bool `json:"taskCompletion"`  // Notify on task completion
    Errors          bool `json:"errors"`          // Notify on errors
    LongRunningOps  bool `json:"longRunningOps"`  // Notify for ops > 30s
}
```

**Default Values:**
```go
func DefaultAgentPreferences() AgentPreferences {
    return AgentPreferences{
        CommunicationStyle:    CommunicationStyleProfessional,
        Verbosity:             VerbosityModerate,
        ResponseFormat:        ResponseFormatMarkdown,
        CodeExamplesPreferred: true,
        ExplainDecisions:      false,
        ProactiveMode:         true,
        SkillDefaults: map[string]SkillConfig{
            "commit": {
                AutoExecute: false,
                Options: map[string]any{
                    "autoStage": true,
                    "signoff":   true,
                },
            },
        },
        NotificationPreferences: NotificationPreferences{
            TaskCompletion: true,
            Errors:         true,
            LongRunningOps: true,
        },
    }
}
```

**Validation Rules:**
- `CommunicationStyle`: Must be valid CommunicationStyle enum
- `Verbosity`: Must be valid Verbosity enum
- `ResponseFormat`: Must be valid ResponseFormat enum
- `SkillDefaults`: Keys must be valid skill names

**Immutability:** Not immutable. Preferences updated via UserProfileService.

**Example:**
```go
prefs := AgentPreferences{
    CommunicationStyle:    CommunicationStyleTechnical,
    Verbosity:             VerbosityDetailed,
    ResponseFormat:        ResponseFormatMarkdown,
    CodeExamplesPreferred: true,
    ExplainDecisions:      true,
    ProactiveMode:         false,
    SkillDefaults: map[string]SkillConfig{
        "commit": {
            AutoExecute: true,
            Options: map[string]any{
                "autoStage": true,
                "signoff":   true,
            },
        },
    },
    NotificationPreferences: NotificationPreferences{
        TaskCompletion: true,
        Errors:         true,
        LongRunningOps: true,
    },
}
```

---

### 4. NotesInformation (Value Object)

**File:** `user_profile.go`

```go
// NotesInformation stores admin notes, flags, and metadata for a user.
// Used for tracking support tickets, beta features, restrictions, etc.
type NotesInformation struct {
    // Admin Notes
    AdminNotes []AdminNote `json:"adminNotes"` // Chronological list of admin notes

    // Feature Flags
    Flags map[string]bool `json:"flags"` // Feature flags (betaTester, earlyAccess, etc.)

    // Support Context
    SupportTickets []string `json:"supportTickets"` // Associated support ticket IDs

    // Access Restrictions
    Restrictions Restrictions `json:"restrictions"` // Rate limits, feature access overrides

    // Custom Metadata
    CustomMetadata map[string]any `json:"customMetadata"` // Freeform metadata
}

// AdminNote represents a timestamped note from an admin user
type AdminNote struct {
    Timestamp time.Time `json:"timestamp"` // When note was created
    AuthorID  string    `json:"authorID"`  // Admin user ID who created note
    Note      string    `json:"note"`      // Note content (max 1000 chars)
}

// Restrictions stores access control overrides and limits
type Restrictions struct {
    RateLimitOverride *int     `json:"rateLimitOverride"` // Override rate limit (req/min), nil = use default
    FeatureAccess     []string `json:"featureAccess"`     // Allowed features (e.g., ["bedrock", "advanced-tools"])
    BlockedFeatures   []string `json:"blockedFeatures"`   // Explicitly blocked features
}
```

**Default Values:**
```go
func DefaultNotesInformation() NotesInformation {
    return NotesInformation{
        AdminNotes:     []AdminNote{},
        Flags:          make(map[string]bool),
        SupportTickets: []string{},
        Restrictions: Restrictions{
            RateLimitOverride: nil,
            FeatureAccess:     []string{},
            BlockedFeatures:   []string{},
        },
        CustomMetadata: make(map[string]any),
    }
}
```

**Validation Rules:**
- `AdminNotes`: Each note's AuthorID must reference valid admin user
- `AdminNotes`: Each note content max 1000 chars
- `Flags`: Keys should be lowercase with underscores (e.g., "beta_tester")
- `SupportTickets`: Each ticket ID max 50 chars
- `Restrictions.FeatureAccess`: Feature names must be valid
- `Restrictions.RateLimitOverride`: Must be > 0 if set

**Business Rules:**
- Admin notes are append-only (never delete, only add)
- Flags can be set/unset by admin users only
- Restrictions override default system limits

**Example:**
```go
notes := NotesInformation{
    AdminNotes: []AdminNote{
        {
            Timestamp: time.Now(),
            AuthorID:  "admin-001",
            Note:      "User requested enterprise features access",
        },
    },
    Flags: map[string]bool{
        "beta_tester":  true,
        "early_access": false,
    },
    SupportTickets: []string{"TICKET-123", "TICKET-456"},
    Restrictions: Restrictions{
        RateLimitOverride: nil, // Use default
        FeatureAccess:     []string{"bedrock", "advanced-tools"},
        BlockedFeatures:   []string{},
    },
    CustomMetadata: map[string]any{
        "department": "Engineering",
        "project":    "AI Integration",
    },
}
```

---

### 5. BotConfig (Entity)

**File:** `bot_config.go`

```go
// BotConfig represents configuration for a Slack or Telegram bot.
// Supports both public (shared) and private (user-owned) bots.
type BotConfig struct {
    // Core Identity
    BotID   string  `json:"botID"`   // Unique identifier (UUID)
    BotName string  `json:"botName"` // Human-readable bot name
    BotType BotType `json:"botType"` // public or private

    // Ownership
    OwnerUserID string `json:"ownerUserID"` // Owner for private bots, null for public

    // Access Control
    Enabled        bool     `json:"enabled"`        // Bot enabled/disabled
    AllowedUserIDs []string `json:"allowedUserIDs"` // User IDs allowed to use bot (public bots only)

    // Platform-Specific (populated based on platform)
    Slack    *SlackBotConfig    `json:"slack,omitempty"`    // Slack-specific config
    Telegram *TelegramBotConfig `json:"telegram,omitempty"` // Telegram-specific config

    // Metadata
    CreatedAt time.Time      `json:"createdAt"` // Bot creation time
    UpdatedAt time.Time      `json:"updatedAt"` // Last modification time
    Metadata  map[string]any `json:"metadata"`  // Custom metadata
}

// SlackBotConfig stores Slack-specific bot configuration
type SlackBotConfig struct {
    BotToken       string `json:"botToken"`       // Slack bot token (xoxb-...) - ENCRYPTED
    AppToken       string `json:"appToken"`       // Slack app token (xapp-...) - ENCRYPTED
    SigningSecret  string `json:"signingSecret"`  // Slack signing secret - ENCRYPTED
    TeamID         string `json:"teamID"`         // Slack workspace/team ID
    BotUserID      string `json:"botUserID"`      // Slack bot user ID (starts with B)
}

// TelegramBotConfig stores Telegram-specific bot configuration
type TelegramBotConfig struct {
    BotToken      string   `json:"botToken"`      // Telegram bot token - ENCRYPTED
    BotUsername   string   `json:"botUsername"`   // Telegram bot username (without @)
    BotID         string   `json:"botID"`         // Telegram bot user ID
    AllowedChatIDs []string `json:"allowedChatIDs"` // Telegram chat IDs allowed (private bots)
}
```

**Methods:**
```go
// Validate checks if bot config is valid according to business rules
func (bc *BotConfig) Validate() error

// IsPublic returns true if bot is public (shared)
func (bc *BotConfig) IsPublic() bool

// IsPrivate returns true if bot is private (user-owned)
func (bc *BotConfig) IsPrivate() bool

// IsUserAllowed checks if user is allowed to use this bot
func (bc *BotConfig) IsUserAllowed(userID string) bool

// Platform returns the platform type (slack or telegram)
func (bc *BotConfig) Platform() Platform
```

**Validation Rules:**
- `BotID`: Required, valid UUID format, <= 64 chars
- `BotName`: Required, max 100 chars, alphanumeric + spaces/hyphens
- `BotType`: Required, must be valid BotType enum
- `OwnerUserID`: Required for private bots, must be null for public bots
- `Enabled`: Required, boolean
- `AllowedUserIDs`: Required for public bots (must have at least one user), must be empty for private bots
- Exactly one of `Slack` or `Telegram` must be set (not both, not neither)

**Slack-Specific Validation:**
- `BotToken`: Required, must match pattern `^xoxb-[a-zA-Z0-9-]+$`
- `AppToken`: Required for Socket Mode, must match pattern `^xapp-[a-zA-Z0-9-]+$`
- `SigningSecret`: Required, hex string
- `TeamID`: Required, must match pattern `^T[A-Z0-9]{8,10}$`
- `BotUserID`: Required, must match pattern `^B[A-Z0-9]{8,10}$`

**Telegram-Specific Validation:**
- `BotToken`: Required, must match pattern `^[0-9]+:[a-zA-Z0-9_-]+$`
- `BotUsername`: Required, alphanumeric + underscores, max 32 chars
- `BotID`: Required, numeric string
- `AllowedChatIDs`: Optional, each chat ID must be numeric string

**Business Rules:**
- Private bot: `BotType=private`, `OwnerUserID` set, `AllowedUserIDs=null`
- Public bot: `BotType=public`, `OwnerUserID=null`, `AllowedUserIDs` contains user IDs
- Private bot access restricted to owner only
- Public bot access restricted to users in `AllowedUserIDs`
- Admins can manage all bots
- Users can create and manage their own private bots
- Bot tokens encrypted at rest (AES-256)
- Platform-specific fields validated based on platform

**Example (Slack Private Bot):**
```go
slackBot := &BotConfig{
    BotID:       "bot-550e8400-e29b-41d4-a716-446655440000",
    BotName:     "Alice Personal Bot",
    BotType:     BotTypePrivate,
    OwnerUserID: "user-123",
    Enabled:     true,
    AllowedUserIDs: nil, // Not used for private bots
    Slack: &SlackBotConfig{
        BotToken:      "xoxb-123456789-abcdefghijk", // Will be encrypted
        AppToken:      "xapp-1-ABC123-xyz",           // Will be encrypted
        SigningSecret: "a1b2c3d4e5f6",                // Will be encrypted
        TeamID:        "T01ABC123",
        BotUserID:     "B01DEF456",
    },
    Telegram:  nil,
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
    Metadata:  map[string]any{},
}
```

**Example (Telegram Public Bot):**
```go
telegramBot := &BotConfig{
    BotID:          "bot-660f9511-f3ac-52e5-b827-557766551111",
    BotName:        "Team Support Bot",
    BotType:        BotTypePublic,
    OwnerUserID:    "", // Public bot has no owner
    Enabled:        true,
    AllowedUserIDs: []string{"user-123", "user-456", "user-789"},
    Slack:          nil,
    Telegram: &TelegramBotConfig{
        BotToken:       "123456789:ABCdefGHIjklMNOpqrsTUVwxyz", // Will be encrypted
        BotUsername:    "team_support_bot",
        BotID:          "123456789",
        AllowedChatIDs: []string{}, // Empty for public bots
    },
    CreatedAt: time.Now(),
    UpdatedAt: time.Now(),
    Metadata:  map[string]any{},
}
```

---

### 6. ServerConfig (Entity)

**File:** `server_config.go`

```go
// ServerConfig holds server configuration including paths, logging, and admin port.
type ServerConfig struct {
    // Environment
    Environment Environment `yaml:"environment"` // development, production, testing
    LogLevel    string      `yaml:"log_level"`   // debug, info, warn, error
    Debug       bool        `yaml:"debug"`       // Enable debug mode

    // Admin Interface
    AdminPort int `yaml:"admin_port"` // Web admin interface port (default 8080)

    // Paths
    Paths PathsConfig `yaml:"paths"` // Configurable paths for config, data, logs

    // Enabled Gateways
    EnabledGateways []Gateway `yaml:"enabled_gateways"` // CLI, Slack, Telegram
}

// PathsConfig holds configurable paths for different data types
type PathsConfig struct {
    Config string `yaml:"config"` // Default: "./config/"
    Data   string `yaml:"data"`   // Default: "./data/"
    Logs   string `yaml:"logs"`   // Default: "./logs/"
}

// Environment represents deployment environment
type Environment string

const (
    EnvironmentDevelopment Environment = "development"
    EnvironmentProduction  Environment = "production"
    EnvironmentTesting     Environment = "testing"
)
```

**Validation Rules:**
- `Environment`: Must be valid Environment enum
- `LogLevel`: Must be one of: "debug", "info", "warn", "error"
- `AdminPort`: Must be > 1024 and <= 65535 (unprivileged ports)
- `Paths.Config`: Must be valid directory path, must exist
- `Paths.Data`: Must be valid directory path, must exist and be writable
- `Paths.Logs`: Must be valid directory path, must exist and be writable

**Default Values:**
```go
func DefaultServerConfig() ServerConfig {
    return ServerConfig{
        Environment: EnvironmentDevelopment,
        LogLevel:    "info",
        Debug:       false,
        AdminPort:   8080,
        Paths: PathsConfig{
            Config: "./config/",
            Data:   "./data/",
            Logs:   "./logs/",
        },
        EnabledGateways: []Gateway{GatewayCLI},
    }
}
```

**Example:**
```go
config := ServerConfig{
    Environment: EnvironmentProduction,
    LogLevel:    "info",
    Debug:       false,
    AdminPort:   8080,
    Paths: PathsConfig{
        Config: "/etc/nuimanbot/config/",
        Data:   "/var/lib/nuimanbot/",
        Logs:   "/var/log/nuimanbot/",
    },
    EnabledGateways: []Gateway{GatewayCLI, GatewaySlack, GatewayTelegram},
}
```

---

### 7. ConfigReloadEvent (Entity)

**File:** `config_reload.go`

```go
// ConfigReloadEvent represents a configuration reload event.
// Used for tracking hot reload history and troubleshooting.
type ConfigReloadEvent struct {
    // Identity
    ID string `json:"id"` // Event UUID

    // Timing
    Timestamp time.Time `json:"timestamp"` // When reload was triggered

    // Trigger
    TriggerSource string `json:"triggerSource"` // "stdio", "api", "file_watcher", "admin_ui"
    TriggerUserID string `json:"triggerUserID"` // User who triggered reload (if applicable)

    // Result
    Success bool   `json:"success"` // Reload succeeded or failed
    Error   string `json:"error"`   // Error message if failed

    // Changes
    ChangedFiles  []string          `json:"changedFiles"`  // Files that changed (config.yaml, users.json, bots.json)
    ChangedFields []string          `json:"changedFields"` // Config fields that changed
    Diff          map[string]string `json:"diff"`          // Before/after diff for key fields
}
```

**Methods:**
```go
// Validate checks if reload event is valid
func (re *ConfigReloadEvent) Validate() error

// String returns human-readable event summary
func (re *ConfigReloadEvent) String() string
```

**Example:**
```go
event := &ConfigReloadEvent{
    ID:            "event-123",
    Timestamp:     time.Now(),
    TriggerSource: "admin_ui",
    TriggerUserID: "admin-001",
    Success:       true,
    Error:         "",
    ChangedFiles:  []string{"config.yaml", "bots.json"},
    ChangedFields: []string{"llm.default_model.primary", "gateways.slack.enabled"},
    Diff: map[string]string{
        "llm.default_model.primary": "anthropic-main -> anthropic-fast",
        "gateways.slack.enabled":    "false -> true",
    },
}
```

---

### 8. AuditLog (Entity)

**File:** `audit_log.go`

```go
// AuditLog represents an auditable admin action.
// Used for compliance, troubleshooting, and security monitoring.
type AuditLog struct {
    // Identity
    ID string `json:"id"` // Log entry UUID

    // Timing
    Timestamp time.Time `json:"timestamp"` // When action occurred

    // Actor
    AdminUserID string `json:"adminUserID"` // Admin user who performed action
    IPAddress   string `json:"ipAddress"`   // IP address of admin (if applicable)

    // Action
    Operation  string `json:"operation"`  // create, update, delete, read
    Resource   string `json:"resource"`   // user, bot, config, server
    ResourceID string `json:"resourceID"` // ID of affected resource

    // Changes
    Changes map[string]ChangeRecord `json:"changes"` // Field-level changes
}

// ChangeRecord represents a before/after change for a single field
type ChangeRecord struct {
    Before any `json:"before"` // Value before change (nil for create)
    After  any `json:"after"`  // Value after change (nil for delete)
}
```

**Methods:**
```go
// Validate checks if audit log is valid
func (al *AuditLog) Validate() error

// String returns human-readable log entry
func (al *AuditLog) String() string
```

**Validation Rules:**
- `ID`: Required, valid UUID
- `Timestamp`: Required, not zero
- `AdminUserID`: Required, must reference valid admin user
- `Operation`: Must be one of: "create", "read", "update", "delete"
- `Resource`: Must be one of: "user", "bot", "config", "server"
- `ResourceID`: Required for all operations

**Example:**
```go
log := &AuditLog{
    ID:          "log-123",
    Timestamp:   time.Now(),
    AdminUserID: "admin-001",
    IPAddress:   "192.168.1.100",
    Operation:   "update",
    Resource:    "user",
    ResourceID:  "user-123",
    Changes: map[string]ChangeRecord{
        "timezone": {
            Before: "UTC",
            After:  "America/New_York",
        },
        "jobRole": {
            Before: "Developer",
            After:  "Engineering Manager",
        },
    },
}
```

---

### 9. UserProfileRepository (Interface)

**File:** `user_profile.go`

```go
// UserProfileRepository defines operations for user profile persistence.
type UserProfileRepository interface {
    // Create creates a new user profile
    Create(ctx context.Context, profile *UserProfile) error

    // Get retrieves a user profile by user ID
    Get(ctx context.Context, userID string) (*UserProfile, error)

    // GetByEmail retrieves a user profile by email address
    GetByEmail(ctx context.Context, email string) (*UserProfile, error)

    // GetByPlatformID retrieves a user profile by platform-specific ID
    GetByPlatformID(ctx context.Context, platform Platform, platformID string) (*UserProfile, error)

    // Update updates an existing user profile
    Update(ctx context.Context, profile *UserProfile) error

    // PartialUpdate updates only specified fields
    PartialUpdate(ctx context.Context, userID string, updates map[string]any) error

    // Delete deletes a user profile (soft delete - sets Enabled=false)
    Delete(ctx context.Context, userID string) error

    // List lists all user profiles with optional filtering
    List(ctx context.Context, filter ProfileFilter) ([]*UserProfile, error)

    // Count returns total number of profiles matching filter
    Count(ctx context.Context, filter ProfileFilter) (int, error)
}

// ProfileFilter defines filtering options for listing profiles
type ProfileFilter struct {
    Enabled    *bool     // Filter by enabled status (nil = all)
    UserType   *UserType // Filter by user type (nil = all)
    SearchTerm string    // Search in username, email, name fields
    Limit      int       // Max results (0 = no limit)
    Offset     int       // Pagination offset
}
```

**Expected Behavior:**
- `Create`: Creates profile and user directory, updates indexes, returns ErrAlreadyExists if userID exists
- `Get`: Returns profile or ErrNotFound if userID doesn't exist
- `GetByEmail`: Returns profile or ErrNotFound, uses email index for O(1) lookup
- `GetByPlatformID`: Returns profile or ErrNotFound, uses platform index for O(1) lookup
- `Update`: Updates profile, timestamps, and indexes, returns ErrNotFound if doesn't exist
- `PartialUpdate`: Updates only specified fields, validates each field, returns list of updated fields
- `Delete`: Soft delete (sets Enabled=false), preserves data for audit purposes
- `List`: Returns paginated results, applies filters, sorted by CreatedAt descending
- `Count`: Returns total count without pagination

**Error Conditions:**
- Returns `ErrNotFound` if profile doesn't exist (Get, Update, Delete)
- Returns `ErrAlreadyExists` if userID/email/platformID already exists (Create)
- Returns `ErrInvalidInput` if validation fails
- Returns `ErrConcurrentModification` if file was modified by another process

---

### 10. BotConfigRepository (Interface)

**File:** `bot_config.go`

```go
// BotConfigRepository defines operations for bot configuration persistence.
type BotConfigRepository interface {
    // Create creates a new bot configuration
    Create(ctx context.Context, bot *BotConfig) error

    // Get retrieves a bot configuration by bot ID
    Get(ctx context.Context, botID string) (*BotConfig, error)

    // GetByName retrieves a bot configuration by bot name
    GetByName(ctx context.Context, botName string) (*BotConfig, error)

    // GetByOwner retrieves all bot configurations owned by a user
    GetByOwner(ctx context.Context, ownerUserID string) ([]*BotConfig, error)

    // GetByPlatform retrieves all bot configurations for a platform
    GetByPlatform(ctx context.Context, platform Platform) ([]*BotConfig, error)

    // Update updates an existing bot configuration
    Update(ctx context.Context, bot *BotConfig) error

    // PartialUpdate updates only specified fields
    PartialUpdate(ctx context.Context, botID string, updates map[string]any) error

    // Delete deletes a bot configuration (hard delete)
    Delete(ctx context.Context, botID string) error

    // List lists all bot configurations with optional filtering
    List(ctx context.Context, filter BotFilter) ([]*BotConfig, error)

    // Count returns total number of bots matching filter
    Count(ctx context.Context, filter BotFilter) (int, error)
}

// BotFilter defines filtering options for listing bots
type BotFilter struct {
    Platform  *Platform // Filter by platform (nil = all)
    BotType   *BotType  // Filter by bot type (nil = all)
    Enabled   *bool     // Filter by enabled status (nil = all)
    OwnerID   string    // Filter by owner (empty = all)
    Limit     int       // Max results (0 = no limit)
    Offset    int       // Pagination offset
}
```

**Expected Behavior:**
- `Create`: Encrypts bot tokens, creates config, updates indexes, returns ErrAlreadyExists if botID exists
- `Get`: Returns decrypted bot config or ErrNotFound
- `GetByName`: Returns bot or ErrNotFound, uses name index
- `GetByOwner`: Returns all bots owned by user (private bots only)
- `GetByPlatform`: Returns all bots for platform (Slack or Telegram)
- `Update`: Encrypts tokens if changed, updates config and indexes
- `PartialUpdate`: Updates only specified fields, encrypts if token fields updated
- `Delete`: Hard delete (removes from bots.json and indexes)
- `List`: Returns paginated results with filters
- `Count`: Returns total count without pagination

**Error Conditions:**
- Returns `ErrNotFound` if bot doesn't exist (Get, Update, Delete)
- Returns `ErrAlreadyExists` if botID/name already exists (Create)
- Returns `ErrInvalidInput` if validation fails
- Returns `ErrEncryptionFailed` if token encryption fails
- Returns `ErrDecryptionFailed` if token decryption fails

---

## Use Case Layer

Location: `internal/usecase/profile/`, `internal/usecase/botmgmt/`, etc.

### 1. UserProfileService

**File:** `internal/usecase/profile/service.go`

```go
// UserProfileService orchestrates user profile management use cases.
type UserProfileService struct {
    repo      domain.UserProfileRepository
    userRepo  domain.UserRepository
    audit     AuditLogger
    validator ProfileValidator
}

// NewUserProfileService creates a new service instance
func NewUserProfileService(
    repo domain.UserProfileRepository,
    userRepo domain.UserRepository,
    audit AuditLogger,
    validator ProfileValidator,
) *UserProfileService {
    return &UserProfileService{
        repo:      repo,
        userRepo:  userRepo,
        audit:     audit,
        validator: validator,
    }
}
```

**Methods:**
```go
// CreateProfile creates a new user profile
func (s *UserProfileService) CreateProfile(
    ctx context.Context,
    input CreateProfileInput,
) (*domain.UserProfile, error)

// GetProfile retrieves a user profile by ID
func (s *UserProfileService) GetProfile(
    ctx context.Context,
    userID string,
) (*domain.UserProfile, error)

// UpdateProfile updates an existing user profile
func (s *UserProfileService) UpdateProfile(
    ctx context.Context,
    input UpdateProfileInput,
) (*domain.UserProfile, error)

// DeleteProfile deletes a user profile (soft delete)
func (s *UserProfileService) DeleteProfile(
    ctx context.Context,
    userID string,
    adminUserID string,
) error

// ListProfiles lists user profiles with filtering and pagination
func (s *UserProfileService) ListProfiles(
    ctx context.Context,
    input ListProfilesInput,
) (*ListProfilesOutput, error)

// LinkPlatform links a platform ID to a user profile
func (s *UserProfileService) LinkPlatform(
    ctx context.Context,
    userID string,
    platform domain.Platform,
    platformID string,
) error

// UnlinkPlatform unlinks a platform ID from a user profile
func (s *UserProfileService) UnlinkPlatform(
    ctx context.Context,
    userID string,
    platform domain.Platform,
) error

// UpdateAgentPreferences updates agent preferences for a user
func (s *UserProfileService) UpdateAgentPreferences(
    ctx context.Context,
    userID string,
    prefs domain.AgentPreferences,
) error

// AddAdminNote adds an admin note to a user profile
func (s *UserProfileService) AddAdminNote(
    ctx context.Context,
    userID string,
    adminUserID string,
    note string,
) error
```

**Use Cases:**
- Create new user profile with validation
- Retrieve profile by ID, email, or platform ID
- Update profile with full or partial updates
- Soft delete profile (preserve data)
- List/search profiles with pagination
- Link/unlink platform identifiers
- Update agent personalization preferences
- Add admin notes for tracking

---

### 2. CreateProfileInput (Use Case Input)

**File:** `internal/usecase/profile/types.go`

```go
// CreateProfileInput represents input for creating a user profile
type CreateProfileInput struct {
    // Core Identity
    UserID    string // Must reference existing User
    Moniker   string
    FirstName string
    LastName  string
    NickName  string

    // Contact
    PrimaryEmail string // Required
    BackupEmail  string
    MobilePhone  string

    // Localization
    PrimaryLanguage   string // Required, default "en"
    SecondaryLanguage string
    Timezone          string // Required, default "UTC"
    PrimaryLocation   string

    // Organizational
    JobRole  string
    UserType domain.UserType // Required, default Individual

    // Platform Integration
    PlatformIDs domain.PlatformIdentifiers

    // Personalization
    AgentPreferences domain.AgentPreferences // Optional, uses defaults if empty
    ProfileInfo      string
}
```

**Validation Rules:**
- `UserID`: Required, must reference existing User.ID
- `PrimaryEmail`: Required, valid email format, unique
- `PrimaryLanguage`: Defaults to "en" if empty
- `Timezone`: Defaults to "UTC" if empty
- `UserType`: Defaults to Individual if empty
- All other fields: Optional, validated if present

---

### 3. UpdateProfileInput (Use Case Input)

**File:** `internal/usecase/profile/types.go`

```go
// UpdateProfileInput represents input for updating a user profile
type UpdateProfileInput struct {
    UserID string // Required, identifies profile to update

    // Fields to update (only non-nil fields are updated)
    Moniker           *string
    FirstName         *string
    LastName          *string
    NickName          *string
    PrimaryEmail      *string
    BackupEmail       *string
    MobilePhone       *string
    PrimaryLanguage   *string
    SecondaryLanguage *string
    Timezone          *string
    PrimaryLocation   *string
    JobRole           *string
    UserType          *domain.UserType
    ProfileInfo       *string
    Enabled           *bool

    // Admin performing update (for audit log)
    AdminUserID string
}
```

**Validation Rules:**
- `UserID`: Required, must exist
- `AdminUserID`: Required for audit trail
- Only non-nil pointer fields are validated and updated
- PrimaryEmail must remain unique if updated

---

### 4. ListProfilesOutput (Use Case Output)

**File:** `internal/usecase/profile/types.go`

```go
// ListProfilesOutput represents output from listing user profiles
type ListProfilesOutput struct {
    Profiles   []*domain.UserProfile // Paginated results
    TotalCount int                   // Total count (for pagination)
    Limit      int                   // Requested limit
    Offset     int                   // Requested offset
    HasMore    bool                  // True if more results available
}
```

---

### 5. BotManagementService

**File:** `internal/usecase/botmgmt/service.go`

```go
// BotManagementService orchestrates bot configuration management use cases.
type BotManagementService struct {
    repo      domain.BotConfigRepository
    userRepo  domain.UserRepository
    encryptor EncryptionService
    audit     AuditLogger
    validator BotValidator
}

// NewBotManagementService creates a new service instance
func NewBotManagementService(
    repo domain.BotConfigRepository,
    userRepo domain.UserRepository,
    encryptor EncryptionService,
    audit AuditLogger,
    validator BotValidator,
) *BotManagementService {
    return &BotManagementService{
        repo:      repo,
        userRepo:  userRepo,
        encryptor: encryptor,
        audit:     audit,
        validator: validator,
    }
}
```

**Methods:**
```go
// CreateBot creates a new bot configuration
func (s *BotManagementService) CreateBot(
    ctx context.Context,
    input CreateBotInput,
) (*domain.BotConfig, error)

// GetBot retrieves a bot configuration by ID
func (s *BotManagementService) GetBot(
    ctx context.Context,
    botID string,
) (*domain.BotConfig, error)

// UpdateBot updates an existing bot configuration
func (s *BotManagementService) UpdateBot(
    ctx context.Context,
    input UpdateBotInput,
) (*domain.BotConfig, error)

// DeleteBot deletes a bot configuration
func (s *BotManagementService) DeleteBot(
    ctx context.Context,
    botID string,
    adminUserID string,
) error

// ListBots lists bot configurations with filtering
func (s *BotManagementService) ListBots(
    ctx context.Context,
    input ListBotsInput,
) (*ListBotsOutput, error)

// EnableBot enables a bot (sets Enabled=true)
func (s *BotManagementService) EnableBot(
    ctx context.Context,
    botID string,
    adminUserID string,
) error

// DisableBot disables a bot (sets Enabled=false)
func (s *BotManagementService) DisableBot(
    ctx context.Context,
    botID string,
    adminUserID string,
) error

// UpdateAllowedUsers updates allowed users for a public bot
func (s *BotManagementService) UpdateAllowedUsers(
    ctx context.Context,
    botID string,
    userIDs []string,
    adminUserID string,
) error

// GetBotsForUser retrieves all bots accessible to a user
func (s *BotManagementService) GetBotsForUser(
    ctx context.Context,
    userID string,
) ([]*domain.BotConfig, error)
```

---

### 6. CreateBotInput (Use Case Input)

**File:** `internal/usecase/botmgmt/types.go`

```go
// CreateBotInput represents input for creating a bot
type CreateBotInput struct {
    BotName        string           // Required
    BotType        domain.BotType   // Required (public or private)
    Platform       domain.Platform  // Required (slack or telegram)
    OwnerUserID    string           // Required for private bots
    AllowedUserIDs []string         // Required for public bots
    Enabled        bool             // Default true

    // Platform-specific configuration (one required based on Platform)
    Slack    *domain.SlackBotConfig    // For Slack bots
    Telegram *domain.TelegramBotConfig // For Telegram bots

    // Admin performing creation (for audit log)
    AdminUserID string
}
```

---

### 7. ConfigurationService

**File:** `internal/usecase/config/service.go`

```go
// ConfigurationService orchestrates configuration management use cases.
type ConfigurationService struct {
    loader   ConfigLoader
    watcher  ConfigWatcher
    audit    AuditLogger
    validator ConfigValidator
}

// NewConfigurationService creates a new service instance
func NewConfigurationService(
    loader ConfigLoader,
    watcher ConfigWatcher,
    audit AuditLogger,
    validator ConfigValidator,
) *ConfigurationService {
    return &ConfigurationService{
        loader:    loader,
        watcher:   watcher,
        audit:     audit,
        validator: validator,
    }
}
```

**Methods:**
```go
// LoadConfig loads configuration from files
func (s *ConfigurationService) LoadConfig(
    ctx context.Context,
) (*Config, error)

// ReloadConfig reloads configuration and applies changes
func (s *ConfigurationService) ReloadConfig(
    ctx context.Context,
    triggerSource string,
    triggerUserID string,
) (*domain.ConfigReloadEvent, error)

// UpdateConfig updates configuration and writes to file
func (s *ConfigurationService) UpdateConfig(
    ctx context.Context,
    updates map[string]any,
    adminUserID string,
) error

// ValidateConfig validates configuration without applying
func (s *ConfigurationService) ValidateConfig(
    ctx context.Context,
    config *Config,
) error

// GetConfig retrieves current configuration
func (s *ConfigurationService) GetConfig(
    ctx context.Context,
) (*Config, error)

// WatchConfig starts watching configuration files for changes
func (s *ConfigurationService) WatchConfig(
    ctx context.Context,
) error
```

---

## Infrastructure Layer

Location: `internal/infrastructure/storage/`, `internal/infrastructure/security/`, etc.

### 1. FileUserProfileRepository

**File:** `internal/infrastructure/storage/file_user_profile_repository.go`

```go
// FileUserProfileRepository implements UserProfileRepository using JSON file storage.
type FileUserProfileRepository struct {
    dataDir      string              // Base data directory
    filePath     string              // Path to users.json
    fileLock     *flock.Flock        // File lock for atomic operations
    cache        *profileCache       // In-memory cache with TTL
    indexer      *profileIndexer     // Indexes for fast lookups
}

// NewFileUserProfileRepository creates a new file-based repository
func NewFileUserProfileRepository(dataDir string) (*FileUserProfileRepository, error) {
    return &FileUserProfileRepository{
        dataDir:  dataDir,
        filePath: filepath.Join(dataDir, "users.json"),
        fileLock: flock.New(filepath.Join(dataDir, "users.json.lock")),
        cache:    newProfileCache(5 * time.Minute), // 5-minute TTL
        indexer:  newProfileIndexer(),
    }
}
```

**Implements:** `domain.UserProfileRepository`

**Dependencies:**
- File system for JSON storage
- File locking library (github.com/gofrs/flock)
- In-memory cache for performance
- Indexer for O(1) lookups

**Storage Structure:**
```
data/
├── users.json          # Central registry with indexes
└── users/
    ├── <user-id-1>/
    │   ├── profile.json       # Full profile (redundant copy)
    │   ├── preferences.json   # Agent preferences
    │   ├── todos.json         # User todos
    │   └── history.json       # Command history
    └── <user-id-2>/
        └── ...
```

---

### 2. FileBotConfigRepository

**File:** `internal/infrastructure/storage/file_bot_config_repository.go`

```go
// FileBotConfigRepository implements BotConfigRepository using JSON file storage.
type FileBotConfigRepository struct {
    dataDir      string              // Base data directory
    filePath     string              // Path to bots.json
    fileLock     *flock.Flock        // File lock for atomic operations
    encryptor    EncryptionService   // Bot token encryption
    cache        *botCache           // In-memory cache with TTL
    indexer      *botIndexer         // Indexes for fast lookups
}

// NewFileBotConfigRepository creates a new file-based repository
func NewFileBotConfigRepository(
    dataDir string,
    encryptor EncryptionService,
) (*FileBotConfigRepository, error) {
    return &FileBotConfigRepository{
        dataDir:   dataDir,
        filePath:  filepath.Join(dataDir, "bots.json"),
        fileLock:  flock.New(filepath.Join(dataDir, "bots.json.lock")),
        encryptor: encryptor,
        cache:     newBotCache(5 * time.Minute),
        indexer:   newBotIndexer(),
    }
}
```

**Implements:** `domain.BotConfigRepository`

**Dependencies:**
- File system for JSON storage
- File locking library
- EncryptionService for token encryption/decryption
- In-memory cache
- Indexer for lookups

---

### 3. EncryptionService

**File:** `internal/infrastructure/security/encryption.go`

```go
// EncryptionService handles encryption/decryption of sensitive data.
type EncryptionService struct {
    key []byte // AES-256 encryption key (32 bytes)
}

// NewEncryptionService creates a new encryption service
func NewEncryptionService(key []byte) (*EncryptionService, error) {
    if len(key) != 32 {
        return nil, errors.New("encryption key must be 32 bytes for AES-256")
    }
    return &EncryptionService{key: key}, nil
}
```

**Methods:**
```go
// Encrypt encrypts plaintext using AES-256-GCM
func (s *EncryptionService) Encrypt(plaintext string) (string, error)

// Decrypt decrypts ciphertext using AES-256-GCM
func (s *EncryptionService) Decrypt(ciphertext string) (string, error)

// RotateKey re-encrypts all data with a new key
func (s *EncryptionService) RotateKey(newKey []byte, data []string) ([]string, error)
```

**Encryption Format:**
```
base64(nonce + encrypted_data + auth_tag)
```
- Nonce: 12 bytes (random)
- Encrypted data: Variable length
- Auth tag: 16 bytes (GCM authentication)

**Key Management:**
- Key stored in environment variable: `NUIMANBOT_ENCRYPTION_KEY`
- Key must be 32 bytes (64 hex chars) for AES-256
- Key rotation supported (re-encrypt all bot tokens)

---

### 4. AuditLogger

**File:** `internal/infrastructure/audit/logger.go`

```go
// AuditLogger writes audit log entries to file.
type AuditLogger struct {
    logPath string       // Path to audit.log
    writer  io.Writer    // Log file writer
    mutex   sync.Mutex   // Concurrent write protection
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logPath string) (*AuditLogger, error) {
    file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
    if err != nil {
        return nil, err
    }
    return &AuditLogger{
        logPath: logPath,
        writer:  file,
    }, nil
}
```

**Methods:**
```go
// Log writes an audit log entry
func (l *AuditLogger) Log(ctx context.Context, entry *domain.AuditLog) error

// Query retrieves audit log entries with filtering
func (l *AuditLogger) Query(ctx context.Context, filter AuditFilter) ([]*domain.AuditLog, error)
```

**Log Format:** JSON lines (one entry per line)
```json
{"id":"log-123","timestamp":"2026-02-08T12:00:00Z","adminUserID":"admin-001","operation":"update","resource":"user","resourceID":"user-123","changes":{"timezone":{"before":"UTC","after":"America/New_York"}}}
```

---

### 5. ConfigLoader

**File:** `internal/infrastructure/config/loader.go`

```go
// ConfigLoader loads configuration from YAML files and environment variables.
type ConfigLoader struct {
    configPath string                 // Path to config.yaml
    envPrefix  string                 // Environment variable prefix
    cache      *configCache           // In-memory cache
}

// NewConfigLoader creates a new config loader
func NewConfigLoader(configPath string) *ConfigLoader {
    return &ConfigLoader{
        configPath: configPath,
        envPrefix:  "NUIMANBOT_",
        cache:      newConfigCache(1 * time.Minute),
    }
}
```

**Methods:**
```go
// Load loads configuration from files and environment
func (l *ConfigLoader) Load() (*Config, error)

// Reload reloads configuration
func (l *ConfigLoader) Reload() (*Config, error)

// Validate validates configuration structure
func (l *ConfigLoader) Validate(config *Config) error

// Write writes configuration to file
func (l *ConfigLoader) Write(config *Config) error
```

**Environment Variable Mapping:**
```
NUIMANBOT_SERVER_ADMIN_PORT=8080 → config.Server.AdminPort
NUIMANBOT_SERVER_PATHS_DATA=/data → config.Server.Paths.Data
NUIMANBOT_GATEWAYS_SLACK_ENABLED=true → config.Gateways.Slack.Enabled
NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY=anthropic → config.LLM.DefaultModel.Primary
```

---

### 6. ConfigWatcher

**File:** `internal/infrastructure/config/watcher.go`

```go
// ConfigWatcher watches configuration files for changes.
type ConfigWatcher struct {
    watcher  *fsnotify.Watcher     // File system watcher
    paths    []string              // Paths to watch
    callback func()                // Callback on change
    debounce time.Duration         // Debounce duration
}

// NewConfigWatcher creates a new config watcher
func NewConfigWatcher(paths []string, callback func()) (*ConfigWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    return &ConfigWatcher{
        watcher:  watcher,
        paths:    paths,
        callback: callback,
        debounce: 1 * time.Second, // 1-second debounce
    }, nil
}
```

**Methods:**
```go
// Start starts watching files
func (w *ConfigWatcher) Start(ctx context.Context) error

// Stop stops watching files
func (w *ConfigWatcher) Stop() error
```

**Watched Files:**
- `config/config.yaml`
- `data/users.json`
- `data/bots.json`

---

## Adapter Layer - REST API

Location: `internal/adapter/rest/`

### 1. UserProfileHandler

**File:** `internal/adapter/rest/profile_handler.go`

```go
// UserProfileHandler handles HTTP requests for user profile management.
type UserProfileHandler struct {
    service *usecase.UserProfileService
    auth    *AuthMiddleware
    rbac    *RBACMiddleware
}

// NewUserProfileHandler creates a new handler
func NewUserProfileHandler(
    service *usecase.UserProfileService,
    auth *AuthMiddleware,
    rbac *RBACMiddleware,
) *UserProfileHandler {
    return &UserProfileHandler{
        service: service,
        auth:    auth,
        rbac:    rbac,
    }
}
```

**Endpoints:**
- `GET /api/v1/admin/profiles` - List profiles
- `GET /api/v1/admin/profiles/:id` - Get profile
- `POST /api/v1/admin/profiles` - Create profile
- `PUT /api/v1/admin/profiles/:id` - Update profile (partial)
- `DELETE /api/v1/admin/profiles/:id` - Delete profile
- `POST /api/v1/admin/profiles/:id/platforms` - Link platform
- `DELETE /api/v1/admin/profiles/:id/platforms/:platform` - Unlink platform

---

### 2. CreateProfileRequest (API Request)

**File:** `internal/adapter/rest/types.go`

```go
// CreateProfileRequest represents API request for creating a user profile
type CreateProfileRequest struct {
    UserID            string                   `json:"userID" binding:"required"`
    Moniker           string                   `json:"moniker"`
    FirstName         string                   `json:"firstName"`
    LastName          string                   `json:"lastName"`
    NickName          string                   `json:"nickName"`
    PrimaryEmail      string                   `json:"primaryEmail" binding:"required,email"`
    BackupEmail       string                   `json:"backupEmail" binding:"omitempty,email"`
    MobilePhone       string                   `json:"mobilePhone"`
    PrimaryLanguage   string                   `json:"primaryLanguage"`
    SecondaryLanguage string                   `json:"secondaryLanguage"`
    Timezone          string                   `json:"timezone"`
    PrimaryLocation   string                   `json:"primaryLocation"`
    JobRole           string                   `json:"jobRole"`
    UserType          string                   `json:"userType"`
    PlatformIDs       *PlatformIDsRequest      `json:"platformIDs"`
    AgentPreferences  *AgentPreferencesRequest `json:"agentPreferences"`
    ProfileInfo       string                   `json:"profileInfo"`
}

// PlatformIDsRequest represents platform identifiers in API request
type PlatformIDsRequest struct {
    CLI      string `json:"cli"`
    Slack    string `json:"slack"`
    Telegram string `json:"telegram"`
}

// AgentPreferencesRequest represents agent preferences in API request
type AgentPreferencesRequest struct {
    CommunicationStyle string                        `json:"communicationStyle"`
    Verbosity          string                        `json:"verbosity"`
    ResponseFormat     string                        `json:"responseFormat"`
    CodeExamplesPreferred bool                       `json:"codeExamplesPreferred"`
    ExplainDecisions      bool                       `json:"explainDecisions"`
    ProactiveMode         bool                       `json:"proactiveMode"`
    SkillDefaults         map[string]SkillConfigRequest `json:"skillDefaults"`
    NotificationPreferences *NotificationPreferencesRequest `json:"notificationPreferences"`
}

type SkillConfigRequest struct {
    AutoExecute bool           `json:"autoExecute"`
    Options     map[string]any `json:"options"`
}

type NotificationPreferencesRequest struct {
    TaskCompletion bool `json:"taskCompletion"`
    Errors         bool `json:"errors"`
    LongRunningOps bool `json:"longRunningOps"`
}
```

**Example Request:**
```json
{
  "userID": "550e8400-e29b-41d4-a716-446655440000",
  "firstName": "Alice",
  "lastName": "Anderson",
  "primaryEmail": "alice@example.com",
  "timezone": "America/Los_Angeles",
  "userType": "enterprise",
  "platformIDs": {
    "slack": "U01ABC123",
    "cli": "alice"
  },
  "agentPreferences": {
    "communicationStyle": "professional",
    "verbosity": "moderate",
    "responseFormat": "markdown"
  }
}
```

---

### 3. ProfileResponse (API Response)

**File:** `internal/adapter/rest/types.go`

```go
// ProfileResponse represents API response for user profile
type ProfileResponse struct {
    UserID            string                  `json:"userID"`
    Moniker           string                  `json:"moniker"`
    FirstName         string                  `json:"firstName"`
    LastName          string                  `json:"lastName"`
    NickName          string                  `json:"nickName"`
    PrimaryEmail      string                  `json:"primaryEmail"`
    BackupEmail       string                  `json:"backupEmail"`
    MobilePhone       string                  `json:"mobilePhone"`
    PrimaryLanguage   string                  `json:"primaryLanguage"`
    SecondaryLanguage string                  `json:"secondaryLanguage"`
    Timezone          string                  `json:"timezone"`
    PrimaryLocation   string                  `json:"primaryLocation"`
    JobRole           string                  `json:"jobRole"`
    UserType          string                  `json:"userType"`
    PlatformIDs       PlatformIDsResponse     `json:"platformIDs"`
    AgentPreferences  AgentPreferencesResponse `json:"agentPreferences"`
    ProfileInfo       string                  `json:"profileInfo"`
    Enabled           bool                    `json:"enabled"`
    CreatedAt         string                  `json:"createdAt"` // RFC3339 format
    UpdatedAt         string                  `json:"updatedAt"` // RFC3339 format
}

// ListProfilesResponse represents API response for listing profiles
type ListProfilesResponse struct {
    Profiles   []ProfileResponse `json:"profiles"`
    TotalCount int               `json:"totalCount"`
    Limit      int               `json:"limit"`
    Offset     int               `json:"offset"`
    HasMore    bool              `json:"hasMore"`
}
```

**Example Response:**
```json
{
  "userID": "550e8400-e29b-41d4-a716-446655440000",
  "firstName": "Alice",
  "lastName": "Anderson",
  "primaryEmail": "alice@example.com",
  "timezone": "America/Los_Angeles",
  "userType": "enterprise",
  "platformIDs": {
    "slack": "U01ABC123",
    "cli": "alice"
  },
  "enabled": true,
  "createdAt": "2026-02-08T12:00:00Z",
  "updatedAt": "2026-02-08T12:00:00Z"
}
```

---

### 4. UpdateProfileRequest (API Request)

**File:** `internal/adapter/rest/types.go`

```go
// UpdateProfileRequest represents API request for updating profile (partial update)
type UpdateProfileRequest struct {
    Moniker           *string                   `json:"moniker"`
    FirstName         *string                   `json:"firstName"`
    LastName          *string                   `json:"lastName"`
    NickName          *string                   `json:"nickName"`
    PrimaryEmail      *string                   `json:"primaryEmail"`
    BackupEmail       *string                   `json:"backupEmail"`
    MobilePhone       *string                   `json:"mobilePhone"`
    PrimaryLanguage   *string                   `json:"primaryLanguage"`
    SecondaryLanguage *string                   `json:"secondaryLanguage"`
    Timezone          *string                   `json:"timezone"`
    PrimaryLocation   *string                   `json:"primaryLocation"`
    JobRole           *string                   `json:"jobRole"`
    UserType          *string                   `json:"userType"`
    ProfileInfo       *string                   `json:"profileInfo"`
    Enabled           *bool                     `json:"enabled"`
}

// UpdateProfileResponse represents API response for profile update
type UpdateProfileResponse struct {
    Success       bool            `json:"success"`
    Profile       ProfileResponse `json:"profile"`
    UpdatedFields []string        `json:"updatedFields"` // List of fields that were updated
}
```

**Example Request (partial update):**
```json
{
  "timezone": "America/New_York",
  "jobRole": "Engineering Manager"
}
```

**Example Response:**
```json
{
  "success": true,
  "profile": {
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "timezone": "America/New_York",
    "jobRole": "Engineering Manager",
    ...
  },
  "updatedFields": ["timezone", "jobRole"]
}
```

---

### 5. BotConfigHandler

**File:** `internal/adapter/rest/bot_handler.go`

```go
// BotConfigHandler handles HTTP requests for bot configuration management.
type BotConfigHandler struct {
    service *usecase.BotManagementService
    auth    *AuthMiddleware
    rbac    *RBACMiddleware
}
```

**Endpoints:**
- `GET /api/v1/admin/bots/slack` - List Slack bots
- `GET /api/v1/admin/bots/telegram` - List Telegram bots
- `GET /api/v1/admin/bots/:id` - Get bot
- `POST /api/v1/admin/bots/slack` - Create Slack bot
- `POST /api/v1/admin/bots/telegram` - Create Telegram bot
- `PUT /api/v1/admin/bots/:id` - Update bot
- `DELETE /api/v1/admin/bots/:id` - Delete bot
- `POST /api/v1/admin/bots/:id/enable` - Enable bot
- `POST /api/v1/admin/bots/:id/disable` - Disable bot

---

### 6. CreateSlackBotRequest (API Request)

**File:** `internal/adapter/rest/types.go`

```go
// CreateSlackBotRequest represents API request for creating a Slack bot
type CreateSlackBotRequest struct {
    BotName        string   `json:"botName" binding:"required"`
    BotType        string   `json:"botType" binding:"required"` // "public" or "private"
    OwnerUserID    string   `json:"ownerUserID"`                // Required for private
    AllowedUserIDs []string `json:"allowedUserIDs"`             // Required for public
    Enabled        bool     `json:"enabled"`

    // Slack-specific fields
    BotToken      string `json:"botToken" binding:"required"`
    AppToken      string `json:"appToken" binding:"required"`
    SigningSecret string `json:"signingSecret" binding:"required"`
    TeamID        string `json:"teamID" binding:"required"`
    BotUserID     string `json:"botUserID" binding:"required"`
}

// CreateTelegramBotRequest represents API request for creating a Telegram bot
type CreateTelegramBotRequest struct {
    BotName        string   `json:"botName" binding:"required"`
    BotType        string   `json:"botType" binding:"required"`
    OwnerUserID    string   `json:"ownerUserID"`
    AllowedUserIDs []string `json:"allowedUserIDs"`
    Enabled        bool     `json:"enabled"`

    // Telegram-specific fields
    BotToken       string   `json:"botToken" binding:"required"`
    BotUsername    string   `json:"botUsername" binding:"required"`
    BotID          string   `json:"botID" binding:"required"`
    AllowedChatIDs []string `json:"allowedChatIDs"`
}
```

**Example Request (Slack Private Bot):**
```json
{
  "botName": "Alice Personal Bot",
  "botType": "private",
  "ownerUserID": "user-123",
  "enabled": true,
  "botToken": "xoxb-123456789-abcdefghijk",
  "appToken": "xapp-1-ABC123-xyz",
  "signingSecret": "a1b2c3d4e5f6",
  "teamID": "T01ABC123",
  "botUserID": "B01DEF456"
}
```

---

### 7. BotResponse (API Response)

**File:** `internal/adapter/rest/types.go`

```go
// BotResponse represents API response for bot configuration
type BotResponse struct {
    BotID          string        `json:"botID"`
    BotName        string        `json:"botName"`
    BotType        string        `json:"botType"`
    Platform       string        `json:"platform"` // "slack" or "telegram"
    OwnerUserID    string        `json:"ownerUserID"`
    Enabled        bool          `json:"enabled"`
    AllowedUserIDs []string      `json:"allowedUserIDs"`
    CreatedAt      string        `json:"createdAt"` // RFC3339
    UpdatedAt      string        `json:"updatedAt"` // RFC3339

    // Platform-specific (only one will be present)
    Slack    *SlackBotResponse    `json:"slack,omitempty"`
    Telegram *TelegramBotResponse `json:"telegram,omitempty"`
}

// SlackBotResponse represents Slack bot details in API response
// NOTE: Tokens are NOT included in response for security
type SlackBotResponse struct {
    TeamID     string `json:"teamID"`
    BotUserID  string `json:"botUserID"`
    HasToken   bool   `json:"hasToken"`   // True if token is set
    HasAppToken bool  `json:"hasAppToken"` // True if app token is set
}

// TelegramBotResponse represents Telegram bot details in API response
type TelegramBotResponse struct {
    BotUsername    string   `json:"botUsername"`
    BotID          string   `json:"botID"`
    AllowedChatIDs []string `json:"allowedChatIDs"`
    HasToken       bool     `json:"hasToken"` // True if token is set
}
```

**Example Response:**
```json
{
  "botID": "bot-550e8400-e29b-41d4-a716-446655440000",
  "botName": "Alice Personal Bot",
  "botType": "private",
  "platform": "slack",
  "ownerUserID": "user-123",
  "enabled": true,
  "slack": {
    "teamID": "T01ABC123",
    "botUserID": "B01DEF456",
    "hasToken": true,
    "hasAppToken": true
  },
  "createdAt": "2026-02-08T12:00:00Z",
  "updatedAt": "2026-02-08T12:00:00Z"
}
```

---

### 8. ConfigHandler

**File:** `internal/adapter/rest/config_handler.go`

```go
// ConfigHandler handles HTTP requests for configuration management.
type ConfigHandler struct {
    service *usecase.ConfigurationService
    auth    *AuthMiddleware
    rbac    *RBACMiddleware
}
```

**Endpoints:**
- `GET /api/v1/admin/config` - Get current config
- `PUT /api/v1/admin/config` - Update config
- `POST /api/v1/admin/config/reload` - Trigger reload
- `POST /api/v1/admin/config/validate` - Validate config
- `GET /api/v1/admin/config/llm` - Get LLM config
- `PUT /api/v1/admin/config/llm` - Update LLM config
- `GET /api/v1/admin/config/server` - Get server config
- `PUT /api/v1/admin/config/server` - Update server config

---

### 9. ServerHandler

**File:** `internal/adapter/rest/server_handler.go`

```go
// ServerHandler handles HTTP requests for server control.
type ServerHandler struct {
    service *usecase.ServerControlService
    auth    *AuthMiddleware
}
```

**Endpoints:**
- `GET /api/v1/admin/status` - Server status
- `GET /api/v1/admin/metrics` - Server metrics
- `GET /api/v1/admin/logs` - Activity logs

---

### 10. StatusResponse (API Response)

**File:** `internal/adapter/rest/types.go`

```go
// StatusResponse represents server status
type StatusResponse struct {
    Status            string            `json:"status"` // "running", "starting", "stopping"
    Version           string            `json:"version"`
    Uptime            int64             `json:"uptime"` // Seconds
    MemoryUsageMB     int               `json:"memoryUsageMB"`
    ActiveConnections ConnectionStatus  `json:"activeConnections"`
    QuickStats        QuickStats        `json:"quickStats"`
}

// ConnectionStatus represents active gateway connections
type ConnectionStatus struct {
    SlackBots    int `json:"slackBots"`    // Number of connected Slack bots
    TelegramBots int `json:"telegramBots"` // Number of connected Telegram bots
    CLISessions  int `json:"cliSessions"`  // Number of active CLI sessions
}

// QuickStats represents quick statistics
type QuickStats struct {
    TotalUsers       int `json:"totalUsers"`
    ActiveBots       int `json:"activeBots"`
    LLMRequestsLast24h int `json:"llmRequestsLast24h"`
    ErrorRate        float64 `json:"errorRate"` // Percentage
}
```

**Example Response:**
```json
{
  "status": "running",
  "version": "1.0.0",
  "uptime": 86400,
  "memoryUsageMB": 256,
  "activeConnections": {
    "slackBots": 2,
    "telegramBots": 1,
    "cliSessions": 5
  },
  "quickStats": {
    "totalUsers": 25,
    "activeBots": 3,
    "llmRequestsLast24h": 1250,
    "errorRate": 0.5
  }
}
```

---

## Adapter Layer - Web UI

Location: `internal/adapter/web/`

### 1. DashboardData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// DashboardData represents data for dashboard page template
type DashboardData struct {
    ServerStatus      ServerStatus
    ActiveConnections ConnectionStatus
    RecentActivity    []ActivityLogEntry
    QuickStats        QuickStats
}

// ServerStatus represents server status for dashboard
type ServerStatus struct {
    Status        string    // "running", "starting"
    Version       string
    Uptime        string    // Human-readable (e.g., "2 days, 3 hours")
    MemoryUsage   string    // Human-readable (e.g., "256 MB")
    LastReload    time.Time
}

// ActivityLogEntry represents a single activity log entry
type ActivityLogEntry struct {
    Timestamp time.Time
    User      string // Username
    Action    string // Human-readable action description
}
```

---

### 2. UserManagementData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// UserManagementData represents data for user management page
type UserManagementData struct {
    Users       []UserListItem
    TotalCount  int
    CurrentPage int
    TotalPages  int
    SearchTerm  string
    Filters     UserFilters
}

// UserListItem represents a user in the list view
type UserListItem struct {
    UserID       string
    Username     string
    FullName     string // FirstName + LastName
    Email        string
    Role         string
    UserType     string
    Enabled      bool
    PlatformIDs  map[string]string // "slack" -> "U01ABC123"
}

// UserFilters represents filter options
type UserFilters struct {
    UserType string // "", "individual", "enterprise", "developer", "admin"
    Enabled  string // "", "true", "false"
    Role     string // "", "user", "admin"
}
```

---

### 3. UserFormData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// UserFormData represents data for user create/edit form
type UserFormData struct {
    Mode string // "create" or "edit"
    User *UserProfile // Existing user (for edit), nil for create

    // Dropdown options
    UserTypeOptions       []SelectOption
    LanguageOptions       []SelectOption
    TimezoneOptions       []SelectOption
    CommunicationStyleOptions []SelectOption
    VerbosityOptions      []SelectOption
    ResponseFormatOptions []SelectOption

    // Validation errors
    Errors map[string]string
}

// SelectOption represents a dropdown option
type SelectOption struct {
    Value    string
    Label    string
    Selected bool
}
```

---

### 4. BotManagementData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// BotManagementData represents data for bot management page
type BotManagementData struct {
    SlackBots    []BotListItem
    TelegramBots []BotListItem
    Users        []UserOption // For owner/allowed users dropdowns
}

// BotListItem represents a bot in the list view
type BotListItem struct {
    BotID       string
    BotName     string
    BotType     string // "public" or "private"
    Owner       string // Owner username (for private bots)
    Enabled     bool
    Status      string // "connected", "disconnected", "error"
}

// UserOption represents a user in dropdown
type UserOption struct {
    UserID   string
    Username string
    FullName string
}
```

---

### 5. LLMConfigData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// LLMConfigData represents data for LLM configuration page
type LLMConfigData struct {
    DefaultModel    DefaultModelConfig
    Providers       ProvidersConfig
    ModelInstances  []ModelInstance
}

// DefaultModelConfig represents default model selection
type DefaultModelConfig struct {
    Primary   string // Provider ID
    Secondary string
    Tertiary  string
    Options   []SelectOption // Available provider IDs
}

// ProvidersConfig represents provider-specific configurations
type ProvidersConfig struct {
    Anthropic AnthropicConfig
    OpenAI    OpenAIConfig
    Ollama    OllamaConfig
    Bedrock   BedrockConfig
}

// ModelInstance represents a configured model instance
type ModelInstance struct {
    ID          string
    Type        string // Provider type
    ModelName   string
    Enabled     bool
    HasOverrides bool  // True if has overridden api_key/base_url
}
```

---

### 6. ServerConfigData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// ServerConfigData represents data for server configuration page
type ServerConfigData struct {
    Paths    PathsConfig
    Logging  LoggingConfig
    Gateways GatewaysConfig
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
    LogLevel string
    Debug    bool
    LogLevelOptions []SelectOption
}

// GatewaysConfig represents gateway enable/disable flags
type GatewaysConfig struct {
    CLI      bool
    Slack    bool
    Telegram bool
}
```

---

### 7. LogViewerData (Template Data)

**File:** `internal/adapter/web/types.go`

```go
// LogViewerData represents data for activity log viewer page
type LogViewerData struct {
    Logs        []AuditLogEntry
    TotalCount  int
    CurrentPage int
    TotalPages  int
    Filters     LogFilters
}

// AuditLogEntry represents an audit log entry for display
type AuditLogEntry struct {
    Timestamp   string // Human-readable timestamp
    User        string // Admin username
    Action      string // Human-readable action description
    ResourceType string // "user", "bot", "config"
    ResourceName string // User/bot name
}

// LogFilters represents filter options for logs
type LogFilters struct {
    TimeRange string // "1h", "24h", "7d", "30d", "custom"
    Resource  string // "", "user", "bot", "config", "server"
    User      string // "", or specific user ID
    Search    string // Text search
}
```

---

## Adapter Layer - CLI

Location: `internal/adapter/cli/`

### 1. AdminUserCommand

**File:** `internal/adapter/cli/admin_user_command.go`

```go
// AdminUserCommand provides CLI commands for user management
type AdminUserCommand struct {
    service *usecase.UserProfileService
}
```

**Commands:**
```bash
nuimanbot admin user create --username alice --email alice@example.com [--first-name Alice] [--last-name Anderson] [...]
nuimanbot admin user list [--format table|json|csv] [--filter user-type=enterprise] [--search alice]
nuimanbot admin user view <user-id>
nuimanbot admin user update <user-id> --timezone America/New_York [--job-role "Engineering Manager"]
nuimanbot admin user delete <user-id>
nuimanbot admin user import --file users.json
nuimanbot admin user export --file users-export.json [--format json|csv]
nuimanbot admin user link-platform <user-id> --platform slack --id U01ABC123
nuimanbot admin user unlink-platform <user-id> --platform slack
```

---

### 2. AdminBotCommand

**File:** `internal/adapter/cli/admin_bot_command.go`

```go
// AdminBotCommand provides CLI commands for bot management
type AdminBotCommand struct {
    service *usecase.BotManagementService
}
```

**Commands:**
```bash
nuimanbot admin bot slack create --name "Team Bot" --type public --bot-token xoxb-... --app-token xapp-... [...]
nuimanbot admin bot slack list [--format table|json]
nuimanbot admin bot slack view <bot-id>
nuimanbot admin bot slack update <bot-id> --enabled true
nuimanbot admin bot slack delete <bot-id>
nuimanbot admin bot slack enable <bot-id>
nuimanbot admin bot slack disable <bot-id>
nuimanbot admin bot slack allow-user <bot-id> <user-id>
nuimanbot admin bot slack disallow-user <bot-id> <user-id>

# Similar commands for telegram
nuimanbot admin bot telegram create [...]
nuimanbot admin bot telegram list
[...]
```

---

### 3. ServerControlCommand

**File:** `internal/adapter/cli/server_command.go`

```go
// ServerControlCommand provides CLI commands for server control
type ServerControlCommand struct {
    service *usecase.ConfigurationService
}
```

**Commands:**
```bash
nuimanbot server reload                          # Trigger config reload
nuimanbot server status                          # Show server status
nuimanbot server logs [--follow] [--level info]  # View logs
nuimanbot server metrics                         # Show metrics
```

---

### 4. ConfigCommand

**File:** `internal/adapter/cli/config_command.go`

```go
// ConfigCommand provides CLI commands for configuration management
type ConfigCommand struct {
    service *usecase.ConfigurationService
}
```

**Commands:**
```bash
nuimanbot config get <key>                       # Get config value
nuimanbot config set <key> <value>               # Set config value
nuimanbot config validate                        # Validate config file
nuimanbot config migrate --input old.yaml --output new.yaml  # Migrate config
nuimanbot config show                            # Show full config
```

---

## Configuration

Location: `internal/config/`

### Server Configuration (YAML)

**File:** `config/config.yaml`

```yaml
server:
  environment: development          # development, production, testing
  log_level: info                   # debug, info, warn, error
  debug: false
  admin_port: 8080                  # Web admin interface port
  paths:
    config: "./config/"             # Configuration files directory
    data: "./data/"                 # Data files directory (users.json, bots.json)
    logs: "./logs/"                 # Log files directory

gateways:
  cli:
    enabled: true
    debug_mode: false
    history_file: ".nuimanbot_history"

  slack:
    enabled: false                  # Enable/disable Slack gateway
    # Bot configurations now in bots.json

  telegram:
    enabled: false                  # Enable/disable Telegram gateway
    # Bot configurations now in bots.json

llm:
  default_model:
    primary: anthropic-main         # Provider ID (not type/name)
    secondary: anthropic-fast       # Fallback 1
    tertiary: openai-gpt4           # Fallback 2

  # Provider-specific default configurations (inheritance base)
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    base_url: "https://api.anthropic.com"

  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    organization: ""

  ollama:
    base_url: "http://localhost:11434"

  bedrock:
    aws_region: "us-east-1"
    aws_profile: ""
    max_retries: 3

  # Provider instances (inherit from type-specific sections)
  providers:
    - id: anthropic-main
      type: anthropic
      enabled: true
      model_name: claude-3-5-sonnet-20241022
      # api_key and base_url inherited from anthropic section

    - id: anthropic-fast
      type: anthropic
      enabled: true
      model_name: claude-3-haiku-20240307
      # Inherits from anthropic section

    - id: openai-gpt4
      type: openai
      enabled: true
      model_name: gpt-4-turbo-preview
      # Inherits from openai section

    - id: ollama-local
      type: ollama
      enabled: false
      model_name: llama2
      # Inherits from ollama section
```

**Environment Variables:**
```bash
# Server paths
NUIMANBOT_SERVER_PATHS_CONFIG="/etc/nuimanbot/config/"
NUIMANBOT_SERVER_PATHS_DATA="/var/lib/nuimanbot/"
NUIMANBOT_SERVER_PATHS_LOGS="/var/log/nuimanbot/"

# Admin port
NUIMANBOT_SERVER_ADMIN_PORT=8080

# Gateway flags
NUIMANBOT_GATEWAYS_CLI_ENABLED=true
NUIMANBOT_GATEWAYS_SLACK_ENABLED=false
NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED=false

# Default models
NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY="anthropic-main"
NUIMANBOT_LLM_DEFAULTMODEL_SECONDARY="anthropic-fast"
NUIMANBOT_LLM_DEFAULTMODEL_TERTIARY="openai-gpt4"

# Provider credentials
ANTHROPIC_API_KEY="sk-ant-..."
OPENAI_API_KEY="sk-..."

# Encryption key (AES-256, 32 bytes = 64 hex chars)
NUIMANBOT_ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
```

---

## Type Aliases & Enums

### Enumerations

#### UserType

```go
// UserType represents the type of user account
type UserType string

const (
    UserTypeIndividual UserType = "individual" // Personal use
    UserTypeEnterprise UserType = "enterprise" // Company/organization use
    UserTypeDeveloper  UserType = "developer"  // Developer account with API access
    UserTypeAdmin      UserType = "admin"      // Administrative account
)

// String returns the string representation
func (ut UserType) String() string {
    return string(ut)
}

// IsValid checks if user type is valid
func (ut UserType) IsValid() bool {
    switch ut {
    case UserTypeIndividual, UserTypeEnterprise, UserTypeDeveloper, UserTypeAdmin:
        return true
    default:
        return false
    }
}
```

---

#### BotType

```go
// BotType represents the type of bot (public or private)
type BotType string

const (
    BotTypePublic  BotType = "public"  // Shared bot accessible to multiple users
    BotTypePrivate BotType = "private" // User-owned bot for personal use
)

// String returns the string representation
func (bt BotType) String() string {
    return string(bt)
}

// IsValid checks if bot type is valid
func (bt BotType) IsValid() bool {
    return bt == BotTypePublic || bt == BotTypePrivate
}
```

---

#### Platform

```go
// Platform represents a messaging platform
type Platform string

const (
    PlatformCLI      Platform = "cli"      // Command-line interface
    PlatformSlack    Platform = "slack"    // Slack
    PlatformTelegram Platform = "telegram" // Telegram
)

// String returns the string representation
func (p Platform) String() string {
    return string(p)
}

// IsValid checks if platform is valid
func (p Platform) IsValid() bool {
    switch p {
    case PlatformCLI, PlatformSlack, PlatformTelegram:
        return true
    default:
        return false
    }
}
```

---

#### CommunicationStyle

```go
// CommunicationStyle represents the agent's communication style preference
type CommunicationStyle string

const (
    CommunicationStyleProfessional CommunicationStyle = "professional" // Formal, business-like
    CommunicationStyleCasual       CommunicationStyle = "casual"       // Informal, friendly
    CommunicationStyleTechnical    CommunicationStyle = "technical"    // Precise, technical jargon
    CommunicationStyleFriendly     CommunicationStyle = "friendly"     // Warm, personable
)

// String returns the string representation
func (cs CommunicationStyle) String() string {
    return string(cs)
}

// IsValid checks if communication style is valid
func (cs CommunicationStyle) IsValid() bool {
    switch cs {
    case CommunicationStyleProfessional, CommunicationStyleCasual,
         CommunicationStyleTechnical, CommunicationStyleFriendly:
        return true
    default:
        return false
    }
}
```

---

#### Verbosity

```go
// Verbosity represents the level of detail in agent responses
type Verbosity string

const (
    VerbosityConcise  Verbosity = "concise"  // Brief, to-the-point responses
    VerbosityModerate Verbosity = "moderate" // Balanced detail
    VerbosityDetailed Verbosity = "detailed" // Comprehensive, thorough responses
)

// String returns the string representation
func (v Verbosity) String() string {
    return string(v)
}

// IsValid checks if verbosity is valid
func (v Verbosity) IsValid() bool {
    return v == VerbosityConcise || v == VerbosityModerate || v == VerbosityDetailed
}
```

---

#### ResponseFormat

```go
// ResponseFormat represents the format of agent responses
type ResponseFormat string

const (
    ResponseFormatMarkdown   ResponseFormat = "markdown"   // Markdown formatting
    ResponseFormatPlain      ResponseFormat = "plain"      // Plain text, no formatting
    ResponseFormatStructured ResponseFormat = "structured" // Structured data (JSON, tables)
)

// String returns the string representation
func (rf ResponseFormat) String() string {
    return string(rf)
}

// IsValid checks if response format is valid
func (rf ResponseFormat) IsValid() bool {
    switch rf {
    case ResponseFormatMarkdown, ResponseFormatPlain, ResponseFormatStructured:
        return true
    default:
        return false
    }
}
```

---

### Type Aliases

```go
// Gateway represents a gateway identifier
type Gateway string

const (
    GatewayCLI      Gateway = "cli"
    GatewaySlack    Gateway = "slack"
    GatewayTelegram Gateway = "telegram"
)
```

---

## File Storage Schema

### users.json Structure

**File:** `data/users.json`

```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-08T12:00:00Z",
  "users": [
    {
      "userID": "550e8400-e29b-41d4-a716-446655440000",
      "moniker": "alice_admin",
      "firstName": "Alice",
      "lastName": "Anderson",
      "nickName": "Ally",
      "primaryEmail": "alice@example.com",
      "backupEmail": "alice.anderson@personal.com",
      "mobilePhone": "+14155552671",
      "primaryLanguage": "en",
      "secondaryLanguage": "",
      "timezone": "America/Los_Angeles",
      "primaryLocation": "San Francisco, CA",
      "jobRole": "Engineering Manager",
      "userType": "enterprise",
      "platformIDs": {
        "cli": "alice",
        "slack": "U01ABC123",
        "telegram": "123456789"
      },
      "agentPreferences": {
        "communicationStyle": "professional",
        "verbosity": "moderate",
        "responseFormat": "markdown",
        "codeExamplesPreferred": true,
        "explainDecisions": false,
        "proactiveMode": true,
        "skillDefaults": {
          "commit": {
            "autoExecute": false,
            "options": {
              "autoStage": true,
              "signoff": true
            }
          }
        },
        "notificationPreferences": {
          "taskCompletion": true,
          "errors": true,
          "longRunningOps": true
        }
      },
      "notesInformation": {
        "adminNotes": [
          {
            "timestamp": "2026-02-07T10:30:00Z",
            "authorID": "admin-001",
            "note": "User requested enterprise features access"
          }
        ],
        "flags": {
          "betaTester": true,
          "earlyAccess": false
        },
        "supportTickets": ["TICKET-123", "TICKET-456"],
        "restrictions": {
          "rateLimitOverride": null,
          "featureAccess": ["bedrock", "advanced-tools"],
          "blockedFeatures": []
        },
        "customMetadata": {
          "department": "Engineering",
          "project": "AI Integration"
        }
      },
      "profileInfo": "Experienced engineering manager focusing on AI integration.",
      "enabled": true,
      "dataDirectory": "users/550e8400-e29b-41d4-a716-446655440000",
      "createdAt": "2026-01-15T09:00:00Z",
      "updatedAt": "2026-02-08T12:00:00Z",
      "lastVerified": "2026-02-08T12:00:00Z"
    }
  ],
  "indexes": {
    "byUsername": {
      "alice_admin": "550e8400-e29b-41d4-a716-446655440000"
    },
    "byEmail": {
      "alice@example.com": "550e8400-e29b-41d4-a716-446655440000"
    },
    "byPlatform": {
      "slack": {
        "U01ABC123": "550e8400-e29b-41d4-a716-446655440000"
      },
      "telegram": {
        "123456789": "550e8400-e29b-41d4-a716-446655440000"
      },
      "cli": {
        "alice": "550e8400-e29b-41d4-a716-446655440000"
      }
    }
  }
}
```

**Indexes:**
- `byUsername`: Map of username → userID for O(1) lookup by username
- `byEmail`: Map of email → userID for O(1) lookup by email
- `byPlatform`: Nested map of platform → platformID → userID for O(1) platform routing

**Atomic Write Pattern:**
1. Write to temp file: `users.json.tmp`
2. Acquire file lock: `users.json.lock`
3. Rename temp file to `users.json` (atomic on POSIX systems)
4. Release file lock

---

### bots.json Structure

**File:** `data/bots.json`

```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-08T12:00:00Z",
  "slackBots": [
    {
      "botID": "bot-550e8400-e29b-41d4-a716-446655440000",
      "botName": "Alice Personal Bot",
      "botType": "private",
      "ownerUserID": "user-123",
      "slackBotToken": "ENC:base64encodedencryptedtoken",
      "slackAppToken": "ENC:base64encodedencryptedtoken",
      "slackSigningSecret": "ENC:base64encodedencryptedtoken",
      "slackTeamID": "T01ABC123",
      "slackBotUserID": "B01DEF456",
      "enabled": true,
      "allowedUserIDs": null,
      "metadata": {},
      "createdAt": "2026-02-08T10:00:00Z",
      "updatedAt": "2026-02-08T10:00:00Z"
    },
    {
      "botID": "bot-660f9511-f3ac-52e5-b827-557766551111",
      "botName": "Team Assistant",
      "botType": "public",
      "ownerUserID": null,
      "slackBotToken": "ENC:base64encodedencryptedtoken",
      "slackAppToken": "ENC:base64encodedencryptedtoken",
      "slackSigningSecret": "ENC:base64encodedencryptedtoken",
      "slackTeamID": "T01ABC123",
      "slackBotUserID": "B01GHI789",
      "enabled": true,
      "allowedUserIDs": ["user-123", "user-456", "user-789"],
      "metadata": {
        "description": "Shared team assistant bot"
      },
      "createdAt": "2026-02-07T14:00:00Z",
      "updatedAt": "2026-02-08T12:00:00Z"
    }
  ],
  "telegramBots": [
    {
      "botID": "bot-770e9622-g4bd-63f6-c938-668877662222",
      "botName": "Bob Personal Assistant",
      "botType": "private",
      "ownerUserID": "user-456",
      "telegramBotToken": "ENC:base64encodedencryptedtoken",
      "telegramBotUsername": "bob_personal_bot",
      "telegramBotID": "987654321",
      "enabled": true,
      "allowedUserIDs": null,
      "allowedChatIDs": ["123456789"],
      "metadata": {},
      "createdAt": "2026-02-08T11:00:00Z",
      "updatedAt": "2026-02-08T11:00:00Z"
    }
  ],
  "indexes": {
    "slackByName": {
      "Alice Personal Bot": "bot-550e8400-e29b-41d4-a716-446655440000",
      "Team Assistant": "bot-660f9511-f3ac-52e5-b827-557766551111"
    },
    "slackByOwner": {
      "user-123": ["bot-550e8400-e29b-41d4-a716-446655440000"]
    },
    "slackByBotUserID": {
      "B01DEF456": "bot-550e8400-e29b-41d4-a716-446655440000",
      "B01GHI789": "bot-660f9511-f3ac-52e5-b827-557766551111"
    },
    "telegramByName": {
      "Bob Personal Assistant": "bot-770e9622-g4bd-63f6-c938-668877662222"
    },
    "telegramByOwner": {
      "user-456": ["bot-770e9622-g4bd-63f6-c938-668877662222"]
    },
    "telegramByBotID": {
      "987654321": "bot-770e9622-g4bd-63f6-c938-668877662222"
    }
  }
}
```

**Token Encryption Format:**
- Prefix: `ENC:`
- Content: Base64-encoded encrypted token
- Encryption: AES-256-GCM
- Structure: `base64(nonce + encrypted_data + auth_tag)`

**Indexes:**
- `slackByName`: Map of bot name → botID
- `slackByOwner`: Map of ownerUserID → array of botIDs
- `slackByBotUserID`: Map of Slack bot user ID → botID
- Similar indexes for Telegram bots

---

### User Directory Structure

**Directory:** `data/users/<user-id>/`

```
users/550e8400-e29b-41d4-a716-446655440000/
├── profile.json              # Full profile (redundant copy of users.json entry)
├── preferences.json          # Agent preferences (redundant)
├── todos.json                # User's todo list
├── repeated-actions.json     # Repeated actions/macros
├── history.json              # Command/conversation history
└── attachments/              # User-uploaded files
    └── <attachment-id>/
```

**profile.json:** Redundant copy of user profile for backup/recovery
**preferences.json:** Redundant copy of agent preferences
**todos.json:** User-specific todo list (future feature)
**repeated-actions.json:** Saved action sequences (future feature)
**history.json:** User command/conversation history

---

## Constants

### Error Messages

```go
const (
    // User Profile Errors
    ErrMsgUserProfileNotFound      = "user profile not found: %s"
    ErrMsgUserProfileAlreadyExists = "user profile already exists: %s"
    ErrMsgInvalidEmail             = "invalid email format: %s"
    ErrMsgInvalidPhoneNumber       = "invalid phone number format: %s"
    ErrMsgInvalidTimezone          = "invalid timezone: %s"
    ErrMsgInvalidLanguageCode      = "invalid ISO 639-1 language code: %s"
    ErrMsgPlatformIDAlreadyLinked  = "platform ID already linked to another user: %s"

    // Bot Config Errors
    ErrMsgBotNotFound              = "bot configuration not found: %s"
    ErrMsgBotAlreadyExists         = "bot configuration already exists: %s"
    ErrMsgInvalidBotType           = "invalid bot type, must be 'public' or 'private'"
    ErrMsgInvalidBotToken          = "invalid bot token format"
    ErrMsgEncryptionFailed         = "failed to encrypt bot token: %v"
    ErrMsgDecryptionFailed         = "failed to decrypt bot token: %v"
    ErrMsgBotNotEnabled            = "bot is disabled: %s"

    // Configuration Errors
    ErrMsgConfigInvalid            = "configuration validation failed: %v"
    ErrMsgConfigReloadFailed       = "configuration reload failed: %v"
    ErrMsgConfigFileNotFound       = "configuration file not found: %s"
    ErrMsgConfigWriteFailed        = "failed to write configuration: %v"

    // File Storage Errors
    ErrMsgFileReadFailed           = "failed to read file: %v"
    ErrMsgFileWriteFailed          = "failed to write file: %v"
    ErrMsgFileLockTimeout          = "file lock acquisition timeout"
    ErrMsgConcurrentModification   = "concurrent modification detected, retry"

    // Audit Log Errors
    ErrMsgAuditLogWriteFailed      = "failed to write audit log: %v"

    // Authentication/Authorization Errors
    ErrMsgUnauthorized             = "unauthorized: invalid credentials"
    ErrMsgForbidden                = "forbidden: insufficient permissions"
    ErrMsgInvalidAPIKey            = "invalid API key"
    ErrMsgSessionExpired           = "session expired, please log in again"
)
```

---

### Limits and Thresholds

```go
const (
    // Profile Limits
    MaxMonikerLength         = 50
    MaxNameLength            = 100
    MaxEmailLength           = 254 // RFC 5321
    MaxPhoneLength           = 20
    MaxLocationLength        = 100
    MaxJobRoleLength         = 100
    MaxProfileInfoLength     = 2000
    MaxAdminNoteLength       = 1000

    // Bot Limits
    MaxBotNameLength         = 100
    MaxAllowedUsers          = 1000 // Max users for public bot
    MaxBotsPerUser           = 50   // Max private bots per user

    // API Limits
    DefaultPageSize          = 50
    MaxPageSize              = 500
    MinPageSize              = 1

    // Rate Limits
    DefaultRateLimit         = 100  // requests per minute
    AdminRateLimit           = 1000 // requests per minute for admins
    AuthRateLimit            = 10   // login attempts per minute

    // File Limits
    MaxFileSizeMB            = 100  // Max file size for uploads
    MaxFileAge               = 90   // Days to retain old files

    // Cache TTL
    CacheTTLMinutes          = 5    // In-memory cache TTL
    ConfigCacheTTLSeconds    = 60   // Config cache TTL
)
```

---

### Timeouts and Durations

```go
const (
    // Operation Timeouts
    DefaultOperationTimeout  = 30 * time.Second
    MaxOperationTimeout      = 5 * time.Minute
    FileOperationTimeout     = 10 * time.Second
    DatabaseOperationTimeout = 30 * time.Second
    HTTPClientTimeout        = 30 * time.Second
    ConfigReloadTimeout      = 10 * time.Second

    // File Locking
    FileLockTimeout          = 5 * time.Second
    FileLockRetryInterval    = 100 * time.Millisecond

    // Session
    SessionTimeout           = 24 * time.Hour // Web interface session timeout
    SessionCleanupInterval   = 1 * time.Hour  // Cleanup expired sessions

    // Config Watcher
    ConfigDebounceDelay      = 1 * time.Second // Debounce file change events
)
```

---

## Type Mapping Reference

### Domain → File Storage (JSON)

| Domain Type | JSON Type | Conversion |
|-------------|-----------|------------|
| `string` | `string` | Direct |
| `int` | `number` | Direct |
| `bool` | `boolean` | Direct |
| `time.Time` | `string` | RFC3339 format (`2006-01-02T15:04:05Z07:00`) |
| `[]string` | `array` | Direct |
| `map[string]any` | `object` | Direct |
| `UserType` (enum) | `string` | `.String()` method |
| `BotType` (enum) | `string` | `.String()` method |
| `Platform` (enum) | `string` | `.String()` method |
| Encrypted token | `string` | `"ENC:" + base64(nonce+encrypted+tag)` |

---

### Domain → API (JSON)

| Domain Type | JSON Type | Conversion |
|-------------|-----------|------------|
| `string` | `string` | Direct |
| `int` | `number` | Direct |
| `bool` | `boolean` | Direct |
| `time.Time` | `string` | RFC3339 format |
| `[]string` | `array` | Direct |
| `map[string]any` | `object` | Direct |
| Enums | `string` | `.String()` method |
| Encrypted token | NOT exposed | Omitted for security (use `hasToken` bool) |

---

### API → Domain

| JSON Type | Domain Type | Validation |
|-----------|-------------|------------|
| `string` | `string` | Length, pattern checks |
| `number` | `int` | Range checks |
| `boolean` | `bool` | Direct |
| `string` (timestamp) | `time.Time` | Parse RFC3339, validate not zero |
| `array` | `[]string` | Validate each element |
| `object` | `map[string]any` | Validate keys/values |
| `string` (enum) | Enum type | Parse and validate against constants |

---

## Validation Patterns

### Email Validation

```go
const EmailPattern = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
```

### Phone Number Validation (E.164)

```go
const E164PhonePattern = `^\+[1-9]\d{1,14}$`
```

### Slack User ID Pattern

```go
const SlackUserIDPattern = `^[UW][A-Z0-9]{8,10}$`
```

### Slack Bot ID Pattern

```go
const SlackBotIDPattern = `^B[A-Z0-9]{8,10}$`
```

### Slack Team ID Pattern

```go
const SlackTeamIDPattern = `^T[A-Z0-9]{8,10}$`
```

### Slack Bot Token Pattern

```go
const SlackBotTokenPattern = `^xoxb-[a-zA-Z0-9-]+$`
```

### Slack App Token Pattern

```go
const SlackAppTokenPattern = `^xapp-[a-zA-Z0-9-]+$`
```

### Telegram Bot Token Pattern

```go
const TelegramBotTokenPattern = `^[0-9]+:[a-zA-Z0-9_-]+$`
```

### UUID Pattern

```go
const UUIDPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
```

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-08 | Initial comprehensive data dictionary |

