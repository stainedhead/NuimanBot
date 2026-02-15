# Self-Organizing Memory v2 - Research

**Created:** 2026-02-15
**Source:** `Feature-Self-Organizing-Memory-PRD.md`
**Status:** In Progress

---

## Overview

Research for implementing a self-organizing memory system in Nuimanbot, inspired by the MarkTechPost article on long-term AI reasoning. This system will transform chat history into structured, queryable knowledge units (memory cells) organized by topics (scenes) with salience-based retrieval.

**Research Questions:**
1. How does SQLite FTS5 work and how to optimize for our use case?
2. What LLM prompts work best for memory cell extraction?
3. How to implement salience scoring for memory cells?
4. What's the optimal scene consolidation strategy (incremental vs full regeneration)?
5. How to handle memory cell expiration and cleanup efficiently?
6. What token counting strategy works best across multiple LLM providers?
7. How to implement circuit breaker pattern for LLM curator failures?

---

## SQLite FTS5 (Full-Text Search)

### What is FTS5?

SQLite FTS5 is a full-text search extension that provides fast text search capabilities using inverted indexes.

**Key Features:**
- Tokenization and stemming
- Phrase queries and boolean operators
- Ranking by relevance (BM25)
- Fast lookups (inverted index)
- Virtual table interface

### FTS5 Schema Design

**Best Practices:**
- Create separate FTS virtual table that shadows main table
- Index only searchable text fields (not IDs, timestamps, etc.)
- Use triggers to keep FTS table in sync with main table
- Choose appropriate tokenizer (unicode61, porter, etc.)

**Example Schema:**
```sql
-- Main table
CREATE TABLE memory_cells (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    scene TEXT NOT NULL,
    cell_type TEXT NOT NULL,
    salience REAL NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,  -- JSON array of message IDs
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP
);

-- FTS virtual table
CREATE VIRTUAL TABLE memory_cells_fts USING fts5(
    content,
    scene,
    cell_type,
    content='memory_cells',  -- Content table
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER memory_cells_ai AFTER INSERT ON memory_cells BEGIN
    INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
    VALUES (new.rowid, new.content, new.scene, new.cell_type);
END;

CREATE TRIGGER memory_cells_ad AFTER DELETE ON memory_cells BEGIN
    DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER memory_cells_au AFTER UPDATE ON memory_cells BEGIN
    UPDATE memory_cells_fts
    SET content = new.content, scene = new.scene, cell_type = new.cell_type
    WHERE rowid = old.rowid;
END;
```

### FTS5 Query Syntax

**Basic Search:**
```sql
-- Simple keyword search
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH 'authentication';

-- Phrase search
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH '"user authentication"';

-- Boolean operators
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH 'authentication AND oauth';
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH 'authentication OR login';
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH 'authentication NOT password';

-- Field-specific search
SELECT * FROM memory_cells_fts WHERE memory_cells_fts MATCH 'scene:project-setup';
```

**Ranking:**
```sql
-- BM25 ranking (default)
SELECT *, rank FROM memory_cells_fts
WHERE memory_cells_fts MATCH 'authentication'
ORDER BY rank;
```

### Performance Optimization

**Strategies:**
1. **Limit result size**: Use `LIMIT` clause to cap results
2. **Filter before FTS**: Apply non-FTS filters first (user_id, timestamps)
3. **Use covering indexes**: Index frequently queried columns
4. **Batch inserts**: Use transactions for bulk operations
5. **Tokenizer choice**: `unicode61` for general text, `porter` for stemming

**Benchmarking:**
```bash
# Run FTS benchmarks
go test -bench=BenchmarkFTS -benchmem ./internal/infrastructure/memory/
```

**Expected Performance:**
- 10K cells: FTS query < 10ms
- 100K cells: FTS query < 50ms
- 1M cells: FTS query < 200ms

---

## Memory Cell Extraction (LLM Prompting)

### Extraction Prompt Design

