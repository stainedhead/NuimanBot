# Status: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Spec directory:** `specs/260811-cli-parity-for-nuimanbot-features-auto-review/`

## Overall Progress

| Phase | Description | Status | Progress |
|---|---|---|---|
| 0 | Spec Creation | Complete | 100% |
| 1 | Research | Complete | 100% (all 5 open questions resolved — see implementation-notes.md) |
| 2 | Data Dictionary | Complete | 100% |
| 3 | Architecture | Complete | 100% |
| 4 | Planning | Complete | 100% |
| 5 | Task Breakdown | Complete | 100% |
| 6 | Implementation | Complete | 100% |

## Task Progress (tasks.md)

| Task | Finding | Status |
|---|---|---|
| A.1 | FR-002 (P1) | Complete |
| B.1 | FR-001 (P1) | Complete |
| B.2 | FR-003 (P2) | Complete |
| B.3 | FR-007 (P2) | Complete |
| C.1 | FR-004 (P2) | Complete |
| C.2 | FR-005 (P2) | Complete |
| FR-006 | (informational) | Closed by acknowledgment — read and confirmed out of scope per its own acceptance criteria (spec-acknowledged TOCTOU note, no code change) |
| X.1 | Final quality gate | Complete |

## Phase 0: Spec Creation — Task Checklist

- [x] Spec directory created: `specs/260811-cli-parity-for-nuimanbot-features-auto-review/`
- [x] PRD moved into spec directory
- [x] `spec.md` populated from PRD (Executive Summary, Problem Statement, Goals/Non-Goals, FR-001 through FR-007, NFRs, Architecture, Scope, Breaking Changes, Success/Acceptance Criteria, Risks)
- [x] `research.md` seeded with research questions derived from PRD's Open Questions and Dependencies sections
- [x] `data-dictionary.md`, `architecture.md`, `plan.md`, `tasks.md`, `implementation-notes.md` initialized with placeholder structure
- [x] `status.md` marked Phase 0 fully complete — final review of all phase files done; all 7 findings implemented and quality-gated (pipeline Step 9 complete, Step 10 archiving in progress)

## Blockers

None.

## Recent Activity

