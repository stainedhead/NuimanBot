# File-Based Storage - Task Breakdown

**Created:** 2026-02-16
**Version:** 1.0
**Status:** Active
**Approach:** Greenfield implementation (no migration from SQLite)

---

## Overview

This document provides a concrete, testable task breakdown for implementing file-based storage in NuimanBot. Each task follows TDD methodology (Red-Green-Refactor) and includes clear acceptance criteria.

**Key Approach:**
- **Greenfield Implementation**: No migration from SQLite—build file repositories from scratch
- **TDD**: Write tests first, implement to pass, refactor for quality
- **Bottom-Up**: Infrastructure (file repos) → Integration → Use cases
- **Quality Gates**: All tasks must pass quality gates before completion

**Total Estimated Duration:** 6-7 weeks (170-210 hours)

---

## Phase 2: Implementation

**Status:** ⬜ Not Started
**Duration:** 2-3 weeks (80-100 hours)

**Goal:** Implement all 6 file-based repositories with comprehensive tests.

---

### P2.1: Implement FileUserProfileRepository
**Duration:** 6-8 hours | **Priority:** P0

**Description:**
Implement file-based user profile repository with unified UserProfile entity.

**Files to Create:**
- `internal/infrastructure/storage/file_user_profile_repository.go`
- `internal/infrastructure/storage/file_user_profile_repository_test.go`

**Methods:** SaveProfile, GetProfileByUserID, GetProfileByEmail, GetProfileByPlatformID, ListProfiles, DeleteProfile, UpdateProfile

**Acceptance Criteria:**
- [ ] All 7 repository methods implemented
- [ ] Unit tests pass (>90% coverage)
- [ ] Atomic writes verified
- [ ] Profile operations < 50ms

---

### P2.2: Implement FileConversationRepository
**Duration:** 10-12 hours | **Priority:** P0

**Files to Create:**
- `internal/infrastructure/storage/file_conversation_repository.go`
- `internal/infrastructure/storage/file_conversation_repository_test.go`
- `internal/infrastructure/storage/conversation_index.go`

**Methods:** SaveConversation, GetConversation, ListConversations, DeleteConversation, GetConversationSummary, CountMessages

**Acceptance Criteria:**
- [ ] All 6 methods implemented
- [ ] Index maintained correctly
- [ ] Large conversation test (10,000 messages)
- [ ] Conversation load < 100ms (p90)

---

### P2.3: Implement FileMemoryCellRepository  
**Duration:** 12-16 hours | **Priority:** P0

**Files to Create:**
- `internal/infrastructure/storage/file_memory_cell_repository.go`
- `internal/infrastructure/storage/file_memory_cell_repository_test.go`
- `internal/infrastructure/storage/memory_indexes.go`
- `internal/infrastructure/storage/keyword_search.go`

**Methods:** Create, Get, List, Delete, SearchFTS (keyword), GetByScene, DeleteExpired, GetBySalience

**Acceptance Criteria:**
- [ ] All 8 methods implemented
- [ ] 4 indexes maintained (by-scene, by-type, by-salience, search)
- [ ] Keyword search functional
- [ ] Memory search < 500ms

---

### P2.4: Implement FileMemorySceneRepository
**Duration:** 4-6 hours | **Priority:** P0

**Files to Create:**
- `internal/infrastructure/storage/file_memory_scene_repository.go`
- `internal/infrastructure/storage/file_memory_scene_repository_test.go`

**Methods:** Upsert, Get, List, Delete

**Acceptance Criteria:**
- [ ] All 4 methods implemented
- [ ] Scene operations < 50ms

---

### P2.5: Implement FileNotesRepository
**Duration:** 6-8 hours | **Priority:** P1

**Files to Create:**
- `internal/infrastructure/storage/file_notes_repository.go`
- `internal/infrastructure/storage/file_notes_repository_test.go`
- `internal/infrastructure/storage/notes_index.go`

**Methods:** Create, GetByID, List, Update, Delete

**Acceptance Criteria:**
- [ ] All 5 methods implemented
- [ ] Tag index maintained
- [ ] Note operations < 100ms

---

### P2.6: Implement FileAuditRepository
**Duration:** 6-8 hours | **Priority:** P1

**Files to Create:**
- `internal/infrastructure/storage/file_audit_repository.go`
- `internal/infrastructure/storage/file_audit_repository_test.go`

**Methods:** Append, Query (with streaming)

**Acceptance Criteria:**
- [ ] JSONL format correct
- [ ] Streaming read (no full load)
- [ ] Concurrent Append safe

---

### P2.7: Integration Tests
**Duration:** 10-12 hours | **Priority:** P0

**Files to Create:**
- `internal/infrastructure/storage/integration_test.go`

**Test Scenarios:** New user onboarding, Conversation workflow, Memory workflow, Notes workflow, Audit workflow, Concurrent access, Index recovery

**Acceptance Criteria:**
- [ ] All 7 scenarios pass
- [ ] No data races (`go test -race`)
- [ ] Index consistency verified

---

### P2.8: Wire File Repositories into Main
**Duration:** 4-6 hours | **Priority:** P0

**Files to Modify:**
- `cmd/nuimanbot/main.go`
- `config.yaml`

**Acceptance Criteria:**
- [ ] File repositories wired
- [ ] Application starts successfully
- [ ] All CLI commands functional
- [ ] Data persists to files

---

## Phase 3: Testing & Validation

**Status:** ⬜ Not Started
**Duration:** 1-2 weeks (40-50 hours)

### P3.1: Performance Benchmarks (8-10 hours, P0)
### P3.2: Edge Case Testing (8-10 hours, P0)
### P3.3: End-to-End Testing (10-12 hours, P1)
### P3.4: Security Testing (6-8 hours, P1)
### P3.5: Data Integrity Validation (6-8 hours, P0)

---

## Phase 4: Deployment

**Status:** ⬜ Not Started
**Duration:** 1 week (20-30 hours)

### P4.1: Create Data Directory Structure (2-3 hours, P0)
### P4.2: Initialize in Production (4-6 hours, P0)
### P4.3: Setup Monitoring (6-8 hours, P1)
### P4.4: Backup Scripts (6-8 hours, P1)

---

## Phase 5: Cleanup

**Status:** ⬜ Not Started  
**Duration:** 1 week (30-40 hours)

### P5.1: Remove SQLite Repos (8-10 hours, P1)
### P5.2: Remove SQLite Dependencies (2-3 hours, P1)
### P5.3: Update Documentation (8-10 hours, P1)
### P5.4: Remove DB Config (2-3 hours, P2)
### P5.5: Performance Tuning (8-10 hours, P2)

---

## Critical Path

**Critical tasks** (must complete in order):

Phase 2: P2.1 → P2.2 → P2.3 → P2.4 → P2.5 → P2.6 → P2.7 → P2.8
Phase 3: P3.1 → P3.2 → P3.5
Phase 4: P4.1 → P4.2 → P4.3

**Total Critical Path:** ~120-150 hours (3-4 weeks)

---

## Quality Gates

Before marking ANY task complete:

- [ ] `go test ./...`
- [ ] `go fmt ./...`
- [ ] `go mod tidy`
- [ ] `go vet ./...`
- [ ] `golangci-lint run`
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot`
- [ ] `./bin/nuimanbot --help`

---

## Notes

**No Migration Work:** This is greenfield implementation—no SQLite data to migrate.

**TDD Required:** Write tests first, implement, refactor. All three phases mandatory.

**Update Status:** Update status.md after completing each task (MANDATORY).
