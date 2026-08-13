# Code Review PRD: CLI Parity for NuimanBot Features

**Reviewed branch:** `worktree-cli-parity`
**Reviewed against:** `origin/main` (`git diff origin/main...HEAD` — 42 files, ~7,300 insertions; local `main` ref was stale and excluded PR #8, so the scoped diff was computed against `origin/main` instead)
**Spec:** `specs/260811-cli-parity-for-nuimanbot-features/spec.md` (44 FRs + FR-044)
**Reviewer:** automated code review, `dev-flow:review-code` (pipeline Step 5)

## Executive Summary

This is a well-executed, carefully-scoped implementation. `go build`, `go vet`, `golangci-lint run`, and `go test ./...` all pass clean on the reviewed branch. The three security/architecture-critical claims this review was asked to scrutinize hold up under direct code inspection, not just commit-message trust:

- **AD-1 (AuthService extraction):** `internal/adapter/web/auth.go`'s `AuthService` is a genuine wrapper struct embedding `*auth.Service` — no duplicated credential/session logic. `bcryptCost=12`, `sessionTimeout=24h`, `sessionIDLength=32`, and `sessionCleanupInterval=5m` all moved into `internal/usecase/auth` byte-for-byte unchanged (confirmed via diff). `internal/adapter/gateway/cli` has zero imports of `internal/adapter/web` (grep-verified) — the hard Clean Architecture acceptance criterion holds.
- **AD-2 (session persistence):** `.nuimanbot_session` is written with `0600`, and stale looser-mode files are removed before rewrite (defends against `os.WriteFile` not tightening an existing file's mode). `RestoreSession` independently re-checks both expiry and username-existence and fails closed on either — verified in code and via dedicated tests (`TestEnsureAuthenticated_OverlyPermissiveSessionFileRejected`, `TestEnsureAuthenticated_DeletedUserRejectsRestore`, `TestEnsureAuthenticated_ExpiredSessionFallsBackToLogin`).
- **AD-6 (identity reconciliation):** `reconcileIdentity` runs on both the fresh-login and session-restore paths, before `Start()`'s REPL loop ever accepts input (a synchronous, blocking call — no race window). Verified that `AuthCommandHandler`'s `UserRoleService` and `chat.Service`'s `userService` are wired to the **same** `*user.Service` instance in `main.go` (`domainUserService` → `app.DomainUserService`, used at both call sites), so a reconciled record is visible to `chat.Service.resolveUser` on every subsequent message. Also confirmed the old unconditional `cli_admin`/`RoleAdmin` auto-grant (`SetCurrentUser` from `main.go`) has **no remaining production call site** — only test code calls `Gateway.SetCurrentUser` now.

**Zero P0 (true blocker) findings.** Two P1s were found: one is a direct, verifiable NFR violation newly reachable through this feature (`/memories browse`'s unbounded output); the other is a spec acceptance criterion (spec.md line 157) that was never actually satisfied by the reused usecase-layer code and was not verified this pass, despite the spec itself flagging it as needing an explicit test. Five P2s cover message-quality/consistency gaps, a documented-but-still-worth-stating "mitigation not a fix" architectural note, an observability gap that pre-dates this feature, a display-parity nit, and a TOCTOU note the spec's own threat model already scopes out.

**Total: 0 P0, 2 P1, 5 P2.**

---

## FR-1 (P1): `/memories browse` produces unbounded terminal output, violating the spec's own Performance NFR

**Files:** `internal/adapter/gateway/cli/memories_commands.go` (`browse`, lines 69–98)

`browse()` calls `h.service.ListCells(ctx, ownerUserID, memoryv2.MemoryCellFilter{})` with a zero-value filter. `memoryv2.MemoryCellFilter` has a `Limit int` field (`internal/domain/memoryv2/filter.go:20`, "maximum number of results to return, 0 = no limit") that is never set, and the handler then renders **every** returned cell's full `Content` (up to `memoryv2.MaxContentLength` = 2000 chars each) with no truncation or page cap. spec.md's own NFR states: "Performance: List commands (`/history list`, `/chat list`, etc.) paginate or truncate for large result sets rather than dumping unbounded output to the terminal." `/history list` (`historyListDisplayLimit = 20`) and `/history show` (`historyContentPreviewLimit = 4000`) both correctly implement this; `/memories browse` is the one list command in the feature that does not.

Separately (lower severity, folded into this finding rather than filed separately): the web equivalent (`internal/adapter/web/memories_handler.go:103`) explicitly sorts results most-recent-first via `sortCellsByCreatedAtDesc`; the CLI handler renders `ListCells`'s return order as-is with no sort, which is not guaranteed to match — a CLI-vs-web display-parity gap, not a correctness/security issue.

**Acceptance Criteria:**
- [ ] `/memories browse` caps the number of cells rendered per invocation (e.g. matching `historyListDisplayLimit`'s pattern) and/or truncates each cell's `Content` field for display, with a "N more not shown, refine your query" trailer matching `/history list`'s existing convention.
- [ ] Following this repo's mandatory Red-Green-Refactor cycle: write a failing test first that seeds more cells than the chosen limit and asserts the response is capped and includes the truncation notice; then implement; then refactor.

**Optional / non-blocking follow-up (does not gate this finding's completion):**
- `browse()` sorts results most-recently-created-first before rendering, matching `internal/adapter/web/memories_handler.go`'s `sortCellsByCreatedAtDesc`. Nice-to-have display-parity improvement, not required to close FR-1.

---

## FR-2 (P1): spec.md's own cross-user acceptance criterion for `/job create --project|--chat <id>` is not met, and was not verified this pass

**Files:** `internal/usecase/jobs/service.go` (`CreateJob`, lines 102–164) — untouched by this branch's diff, confirmed via `git diff origin/main...HEAD -- internal/usecase/jobs/service.go` (no output); `internal/adapter/gateway/cli/job_commands.go` (`create`, lines 118–143) — new CLI entry point that reaches this code path with a real, authenticated `ownerUserID` for the first time.

spec.md's Success Criteria (line 157) states explicitly: *"a `/job create --project <id>` (or `--chat <id>`) where `<id>` belongs to a Project/Chat the logged-in user does not own returns a not-found/permission error, never silently attaches to another user's Project/Chat. ... Needs an explicit acceptance test, not just reliance on the underlying scoping."*

Verified against the actual code: `CreateJob` only performs an ownership-scoped lookup for `ContextTypeProject` (`s.projectLookup.OutputDirectoryFor(ctx, ownerUserID, contextID)`), and when that lookup errors — which it does for a project ID belonging to another user, indistinguishably from a deleted/stale one — the error is silently swallowed (`if dir, err := ...; err == nil { workingDirectory = dir }`), `WorkingDirectory` is simply left `""`, and **Job creation still succeeds with `ContextID` set to the other user's Project ID.** This is deliberate, tested behavior for the stale-reference case (`TestCreateJob_StaleProjectReferenceStillCreatesJob`, "Edge Case #2"), but that test only proves the stale-ID case is handled gracefully — it does not distinguish "stale" from "exists but belongs to someone else," and no test asserts the latter is rejected. The job record's `ContextID`/`ContextType` (visible via `/job show <id>` and `/job list --project <id>`) then persists a reference to a Project the current user does not own.

`ContextTypeChat` is worse: `CreateJob` performs **no ownership check of any kind** for `--chat <id>` — not even the best-effort one Project gets — so a chat ID belonging to another user is stored as `ContextID` unconditionally.

No actual cross-user *data* is exposed here (`WorkingDirectory` for a non-owned project stays unresolved, so job execution can't read another user's files through this path) — this is a metadata/spec-conformance gap, not a file-access IDOR. It is filed as P1, not P0, on that basis. Confirmed directly in code: `writeJobSummary`/`writeJobDetail` (`job_commands.go:220-247`) echo back only the raw `ContextID` string the caller already supplied — never the referenced Project/Chat's title or content — and `CreateJob` succeeds identically whether `contextID` is a foreign-owned ID or one that doesn't exist at all, so there is no existence-oracle either. **This "no exposure" rationale is contingent on `StubExecutor` (`internal/infrastructure/scheduler/stub_executor.go`) being a placeholder that never reads Chat content and only checks Project *existence* (owner-scoped, so a foreign Project ID fails closed as "deleted," never returning its content) — when a real agent-invoking executor replaces the stub, an unowned `ContextTypeChat` reference becomes a live cross-user read path, and this finding must be re-rated at that time, not silently re-inherited as P1.** Because `internal/usecase/jobs/service.go` is reused as-is per this PRD's own Non-Goals/Dependencies section (spec.md line 141: "reused as-is per Non-Goals"), fixing this crosses that stated dependency boundary — see Dependencies section below — but the fix still belongs in this pass: spec.md's own line-157 acceptance criterion demands it and explicitly calls out that it "needs an explicit acceptance test," so re-scoping it away is not an option available to this PRD.

**Acceptance Criteria:**
- [ ] `jobs.Service.CreateJob` rejects (returns a not-found/permission error) a `--project`/`--chat` `contextID` that does not resolve to a Project/Chat owned by `ownerUserID`, distinguishing "not owned" from the already-handled "stale/deleted, owned" case. (Fix in usecase layer; do not re-scope as a follow-up — see justification above.)
- [ ] Following this repo's mandatory Red-Green-Refactor cycle (AGENTS.md): write the failing test first — mirroring `TestGetJob_CrossOwnerIsolation`'s pattern already used elsewhere in this file — asserting `/job create --project <id>` and `/job create --chat <id>` with an `<id>` owned by a different user is rejected, not silently created with an unresolved context; then implement; then refactor.
- [ ] `ContextTypeChat` gets the same ownership-scoping treatment as `ContextTypeProject` currently attempts (extending, not just fixing, the existing best-effort check).
- [ ] Add a code comment (or update the doc comment in `internal/infrastructure/scheduler/stub_executor.go`) flagging that `StubExecutor`'s replacement with a real executor must re-verify Chat-context ownership before reading Chat content, since this fix closes the CreateJob-time gap but a future executor could reopen an equivalent one at run time if it reads Chat data via `ContextID` without its own ownership check.

---

## FR-3 (P2): Deferred per-item "chat" subcommands (`/project chat`, `/job chat`, `/chore chat`, `/history chat`) are indistinguishable from a typo

**Files:** `internal/adapter/gateway/cli/project_commands.go:58-65`, `job_commands.go` (falls to `HandleJobCommand`'s `default:` case, line 73-74), `chore_commands.go:57-65`, `history_commands.go:61-63`

All four deferred FRs (FR-020, FR-026, FR-031, FR-036) fall through to each handler's generic unrecognized-subcommand branch, e.g. `"Unknown project command: chat\nUse '/project help' for usage information."`. Functionally this is correct — nothing partially works, nothing crashes, no misleading success — but the message is indistinguishable from a genuine typo (e.g. `/project chta`), so a user has no way to learn from the CLI itself that `chat` is a deliberately-deferred, known command rather than one that was never planned. This is inconsistent with the convention this same feature establishes for Settings' deferred subcommands (`retentionSetNotImplementedMessage`, `networkModeNotImplementedMessage` in `settings_commands.go`), which do give a clear, specific "not yet implemented" explanation.

**Acceptance Criteria:**
- [ ] Each of the four deferred `chat` subcommands returns a message that names it specifically (e.g. `"'/project chat' is not yet implemented — see spec.md FR-020. Use '/project help' for available commands."`), matching the Settings deferred-command convention, instead of falling through to the generic "Unknown command" branch.
- [ ] Following this repo's mandatory Red-Green-Refactor cycle: write a failing test per handler first, asserting the specific message text (not just that *some* non-crashing response is returned); then implement; then refactor.

---

## FR-4 (P2): AD-6 is a mitigation wired at `main.go`, not a structural fix — `defaultRoleForPlatform(PlatformCLI)` still returns `RoleAdmin`

**Files:** `internal/usecase/chat/service.go` (`defaultRoleForPlatform`, ~line 254); `internal/adapter/gateway/cli/gateway.go` (`platformUID`, `unauthenticatedPlatformUID`, lines 32-36, 602-613); `cmd/nuimanbot/main.go:1230-1233`

The actual RBAC landmine this feature was built to close — `internal/usecase/chat`'s `defaultRoleForPlatform(PlatformCLI) = RoleAdmin` auto-provisioning shortcut — is untouched by this branch; it still exists exactly as before. The fix is entirely a wiring-order guarantee: `Gateway.Start` blocks on `EnsureAuthenticated`/`reconcileIdentity` before entering its REPL loop, so `chat.Service.resolveUser` never hits the auto-create path for an authenticated identity. This is confirmed sound (see Executive Summary), but it is a mitigation that depends on every future CLI code path remembering to call the auth gate first — not a fix to the underlying `defaultRoleForPlatform` shortcut itself. The security property currently lives entirely in `main.go`'s `if app.DomainUserService == nil { os.Exit(1) }` guard and in `Gateway.authHandler` being non-nil; `Gateway.Start` explicitly documents (comment on `unauthenticatedPlatformUID`) that skipping authentication (e.g. `g.authHandler == nil`, which every existing unit test not exercising auth relies on) falls back to an unauthenticated identity that would still resolve through the same `defaultRoleForPlatform(PlatformCLI) = RoleAdmin` path if a message were ever processed through it. `main.go` does guard this correctly today (fatal exit if `DomainUserService` is nil), which is why this is P2 and not higher.

**Acceptance Criteria:**
- [ ] Document (in `architecture.md` or a code comment on `defaultRoleForPlatform`) that its `PlatformCLI → RoleAdmin` branch is dead-in-practice-but-not-dead-in-code, and that any new CLI entry point (a new command source, a background job, a future socket-server mode per the Non-Goals list) must go through `AuthCommandHandler.EnsureAuthenticated`/`reconcileIdentity` first or this shortcut re-arms.

**Optional / non-blocking follow-up (does not gate this finding's completion):**
- Either have `Gateway.Start` refuse to run at all when `authHandler == nil` outside of test builds, or change `defaultRoleForPlatform(PlatformCLI)` to a non-admin default now that a real reconciliation path exists — either would remove the landmine structurally instead of only procedurally. Track as a separate FR if picked up; not required to close FR-4.

---

## FR-5 (P2): Failed CLI login attempts are not logged/audited, and the NFR's own baseline ("existing web audit logging") doesn't fully exist either

**Files:** `internal/adapter/gateway/cli/auth_commands.go` (`login`, lines 165-199); `internal/adapter/web/auth.go` (`handleLogin`, lines 148-261); `internal/infrastructure/audit/logger.go`

spec.md's Observability NFR states: "Failed login attempts are logged/audited, consistent with the web admin's existing login rate-limiter and audit logging." Verified: `internal/adapter/gateway/cli/auth_commands.go` has no `slog` import and no logging of any kind on a failed `ValidateCredentials` call — only a per-process-invocation retry cap (`maxLoginAttempts = 3`), which resets on every re-run of the binary and is therefore not a meaningful rate limit. The web side does have a real per-IP rate limiter (`loginRateLimiterStore`, 5 attempts/minute), but on inspection it also does **not** call into `internal/infrastructure/audit` (the `AuditLogger` type exists in the codebase but is unused by `internal/adapter/web/auth.go`) — so the NFR's premise of "existing... audit logging" to be consistent with does not actually exist yet on the web side either. This is a pre-existing gap, not one this feature introduced, and the CLI is strictly better than the pre-feature state (previously: no password was required at all). Filed as P2 because it's a real, stated NFR miss, not because this feature made anything worse.

**Acceptance Criteria:**
- [ ] Update spec.md's Observability NFR text now (cheap, honest) to accurately reflect that "consistent with the web admin's existing... audit logging" currently means "consistent with the web admin's existing lack of failed-login audit logging," so the gap is tracked honestly rather than implied-resolved.
- [ ] File a separate tracked FR for adding failed-login audit logging to both `web/auth.go` and `cli/auth_commands.go` via the existing `internal/infrastructure/audit` package. Both checkboxes are required to close this finding — correcting the spec text alone leaves the underlying observability gap untracked as real follow-up work, and adding the FR alone leaves spec.md's NFR text overstating what already exists.

---

## FR-6 (P2): `readSessionFile`'s `Stat`-then-`ReadFile` has a theoretical TOCTOU window (spec-acknowledged, worth stating explicitly)

**Files:** `internal/adapter/gateway/cli/auth_commands.go` (`readSessionFile`, lines 291-317)

`readSessionFile` calls `os.Stat` to check permission bits, then makes a separate `os.ReadFile` call. Between the two, a local actor with write access to the session file's directory could in principle swap the file (e.g. via a symlink) after the permission check passes but before the content is read. The code's own doc comment on `RestoreSession` (`internal/usecase/auth/session.go:127-130`) explicitly scopes this out: *"not a defense against a malicious local user forging their own session file — that threat is explicitly out of scope per AD-2's decided threat model (the trust boundary is OS file permissions on the session file itself)."* Given that framing, a local actor with write access to that directory already has equivalent access to the running process's own user, so this is not a practical escalation. Filed as P2/informational to close out the review checklist's explicit TOCTOU question, not because it represents an unaddressed risk under the stated threat model.

**Acceptance Criteria:**
- [ ] None. This finding is informational and does not require a code change to close — it exists to close out the review checklist's explicit TOCTOU question against the documented threat model.

**Optional / non-blocking follow-up (does not gate this finding's completion):**
- `readSessionFile` could open the file once (`os.Open` + `Fstat` on the resulting handle) rather than `Stat` + separate `ReadFile`, closing the theoretical window at negligible cost — optional hardening, not a correctness requirement under the documented threat model.

---

## FR-7 (P2): `TestEnsureAuthenticated_RestoreValidSession` doesn't exercise the stale-role-on-restore path end-to-end

**Files:** `internal/adapter/gateway/cli/auth_commands_test.go` (`TestEnsureAuthenticated_RestoreValidSession`, lines 185-214; `TestReconcileIdentity_UpdatesStaleRole`, line 358)

`reconcileIdentity`'s "update a stale role" behavior is well-tested when called directly (`TestReconcileIdentity_UpdatesStaleRole`), and the restore path is well-tested for the "already-consistent role" case (`TestEnsureAuthenticated_RestoreValidSession` pre-seeds a `domain.User` whose role already matches the session). No test exercises `EnsureAuthenticated`'s **restore** path specifically with a *mismatched* pre-seeded role, to confirm the update happens end-to-end through the public entry point rather than only through the internal method in isolation. Low severity: it's the same code path (`reconcileIdentity` is called identically from both branches of `EnsureAuthenticated`), so this is a test-completeness nit, not a coverage gap that hides a real behavioral risk.

**Acceptance Criteria:**
- [ ] Add a test that pre-seeds a `domain.User` for `(PlatformCLI, "alice")` with a stale role, writes a valid session file whose `Role` differs, and asserts `EnsureAuthenticated`'s **restore** path (not `reconcileIdentity` called directly) returns the corrected role.

---

## Fix Process Guidance

This section governs how the fix pass driven by this PRD must be carried out. It is binding, not advisory — a fix that skips any of these steps is not complete, regardless of whether its acceptance criteria checkboxes are ticked.

**TDD (Red-Green-Refactor, mandatory per AGENTS.md):** For every finding above with a code change, follow the full cycle, not just "add a test":
1. **Red** — write the failing test named in the finding's acceptance criteria first; confirm it fails for the right reason.
2. **Green** — write the minimal code change to pass it.
3. **Refactor (mandatory, not optional)** — clean up the change (naming, duplication, structure) with tests kept green throughout. Do not skip this phase; AGENTS.md is explicit that moving on without it is not permitted.

**Code review per fix, not one review at the end:** Run `dev-flow:review-code` against each finding's fix individually before moving to the next finding, rather than batching all seven fixes and reviewing once at the end. This keeps a bad fix from compounding into the next one and keeps each review's diff small enough to actually verify.

**Quality gates (must pass after every individual fix, not just once at the end of the pass):**
```
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```
All must succeed with zero errors before a finding is marked resolved.

**Agent teammates and git worktrees for parallel workstreams:** These seven findings partition into independent workstreams that can run in parallel, each in its own git worktree:
- **Workstream A (FR-2, P1):** `internal/usecase/jobs/service.go` + its tests. The largest and highest-risk fix — give it its own worktree and, if using agent teammates, its own dedicated teammate. Touches the usecase layer, which the base spec's Non-Goals/Dependencies section (spec.md line 141) says is reused as-is — flag this crossing explicitly in that teammate's commit/PR description.
- **Workstream B (FR-1, FR-3, FR-7, all P2 + one P1):** All live in `internal/adapter/gateway/cli/`, but touch distinct files (`memories_commands.go`; `project_commands.go`/`job_commands.go`/`chore_commands.go`/`history_commands.go`; `auth_commands_test.go`) — safe to parallelize across teammates or run sequentially in one worktree if headcount is limited.
- **Workstream C (FR-4, FR-5, doc/spec-text only, both P2):** No production code change required for either — `architecture.md`/code-comment documentation (FR-4) and spec.md NFR text plus a new tracked FR (FR-5). Group as one workstream; low risk of touching the same lines as A or B.
- **FR-6 (P2, informational):** No fix required (see its Acceptance Criteria); does not need a workstream. Only pick up if the optional hardening follow-up is chosen.

Per AGENTS.md, do not specify a non-default model when spawning teammates for these workstreams unless the user explicitly requests one.

## Dependencies

- **`internal/usecase/jobs` (FR-2):** spec.md line 141 states this package is "reused as-is per Non-Goals." FR-2's fix requires a real code change inside it (see FR-2's Acceptance Criteria), which crosses that stated boundary. This is a deliberate, necessary exception — spec.md's own line-157 acceptance criterion requires the behavior this fix delivers — but it should be called out in the fix's PR description so it isn't mistaken for scope creep.
- **`internal/infrastructure/audit`:** FR-5's tracked follow-up FR depends on this existing package; no new infrastructure is required, only wiring it into `web/auth.go` and `cli/auth_commands.go`.
- **spec.md:** FR-5 and FR-2 both require edits to `specs/260811-cli-parity-for-nuimanbot-features/spec.md` (NFR text correction for FR-5; the line-157 acceptance criterion is satisfied, not changed, by FR-2). Coordinate these edits with whichever workstream touches spec.md last to avoid a merge conflict on the same file.

## Open Questions

- **FR-4's optional follow-up** (refuse-to-run-without-authHandler vs. changing `defaultRoleForPlatform`'s default): which structural fix is preferred, if either is picked up, is not decided by this PRD — left to whoever picks up that follow-up FR.
- **FR-5's tracked follow-up FR:** should CLI and web failed-login audit logging ship as one combined FR or two independent ones (they touch different files and have no shared code path apart from the `audit` package)? Not decided here.
- **FR-2's Chat-context ownership check:** should it reuse the same best-effort "swallow lookup error, leave field unresolved" pattern `ContextTypeProject` uses today, or should Chat get a harder rejection (return an error to the CLI immediately) since, unlike Project, there is currently no "stale/deleted" tolerance test establishing a soft-fail precedent for Chat? Left to the implementing workstream to decide and document, since spec.md's line-157 criterion only requires rejection, not a specific failure mode.
- **StubExecutor's eventual replacement:** FR-2's justification notes that a real executor reading Chat content via `ContextID` would need its own ownership re-check. No FR in this PRD tracks that follow-up work explicitly (it doesn't exist yet); flagged here so it isn't lost when the stub is eventually replaced.

---

## Positive Observations

- Consistent, correct use of the `ownerUserID` (never `currentUser.ID`) scoping convention across all seven environment handlers (Chats, Projects, Jobs, Chores, History, Memories, Settings) — spot-checked all seven, not just three.
- Strong cross-owner isolation test coverage at the usecase layer, predating and reused by this feature: `TestGetJob_CrossOwnerIsolation`, `TestDeleteJob_CrossOwnerIsolation`, `TestGetProject_CrossOwnerIsolation`, `TestDeleteProject_CrossOwnerIsolation`, `TestAddAgentsFile_CrossOwnerIsolation`, `TestGetChore_CrossOwnerReturnsNotFound`, `TestDeleteChore_CrossOwnerReturnsNotFoundAndDoesNotDelete`, `TestConfirmSchedule_CrossOwnerReturnsNotFound`, `TestGetChat_CrossOwnerIsolation`, `TestDeleteChat_CrossOwnerIsolation`, `TestAppendUserMessage_CrossOwnerIsolation`, `TestExportChat_CrossOwnerIsolation`.
- The `/memory` (singular, admin) vs `/memories` (plural, per-user) prefix non-collision (AD-3) is correctly implemented and clearly documented in `isEnvCommand`'s doc comment.
- The `--help`/empty-stdin exit-code change (0 → 1) is a real, intentional, and clearly documented behavioral change (`support_docs/cli-environments-guide.md:49`), correctly scoped to the username-prompt-EOF case only — post-login EOF still exits 0 via `Start`'s existing `scanner.Scan()` handling, unchanged.
- `writeSessionFile` correctly removes a pre-existing file before rewriting with `0600`, defending against `os.WriteFile`'s mode argument not tightening an already-existing looser-mode file — a subtle correctness detail that's easy to get wrong, and is directly tested (`TestWriteSessionFile_TightensExistingLoosePermissions`).
- The `AuthService` wrapper-struct decision (not a type alias) is well-justified and documented in-line, with the exact compile failure it avoids named in the comment.
- `go build`, `go vet`, `golangci-lint run` (0 issues), and `go test ./...` all pass clean on the reviewed branch as of this review.

---

## Summary

| Priority | Count | Findings |
|---|---|---|
| P0 | 0 | — |
| P1 | 2 | FR-1 (`/memories browse` unbounded output), FR-2 (job-create unowned project/chat context) |
| P2 | 5 | FR-3 (deferred-chat messaging), FR-4 (AD-6 mitigation-not-fix documentation), FR-5 (failed-login audit logging), FR-6 (TOCTOU note), FR-7 (test-completeness nit) |

**P0/P1 sanity check (re-verified this pass):** FR-2 was independently re-checked against `internal/usecase/jobs/service.go`, `internal/infrastructure/scheduler/stub_executor.go`, and `internal/adapter/gateway/cli/job_commands.go` before accepting the P1 call. Confirmed: (1) `writeJobSummary`/`writeJobDetail` echo back only the raw `ContextID` the caller already supplied, never the referenced Project/Chat's actual title or content; (2) `CreateJob` succeeds identically for a nonexistent ID and a foreign-owned one, so there is no existence-oracle; (3) the one place a Project reference is actually dereferenced at runtime (`StubExecutor.checkProjectExists`) is itself owner-scoped, so a foreign Project ID fails closed as "deleted" rather than returning its content. No true file- or content-level IDOR exists today — **P1, not P0, is correct**, but is explicitly contingent on `StubExecutor` remaining a non-content-reading stub (see FR-2's justification and the Open Questions/Dependencies sections above for what changes that). No findings were found to be under-prioritized; 0 P0 is accurate for the code as it exists on this branch today.

**Non-blocking follow-ups called out separately (not part of the P0/P1/P2 counts above, and do not gate any finding's completion):** FR-1's optional sort-order match, FR-4's optional structural-fix follow-up, and FR-6's optional TOCTOU hardening — see each finding's "Optional / non-blocking follow-up" subsection.
