# Self-Organizing Memory v2 - System Architecture

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Draft
**Last Updated:** 2026-02-15

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

---

## Architecture Overview

**High-Level Summary:**
The Self-Organizing Memory v2 system extends Nuimanbot with structured, queryable long-term memory using a dual-process architecture: a **Memory Curator** extracts and consolidates knowledge asynchronously, while a **Memory Recall** service injects relevant context synchronously during chat interactions. Storage uses SQLite with FTS5 for fast full-text search, and salience scoring enables precision recall.

**Architectural Style:** Clean Architecture + Event-Driven (async curator)

**Key Principles:**
- **Dependency Inversion**: Outer layers depend on inner layers (infra implements domain interfaces)
- **Separation of Concerns**: Curator (write) and recall (read) are independent services
- **Single Responsibility**: Each component has one clear purpose
- **Asynchronous Processing**: Memory curation doesn't block chat responses
- **Graceful Degradation**: Memory failures don't break chat functionality

**Architecture Diagram:**
```
┌──────────────────────────────────────────────────────────────────┐
│                     Infrastructure Layer                          │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ SQLiteMemory    │  │ SQLiteScene     │  │ LLM Provider    │  │
│  │ CellRepository  │  │ Repository      │  │ (Anthropic)     │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────┬──────────────────┬───────────────────┬────────────┘
              │ implements       │ implements         │ uses
┌─────────────▼──────────────────▼───────────────────▼────────────┐
│                        Adapter Layer                              │
│  ┌──────────────────┐            ┌──────────────────┐            │
│  │ ChatService      │            │ MemoryCLI        │            │
│  │ (integration)    │            │ Adapter          │            │
│  └──────────────────┘            └──────────────────┘            │
└─────────────┬────────────────────────────────────────────────────┘
              │ uses
┌─────────────▼──────────────────────────────────────────────────┐
│                      Use Case Layer                             │
│  ┌──────────────────┐            ┌──────────────────┐           │
│  │ MemoryCurator    │            │ MemoryRecall     │           │
│  │ Service          │            │ Service          │           │
│  │ (extract + cons) │            │ (retrieve + rank)│           │
│  └──────────────────┘            └──────────────────┘           │
└─────────────┬────────────────────────────────────────────────────┘
              │ uses
┌─────────────▼──────────────────────────────────────────────────┐
│                       Domain Layer                              │
│  ┌──────────┐  ┌──────────┐  ┌─────────────┐  ┌──────────────┐│
│  │MemoryCell│  │MemoryScene│ │CellRepo     │  │SceneRepo     ││
│  │ (entity) │  │ (entity)  │  │(interface)  │  │(interface)   ││
│  └──────────┘  └──────────┘  └─────────────┘  └──────────────┘│
└──────────────────────────────────────────────────────────────────┘
```

---

## System Context

**External Systems:**
```
┌──────────────┐
│   User/CLI   │
└──────┬───────┘
       │ sends message
       ▼
┌────────────────────────────────────────────────────────┐
│                   NuimanBot System                      │
│                                                         │
│  ┌────────────────────────────────────────────────┐   │
│  │         Chat Service (existing)                 │   │
│  │  ┌──────────────┐        ┌──────────────┐      │   │
│  │  │ProcessMessage│◄──────►│BuildContext  │      │   │
│  │  │              │        │Window        │      │   │
│  │  └──────┬───────┘        └──────┬───────┘      │   │
│  │         │ triggers              │ calls        │   │
│  │         ▼                       ▼              │   │
│  │  ┌──────────────┐        ┌──────────────┐      │   │
│  │  │MemoryCurator │        │MemoryRecall  │      │   │
│  │  │Service       │        │Service       │      │   │
│  │  │(async)       │        │(sync)        │      │   │
│  │  └──────┬───────┘        └──────┬───────┘      │   │
│  └─────────┼─────────────────────┼────────────────┘   │
│            │                      │                     │
│            ▼                      ▼                     │
│  ┌───────────────────────────────────────────────┐    │
│  │  Memory Storage (SQLite + FTS5)               │    │
│  │  - memory_cells (main table)                  │    │
│  │  - memory_cells_fts (virtual FTS table)       │    │
│  │  - memory_scenes (summaries)                  │    │
│  └───────────────────────────────────────────────┘    │
└───────────────────────┬───────────────────────────────┘
                        │ calls
                        ▼
              ┌──────────────────┐
              │ Anthropic API    │
              │ (Claude LLM)     │
              └──────────────────┘
```

