# Self-Organizing Memory v2 - Specification

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Draft
**Source PRD:** `Feature-Self-Organizing-Memory-PRD.md`

---

## Executive Summary

Transform Nuimanbot's memory capabilities from simple chat history compression into a structured, queryable, long-term knowledge system. Inspired by research on self-organizing agent memory systems, this feature introduces **memory cells** (typed knowledge units with salience scoring), **scenes** (topic-organized buckets with stable summaries), and **FTS-based retrieval** to provide high-precision, long-horizon recall during agent conversations.

**Key Deliverables:**
- Structured memory cell extraction from conversations (facts, decisions, tasks, preferences, plans, risks)
- Scene-based topic organization with rolling summaries
- SQLite FTS5-powered retrieval system
- Context window integration for memory recall at inference time
- Admin/inspection API for memory management

**Timeline:** 40-60 hours (2-3 weeks full-time)

---

## Problem Statement

### Current State

Nuimanbot currently has three memory-like capabilities:
1. **Conversation history memory**: Messages persisted and used to build context windows (loads recent messages up to token limits)
2. **Conversation summarization**: LLM-powered compression when context grows large (exists but not fully wired into runtime flows)
3. **Skill memory**: Persistent key/value state stored in SQLite (`skill_memory` table with skill/user/global/session scopes)

While these provide basic memory functionality, they lack semantic organization and queryable knowledge structure.

### Pain Points

- **Buried knowledge**: Relevant decisions, facts, and preferences are trapped in message logs with no semantic indexing
- **Lossy summarization**: Current summarization compresses but doesn't become typed, queryable knowledge
- **Poor long-horizon recall**: Agent lacks stable, topic-organized memory that can be recalled reliably across sessions
- **Limited precision**: No salience scoring or relevance-based retrieval leads to information overload or missed context
- **Not queryable**: Skill memory is KV-only (no FTS, no salience, no scene/topic grouping)

### Desired State

After implementation:
- Conversations automatically extract structured knowledge units (memory cells)
- Memory organized by topics (scenes) with stable summaries
- High-precision retrieval based on FTS matching + salience scoring
- Agent context windows enhanced with relevant long-term memory
- Separation between "memory curation" and "reasoning" for maintainability
- Inspectable, editable memory via admin interfaces

---

## Goals and Non-Goals

### Goals

1. **Provide long-horizon recall** of key facts/decisions/tasks/preferences/risks across conversations
2. **Keep recall high-precision** by using salience scores and relevance matching (avoid prompt pollution)
3. **Separate memory curation from reasoning** for better maintainability and safety
4. **Work with current stack** (SQLite, Clean Architecture, existing message infrastructure)
5. **Enable inspection and correction** via admin APIs and CLI commands
6. **Integrate seamlessly** into existing context window assembly without breaking changes

### Non-Goals (Initial Release)

- ❌ Full vector DB / embedding-based retrieval (can be Phase 2)
- ❌ Fully autonomous background processing (can be Phase 2/3)
- ❌ Perfect topic modeling with ML/clustering (start with heuristic + LLM classification)
- ❌ Cross-user memory sharing in initial release (per-user isolation by default)
- ❌ Real-time streaming memory updates (batch processing acceptable for v1)

---

## User Requirements

### Primary Users

**Agent/LLM** (primary consumer):
- Needs relevant memory injected into context for better responses
- Benefits from stable scene summaries and high-salience facts
- Requires token-budget-aware memory injection

**Admin/Developer** (secondary consumer):
- Needs to inspect memory state
- Wants to correct/delete incorrect memories
- Requires audit trails for memory modifications

### Functional Requirements

#### FR-001: Extract Memory Cells from Interactions

**Priority:** P0 (Critical)

**Description:**
After each completed interaction (user message + assistant response + optional tool outputs), run a **Memory Curator** service that:
- Analyzes the interaction using an LLM
- Extracts structured memory cells as JSON
- Validates and persists cells to database
- Handles invalid JSON with retry/repair logic

