# File-Based Storage Migration - Research

**Created:** 2026-02-16
**Phase:** Phase 1 - Preparation & Design
**Status:** In Progress

---

## Table of Contents

1. [Current State Analysis](#current-state-analysis)
2. [SQLite Schema Analysis](#sqlite-schema-analysis)
3. [File Format Decisions](#file-format-decisions)
4. [Directory Structure Design](#directory-structure-design)
5. [Performance Considerations](#performance-considerations)
6. [Indexing Strategy](#indexing-strategy)
7. [Migration Path](#migration-path)

---

## Current State Analysis

### Current Storage Architecture

**SQLite Database (`nuimanbot.db`):**
- Users table
- Conversations table
- Messages table
- Notes table
- Audit logs (via SQLiteAuditor)

**SQLite Database (`nuimanbot-memory.db`):**
- Memory cells table
- Memory scenes table
- FTS5 full-text search index

**File-Based Storage (Already Exists):**
- User profiles: `data/users.json` (FileUserProfileRepository)
- Bot configurations: `data/bots.json` (FileBotConfigRepository)
- Persona files: `~/.nuimanbot/personas/<user-id>/` (SOUL.md, USER.md, RULES.md)

### Pain Points Identified

1. **Dual Storage Complexity**: Managing both SQLite and file-based storage
2. **Data Fragmentation**: User data split across database and filesystem
3. **Dependency Management**: SQLite drivers (mattn/go-sqlite3 + modernc.org/sqlite for FTS5)
4. **Backup Complexity**: Must backup both database files and filesystem separately
5. **Portability**: Database files are opaque, require tools to inspect
6. **FTS5 Search**: Need to replace full-text search capability

---

## SQLite Schema Analysis

### Core Tables Schema

**Users Table:**
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    platform_uid TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(platform, platform_uid)
)
```

**Conversations Table:**
```sql
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
)
```

**Messages Table:**
```sql
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_calls TEXT,
    tool_results TEXT,
    token_count INTEGER DEFAULT 0,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
)
```

**Notes Table:**
```sql
CREATE TABLE notes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES users(id)
)
```

### Memory V2 Tables Schema

**Memory Cells Table:**
```sql
CREATE TABLE memory_cells (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    scene TEXT NOT NULL,
    cell_type TEXT NOT NULL,
    salience REAL NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,
    CONSTRAINT chk_salience CHECK (salience >= 0.0 AND salience <= 1.0),
    CONSTRAINT chk_cell_type CHECK (cell_type IN ('fact', 'decision', 'task', 'preference', 'plan', 'risk'))
)
```

**Memory Scenes Table:**
```sql
CREATE TABLE memory_scenes (
    scene TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT chk_token_count CHECK (token_count > 0 AND token_count <= 2000)
)
```

**FTS5 Virtual Table:**
```sql
CREATE VIRTUAL TABLE memory_cells_fts USING fts5(
    content,
    scene,
    cell_type,
    content='memory_cells',
    content_rowid='rowid'
)
```

### Key Observations

1. **Simple Schema**: No complex joins or transactions required
2. **Text IDs**: All IDs are TEXT (UUIDs/slugs), easy to serialize
3. **Foreign Keys**: Used for referential integrity, must maintain in file system
4. **Indexes**: Several indexes for performance (conversation_id, user_id, timestamps)
5. **FTS5**: Full-text search on memory cells needs alternative implementation
6. **JSON Fields**: tool_calls, tool_results stored as JSON TEXT

---

## File Format Decisions

### Chosen Formats

**Primary Format: JSON**
- **Pros**: Human-readable, well-supported in Go, easy to edit/inspect
- **Cons**: Slightly larger than binary formats
- **Use Cases**: All structured data (profiles, conversations, notes, memory)

**Secondary Format: JSONL (JSON Lines)**
- **Pros**: Append-friendly, streaming-friendly, crash-resistant
- **Cons**: Slightly harder to parse entire file
- **Use Cases**: Audit logs, conversation messages (time-series data)

**Tertiary Format: Markdown**
- **Pros**: Human-readable, git-friendly, easy to edit
- **Cons**: Requires parsing for structured data
- **Use Cases**: Persona files (SOUL.md, USER.md, RULES.md) - already in use

### JSON Structure Examples

**Profile (profile.json):**
```json
{
  "userID": "user-123",
  "identity": {
    "username": "alice",
    "moniker": "Alice Cooper",
    "firstName": "Alice"
  },
  "authentication": {
    "role": "admin",
    "platformIDs": {
      "cli": "alice",
      "slack": "U123"
    }
  },
  "contact": {
    "primaryEmail": "alice@example.com"
  },
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-02-16T00:00:00Z"
}
```

**Conversation (conversations/conv-123.json):**
```json
{
  "id": "conv-123",
  "userID": "user-123",
  "platform": "cli",
  "messages": [
    {
      "id": "msg-001",
      "role": "user",
      "content": "Hello",
      "timestamp": "2026-02-16T10:00:00Z",
      "tokenCount": 5
    },
    {
      "id": "msg-002",
      "role": "assistant",
      "content": "Hi there!",
      "timestamp": "2026-02-16T10:00:01Z",
      "tokenCount": 7,
      "toolCalls": null,
      "toolResults": null
    }
  ],
  "createdAt": "2026-02-16T10:00:00Z",
  "updatedAt": "2026-02-16T10:00:01Z"
}
```

**Memory Cell (memory/cells/cell-001.json):**
```json
{
  "id": "cell-001",
  "conversationID": "conv-123",
  "scene": "project-planning",
  "cellType": "fact",
  "salience": 0.8,
  "content": "User prefers TypeScript for new projects",
  "source": "conversation",
  "createdAt": "2026-02-16T10:00:00Z",
  "updatedAt": "2026-02-16T10:00:00Z",
  "expiresAt": null
}
```

**Note (notes/note-456.json):**
```json
{
  "id": "note-456",
  "userID": "user-123",
  "title": "Meeting Notes",
  "content": "Discussed Q1 roadmap...",
  "tags": ["meeting", "roadmap"],
  "createdAt": "2026-02-16T09:00:00Z",
  "updatedAt": "2026-02-16T09:30:00Z"
}
```

**Audit Log (audit.jsonl):**
```jsonl
{"timestamp":"2026-02-16T10:00:00Z","userID":"user-123","action":"login","platform":"cli","success":true}
{"timestamp":"2026-02-16T10:01:00Z","userID":"user-123","action":"chat","conversationID":"conv-123","success":true}
```

---

## Directory Structure Design

### User-Centric Structure

```
data/
├── users/
│   └── <user-id>/
│       ├── profile.json                    # Unified user profile
│       ├── conversations/
│       │   ├── <conversation-id>.json      # Full conversation with messages
│       │   └── index.json                  # Conversation list metadata
│       ├── memory/
│       │   ├── cells/
│       │   │   └── <cell-id>.json          # Individual memory cells
│       │   ├── scenes/
│       │   │   └── <scene-name>.json       # Scene summaries
│       │   └── indexes/
│       │       ├── by-scene.json           # Scene → cells mapping
│       │       ├── by-type.json            # Type → cells mapping
│       │       └── by-salience.json        # Salience-sorted cell IDs
│       ├── notes/
│       │   ├── <note-id>.json              # Individual notes
│       │   └── index.json                  # Notes metadata
│       └── personas/
│           ├── SOUL.md                     # Bot personality
│           ├── USER.md                     # User context
│           └── RULES.md                    # Behavior rules
├── system/
│   ├── audit.jsonl                         # System-wide audit log
│   └── bots.json                           # Bot configurations (existing)
└── cache/                                   # Optional performance cache
    └── <user-id>/
        └── conversation-summaries.json     # Cached summaries
```

### Design Rationale

**User-Centric:**
- All user data under single directory: easy backup/restore/deletion
- Portable: copy directory to migrate user data
- Privacy: user data isolated by directory

**Index Files:**
- Quick lookup without scanning all files
- Trade-off: must keep indexes in sync with data
- Rebuild-able from source data if corrupted

**Separate System Directory:**
- System-wide data (audit logs, bot configs) not user-specific
- Easier to manage separately from user data

---

## Performance Considerations

### Read Performance

**Conversation Loading:**
- **Current**: Single SQL query with JOIN
- **Future**: Single file read (JSON parse)
- **Optimization**: Keep file sizes reasonable (<1MB per conversation)
- **Mitigation**: Archive old conversations to separate files

**Memory Search:**
- **Current**: FTS5 full-text search (very fast)
- **Future**: In-memory search with indexes
- **Optimization**: Build search index on startup
- **Alternative**: Consider lightweight search library (bleve) if needed

**Notes Retrieval:**
- **Current**: SQL query with indexes
- **Future**: Index file + individual file reads
- **Optimization**: Cache index in memory

### Write Performance

**Atomic Writes:**
- Use existing `AtomicFileWriter` (write to temp → rename)
- Ensures no partial writes on crash
- OS-level atomicity guarantee

**Index Updates:**
- Update indexes after successful data write
- Use write-through cache pattern
- Rebuild indexes on corruption detection

### Memory Usage

**Trade-offs:**
- File-based: Higher memory for indexes
- SQLite: Database manages memory efficiently
- **Mitigation**: Lazy-load data, cache frequently accessed items

### Benchmarks (Target)

Based on typical CLI use case:

| Operation | Target Latency (p90) | Notes |
|-----------|---------------------|--------|
| Read profile | <50ms | Single file read |
| Load conversation | <100ms | Parse JSON, rebuild messages |
| Search memory | <500ms | In-memory search with indexes |
| Write note | <200ms | Atomic write + index update |
| List conversations | <100ms | Read index file |

---

## Indexing Strategy

### Conversation Index

**Purpose**: Fast conversation listing without loading full files

**Structure (conversations/index.json):**
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
    }
  ],
  "updatedAt": "2026-02-16T10:30:00Z"
}
```

### Memory Indexes

**By Scene (memory/indexes/by-scene.json):**
```json
{
  "project-planning": ["cell-001", "cell-003", "cell-007"],
  "architecture": ["cell-002", "cell-005"],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

**By Type (memory/indexes/by-type.json):**
```json
{
  "fact": ["cell-001", "cell-002"],
  "decision": ["cell-003", "cell-005"],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

**By Salience (memory/indexes/by-salience.json):**
```json
{
  "cells": [
    {"id": "cell-003", "salience": 0.95},
    {"id": "cell-001", "salience": 0.80},
    {"id": "cell-002", "salience": 0.65}
  ],
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

**Search Index (memory/indexes/search.json):**
```json
{
  "keywords": {
    "typescript": ["cell-001", "cell-007"],
    "planning": ["cell-001", "cell-003"],
    "architecture": ["cell-002", "cell-005"]
  },
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

### Notes Index

**Structure (notes/index.json):**
```json
{
  "notes": [
    {
      "id": "note-456",
      "title": "Meeting Notes",
      "tags": ["meeting", "roadmap"],
      "createdAt": "2026-02-16T09:00:00Z",
      "updatedAt": "2026-02-16T09:30:00Z"
    }
  ],
  "byTag": {
    "meeting": ["note-456", "note-789"],
    "roadmap": ["note-456"]
  },
  "updatedAt": "2026-02-16T10:00:00Z"
}
```

### Index Maintenance Strategy

**On Write:**
1. Write data file atomically
2. Update relevant indexes
3. Persist updated indexes

**On Startup:**
1. Load indexes into memory
2. Validate index freshness (compare timestamps)
3. Rebuild indexes if stale or corrupted

**Rebuild Logic:**
- Scan all data files
- Reconstruct indexes from scratch
- Persist rebuilt indexes

---

## Migration Path

### Migration Phases

**Phase 1: Dual-Write Mode (Optional)**
- Write to both SQLite and files
- Validate file writes work correctly
- Test rollback scenarios

**Phase 2: Migration Execution**
1. Backup SQLite databases
2. Export all users, conversations, messages, notes, memory
3. Create user directories
4. Write JSON files
5. Build indexes
6. Validate data integrity

**Phase 3: File-Only Mode**
1. Switch repositories to file-based implementations
2. Keep SQLite backup for 30 days
3. Monitor performance and errors

### Migration Utility Features

**Commands:**
```bash
# Dry run - simulate migration
nuimanbot migrate sqlite-to-files --dry-run

# Execute migration with verification
nuimanbot migrate sqlite-to-files --verify

# Rollback to SQLite
nuimanbot migrate rollback-files-to-sqlite

# Validate migrated data
nuimanbot migrate validate
```

**Progress Reporting:**
- Total records to migrate
- Current progress (users, conversations, notes, memory)
- Estimated time remaining
- Errors/warnings

**Rollback Strategy:**
- Keep SQLite backup
- Restore from backup if migration fails
- Validate restored data

---

## FTS5 Replacement Options

### Option 1: Simple Keyword Search (Recommended for MVP)

**Approach:**
- Split content into keywords
- Build inverted index: keyword → cell IDs
- Search by keyword matching

**Pros:**
- Simple to implement
- No external dependencies
- Fast for exact keyword matches

**Cons:**
- No fuzzy matching
- No relevance ranking
- Limited query syntax

### Option 2: Bleve (Lightweight Search Library)

**Approach:**
- Use `github.com/blevesearch/bleve` (pure Go)
- Build search index on startup
- Full-text search with ranking

**Pros:**
- Full-text search capabilities
- Relevance ranking
- Query syntax support
- Pure Go (no CGO)

**Cons:**
- External dependency
- More complex implementation
- Higher memory usage

### Option 3: Hybrid Approach

**Approach:**
- Use simple keyword search by default
- Add Bleve as optional enhancement
- Feature flag to enable advanced search

**Decision**: Start with Option 1 (simple keyword search), add Bleve if user feedback indicates need for better search.

---

## Next Steps

1. ✅ **Research complete** - File format and structure decided
2. **Next**: Create data-dictionary.md with all data types
3. **Next**: Update architecture.md with file repository designs
4. **Next**: Create detailed implementation plan in plan.md
5. **Next**: Break down into concrete tasks in tasks.md

---

## References

- Spec: `specs/file-based-storage/spec.md`
- Current SQLite schema: `internal/adapter/repository/sqlite/*.go`
- Current file storage: `internal/infrastructure/storage/file_*.go`
- Persona system: `internal/infrastructure/persona/file_repository.go`
