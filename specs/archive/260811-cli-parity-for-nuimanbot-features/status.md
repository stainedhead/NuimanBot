# Status: CLI Parity for NuimanBot Features

**Feature:** cli-parity-for-nuimanbot-features
**Created:** 2026-08-11
**Spec directory:** `specs/260811-cli-parity-for-nuimanbot-features/`

## Overall Progress

| Phase | Description | Status | Progress |
|---|---|---|---|
| 0 | Spec Creation | Complete | 100% |
| 1 | Research | Complete | 100% |
| 2 | Data Dictionary | Complete | 100% |
| 3 | Architecture | Complete | 100% |
| 4 | Planning | Complete | 100% |
| 5 | Task Breakdown | Complete | 100% |
| 6 | Implementation | Complete | 100% |

**Phases 1–5 were completed during pipeline Step 2 (`dev-flow:review-spec`)**, not left as architecture-phase placeholders. Per this pipeline's operating mode (no human available mid-pipeline for follow-ups), all Warnings/Fails found during review were resolved directly in the spec files rather than flagged and left open. See "Step 2 Review Outcome" below for what was found and changed, and `architecture.md`/`spec.md`/`data-dictionary.md`/`tasks.md`/`research.md`/`implementation-notes.md` for the resolutions themselves.

## Phase 0: Spec Creation — Task Checklist

- [x] Spec directory created: `specs/260811-cli-parity-for-nuimanbot-features/`
- [x] PRD moved into spec directory
- [x] `spec.md` populated from PRD (Executive Summary, Problem Statement, Goals/Non-Goals, FRs, NFRs, Architecture stub, Scope, Breaking Changes, Acceptance Criteria, Risks)
- [x] `research.md` seeded with research questions derived from PRD's Open Questions and Dependencies/Risks
- [x] `data-dictionary.md`, `architecture.md`, `plan.md`, `tasks.md`, `implementation-notes.md` initialized with placeholder structure
- [x] `status.md` marked Phase 0 fully complete; ready for pipeline Step 2 — Review Spec

## Phase 6: Implementation — Task Checklist (all 19 tasks, tasks.md Phases A–E)

- [x] P1.1 — Extract `internal/usecase/auth` package (AD-1)
- [x] P1.2 — `RestoreSession` with defense-in-depth hardening (AD-2)
- [x] P2.1 — Login prompt and fresh-login flow
- [x] P2.2 — Session restore on process restart, with fallback
- [x] P2.3 — Identity reconciliation with `domain.User`/`userService` (AD-6)
- [x] P2.4 — `/logout` command
- [x] P2.5 — Replace auto-admin grant and hardcoded chat attribution
- [x] P2.6 — Role-gate existing admin commands
- [x] P3.1 — Chats
- [x] P3.2 — Projects
- [x] P3.3 — Jobs
- [x] P3.4 — Chores
- [x] P3.5 — History
- [x] P3.6 — Memories
- [x] P4.1 — Per-user Settings (partial — `/settings set retention` deferred, see below)
- [x] P4.2 — Admin-only Settings (partial — `/settings set network-mode` deferred, see below)
- [x] P5.1 — Deprecation note for auto-admin removal
- [x] P5.2 — Update `documentation/` product docs
- [x] P5.3 — Final quality-gate pass

## Blockers

None remaining. Two gaps were found and resolved mid-implementation (see "Step 3 Implementation Outcome" below); both went through a coordinator decision rather than being guessed at, and are fully documented in `spec.md` and `implementation-notes.md`.

## Step 2 Review Outcome (`dev-flow:review-spec`)

