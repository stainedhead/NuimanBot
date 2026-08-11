# Tasks: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Status:** Planning

This task breakdown is a direct translation of the source PRD's "Fix Process Guidance" three-workstream parallelization plan — it does not invent an alternative grouping. Each workstream maps 1:1 to the PRD's Workstream A/B/C, and each task within a workstream maps 1:1 to one of the PRD's 7 findings (FR-1 through FR-7, renumbered here as FR-001 through FR-007 per spec.md).

## Progress Summary

**0 / 7 tasks complete** (A.1, B.1, B.2, B.3, C.1, C.2, X.1 — FR-006 has no task, it closes by acknowledgment; see "No Workstream Required" below)

## Workstream A — jobs-usecase fix (own git worktree, own teammate if using agent teammates)

### A.1 — FR-002 (P1): `/job create --project|--chat <id>` rejects foreign-owned context IDs

- **Depends on:** Research Question 1 resolved (Chat ownership failure-mode decision: soft-fail vs. hard-reject); Research Question 5 resolved (test-harness approach).
- **Duration estimate:** Largest, highest-risk task in this spec — allocate accordingly.
- **Files:** `internal/usecase/jobs/service.go` (`CreateJob`), corresponding test file, `internal/infrastructure/scheduler/stub_executor.go` (doc comment only).
- **Acceptance criteria:**
  - [ ] Red: failing test mirroring `TestGetJob_CrossOwnerIsolation`, asserting `/job create --project <id>` and `/job create --chat <id>` with a foreign-owned `<id>` are rejected.
  - [ ] Green: `CreateJob` rejects (not-found/permission error) a `contextID` that does not resolve to a Project/Chat owned by `ownerUserID`, distinguishing "not owned" from the existing "stale/deleted, owned" case.
  - [ ] `ContextTypeChat` gets the same (extended) ownership-scoping treatment as `ContextTypeProject`.
  - [ ] Refactor: mandatory cleanup pass, tests kept green throughout.
  - [ ] Doc comment added to `stub_executor.go` flagging the future-executor ownership-recheck obligation.
  - [ ] Commit/PR description explicitly calls out the "reused as-is" Non-Goals boundary crossing (spec.md line 141 of the original archived spec).
  - [ ] `dev-flow:review-code` run against this fix individually.
  - [ ] Full quality gate passes.
- **Note:** A.1 does not edit the original spec.md — per the PRD's Dependencies section, FR-2 *satisfies* spec.md's line-157 acceptance criterion, it does not require editing spec.md's text. Only C.2 (FR-005) edits spec.md. A.1 and C.2 are independent and can run fully in parallel.

## Workstream B — CLI-handler cluster (`internal/adapter/gateway/cli/`, distinct files — parallelize across teammates or run sequentially in one worktree)

### B.1 — FR-001 (P1): `/memories browse` caps/truncates output

- **Depends on:** None (independent of Workstream A).
- **Files:** `internal/adapter/gateway/cli/memories_commands.go` (`browse()`), corresponding test file.
- **Acceptance criteria:**
  - [ ] Red: failing test seeding more cells than the chosen limit, asserting capped output + "N more not shown, refine your query" trailer.
  - [ ] Green: `browse()` sets `MemoryCellFilter.Limit` and/or truncates each cell's `Content`, matching `historyListDisplayLimit`'s pattern and `/history list`'s trailer convention.
  - [ ] Refactor: mandatory cleanup pass, tests kept green throughout.
  - [ ] `dev-flow:review-code` run against this fix individually.
  - [ ] Full quality gate passes.
