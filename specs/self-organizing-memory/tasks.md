# Self-Organizing Memory v2 - Task Breakdown

**Feature:** Self-Organizing Memory v2
**Version:** 1.0
**Created:** 2026-02-15
**Status:** Planning
**Estimated Total Duration:** 40-54 hours

---

## Task Organization

Tasks are organized by implementation phase following Clean Architecture layers (bottom-up). Each task includes:
- **ID:** Unique task identifier (e.g., P0.1, P1.1)
- **Phase:** Implementation phase (0-6)
- **Dependencies:** Prerequisites that must complete first
- **Estimated Duration:** Time estimate in hours
- **Priority:** P0 (Critical), P1 (High), P2 (Medium), P3 (Low)
- **Description:** What needs to be done
- **Files to Create/Modify:** Specific files affected
- **Acceptance Criteria:** Definition of done (checkboxes)
- **Verification Commands:** Commands to verify completion

**Development Approach:** TDD + Bottom-Up Clean Architecture
- Red → Green → Refactor for each component
- Domain → Infrastructure → Use Case → Adapter
- Each phase produces working, tested code

---

## Progress Summary

**Overall Progress:** 2/41 tasks complete (5%)

**By Phase:**
- Phase 0 (Planning): 2/5 tasks complete (40%)
- Phase 1 (Domain): 0/8 tasks complete (0%)
- Phase 2 (Infrastructure): 0/6 tasks complete (0%)
- Phase 3 (Use Case): 0/7 tasks complete (0%)
- Phase 4 (Adapter): 0/6 tasks complete (0%)
- Phase 5 (Admin): 0/5 tasks complete (0%)
- Phase 6 (Documentation): 0/4 tasks complete (0%)

**By Priority:**
- P0 (Critical): 2/30 complete (7%)
- P1 (High): 0/8 complete (0%)
- P2 (Medium): 0/3 complete (0%)

---

## Phase 0: Planning (4-6 hours)

### Task P0.1: PRD Analysis and Comparison

**ID:** P0.1
**Dependencies:** None
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ✅ Complete

**Description:**
Analyze the MarkTechPost PRD and compare with existing Nuimanbot implementation to identify gaps and integration points

**Files Created:**
- ✅ `Feature-Self-Organizing-Memory-PRD.md`

**Acceptance Criteria:**
- [x] PRD analyzed and understood
- [x] Gaps identified between article approach and Nuimanbot
- [x] Integration points documented
- [x] Merge strategy defined

**Verification:** Manual review of PRD document

---

### Task P0.2: Feature Specification Document

**ID:** P0.2
**Dependencies:** P0.1
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ✅ Complete

**Description:**
Create comprehensive feature specification from PRD covering all requirements, architecture, and success criteria

**Files Created:**
- ✅ `specs/self-organizing-memory/spec.md`
- ✅ `specs/self-organizing-memory/status.md`
- ✅ `specs/self-organizing-memory/research.md`

**Acceptance Criteria:**
- [x] Spec.md complete with all sections
- [x] All functional requirements documented
- [x] All non-functional requirements documented
- [x] Success criteria defined
- [x] Risks and mitigations identified

**Verification:** Manual review of spec.md

---

### Task P0.3: Data Dictionary

**ID:** P0.3
**Dependencies:** P0.2
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ✅ Complete

**Description:**
Define all entities, types, interfaces, and database schemas

**Files Created:**
- ✅ `specs/self-organizing-memory/data-dictionary.md`

**Acceptance Criteria:**
- [x] All domain entities documented
- [x] All interfaces documented
- [x] Database schemas defined
- [x] Type mappings specified
- [x] Validation rules documented

**Verification:** Manual review of data-dictionary.md

---

### Task P0.4: Architecture Document

**ID:** P0.4
**Dependencies:** P0.3
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ✅ Complete

**Description:**
Document system architecture, component design, data flows, and architectural decisions

**Files Created:**
- ✅ `specs/self-organizing-memory/architecture.md`

**Acceptance Criteria:**
- [x] Component architecture documented
- [x] Data flow diagrams created
- [x] Sequence diagrams created
- [x] Architectural decisions recorded (ADRs)
- [x] Trade-offs documented

