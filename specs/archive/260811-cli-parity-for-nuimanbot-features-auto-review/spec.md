# Spec: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Spec directory:** `specs/260811-cli-parity-for-nuimanbot-features-auto-review/`
**Source PRD:** `specs/260811-cli-parity-for-nuimanbot-features-auto-review/cli-parity-for-nuimanbot-features-auto-review-PRD.md`
**Prior feature spec (archived):** `specs/archive/260811-cli-parity-for-nuimanbot-features/`

## Executive Summary

This spec drives the fix pass for 7 findings (0 P0, 2 P1, 5 P2) raised by an automated code review (`dev-flow:review-code`, pipeline Step 5) of the `worktree-cli-parity` branch against `origin/main`. The reviewed branch is well-executed and all three security/architecture-critical claims it was asked to scrutinize (AD-1 AuthService extraction, AD-2 session persistence, AD-6 identity reconciliation) hold up under direct code inspection. No blocking (P0) issues were found. The two P1s are: unbounded terminal output from `/memories browse` (a stated NFR violation), and a spec acceptance criterion for `/job create --project|--chat <id>` cross-user context ownership that is not currently enforced. The five P2s cover message-quality/consistency, a "mitigation not a structural fix" documentation gap, missing failed-login audit logging, an explicitly out-of-scope TOCTOU note, and a test-completeness nit.

## Problem Statement

The CLI-parity feature (archived spec `specs/archive/260811-cli-parity-for-nuimanbot-features/`) shipped with `go build`, `go vet`, `golangci-lint run`, and `go test ./...` all passing, but a subsequent code review surfaced 7 concrete gaps between the implementation and the spec's own stated acceptance criteria and NFRs. These gaps must be closed (or explicitly documented/accepted) in a dedicated, tracked fix pass rather than folded silently into future feature work.

## Goals / Non-Goals

**Goals:**
- Close both P1 findings (FR-1, FR-2) with code changes and Red-Green-Refactor-disciplined tests.
- Close the P2 findings that require code/doc/spec changes (FR-3, FR-4, FR-5, FR-7).
- Formally close FR-6 as informational/no-action, per its own acceptance criteria.
- Run `dev-flow:review-code` per individual fix, not once at the end.
- Keep all quality gates green after every individual fix.

**Non-Goals:**
- Re-opening or re-litigating AD-1/AD-2/AD-6, which this review already confirmed sound.
- Implementing a real (non-stub) job executor — FR-2's fix only closes the `CreateJob`-time gap; a future executor replacing `StubExecutor` must independently re-verify Chat-context ownership at read time (flagged, not solved, here — see Open Questions).
- Deciding between FR-4's two optional structural-fix alternatives (refuse-to-run-without-authHandler vs. changing `defaultRoleForPlatform`'s default) — left to whoever later picks up that optional follow-up.
- Deciding whether FR-5's tracked follow-up FR (failed-login audit logging) ships as one combined FR or two — this spec only requires that the follow-up FR be filed and spec.md's NFR text corrected, not that the audit-logging feature itself be implemented in this pass.

## User Requirements

### FR-001 (P1, source PRD FR-1): `/memories browse` must cap/truncate output

`/memories browse` (`internal/adapter/gateway/cli/memories_commands.go`, `browse()`, lines 69–98) calls `ListCells` with a zero-value `MemoryCellFilter`, never setting `Limit`, and renders every returned cell's full `Content` (up to `MaxContentLength` = 2000 chars each) with no truncation or page cap. This violates spec.md's own Performance NFR ("List commands ... paginate or truncate for large result sets"), which `/history list`/`/history show` already correctly implement.