**Goal:** Extract structured memory cells from conversation interactions with high precision and recall.

**Prompt Template (v1):**
```
You are a memory curator for an AI assistant. Your job is to extract key knowledge from conversations and structure it as memory cells.

# Conversation Interaction

## User Message:
{user_message}

## Assistant Response:
{assistant_response}

## Tool Outputs (if any):
{tool_outputs}

# Task

Extract memory cells from this interaction. For each piece of important information, create a memory cell with:
- **scene**: Topic/project name (e.g., "project-setup", "user-preferences", "authentication-design")
- **cell_type**: One of [fact, decision, task, preference, plan, risk]
- **salience**: Float 0.0-1.0 indicating importance (0.9+ = critical, 0.7-0.9 = high, 0.5-0.7 = medium, <0.5 = low)
- **content**: Concise summary of the knowledge (50-200 chars)
- **source**: List of message IDs involved

# Cell Type Definitions

- **fact**: Objective information (e.g., "User is building a Golang CLI app")
- **decision**: A choice was made (e.g., "Decided to use SQLite instead of PostgreSQL")
- **task**: Something to do or track (e.g., "Need to implement authentication flow")
- **preference**: User's preference or style (e.g., "User prefers TDD workflow")
- **plan**: Future intention or strategy (e.g., "Plan to add OAuth support in Phase 2")
- **risk**: Potential issue or concern (e.g., "Risk: FTS performance may degrade with 1M+ cells")

# Output Format

Return valid JSON only (no markdown, no commentary):

{
  "cells": [
    {
      "scene": "string",
      "cell_type": "fact|decision|task|preference|plan|risk",
      "salience": 0.0-1.0,
      "content": "string",
      "source": ["msg-id-1", "msg-id-2"]
    }
  ]
}

If no important information to extract, return: {"cells": []}

# Guidelines

- Only extract genuinely important information (not casual chat)
- Assign salience based on long-term value (critical decisions = 0.9+, preferences = 0.7+)
- Group related cells into the same scene
- Keep content concise but self-contained
- Use consistent scene naming (lowercase-with-dashes)
```

**Testing Strategy:**
- Manual testing with sample conversations
- Measure extraction accuracy (precision/recall)
- Iterate on prompt based on failures
- Use cheaper model (haiku) to reduce costs

---

## Salience Scoring

### What is Salience?

Salience = importance/relevance of a memory cell for long-term recall.

**Salience Scale:**
- **0.9-1.0**: Critical (major decisions, core facts, essential preferences)
- **0.7-0.9**: High (important context, key tasks, significant risks)
- **0.5-0.7**: Medium (useful details, minor decisions, background info)
- **0.0-0.5**: Low (ephemeral info, easily derivable facts)

### Salience Scoring Strategies

**Option 1: LLM-Assigned Salience**
- Pros: Contextual understanding, nuanced scoring
- Cons: Inconsistent, harder to calibrate
- Implementation: Include salience in extraction prompt

**Option 2: Rule-Based Salience**
- Pros: Consistent, predictable
- Cons: Lacks context, overly simplistic
- Implementation:
  - `decision` type = 0.8-0.9
  - `preference` type = 0.7-0.8
  - `fact` type = 0.5-0.7
  - `task` type = 0.6-0.8
  - `plan` type = 0.6-0.7
  - `risk` type = 0.7-0.9

**Option 3: Hybrid (Recommended)**
- LLM suggests salience
- Rules enforce minimum/maximum based on type
- Calibration over time based on recall hit rate

**Example Calibration:**
```go
func CalibrateS Salience(cellType CellType, llmSalience float64) float64 {
    min, max := getSalienceRange(cellType)
    calibrated := llmSalience
    if calibrated < min {
        calibrated = min
    }
    if calibrated > max {
        calibrated = max
    }
    return calibrated
}

func getSalienceRange(cellType CellType) (min, max float64) {
    switch cellType {
    case CellTypeDecision:
        return 0.7, 1.0
    case CellTypePreference:
        return 0.6, 0.9
    case CellTypeFact:
        return 0.3, 0.8
    case CellTypeTask:
        return 0.5, 0.9
    case CellTypePlan:
        return 0.5, 0.8
    case CellTypeRisk:
        return 0.6, 0.9
    default:
        return 0.0, 1.0
    }
}
```