**Verification:** Manual review of architecture.md

---

### Task P0.5: Implementation Plan and Task Breakdown

**ID:** P0.5
**Dependencies:** P0.4
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** 🟡 In Progress

**Description:**
Create detailed implementation plan with phases, task breakdown, and timeline

**Files Created:**
- 🟡 `specs/self-organizing-memory/plan.md` (in progress)
- 🟡 `specs/self-organizing-memory/tasks.md` (this file)
- [ ] `specs/self-organizing-memory/implementation-notes.md`

**Acceptance Criteria:**
- [x] Plan.md complete with all phases
- [x] Tasks.md complete with all tasks
- [ ] Implementation-notes.md template created
- [ ] All tasks have estimates and acceptance criteria
- [ ] Dependencies mapped

**Verification:**
```bash
# Check all planning files exist
ls specs/self-organizing-memory/*.md

# Should show: spec.md, status.md, research.md, data-dictionary.md,
#              architecture.md, plan.md, tasks.md, implementation-notes.md
```

---

## Phase 1: Domain Layer (6-8 hours)

### Task P1.1: Define MemoryCell Entity

**ID:** P1.1
**Dependencies:** P0.5
**Duration:** 1.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement MemoryCell entity with all fields, validation rules, and business logic

**Files to Create:**
- `internal/domain/memory_cell.go` - MemoryCell entity
- `internal/domain/memory_cell_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] MemoryCell struct with all fields defined
- [ ] Validate() method implements all validation rules
- [ ] IsExpired() method checks expiration
- [ ] String() method for human-readable representation
- [ ] Unit tests for all validation rules
- [ ] Unit tests for edge cases (empty fields, invalid salience, etc.)
- [ ] Tests for IsExpired() with various timestamps
- [ ] Code coverage >90%

**Implementation Details:**
```go
type MemoryCell struct {
    ID             string
    ConversationID string
    Scene          string
    CellType       CellType
    Salience       float64
    Content        string
    Source         string    // JSON array
    CreatedAt      time.Time
    UpdatedAt      time.Time
    ExpiresAt      *time.Time
}

func (m *MemoryCell) Validate() error {
    // Validate ID (UUID)
    // Validate ConversationID (non-empty, <=128 chars)
    // Validate Scene (pattern, length)
    // Validate CellType (enum)
    // Validate Salience (0.0-1.0)
    // Validate Content (non-empty, <=2000 chars)
    // Validate Source (valid JSON)
    // Validate timestamps
}
```

**Verification Commands:**
```bash
# Run tests
go test ./internal/domain -run TestMemoryCell -v

# Check coverage
go test ./internal/domain -run TestMemoryCell -cover

# Build
go build -o bin/nuimanbot ./cmd/nuimanbot
```

---

### Task P1.2: Define MemoryScene Entity

**ID:** P1.2
**Dependencies:** None (parallel with P1.1)
**Duration:** 0.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement MemoryScene entity with validation and methods

**Files to Create:**
- `internal/domain/memory_scene.go` - MemoryScene entity
- `internal/domain/memory_scene_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] MemoryScene struct defined
- [ ] Validate() method implements validation rules
- [ ] String() method implemented
- [ ] Unit tests for validation
- [ ] Code coverage >90%

**Verification Commands:**
```bash
go test ./internal/domain -run TestMemoryScene -v -cover
```

---

### Task P1.3: Define CellType Enum

**ID:** P1.3
**Dependencies:** None (parallel)
**Duration:** 0.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement CellType enum with parsing and validation

**Files to Create:**
- `internal/domain/cell_type.go` - CellType enum
- `internal/domain/cell_type_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] CellType enum defined (fact, decision, task, preference, plan, risk)
- [ ] String() method returns enum name
- [ ] IsValid() method validates enum value
- [ ] ParseCellType() parses string to enum
- [ ] Unit tests for all enum values
- [ ] Unit tests for invalid parsing
- [ ] Code coverage 100%

**Implementation:**
```go
type CellType int

const (
    CellTypeFact CellType = iota
    CellTypeDecision
    CellTypeTask
    CellTypePreference
    CellTypePlan
    CellTypeRisk
)