**Verdict: Implementation-ready.** Reviewed against the dimension table in `dev-flow:review-spec`: Acceptance criteria (Pass, after correcting one wrong package path and four deferred FRs), Technical approach (Pass, after replacing AD-1's non-compiling type-alias draft), Component coverage (Pass), Edge case handling (Pass, after adding the cross-user-Project edge case and the RBAC identity-bridge gap), Open questions (Pass — all three carried-forward questions resolved, none left open), Out-of-scope clarity (Pass, Non-Goals extended to cover the newly-descoped chat sub-commands), Status initialization (Pass).

No human was available mid-pipeline for follow-ups (per this pipeline's operating mode), so every gap found below was resolved directly in the spec files during this review, not left as a flagged Warning/Fail for a later step.

**Six concrete gaps found by reading the actual code (not just the PRD's description of it), all resolved:**

1. **AD-1's type-alias draft does not compile.** `type AuthService = auth.Service` breaks `internal/adapter/web/auth_coverage_test.go`'s white-box tests (unexported method/field access) and two production call sites. Replaced with a wrapper struct embedding `*auth.Service`, with unexported shim methods declared in `web` so existing lowercase-named tests keep compiling unmodified — except two test functions (`TestSessionExpiry`, `TestCleanupExpiredSessions`) that relocate verbatim into the new package, a documented, narrow exception. See `architecture.md` AD-1.
2. **AD-2's original "re-derive Role at restore time" framing mischaracterized a correctness fix as a security fix.** It does not stop a malicious local user from forging their own session file's `username` field. Replaced with a decided threat-model statement (OS-user-account trust boundary) plus defense-in-depth hardening (`RestoreSession` independently re-validates expiry and username-existence). See `architecture.md` AD-2.
3. **A second, independent "CLI = trusted = admin" shortcut** exists in `internal/usecase/chat/service.go`'s `defaultRoleForPlatform(PlatformCLI) = RoleAdmin`, untouched by anything else in the original architecture draft — would have silently granted every CLI user RBAC admin privileges on their first chat message regardless of their real login role. Resolved as new AD-6 (identity reconciliation), added as task P2.3.
4. **The `ownerUserID` convention (session's `Username`, not `ID`) was undocumented** despite being load-bearing for the "data visible in both CLI and web" acceptance criterion. Verified by grep across all six existing web handlers; documented as new AD-5.
5. **Three of five per-item "chat" sub-commands (FR-020, FR-026, FR-031, FR-036) had no backing web-side implementation to mirror** — verified against `internal/adapter/web/{projects,jobs,chores,history}_handler.go`, three of which have explicit pre-existing "out of scope for this pass" comments. Building them would have violated the PRD's own Non-Goal. Marked DEFERRED in `spec.md`; `/memories chat` (FR-038) is unaffected, it has a real backing method (`AskAboutCell`).
6. **A hard acceptance criterion referenced the wrong package** (`internal/adapter/cli`, an unrelated pre-existing package, instead of `internal/adapter/gateway/cli`, the package this feature actually modifies) — as originally written the check was trivially, meaninglessly true. Corrected in `spec.md`.

**Mechanical completeness work also done:** `tasks.md` expanded from one stub task to a full 19-task breakdown across Phases A–E; `research.md`'s three open questions all pinned (24h session expiry matching web's existing constant; `golang.org/x/term` for password masking, confirmed absent from `go.mod`/`go.sum` and needs adding; network-mode question deleted as already superseded by FR-043's decision); `data-dictionary.md` filled in with the concrete `auth.Service` method surface, session-file JSON schema, and the `Session.Role string` → `domain.Role` conversion point; `plan.md`'s TBDs (Phase Breakdown, Testing Strategy, Rollout Strategy, Success Metrics) resolved.

## Step 3 Implementation Outcome (`dev-flow:implm-frm-prd`)

**All 19 tasks complete.** Full quality-gate chain (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...`, `go build -o bin/nuimanbot ./cmd/nuimanbot`, `./bin/nuimanbot --help`) passes with zero errors; verified end-to-end against the built binary (login success/failure/retry-exhaustion, session restore/expiry/corruption, `/logout`, and the new environment commands all manually exercised through real piped stdin, not just unit tests).

**Two more gaps found by reading the actual code, same pattern as Step 2's six — both surfaced to the coordinator for a decision rather than guessed at:**

1. **FR-039/040's "per-user retention" has no backing capability anywhere** — `internal/domain/preferences.go`'s `UserPreferences` has no retention fields, and `internal/adapter/web/settings_handler.go`'s own doc comment states per-user retention override storage is deferred (system-wide/admin-only today). Decision: defer `/settings set retention` and FR-039's per-user "set" half, same DEFERRED treatment as FR-020/026/031/036; `/settings show` displays the system-wide default read-only instead.
2. **FR-043's `/settings set network-mode` has no shared state the CLI can reach** — `domain.NetworkAccessConfig` is private to `*web.Server`, and the CLI must not import `internal/adapter/web`. This PRD's own Open Questions section had already flagged this exact doubt, tied to FR-R11 (a known, pre-existing web-UI limitation: even the web UI's network-mode control doesn't rebind the running listener). Decision: defer FR-043 entirely — extracting shared state wouldn't make the setting functionally effective until FR-R11 itself is fixed, so not worth the unplanned cross-package refactor now.

Both are documented in `spec.md` (FR-039/040/041/043's corrected text) and `implementation-notes.md`'s Deviations from Plan section.

**AD-1 verification (hard acceptance criterion):** confirmed compiling; 29 of 31 pre-existing `internal/adapter/web` auth tests pass unmodified; `TestSessionExpiry` and `TestCleanupExpiredSessions` relocated verbatim to `internal/usecase/auth/session_test.go` (documented exception, they reach unexported fields that moved). A third test (`TestGenerateRandomString`) that the original architecture draft's breakage analysis missed was kept compiling by duplicating the small `generateRandomString` helper in both packages, preserving the "exactly two relocated" count.

**AD-6 verification:** `TestReconcileIdentity_NonAdminChatDoesNotBecomeAdmin` and the end-to-end `TestGateway_AuthGatesInputAndAttributesMessages` both explicitly assert a non-admin CLI login's reconciled `domain.User` is never `RoleAdmin`, including through the actual dispatch path (not just the reconciliation function in isolation).

**Coverage:** `internal/usecase/auth` 94.7%, `internal/usecase/user` 95.8% (both exceed AGENTS.md's 90% usecase-layer target); `internal/adapter/gateway/cli` 79.6%, `internal/adapter/web` 81.1% (adapter layer, not held to the 90% bar — the low-coverage items are mostly trivial delegators/setters and one terminal-specific I/O function not practically unit-testable). No regression versus the pre-existing baseline.

**Cross-adapter import check:** `go list -deps ./internal/adapter/gateway/cli/...` contains no `internal/adapter/web` — verified, not assumed.

**Commits (11, in dependency order):** `2955503` (AD-1 extraction), `19c4b09` (CLI login/session/AD-6/P2.6), `0a81d67` (dispatch scaffold), `123becf` (Settings), `9671f07`/`c0cb1f2`/`ad36b3b`/`87a9089`/`49dac62`/`3faea7a` (Chats/Projects/Jobs/Chores/History/Memories, one per environment), `b49f995` (main.go integration wiring), `d4fe8f6` (docs).

## Recent Activity

- 2026-08-11: Spec directory created from `cli-parity-for-nuimanbot-features-PRD.md` via `dev-flow:create-spec` skill (pipeline Step 1 of `/implm-frm-prd`). PRD moved from repo root into this spec directory. Two load-bearing architectural facts from the PRD re-verified against the current worktree (post-PR #8, no drift):
  1. `cmd/nuimanbot/main.go:1156-1159` still unconditionally grants `cli_admin`/`RoleAdmin` via `SetCurrentUser`; line 1173 still hardcodes `"cli_user"` chat attribution.
  2. `internal/adapter/web/auth.go` still defines `AuthService` (`ValidateCredentials`, `CreateSession`, `ValidateSession`, `GetSession`, `DestroySession`, etc.) entirely within the `web` adapter package — not yet in `internal/usecase`.
  Also confirmed `internal/usecase/{chats,projects,jobs,chores,history,memories,settings}` all exist, and `internal/adapter/gateway/cli/gateway.go` already exposes the `Set*Handler` wiring pattern (`SetAdminHandler`, `SetProfileHandler`, `SetBotHandler`, `SetMemoryHandler`, `SetConfigHandler`, `SetSkillHandler`) new environment handlers should follow. `IsMemoryCommand()` in `internal/adapter/gateway/cli/memory_commands.go` confirmed to claim the `/memory` prefix only.
- 2026-08-11: Pipeline Step 4 (documentation, `/implm-frm-prd`) completed by a dedicated documentation pass, separate from and after Step 3's own P5.1/P5.2 doc commit (`d4fe8f6`). Read `d4fe8f6` and the coverage-hardening commit (`8778206`) first to avoid duplicating work, then verified every doc claim against the actual merged code rather than trusting the mid-implementation draft. Added six new ADR entries (`documentation/architectural-decision-record.md` ADR-015 through ADR-020, one per `architecture.md` decision AD-1 through AD-6) — the ADR file had none of this feature's decisions yet, since `d4fe8f6` only touched product-summary/product-details/technical-details. Found and fixed two real gaps left by Step 3's mid-implementation doc pass: (1) `documentation/product-summary.md`'s Known Limitations section still claimed no CLI/web Memories identity bridge exists, stale since FR-032 resolves it — fixed, plus added a completed-phase changelog bullet for CLI Parity matching the format of adjacent entries; (2) `support_docs/cli-environments-guide.md` had two factual inaccuracies against the final `chore_commands.go` — `/chore create` documented a positional `<working-directory>` argument that was never real (the actual flag is optional `--dir`), and `/chore delete` was described as "blocked" when it actually soft-deletes and completes once the active run finishes. Also documented a previously-unmentioned behavioral change surfaced by the coordinator: `./bin/nuimanbot` (including `--help`, never a real flag) now exits 1 instead of 0 on empty/closed stdin, since login gates all input. Verified `support_docs/cli-admin-guide.md` and `README.md`'s changelog against the final implementation (post-`8778206`) and found both accurate — no changes needed. Committed as `55f46bc` and pushed to `worktree-cli-parity`.
- 2026-08-11: Pipeline Step 2 (`dev-flow:review-spec`) completed. Read all eight files plus `AGENTS.md`, then verified AD-1/AD-2 and the rest of the spec against the actual current code (`internal/adapter/web/auth.go` + its two test files, `internal/adapter/gateway/cli/gateway.go` + `memory_commands.go`, `cmd/nuimanbot/main.go`, `internal/usecase/chat/service.go`, all six existing web `*Service` handler interfaces, `go.mod`/`go.sum`). Found and resolved six concrete gaps (see "Step 2 Review Outcome" above) rather than leaving them flagged, since no human is available mid-pipeline. Verdict: **implementation-ready**. `git status` shows only spec-directory changes (gitignored, not committed) — no repo-tracked files were touched.
