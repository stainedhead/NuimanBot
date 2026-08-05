# Spec: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Source PRD:** [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md)
**Reviewed branch:** `worktree-splendid-petting-falcon`
**Reviewed against:** `specs/archive/260805-nuimanbot-extend-context-and-ui/spec.md` (47 FRs, 13 Edge Cases, NFRs)
**Relationship to prior spec:** This is a **separate, follow-on spec** to `specs/archive/260805-nuimanbot-extend-context-and-ui/`. That spec covered the original six-environment feature build (now archived, feature complete per its own status.md). This spec covers **only** the 19 code-review findings raised against that already-merged implementation — it does not re-scope or re-plan the original feature.

---

## Executive Summary

A Step 5 code/design review of the just-implemented Chats/Projects/Jobs/Chores/History/Memories/Settings feature (~15,500 lines: domain entities, a worker-pool/scheduler subsystem, four file-based repositories, a WebSocket push layer, ~2,900 lines of tests) found 19 findings against the original spec's 47 FRs, 13 Edge Cases, and NFRs: **5 P0, 9 P1, 5 P2**. The engineering discipline is strong in several places verified independently (per-user IDOR protection with dedicated cross-owner-404 tests in every new handler, correct allowlist fail-closed/fail-open semantics, a well-built WebSocket hub, genuine restart-durability tests for the queue). It does not extend evenly everywhere: a Run already dequeued to a worker at crash time has no restart-recovery; the retention sweep is fully modeled but never scheduled; four per-entity chat interfaces (Job/Chore/Run/Memory) are deferred entirely; three usecase packages (`jobs`, `chores`, `projects`) import `internal/infrastructure/fsguard` directly, violating the Clean Architecture layering AGENTS.md mandates; `fsguard.ResolveWithin` is lexical-only and none of its eight call sites mitigate symlink escape despite its own doc comment calling for it; and a Project's `outputDirectory` is accepted from a non-admin-gated route with no root confinement, allowing any authenticated user to point a Project at the app's own `data/` directory or another user's tree.

**Goal:** close all 19 findings so the branch matches the original spec's FRs, Edge Cases, and NFRs, without introducing new user-facing scope beyond what that spec already promised.

## Problem Statement

The original feature implementation shipped with real, verified gaps against its own spec:

- **Reliability:** in-flight Run state is not restart-safe; retention deletion never fires despite being fully implemented; `PendingDeletion` Jobs/Chores never get hard-deleted.
- **Security:** a symlink-escape gap in the one mechanism designated as the sandboxing enforcement point; a Project output directory reachable by any non-admin user with no root confinement (P0, independently confirmed, not present in the original review pass).
- **Architecture:** a Clean Architecture dependency-rule violation (usecase importing infrastructure directly) — the only clear-cut layering violation found among the new code.
- **Functional completeness:** four numbered FRs (per-item chat interfaces) unimplemented; the flagship Chats surface never invokes the agent; the notification badge (FR-044) doesn't work as an ambient cross-page indicator; WebSocket live-update has a solid server side with zero browser-side consumer; several smaller consistency/polish gaps.

This spec exists to close all 19 findings via a disciplined, TDD, per-finding fix pass, using the workstream grouping the PRD's own "Fix-Pass Execution Guidance" section already worked out (see `plan.md` and `tasks.md` — that grouping is carried through unchanged, not re-derived).

## Goals / Non-Goals

**Goals:**
- Close all 19 findings below (5 P0, 9 P1, 5 P2) so the branch matches the original spec.md's 47 FRs, 13 Edge Cases, and NFRs.
- Follow the PRD's mandated execution discipline: TDD per finding (Red-Green-Refactor, one finding per commit), code review per fix (not batched at the end), quality gates after every fix, agent-teammate + git-worktree parallelism by workstream.

**Non-Goals (this fix pass):**
- Wiring the *live* Chats reply loop (FR-R1, full `internal/usecase/chat` multi-turn/tool/RBAC orchestration) and replacing Jobs/Chores' `StubExecutor` with real agent execution — both are separately-scoped follow-up efforts. FR-R1's acceptance criteria explicitly accepts "implement, or explicitly document the deferral + notify the user in the UI" so this pass isn't blocked on that larger integration.
  - This is distinct from **FR-R4** (per-item Job/Chore/Run/Memory chat), which **stays in scope and P0**: narrower, single-turn/context-grounded interaction, not the full orchestration engine — no documentation-only fallback accepted.
- No new user-facing features beyond what the original spec's 47 FRs already describe.
- Carried forward from the original spec's own Non-Goals, still out of scope here: Chore pause/disable, user-initiated run cancellation, storage/disk quota enforcement or warning UI, a general Chats file browser, real-time multi-user collaborative Project editing.
- A live HTTP listener rebind for Settings' network-access mode (FR-R11) may remain out of scope for this pass provided the UI stops presenting config-file-only fields as live-editable.

## User Requirements — Functional Requirements

