# File-Based Storage Migration PRD

**Document Version:** 1.1
**Date:** 2026-02-16
**Last Updated:** 2026-02-16
**Status:** Draft - Ready for Review
**Author:** Architecture Team

**Changelog:**
- v1.1 (2026-02-16): Added Phase 0 (Code Cleanup & Integration Fixes)
- v1.0 (2026-02-16): Initial draft

---

## Executive Summary

This PRD outlines the migration from SQLite-based storage to a pure file-based storage system for NuimanBot. The change will eliminate the SQLite dependency, simplify deployment, improve data portability, and align all storage mechanisms under a consistent file-based architecture.

**Key Changes:**
- Remove SQLite database dependency entirely
- Consolidate all user data under `data/users/<user-id>/` directory structure
- Merge `domain.User` and `domain.UserProfile` into a single unified profile
- Implement file-based repositories for conversations, memory, notes, and audit logs
- Provide migration utilities for existing SQLite data

**Expected Benefits:**
- Simpler deployment (no database setup)
- Enhanced portability (copy directories to move data)
- Human-readable data formats (JSON/JSONL/Markdown)
- Easier backup/restore operations
- Reduced system dependencies

---

## Background & Motivation

### Current State

NuimanBot currently uses a hybrid storage approach:

**SQLite Database (`data/nuimanbot.db`):**
- Conversations and messages
- Memory cells and scenes (with FTS5 full-text search)
- Basic user authentication/authorization data
- Notes
- Audit logs

**File-Based Storage:**
- User profiles (JSON files)
- Persona files (Markdown: SOUL.md, USER.md, RULES.md)
- Bot configuration (YAML)

### Problems with Current Approach

1. **Complexity**: Dual storage systems increase cognitive load and maintenance burden
2. **Dependency Management**: SQLite with FTS5 requires specific driver versions
3. **Data Fragmentation**: User data split across database and filesystem
4. **Portability**: Database files are opaque and require tools to inspect/modify
5. **Backup Complexity**: Must backup both database and file system separately
6. **Development Friction**: Different patterns for similar operations

### Why File-Based Storage?

NuimanBot is fundamentally a **CLI tool**, not a multi-tenant SaaS application. File-based storage aligns better with CLI tool characteristics:

- **Single-user or small team usage**: No need for high-concurrency database
- **Local-first architecture**: Data stays on user's machine
- **Transparency**: Users can inspect/edit their own data
- **Simplicity**: Lower barrier to entry for contributors
- **Unix philosophy**: Plain text files that compose well with other tools

---

## Goals & Non-Goals

### Goals

✅ **Remove SQLite dependency** - Eliminate database requirement entirely
✅ **Unify storage architecture** - All data uses consistent file-based patterns
✅ **Improve data portability** - Easy to backup, restore, move, or version control
✅ **Consolidate user data** - All user data under single directory hierarchy
✅ **Maintain performance** - Acceptable performance for CLI use cases (single user, local disk)
✅ **Provide migration path** - Safe, reversible migration from existing SQLite data
✅ **Human-readable formats** - JSON/JSONL for structured data, Markdown for content

### Non-Goals

❌ **High-concurrency support** - Not optimizing for thousands of simultaneous users
❌ **Complex querying** - Not building SQL-equivalent query capabilities
❌ **Distributed storage** - Local filesystem only, no cloud/distributed storage
❌ **Real-time indexing** - Indexes rebuilt on startup/as needed, not real-time
❌ **ACID transactions** - Using atomic file writes instead of database transactions

---

## Proposed Solution

### File Structure

```
data/
├── users/
│   └── <user-id>/                     # User-specific data directory
│       ├── profile.json               # Unified user profile (merged user + profile)
│       ├── conversations/             # Conversation history
│       │   ├── <conversation-id>.json # Full conversation with messages
│       │   └── index.json             # Conversation metadata index
│       ├── memory/                    # Memory system v2 data
│       │   ├── cells/                 # Individual memory cells
│       │   │   └── <cell-id>.json
│       │   ├── scenes/                # Scene summaries
│       │   │   └── <scene-name>.json
│       │   └── indexes/               # Search and query indexes
│       │       ├── by-scene.json      # Cells grouped by scene
│       │       ├── by-salience.json   # Cells sorted by salience
│       │       ├── by-type.json       # Cells grouped by type
│       │       └── search-terms.json  # Simple keyword search index
│       ├── notes/                     # User notes
│       │   ├── <note-id>.json
│       │   └── index.json             # Notes metadata index
│       └── personas/                  # Persona customization (moved from ~/.nuimanbot)
│           ├── SOUL.md                # Bot personality/identity
│           ├── USER.md                # User context/preferences
│           └── RULES.md               # Custom behavior rules
├── audit/                             # System-wide audit log
│   └── <YYYY-MM-DD>/                  # Daily audit log directories
│       └── events.jsonl               # Append-only JSONL format
└── system/                            # System-level data
    ├── config.yaml                    # Application configuration
    ├── vault.enc                      # Encrypted credential vault
    └── users-index.json               # Quick user lookup index
```