---

## Scene Consolidation Strategies

### Incremental vs Full Regeneration

**Option 1: Full Regeneration**
- Regenerate scene summary from scratch every time
- Pros: Always accurate, no drift
- Cons: Expensive (reprocess all cells), slower

**Option 2: Incremental Update**
- Update summary with only new cells
- Pros: Fast, cheap
- Cons: Can drift over time, harder to maintain coherence

**Option 3: Hybrid (Recommended)**
- **Fast path**: Incremental update for small changes (< N new cells)
- **Slow path**: Full regeneration periodically (every M updates or K days)
- Best of both worlds

### Scene Summary Prompt

**Prompt Template (Incremental):**
```
You are summarizing a topic (scene) in an AI assistant's memory system.

# Current Scene Summary

Scene: {scene_name}
Current Summary: {current_summary}

# New Memory Cells Added

{list of new cells with type, salience, content}

# Task

Update the scene summary to incorporate the new cells. The summary should:
- Be 200-500 tokens max
- Provide stable context (avoid "recently", "this week", etc.)
- Highlight high-salience information
- Group related concepts
- Be self-contained (readable without seeing cells)

Return ONLY the updated summary text (no JSON, no commentary).
```

**Prompt Template (Full Regeneration):**
```
You are summarizing a topic (scene) in an AI assistant's memory system.

# Scene: {scene_name}

# All Memory Cells in This Scene

{list of all cells with type, salience, content}

# Task

Create a comprehensive scene summary that:
- Is 200-500 tokens max
- Provides stable context (avoid temporal references)
- Highlights high-salience information
- Groups related concepts
- Is self-contained

Return ONLY the summary text (no JSON, no commentary).
```

---

## Token Counting Strategies

### Why Token Counting Matters

Accurate token counts are critical for:
- Context window trimming (stay within provider limits)
- Memory budget enforcement (limit injected memory tokens)
- Summarization triggers (summarize when threshold exceeded)
- Performance optimization (predict LLM costs)

### Token Counting Options

**Option 1: Heuristic (Fast, Approximate)**
```go
// Rough approximation: 1 token ≈ 4 characters for English text
func ApproximateTokenCount(text string) int {
    return len(text) / 4
}
```
- Pros: Fast, no dependencies
- Cons: Inaccurate (20-30% error), provider-agnostic
- Use case: Quick estimates, non-critical trimming

**Option 2: Provider-Specific Tokenizers**
```go
// Anthropic (Claude) - use tiktoken or similar
import "github.com/pkoukk/tiktoken-go"

func ClaudeTokenCount(text string) (int, error) {
    tkm, err := tiktoken.EncodingForModel("claude-3-sonnet-20240229")
    if err != nil {
        return 0, err
    }
    tokens := tkm.Encode(text, nil, nil)
    return len(tokens), nil
}
```
- Pros: Accurate (< 5% error), correct for billing
- Cons: Slower, requires library, provider-specific
- Use case: Precise budgeting, billing estimation

**Option 3: Hybrid (Recommended)**
- Use heuristic for fast estimates (context trimming)
- Use provider tokenizer for critical operations (billing, final checks)
- Cache results where possible

### Implementation Strategy