**System Boundaries:**
- **Inputs**: User messages → ChatService → MemoryCurator (async) + MemoryRecall (sync)
- **Outputs**: Structured memory cells → SQLite; Scene summaries → SQLite; Enhanced context → LLM
- **External Dependencies**: Anthropic API (for extraction/consolidation), SQLite (persistence)

**Integration Points:**
| System | Type | Protocol | Purpose |
|--------|------|----------|---------|
| SQLite | Database | SQL | Memory cell and scene persistence |
| SQLite FTS5 | Search Engine | SQL (virtual table) | Full-text search on cell content |
| Anthropic API | LLM Service | REST/HTTP (JSON) | Memory cell extraction and scene summarization |
| ChatService | Internal | Go function calls | Triggers curator and consumes recall |

---

## Component Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                   Self-Organizing Memory v2                      │
│                                                                  │
│  ┌──────────────────────┐         ┌──────────────────────┐     │
│  │ MemoryCurator        │         │ MemoryRecall         │     │
│  │ Service              │         │ Service              │     │
│  │                      │         │                      │     │
│  │ - ExtractCells()     │         │ - Recall()           │     │
│  │ - ConsolidateScene() │         │ - FormatInjection()  │     │
│  └──────────┬───────────┘         └──────────┬───────────┘     │
│             │                                 │                  │
│             │                                 │                  │
│             ▼                                 ▼                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │           MemoryCellRepository (interface)              │    │
│  │  - Create(), Get(), List(), Delete()                    │    │
│  │  - SearchFTS(), GetByScene(), GetHighSalience()         │    │
│  └────────────────────────┬───────────────────────────────┘    │
│                            │                                     │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────┐    │
│  │      SQLiteMemoryCellRepository (implementation)        │    │
│  │  - Uses memory_cells table + memory_cells_fts FTS5     │    │
│  │  - Implements FTS search with BM25 ranking             │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │         MemorySceneRepository (interface)               │    │
│  │  - Upsert(), Get(), List(), Delete()                    │    │
│  └────────────────────────┬───────────────────────────────┘    │
│                            │                                     │
│                            ▼                                     │
│  ┌────────────────────────────────────────────────────────┐    │
│  │    SQLiteMemorySceneRepository (implementation)         │    │
│  │  - Uses memory_scenes table                             │    │
│  └────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### Component Descriptions

#### Component A: MemoryCuratorService

**Responsibility:**
Asynchronously extract structured memory cells from conversation interactions and consolidate scene summaries

**Dependencies:**
- MemoryCellRepository (via interface)
- MemorySceneRepository (via interface)
- LLMProvider (for extraction prompts)
- TokenCounter (for summary budgets)

**Provides:**
- `ExtractCells(ctx, ExtractionInput)`: Parse interaction, call LLM, persist cells
- `ConsolidateScene(ctx, scene)`: Update scene summary with all cells

**Lifecycle:**
- Created during application startup (dependency injection)
- Lifespan: Singleton (one instance per app)

**Concurrency:**
- Thread-safe: Yes (each extraction runs in goroutine)
- Synchronization: Repository handles DB locking

---

#### Component B: MemoryRecallService

**Responsibility:**
Synchronously retrieve relevant memory cells and scene summaries for prompt enhancement

**Dependencies:**
- MemoryCellRepository (via interface)
- MemorySceneRepository (via interface)
- TokenCounter (for budget enforcement)

**Provides:**
- `Recall(ctx, RecallInput)`: FTS search + salience fallback + scene retrieval
- `FormatInjection(cells, scenes)`: Format memory as prompt-ready text