### Unified User Profile Schema

Merging `domain.User` and `domain.UserProfile` into a single comprehensive profile:

```json
{
  "version": "1.0",
  "userID": "uuid-here",

  "identity": {
    "username": "alice",
    "moniker": "Alice Cooper",
    "firstName": "Alice",
    "lastName": "Cooper",
    "nickName": "Ally"
  },

  "contact": {
    "primaryEmail": "alice@example.com",
    "backupEmail": "alice.backup@example.com",
    "mobilePhone": "+1-555-0123"
  },

  "localization": {
    "primaryLanguage": "en",
    "secondaryLanguage": "es",
    "timezone": "America/Los_Angeles",
    "dateFormat": "YYYY-MM-DD",
    "timeFormat": "24h"
  },

  "authentication": {
    "role": "admin",
    "platformIDs": {
      "cli": "alice",
      "slack": "U123456",
      "telegram": "987654321"
    },
    "apiKey": "encrypted-api-key-here"
  },

  "authorization": {
    "allowedTools": [],
    "customPermissions": {}
  },

  "preferences": {
    "theme": "dark",
    "notificationsEnabled": true,
    "defaultModel": "claude-sonnet"
  },

  "metadata": {
    "createdAt": "2026-01-15T10:00:00Z",
    "updatedAt": "2026-02-16T14:30:00Z",
    "lastLoginAt": "2026-02-16T09:00:00Z"
  }
}
```

---

## Technical Design

### 1. File Formats

#### Structured Data (JSON)
- **Profiles**: `profile.json`
- **Conversations**: `<conversation-id>.json`
- **Memory Cells**: `<cell-id>.json`
- **Memory Scenes**: `<scene-name>.json`
- **Notes**: `<note-id>.json`
- **Indexes**: Various `index.json` files

**Format:** Pretty-printed JSON (2-space indent) for human readability

#### Append-Only Logs (JSONL)
- **Audit Logs**: `events.jsonl`

**Format:** One JSON object per line, append-only for atomic writes

#### Content Files (Markdown)
- **Personas**: `SOUL.md`, `USER.md`, `RULES.md`

**Format:** Standard Markdown with frontmatter metadata if needed

### 2. Repository Implementations

Each repository will implement existing domain interfaces using file-based storage:

#### ConversationRepository (File-Based)
```go
type FileConversationRepository struct {
    basePath      string
    cache         *cache.LRUCache
    indexCache    *ConversationIndex
    atomicWriter  *AtomicFileWriter
}

// Methods:
// - SaveMessage(ctx, convID, userID, platform, msg) error
// - GetConversation(ctx, convID) (*Conversation, error)
// - GetRecentMessages(ctx, convID, maxTokens) ([]Message, error)
// - DeleteConversation(ctx, convID) error
// - ListConversations(ctx, userID) ([]ConversationSummary, error)
```

**Implementation Strategy:**
- Read entire conversation file into memory (acceptable for CLI use)
- Use indexes for fast conversation listing
- Cache frequently accessed conversations
- Atomic writes using temp file + rename pattern

#### MemoryCellRepository (File-Based)
```go
type FileMemoryCellRepository struct {
    basePath     string
    indexes      *MemoryIndexes
    searchIndex  *SearchIndex
    atomicWriter *AtomicFileWriter
}

// Methods:
// - Save(ctx, cell) error
// - Get(ctx, cellID) (*MemoryCell, error)
// - Search(ctx, query) ([]*MemoryCell, error)
// - ListByScene(ctx, scene) ([]*MemoryCell, error)
// - ListBySalience(ctx, minSalience) ([]*MemoryCell, error)
// - Delete(ctx, cellID) error
// - PruneExpired(ctx) (int, error)
```

