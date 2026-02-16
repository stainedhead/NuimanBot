# File-Based Storage Migration - Implementation Plan

**Created:** 2026-02-16
**Version:** 1.0
**Status:** Complete
**Estimated Duration:** 6 weeks (Phases 1-5)

---

## Table of Contents

1. [Development Approach](#development-approach)
2. [Phase Breakdown](#phase-breakdown)
3. [Critical Path](#critical-path)
4. [Dependency Graph](#dependency-graph)
5. [Testing Strategy](#testing-strategy)
6. [Migration Utility Specification](#migration-utility-specification)
7. [Rollout Strategy](#rollout-strategy)
8. [Success Metrics](#success-metrics)

---

## Development Approach

### Methodology

**TDD + Bottom-Up Clean Architecture**

```
Red → Green → Refactor (for each component)
  ↓
Domain (unchanged) → Infrastructure → Integration → Verification
  ↓
Migration Tool → Testing → Deployment
```

**Why This Approach?**
1. **TDD Ensures Correctness**: Write tests first, implement to pass, refactor for quality
2. **Bottom-Up**: Build infrastructure (file repos) before wiring them into use cases
3. **Incremental**: Each phase produces working, tested code
4. **Risk Reduction**: Test migration on sample data before production

**Key Principles:**
1. **Domain Interfaces Unchanged**: Swap implementations only, no interface changes
2. **Test Every Layer**: Unit tests for repos, integration tests for workflows
3. **Migration Safety**: Dry-run, backup, validate, rollback capability
4. **Performance Validation**: Benchmark against targets before deployment

**Incremental Milestones:**
- Milestone 1: File repositories implemented and tested (Phase 2, Week 3-4)
- Milestone 2: Migration utility functional with validation (Phase 2, Week 4)
- Milestone 3: Migration tested on sample datasets (Phase 3, Week 5)
- Milestone 4: Production migration executed and validated (Phase 4, Week 6)

---

## Phase Breakdown

### Phase 0: Code Cleanup & Integration Fixes (Week 1)

**Status:** ✅ Complete (per status.md)

**Goal:** Clean codebase and complete integrations before migration work

**Deliverables:**
- ✅ Dead code removed (~5,253 LOC)
- ✅ LLM provider orchestration complete
- ✅ Hardcoded LLM parameters configurable
- ✅ Alerting operational (Slack + Email)
- ✅ Config hot-reload functional

---

### Phase 1: Preparation & Design (Week 2)

**Status:** 🟡 In Progress (50% complete)

**Goal:** Complete planning and design documentation

**Tasks:**
- [x] P1.1 - Finalize file structure and schemas (data-dictionary.md)
- [x] P1.2 - Design repository interfaces (data-dictionary.md)
- [x] P1.3 - Create migration utility specification (this document)
- [x] P1.4 - Document test strategy (this document)

**Deliverables:**
- [x] research.md - Complete
- [x] data-dictionary.md - Complete (1,000+ lines)
- [x] architecture.md - Complete (1,200+ lines)
- [x] plan.md - In progress (this document)
- [ ] tasks.md - Next (concrete task breakdown)

**Dependencies:** Phase 0 complete ✅

**Quality Gates:**
- [x] All spec documents reviewed
- [ ] Implementation approach validated
- [ ] Timeline approved

---

### Phase 2: Implementation (Weeks 3-4)

**Status:** ⬜ Not Started

**Goal:** Implement file-based repositories and migration utility

**Duration:** 2 weeks (80-100 hours)

**Tasks (8 tasks):**

#### P2.1: Implement FileUserProfileRepository (6-8 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_user_profile_repository.go`
  - `internal/infrastructure/storage/file_user_profile_repository_test.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement SaveProfile, GetProfileByUserID, GetProfileByEmail, etc.
  - Use AtomicFileWriter for writes
  - Add optional caching
- **Acceptance Criteria:**
  - All domain.UserProfileRepository methods implemented
  - Unit tests pass (>90% coverage)
  - Atomic writes verified
  - Profile save/load < 100ms

#### P2.2: Implement FileConversationRepository (10-12 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_conversation_repository.go`
  - `internal/infrastructure/storage/file_conversation_repository_test.go`
  - `internal/infrastructure/storage/conversation_index.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement SaveConversation, GetConversation, ListConversations, etc.
  - Build conversation index for fast listing
  - Handle large conversations (10,000+ messages)
- **Acceptance Criteria:**
  - All domain.ConversationRepository methods implemented
  - Index maintained correctly
  - Unit tests pass (>90% coverage)
  - Conversation load < 100ms (p90)

#### P2.3: Implement FileMemoryCellRepository (12-16 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_memory_cell_repository.go`
  - `internal/infrastructure/storage/file_memory_cell_repository_test.go`
  - `internal/infrastructure/storage/memory_indexes.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement Create, Get, List, Delete, SearchFTS, GetByScene, etc.
  - Build 4 indexes: by-scene, by-type, by-salience, search
  - Implement keyword search algorithm
- **Acceptance Criteria:**
  - All memoryv2.MemoryCellRepository methods implemented
  - 4 indexes maintained correctly
  - Keyword search functional
  - Unit tests pass (>90% coverage)
  - Memory search < 500ms

#### P2.4: Implement FileMemorySceneRepository (4-6 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_memory_scene_repository.go`
  - `internal/infrastructure/storage/file_memory_scene_repository_test.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement Upsert, Get, List, Delete
  - Simple file-per-scene pattern
- **Acceptance Criteria:**
  - All memoryv2.MemorySceneRepository methods implemented
  - Unit tests pass (>90% coverage)
  - Scene operations < 50ms

#### P2.5: Implement FileNotesRepository (6-8 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_notes_repository.go`
  - `internal/infrastructure/storage/file_notes_repository_test.go`
  - `internal/infrastructure/storage/notes_index.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement Create, GetByID, List, Update, Delete
  - Build notes index with tag mapping
- **Acceptance Criteria:**
  - All repository methods implemented
  - Tag-based indexing functional
  - Unit tests pass (>90% coverage)
  - Note operations < 100ms

#### P2.6: Implement FileAuditRepository (6-8 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/file_audit_repository.go`
  - `internal/infrastructure/storage/file_audit_repository_test.go`
- **Approach:**
  - Write tests first (TDD)
  - Implement JSONL append-only writes
  - Implement streaming query with filters
- **Acceptance Criteria:**
  - Append and Query methods implemented
  - JSONL format correct (one JSON per line)
  - Streaming read (no full load)
  - Unit tests pass (>90% coverage)

#### P2.7: Implement Migration Utility (16-20 hours)
- **Files to Create:**
  - `cmd/migrate/main.go`
  - `cmd/migrate/sqlite_to_file.go`
  - `cmd/migrate/validator.go`
  - `cmd/migrate/rollback.go`
  - `cmd/migrate/progress.go`
  - `cmd/migrate/migrate_test.go`
- **Approach:**
  - See [Migration Utility Specification](#migration-utility-specification) below
  - Implement dry-run, execute, validate, rollback commands
  - Add progress reporting
  - Test with sample data
- **Acceptance Criteria:**
  - All migration commands functional
  - Dry-run mode works
  - Progress reporting accurate
  - Validation comprehensive
  - Rollback tested
  - Unit tests pass

#### P2.8: Integration Tests for All Repositories (10-12 hours)
- **Files to Create:**
  - `internal/infrastructure/storage/integration_test.go`
- **Approach:**
  - Test full workflows (create → read → update → delete)
  - Test cross-repository interactions
  - Test index consistency
  - Test concurrent access (RWMutex)
- **Acceptance Criteria:**
  - Integration tests for all repos pass
  - Concurrent access tested
  - Index consistency verified
  - No data races

**Phase 2 Deliverables:**
- [ ] All 6 file repositories implemented and tested
- [ ] Migration utility functional
- [ ] Unit tests passing (>90% coverage)
- [ ] Integration tests passing
- [ ] All quality gates passing

**Dependencies:** Phase 1 complete

---

### Phase 3: Testing & Validation (Week 5)

**Status:** ⬜ Not Started

**Goal:** Validate migration on sample datasets and benchmark performance

**Duration:** 1 week (40-50 hours)

**Tasks (7 tasks):**

#### P3.1: Test Migration on Small Dataset (4-6 hours)
- **Approach:**
  - Create sample SQLite database (10 users, 100 conversations, 1000 memory cells)
  - Run dry-run migration
  - Execute migration
  - Validate data integrity
- **Acceptance Criteria:**
  - Migration completes successfully
  - 100% data integrity (row counts match)
  - Checksums validate
  - No data loss

#### P3.2: Test Migration on Medium Dataset (6-8 hours)
- **Approach:**
  - Create sample database (100 users, 1000 conversations, 10,000 memory cells)
  - Run dry-run migration
  - Execute migration
  - Validate data integrity
  - Measure performance
- **Acceptance Criteria:**
  - Migration completes in reasonable time (<30 min)
  - 100% data integrity
  - Performance acceptable

#### P3.3: Test Migration on Large Dataset (8-10 hours)
- **Approach:**
  - Create sample database (1000 users, 10,000 conversations, 100,000 memory cells)
  - Run dry-run migration
  - Execute migration (may take hours)
  - Validate data integrity
  - Measure performance
- **Acceptance Criteria:**
  - Migration completes successfully (even if slow)
  - 100% data integrity
  - Performance documented
  - Memory usage acceptable

#### P3.4: Performance Benchmarks (8-10 hours)
- **Approach:**
  - Benchmark all repository operations
  - Compare file-based vs SQLite performance
  - Test search performance
  - Profile hot paths
- **Acceptance Criteria:**
  - Profile read: <50ms (p90) ✅
  - Conversation read: <100ms (p90) ✅
  - Memory search: <500ms ✅
  - Note operations: <100ms ✅
  - Benchmarks documented

#### P3.5: Edge Case Testing (6-8 hours)
- **Approach:**
  - Test empty database migration
  - Test very large conversations (10,000+ messages)
  - Test missing/corrupted files
  - Test concurrent access
  - Test disk full scenarios
- **Acceptance Criteria:**
  - All edge cases handled gracefully
  - No panics
  - Useful error messages

#### P3.6: Rollback Testing (4-6 hours)
- **Approach:**
  - Migrate sample data
  - Rollback to SQLite
  - Validate rolled-back data
  - Test multiple rollback scenarios
- **Acceptance Criteria:**
  - Rollback functional
  - Data restored correctly
  - No data loss during rollback

#### P3.7: Data Integrity Validation (4-6 hours)
- **Approach:**
  - Implement comprehensive validation checks
  - Compare SQLite vs file data
  - Verify relationships (user → conversations, conv → messages)
  - Check index consistency
- **Acceptance Criteria:**
  - Validation script complete
  - All checks pass on sample data
  - Relationships verified

**Phase 3 Deliverables:**
- [ ] Migration tested on 3 dataset sizes
- [ ] Performance benchmarks complete
- [ ] Edge cases handled
- [ ] Rollback verified
- [ ] Data integrity validation passing

**Dependencies:** Phase 2 complete

---

### Phase 4: Deployment (Week 6)

**Status:** ⬜ Not Started

**Goal:** Execute production migration and validate

**Duration:** 1 week (20-30 hours + monitoring)

**Tasks (4 tasks):**

#### P4.1: Backup Existing SQLite Databases (2-4 hours)
- **Approach:**
  - Copy `nuimanbot.db` to backup location
  - Copy `nuimanbot-memory.db` to backup location
  - Verify backups readable
  - Document backup locations
- **Acceptance Criteria:**
  - Backups created
  - Backups verified
  - Backup paths documented

#### P4.2: Execute Production Migration (4-8 hours)
- **Approach:**
  - Run dry-run migration first
  - Review dry-run results
  - Execute migration with progress reporting
  - Monitor for errors
  - Capture logs
- **Acceptance Criteria:**
  - Migration completes successfully
  - Progress logged
  - No errors
  - All data migrated

#### P4.3: Validate Migrated Data (4-6 hours)
- **Approach:**
  - Run validation script
  - Spot-check random samples
  - Compare row counts
  - Verify indexes
  - Test application functionality
- **Acceptance Criteria:**
  - Validation passes
  - Spot checks pass
  - Application functional
  - No data loss

#### P4.4: Monitor Performance Metrics (8-12 hours over 1 week)
- **Approach:**
  - Monitor application performance
  - Track read/write latencies
  - Check memory usage
  - Gather user feedback
  - Document any issues
- **Acceptance Criteria:**
  - Performance within targets
  - No user-reported issues
  - Metrics logged
  - Issues triaged

**Phase 4 Deliverables:**
- [ ] Production migration complete
- [ ] Data validated
- [ ] Performance monitored
- [ ] SQLite backup retained (30 days)

**Dependencies:** Phase 3 complete

---

### Phase 5: Cleanup (Week 7)

**Status:** ⬜ Not Started

**Goal:** Remove SQLite code and finalize migration

**Duration:** 1 week (30-40 hours)

**Tasks (5 tasks):**

#### P5.1: Remove SQLite Repository Implementations (8-10 hours)
- **Approach:**
  - Remove `internal/adapter/repository/sqlite/` directory
  - Update main.go to remove SQLite initialization
  - Remove SQLite-specific configuration
- **Acceptance Criteria:**
  - SQLite repos deleted
  - main.go updated
  - Application builds and runs
  - All tests pass

#### P5.2: Remove SQLite Dependencies from go.mod (2-4 hours)
- **Approach:**
  - Remove `github.com/mattn/go-sqlite3` dependency
  - Remove `modernc.org/sqlite` dependency
  - Run `go mod tidy`
  - Verify no orphaned dependencies
- **Acceptance Criteria:**
  - Dependencies removed
  - go.mod clean
  - Application builds
  - No CGO dependencies

#### P5.3: Update All Documentation (8-10 hours)
- **Approach:**
  - Update README.md (installation, configuration)
  - Update technical-details.md (architecture, storage)
  - Update product-summary.md if needed
  - Update AGENTS.md if needed
- **Acceptance Criteria:**
  - README reflects file-based storage
  - No mention of SQLite in user docs
  - Architecture docs updated
  - Migration guide included

#### P5.4: Remove Database-Related Configuration (4-6 hours)
- **Approach:**
  - Remove `storage.type: sqlite` from config.yaml
  - Remove `storage.dsn` setting
  - Add `storage.base_path` setting
  - Update config validation
- **Acceptance Criteria:**
  - Config updated
  - Validation passes
  - Example config correct
  - Migration notes in config

#### P5.5: Performance Tuning Based on Real Usage (8-10 hours)
- **Approach:**
  - Analyze performance metrics from Phase 4
  - Identify bottlenecks
  - Optimize hot paths
  - Add caching if needed
  - Re-benchmark
- **Acceptance Criteria:**
  - Performance optimized
  - Benchmarks improved
  - No regressions
  - Optimizations documented

**Phase 5 Deliverables:**
- [ ] SQLite code removed
- [ ] Dependencies cleaned
- [ ] Documentation updated
- [ ] Performance tuned

**Dependencies:** Phase 4 complete + 30-day monitoring period

---

## Critical Path

**Critical Path Tasks (Must complete sequentially):**

```
P1.4 (Test Strategy) → P2.1 (ProfileRepo) → P2.2 (ConvRepo) → P2.3 (MemoryCellRepo)
   [2h]                   [8h]               [12h]              [16h]
                                                ↓
                          → P2.7 (Migration Tool) → P3.1-3.3 (Test Migration)
                             [20h]                   [20h]
                                                      ↓
                          → P4.2 (Execute Migration) → P4.3 (Validate)
                             [8h]                      [6h]

Total Critical Path: ~92 hours (11.5 days at 8h/day)
```

**Parallel Work Opportunities:**
- P2.4 (SceneRepo), P2.5 (NotesRepo), P2.6 (AuditRepo) can run in parallel with P2.1-2.3
- P3.4 (Benchmarks), P3.5 (Edge cases) can run in parallel with P3.1-3.3
- P5.1-P5.4 can run in parallel

**Blocking Dependencies:**
- All Phase 2 repos must complete before P2.7 (migration tool)
- Migration tool must complete before Phase 3 testing
- Testing must complete before Phase 4 deployment
- Deployment must complete + 30 days before Phase 5 cleanup

---

## Dependency Graph

**Visual Dependency Map:**

```
Phase 1: Preparation (Week 2)
    ├─ P1.1 ✅ (data-dictionary.md)
    ├─ P1.2 ✅ (repository interfaces)
    ├─ P1.3 ✅ (migration spec)
    └─ P1.4 ✅ (test strategy)
          ↓
Phase 2: Implementation (Weeks 3-4)
    ├─ P2.1 (ProfileRepo) ──┐
    ├─ P2.2 (ConvRepo) ─────┤
    ├─ P2.3 (MemoryCellRepo)├─→ P2.7 (Migration Tool)
    ├─ P2.4 (SceneRepo) ────┤
    ├─ P2.5 (NotesRepo) ────┤
    └─ P2.6 (AuditRepo) ────┘
          ↓
    └─ P2.8 (Integration Tests)
          ↓
Phase 3: Testing (Week 5)
    ├─ P3.1 (Small dataset) ──┐
    ├─ P3.2 (Medium dataset) ─┤
    ├─ P3.3 (Large dataset) ──┼─→ P4.2 (Execute Migration)
    ├─ P3.4 (Benchmarks) ─────┤
    ├─ P3.5 (Edge cases) ─────┤
    ├─ P3.6 (Rollback) ───────┤
    └─ P3.7 (Validation) ─────┘
          ↓
Phase 4: Deployment (Week 6)
    ├─ P4.1 (Backup) → P4.2 (Migrate) → P4.3 (Validate)
    └─ P4.4 (Monitor) ─────────────────────────┘
          ↓ (+ 30 days)
Phase 5: Cleanup (Week 7)
    ├─ P5.1 (Remove SQLite repos)
    ├─ P5.2 (Remove dependencies)
    ├─ P5.3 (Update docs)
    ├─ P5.4 (Remove DB config)
    └─ P5.5 (Performance tuning)
```

**External Dependencies:**
- None (pure Go, file system only)

---

## Testing Strategy

### Unit Testing

**Approach:**
- **Test-first (TDD)**: Write failing test → Implement → Refactor
- **One test file per implementation file**: `*_test.go` in same package
- **Table-driven tests**: Use table-driven tests for multiple scenarios
- **Coverage target**: >90% for all new code

**Test Organization:**
```
internal/infrastructure/storage/
├── file_user_profile_repository.go
├── file_user_profile_repository_test.go
├── file_conversation_repository.go
├── file_conversation_repository_test.go
...
```

**Key Test Scenarios:**

**FileUserProfileRepository:**
- [ ] SaveProfile creates new profile
- [ ] SaveProfile updates existing profile
- [ ] GetProfileByUserID retrieves correct profile
- [ ] GetProfileByUserID returns ErrNotFound for missing profile
- [ ] GetProfileByEmail finds profile
- [ ] GetProfileByPlatformID finds profile
- [ ] ListProfiles handles pagination
- [ ] DeleteProfile removes profile
- [ ] Concurrent SaveProfile operations (RWMutex)
- [ ] Atomic write failure handling

**FileConversationRepository:**
- [ ] SaveConversation creates new conversation
- [ ] SaveConversation updates existing conversation
- [ ] GetConversation retrieves full conversation with messages
- [ ] ListConversations uses index (no full load)
- [ ] DeleteConversation removes conversation and updates index
- [ ] AppendMessage adds message to conversation
- [ ] Large conversations (10,000+ messages) handled
- [ ] Index consistency after operations
- [ ] Concurrent access safe

**FileMemoryCellRepository:**
- [ ] Create inserts new cell
- [ ] Create returns ErrAlreadyExists for duplicate
- [ ] Get retrieves cell by ID
- [ ] List with filters works
- [ ] Delete removes cell and updates indexes
- [ ] SearchFTS finds cells by keywords
- [ ] SearchFTS handles multi-keyword queries (AND)
- [ ] GetByScene retrieves cells for scene
- [ ] GetHighSalience filters by threshold
- [ ] DeleteExpired removes old cells
- [ ] All 4 indexes maintained correctly
- [ ] Index rebuild from data files works

**FileMemorySceneRepository:**
- [ ] Upsert creates new scene
- [ ] Upsert updates existing scene
- [ ] Get retrieves scene by name
- [ ] List returns all scenes
- [ ] Delete removes scene

**FileNotesRepository:**
- [ ] Create inserts new note
- [ ] GetByID retrieves note
- [ ] List retrieves all notes for user
- [ ] Update modifies note
- [ ] Delete removes note
- [ ] Tag index maintained
- [ ] Search by tag works

**FileAuditRepository:**
- [ ] Append writes JSONL line
- [ ] Append doesn't corrupt existing log
- [ ] Query with filters reads JSONL
- [ ] Query uses streaming (no full load)
- [ ] Concurrent appends safe

### Integration Testing

**Approach:**
- Test component interactions
- Use real file system (temp directories)
- Test full workflows end-to-end
- Clean up test data after tests

**Integration Test Suites:**

**Suite 1: User Profile Workflow**
- Create user profile
- Load profile by various lookups
- Update profile
- Verify changes persisted
- Delete profile
- Verify deleted

**Suite 2: Conversation Workflow**
- Create conversation with messages
- Append messages
- Load conversation
- List conversations (verify index)
- Delete conversation
- Verify index updated

**Suite 3: Memory Workflow**
- Create memory cells for multiple scenes
- Create memory scenes
- Search cells by keyword
- Get cells by scene
- Get high-salience cells
- Delete expired cells
- Verify indexes consistent

**Suite 4: Cross-Repository Interactions**
- Create user profile
- Create conversation for user
- Create memory cells for conversation
- Create notes for user
- Verify relationships
- Delete user (cascade?)

**Suite 5: Concurrent Access**
- Multiple goroutines reading same profile
- Multiple goroutines writing different profiles
- Verify no data races
- Verify RWMutex correct

### Migration Testing

**Approach:**
- Test migration on various dataset sizes
- Test dry-run mode
- Test rollback
- Validate data integrity

**Migration Test Scenarios:**

**MT-1: Empty Database Migration**
- Migrate empty SQLite database
- Verify no errors
- Verify empty file structure created

**MT-2: Small Dataset Migration**
- 10 users, 100 conversations, 1000 cells
- Verify all data migrated
- Verify indexes built
- Verify relationships maintained

**MT-3: Large Dataset Migration**
- 1000 users, 10,000 conversations, 100,000 cells
- Verify migration completes
- Verify data integrity
- Measure performance

**MT-4: Dry-Run Mode**
- Run dry-run migration
- Verify no files written
- Verify report generated
- Verify estimates accurate

**MT-5: Rollback**
- Migrate data
- Rollback to SQLite
- Verify data restored
- Verify no data loss

**MT-6: Validation**
- Migrate data
- Run validation script
- Verify row counts match
- Verify relationships intact
- Verify indexes consistent

### Performance Testing

**Benchmarks:**

```go
// Benchmark profile operations
func BenchmarkSaveProfile(b *testing.B)
func BenchmarkGetProfile(b *testing.B)

// Benchmark conversation operations
func BenchmarkSaveConversation(b *testing.B)
func BenchmarkGetConversation(b *testing.B)
func BenchmarkListConversations(b *testing.B)

// Benchmark memory operations
func BenchmarkCreateCell(b *testing.B)
func BenchmarkSearchCells(b *testing.B)
func BenchmarkGetByScene(b *testing.B)

// Benchmark notes operations
func BenchmarkCreateNote(b *testing.B)
func BenchmarkListNotes(b *testing.B)
```

**Performance Targets:**
- Profile operations: <50ms (p90)
- Conversation read: <100ms (p90)
- Memory search: <500ms
- Note operations: <100ms

**Load Testing:**
- Simulate 1000 conversations loaded
- Simulate 10,000 memory cells searched
- Verify memory usage acceptable
- Verify no performance degradation

### End-to-End Testing

**Approach:**
- Test real user workflows via CLI
- Use migrated file-based storage
- Verify application functionality

**E2E Test Scenarios:**

**E2E-1: Chat Workflow**
- Start new conversation
- Send multiple messages
- Verify messages saved
- List conversations
- Load previous conversation
- Continue conversation

**E2E-2: Memory Workflow**
- Create memory cells during chat
- Search memory
- Verify results correct
- View memory by scene
- Delete old memories

**E2E-3: Notes Workflow**
- Create note
- List notes
- Update note
- Search notes by tag
- Delete note

**E2E-4: User Management**
- Create user profile
- Update profile
- View profile
- Switch between multiple users

### Security Testing

**Security Checks:**
- [ ] Input validation (prevent path traversal)
- [ ] File permissions correct (0644 for data, 0755 for dirs)
- [ ] No sensitive data in logs
- [ ] Atomic writes prevent partial data exposure
- [ ] No SQL injection (not applicable - file-based)

---

## Migration Utility Specification

### Overview

The migration utility migrates data from SQLite databases to file-based storage with validation, rollback capability, and progress reporting.

**Commands:**
- `migrate sqlite-to-files` - Execute migration
- `migrate rollback-files-to-sqlite` - Rollback migration
- `migrate validate` - Validate migrated data
- `migrate dry-run` - Simulate migration without writing

---

### Command: migrate sqlite-to-files

**Usage:**
```bash
nuimanbot migrate sqlite-to-files [flags]
```

**Flags:**
- `--dry-run` - Simulate migration without writing files
- `--verify` - Validate after migration
- `--backup-path <path>` - Custom backup location
- `--progress` - Show progress bar
- `--continue-on-error` - Continue on non-critical errors
- `--verbose` - Verbose logging

**Execution Flow:**

**Step 1: Pre-Migration Checks**
- Verify SQLite databases exist (`nuimanbot.db`, `nuimanbot-memory.db`)
- Check disk space (need 2x current DB size)
- Verify write permissions on target directory
- Check if target directory already exists (warn if so)

**Step 2: Backup**
- Copy `nuimanbot.db` to `nuimanbot.db.backup.<timestamp>`
- Copy `nuimanbot-memory.db` to `nuimanbot-memory.db.backup.<timestamp>`
- Verify backups readable
- Log backup paths

**Step 3: Initialize File Structure**
- Create `data/users/` directory
- Create `data/system/` directory
- Set permissions correctly

**Step 4: Migrate User Profiles**
- Query all users from SQLite
- For each user:
  - Merge User + UserProfile data (if both exist)
  - Create user directory: `data/users/<user-id>/`
  - Write `profile.json`
  - Validate written profile
- Update progress: "Migrated X/Y users"

**Step 5: Migrate Conversations**
- Query all conversations from SQLite
- For each conversation:
  - Load all messages
  - Create conversation file: `data/users/<user-id>/conversations/<conv-id>.json`
  - Update conversation index
- Update progress: "Migrated X/Y conversations"

**Step 6: Migrate Memory Cells**
- Query all memory cells from SQLite
- For each cell:
  - Create cell file: `data/users/<user-id>/memory/cells/<cell-id>.json`
  - Track for index building
- Build memory indexes after all cells migrated
- Update progress: "Migrated X/Y memory cells"

**Step 7: Migrate Memory Scenes**
- Query all memory scenes from SQLite
- For each scene:
  - Create scene file: `data/users/<user-id>/memory/scenes/<scene-name>.json`
- Update progress: "Migrated X/Y memory scenes"

**Step 8: Migrate Notes**
- Query all notes from SQLite
- For each note:
  - Create note file: `data/users/<user-id>/notes/<note-id>.json`
  - Track for index building
- Build notes index after all notes migrated
- Update progress: "Migrated X/Y notes"

**Step 9: Migrate Audit Logs**
- Query all audit logs from SQLite (if any)
- Write to `data/system/audit.jsonl` (JSONL format)
- Update progress: "Migrated X/Y audit entries"

**Step 10: Build Indexes**
- Build all conversation indexes
- Build all memory indexes (4 types)
- Build all notes indexes
- Update progress: "Building indexes..."

**Step 11: Validation (if --verify flag)**
- Compare row counts (SQLite vs files)
- Verify checksums
- Check relationships
- Report validation results

**Step 12: Summary**
- Report migration statistics:
  - Users migrated: X
  - Conversations migrated: Y
  - Messages migrated: Z
  - Memory cells migrated: A
  - Memory scenes migrated: B
  - Notes migrated: C
  - Audit entries migrated: D
  - Duration: HH:MM:SS
  - Success: Yes/No
- Log backup paths

**Progress Reporting:**
```
Migrating SQLite data to file-based storage...

[1/9] Pre-migration checks...        ✓
[2/9] Backing up databases...        ✓ (Backed up to: /path/to/backup)
[3/9] Initializing file structure... ✓
[4/9] Migrating user profiles...     [██████████----------] 50% (10/20 users)
[5/9] Migrating conversations...     [████████████--------] 60% (600/1000 conversations)
[6/9] Migrating memory cells...      [████████████████----] 80% (8000/10000 cells)
[7/9] Migrating memory scenes...     [████████████████████] 100% (50/50 scenes)
[8/9] Migrating notes...             [████████████████████] 100% (100/100 notes)
[9/9] Building indexes...            ✓

Migration complete! ✓

Statistics:
  Users:         20
  Conversations: 1000
  Messages:      50000
  Memory cells:  10000
  Memory scenes: 50
  Notes:         100
  Duration:      00:05:32

Backup location: /path/to/nuimanbot.db.backup.2026-02-16-10-30-00

Next steps:
1. Validate migrated data: nuimanbot migrate validate
2. Test application with file-based storage
3. If issues found: nuimanbot migrate rollback-files-to-sqlite
```

---

### Command: migrate rollback-files-to-sqlite

**Usage:**
```bash
nuimanbot migrate rollback-files-to-sqlite [flags]
```

**Flags:**
- `--backup-path <path>` - Path to backup SQLite databases
- `--force` - Force rollback without confirmation

**Execution Flow:**

**Step 1: Confirmation**
- Prompt user: "This will delete all file-based data and restore from SQLite backup. Continue? [y/N]"
- If not --force flag, require user confirmation

**Step 2: Locate Backup**
- Find most recent backup if --backup-path not specified
- Verify backup exists and is readable
- Show backup path to user

**Step 3: Delete File-Based Data**
- Remove `data/users/` directory
- Remove `data/system/` directory (optional: keep audit log)
- Log deletions

**Step 4: Restore SQLite Databases**
- Copy backup to original location
- Verify databases readable
- Run SQLite integrity check

**Step 5: Verification**
- Query row counts from restored databases
- Compare with expected counts (from migration log, if available)
- Report results

**Step 6: Summary**
- Report rollback successful
- Remind user to restart application

---

### Command: migrate validate

**Usage:**
```bash
nuimanbot migrate validate [flags]
```

**Flags:**
- `--verbose` - Detailed validation output

**Execution Flow:**

**Step 1: Count Validation**
- Compare row counts:
  - SQLite users vs file profiles
  - SQLite conversations vs file conversations
  - SQLite messages vs file messages (in conversations)
  - SQLite memory cells vs file cells
  - SQLite memory scenes vs file scenes
  - SQLite notes vs file notes
- Report any mismatches

**Step 2: Relationship Validation**
- Verify all conversations reference valid users
- Verify all messages reference valid conversations
- Verify all memory cells reference valid conversations
- Verify all notes reference valid users
- Report any orphaned records

**Step 3: Index Validation**
- Verify conversation indexes accurate
- Verify memory indexes accurate (4 types)
- Verify notes indexes accurate
- Rebuild indexes if mismatches found

**Step 4: Data Integrity Validation**
- Random sample validation (10% of records)
- Compare SQLite data vs file data
- Verify timestamps, IDs, content match
- Report any mismatches

**Step 5: Summary**
- Report validation results:
  - Passed: X checks
  - Failed: Y checks
  - Warnings: Z checks
- Overall: Pass/Fail

**Example Output:**
```
Validating migrated data...

[1/5] Validating row counts...       ✓
  Users:         20 / 20
  Conversations: 1000 / 1000
  Messages:      50000 / 50000
  Memory cells:  10000 / 10000
  Memory scenes: 50 / 50
  Notes:         100 / 100

[2/5] Validating relationships...    ✓
  All conversations reference valid users
  All messages reference valid conversations
  All memory cells reference valid conversations
  All notes reference valid users

[3/5] Validating indexes...          ⚠
  Conversation index: OK
  Memory index (by-scene): OK
  Memory index (by-type): OK
  Memory index (by-salience): Outdated - rebuilt ✓
  Memory index (search): OK
  Notes index: OK

[4/5] Validating data integrity...   ✓
  Sampled 1000 random records
  100% match SQLite data

[5/5] Summary                        ✓

Validation passed! ✓
Warnings: 1 (memory index rebuilt automatically)
```

---

## Rollout Strategy

### Development Environment

**Phase:** Implementation and testing (Phases 2-3)

**Steps:**
1. Implement file repositories
2. Write unit tests
3. Run integration tests
4. Test migration on sample data

**Criteria:**
- All unit tests passing (>90% coverage)
- Integration tests passing
- Migration tested on sample data

**Rollback:** Git revert (code not deployed)

---

### Staging Environment (Optional)

**Phase:** Pre-production validation (Phase 3)

**Steps:**
1. Copy production-like SQLite database
2. Run migration on staging
3. Validate migrated data
4. Test application functionality
5. Run performance benchmarks

**Criteria:**
- Migration successful
- Validation passing
- Performance targets met
- Application functional

**Rollback:** Restore from backup

---

### Production Environment

**Phase:** Production deployment (Phase 4)

**Deployment Strategy:**
- Feature flag: Not applicable (storage layer change)
- Gradual rollout: Not applicable (single-user CLI tool)
- Blue-green deployment: Not applicable (file-based migration)

**Go-Live Checklist:**
- [ ] All Phase 3 tests passed in staging
- [ ] Performance benchmarks meet targets
- [ ] Migration utility tested and validated
- [ ] Backup strategy confirmed
- [ ] Rollback plan tested
- [ ] Documentation updated
- [ ] User notified of migration (if multi-user)

**Deployment Steps:**
1. Stop application (if running)
2. Backup SQLite databases
3. Run migration: `nuimanbot migrate sqlite-to-files --verify --progress`
4. Validate migration results
5. Update main.go to use file repositories (or rebuild with new code)
6. Start application
7. Verify functionality
8. Monitor for issues

**Rollback Plan:**
1. Stop application
2. Run rollback: `nuimanbot migrate rollback-files-to-sqlite`
3. Restore original main.go (or rebuild old version)
4. Start application
5. Verify functionality

**Post-Deployment:**
- Monitor performance for 1 week
- Gather user feedback
- Retain SQLite backup for 30 days
- After 30 days: Proceed to Phase 5 cleanup

---

## Success Metrics

### Development Metrics

**Code Quality:**
- Test coverage: >90% ✓
- Linter warnings: 0 ✓
- All quality gates pass ✓

**Velocity:**
- Phase 2 complete: Week 4 ✓
- Phase 3 complete: Week 5 ✓
- Phase 4 complete: Week 6 ✓

### Migration Metrics

**Correctness:**
- Migration success rate: 100% ✓
- Data integrity: 100% ✓
- Zero data loss ✓

**Performance:**
- Profile read: <50ms (p90)
- Conversation read: <100ms (p90)
- Memory search: <500ms
- Note operations: <100ms

### Operational Metrics

**Reliability:**
- Application uptime: >99% (no crashes)
- Error rate: <1%
- No data corruption

**Adoption:**
- Migration completed successfully
- User feedback positive
- No rollback required

### Post-Migration Metrics (30 days)

**Performance:**
- Read latencies within targets
- Write latencies within targets
- Memory usage acceptable

**Reliability:**
- No file corruption
- No data loss
- Indexes consistent

**User Satisfaction:**
- No user-reported issues related to storage
- Positive feedback on data portability
- No performance complaints

---

## Timeline Summary

**Total Estimated Duration:** 6 weeks

**Phase Breakdown:**
- Phase 0: Code Cleanup - Complete ✅ (Week 1)
- Phase 1: Preparation - 50% complete (Week 2)
- Phase 2: Implementation - Not started (Weeks 3-4, 80-100 hours)
- Phase 3: Testing - Not started (Week 5, 40-50 hours)
- Phase 4: Deployment - Not started (Week 6, 20-30 hours)
- Phase 5: Cleanup - Not started (Week 7 + 30 days, 30-40 hours)

**Key Milestones:**
- End of Week 2: Planning complete ✓
- End of Week 4: All repositories implemented and tested
- End of Week 5: Migration tested and validated
- End of Week 6: Production migration complete
- Week 7 + 30 days: Cleanup complete

**Contingency:**
- Buffer: 20% (1 week for unknown unknowns)
- Total with contingency: 7 weeks

---

## Risk Mitigation Timeline

### Pre-Implementation (Week 2)
- [x] All dependencies verified (no external deps)
- [x] Architecture validated (Clean Architecture, file-based)
- [x] Team capacity confirmed (solo or small team)

### Implementation (Weeks 3-4)
- [ ] TDD followed strictly (tests first, then implement)
- [ ] Daily progress tracking (% complete by task)
- [ ] Blockers escalated immediately

### Testing (Week 5)
- [ ] Migration tested on 3 dataset sizes
- [ ] Performance benchmarks documented
- [ ] Rollback validated

### Deployment (Week 6)
- [ ] Backup verified before migration
- [ ] Migration monitored in real-time
- [ ] Validation automated
- [ ] Rollback plan ready

---

## Assumptions

- Single-user CLI tool (no concurrent users)
- Datasets < 100,000 records (conversations, cells, notes)
- Local file system (not network storage)
- Development environment has SQLite databases with sample data
- Go 1.21+ (for testing features)

---

## Open Questions

None (all questions resolved in research and design phases)

---

## Constraints

- Must maintain domain interfaces unchanged
- Must support rollback to SQLite
- Must validate data integrity (zero data loss)
- Must meet performance targets
- Must follow TDD (Red-Green-Refactor)
- Must pass all quality gates

---

## References

- [spec.md](spec.md) - Feature specification
- [research.md](research.md) - Research findings
- [data-dictionary.md](data-dictionary.md) - Data structures
- [architecture.md](architecture.md) - System architecture
- [AGENTS.md](../../AGENTS.md) - Development workflow and quality gates

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-02-16 | Initial implementation plan complete |
