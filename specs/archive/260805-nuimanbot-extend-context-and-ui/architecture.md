# Architecture: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05
**Status:** Draft

## Architecture Overview

[TBD — to be drafted from spec.md's "System Architecture" and "Scope of Changes" sections once research.md's open questions (cron library choice, worker-pool crash-recovery design, WebSocket connection lifecycle) are resolved. This document should produce the definitive component diagram and layer-by-layer responsibility breakdown that `plan.md` and `tasks.md` build task sequencing from.]

## Component Architecture

[TBD — expected top-level components:]
- Six environment handlers (Chats, Projects, Jobs, Chores, History, Memories) + Settings handler in `internal/adapter/web`
- Worker pool + FIFO queue + cron scheduler in `internal/infrastructure/scheduler` (net-new, highest-risk subsystem per spec.md's Risks table)
- File-based repositories for Project/Job/Chore/Run in `internal/infrastructure/storage`, modeled on `FileConversationRepository`
- WebSocket hub/broadcaster for run status/log/notification push (new use of `gorilla/websocket` in `internal/adapter/web`)
- Network-access middleware (allowlist enforcement, pre-auth) in `internal/adapter/web/middleware.go`

## Layer Responsibilities

Per AGENTS.md's Clean Architecture rule (dependencies flow inward only; inner layers define interfaces, outer layers implement them):

- **`internal/domain`**: `Project`, `Job`, `Chore`, `Run` entities; `RetentionPolicy`, `Schedule` value objects; repository interfaces (`ProjectRepository`, `JobRepository`, `ChoreRepository`, `RunRepository`). No dependency on usecase/adapter/infrastructure.
- **`internal/usecase`**: Orchestration for create/list/delete across all six environments; Job/Chore enqueue logic; scheduler evaluation logic (depends only on domain interfaces, not concrete infrastructure).
- **`internal/adapter/web`**: HTTP handlers, templates, WebSocket upgrade/broadcast, converts domain entities to view models. Implements no business logic beyond request/response shaping.
- **`internal/infrastructure`**: `scheduler` package (worker pool, queue, cron evaluation), `storage` package additions (new file-based repositories using `AtomicFileWriter`/`FileLock`), extended `gorilla/websocket` usage.

## Data Flow

[TBD — sequence for: (1) Job creation → queue → worker pickup → run → completion → notification → History; (2) Chore cron fire → skip-if-active check → run → same downstream path; (3) WebSocket push flow from a completing run to a connected browser tab, with per-user isolation enforced.]

## Sequence Diagrams

[TBD — add during Phase 3. Priority diagrams: Job lifecycle (queued→running→completed/failed), Chore cron-fire-with-skip, WebSocket connect/auth/per-user-subscribe/push, server-restart recovery (queue + in-flight runs + Chore next-fire-times all restored correctly).]

## Integration Points

- Existing `ConversationRepository` / `FileConversationRepository` — Chats extend this, not a new store.
- Existing `AtomicFileWriter` / `FileLock` (`internal/infrastructure/storage/atomic_file_writer.go`) — persistence primitive for all new entities and worker-pool run-state.
- Existing `memoryv2` repositories — read-only integration point for Memories.
- Existing `skill.go` / `skills_config.go` — read integration point for Settings' Skills/Plugins surface.
- Existing `internal/adapter/web/middleware.go` `requireRole(domain.Role)`, CSRF protection, login rate limiting — all new routes must compose with these, not bypass them.
- Existing `internal/config/config.go` `ServerConfig`/`SecurityConfig` pattern — new `NetworkAccessConfig`, retention defaults, and `WorkerPoolConfig` follow the same YAML-tagged struct convention.
- `github.com/gorilla/websocket` — existing dependency, net-new usage surface in `internal/adapter/web`; only prior usage is `internal/infrastructure/nostr/client.go` (Buzz gateway), a different protocol context — treat as new engineering per spec.md's Risks table.

## Architectural Decisions

**Resolved during spec review (2026-08-05)**, to unblock implementation-readiness ahead of Phase 3's detailed design work (sequence diagrams, exact data flow, and any refinement below build on — not reopen — these decisions unless Phase 3 surfaces new evidence):

| Decision | Choice | Rationale (full detail in spec.md / research.md) |
|---|---|---|
| Cron library | `github.com/robfig/cron/v3` (new `go.mod` dep) | De facto standard; provides `Schedule.Next()` needed for FR-035 + restart-durability NFR; in-house parser would reinvent tested logic. research.md Q1. |
| Worker-pool persistence | `AtomicFileWriter` + `FileLock` (existing primitives), no WAL | Single-process/single-machine pool; same atomic-rename+flock guarantee already protects `users.json`/`bots.json`. research.md Q2. |
| Usecase package structure | Per-environment packages: `internal/usecase/{project,job,chore,history,memories}/` | Matches existing convention (`chat/`, `user/`, `skill/`, `memory/`, `memoryv2/` are all per-domain packages already). Verified against current repo layout. |
| WebSocket connection/subscription model | One connection per browser tab, subscribed to a per-user broadcast channel; reconnect = client backoff + full HTTP resync, no server replay buffer | Simplest model that satisfies per-user isolation and multi-tab support; avoids replay-buffer complexity. research.md Q5. |
| Project directory sandboxing mechanism | New dedicated helper (indicative `internal/infrastructure/fsguard`, exact name/API TBD in Phase 3) exposing `ResolveWithin(baseDir, relPath string) (string, error)` | No existing reusable path-confinement utility (`preprocess.CommandSandbox` sandboxes command execution, not paths; `FetchSecurityConfig` guards SSRF, not local paths) — confirmed by direct repo search. research.md Q6. |

**Still open, deliberately deferred to later phases (not a gap):**
- Default numeric values (retention windows, worker pool size N) — `plan.md` Phase 4, after this document's Phase 3 output is available (research.md Q3).
- Exact WebSocket message schema, sequence diagrams, detailed data flow — this document's own Phase 3 completion (see stubs above/below), building on the connection-model decision already made.
- Exact `fsguard`-equivalent package name/API surface — Phase 3, building on the "must be net-new" decision already made.
