# Code Review PRD: NuimanBot Extend Context & UI

**Reviewed branch:** `worktree-splendid-petting-falcon`
**Reviewed against:** `specs/260805-nuimanbot-extend-context-and-ui/spec.md` (47 FRs, 13 Edge Cases, NFRs)
**Review type:** Step 5 of `/implm-frm-prd` — code and design review (findings-only, no fixes applied)

## Executive Summary

This branch adds six new web-admin environments (Chats, Projects, Jobs, Chores, History, Memories) plus Settings and network-access configuration to NuimanBot — roughly 15,500 lines across domain entities, a net-new worker-pool/scheduler subsystem, four file-based repositories, a WebSocket push layer, and ~2,900 lines of tests. The engineering is disciplined in the places that matter most for a first pass of this size: per-user data isolation is enforced at the repository layer (not just the UI) and is backed by dedicated cross-owner-returns-404 tests in every one of the six new handlers; the network-access allowlist's fail-closed/fail-open semantics (Edge Case #11) are implemented correctly at both the config-decode and domain layers; the WebSocket hub correctly authenticates on upgrade, checks `Origin`, and isolates per-user channels; and the FIFO queue has real restart-durability tests, not just in-memory happy-path coverage.

That discipline does not extend evenly to every subsystem. Two Reliability/Security-NFR-level gaps stand out: queued Jobs survive a restart, but a Run already dequeued to a worker at crash time does not (no reconciliation of Runs stuck in `Running` state on startup), and the retention system is fully modeled (`RetentionPolicy`, per-environment `SweepExpired` methods) but never invoked by anything — three FRs promise automatic deletion that currently never happens. A new, previously-unreviewed finding of similar weight: three usecase packages (`jobs`, `chores`, `projects`) import `internal/infrastructure/fsguard` directly and perform raw `os.MkdirAll`/`os.WriteFile`/`os.Stat` calls, inverting the Clean Architecture dependency rule AGENTS.md mandates — this is the only clear-cut architecture-layering violation found among the new code (`domain` has zero non-stdlib imports; `internal/infrastructure/scheduler` never imports `adapter`). A second new finding: `fsguard.ResolveWithin` is explicitly documented as lexical-only (no symlink resolution), and none of its seven call sites perform the `EvalSymlinks` mitigation its own doc comment calls for, leaving a documented-but-unaddressed symlink-escape gap in the one mechanism the spec designates as the sandboxing enforcement point. A third: the Memories environment's `ownerUserID → ConversationID` scoping is an explicitly self-flagged, unverified assumption in the code — worth a reviewer's explicit sign-off before relying on it, per the comment's own request.

The deliberate scope decision to ship `StubExecutor` (no LLM/agent invocation) instead of wiring real agent execution is sound engineering judgment for a first pass and is not re-litigated here. The finding that overlaps with it — Chats persisting messages but never invoking the LLM at all — is treated separately below because it defeats the primary purpose of the Chats environment specifically (a user who sends a message never gets a reply, full stop), distinct from Jobs/Chores having a placeholder execution backend.

Two additional findings, independently verified against the code directly (not carried over from the Step 4 known-gaps list), round out this pass. First, a P0: `internal/usecase/projects/service.go`'s `CreateProject` accepts a user-supplied `outputDirectory` with no root confinement — only `filepath.Abs(filepath.Clean(...))` — and the route creating it (`/admin/projects`) is gated by the lowest role (`RoleUser`), not admin-only. Any authenticated non-admin user can therefore point a Project at the application's own `data/` directory or another user's tree, and the app will `MkdirAll` and later write `AGENTS.md` into it — `fsguard`'s confinement only applies to paths *relative to* this already-unconfined root, so the hardening built on top of it doesn't help. This breaks FR-010 and the Security NFR exactly as directly as the other P0s. Second, a P1: the History notification badge (FR-044) that `nav.html` renders on every page is only ever populated by `history_handler.go`'s own page renders — `BaseData.UnviewedRunCount` defaults to zero everywhere else (`server.go:321-325,337`, with a comment acknowledging the gap) — so a user must already be on the History page to see the count, defeating the badge's purpose as an ambient, cross-page indicator (FR-030/FR-038 are effectively unmet outside that one page as a result).

**Findings: 19 total — 5 P0, 9 P1, 5 P2.**

---

## P0 Findings (true blockers — break a stated NFR/security guarantee or a core FR entirely)

### FR-R1: Web Chats never invokes the agent — no reply is ever produced

**Finding:** `internal/usecase/chats/service.go` and `internal/adapter/web/chats_handler.go` implement create/list/persist/retain/delete/export for Chats (FR-011–FR-016) by extending `domain.Conversation`/`ConversationRepository`, but neither the handler nor the service ever calls into `internal/usecase/chat` (the existing LLM/tool/RBAC orchestration engine) or any other agent-invocation path. A user sending a message in the web Chats UI gets it persisted and nothing else — no assistant turn is ever appended. Other gateways (Telegram, Slack, CLI, Buzz) are unaffected; this is specific to the new web Chats environment's wiring.

