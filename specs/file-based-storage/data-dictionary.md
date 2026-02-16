# File-Based Storage Migration - Data Dictionary

**Created:** 2026-02-16
**Version:** 1.0
**Status:** Complete

---

## Overview

This document defines all data structures, types, interfaces, and constants for the File-Based Storage Migration implementation. Organized by Clean Architecture layers.

**Purpose:**
- Single source of truth for all data types
- Ensure consistency across layers
- Document validation rules and constraints
- Specify file schemas (JSON/JSONL formats)

---

## Table of Contents

1. [Domain Layer](#domain-layer)
2. [Infrastructure Layer](#infrastructure-layer)
3. [File Formats & JSON Schemas](#file-formats--json-schemas)
4. [Index Structures](#index-structures)
5. [Type Aliases & Enums](#type-aliases--enums)
6. [Constants & Limits](#constants--limits)
7. [Type Mapping Reference](#type-mapping-reference)

---

## Domain Layer

Location: `internal/domain/`

###  1. UserProfile (Unified Entity)

**File:** `user_profile.go`

**Purpose:** Represents comprehensive user identity, authentication, and preferences. Merges `domain.User` and `domain.UserProfile` into single unified profile.

```go
// UserProfile represents comprehensive user identity and preferences
type UserProfile struct {
	// Core Identity
	UserID    string `json:"userID"`    // Primary key, unique identifier
	Moniker   string `json:"moniker"`   // Display name or handle
	FirstName string `json:"firstName"` // Given name
	LastName  string `json:"lastName"`  // Family name
	NickName  string `json:"nickName"`  // Preferred informal name

	// Contact Information
	PrimaryEmail string `json:"primaryEmail"` // Primary contact email (required)
	BackupEmail  string `json:"backupEmail"`  // Secondary contact email
	MobilePhone  string `json:"mobilePhone"`  // Mobile phone number (E.164 format)

	// Localization
	PrimaryLanguage   string `json:"primaryLanguage"`   // ISO 639-1 code (e.g., "en", "es")
	SecondaryLanguage string `json:"secondaryLanguage"` // ISO 639-1 code for fallback
	Timezone          string `json:"timezone"`          // IANA timezone (e.g., "America/New_York")
	PrimaryLocation   string `json:"primaryLocation"`   // Geographic location

	// Organizational Context
	JobRole  string   `json:"jobRole"`  // User's organizational role
	UserType UserType `json:"userType"` // Individual, Enterprise, Developer, Admin

	// Multi-Platform Integration (merged from domain.User)
	PlatformIDs PlatformIdentifiers `json:"platformIDs"` // Platform identifiers

	// Authentication & Security (merged from domain.User)
	Role         Role     `json:"role"`         // User role (guest, user, admin)
	AllowedTools []string `json:"allowedTools"` // Optional tool whitelist (empty = all allowed)
	APIKey       string   `json:"apiKey"`       // API key for REST API authentication

	// Personalization
	ProfileInfo string `json:"profileInfo"` // Freeform biographical info (max 2000 chars)

	// Metadata
	Enabled       bool      `json:"enabled"`       // Account enabled/disabled
	DataDirectory string    `json:"dataDirectory"` // Path to user's data directory
	CreatedAt     time.Time `json:"createdAt"`     // Profile creation time
	UpdatedAt     time.Time `json:"updatedAt"`     // Last modification time
	LastVerified  time.Time `json:"lastVerified"`  // Last time user verified their info
}

// PlatformIdentifiers stores user IDs for different messaging platforms
type PlatformIdentifiers struct {
	CLI      string `json:"cli"`      // CLI username
	Slack    string `json:"slack"`    // Slack user ID (e.g., "U01ABC123")
	Telegram string `json:"telegram"` // Telegram user ID (numeric string)
}
```

**Methods:**
```go
// Validate checks if profile is valid according to business rules
func (up *UserProfile) Validate() error

// GetDisplayName returns the best available display name
// Priority: NickName > Moniker > FirstName > UserID
func (up *UserProfile) GetDisplayName() string

// GetFullName returns "FirstName LastName"
func (up *UserProfile) GetFullName() string

// GetPreferredLanguage returns PrimaryLanguage or "en" if not set
func (up *UserProfile) GetPreferredLanguage() string

// GetTimezone returns Timezone or "UTC" if not set
func (up *UserProfile) GetTimezone() string
```

**Validation Rules:**
- `UserID`: Required, non-empty, <= 64 chars
- `PrimaryEmail`: Required, valid email format, <= 254 chars
- `BackupEmail`: If set, valid email format, != PrimaryEmail, <= 254 chars
- `MobilePhone`: If set, E.164 format (starts with +), 8-16 chars
- `Moniker`: <= 50 chars
- `FirstName`, `LastName`: <= 100 chars
- `NickName`: <= 50 chars
- `PrimaryLocation`, `JobRole`: <= 100 chars
- `ProfileInfo`: <= 2000 chars
- `DataDirectory`: Required, non-empty

**Business Rules:**
- Default `Role` = `RoleUser` for new profiles
- Default `PrimaryLanguage` = "en"
- Default `Timezone` = "UTC"
- Default `Enabled` = true
- `CreatedAt`, `UpdatedAt`, `LastVerified` set to current time on creation

---

### 2. Conversation (Entity)

**File:** `message.go`

**Purpose:** Represents a conversation containing messages between user and bot.

```go
// Conversation represents a conversation in memory/database
type Conversation struct {
	ID        string          `json:"id"`        // Unique conversation ID
	UserID    string          `json:"userID"`    // Owner user ID
	Platform  Platform        `json:"platform"`  // Platform (cli, slack, telegram)
	Messages  []StoredMessage `json:"messages"`  // All messages in conversation
	CreatedAt time.Time       `json:"createdAt"` // Creation timestamp
	UpdatedAt time.Time       `json:"updatedAt"` // Last update timestamp
}
```

**Validation Rules:**
- `ID`: Required, non-empty, <= 128 chars
- `UserID`: Required, non-empty, <= 64 chars
- `Platform`: Must be valid Platform value
- `Messages`: Can be empty array (never nil)
- `CreatedAt`, `UpdatedAt`: Required, non-zero
- `UpdatedAt >= CreatedAt`

---

### 3. StoredMessage (Entity)

**File:** `message.go`

**Purpose:** Represents a message stored in a conversation.

```go
// StoredMessage represents a message stored in memory/database
type StoredMessage struct {
	ID          string       `json:"id"`                    // Unique message ID
	Role        string       `json:"role"`                  // "user", "assistant", "system"
	Content     string       `json:"content"`               // Message text content
	ToolCalls   []ToolCall   `json:"toolCalls,omitempty"`   // Tool calls made (if assistant)
	ToolResults []ToolResult `json:"toolResults,omitempty"` // Tool results (if tool)
	TokenCount  int          `json:"tokenCount"`            // Token count (for billing/limits)
	Timestamp   time.Time    `json:"timestamp"`             // Message timestamp
}

// ToolCall represents a tool/function call
type ToolCall struct {
	ID       string `json:"id"`       // Tool call ID
	Type     string `json:"type"`     // "function"
	Function struct {
		Name      string `json:"name"`      // Function name
		Arguments string `json:"arguments"` // JSON arguments
	} `json:"function"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"` // References ToolCall.ID
	Content    string `json:"content"`      // Result content
}
```

**Validation Rules:**
- `ID`: Required, non-empty, <= 128 chars
- `Role`: Required, must be "user", "assistant", or "system"
- `Content`: Required if no ToolCalls/ToolResults
- `TokenCount`: >= 0
- `Timestamp`: Required, non-zero

---

### 4. MemoryCell (Entity)

**File:** `internal/domain/memoryv2/memory_cell.go`

**Purpose:** Represents a structured knowledge unit extracted from conversations.

```go
// MemoryCell represents a structured knowledge unit
type MemoryCell struct {
	ID             string     `json:"id"`             // UUID
	ConversationID string     `json:"conversationID"` // Conversation or user ID
	Scene          string     `json:"scene"`          // Topic/scene name (e.g., "project-setup")
	CellType       CellType   `json:"cellType"`       // Type of knowledge
	Salience       float64    `json:"salience"`       // Importance score (0.0-1.0)
	Content        string     `json:"content"`        // Structured content
	Source         string     `json:"source"`         // Source message IDs (JSON array)
	CreatedAt      time.Time  `json:"createdAt"`      // Creation timestamp
	UpdatedAt      time.Time  `json:"updatedAt"`      // Last update timestamp
	ExpiresAt      *time.Time `json:"expiresAt"`      // Optional expiration time
}
```

**Methods:**
```go
// Validate checks if the memory cell is valid
func (m *MemoryCell) Validate() error

// IsExpired checks if the cell has expired
func (m *MemoryCell) IsExpired() bool

// String returns human-readable representation
func (m *MemoryCell) String() string
```

**Validation Rules:**
- `ID`: Required, valid UUID format
- `ConversationID`: Required, <= 128 chars
- `Scene`: Required, matches pattern `^[a-z0-9-]{3,64}$` (3-64 lowercase, numbers, dashes)
- `CellType`: Must be valid CellType enum value
- `Salience`: 0.0 <= value <= 1.0
- `Content`: Required, non-empty, <= 2000 chars
- `Source`: Required, valid JSON array
- `CreatedAt`: Required, non-zero
- `UpdatedAt`: >= CreatedAt
- `ExpiresAt`: If set, must be > CreatedAt

**Business Rules:**
- UUID generated on creation
- `UpdatedAt` updated on any modification
- Cell auto-deleted when expired (via cleanup job)

---

### 5. MemoryScene (Entity)

**File:** `internal/domain/memoryv2/memory_scene.go`

**Purpose:** Represents a topic with consolidated summary of related memory cells.

```go
// MemoryScene represents a topic with consolidated summary
type MemoryScene struct {
	Scene      string    `json:"scene"`      // Scene name (primary key)
	Summary    string    `json:"summary"`    // Consolidated summary
	TokenCount int       `json:"tokenCount"` // Token count of summary
	UpdatedAt  time.Time `json:"updatedAt"`  // Last update timestamp
}
```

**Methods:**
```go
// Validate checks if the scene is valid
func (m *MemoryScene) Validate() error

// String returns human-readable representation
func (m *MemoryScene) String() string
```

**Validation Rules:**
- `Scene`: Required, matches pattern `^[a-z0-9-]{3,64}$`
- `Summary`: Required, non-empty, <= 10000 chars
- `TokenCount`: > 0, <= 2000
- `UpdatedAt`: Required, non-zero

**Business Rules:**
- Scene summary updated periodically from cells
- Token count limits prevent summary bloat
- Scene can be manually curated

---

### 6. Note (Entity)

**File:** `note.go`

**Purpose:** Represents a user note.

```go
// Note represents a user note
type Note struct {
	ID        string    `json:"id"`        // Unique note ID
	UserID    string    `json:"userID"`    // Owner user ID
	Title     string    `json:"title"`     // Note title
	Content   string    `json:"content"`   // Note content
	Tags      []string  `json:"tags"`      // Tags for organization
	CreatedAt time.Time `json:"createdAt"` // Creation timestamp
	UpdatedAt time.Time `json:"updatedAt"` // Last update timestamp
}
```

**Methods:**
```go
// Validate checks if the note has valid data
func (n *Note) Validate() error
```

**Validation Rules:**
- `ID`: Required, non-empty
- `UserID`: Required, non-empty
- `Title`: Required, non-empty
- `Content`: Required, non-empty, <= 100000 chars
- `Tags`: Can be empty array (never nil)
- `CreatedAt`, `UpdatedAt`: Required, non-zero

---

### 7. AuditLogEntry (Entity)

**File:** New - `audit.go`

**Purpose:** Represents an audit log entry for system events.

```go
// AuditLogEntry represents an audit log entry
type AuditLogEntry struct {
	Timestamp      time.Time         `json:"timestamp"`      // Event timestamp
	UserID         string            `json:"userID"`         // User who triggered event
	Action         string            `json:"action"`         // Action performed
	Platform       Platform          `json:"platform"`       // Platform (cli, slack, etc.)
	ConversationID string            `json:"conversationID"` // Conversation ID (if applicable)
	Success        bool              `json:"success"`        // Whether action succeeded
	ErrorMessage   string            `json:"errorMessage"`   // Error message (if failed)
	Metadata       map[string]string `json:"metadata"`       // Additional context
}
```

**Validation Rules:**
- `Timestamp`: Required, non-zero
- `UserID`: Required if user-initiated action
- `Action`: Required, non-empty
- `Success`: Required boolean
- Format: JSONL (one entry per line, append-only)

---

## Repository Interfaces

### UserProfileRepository

**File:** `user_profile.go`

```go
// UserProfileRepository defines the contract for user profile data persistence
type UserProfileRepository interface {
	// SaveProfile creates or updates a user profile
	SaveProfile(ctx context.Context, profile *UserProfile) error

	// GetProfileByUserID retrieves a profile by user ID
	GetProfileByUserID(ctx context.Context, userID string) (*UserProfile, error)

	// GetProfileByEmail retrieves a profile by email address
	GetProfileByEmail(ctx context.Context, email string) (*UserProfile, error)

	// GetProfileByPlatformID retrieves a profile by platform-specific ID
	GetProfileByPlatformID(ctx context.Context, platform Platform, platformID string) (*UserProfile, error)

	// GetProfileByAPIKey retrieves a profile by API key
	GetProfileByAPIKey(ctx context.Context, apiKey string) (*UserProfile, error)

	// ListProfiles returns all profiles (with pagination support)
	ListProfiles(ctx context.Context, offset, limit int) ([]*UserProfile, error)

	// DeleteProfile removes a profile by user ID
	DeleteProfile(ctx context.Context, userID string) error
}
```

**Expected Behavior:**
- `SaveProfile`: Creates if new, updates if exists (upsert)
- `GetProfile*`: Returns `ErrNotFound` if not exists
- `DeleteProfile`: Returns `ErrNotFound` if not exists
- All methods are context-aware and cancellable

---

### ConversationRepository

**File:** New - `conversation.go`

```go
// ConversationRepository defines operations for conversation persistence
type ConversationRepository interface {
	// SaveConversation creates or updates a conversation
	SaveConversation(ctx context.Context, conv *Conversation) error

	// GetConversation retrieves a conversation by ID
	GetConversation(ctx context.Context, convID string) (*Conversation, error)

	// ListConversations returns conversation summaries for a user
	ListConversations(ctx context.Context, userID string) ([]ConversationSummary, error)

	// DeleteConversation removes a conversation by ID
	DeleteConversation(ctx context.Context, convID string) error

	// AppendMessage adds a message to an existing conversation
	AppendMessage(ctx context.Context, convID string, message StoredMessage) error
}
```

---

### MemoryCellRepository

**File:** `internal/domain/memoryv2/memory_cell_repository.go`

```go
// MemoryCellRepository defines operations for memory cell persistence
type MemoryCellRepository interface {
	// Create inserts a new memory cell
	Create(ctx context.Context, cell *MemoryCell) error

	// Get retrieves a cell by ID
	Get(ctx context.Context, id string) (*MemoryCell, error)

	// List retrieves cells matching the filter
	List(ctx context.Context, filter MemoryCellFilter) ([]*MemoryCell, error)

	// Delete removes a cell by ID
	Delete(ctx context.Context, id string) error

	// SearchFTS performs full-text search on cell content
	SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error)

	// GetByScene retrieves cells for a specific scene
	GetByScene(ctx context.Context, scene string, limit int) ([]*MemoryCell, error)

	// GetHighSalience retrieves cells above a salience threshold
	GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*MemoryCell, error)

	// DeleteExpired removes cells past their expiration time
	DeleteExpired(ctx context.Context) (int, error)
}
```

---

### MemorySceneRepository

**File:** `internal/domain/memoryv2/memory_scene_repository.go`

```go
// MemorySceneRepository defines operations for scene persistence
type MemorySceneRepository interface {
	// Upsert creates a scene if it doesn't exist, or updates it if it does
	Upsert(ctx context.Context, scene *MemoryScene) error

	// Get retrieves a scene by name
	Get(ctx context.Context, scene string) (*MemoryScene, error)

	// List retrieves all scenes
	List(ctx context.Context) ([]*MemoryScene, error)

	// Delete removes a scene by name
	Delete(ctx context.Context, scene string) error
}
```

---

### NotesRepository

**File:** New - `note.go`

```go
// NotesRepository defines operations for notes persistence
type NotesRepository interface {
	// Create inserts a new note
	Create(ctx context.Context, note *Note) error

	// GetByID retrieves a note by ID
	GetByID(ctx context.Context, noteID string) (*Note, error)

	// List retrieves notes for a user
	List(ctx context.Context, userID string) ([]*Note, error)

	// Update updates an existing note
	Update(ctx context.Context, note *Note) error

	// Delete removes a note by ID
	Delete(ctx context.Context, noteID string) error
}
```

---

### AuditRepository

**File:** New - `audit.go`

```go
// AuditRepository defines operations for audit log persistence
type AuditRepository interface {
	// Append adds a new audit entry (append-only)
	Append(ctx context.Context, entry *AuditLogEntry) error

	// Query retrieves audit entries matching filter
	Query(ctx context.Context, filter AuditFilter) ([]*AuditLogEntry, error)
}
```

---

## Infrastructure Layer

Location: `internal/infrastructure/storage/`

### File Repository Implementations

**Files to Create:**
- `file_user_profile_repository.go` - Implements `UserProfileRepository`
- `file_conversation_repository.go` - Implements `ConversationRepository`
- `file_memory_cell_repository.go` - Implements `MemoryCellRepository`
- `file_memory_scene_repository.go` - Implements `MemorySceneRepository`
- `file_notes_repository.go` - Implements `NotesRepository`
- `file_audit_repository.go` - Implements `AuditRepository`

**Common Pattern:**
```go
type File<Entity>Repository struct {
	basePath string            // Base directory path
	mu       sync.RWMutex      // Concurrency control
	index    *<Entity>Index    // In-memory index (optional)
	cache    *Cache            // Optional cache
}

func NewFile<Entity>Repository(basePath string) *File<Entity>Repository
```

---

## File Formats & JSON Schemas

### User Profile (profile.json)

```json
{
  "userID": "user-123",
  "moniker": "Alice Cooper",
  "firstName": "Alice",
  "lastName": "Cooper",
  "nickName": "Ally",
  "primaryEmail": "alice@example.com",
  "backupEmail": "",
  "mobilePhone": "+12125551234",
  "primaryLanguage": "en",
  "secondaryLanguage": "",
  "timezone": "America/New_York",
  "primaryLocation": "New York, NY",
  "jobRole": "Software Engineer",
  "userType": "individual",
  "platformIDs": {
    "cli": "alice",
    "slack": "U01ABC123",
    "telegram": "123456789"
  },
  "role": "user",
  "allowedTools": [],
  "apiKey": "key-abc123...",
  "profileInfo": "Senior software engineer interested in AI...",
  "enabled": true,
  "dataDirectory": "users/user-123",
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-02-16T00:00:00Z",
  "lastVerified": "2026-02-01T00:00:00Z"
}
```

---

### Conversation (conversations/<conversation-id>.json)

```json
{
  "id": "conv-123",
  "userID": "user-123",
  "platform": "cli",
  "messages": [
    {
      "id": "msg-001",
      "role": "user",
      "content": "Hello, can you help me?",
      "toolCalls": null,
      "toolResults": null,
      "tokenCount": 12,
      "timestamp": "2026-02-16T10:00:00Z"
    },
    {
      "id": "msg-002",
      "role": "assistant",
      "content": "Of course! What can I help you with?",
      "toolCalls": null,
      "toolResults": null,
      "tokenCount": 15,
      "timestamp": "2026-02-16T10:00:01Z"
    }
  ],
  "createdAt": "2026-02-16T10:00:00Z",
  "updatedAt": "2026-02-16T10:00:01Z"
}
```

---

### Memory Cell (memory/cells/<cell-id>.json)

```json
{
  "id": "cell-001",
  "conversationID": "conv-123",
  "scene": "project-planning",
  "cellType": "fact",
  "salience": 0.8,
  "content": "User prefers TypeScript for new projects",
  "source": "[\"msg-001\", \"msg-003\"]",
  "createdAt": "2026-02-16T10:00:00Z",
  "updatedAt": "2026-02-16T10:00:00Z",
  "expiresAt": null
}
```

---

### Memory Scene (memory/scenes/<scene-name>.json)

```json
{
  "scene": "project-planning",
  "summary": "User is planning a new TypeScript project with React frontend...",
  "tokenCount": 156,
  "updatedAt": "2026-02-16T10:30:00Z"
}
```

---

### Note (notes/<note-id>.json)

```json
{
  "id": "note-456",
  "userID": "user-123",
  "title": "Meeting Notes - Q1 Planning",
  "content": "## Q1 2026 Goals\n\n- Launch new feature\n- Improve performance...",
  "tags": ["meeting", "roadmap", "q1-2026"],
  "createdAt": "2026-02-16T09:00:00Z",
  "updatedAt": "2026-02-16T09:30:00Z"
}
```

---

### Audit Log (system/audit.jsonl)

**Format:** JSONL (one entry per line)

```jsonl
{"timestamp":"2026-02-16T10:00:00Z","userID":"user-123","action":"login","platform":"cli","conversationID":"","success":true,"errorMessage":"","metadata":{}}
{"timestamp":"2026-02-16T10:01:00Z","userID":"user-123","action":"chat","platform":"cli","conversationID":"conv-123","success":true,"errorMessage":"","metadata":{"model":"claude-3-5-sonnet"}}
{"timestamp":"2026-02-16T10:02:00Z","userID":"user-123","action":"memory-create","platform":"cli","conversationID":"conv-123","success":true,"errorMessage":"","metadata":{"scene":"project-planning","cellType":"fact"}}
```

---

## Index Structures

### Conversation Index (conversations/index.json)

**Purpose:** Fast conversation listing without loading full files

```json
{
  "conversations": [
    {
      "id": "conv-123",
      "platform": "cli",
      "messageCount": 42,
      "lastMessageAt": "2026-02-16T10:30:00Z",
      "createdAt": "2026-02-16T10:00:00Z",
      "updatedAt": "2026-02-16T10:30:00Z"
    },
    {
      "id": "conv-456",
      "platform": "slack",
      "messageCount": 15,
      "lastMessageAt": "2026-02-15T14:20:00Z",
      "createdAt": "2026-02-15T14:00:00Z",
      "updatedAt": "2026-02-15T14:20:00Z"
    }
  ],
  "updatedAt": "2026-02-16T10:30:00Z"
}
```

---

### Memory Index by Scene (memory/indexes/by-scene.json)

```json
{
  "project-planning": ["cell-001", "cell-003", "cell-007"],
  "architecture": ["cell-002", "cell-005"],
  "user-preferences": ["cell-004", "cell-006"],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

---

### Memory Index by Type (memory/indexes/by-type.json)

```json
{
  "fact": ["cell-001", "cell-002", "cell-004"],
  "decision": ["cell-003", "cell-005"],
  "task": ["cell-006"],
  "preference": ["cell-007"],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

---

### Memory Index by Salience (memory/indexes/by-salience.json)

```json
{
  "cells": [
    {"id": "cell-003", "salience": 0.95},
    {"id": "cell-001", "salience": 0.80},
    {"id": "cell-007", "salience": 0.75},
    {"id": "cell-002", "salience": 0.65}
  ],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

---

### Memory Search Index (memory/indexes/search.json)

**Purpose:** Keyword search without FTS5

```json
{
  "keywords": {
    "typescript": ["cell-001", "cell-007"],
    "planning": ["cell-001", "cell-003"],
    "architecture": ["cell-002", "cell-005"],
    "react": ["cell-007"],
    "performance": ["cell-002"]
  },
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

---

### Notes Index (notes/index.json)

```json
{
  "notes": [
    {
      "id": "note-456",
      "title": "Meeting Notes - Q1 Planning",
      "tags": ["meeting", "roadmap", "q1-2026"],
      "createdAt": "2026-02-16T09:00:00Z",
      "updatedAt": "2026-02-16T09:30:00Z"
    }
  ],
  "byTag": {
    "meeting": ["note-456", "note-789"],
    "roadmap": ["note-456"],
    "q1-2026": ["note-456"]
  },
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

---

## Type Aliases & Enums

### Role Enum

```go
// Role defines the role of a user in the system
type Role string

const (
	RoleGuest Role = "guest" // Limited access, unauthenticated users
	RoleUser  Role = "user"  // Standard access, registered users
	RoleAdmin Role = "admin" // Full access, administrators
)

// Level returns the numeric level of a role (higher = more permissions)
func (r Role) Level() int

// HasPermission checks if this role has at least the permissions of the required role
func (r Role) HasPermission(required Role) bool
```

**Valid Values:**
- `guest`: Limited access (level 0)
- `user`: Standard access (level 1)
- `admin`: Full access (level 2)

---

### UserType Enum

```go
// UserType defines the type/tier of user account
type UserType string

const (
	UserTypeIndividual UserType = "individual" // Individual user
	UserTypeEnterprise UserType = "enterprise" // Enterprise/organization user
	UserTypeDeveloper  UserType = "developer"  // Developer account
	UserTypeAdmin      UserType = "admin"      // System administrator
)
```

---

### Platform Enum

```go
// Platform defines the messaging platform
type Platform string

const (
	PlatformTelegram Platform = "telegram"
	PlatformSlack    Platform = "slack"
	PlatformCLI      Platform = "cli"
)
```

---

### CellType Enum

```go
// CellType represents the type of knowledge in a memory cell
type CellType int

const (
	CellTypeFact       CellType = 0 // Objective information
	CellTypeDecision   CellType = 1 // A choice that was made
	CellTypeTask       CellType = 2 // Something to do or track
	CellTypePreference CellType = 3 // User's preference or style
	CellTypePlan       CellType = 4 // Future intention or strategy
	CellTypeRisk       CellType = 5 // Potential issue or concern
)

// String returns the string representation
func (c CellType) String() string

// IsValid checks if the CellType value is valid
func (c CellType) IsValid() bool

// ParseCellType parses a string to CellType
func ParseCellType(s string) (CellType, error)
```

**Valid Values:**
- `fact` (0): Objective information
- `decision` (1): A choice that was made
- `task` (2): Something to do or track
- `preference` (3): User's preference or style
- `plan` (4): Future intention or strategy
- `risk` (5): Potential issue or concern

---

## Constants & Limits

### Memory Cell Limits

```go
const (
	MaxContentLength     = 2000 // Max chars in cell content
	MaxConversationIDLen = 128  // Max chars in conversation ID
	MinSceneNameLength   = 3    // Min chars in scene name
	MaxSceneNameLength   = 64   // Max chars in scene name
)
```

### Memory Scene Limits

```go
const (
	MaxSummaryLength = 10000 // Max chars in scene summary
	MaxSummaryTokens = 2000  // Max tokens in summary
)
```

### User Profile Limits

```go
const (
	MaxUserIDLength    = 64    // Max chars in user ID
	MaxMonikerLength   = 50    // Max chars in moniker
	MaxNameLength      = 100   // Max chars in first/last name
	MaxNickNameLength  = 50    // Max chars in nickname
	MaxEmailLength     = 254   // Max chars in email (RFC 5321)
	MaxProfileInfoLen  = 2000  // Max chars in profile info
	MaxJobRoleLength   = 100   // Max chars in job role
	MaxLocationLength  = 100   // Max chars in location
)
```

### Note Limits

```go
const (
	MaxNoteContentLength = 100000 // Max chars in note content
)
```

---

## Type Mapping Reference

### Domain → JSON

| Domain Type | JSON Type | Conversion | Notes |
|-------------|-----------|------------|-------|
| `string` | `string` | Direct | - |
| `int` | `number` | Direct | - |
| `float64` | `number` | Direct | Salience, etc. |
| `bool` | `boolean` | Direct | - |
| `time.Time` | `string` | RFC3339 | ISO 8601 format |
| `[]string` | `array` | Direct | Tags, AllowedTools |
| `[]ToolCall` | `array` | JSON marshal | Nested objects |
| `map[string]string` | `object` | JSON marshal | PlatformIDs, Metadata |
| `*time.Time` | `string or null` | RFC3339 or null | ExpiresAt |
| `Platform` | `string` | String cast | "cli", "slack", "telegram" |
| `Role` | `string` | String cast | "guest", "user", "admin" |
| `UserType` | `string` | String cast | "individual", "enterprise", etc. |
| `CellType` | `string` | String cast | "fact", "decision", etc. |

### JSON → Domain

Reverse of above. Use `json.Unmarshal()` with proper struct tags.

---

## Error Handling

### Domain Errors

```go
var (
	ErrNotFound       = errors.New("resource not found")
	ErrAlreadyExists  = errors.New("resource already exists")
	ErrInvalidInput   = errors.New("invalid input")
	ErrUnauthorized   = errors.New("unauthorized access")
	ErrConflict       = errors.New("resource conflict")
)
```

### Repository Error Behavior

- **Not Found**: Return `ErrNotFound` for Get/Delete operations on missing resources
- **Already Exists**: Return `ErrAlreadyExists` for Create operations on existing resources
- **Invalid Input**: Return `ErrInvalidInput` for validation failures
- **Wrapping**: Wrap errors with context: `fmt.Errorf("failed to read file: %w", err)`

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-16 | Initial complete version with all domain entities and file schemas |