**Lifecycle:**
- Created during application startup (dependency injection)
- Lifespan: Singleton

**Concurrency:**
- Thread-safe: Yes (read-only operations, repository handles locking)
- Synchronization: None needed (stateless)

---

#### Component C: SQLiteMemoryCellRepository

**Responsibility:**
Persist and retrieve memory cells using SQLite with FTS5 for full-text search

**Dependencies:**
- SQLite database connection
- FTS5 virtual table (memory_cells_fts)

**Provides:**
- CRUD operations for memory cells
- FTS search with BM25 ranking
- Salience-based retrieval
- Expiration cleanup

**Lifecycle:**
- Created during application startup
- Lifespan: Singleton (shares DB connection pool)

**Concurrency:**
- Thread-safe: Yes (SQLite handles locking via WAL mode)
- Synchronization: SQLite internal locking

---

#### Component D: SQLiteMemorySceneRepository

**Responsibility:**
Persist and retrieve scene summaries using SQLite

**Dependencies:**
- SQLite database connection

**Provides:**
- Upsert, get, list, delete operations for scenes

**Lifecycle:**
- Created during application startup
- Lifespan: Singleton

**Concurrency:**
- Thread-safe: Yes (SQLite WAL mode)

---

## Layer Responsibilities

### Domain Layer

**Location:** `internal/domain/`

**Responsibility:**
- Define core memory entities (MemoryCell, MemoryScene)
- Specify business rules (validation, salience ranges)
- Define repository interfaces (no implementation)

**Contains:**
- `MemoryCell` entity with validation
- `MemoryScene` entity with validation
- `CellType` enum (fact, decision, task, preference, plan, risk)
- `MemoryCellRepository` interface
- `MemorySceneRepository` interface

**Dependencies:** None (pure domain logic, only stdlib)

**Example:**
```go
// Domain entity
type MemoryCell struct {
    ID             string
    ConversationID string
    Scene          string
    CellType       CellType
    Salience       float64
    Content        string
    Source         string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    ExpiresAt      *time.Time
}

// Domain interface
type MemoryCellRepository interface {
    Create(ctx context.Context, cell *MemoryCell) error
    SearchFTS(ctx context.Context, query string, limit int) ([]*MemoryCell, error)
}
```

---

### Use Case Layer

**Location:** `internal/usecase/memory/`

**Responsibility:**
- Orchestrate memory extraction and consolidation (MemoryCuratorService)
- Orchestrate memory retrieval and ranking (MemoryRecallService)
- Implement application-specific workflows (FTS + salience fallback)

**Contains:**
- `MemoryCuratorService` (extraction + consolidation)
- `MemoryRecallService` (retrieval + ranking)
- `ExtractionInput`, `ExtractionOutput`, `RecallInput`, `RecallOutput` (DTOs)

**Dependencies:**
- Domain layer (entities, interfaces)

**Example:**
```go
// Use case orchestrator
type MemoryCuratorService struct {
    cellRepo     domain.MemoryCellRepository
    sceneRepo    domain.MemorySceneRepository
    llmProvider  LLMProvider
}

func (s *MemoryCuratorService) ExtractCells(ctx context.Context, input ExtractionInput) (ExtractionOutput, error) {
    // 1. Call LLM with extraction prompt
    // 2. Parse JSON response to memory cells
    // 3. Persist cells via repository
    // 4. Trigger scene consolidation
}
```

---

### Infrastructure Layer

**Location:** `internal/infrastructure/memory/`

**Responsibility:**
- Implement domain interfaces (repositories)
- Handle external system interactions (SQLite, FTS5)
- Provide technical capabilities (SQL queries, transactions)

**Contains:**
- `SQLiteMemoryCellRepository` (implements MemoryCellRepository)
- `SQLiteMemorySceneRepository` (implements MemorySceneRepository)
- Migration scripts (`migrations/001_memory_tables.sql`)

**Dependencies:**
- Domain layer (interfaces to implement)
- External libraries (`database/sql`, `mattn/go-sqlite3`)

