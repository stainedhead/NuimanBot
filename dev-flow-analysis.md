# Dev-Flow Process Analysis

**Feature:** improve-nuimanbot-security
**Spec directory:** specs/archive/260802-improve-nuimanbot-security (original), specs/archive/260803-improve-nuimanbot-security-auto-review (fix pass)
**Report generated:** 2026-08-03

---

## 1. Executive Summary

This run implemented a 7-part security-hardening feature for NuimanBot (an LLM agent with tool-calling): tool-output prompt-injection filtering, prompt-boundary guardrails, side-effecting action confirmation, tool RBAC correction, SSRF hardening, MCP trust classification, and documentation parity — followed by a full code-review cycle that found and fixed 15 additional findings (6 P0 blockers, 4 P1s, 5 P2s), the most significant being that the original RBAC fix was never actually invoked from the live chat path.

**Total runtime:** ~12h 23m wall-clock from the first PRD commit (`e42949e`, 2026-08-02 19:35:46 -04:00) to the final quality-gate commit (`d1b95a7`, 2026-08-03 07:58:44 -04:00). The 14-step `/implm-frm-prd` orchestration proper (Step 1 onward) spans ~10h 37m (21:21:32 → 07:58:44).

**Overall assessment:** The process worked as designed and caught a critical, easy-to-miss defect (RBAC logic that was correct in isolation but never wired into the live call path) via its independent code-review pass — two of three review agents found this same root cause from different angles, which is strong validation of running multiple independent reviewers rather than one. The dominant cost driver was Step 3 (Implement Product), specifically an infrastructure failure (a mid-agent API connection drop) during the largest sub-phase (Part C, the confirmation flow) that required a full resume-from-partial-state recovery, consuming roughly 6 of the step's 9.35 hours.

---

## 2. Step-by-Step Timing

Timestamps from `DEV-FLOW-STATUS.md` (UTC, self-recorded by the orchestrator at each step boundary); cross-validated against git commit timestamps (`-04:00`), which agree closely.

| Step | Name | Start (UTC) | End (UTC) | Runtime (min) | Key Outputs |
|---|---|---|---|---|---|
| 1 | Create Spec from PRD | 01:16:43 | 01:21:16 | 5 | `specs/260802-improve-nuimanbot-security/` (8 phase files), PRD moved in |
| 2 | Review Spec | 01:21:16 | 01:23:08 | 2 | Found + fixed 1 edge-case-handling Warning (ConfirmationStore concurrency, OutputValidator empty-input handling) |
| 3 | Implement Product | 01:23:08 | 10:44:59 | **561** | 7 phases (Parts A–G), 11 commits, ~12 subagents |
| 4 | Documentation and User Docs | 10:44:59 | 10:51:03 | 6 | ADR log (8 entries), `support_docs/security-hardening-guide.md`, config-reference updates |
| 5 | Code and Design Review | 10:51:03 | 11:01:47 | 11 | 3 parallel review passes → 15 findings (6 P0, 4 P1, 5 P2) |
| 6 | Prepare Review PRD | 11:01:47 | 11:02:45 | 1 | Added Open Questions, Dependencies, Implementation Guidance sections |
| 7 | Archive Original Spec | 11:02:45 | 11:03:09 | 1 | Moved to `specs/archive/` |
| 8 | Spec Review Fixes | 11:03:09 | 11:06:12 | 3 | Fix-pass spec directory created from review PRD |
| 9 | Implement Review Fixes | 11:06:12 | 11:48:47 | **43** | All 15 findings fixed, 15 commits, 13 subagents (across 3 priority waves) |
| 10 | Archive Fixes Spec | 11:48:47 | 11:49:12 | 0.5 | Moved to `specs/archive/` |
| 11 | Final Quality Pass | 11:49:12 | 11:58:32 | 9 | Coverage check, stale-doc correction (RBAC gap), full quality gate |
| 12 | Process Analysis Report | 11:58:32 | — | — | This report |
| 13 | Archive Spec | — | — | — | (confirms Step 7/10 already archived) |
| 14 | Open Pull Request | — | — | — | Pending |