**Acceptance Criteria:**
- [ ] `/memories browse` caps the number of cells rendered per invocation (matching `historyListDisplayLimit`'s pattern) and/or truncates each cell's `Content`, with a "N more not shown, refine your query" trailer matching `/history list`'s convention.
- [ ] Red-Green-Refactor: failing test first (seed more cells than the limit, assert capped output + truncation notice), then implement, then refactor.

**Optional / non-blocking follow-up (does not gate this finding's completion):** sort `browse()` results most-recently-created-first to match `internal/adapter/web/memories_handler.go`'s `sortCellsByCreatedAtDesc` (display-parity nice-to-have).

### FR-002 (P1, source PRD FR-2): `/job create --project|--chat <id>` must reject foreign-owned context IDs

spec.md's Success Criteria (line 157 of the original spec) requires that a `--project`/`--chat <id>` belonging to a Project/Chat the logged-in user does not own returns a not-found/permission error, never silently attaches. `jobs.Service.CreateJob` (`internal/usecase/jobs/service.go`, lines 102–164) only performs a best-effort, ownership-scoped lookup for `ContextTypeProject`, and silently swallows the lookup error (leaving `WorkingDirectory` empty) rather than rejecting the job — so job creation still succeeds with `ContextID` set to another user's Project. `ContextTypeChat` gets **no** ownership check at all. No content-level IDOR exists today (confirmed: `writeJobSummary`/`writeJobDetail` never echo the referenced Project/Chat's title/content, and `StubExecutor.checkProjectExists` is owner-scoped and fails closed) — this is a metadata/spec-conformance gap, not a file-access exploit today, which is why it is P1 and not P0. **This "no exposure" rationale is explicitly contingent on `StubExecutor` remaining a non-content-reading placeholder** — when a real executor replaces the stub, an unowned `ContextTypeChat` reference becomes a live cross-user read path and this finding must be re-rated at that time.

**Acceptance Criteria:**
- [ ] `jobs.Service.CreateJob` rejects (not-found/permission error) a `--project`/`--chat` `contextID` that does not resolve to a Project/Chat owned by `ownerUserID`, distinguishing "not owned" from the already-handled "stale/deleted, owned" case.
- [ ] Red-Green-Refactor: failing test first, mirroring `TestGetJob_CrossOwnerIsolation`'s pattern, asserting both `--project` and `--chat` with a foreign-owned `<id>` are rejected — then implement, then refactor.
- [ ] `ContextTypeChat` gets the same ownership-scoping treatment as `ContextTypeProject` currently attempts (extended, not just matched).
- [ ] Add a code comment (in `internal/infrastructure/scheduler/stub_executor.go` or nearby) flagging that a future real executor must re-verify Chat-context ownership before reading Chat content.
- [ ] This fix crosses spec.md line 141's stated "`internal/usecase/jobs` reused as-is per Non-Goals" boundary — call this out explicitly in the fix's commit/PR description, not as scope creep but as a deliberate, PRD-required exception.

### FR-003 (P2, source PRD FR-3): Deferred `chat` subcommands must be distinguishable from typos

All four deferred FRs (FR-020, FR-026, FR-031, FR-036 of the original spec) — `/project chat`, `/job chat`, `/chore chat`, `/history chat` — fall through to each handler's generic "Unknown command" branch, indistinguishable from a genuine typo. This is inconsistent with the Settings deferred-subcommand convention (`retentionSetNotImplementedMessage`, `networkModeNotImplementedMessage`), which gives a clear "not yet implemented" explanation.

**Acceptance Criteria:**
- [ ] Each of the four deferred `chat` subcommands returns a specific "not yet implemented — see spec.md FR-0XX" message, matching the Settings convention, instead of the generic "Unknown command" branch.
- [ ] Red-Green-Refactor: failing test per handler asserting the specific message text, then implement, then refactor.

### FR-004 (P2, source PRD FR-4): Document that AD-6 is a wiring-order mitigation, not a structural fix

`chat.Service.defaultRoleForPlatform(PlatformCLI)` still returns `RoleAdmin`, untouched by this branch. The fix is entirely a wiring-order guarantee (`Gateway.Start` blocks on `EnsureAuthenticated`/`reconcileIdentity` before the REPL loop). This is confirmed sound but depends on every future CLI code path remembering to call the auth gate first.

**Acceptance Criteria:**
- [ ] Document (in `architecture.md` or a code comment on `defaultRoleForPlatform`) that the `PlatformCLI → RoleAdmin` branch is dead-in-practice-but-not-dead-in-code, and that any new CLI entry point must go through `AuthCommandHandler.EnsureAuthenticated`/`reconcileIdentity` first or the shortcut re-arms.

**Optional / non-blocking follow-up (does not gate this finding's completion):** track a follow-up FR for a structural fix (refuse-to-run-without-authHandler, or change `defaultRoleForPlatform`'s default); see Open Questions.

### FR-005 (P2, source PRD FR-5): Failed CLI logins are not audited; spec's NFR premise doesn't fully hold either

`internal/adapter/gateway/cli/auth_commands.go`'s `login` has no logging on failed `ValidateCredentials` calls — only a per-process retry cap that resets on every binary restart. The web side has a real rate limiter but also does not call into `internal/infrastructure/audit`, so spec.md's Observability NFR ("consistent with the web admin's existing... audit logging") describes something that doesn't fully exist yet on the web side either. Pre-existing gap, not introduced by this feature; CLI is strictly better than pre-feature state (previously no password required at all).

**Acceptance Criteria:**
- [ ] Update spec.md's (original spec, `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`) Observability NFR text to accurately state that "consistent with the web admin's existing audit logging" currently means "consistent with the web admin's existing lack of failed-login audit logging."
- [ ] File a separate tracked FR for adding failed-login audit logging to both `web/auth.go` and `cli/auth_commands.go` via the existing `internal/infrastructure/audit` package. Both checkboxes required — correcting the text alone leaves the gap untracked; filing the FR alone leaves the NFR text overstating what exists.

### FR-006 (P2, informational, source PRD FR-6): TOCTOU note in `readSessionFile` — no action required

`readSessionFile`'s `Stat`-then-`ReadFile` has a theoretical TOCTOU window, explicitly scoped out of AD-2's documented threat model (trust boundary is OS file permissions on the session file; a local actor able to exploit the window already has equivalent access to the process's own user).

**Acceptance Criteria:** None. This finding closes simply by this acknowledgment being on record against the documented threat model — there is no checklist item to complete and no code change gates completion.

**Optional / non-blocking follow-up (does not gate this finding's completion):** `os.Open` + `Fstat` on the handle instead of `Stat` + separate `ReadFile`, closing the theoretical window at negligible cost.

### FR-007 (P2, source PRD FR-7): Add end-to-end test for stale-role-on-restore

`TestReconcileIdentity_UpdatesStaleRole` tests the stale-role-update behavior directly; `TestEnsureAuthenticated_RestoreValidSession` only covers the already-consistent-role case. No test exercises `EnsureAuthenticated`'s restore path specifically with a mismatched pre-seeded role through the public entry point.

**Acceptance Criteria:**
- [ ] Add a test pre-seeding a `domain.User` for `(PlatformCLI, "alice")` with a stale role, a valid session file whose `Role` differs, asserting `EnsureAuthenticated`'s restore path (not `reconcileIdentity` called directly) returns the corrected role.

## Non-Functional Requirements

- **Performance:** FR-001's fix must not regress `/memories browse`'s response time; capping/truncation should reduce, not increase, work done per invocation.
- **Testing discipline:** Every finding with a code change follows strict Red-Green-Refactor (AGENTS.md, mandatory) — no test-added-after-the-fact.
- **Review discipline:** `dev-flow:review-code` runs per individual fix, not batched at the end of the pass.
- **Quality gates:** `go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help` must pass with zero errors after every individual fix, not just once at the end.

## System Architecture

**Affected layers:**
- **Usecase layer** (`internal/usecase/jobs`): FR-002 requires a real code change here — the one place this fix pass crosses the original spec's "reused as-is" Non-Goals boundary (spec.md line 141). Deliberate and required by spec.md's own line-157 acceptance criterion; call out explicitly in commit/PR description.
- **Adapter/CLI layer** (`internal/adapter/gateway/cli/`): FR-001 (`memories_commands.go`), FR-003 (`project_commands.go`, `job_commands.go`, `chore_commands.go`, `history_commands.go`), FR-007 (`auth_commands_test.go`).
- **Infrastructure layer** (`internal/infrastructure/audit`, `internal/infrastructure/scheduler`): FR-002's stub-executor doc comment; FR-005's follow-up FR depends on the existing `audit` package (no new infrastructure required).
- **Documentation:** FR-004 (architecture.md or code comment), FR-005 (spec.md NFR text + new tracked FR).

**No new components are introduced.** This is entirely a remediation pass against existing components from the archived CLI-parity spec.

## Scope of Changes

**Files to modify:**
- `internal/usecase/jobs/service.go` — FR-002 (ownership check for Project and Chat context IDs)
- `internal/usecase/jobs/service_test.go` (or equivalent) — FR-002 tests
- `internal/adapter/gateway/cli/memories_commands.go` — FR-001
- `internal/adapter/gateway/cli/memories_commands_test.go` (or equivalent) — FR-001 tests
- `internal/adapter/gateway/cli/project_commands.go`, `job_commands.go`, `chore_commands.go`, `history_commands.go` — FR-003
- corresponding `*_test.go` files — FR-003 tests
- `internal/adapter/gateway/cli/auth_commands_test.go` — FR-007
- `internal/infrastructure/scheduler/stub_executor.go` — FR-002 doc comment
- `architecture.md` and/or a code comment on `defaultRoleForPlatform` — FR-004 (no spec.md edit; documentation-only)
- `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md` — FR-005 only (Observability NFR text correction; this is the *archived* prior spec's file, which is git-tracked unlike this active spec's own directory — see `implementation-notes.md` AD-F4 for the explicit decision to edit a tracked archived artifact)

**Dependencies:**
- `internal/usecase/jobs` (FR-002): crosses the original spec's "reused as-is" Non-Goals boundary — deliberate exception, not scope creep. FR-002 *satisfies* the original spec.md's line-157 acceptance criterion; it does not require editing spec.md's text, so it has no file-level dependency on FR-005.
- `internal/infrastructure/audit` (FR-005 follow-up FR): existing package, no new infrastructure needed.
- Only FR-005 edits the original spec.md — no cross-workstream coordination is needed on that file (FR-002/Workstream A and FR-005/Workstream C are independent).

## Breaking Changes

None. All fixes are behavior corrections or documentation additions; no public API, config, or schema changes.

## Success Criteria and Acceptance Criteria

- 0 P0, 2 P1 (FR-001, FR-002), 5 P2 (FR-003 through FR-007) findings all closed per their individual acceptance criteria above.
- Full Red-Green-Refactor cycle followed and documented for every finding with a code change.
- `dev-flow:review-code` run and passing per individual fix.
- All quality gates green after every individual fix and at the end of the pass.
- `status.md` updated after every task/phase completion (mandatory per AGENTS.md).

## Risks and Mitigation

| Risk | Mitigation |
|---|---|
| FR-002's usecase-layer change (crossing the "reused as-is" boundary) introduces a regression in existing job-creation flows | Mandatory Red-Green-Refactor with a failing test mirroring `TestGetJob_CrossOwnerIsolation`'s established pattern before any implementation change; full quality gate + `dev-flow:review-code` before merging. |
| FR-005's edit lands on a git-tracked archived spec file, which could be mistaken for scope creep or accidentally reverted | Documented explicitly as a deliberate decision (see Dependencies above and `implementation-notes.md` AD-F4); only FR-005/Workstream C touches this file, so no cross-workstream coordination is required. |
| FR-004/FR-005 documentation-only fixes get deprioritized as "not real work" | Both are gating findings (P2, not optional) with explicit acceptance criteria; tracked equally in tasks.md alongside code fixes. |
| Parallel workstreams (A/B/C) drift out of sync on shared conventions (e.g., truncation-trailer wording) | FR-001 and FR-003 both reference `/history list`'s existing trailer convention as the pattern to match — use it as the single source of truth. |

## Timeline and Milestones

[TBD] — not specified in source PRD beyond the Fix Process Guidance's workstream parallelization plan (see plan.md/tasks.md for workstream breakdown and dependency ordering).

## References

- Source PRD: `specs/260811-cli-parity-for-nuimanbot-features-auto-review/cli-parity-for-nuimanbot-features-auto-review-PRD.md`
- Original (archived) feature spec: `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`
- AGENTS.md — Red-Green-Refactor discipline, quality gates, spec workflow rules
