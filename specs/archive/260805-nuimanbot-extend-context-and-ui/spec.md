# Spec: NuimanBot — Extend Context & UI (Chats, Projects, Jobs, Chores, History, Memories)

**Created:** 2026-08-05
**Source PRD:** `specs/260805-nuimanbot-extend-context-and-ui/nuimanbot-extend-context-and-ui-PRD.md`
**Status:** Draft

## Executive Summary

Extend NuimanBot's existing web admin (`internal/adapter/web`) — today scoped to operational management (bots, users, confirmations) — with six new user-facing environments: **Chats**, **Projects**, **Jobs**, **Chores**, **History**, and **Memories**. Add a **Settings** environment covering Skills/Plugins/Gateways/Users (system-wide) and retention/worker-pool configuration, plus configurable network exposure (localhost-only vs. remote with optional IP/hostname allowlisting) so the expanded surface can run safely outside a single trusted machine.

## Problem Statement

There is currently no user-facing surface for working with the agent: no persistent conversation history, no durable project context the agent can read/write against, no way to run a one-off or recurring agent task and inspect results later, and no browsable knowledge/memory store. This spec closes that gap by adding six environments plus Settings and network-access controls to the existing web UI.

## Goals

- Persistent, organized workspace for agent interaction: lightweight ephemeral Chats plus durable, directory-scoped Projects.
- Autonomous agent work beyond live chat: one-time Jobs (FIFO-queued) and recurring Chores (cron-scheduled), each with full run history, logs, and a post-hoc chat interface.
- Safe deployment beyond a single trusted machine via configurable network exposure (localhost-only, or remote with optional allowlisting).

## Non-Goals

- A separate compiled desktop application (stays on the web UI).
- Rebuilding the skills/plugins config system (`internal/domain/skill.go` / `skills_config.go` surfaced as-is, not redesigned).
- A general-purpose file browser/manager for Chats (Chats stay directory-less by design).
- A general-purpose OS cron replacement (Chores schedule agent-driven tasks only).
- Changing or deprecating existing gateways (CLI, Telegram, Slack, Buzz) — additive to the web UI only.
- Real-time multi-user collaborative editing of Project files.
- **(Added, spec review 2026-08-05) Chore pause/disable:** no FR describes suspending a Chore without deleting it. A Chore is either Active (has a schedule) or deleted; there is no "paused" state in this spec. A follow-up PRD can add this if requested.
- **(Added, spec review 2026-08-05) User-initiated run cancellation:** no FR describes cancelling an in-flight Job/Chore run. Once a run starts, it runs to completion, failure, or process crash (recorded via the Error field) — there is no "Cancelled" run status in this spec.
- **(Added, spec review 2026-08-05) Storage/disk quota enforcement or usage-warning UI:** "Never" retention (FR-003/014/023/043) is a valid, unenforced user choice in this spec; no disk-usage surfacing, warning, or quota is implemented. Flagged in research.md as a candidate follow-up PRD if unbounded growth becomes an operational problem.

## User Requirements / Functional Requirements

### Settings