**Why it matters:** Chats is the flagship new interactive surface (spec.md Goals: "Persistent, organized workspace for agent interaction"). Without an agent reply, the environment satisfies its CRUD/retention/export sub-requirements but delivers none of its stated purpose. This is distinct from the Jobs/Chores `StubExecutor` descope: that one still produces an observable Run lifecycle end-to-end; this one produces literally no agent output under any circumstance.

**Acceptance criteria:**
- Sending a message in the web Chats UI results in an assistant reply appended to the conversation, using the existing `internal/usecase/chat` orchestration (or an explicitly documented equivalent), within the same reliability/observability guarantees already applied to other gateways.
- If deferring this to a follow-up pass is the accepted call, `implementation-notes.md`'s "Deviations from Plan" section must say so explicitly (it currently does not — it only documents the Job/Chore `Executor` deferral) and the Chats UI must visibly communicate to the user that replies are not yet available, rather than silently doing nothing.

---

### FR-R2: No restart-recovery for a Run already dequeued to a worker at crash time

**Finding:** `internal/infrastructure/scheduler/queue.go`'s `Dequeue` removes an item from the persisted queue file *before* the worker begins executing it (`pool.go`'s `tryDispatch`/`runOne`). Once dequeued, a Run's only durable state is whatever `RunRepository.SaveRun` last wrote — typically `Status = Running`. If the process crashes between dequeue and completion, that Run record is left permanently at `Running` on disk; nothing at startup scans for Runs stuck in a non-terminal state and reconciles them (re-enqueues, or marks `Failed` with a clear restart-related `Error`). Confirmed via `grep` across `cmd/nuimanbot/*.go` and `internal/infrastructure/scheduler/*.go` for any reconciliation/recovery logic — none exists. `TestQueue_RestartDurability`/`TestQueue_RestartDurability_AfterPartialDequeue` in `queue_test.go` only exercise the still-queued case.

**Why it matters:** This is a direct, unambiguous violation of the spec's own Reliability NFR: *"a server restart must not lose queued Jobs, drop an in-flight run's record, or cause a Chore to miss its next scheduled fire time."* Queued-but-not-dispatched Jobs are durable (tested); in-flight Runs are not (untested and unimplemented). A crash during any active Job/Chore execution silently strands that Run at `Running` forever, with no error, no retry, and no way for the user to know it isn't still progressing.

**Acceptance criteria:**
- On startup, any `Run` found in a non-terminal state (`Running`, or `Queued` with no matching entry in the persisted queue) is reconciled: either re-enqueued (if idempotent-safe) or transitioned to `Failed` with `Error` set to something like `"run interrupted by server restart"`.
- A test simulates the crash scenario directly: persist a Run at `Running`, construct a fresh `WorkerPool`/`Queue`/reconciliation path pointed at the same storage directory (no live in-memory state carried over), and assert the Run is no longer stuck at `Running` after the simulated restart.
- History surfaces restart-interrupted runs distinguishably (e.g., via the `Error` message), consistent with the Observability NFR.

---

### FR-R3: Retention sweep is fully modeled but never scheduled or invoked — three FRs' "auto-delete" promise doesn't fire

**Finding:** `chats.Service.SweepExpired`, `projects.Service.SweepExpired`, and `history.Service.SweepExpired` are implemented and unit-tested in isolation, but `grep -rn "SweepExpired"` across the entire non-test codebase shows zero callers outside their own definitions. No cron job, background goroutine, or scheduler hook in `cmd/nuimanbot/main.go` or `extended_context.go` ever invokes any of the three. Retention windows are configurable and displayed in Settings (`RetentionDefaults`), and `RetentionPolicy.IsExpired` is correct, but nothing ever asks "is this expired?" against real data.

**Why it matters:** FR-014 (Chat retention), FR-023 (Project retention), and FR-043 (History retention) all explicitly promise "Chats/Projects/runs older than a configured, non-'Never' period are deleted automatically." A user who sets a 30-day Chat retention window today gets no deletion, ever — the setting is cosmetic. This is a real, user-visible data-handling promise that silently doesn't hold, not a missing nice-to-have.