**Example:**
```go
// Infrastructure implementation
type SQLiteMemoryCellRepository struct {
    db *sql.DB
}

func (r *SQLiteMemoryCellRepository) SearchFTS(ctx context.Context, query string, limit int) ([]*domain.MemoryCell, error) {
    // Execute FTS query against virtual table
    rows, err := r.db.QueryContext(ctx, `
        SELECT mc.* FROM memory_cells mc
        JOIN memory_cells_fts fts ON mc.rowid = fts.rowid
        WHERE memory_cells_fts MATCH ?
        ORDER BY rank
        LIMIT ?
    `, query, limit)
    // Parse rows into domain entities
}
```

---

### Adapter Layer

**Location:** `internal/adapter/cli/`, `internal/usecase/chat/`

**Responsibility:**
- Integrate memory services into ChatService (ProcessMessage, BuildContextWindow)
- Provide CLI commands for memory inspection
- Handle request/response transformations

**Contains:**
- `ChatService` modifications (wire curator + recall)
- `MemoryCLIAdapter` (CLI commands for memory management)

**Dependencies:**
- Use case layer (MemoryCuratorService, MemoryRecallService)
- Infrastructure layer (for dependency injection)

**Example:**
```go
// Chat adapter integration
func (s *ChatService) ProcessMessage(ctx context.Context, msg Message) (Response, error) {
    // ... existing logic ...

    // Send response to user
    response := s.llmProvider.Complete(prompt)

    // ASYNC: Extract memory cells (don't block response)
    go func() {
        _ = s.memoryCurator.ExtractCells(context.Background(), ExtractionInput{
            ConversationID:   msg.ConversationID,
            UserMessage:      msg.Text,
            AssistantMessage: response.Text,
        })
    }()

    return response, nil
}
```

---

## Data Flow

### Request Flow: Memory Cell Extraction (Async)

**1. User sends message → Assistant responds:**
```
User → ChatService.ProcessMessage()
  ├─> Build context (with memory recall)
  ├─> Call LLM
  ├─> Send response to user
  └─> Trigger MemoryCuratorService.ExtractCells() [ASYNC]
```

**Step-by-Step:**
1. User sends message via CLI/Slack/etc.
2. ChatService receives message
3. ChatService calls BuildContextWindow (includes memory recall)
4. ChatService calls LLM with enhanced context
5. ChatService sends response to user
6. **ASYNC**: ChatService spawns goroutine to extract memory cells
7. MemoryCuratorService builds extraction prompt
8. MemoryCuratorService calls Anthropic API (LLM)
9. LLM returns JSON with memory cells
10. MemoryCuratorService parses JSON, validates cells
11. MemoryCuratorService persists cells via MemoryCellRepository
12. MemoryCuratorService calls ConsolidateScene for touched scenes
13. Scene summaries updated in MemorySceneRepository

**Example:**
```go
// ASYNC extraction triggered after response sent
go func() {
    cells, err := s.memoryCurator.ExtractCells(ctx, ExtractionInput{
        ConversationID:   conversationID,
        UserMessage:      userMsg,
        AssistantMessage: assistantMsg,
        MessageIDs:       []string{userMsgID, assistantMsgID},
    })
    if err != nil {
        log.Error("Memory extraction failed", "error", err)
        return // Don't crash chat on failure
    }
    log.Info("Extracted memory cells", "count", len(cells.Cells))
}()
```

---

### Request Flow: Memory Recall (Sync)

**1. User sends message → Build context with memory:**
```
User → ChatService.ProcessMessage()
  └─> BuildContextWindow()
      ├─> Load recent messages
      ├─> MemoryRecallService.Recall() [SYNC]
      │   ├─> SearchFTS(prompt keywords)
      │   ├─> GetHighSalience(fallback)
      │   └─> GetSceneSummaries(for matched scenes)
      ├─> Format memory injection
      └─> Return enhanced context
```

