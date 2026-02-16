# File-Based Storage Migration - Specification

**Created:** 2026-02-16
**Version:** 1.0
**Status:** Draft
**Source PRD:** `02-16-file-based-memory-PRD.md`

---

## Executive Summary

This specification outlines the migration from SQLite-based storage to a pure file-based storage system for NuimanBot. The change will eliminate the SQLite dependency, simplify deployment, improve data portability, and align all storage mechanisms under a consistent file-based architecture.

**Key Deliverables:**
- File-based repositories for conversations, memory, notes, and audit logs
- Unified user profile (merged domain.User + domain.UserProfile)
- User-centric directory structure: `data/users/<user-id>/`
- Migration utilities for existing SQLite data
- Phase 0: Code cleanup (~1,400 LOC removal) and integration fixes

**Timeline:** 7 weeks

---

## Problem Statement

### Current State

NuimanBot currently uses a hybrid storage approach:

**SQLite Database:**
- Conversations and messages
- Memory cells and scenes (with FTS5 full-text search)
- Basic user authentication/authorization data
- Notes
- Audit logs

**File-Based Storage:**
- User profiles (JSON files)
- Persona files (Markdown: SOUL.md, USER.md, RULES.md)
- Bot configuration (YAML)

### Pain Points

- **Complexity**: Dual storage systems increase cognitive load and maintenance burden
- **Dependency Management**: SQLite with FTS5 requires specific driver versions
- **Data Fragmentation**: User data split across database and filesystem
- **Portability**: Database files are opaque and require tools to inspect/modify
- **Backup Complexity**: Must backup both database and file system separately
- **Development Friction**: Different patterns for similar operations
- **Dead Code**: ~1,400 LOC of orphaned features (Plugin, Analytics, Resilience, REST API)
- **Incomplete Integrations**: LLM provider selection, config hot-reload, alerting not fully integrated

### Desired State

After this feature is implemented:

- **Single storage mechanism**: All data uses file-based patterns
- **User-centric structure**: All user data under `data/users/<user-id>/`
- **Human-readable**: JSON/JSONL for data, Markdown for content
- **Portable**: Copy directories to backup/move data
- **Clean codebase**: Dead code removed, integrations complete
- **Simplified deployment**: No database setup required

---

## Goals and Non-Goals

### Goals

✅ **Phase 0: Code Cleanup & Integration**
- Remove dead code (~1,400 LOC): Plugin, Analytics, Resilience, REST API
- Complete LLM provider orchestration
- Fix hardcoded LLM parameters (wire to config)
- Complete Alerting integration (Slack + Email)
- Complete Config hot-reload integration

✅ **File-Based Migration**
- Remove SQLite dependency entirely
- Unify storage architecture under file-based patterns
- Improve data portability
- Consolidate user data under single directory hierarchy
- Maintain acceptable performance for CLI use cases
- Provide safe, reversible migration path
- Human-readable formats (JSON/JSONL/Markdown)

### Non-Goals

❌ **High-concurrency support** - Not optimizing for thousands of simultaneous users
❌ **Complex querying** - Not building SQL-equivalent query capabilities
❌ **Distributed storage** - Local filesystem only
❌ **Real-time indexing** - Indexes rebuilt on startup/as needed
❌ **ACID transactions** - Using atomic file writes instead

---

## User Requirements

### Functional Requirements

#### FR-001: Remove SQLite Dependency
**Priority:** P0 (Critical)

**Description:**
Eliminate all SQLite database usage and replace with file-based storage

**Acceptance Criteria:**
- [ ] All data migrated from SQLite to files
- [ ] SQLite driver dependencies removed from go.mod
- [ ] No database connection initialization in main.go
- [ ] All repository implementations use file-based storage
- [ ] Migration utility successfully converts existing data
- [ ] Rollback capability functional

**Example:**
```bash
# Before: SQLite database
data/nuimanbot.db  (opaque binary file)

# After: File-based storage
data/users/alice/
  ├── profile.json
  ├── conversations/conv-123.json
  ├── memory/cells/cell-001.json
  └── notes/note-456.json
```

---

#### FR-002: Unified User Profile
**Priority:** P0 (Critical)

**Description:**
Merge `domain.User` and `domain.UserProfile` into single comprehensive profile

**Acceptance Criteria:**
- [ ] Single `profile.json` per user contains all user data
- [ ] Authentication fields (role, platformIDs, allowedTools)
- [ ] Profile fields (names, emails, localization, preferences)
- [ ] All code updated to use unified profile
- [ ] Migration merges existing User + UserProfile data