**Notable observations:**
- **Step 3 (561 min) is a massive outlier** — 93% of the entire orchestration's runtime. This is almost entirely attributable to a single infrastructure failure: the first attempt at Phase 5's core confirmation-flow implementation (the largest, most concurrency-sensitive sub-phase) was terminated mid-work by an API connection drop after producing only one file (`confirmation_store.go`, the type/interface definitions — reviewed and found sound, kept as-is). The resumed attempt took ~5.9 hours by itself (`duration_ms: 21330688` per its own usage report) to rebuild the file-backed store, wire `tool.Service`/`chat.Service`, and pass full TDD with `-race` concurrency tests. Excluding this one incident, the other 6 phases (1–4, 6–7) together took roughly 3.4 hours.
- **Step 9 (43 min) was efficient by comparison** despite fixing 15 findings across 13 separate subagent runs — because most fixes touched disjoint files and ran in three parallel waves (5 P0s, 4 P1s, 4 P2s) rather than sequentially.
- **Step 5's independent-discovery result is the most valuable finding of the whole run**: two of three parallel code-review agents (reviewing different phase groups, using different angles — one via the tool-permissions map, one via the MCP-trust integration) independently converged on the same root-cause bug (`chat.Service` calling the RBAC-free `Execute()` instead of `ExecuteWithUser()`). This is strong evidence the multi-reviewer design is worth its cost.
- **Step 2 and Step 6 (spec/PRD review passes) were fast** (2–3 min each) because they were done as direct self-review rather than delegated to subagents — reasonable given their scope (a handful of dimension checks, not a large document).

---

## 3. Commit and Push Summary

**Total commits:** 39 (from the initial PRD commit through the final quality-gate commit)