**Acceptance criteria:**
- A periodic sweep (goroutine on a ticker, analogous to `ChoreScheduler`'s poll loop, or reusing that pattern) calls all three `SweepExpired` methods for every user on a defined interval, wired in `cmd/nuimanbot`'s DI.
- An integration-level test proves an expired Chat/Project/Run created via the real repository is actually gone after a sweep cycle runs against it — not just that the use-case method returns the right count in isolation.
- Edge Case #7 (a retention sweep deleting an unviewed Run must decrement the notification badge, never reference a deleted Run) is covered by a test once the sweep is actually wired to a live `RunRepository`/`Hub`.

---

### FR-R4: Chat/Project/Job/Chore per-item chat interfaces are deferred — four numbered FRs unimplemented

**Finding:** FR-029 (Job chat), FR-037 (Chore chat), FR-042 (History/Run chat), FR-047 (Memories chat) are each explicitly deferred with inline comments (`jobs_handler.go:155`, `history_handler.go:225`, `memories_handler.go:21`, and the analogous Chore path) rather than partially built or stubbed. Confirmed by reading each cited location directly.

**Why it matters:** This is called out by the task brief as "a real gap against multiple numbered FRs, not a minor nit," and independent reading confirms that characterization — four distinct FRs, spanning four of the six new environments, have no implementation at all (not even a placeholder UI element), as opposed to a stub backend. Given the environments' entire post-hoc-review workflow (per spec.md's Success Criteria: "per-run chat grounded in run log/results") depends on this, its total absence materially narrows what a user can actually do with Job/Chore/Run/Memory records once created.

**Acceptance criteria:**
- Either implement a minimal per-item chat interface for at least one of the four (Job, Chore, Run, Memory) grounded in that item's own context (description/log/results/memory content) as a template the other three can follow, or
- If deferred to a follow-up pass, document that decision explicitly in `implementation-notes.md`'s "Deviations from Plan" (currently silent on this) and ensure each detail page's UI clearly communicates "chat not yet available" rather than omitting the affordance with no explanation.

---

### FR-R18: Project output directory is unconfined — reachable, non-admin-gated arbitrary directory creation/write

**Finding:** `internal/usecase/projects/service.go`'s `CreateProject` (~lines 51-73) accepts a user-supplied `outputDirectory` and only applies `filepath.Abs(filepath.Clean(outputDirectory))` before `os.MkdirAll(absOutput, 0755)` — no containment to any allowed root. `/admin/projects` is registered with `userHandler` (`server.go:188,198`), which resolves to `s.requireRole(domain.RoleUser)` — the lowest role, not admin-gated, so any authenticated user can reach this. Once created, `AddAgentsFile` and agent-chat-driven edits write into that directory via `fsguard.ResolveWithin(p.OutputDirectory, ...)` — but `fsguard` only confines paths *relative to* `OutputDirectory`; it does nothing to constrain what `OutputDirectory` itself may be, so the confinement it provides sits on an unconfined foundation.

**Why it matters:** a non-admin user can create a Project rooted at, e.g., the app's own `data/` directory (where `users.json`/`bots.json`/config live) or another user's Project/hidden directory, and the app will create it and later write into it. This directly breaks FR-010 ("full isolation, including from admins") and the Security NFR's sandboxing guarantee, which is only meaningful once the confined root is itself trustworthy — exactly the class of gap the rest of this review found the branch otherwise avoided everywhere else (IDOR, allowlist, WebSocket auth, CSRF all check out clean).

**Acceptance criteria:**
- `CreateProject` validates `outputDirectory` against a configured allowed root (e.g. a per-deployment "projects root," or confinement under `<storagePath>/users/<ownerUserID>/projects/`) using `fsguard.ResolveWithin` (or equivalent) on the directory itself, not only on paths relative to it.
- A request pointing `outputDirectory` at the app's `data/` directory, or at another user's directory (via relative traversal or an absolute path outside the allowed root), is rejected with a clear validation error.
- Tests cover both the relative-traversal-escape and absolute-path-outside-root cases and assert rejection; a positive-path test confirms legitimate in-root paths still work.

---

## P1 Findings (high — significant FR gap or real bug)

### FR-R5: `internal/usecase/{jobs,chores,projects}` import `internal/infrastructure/fsguard` directly and perform raw filesystem I/O — Clean Architecture layering violation

**Finding:** `internal/usecase/jobs/service.go`, `chores/service.go`, and `projects/service.go` all `import "nuimanbot/internal/infrastructure/fsguard"` directly, and all three call `os.MkdirAll`/`os.WriteFile`/`os.Stat` inline (e.g. `jobs/service.go:108,171`; `chores/service.go:88,95`; `projects/service.go:68,71,135,142`). Per AGENTS.md: *"Use Case Layer: Orchestrates domain entities. Defines repository/service interfaces... Dependencies flow inward only. Inner layers define interfaces; outer layers implement them."* Here the usecase layer both imports a concrete infrastructure package and performs the filesystem I/O itself, rather than depending on a domain-defined interface that an infrastructure implementation satisfies via DI.

By contrast, `internal/domain/{project,job,chore,run,retention,schedule}.go` have zero non-stdlib imports (verified), and `internal/infrastructure/scheduler` never imports `internal/adapter` (verified) — so this is a localized violation in three specific packages, not a systemic one.