**Example:**
```json
{
  "userID": "alice-123",
  "identity": {
    "username": "alice",
    "moniker": "Alice Cooper",
    "firstName": "Alice"
  },
  "authentication": {
    "role": "admin",
    "platformIDs": {"cli": "alice", "slack": "U123"}
  },
  "contact": {
    "primaryEmail": "alice@example.com"
  }
}
```

---

#### FR-003: User-Centric Directory Structure
**Priority:** P0 (Critical)

**Description:**
All user data organized under `data/users/<user-id>/` directory

**Acceptance Criteria:**
- [ ] User directory created for each user
- [ ] Conversations stored in user directory
- [ ] Memory cells and scenes in user directory
- [ ] Notes in user directory
- [ ] Personas (SOUL, USER, RULES) in user directory
- [ ] Easy backup/restore (copy directory)
- [ ] Easy user deletion (remove directory)

**Structure:**
```
data/users/<user-id>/
  ├── profile.json
  ├── conversations/
  │   ├── <conversation-id>.json
  │   └── index.json
  ├── memory/
  │   ├── cells/<cell-id>.json
  │   ├── scenes/<scene-name>.json
  │   └── indexes/...
  ├── notes/
  │   ├── <note-id>.json
  │   └── index.json
  └── personas/
      ├── SOUL.md
      ├── USER.md
      └── RULES.md
```

---

#### FR-004: Migration Utility
**Priority:** P0 (Critical)

**Description:**
Provide CLI utility to migrate existing SQLite data to file-based storage

**Acceptance Criteria:**
- [ ] `nuimanbot migrate sqlite-to-files` command
- [ ] Dry-run mode for testing
- [ ] Verification mode to check integrity
- [ ] Rollback capability
- [ ] Progress reporting
- [ ] Data validation post-migration
- [ ] Zero data loss

**Example:**
```bash
# Test migration (dry run)
./bin/nuimanbot migrate sqlite-to-files --dry-run

# Execute migration with verification
./bin/nuimanbot migrate sqlite-to-files --verify

# Rollback if needed
./bin/nuimanbot migrate rollback-files-to-sqlite
```

---

#### FR-005: File-Based Repositories
**Priority:** P0 (Critical)

**Description:**
Implement file-based repositories for all data types

**Acceptance Criteria:**
- [ ] FileUserProfileRepository (unified profile)
- [ ] FileConversationRepository (conversations + messages)
- [ ] FileMemoryCellRepository (memory cells)
- [ ] FileMemorySceneRepository (memory scenes)
- [ ] FileNotesRepository (user notes)
- [ ] FileAuditRepository (audit logs as JSONL)
- [ ] All implement existing domain interfaces
- [ ] Performance acceptable (<100ms reads, <200ms writes)

---

#### FR-006: Remove Dead Code
**Priority:** P1 (High)

**Description:**
Remove orphaned/unused code (~1,400 LOC)

**Acceptance Criteria:**
- [ ] Plugin System removed (~300 LOC)
- [ ] Analytics System removed (~200 LOC)
- [ ] Resilience Wrapper removed (~300 LOC)
- [ ] REST API removed (~600 LOC)
- [ ] All references cleaned up
- [ ] Tests still pass after removal
- [ ] Build succeeds

---

#### FR-007: Complete LLM Provider Orchestration
**Priority:** P1 (High)

**Description:**
Implement provider selection logic for multi-LLM support

**Acceptance Criteria:**
- [ ] Provider selection by model name functional
- [ ] Provider registration in main.go
- [ ] Provider→model mapping from config
- [ ] Fallback to default provider
- [ ] Dynamic routing working
- [ ] Tests for provider selection

---

#### FR-008: Complete Config Hot-Reload
**Priority:** P1 (High)

**Description:**
Enable runtime configuration reload without restart

**Acceptance Criteria:**
- [ ] ConfigManager instantiated in main.go
- [ ] `/admin config reload` CLI command
- [ ] Config change listeners registered
- [ ] File watcher implemented (optional)
- [ ] Services pick up new config
- [ ] No downtime during reload

---

#### FR-009: Complete Alerting Integration
**Priority:** P2 (Medium)

**Description:**
Integrate alerting system with Slack and Email support

**Acceptance Criteria:**
- [ ] Alerting initialized in main.go
- [ ] Slack webhook implementation complete
- [ ] Email SMTP implementation complete
- [ ] Configuration in config.yaml
- [ ] Alerts functional in memory curator
- [ ] Throttling working

---

### Non-Functional Requirements

#### NFR-001: Performance
**Category:** Performance

**Description:**
File-based storage must maintain acceptable performance for CLI tool

**Metrics:**
- Read latency: <100ms (90th percentile)
- Write latency: <200ms (90th percentile)
- Search latency: <500ms for keyword search
- Startup time: <1s for index loading
- No performance regression vs SQLite for typical workloads