**Step-by-Step:**
1. User sends message
2. ChatService calls BuildContextWindow
3. BuildContextWindow calls MemoryRecallService.Recall with prompt
4. MemoryRecallService extracts keywords from prompt
5. MemoryRecallService calls MemoryCellRepository.SearchFTS(keywords)
6. If FTS results < threshold, call GetHighSalience(fallback)
7. MemoryRecallService retrieves scene summaries for matched cells
8. MemoryRecallService formats cells + summaries as prompt injection
9. BuildContextWindow inserts memory injection before message history
10. ChatService sends enhanced context to LLM
11. LLM generates response with long-term memory context

**Example:**
```go
// SYNC recall during context building
func (s *ChatService) BuildContextWindow(ctx context.Context, conversationID string) ([]Message, error) {
    // Load recent messages
    recentMessages := s.messageRepo.GetRecent(ctx, conversationID, 50)

    // Recall memory (SYNC, < 100ms)
    memory, err := s.memoryRecall.Recall(ctx, RecallInput{
        ConversationID: conversationID,
        Prompt:         recentMessages[len(recentMessages)-1].Text,
        TokenBudget:    1000,
    })
    if err != nil {
        log.Warn("Memory recall failed, continuing without memory", "error", err)
        memory = RecallOutput{} // Graceful degradation
    }

    // Build final context
    context := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "system", Content: memory.FormattedText}, // INJECT MEMORY
    }
    context = append(context, recentMessages...)

    return context, nil
}
```

---

### Error Flow

**Error Propagation:**
```
Anthropic API failure (rate limit)
  → MemoryCuratorService (retry logic)
    → If retry fails: Log error, emit metric, skip extraction
      → ChatService continues normally (graceful degradation)

FTS query timeout
  → SQLiteMemoryCellRepository (catch timeout)
    → Return empty results
      → MemoryRecallService (fallback to high-salience)
        → If fallback also fails: Return empty memory
          → BuildContextWindow continues without memory
```

**Error Handling Strategy:**
- **Infrastructure**: Wrap errors with context (`fmt.Errorf("FTS query failed: %w", err)`)
- **Use Case**: Categorize errors (user/system/external), retry transient failures
- **Adapter**: Graceful degradation (continue without memory), log + emit metrics

---

## Sequence Diagrams

### Sequence 1: Memory Cell Extraction (Primary Workflow)

**Scenario:** User sends message, assistant responds, memory cells extracted asynchronously

```
User    ChatService  LLM  MemoryCurator  CellRepo  SceneRepo
 |          |         |         |            |         |
 |─request─>|         |         |            |         |
 |          |─recall─>|         |            |         | [Sync: Recall memory]
 |          |<──mem──|         |            |         |
 |          |─prompt─>|         |            |         | [LLM call with memory]
 |          |<─resp──|         |            |         |
 |<response─|         |         |            |         | [User gets answer]
 |          |─────────┼────────>|            |         | [ASYNC: Extract cells]
 |          |         |         |─extract───>|         | [LLM call for extraction]
 |          |         |         |<─cells────|         |
 |          |         |         |─persist───>|         | [Save cells to DB]
 |          |         |         |<─ack──────|         |
 |          |         |         |─consolidate────────>| [Update scene summary]
 |          |         |         |<─ack───────────────|
```

**Steps:**
1. User sends message to ChatService
2. ChatService recalls memory (sync) via MemoryRecallService
3. ChatService calls LLM with enhanced context
4. ChatService returns response to user
5. **[ASYNC]** ChatService triggers MemoryCuratorService
6. MemoryCuratorService calls LLM with extraction prompt
7. LLM returns JSON with memory cells
8. MemoryCuratorService persists cells via CellRepo
9. MemoryCuratorService consolidates scene summaries via SceneRepo

---

### Sequence 2: Memory Recall with FTS + Salience Fallback

**Scenario:** BuildContextWindow requests memory, FTS returns few results, fallback to salience