- **Optional / non-blocking follow-up (does not gate this task's completion):** sort results most-recently-created-first (`sortCellsByCreatedAtDesc` parity).

### B.2 — FR-003 (P2): Deferred `chat` subcommands get specific "not yet implemented" messages

- **Depends on:** Data Dictionary Phase 2 decision on message-constant naming convention.
- **Files:** `internal/adapter/gateway/cli/project_commands.go`, `job_commands.go`, `chore_commands.go`, `history_commands.go`, corresponding test files.
- **Acceptance criteria:**
  - [ ] Red: failing test per handler, asserting the specific message text (not just "some non-crashing response").
  - [ ] Green: each of the four deferred `chat` subcommands returns a specific "not yet implemented — see spec.md FR-0XX" message, matching the Settings deferred-command convention (`retentionSetNotImplementedMessage`, `networkModeNotImplementedMessage`).
  - [ ] Refactor: mandatory cleanup pass, tests kept green throughout.
  - [ ] `dev-flow:review-code` run against this fix individually.
  - [ ] Full quality gate passes.

### B.3 — FR-007 (P2): End-to-end test for stale-role-on-restore

- **Depends on:** None (independent test-only addition).
- **Files:** `internal/adapter/gateway/cli/auth_commands_test.go`.
- **Acceptance criteria:**
  - [ ] Test pre-seeds a `domain.User` for `(PlatformCLI, "alice")` with a stale role, writes a valid session file whose `Role` differs, and asserts `EnsureAuthenticated`'s **restore** path (not `reconcileIdentity` called directly) returns the corrected role.
  - [ ] `dev-flow:review-code` run against this fix individually.
  - [ ] Full quality gate passes.

## Workstream C — doc-only cluster (no production code change; low risk of overlap with A or B)

### C.1 — FR-004 (P2): Document AD-6 as a wiring-order mitigation, not a structural fix

- **Depends on:** Architecture Phase 3 decision on documentation location (architecture.md vs. code comment, or both).
- **Files:** `architecture.md` (this spec's, and/or the original archived spec's) and/or a code comment on `defaultRoleForPlatform` in `internal/usecase/chat/service.go`.
- **Acceptance criteria:**
  - [ ] Documentation states that the `PlatformCLI → RoleAdmin` branch is dead-in-practice-but-not-dead-in-code, and that any new CLI entry point must go through `AuthCommandHandler.EnsureAuthenticated`/`reconcileIdentity` first or the shortcut re-arms.
  - [ ] `dev-flow:review-code` run against this change individually (documentation diff still reviewed, per PRD's binding process guidance).
- **Optional / non-blocking follow-up (does not gate this task's completion):** file a follow-up FR for a structural fix (refuse-to-run-without-authHandler, or change `defaultRoleForPlatform`'s default), if picked up.

### C.2 — FR-005 (P2): Correct spec's Observability NFR text; file tracked follow-up FR for failed-login audit logging

- **Depends on:** None — independent of Workstream A. (The PRD's Dependencies section notes FR-2 *satisfies* spec.md's line-157 criterion, it does not edit spec.md's text; only C.2 edits spec.md, so there is no A.1/C.2 file-conflict to coordinate.)
- **Files:** `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md` (Observability NFR text); new tracked FR (location per whichever spec/backlog mechanism is in use — not created by this task itself, only filed).
- **Note (explicit decision, not inherited from the PRD):** this spec's original spec.md now lives under `specs/archive/`, which — unlike the rest of this fix pass's `specs/` tree — is git-tracked. Editing it here means a tracked commit to a completed, archived artifact. That is the intended resolution (an NFR that overstates reality belongs corrected in the historical record), but it is a deliberate call being made now, not an assumption carried over unchanged from when the PRD was drafted against an active spec.md. See `implementation-notes.md` "Technical Decisions" (AD-F4) once this task starts.
- **Acceptance criteria:**
  - [ ] `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`'s Observability NFR text updated to accurately state that "consistent with the web admin's existing audit logging" currently means "consistent with the web admin's existing lack of failed-login audit logging."
  - [ ] A separate tracked FR is filed for adding failed-login audit logging to both `web/auth.go` and `cli/auth_commands.go` via the existing `internal/infrastructure/audit` package. (Whether it's one combined FR or two is left open per Research Question 3 — either satisfies this acceptance criterion.)
  - [ ] Both checkboxes required — text correction alone leaves the gap untracked; filing the FR alone leaves the NFR text overstating what exists.
  - [ ] `dev-flow:review-code` run against this change individually.

## No Workstream Required

### FR-006 (P2, informational) — no task, closes by acknowledgment

`readSessionFile`'s TOCTOU note is explicitly scoped out of AD-2's documented threat model. This finding is closed simply by this acknowledgment being on record — there is no checklist to complete and no code change gates it.

**Optional / non-blocking follow-up (does not gate this finding's completion):** `os.Open` + `Fstat` instead of `Stat` + separate `ReadFile`, closing the theoretical window at negligible cost — may be picked up separately but is not tracked as a task in this spec.

## Cross-Cutting

### X.1 — Final quality-gate pass across all workstreams

- **Depends on:** A.1, B.1, B.2, B.3, C.1, C.2 all complete.
- **Acceptance criteria:**
  - [ ] `go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help` passes with zero errors on the fully merged fix-pass branch.
  - [ ] All findings with acceptance criteria (spec.md) confirmed checked off (FR-006 has none — it closes by acknowledgment, see spec.md).
  - [ ] `status.md` updated to 100% across all phases.