**Implementation Strategy:**
- Individual cell files for easy inspection
- Multiple indexes for different query patterns
- Simple keyword-based search (replace FTS5)
- Rebuild indexes on startup or cache invalidation

#### NotesRepository (File-Based)
```go
type FileNotesRepository struct {
    basePath     string
    indexCache   *NotesIndex
    atomicWriter *AtomicFileWriter
}

// Methods:
// - Create(ctx, note) error
// - GetByID(ctx, noteID) (*Note, error)
// - List(ctx, userID) ([]*Note, error)
// - Update(ctx, note) error
// - Delete(ctx, noteID) error
```

**Implementation Strategy:**
- One file per note
- Index file for fast listing by user
- Tags stored in note metadata

#### AuditRepository (File-Based)
```go
type FileAuditRepository struct {
    basePath     string
    currentDate  string
    currentFile  *os.File
    fileMutex    sync.Mutex
}

// Methods:
// - Audit(ctx, event) error
// - Query(ctx, criteria) ([]*AuditEvent, error)
```

**Implementation Strategy:**
- Append-only JSONL files (one per day)
- No deletion (audit logs are immutable)
- Rotate files daily
- Simple grep-based queries for CLI tool

### 3. Indexing Strategy

**Purpose:** Fast lookups without scanning all files

#### Conversation Index (`conversations/index.json`)
```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-16T14:30:00Z",
  "conversations": [
    {
      "id": "conv-123",
      "userID": "user-456",
      "platform": "cli",
      "createdAt": "2026-02-15T10:00:00Z",
      "updatedAt": "2026-02-16T14:30:00Z",
      "messageCount": 15,
      "lastMessageSnippet": "Thanks for the help!"
    }
  ]
}
```

#### Memory Indexes (`memory/indexes/`)

**by-scene.json:**
```json
{
  "scenes": {
    "authentication-flow": ["cell-001", "cell-002", "cell-005"],
    "api-integration": ["cell-003", "cell-004"]
  }
}
```

**by-salience.json:**
```json
{
  "cells": [
    {"id": "cell-001", "salience": 0.95},
    {"id": "cell-002", "salience": 0.87},
    {"id": "cell-003", "salience": 0.75}
  ]
}
```

**search-terms.json:**
```json
{
  "terms": {
    "authentication": ["cell-001", "cell-002"],
    "api": ["cell-003", "cell-004"],
    "error": ["cell-005"]
  }
}
```

**Index Rebuilding:**
- On startup: Load indexes into memory
- On write: Update affected indexes
- On corruption: Full rebuild from source files

### 4. Concurrency & Atomicity

**File Locking:**
- Use existing `AtomicFileWriter` for atomic writes
- Read operations: No locks (eventual consistency acceptable)
- Write operations: Per-file locks using `flock` or similar

**Atomic Write Pattern:**
```go
// 1. Write to temp file
tempFile := filepath.Join(dir, ".tmp-" + filename)
os.WriteFile(tempFile, data, 0644)

// 2. Rename to final location (atomic on POSIX)
os.Rename(tempFile, finalPath)
```

**Concurrency Model:**
- Single-user CLI: Low concurrency requirements
- Read-heavy workloads: No locking needed
- Write operations: Serialize per resource (per conversation, per note, etc.)

### 5. Performance Considerations

**Optimization Strategies:**

1. **Caching:**
   - LRU cache for frequently accessed conversations
   - In-memory indexes loaded on startup
   - Profile data cached in memory

2. **Lazy Loading:**
   - Load conversation messages on demand
   - Build indexes incrementally

3. **Batching:**
   - Batch index updates
   - Periodic index flushes

4. **File Size Limits:**
   - Split large conversations (e.g., >1000 messages)
   - Archive old conversations

**Expected Performance:**
- **Read latency**: <10ms for cached data, <100ms for disk reads
- **Write latency**: <50ms for small writes, <200ms for index updates
- **Search**: <500ms for keyword search across user's memory
- **Startup time**: <1s for index loading

**Acceptable Trade-offs:**
- Slower complex queries (no SQL joins)
- Rebuild indexes on corruption
- Manual optimization of file organization

---

## Migration Plan

### Phase 0: Code Cleanup & Integration Fixes (Week 1)

**Objectives:**
- Remove dead code and orphaned features
- Complete critical integrations
- Fix high-priority bugs
- Prepare codebase for file-based migration

**Tasks:**