```go
type TokenCounter interface {
    CountTokens(text string) (int, error)
}

// Fast approximate counter
type HeuristicTokenCounter struct{}

func (h *HeuristicTokenCounter) CountTokens(text string) (int, error) {
    return len(text) / 4, nil
}

// Accurate provider-specific counter
type ClaudeTokenCounter struct {
    encoding tiktoken.Encoding
}

func (c *ClaudeTokenCounter) CountTokens(text string) (int, error) {
    tokens := c.encoding.Encode(text, nil, nil)
    return len(tokens), nil
}

// Cached counter to avoid re-counting
type CachedTokenCounter struct {
    underlying TokenCounter
    cache      map[string]int
    mu         sync.RWMutex
}

func (c *CachedTokenCounter) CountTokens(text string) (int, error) {
    // Check cache first
    c.mu.RLock()
    if count, ok := c.cache[text]; ok {
        c.mu.RUnlock()
        return count, nil
    }
    c.mu.RUnlock()

    // Count and cache
    count, err := c.underlying.CountTokens(text)
    if err != nil {
        return 0, err
    }

    c.mu.Lock()
    c.cache[text] = count
    c.mu.Unlock()

    return count, nil
}
```

---

## Circuit Breaker Pattern for LLM Failures

### Why Circuit Breaker?

Memory curator makes LLM calls that can fail due to:
- Rate limits
- Network errors
- Invalid JSON responses
- Timeout

Circuit breaker prevents:
- Cascading failures
- Wasted retries during outages
- Excessive latency from repeated timeouts

### Circuit Breaker States

```
       ┌──────────┐
       │  Closed  │ (Normal operation)
       └─────┬────┘
             │ Failure threshold exceeded
             ▼
       ┌──────────┐
       │   Open   │ (Reject requests immediately)
       └─────┬────┘
             │ Timeout elapsed
             ▼
    ┌────────────────┐
    │  Half-Open     │ (Test with single request)
    └────┬───────┬───┘
         │       │
   Success│       │Failure
         ▼       ▼
    ┌────────┐ ┌────────┐
    │ Closed │ │  Open  │
    └────────┘ └────────┘
```

### Implementation

```go
type CircuitBreaker struct {
    maxFailures int
    resetTimeout time.Duration
    state atomic.Value // State: Closed, Open, HalfOpen
    failures int
    lastFailureTime time.Time
    mu sync.RWMutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    state := cb.getState()

    if state == StateOpen {
        // Check if reset timeout elapsed
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.setState(StateHalfOpen)
        } else {
            return ErrCircuitOpen
        }
    }

    err := fn()

    if err != nil {
        cb.recordFailure()
        return err
    }

    cb.recordSuccess()
    return nil
}

func (cb *CircuitBreaker) recordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures++
    cb.lastFailureTime = time.Now()

    if cb.failures >= cb.maxFailures {
        cb.setState(StateOpen)
    }
}

func (cb *CircuitBreaker) recordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failures = 0
    cb.setState(StateClosed)
}
```

**Configuration:**
```yaml
memory:
  curator:
    circuit_breaker:
      max_failures: 5        # Open after 5 consecutive failures
      reset_timeout: 60s     # Try again after 60 seconds
```

---

## Open Questions

### Question 1: Per-Conversation vs Per-User Memory Scope?

**Context:** Should memory cells be scoped per conversation or per user?

**Options:**
1. **Per-Conversation**: Each conversation has its own memory (isolated)
   - Pros: Clean separation, easier to manage, no cross-conversation contamination
   - Cons: Agent forgets across conversations, user has to repeat context

2. **Per-User**: All user conversations share memory (unified)
   - Pros: Long-term learning, no repetition, better continuity
   - Cons: More complex, potential privacy concerns, harder to delete

3. **Hybrid**: Conversations have local memory + user has global memory
   - Pros: Best of both worlds
   - Cons: Most complex, harder to implement

**Research Needed:**
- Survey user expectations (do they expect agent to remember across sessions?)
- Privacy implications (GDPR, data retention)
- Technical feasibility with current conversation model

**Decision:** [TBD after research]

---

### Question 2: When to Run Memory Curation?

**Context:** Should curation run synchronously (blocks response), asynchronously (background), or batched?

**Options:**
1. **Synchronous**: Extract cells before sending response
   - Pros: Guaranteed execution, cells available immediately
   - Cons: Adds latency to every response (5-10s)

