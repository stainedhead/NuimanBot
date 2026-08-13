# Dev-Flow Process Analysis

**Feature:** CLI Parity for NuimanBot Features (Chats, Projects, Jobs, Chores, History, Memories + Settings, with real CLI login/session identity)
**Spec directory:** `specs/archive/260811-cli-parity-for-nuimanbot-features/` (implementation spec); `specs/archive/260811-cli-parity-for-nuimanbot-features-auto-review/` (fix-pass spec)
**Report generated:** 2026-08-11

---

## 1. Executive Summary

This run replaced the CLI's auto-admin/no-login startup behavior with real authenticated identity (extracting a shared `internal/usecase/auth` service out of the web adapter, FR-044/AD-1), then added CLI slash-command parity for all six web-admin environments plus per-user Settings, reusing the existing `internal/usecase/*` services rather than duplicating business logic. Two capabilities (per-user retention "set", network-mode "set") were explicitly deferred mid-implementation because the underlying capability doesn't exist anywhere in the codebase yet, not just in the CLI.

**Total runtime (Steps 1–11, this report at Step 12):** 3h 17m (2026-08-11T16:57:52Z → 2026-08-11T20:15:00Z), across 35 commits.
**Overall assessment:** Clean, low-rework run. No step was redone or reverted. The two mid-implementation scope check-ins were genuine "capability doesn't exist" discoveries surfaced to the human orchestrator rather than guessed at, and the Step 5 review's one security-adjacent finding was deliberately re-verified against primary source (code, not claims) before being filed as P1 rather than P0. The largest time sinks were Step 3 (implementation, 69 min, 35% of total) and Step 9 (review-fix implementation, 35 min, 18%) — both expected, both proportional to the amount of production code touched.

---

## 2. Step-by-Step Timing