#### **0.1: Remove Dead Code (~800-1000 LOC)**
- [ ] Remove Plugin System
  - Delete `internal/usecase/plugin/manager.go`
  - Delete `internal/adapter/cli/plugin.go`
  - Remove plugin tests
  - Update documentation to remove plugin references
  - **Impact:** Remove ~300 LOC

- [ ] Remove Analytics System
  - Delete `internal/infrastructure/analytics/` package
  - Remove analytics.Initialize() references (if any)
  - **Impact:** Remove ~200 LOC

- [ ] Remove Resilience Wrapper
  - Delete `internal/infrastructure/resilience/` package
  - Remove circuit breaker and retry wrapper code
  - **Impact:** Remove ~300 LOC

- [ ] Remove REST API (Not needed for CLI tool)
  - Delete `internal/adapter/rest/` package
  - Delete `internal/adapter/web/` package
  - Remove HTTP server initialization stub
  - **Impact:** Remove ~600 LOC

**Total Dead Code Removed:** ~1,400 LOC

#### **0.2: Complete LLM Provider Orchestration**
- [ ] Implement provider selection logic in `/internal/usecase/llm/service.go`
  - Complete TODO at line 80: "select correct provider based on req.Model"
  - Build provider→model mapping from config
  - Implement fallback to default provider
- [ ] Wire provider registration in `cmd/nuimanbot/main.go`
  - Call `RegisterProviderClient()` for each initialized provider
  - Register Anthropic, OpenAI, Bedrock, Ollama based on config
- [ ] Test multi-provider routing
  - Verify requests route to correct provider by model name
  - Test fallback behavior
- [ ] **Impact:** HIGH - Enables multi-LLM support

#### **0.3: Fix Hardcoded LLM Parameters**
- [ ] Update Chat Service to use config/user preferences
  - Remove hardcoded model at line 218: `"claude-3-sonnet-20240229"`
  - Remove hardcoded MaxTokens at line 219: `1024`
  - Remove hardcoded Temperature at line 220: `0.7`
- [ ] Wire to configuration
  - Use `config.LLM.DefaultModel` for model selection
  - Add `chat.default_max_tokens` to config.yaml
  - Add `chat.default_temperature` to config.yaml
- [ ] Support user-level preferences (optional enhancement)
  - Check user profile for model/temperature overrides
  - Fall back to global defaults if not set
- [ ] **Impact:** MEDIUM - Makes LLM params configurable

#### **0.4: Complete Alerting System Integration (Slack + Email)**
- [ ] Initialize alerting in `cmd/nuimanbot/main.go`
  - Call `alerting.Initialize(config.AlertingConfig)` after config load
  - Wire alerting config from config.yaml
- [ ] Add alerting configuration to `config.yaml`
  ```yaml
  alerting:
    enabled: true
    service_name: "NuimanBot"
    throttle_window: 300  # seconds
    channels:
      - type: log
        enabled: true
      - type: slack
        enabled: false
        config:
          webhook_url: "${SLACK_WEBHOOK_URL}"
      - type: email
        enabled: false
        config:
          smtp_server: "smtp.gmail.com:587"
          smtp_username: "${SMTP_USERNAME}"
          smtp_password: "${SMTP_PASSWORD}"
          from_address: "alerts@nuimanbot.local"
          to_addresses:
            - "admin@example.com"
  ```
- [ ] Complete Slack webhook implementation
  - Replace TODO at line 182 in `alerter.go`
  - Implement HTTP POST to Slack webhook URL
  - Format alert as Slack message JSON
  - Test Slack alert delivery
- [ ] Complete Email implementation
  - Replace TODO at line 229 in `alerter.go`
  - Implement SMTP email sending using net/smtp
  - Format alert as HTML/plain text email
  - Test email delivery
- [ ] Remove PagerDuty stub (not needed)
  - Remove PagerDuty channel type
  - Delete PagerDuty implementation code
- [ ] **Impact:** MEDIUM - Enables production alerting for memory system

#### **0.5: Complete Config Hot-Reload Integration**
- [ ] Instantiate ConfigManager in `cmd/nuimanbot/main.go`
  - Create ConfigManager after initial config load
  - Pass to services that need config reload capability
- [ ] Implement CLI trigger
  - Create `internal/adapter/cli/config_reload.go`
  - Add `HandleConfigReloadCommand()` function
  - Add authorization check (admin users only)
  - Wire to CLI gateway with `/admin config reload` command
