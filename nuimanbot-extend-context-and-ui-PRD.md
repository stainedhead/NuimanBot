# PRD: NuimanBot — Extend Context & UI (Chats, Projects, Jobs, Chores, History, Memories)

**Created:** 2026-08-04
**Jira:** N/A
**Status:** Draft
**Scope:** Extends the existing web admin (`internal/adapter/web`) with six new user-facing environments (Chats, Projects, Jobs, Chores, History, Memories), a Settings environment for skills/plugins/users/gateways/access/retention configuration, and configurable network exposure (localhost-only vs. remote with optional allowlisting).

---

## Problem Statement

NuimanBot's web admin today is scoped to operational management — bots, users, confirmations. There is no user-facing surface for actually working with the agent: no persistent conversation history, no durable project context the agent can read/write against, no way to run a one-off or recurring agent task and see its results later, and no browsable knowledge/memory store. Users who want ongoing agent work (multi-turn conversations, project-scoped coding/document work, scheduled recurring tasks, historical run inspection) have no home for it in the product. This PRD extends the existing web UI with six new environments — Chats, Projects, Jobs, Chores, History, Memories — plus a settings area for skills/plugins/users/gateways, and adds configurable network exposure (localhost-only vs. remote with optional allowlisting) so the expanded surface can be run safely outside a single trusted machine.

## Goals

- Give users a persistent, organized workspace for agent interaction — lightweight ephemeral Chats plus durable, directory-scoped Projects — instead of one-off, unstructured conversations.
- Enable autonomous agent work beyond live chat: one-time Jobs (FIFO-queued) and recurring Chores (cron-scheduled), each with full run history, logs, and a post-hoc chat interface for follow-up questions.
- Make the expanded surface safely deployable beyond a single trusted machine via configurable network exposure (localhost-only, or remote with optional IP/hostname allowlisting).

## Non-Goals

- Building a separate compiled desktop application — this extends the existing web UI, not a replacement runtime (considered and explicitly rejected in favor of staying on the web UI).
- Rebuilding the skills/plugins config system — the new Settings environment surfaces the existing `internal/domain/skill.go` / `skills_config.go` model, not a redesign of it.
- A general-purpose file browser/manager for Chats — Chats stay directory-less by design; only Projects expose a working directory.
- A general-purpose OS cron replacement — Chores schedule *agent-driven* tasks only, not arbitrary shell/system jobs.
- Changing or deprecating the existing gateways (CLI, Telegram, Slack, Buzz) — this is additive to the web UI; those remain as-is.
- Real-time multi-user collaborative editing of Project files (concurrent live co-editing).

## Functional Requirements

### Settings

Settings are split by scope: **system-wide/admin-only** settings apply to the whole deployment and can only be changed by an admin; **per-user** settings apply only to the logged-in user's own resources.

- **FR-001:** Settings environment surfaces configuration for Skills, Plugins, and Gateways (system-wide/admin-only — existing systems, not rebuilt) and Users (system-wide/admin-only user management).
- **FR-002:** Settings environment surfaces Network Access configuration (system-wide/admin-only; see Access / Network).
- **FR-003:** Settings environment surfaces retention configuration independently for Chats, Projects, and History (per-user); each supports a "Never" value (no auto-expiry) in addition to a configurable time period.
- **FR-004:** Settings environment surfaces the Job/Chore worker pool size (system-wide/admin-only — N concurrent workers is a process-level resource, not a per-user setting).

### Access / Network

- **FR-005:** Web server supports localhost-only mode (binds 127.0.0.1 only).
- **FR-006:** Web server supports remote-access mode (binds a configured interface).
- **FR-007:** In remote-access mode, an optional IP/hostname allowlist may be configured; requests from non-allowlisted sources are rejected before reaching application handlers.
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
- **FR-016:** User can download/save-as any output produced within a Chat; no filesystem browsing is provided.

### Projects