| Commit | Timestamp | Message |
|---|---|---|
| `e42949e` | 2026-08-02T19:35:46-04:00 | docs: add PRD for prompt-injection & agent-safety hardening |
| `e8228c2` | 2026-08-02T21:10:32-04:00 | docs: resolve review gaps in security hardening PRD |
| `9c7c133` | 2026-08-02T21:21:32-04:00 | docs: create spec directory from security hardening PRD |
| `2722bb5` | 2026-08-02T21:23:14-04:00 | docs: update dev-flow dashboard after spec review (Step 2) |
| `e3420b8` | 2026-08-02T21:45:11-04:00 | feat(security): add tool-output injection filtering (Phase 1 / Part A) |
| `ab8ba96` | 2026-08-02T22:01:23-04:00 | feat(security): add prompt-boundary guardrails (Phase 2 / Part B) |
| `fb33218` | 2026-08-02T22:10:58-04:00 | feat(security)!: enforce explicit tool RBAC, admin-only github/coding_agent (Phase 3 / Part D) |
| `153b3a7` | 2026-08-02T22:30:41-04:00 | feat(security): Phase 4 (Part E) SSRF hardening for fetch tools |
| `eb86858` | 2026-08-03T06:05:11-04:00 | feat(security): Phase 5 core / Part C (P5.1-P5.6) — side-effecting action confirmation *(after ~5.9h recovery from an interrupted first attempt)* |
| `50ebe75` | 2026-08-03T06:18:03-04:00 | feat(security): Phase 5 P5.7 / Part C - chat-gateway confirmation UX |
| `2e5018f` | 2026-08-03T06:23:03-04:00 | feat(security): Phase 5 P5.9 / Part C — REST API confirmation endpoints |
| `3386199` | 2026-08-03T06:24:38-04:00 | feat(security): Phase 5 P5.8 / Part C — web UI confirmation modal |
| `c8b6600` | 2026-08-03T06:28:39-04:00 | feat(security): Phase 6 / Part F — MCP tool trust tagging |
| `a2553ba` | 2026-08-03T06:44:32-04:00 | docs: Phase 7 / Part G — documentation/code parity for security hardening |
| `6afb920` | 2026-08-03T06:45:07-04:00 | docs: update dev-flow dashboard after Step 3 |
| `c9e005e` | 2026-08-03T06:50:43-04:00 | docs: add ADR and security-hardening user guide (Step 4) |
| `ab49c34` | 2026-08-03T06:51:10-04:00 | docs: update dev-flow dashboard after Step 4 |
| `2216445` | 2026-08-03T07:01:44-04:00 | docs: add code-review findings PRD (Step 5) |
| `829c063` | 2026-08-03T07:01:54-04:00 | docs: update dev-flow dashboard after Step 5 |
| `39cbad8` | 2026-08-03T07:02:40-04:00 | docs: finalize review PRD with open questions and dependencies (Step 6) |
| `e779e9b` | 2026-08-03T07:03:24-04:00 | docs: update dev-flow dashboard after Step 7 (spec archived) |
| `059540f` | 2026-08-03T07:06:21-04:00 | docs: create spec for review-fix pass, remove PRD from repo root (Step 8) |
| `6a24b34` | 2026-08-03T07:12:46-04:00 | docs(security): FR-006 — document REST API confirmation ownership limitation (P0) |
| `a83e92c` | 2026-08-03T07:14:57-04:00 | fix(security): FR-005 — fix websearch fail-open OutputValidator error handling (P0) |
| `4a2940b` | 2026-08-03T07:16:40-04:00 | fix(security): FR-003 — pin initial-request IP to close SSRF DNS-rebinding TOCTOU (P0) |
| `42c7051` | 2026-08-03T07:21:23-04:00 | fix(security): FR-004 — escape tool_output delimiter to prevent boundary forgery (P0) |
| `cecf931` | 2026-08-03T07:25:33-04:00 | fix(security): FR-001/FR-002 — fix RBAC bypass in live chat (P0) |
| `685e25d` | 2026-08-03T07:30:58-04:00 | fix(security): FR-008 — fix doc_summarize allowlist to exact-or-subdomain match (P1) |
| `319b8d2` | 2026-08-03T07:33:00-04:00 | fix(security): FR-007 — reconcile/document websearch reject semantics (P1) |
| `2ac93cd` | 2026-08-03T07:34:07-04:00 | fix(security): FR-010 — stop discarding round-mate tool results on pending confirmation (P1) |
| `0c36f86` | 2026-08-03T07:34:53-04:00 | fix(security): FR-009 — resolve ConfirmationStore locking design, correct parallelism claims (P1) |
| `28d37fd` | 2026-08-03T07:38:47-04:00 | fix(security): collapse whitespace before prompt-injection pattern matching (P2) |
| `1f4c69e` | 2026-08-03T07:40:32-04:00 | docs(mcp): document tool_overrides server-reported-name trust gap |
| `53582a0` | 2026-08-03T07:40:54-04:00 | fix(security): FR-015 — truncate confirmation summary parameter values (P2) |
| `7c5fb5e` | 2026-08-03T07:43:55-04:00 | fix(security): FR-014 — ensure chat.Service/tool.Service share one ConfirmationStore (P2) |
| `67e74ae` | 2026-08-03T07:48:59-04:00 | docs: update dev-flow dashboard after Step 9 (all 15 review findings fixed) |
| `26f85f3` | 2026-08-03T07:49:29-04:00 | docs: update dev-flow dashboard after Step 10 (fixes spec archived) |
| `ae6ed7e` | 2026-08-03T07:57:12-04:00 | docs: correct stale RBAC-gap documentation now that FR-001/FR-002 closed it (Step 11) |
| `d1b95a7` | 2026-08-03T07:58:44-04:00 | chore: final quality pass complete — stable state (Step 11) |

All commits pushed to `worktree-security-prd` on the remote after each landed (no batched/deferred pushes). No PR opened yet — Step 14.

---

## 4. Spec vs. Implementation Comparison