- [ ] Register config change listeners
  - LLM service: reload provider config
  - Security service: reload input validation rules
  - Tool service: reload tool configurations
- [ ] Implement file watcher (optional)
  - Use fsnotify to watch config.yaml
  - Trigger reload on file change
  - Add debouncing (300ms) to prevent rapid reloads
- [ ] Test reload workflow
  - Change config.yaml
  - Trigger reload via CLI
  - Verify services pick up new config
- [ ] **Impact:** HIGH - Enables runtime config changes without restart

**Phase 0 Deliverables:**
- [ ] Clean codebase with ~1,400 LOC removed
- [ ] Multi-provider LLM routing functional
- [ ] LLM parameters configurable via config.yaml
- [ ] Alerting system operational (Slack + Email)
- [ ] Config hot-reload working with CLI trigger
- [ ] Updated configuration documentation
- [ ] All quality gates passing (fmt, vet, lint, test, build)

---

### Phase 1: Preparation & Design (Week 2)

**Objectives:**
- Finalize file structure and schemas
- Design repository interfaces
- Create migration utilities

**Deliverables:**
- [ ] Detailed technical design document
- [ ] Migration utility specification
- [ ] Rollback procedure documentation
- [ ] Test data set for migration testing

### Phase 2: Implementation (Weeks 3-4)

**Objectives:**
- Implement file-based repositories
- Build migration utilities
- Create comprehensive tests

**Tasks:**

**Week 3: Core Repositories**
- [ ] Implement `FileUserProfileRepository` (merge User + UserProfile)
- [ ] Implement `FileConversationRepository`
- [ ] Implement `FileMemoryCellRepository`
- [ ] Implement `FileMemorySceneRepository`
- [ ] Implement `FileNotesRepository`
- [ ] Implement `FileAuditRepository`

**Week 4: Migration & Tooling**
- [ ] Create SQLite-to-Files migration utility (`cmd/migrate/main.go`)
- [ ] Build index generation utilities
- [ ] Implement data validation tools
- [ ] Write unit tests for all repositories
- [ ] Write integration tests for migration

### Phase 3: Testing & Validation (Week 5)

**Objectives:**
- Validate migration correctness
- Performance testing
- Edge case handling

**Test Scenarios:**
- [ ] Migrate small dataset (<100 conversations)
- [ ] Migrate medium dataset (1K-10K conversations)
- [ ] Migrate large dataset (>10K conversations)
- [ ] Test concurrent access during migration
- [ ] Test migration rollback
- [ ] Verify data integrity post-migration
- [ ] Performance benchmarks (read/write/search)

### Phase 4: Deployment (Week 6)

**Objectives:**
- Deploy to production
- Monitor performance
- Support users during transition

**Steps:**

1. **Pre-migration:**
   - [ ] Backup existing SQLite database
   - [ ] Document migration procedure
   - [ ] Announce migration to users (if multi-user)

2. **Migration execution:**
   - [ ] Run migration utility
   - [ ] Validate migrated data
   - [ ] Run integrity checks

3. **Cutover:**
   - [ ] Update application to use file-based repositories
   - [ ] Deploy new version
   - [ ] Monitor for errors

4. **Post-migration:**
   - [ ] Keep SQLite backup for 30 days
   - [ ] Monitor performance metrics
   - [ ] Gather user feedback

### Phase 5: Cleanup (Week 7)

**Objectives:**
- Remove SQLite dependencies
- Update documentation
- Optimize performance

**Tasks:**
- [ ] Remove SQLite repository implementations
- [ ] Remove SQLite dependencies from `go.mod`
- [ ] Update documentation (README, admin guides, etc.)
- [ ] Remove database-related configuration
- [ ] Performance tuning based on real usage

---

## Migration Utility Design

### CLI Command: `nuimanbot migrate sqlite-to-files`

```bash
# Migrate all data from SQLite to files
./bin/nuimanbot migrate sqlite-to-files \
  --sqlite-path ./data/nuimanbot.db \
  --output-dir ./data/users \
  --dry-run

# Migrate with verification
./bin/nuimanbot migrate sqlite-to-files \
  --sqlite-path ./data/nuimanbot.db \
  --output-dir ./data/users \
  --verify

# Rollback migration
./bin/nuimanbot migrate rollback-files-to-sqlite \
  --input-dir ./data/users \
  --sqlite-path ./data/nuimanbot.db
```