- **FR-017:** User can create a Project with a configured output directory.
- **FR-018:** Project has a hidden backend directory (not shown in the Project's file view) for agent-managed memory/context files, parallel to the hidden directories used by Chats, Jobs, and Chores.
- **FR-019:** Project's output directory may contain an `AGENTS.md` file the agent uses to steer its work in that Project.
- **FR-020:** The primary path to edit `AGENTS.md` is by chatting with the agent in the Project's context; the agent makes the edit.
- **FR-021:** UI provides a subdued, secondary control to add an `AGENTS.md` file to a Project that doesn't yet have one.
- **FR-022:** User retains direct filesystem access to the Project's output directory (can view/edit any file, including `AGENTS.md`, outside the app).
- **FR-023:** Project retention is configurable independently from Chat retention (including "Never").

### Jobs (one-time tasks)

- **FR-024:** User can create a Job with a Title and Description.
- **FR-025:** Job Description is persisted as a `JOB-DESCRIPTION.md` file in the Job's hidden backend directory.
- **FR-026:** A Job can run in the context of a Chat or a Project; if run in a Project's context, that Project's output directory is the Job's default working directory.
- **FR-027:** Jobs are queued and executed via the shared worker pool (FR-039) in FIFO order.
- **FR-028:** Every Job run records a processing log, results, and timing (start/end/duration).
- **FR-029:** Each Job has its own chat interface for the user to converse with the agent about that Job (ask questions, request edits, get agentic support).
- **FR-030:** User receives an in-app notification when a Job run completes.

### Chores (repeated jobs)

- **FR-031:** User can create a Chore with a Title, a Description (persisted the same way as a Job's `JOB-DESCRIPTION.md`, in its hidden backend directory), and an optional working directory.
- **FR-032:** Chore runs on a repeating, cron-style schedule.
- **FR-033:** Schedule can be set directly by the user, or proposed by the agent on the user's behalf; an agent-proposed schedule requires explicit user confirmation before it takes effect.
- **FR-034:** Schedule UI offers common presets (e.g. daily, weekly) plus an advanced field for a raw cron expression.
- **FR-035:** If a Chore's next scheduled run comes due while its previous run is still executing, the new run is skipped and logged ("skipped — previous run still active"); the Chore resumes at its next scheduled time.
- **FR-036:** Every Chore run records a processing log, results, and timing — same as Jobs.
- **FR-037:** Each Chore has its own chat interface for conversation/edit requests, same as Jobs.
- **FR-038:** User receives an in-app notification when a Chore run completes.

### Execution

- **FR-039:** Job and Chore execution is served by a configurable worker pool (N concurrent workers, set in Settings); one shared FIFO queue feeds runs to available workers, up to N concurrent executions system-wide, across all users.

### History

- **FR-040:** History environment lists all of the user's own Job and Chore runs, with status, timing, and links to logs/results.
- **FR-041:** History supports filtering by Job/Chore, date range, and status.
- **FR-042:** Each run in History has a chat interface where the user can ask the agent questions about that specific run, using the run's log/results as context.
- **FR-043:** History retention is configurable independently from Chat and Project retention (including "Never").
- **FR-044:** An in-app notification indicator (e.g. a badge on the History nav item) shows completed runs since the user last viewed History, and clears once viewed.

### Memories

- **FR-045:** Memories environment provides a read-only browse/search view over the agent-maintained memory store (`internal/domain/memoryv2` cells/scenes).
- **FR-046:** Users cannot directly create, edit, or delete memory entries via the UI.
- **FR-047:** Memories environment includes a chat interface where the user can discuss, ask questions about, or request edits to memory entries; the agent performs any resulting changes as the sole writer.

## Non-Functional Requirements

- **Performance:** List views (Chats, Projects, History, Memories) load within typical web-admin latency, consistent with the existing dashboard. Job/Chore run status, logs, and notification badges update in near-real-time while a run is active via a WebSocket connection (`gorilla/websocket`), without requiring a manual page refresh.
- **Reliability:** Run state (queue position, in-flight runs, Chore schedules, next-fire times) is persisted, not held only in memory — a server restart must not lose queued Jobs, drop an in-flight run's record, or cause a Chore to miss its next scheduled fire time. New entities (Project, Job, Chore, run history) should follow the same file-based, atomic-write persistence pattern already used for conversations and memory (`internal/infrastructure/storage`, `AtomicFileWriter`) rather than introducing a new persistence mechanism.
- **Security:** Per-user data isolation (FR-010) is enforced at the data-access layer, not just hidden in the UI. Remote-access IP/hostname allowlist enforcement happens at the network/middleware layer, before authentication, fail-closed. Existing web admin protections (CSRF, forced password change, TLS, role middleware) extend to all new environments. Job/Chore filesystem access is sandboxed to the assigned Project's output directory — no path traversal outside it.
- **Observability:** Every Job/Chore run's full log is captured and durably stored, retrievable via History even after the run completes (subject to History retention settings, FR-043). Worker pool errors/crashes are recorded against the run, not silently dropped, and surfaced in History.
- **Design/Branding:** Layout may take inspiration from claude.ai's left-nav structure, but must use a visually distinct color palette and must keep Nuiman branding (existing marks in `img/NuimanProfile.jpg`, `img/NuimanWebImage.jpg`) visibly present across the UI (nav/header, login screen, favicon).

## Acceptance Criteria

- [ ] Create a Chat, send messages, and see it auto-named from the first message's text.
- [ ] Configure a Chat retention period; Chats older than it are deleted automatically, no manual action required.
- [ ] Manually delete a Chat immediately; it's gone and unrecoverable.
- [ ] Download/save-as any output produced during a Chat, from the Chat UI.
- [ ] Create a Project with an output directory; files placed there are usable by the agent as context.
- [ ] Project's hidden backend directory is never visible/browsable in the Project's file view.
- [ ] Request an `AGENTS.md` change via Project chat; agent applies the edit to the file.
- [ ] "Add AGENTS.md" control appears (subdued) only when a Project lacks the file; using it creates it.
- [ ] Each of Chats, Projects, and History has its own independently configurable retention setting in Settings, and each supports "Never" as a value (no auto-expiry).
- [ ] Create a Job with Title + Description; `JOB-DESCRIPTION.md` exists in its hidden directory with that content.
- [ ] Creating a Job inside a Project defaults its working directory to that Project's output directory.
- [ ] When the worker pool is at capacity, a new Job queues, shows its status/position, and runs FIFO once a worker frees up.
- [ ] Completed Job run shows log, results, and start/end timing in History.
- [ ] Job's own chat interface answers a follow-up question referencing that Job's run.
- [ ] Create a Chore with a cron-style schedule; it fires automatically without manual intervention.
- [ ] Chore schedule UI offers common presets plus an advanced raw-cron field for custom schedules.
- [ ] When the agent proposes a Chore schedule via chat, it does not take effect until the user explicitly confirms it.
- [ ] A Chore run still executing when its next scheduled time arrives is skipped and logged ("skipped — previous run still active").
- [ ] Restarting the app preserves queued Jobs, in-flight run records, and each Chore's next scheduled fire time.
- [ ] Completing a Job or Chore run surfaces an in-app notification/badge (e.g. on the History nav item) that clears once viewed.
- [ ] History lists the current user's Job/Chore runs, filterable by Job/Chore, date range, and status.
- [ ] A run's chat in History answers questions grounded in that run's log/results.
- [ ] Memories view shows memoryv2 entries read-only, with no create/edit/delete controls.
- [ ] Asking the Memories chat to correct/add an entry results in the agent updating the store, reflected afterward in the read-only view.
- [ ] A second user account cannot see or access the first user's Chats/Projects/Jobs/Chores/History via any UI path, including as an admin.
- [ ] Localhost-only mode refuses remote connection attempts outright.
- [ ] Remote-access mode with an allowlist rejects non-listed IPs/hostnames (e.g. 403) before authentication runs; allowlisted sources succeed.
- [ ] Visual review confirms Nuiman branding is present and the palette is distinct from claude.ai's.
- [ ] `go fmt`, `go vet`, `golangci-lint run`, `go test ./...`, and `go build -o bin/nuimanbot ./cmd/nuimanbot` all pass.

## Dependencies and Risks

| Item | Type | Notes |
|---|---|---|
| `internal/domain/memoryv2` (cells/scenes) | Dependency | Memories environment reads/searches this; must support the query patterns the UI needs. |
| `internal/domain/skill.go`, `skills_config.go` | Dependency | Settings environment surfaces existing skills/plugins config, doesn't rebuild it. |
| `internal/adapter/web` auth/session/CSRF/role middleware | Dependency | New environments build on existing web admin auth, not a parallel auth system. |
| `domain.Conversation`, `ConversationRepository`, `FileConversationRepository` (`internal/infrastructure/storage/file_conversation_repository.go`) | Dependency | Existing per-user, file-based, restart-durable conversation storage (already tracks `UserID`, `CreatedAt`/`UpdatedAt`, message count, last-message snippet). Chats should extend this rather than introduce a new entity — directly reduces the "Chat domain entities" risk below. |
| `internal/infrastructure/storage` (`AtomicFileWriter`) | Dependency | Existing atomic file-write primitive already used by conversation and memory repositories; new entities (Project, Job, Chore, run history) should persist through the same mechanism to satisfy the Reliability NFR without inventing a new storage layer. |
| `github.com/gorilla/websocket` (already in `go.mod`) | Dependency | Currently used only by the Buzz/Nostr gateway (`internal/infrastructure/nostr/client.go`); this PRD adds its first use in the web admin, for near-real-time run status/log/notification updates. New connection-lifecycle and per-user-isolation handling for this surface is net-new work, not a reused pattern. |
| Job/Chore/worker-pool subsystem | Risk (highest) | No existing scheduler, job queue, or run-persistence infrastructure — this is the largest net-new subsystem and the one most likely to blow the estimate. |
| Chat domain entity | Risk (low) | Largely covered by the existing `ConversationRepository` — extending it (auto-naming, retention, "Never" support) is materially lower-risk than building from scratch. |
| Project/Job/Chore domain entities | Risk | No existing "Project", "Job", or "Chore" concept in the domain layer — these need new entities, repositories, and hidden-directory conventions built from scratch (can follow the `AtomicFileWriter` pattern above, but the domain model itself is net-new). |
| Worker pool crash recovery | Risk | Persisting queue state and in-flight runs correctly across restarts is concurrency-sensitive; needs deliberate design, not an afterthought. |
| Project directory sandboxing | Risk | Job/Chore filesystem access must not escape the assigned Project directory — path traversal is a real security risk given agent file access. |
| Remote-access allowlist | Risk | A bug here could expose the app publicly; needs thorough testing before remote mode ships as usable. |
| Chat/run log storage growth | Risk | Per-type "Never" retention (FR-003) makes unbounded growth an explicit, user-selectable outcome, not just an oversight — storage usage should be monitored/surfaced so users understand the tradeoff. |
| Default retention values | Risk | If the default auto-delete window for Chats/Projects/History is set too aggressively, users could lose data unexpectedly before they think to configure it. |

## Open Questions

- **Resolved:** Settings scope is split by nature — system-wide/admin-only (worker pool size N, network access mode/allowlist, Gateway integrations, global Users management, Skills/Plugins enablement) vs. per-user (Chat/Project/History retention, notification preferences). See FR-001–FR-004.
- **Resolved:** Near-real-time updates for run status, logs, and notification badges use WebSockets (`gorilla/websocket`, already a `go.mod` dependency currently used only by the Buzz/Nostr gateway) rather than polling. This is a new connection-lifecycle/auth surface for the web admin and should be scoped explicitly in the spec's architecture.md.
- Default numeric values for Chat/Project/History retention and default worker pool size N are not pinned in this PRD; propose sensible defaults during spec/implementation planning.
- No explicit storage/disk quota policy is defined for Chat/Job/Chore logs and Project files given "Never" retention is a valid choice; may warrant a follow-up PRD if usage grows unmanageable.