**Acceptance Criteria:**
- [ ] Memory cells extracted with type, salience, scene, content
- [ ] Extraction runs automatically on message completion
- [ ] Invalid JSON handled gracefully (retry once, skip if still invalid)
- [ ] Configurable: `memory.self_organizing.enabled` feature flag
- [ ] Curator failures logged with metrics

**Example:**
```json
{
  "cells": [
    {
      "scene": "project-setup",
      "cell_type": "decision",
      "salience": 0.85,
      "content": "User decided to use SQLite FTS5 for memory retrieval instead of vector DB",
      "source": ["msg-abc123", "msg-def456"]
    },
    {
      "scene": "user-preferences",
      "cell_type": "preference",
      "salience": 0.92,
      "content": "User prefers TDD workflow with explicit refactoring phase",
      "source": ["msg-ghi789"]
    }
  ]
}
```

---

#### FR-002: Consolidate Scene Summaries

**Priority:** P0 (Critical)

**Description:**
After inserting memory cells, update scene summaries for any touched scenes. Summaries should be:
- Stable and reusable across multiple interactions
- Token-budget constrained (e.g., 200-500 tokens per scene)
- Generated by LLM using all cells in the scene
- Updated incrementally (not regenerated from scratch each time)

**Acceptance Criteria:**
- [ ] Scene summary created/updated when new cells added to scene
- [ ] Summary respects token budget (configurable max)
- [ ] Summary uses cheaper model (e.g., claude-3-haiku)
- [ ] Summary stable across updates (incremental, not regenerated)
- [ ] Ephemeral phrasing avoided (no "recently" or "this week")

**Metrics:**
- Summary generation time < 5 seconds
- Summary token count within configured limit

---

#### FR-003: Retrieve Relevant Memory for Prompts

**Priority:** P0 (Critical)

**Description:**
When building a context window for a new prompt, enhance it with:
- **Top-k matching cells** via FTS search on prompt keywords
- **High-salience fallback** if FTS yields too few results
- **Scene summaries** for all scenes referenced by retrieved cells
- **Token budget enforcement** (e.g., 500-1500 tokens total for injected memory)

**Acceptance Criteria:**
- [ ] FTS search returns relevant cells based on prompt keywords
- [ ] Salience-based fallback provides high-value cells when FTS is sparse
- [ ] Scene summaries included for context
- [ ] Total injected memory respects token budget
- [ ] Memory injection clearly separated from chat history in prompt

**Metrics:**
- FTS recall hit rate (FTS matches vs fallback usage)
- Average injected token count
- Retrieval latency < 100ms

**Example Injection:**
```
### Relevant Long-Term Memory (Curated)

**Scene: project-setup**
Summary: User is building a Golang CLI app using Clean Architecture...

**High-Salience Facts:**
- [decision, salience=0.85] User decided to use SQLite FTS5 for memory retrieval
- [preference, salience=0.92] User prefers TDD workflow with explicit refactoring

**Scene: user-preferences**
Summary: User's development preferences include...
```

---

#### FR-004: Admin/Inspection API

**Priority:** P1 (High)

**Description:**
Provide REST endpoints (or CLI commands) for memory management:
- **List scenes**: Get all scenes with cell counts
- **Get scene summary**: Retrieve summary for a specific scene
- **Search cells**: Query cells by scene, type, salience, or keyword
- **Delete cell/scene**: Remove individual cells or entire scenes
- **Audit trail**: Log all modifications with user, timestamp, action

**Acceptance Criteria:**
- [ ] `GET /api/memory/scenes` - list all scenes
- [ ] `GET /api/memory/scenes/{scene}` - get scene details + summary
- [ ] `GET /api/memory/cells?scene=X&type=Y` - search cells
- [ ] `DELETE /api/memory/cells/{id}` - delete cell (with audit log)
- [ ] `DELETE /api/memory/scenes/{scene}` - delete scene (with audit log)
- [ ] All modifications audited (user, timestamp, action)
- [ ] RBAC enforced (admin-only access)