| Step | Name | Start | End | Runtime (min) | Key Outputs |
|---|---|---|---|---|---|
| 1 | Create Spec from PRD | 2026-08-11T16:57:52Z | 2026-08-11T17:02:53Z | 5 | `specs/260811-cli-parity-for-nuimanbot-features/`, PRD moved in |
| 2 | Review Spec | 2026-08-11T17:03:40Z | 2026-08-11T17:23:37Z | 20 | Found compile-breaking AD-1 draft + independent CLI-admin RBAC bug in `chat/service.go`; 6 gaps resolved directly (no human available mid-pipeline) |
| 3 | Implement Product | 2026-08-11T17:23:37Z | 2026-08-11T18:32:24Z | 69 | 13 commits; AD-1–AD-6 delivered; FR-039/040 (retention "set") and FR-043 (network-mode "set") deferred per coordinator decision; independent RBAC bug in `defaultRoleForPlatform(PlatformCLI)` (hardcoded admin) caught and fixed as new AD-6; self-caught 6 coverage/hardening gaps via post-hoc advisor pass (commit `8778206`) |
| 4 | Documentation and User Docs | 2026-08-11T18:32:24Z | 2026-08-11T18:42:50Z | 10 | ADR-015–020 added; 2 doc inaccuracies left behind by `d4fe8f6` (Step 3's own inline P5.1/P5.2 doc pass) found and fixed; commit `55f46bc` |
| 5 | Code and Design Review | 2026-08-11T18:42:50Z | 2026-08-11T19:10:45Z | 28 | 0 P0 / 2 P1 / 5 P2; P1 call on foreign-owned Project/Chat ID re-verified against primary source (not trusted from a claim) before being filed as P1, not P0; commit `66b8435` |
| 6 | Prepare Review PRD | 2026-08-11T19:10:45Z | 2026-08-11T19:15:40Z | 5 | P1 sanity-checked for FR-2, vague ACs tightened; commit `ce7fcfc` |
| 7 | Archive Original Spec | 2026-08-11T19:15:40Z | 2026-08-11T19:16:59Z | 1 | `specs/archive/260811-cli-parity-for-nuimanbot-features/`; commit `aad286b` |
| 8 | Spec Review Fixes | 2026-08-11T19:16:59Z | 2026-08-11T19:26:32Z | 10 | `specs/260811-cli-parity-for-nuimanbot-features-auto-review/`, 7 findings broken into 3 workstreams; commit `0a63228` |
| 9 | Implement Review Fixes | 2026-08-11T19:26:32Z | 2026-08-11T20:01:15Z | 35 | All 7 findings closed; job-create ownership fix (FR-2/P1) landed and, along the way, discovered a cross-adapter side effect on the pre-existing web admin's job-create form (same usecase fix now also protects it); GH issue #9 filed for audit-logging follow-up; 0 lint issues |
| 10 | Archive Fixes Spec | 2026-08-11T20:01:15Z | 2026-08-11T20:02:50Z | 2 | `specs/archive/260811-cli-parity-for-nuimanbot-features-auto-review/`; commit `0128f25` |
| 11 | Final Quality Pass | 2026-08-11T20:02:50Z | 2026-08-11T20:15:00Z | 12 | All gates green; domain/usecase coverage ≥90% confirmed on every touched package; 3 doc gaps closed |
| 12 | Process Analysis Report | 2026-08-11T20:15:00Z | (this report) | — | `dev-flow-analysis.md` (this file) |
| 13 | Archive Spec | — | — | — | Pending |
| 14 | Open Pull Request | — | — | — | Pending |

**Notable observations:**
- Step 3 (implementation) is the largest single step (69 min, 35% of Steps 1–11 runtime) but was not a smooth monolithic push — it paused twice for genuine scope-decision check-ins with the human orchestrator (deferring `/settings set retention` and `/settings set network-mode`) because neither capability exists anywhere in the codebase yet — not a CLI gap, a system gap. Both were escalated rather than guessed at, which is the correct call given AGENTS.md's Clean Architecture and Non-Goals constraints (building CLI-only capability with no backing usecase/domain support would itself have violated the PRD's own Non-Goals).
- Step 3 also independently caught and fixed a real RBAC bug outside the original spec's radar: `internal/usecase/chat/service.go`'s `defaultRoleForPlatform(PlatformCLI)` was hardcoded to return `RoleAdmin`, which would have silently granted every CLI user admin privileges on their first chat message regardless of their real login role — undiscovered until real CLI identity (this feature's core deliverable) made the bug reachable. Fixed as new architecture decision AD-6 (identity reconciliation), not left for a later step.
- Step 3 ran a self-directed advisor pass *before* Step 4 even started (commit `8778206`, "close coverage/hardening gaps found in advisor review") — proactive rework caught and closed within the same step rather than surfacing later in Step 5's formal review.
- Step 5 (code review) is the third-largest single step by runtime (28 min, after Step 3's 69 and Step 9's 35) precisely because it did primary-source re-verification rather than trusting the draft: the P1-vs-P0 call on the foreign-owned Project/Chat ID finding was confirmed by re-reading `internal/usecase/jobs/service.go`, `internal/infrastructure/scheduler/stub_executor.go`, and the CLI handler's output-rendering code to prove no existence-oracle or content-level IDOR exists today — explicitly flagged as contingent on `StubExecutor` remaining a non-content-reading stub, to be re-rated if a real executor replaces it later.
- Step 9's job-create ownership fix surfaced an unplanned but valuable side effect: because the fix lives in the shared usecase layer (`internal/usecase/jobs/service.go`), it also closes the same gap in the pre-existing web admin's job-create form — a cross-adapter benefit documented in `implementation-notes.md` rather than silently absorbed.
- No step was retried, reverted, or redone. Every step's "Complete" status held on first pass; all rework (advisor-caught gaps, review findings) was absorbed within its owning step rather than bouncing back to an earlier one.

---

## 3. Commit and Push Summary

**Total commits (Steps 1–11):** 35

| Commit | Timestamp | Message |
|---|---|---|
| `bf198cb` | 2026-08-11T17:03:15Z | docs(dev-flow): create spec dir for cli-parity-for-nuimanbot-features (Step 1) |
| `2955503` | 2026-08-11T17:35:47Z | feat(auth): extract AuthService into internal/usecase/auth (AD-1, FR-044) |
| `19c4b09` | 2026-08-11T17:49:41Z | feat(cli): real login/session identity for the CLI REPL (FR-001-007, AD-2, AD-5, AD-6) |
| `0a81d67` | 2026-08-11T17:55:51Z | feat(cli): dispatch scaffold for the six environments + Settings (Phase C/D prep) |
| `123becf` | 2026-08-11T18:05:07Z | feat(cli): Settings command handler, worker-pool-size real (FR-041-042), retention/network-mode deferred (FR-040/043) |
| `9671f07` | 2026-08-11T18:06:52Z | feat(cli): Chats environment command handler (FR-011-016) |
| `c0cb1f2` | 2026-08-11T18:06:56Z | feat(cli): Projects environment command handler (FR-017-022, FR-020 deferred) |
| `ad36b3b` | 2026-08-11T18:07:01Z | feat(cli): Jobs environment command handler (FR-023-027, FR-026 deferred) |
| `87a9089` | 2026-08-11T18:07:07Z | feat(cli): Chores environment command handler (FR-028-033, FR-031 deferred) |
| `49dac62` | 2026-08-11T18:07:15Z | feat(cli): History environment command handler (FR-034-036, chat half deferred) |
| `3faea7a` | 2026-08-11T18:07:20Z | feat(cli): Memories environment command handler (FR-037-038) |
| `b49f995` | 2026-08-11T18:11:10Z | feat(cli): wire the six environment handlers + Settings into the CLI gateway |
| `d4fe8f6` | 2026-08-11T18:16:41Z | docs: CLI Parity — breaking-change note, new environments guide, product docs (P5.1-P5.2) |
| `8778206` | 2026-08-11T18:30:14Z | test(cli-parity): close coverage/hardening gaps found in advisor review |
| `4343f0d` | 2026-08-11T18:32:34Z | chore(dev-flow): mark Step 3 complete, Step 4 in progress |
| `55f46bc` | 2026-08-11T18:41:56Z | docs: CLI Parity — ADR entries AD-1..AD-6, close gaps in Step 3 doc pass |
| `66deda5` | 2026-08-11T18:42:59Z | chore(dev-flow): mark Step 4 complete, Step 5 in progress |
| `66b8435` | 2026-08-11T18:55:35Z | docs(dev-flow): add Step 5 code review PRD for cli-parity-for-nuimanbot-features |
| `d894d11` | 2026-08-11T19:10:53Z | chore(dev-flow): mark Step 5 complete, Step 6 in progress |
| `ce7fcfc` | 2026-08-11T19:15:14Z | docs(dev-flow): Step 6 review-prd pass on cli-parity code-review PRD |
| `b74c682` | 2026-08-11T19:15:49Z | chore(dev-flow): mark Step 6 complete, Step 7 in progress |
| `aad286b` | 2026-08-11T19:16:40Z | docs(spec): archive cli-parity-for-nuimanbot-features spec after implementation |
| `a0d8437` | 2026-08-11T19:17:09Z | chore(dev-flow): mark Step 7 complete, Step 8 in progress |
| `0a63228` | 2026-08-11T19:21:38Z | chore: move cli-parity auto-review PRD into new spec directory |
| `8cd2d5a` | 2026-08-11T19:26:40Z | chore(dev-flow): mark Step 8 complete, Step 9 in progress |
| `50c6afc` | 2026-08-11T19:44:05Z | fix(jobs): reject foreign-owned/unresolvable --project\|--chat context on job create |
| `e475b19` | 2026-08-11T19:47:10Z | fix(cli): cap /memories browse output, matching /history list's convention |
| `64560a0` | 2026-08-11T19:51:36Z | fix(cli): give deferred chat subcommands a specific not-yet-implemented message |
| `f4fab59` | 2026-08-11T19:54:07Z | test(cli): add end-to-end restore-path test for stale-role correction |
| `afb9c0f` | 2026-08-11T19:57:23Z | docs: document AD-6 as a wiring-order mitigation, not a structural fix |
| `49a88fe` | 2026-08-11T19:58:46Z | docs(spec): correct archived spec's Observability NFR re: failed-login audit logging |
| `7159f6b` | 2026-08-11T20:01:23Z | chore(dev-flow): mark Step 9 complete, Step 10 in progress |
| `0128f25` | 2026-08-11T20:02:06Z | chore: archive cli-parity auto-review spec (Step 10) |
| `b453483` | 2026-08-11T20:02:57Z | chore(dev-flow): mark Step 10 complete, Step 11 in progress |
| `7c51963` | 2026-08-11T20:13:05Z | chore: Step 11 final quality pass — docs accuracy fixes, dev-flow status update |

*(Timestamps above are shown in UTC; `git log` on this branch records them in `-04:00` local time — subtract 4 hours to cross-check against a raw `git log` output.)*

All commits pushed individually to `worktree-cli-parity` as each task/step completed. No PR opened yet — that is Step 14, pending.

**Commit count reconciliation:** `git rev-list --count main..HEAD` returns 38, three more than the 35 in the table above. The difference is three commits that predate this pipeline's Step 1 (`Process Start` 2026-08-11T16:57:52Z): `52b69d5` (merge of PR #8, "extend web UI with Chats, Projects, Jobs, Chores, History, Memories" — the previously-merged six-environments feature referenced in Section 6), `9d630ed` (add the CLI Parity PRD), and `68e908a` (add FR-044 to that PRD) — all at or before 2026-08-11T16:56:01Z, i.e. pre-pipeline setup, not part of Steps 1–11.

**GitHub issue filed mid-run:** [#9](https://github.com/stainedhead/NuimanBot/issues/9) — "Add failed-login audit logging (web admin + CLI)", filed during Step 9 as the tracked follow-up for review finding FR-5, covering both `web/auth.go` and `cli/auth_commands.go` since they share no code path apart from the `audit` package and the fix shape is identical.

---

## 4. Spec vs. Implementation Comparison

`spec.md`/`plan.md` for this feature carry no time/effort estimates (this pipeline does not produce human-facing time estimates at spec time), so this section compares **planned scope** to **actual delivered scope**, not planned-vs-actual duration.

| Phase | Planned (spec) | Actual (git log) | Difference | Notes |
|---|---|---|---|---|
| Auth extraction (AD-1/FR-044) | Extract `AuthService` into `internal/usecase/auth` | Delivered as designed; wrapper-struct approach, not the original type-alias draft (Step 2 caught the type alias doesn't compile) | Design corrected before implementation, not after | `2955503` |
| CLI login/session (FR-001–007) | Login prompt, persisted session, `/logout`, role gating, real chat attribution | Delivered, plus AD-6 identity reconciliation (not originally scoped) added mid-implementation | +1 unplanned fix (AD-6) | `19c4b09` |
| Six environments + Settings (FR-011–043) | All six environments + per-user/admin Settings | All six environments delivered (`9671f07` Chats, `c0cb1f2` Projects, `ad36b3b` Jobs, `87a9089` Chores, `49dac62` History, `3faea7a` Memories); four carry a deferred per-item `chat` subcommand (see row below); Settings delivered with 2 of its "set" sub-capabilities deferred (FR-040 retention, FR-043 network-mode) | −2 FRs deferred (capability doesn't exist system-wide) | `9671f07`…`123becf` |
| Chat sub-commands (FR-020/026/031/036) | Originally drafted as in-scope | Marked DEFERRED during Step 2 review — no backing web-side implementation exists to mirror; building CLI-only would violate the PRD's own Non-Goals | −4 FRs deferred (caught before implementation started) | Resolved in `spec.md` during Step 2 |
| Documentation (P5.1/P5.2) | Product docs + breaking-change note | Delivered in two passes: Step 3's own doc commit (`d4fe8f6`) plus a dedicated Step 4 pass (`55f46bc`) that found and fixed 2 gaps the first pass left stale | +1 extra pass, self-corrected | Expected two-pass pattern for this pipeline |
| Code review fixes (Steps 5–10) | Not part of original spec — pipeline-generated from Step 5 findings | 7 findings (2 P1, 5 P2) all closed; 1 closed by acknowledgment (no code change needed) | +7 tasks (fully unplanned at spec time, expected pipeline behavior) | Full review/fix/archive sub-cycle, Steps 5–10 |

**Phases skipped:** None outright — four chat-subcommand FRs and two Settings "set" FRs were reclassified DEFERRED (not silently dropped; each has a documented reason and a specific "not yet implemented" message at the command surface) rather than skipped without explanation.

**Phases added:**
- AD-6 (identity reconciliation) — a real RBAC bug fix outside the original architecture draft's scope, found by reading `internal/usecase/chat/service.go` during implementation.
- A self-directed advisor/coverage-hardening pass within Step 3 (`8778206`), before the formal Step 5 review even started.
- The full 7-finding review-fix sub-cycle (Steps 5–10), which is pipeline-standard but represents real added implementation work: one P1 fix (`internal/usecase/jobs/service.go` ownership check) crossed a stated Non-Goals boundary (the spec had said this package would be "reused as-is") — a deliberate, justified, and explicitly-flagged exception, not scope creep.

---

## 5. Token / Message Usage

Exact token counts are unavailable in this environment. Rough shape based on step complexity and sub-agent fan-out observed in `DEV-FLOW-STATUS.md` and commit granularity:

- **Orchestrator turns:** Roughly proportional to step count and step complexity — Steps 3, 5, and 9 (implementation-heavy, primary-source-verification-heavy) almost certainly consumed the largest share; Steps 1, 6, 7, 10 (thin coordination/archival steps, 1–5 min each) consumed the least.
- **Sub-agent/workstream fan-out:** the auto-review PRD's Fix Process Guidance planned 3 parallel workstreams for Step 9 (Workstream A: job-create ownership fix; Workstream B: 4-file CLI-handler cluster; Workstream C: doc-only). The observable commit trace does not confirm parallel execution, though — Step 9's six commits (`50c6afc`, `e475b19`, `64560a0`, `f4fab59`, `afb9c0f`, `49a88fe`) are serial and evenly spaced (19:44:05Z–19:58:46Z), and the auto-review `status.md` itself states "6 commits made to `worktree-cli-parity`, each pushed individually as its task completed" — consistent with sequential execution in one worktree rather than concurrent sub-agents.
- **Estimated conservatively:** given 35 commits, 6 environment handlers implemented from a shared pattern, 2 full spec-review passes (Step 2, Step 6), and 2 rounds of primary-source code re-verification (Step 5's P1 sanity check, Step 9's cross-adapter discovery), this run sits in the "substantial multi-hour agentic implementation" band rather than a light single-pass run — consistent with the observed 3h17m wall-clock runtime across Steps 1–11.

---

## 6. Process Observations

### What worked well
- **Scope discipline under uncertainty.** Both mid-implementation deferrals (retention "set", network-mode "set") were escalated to the human orchestrator with a specific, code-grounded reason ("this capability doesn't exist anywhere in the codebase, not just the CLI") rather than either guessing at a fabricated backing implementation or silently dropping the requirement. This is the correct failure mode for an agentic pipeline operating under Clean Architecture constraints where inventing a CLI-only capability would itself be an architecture violation.
- **Primary-source verification over trusting claims, twice.** Step 5 re-verified the P1-vs-P0 call on the job-create finding by re-reading three files end-to-end (not accepting the draft's framing), and explicitly scoped the "no exposure" rationale as contingent on `StubExecutor` remaining a stub — a forward-looking caveat that will save a future pass from silently re-inheriting a stale severity rating. Step 9 similarly discovered (not assumed) that the usecase-layer fix also protected the pre-existing web admin form, and documented it rather than either fixing further out-of-scope or ignoring it.
- **Bugs caught outside the original ticket's radar were fixed, not deferred to "someday."** The `defaultRoleForPlatform(PlatformCLI)` RBAC bug (Step 3) was an existing latent issue that this feature's real-identity work made newly reachable; it was fixed in-pipeline with its own architecture decision (AD-6) and its own tests, not left as a TODO.
- **Two-pass documentation caught real drift.** Step 4's dedicated documentation pass found and fixed two inaccuracies left behind by Step 3's own inline doc commit (a phantom CLI flag, a wrong "blocked" claim) — evidence the second, independent pass earns its keep rather than being redundant with the first.
- **No rework bounced backward across steps.** Every fix, deferral, or correction was absorbed within the step that found it (Step 2's 6 gaps fixed in Step 2; Step 3's advisor-caught gaps fixed in Step 3; Step 5's findings fixed in Steps 8–9). No step had to be reopened after being marked complete.

### What caused delays or rework
- **Step 3 was interrupted twice for scope decisions.** Necessary and correctly handled, but each check-in is real wall-clock cost that a fully-scoped spec (with retention/network-mode capability gaps caught at Step 2 review time, before implementation started) could have avoided. The four chat-subcommand FRs *were* caught this way at Step 2 — the retention/network-mode gaps were not, and surfaced one step later than they could have.
- **The FR-2 (job-create ownership) fix crossed a stated Non-Goals boundary** (`internal/usecase/jobs/service.go` was declared "reused as-is" in the original spec). This was justified and explicitly flagged, but it's a sign the original spec's Non-Goals section was written before the cross-user acceptance criterion's implications were fully traced through the code — a gap that a more thorough Step 2 trace of "what does this acceptance criterion actually require touching" might have caught earlier, before Step 3 built on top of the untouched assumption.

### Discovered, out of scope
- **Two real bugs in the previously-merged six-environments feature (`52b69d5`, PR #8) surfaced mid-pipeline**, via a coordination message from a different concurrent sub-agent, not from this pipeline's own review steps: a GET-triggered state-change CSRF gap, and `Job.Status` never syncing with `Run` lifecycle. Neither caused this run any delay or rework — both were correctly recognized as out of this pipeline's own spec scope and explicitly deferred to a follow-up PRD rather than pulled in mid-flight, which would have expanded this run's blast radius past its own spec. They are preserved here as a finding, not a cost: no GitHub issue has been filed for either as of this report (confirmed via `gh issue list --state all`, which shows only #9), so they risk being lost once this pipeline closes out unless tracked explicitly.

### Recommendations for future runs
- When Step 2 (review-spec) is verifying acceptance criteria against actual code, extend that trace one layer further: for any acceptance criterion that implies a change to a package the spec's own Non-Goals/Dependencies section declares "reused as-is," flag the conflict at review time rather than letting Step 5 discover it as a review finding. This would have caught FR-2's boundary-crossing one full step earlier.
- Capture the two six-environments bugs (CSRF gap, Job.Status/Run desync) surfaced mid-pipeline as a tracked GitHub issue now, immediately after this pipeline closes, rather than relying on this report alone as the record — a report that isn't cross-linked to an actionable issue risks being the only place the finding is written down.
- Consider running Step 2's "does this capability exist anywhere in the codebase" check explicitly for every FR up front (as was done successfully for the four chat-subcommands) rather than only for some — this would likely have caught the retention/network-mode gaps at Step 2 instead of mid-Step-3, removing one of the two implementation check-in interruptions.

---

## 7. Manual vs. Automated Comparison

**Estimated manual duration:** 3–5 business days for a senior engineer, assuming: extracting a shared auth service across two adapters without breaking 31 existing tests; designing and implementing 6 new CLI environment handlers against existing usecase services; writing tests to this repo's ≥90% usecase-layer coverage bar; a full manual code-review cycle with a separate reviewer; and fixing review findings including one that crosses a package boundary the original design assumed was out of scope. This estimate excludes calendar/scheduling overhead (PR review turnaround, meeting time) and assumes the engineer already has full context on the codebase's Clean Architecture conventions.

**Actual automated runtime:** 3h 17m wall-clock across Steps 1–11 (spec creation through final quality pass), 35 commits, all quality gates green on every step with no rollback.

**Efficiency gain:** Roughly one to two orders of magnitude in wall-clock time (3–5 days ≈ 24–40 working hours, versus ~3.3 hours), though this comparison understates manual overhead this run avoided entirely (context-switching between spec/implementation/review/fix cycles, waiting for a human reviewer's availability) and does not capture that a human engineer would likely have caught the AD-6 RBAC bug and the FR-2 ownership gap during design rather than after the fact — this pipeline's structure (explicit review steps with primary-source re-verification) compensated for that by catching both before merge rather than in production.

**Narrative basis:** The comparison assumes a single senior engineer working this feature end-to-end manually, including writing their own tests to this repo's coverage bar and running their own code review pass — it does not model a team with a dedicated separate reviewer (which would add calendar latency but not necessarily engineering-hours). The two mid-implementation scope check-ins in this automated run stand in for what would, in a manual process, likely be Slack threads or a design-doc revision cycle — the automated version resolved both within the same session rather than blocking on asynchronous human availability.
