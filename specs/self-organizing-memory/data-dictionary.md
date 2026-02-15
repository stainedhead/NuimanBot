# Self-Organizing Memory v2 - Data Dictionary

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Draft

---

## Overview

This document defines all data structures, types, interfaces, and constants for the Self-Organizing Memory v2 implementation. Organized by Clean Architecture layers.

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
4. [Adapter Layer](#adapter-layer)
5. [Configuration](#configuration)
6. [Type Aliases & Enums](#type-aliases--enums)
7. [Database Schema](#database-schema)
8. [API Types](#api-types)

---

## Domain Layer

Location: `internal/domain/`

### 1. MemoryCell (Entity)

**File:** `memory_cell.go`

```go
// MemoryCell represents a structured knowledge unit extracted from conversations
type MemoryCell struct {
    // Unique identifier (UUID)
    ID string

    // Conversation or user ID this cell belongs to
    ConversationID string

    // Topic/scene name (e.g., "project-setup", "user-preferences")
    Scene string

    // Type of knowledge
    CellType CellType

    // Importance score (0.0-1.0)
    Salience float64

    // Structured content (JSON or text)
    Content string

    // Source message IDs (JSON array)
    Source string

    // Timestamps
    CreatedAt time.Time
    UpdatedAt time.Time

    // Optional expiration
    ExpiresAt *time.Time
}
```

**Methods:**
```go
// Validate checks if memory cell is valid according to business rules
func (m *MemoryCell) Validate() error

// IsExpired checks if cell has expired
func (m *MemoryCell) IsExpired() bool

// String returns human-readable representation
func (m *MemoryCell) String() string
```

**Validation Rules:**
- `ID` must be non-empty UUID
- `ConversationID` must be non-empty, <= 128 chars
- `Scene` must match pattern `[a-z0-9-]+`, 3-64 chars
- `CellType` must be valid enum value
- `Salience` must be >= 0.0, <= 1.0
- `Content` must be non-empty, <= 2000 chars
- `Source` must be valid JSON array
- `CreatedAt` must not be zero
- `UpdatedAt` must be >= CreatedAt
- `ExpiresAt` if set, must be > CreatedAt

**Business Rules:**
- Cells with salience < 0.3 are considered low-value and may be pruned
- Cells with salience >= 0.9 are critical and preserved longer
- Expired cells are not retrieved but remain in DB until cleanup

**Example:**
```go
cell := &MemoryCell{
    ID:             "550e8400-e29b-41d4-a716-446655440000",
    ConversationID: "conv-123",
    Scene:          "project-setup",
    CellType:       CellTypeDecision,
    Salience:       0.85,
    Content:        "User decided to use SQLite FTS5 for memory retrieval",
    Source:         `["msg-abc123", "msg-def456"]`,
    CreatedAt:      time.Now(),
    UpdatedAt:      time.Now(),
    ExpiresAt:      nil, // No expiration
}

if err := cell.Validate(); err != nil {
    // Handle validation error
}
```

---

### 2. MemoryScene (Entity)

**File:** `memory_scene.go`

```go
// MemoryScene represents a topic with consolidated summary
type MemoryScene struct {
    // Scene name (primary key)
    Scene string

    // Consolidated summary of all cells in this scene
    Summary string

    // Token count of summary
    TokenCount int

    // Last update timestamp
    UpdatedAt time.Time
}
```

**Methods:**
```go
// Validate checks if scene is valid
func (m *MemoryScene) Validate() error

// String returns human-readable representation
func (m *MemoryScene) String() string
```

**Validation Rules:**
- `Scene` must match pattern `[a-z0-9-]+`, 3-64 chars
- `Summary` must be non-empty, <= 10000 chars
- `TokenCount` must be > 0, <= 2000
- `UpdatedAt` must not be zero

**Business Rules:**
- Summaries are regenerated when cell count changes significantly
- Token count enforced at generation time (500 token max by default)
- Scenes with no cells may be deleted during cleanup

---

### 3. MemoryCellRepository (Interface)

**File:** `memory_cell_repository.go`

```go
// MemoryCellRepository defines operations for memory cell persistence
type MemoryCellRepository interface {
    // Create inserts a new memory cell
    Create(ctx context.Context, cell *MemoryCell) error

    // Get retrieves a cell by ID
    Get(ctx context.Context, id string) (*MemoryCell, error)

    // List retrieves cells matching filter
    List(ctx context.Context, filter MemoryCellFilter) ([]*MemoryCell, error)

    // Delete removes a cell
    Delete(ctx context.Context, id string) error

    // SearchFTS performs full-text search
    SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error)

    // GetByScene retrieves cells for a specific scene
    GetByScene(ctx context.Context, scene string, limit int) ([]*MemoryCell, error)

    // GetHighSalience retrieves cells above salience threshold
    GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*MemoryCell, error)

    // DeleteExpired removes cells past expiration
    DeleteExpired(ctx context.Context) (int, error)
}
```

**Expected Behavior:**
- `Create`: Returns `ErrAlreadyExists` if ID already exists
- `Get`: Returns `ErrNotFound` if cell doesn't exist
- `List`: Returns empty slice if no matches, never nil
- `Delete`: Returns `ErrNotFound` if cell doesn't exist
- `SearchFTS`: Returns results ranked by relevance (BM25)
- `GetByScene`: Returns cells ordered by salience desc
- `GetHighSalience`: Returns cells with salience >= threshold
- `DeleteExpired`: Returns count of deleted cells

**Error Conditions:**
- Returns `ErrNotFound` if entity doesn't exist
- Returns `ErrAlreadyExists` if duplicate detected
- Returns `ErrInvalidInput` if validation fails
- Returns context errors (DeadlineExceeded, Canceled) as-is

---

### 4. MemorySceneRepository (Interface)

**File:** `memory_scene_repository.go`

```go
// MemorySceneRepository defines operations for scene persistence
type MemorySceneRepository interface {
    // Upsert creates or updates a scene
    Upsert(ctx context.Context, scene *MemoryScene) error

    // Get retrieves a scene by name
    Get(ctx context.Context, scene string) (*MemoryScene, error)

    // List retrieves all scenes
    List(ctx context.Context) ([]*MemoryScene, error)

    // Delete removes a scene
    Delete(ctx context.Context, scene string) error
}
```

**Expected Behavior:**
- `Upsert`: Creates if not exists, updates if exists
- `Get`: Returns `ErrNotFound` if scene doesn't exist
- `List`: Returns empty slice if no scenes, never nil
- `Delete`: Returns `ErrNotFound` if scene doesn't exist

---

## Use Case Layer

Location: `internal/usecase/memory/`

### 1. MemoryCuratorService (Service)

**File:** `curator_service.go`

```go
// MemoryCuratorService orchestrates memory cell extraction and scene consolidation
type MemoryCuratorService struct {
    cellRepo      domain.MemoryCellRepository
    sceneRepo     domain.MemorySceneRepository
    llmProvider   LLMProvider
    tokenCounter  TokenCounter
    config        CuratorConfig
}

// NewMemoryCuratorService creates a new service instance
func NewMemoryCuratorService(
    cellRepo domain.MemoryCellRepository,
    sceneRepo domain.MemorySceneRepository,
    llmProvider LLMProvider,
    tokenCounter TokenCounter,
    config CuratorConfig,
) *MemoryCuratorService {
    return &MemoryCuratorService{
        cellRepo:     cellRepo,
        sceneRepo:    sceneRepo,
        llmProvider:  llmProvider,
        tokenCounter: tokenCounter,
        config:       config,
    }
}
```

**Methods:**
```go
// ExtractCells extracts memory cells from conversation interaction
func (s *MemoryCuratorService) ExtractCells(
    ctx context.Context,
    input ExtractionInput,
) (ExtractionOutput, error)

// ConsolidateScene updates scene summary with all cells
func (s *MemoryCuratorService) ConsolidateScene(
    ctx context.Context,
    scene string,
) error
```

**Use Cases:**
- Extract structured knowledge from user/assistant messages
- Persist memory cells to repository
- Consolidate scene summaries when cells added
- Handle extraction failures gracefully

---

### 2. ExtractionInput (Use Case Input)

**File:** `types.go`

```go
// ExtractionInput represents input for memory cell extraction
type ExtractionInput struct {
    ConversationID string
    UserMessage    string
    AssistantMessage string
    ToolOutputs    []string // Optional
    MessageIDs     []string // Source tracking
}

// Validate checks if input is valid
func (i *ExtractionInput) Validate() error
```

**Validation Rules:**
- `ConversationID` must be non-empty
- `UserMessage` must be non-empty
- `AssistantMessage` must be non-empty
- `MessageIDs` must have at least 1 element

---

### 3. ExtractionOutput (Use Case Output)

**File:** `types.go`

```go
// ExtractionOutput represents output from memory cell extraction
type ExtractionOutput struct {
    Cells         []*domain.MemoryCell
    ScenesUpdated []string
    Duration      time.Duration
}
```

---

### 4. MemoryRecallService (Service)

**File:** `recall_service.go`

```go
// MemoryRecallService orchestrates memory retrieval and ranking
type MemoryRecallService struct {
    cellRepo      domain.MemoryCellRepository
    sceneRepo     domain.MemorySceneRepository
    tokenCounter  TokenCounter
    config        RecallConfig
}

// NewMemoryRecallService creates a new service instance
func NewMemoryRecallService(
    cellRepo domain.MemoryCellRepository,
    sceneRepo domain.MemorySceneRepository,
    tokenCounter TokenCounter,
    config RecallConfig,
) *MemoryRecallService
```

**Methods:**
```go
// Recall retrieves relevant memory for a prompt
func (s *MemoryRecallService) Recall(
    ctx context.Context,
    input RecallInput,
) (RecallOutput, error)
```

---

### 5. RecallInput (Use Case Input)

**File:** `types.go`

```go
// RecallInput represents input for memory recall
type RecallInput struct {
    ConversationID string
    Prompt         string
    TokenBudget    int // Max tokens for injected memory
}

// Validate checks if input is valid
func (i *RecallInput) Validate() error
```

**Validation Rules:**
- `ConversationID` must be non-empty
- `Prompt` must be non-empty
- `TokenBudget` must be > 0, <= 5000

---

### 6. RecallOutput (Use Case Output)

**File:** `types.go`

```go
// RecallOutput represents output from memory recall
type RecallOutput struct {
    Cells          []*domain.MemoryCell
    Scenes         []*domain.MemoryScene
    FormattedText  string // Ready to inject into prompt
    TokenCount     int
    FTSHitCount    int // Number of FTS matches
    SalienceHitCount int // Number of salience fallback matches
}
```

---

## Infrastructure Layer

Location: `internal/infrastructure/memory/`

### 1. SQLiteMemoryCellRepository (Implementation)

**File:** `sqlite_cell_repository.go`

```go
// SQLiteMemoryCellRepository implements domain.MemoryCellRepository
type SQLiteMemoryCellRepository struct {
    db *sql.DB
}

// NewSQLiteMemoryCellRepository creates a new implementation
func NewSQLiteMemoryCellRepository(db *sql.DB) *SQLiteMemoryCellRepository {
    return &SQLiteMemoryCellRepository{db: db}
}
```

**Implements:** `domain.MemoryCellRepository`

**Dependencies:**
- SQLite database connection
- FTS5 virtual table

---

### 2. SQLiteMemorySceneRepository (Implementation)

**File:** `sqlite_scene_repository.go`

```go
// SQLiteMemorySceneRepository implements domain.MemorySceneRepository
type SQLiteMemorySceneRepository struct {
    db *sql.DB
}

// NewSQLiteMemorySceneRepository creates a new implementation
func NewSQLiteMemorySceneRepository(db *sql.DB) *SQLiteMemorySceneRepository {
    return &SQLiteMemorySceneRepository{db: db}
}
```

**Implements:** `domain.MemorySceneRepository`

---

## Adapter Layer

Location: `internal/adapter/cli/`

### 1. MemoryCLIAdapter (Adapter)

**File:** `memory.go`

```go
// MemoryCLIAdapter adapts CLI commands to memory use cases
type MemoryCLIAdapter struct {
    recallService  *usecase.MemoryRecallService
    curatorService *usecase.MemoryCuratorService
}
```

**Purpose:** Provide CLI commands for memory inspection and management

**Commands:**
- `nuimanbot memory list-scenes` - List all scenes
- `nuimanbot memory get-scene <scene>` - Get scene details
- `nuimanbot memory search --query=X` - Search cells
- `nuimanbot memory delete-cell <id>` - Delete cell
- `nuimanbot memory delete-scene <scene>` - Delete scene

---

## Configuration

Location: `internal/config/`

### Feature Configuration

**File:** `config.go`

```yaml
# Configuration structure (YAML format)
memory:
  self_organizing:
    enabled: false  # Feature flag
    curator:
      model: "claude-3-haiku-20240307"
      timeout_seconds: 10
      max_retries: 1
      circuit_breaker:
        max_failures: 5
        reset_timeout_seconds: 60
    recall:
      token_budget: 1000
      fts_limit: 10
      salience_threshold: 0.7
    scenes:
      max_summary_tokens: 500
      consolidation_strategy: "incremental"  # or "full"
    cleanup:
      enabled: true
      ttl_days: 90
      interval_hours: 24
```

**Go Struct:**
```go
type SelfOrganizingMemoryConfig struct {
    Enabled bool                `yaml:"enabled"`
    Curator CuratorConfig       `yaml:"curator"`
    Recall  RecallConfig        `yaml:"recall"`
    Scenes  ScenesConfig        `yaml:"scenes"`
    Cleanup CleanupConfig       `yaml:"cleanup"`
}

type CuratorConfig struct {
    Model          string              `yaml:"model"`
    TimeoutSeconds int                 `yaml:"timeout_seconds"`
    MaxRetries     int                 `yaml:"max_retries"`
    CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type RecallConfig struct {
    TokenBudget        int     `yaml:"token_budget"`
    FTSLimit           int     `yaml:"fts_limit"`
    SalienceThreshold  float64 `yaml:"salience_threshold"`
}

type ScenesConfig struct {
    MaxSummaryTokens         int    `yaml:"max_summary_tokens"`
    ConsolidationStrategy    string `yaml:"consolidation_strategy"`
}

type CleanupConfig struct {
    Enabled       bool `yaml:"enabled"`
    TTLDays       int  `yaml:"ttl_days"`
    IntervalHours int  `yaml:"interval_hours"`
}

type CircuitBreakerConfig struct {
    MaxFailures           int `yaml:"max_failures"`
    ResetTimeoutSeconds   int `yaml:"reset_timeout_seconds"`
}
```

**Environment Variable Overrides:**
- `NUIMANBOT_MEMORY_SELF_ORGANIZING_ENABLED` → `Enabled`
- `NUIMANBOT_MEMORY_CURATOR_MODEL` → `Curator.Model`
- `NUIMANBOT_MEMORY_RECALL_TOKEN_BUDGET` → `Recall.TokenBudget`

---

## Type Aliases & Enums

### Enumerations

#### CellType

```go
// CellType represents the type of knowledge in a memory cell
type CellType int

const (
    CellTypeFact       CellType = iota // Objective information
    CellTypeDecision                    // A choice was made
    CellTypeTask                        // Something to do or track
    CellTypePreference                  // User's preference or style
    CellTypePlan                        // Future intention or strategy
    CellTypeRisk                        // Potential issue or concern
)

// String returns the string representation
func (c CellType) String() string {
    return [...]string{
        "fact",
        "decision",
        "task",
        "preference",
        "plan",
        "risk",
    }[c]
}

// IsValid checks if enum value is valid
func (c CellType) IsValid() bool {
    return c >= CellTypeFact && c <= CellTypeRisk
}

// ParseCellType parses string to CellType
func ParseCellType(s string) (CellType, error) {
    switch s {
    case "fact":
        return CellTypeFact, nil
    case "decision":
        return CellTypeDecision, nil
    case "task":
        return CellTypeTask, nil
    case "preference":
        return CellTypePreference, nil
    case "plan":
        return CellTypePlan, nil
    case "risk":
        return CellTypeRisk, nil
    default:
        return 0, fmt.Errorf("invalid cell type: %s", s)
    }
}
```

**Valid Values:**
- `fact`: Objective information (e.g., "User is building a Golang CLI app")
- `decision`: A choice was made (e.g., "Decided to use SQLite")
- `task`: Something to do or track (e.g., "Implement authentication")
- `preference`: User's preference or style (e.g., "Prefers TDD workflow")
- `plan`: Future intention or strategy (e.g., "Plan to add OAuth in Phase 2")
- `risk`: Potential issue or concern (e.g., "Risk: FTS may degrade with 1M+ cells")

---

### Type Aliases

```go
// MemoryCellID is a unique identifier for a memory cell (UUID)
type MemoryCellID string

// Validate checks if ID is valid UUID
func (id MemoryCellID) Validate() error {
    if _, err := uuid.Parse(string(id)); err != nil {
        return fmt.Errorf("invalid memory cell ID: %w", err)
    }
    return nil
}

// SceneName is a topic/scene identifier
type SceneName string

// Validate checks if scene name is valid (lowercase-with-dashes)
func (s SceneName) Validate() error {
    pattern := regexp.MustCompile(`^[a-z0-9-]{3,64}$`)
    if !pattern.MatchString(string(s)) {
        return fmt.Errorf("invalid scene name: must be 3-64 chars, lowercase, numbers, dashes only")
    }
    return nil
}
```

---

## Database Schema

### Table: memory_cells

**Purpose:** Stores individual memory cells

**Schema:**
```sql
CREATE TABLE memory_cells (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    scene TEXT NOT NULL,
    cell_type TEXT NOT NULL,
    salience REAL NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,  -- JSON array of message IDs
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,

    -- Constraints
    CONSTRAINT chk_salience CHECK (salience >= 0.0 AND salience <= 1.0),
    CONSTRAINT chk_cell_type CHECK (cell_type IN ('fact', 'decision', 'task', 'preference', 'plan', 'risk'))
);

-- Indexes
CREATE INDEX idx_memory_cells_conversation ON memory_cells(conversation_id);
CREATE INDEX idx_memory_cells_scene ON memory_cells(scene);
CREATE INDEX idx_memory_cells_salience ON memory_cells(salience DESC);
CREATE INDEX idx_memory_cells_expires_at ON memory_cells(expires_at) WHERE expires_at IS NOT NULL;
```

**Columns:**
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | TEXT | No | Primary key, UUID |
| `conversation_id` | TEXT | No | Conversation or user ID |
| `scene` | TEXT | No | Topic/scene name |
| `cell_type` | TEXT | No | Type of knowledge (enum) |
| `salience` | REAL | No | Importance score (0.0-1.0) |
| `content` | TEXT | No | Knowledge content |
| `source` | TEXT | No | JSON array of source message IDs |
| `created_at` | TIMESTAMP | No | Record creation time |
| `updated_at` | TIMESTAMP | No | Last update time |
| `expires_at` | TIMESTAMP | Yes | Expiration time (optional) |

**Indexes:**
- `idx_memory_cells_conversation`: Fast lookup by conversation
- `idx_memory_cells_scene`: Fast lookup by scene
- `idx_memory_cells_salience`: Fast sorting by importance
- `idx_memory_cells_expires_at`: Fast cleanup of expired cells

---

### Table: memory_scenes

**Purpose:** Stores scene summaries

**Schema:**
```sql
CREATE TABLE memory_scenes (
    scene TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    -- Constraints
    CONSTRAINT chk_token_count CHECK (token_count > 0 AND token_count <= 2000)
);
```

**Columns:**
| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `scene` | TEXT | No | Primary key, scene name |
| `summary` | TEXT | No | Consolidated summary |
| `token_count` | INTEGER | No | Token count of summary |
| `updated_at` | TIMESTAMP | No | Last update time |

---

### Virtual Table: memory_cells_fts

**Purpose:** Full-text search index for memory cells

**Schema:**
```sql
CREATE VIRTUAL TABLE memory_cells_fts USING fts5(
    content,
    scene,
    cell_type,
    content='memory_cells',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER memory_cells_ai AFTER INSERT ON memory_cells BEGIN
    INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
    VALUES (new.rowid, new.content, new.scene, new.cell_type);
END;

CREATE TRIGGER memory_cells_ad AFTER DELETE ON memory_cells BEGIN
    DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER memory_cells_au AFTER UPDATE ON memory_cells BEGIN
    UPDATE memory_cells_fts
    SET content = new.content, scene = new.scene, cell_type = new.cell_type
    WHERE rowid = old.rowid;
END;
```

---

## API Types

### CLI Request Types

**Command:** `nuimanbot memory search`

```go
type MemorySearchRequest struct {
    Query     string   `flag:"query"`
    Scene     string   `flag:"scene"`
    CellType  string   `flag:"type"`
    MinSalience float64 `flag:"min-salience"`
    Limit     int      `flag:"limit"`
}
```

**Example:**
```bash
nuimanbot memory search --query="authentication" --scene="project-setup" --min-salience=0.7
```

### CLI Response Types

**Command:** `nuimanbot memory list-scenes`

```go
type MemoryListScenesResponse struct {
    Scenes []SceneSummary `json:"scenes"`
}

type SceneSummary struct {
    Scene      string `json:"scene"`
    CellCount  int    `json:"cell_count"`
    UpdatedAt  string `json:"updated_at"`
}
```

---

## Constants

### Error Messages

```go
const (
    ErrMsgInvalidSalience  = "salience must be between 0.0 and 1.0"
    ErrMsgInvalidCellType  = "invalid cell type: %s"
    ErrMsgEmptyScene       = "scene name cannot be empty"
    ErrMsgSceneNotFound    = "scene not found: %s"
    ErrMsgCellNotFound     = "memory cell not found: %s"
    ErrMsgExtractionFailed = "failed to extract memory cells: %w"
    ErrMsgConsolidationFailed = "failed to consolidate scene: %w"
)
```

### Limits and Thresholds

```go
const (
    MaxContentLength     = 2000   // Max chars in cell content
    MaxSummaryLength     = 10000  // Max chars in scene summary
    MaxSceneNameLength   = 64     // Max chars in scene name
    MinSceneNameLength   = 3      // Min chars in scene name
    MaxSummaryTokens     = 500    // Max tokens in scene summary
    DefaultTokenBudget   = 1000   // Default memory injection budget
    DefaultFTSLimit      = 10     // Default FTS result limit
    DefaultSalienceThreshold = 0.7 // Default salience threshold
    DefaultTTLDays       = 90     // Default cell expiration (days)
)
```

### Timeouts and Durations

```go
const (
    DefaultCuratorTimeout = 10 * time.Second
    DefaultRecallTimeout  = 5 * time.Second
    DefaultCleanupInterval = 24 * time.Hour
    CircuitBreakerResetTimeout = 60 * time.Second
)
```

---

## Type Mapping Reference

**Domain → Database:**
| Domain Type | Database Type | Conversion |
|-------------|---------------|------------|
| `string` | `TEXT` | Direct |
| `float64` (salience) | `REAL` | Direct |
| `CellType` | `TEXT` | Enum string |
| `time.Time` | `TIMESTAMP` | RFC3339 |
| `[]string` (source) | `TEXT` | JSON array |

**Domain → LLM API:**
| Domain Type | JSON Type | Conversion |
|-------------|-----------|------------|
| `MemoryCell` | `object` | Struct to JSON |
| `CellType` | `string` | Enum string |
| `time.Time` | `string` | RFC3339 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-15 | Initial version |
