# Phase 2: Multi-Platform - Data Dictionary

**Last Updated:** 2026-02-06

This document defines all data structures, types, and schemas for Phase 2 components.

---

## 1. Domain Entities (Extensions)

### 1.1. User Entity (Extended)

**Location:** `internal/domain/user.go`

**Existing Structure:**
```go
type User struct {
    ID          string
    Platform    Platform
    PlatformUID string
    Role        Role
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Phase 2 Extensions:**
```go
type User struct {
    ID            string     // Unique user ID (UUID)
    Platform      Platform   // Platform where user originated
    PlatformUID   string     // Platform-specific user ID
    Role          Role       // User role (Admin, User, Guest)
    AllowedSkills []string   // Per-user skill whitelist (empty = all allowed)
    CreatedAt     time.Time  // When user was created
    UpdatedAt     time.Time  // When user was last updated
}
```

**New Field:**
- `AllowedSkills []string` - Optional whitelist of skills this user can execute. Empty means all skills allowed for their role. Useful for restricting specific users.

**Role Enum (No Changes):**
```go
type Role int

const (
    RoleGuest Role = iota  // Limited access
    RoleUser               // Standard access
    RoleAdmin              // Full access
)
```

---

## 2. Configuration Structures (New)

### 2.1. Telegram Gateway Config

**Location:** `internal/config/gateway_config.go`

```go
type TelegramConfig struct {
    Enabled      bool         `mapstructure:"enabled" yaml:"enabled"`
    BotToken     SecureString `mapstructure:"bot_token" yaml:"bot_token"`
    AllowedUsers []string     `mapstructure:"allowed_users" yaml:"allowed_users"` // Empty = all
}
```

**Fields:**
- `Enabled` - Whether Telegram gateway is active
- `BotToken` - Bot token from @BotFather (stored in credential vault)
- `AllowedUsers` - List of allowed Telegram user IDs (empty = all registered users)

**Environment Variables:**
```bash
NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED=true
NUIMANBOT_GATEWAYS_TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
NUIMANBOT_GATEWAYS_TELEGRAM_ALLOWED_USERS=123456789,987654321
```

---

### 2.2. Slack Gateway Config

**Location:** `internal/config/gateway_config.go`

```go
type SlackConfig struct {
    Enabled         bool         `mapstructure:"enabled" yaml:"enabled"`
    BotToken        SecureString `mapstructure:"bot_token" yaml:"bot_token"`
    AppToken        SecureString `mapstructure:"app_token" yaml:"app_token"`
    AllowedChannels []string     `mapstructure:"allowed_channels" yaml:"allowed_channels"` // Empty = all
}
```

**Fields:**
- `Enabled` - Whether Slack gateway is active
- `BotToken` - Bot User OAuth Token (xoxb-...)
- `AppToken` - App-Level Token for Socket Mode (xapp-...)
- `AllowedChannels` - List of allowed channel IDs (empty = all channels)

**Environment Variables:**
```bash
NUIMANBOT_GATEWAYS_SLACK_ENABLED=true
NUIMANBOT_GATEWAYS_SLACK_BOT_TOKEN=xoxb-your-bot-token
NUIMANBOT_GATEWAYS_SLACK_APP_TOKEN=xapp-your-app-token
NUIMANBOT_GATEWAYS_SLACK_ALLOWED_CHANNELS=C01234567,C98765432
```

---

### 2.3. Updated Gateways Config

**Location:** `internal/config/gateway_config.go`

```go
type GatewaysConfig struct {
    CLI      CLIConfig      `mapstructure:"cli" yaml:"cli"`
    Telegram TelegramConfig `mapstructure:"telegram" yaml:"telegram"`
    Slack    SlackConfig    `mapstructure:"slack" yaml:"slack"`
}
```

---

### 2.4. OpenAI Provider Config

**Location:** `internal/config/llm_config.go`

```go
type OpenAIProviderConfig struct {
    Type         string       `mapstructure:"type" yaml:"type"` // "openai"
    APIKey       SecureString `mapstructure:"api_key" yaml:"api_key"`
    DefaultModel string       `mapstructure:"default_model" yaml:"default_model"`
    Organization string       `mapstructure:"organization" yaml:"organization"` // Optional
    BaseURL      string       `mapstructure:"base_url" yaml:"base_url"`         // Optional, for proxies
}
```

**Fields:**
- `Type` - Must be "openai"
- `APIKey` - OpenAI API key (sk-...)
- `DefaultModel` - Model to use (e.g., "gpt-4o", "gpt-4-turbo")
- `Organization` - Optional organization ID
- `BaseURL` - Optional custom endpoint (for Azure OpenAI, etc.)

**Environment Variables:**
```bash
NUIMANBOT_LLM_PROVIDERS_1_TYPE=openai
NUIMANBOT_LLM_PROVIDERS_1_API_KEY=sk-your-openai-key
NUIMANBOT_LLM_PROVIDERS_1_DEFAULT_MODEL=gpt-4o
```

---

### 2.5. Ollama Provider Config

**Location:** `internal/config/llm_config.go`

```go
type OllamaProviderConfig struct {
    Type         string `mapstructure:"type" yaml:"type"` // "ollama"
    BaseURL      string `mapstructure:"base_url" yaml:"base_url"`
    DefaultModel string `mapstructure:"default_model" yaml:"default_model"`
}
```

**Fields:**
- `Type` - Must be "ollama"
- `BaseURL` - Ollama server URL (default: http://localhost:11434)
- `DefaultModel` - Model to use (e.g., "llama3", "mistral")

**Environment Variables:**
```bash
NUIMANBOT_LLM_PROVIDERS_2_TYPE=ollama
NUIMANBOT_LLM_PROVIDERS_2_BASE_URL=http://localhost:11434
NUIMANBOT_LLM_PROVIDERS_2_DEFAULT_MODEL=llama3
```

---

### 2.6. Weather Skill Config

**Location:** `internal/config/skills_config.go`

```go
type WeatherSkillConfig struct {
    Enabled         bool         `mapstructure:"enabled" yaml:"enabled"`
    APIKey          SecureString `mapstructure:"api_key" yaml:"api_key"`
    DefaultLocation string       `mapstructure:"default_location" yaml:"default_location"`
    CacheTTL        int          `mapstructure:"cache_ttl" yaml:"cache_ttl"` // Minutes
}
```

**Fields:**
- `Enabled` - Whether weather skill is active
- `APIKey` - OpenWeatherMap API key
- `DefaultLocation` - Default city if user doesn't specify
- `CacheTTL` - How long to cache weather data (minutes, default: 30)

**Environment Variables:**
```bash
NUIMANBOT_SKILLS_WEATHER_ENABLED=true
NUIMANBOT_SKILLS_WEATHER_API_KEY=your-openweathermap-key
NUIMANBOT_SKILLS_WEATHER_DEFAULT_LOCATION=San Francisco, CA
NUIMANBOT_SKILLS_WEATHER_CACHE_TTL=30
```

---

### 2.7. Web Search Skill Config

**Location:** `internal/config/skills_config.go`

```go
type WebSearchSkillConfig struct {
    Enabled     bool   `mapstructure:"enabled" yaml:"enabled"`
    Provider    string `mapstructure:"provider" yaml:"provider"` // "duckduckgo" or "serpapi"
    APIKey      SecureString `mapstructure:"api_key" yaml:"api_key"` // Only for SerpAPI
    MaxResults  int    `mapstructure:"max_results" yaml:"max_results"` // Default: 5
}
```

**Fields:**
- `Enabled` - Whether web search skill is active
- `Provider` - Search provider ("duckduckgo" or "serpapi")
- `APIKey` - API key (only for SerpAPI, empty for DuckDuckGo)
- `MaxResults` - Maximum search results to return (default: 5)

**Environment Variables:**
```bash
NUIMANBOT_SKILLS_WEB_SEARCH_ENABLED=true
NUIMANBOT_SKILLS_WEB_SEARCH_PROVIDER=duckduckgo
NUIMANBOT_SKILLS_WEB_SEARCH_MAX_RESULTS=5
```

---

### 2.8. Notes Skill Config

**Location:** `internal/config/skills_config.go`

```go
type NotesSkillConfig struct {
    Enabled        bool `mapstructure:"enabled" yaml:"enabled"`
    MaxNoteLength  int  `mapstructure:"max_note_length" yaml:"max_note_length"`   // Default: 10000
    MaxNotesPerUser int  `mapstructure:"max_notes_per_user" yaml:"max_notes_per_user"` // Default: 100
}
```

**Fields:**
- `Enabled` - Whether notes skill is active
- `MaxNoteLength` - Maximum characters per note (default: 10000)
- `MaxNotesPerUser` - Maximum notes per user (default: 100)

**Environment Variables:**
```bash
NUIMANBOT_SKILLS_NOTES_ENABLED=true
NUIMANBOT_SKILLS_NOTES_MAX_NOTE_LENGTH=10000
NUIMANBOT_SKILLS_NOTES_MAX_NOTES_PER_USER=100
```

---

### 2.9. Updated SkillsSystemConfig

**Location:** `internal/config/skills_config.go`

```go
type SkillsSystemConfig struct {
    Calculator SkillConfig          `mapstructure:"calculator" yaml:"calculator"`
    Datetime   SkillConfig          `mapstructure:"datetime" yaml:"datetime"`
    Weather    WeatherSkillConfig   `mapstructure:"weather" yaml:"weather"`
    WebSearch  WebSearchSkillConfig `mapstructure:"web_search" yaml:"web_search"`
    Notes      NotesSkillConfig     `mapstructure:"notes" yaml:"notes"`
}
```

---

## 3. Repository Interfaces (New)

### 3.1. Notes Repository

**Location:** `internal/usecase/notes/repository.go`

```go
type NotesRepository interface {
    // CreateNote creates a new note
    CreateNote(ctx context.Context, note *Note) error

    // GetNote retrieves a note by ID
    GetNote(ctx context.Context, noteID string) (*Note, error)

    // ListNotes retrieves all notes for a user
    ListNotes(ctx context.Context, userID string) ([]*Note, error)

    // UpdateNote updates an existing note
    UpdateNote(ctx context.Context, note *Note) error

    // DeleteNote deletes a note by ID
    DeleteNote(ctx context.Context, noteID string) error

    // CountUserNotes counts notes for a user
    CountUserNotes(ctx context.Context, userID string) (int, error)
}
```

---

### 3.2. Note Entity

**Location:** `internal/domain/note.go`

```go
type Note struct {
    ID        string    // Unique note ID (UUID)
    UserID    string    // Owner user ID
    Title     string    // Note title (max 200 chars)
    Content   string    // Note content (max 10000 chars)
    CreatedAt time.Time // When note was created
    UpdatedAt time.Time // When note was last updated
}
```

**Validation Rules:**
- Title: 1-200 characters, non-empty
- Content: 0-10000 characters (can be empty)
- UserID: Must reference existing user
- ID: UUID v4

---

## 4. Service Interfaces (New)

### 4.1. User Management Service

**Location:** `internal/usecase/user/service.go`

```go
type UserService interface {
    // CreateUser creates a new user
    CreateUser(ctx context.Context, platform Platform, platformUID string, role Role) (*User, error)

    // GetUser retrieves a user by ID
    GetUser(ctx context.Context, userID string) (*User, error)

    // GetUserByPlatformUID retrieves a user by platform+platformUID
    GetUserByPlatformUID(ctx context.Context, platform Platform, platformUID string) (*User, error)

    // ListUsers retrieves all users
    ListUsers(ctx context.Context) ([]*User, error)

    // UpdateUserRole updates a user's role
    UpdateUserRole(ctx context.Context, userID string, role Role) error

    // UpdateAllowedSkills updates a user's allowed skills list
    UpdateAllowedSkills(ctx context.Context, userID string, skills []string) error

    // DeleteUser deletes a user
    DeleteUser(ctx context.Context, userID string) error
}
```

**Implementation Notes:**
- Uses existing `UserRepository` from Phase 1
- Enforces business rules (e.g., can't delete last admin)
- Audits all user management operations
- Validates platform+platformUID uniqueness

---

## 5. Skill Parameters (New)

### 5.1. Weather Skill Parameters

**InputSchema:**
```json
{
  "type": "object",
  "properties": {
    "location": {
      "type": "string",
      "description": "City name or coordinates (e.g., 'San Francisco' or '37.77,-122.42')"
    },
    "days": {
      "type": "integer",
      "description": "Number of forecast days (1-3)",
      "minimum": 1,
      "maximum": 3,
      "default": 1
    },
    "units": {
      "type": "string",
      "description": "Temperature units",
      "enum": ["metric", "imperial"],
      "default": "metric"
    }
  },
  "required": ["location"]
}
```

**Example Execution:**
```go
params := map[string]interface{}{
    "location": "San Francisco",
    "days":     3,
    "units":    "metric",
}