**Why it matters:** This is the exact kind of Clean Architecture boundary violation the review was asked to specifically hunt for. It also has a practical cost beyond layering purity: these usecase services are now impossible to unit-test without touching a real filesystem (or `os`-level test doubles), and any future infrastructure swap (e.g. moving hidden-directory storage off local disk) requires touching usecase code instead of only its infrastructure implementation.

**Acceptance criteria:**
- Define a small domain-level interface (e.g. `domain.HiddenFileWriter` or extend the existing repository interfaces) that captures "write/read a file confined to this entity's hidden/output directory."
- `internal/infrastructure/storage` (or a new small package) implements it using `fsguard.ResolveWithin` + `os`.
- `jobs`, `chores`, `projects` usecase services depend only on the new interface, with no `os` or `fsguard` import remaining in any of the three `service.go` files.
- Existing tests continue to pass against a fake implementation of the new interface.

---

### FR-R6: `fsguard.ResolveWithin` is lexical-only; no call site mitigates symlink escape despite the doc comment requiring it

**Finding:** `fsguard.go`'s own doc comment states: *"ResolveWithin is a pure path computation and performs no filesystem I/O (in particular, it does not resolve symlinks — callers that subsequently open the returned path should use O_NOFOLLOW-equivalent care, or resolve baseDir itself via filepath.EvalSymlinks before calling ResolveWithin, if symlink escape... is a concern for that call site)."* `grep` across every caller (`stub_executor.go`, `file_run_repository.go`, `file_project_repository.go`, `file_chore_repository.go`, `file_job_repository.go`, `jobs/service.go`, `chores/service.go`, `projects/service.go`) shows zero uses of `filepath.EvalSymlinks` or any `O_NOFOLLOW`-equivalent open flag anywhere in the new code. `fsguard_test.go` has strong adversarial coverage for `../` traversal, absolute-path rejection, NUL-byte rejection, and sibling-directory-prefix confusion (`TestResolveWithin_SiblingDirectoryPrefixNotConfused`) — but no symlink-escape test, consistent with the gap.

**Why it matters:** spec.md's Security NFR states plainly: *"Job/Chore filesystem access is sandboxed to the assigned Project's output directory — no path traversal outside it."* A Project's output directory is a location the owning user (and, per FR-020/FR-022, the agent) has direct read/write access to — either could place a symlink inside it pointing outside the sandbox. Every subsequent `ResolveWithin`-confined read/write against a path that traverses through that symlink would silently escape confinement, despite `ResolveWithin` reporting success. This is the one mechanism the spec designates as the sandboxing enforcement point (Risks table: "Centralize path-resolution/validation in one place... used by every Job/Chore/Project file operation"), so a gap here undercuts that guarantee everywhere it's used, not just one call site.

**Acceptance criteria:**
- At minimum, document and test the actual current exposure: add a test that creates a symlink inside a confined base directory pointing outside it, and shows `ResolveWithin` currently returns a path that, when opened, escapes the sandbox — making the gap explicit rather than implicit.
- Close the gap for at least the highest-risk call sites (Project output directory operations, since that's the one directory a non-admin user has direct filesystem access to per FR-022) via `filepath.EvalSymlinks` on resolved paths before use, or an equivalent guard.
- Re-run the new adversarial test to confirm it now passes.

---

### FR-R7: Memories environment's `ownerUserID → ConversationID` scoping is an unverified assumption, self-flagged in the code as needing reviewer confirmation

**Finding:** `internal/usecase/memories/service.go`'s package doc comment states outright: *"ownerUserID -> ConversationID mapping (ASSUMPTION, needs reviewer confirmation)... No existing 1:1 mapping from a web-admin ownerUserID... to a set of ConversationIDs was found in this codebase. Pending a real mapping, this Service takes the pragmatic, explicit choice of treating ownerUserID itself as the ConversationID filter value... This under-shows a user's memories if their cells were actually filed under a session/conversation UUID instead."* Tracing `memoryv2.MemoryCell.ConversationID`'s doc comment ("the conversation **or user** ID this cell belongs to") and its production populators in `internal/usecase/memoryv2/curator_service.go` (populated from `interaction.ConversationID`, which flows from whatever the calling gateway passes) confirms the ambiguity is real and unresolved — this reviewer could not confirm from the code alone whether production memory cells are actually keyed by username or by a per-conversation/session identifier for any given gateway.

**Why it matters:** If the assumption is wrong for any gateway a user actually uses, FR-045 ("read-only browse/search view over the agent-maintained memory store") silently under-shows or empty-shows that user's real memory data in the web UI, while the isolation check (`GetCell`'s `cell.ConversationID != conversationIDFor(ownerUserID)` → 404) would also silently reject legitimate access to the user's own memories. The code asks explicitly for this confirmation; this review is that confirmation pass, and it could not resolve the ambiguity from static reading alone.

