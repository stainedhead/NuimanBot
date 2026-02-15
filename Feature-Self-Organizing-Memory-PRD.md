# Nuimanbot vs “Self-Organizing Agent Memory System” (MarkTechPost) — Comparison + Merge Plan + PRD
_Date: 2026-02-14_

## Inputs
- Article saved as: `article-self-organizing-agent-memory-system-marktechpost-2026-02-14.md`
  - URL: https://www.marktechpost.com/2026/02/14/how-to-build-a-self-organizing-agent-memory-system-for-long-term-ai-reasoning/
- Repo reviewed: `stainedhead/Nuimanbot` @ `079fb81`
  - Notable files:
    - `internal/domain/memory.go` (skill memory model + scopes)
    - `internal/infrastructure/memory/storage.go` (SQLite `skill_memory` table)
    - `internal/usecase/chat/context_window.go` (context-window builder)
    - `internal/usecase/chat/summarization.go` (LLM conversation summarization)
    - `support_docs/memory-guide.md` (skill memory guide)

---

## 1) What the article proposes (in plain terms)
A split-brain design:
- **Reasoner/Worker Agent**: focuses on answering.
- **Memory Manager**: separately extracts *structured memory cells* from each interaction, assigns them to **scenes/topics**, and periodically consolidates scenes into stable summaries.
- **Memory DB**: stores atomic “cells” with salience and supports lexical retrieval via **SQLite FTS**.

Key mechanisms:
- **Atomic cells**: typed (`fact/plan/preference/decision/task/risk`) + **salience score**.
- **Scenes**: named buckets that get a rolling **scene summary**.
- **Retrieval**: match query tokens against FTS; fallback to high-salience cells.

### Article structure pyramid
```
           ┌──────────────────────────────┐
           │  Worker Agent (reasoning)    │
           └──────────────┬───────────────┘
                          │ uses
           ┌──────────────▼───────────────┐
           │  Scene summaries + recall    │
           └──────────────┬───────────────┘
                          │ curated by
           ┌──────────────▼───────────────┐
           │  Memory Manager (extract +   │
           │  salience + consolidate)     │
           └──────────────┬───────────────┘
                          │ stores in
           ┌──────────────▼───────────────┐
           │  SQLite: cells + scenes +    │
           │  FTS index                   │
           └──────────────────────────────┘
```

---

## 2) What Nuimanbot implements today
Nuimanbot is broader and more production-oriented overall (security, RBAC, gateways, tracing, caching, etc.) and already includes **two memory-like concepts**:

1) **Conversation history memory** (messages persisted; used to build context windows)
- `BuildContextWindow()` loads recent messages up to token limits.

2) **Conversation summarization** (compression when context is large)
- `SummarizeConversation()` uses a cheaper model to generate summaries of older messages.

3) **Skill memory** (persistent key/value state)
- `skill_memory` table in SQLite stores arbitrary JSON-serializable values keyed by `(skill_name, scope, key)`.
- Scopes exist: skill/user/global/session (user scope marked “future” in docs).

### Nuimanbot structure pyramid (simplified)
```
            ┌──────────────────────────────┐
            │ Gateways (Slack/Telegram/CLI)│
            └──────────────┬───────────────┘
                           │
            ┌──────────────▼───────────────┐
            │ Chat/Agent Use Cases         │
            │ - context window             │
            │ - tool routing               │
            │ - summarization              │
            └──────────────┬───────────────┘
                           │
            ┌──────────────▼───────────────┐
            │ Repos/Services               │
            │ - memoryRepo (messages)      │
            │ - memory storage (skill KV)  │
            │ - LLM provider service       │
            └──────────────┬───────────────┘
                           │
            ┌──────────────▼───────────────┐
            │ Infra: SQLite + vault + obs  │
            └──────────────────────────────┘
```

---

## 3) Comparison: where each is “better”

### Nuimanbot is stronger in production readiness
- Clean architecture boundaries, multi-gateway, RBAC, rate limiting, vault, audits, observability.
- Multi-LLM + fallback, streaming, tool ecosystem.
- Conversation summarization is already a pragmatic form of “compression.”

### The article is stronger in *semantic, long-horizon memory* design
- Explicit **separation**: a memory curator component distinct from the reasoning agent.
- Memory stored as **typed units** with salience, enabling selective recall.
- “Scenes” provide topic-level organization and stable summaries over time.

### Gaps / missed opportunities in Nuimanbot (relative to article)
- Skill memory is **KV**, not queryable by relevance (no FTS, no salience, no scene/topic grouping).
- Conversation summarization exists, but it’s mostly **compressing history**, not turning it into reusable knowledge units.
- Context window assembly doesn’t inject:
  - stable scene summaries
  - high-salience facts/decisions/tasks/preferences

---

## 4) Best answer: merge them
The best path is not “choose one.”

Nuimanbot should keep its production-grade platform features, and **add a new memory subsystem** inspired by the article:
- Convert some conversation content into structured “memory cells” (facts/decisions/tasks/preferences/risks)
- Organize cells into scenes/topics
- Consolidate scenes into stable summaries
- Retrieve relevant cells/summaries at inference time and inject into the context window