result, err := weatherSkill.Execute(ctx, params)
// result.Output: "San Francisco: 18°C, Clear sky. Forecast: Mon 20°C, Tue 19°C, Wed 17°C"
```

---

### 5.2. Web Search Skill Parameters

**InputSchema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search query"
    },
    "num_results": {
      "type": "integer",
      "description": "Number of results to return (1-10)",
      "minimum": 1,
      "maximum": 10,
      "default": 3
    }
  },
  "required": ["query"]
}
```

**Example Execution:**
```go
params := map[string]interface{}{
    "query":       "Golang best practices",
    "num_results": 5,
}

result, err := webSearchSkill.Execute(ctx, params)
// result.Output: JSON array of search results
```

**Output Format:**
```json
{
  "results": [
    {
      "title": "Effective Go - The Go Programming Language",
      "url": "https://go.dev/doc/effective_go",
      "snippet": "A comprehensive guide to writing clear, idiomatic Go code..."
    },
    {
      "title": "Go Code Review Comments",
      "url": "https://github.com/golang/go/wiki/CodeReviewComments",
      "snippet": "Common comments made during reviews of Go code..."
    }
  ]
}
```

---

### 5.3. Notes Skill Parameters

**InputSchema:**
```json
{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "Note operation",
      "enum": ["create", "list", "get", "update", "delete"]
    },
    "note_id": {
      "type": "string",
      "description": "Note ID (required for get, update, delete)"
    },
    "title": {
      "type": "string",
      "description": "Note title (required for create, optional for update)"
    },
    "content": {
      "type": "string",
      "description": "Note content (required for create, optional for update)"
    }
  },
  "required": ["command"]
}
```

