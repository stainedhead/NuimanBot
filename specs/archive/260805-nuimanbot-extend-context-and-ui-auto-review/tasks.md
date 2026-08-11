# Tasks: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Status:** Planning
**Structure note:** Task grouping below is a **direct translation of the source PRD's own "Fix-Pass Execution Guidance" 7-workstream table** (workstreams A–G) into per-finding tasks — this is intentional and matches the pipeline's instruction not to invent a different structure. Each task = one finding. Task IDs are `<Workstream>.<sequence>` (e.g. `A.1`). Full acceptance criteria for each finding live in the source PRD; this file summarizes them and adds TDD-cycle sub-steps per the mandatory execution guidance.

## Progress Summary

**Correction (2026-08-05):** the original workstream table below summed to 18, not 19 — **FR-R7** (Memories `ownerUserID`→`ConversationID` mapping, P1) was present in the source PRD and `research.md`/`architecture.md` but never assigned to a workstream. Added here as **Task D.3**, grouped into Workstream D because it shares D's "trace/implement-or-defer + document" shape and is Memories-scoped like D.1's likely template target, not because it shares files with D.1/D.2 (it touches `internal/usecase/memories/service.go` and `internal/usecase/memoryv2/curator_service.go`, not the handler/template files D.1 touches).

**19/19 tasks complete** (5 P0 / 5, 9 P1 / 9, 5 P2 / 5) — all 7 workstreams done.

| Workstream | Tasks | Complete |
|---|---|---|
| A — Restart & retention sweeps | 3 | 3/3 |
| B — Filesystem layering & sandboxing | 4 | 4/4 |
| C — Chore parity | 1 | 1/1 |
| D — Deferred agent-facing surfaces | 3 | 3/3 |
| E — Live-update & observability plumbing | 2 | 2/2 |
| F — Settings & execution edge cases | 4 | 4/4 |
| G — Consistency polish | 2 | 2/2 |
| **Total** | **19** | **19/19** |

---

## Workstream A — Restart & retention sweeps

*All three add a startup-reconciliation or periodic-sweep loop wired into `cmd/nuimanbot`'s DI. Internal sequencing: **A.1 (FR-R3) → A.2 (FR-R9) → A.3 (FR-R2)**, though A.3 can proceed in parallel with A.1/A.2 provided the shared `cmd/nuimanbot` wiring file is coordinated. A.2 additionally depends on Workstream C landing first (see Task C.1).*