### Migration Algorithm

```
1. Verify SQLite database exists and is readable
2. Create backup of SQLite database
3. Create target directory structure
4. Migrate users:
   - Read all users from SQLite
   - Read corresponding user profiles from JSON
   - Merge into unified profile.json
   - Write to data/users/<user-id>/profile.json
5. For each user:
   - Migrate conversations:
     - Read conversations for user
     - Write to data/users/<user-id>/conversations/<conv-id>.json
     - Build conversation index
   - Migrate memory cells:
     - Read memory cells for user
     - Write to data/users/<user-id>/memory/cells/<cell-id>.json
     - Build memory indexes
   - Migrate memory scenes:
     - Read memory scenes
     - Write to data/users/<user-id>/memory/scenes/<scene>.json
   - Migrate notes:
     - Read notes for user
     - Write to data/users/<user-id>/notes/<note-id>.json
     - Build notes index
6. Migrate audit logs:
   - Read audit events
   - Group by date
   - Write to data/audit/<date>/events.jsonl
7. Move persona files:
   - Move from ~/.nuimanbot/personas/<user-id>/ to data/users/<user-id>/personas/
8. Verify migration:
   - Check file counts match record counts
   - Validate JSON structure
   - Verify data integrity
9. Generate migration report
```

### Rollback Strategy

**Trigger rollback if:**
- Migration fails midway
- Data validation errors detected
- Critical data missing post-migration

**Rollback procedure:**
1. Stop application
2. Delete migrated file structure
3. Restore SQLite from backup
4. Restart application with SQLite repositories
5. Investigate migration failure
6. Fix issues and retry

### Data Validation

**Post-migration checks:**
- [ ] User count matches
- [ ] Conversation count matches per user
- [ ] Message count matches per conversation
- [ ] Memory cell count matches
- [ ] Notes count matches
- [ ] Audit event count matches
- [ ] No corrupted JSON files
- [ ] All indexes build successfully
- [ ] Sample queries return expected results

---

## Testing Strategy

### Unit Tests

**File Repository Tests:**
```go
// Test user profile repository
func TestFileUserProfileRepository_SaveAndGet(t *testing.T) { ... }
func TestFileUserProfileRepository_MergedProfile(t *testing.T) { ... }
func TestFileUserProfileRepository_ConcurrentWrites(t *testing.T) { ... }

// Test conversation repository
func TestFileConversationRepository_SaveMessage(t *testing.T) { ... }
func TestFileConversationRepository_GetRecentMessages(t *testing.T) { ... }
func TestFileConversationRepository_Index(t *testing.T) { ... }

// Test memory repository
func TestFileMemoryCellRepository_Search(t *testing.T) { ... }
func TestFileMemoryCellRepository_IndexRebuild(t *testing.T) { ... }
```

### Integration Tests

**End-to-End Migration:**
```go
func TestMigration_SQLiteToFiles_Complete(t *testing.T) {
    // 1. Set up test SQLite database with sample data
    // 2. Run migration
    // 3. Verify all data migrated correctly
    // 4. Test rollback
    // 5. Verify SQLite restored
}
```

**Performance Benchmarks:**
```go
func BenchmarkFileConversationRepository_GetRecentMessages(b *testing.B) { ... }
func BenchmarkFileMemoryCellRepository_Search(b *testing.B) { ... }
func BenchmarkFileNotesRepository_List(b *testing.B) { ... }
```

### Manual Testing

**Test Scenarios:**
1. Create new user with file-based storage
2. Import large conversation history
3. Search memory across multiple scenes
4. Export user data for backup
5. Delete user and verify cleanup
6. Concurrent access from multiple terminals

---

## Success Metrics

### Phase 0 Metrics (Code Cleanup)

- **Dead Code Removal**: Remove ~1,400 LOC (plugin, analytics, resilience, REST) ✅
- **LLM Provider Selection**: Multi-provider routing functional ✅
- **LLM Configuration**: Model/temperature/tokens configurable via config ✅
- **Alerting Integration**: Slack + Email alerts operational ✅
- **Config Hot-Reload**: CLI trigger `/admin config reload` working ✅

### Technical Metrics (Overall Migration)

- **Dependency Count**: Reduce from 2 storage systems to 1 ✅
- **Codebase Simplification**: Remove ~500 lines of SQLite-specific code (+ 1,400 from Phase 0) ✅
- **Performance**: <100ms read latency, <200ms write latency ✅
- **Reliability**: Zero data loss during migration ✅
- **Test Coverage**: >80% coverage for new file repositories ✅