**Example Executions:**

**Create:**
```go
params := map[string]interface{}{
    "command": "create",
    "title":   "Meeting Notes",
    "content": "Discussed Q1 roadmap...",
}
// result.Output: "Note created with ID: abc123"
```

**List:**
```go
params := map[string]interface{}{
    "command": "list",
}
// result.Output: JSON array of notes with id, title, created_at
```

**Get:**
```go
params := map[string]interface{}{
    "command": "get",
    "note_id": "abc123",
}
// result.Output: Full note content with metadata
```

**Update:**
```go
params := map[string]interface{}{
    "command": "update",
    "note_id": "abc123",
    "content": "Updated content...",
}
// result.Output: "Note updated successfully"
```

**Delete:**
```go
params := map[string]interface{}{
    "command":  "delete",
    "note_id": "abc123",
}
// result.Output: "Note deleted successfully"
```

---

## 6. Database Schema (New Tables)

### 6.1. Notes Table

**SQL:**
```sql
CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL CHECK(LENGTH(title) <= 200),
    content TEXT NOT NULL CHECK(LENGTH(content) <= 10000),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_created_at ON notes(created_at DESC);
```

**Columns:**
- `id` - UUID primary key
- `user_id` - Foreign key to users table
- `title` - Note title (1-200 chars)
- `content` - Note content (0-10000 chars)
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp

**Constraints:**
- Primary key on `id`
- Foreign key constraint on `user_id`
- Length checks on `title` and `content`
- Indexes on `user_id` and `created_at` for fast queries

---

### 6.2. Users Table (Schema Update)

**SQL Migration:**
```sql
-- Add allowed_skills column to existing users table
ALTER TABLE users ADD COLUMN allowed_skills TEXT DEFAULT '[]';
```

**Storage Format:**
- Store as JSON array string: `'["calculator","datetime","weather"]'`
- Empty array `'[]'` means all skills allowed for user's role
- Parse/serialize with `json.Marshal`/`json.Unmarshal`

**Example:**
```sql
INSERT INTO users (id, platform, platform_uid, role, allowed_skills)
VALUES ('user123', 'telegram', '987654321', 1, '["calculator","datetime"]');
```

---

## 7. Error Types (New)

**Location:** `internal/domain/errors.go`

```go
var (
    // User Management Errors
    ErrUserNotFound      = errors.New("user not found")
    ErrUserAlreadyExists = errors.New("user already exists")
    ErrCannotDeleteAdmin = errors.New("cannot delete last admin user")
    ErrInvalidRole       = errors.New("invalid user role")

    // Notes Errors
    ErrNoteNotFound        = errors.New("note not found")
    ErrNoteTooLong         = errors.New("note content exceeds maximum length")
    ErrNoteTitleTooLong    = errors.New("note title exceeds maximum length")
    ErrMaxNotesExceeded    = errors.New("maximum notes per user exceeded")
    ErrUnauthorizedAccess  = errors.New("unauthorized access to note")

    // Gateway Errors
    ErrGatewayNotEnabled   = errors.New("gateway is not enabled")
    ErrInvalidBotToken     = errors.New("invalid bot token")
    ErrConnectionFailed    = errors.New("connection to gateway failed")

    // LLM Provider Errors
    ErrProviderNotFound    = errors.New("LLM provider not found")
    ErrProviderUnavailable = errors.New("LLM provider is unavailable")
    ErrContextTooLong      = errors.New("context exceeds provider's limit")

    // Skill Errors
    ErrWeatherAPIFailed    = errors.New("weather API request failed")
    ErrSearchAPIFailed     = errors.New("search API request failed")
    ErrInvalidLocation     = errors.New("invalid location specified")
)
```

