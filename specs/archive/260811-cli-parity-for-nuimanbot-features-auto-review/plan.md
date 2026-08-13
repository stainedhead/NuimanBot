# Plan: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Status:** Planning

## Development Approach

This plan directly translates the source PRD's "Fix Process Guidance" section — it is binding, not a suggestion, and this spec does not invent an alternative structure. Three workstreams, each independently completable, mapped 1:1 to the PRD's own grouping:

- **Workstream A — jobs-usecase fix (FR-002, P1):** `internal/usecase/jobs/service.go` + its tests. The largest, highest-risk fix; the only one crossing the "reused as-is" Non-Goals boundary. Runs in its own git worktree with, if using agent teammates, its own dedicated teammate.
- **Workstream B — CLI-handler cluster (FR-001, FR-003, FR-007; one P1 + two P2):** All live in `internal/adapter/gateway/cli/`, touching distinct files (`memories_commands.go`; `project_commands.go`/`job_commands.go`/`chore_commands.go`/`history_commands.go`; `auth_commands_test.go`). Safe to parallelize across teammates within the workstream, or run sequentially in one worktree if headcount is limited.
- **Workstream C — doc-only cluster (FR-004, FR-005; both P2):** No production code change required for either. `architecture.md`/code-comment documentation (FR-004) and the original spec's NFR text plus a new tracked FR (FR-005). Low risk of touching the same lines as A or B.
- **FR-006 (P2, informational):** No workstream needed — closes by acknowledgment. Only pick up the optional TOCTOU hardening follow-up if separately prioritized.

Every finding with a code change (all of Workstream A and B, i.e. FR-001, FR-002, FR-003, FR-007) follows the full mandatory Red-Green-Refactor cycle per AGENTS.md — Red (failing test first, matching the acceptance criteria's named test), Green (minimal implementation), Refactor (mandatory cleanup pass, not optional). `dev-flow:review-code` runs against each individual finding's fix before moving to the next finding — not batched at the end.

Per AGENTS.md, do not specify a non-default model when spawning teammates for these workstreams unless the user explicitly requests one.

## Phase Breakdown

- **Phase 1 (this spec's Research):** Confirm the FR-002 Chat-ownership failure-mode decision (soft-fail vs. hard-reject) and the FR-002 test-harness approach before Workstream A starts implementation (see `research.md` Research Questions 1 and 5).
- **Phase 2 (this spec's Data Dictionary):** Confirm whether FR-002 needs a new sentinel error type; confirm FR-003's message-constant naming.
- **Phase 3 (this spec's Architecture):** Finalize FR-002's corrected `CreateJob` flow (three-way branch: not-owned / stale-but-owned / genuinely-not-found); finalize FR-004's documentation location (architecture.md vs. code comment, or both).
- **Phase 4 (this Plan):** As below.
- **Phase 5 (Task Breakdown):** See `tasks.md` — direct translation of Workstreams A/B/C.
- **Phase 6 (Implementation):** Execute Workstreams A, B, C (A and C can start in parallel with each other and with B; within B, FR-001/FR-003/FR-007 can parallelize across teammates or run sequentially).

## Critical Path

Workstream A (FR-002) is the critical path for P1 closure — it is the largest, highest-risk fix and the only one requiring a design decision (Chat ownership failure mode) before implementation can start. Workstreams B and C have no cross-dependency on A or each other and can run fully in parallel. There is no shared-file coordination point between them: FR-002 (Workstream A) *satisfies* the original archived spec's line-157 acceptance criterion without editing its text; only FR-005 (Workstream C) edits that spec.md (Observability NFR text correction) — see Dependencies in `spec.md`. Workstream C's C.2 task should note in `implementation-notes.md` (AD-F4) that this edit lands on a git-tracked archived file, since that's a deliberate decision made in this pass, not inherited unchanged from the PRD.

## Testing Strategy

- Every code-changing finding (FR-001, FR-002, FR-003, FR-007) gets a failing test first, named per its acceptance criteria, before any implementation.
- FR-002's test mirrors the existing `TestGetJob_CrossOwnerIsolation` pattern already established in the codebase.
- FR-001's test seeds more memory cells than the chosen display limit and asserts both the cap and the truncation trailer.
- FR-003's tests assert the *specific* message text per deferred subcommand, not just "some non-crashing response."
- FR-007's test exercises `EnsureAuthenticated`'s restore path end-to-end (not `reconcileIdentity` in isolation) with a mismatched pre-seeded role.
- Full quality gate (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...`, `go build`, `./bin/nuimanbot --help`) runs after every individual fix, not just at the end of the pass.
- `dev-flow:review-code` runs per individual fix.

## Rollout Strategy

No feature flag or staged rollout needed — these are bug fixes and documentation corrections to an already-shipped feature branch (`worktree-cli-parity`, not yet merged to `main`). All fixes land on the same branch before it merges. No user-facing migration or backward-compatibility concern, since the branch has not shipped to `main` yet.

## Success Metrics

- All 7 findings closed per their spec.md acceptance criteria (FR-006 closes by acknowledgment, no code change).
- Zero quality-gate failures across the full pass.
- `dev-flow:review-code` passes clean on each individual fix.
- `status.md` reflects 100% completion across all phases before this spec is archived.