### User Experience Metrics

- **Deployment Simplicity**: Single binary + data directory (no DB setup) ✅
- **Data Portability**: Copy directory to move data ✅
- **Debuggability**: Human-readable files for inspection ✅
- **Backup Time**: <10 seconds for typical user data ✅

### Migration Metrics

- **Migration Success Rate**: 100% of test migrations succeed ✅
- **Data Integrity**: 100% of records migrated correctly ✅
- **Downtime**: <5 minutes for typical user ✅

---

## Risks & Mitigation

### Risk 1: Data Loss During Migration
**Impact:** High
**Probability:** Low
**Mitigation:**
- Automatic backup before migration
- Dry-run mode for testing
- Comprehensive validation checks
- Rollback capability
- Keep SQLite backup for 30 days

### Risk 2: Performance Degradation
**Impact:** Medium
**Probability:** Medium
**Mitigation:**
- Benchmark before/after migration
- Implement caching strategically
- Optimize hot paths
- Accept trade-offs for CLI use case
- Profile and optimize as needed

### Risk 3: Search Quality Reduction (FTS5 Replacement)
**Impact:** Medium
**Probability:** Medium
**Mitigation:**
- Implement simple but effective keyword search
- Consider integrating lightweight search library (bleve)
- User feedback on search quality
- Iterate on search improvements

### Risk 4: Concurrency Issues
**Impact:** Low
**Probability:** Low
**Mitigation:**
- File locking for writes
- Atomic file operations
- Accept eventual consistency for reads
- CLI use case = low concurrency

### Risk 5: Large File Handling
**Impact:** Low
**Probability:** Low
**Mitigation:**
- Split large conversations into chunks
- Implement pagination for listing operations
- Archive old conversations
- Monitor file sizes and warn users

---

## Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| **Phase 0: Code Cleanup** | Week 1 | Dead code removed, integrations complete, alerting/config working |
| **Phase 1: Preparation** | Week 2 | Design docs, migration spec, test plan |
| **Phase 2: Implementation** | Weeks 3-4 | File repositories, migration utility, tests |
| **Phase 3: Testing** | Week 5 | Validated migration, benchmarks, edge cases |
| **Phase 4: Deployment** | Week 6 | Production migration, monitoring |
| **Phase 5: Cleanup** | Week 7 | Remove SQLite code, documentation updates |

**Total Duration:** 7 weeks

**Milestones:**
- ✅ Week 1: Codebase cleaned, integrations complete
- ✅ Week 2: Design approved
- ✅ Week 3: Core repositories implemented
- ✅ Week 4: Migration utility complete
- ✅ Week 5: All tests passing
- ✅ Week 6: Production cutover
- ✅ Week 7: SQLite fully removed

---

## Appendices

### Appendix A: File Size Estimates

**Typical User Data:**
- Profile: ~5 KB
- Conversation (100 messages): ~50 KB
- Memory cell: ~2 KB
- Note: ~5 KB
- Audit event: ~500 bytes

**Storage for 1,000 conversations:**
- Conversations: ~50 MB
- Indexes: ~5 MB
- Total: ~55 MB

**Conclusion:** File sizes manageable for local filesystem

### Appendix B: Alternative Considered

**Embedded Databases (BoltDB, BadgerDB):**
- Pros: Better performance than files, still embeddable
- Cons: Another dependency, less transparent than files
- Decision: File-based storage aligns better with CLI tool philosophy

**Hybrid Approach (SQLite for some, files for others):**
- Pros: Keep SQLite for complex queries
- Cons: Maintains complexity, doesn't solve root problem
- Decision: Full migration preferred for consistency

### Appendix C: Related Documentation

- [Memory Migration Guide](support_docs/memory-migration-guide.md)
- [Admin Guide](support_docs/admin-guide.md)
- [Technical Details](documentation/technical-details.md)
- [AGENTS.md](AGENTS.md) - Development guidelines

---

## Approval

**Prepared by:** Architecture Team
**Reviewed by:** [Name]
**Approved by:** [Name]
**Date:** 2026-02-16

**Next Steps:**
1. Review and approve PRD
2. Create feature spec in `specs/file-based-storage/`
3. Begin Phase 1 implementation
4. Track progress in `specs/file-based-storage/status.md`