```
BuildContext  MemoryRecall  CellRepo  SceneRepo
     |            |            |         |
     |─recall────>|            |         |
     |            |─searchFTS─>|         |
     |            |<─3 cells──|         | [FTS found 3 cells, threshold=10]
     |            |─salience──>|         | [Fallback: get high-salience cells]
     |            |<─7 cells──|         |
     |            |─getScenes─────────>| [Get summaries for scenes]
     |            |<─summaries────────|
     |            |─format────|         | [Format injection text]
     |<─memory───|            |         | [Return formatted memory]
```

**Steps:**
1. BuildContextWindow requests memory recall
2. MemoryRecallService searches FTS for keywords
3. FTS returns 3 cells (below threshold of 10)
4. MemoryRecallService falls back to high-salience query
5. CellRepo returns 7 more cells with salience >= 0.7
6. MemoryRecallService retrieves scene summaries
7. MemoryRecallService formats cells + summaries
8. BuildContextWindow receives formatted memory injection

---

## Integration Points

### Integration 1: SQLite Database

**Type:** SQLite
**Purpose:** Persistent storage for memory cells and scenes
**Protocol:** SQL (via database/sql + mattn/go-sqlite3)

**Connection:**
```go
db, err := sql.Open("sqlite3", "./data/nuimanbot.db?_journal_mode=WAL")
```

**Schema:**
- Tables: `memory_cells`, `memory_scenes`, `memory_cells_fts` (FTS5 virtual)
- Migrations: Applied on startup via migration tool

**Error Handling:**
- Connection failures: Retry with exponential backoff, disable memory if persistent failure
- Query errors: Wrapped and propagated to use case layer
- Constraint violations: Return `ErrAlreadyExists` or `ErrInvalidInput`

**Performance:**
- WAL mode for concurrent reads/writes
- Indexes on conversation_id, scene, salience
- FTS5 for fast full-text search (BM25 ranking)

---

### Integration 2: Anthropic API (Claude LLM)

**Type:** REST API
**Purpose:** Memory cell extraction and scene summarization
**Protocol:** HTTP/JSON

**Endpoint:**
```
POST https://api.anthropic.com/v1/messages
Authorization: x-api-key: <API_KEY>
Content-Type: application/json
anthropic-version: 2023-06-01
```

**Request (Extraction):**
```json
{
  "model": "claude-3-haiku-20240307",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "You are a memory curator...\n\n# User Message:\n...\n\n# Task:\nExtract memory cells..."
    }
  ]
}
```

**Response:**
```json
{
  "id": "msg-abc123",
  "type": "message",
  "content": [
    {
      "type": "text",
      "text": "{\"cells\": [{\"scene\": \"...\", \"cell_type\": \"decision\", ...}]}"
    }
  ]
}
```

**Error Handling:**
- Rate limits (429): Retry with exponential backoff, max 3 retries
- Network errors: Retry once, then fail gracefully
- Invalid JSON: Attempt to repair, log error, skip extraction if unrepairable
- Timeout: Circuit breaker opens after 5 consecutive timeouts

---

### Integration 3: ChatService (Internal)

**Type:** Internal Go service
**Purpose:** Trigger memory extraction and consume memory recall
**Protocol:** Go function calls (in-process)

**Integration Points:**

**A. ProcessMessage (Extraction Trigger):**
```go
func (s *ChatService) ProcessMessage(ctx context.Context, msg Message) (Response, error) {
    // ... existing logic ...

    // ASYNC: Trigger memory extraction
    go func() {
        _ = s.memoryCurator.ExtractCells(context.Background(), ExtractionInput{...})
    }()
}
```

**B. BuildContextWindow (Recall Consumer):**
```go
func (s *ChatService) BuildContextWindow(ctx context.Context, conversationID string, prompt string) ([]Message, error) {
    // Recall memory
    memory, err := s.memoryRecall.Recall(ctx, RecallInput{...})

    // Inject memory into context
    context := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "system", Content: memory.FormattedText}, // MEMORY INJECTION
    }
}
```

**Error Handling:**
- Extraction failures: Log, emit metric, don't crash chat
- Recall failures: Graceful degradation (continue without memory)

---

## Architectural Decisions