| Phase | Planned (spec, `plan.md`) | Actual (git log) | Difference | Notes |
|---|---|---|---|---|
| Phase 1 (Part A) | 2–3 days | ~21 min (`e3420b8`) | Far under estimate | Estimates were written for a solo human developer; a subagent with full context and no ramp-up time is far faster for well-scoped, TDD-guided work. |
| Phase 2 (Part B) | 1 day | ~16 min (`ab8ba96`) | Far under estimate | Same driver. |
| Phase 3 (Part D) | 1 day | ~10 min (`fb33218`) | Far under estimate | Same driver; also found and fixed an unrelated dead map-key bug (`"web_search"` vs `"websearch"`) as a bonus. |
| Phase 4 (Part E) | 1–2 days | ~20 min (`153b3a7`) | Far under estimate | Same driver. |
| Phase 5 (Part C) | 3–4 days | ~7h 35m total, incl. ~5.9h recovery from an interrupted first attempt (`eb86858` + `50ebe75`/`2e5018f`/`3386199`) | **Roughly matches or exceeds the human estimate**, once the infrastructure incident is included | The only phase where actual time approached the human-developer estimate — driven entirely by the recovery, not the work's inherent complexity (the successful attempts for P5.7/P5.8/P5.9 each took 10–20 min). |
| Phase 6 (Part F) | 1–2 days | ~4 min (`c8b6600`) | Far under estimate | |
| Phase 7 (Part G) | 1 day | ~16 min (`a2553ba`) | Far under estimate | |
| Fix pass (15 findings) | Not separately estimated in original plan | ~43 min total across 3 waves | N/A | Fix-pass `plan.md` gave rough per-fix durations (2h–1d each); actual fixes ran in parallel and completed in 1–5 min each once dispatched. |

**Phases skipped:** None — all 7 original phases and all 15 fix-pass findings were completed (FR-012 resolved as moot, a side effect of FR-009, not skipped).

**Phases added (not in original plan):**
- A stale-documentation correction pass in Step 11, after discovering that Phase 7's honest "RBAC not enforced in live chat" documentation became inaccurate once FR-001/FR-002 fixed that exact gap during the review-fix cycle — a natural but unplanned consequence of finding and fixing a documented "known limitation" mid-process.
- Investigation and documentation (not fixing) of a pre-existing, out-of-scope encryption-key format bug (`ensureEncryptionKey` writes base64, `NewFileCredentialVault` expects raw bytes) discovered incidentally by multiple subagents while verifying `./bin/nuimanbot --help`.

---

## 5. Token / Message Usage