**Acceptance criteria:**
- Trace (or add a test proving) what `ConversationID` value memory cells actually carry for at least one real gateway (e.g. CLI or Telegram) end-to-end from message receipt through `curator_service.go` to persisted `MemoryCell.ConversationID`.
- If it's a per-session/conversation UUID rather than a stable per-user identifier, replace `conversationIDFor`'s pass-through with a real mapping (e.g. a lookup table, or plumbing the user's stable ID through the existing conversation flow) before shipping Memories as a claimed "browse a user's memories" feature.
- Add an integration test creating a memory cell via the real curator path and asserting it's visible via `memories.Service.ListCells` for that same user — not just the current unit test's synthetic `ConversationID = username` fixture.

---

### FR-R8: `chores.Service.DeleteChore` has no soft-delete — inconsistent with Jobs, violates spec Edge Case #3

**Finding:** `jobs.Service.DeleteJob` correctly implements Edge Case #3 (soft-mark `PendingDeletion` when a Job is `Running`/`Queued`, defer hard delete until the run reaches a terminal state). `chores.Service.DeleteChore` has an explicit `TODO(soft-delete)` comment acknowledging the same requirement applies to Chores, but the current implementation is *"a plain, ownership-scoped immediate delete"* with no status check at all.

**Why it matters:** Edge Case #3 is a required test case per spec.md, not a stretch goal, and applies equally to Jobs and Chores by its own wording ("Deleting a Job/**Chore** while its run is active"). Deleting a Chore mid-run can corrupt in-flight run bookkeeping (the worker pool's `running` map is keyed by `SourceID`, and the Chore record it points back to may vanish underneath it) exactly the scenario Edge Case #3 exists to prevent.

**Acceptance criteria:**
- `DeleteChore` checks the Chore's current run status the same way `DeleteJob` does and soft-marks `PendingDeletion` when a run is active, deferring the hard delete.
- A test mirrors `jobs`' existing soft-delete test coverage for the Chore path.
- The TODO comment is removed once implemented (or replaced with the same "sweep integration deferred" comment `jobs/service.go` already carries, if the terminal-state cleanup sweep is deferred consistently with Jobs).

---

### FR-R9: `PendingDeletion` Jobs/Chores have no cleanup sweep — Edge Case #3's second half is unimplemented for both

**Finding:** Both `DeleteJob` (soft-delete implemented) and the to-be-fixed `DeleteChore` share the same gap once soft-deleted: nothing hard-deletes a `PendingDeletion` record once its run reaches a terminal state. `jobs/service.go`'s own comment confirms this: *"a background sweep that hard-deletes a PendingDeletion Job once its run reaches a terminal state is out of scope for this pass."*

**Why it matters:** A soft-deleted Job/Chore is permanently stuck in `PendingDeletion` limbo — its record and history never actually disappear, contradicting Edge Case #3's stated behavior ("the record and its history are removed once the run reaches a terminal state").

**Acceptance criteria:**
- A sweep (can be combined with the FR-R3 retention sweep's polling loop) checks `PendingDeletion` Jobs/Chores whose associated run has reached a terminal state and hard-deletes them.
- A test proves a `PendingDeletion` Job/Chore is actually removed once its run completes, not just that the flag is set correctly at delete time.

---

### FR-R10: WebSocket hub has a solid server-side implementation but zero browser-side consumer — Performance NFR's "no manual refresh" promise doesn't hold

**Finding:** `internal/adapter/web/websocket_handler.go` is well-built (auth-checked upgrade, `Origin` validation, per-user channel isolation, bounded send buffers, ping/pong keepalive — see Positive Observations). However, `grep` across `internal/adapter/web/static/` and every `.html` template for `WebSocket`/`websocket` returns zero matches — there is no client-side JavaScript anywhere in the new templates or static assets that opens a connection to `/admin/ws` (or wherever it's mounted) or renders an incoming `RunEvent` into the DOM.

**Why it matters:** The Performance NFR states: *"Job/Chore run status, logs, and notification badges update in near-real-time while a run is active via WebSocket... without a manual page refresh."* The entire server-side mechanism for this exists and is tested, but with no consumer, a user watching a Job/Chore run in the browser today sees no live updates at all — they must manually reload the page, which is precisely what the NFR says must not be required.

**Acceptance criteria:**
- Add a minimal client-side script (vanilla JS is consistent with the existing template style) on the Job/Chore/History detail pages that opens the WebSocket connection, and on `run_status`/`run_log`/`notification_badge` events, updates the relevant DOM elements (status badge, log tail, nav notification count) without a page reload.
- Manually verify (or add a browser-level test if the project has tooling for one) that starting a Run and watching its detail page shows the status transition and log growth live.

---

### FR-R11: Settings' network-access controls are only partially wireable from the UI — FR-005/006/007 not fully deliverable via Settings as the FRs imply

**Finding:** `settings_handler.go`'s `handleSettingsUpdate` only accepts `worker_pool_size` and `network_mode` from the POST form. The allowlist and bind address are `config.yaml`-only (`SettingsPageData` displays `NetworkBindAddress`/`AllowlistCount` read-only; there's no corresponding form field or handler branch to change either). Additionally, `SetNetworkAccessConfig` only ever updates the in-memory `networkAccessState` the allowlist middleware reads — it does not rebind the actual HTTP listener, so switching `network_mode` from localhost-only to remote via the UI updates the allowlist-check logic's view of the mode but does not actually make the server reachable from outside 127.0.0.1 (the listener bind address is fixed at process-start wiring).

**Why it matters:** FR-008 states access mode and allowlist are "configurable via config file **and Settings UI**." Today only `network_mode`'s *allowlist-check* behavior is Settings-UI-configurable; the allowlist itself, the bind address, and the actual listener rebind are not — despite the Settings page displaying all three as if they were live, editable state.

**Acceptance criteria:**
- Either extend the Settings POST handler to accept and apply allowlist entries and bind address changes (requiring an actual listener restart/rebind, which is a meaningfully larger change), or
- If a live listener rebind is out of scope for this pass, the Settings UI must clearly indicate which fields are config-file-only (e.g. disable/grey those inputs with an explanatory note) rather than presenting `NetworkMode` as a live-effect toggle while allowlist/bind-address are silently read-only.

---

### FR-R12: `Job/Chore` run against a deleted Project's context does not fail cleanly — Edge Case #2 unhandled

**Finding:** `StubExecutor.Execute` (the currently-wired `Executor`) never looks up or validates the referenced Project before writing its placeholder `RESULTS.md` under `<baseDir>/users/<ownerUserID>/runs/<runID>/` — a path that never actually depends on the Project's own output directory. It has no code path that checks Project existence at all, so a Job/Chore whose `ContextID` points at a deleted Project runs to `Completed` exactly as if the Project still existed, rather than failing with `Error = "referenced Project no longer exists"` as Edge Case #2 requires.

**Why it matters:** This is distinct from the general "stub doesn't do real agent work" descope (which the task brief already excludes from scoring) — Edge Case #2 is a specific, spec-mandated failure-mode contract that the executor doesn't implement at all, stub or not. A future real `Executor` inheriting this gap would silently execute against a missing/wrong directory instead of failing fast, exactly the failure mode Edge Case #2 exists to prevent.

**Acceptance criteria:**
- `StubExecutor` (or whichever `Executor` is live at merge time) checks Project existence via `ProjectRepository` when `SourceType`/context indicates a Project-scoped run, before proceeding, and fails the Run with `Error = "referenced Project no longer exists"` per Edge Case #2's exact wording if the Project is missing.
- A test creates a Job/Chore against a Project, deletes the Project, triggers a run, and asserts the Run ends `Failed` with the specified error message rather than `Completed`.

---

### FR-R19: Completion notification badge is invisible outside the History page

**Finding:** `BaseData.UnviewedRunCount` (`server.go:321-325`) defaults to zero via `NewBaseData` (`server.go:337`), with a comment acknowledging the scope: "pages without one simply leave it at zero rather than requiring every handler to know about History." The only call sites for `withUnviewedRunCount`, which populates that field, are `history_handler.go:90` and `:147` — both inside History's own page renders. `nav.html:67` renders the badge on every page (since `nav` is included everywhere the new templates are used), but the value is only ever non-zero when the user is already viewing History.

**Why it matters:** FR-044 describes an ambient nav-item badge whose entire purpose is to be visible from anywhere, so a user doesn't need to already suspect a run finished before checking. As implemented, a user must navigate to History — the one place they'd already know the answer — before the count populates. Combined with FR-R10 (no WebSocket consumer), this means FR-030 and FR-038 ("user receives an in-app notification when a Job/Chore run completes") are effectively unmet everywhere except that one page: the underlying computation (`UnviewedCount`) is correct, only the rendering plumbing is scoped too narrowly to satisfy what the FRs describe.

**Acceptance criteria:**
- `UnviewedRunCount` is populated for every authenticated page render (e.g. centralized in whatever shared "build BaseData for an authenticated request" path exists, or composed into every environment handler), not only History's.
- Navigating to Dashboard, Chats, Jobs, Projects, etc. after a run completes shows the badge without visiting History first.
- A test renders a non-History page after an unviewed completed run exists and asserts the badge count appears in the response.

---

## P2 Findings (medium — polish/consistency)

### FR-R13: `FileConversationRepository` doesn't route paths through `fsguard`, unlike the four new repositories

**Finding:** `file_conversation_repository.go` builds every path via plain `filepath.Join(r.basePath, "users", userID, "conversations", ...)` with no `fsguard.ResolveWithin` call, while `file_project_repository.go`/`file_job_repository.go`/`file_chore_repository.go`/`file_run_repository.go` (all new in this pass) all resolve their paths through `fsguard.ResolveWithin(r.userDir(ownerUserID), id+".json")`.

**Why it matters:** This is an inconsistency in hardening posture rather than a currently-exploitable gap — `userID` and `convID` values reaching these paths are server-generated (session username, UUID conversation IDs), not raw user input, so the practical traversal risk today is low. Still, Chats is the one environment among the six that isn't protected by the mechanism the spec designates as the standard for this codebase going forward, and any future code path that lets a less-trusted value reach `getUserConvDir`/`getConvDir` inherits the gap silently.

**Acceptance criteria:**
- `file_conversation_repository.go`'s path-building helpers route through `fsguard.ResolveWithin`, matching the pattern in the four new repositories.
- Existing conversation-repository tests continue to pass unmodified (behavior for well-formed IDs is unchanged).

---

### FR-R14: `Job.IsQueueable()` is dead code — implemented, tested, never called

**Finding:** `internal/domain/job.go:79`'s `IsQueueable()` has four dedicated tests (`job_test.go`) covering fresh/running/queued/pending-deletion states, but `grep -rn "IsQueueable"` across all non-test, non-domain code shows zero callers — neither `jobs/service.go`'s enqueue path nor `pool.go`'s dispatch path uses it.

**Why it matters:** Minor, but worth flagging: either the enqueue-guard logic it encodes (don't re-queue an already-running/queued/pending-deletion Job) isn't actually enforced anywhere it should be, or the logic is duplicated ad hoc elsewhere and this method is simply orphaned. Both are worth a deliberate decision rather than silent dead code.

**Acceptance criteria:**
- Either wire `IsQueueable()` into the actual enqueue path (`jobs.Service`'s create/re-run flow, or `pool.Enqueue`'s caller) where its guard logic is meant to apply, or remove it and its tests if the equivalent check is intentionally implemented differently elsewhere.

---

### FR-R15: `Settings.SetWorkerPoolSize` has no upper bound — large values bypass the throttle entirely

**Finding:** `handleSettingsUpdate` rejects `worker_pool_size <= 0` but accepts any positive integer, and `domain.WorkerPoolConfig.Validate()` (the domain-level guard) likewise only rejects non-positive values — there is no ceiling anywhere in the validation chain. `WorkerPool.SetConcurrency`/`tryDispatch` will happily attempt to dispatch as many concurrent goroutines as the queue has depth for once `concurrency` is set arbitrarily high.

**Why it matters:** This is admin-only (FR-004), so the practical severity is bounded — an attacker would need admin credentials already. But an accidental fat-fingered value (e.g. a stray extra digit) has no guardrail, and could momentarily burst-dispatch an unbounded number of concurrent Runs (each potentially an LLM call, once real execution is wired) against a single-process worker pool with no other resource limit in front of it.

**Acceptance criteria:**
- Add a sane upper bound (e.g. a documented constant, or a multiple of `runtime.NumCPU()`) to `domain.WorkerPoolConfig.Validate()`, surfaced as a rejected/flash-error value in the Settings form the same way `<= 0` already is.

---

### FR-R16: Legacy admin pages (dashboard, bots, users, confirmations) don't get the new left-nav sidebar

**Finding:** Of all templates in `internal/adapter/web/templates/`, only the six new environment templates (plus `nav.html`/`base.html`/`settings.html`) reference `{{template "nav" .}}`. `dashboard.html`, `bots.html`, `users.html`, and `confirmations.html` — the pre-existing admin pages — do not (confirmed via `grep -L`).

**Why it matters:** A user landing on any pre-existing admin page has no navigation path to any of the six new environments except by typing a URL directly; conversely, someone on a new environment page who navigates to `/admin/bots` loses the new nav entirely. This is a real navigation dead-end, not just a cosmetic gap, though it doesn't block any FR outright.

**Acceptance criteria:**
- `dashboard.html`, `bots.html`, `users.html`, and `confirmations.html` include `{{template "nav" .}}` consistent with the six new environment pages, and render correctly with it (verified visually, not just that the template executes without error).

---

### FR-R17: `history_handler.go` reads Run log/results via raw `os.ReadFile` instead of the repository abstraction

**Finding:** `readFileContentOrEmpty` in `history_handler.go:181` calls `os.ReadFile(path)` directly against `run.LogPath`/`run.ResultsPath` rather than going through `domain.RunRepository`'s own log/results accessors. The paths themselves are server-set (not user input), so this is not currently exploitable, but it's an adapter-layer file I/O call bypassing the repository boundary the rest of the new code (and AGENTS.md's layering) otherwise respects consistently.

**Why it matters:** Minor consistency gap; worth noting because every other new piece of filesystem access in this pass goes through a repository or `fsguard`, making this one raw `os.ReadFile` call stand out as the exception.

**Acceptance criteria:**
- Add a `RunRepository` method (e.g. `ReadLog`/`ReadResults`) that encapsulates this read, and have `history_handler.go` call it instead of touching `os` directly.

---

## Positive Observations

- **All spec-mandated quality gates pass cleanly**, independently re-run for this review: `go vet ./...` is silent, `golangci-lint run` reports 0 issues, `go test ./...` is fully green, and a targeted `go test -race ./internal/infrastructure/scheduler/... ./internal/adapter/web/...` — the two packages with the branch's actual concurrency (worker pool, FIFO queue, WebSocket hub) — shows no detected races. This satisfies the spec's own Success Criteria checklist in full.
- **IDOR protection is comprehensive and consistently tested.** Every one of the six new environments has dedicated `TestHandle*_CrossOwnerReturns404` tests (13 total across Chats, Chores, History, Memories, Jobs, Projects) — a genuinely strong, systematic pattern, not incidental coverage. The underlying mechanism (`ownerUserID` passed explicitly to every repository method, cross-owner lookups resolving to `ErrNotFound` at the data-access layer per `implementation-notes.md`'s documented design choice) is sound and matches Edge Case #10's requirement that existence never be disclosed across owners, including to admins.
- **Network-access allowlist fail-closed/fail-open semantics (Edge Case #11) are correctly implemented** at both `internal/config/network_access_config.go` (decode-path nil-vs-empty preservation, verified via its own decode-path test) and `internal/domain/network_access.go`'s `IsAllowed` (nil allowlist → allow all; non-nil empty → deny all). The middleware enforces this pre-auth as required by the Security NFR.
- **WebSocket hub is well-engineered on the server side**: auth-checked on upgrade, `Origin` validated against `r.Host` (CSRF-equivalent protection), strict per-user channel isolation keyed by `ownerUserID`, bounded per-client send buffers with drop-not-block semantics for slow clients, and proper ping/pong keepalive. The one gap (FR-R10) is entirely on the missing client-side consumer, not this implementation.
- **`Queue` has genuine restart-durability tests**, not just happy-path coverage — `TestQueue_RestartDurability` and `TestQueue_RestartDurability_AfterPartialDequeue` construct a fresh `Queue` instance against the same persisted file and verify state survives, which is the right pattern (the gap is that this pattern wasn't extended to in-flight Run state — see FR-R2).
- **`fsguard_test.go`'s adversarial coverage is solid** for the traversal classes it does cover: `../` traversal, absolute-path rejection, NUL-byte rejection, and a sibling-directory-prefix-confusion case (`/base` vs `/basement`) that's easy to get wrong and was clearly deliberately tested.

---

## Summary Table

| # | Finding | Priority |
|---|---|---|
| FR-R1 | Web Chats never invokes the agent — no reply ever produced | P0 |
| FR-R2 | No restart-recovery for a Run dequeued at crash time | P0 |
| FR-R3 | Retention sweep implemented but never scheduled/invoked | P0 |
| FR-R4 | Per-item chat interfaces (FR-029/037/042/047) deferred entirely | P0 |
| FR-R18 | Project output directory unconfined — reachable, non-admin arbitrary directory write | P0 |
| FR-R5 | Usecase layer imports `infrastructure/fsguard` + raw `os` I/O (Clean Architecture violation) | P1 |
| FR-R6 | `fsguard.ResolveWithin` has no symlink-escape mitigation at any call site | P1 |
| FR-R7 | Memories `ownerUserID→ConversationID` mapping is a self-flagged unverified assumption | P1 |
| FR-R8 | `chores.Service.DeleteChore` has no soft-delete (inconsistent with Jobs) | P1 |
| FR-R9 | `PendingDeletion` Jobs/Chores have no terminal-state cleanup sweep | P1 |
| FR-R10 | WebSocket hub has no browser-side JS consumer | P1 |
| FR-R11 | Settings network-access controls only partially wireable from the UI | P1 |
| FR-R12 | Job/Chore run against a deleted Project doesn't fail per Edge Case #2 | P1 |
| FR-R19 | Notification badge (FR-044) invisible outside the History page | P1 |
| FR-R13 | `FileConversationRepository` doesn't use `fsguard`, unlike the four new repos | P2 |
| FR-R14 | `Job.IsQueueable()` is dead code | P2 |
| FR-R15 | `Settings.SetWorkerPoolSize` has no upper bound | P2 |
| FR-R16 | Legacy admin pages lack the new left-nav sidebar | P2 |
| FR-R17 | `history_handler.go` reads Run files via raw `os.ReadFile` instead of the repository | P2 |

**Totals: 5 P0, 9 P1, 5 P2 (19 findings)**