### ADR-001: Use SQLite FTS5 Instead of Vector DB

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
Need fast, relevant memory retrieval. Options: vector embeddings (e.g., pgvector, Pinecone) or full-text search (SQLite FTS5).

**Decision:**
Use SQLite FTS5 for initial release.

**Rationale:**
- **Simplicity**: FTS5 is built into SQLite, no additional infrastructure
- **Performance**: FTS5 is fast enough for <100K cells (query <50ms)
- **Cost**: No embedding API calls, no vector DB hosting
- **Good enough**: Keyword + salience scoring provides sufficient precision
- **Iterative**: Can add vector embeddings in Phase 2 if needed

**Consequences:**
**Positive:**
- Faster implementation (no new dependencies)
- Lower operational complexity
- No embedding costs
- Proven technology (FTS5 is mature)

**Negative:**
- Semantic search less sophisticated (no "similar concepts" matching)
- May not scale to millions of cells without degradation
- Can't do cross-lingual search easily

**Mitigation:**
- Monitor FTS performance with metrics (P95 latency)
- Add vector embeddings in Phase 2 if precision/scale issues arise

**Alternatives Considered:**
1. **Pinecone (vector DB SaaS)**
   - Pros: Excellent semantic search, scales well, managed
   - Cons: Additional cost (~$70/mo), external dependency, overkill for v1
   - Why rejected: Too complex for initial release, unclear ROI

2. **pgvector (PostgreSQL extension)**
   - Pros: Open-source, good semantic search, well-integrated
   - Cons: Requires PostgreSQL (not current stack), embedding costs, heavier infra
   - Why rejected: Stack complexity, want to stay with SQLite for simplicity

---

### ADR-002: Async Memory Extraction, Sync Memory Recall

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
Should memory extraction run synchronously (blocking user response) or asynchronously (in background)?

**Decision:**
Run memory extraction asynchronously (non-blocking), but memory recall synchronously (blocking).

**Rationale:**
- **UX Priority**: User shouldn't wait 5-10 seconds for extraction to complete
- **Acceptable Delay**: Memory not available until next turn, but that's acceptable
- **Recall Requirement**: Memory recall must be fast (<100ms) to not degrade response time

**Consequences:**
**Positive:**
- No response latency added by extraction
- Better user experience (fast responses)
- Scalable (extraction can be offloaded to workers in future)

**Negative:**
- Memory cells not available until next interaction
- Failures may be silent (async errors harder to surface)
- Complexity: need robust error handling + logging for async flows

**Mitigation:**
- Comprehensive logging and metrics for extraction failures
- Circuit breaker to disable extraction if failing repeatedly
- Consider adding retry queue for failed extractions (Phase 2)

**Alternatives Considered:**
1. **Synchronous Extraction**
   - Pros: Guaranteed execution, cells available immediately, simpler error handling
   - Cons: Adds 5-10s latency to every response (unacceptable UX)
   - Why rejected: User experience degradation

2. **Batched Extraction**
   - Pros: More efficient (bulk processing), reduced LLM calls
   - Cons: Significant delay before cells available, more complex
   - Why rejected: Increased complexity, delayed availability

---

### ADR-003: Per-User Memory Scope by Default

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
Should memory cells be scoped per-conversation, per-user, or global?

**Decision:**
Per-user scope by default (all conversations for a user share memory).

**Rationale:**
- **Long-term Learning**: Agent remembers preferences/context across sessions
- **User Expectation**: Users expect AI to "remember me" across conversations
- **Privacy**: Per-user isolation ensures no cross-user leakage
- **Flexibility**: Can still scope by conversation if needed via config

**Consequences:**
**Positive:**
- Better continuity across conversations
- No need to repeat context/preferences
- More natural user experience

**Negative:**
- More complex memory management (larger scope)
- Potential privacy concerns (long-term data retention)
- Harder to delete all user data (GDPR compliance)

**Mitigation:**
- Auto-expiration (90-day TTL by default)
- Admin commands for bulk deletion
- Audit logging for compliance
- Configuration option to switch to per-conversation scope

---

## Trade-offs

### Trade-off 1: Cheaper Model (Haiku) for Extraction

