# Plan: CLI Parity for NuimanBot Features

**Feature:** cli-parity-for-nuimanbot-features
**Created:** 2026-08-11
**Status:** Finalized (pipeline Step 2 review) — Phase Breakdown, Testing Strategy, Rollout Strategy, and Success Metrics resolved below; see `tasks.md` for the full 19-task breakdown these phases expand into.

## Development Approach

Confirmed during Step 2 review (`architecture.md` AD-1/AD-2 finalized against the actual code, not just as drafted). Per `AGENTS.md`'s mandatory TDD workflow: land the `internal/usecase/auth` extraction (AD-1, wrapper-struct approach) and `RestoreSession` (AD-2) first, in isolation, verified against the existing web auth test suite passing unmodified (with the two documented, narrow exceptions — `TestSessionExpiry`/`TestCleanupExpiredSessions` relocate verbatim). This is the foundation everything else depends on. Only then build the CLI login/session flow on top of it — including the newly-discovered AD-6 identity-reconciliation step, which must land before any chat message is processed by an authenticated CLI user, not as an afterthought — followed by the six environment command handlers (Chats, Projects, Jobs, Chores, History, Memories — note Projects/Jobs/Chores/History ship without their originally-drafted chat sub-command; see spec.md's Non-Goals) and Settings, each independently addable once auth is in place.

## Phase Breakdown

Expanded into `tasks.md`'s 19-task breakdown (P1.1–P5.3) during Step 2 review.

1. **Phase A — AuthService extraction (FR-044).** Move credential/session logic to `internal/usecase/auth` per AD-1. Web adapter refactored to delegate. Existing web auth tests pass unmodified.
2. **Phase B — CLI authentication (FR-001–FR-007, plus AD-6's identity reconciliation).** Login prompt, session persistence/restore (AD-2), `/logout`, role-gating of existing admin commands, real chat-message attribution, and reconciling `domain.User`'s Role so `internal/usecase/chat`'s pre-existing `defaultRoleForPlatform(PlatformCLI)=RoleAdmin` auto-provisioning path is never reached for an authenticated CLI user (AD-6 — see `architecture.md`).
3. **Phase C — Environment command handlers (FR-008–FR-038, minus deferred chat sub-commands).** One sub-phase per environment (Chats, Projects, Jobs, Chores, History, Memories), each following the existing `cli.New*Handler`/`Set*Handler` pattern and consuming the corresponding existing `internal/usecase/*` service, with `ownerUserID = session.Username` (AD-5). Projects/Jobs/Chores/History ship list/show/create/delete(/confirm-schedule) only this pass — their originally-drafted per-item chat sub-commands (FR-020/026/031/036) are deferred; no backing web-side capability exists to mirror (verified against `internal/adapter/web/{projects,jobs,chores,history}_handler.go`).
4. **Phase D — Settings (FR-039–FR-043).** Per-user and admin-only Settings commands.
5. **Phase E — Cleanup and docs.** Remove auto-admin grant fully, deprecation note in `support_docs/`, update `documentation/` product docs per AGENTS.md's documentation-maintenance rules.

## Critical Path

Phase A (AuthService extraction) blocks Phase B, which blocks Phases C/D's admin-gating and per-user attribution — the PRD's own Dependencies/Risks table and this spec's `architecture.md` both identify FR-044 as the item that must be settled first.

## Testing Strategy

Per `AGENTS.md`'s mandatory TDD (Red-Green-Refactor) methodology, applied per task in `tasks.md`:
- `internal/usecase/auth`: mostly new tests (`GetUser`, `RestoreSession`'s three cases, exported `CreateSessionWithFlags`/`IsDefaultCredentials`), plus the two relocated-verbatim tests per AD-1.
- `internal/adapter/web`: existing test suite must pass unmodified (the two-function exception above) — this is the regression gate for FR-044's acceptance criterion, not a new-tests task.
- AD-6's identity reconciliation: explicit tests for the "non-admin CLI user's first chat doesn't get RoleAdmin" and "demoted admin gets corrected on next login" cases (see `tasks.md` P2.3).
- Per new CLI command handler: unit tests following existing `*_test.go` patterns already present in `internal/adapter/gateway/cli/` (e.g. `admin_commands_test.go`), plus one cross-user-isolation test per environment (two different logged-in users, verify no cross-visibility) and, for Jobs specifically, the "`--project <id>` belonging to another user" edge case (`tasks.md` P3.3).
- `/memories` vs `/memory` dispatch: explicit side-by-side test, plus a bare-`/memories`-with-no-args case (`tasks.md` P3.6).
- End-to-end: `go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help` must succeed after every phase, not just at the end — `AGENTS.md`'s quality gate is a per-task discipline, not a final step.

## Rollout Strategy

Staged by phase, matching the critical path (Phase A must land and be verified — web auth tests green — before Phase B starts; Phase B before Phase C/D). Whether each phase is a separate PR or accumulates in one branch with staged commits is an implementation-time call, not a spec-level decision — either satisfies the same quality gates. The breaking-change deprecation note (`tasks.md` P5.1) lands in the same change as the auto-admin removal (Phase B), not deferred to the end, so the breaking change and its documentation are never merged separately.

## Success Metrics

No additional product metrics beyond `spec.md`'s Acceptance Criteria and the `AGENTS.md` quality-gate chain — this is an internal-tooling/architecture feature with no user-facing usage-metric target. "Success" is binary: all acceptance criteria pass, all quality gates pass, existing web auth tests are green.