Each finding below is numbered as it appears in the source PRD (FR-R1–FR-R19, non-sequential numbering preserved from the review document). Full finding text, rationale, and acceptance criteria are in the source PRD; this table is the spec-level index.

### P0 — True blockers (break a stated NFR/security guarantee or a core FR entirely)

| ID | Finding | Primary files |
|---|---|---|
| FR-R2 | No restart-recovery for a Run already dequeued to a worker at crash time | `internal/infrastructure/scheduler/queue.go`, `pool.go`, `cmd/nuimanbot/*.go` |
| FR-R3 | Retention sweep fully modeled but never scheduled/invoked (Chats/Projects/History `SweepExpired`) | `chats.Service`, `projects.Service`, `history.Service`, `cmd/nuimanbot/main.go` / `extended_context.go` |
| FR-R4 | Per-item chat interfaces (FR-029/037/042/047: Job/Chore/History/Memories) deferred entirely | `jobs_handler.go`, `history_handler.go`, `memories_handler.go`, Chore handler |
| FR-R6 | `fsguard.ResolveWithin` is lexical-only; no call site mitigates symlink escape | `fsguard.go` + 8 call sites (`stub_executor.go`, 4 file repos, `jobs`/`chores`/`projects` services) |
| FR-R18 | Project output directory unconfined — non-admin-gated arbitrary directory creation/write | `internal/usecase/projects/service.go`, `server.go` route registration |

### P1 — High (significant FR gap or real bug)