func (c CellType) String() string { ... }
func (c CellType) IsValid() bool { ... }
func ParseCellType(s string) (CellType, error) { ... }
```

**Verification:**
```bash
go test ./internal/domain -run TestCellType -v -cover
```

---

### Task P1.4: Define MemoryCellRepository Interface

**ID:** P1.4
**Dependencies:** P1.1, P1.3
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Define repository interface for memory cell operations

**Files to Create:**
- `internal/domain/memory_cell_repository.go` - Interface definition

**Acceptance Criteria:**
- [ ] All CRUD methods defined (Create, Get, List, Delete)
- [ ] SearchFTS method defined
- [ ] GetByScene method defined
- [ ] GetHighSalience method defined
- [ ] DeleteExpired method defined
- [ ] All methods have clear documentation
- [ ] Error conditions documented

**Implementation:**
```go
type MemoryCellRepository interface {
    Create(ctx context.Context, cell *MemoryCell) error
    Get(ctx context.Context, id string) (*MemoryCell, error)
    List(ctx context.Context, filter MemoryCellFilter) ([]*MemoryCell, error)
    Delete(ctx context.Context, id string) error
    SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error)
    GetByScene(ctx context.Context, scene string, limit int) ([]*MemoryCell, error)
    GetHighSalience(ctx context.Context, conversationID string, threshold float64, limit int) ([]*MemoryCell, error)
    DeleteExpired(ctx context.Context) (int, error)
}
```

**Verification:** Manual review (interface only, no tests needed)

---

### Task P1.5: Define MemorySceneRepository Interface

**ID:** P1.5
**Dependencies:** P1.2
**Duration:** 0.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Define repository interface for scene operations

**Files to Create:**
- `internal/domain/memory_scene_repository.go` - Interface definition

**Acceptance Criteria:**
- [ ] Upsert, Get, List, Delete methods defined
- [ ] All methods documented
- [ ] Error conditions documented

**Verification:** Manual review

---

### Task P1.6: Domain Layer Unit Tests

**ID:** P1.6
**Dependencies:** P1.1, P1.2, P1.3
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Write comprehensive unit tests for all domain entities

**Acceptance Criteria:**
- [ ] All validation rules tested
- [ ] All edge cases tested (empty, nil, invalid, boundaries)
- [ ] All methods tested (Validate, IsExpired, String, etc.)
- [ ] Table-driven tests for enum parsing
- [ ] Code coverage >90%

**Verification:**
```bash
go test ./internal/domain/... -cover
# Should show >90% coverage

go test ./internal/domain/... -v
# All tests pass
```

---

### Task P1.7: Domain Layer Quality Gates

**ID:** P1.7
**Dependencies:** P1.6
**Duration:** 0.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Run all quality gates on domain layer

**Acceptance Criteria:**
- [ ] `go fmt ./internal/domain/...` - All files formatted
- [ ] `go mod tidy` - Dependencies clean
- [ ] `go vet ./internal/domain/...` - No warnings
- [ ] `golangci-lint run ./internal/domain/...` - No errors
- [ ] `go test ./internal/domain/... -cover` - >90% coverage
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` - Builds successfully

**Verification:**
```bash
# Run all gates
go fmt ./internal/domain/... && \
go mod tidy && \
go vet ./internal/domain/... && \
golangci-lint run ./internal/domain/... && \
go test ./internal/domain/... -cover && \
go build -o bin/nuimanbot ./cmd/nuimanbot
```

---

### Task P1.8: Domain Layer Refactoring

**ID:** P1.8
**Dependencies:** P1.7
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Refactor domain layer code for clarity, maintainability, and DRY principles

**Acceptance Criteria:**
- [ ] No code duplication (DRY)
- [ ] Clear, descriptive names for types and methods
- [ ] Constants extracted for magic numbers/strings
- [ ] Error messages consistent and clear
- [ ] All tests still pass after refactoring

**Verification:**
```bash
# Re-run quality gates after refactoring
go test ./internal/domain/... -cover
```

---

## Phase 2: Infrastructure Layer (8-10 hours)

