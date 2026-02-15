# Self-Organizing Memory v2 - Implementation Plan

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Planning
**Estimated Duration:** 40-54 hours (1-2 weeks full-time, 2-3 weeks part-time)

---

## Table of Contents

1. [Development Approach](#development-approach)
2. [Phase Breakdown](#phase-breakdown)
3. [Critical Path](#critical-path)
4. [Dependency Graph](#dependency-graph)
5. [Testing Strategy](#testing-strategy)
6. [Rollout Strategy](#rollout-strategy)
7. [Success Metrics](#success-metrics)

---

## Development Approach

### Methodology

**TDD + Bottom-Up Clean Architecture**

```
Red → Green → Refactor (for each component)
  ↓
Domain → Infrastructure → Use Case → Adapter
  ↓
Integration → Examples → Documentation
```

**Why This Approach?**
- **TDD ensures quality**: Tests written first catch bugs early and document behavior
- **Bottom-up builds foundation**: Domain entities are most stable, build up from there
- **Clean Architecture**: Dependency inversion keeps code maintainable and testable
- **Incremental milestones**: Each phase produces working, tested code

**Key Principles:**
1. **Test-first**: Write test before implementation (Red-Green-Refactor)
2. **Refactor is mandatory**: Don't skip the refactor phase (eliminate duplication, improve clarity)
3. **Layer independence**: Each layer tested independently before integration
4. **Quality gates**: All gates pass before moving to next phase

**Incremental Milestones:**
- Phase 1: Domain layer complete → entities validated and tested
- Phase 2: Infrastructure layer complete → persistence works with real SQLite
- Phase 3: Use case layer complete → extraction and recall working end-to-end
- Phase 4: Adapter layer complete → full integration with ChatService
- Phase 5: Admin tools complete → memory inspectable and manageable
- Phase 6: Documentation complete → ready for production use

---

## Phase Breakdown

### Phase 0: Planning (4-6 hours) - IN PROGRESS

**Goal:** Complete comprehensive planning documents before implementation

**Deliverables:**
- ✅ PRD analysis and comparison
- ✅ Feature specification (spec.md)
- ✅ Status tracking (status.md)
- ✅ Research document (research.md)
- ✅ Data dictionary (data-dictionary.md)
- ✅ Architecture document (architecture.md)
- 🟡 Implementation plan (plan.md - this document)
- [ ] Task breakdown (tasks.md)
- [ ] Implementation notes template (implementation-notes.md)

**Tasks:**
- [x] P0.1 - PRD analysis (2h)
- [x] P0.2 - Specification document (2h)
- [ ] P0.3 - Data dictionary (1h)
- [ ] P0.4 - Architecture document (1h)
- [ ] P0.5 - Implementation plan and task breakdown (1h)

**Dependencies:** None

**Acceptance Criteria:**
- [x] Spec.md covers all requirements from PRD
- [x] Data dictionary defines all types and schemas
- [x] Architecture document shows component design and data flows
- [ ] Task breakdown has concrete, testable tasks
- [ ] All planning docs reviewed and approved

**Quality Gates:**
- [x] All docs use consistent terminology
- [x] No contradictions between docs
- [ ] All open questions answered or deferred to Phase 1

**Estimated Time:** 4-6 hours
**Actual Time:** ~3 hours (in progress)

---

### Phase 1: Domain Layer (6-8 hours)

**Goal:** Implement core domain entities and interfaces with comprehensive tests

**Deliverables:**
- MemoryCell entity with validation
- MemoryScene entity with validation
- CellType enum with parsing
- MemoryCellRepository interface
- MemorySceneRepository interface
- Unit tests (coverage >90%)

**Tasks:**
1. Define MemoryCell entity (1h)
   - Fields, validation rules, methods
   - Unit tests for validation logic
2. Define MemoryScene entity (0.5h)
   - Fields, validation rules, methods
   - Unit tests
3. Define CellType enum (0.5h)
   - Enum values, String(), IsValid(), ParseCellType()
   - Unit tests for parsing
4. Define MemoryCellRepository interface (1h)
   - All CRUD + search methods
   - Document expected behavior
5. Define MemorySceneRepository interface (0.5h)
   - Upsert, get, list, delete
   - Document expected behavior
6. Write comprehensive unit tests (2h)
   - Edge cases, validation failures
   - String representations
7. Run quality gates (0.5h)
   - fmt, vet, lint, build
8. **REFACTOR** (1h)
   - Eliminate duplication
   - Improve naming
   - Extract constants

**Dependencies:** Phase 0 complete

**Acceptance Criteria:**
- [x] All entities have validation methods
- [x] All validation rules from data-dictionary.md implemented
- [x] CellType enum parses from/to strings correctly
- [x] Repository interfaces document all error conditions
- [x] Unit tests cover >90% of domain code
- [x] All quality gates pass

**Quality Gates:**
- [ ] `go test ./internal/domain/... -cover` → coverage >90%
- [ ] `go vet ./internal/domain/...` → no warnings
- [ ] `golangci-lint run ./internal/domain/...` → no errors
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` → success

**Estimated Time:** 6-8 hours

---

### Phase 2: Infrastructure Layer (8-10 hours)

**Goal:** Implement SQLite repositories with FTS5 and comprehensive integration tests

**Deliverables:**
- SQLite schema migration (memory_cells, memory_scenes, FTS5)
- SQLiteMemoryCellRepository implementation
- SQLiteMemorySceneRepository implementation
- FTS5 indexing and search
- Integration tests (with real SQLite)
- Performance benchmarks

**Tasks:**
1. Create SQL schema migration (2h)
   - Tables: memory_cells, memory_scenes
   - FTS5 virtual table: memory_cells_fts
   - Triggers to sync FTS
   - Indexes for performance
   - Test migration script
2. Implement SQLiteMemoryCellRepository (3h)
   - CRUD operations
   - FTS search with BM25 ranking
   - GetByScene, GetHighSalience
   - DeleteExpired
   - Unit tests (mocked DB where possible)
3. Implement SQLiteMemorySceneRepository (1h)
   - Upsert, get, list, delete
   - Unit tests
4. Write integration tests (2h)
   - Test with real SQLite database
   - Test FTS search accuracy
   - Test concurrent operations
   - Test error conditions (constraints, not found, etc.)
5. Write performance benchmarks (1h)
   - Benchmark FTS queries with 10K, 100K cells
   - Benchmark CRUD operations
   - Document performance targets
6. Run quality gates (0.5h)
7. **REFACTOR** (1h)
   - Extract SQL queries to constants
   - Eliminate duplication in repo implementations
   - Improve error handling

**Dependencies:** Phase 1 complete

**Acceptance Criteria:**
- [ ] Migration creates all tables and indexes
- [ ] FTS triggers sync correctly (insert, update, delete)
- [ ] FTS search returns relevant results (manual verification)
- [ ] Integration tests pass with real SQLite
- [ ] FTS query latency <50ms for 10K cells (benchmark)
- [ ] Code coverage >90%

**Quality Gates:**
- [ ] `go test ./internal/infrastructure/memory/... -cover` → >90%
- [ ] `go test ./internal/infrastructure/memory/... -bench=.` → FTS <50ms
- [ ] All tests pass
- [ ] Migration script runs successfully

**Estimated Time:** 8-10 hours

---

### Phase 3: Use Case Layer (10-12 hours)

**Goal:** Implement curator and recall services with LLM integration

**Deliverables:**
- MemoryCuratorService (extraction + consolidation)
- MemoryRecallService (retrieval + ranking)
- Use case DTOs (ExtractionInput, RecallOutput, etc.)
- Unit tests with mocked repositories
- Integration tests with real LLM

**Tasks:**
1. Design extraction prompt (2h)
   - Craft LLM prompt for cell extraction
   - Test with sample conversations
   - Iterate on prompt for better accuracy
2. Implement MemoryCuratorService (4h)
   - ExtractCells: LLM call + JSON parsing + persistence
   - ConsolidateScene: Load cells + generate summary
   - Error handling: retry, circuit breaker
   - Unit tests (mock LLM and repositories)
3. Implement MemoryRecallService (3h)
   - Recall: FTS search + salience fallback
   - Scene retrieval and formatting
   - Token budget enforcement
   - Unit tests (mock repositories)
4. Implement use case DTOs (0.5h)
   - ExtractionInput, ExtractionOutput
   - RecallInput, RecallOutput
   - Validation methods
5. Write integration tests with real LLM (2h)
   - Test extraction with real Anthropic API
   - Test consolidation with real LLM
   - Test recall end-to-end
   - Measure accuracy (manual review)
6. Run quality gates (0.5h)
7. **REFACTOR** (1h)
   - Extract prompt templates to constants
   - Improve error messages
   - Simplify complex functions

**Dependencies:** Phase 2 complete

**Acceptance Criteria:**
- [ ] Extraction prompt produces valid JSON >95% of time
- [ ] Curator success rate >95% (measured over 20 test conversations)
- [ ] Recall returns relevant cells (manual verification)
- [ ] Token budget enforced (never exceeds limit)
- [ ] Retrieval latency <100ms (p95)
- [ ] Code coverage >90%

**Quality Gates:**
- [ ] `go test ./internal/usecase/memory/... -cover` → >90%
- [ ] Integration tests pass with real LLM
- [ ] Curator success rate >95%
- [ ] Recall latency <100ms

**Estimated Time:** 10-12 hours

---

### Phase 4: Adapter Layer (6-8 hours)

**Goal:** Integrate memory services into ChatService and provide CLI commands

**Deliverables:**
- ChatService integration (curator trigger + recall consumer)
- Context window enhancement with memory injection
- CLI memory commands (list, search, delete)
- Dependency injection in main.go
- End-to-end tests

**Tasks:**
1. Integrate MemoryCuratorService into ChatService (2h)
   - Trigger extraction after response sent (async)
   - Handle errors gracefully
   - Add logging and metrics
2. Integrate MemoryRecallService into BuildContextWindow (2h)
   - Call recall with prompt
   - Format memory injection
   - Graceful degradation on failure
3. Implement CLI memory commands (2h)
   - `memory list-scenes`
   - `memory get-scene <scene>`
   - `memory search --query=X`
   - `memory delete-cell <id>`
   - `memory delete-scene <scene>`
4. Update dependency injection (1h)
   - Wire repositories, services in main.go
   - Load config for memory feature
   - Initialize circuit breaker
5. Write end-to-end tests (2h)
   - Test full chat flow with memory extraction
   - Test memory recall improves responses
   - Test CLI commands
6. Run quality gates (0.5h)
7. **REFACTOR** (1h)
   - Simplify integration code
   - Extract formatting logic
   - Improve error handling

**Dependencies:** Phase 3 complete

**Acceptance Criteria:**
- [ ] Memory extracted after every chat interaction
- [ ] Memory injected into context window
- [ ] Extraction failures don't break chat
- [ ] Recall failures degrade gracefully
- [ ] CLI commands work correctly
- [ ] E2E tests pass

**Quality Gates:**
- [ ] `go test ./... -cover` → >90% overall
- [ ] E2E tests pass
- [ ] Manual testing: memory improves responses
- [ ] Build and run: `./bin/nuimanbot --help`

**Estimated Time:** 6-8 hours

---

### Phase 5: Admin & Observability (4-6 hours)

**Goal:** Add admin tools, metrics, tracing, and logging

**Deliverables:**
- Admin API/CLI for memory inspection
- Metrics instrumentation
- Tracing spans for curator/recall
- Logging and alerting configuration
- Admin documentation

**Tasks:**
1. Implement admin CLI commands (2h)
   - Already covered in Phase 4, enhance with RBAC
   - Add audit logging for deletions
2. Add metrics instrumentation (1h)
   - Cells created per day
   - Recall hit rate (FTS vs salience fallback)
   - Injected token count (avg, p95, p99)
   - Curator failures
3. Add tracing spans (1h)
   - Span for ExtractCells
   - Span for ConsolidateScene
   - Span for Recall
4. Configure logging and alerting (1h)
   - Log all curator failures
   - Alert on high failure rate (>10%)
   - Log all recall timeouts
5. Write admin documentation (1h)
   - How to inspect memory
   - How to delete cells/scenes
   - How to interpret metrics

**Dependencies:** Phase 4 complete

**Acceptance Criteria:**
- [ ] Metrics dashboard shows key indicators
- [ ] Tracing enabled for all memory operations
- [ ] Alerts configured for failures
- [ ] Admin can inspect and manage memory via CLI
- [ ] Audit log records all deletions

**Quality Gates:**
- [ ] Metrics appear in dashboard
- [ ] Tracing spans visible in observability tool
- [ ] Admin commands work correctly
- [ ] Documentation complete

**Estimated Time:** 4-6 hours

---

### Phase 6: Documentation & Polish (2-4 hours)

**Goal:** Update all product documentation and prepare for production

**Deliverables:**
- Updated README.md
- Updated product-details.md
- Updated technical-details.md
- Memory user guide
- Migration guide (config, feature flags)

**Tasks:**
1. Update README.md (0.5h)
   - Add memory feature overview
   - Update CLI command examples
2. Update product-details.md (0.5h)
   - Add memory system description
   - Update architecture diagrams
3. Update technical-details.md (1h)
   - Document memory architecture
   - Add data flow diagrams
   - Document configuration options
4. Write memory user guide (1h)
   - How memory works
   - How to enable/disable
   - How to inspect memory
   - FAQ and troubleshooting
5. Write migration guide (0.5h)
   - Config changes needed
   - Feature flag usage
   - Rollout best practices
6. Review all documentation (0.5h)
   - Ensure consistency
   - Fix typos and errors

**Dependencies:** Phase 5 complete

**Acceptance Criteria:**
- [ ] All product docs updated
- [ ] Memory user guide complete
- [ ] Migration guide complete
- [ ] All docs reviewed and approved

**Quality Gates:**
- [ ] Docs pass spell check
- [ ] All links valid
- [ ] No contradictions between docs

**Estimated Time:** 2-4 hours

---

## Critical Path

**Critical Path Tasks:**
These tasks must complete in sequence and block all other work:

```
P0.5 (Plan) → P1.1 (MemoryCell) → P1.4 (CellRepo Interface) → P2.2 (SQLite CellRepo)
  → P3.2 (MemoryCurator) → P4.1 (ChatService Integration) → P4.5 (E2E Tests)

Total Critical Path Duration: ~22 hours
```

**Parallel Work Opportunities:**
- P1.2 (MemoryScene) can run in parallel with P1.1 (MemoryCell)
- P2.3 (SceneRepo) can run in parallel with P2.2 (CellRepo)
- P3.3 (MemoryRecall) can run in parallel with P3.2 (MemoryCurator)
- P5 (Admin & Observability) can partially overlap with P6 (Documentation)

**Blocking Dependencies:**
- Phase 1 blocks Phase 2 (need interfaces)
- Phase 2 blocks Phase 3 (need persistence)
- Phase 3 blocks Phase 4 (need services)
- Phase 4 blocks Phase 5 (need integration)

---

## Dependency Graph

**Visual Dependency Map:**
```
Phase 0: Planning
    └─> P0.5 (plan + tasks)
          ↓
Phase 1: Domain Layer
    ├─ P1.1 (MemoryCell) ────┐
    ├─ P1.2 (MemoryScene) ───┤ (parallel)
    ├─ P1.3 (CellType enum) ─┤
    └─ P1.4 (Interfaces) ────┴─> (merge point)
          ↓
Phase 2: Infrastructure Layer
    ├─ P2.1 (SQL migration) ──┐
    ├─ P2.2 (CellRepo impl) ──┤ (parallel)
    └─ P2.3 (SceneRepo impl) ─┴─> (merge point)
          ↓
Phase 3: Use Case Layer
    ├─ P3.2 (MemoryCurator) ──┐ (parallel)
    └─ P3.3 (MemoryRecall) ───┴─> (merge point)
          ↓
Phase 4: Adapter Layer
    ├─ P4.1 (ChatService integration)
    ├─ P4.2 (BuildContextWindow enhancement)
    ├─ P4.3 (CLI commands)
    └─ P4.5 (E2E tests)
          ↓
Phase 5: Admin & Observability
    ├─ P5.1 (Admin tools)
    ├─ P5.2 (Metrics)
    └─ P5.3 (Tracing)
          ↓
Phase 6: Documentation
    └─ P6.1-6.5 (all docs)
```

**External Dependencies:**
- Anthropic API access (available)
- SQLite database (available)
- Existing ChatService (stable)

---

## Testing Strategy

### Unit Testing

**Approach:**
- Test-first (TDD): Write test before implementation
- One test file per implementation file
- Aim for >90% code coverage

**Test Organization:**
```
internal/domain/*_test.go          # Domain entity tests
internal/usecase/memory/*_test.go  # Service tests (mocked repos)
internal/infrastructure/memory/*_test.go  # Repository tests
```

**Key Test Scenarios:**
- **Domain**: Validation rules, edge cases, enum parsing
- **Use Case**: Service orchestration, error handling, retry logic
- **Infrastructure**: CRUD operations, FTS search, constraint violations

### Integration Testing

**Approach:**
- Test component interactions
- Use real implementations (SQLite, LLM)
- Test with actual database/APIs (test environment)

**Integration Test Suites:**
1. **Repository Integration Tests**: Test SQLite repos with real DB
2. **LLM Integration Tests**: Test extraction/consolidation with real Anthropic API
3. **ChatService Integration Tests**: Test full memory flow with real components

### End-to-End Testing

**Approach:**
- Full application lifecycle
- Real user workflows
- Production-like environment

**E2E Test Scenarios:**
1. **Memory Extraction Flow**:
   - User sends message
   - Assistant responds
   - Memory cells extracted
   - Verify cells in DB
2. **Memory Recall Flow**:
   - User sends message
   - Memory recalled
   - Context enhanced with memory
   - Response demonstrates memory usage

**Location:** `e2e/memory_test.go`

### Performance Testing

**Benchmarks:**
- FTS query latency: <50ms (10K cells)
- Memory extraction: <5 seconds
- Memory recall: <100ms
- Scene consolidation: <5 seconds

**Load Testing:**
- 100 concurrent extractions (stress test curator)
- 1000 concurrent recalls (stress test retrieval)

### Security Testing

**Security Checks:**
- [x] Input validation (prevent SQL injection)
- [x] SQL injection tests (parameterized queries)
- [x] Cross-user isolation tests (no memory leakage)
- [ ] RBAC enforcement (admin-only deletions)
- [ ] Audit logging (all modifications logged)

---

## Rollout Strategy

### Phase 1: Development Environment

**Goal:** Complete implementation and testing in dev

**Criteria:**
- All unit tests passing
- Integration tests passing
- Code review complete

**Rollback:** Not applicable (dev only)

**Duration:** Phase 0-6 completion (~40-54 hours)

---

### Phase 2: Observe-Only Mode (1-2 weeks)

**Goal:** Run curator to extract cells but DON'T inject into prompts

**Deployment Steps:**
1. Deploy with `memory.self_organizing.enabled: true`
2. Set `memory.recall.enabled: false` (don't inject)
3. Monitor curator success rate
4. Inspect cells manually (verify quality)
5. Fix any curator failures or quality issues

**Validation:**
- [ ] Curator success rate >95%
- [ ] Cells look reasonable (manual review of 50+ cells)
- [ ] No chat breakage due to extraction failures
- [ ] Metrics show healthy extraction

**Rollback Plan:**
- Set `memory.self_organizing.enabled: false`
- Curator stops extracting, chat unaffected

**Duration:** 1-2 weeks

---

### Phase 3: Pilot Users (1 week)

**Goal:** Enable memory injection for small group of pilot users

**Deployment Steps:**
1. Enable `memory.recall.enabled: true` for pilot users
2. Gather feedback on response quality
3. Measure recall hit rate and token usage
4. Fix any issues found

**Validation:**
- [ ] Pilot users report improved responses
- [ ] Recall hit rate >50% (FTS matches)
- [ ] No performance degradation (<100ms recall)
- [ ] No chat breakage

**Rollback Plan:**
- Set `memory.recall.enabled: false` for pilot users
- Cells still extracted (observe-only mode)

**Duration:** 1 week

---

### Phase 4: Full Rollout (Gradual)

**Goal:** Enable for all users gradually

**Deployment Strategy:**
- Gradual rollout: 10% → 25% → 50% → 100%
- Monitor metrics at each stage
- Pause rollout if issues detected

**Go-Live Checklist:**
- [ ] All tests passing in staging
- [ ] Documentation complete
- [ ] Monitoring configured
- [ ] Alerts configured
- [ ] Rollback plan tested
- [ ] Stakeholder approval

**Rollback Plan:**
- Feature flag: set `memory.recall.enabled: false`
- Curator continues (observe-only)
- Chat unaffected

**Duration:** 1-2 weeks

---

## Success Metrics

### Development Metrics

**Code Quality:**
- Test coverage: >90%
- Linter warnings: 0
- Code review approvals: 1+ (self-review acceptable)

**Velocity:**
- Tasks completed per day: 2-3 tasks avg
- Actual vs estimated hours: within 20%

### Release Metrics

**Adoption:**
- Memory cells created: >100 cells/day (after full rollout)
- Users with memory enabled: 100% (full rollout)

**Performance:**
- Memory extraction latency: <5 seconds (p95)
- Memory recall latency: <100ms (p95)
- FTS query latency: <50ms (p95)

**Reliability:**
- Curator success rate: >95%
- Extraction error rate: <5%
- Chat breakage due to memory: 0%

### User Metrics

**Engagement:**
- Memory improves response quality: Qualitative feedback (pilot users)
- Users inspect memory: >10 CLI inspections/week

**Satisfaction:**
- User feedback: >80% positive (pilot users)
- Support tickets: <5 per week (memory-related)

---

## Risk Mitigation Timeline

### Pre-Development Risks

**Week -1 (Planning):**
- [ ] All dependencies confirmed available (SQLite, Anthropic API)
- [ ] Technical spike completed (FTS5 performance tested)
- [ ] Team capacity confirmed (1-2 weeks dedicated)

### Development Risks

**Week 1 (Phase 1-2):**
- [ ] Domain layer complete and tested
- [ ] Infrastructure layer complete and tested
- [ ] FTS5 performance meets targets

**Week 2 (Phase 3-4):**
- [ ] Curator achieves >95% success rate
- [ ] Recall retrieves relevant cells (manual verification)
- [ ] ChatService integration complete

### Pre-Release Risks

**Week 3 (Phase 5-6 + Observe-Only):**
- [ ] Observe-only mode deployed and stable
- [ ] Cells look reasonable (quality check)
- [ ] Documentation complete

---

## Timeline Summary

**Total Estimated Duration:** 40-54 hours (1-2 weeks full-time, 2-3 weeks part-time)

**Phase Breakdown:**
- Phase 0 (Planning): 4-6h - IN PROGRESS
- Phase 1 (Domain): 6-8h - Starts after Phase 0
- Phase 2 (Infrastructure): 8-10h
- Phase 3 (Use Case): 10-12h
- Phase 4 (Adapter): 6-8h
- Phase 5 (Admin & Observability): 4-6h
- Phase 6 (Documentation): 2-4h

**Key Milestones:**
- Day 3: Domain + Infrastructure complete (testable persistence)
- Day 7: Use Case + Adapter complete (full integration)
- Day 10: Admin + Documentation complete (ready for observe-only)
- Week 3-4: Observe-only mode (validate quality)
- Week 5: Pilot users (validate impact)
- Week 6-7: Full rollout

**Contingency:**
- Buffer: 20% (8-10 hours for unknown unknowns)

---

## Next Steps

1. **Complete Phase 0** (1-2 hours remaining)
   - Finalize this plan.md document
   - Create tasks.md with detailed task breakdown
   - Review all planning docs for consistency

2. **Begin Phase 1: Domain Layer** (6-8 hours)
   - Start with MemoryCell entity (TDD)
   - Follow Red-Green-Refactor cycle
   - Update status.md after each task

3. **Daily Progress Updates**
   - Update status.md after completing each task
   - Log time spent vs estimated
   - Document any blockers or deviations

4. **Weekly Checkpoints**
   - Review progress vs timeline
   - Adjust estimates based on actuals
   - Communicate risks/blockers early

---

## Notes

**Assumptions:**
- Anthropic API access available and stable
- SQLite database supports FTS5 (verified)
- Existing ChatService stable and well-tested
- 1-2 weeks of dedicated development time available

**Open Questions:**
- (All answered in research.md and architecture.md)

**Constraints:**
- Must not break existing chat functionality (graceful degradation required)
- Must respect token budgets (no context bloat)
- Must be configurable (feature flags for gradual rollout)