---

#### NFR-002: Reliability
**Category:** Reliability

**Description:**
Zero data loss during migration and normal operations

**Metrics:**
- Migration success rate: 100%
- Data integrity: 100% of records migrated correctly
- Atomic writes: All or nothing (no partial writes)
- Automatic backup before migration
- Rollback capability tested and functional

---

#### NFR-003: Maintainability
**Category:** Maintainability

**Description:**
Simpler codebase easier to understand and modify

**Metrics:**
- LOC reduction: ~1,900 LOC removed (SQLite + dead code)
- Test coverage: >80% for new file repositories
- Code complexity: Reduced cyclomatic complexity
- Documentation: All file formats documented
- Contributor onboarding time reduced

---

## System Architecture

### Affected Layers

- [x] Domain Layer - New unified UserProfile entity
- [x] Use Case Layer - Updated to use file repositories
- [x] Infrastructure Layer - New file-based repository implementations
- [x] Adapter Layer - CLI migration commands

### New Components

**File Repositories** (`internal/infrastructure/storage/`):
- `FileUserProfileRepository` - Unified profile management
- `FileConversationRepository` - Conversation/message storage
- `FileMemoryCellRepository` - Memory cell storage with indexing
- `FileMemorySceneRepository` - Memory scene storage
- `FileNotesRepository` - Notes storage
- `FileAuditRepository` - Append-only audit log (JSONL)

**Migration Utility** (`cmd/migrate/`):
- `SQLiteToFileMigrator` - Orchestrates migration
- `DataValidator` - Validates migrated data
- `RollbackManager` - Handles rollback operations

**Indexing** (`internal/infrastructure/storage/indexes/`):
- `ConversationIndex` - Fast conversation lookups
- `MemoryIndexes` - Scene, salience, type, search indexes
- `NotesIndex` - Notes metadata index

### Modified Components

**Domain** (`internal/domain/`):
- `User` + `UserProfile` → Unified `UserProfile`
- Repository interfaces unchanged (implementation swapped)

**Main** (`cmd/nuimanbot/main.go`):
- Remove SQLite initialization
- Initialize file-based repositories
- Remove plugin, analytics, resilience code
- Complete LLM provider registration
- Initialize ConfigManager
- Initialize Alerting

**Config** (`config.yaml`):
- Remove `storage.type: sqlite`
- Add `storage.type: file`
- Add `alerting` section
- Add `chat.default_*` parameters

---

## Scope of Changes

### Files to Create

**Phase 0 (Code Cleanup):**
- (Delete files: plugin/, analytics/, resilience/, rest/, web/)

**Phase 2 (File Repositories):**
- `internal/infrastructure/storage/file_user_profile_repository.go`
- `internal/infrastructure/storage/file_conversation_repository.go`
- `internal/infrastructure/storage/file_memory_cell_repository.go`
- `internal/infrastructure/storage/file_memory_scene_repository.go`
- `internal/infrastructure/storage/file_notes_repository.go`
- `internal/infrastructure/storage/file_audit_repository.go`
- `internal/infrastructure/storage/indexes/conversation_index.go`
- `internal/infrastructure/storage/indexes/memory_indexes.go`
- `internal/infrastructure/storage/indexes/notes_index.go`

**Phase 2 (Migration Utility):**
- `cmd/migrate/main.go`
- `cmd/migrate/sqlite_to_file.go`
- `cmd/migrate/validator.go`
- `cmd/migrate/rollback.go`

**Phase 2 (Tests):**
- `*_test.go` files for all new repositories
- `internal/infrastructure/storage/integration_test.go`
- `cmd/migrate/migration_test.go`

### Files to Modify

**Phase 0:**
- `cmd/nuimanbot/main.go` - Remove dead code, complete integrations
- `internal/usecase/llm/service.go` - Complete provider selection
- `internal/usecase/chat/service.go` - Remove hardcoded params
- `internal/infrastructure/alerting/alerter.go` - Complete Slack/Email
- `config.yaml` - Add alerting, chat config

**Phase 1-2:**
- `cmd/nuimanbot/main.go` - Swap SQLite → File repositories
- `internal/domain/user_profile.go` - Unified profile struct
- `config.yaml` - Update storage configuration
- `go.mod` - Remove SQLite dependencies
- `README.md` - Update documentation

### Dependencies

**External:**
- None (removing SQLite dependency)

**Internal:**
- Existing `AtomicFileWriter` for atomic writes
- Existing cache infrastructure for performance
- Domain interfaces remain unchanged

---

## Breaking Changes

### API Changes