### Task P2.1: Create SQLite Schema Migration

**ID:** P2.1
**Dependencies:** P1.4, P1.5
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Create SQL migration script for memory tables with FTS5

**Files to Create:**
- `internal/infrastructure/memory/migrations/001_memory_tables.sql`
- `internal/infrastructure/memory/migrations/migrations_test.go`

**Acceptance Criteria:**
- [ ] `memory_cells` table created
- [ ] `memory_scenes` table created
- [ ] `memory_cells_fts` virtual table created (FTS5)
- [ ] Triggers created to sync FTS table (insert, update, delete)
- [ ] Indexes created (conversation_id, scene, salience, expires_at)
- [ ] Constraints defined (salience check, cell_type check)
- [ ] Migration script tested (runs successfully)
- [ ] Rollback script created

**SQL Schema:**
```sql
-- memory_cells table
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
);

-- Indexes
CREATE INDEX idx_memory_cells_conversation ON memory_cells(conversation_id);
CREATE INDEX idx_memory_cells_scene ON memory_cells(scene);
CREATE INDEX idx_memory_cells_salience ON memory_cells(salience DESC);
CREATE INDEX idx_memory_cells_expires_at ON memory_cells(expires_at) WHERE expires_at IS NOT NULL;

-- FTS5 virtual table
CREATE VIRTUAL TABLE memory_cells_fts USING fts5(
    content,
    scene,
    cell_type,
    content='memory_cells',
    content_rowid='rowid'
);

-- Triggers to sync FTS
CREATE TRIGGER memory_cells_ai AFTER INSERT ON memory_cells BEGIN
    INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
    VALUES (new.rowid, new.content, new.scene, new.cell_type);
END;

-- ... (update and delete triggers)

-- memory_scenes table
CREATE TABLE memory_scenes (
    scene TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT chk_token_count CHECK (token_count > 0 AND token_count <= 2000)
);
```

**Verification:**
```bash
# Test migration
go test ./internal/infrastructure/memory/migrations -v

# Apply migration to test DB
sqlite3 test.db < internal/infrastructure/memory/migrations/001_memory_tables.sql

# Verify tables created
sqlite3 test.db "SELECT name FROM sqlite_master WHERE type='table';"
# Should show: memory_cells, memory_scenes, memory_cells_fts
```

---

### Task P2.2: Implement SQLiteMemoryCellRepository

**ID:** P2.2
**Dependencies:** P2.1
**Duration:** 3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement SQLite repository for memory cells with FTS search

**Files to Create:**
- `internal/infrastructure/memory/sqlite_cell_repository.go`
- `internal/infrastructure/memory/sqlite_cell_repository_test.go`

**Acceptance Criteria:**
- [ ] All MemoryCellRepository interface methods implemented
- [ ] FTS search implemented with BM25 ranking
- [ ] Parameterized queries (prevent SQL injection)
- [ ] Error handling for constraints, not found, etc.
- [ ] Unit tests with mocked DB (where possible)
- [ ] Integration tests with real SQLite
- [ ] Code coverage >90%

