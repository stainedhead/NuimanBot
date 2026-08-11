# Implementation Notes: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05

## Purpose

Running log of implementation decisions, edge cases, and lessons learned during development. Update this file continuously during implementation — do not wait until the end. Each entry should be dated and reference the relevant FR/task ID where applicable.

## Technical Decisions

**2026-08-05 — `dev-flow:review-spec` pass.** The spec was reviewed for completeness/implementation-readiness before the large TDD effort begins. Several architecture-level decisions were left open in the initial spec creation pass (Phase 0) and were resolved during this review, since no human is available mid-pipeline for follow-up questions. Full rationale for each lives in `spec.md` ("Architecture decisions resolved during spec review") and `research.md` (Q1/Q2/Q5/Q6); summary:

- **Cron library:** `github.com/robfig/cron/v3` (new `go.mod` dependency) — chosen over an in-house parser. (research.md Q1)
- **Worker-pool persistence:** `AtomicFileWriter` + `FileLock` (existing primitives) — no WAL needed given single-process/single-machine scope. (research.md Q2)
- **Usecase package structure:** per-environment packages (`internal/usecase/project`, `job`, `chore`, `history`, `memories`) — matches the existing per-domain convention already in the repo (`chat/`, `user/`, `skill/`, `memory/`, `memoryv2/`). Verified against current `internal/usecase/` layout.
- **WebSocket model:** one connection per browser tab, subscribed to a per-user broadcast channel; reconnect = client backoff + full HTTP resync, no server-side replay buffer. (research.md Q5)
- **Project directory sandboxing:** must be a net-new dedicated helper (indicative `internal/infrastructure/fsguard`) — confirmed no existing reusable path-confinement utility exists (`preprocess.CommandSandbox` sandboxes command execution, not paths). (research.md Q6)
- **Storage/disk quota policy:** explicitly out of scope for this spec (added to spec.md Non-Goals), not deferred ambiguously. (research.md Q4)
- **Chat output download (FR-016):** reuse `internal/usecase/chat.Service.ExportConversation` (`internal/usecase/chat/export.go`, already implements JSON/Markdown transcript export) rather than building new export logic — found during this review; not referenced in the original PRD.
- **Chore Status / Run Status scope:** no "Paused" Chore state and no "Cancelled" Run state — neither is described by any FR in the PRD. Added as explicit Non-Goals in spec.md rather than left as `[TBD]` in the data dictionary.
- **Default retention values / worker pool size N:** deliberately left open, not resolved in this pass — the PRD explicitly defers this to spec/implementation planning, and `plan.md`'s Phase 4 is the designated place, after Phase 3 (Architecture) provides context (e.g., the worker-pool concurrency model informs a sensible default N). This is a scheduled future decision, not a gap.

## Edge Cases & Solutions

**2026-08-05 — 13 edge cases identified and resolved during spec review**, added as a new "Edge Cases" section in `spec.md` (between Non-Functional Requirements and System Architecture) so they read as required test cases, not an afterthought:

1. Chat auto-naming with empty/whitespace/non-text first message → timestamp-based fallback name.
2. Job/Chore referencing a since-deleted Project → run fails with explicit `Error`, record/history preserved, not auto-deleted.
3. Deleting a Job/Chore mid-run → in-flight run finishes; record removed only once terminal; no new runs enqueued in the meantime.
4. Worker pool N reduced below current running count → no pre-emption, pool just stops pulling new work until concurrency drops.
5. WebSocket reconnect mid-run → full HTTP-API state resync on reconnect, no server-side replay buffer.
6. Agent-proposed Chore schedule never confirmed → stays visibly "pending confirmation" indefinitely, never silently expires, never fires.
7. Retention sweep deletes an unviewed Run → notification badge count decrements as if viewed; badge never references a deleted run.
8. Concurrent `AGENTS.md` edits (agent via chat vs. user via filesystem) → last-write-wins, accepted tradeoff, no cross-boundary locking.
9. Project output directory becomes inaccessible mid-run → run fails with descriptive `Error`, worker doesn't crash/hang.
10. Cross-user resource access by ID (IDOR) → 404, not 403; existence never disclosed, including to admins.
11. Empty vs. absent network allowlist → absent = allow all; explicitly empty (`allowlist: []`) = deny all, fail-closed. Must be preserved through config parsing, not collapsed.
12. Chat retention vs. active use → retention window measured from `UpdatedAt`, not `CreatedAt`.
13. LLM/agent failure mid-run → treated as any other run failure (`Status=Failed`, `Error` populated, partial log preserved); no silent retry (retry policy out of scope).

## Deviations from Plan

- **Repository isolation strengthened beyond ConversationRepository's existing pattern.** `ConversationRepository.GetConversation`/`DeleteConversation` resolve by ID alone (scanning across all users' indexes) and rely on the usecase/handler layer to check `conv.UserID == requester`. The new `ProjectRepository`/`JobRepository`/`ChoreRepository`/`RunRepository` interfaces instead take `ownerUserID` explicitly on every Get/Delete/mutate method, so cross-owner access resolves to `ErrNotFound` at the data-access layer itself (file path is owner-scoped, so a wrong-owner lookup simply misses on disk). This is a deliberate strengthening, not an inconsistency: spec.md's Security NFR explicitly requires isolation "enforced at the data-access layer, not just hidden in the UI," and Edge Case #10 requires cross-user ID access to return 404 without disclosing existence, including to admins. Documented in each new repository interface's doc comment.
- **Worker pool `Executor` is a pluggable interface with no concrete agent-invocation implementation shipped in this pass.** `internal/infrastructure/scheduler.Executor` is the seam where a Job/Chore run would actually invoke the agent (LLM/tool-loop orchestration via `internal/usecase/chat` and friends) against the run's working directory, streaming output into the Run's log via `RunRepository.AppendLog`. Wiring a concrete `Executor` requires deep integration with the existing chat/LLM orchestration subsystem (context assembly, tool loop, timeout/cancellation, partial-log capture on failure per Edge Case #13) — a substantial, separately-testable body of work in its own right. `WorkerPool`/`Queue`/`ChoreScheduler` are fully built, tested (including under `-race`), and exercise the FIFO/concurrency/skip-if-still-running/restart-durability contracts against a fake `Executor` in tests. See the final implementation report for the explicit call on why this was descoped rather than rushed.

## Lessons Learned

- A test-only naming collision surfaced immediately via `go build ./...`: `internal/infrastructure/storage` already had a package-level `matchesFilter` helper (`ingatan_memory_cell_repository.go`) before `FileRunRepository`'s `ListRuns` filter helper was added under the same name. Renamed to `runMatchesFilter`. Running `go build ./...` (not just the new package) after every new file catches this class of issue immediately — kept doing so throughout.