**None** - All repository interfaces unchanged, only implementations swapped

### Configuration Changes

**Before (SQLite):**
```yaml
storage:
  type: sqlite
  dsn: "./data/nuimanbot.db"
```

**After (File-Based):**
```yaml
storage:
  type: file
  base_path: "./data/users"
```

**Migration Path:**
Run migration utility before upgrading

### Database Schema Changes

**Migration Required:**
- Users must run `nuimanbot migrate sqlite-to-files` before using new version
- Automatic backup created before migration
- Rollback available if migration fails
- Keep SQLite backup for 30 days

---

## Success Criteria

### Acceptance Criteria

- [ ] Phase 0: Dead code removed (~1,400 LOC)
- [ ] Phase 0: LLM provider orchestration complete
- [ ] Phase 0: Config hot-reload functional
- [ ] Phase 0: Alerting operational (Slack + Email)
- [ ] All data migrated from SQLite to files
- [ ] Zero data loss during migration
- [ ] File repositories implement domain interfaces
- [ ] Performance targets met (<100ms reads, <200ms writes)
- [ ] Migration utility tested and functional
- [ ] Rollback capability verified
- [ ] All tests passing
- [ ] Documentation updated

### Quality Gates

- [ ] All tests pass (unit, integration, e2e)
- [ ] Code coverage >80%
- [ ] `go fmt ./...` clean
- [ ] `go vet ./...` clean
- [ ] `golangci-lint run` clean
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds
- [ ] `./bin/nuimanbot --help` runs without errors
- [ ] Performance benchmarks meet targets
- [ ] Migration tested on sample datasets

### User Validation

- [ ] Migrate test data successfully
- [ ] Verify conversations accessible post-migration
- [ ] Verify memory search functional
- [ ] Verify notes CRUD operations work
- [ ] Verify audit logs readable
- [ ] Backup/restore tested (copy directory)
- [ ] User deletion tested (remove directory)

---

## Risks and Mitigation

### Risk 1: Data Loss During Migration
**Likelihood:** Low
**Impact:** High

**Mitigation:**
- Automatic backup before migration
- Dry-run mode for testing
- Comprehensive validation checks
- Rollback capability
- Keep SQLite backup for 30 days
- Test migration on sample datasets first

---

### Risk 2: Performance Degradation
**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Benchmark before/after migration
- Implement caching strategically
- Optimize hot paths (conversation reads, memory search)
- Accept trade-offs for CLI use case
- Profile and optimize as needed
- Monitor performance metrics

---

### Risk 3: Search Quality Reduction (FTS5 Replacement)
**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Implement simple but effective keyword search
- Consider integrating lightweight search library (bleve) if needed
- User feedback on search quality
- Iterate on search improvements post-migration

---

### Risk 4: Large File Handling
**Likelihood:** Low
**Impact:** Low

**Mitigation:**
- Split large conversations into chunks
- Implement pagination for listing operations
- Archive old conversations
- Monitor file sizes and warn users
- Document file size limits

---

### Risk 5: Phase 0 Integration Failures
**Likelihood:** Medium
**Impact:** Medium

**Mitigation:**
- Incremental integration testing
- Rollback plan for each integration
- Thorough testing of LLM provider routing
- Test config reload with all services
- Test Slack/Email alerts in staging

---

## Timeline and Milestones

### Phase 0: Code Cleanup & Integration Fixes (Week 1)

**Deliverables:**
- Dead code removed (~1,400 LOC)
- LLM provider orchestration complete
- LLM params configurable
- Alerting integrated (Slack + Email)
- Config hot-reload functional
- All quality gates passing

### Phase 1: Preparation & Design (Week 2)

**Deliverables:**
- File structure finalized
- Repository interfaces designed
- Migration utility specification
- Test plan documented

### Phase 2: Implementation (Weeks 3-4)

**Deliverables:**
- File repositories implemented
- Migration utility complete
- Unit tests passing
- Integration tests passing

### Phase 3: Testing & Validation (Week 5)

**Deliverables:**
- Migration tested on sample datasets
- Performance benchmarks complete
- Edge cases handled
- Rollback tested

### Phase 4: Deployment (Week 6)

**Deliverables:**
- Production migration executed
- Data validated
- Monitoring in place
- User feedback gathered

### Phase 5: Cleanup (Week 7)

**Deliverables:**
- SQLite code removed
- Dependencies cleaned
- Documentation updated
- Performance tuned

**Total Estimated Duration:** 7 weeks

---

## References

- **Source PRD:** `02-16-file-based-memory-PRD.md`
- **Related Specs:** None (initial spec)
- **AGENTS.md:** Development workflow and quality gates
- **README.md:** Product overview and usage