**FTS Search Implementation:**
```go
func (r *SQLiteMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*domain.MemoryCell, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT mc.* FROM memory_cells mc
        JOIN memory_cells_fts fts ON mc.rowid = fts.rowid
        WHERE memory_cells_fts MATCH ?
        ORDER BY rank
        LIMIT ?
    `, query, limit)
    // ... parse rows
}
```

**Verification:**
```bash
go test ./internal/infrastructure/memory -run TestSQLiteMemoryCellRepository -v -cover
```

---

### Task P2.3: Implement SQLiteMemorySceneRepository

**ID:** P2.3
**Dependencies:** P2.1
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement SQLite repository for memory scenes

**Files to Create:**
- `internal/infrastructure/memory/sqlite_scene_repository.go`
- `internal/infrastructure/memory/sqlite_scene_repository_test.go`

**Acceptance Criteria:**
- [ ] All MemorySceneRepository interface methods implemented
- [ ] Upsert uses INSERT OR REPLACE
- [ ] Unit tests
- [ ] Code coverage >90%

**Verification:**
```bash
go test ./internal/infrastructure/memory -run TestSQLiteMemorySceneRepository -v -cover
```

---

### Task P2.4: Infrastructure Integration Tests

**ID:** P2.4
**Dependencies:** P2.2, P2.3
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Write integration tests with real SQLite database

**Test Scenarios:**
- [ ] CRUD operations work end-to-end
- [ ] FTS search returns relevant results
- [ ] Constraints enforced (salience, cell_type, token_count)
- [ ] Concurrent operations work (multiple goroutines)
- [ ] Triggers sync FTS table correctly
- [ ] DeleteExpired removes expired cells

**Verification:**
```bash
go test ./internal/infrastructure/memory -v -cover
# Run integration tests with real DB
```

---

### Task P2.5: Infrastructure Performance Benchmarks

**ID:** P2.5
**Dependencies:** P2.4
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Write benchmarks for repository operations

**Files to Create:**
- `internal/infrastructure/memory/bench_test.go`

**Acceptance Criteria:**
- [ ] Benchmark FTS queries with 10K, 100K cells
- [ ] Benchmark CRUD operations
- [ ] Document performance targets
- [ ] FTS query <50ms for 10K cells

**Benchmark:**
```go
func BenchmarkSearchFTS_10K(b *testing.B) {
    // Setup DB with 10K cells
    for i := 0; i < b.N; i++ {
        repo.SearchFTS(ctx, "authentication", 10)
    }
}
```

**Verification:**
```bash
go test ./internal/infrastructure/memory -bench=. -benchmem
```

---

### Task P2.6: Infrastructure Quality Gates & Refactoring

**ID:** P2.6
**Dependencies:** P2.5
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Run quality gates and refactor infrastructure layer

**Acceptance Criteria:**
- [ ] All quality gates pass
- [ ] SQL queries extracted to constants
- [ ] No code duplication
- [ ] Error handling consistent
- [ ] All tests pass

**Verification:**
```bash
go fmt ./internal/infrastructure/memory/... && \
go vet ./internal/infrastructure/memory/... && \
golangci-lint run ./internal/infrastructure/memory/... && \
go test ./internal/infrastructure/memory/... -cover
```

---

## Phase 3: Use Case Layer (10-12 hours)

### Task P3.1: Design Memory Extraction Prompt

**ID:** P3.1
**Dependencies:** P2.6
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Design and test LLM prompt for memory cell extraction

**Files to Create:**
- `internal/usecase/memory/prompts.go` - Prompt templates

**Acceptance Criteria:**
- [ ] Extraction prompt produces valid JSON >95% of time
- [ ] Prompt tested with 20+ sample conversations
- [ ] Salience scores reasonable (manual review)
- [ ] Scene names consistent (lowercase-with-dashes)
- [ ] Cell types correctly assigned

**Prompt Template:**
```
You are a memory curator for an AI assistant...

# Conversation Interaction
## User Message: ...
## Assistant Response: ...

# Task
Extract memory cells as JSON: {"cells": [...]}
```

**Verification:** Manual testing with sample conversations

---

### Task P3.2: Implement MemoryCuratorService

**ID:** P3.2
**Dependencies:** P3.1
**Duration:** 4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement service for cell extraction and scene consolidation

**Files to Create:**
- `internal/usecase/memory/curator_service.go`
- `internal/usecase/memory/curator_service_test.go`
- `internal/usecase/memory/types.go` - DTOs

**Acceptance Criteria:**
- [ ] ExtractCells method calls LLM with prompt
- [ ] JSON parsing handles invalid responses
- [ ] Retry logic for failures (max 1 retry)
- [ ] ConsolidateScene generates summaries
- [ ] Unit tests with mocked LLM and repositories
- [ ] Code coverage >90%

**Implementation:**
```go
type MemoryCuratorService struct {
    cellRepo     domain.MemoryCellRepository
    sceneRepo    domain.MemorySceneRepository
    llmProvider  LLMProvider
}