Settings are split by scope: **system-wide/admin-only** (whole deployment, admin-changeable only) vs. **per-user** (the logged-in user's own resources).

- **FR-001:** Settings surfaces configuration for Skills, Plugins, and Gateways (system-wide/admin-only, existing systems) and Users (system-wide/admin-only user management).
- **FR-002:** Settings surfaces Network Access configuration (system-wide/admin-only).
- **FR-003:** Settings surfaces retention configuration independently for Chats, Projects, and History (per-user); each supports "Never" (no auto-expiry) plus a configurable time period.
- **FR-004:** Settings surfaces the Job/Chore worker pool size (system-wide/admin-only — N concurrent workers is a process-level resource).

### Access / Network

- **FR-005:** Web server supports localhost-only mode (binds 127.0.0.1 only).
- **FR-006:** Web server supports remote-access mode (binds a configured interface).
- **FR-007:** In remote-access mode, an optional IP/hostname allowlist may be configured; non-allowlisted requests are rejected before reaching application handlers.
- **FR-008:** Access mode and allowlist are configurable via config file and Settings UI, consistent with the existing `config.yaml` pattern.

### Users / Ownership

- **FR-009:** Every Chat, Project, Job, and Chore belongs to exactly one owning user.
- **FR-010:** Users see and access only their own Chats, Projects, Jobs, Chores, and History — full isolation, including from admins.

### Chats

- **FR-011:** User can create a Chat; no working directory is exposed.
- **FR-012:** Chat is auto-named from the text of its first message.
- **FR-013:** Full chat message history is persisted.
- **FR-014:** Chat retention is configurable (including "Never"); Chats older than a configured, non-"Never" period are deleted automatically.
- **FR-015:** User can immediately and manually delete a Chat at any time.
- **FR-016:** User can download/save-as any output produced within a Chat; no filesystem browsing is provided. **Resolved scope (spec review, 2026-08-05):** "output" means the Chat's transcript/conversation content, exposed via a download control in the Chat UI — not a general artifact/attachment system. `internal/usecase/chat.Service.ExportConversation` (`internal/usecase/chat/export.go`) already implements this for the existing `Conversation` entity (JSON and Markdown formats, full message history) and should be reused/extended rather than rebuilt. This is consistent with the Non-Goals' explicit rejection of a Chats file browser (Chats stay directory-less).

### Projects

- **FR-017:** User can create a Project with a configured output directory.
- **FR-018:** Project has a hidden backend directory (not shown in the Project's file view) for agent-managed memory/context files, parallel to the hidden directories used by Chats, Jobs, and Chores.
- **FR-019:** Project's output directory may contain an `AGENTS.md` the agent uses to steer its work in that Project.
- **FR-020:** The primary path to edit `AGENTS.md` is by chatting with the agent in the Project's context; the agent makes the edit.
- **FR-021:** UI provides a subdued, secondary control to add an `AGENTS.md` to a Project that doesn't yet have one.
- **FR-022:** User retains direct filesystem access to the Project's output directory (can view/edit any file, including `AGENTS.md`, outside the app).
- **FR-023:** Project retention is configurable independently from Chat retention (including "Never").

### Jobs (one-time tasks)

- **FR-024:** User can create a Job with a Title and Description.
- **FR-025:** Job Description is persisted as `JOB-DESCRIPTION.md` in the Job's hidden backend directory.
- **FR-026:** A Job can run in the context of a Chat or a Project; if run in a Project's context, that Project's output directory is the Job's default working directory.
- **FR-027:** Jobs are queued and executed via the shared worker pool (FR-039) in FIFO order.
- **FR-028:** Every Job run records a processing log, results, and timing (start/end/duration).
- **FR-029:** Each Job has its own chat interface for conversing with the agent about that Job.
- **FR-030:** User receives an in-app notification when a Job run completes.

### Chores (repeated jobs)

- **FR-031:** User can create a Chore with a Title, a Description (persisted the same way as a Job's `JOB-DESCRIPTION.md`), and an optional working directory.
- **FR-032:** Chore runs on a repeating, cron-style schedule.
- **FR-033:** Schedule can be set directly by the user, or proposed by the agent; an agent-proposed schedule requires explicit user confirmation before it takes effect.
- **FR-034:** Schedule UI offers common presets (daily, weekly, etc.) plus an advanced raw cron expression field.
- **FR-035:** If a Chore's next scheduled run comes due while its previous run is still executing, the new run is skipped and logged ("skipped — previous run still active"); the Chore resumes at its next scheduled time.
- **FR-036:** Every Chore run records a processing log, results, and timing — same as Jobs.
- **FR-037:** Each Chore has its own chat interface for conversation/edit requests, same as Jobs.
- **FR-038:** User receives an in-app notification when a Chore run completes.

### Execution

- **FR-039:** Job and Chore execution is served by a configurable worker pool (N concurrent workers, set in Settings); one shared FIFO queue feeds runs to available workers, up to N concurrent executions system-wide, across all users.

### History

- **FR-040:** History lists all of the user's own Job and Chore runs, with status, timing, and links to logs/results.
- **FR-041:** History supports filtering by Job/Chore, date range, and status.
- **FR-042:** Each run in History has a chat interface where the user can ask about that specific run, using the run's log/results as context.
- **FR-043:** History retention is configurable independently from Chat and Project retention (including "Never").
- **FR-044:** An in-app notification indicator (e.g. badge on the History nav item) shows completed runs since last viewed, and clears once viewed.

### Memories

- **FR-045:** Memories provides a read-only browse/search view over the agent-maintained memory store (`internal/domain/memoryv2` cells/scenes).
- **FR-046:** Users cannot directly create, edit, or delete memory entries via the UI.
- **FR-047:** Memories includes a chat interface where the user can discuss/ask about/request edits to memory entries; the agent performs any resulting changes as the sole writer.

## Non-Functional Requirements

- **Performance:** List views (Chats, Projects, History, Memories) load within typical web-admin latency, consistent with the existing dashboard. Job/Chore run status, logs, and notification badges update in near-real-time while a run is active via WebSocket (`gorilla/websocket`), without a manual page refresh.
- **Reliability:** Run state (queue position, in-flight runs, Chore schedules, next-fire times) is persisted, not held only in memory — a server restart must not lose queued Jobs, drop an in-flight run's record, or cause a Chore to miss its next scheduled fire time. New entities (Project, Job, Chore, run history) follow the same file-based, atomic-write persistence pattern already used for conversations and memory (`internal/infrastructure/storage`, `AtomicFileWriter`) rather than introducing a new persistence mechanism.
- **Security:** Per-user data isolation (FR-010) enforced at the data-access layer, not just hidden in the UI. Remote-access allowlist enforcement happens at the network/middleware layer, before authentication, fail-closed. Existing web admin protections (CSRF, forced password change, TLS, role middleware) extend to all new environments. Job/Chore filesystem access is sandboxed to the assigned Project's output directory — no path traversal outside it.
- **Observability:** Every Job/Chore run's full log is captured and durably stored, retrievable via History even after the run completes (subject to History retention, FR-043). Worker pool errors/crashes are recorded against the run, not silently dropped, and surfaced in History.
- **Design/Branding:** Layout may take inspiration from claude.ai's left-nav structure but must use a visually distinct color palette and keep Nuiman branding (`img/NuimanProfile.jpg`, `img/NuimanWebImage.jpg`) visibly present across the UI (nav/header, login screen, favicon).

## Edge Cases

**(Added, spec review 2026-08-05)** — boundary conditions and error paths that must be handled; each is a required test case, not a stretch goal:

1. **Chat auto-naming, empty/non-text first message (FR-012):** if the first message is empty, whitespace-only, or has no text content (e.g., an attachment/tool-call-only turn once such turns exist), the Chat name falls back to a timestamp-based default (e.g. `"Chat — 2026-08-05 14:32"`), never an empty string.
2. **Job/Chore referencing a deleted Project (FR-026/FR-031):** deleting a Project does not delete Jobs/Chores that reference it as context; their `ContextID`/working directory becomes stale. On next run attempt, the run fails immediately with a clear `Error` ("referenced Project no longer exists") rather than executing against a missing/wrong directory. The Job/Chore itself is not auto-deleted (its history remains inspectable).
3. **Deleting a Job/Chore while its run is active:** the definition (Job/Chore record) is soft-marked for deletion but the in-flight run is allowed to finish (to avoid killing a worker mid-write and corrupting run state); the record and its history are removed once the run reaches a terminal state. No new runs are enqueued for a Job/Chore pending deletion.
4. **Worker pool size N reduced below current running count (FR-004/FR-039):** in-flight runs are never pre-empted; the pool simply stops pulling new work until concurrent runs drop at or below the new N.
5. **WebSocket reconnect mid-run (Performance NFR):** on reconnect, the client does not attempt to replay a missed message stream. It performs a fresh state fetch (current run status + log tail) over the existing HTTP API, then resumes incremental push from the WebSocket. No server-side message replay buffer is required.
6. **Agent-proposed Chore schedule never confirmed (FR-033):** an unconfirmed, agent-proposed schedule does not fire. It is visibly flagged "pending confirmation" in the Chore's UI indefinitely until the user confirms or discards it; it does not silently expire.
7. **Retention sweep deletes a Run the user hasn't viewed (FR-043/FR-044):** the notification badge count is decremented when an unviewed completed run is deleted by a retention sweep, same as if the user had viewed it — the badge must never reference a run that no longer exists.
8. **Concurrent `AGENTS.md` edits (FR-020 vs. FR-022):** the agent (via chat) and the user (via direct filesystem access) can both write `AGENTS.md`. No locking is introduced across that boundary (the user's editor holds no lock the app could see); last write wins, consistent with normal filesystem semantics for a file also open in an external editor. This is a known, accepted tradeoff, not a defect.
9. **Job/Chore run whose Project output directory becomes inaccessible mid-run (e.g., deleted/unmounted externally):** the run fails with a descriptive `Error` recorded against it (Observability NFR); the worker does not crash or hang.
10. **Cross-user resource access by ID (FR-010, IDOR):** requesting another user's Chat/Project/Job/Chore/Run by ID (via URL manipulation) returns 404, not 403 — existence of another user's resource is never disclosed, including to admins.
11. **Empty vs. absent allowlist in remote-access mode (FR-007):** an *absent* allowlist means "allow all sources" (open remote access, admin's explicit choice). An *empty but present* allowlist (`allowlist: []`) means "deny all" (fail-closed) — this distinction must be explicit in config parsing and documented in `config.yaml` comments, since the two are easy to conflate.
12. **Chat retention vs. an actively-in-progress conversation (FR-014):** the retention window is measured from the Chat's last activity (`UpdatedAt`), not `CreatedAt` — an old Chat that is still being actively used is never auto-deleted mid-use.
13. **LLM/agent failure during a Job/Chore run (Observability NFR):** treated identically to any other run failure — `Status = Failed`, `Error` populated with the underlying provider error, full partial log preserved, surfaced in History. No silent retry (retry policy, if any, is out of scope for this spec).

## System Architecture

**Affected layers (Clean Architecture, per AGENTS.md):**

- **`internal/domain`**: New entities — `Project`, `Job`, `Chore`, `Run` (Job/Chore run record), plus extensions to `Conversation`/`ConversationRepository` (auto-naming, retention). New repository interfaces: `ProjectRepository`, `JobRepository`, `ChoreRepository`, `RunRepository`. New value objects: `RetentionPolicy` (supports "Never"), `Schedule` (cron expression + presets), `WorkerPoolConfig`.
- **`internal/usecase`**: New use cases for Chat/Project/Job/Chore/History/Memories CRUD and orchestration; a Job/Chore execution use case that enqueues onto the worker pool; a scheduler use case that evaluates Chore cron expressions and enqueues due runs; a notification use case for run-completion badges.
- **`internal/adapter/web`**: New handlers/routes for each of the six environments plus Settings; new WebSocket handler(s) for near-real-time run status/log/notification push; templates under `internal/adapter/web/templates/`; nav/left-rail UI following the claude.ai-inspired-but-distinct layout.
- **`internal/infrastructure`**: New scheduler/worker-pool infrastructure (no existing equivalent) — FIFO queue, N-worker pool, cron evaluation, crash-safe run-state persistence via `AtomicFileWriter`/`FileLock`. New file-based repositories for Project/Job/Chore/Run, modeled on `FileConversationRepository`. Extended use of `github.com/gorilla/websocket` beyond the Buzz/Nostr gateway (its first use inside the web admin) — new connection lifecycle, per-user isolation, and auth handling for this surface.
- **`internal/config`**: New config sections for network access mode/allowlist, per-user default retention values, and worker pool size N, following the existing `config.yaml` pattern (see `internal/config/config.go`).

**Key existing components to build on (verified in the repo during spec creation):**

- `internal/domain/conversation_repository.go` (`ConversationRepository` interface) + `internal/infrastructure/storage/file_conversation_repository.go` (`FileConversationRepository`) — per-user, file-based, atomic-write conversation store already tracking `UserID`, `CreatedAt`/`UpdatedAt`, message count, and last-message snippet. Chats extend this rather than introducing a new entity.
- `internal/infrastructure/storage/atomic_file_writer.go` — `AtomicFileWriter` (temp-file + rename atomic write) and `FileLock` (flock-based exclusive locking, already used to protect `users.json`/`bots.json`). New entities (Project, Job, Chore, Run) and the worker-pool's run-state persistence should use both primitives.
- `internal/domain/memoryv2` (`memory_cell.go`, `memory_scene.go`, `memory_cell_repository.go`, `memory_scene_repository.go`, `filter.go`) — backs the Memories environment's read-only browse/search.
- `internal/domain/skill.go` + `internal/config/skills_config.go` — backs Settings' Skills/Plugins surface (`SkillRepository`, `SkillsConfig`, `DefaultSkillsConfig()`).
- `internal/adapter/web/server.go` — existing `net/http` `ServeMux` routing pattern (`/admin/...` routes wrapped by `adminHandler`/`requireRole`/`requirePasswordChange` composition) that new environment routes should follow; `internal/adapter/web/middleware.go` (`requireRole(domain.Role)`, CSRF, login rate limiting) is the existing auth/session/role middleware new environments build on rather than a parallel auth system.
- `github.com/gorilla/websocket` (already a `go.mod` dependency) — currently only used by `internal/infrastructure/nostr/client.go` (Buzz gateway). This is its first use in `internal/adapter/web`; treat the WebSocket connection lifecycle, auth, and per-user channel isolation as net-new engineering, not a reused pattern.
- `internal/usecase/chat/export.go` (`Service.ExportConversation`) — existing conversation-transcript export (JSON/Markdown) covering the full message history of a `Conversation`; reuse/extend for FR-016 (Chat output download) rather than building new export logic. Verified in the repo during spec review (2026-08-05); not referenced in the original PRD.

**Largest net-new subsystem (flagged as highest risk — see Risks below):** the Job/Chore worker pool + scheduler. No existing scheduler, job queue, or run-persistence infrastructure exists in the codebase today.

**Architecture decisions resolved during spec review (2026-08-05)** — made now to keep the spec implementation-ready; each is detailed further in `architecture.md`'s "Architectural Decisions" section and may be refined, not reopened from scratch, during Phase 3 (Architecture):

- **Cron library:** use `github.com/robfig/cron/v3` (adds one new `go.mod` dependency) rather than an in-house parser. Rationale: it directly provides `Schedule.Next(time.Time) time.Time`, which FR-035 (skip-if-still-running) and the restart-durability NFR both need; it is the de facto standard Go cron library (stable, widely used, MIT-licensed, small dependency footprint), so building an in-house parser would be reinventing well-tested logic for no benefit given the project already accepts third-party deps (`gorilla/websocket`, etc.).
- **Worker-pool persistence:** `AtomicFileWriter` + `FileLock` (existing primitives) are sufficient — no write-ahead log is needed. Rationale: run/queue volume is bounded by a single-process, single-machine worker pool (not a distributed system); the existing atomic-rename + flock pattern already gives crash-safe, torn-write-free persistence for `users.json`/`bots.json`, and the same guarantee (a write either lands whole or not at all) is exactly what queue/run-state persistence needs. Revisit only if profiling shows write-frequency contention Phase 2 testing can't otherwise handle.
- **New usecase package structure:** per-environment packages under `internal/usecase/` (`project/`, `job/`, `chore/`, `history/`, `memories/`), matching the existing convention already used for `internal/usecase/chat/`, `user/`, `skill/`, `memory/`, `memoryv2/` — not a consolidated package. Verified against the current `internal/usecase/` layout during spec review.
- **WebSocket connection/subscription model:** one WebSocket connection per browser tab, subscribed server-side to a per-user broadcast channel (not per-run) — a user with multiple tabs open gets multiple connections, all fed from the same per-user channel; a run-status/log/notification event is published once per user and fanned out to every open connection for that user. Reconnect is client-driven (simple backoff) with a full state resync over the existing HTTP API on reconnect (see Edge Cases #5) — no server-side replay buffer.
- **Project directory sandboxing mechanism:** a new dedicated helper (indicatively `internal/infrastructure/fsguard` or similar — exact package name decided in `architecture.md`) providing a single `ResolveWithin(baseDir, relPath string) (string, error)` used by every Job/Chore/Project file operation. Verified during spec review: no existing reusable path-confinement helper exists — `internal/infrastructure/preprocess/sandbox.go` (`CommandSandbox`) sandboxes *command execution*, not filesystem path containment, and `internal/config.FetchSecurityConfig` guards SSRF (network targets), not local paths. This must be built net-new; treat as required, not optional, hardening work (Risks table).

## Scope of Changes

**New files (indicative, refined in architecture.md/tasks.md):**
- `internal/domain/project.go`, `job.go`, `chore.go`, `run.go`, `retention.go`, `schedule.go`
- `internal/domain/project_repository.go`, `job_repository.go`, `chore_repository.go`, `run_repository.go`
- `internal/infrastructure/storage/file_project_repository.go`, `file_job_repository.go`, `file_chore_repository.go`, `file_run_repository.go`
- `internal/infrastructure/scheduler/` (worker pool, FIFO queue, cron evaluator, crash-recovery)
- `internal/usecase/chat/`, `internal/usecase/project/`, `internal/usecase/job/`, `internal/usecase/chore/`, `internal/usecase/history/`, `internal/usecase/memories/` (or a consolidated package structure decided in plan.md)
- `internal/adapter/web/chats_handler.go`, `projects_handler.go`, `jobs_handler.go`, `chores_handler.go`, `history_handler.go`, `memories_handler.go`, `settings_handler.go`, `websocket_handler.go`
- `internal/adapter/web/templates/{chats,projects,jobs,chores,history,memories,settings}/*.html`
- `internal/config/network_config.go` (or additions to `config.go`) for access mode/allowlist/retention defaults/worker pool size

**Modified files:**
- `internal/domain/conversation_repository.go` / `conversation.go` (auto-naming, retention fields)
- `internal/infrastructure/storage/file_conversation_repository.go` (retention sweep)
- `internal/adapter/web/server.go` (route registration, WebSocket upgrade route)
- `internal/adapter/web/middleware.go` (network allowlist middleware, ahead of auth)
- `internal/config/config.go` (new config sections)
- `config.yaml` (documented new keys)
- `cmd/nuimanbot/main.go` (DI wiring for new repos/use cases/scheduler)

**Dependencies:** `github.com/gorilla/websocket` (existing, new usage surface). `github.com/robfig/cron/v3` — **new** third-party dependency, decided during spec review (see "Architecture decisions resolved during spec review" above and research.md Q1). No other new third-party dependencies anticipated.

**Also verified during spec review (2026-08-05):** `internal/usecase/chat/export.go` (`Service.ExportConversation`, formats `ExportFormatJSON`/`ExportFormatMarkdown`) already exports a `Conversation`'s full message history — reuse for FR-016 (Chat output download) rather than building new export logic.

## Breaking Changes

- **Config schema:** New required-or-defaulted `config.yaml` sections (network access mode, worker pool size, per-user default retention). Must ship with safe defaults so existing deployments upgrade without manual config edits (localhost-only, no allowlist, by default).
- **No breaking changes anticipated** to existing gateways (CLI, Telegram, Slack, Buzz), existing `ConversationRepository` consumers (additive fields only), or existing `/admin/*` routes.

## Success Criteria and Acceptance Criteria

All PRD acceptance criteria carry forward unchanged (see source PRD, "Acceptance Criteria" section, and `nuimanbot-extend-context-and-ui-PRD.md` co-located in this directory). Summarized by environment:

- **Chats:** create/auto-name/persist/retain("Never" supported)/manually delete/download output.
- **Projects:** create with output dir; hidden backend dir never browsable; `AGENTS.md` edited via chat or "Add AGENTS.md" control; independent retention.
- **Jobs:** create with Title+Description → `JOB-DESCRIPTION.md`; Project-context Jobs default working dir; FIFO queue with visible position; run captures log/results/timing; per-Job chat; completion notification.
- **Chores:** cron schedule with presets + raw cron; agent-proposed schedules require confirmation; overlapping-run skip+log; per-Chore chat; completion notification.
- **Execution/Reliability:** restart preserves queued Jobs, in-flight run records, and Chore next-fire times.
- **History:** lists/filters user's own runs; per-run chat grounded in run log/results; independent retention; notification badge clears on view.
- **Memories:** read-only view, no create/edit/delete UI controls; chat-driven edits reflected afterward.
- **Isolation/Security:** second user cannot see/access first user's resources via any UI path, including as admin; localhost-only mode refuses remote connections; allowlist rejects non-listed sources pre-auth.
- **Branding:** Nuiman marks visible; palette distinct from claude.ai.
- **Quality gates:** `go fmt`, `go vet`, `golangci-lint run`, `go test ./...`, `go build -o bin/nuimanbot ./cmd/nuimanbot` all pass.

## Risks and Mitigation

| Risk | Severity | Mitigation |
|---|---|---|
| Job/Chore/worker-pool subsystem is entirely net-new (no existing scheduler/queue/run-persistence) | Highest | Isolate in `internal/infrastructure/scheduler`; design crash-recovery and persistence first (research.md/architecture.md) before UI work; add dedicated integration tests for restart-durability. |
| Worker pool crash recovery — concurrency-sensitive state (queue, in-flight runs) across restarts | High | Use `AtomicFileWriter` + `FileLock` (existing primitives) for all run-state writes; write recovery-path tests explicitly, not as an afterthought. |
| Project directory sandboxing — path traversal via agent file access | High | Centralize path-resolution/validation in one place (e.g. a `SandboxedFS` helper) used by every Job/Chore/Project file operation; add adversarial path-traversal tests. |
| Remote-access allowlist bug could expose the app publicly | High | Enforce allowlist at middleware layer before auth, fail-closed; add explicit tests for allow/deny/malformed-entry cases. |
| Project/Job/Chore domain entities are net-new (no existing concept) | Medium | Model closely on the existing `Conversation`/`ConversationRepository` pattern for consistency; keep repository interfaces small per AGENTS.md guidance. |
| Chat domain entity | Low | Largely covered by extending existing `ConversationRepository` (auto-naming, retention, "Never" support) rather than building from scratch. |
| `gorilla/websocket` net-new use in web admin — connection lifecycle/per-user isolation | Medium | Treat as new engineering, not a reused pattern; scope explicitly in architecture.md; add per-user channel isolation tests. |
| Chat/run log storage growth under "Never" retention | Medium | Surface storage usage in Settings/UI so users understand the tradeoff (may be a stretch goal — confirm in plan.md). |
| Default retention values set too aggressively, causing unexpected data loss | Medium | Propose and document explicit defaults during planning (see Open Questions); require explicit confirmation before enabling any auto-delete period below a sane floor. |

## Timeline and Milestones

[TBD] — to be defined in `plan.md` (Phase Breakdown) and `tasks.md` once architecture is finalized. The Job/Chore/worker-pool subsystem is expected to dominate the critical path per the Dependencies/Risks table above.

## References

- Source PRD: `specs/260805-nuimanbot-extend-context-and-ui/nuimanbot-extend-context-and-ui-PRD.md`
- `internal/domain/conversation_repository.go`, `internal/infrastructure/storage/file_conversation_repository.go`
- `internal/infrastructure/storage/atomic_file_writer.go`
- `internal/domain/memoryv2/`
- `internal/domain/skill.go`, `internal/config/skills_config.go`
- `internal/adapter/web/server.go`, `middleware.go`
- `AGENTS.md` (Clean Architecture, TDD, quality gates, specs workflow)