**Example:**
```bash
# CLI usage
./bin/nuimanbot memory list-scenes
./bin/nuimanbot memory get-scene project-setup
./bin/nuimanbot memory search --scene=user-preferences --type=preference
./bin/nuimanbot memory delete-cell cell-abc123
```

---

#### FR-005: Safety & Access Controls

**Priority:** P0 (Critical)

**Description:**
Ensure memory system respects existing RBAC model and provides per-user isolation:
- Memory cells scoped to user by default
- Optional global scope only if explicitly configured
- All memory operations respect user permissions
- No cross-user memory leakage
- Admin-only access to delete/modify operations

**Acceptance Criteria:**
- [ ] Memory cells tagged with `user_id` (or `conversation_id`)
- [ ] Retrieval filters by current user's cells only
- [ ] Admin role required for delete operations
- [ ] Audit log records all access (read/write/delete)
- [ ] Configuration option: `memory.global_scope.enabled` (default: false)

**Metrics:**
- Zero cross-user memory leakage in testing
- Audit log coverage: 100% of write/delete operations

---

### Non-Functional Requirements

#### NFR-001: Performance

**Category:** Performance
**Priority:** P1 (High)

**Description:**
Memory operations must not significantly degrade chat responsiveness

**Metrics:**
- Memory cell extraction: < 5 seconds (async, non-blocking)
- Memory retrieval: < 100ms (synchronous, in critical path)
- Scene consolidation: < 5 seconds (async, non-blocking)
- FTS indexing latency: < 1 second