| ID | Finding | Primary files |
|---|---|---|
| FR-R1 | Web Chats never invokes the agent — no reply is ever produced | `internal/usecase/chats/service.go`, `internal/adapter/web/chats_handler.go` |
| FR-R5 | `usecase/{jobs,chores,projects}` import `infrastructure/fsguard` directly + raw `os` I/O (Clean Architecture violation) | `jobs/service.go`, `chores/service.go`, `projects/service.go` |
| FR-R7 | Memories `ownerUserID → ConversationID` scoping is a self-flagged, unverified assumption | `internal/usecase/memories/service.go`, `internal/usecase/memoryv2/curator_service.go` |
| FR-R8 | `chores.Service.DeleteChore` has no soft-delete (inconsistent with Jobs; violates Edge Case #3) | `chores/service.go` |
| FR-R9 | `PendingDeletion` Jobs/Chores have no cleanup sweep (Edge Case #3's second half unimplemented) | `jobs/service.go`, `chores/service.go`, sweep loop (shared with FR-R3) |
| FR-R10 | WebSocket hub has zero browser-side consumer — no live updates in the browser | `internal/adapter/web/static/`, templates, `websocket_handler.go` (server side already correct) |
| FR-R11 | Settings network-access controls only partially wireable from the UI | `settings_handler.go` |
| FR-R12 | Job/Chore run against a deleted Project doesn't fail cleanly (Edge Case #2 unhandled) | `StubExecutor.Execute` (or whichever `Executor` is live) |
| FR-R19 | Completion notification badge (FR-044) invisible outside the History page | `server.go` (`BaseData`, `NewBaseData`, `withUnviewedRunCount`), `nav.html` |

### P2 — Medium (polish/consistency)

| ID | Finding | Primary files |
|---|---|---|
| FR-R13 | `FileConversationRepository` doesn't route paths through `fsguard`, unlike the four new repositories | `file_conversation_repository.go` |
| FR-R14 | `Job.IsQueueable()` is dead code — implemented, tested, never called | `internal/domain/job.go` |
| FR-R15 | `Settings.SetWorkerPoolSize` has no upper bound | `domain.WorkerPoolConfig.Validate()`, `settings_handler.go` |
| FR-R16 | Legacy admin pages (dashboard, bots, users, confirmations) don't get the new left-nav sidebar | `dashboard.html`, `bots.html`, `users.html`, `confirmations.html` |
| FR-R17 | `history_handler.go` reads Run log/results via raw `os.ReadFile` instead of the repository | `history_handler.go`, `domain.RunRepository` |

**Totals: 5 P0, 9 P1, 5 P2 (19 findings).** Full acceptance criteria for each finding are defined in the source PRD (see References) and are not restated here to avoid drift — `tasks.md` links each task back to its finding ID for the authoritative acceptance criteria.

## Non-Functional Requirements

Carried forward from the original spec, re-affirmed as the standard this fix pass must restore compliance with:

- **Reliability NFR:** "a server restart must not lose queued Jobs, drop an in-flight run's record, or cause a Chore to miss its next scheduled fire time." (Directly violated today per FR-R2; this pass must close it.)
- **Security NFR:** "Job/Chore filesystem access is sandboxed to the assigned Project's output directory — no path traversal outside it." (Violated today per FR-R6 and FR-R18.)
- **Observability NFR:** restart-interrupted runs and sweep-driven deletions must be surfaced distinguishably (FR-R2, FR-R3's Edge Case #7 coverage).
- **Performance NFR:** "Job/Chore run status, logs, and notification badges update in near-real-time... via WebSocket... without a manual page refresh." (Unmet today per FR-R10.)
- **Concurrency/race safety:** any fix touching `internal/infrastructure/scheduler` or the WebSocket path in `internal/adapter/web` must pass `go test -race` on those packages — currently clean, must stay clean.
- **Regression safety:** the 13 existing `TestHandle*_CrossOwnerReturns404` tests must keep passing unmodified after every fix (FR-R5 and FR-R18 touch the exact repository/service code paths those tests exercise).

## System Architecture

Affected layers by workstream (see `architecture.md` for full detail):

- **Domain layer:** new interface for confined filesystem I/O (FR-R5); no change to entity invariants otherwise.
- **Usecase layer:** `jobs`, `chores`, `projects`, `chats`, `history`, `memories` services — sweep loops, soft-delete parity, Chats agent-invocation call (or documented deferral), Project root confinement, Memories `ConversationID` mapping fix.
- **Adapter layer (web):** `chats_handler.go`, `jobs_handler.go`, `history_handler.go`, `memories_handler.go`, `settings_handler.go`, `server.go` (`BaseData`), templates (`nav.html`, legacy admin pages), new browser-side WebSocket-consumer JS.
- **Infrastructure layer:** `fsguard` (symlink mitigation), `scheduler/queue.go` + `pool.go` (restart reconciliation), four file repositories + `file_conversation_repository.go` (fsguard routing), `cmd/nuimanbot` DI wiring (sweep loop, reconciliation hook).

## Scope of Changes

Files to modify are listed per-finding in the summary tables above and in full detail in the source PRD. No new top-level packages are anticipated except possibly a small infrastructure package implementing the new domain-level "confined file I/O" interface introduced by FR-R5 (workstream B).

## Breaking Changes

None expected at the API/config/schema level. FR-R18 changes `CreateProject`'s validation behavior (previously-accepted out-of-root `outputDirectory` values will now be rejected) — this is a deliberate security fix, not treated as a breaking change requiring migration, since no legitimate use case relied on the unconfined behavior.

## Success Criteria and Acceptance Criteria

- All 19 findings closed per their individual acceptance criteria (source PRD, Findings sections).
- Full quality gate passes after every individual fix (AGENTS.md Pre-Completion Checklist): `go fmt ./...` → `go mod tidy` → `go vet ./...` → `golangci-lint run` → `go test ./...` → `go build -o bin/nuimanbot ./cmd/nuimanbot` → `./bin/nuimanbot --help`.
- `go test -race` clean on `internal/infrastructure/scheduler/...` and `internal/adapter/web/...` after every fix touching those packages.
- All 13 existing `TestHandle*_CrossOwnerReturns404` tests pass unmodified throughout.
- TDD discipline: each finding's Red phase (failing test) is a separate, traceable commit from its Green/Refactor phases — no batching of multiple findings' Red phases into one commit.
- Code review per fix before merge (`dev-flow:review-code` or equivalent); FR-R6, FR-R18, FR-R5 get a **second** reviewer pass given their security/architecture stakes (FR-R18 was itself missed by the original review pass once already).
- `implementation-notes.md` records the Open Questions defaults actually applied (see below) rather than leaving the choice silent.

## Risks and Mitigation

| Risk | Mitigation |
|---|---|
| Parallel workstreams edit the same file (e.g. `projects/service.go` touched by both FR-R5 and FR-R18; `server.go`'s `BaseData` touched by both FR-R10 and FR-R19) | Explicit sequencing/coordination notes carried into `plan.md`/`architecture.md` from the PRD's workstream table — FR-R18 must rebase against FR-R5 before merging; FR-R10/FR-R19 coordinate only if editing `BaseData` in the same window. |
| Fixing FR-R6 twice (once per call site set, inconsistently) | FR-R5 relocates `fsguard` calls out of usecase into the new infrastructure interface *first*; FR-R6 adds `EvalSymlinks` at the *relocated* call sites, not the old ones. |
| FR-R9 assumes Chores already soft-delete correctly | Workstream C (FR-R8) must merge before workstream A's FR-R9 lands. |
| Introducing symlink/traversal fixes regresses legitimate in-root paths | Each security fix (FR-R6, FR-R18) includes a positive-path test alongside the adversarial test, per its acceptance criteria. |
| Retention sweep or restart-reconciliation race with live WebSocket/notification state (Edge Case #7) | FR-R3's acceptance criteria explicitly requires this interaction be tested once the sweep is wired to a live `RunRepository`/`Hub`. |

## Timeline and Milestones

No fixed calendar timeline specified by the PRD. Execution is workstream-parallel (see `plan.md`/`tasks.md`): up to 7 concurrent agent-teammate + git-worktree tracks (A–G), each internally sequenced per the PRD's Fix-Pass Execution Guidance, merging in dependency order as each workstream goes green.

## References

- Source PRD (moved into this spec directory): [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md)
- Original feature spec (archived): `specs/archive/260805-nuimanbot-extend-context-and-ui/spec.md`
- AGENTS.md — Clean Architecture layering rules (relevant to FR-R5), Pre-Completion Checklist quality gates.