### Task A.1 — FR-R3: Wire retention sweep (P0)
- **Dependencies:** none (builds the sweep scaffold other tasks extend)
- **Files:** `chats.Service`, `projects.Service`, `history.Service`, `cmd/nuimanbot/main.go`/`extended_context.go`
- **Red:** integration test proving an expired Chat/Project/Run created via the real repository is *not* deleted before the fix (sweep never runs).
- **Green:** periodic sweep (ticker goroutine, `ChoreScheduler` poll-loop pattern) calls all three `SweepExpired` methods for every user on a defined interval, wired in `cmd/nuimanbot`'s DI.
- **Refactor:** extract shared per-user sweep-iteration logic if duplicated across the three calls.
- **Acceptance criteria:** integration test proves expired data is actually gone after a sweep cycle against real repositories (not just the use-case method's isolated return value); Edge Case #7 (sweep deleting an unviewed Run decrements the notification badge, never references a deleted Run) covered by a test once wired to a live `RunRepository`/`Hub`.

### Task A.2 — FR-R9: Cleanup sweep for `PendingDeletion` Jobs/Chores (P1)
- **Dependencies:** A.1 (extends the same sweep loop); **C.1 must merge first** (assumes Chores already soft-delete correctly)
- **Files:** `jobs/service.go`, `chores/service.go`, sweep loop from A.1
- **Red:** test proving a `PendingDeletion` Job/Chore whose Run has reached a terminal state is still present after a sweep cycle (pre-fix).
- **Green:** sweep (combined with A.1's loop) checks `PendingDeletion` Jobs/Chores whose associated Run has reached a terminal state and hard-deletes them.
- **Refactor:** ensure the combined sweep loop doesn't duplicate per-user iteration logic between the retention and cleanup checks.
- **Acceptance criteria:** test proves a `PendingDeletion` Job/Chore is actually removed once its run completes, not just that the flag was set correctly at delete time.

### Task A.3 — FR-R2: Restart-recovery for in-flight Runs (P0)
- **Dependencies:** none strictly, but coordinate with A.1/A.2 on shared `cmd/nuimanbot` wiring file
- **Files:** `internal/infrastructure/scheduler/queue.go`, `pool.go`, `cmd/nuimanbot/*.go`
- **Red:** crash-simulation test — persist a Run at `Running`, construct a fresh `WorkerPool`/`Queue`/reconciliation path against the same storage directory (no live in-memory state carried over), assert the Run is still stuck at `Running` (pre-fix).
- **Green:** on startup, any Run in a non-terminal state (`Running`, or `Queued` with no matching queue entry) is reconciled — re-enqueued if idempotent-safe, else transitioned to `Failed` with `Error = "run interrupted by server restart"`.
- **Refactor:** extract the reconciliation scan into a clearly-named startup step, distinct from queue restoration.
- **Acceptance criteria:** same crash-simulation test now passes; History surfaces restart-interrupted runs distinguishably via the `Error` field.

---

## Workstream B — Filesystem layering & sandboxing

*FR-R5/FR-R6/FR-R18 all touch `fsguard` call sites or the usecase/infrastructure boundary. Internal sequencing: **B.1 (FR-R5) → B.2 (FR-R6)** — don't fix symlink escape twice. B.3 (FR-R18) can start in parallel but must rebase against B.1 before merging (shared file: `projects/service.go`). B.4 (FR-R13) is fully independent.*

### Task B.1 — FR-R5: Relocate `fsguard`/`os` calls out of usecase layer (P1)
- **Dependencies:** none (must land before B.2)
- **Files:** `jobs/service.go`, `chores/service.go`, `projects/service.go`, new domain interface + infrastructure implementation
- **Red:** a test (or lint/import-check) demonstrating `jobs`/`chores`/`projects` currently import `internal/infrastructure/fsguard` and `os` directly.
- **Green:** define a domain-level interface (e.g. `domain.HiddenFileWriter`/`ConfinedFileStore`, or extend existing repository interfaces — see `data-dictionary.md`); implement it in `internal/infrastructure/storage` (or a new small package) using `fsguard.ResolveWithin` + `os`; `jobs`/`chores`/`projects` depend only on the new interface.
- **Refactor:** ensure no `os` or `fsguard` import remains in any of the three `service.go` files; simplify call sites now that I/O is behind the interface.
- **Acceptance criteria:** existing tests continue to pass against a fake implementation of the new interface; zero `os`/`fsguard` imports remain in the three usecase `service.go` files.

### Task B.2 — FR-R6: Symlink-escape mitigation for `fsguard.ResolveWithin` (P0)
- **Dependencies:** B.1 (fix at the relocated call sites, not the pre-relocation ones)
- **Files:** `fsguard.go` + call sites relocated by B.1 (`file_project_repository.go`, live `Executor`, `jobs`/`chores`/`projects` via the new interface)
- **Red:** adversarial test creating a symlink inside a confined base directory pointing outside it, showing `ResolveWithin` currently returns a path that, when opened, escapes the sandbox.
- **Green:** close the gap via `filepath.EvalSymlinks` (or equivalent guard) on resolved paths before use, at minimum at every Project-output-directory call site.
- **Refactor:** consider centralizing the `EvalSymlinks` guard inside the B.1 interface implementation so future call sites inherit it automatically.
- **Acceptance criteria:** the adversarial test now passes at the fixed call sites; positive-path test confirms legitimate in-root symlink-free access still works.
- **Review note:** second reviewer pass required (security-sensitive).

### Task B.3 — FR-R18: Confine Project output directory to an allowed root (P0)
- **Dependencies:** rebase against B.1 before merge (shared file `projects/service.go`); may start in parallel
- **Files:** `internal/usecase/projects/service.go`, `server.go` route registration (verify/confirm role gating)
- **Red:** two tests — relative-traversal-escape and absolute-path-outside-root — both currently succeed in creating a Project outside the allowed root (pre-fix).
- **Green:** `CreateProject` validates `outputDirectory` against a configured allowed root (default: `<storagePath>/users/<ownerUserID>/projects/`, per `research.md` Q4) using `fsguard.ResolveWithin` (or equivalent) on the directory itself.
- **Refactor:** factor the allowed-root check into a reusable validator if the pattern is likely to recur.
- **Acceptance criteria:** both traversal and absolute-path-outside-root tests now assert rejection with a clear validation error; positive-path test confirms legitimate in-root paths still work.
- **Review note:** second reviewer pass required (security-sensitive, P0).

### Task B.4 — FR-R13: Route `FileConversationRepository` through `fsguard` (P2)
- **Dependencies:** none (fully independent)
- **Files:** `file_conversation_repository.go`
- **Red:** test (or inspection-backed assertion) confirming current path-building bypasses `fsguard.ResolveWithin`.
- **Green:** `file_conversation_repository.go`'s path-building helpers route through `fsguard.ResolveWithin`, matching the four new repositories' pattern.
- **Refactor:** align helper naming/structure with the four new repositories for consistency.
- **Acceptance criteria:** existing conversation-repository tests continue to pass unmodified (behavior for well-formed IDs unchanged).

---

## Workstream C — Chore parity

*Single finding; must merge before Workstream A's Task A.2 (FR-R9).*

### Task C.1 — FR-R8: `chores.Service.DeleteChore` soft-delete (P1)
- **Dependencies:** none; **blocks A.2**
- **Files:** `chores/service.go`
- **Red:** test mirroring `jobs`' existing soft-delete test coverage, currently failing against `DeleteChore`'s plain immediate-delete behavior.
- **Green:** `DeleteChore` checks the Chore's current run status the same way `DeleteJob` does, soft-marks `PendingDeletion` when a run is active, defers the hard delete.
- **Refactor:** remove the `TODO(soft-delete)` comment; consider extracting shared soft-delete logic between `Job`/`Chore` if duplication is significant.
- **Acceptance criteria:** test mirrors `jobs`' existing soft-delete coverage for the Chore path; TODO comment removed (or replaced with the same "sweep integration deferred" comment `jobs/service.go` carries, for consistency with Task A.2 landing after this one).

---

## Workstream D — Deferred agent-facing surfaces

*Both are "implement, or explicitly document the deferral + UI notice" calls. Independent of every other workstream. Ideally the same teammate, since the fallback documentation path is identical for both.*

### Task D.1 — FR-R4: Per-item chat interface template (P0)
- **Dependencies:** none
- **Files:** `jobs_handler.go`, `history_handler.go`, `memories_handler.go`, Chore handler equivalent, associated templates
- **Red:** test asserting no chat affordance exists yet for at least one of Job/Chore/Run/Memory detail pages.
- **Green:** implement a minimal per-item chat interface for **at least one** of the four (Job, Chore, Run, Memory), grounded in that item's own context (description/log/results/memory content), as a template. **No documentation-only fallback accepted for this finding** — at least one must actually be implemented.
- **Refactor:** structure the implementation so the remaining three can follow the same template in a fast-follow pass.
- **Acceptance criteria:** at least one item type has a working per-item chat; the other three detail pages, if not yet live, explicitly say "chat not yet available" rather than silently omitting the affordance.

### Task D.2 — FR-R1: Web Chats agent invocation (P1)
- **Dependencies:** none; may share a teammate with D.1
- **Files:** `internal/usecase/chats/service.go`, `internal/adapter/web/chats_handler.go`, `internal/usecase/chat` (existing orchestration engine)
- **Red:** test asserting a message sent via web Chats produces no assistant reply (pre-fix, documents current behavior).
- **Green (preferred path):** sending a message in the web Chats UI results in an assistant reply appended to the conversation, using the existing `internal/usecase/chat` orchestration (or an explicitly documented equivalent).
- **Green (fallback path, only if genuinely blocked):** `implementation-notes.md`'s "Deviations from Plan" explicitly documents the deferral (same treatment as the Job/Chore `Executor` deferral), and the Chats UI visibly communicates to the user that replies aren't yet available.
- **Refactor:** N/A if fallback taken; otherwise align the new invocation path's error handling/observability with other gateways.
- **Acceptance criteria:** either a working reply loop, or an explicit, non-silent documented deferral + UI notice — the choice must be recorded, not left silent as it is today.

### Task D.3 — FR-R7: Memories `ownerUserID → ConversationID` mapping (P1)
- **Dependencies:** none; added late (see Progress Summary correction above) — orphaned from the original workstream table
- **Files:** `internal/usecase/memories/service.go`, `internal/usecase/memoryv2/curator_service.go`, whichever gateway is traced (CLI or Telegram)
- **Red:** integration test tracing a message through the real curator path for one gateway and asserting what `ConversationID` a persisted `MemoryCell` actually carries.
- **Green:** if the traced value is a stable per-user identifier, the current `ownerUserID`-as-filter pass-through is confirmed correct — remove the "ASSUMPTION, needs reviewer confirmation" doc comment and replace it with the confirmed mapping rationale. If it's a per-session/conversation UUID instead, replace `conversationIDFor`'s pass-through with a real mapping.
- **Refactor:** N/A if confirmed as-is; otherwise align the new mapping lookup with existing repository patterns.
- **Acceptance criteria:** a test creates a memory cell via the real curator path and asserts it's visible via `memories.Service.ListCells` for that same user — not just the existing unit test's synthetic `ConversationID = username` fixture; the self-flagged doc-comment ambiguity in `memories/service.go` is resolved one way or the other, not left standing.

---

## Workstream E — Live-update & observability plumbing

*Both wire an already-correct backend computation through to the browser/every page. Independent of every other workstream; coordinate only if both touch `server.go`'s `BaseData` construction in the same edit window.*

### Task E.1 — FR-R10: Browser-side WebSocket consumer (P1)
- **Dependencies:** none
- **Files:** `internal/adapter/web/static/`, Job/Chore/History detail page templates
- **Red:** confirm (via grep-based test or manual check) zero client-side JS currently opens a WebSocket connection.
- **Green:** minimal client-side script (vanilla JS) on Job/Chore/History detail pages opens the WebSocket connection; on `run_status`/`run_log`/`notification_badge` events, updates the relevant DOM elements without a page reload.
- **Refactor:** factor shared connection/reconnect logic if duplicated across the three page types.
- **Acceptance criteria:** manually verified (or browser-level test if tooling exists) that starting a Run and watching its detail page shows status transition and log growth live.

### Task E.2 — FR-R19: Notification badge on every page (P1)
- **Dependencies:** none; coordinate with E.1 if both edit `BaseData` construction concurrently
- **Files:** `server.go` (`BaseData`, `NewBaseData`, `withUnviewedRunCount`)
- **Red:** test rendering a non-History page after an unviewed completed run exists, asserting the badge count is currently zero (pre-fix).
- **Green:** `UnviewedRunCount` populated for every authenticated page render (centralized in the shared "build BaseData for an authenticated request" path, or composed into every environment handler).
- **Refactor:** prefer centralizing over per-handler duplication if a shared construction path exists.
- **Acceptance criteria:** same test now asserts the badge count appears; navigating to Dashboard, Chats, Jobs, Projects, etc. after a run completes shows the badge without visiting History first.

---

## Workstream F — Settings & execution edge cases — COMPLETE (2026-08-05)

*Narrow, independent fixes with no file overlap with each other or other workstreams. Fully parallelizable.*

### Task F.1 — FR-R11: Settings network-access UI honesty (P1) — DONE
- **Dependencies:** none
- **Files:** `settings_handler.go`
- **Default scope (per `research.md` Q3):** no live listener rebind this pass — only stop presenting config-file-only fields as live-editable.
- **Red:** test/inspection confirming allowlist and bind-address fields are currently presented as editable despite being config-file-only.
- **Green:** either extend the POST handler to accept+apply allowlist/bind-address changes (larger change, only if scope decision changes), or grey out/disable those fields with an explanatory note.
- **Refactor:** N/A beyond template cleanup.
- **Acceptance criteria:** Settings UI clearly indicates which fields are config-file-only rather than presenting `NetworkMode` as live while allowlist/bind-address are silently read-only.
- **Outcome:** `settings_handler_test.go` gained `TestHandleSettings_NetworkFieldsIndicateConfigFileOnly` (Red, confirmed failing on the missing bind-address display before the fix). `settings.html` now renders a disabled Bind address input labeled "(config file only)", an explicit "(config file only)" label on the Allowlist block, and a note under Network access mode clarifying it only affects the allowlist-check middleware, not the actual listener bind address.

### Task F.2 — FR-R12: Executor fails cleanly against a deleted Project (P1) — DONE
- **Dependencies:** none
- **Files:** `StubExecutor.Execute` (or whichever `Executor` is live)
- **Red:** test creating a Job/Chore against a Project, deleting the Project, triggering a run — currently completes successfully instead of failing (pre-fix).
- **Green:** `Executor` checks Project existence via `ProjectRepository` when context indicates a Project-scoped run, before proceeding; fails the Run with `Error = "referenced Project no longer exists"` per Edge Case #2's exact wording if missing.
- **Refactor:** N/A beyond ensuring the check is reusable if multiple `Executor` implementations exist later.
- **Acceptance criteria:** same test now asserts the Run ends `Failed` with the specified error message rather than `Completed`.
- **Outcome:** `StubExecutor` gained `jobs domain.JobRepository`/`chores domain.ChoreRepository`/`projects domain.ProjectRepository` constructor params (compile-red until wired, then green). Before writing results, `Execute` calls `checkProjectExists`, which resolves the source Job/Chore's `ContextType`/`ContextID` and — only when `ContextType == ContextTypeProject` — calls `ProjectRepository.GetProject`; any error fails the Run with `Error = "referenced Project no longer exists"`. A source record that can't itself be resolved (missing Job/Chore) is treated as "no context to check", not a failure — that's a distinct problem outside this check's scope. New tests: `TestStubExecutor_FailsWhenJobsProjectDeleted` (asserts Failed + exact message), `TestStubExecutor_NoProjectContextCompletesNormally` (a Chat-context Job is unaffected). `cmd/nuimanbot/extended_context.go`'s `NewStubExecutor` call updated to pass `app.JobRepo`/`app.ChoreRepo`/`app.ProjectRepo`.

### Task F.3 — FR-R14: `Job.IsQueueable()` dead code decision (P2) — DONE
- **Dependencies:** none
- **Files:** `internal/domain/job.go`, `jobs/service.go`, `pool.go`
- **Red:** N/A (dead-code finding, not a behavior gap) — confirm via grep that no non-test caller exists.
- **Green:** either wire `IsQueueable()` into the actual enqueue path where its guard logic is meant to apply, or remove it and its tests if an equivalent check is intentionally implemented elsewhere.
- **Refactor:** N/A.
- **Acceptance criteria:** a deliberate decision is made and reflected in code (no orphaned dead code remains either way).
- **Outcome:** removed. Confirmed via `grep -rn "IsQueueable"` that only `job.go`'s own definition and `job_test.go`'s 4 dedicated tests referenced it — zero callers in `jobs/service.go` or `pool.go`. Decision: the guard has no home to be wired into, because there is no re-run/re-enqueue path anywhere in the codebase today — `CreateJob` (the only place that ever enqueues) always builds a brand-new Job, which is queueable by construction, so the invariant `IsQueueable` encoded was never actually at risk of being violated by any existing caller. Removed `IsQueueable()` from `job.go` and deleted `job_test.go` (its sole contents). Documented the decision and the "reintroduce when a re-run flow exists" guidance in a comment in `jobs/service.go` next to `DeleteJob`'s existing PendingDeletion TODO.

### Task F.4 — FR-R15: Worker pool size upper bound (P2) — DONE
- **Dependencies:** none
- **Files:** `domain.WorkerPoolConfig.Validate()`, `settings_handler.go`
- **Red:** test asserting an arbitrarily large `worker_pool_size` is currently accepted.
- **Green:** add a sane upper bound (documented constant, or a multiple of `runtime.NumCPU()`) to `Validate()`, surfaced as a rejected/flash-error value in the Settings form the same way `<= 0` already is.
- **Refactor:** N/A.
- **Acceptance criteria:** same test now asserts rejection above the bound; existing valid-range tests still pass.
- **Outcome:** added `domain.MaxWorkerPoolSize = 256` (a documented, generous fixed cap — chosen to catch a fat-fingered input mistake, not to tune production throughput) and a `> MaxWorkerPoolSize` branch in `WorkerPoolConfig.Validate()`. `worker_pool_config_test.go` gained cases at/above the bound. `settings_handler.go`'s POST handler now has a 3-way switch (`<= 0` / `> MaxWorkerPoolSize` / valid) instead of a 2-way if/else, surfacing a flash error mentioning the bound; `TestHandleSettings_AdminWorkerPoolSizeAboveUpperBoundRejected` added.

---

## Workstream G — Consistency polish — COMPLETE (2026-08-05)

*Template/adapter-layer polish; no security or data-integrity stakes. Fully parallelizable; lowest scheduling priority.*

### Task G.1 — FR-R16: Legacy admin pages get the new nav sidebar (P2) — DONE
- **Dependencies:** none
- **Files:** `dashboard.html`, `bots.html`, `users.html`, `confirmations.html`; also `bot_handler.go`, `user_handler.go`, `confirmation_handler.go` (unplanned but required — see below)
- **Red:** `TestLegacyAdminPagesIncludeNavSidebar` (new, `internal/adapter/web/legacy_admin_nav_test.go`) confirmed all four pages missing `id="app-sidebar"` in rendered output.
- **Green:** added `{{template "nav" .}}` to all four templates, same position as the six newer pages (immediately after `<body>`). Discovered `bot_handler.go`/`user_handler.go`/`confirmation_handler.go` passed bare anonymous-struct template data with no `*BaseData` — nav's `.IsAuthenticated`/`.ActivePage`/`.CurrentUser`/`.UnviewedRunCount` field lookups would have failed against those structs, so all three handlers were switched to embed `*BaseData` via the existing `s.baseDataFor(user, title, activePage)` helper (same pattern Jobs/Chats/History already use).
- **Refactor:** N/A (matches existing repo conventions exactly, no new abstraction introduced).
- **Acceptance criteria:** met — all four templates include the nav include; verified rendering correctness (not just error-free execution) via an ad-hoc test confirming the Bots nav link receives the active-page highlight class and the session username renders in the user menu.

### Task G.2 — FR-R17: History reads via `RunRepository` instead of raw `os.ReadFile` (P2) — DONE
- **Dependencies:** none
- **Files:** `history_handler.go`, `internal/domain/run_repository.go`, `internal/infrastructure/storage/file_run_repository.go`, `internal/usecase/history/service.go`, plus ripple updates to `history_handler_test.go`'s `MockHistoryService`
- **Red:** `TestFileRunRepository_ReadLog`/`_ReadLog_NoLogYet`/`_ReadLog_NotFound`/`_ReadResults`/`_ReadResults_NoResultsYet`/`_ReadResults_NotFound` (new, `file_run_repository_test.go`) — confirmed build failure (`ReadLog`/`ReadResults` undefined) before implementation.
- **Green:** added `ReadLog`/`ReadResults` to `domain.RunRepository`, implemented in `FileRunRepository`. `ReadLog` reads through the repository's own fsguard-confined per-owner log path (the path `AppendLog` actually writes to — `Run.LogPath` turned out to be a dead field no Executor ever populates, so this is a genuine behavior fix, not just a refactor). `ReadResults` reads `Run.ResultsPath` directly, since that path is set by the Executor at a root (`<scheduler-runs-root>/users/<owner>/runs/<runID>/RESULTS.md`) outside `FileRunRepository`'s own storage root and can't be re-resolved through its fsguard base — matches the finding's own "server-set, not currently exploitable" characterization. Both methods threaded through `usecase/history.Service` and the `web.HistoryService` interface; `history_handler.go` calls `s.historyService.ReadLog`/`ReadResults` and no longer imports `"os"`.
- **Refactor:** method naming/doc-comment style matched to existing `RunRepository` conventions (`GetRun`/`AppendLog`/`MarkNotified` etc.); extracted a shared `readFileOrEmpty` helper in `file_run_repository.go` for the two methods' identical empty/missing-file handling.
- **Acceptance criteria:** met — `history_handler.go` no longer imports `os`; all pre-existing History handler tests pass unmodified (`MockHistoryService` gained matching `ReadLog`/`ReadResults` methods that preserve the old file-path-based test behavior, so test *bodies* were untouched).

---

## References

- Source PRD (authoritative acceptance criteria + full finding rationale): [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md)
- `architecture.md` — workstream dependency graph
- `plan.md` — critical path and rollout strategy