In other words:
- **Nuimanbot today**: “chat history + summarization + KV skill state”
- **Nuimanbot + article**: “chat history + summarization + structured long-term memory + KV skill state”

---

# PRD: Self-Organizing Memory v2 for Nuimanbot

## Problem
Long-running use becomes less coherent because:
- relevant decisions/facts/preferences are buried in message logs
- summarization compresses but doesn’t become *queryable, typed knowledge*
- the agent lacks stable, topic-organized memory that can be recalled reliably

## Goals
1) Provide **long-horizon recall** of key facts/decisions/tasks/preferences/risks.
2) Keep recall **high precision** (avoid dumping too much into prompt).
3) Separate “memory curation” from “reasoning” for maintainability and safety.
4) Work with current storage stack (SQLite) and clean architecture.

## Non-goals (initial release)
- Full vector DB / embedding retrieval (can be Phase 2).
- Fully autonomous background processing (can be Phase 2/3).
- Perfect topic modeling; start with heuristic+LLM classification.

## Users
- Primary: the agent itself (better responses)
- Secondary: admins/devs (inspect memory, correct it, delete it)

## Key Concepts / Data Model
### Memory Cell
Fields:
- `id` (uuid)
- `conversation_id` (or `user_id` scope)
- `scene` (string topic)
- `cell_type` enum: `fact | decision | task | preference | plan | risk`
- `salience` float (0..1)
- `content` JSON/text (compressed)
- `source` (message ids / timestamps)
- `created_at`, `updated_at`
- optional TTL / expiry

### Scene
Fields:
- `scene` primary key
- `summary` text (stable, <= N tokens)
- `updated_at`

### Storage (SQLite)
- `memory_cells` table
- `memory_scenes` table
- `memory_cells_fts` virtual table using FTS5 over (content, scene, cell_type)

## Functional Requirements
### FR1 — Extract memory cells from interactions
- On each completed interaction (or after a threshold), run a **Memory Curator** step:
  - input: user msg + assistant msg (+ optional tool outputs)
  - output: JSON array of memory cells
- Must be robust to invalid JSON (retry/repair once; else skip).

### FR2 — Consolidate scenes
- After inserting cells, update scene summaries for any touched scenes.
- Summaries should be stable and reusable; avoid ephemeral phrasing.

### FR3 — Retrieve relevant memory for a new prompt
- When building a context window, add:
  - top-k matching cells by FTS
  - plus top salience cells if FTS yields too little
  - plus summaries for the scenes involved
- Hard token budget for injected memory (e.g., 500–1500 tokens configurable).

### FR4 — Admin/inspection API
- REST endpoints (or CLI) to:
  - list scenes
  - get scene summary
  - search cells
  - delete cell/scene
- Audit all modifications.

### FR5 — Safety & access controls
- Respect existing RBAC model.
- Support per-user isolation by default; allow global scope only if configured.

## Integration Plan (Clean Architecture)
- Domain:
  - `MemoryCell`, `MemoryScene` entities
  - `MemoryCellRepository` interface
- Usecase:
  - `MemoryCuratorService` (extract + consolidate)
  - `MemoryRecallService` (retrieve injection bundle)
- Infrastructure:
  - SQLite implementation + FTS schema migration
- Chat service changes:
  - `BuildContextWindow()` calls `MemoryRecallService` and injects results.

## UX/Prompting
- Inject memory into the LLM request as a distinct section, e.g.:
  - `### Relevant long-term memory (curated)`
  - `### Scene summaries`
- Keep it clearly separated from recent message history.

## Observability
- Metrics:
  - memory cells created per conversation/day
  - recall hit rate (FTS matches vs fallback)
  - injected token count
  - curator failures (JSON parse, LLM failures)
- Tracing spans around curate/retrieve.

## Rollout
- Feature flag: `memory.self_organizing.enabled`
- Start in “observe-only” mode (store cells but don’t inject) for 1–2 weeks.
- Then enable injection for pilot users.

## Open Questions
- Should scenes be per-conversation, per-user, or global? (Recommendation: per-user by default.)
- Do we want human-editable scenes? (Probably yes, via admin API.)
- When to run curation: every turn vs batch vs background worker?

---

## Recommendation summary
- **Which is better?** Nuimanbot is better as a platform; the article’s approach is better as a memory strategy.
- **Merge strategy:** add a structured memory subsystem (cells/scenes/salience/FTS) and hook it into the context window builder.
- **Impact:** improved long-horizon coherence, better task/decision continuity, and a clearer, inspectable memory model.

---

# Addendum: Complete Nuimanbot’s existing (but not fully wired) memory hooks
This addendum focuses on **finishing what Nuimanbot already started**:
- `BuildContextWindow()` exists but `ProcessMessage()` doesn’t use it.
- `SummarizeConversation()` exists but is not called in runtime flows.
- Skill-memory expiration exists but cleanup is not clearly scheduled.
- Token counting is inconsistently applied (some TODOs), limiting correctness of any “max tokens” behaviors.

