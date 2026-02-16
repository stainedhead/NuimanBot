# File-Based Storage Migration - System Architecture

**Created:** 2026-02-16
**Version:** 1.0
**Status:** Complete
**Last Updated:** 2026-02-16

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [System Context](#system-context)
3. [Component Architecture](#component-architecture)
4. [Layer Responsibilities](#layer-responsibilities)
5. [Data Flow](#data-flow)
6. [Sequence Diagrams](#sequence-diagrams)
7. [Integration Points](#integration-points)
8. [Architectural Decisions](#architectural-decisions)
9. [Trade-offs](#trade-offs)
10. [Performance Considerations](#performance-considerations)

---

## Architecture Overview

**High-Level Summary:**

The File-Based Storage Migration replaces SQLite database persistence with a pure file-based storage system using JSON/JSONL formats. The migration maintains Clean Architecture principles by swapping only the infrastructure layer implementations while keeping domain interfaces unchanged. This enables simplified deployment, improved data portability, and unified storage architecture across the application.

**Architectural Style:** Clean Architecture + Repository Pattern + File-Based Persistence

**Key Principles:**
- **Dependency Inversion**: Domain layer defines repository interfaces; infrastructure implements them
- **Single Responsibility**: Each repository manages one entity type; each index manages one access pattern
- **Open/Closed**: New storage mechanisms can be added without changing domain logic
- **Interface Segregation**: Repository interfaces tailored to specific use cases
- **Atomic Operations**: All writes use atomic file operations (write-to-temp + rename)

**Architecture Diagram:**

```
┌─────────────────────────────────────────────────────────────┐
│              Infrastructure Layer (File System)              │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐ │
│  │ FileUserProfile│  │FileConversation│  │ FileMemoryCell│ │
│  │   Repository   │  │   Repository   │  │   Repository  │ │
│  └────────────────┘  └────────────────┘  └───────────────┘ │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐ │
│  │FileMemoryScene │  │   FileNotes    │  │  FileAudit    │ │
│  │   Repository   │  │   Repository   │  │  Repository   │ │
│  └────────────────┘  └────────────────┘  └───────────────┘ │
│           │                  │                    │          │
│           └──────────────────┼────────────────────┘          │
│                              │ implements                    │
└──────────────────────────────┼───────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────┐
│                      Domain Layer                             │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │         Repository Interfaces (Contracts)                │ │
│  │  - UserProfileRepository                                 │ │
│  │  - ConversationRepository                                │ │
│  │  - MemoryCellRepository                                  │ │
│  │  - MemorySceneRepository                                 │ │
│  │  - NotesRepository                                       │ │
│  │  - AuditRepository                                       │ │
│  └─────────────────────────────────────────────────────────┘ │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Domain Entities                             │ │
│  │  UserProfile | Conversation | MemoryCell | Note         │ │
│  └─────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────┘
                               ▲
                               │ uses
┌──────────────────────────────┴───────────────────────────────┐
│                      Use Case Layer                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │   Chat   │  │  Memory  │  │  Notes   │  │   Auth   │    │
│  │ Service  │  │ Service  │  │ Service  │  │ Service  │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└───────────────────────────────────────────────────────────────┘
                               ▲
                               │ uses
┌──────────────────────────────┴───────────────────────────────┐
│                      Adapter Layer                            │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐ │
│  │  CLI Gateway   │  │  Slack Gateway │  │ Migration Tool │ │
│  └────────────────┘  └────────────────┘  └────────────────┘ │
└───────────────────────────────────────────────────────────────┘
```

---

## System Context

**External Systems:**

```
┌──────────────────┐
│  User via CLI    │
│  Slack, Telegram │
└────────┬─────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│              NuimanBot Application                       │
│                                                          │
│  ┌────────────────────────────────────────────────┐    │
│  │       File-Based Storage System                │    │
│  │                                                 │    │
│  │   data/users/<user-id>/                        │    │
│  │     ├── profile.json                           │    │
│  │     ├── conversations/*.json                   │    │
│  │     ├── memory/cells/*.json                    │    │
│  │     ├── memory/scenes/*.json                   │    │
│  │     ├── memory/indexes/*.json                  │    │
│  │     └── notes/*.json                           │    │
│  │                                                 │    │
│  │   data/system/                                 │    │
│  │     └── audit.jsonl                            │    │
│  └────────────────────────────────────────────────┘    │
│                                                          │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
            ┌────────────────┐
            │  File System   │
            │  (Local Disk)  │
            └────────────────┘
```

**System Boundaries:**
- **Inputs:** User commands, API requests, webhook events
- **Outputs:** Chat responses, stored data files, audit logs
- **External Dependencies:**
  - File system (local disk)
  - Existing AtomicFileWriter utility
  - Go standard library (encoding/json, os, io)

**Integration Points:**

| System | Type | Protocol | Purpose |
|--------|------|----------|---------|
| File System | Local Storage | OS file I/O | Data persistence |
| Use Case Services | Internal | Go interfaces | Business logic orchestration |
| CLI/Gateways | Internal | Go interfaces | User interaction |
| Migration Tool | Internal | Go CLI | SQLite → Files migration |

---

## Component Architecture

### File Repository Components

```
┌─────────────────────────────────────────────────────────────┐
│              FileUserProfileRepository                       │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐ │
│  │  File Manager  │  │  JSON Codec    │  │    Cache     │ │
│  │  (read/write)  │  │ (marshal/un)   │  │  (optional)  │ │
│  └────────────────┘  └────────────────┘  └──────────────┘ │
│         │                    │                    │         │
│         └────────────────────┼────────────────────┘         │
│                              │                               │
│  Path: data/users/<user-id>/profile.json                    │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│           FileConversationRepository                         │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐ │
│  │  File Manager  │  │  JSON Codec    │  │  Index Mgr   │ │
│  │  (read/write)  │  │ (marshal/un)   │  │  (metadata)  │ │
│  └────────────────┘  └────────────────┘  └──────────────┘ │
│         │                    │                    │         │
│         └────────────────────┼────────────────────┘         │
│                              │                               │
│  Files: data/users/<user-id>/conversations/                 │
│    ├── <conv-id>.json     (full conversation + messages)    │
│    └── index.json          (conversation metadata)          │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│            FileMemoryCellRepository                          │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐ │
│  │  File Manager  │  │  JSON Codec    │  │  Index Mgr   │ │
│  │  (read/write)  │  │ (marshal/un)   │  │ (4 indexes)  │ │
│  └────────────────┘  └────────────────┘  └──────────────┘ │
│         │                    │                    │         │
│         └────────────────────┼────────────────────┘         │
│                              │                               │
│  Files: data/users/<user-id>/memory/                        │
│    ├── cells/<cell-id>.json       (individual cells)        │
│    └── indexes/                                              │
│        ├── by-scene.json           (scene → cell IDs)       │
│        ├── by-type.json            (type → cell IDs)        │
│        ├── by-salience.json        (sorted by salience)     │
│        └── search.json             (keyword → cell IDs)     │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│            FileMemorySceneRepository                         │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐                    │
│  │  File Manager  │  │  JSON Codec    │                    │
│  │  (read/write)  │  │ (marshal/un)   │                    │
│  └────────────────┘  └────────────────┘                    │
│         │                    │                               │
│         └────────────────────┘                               │
│                                                              │
│  Files: data/users/<user-id>/memory/scenes/                 │
│    └── <scene-name>.json   (scene summary)                  │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│               FileNotesRepository                            │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────┐ │
│  │  File Manager  │  │  JSON Codec    │  │  Index Mgr   │ │
│  │  (read/write)  │  │ (marshal/un)   │  │  (by tag)    │ │
│  └────────────────┘  └────────────────┘  └──────────────┘ │
│         │                    │                    │         │
│         └────────────────────┼────────────────────┘         │
│                              │                               │
│  Files: data/users/<user-id>/notes/                         │
│    ├── <note-id>.json     (individual notes)                │
│    └── index.json          (notes metadata + tag index)     │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│               FileAuditRepository                            │
│                                                              │
│  ┌────────────────┐  ┌────────────────┐                    │
│  │  JSONL Writer  │  │  JSONL Reader  │                    │
│  │  (append-only) │  │  (streaming)   │                    │
│  └────────────────┘  └────────────────┘                    │
│         │                    │                               │
│         └────────────────────┘                               │
│                                                              │
│  File: data/system/audit.jsonl                              │
│    (one JSON object per line, append-only)                  │
└──────────────────────────────────────────────────────────────┘
```

### Shared Infrastructure Components

```
┌─────────────────────────────────────────────────────────────┐
│                   AtomicFileWriter                           │
│  (Existing utility - reused for atomic writes)              │
│                                                              │
│  Flow: Write to temp file → Sync → Rename to target         │
│  Guarantees: No partial writes, crash-safe                  │
└──────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                   Index Manager                              │
│  (New component for managing JSON indexes)                  │
│                                                              │
│  - Load index on startup (lazy or eager)                    │
│  - Update index on data write                               │
│  - Rebuild index from data files (recovery)                 │
│  - Persist index atomically                                 │
└──────────────────────────────────────────────────────────────┘
```

---

## Layer Responsibilities

### Domain Layer

**Location:** `internal/domain/`, `internal/domain/memoryv2/`

**Responsibility:**
- Define core business entities (UserProfile, Conversation, MemoryCell, etc.)
- Specify business rules and validation logic
- Define repository interfaces (contracts for persistence)

**File-Based Storage Impact:**
- **No changes to entity definitions** (UserProfile, Conversation, MemoryCell, etc. remain unchanged)
- **No changes to repository interfaces** (Same method signatures)
- Repository implementations swap from SQLite to file-based

**Example:**
```go
// Domain entity (unchanged)
type UserProfile struct {
    UserID       string
    PrimaryEmail string
    // ... fields
}

// Domain interface (unchanged)
type UserProfileRepository interface {
    SaveProfile(ctx context.Context, profile *UserProfile) error
    GetProfileByUserID(ctx context.Context, userID string) (*UserProfile, error)
    // ... methods
}
```

---

### Use Case Layer

**Location:** `internal/usecase/[feature]/`

**Responsibility:**
- Orchestrate business logic using domain entities
- Call repository interfaces (agnostic to implementation)
- Coordinate between multiple repositories

**File-Based Storage Impact:**
- **No changes required** - Use cases depend on repository interfaces, not implementations
- Dependency injection provides file-based implementations instead of SQLite

**Example:**
```go
// Use case service (unchanged)
type ChatService struct {
    convRepo    domain.ConversationRepository  // Interface
    memoryRepo  memoryv2.MemoryCellRepository  // Interface
    profileRepo domain.UserProfileRepository   // Interface
}

// Business logic unchanged - works with any repository implementation
func (s *ChatService) HandleMessage(ctx context.Context, msg IncomingMessage) error {
    conv, err := s.convRepo.GetConversation(ctx, msg.ConversationID)
    // ... orchestration logic
}
```

---

### Infrastructure Layer

**Location:** `internal/infrastructure/storage/`

**Responsibility:**
- Implement domain repository interfaces using file-based storage
- Handle JSON serialization/deserialization
- Manage file I/O and atomic writes
- Maintain indexes for fast access

**File-Based Storage Impact:**
- **New implementations created** - File-based repository implementations
- Old SQLite implementations remain (for migration tool)

**Example:**
```go
// File-based implementation
type FileUserProfileRepository struct {
    basePath string       // Base directory: data/users/
    mu       sync.RWMutex // Concurrency control
    cache    *ProfileCache // Optional in-memory cache
}

func (r *FileUserProfileRepository) SaveProfile(ctx context.Context, profile *UserProfile) error {
    // 1. Validate entity
    if err := profile.Validate(); err != nil {
        return fmt.Errorf("invalid profile: %w", err)
    }

    // 2. Marshal to JSON
    data, err := json.MarshalIndent(profile, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal failed: %w", err)
    }

    // 3. Write atomically
    path := filepath.Join(r.basePath, profile.UserID, "profile.json")
    if err := AtomicFileWriter.Write(path, data); err != nil {
        return fmt.Errorf("write failed: %w", err)
    }

    // 4. Update cache
    if r.cache != nil {
        r.cache.Set(profile.UserID, profile)
    }

    return nil
}
```

---

### Adapter Layer

**Location:** `internal/adapter/cli/`, `cmd/migrate/`

**Responsibility:**
- CLI command handlers (unchanged - use repository interfaces)
- Migration utility (new - SQLite → Files conversion)

**File-Based Storage Impact:**
- **New migration commands added** (`migrate sqlite-to-files`, `migrate rollback`, etc.)
- Existing CLI commands unchanged (use repository interfaces)

---

## Data Flow

### Write Flow: Save Conversation

**Sequence:**
```
User → CLI → ChatService → ConversationRepo → File System

1. User sends message via CLI
2. CLI adapter calls ChatService.HandleMessage()
3. ChatService appends message to conversation
4. ChatService calls convRepo.SaveConversation(conv)
5. FileConversationRepository:
   a. Validates conversation
   b. Marshals to JSON
   c. Writes atomically: conversations/<conv-id>.json
   d. Updates index: conversations/index.json
6. File system persists data
```

**Detailed Steps:**
```go
// 1. User message arrives
msg := domain.IncomingMessage{
    ID: "msg-123",
    Text: "Hello",
}

// 2. Chat service orchestrates
func (s *ChatService) HandleMessage(ctx context.Context, msg IncomingMessage) error {
    // Load conversation
    conv, err := s.convRepo.GetConversation(ctx, msg.ConversationID)
    if err != nil {
        return err
    }

    // Append message
    conv.Messages = append(conv.Messages, domain.StoredMessage{
        ID: msg.ID,
        Role: "user",
        Content: msg.Text,
        Timestamp: time.Now(),
    })
    conv.UpdatedAt = time.Now()

    // Save (triggers file write)
    return s.convRepo.SaveConversation(ctx, conv)
}

// 3. File repository writes
func (r *FileConversationRepository) SaveConversation(ctx context.Context, conv *Conversation) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Validate
    if err := validateConversation(conv); err != nil {
        return err
    }

    // Marshal
    data, err := json.MarshalIndent(conv, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal: %w", err)
    }

    // Write atomically
    path := r.getConversationPath(conv.UserID, conv.ID)
    if err := r.atomicWriter.Write(path, data); err != nil {
        return fmt.Errorf("write: %w", err)
    }

    // Update index
    return r.updateIndex(conv)
}
```

---

### Read Flow: Load Conversation

**Sequence:**
```
User → CLI → ChatService → ConversationRepo → File System

1. User requests conversation listing
2. CLI calls ChatService.ListConversations(userID)
3. ChatService calls convRepo.ListConversations(userID)
4. FileConversationRepository:
   a. Reads index file: conversations/index.json
   b. Returns conversation summaries (no full load)
5. User selects conversation
6. CLI calls ChatService.GetConversation(convID)
7. FileConversationRepository:
   a. Reads file: conversations/<conv-id>.json
   b. Unmarshals JSON to Conversation struct
   c. Returns conversation with all messages
```

**Performance Optimization:**
- Index read is fast (<100ms) - small JSON file
- Full conversation load only when needed
- Cache frequently accessed conversations

---

### Search Flow: Memory Search

**Sequence:**
```
User → CLI → MemoryService → MemoryCellRepo → Index + File System

1. User searches: "typescript projects"
2. CLI calls MemoryService.Search(query)
3. MemoryService calls cellRepo.SearchFTS(query, limit)
4. FileMemoryCellRepository:
   a. Parses query into keywords: ["typescript", "projects"]
   b. Reads search index: memory/indexes/search.json
   c. Finds cell IDs for each keyword:
      - "typescript" → ["cell-001", "cell-007"]
      - "projects" → ["cell-001", "cell-003", "cell-007"]
   d. Computes intersection: ["cell-001", "cell-007"]
   e. Loads cells from disk:
      - memory/cells/cell-001.json
      - memory/cells/cell-007.json
   f. Ranks by salience (or relevance score)
   g. Returns top N results
```

**Search Algorithm:**
```go
func (r *FileMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error) {
    // 1. Parse query to keywords
    keywords := parseKeywords(query) // ["typescript", "projects"]

    // 2. Load search index
    searchIndex, err := r.loadSearchIndex(ctx)
    if err != nil {
        return nil, err
    }

    // 3. Find cell IDs for each keyword
    cellIDSets := make([]map[string]bool, len(keywords))
    for i, keyword := range keywords {
        cellIDs := searchIndex.Keywords[keyword] // ["cell-001", "cell-007"]
        cellIDSets[i] = toSet(cellIDs)
    }

    // 4. Compute intersection (cells matching ALL keywords)
    matchingIDs := intersect(cellIDSets)

    // 5. Load matching cells
    cells := make([]*MemoryCell, 0, len(matchingIDs))
    for cellID := range matchingIDs {
        cell, err := r.Get(ctx, cellID)
        if err != nil {
            continue // Skip errors, log
        }
        cells = append(cells, cell)
    }

    // 6. Rank by salience
    sort.Slice(cells, func(i, j int) bool {
        return cells[i].Salience > cells[j].Salience
    })

    // 7. Return top N
    if len(cells) > limit {
        cells = cells[:limit]
    }
    return cells, nil
}
```

---

### Migration Flow: SQLite → Files

**Sequence:**
```
Migration Tool → SQLite Repos → Domain Entities → File Repos → File System

1. User runs: nuimanbot migrate sqlite-to-files --verify
2. Migration tool:
   a. Opens SQLite database (read-only)
   b. Backs up database file
   c. Creates file directory structure
   d. For each entity type:
      - Query all records from SQLite
      - Convert to domain entities
      - Write to file-based storage
      - Validate written data
      - Update progress
   e. Build all indexes from scratch
   f. Validate data integrity (counts, checksums)
   g. Report success/failure
3. User validates migrated data
4. User switches main.go to use file repositories
```

---

## Sequence Diagrams

### Sequence 1: Create User Profile

```
User     CLI     ProfileSvc    FileProfileRepo    FileSystem
 |        |           |               |                |
 |─create─>           |               |                |
 |        |─validate─>|               |                |
 |        |           |─SaveProfile──>|                |
 |        |           |               |─validate──┐    |
 |        |           |               |           │    |
 |        |           |               |<──────────┘    |
 |        |           |               |─marshal───┐    |
 |        |           |               |           │    |
 |        |           |               |<──────────┘    |
 |        |           |               |─atomic write──>|
 |        |           |               |                |─write temp─┐
 |        |           |               |                |            │
 |        |           |               |                |<───────────┘
 |        |           |               |                |─sync───┐
 |        |           |               |                |        │
 |        |           |               |                |<───────┘
 |        |           |               |                |─rename─┐
 |        |           |               |                |        │
 |        |           |               |                |<───────┘
 |        |           |               |<──success──────|
 |        |           |<──success─────|                |
 |        |<──profile─|               |                |
 |<─ok────|           |               |                |
```

**Steps:**
1. User provides profile data via CLI
2. CLI validates input, calls ProfileService
3. ProfileService calls repository.SaveProfile()
4. Repository validates domain entity
5. Repository marshals to JSON
6. Repository uses atomic write:
   - Write to temp file
   - Sync to disk (fsync)
   - Rename to target (atomic)
7. Success flows back to user

---

### Sequence 2: Load Conversation with Messages

```
User     CLI     ChatSvc    FileConvRepo    FileSystem
 |        |         |             |              |
 |─get────>         |             |              |
 |        |─get────>|             |              |
 |        |         |─GetConv────>|              |
 |        |         |             |─read file───>|
 |        |         |             |              |─read─┐
 |        |         |             |              |      │
 |        |         |             |              |<─────┘
 |        |         |             |<──JSON data──|
 |        |         |             |─unmarshal──┐ |
 |        |         |             |            │ |
 |        |         |             |<───────────┘ |
 |        |         |<──conv──────|              |
 |        |<─conv───|             |              |
 |<─result─         |             |              |
```

**Steps:**
1. User requests conversation
2. CLI calls ChatService.GetConversation()
3. ChatService calls repository.GetConversation()
4. Repository reads JSON file from disk
5. Repository unmarshals JSON to Conversation struct
6. Conversation (with all messages) returned

**Optimization:**
- Cache recently accessed conversations
- Index provides metadata without full load

---

### Sequence 3: Memory Cell Search

```
User    CLI    MemorySvc    FileCellRepo    SearchIndex    FileSystem
 |       |         |              |               |             |
 |─search─>        |              |               |             |
 |       |─search─>|              |               |             |
 |       |         |─SearchFTS───>|               |             |
 |       |         |              |─load index───>|             |
 |       |         |              |               |─read file──>|
 |       |         |              |               |<──JSON──────|
 |       |         |              |<─index────────|             |
 |       |         |              |─parse query─┐ |             |
 |       |         |              |             │ |             |
 |       |         |              |<────────────┘ |             |
 |       |         |              |─lookup IDs──┐ |             |
 |       |         |              |             │ |             |
 |       |         |              |<────────────┘ |             |
 |       |         |              |─load cells──────────────────>|
 |       |         |              |<──cell data──────────────────|
 |       |         |              |─rank by salience─┐            |
 |       |         |              |                  │            |
 |       |         |              |<─────────────────┘            |
 |       |         |<─results─────|                               |
 |       |<─cells──|              |                               |
 |<─results        |              |                               |
```

**Steps:**
1. User enters search query
2. CLI calls MemoryService.Search()
3. MemoryService calls repository.SearchFTS()
4. Repository loads search index (keyword → cell IDs)
5. Repository parses query into keywords
6. Repository looks up cell IDs for each keyword
7. Repository computes intersection (AND query)
8. Repository loads matching cell files
9. Repository ranks by salience
10. Results returned to user

---

### Sequence 4: Migration (SQLite → Files)

```
MigrateTool  SQLiteRepo  DomainEntity  FileRepo  FileSystem
     |            |            |           |          |
     |─backup DB─────────────────────────────────────>|
     |<─ok────────────────────────────────────────────|
     |─query all─>|            |           |          |
     |<─rows──────|            |           |          |
     |─for each row            |           |          |
     |  |─convert──>           |           |          |
     |  |<─entity──┘           |           |          |
     |  |─validate─>           |           |          |
     |  |<─ok──────┘           |           |          |
     |  |─save─────────────────>          |          |
     |  |                      |─write────>          |
     |  |                      |          |─atomic──>|
     |  |                      |<─ok──────|          |
     |  |<─ok──────────────────┘          |          |
     |  |                                             |
     |  └─repeat for all rows                        |
     |                                                |
     |─build indexes──────────────────────>          |
     |                                     |─write───>|
     |<─ok─────────────────────────────────          |
     |                                                |
     |─validate counts/checksums                     |
     |<─ok───────────────────────────────────────────|
```

---

## Integration Points

### Integration 1: File System

**Type:** Local File System
**Purpose:** Persistent data storage
**Protocol:** OS file I/O (POSIX)

**Operations:**
- `os.OpenFile()` - Open files for reading/writing
- `os.Rename()` - Atomic file rename (commit write)
- `os.Remove()` - Delete files
- `os.Mkdir()` / `os.MkdirAll()` - Create directories
- `io.ReadAll()` - Read file contents
- `io.WriteString()` - Write file contents

**Directory Structure:**
```
data/
├── users/
│   └── <user-id>/
│       ├── profile.json
│       ├── conversations/
│       ├── memory/
│       └── notes/
└── system/
    └── audit.jsonl
```

**Error Handling:**
- Permission errors: Return `ErrUnauthorized`
- File not found: Return `ErrNotFound`
- Disk full: Return wrapped error with context
- Corruption: Attempt index rebuild, return error if fails

---

### Integration 2: Existing AtomicFileWriter

**Type:** Internal Utility
**Purpose:** Crash-safe atomic file writes

**Pattern:**
```go
// AtomicFileWriter pattern
func Write(path string, data []byte) error {
    // 1. Create temp file in same directory
    tempPath := path + ".tmp"

    // 2. Write data to temp
    if err := ioutil.WriteFile(tempPath, data, 0644); err != nil {
        return err
    }

    // 3. Sync to disk (fsync)
    f, err := os.Open(tempPath)
    if err != nil {
        return err
    }
    if err := f.Sync(); err != nil {
        f.Close()
        return err
    }
    f.Close()

    // 4. Atomic rename (commit)
    return os.Rename(tempPath, path)
}
```

**Guarantees:**
- No partial writes visible
- Crash-safe (write complete or not visible)
- Works on all POSIX systems

---

### Integration 3: JSON Codec (encoding/json)

**Type:** Standard Library
**Purpose:** Serialize/deserialize domain entities

**Usage:**
```go
// Marshal (struct → JSON)
data, err := json.MarshalIndent(entity, "", "  ")

// Unmarshal (JSON → struct)
var entity Entity
err := json.Unmarshal(data, &entity)
```

**Considerations:**
- Use `json:",omitempty"` for optional fields
- Use `time.Time` RFC3339 format (automatic)
- Handle null vs empty array distinction
- Validate after unmarshal

---

## Architectural Decisions

### ADR-001: File-Based Storage Over SQLite

**Date:** 2026-02-16
**Status:** Accepted

**Context:**
NuimanBot currently uses SQLite for conversations, messages, notes, and memory (with FTS5 search). This creates complexity: dual storage systems (SQLite + files for profiles/configs), opaque database files, and SQLite driver dependencies.

**Decision:**
Migrate all persistent data to file-based storage using JSON/JSONL formats.

**Rationale:**
1. **Simplification**: Single storage mechanism across entire application
2. **Portability**: Human-readable files, easy to inspect and modify
3. **Backup**: Simple file copy, no database dump/restore
4. **Version Control**: Text files can be versioned (though not recommended for production)
5. **No Dependencies**: Pure Go, no CGO, no native dependencies
6. **Sufficient Performance**: CLI tool with single user at a time, file I/O is fast enough

**Consequences:**

**Positive:**
- Simplified deployment (no database setup)
- Improved data portability (copy directories)
- Easier debugging (cat/jq files)
- Reduced dependencies (remove mattn/go-sqlite3, modernc.org/sqlite)
- Unified architecture (files for everything)

**Negative:**
- No ACID transactions (mitigated by atomic writes per file)
- No built-in FTS (mitigated by keyword search index)
- Must manage indexes manually (mitigated by index manager component)
- Potential performance degradation for large datasets (acceptable for CLI use case)

**Alternatives Considered:**

1. **Keep SQLite**
   - Pros: Mature, fast, FTS5 search
   - Cons: Complexity, dependencies, opaque files
   - Rejected: Complexity outweighs benefits for CLI tool

2. **Embedded Database (BoltDB, BadgerDB)**
   - Pros: Fast, no CGO, ACID
   - Cons: Still opaque files, adds dependency
   - Rejected: Doesn't solve portability/simplicity goals

---

### ADR-002: User-Centric Directory Structure

**Date:** 2026-02-16
**Status:** Accepted

**Context:**
Need to organize file storage. Two approaches: flat (all files in one directory) or hierarchical (user-centric directories).

**Decision:**
Use user-centric directory structure: `data/users/<user-id>/`

**Rationale:**
1. **Isolation**: User data isolated by directory
2. **Easy Backup**: Copy user directory to backup user data
3. **Easy Deletion**: Remove user directory to delete user
4. **Portability**: Move user directory to migrate user
5. **Organization**: Clear data ownership

**Consequences:**

**Positive:**
- Clear data boundaries
- Simple backup/restore per user
- Easy user migration
- Scales to multi-user scenarios

**Negative:**
- More directories to manage
- Slightly more complex path resolution

**Alternatives Considered:**

1. **Flat Structure** (`data/conversations/`, `data/notes/`)
   - Rejected: Hard to isolate user data, complex backup/deletion

---

### ADR-003: JSON for Structured Data, JSONL for Append-Only

**Date:** 2026-02-16
**Status:** Accepted

**Context:**
Need to choose file format for different data types.

**Decision:**
- **JSON** for structured entities (profiles, conversations, notes, memory)
- **JSONL** for append-only logs (audit logs)
- **Markdown** for human-authored content (personas)

**Rationale:**
1. JSON: Human-readable, well-supported, easy to parse
2. JSONL: Efficient for append operations, streaming-friendly
3. Markdown: Best for human-authored, narrative content

**Consequences:**

**Positive:**
- JSON is ubiquitous, tooling available (jq, etc.)
- JSONL prevents loading entire file for audit log queries
- Clear separation of concerns

**Negative:**
- Slightly larger than binary formats (acceptable trade-off)

---

### ADR-004: Keyword Search Over FTS5

**Date:** 2026-02-16
**Status:** Accepted

**Context:**
SQLite FTS5 provides full-text search. File-based storage needs search alternative.

**Decision:**
Implement simple keyword-based search using inverted index (keyword → cell IDs).

**Rationale:**
1. **Simplicity**: Easy to implement and maintain
2. **No Dependencies**: Pure Go implementation
3. **Sufficient**: CLI use case doesn't need fuzzy matching or ranking
4. **Extensible**: Can add bleve later if needed

**Consequences:**

**Positive:**
- Simple implementation
- Fast for exact keyword matches
- Controllable and debuggable

**Negative:**
- No fuzzy matching
- No relevance ranking (use salience instead)
- Limited query syntax (AND only)

**Alternatives Considered:**

1. **Bleve** (Go FTS library)
   - Pros: Full-text search, ranking
   - Cons: Additional dependency, complexity
   - Decision: Start with keyword search, add bleve if user feedback indicates need

---

### ADR-005: In-Memory Indexes with Lazy Rebuild

**Date:** 2026-02-16
**Status:** Accepted

**Context:**
Need fast lookups without scanning all files. Indexes can be persisted or in-memory.

**Decision:**
- Persist indexes as JSON files
- Load indexes into memory on startup (lazy or eager)
- Rebuild from data files if index corrupted/missing
- Update index atomically with data writes

**Rationale:**
1. **Fast Startup**: Load indexes faster than scanning all data files
2. **Recovery**: Can rebuild from data files if needed
3. **Consistency**: Update index with each write
4. **Simplicity**: JSON indexes, same format as data

**Consequences:**

**Positive:**
- Fast lookups (in-memory index)
- Automatic recovery (rebuild on corruption)
- Simple format (JSON)

**Negative:**
- Index must stay in sync with data
- Memory usage for large indexes (acceptable for CLI)

---

## Trade-offs

### Trade-off 1: Simplicity vs. Performance

**Choice:** File-based storage over SQLite

**Benefits:**
- Simpler deployment (no database setup)
- Portable data (copy directories)
- Human-readable (inspect with cat/jq)
- No native dependencies (pure Go)

**Costs:**
- Potentially slower for large datasets
- No ACID transactions
- Manual index management
- No built-in FTS

**Mitigation:**
- Atomic file writes ensure consistency
- Caching for frequently accessed data
- CLI use case: single user, small datasets
- Keyword search sufficient for CLI tool

---

### Trade-off 2: Write Performance vs. Data Safety

**Choice:** Atomic writes (write-to-temp + sync + rename)

**Benefits:**
- Crash-safe (no partial writes)
- Data integrity guaranteed
- Works on all POSIX systems

**Costs:**
- Slightly slower writes (fsync overhead)
- Temp file creation overhead
- Double disk space briefly (temp + final)

**Mitigation:**
- Acceptable for CLI tool (not high-throughput)
- fsync cost amortized over message content
- Temp files cleaned up automatically

---

### Trade-off 3: Index Accuracy vs. Memory Usage

**Choice:** In-memory indexes with lazy rebuild

**Benefits:**
- Fast lookups (O(1) for index queries)
- Automatic recovery (rebuild from data)
- Consistent with data (updated on write)

**Costs:**
- Memory usage scales with data size
- Index rebuild time on corruption

**Mitigation:**
- CLI use case: small datasets
- Lazy loading (load index only when needed)
- Indexes much smaller than full data

---

## Performance Considerations

### Read Performance

**Target Latencies (p90):**
- Profile read: <50ms
- Conversation read: <100ms
- Memory search: <500ms
- Notes list: <100ms

**Optimizations:**
1. **Indexes**: In-memory indexes for fast metadata queries
2. **Caching**: LRU cache for frequently accessed entities
3. **Lazy Loading**: Load only when needed (index first, full load on demand)
4. **Concurrent Reads**: RWMutex allows concurrent readers

**Bottlenecks:**
- File I/O: SSD recommended (HDD acceptable)
- JSON parsing: Acceptable for CLI tool
- Search: Keyword index makes search fast

---

### Write Performance

**Target Latencies (p90):**
- Profile save: <100ms
- Message append: <200ms
- Memory cell create: <150ms
- Note save: <100ms

**Optimizations:**
1. **Atomic Writes**: Single fsync per write
2. **Buffering**: Buffer multiple messages before write (optional)
3. **Index Updates**: Batch index updates (optional)

**Bottlenecks:**
- fsync latency: Necessary for data safety
- Index updates: Small overhead, acceptable

---

### Memory Usage

**Expected:**
- Indexes: ~1-10MB per 10,000 cells
- Cache: Configurable (e.g., 100 profiles, 50 conversations)
- Streaming: JSONL audit log read with streaming (no full load)

**Optimizations:**
1. **LRU Cache**: Evict least recently used
2. **Streaming**: Process JSONL line-by-line
3. **Lazy Loading**: Load on demand, not eagerly

---

### Scalability

**Current Limits:**
- Users: 1,000+ (limited by disk space, not architecture)
- Conversations per user: 1,000+ (index-based listing)
- Messages per conversation: 10,000+ (single JSON file)
- Memory cells per user: 100,000+ (individual files + indexes)

**Future Considerations:**
- Large conversations: Split into multiple files (e.g., by date)
- Archive old data: Move old conversations to archive directory
- Compression: Gzip old files to save space

---

## References

- [spec.md](spec.md) - Feature specification
- [data-dictionary.md](data-dictionary.md) - Data structures and schemas
- [plan.md](plan.md) - Implementation plan
- [research.md](research.md) - Research findings and decisions

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-16 | Initial architecture design complete |
