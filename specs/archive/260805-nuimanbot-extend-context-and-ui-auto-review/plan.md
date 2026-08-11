# Plan: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Status:** Planning
**Source:** PRD "Fix-Pass Execution Guidance" section (mandatory) — this plan implements that guidance directly.

## Development Approach

This is a **code-review-findings fix pass, not a fresh feature**. Per the PRD's own mandatory guidance for this case:

1. **TDD, per finding.** Each of the 19 findings is its own Red-Green-Refactor cycle: write the failing test that encodes the finding's acceptance criteria first (e.g., FR-R2's crash-simulation test, FR-R6's symlink-escape test, FR-R18's traversal/absolute-path-escape tests), confirm it fails for the right reason, implement the minimal fix, then refactor. **Do not batch multiple findings' Red phases into one commit** — a reviewer must be able to trace each fix to the specific finding it closes.

2. **Code review per fix.** Run `dev-flow:review-code` (or an equivalent focused review) against each finding's diff **before it merges — not once at the end of all 19**. FR-R6, FR-R18, and FR-R5 (the security- and architecture-sensitive fixes) get a **second reviewer pass** — this same review process already missed FR-R18 once (absent from the prior known-gaps list, only surfaced via a parallel review pass), which is itself the reason not to single-review the highest-stakes fixes here.

3. **Quality gates, every fix** (AGENTS.md's Pre-Completion Checklist): `go fmt ./...` → `go mod tidy` → `go vet ./...` → `golangci-lint run` → `go test ./...` → `go build -o bin/nuimanbot ./cmd/nuimanbot` → `./bin/nuimanbot --help`. Additionally:
   - Any fix touching `internal/infrastructure/scheduler` or the WebSocket path in `internal/adapter/web` must pass `go test -race` on those packages.
   - The 13 existing `TestHandle*_CrossOwnerReturns404` tests must keep passing unmodified after every fix (FR-R5 and FR-R18 touch the exact repository/service code paths those tests exercise).

4. **Agent teammates and git worktrees, by workstream.** One agent teammate per workstream (A–G), each in its own git worktree/branch, running TDD + quality gates + per-finding code review inside that worktree. Findings are grouped by shared-file/shared-concern collision (see `architecture.md`), not just priority — groups run fully in parallel; findings inside a group are sequenced or done by the same teammate to avoid conflicting edits and ordering bugs.

## Phase Breakdown

The "phases" for this fix pass **are** the PRD's 7 workstreams — this plan does not introduce a separate phase numbering scheme on top of them.

| Phase (Workstream) | Findings | Priority mix | Parallelizable with |
|---|---|---|---|
| A — Restart & retention sweeps | FR-R2 (P0), FR-R3 (P0), FR-R9 (P1) | 2×P0, 1×P1 | B, D, E, F, G (not C — see Critical Path) |
| B — Filesystem layering & sandboxing | FR-R5 (P1), FR-R6 (P0), FR-R18 (P0), FR-R13 (P2) | 2×P0, 1×P1, 1×P2 | A, C, D, E, F, G |
| C — Chore parity | FR-R8 (P1) | 1×P1 | A (must finish before A's FR-R9), B, D, E, F, G |
| D — Deferred agent-facing surfaces | FR-R1 (P1), FR-R4 (P0) | 1×P0, 1×P1 | A, B, C, E, F, G |
| E — Live-update & observability plumbing | FR-R10 (P1), FR-R19 (P1) | 2×P1 | A, B, C, D, F, G |
| F — Settings & execution edge cases | FR-R11 (P1), FR-R12 (P1), FR-R14 (P2), FR-R15 (P2) | 2×P1, 2×P2 | All |
| G — Consistency polish | FR-R16 (P2), FR-R17 (P2) | 2×P2 | All (lowest scheduling priority) |

## Critical Path

The only hard cross-workstream ordering constraints:

1. **C (FR-R8) before A's FR-R9** — FR-R9's cleanup sweep assumes Chores already soft-delete correctly; if A reaches FR-R9 before C lands, either block on C or reorder A internally to do FR-R3 and FR-R2 first.
2. **B's FR-R5 before B's FR-R6** — same call sites; fixing symlink escape at the pre-relocation call sites would need to be redone after FR-R5 moves them.
3. **B's FR-R5 before B's FR-R18 merges** (FR-R18 may start in parallel, but must rebase before merge) — both touch `projects/service.go`.
4. **A's FR-R3 before A's FR-R9** — FR-R9 extends the same sweep loop FR-R3 builds.

Everything else (all of D, E, F, G; A's FR-R2 relative to FR-R3/FR-R9 aside from shared-file coordination; B's FR-R13) is independently mergeable as soon as its own workstream's internal sequencing is satisfied.

**Priority-driven suggested landing order** (if serializing rather than fully parallelizing): P0 findings first regardless of workstream — FR-R2, FR-R3, FR-R4, FR-R6, FR-R18 — respecting the critical-path constraints above (i.e., FR-R6 still waits on FR-R5 even though FR-R5 is P1 and FR-R6 is P0).

## Testing Strategy

- **Per-finding TDD:** failing test first, encoding the finding's stated acceptance criteria exactly (many findings specify the test almost verbatim — e.g. FR-R2's crash-simulation test, FR-R6's adversarial symlink test, FR-R18's traversal/absolute-path rejection tests, FR-R12's delete-Project-then-run test).
- **Regression guard:** all 13 `TestHandle*_CrossOwnerReturns404` tests re-run after every fix in workstream B (FR-R5, FR-R18 touch this code) and spot-checked elsewhere.
- **Race safety:** `go test -race ./internal/infrastructure/scheduler/... ./internal/adapter/web/...` after every fix touching those packages (workstreams A, E primarily).
- **Integration-level proof over unit-only:** FR-R3 explicitly requires an integration-level test (real repository, not isolated use-case return value) proving expired data is actually gone after a sweep. FR-R7 requires an integration test against the real curator path, not just a synthetic fixture.
- **Positive-path coverage alongside adversarial tests:** FR-R6 and FR-R18 both require a legitimate-path test proving the fix doesn't break normal operation, not just that the attack is blocked.

## Rollout Strategy

- Each workstream develops in its own git worktree/branch (per AGENTS.md's Agent Teams guidance: teammates inherit the current model unless the user requests otherwise).
- Merge order: respect the critical path above; otherwise merge whenever a workstream's findings are all green (tests pass, quality gates pass, code review complete, second review complete for FR-R5/FR-R6/FR-R18).
- No feature-flagging or staged rollout needed — these are bug/gap fixes against an already-merged, not-yet-externally-released feature branch.
- Final integration step: after all workstreams merge, re-run the full Pre-Completion Checklist once more against the fully-merged state (not just per-fix) to catch any cross-workstream interaction issues (e.g. FR-R10's WebSocket consumer against FR-R19's now-everywhere `BaseData.UnviewedRunCount`).

## Success Metrics

- 19/19 findings closed with acceptance criteria met (tracked in `tasks.md` and `status.md`).
- Zero regressions in the 13 existing cross-owner-404 tests.
- `go vet`, `golangci-lint run`, `go test ./...`, `go test -race` (scoped packages) all clean at final integration.
- `implementation-notes.md` records the actual resolution chosen for each Open Question (FR-R1/R4's implement-vs-defer choice, FR-R7's gateway trace finding, FR-R11's scope decision, FR-R18's allowed-root source) rather than leaving any silent.

## References

- Source PRD "Fix-Pass Execution Guidance" (mandatory) and "Open Questions" sections: [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md)
- `architecture.md` — full workstream table and dependency graph (reproduced from the same PRD section).