The goal is to make the **current implementation complete and coherent** *before* introducing the new self-organizing memory subsystem.

## A1) Wire `BuildContextWindow()` into `ChatService.ProcessMessage()`
**Current:** `ProcessMessage()` directly calls `GetRecentMessages(..., 4096)` and appends them.

**Change:** replace that manual history assembly with:
1) Determine provider + token budget (from config/user prefs)
2) Call `BuildContextWindow(ctx, conversationID, provider, maxTokens)`
3) Use returned messages as the history section

**Acceptance criteria**
- Context windows respect provider limits (`AnthropicTokenLimit`, etc.) and reserve response tokens.
- Tests: add/adjust tests to ensure newest messages are retained and older messages trimmed deterministically.

**Implementation sketch**
- In `ProcessMessage()`:
  - Build `conversationID`
  - `history, historyTokens := s.BuildContextWindow(ctx, conversationID, provider, maxContextTokens)`
  - `llmMessages := append(history, domain.Message{Role:"user", Content: incomingMsg.Text})`

## A2) Implement runtime summarization trigger (use `SummarizeConversation()` for compression)
**Current:** `SummarizeConversation()` is implemented and tested, but appears not invoked in production message processing.

**Design intent:** When conversations get long, summarize older segments so the agent can keep context without exploding token budgets.

**Proposed behavior (Phase 1: pragmatic)**
- Add a config setting:
  - `chat.summarization.enabled` (bool)
  - `chat.summarization.threshold_tokens` (e.g., 0.7 * maxContextTokens)
  - `chat.summarization.max_tokens` (tokens of messages to summarize)
- On `ProcessMessage()` before building the final prompt:
  1) Compute current conversation token footprint (see A3)
  2) If above threshold, call `SummarizeConversation(conversationID, maxTokensToSummarize)`
  3) Persist the summary as a **system** message (or dedicated summary record)
  4) Optionally delete/compact the summarized messages (depends on retention requirements)

**Key decision point:** *Where to store summaries?*
- **Option A (quick, consistent with current model):** store summary as a `StoredMessage{Role:"system"}` with metadata tag `summary=true`.
- **Option B (cleaner):** add a `conversation_summaries` table keyed by conversation + range.

**Acceptance criteria**
- When a conversation grows, summarization happens automatically and doesn’t break the flow.
- Summaries are included in subsequent context windows.
- Summarization uses a cheaper model already chosen (`claude-3-haiku-20240307`).

## A3) Finish token accounting so context logic is correct
Several places rely on token counts but have TODOs.

**Current:**
- `BuildContextWindow()` expects `StoredMessage.TokenCount` to be accurate.
- `ProcessMessage()` sometimes sets TokenCount from LLM usage for assistant replies, but user messages are TODO.

**Change:** implement a token counting strategy:
- **Option A:** approximate tokens by a heuristic (fast, provider-agnostic; good enough for trimming).
- **Option B:** provider-specific tokenization libraries (more accurate; more complexity).

**Recommendation:** Start with A (heuristic), then upgrade to B if needed.

**Acceptance criteria**
- Every stored message has a non-zero `TokenCount`.
- Context window trimming and summarization thresholding behave predictably.

## A4) Schedule/drive `skill_memory` cleanup (expired TTL rows)
**Current:**
- Expiration is enforced in `MemoryAPI.Recall()` (expired key triggers delete-on-read).
- Storage layer has `SQLiteMemoryStorage.Cleanup()`.
- But there’s no obvious periodic trigger.

**Change:** Add a lightweight background cleanup loop in the application composition root (e.g., `cmd/nuimanbot`), guarded by config:
- `memory.cleanup.enabled` (bool)
- `memory.cleanup.interval_seconds` (default 3600)

**Acceptance criteria**
- Expired memory entries are deleted even if never recalled.
- Cleanup runs with logging + metrics.

## A5) Make “memory management” a first-class pipeline stage
Once A1–A4 exist, formalize the stages in `ProcessMessage()`:
1) Validate input
2) Ensure conversation exists / load metadata
3) **(Optional) Summarize/compact conversation**
4) Build context window
5) LLM/tool loop
6) Persist messages (+ token counts)
7) Emit telemetry

This staging makes it straightforward later to insert the “self-organizing memory curator” as:
- Post-response hook (curate cells)
- Pre-prompt hook (recall cells)

## A6) Docs & tests to prove completeness
- Update docs to reflect real behavior:
  - `support_docs/memory-guide.md` should distinguish:
    - conversation history
    - conversation summaries (auto)
    - skill KV memory
- Add tests:
  - context window respects limits (already present)
  - summarization triggers and summary inclusion
  - cleanup removes expired memory

---

## Outcome
After this addendum:
- Nuimanbot’s current memory primitives (history/context/summarization/TTL) operate as a complete system.
- The new structured “cells/scenes” memory can be added with fewer moving parts and a clean insertion point.