**Acceptance Criteria:**
- [ ] Retrieval adds < 100ms to total response time
- [ ] Curator runs async (doesn't block user response)
- [ ] Token budget enforced to prevent context bloat

---

#### NFR-002: Reliability

**Category:** Reliability
**Priority:** P0 (Critical)

**Description:**
Memory system failures must not break chat functionality

**Metrics:**
- Memory curator failure rate: < 5%
- Graceful degradation when memory unavailable
- Zero chat breakage due to memory errors

**Acceptance Criteria:**
- [ ] Chat continues if memory curator fails
- [ ] Retrieval failures degrade gracefully (continue without memory)
- [ ] Invalid JSON in extraction logged and skipped (no crash)
- [ ] Circuit breaker pattern for repeated failures

---

#### NFR-003: Observability

**Category:** Observability
**Priority:** P1 (High)

**Description:**
Comprehensive metrics and tracing for debugging and optimization

**Metrics to Track:**
- Memory cells created per conversation/day
- Recall hit rate (FTS matches vs salience fallback)
- Injected token count (avg, p50, p95, p99)
- Curator failures (JSON parse errors, LLM failures)
- Scene consolidation frequency
- FTS query latency

**Acceptance Criteria:**
- [ ] Tracing spans for curate/retrieve/consolidate operations
- [ ] Metrics dashboard showing key indicators
- [ ] Alerting on high curator failure rate (>10%)
- [ ] Logging includes conversation_id for correlation

---

## System Architecture

### Affected Layers

- ✅ **Domain Layer**: New entities (`MemoryCell`, `MemoryScene`), repository interfaces
- ✅ **Use Case Layer**: New services (`MemoryCuratorService`, `MemoryRecallService`)
- ✅ **Infrastructure Layer**: SQLite implementation with FTS5, migration scripts
- ✅ **Adapter Layer**: Integration into `ChatService.ProcessMessage()` and context window builder

### New Components

#### Domain Layer (`internal/domain/`)

**MemoryCell Entity:**
- Fields: `ID`, `ConversationID`, `Scene`, `CellType`, `Salience`, `Content`, `Source`, `CreatedAt`, `UpdatedAt`, `ExpiresAt`
- Methods: `Validate()`, `IsExpired()`, `String()`

**MemoryScene Entity:**
- Fields: `Scene` (PK), `Summary`, `TokenCount`, `UpdatedAt`
- Methods: `Validate()`, `String()`

**MemoryCellRepository Interface:**
- `Create(ctx, cell)`, `Get(ctx, id)`, `List(ctx, filter)`, `Delete(ctx, id)`
- `SearchFTS(ctx, query, limit)`, `GetByScene(ctx, scene, limit)`

**MemorySceneRepository Interface:**
- `Upsert(ctx, scene)`, `Get(ctx, scene)`, `List(ctx)`, `Delete(ctx, scene)`

#### Use Case Layer (`internal/usecase/memory/`)

**MemoryCuratorService:**
- `ExtractCells(ctx, interaction)`: Extract cells from user+assistant messages
- `ConsolidateScene(ctx, scene)`: Update scene summary with all cells
- Orchestrates LLM calls for extraction and summarization

**MemoryRecallService:**
- `Recall(ctx, prompt, tokenBudget)`: Retrieve relevant cells and summaries
- Implements FTS search + salience fallback + scene retrieval
- Returns formatted memory injection bundle

#### Infrastructure Layer (`internal/infrastructure/memory/`)

**SQLiteMemoryCellRepository:**
- Implements `MemoryCellRepository`
- Uses `memory_cells` table + `memory_cells_fts` virtual table (FTS5)
- Handles FTS indexing and search

**SQLiteMemorySceneRepository:**
- Implements `MemorySceneRepository`
- Uses `memory_scenes` table

#### Integration Points

**ChatService Changes:**
- `ProcessMessage()`: After sending response, call `MemoryCuratorService.ExtractCells()` async
- `BuildContextWindow()`: Call `MemoryRecallService.Recall()` and inject results before message history

---

## Scope of Changes

### Files to Create

**Domain:**
- `internal/domain/memory_cell.go` - MemoryCell entity and CellType enum
- `internal/domain/memory_scene.go` - MemoryScene entity
- `internal/domain/memory_cell_repository.go` - Repository interface
- `internal/domain/memory_scene_repository.go` - Repository interface

**Use Case:**
- `internal/usecase/memory/curator_service.go` - Extraction and consolidation
- `internal/usecase/memory/recall_service.go` - Retrieval and ranking
- `internal/usecase/memory/types.go` - Use case DTOs

**Infrastructure:**
- `internal/infrastructure/memory/sqlite_cell_repository.go` - SQLite implementation
- `internal/infrastructure/memory/sqlite_scene_repository.go` - SQLite implementation
- `internal/infrastructure/memory/migrations/001_memory_tables.sql` - DB schema

**Adapter:**
- `internal/adapter/cli/memory.go` - CLI commands for memory inspection
- `internal/adapter/rest/memory_handler.go` - REST endpoints (if web UI needed)

**Tests:**
- All `*_test.go` files for above components

### Files to Modify

- `internal/usecase/chat/service.go` - Wire in memory curator and recall
- `internal/usecase/chat/context_window.go` - Inject memory into context
- `cmd/nuimanbot/main.go` - Dependency injection for new services
- `internal/config/config.go` - Add memory configuration section
- `documentation/technical-details.md` - Document memory architecture

---

## Breaking Changes

### API Changes

None. This is a new feature with no breaking changes to existing APIs.

### Configuration Changes

**New config section:**
```yaml
memory:
  self_organizing:
    enabled: false  # Feature flag (start disabled)
    curator:
      model: "claude-3-haiku-20240307"  # Cheaper model for extraction
      timeout_seconds: 10
    recall:
      token_budget: 1000  # Max tokens for injected memory
      fts_limit: 10       # Max FTS results
      salience_threshold: 0.7  # Min salience for fallback
    scenes:
      max_summary_tokens: 500
    cleanup:
      enabled: true
      ttl_days: 90  # Auto-expire cells after 90 days
```

**Migration Path:**
- Default: `enabled: false` (opt-in)
- Users enable via config after testing in observe-only mode

### Database Schema Changes

**New tables:**
- `memory_cells`
- `memory_scenes`
- `memory_cells_fts` (FTS5 virtual table)

**Migration Strategy:**
- Schema migration script: `001_memory_tables.sql`
- Applied automatically on startup if tables don't exist
- Backward compatible (no changes to existing tables)

---

## Success Criteria

### Acceptance Criteria

#### Technical Criteria
- [ ] Memory cells extracted from conversations with 95%+ success rate
- [ ] FTS retrieval returns relevant results (manual testing + metrics)
- [ ] Scene summaries generated and updated correctly
- [ ] Token budgets enforced (no prompt overflow)
- [ ] All quality gates pass (tests, lint, build, coverage >90%)

#### Functional Criteria
- [ ] Agent responses demonstrate improved long-horizon recall (qualitative testing)
- [ ] Admin can inspect, search, and delete memory via CLI
- [ ] Per-user memory isolation verified (no cross-user leakage)
- [ ] Curator failures don't break chat (graceful degradation)

#### Performance Criteria
- [ ] Memory retrieval adds < 100ms to response time
- [ ] Memory extraction completes < 5 seconds (async)
- [ ] FTS queries execute < 50ms

### Quality Gates

- [ ] All tests pass (`go test ./...`)
- [ ] Code coverage >90% overall
- [ ] Documentation complete (spec, architecture, data dictionary, tasks)
- [ ] Performance benchmarks met
- [ ] Security review complete (RBAC, data isolation)
- [ ] Observability in place (metrics, traces, logs)

### User Validation

#### Phase 1: Observe-Only Mode (Week 1-2)
- [ ] Enable curator to extract and store cells
- [ ] Do NOT inject memory into prompts
- [ ] Verify cells are created correctly (manual inspection)
- [ ] Monitor curator failure rate

#### Phase 2: Pilot Users (Week 3)
- [ ] Enable injection for pilot users (e.g., developers)
- [ ] Gather feedback on response quality
- [ ] Measure retrieval hit rate and token usage
- [ ] Fix any issues found

#### Phase 3: Full Rollout (Week 4+)
- [ ] Enable for all users
- [ ] Monitor metrics (error rate, performance, adoption)
- [ ] Iterate based on feedback

---

## Risks and Mitigation

### Risk 1: Memory Curator LLM Failures

**Likelihood:** Medium
**Impact:** Medium (degrades memory quality but doesn't break chat)

**Mitigation:**
- Retry once on JSON parse failure
- Skip extraction on repeated failures (log and alert)
- Use cheaper, faster model (haiku) to reduce failure rate
- Circuit breaker pattern: disable curator after N consecutive failures
- Fallback: continue chat without memory extraction

---

### Risk 2: FTS Query Performance Degradation

**Likelihood:** Low
**Impact:** High (adds latency to every message)

**Mitigation:**
- FTS5 is highly optimized for full-text search
- Limit FTS result size (configurable, default 10)
- Index only necessary fields (content, scene, cell_type)
- Monitor FTS query latency with p95/p99 metrics
- Add database indexes on frequently queried columns
- Fallback: reduce FTS limit or disable memory on timeout

---

### Risk 3: Memory Bloat (Too Many Cells)

**Likelihood:** Medium
**Impact:** Medium (storage + retrieval performance degradation)

**Mitigation:**
- Auto-expire cells after configurable TTL (default 90 days)
- Salience-based pruning: periodically delete low-salience cells
- Admin tools to bulk-delete scenes or cells
- Monitoring: track total cell count per user
- Alerting: warn when user exceeds threshold (e.g., 10K cells)

---

### Risk 4: Poor Topic/Scene Classification

**Likelihood:** High
**Impact:** Low (reduces retrieval precision but doesn't break system)

**Mitigation:**
- Start with heuristic + LLM-generated scenes (good enough for v1)
- Allow manual scene editing via admin API
- Future enhancement: ML-based topic modeling (Phase 2)
- Measure recall hit rate to detect poor classification
- Iterate on extraction prompts based on feedback

---

## Timeline and Milestones

### Phase 0: Planning (4-6 hours) - COMPLETE
**Deliverables:**
- ✅ PRD analysis and comparison with existing system
- ✅ Feature specification (this document)
- [ ] Data dictionary (types, schemas)
- [ ] Architecture document (component design)
- [ ] Implementation plan (task breakdown)

### Phase 1: Domain Layer (6-8 hours)
**Deliverables:**
- Memory cell and scene entities
- Repository interfaces
- Validation logic
- Unit tests (coverage >90%)

### Phase 2: Infrastructure Layer (8-10 hours)
**Deliverables:**
- SQLite repository implementations
- FTS5 schema and migrations
- Repository integration tests
- Performance benchmarks

### Phase 3: Use Case Layer (10-12 hours)
**Deliverables:**
- Memory curator service (extraction + consolidation)
- Memory recall service (retrieval + ranking)
- Service unit tests
- Integration tests with LLM

### Phase 4: Adapter Layer (6-8 hours)
**Deliverables:**
- ChatService integration (curator + recall)
- Context window enhancement
- CLI memory commands
- End-to-end tests

### Phase 5: Admin & Observability (4-6 hours)
**Deliverables:**
- Admin API endpoints (or CLI commands)
- Metrics and tracing instrumentation
- Logging and alerting setup
- Admin documentation

### Phase 6: Documentation & Polish (2-4 hours)
**Deliverables:**
- Update product documentation
- Memory user guide
- Architecture diagrams
- Migration guide (config, feature flags)

**Total Estimated Duration:** 40-54 hours (1-2 weeks full-time, 2-3 weeks part-time)

---

## References

- **Source PRD:** `Feature-Self-Organizing-Memory-PRD.md`
- **Related Research:** [MarkTechPost Article - Self-Organizing Agent Memory System](https://www.marktechpost.com/2026/02/14/how-to-build-a-self-organizing-agent-memory-system-for-long-term-ai-reasoning/)
- **Existing Memory Implementation:** `internal/domain/memory.go`, `internal/infrastructure/memory/storage.go`
- **Current Context Window Logic:** `internal/usecase/chat/context_window.go`
- **Existing Summarization:** `internal/usecase/chat/summarization.go`

---

## Appendix: Addendum Tasks (Complete Existing Memory Hooks)

Before implementing the new self-organizing memory subsystem, we should **complete the wiring of existing memory primitives** that are implemented but not fully integrated:

### A1: Wire `BuildContextWindow()` into `ChatService.ProcessMessage()`
- Currently: `ProcessMessage()` calls `GetRecentMessages(..., 4096)` directly
- Change: Use `BuildContextWindow(ctx, conversationID, provider, maxTokens)`
- Ensures token budgets respect provider limits

### A2: Implement Runtime Summarization Trigger
- Currently: `SummarizeConversation()` exists but isn't called in production
- Change: Add config-driven trigger when conversation exceeds token threshold
- Store summaries as system messages or in dedicated table

### A3: Finish Token Accounting
- Currently: Some `StoredMessage.TokenCount` fields are TODO
- Change: Implement token counting heuristic or use provider-specific tokenizers
- Ensures context window trimming is accurate

### A4: Schedule Skill Memory Cleanup
- Currently: `SQLiteMemoryStorage.Cleanup()` exists but no periodic trigger
- Change: Add background cleanup loop (configurable interval)
- Removes expired memory entries proactively

### A5: Formalize Memory Management Pipeline
- Document the staged pipeline in `ProcessMessage()`:
  1. Validate input
  2. (Optional) Summarize/compact conversation
  3. Build context window
  4. LLM/tool loop
  5. Persist messages
  6. (Future) Curate memory cells
  7. Emit telemetry

### A6: Update Documentation
- Update `support_docs/memory-guide.md` to distinguish:
  - Conversation history
  - Conversation summaries (auto)
  - Skill KV memory
  - (Future) Self-organizing memory cells/scenes

**Estimated Effort:** 8-12 hours (can be done in parallel with Phase 1-2 of new memory system)

**Decision:** Recommend completing A1-A6 first to have a solid foundation before adding self-organizing memory. This ensures existing primitives work correctly and reduces moving parts.