- 2026-08-11: **X.1 (final quality gate) complete. All 6 code/doc tasks (A.1, B.1, B.2, B.3, C.1, C.2) done; FR-006 closed by acknowledgment.** Full chain run and passed with zero errors: `go fmt ./... && go mod tidy && go vet ./... && golangci-lint run` (0 issues) `&& go test ./...` (all packages pass) `&& go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help` (exit 1, confirmed pre-existing documented behavior per support_docs/cli-environments-guide.md, not a regression from this pass). This fix pass is complete: 7 findings from the review PRD (0 P0, 2 P1, 5 P2) all addressed — 2 P1s and 3 P2s via code+test changes (A.1/B.1/B.2/B.3), 2 P2s via documentation-only changes (C.1/C.2), 1 P2 (FR-006) closed by acknowledgment per its own acceptance criteria. All 5 research.md open questions resolved — see implementation-notes.md Technical Decisions for each. 6 commits made to `worktree-cli-parity`, each pushed individually as its task completed.
- 2026-08-11: Spec directory created from `cli-parity-for-nuimanbot-features-auto-review-PRD.md` (7 findings: 0 P0, 2 P1, 5 P2) via `dev-flow:create-spec` (pipeline Step 8 of `implm-frm-prd`). `tasks.md` translates the PRD's own 3-workstream parallelization plan (Workstream A: FR-2/jobs-usecase; Workstream B: FR-1/FR-3/FR-7/CLI-handler cluster; Workstream C: FR-4/FR-5/doc-only cluster) directly rather than inventing a different task structure, per the PRD's Fix Process Guidance section.
- 2026-08-11: **C.2 (FR-005) complete.** Corrected `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`'s Observability NFR text in place (original claim preserved, dated correction appended): the web admin's login rate limiter is real, its failed-login audit logging is not — so "consistent with the web admin's existing... audit logging" accurately meant "consistent with the web admin's existing lack of" it. Filed https://github.com/stainedhead/NuimanBot/issues/9 as the separately-tracked follow-up FR (one combined issue for both `web/auth.go` and `cli/auth_commands.go` — Research Question 3 resolved this way since they share no code path apart from the `audit` package and the fix shape is identical). Confirmed `gh` issues are usable in this repo before committing to that mechanism over a (non-durable, gitignored) new spec directory. Deliberate git-tracked edit to a completed archived artifact per AD-F4 — called out in the commit message.
- 2026-08-11: **C.1 (FR-004) complete.** Documented, in three places, that AD-6's identity-reconciliation fix is a wiring-order mitigation, not a structural fix, and that `defaultRoleForPlatform(PlatformCLI) = RoleAdmin` is dead-in-practice-but-not-dead-in-code: a code comment directly on `defaultRoleForPlatform` (internal/usecase/chat/service.go), documentation/technical-details.md's "Identity Reconciliation (AD-6)" section plus its CLI default-role table row, and documentation/architectural-decision-record.md's ADR-020 Consequences. All three name the PRD's own future-CLI-entry-point examples. Research Question 2 (which structural fix, if any) recorded as deliberately undecided per AD-F2. Also folded in an A.1-owed documentation update (Job-lifecycle flow in technical-details.md now describes verifyContextOwnership/ChatOwnershipCheck) and an ADR-019 accuracy correction discovered along the way (its cross-owner-rejection claim was aspirational until A.1's fix). Full quality gate passed (comment/doc-only changes; no test behavior affected).
- 2026-08-11: **B.3 (FR-007) complete.** Added `TestEnsureAuthenticated_RestoreCorrectsStaleRole` (auth_commands_test.go), pre-seeding a stale RoleAdmin domain.User for (PlatformCLI, "alice") and a valid session file whose Role is "user", then asserting EnsureAuthenticated's restore path (not reconcileIdentity called directly) both returns and durably stores the corrected RoleUser. Test-only addition (no production code change, per tasks.md B.3) — it passed immediately against existing code, confirming this was a genuine test-completeness gap, not a hidden behavioral bug. Full quality gate passed.
- 2026-08-11: **B.2 (FR-003) complete.** The four deferred `chat` subcommands (`/project chat`, `/job chat`, `/chore chat`, `/history chat`) now return a specific "not yet implemented (deferred, see spec.md FR-0NN)" message via a dedicated `case "chat":` in each handler's switch, instead of falling through to the generic "Unknown command" response — matching Settings' existing deferred-command convention. Genuine typos still get the generic response (verified by new tests). Full quality gate passed.
- 2026-08-11: **B.1 (FR-001) complete.** `/memories browse` (`internal/adapter/gateway/cli/memories_commands.go`) now caps rendered output to `memoriesBrowseDisplayLimit` (aliased directly to `historyListDisplayLimit`, so it stays in lockstep with `/history list`'s cap) with a "N more result(s) not shown" trailer matching `/history list`'s wording. Help text and `support_docs/cli-environments-guide.md` updated to document the cap. Deliberately did not set `memoryv2.MemoryCellFilter.Limit` at the `ListCells` call (see implementation-notes.md) — the display cap is applied client-side, after the existing query filter, to avoid an incorrect count/trailer when a query narrows a larger fetched set; this mirrors `/history list`'s own fetch-then-filter-then-cap shape rather than deviating from it. Full quality gate passed.
- 2026-08-11: **A.1 (FR-002) complete.** Full Red-Green-Refactor cycle in `internal/usecase/jobs/service.go`: `CreateJob` now rejects (domain.ErrNotFound) a `--project`/`--chat` contextID that doesn't resolve to a Project/Chat owned by `ownerUserID`, for both context types (Chat previously had no check at all). New `ChatOwnershipCheck` interface + `chatOwnershipCheckAdapter` (cmd/nuimanbot/extended_context.go). Resolved Research Questions 1, 4, 5 (see implementation-notes.md) — key finding: "not owned" and "stale/deleted" are provably indistinguishable at the repository layer by design (anti-IDOR), so both are rejected identically; this required repurposing `TestCreateJob_StaleProjectReferenceStillCreatesJob` (documented as a deliberate deviation, not a silent test change). Self-review via `advisor` surfaced and fixed: a fail-open `default:` branch inconsistent with the fail-closed philosophy, and an undocumented cross-adapter effect on `internal/adapter/web/jobs_handler.go` (same usecase-layer fix now also protects the web job-create form — recorded in implementation-notes.md, not fixed further, out of scope). Full quality gate passed (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run` — 0 issues, `go test ./...` — all packages pass, `go build`, `./bin/nuimanbot --help` — exit 1 confirmed as pre-existing documented behavior, not a regression).