2. **Asynchronous**: Extract cells in background after response sent
   - Pros: No response latency, better UX
   - Cons: Cells not available until next turn, failures may be silent

3. **Batched**: Periodically process multiple interactions at once
   - Pros: More efficient, bulk processing
   - Cons: Significant delay before cells available, more complex

**Research Needed:**
- Measure curator latency (P50, P95, P99)
- User acceptance of response delay vs memory delay
- Failure handling for async processes

**Decision:** [TBD - leaning toward async for better UX]

---

### Question 3: How to Handle Scene Naming/Discovery?

**Context:** Should scenes be LLM-generated, user-defined, or rule-based?

**Options:**
1. **LLM-Generated**: Curator assigns scene name during extraction
   - Pros: Automatic, no manual work
   - Cons: Inconsistent naming, scene proliferation

2. **User-Defined**: User creates scenes, curator assigns to existing
   - Pros: Consistent, intentional
   - Cons: Manual overhead, user burden

3. **Hybrid**: LLM suggests, rules enforce patterns, user can override
   - Pros: Balance automation and control
   - Cons: Most complex

**Research Needed:**
- How many scenes does typical user generate?
- Can LLM produce consistent names with good prompting?
- Should scenes be hierarchical (parent/child)?

**Decision:** [TBD - start with LLM-generated, add manual tools later]

---

## References

### SQLite FTS Documentation
- [SQLite FTS5 Extension](https://www.sqlite.org/fts5.html)
- [FTS5 Full-Text Query Syntax](https://www.sqlite.org/fts5.html#full_text_query_syntax)
- [FTS5 Auxiliary Functions](https://www.sqlite.org/fts5.html#the_bm25_function)

### Go SQLite Libraries
- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) - CGo-based driver
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - Pure Go implementation

### Token Counting Libraries
- [tiktoken-go](https://github.com/pkoukk/tiktoken-go) - Go port of OpenAI's tiktoken
- [anthropic-tokenizer](https://github.com/anthropics/anthropic-sdk-go) - Anthropic SDK with tokenization

### Circuit Breaker Patterns
- [gobreaker](https://github.com/sony/gobreaker) - Circuit breaker library for Go
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html) - Martin Fowler

### Research Papers
- [MarkTechPost Article](https://www.marktechpost.com/2026/02/14/how-to-build-a-self-organizing-agent-memory-system-for-long-term-ai-reasoning/) - Inspiration for this feature

---

## Experiments & Prototypes

### Experiment 1: FTS5 Performance Benchmarking

**Goal:** Measure FTS query performance with different cell counts

**Setup:**
- Create test database with 10K, 100K, 1M cells
- Run various query patterns (keyword, phrase, boolean)
- Measure latency (P50, P95, P99)

**Results:** [To be filled after experiment]

---

### Experiment 2: Memory Cell Extraction Accuracy

**Goal:** Measure precision/recall of cell extraction

**Setup:**
- Manually label 20-30 sample conversations with expected cells
- Run curator on same conversations
- Compare extracted cells with ground truth
- Calculate precision (% extracted that are correct) and recall (% correct that were extracted)

**Results:** [To be filled after experiment]

---

### Experiment 3: Salience Scoring Consistency

**Goal:** Measure consistency of LLM-assigned salience scores

**Setup:**
- Run curator on same conversations multiple times
- Measure variance in salience scores
- Compare LLM salience with manual expert salience

**Results:** [To be filled after experiment]

---

## Next Steps

1. **Complete FTS5 schema design** - Finalize table structure and indexes
2. **Prototype extraction prompt** - Test with sample conversations
3. **Benchmark FTS queries** - Measure performance with realistic data
4. **Decide on token counting strategy** - Choose heuristic vs provider-specific
5. **Finalize scene consolidation approach** - Incremental vs full regeneration
6. **Test circuit breaker implementation** - Verify failure handling

---

**Research Status:** In Progress
**Next Update:** After FTS5 benchmarking and extraction prototype