func (s *MemoryCuratorService) ExtractCells(ctx context.Context, input ExtractionInput) (ExtractionOutput, error) {
    // 1. Build extraction prompt
    // 2. Call LLM
    // 3. Parse JSON
    // 4. Validate cells
    // 5. Persist cells
    // 6. Trigger scene consolidation
}
```

**Verification:**
```bash
go test ./internal/usecase/memory -run TestMemoryCurator -v -cover
```

---

### Task P3.3: Implement MemoryRecallService

**ID:** P3.3
**Dependencies:** P2.6 (parallel with P3.2)
**Duration:** 3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement service for memory retrieval and ranking

**Files to Create:**
- `internal/usecase/memory/recall_service.go`
- `internal/usecase/memory/recall_service_test.go`

**Acceptance Criteria:**
- [ ] Recall method extracts keywords from prompt
- [ ] FTS search returns ranked results
- [ ] Salience fallback when FTS yields few results
- [ ] Scene summaries retrieved for matched cells
- [ ] Memory formatted as prompt-ready text
- [ ] Token budget enforced
- [ ] Unit tests with mocked repositories
- [ ] Code coverage >90%

**Implementation:**
```go
func (s *MemoryRecallService) Recall(ctx context.Context, input RecallInput) (RecallOutput, error) {
    // 1. SearchFTS with prompt keywords
    // 2. If FTS < threshold, fallback to GetHighSalience
    // 3. Get scene summaries for matched cells
    // 4. Format memory injection
    // 5. Enforce token budget
}
```

**Verification:**
```bash
go test ./internal/usecase/memory -run TestMemoryRecall -v -cover
```

---

### Task P3.4: Use Case Integration Tests with LLM

**ID:** P3.4
**Dependencies:** P3.2, P3.3
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Integration tests with real Anthropic API

**Test Scenarios:**
- [ ] Extract cells from real conversation
- [ ] Consolidate scene summary
- [ ] Recall retrieves relevant cells
- [ ] End-to-end: extract → consolidate → recall

**Verification:**
```bash
# Run integration tests (requires API key)
ANTHROPIC_API_KEY=sk-... go test ./internal/usecase/memory -v -tags=integration
```

---

### Task P3.5: Measure Curator Accuracy

**ID:** P3.5
**Dependencies:** P3.4
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Measure extraction accuracy with ground truth

**Acceptance Criteria:**
- [ ] 20-30 conversations manually labeled
- [ ] Curator run on same conversations
- [ ] Precision and recall calculated
- [ ] Curator success rate >95%

**Verification:** Manual accuracy measurement

---

### Task P3.6: Use Case Quality Gates & Refactoring

**ID:** P3.6
**Dependencies:** P3.5
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Quality gates and refactoring

**Acceptance Criteria:**
- [ ] All tests pass
- [ ] Code coverage >90%
- [ ] Linter clean
- [ ] Prompts extracted to constants
- [ ] Error messages clear

**Verification:**
```bash
go test ./internal/usecase/memory/... -cover
golangci-lint run ./internal/usecase/memory/...
```

---

### Task P3.7: Circuit Breaker Implementation

**ID:** P3.7
**Dependencies:** P3.6
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Add circuit breaker pattern for LLM failures

**Acceptance Criteria:**
- [ ] Circuit breaker wraps LLM calls
- [ ] Opens after N consecutive failures
- [ ] Resets after timeout
- [ ] Tests for circuit breaker logic

**Verification:**
```bash
go test ./internal/usecase/memory -run TestCircuitBreaker -v
```

---

## Phase 4: Adapter Layer (6-8 hours)

### Task P4.1: Integrate MemoryCurator into ChatService

**ID:** P4.1
**Dependencies:** P3.6
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Wire curator into ProcessMessage for async extraction

**Files to Modify:**
- `internal/usecase/chat/service.go`

**Acceptance Criteria:**
- [ ] Curator triggered after response sent (async)
- [ ] Error handling doesn't break chat
- [ ] Logging for extraction success/failure
- [ ] Metrics emitted

**Implementation:**
```go
// In ProcessMessage, after sending response:
go func() {
    err := s.memoryCurator.ExtractCells(context.Background(), ExtractionInput{...})
    if err != nil {
        log.Error("Memory extraction failed", "error", err)
        // Don't crash chat
    }
}()
```

**Verification:**
```bash
# Manual testing: send message, check cells extracted
./bin/nuimanbot
> user message
# Check DB for cells
sqlite3 data/nuimanbot.db "SELECT * FROM memory_cells;"
```

---

### Task P4.2: Integrate MemoryRecall into BuildContextWindow

**ID:** P4.2
**Dependencies:** P3.6
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Wire recall into context window builder

**Files to Modify:**
- `internal/usecase/chat/context_window.go`

**Acceptance Criteria:**
- [ ] Recall called with prompt
- [ ] Memory injected before message history
- [ ] Graceful degradation on failure
- [ ] Token budget enforced

**Implementation:**
```go
func (s *ChatService) BuildContextWindow(...) {
    // Recall memory
    memory, err := s.memoryRecall.Recall(ctx, RecallInput{...})
    if err != nil {
        log.Warn("Memory recall failed, continuing without memory")
        memory = RecallOutput{}
    }

    // Inject memory
    context := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "system", Content: memory.FormattedText},
    }
    context = append(context, recentMessages...)
}
```

**Verification:**
```bash
# Manual testing: memory in context improves responses
```

---

### Task P4.3: Implement CLI Memory Commands

**ID:** P4.3
**Dependencies:** P3.6
**Duration:** 2 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Add CLI commands for memory inspection

**Files to Create:**
- `internal/adapter/cli/memory.go`

**Commands:**
- `memory list-scenes`
- `memory get-scene <scene>`
- `memory search --query=X --scene=Y`
- `memory delete-cell <id>`
- `memory delete-scene <scene>`

**Acceptance Criteria:**
- [ ] All commands implemented
- [ ] Output formatted nicely (table or JSON)
- [ ] Error handling
- [ ] Help text

**Verification:**
```bash
./bin/nuimanbot memory list-scenes
./bin/nuimanbot memory search --query="authentication"
```

---

### Task P4.4: Update Dependency Injection

**ID:** P4.4
**Dependencies:** P4.1, P4.2, P4.3
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Wire all memory components in main.go

**Files to Modify:**
- `cmd/nuimanbot/main.go`
- `internal/config/config.go`

**Acceptance Criteria:**
- [ ] Repositories initialized
- [ ] Services initialized
- [ ] Config loaded
- [ ] Dependencies injected into ChatService

**Verification:**
```bash
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help  # Should not crash
```

---

### Task P4.5: End-to-End Tests

**ID:** P4.5
**Dependencies:** P4.4
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Write E2E tests for full memory flow

**Files to Create:**
- `e2e/memory_test.go`

**Test Scenarios:**
- [ ] User sends message → memory extracted → cells in DB
- [ ] User sends message → memory recalled → context enhanced
- [ ] Curator failure doesn't break chat
- [ ] Recall failure degrades gracefully

**Verification:**
```bash
go test ./e2e -v -tags=e2e
```

---

### Task P4.6: Adapter Quality Gates

**ID:** P4.6
**Dependencies:** P4.5
**Duration:** 0.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Run quality gates on all code

**Acceptance Criteria:**
- [ ] All tests pass
- [ ] Code coverage >90% overall
- [ ] Linter clean
- [ ] Build succeeds

**Verification:**
```bash
go test ./... -cover
golangci-lint run
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help
```

---

## Phase 5: Admin & Observability (4-6 hours)

### Task P5.1: Add RBAC to Admin Commands

**ID:** P5.1
**Dependencies:** P4.6
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Enforce admin-only access to delete operations

**Acceptance Criteria:**
- [ ] Delete operations check admin role
- [ ] Audit logging for deletions
- [ ] Error if non-admin attempts delete

**Verification:**
```bash
# Test as non-admin user (should fail)
./bin/nuimanbot memory delete-cell abc123
# Error: Permission denied
```

---

### Task P5.2: Add Metrics Instrumentation

**ID:** P5.2
**Dependencies:** P4.6
**Duration:** 2 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Instrument memory operations with metrics

**Metrics:**
- Cells created per day
- Recall hit rate (FTS vs salience fallback)
- Injected token count (avg, p95, p99)
- Curator failures

**Acceptance Criteria:**
- [ ] Metrics emitted from curator and recall
- [ ] Metrics visible in dashboard
- [ ] Documentation for metrics

**Verification:** Check metrics dashboard

---

### Task P5.3: Add Tracing Spans

**ID:** P5.3
**Dependencies:** P4.6
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Add tracing for memory operations

**Spans:**
- `memory.curator.extract_cells`
- `memory.curator.consolidate_scene`
- `memory.recall.recall`

**Acceptance Criteria:**
- [ ] Tracing enabled for all memory operations
- [ ] Spans visible in observability tool

**Verification:** Check tracing dashboard

---

### Task P5.4: Configure Logging and Alerting

**ID:** P5.4
**Dependencies:** P5.2, P5.3
**Duration:** 1 hour
**Priority:** P2 (Medium)
**Status:** ⬜ Not Started

**Description:**
Configure logging and alerts

**Alerts:**
- Curator failure rate >10%
- Recall timeout rate >5%

**Acceptance Criteria:**
- [ ] All failures logged
- [ ] Alerts configured
- [ ] Alert tested (manual trigger)

**Verification:** Trigger alert manually

---

### Task P5.5: Write Admin Documentation

**ID:** P5.5
**Dependencies:** P5.1
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Document admin tools and memory management

**Files to Create:**
- `support_docs/memory-admin-guide.md`

**Content:**
- How to inspect memory
- How to delete cells/scenes
- How to interpret metrics
- Troubleshooting guide

**Verification:** Manual review

---

## Phase 6: Documentation & Polish (2-4 hours)

### Task P6.1: Update Product Documentation

**ID:** P6.1
**Dependencies:** P5.5
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Update all product docs with memory features

**Files to Modify:**
- `README.md`
- `documentation/product-details.md`
- `documentation/technical-details.md`

**Acceptance Criteria:**
- [ ] README updated with memory overview
- [ ] Product-details updated
- [ ] Technical-details updated with architecture

**Verification:** Manual review

---

### Task P6.2: Write Memory User Guide

**ID:** P6.2
**Dependencies:** P6.1
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Write user-facing memory guide

**Files to Create:**
- `support_docs/memory-guide.md`

**Content:**
- What is memory
- How it works
- How to enable/disable
- How to inspect memory
- FAQ

**Verification:** Manual review

---

### Task P6.3: Write Migration Guide

**ID:** P6.3
**Dependencies:** P6.1
**Duration:** 0.5 hours
**Priority:** P2 (Medium)
**Status:** ⬜ Not Started

**Description:**
Write migration guide for users

**Files to Create:**
- `support_docs/memory-migration-guide.md`

**Content:**
- Config changes
- Feature flag usage
- Rollout best practices

**Verification:** Manual review

---

### Task P6.4: Final Review and Polish

**ID:** P6.4
**Dependencies:** P6.1, P6.2, P6.3
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Final review of all documentation and code

**Acceptance Criteria:**
- [ ] All docs reviewed
- [ ] No typos or errors
- [ ] All links valid
- [ ] No contradictions

**Verification:** Manual review

---

## Estimation Accuracy Tracking

| Task ID | Estimated | Actual | Variance | Notes |
|---------|-----------|--------|----------|-------|
| P0.1    | 2h        | 2h     | 0%       | Accurate |
| P0.2    | 2h        | 2h     | 0%       | Accurate |
| P0.3    | 1h        | 1h     | 0%       | Accurate |
| P0.4    | 1h        | 1h     | 0%       | Accurate |
| P0.5    | 1h        | [TBD]  | [TBD]    | In progress |

**Average Variance:** 0% (so far)

---

## Notes

**Assumptions:**
- Anthropic API available and stable
- SQLite supports FTS5 (verified)
- Existing ChatService stable
- 1-2 weeks dedicated development time

**Open Questions:**
- None remaining (all answered in research.md)

**CRITICAL REMINDER:**
- **Update status.md after EVERY task completion**
- **Run quality gates before marking task complete**
- **Follow Red-Green-Refactor for ALL tasks**
- **Build to bin/ and run ./bin/nuimanbot to verify**