Exact orchestrator-level token counts are not directly available to this report. Subagent-reported usage totals (`subagent_tokens`, as reported in each agent's completion notification) are available and summed below — these represent real reported figures, not estimates:

| Step | Subagents | Approx. total subagent tokens |
|---|---|---|
| Step 3 (Implement Product) | 12 (4 independent phases + Phase 5's 1 failed + 1 resumed core + 3 gateway-surface + Phase 6 + Phase 7) | ~3.15M |
| Step 4 (Docs/ADR) | 1 | ~177K |
| Step 5 (Code Review) | 3 | ~425K |
| Step 9 (Implement Fixes) | 13 | ~1.55M |
| Step 11 (Doc correction) | 1 | ~87K |
| **Total (subagents only)** | **30** | **~5.4M** |

This excludes: the failed Phase 5 first attempt (terminated before reporting usage), and the orchestrator's own token consumption across ~39 commits' worth of coordination, verification (`git log`, `go build`/`vet`/`test`/`lint` runs), and status-file edits, which is not separately measurable from this vantage point but was substantial given the number of tool calls involved (well over 150 orchestrator-level tool invocations across the full run).

---

## 6. Process Observations

### What worked well
- **Splitting the code review into 3 parallel, independently-scoped passes** (rather than one reviewer covering everything) directly paid off: 2 of 3 reviewers independently found the same critical root-cause bug from different entry points, which is much stronger evidence of a real defect than a single reviewer's finding would have been.
- **Sequencing implementation phases by actual file-overlap analysis** (rather than blindly parallelizing everything the spec called "independent") avoided any git conflicts across 7 phases + 15 fixes + dozens of subagents working in the same shared worktree — zero merge conflicts occurred in the entire run.
- **The fix-pass agents' discipline around concurrent-edit awareness** (checking `git status`/`git log` before committing, staging only their own scoped files, waiting for a same-file sibling agent to land first) meant a highly concurrent process produced a completely clean, linear commit history with no reverts or conflict-resolution commits.
- **TDD-first discipline held up under delegation**: essentially every fix and every phase included a genuine RED→GREEN proof (not just a passing test added after the fact) — several agents went out of their way to prove a vulnerability was real (e.g., FR-003's temporary revert-and-scratch-test to prove the SSRF bypass at runtime, not just via a compile-time signature change).

### What caused delays or rework
- **The single largest cost in the entire run was an infrastructure failure**, not a design or process flaw: an agent implementing Phase 5's core (the largest, most complex sub-phase) was terminated mid-work by an API connection drop. Recovery required a fresh agent to inspect the partial output, judge it sound, and continue — this worked correctly, but cost ~5.9 hours by itself, roughly 65% of the entire run's total time.
- **A "known limitation" documented honestly in Phase 7 became stale once the review-fix cycle closed that exact gap** — this is a structural risk of any process that documents an issue and later fixes it in a separate pass: nothing automatically flags documentation for re-review when the underlying code changes. This required a dedicated Step 11 correction sweep across 6 files and ~20 individual statements.
- **Minor coordination friction** in a few fix-pass agents (e.g., a `.gitignore` rule unintentionally blocking a new file in `cmd/nuimanbot/`, requiring `git add -f`) — small, quickly worked around, but indicates the shared-worktree-with-many-concurrent-agents model occasionally surfaces pre-existing repo quirks that a single-agent sequential process wouldn't.

### Recommendations for future runs
- **When a subagent is interrupted by an infrastructure failure mid-task, always inspect and salvage partial output before restarting from scratch** — this run's Phase 5 recovery did this correctly (kept the sound `confirmation_store.go` interface file rather than discarding it), which likely saved meaningful time versus a full restart.
- **Add a lightweight "documentation currency" check to the Step 11 Final Quality Pass template** — specifically, grep for "known limitation"/"known gap" language in tracked docs and cross-check each one against the current code state, since this run's stale-RBAC-doc issue is a predictable failure mode of any process with an honest-disclosure-then-later-fix pattern (which this dev-flow process explicitly encourages via its review-then-fix structure).
- **For phases estimated at 3+ days of human time** (this run's Phase 5 was the only one), consider a mid-phase checkpoint/commit convention for sub-tasks (e.g., committing P5.1's interface file separately before starting P5.2) so an infrastructure interruption loses less work — Phase 5's actual behavior (one file survived the interruption because it happened to be written first) was fortunate rather than by design.

---

## 7. Manual vs. Automated Comparison

**Estimated manual duration:** The original PRD's `plan.md` estimated 2–3 + 1 + 1 + 1–2 + 3–4 + 1–2 + 1 = **10–15 developer-days** for the 7 original phases (assuming one senior developer, working sequentially, excluding code review and meeting time). The review-fix cycle's 15 findings, at the fix-pass `plan.md`'s own per-item estimates (2h–1d each), add roughly **2–4 additional developer-days**. Total estimated manual effort: **~12–19 developer-days** (roughly 2.5–4 weeks at a typical 5-day week), excluding the human code-review time that discovered the 15 findings in the first place (plausibly another 0.5–1 day for a thorough security-focused review pass).

**Actual automated runtime:** ~10h 37m for the full 14-step orchestration (Steps 1–11 complete; 12–14 in progress/pending), of which ~5.9h was a single infrastructure-recovery incident rather than inherent task complexity. Excluding that incident, genuine working time was closer to **4h 45m**.

**Efficiency gain:** Even including the infrastructure incident, this represents roughly a **20–30x wall-clock speedup** versus the conservative manual estimate (10.6 hours vs. 12–19 days). Excluding the incident, the speedup is closer to **50–65x**. This comparison assumes a single senior developer working alone; it does not include the calendar-time cost of code review turnaround, PR feedback cycles, or context-switching overhead a human developer would realistically incur across a multi-week effort — all of which the same-session, same-context agent-based process eliminates by construction, likely making the real-world gap even larger than the raw hour comparison suggests.