---

## 8. Permission Matrix (Reference)

**Location:** `internal/usecase/skill/permissions.go`

```go
var SkillPermissions = map[string]Role{
    "calculator":   RoleGuest,  // Available to all users
    "datetime":     RoleGuest,  // Available to all users
    "weather":      RoleUser,   // Requires registered user
    "web_search":   RoleUser,   // Requires registered user
    "notes":        RoleUser,   // Requires registered user
    "admin.user":   RoleAdmin,  // Admin-only commands
}
```

---

## 9. Type Mappings

### 9.1. Platform → PlatformUID Format

| Platform | PlatformUID Format | Example |
|----------|-------------------|---------|
| CLI | `cli:<username>` | `cli:alice` |
| Telegram | `tg:<telegram_user_id>` | `tg:123456789` |
| Slack | `slack:<slack_user_id>` | `slack:U01234ABC` |

**Implementation:**
```go
func FormatPlatformUID(platform Platform, uid string) string {
    switch platform {
    case PlatformCLI:
        return fmt.Sprintf("cli:%s", uid)
    case PlatformTelegram:
        return fmt.Sprintf("tg:%s", uid)
    case PlatformSlack:
        return fmt.Sprintf("slack:%s", uid)
    default:
        return uid
    }
}
```

---

## 10. Message Metadata Extensions

### 10.1. Telegram Metadata

```go
type TelegramMetadata struct {
    MessageID   int    `json:"message_id"`
    ChatID      int64  `json:"chat_id"`
    ChatType    string `json:"chat_type"` // "private", "group", "supergroup", "channel"
    IsReply     bool   `json:"is_reply"`
    ReplyToID   int    `json:"reply_to_id,omitempty"`
}
```

Store in `IncomingMessage.Metadata["telegram"]`

---

### 10.2. Slack Metadata

```go
type SlackMetadata struct {
    MessageTS  string `json:"message_ts"`  // Message timestamp (unique ID)
    Channel    string `json:"channel"`     // Channel ID
    ChannelType string `json:"channel_type"` // "channel", "group", "im"
    ThreadTS   string `json:"thread_ts,omitempty"` // If in thread
}
```

Store in `IncomingMessage.Metadata["slack"]`

---

## 11. Configuration File Example

**Complete Phase 2 config.yaml:**
```yaml
server:
  name: NuimanBot
  version: 0.2.0

security:
  encryption_key: ${NUIMANBOT_ENCRYPTION_KEY}
  audit_log_path: ./logs/audit.log

llm:
  default_provider: anthropic

  providers:
    - name: anthropic
      type: anthropic
      api_key: ${ANTHROPIC_API_KEY}
      default_model: claude-sonnet-4-20250514

    - name: openai
      type: openai
      api_key: ${OPENAI_API_KEY}
      default_model: gpt-4o

    - name: ollama
      type: ollama
      base_url: http://localhost:11434
      default_model: llama3

gateways:
  cli:
    enabled: true

  telegram:
    enabled: true
    bot_token: ${TELEGRAM_BOT_TOKEN}
    allowed_users: []

  slack:
    enabled: true
    bot_token: ${SLACK_BOT_TOKEN}
    app_token: ${SLACK_APP_TOKEN}
    allowed_channels: []

storage:
  type: sqlite
  sqlite:
    path: ./data/nuimanbot.db

skills:
  calculator:
    enabled: true

  datetime:
    enabled: true

  weather:
    enabled: true
    api_key: ${OPENWEATHER_API_KEY}
    default_location: San Francisco, CA
    cache_ttl: 30

  web_search:
    enabled: true
    provider: duckduckgo
    max_results: 5

  notes:
    enabled: true
    max_note_length: 10000
    max_notes_per_user: 100
```

---

## 12. Summary

### New Types Added:
- ✅ `User.AllowedSkills` field
- ✅ `Note` entity
- ✅ `TelegramConfig`
- ✅ `SlackConfig`
- ✅ `OpenAIProviderConfig`
- ✅ `OllamaProviderConfig`
- ✅ `WeatherSkillConfig`
- ✅ `WebSearchSkillConfig`
- ✅ `NotesSkillConfig`
- ✅ `UserService` interface
- ✅ `NotesRepository` interface
- ✅ New error types
- ✅ Permission matrix

### Database Changes:
- ✅ Add `notes` table
- ✅ Add `allowed_skills` column to `users` table

### Ready for Implementation:
All data structures and schemas are now fully defined and documented. Ready to proceed to plan.md.
