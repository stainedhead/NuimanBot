# Data Dictionary: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05

## Purpose

Defines the domain entities, value objects, interfaces, enumerations, and API request/response shapes implied by the PRD's 47 functional requirements. This is a working document — refine field names/types once `architecture.md` and TDD test-writing settle the exact contracts. All entities below belong in `internal/domain` per AGENTS.md's Clean Architecture rule (no external imports except stdlib).

## Entities

### Conversation (existing — extend, do not replace)

Source: `internal/domain/conversation.go` / `conversation_repository.go`. Already has `UserID`, `CreatedAt`/`UpdatedAt`, message count, last-message snippet. Extensions needed for Chats (FR-011–FR-016):
- `Name` (string) — auto-derived from first message text (FR-012); falls back to a timestamp-based default when the first message is empty/whitespace-only/non-text (Edge Case #1, spec.md).
- `RetentionPolicy` (see Value Objects) — per-Chat or inherited from user's Chat retention setting; window measured from `UpdatedAt` (last activity), not `CreatedAt`, so an old but still-active Chat is never auto-deleted mid-use (Edge Case #12, spec.md).
- Output download (FR-016) reuses `internal/usecase/chat.Service.ExportConversation` (JSON/Markdown) — found during spec review, see spec.md's "Key existing components to build on."

### Project (net-new)

Backs FR-017–FR-023.
- `ID` (string)
- `OwnerUserID` (string) — FR-009/FR-010 ownership
- `Name` (string)
- `OutputDirectory` (string, absolute path) — FR-017
- `HiddenDirectory` (string, absolute path) — agent-managed memory/context files, not shown in file view (FR-018)
- `HasAgentsFile` (bool, derived) — whether `AGENTS.md` exists in `OutputDirectory` (FR-019, drives FR-021's "Add AGENTS.md" control)
- `RetentionPolicy` (value object)
- `CreatedAt`, `UpdatedAt` (time.Time)

### Job (net-new)

Backs FR-024–FR-030.
- `ID` (string)
- `OwnerUserID` (string)
- `Title` (string), `Description` (string) — persisted also as `JOB-DESCRIPTION.md` in `HiddenDirectory` (FR-025)
- `HiddenDirectory` (string, absolute path)
- `ContextType` (enum: `ChatContext` | `ProjectContext`) + `ContextID` (string) — FR-026
- `WorkingDirectory` (string, absolute path) — defaults to the Project's `OutputDirectory` when `ContextType == ProjectContext` (FR-026)
- `Status` (enum: see Enumerations)
- `QueuePosition` (int, transient/derived while queued)
- `CreatedAt`, `UpdatedAt` (time.Time)

### Chore (net-new)

Backs FR-031–FR-038.
- `ID` (string)
- `OwnerUserID` (string)
- `Title` (string), `Description` (string) — persisted as `JOB-DESCRIPTION.md`-equivalent in `HiddenDirectory` (FR-031)
- `HiddenDirectory` (string, absolute path)
- `WorkingDirectory` (string, absolute path, optional) — FR-031
- `Schedule` (value object) — FR-032/FR-034
- `ScheduleConfirmed` (bool) — for agent-proposed schedules pending user confirmation (FR-033); `false` means "pending confirmation" and the Chore does not fire (Edge Case #6, spec.md)
- `NextFireTime` (time.Time) — persisted, must survive restart (Reliability NFR)
- `Status` (enum: `Active` only) — **resolved during spec review (2026-08-05):** no pause/disable action is named in any FR; a Chore is Active (has a schedule) or deleted, no third state. See spec.md's Non-Goals ("Chore pause/disable").
- `CreatedAt`, `UpdatedAt` (time.Time)

### Run (net-new — Job run or Chore run)

Backs FR-028, FR-036, FR-040–FR-044, the Reliability and Observability NFRs.
- `ID` (string)
- `OwnerUserID` (string)
- `SourceType` (enum: `Job` | `Chore`) + `SourceID` (string)
- `Status` (enum: see Enumerations)
- `StartedAt`, `EndedAt` (*time.Time — nil until set)
- `Duration` (time.Duration, derived)
- `Log` (string or path to durably stored log content) — captured per Observability NFR
- `Results` (string, path to `RESULTS.md` in the Run's hidden directory) — **resolved during spec review (2026-08-05):** stored as a markdown file, parallel to `JOB-DESCRIPTION.md`'s pattern, rather than a structured/typed field — keeps Results human-readable, diffable, and consistent with the project's existing file-based persistence conventions. Content is the agent's final summary/output for the run.
- `SkipReason` (*string) — populated when a Chore run is skipped due to an in-flight previous run (FR-035), e.g. `"skipped — previous run still active"`
- `NotifiedAt` (*time.Time) — drives the History badge clear-on-view behavior (FR-044); also set (not left nil) when a retention sweep deletes an unviewed run, so the badge count never references a deleted run (Edge Case #7, spec.md)
- `Error` (*string) — worker pool crash/error recorded against the run, not silently dropped (Observability NFR)

### MemoryCell / MemoryScene (existing — read-only consumption)

Source: `internal/domain/memoryv2/memory_cell.go`, `memory_scene.go`. Memories environment (FR-045–FR-047) reads via existing `MemoryCellRepository`/`MemorySceneRepository` and `MemoryCellFilter`; no new persisted fields anticipated, but UI-facing DTOs/view-models will be needed in the adapter layer.

## Value Objects

- **`RetentionPolicy`**: `{ Period *time.Duration }` where `nil`/absent represents "Never" (no auto-expiry). Applies independently to Chats (FR-014), Projects (FR-023), History (FR-043). Consider a single shared type used by all three rather than three separate types.
- **`Schedule`**: `{ CronExpression string, Preset *string }` — raw cron expression is authoritative; `Preset` (e.g. `"daily"`, `"weekly"`) is a UI convenience that resolves to a `CronExpression` (FR-034).
- **`WorkerPoolConfig`**: `{ MaxConcurrentWorkers int }` — system-wide, admin-only (FR-004, FR-039).
- **`NetworkAccessConfig`**: `{ Mode enum(LocalhostOnly|Remote), BindAddress string, Allowlist []string }` — FR-005–FR-008. **Resolved during spec review (2026-08-05):** `Allowlist` must be a `[]string` with a nil/absent-vs-empty-vs-populated distinction preserved through config parsing (not collapsed to a single "empty slice" representation) — nil/absent means allow all sources; an explicitly empty list (`allowlist: []`) means deny all, fail-closed (Edge Case #11, spec.md). Use a pointer or an explicit "configured" flag if `internal/config`'s YAML parsing doesn't naturally preserve this distinction for `[]string`.

## Interfaces (Repository Contracts — domain layer)

Modeled on the existing `ConversationRepository` interface shape (`internal/domain/conversation_repository.go`):

- **`ProjectRepository`**: `SaveProject`, `GetProject`, `ListProjects(userID)`, `DeleteProject`.
- **`JobRepository`**: `SaveJob`, `GetJob`, `ListJobs(userID)`, `DeleteJob`, `UpdateStatus`.
- **`ChoreRepository`**: `SaveChore`, `GetChore`, `ListChores(userID)`, `DeleteChore`, `UpdateNextFireTime`.
- **`RunRepository`**: `SaveRun`, `GetRun`, `ListRuns(userID, filter)`, `AppendLog`.
- **`WorkerPool`** (usecase-layer interface, infrastructure-layer implementation): `Enqueue(run)`, `Status()`, `SetConcurrency(n)`.
- **`Scheduler`** (usecase-layer interface): `NextFireTime(schedule, after time.Time) time.Time`, `DueChores(now time.Time) []Chore`.

## Enumerations

- **`JobStatus`** / **`RunStatus`**: `Queued`, `Running`, `Completed`, `Failed`, `Skipped` (Chore-only, FR-035). **Resolved during spec review (2026-08-05):** no `Cancelled` state — no FR describes user-initiated cancellation of an in-flight run (see spec.md's Non-Goals, "User-initiated run cancellation"); once started, a run runs to `Completed`, `Failed`, or crashes (recorded as `Failed` with `Error` populated).
- **`ContextType`** (Job): `ChatContext`, `ProjectContext`.
- **`AccessMode`** (Network): `LocalhostOnly`, `Remote`.
- **`SettingsScope`**: `SystemWide`, `PerUser` — used to drive which settings the Settings UI shows/edits based on the current user's role.

## API Request/Response Types (adapter layer — indicative)

To be refined once `architecture.md` settles on REST-style form-POST (matching existing `/admin/*` handlers) vs. JSON API for WebSocket-driven views:

- `CreateChatRequest { FirstMessage string }` → `ChatSummaryResponse { ID, Name, CreatedAt }`
- `CreateProjectRequest { Name, OutputDirectory string }`
- `CreateJobRequest { Title, Description string, ContextType, ContextID string }`
- `CreateChoreRequest { Title, Description, WorkingDirectory string, Schedule ScheduleInput }`
- `RunStatusUpdate` (WebSocket push payload) `{ RunID, Status, Progress *string, LogTail []string }`
- `NotificationBadge` (WebSocket push payload) `{ UnviewedCount int }`

## Notes

Field lists above are a starting point for `data-dictionary.md`'s intended purpose (seed TDD test design and repository interface design) — expect refinement once `architecture.md` and the first Red-Green-Refactor cycles begin.