**Choice:** Use claude-3-haiku for extraction instead of sonnet

**Benefits:**
- Lower cost (~$0.25 per 1M tokens vs $3 for sonnet)
- Faster response time (less latency for extraction)
- Good enough accuracy for structured extraction

**Costs:**
- Slightly lower extraction quality (may miss nuanced knowledge)
- May require more prompt engineering to maintain accuracy
- Less sophisticated scene naming

**Mitigation:**
- Extensive prompt testing to ensure haiku accuracy
- Monitor extraction precision/recall metrics
- Option to upgrade to sonnet if quality issues arise

---

### Trade-off 2: Token Budget Limits Memory Injection

**Choice:** Hard cap memory injection at 1000 tokens (configurable)

**Benefits:**
- Prevents context bloat (keeps context window manageable)
- Ensures response quality (don't overwhelm LLM with memory)
- Predictable performance (bounded token count)

**Costs:**
- May miss relevant memory if budget too small
- Requires smart ranking/filtering (FTS + salience)
- Users can't access all memory at once

**Mitigation:**
- Make budget configurable (users can increase if needed)
- Prioritize high-salience cells (most important first)
- Future: adaptive budget based on conversation complexity

---

## Performance Considerations

**Bottlenecks:**
- **FTS Query Latency**: FTS queries may slow down with >100K cells
  - **Mitigation**: Result limits (10 default), indexes, query optimization
- **LLM Extraction Latency**: Anthropic API calls take 2-5 seconds
  - **Mitigation**: Run async (don't block user), circuit breaker for failures
- **Scene Consolidation**: Regenerating summaries can be slow
  - **Mitigation**: Incremental updates (not full regeneration), batching

**Optimization Strategies:**
- **Caching**: Cache scene summaries (TTL 1 hour)
- **Indexing**: SQLite indexes on conversation_id, scene, salience
- **Batching**: Batch scene consolidations (update every N cells)

**Concurrency:**
- Multiple extractions can run concurrently (goroutines)
- SQLite WAL mode handles concurrent reads/writes
- Repository operations are thread-safe (via database locking)

---

## Security Architecture

**Security Layers:**
1. **Input validation** (Use case layer): Validate all inputs before processing
2. **Authorization** (Adapter layer): RBAC for admin operations (delete, inspect)
3. **Data isolation** (Repository layer): Per-user scoping, no cross-user queries

**Threat Model:**
- **Threat 1: SQL Injection**: User input in FTS queries
  - **Mitigation**: Parameterized queries (prepared statements), no string concatenation
- **Threat 2: Cross-User Memory Leakage**: User A sees User B's memories
  - **Mitigation**: Always filter by conversation_id/user_id, integration tests for isolation
- **Threat 3: Sensitive Data in Memory**: Passwords/secrets stored in cells
  - **Mitigation**: Prompt curator to avoid storing credentials, PII scrubbing (Phase 2)

**Security Controls:**
- Parameterized SQL queries (prevent injection)
- RBAC for delete operations (admin-only)
- Audit logging for all modifications
- Per-user data isolation (enforced in repository layer)

---

## Scalability

**Current Limits:**
- Memory cells: ~100K per user (FTS performance tested up to this)
- Concurrent extractions: Limited by Anthropic API rate limits
- Scene count: ~1K scenes per user (reasonable topic diversity)

**Scaling Strategy:**
- **Vertical**: Add indexes, optimize FTS queries, increase DB connection pool
- **Horizontal**: Future: shard by user_id (each user gets own SQLite DB)

**Future Considerations:**
- Migrate to vector DB if semantic search needed
- Add background worker pool for extraction (vs goroutines)
- Implement scene archival (move old scenes to cold storage)

---

## References

- [spec.md](spec.md) - Feature specification
- [data-dictionary.md](data-dictionary.md) - Data structures
- [plan.md](plan.md) - Implementation plan
- [SQLite FTS5 Documentation](https://www.sqlite.org/fts5.html)
- [Anthropic API Reference](https://docs.anthropic.com/claude/reference)
