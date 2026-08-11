# Architecture: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Status:** Draft
**Source:** PRD "Fix-Pass Execution Guidance" section (mandatory) — this document carries that section's structure through unchanged; it is not re-derived.

## Architecture Overview

This fix pass makes no new architectural additions beyond what's needed to close the 19 findings. The primary architectural correction is FR-R5: relocating three usecase packages' direct `os`/`fsguard` calls behind a new domain-defined interface, restoring the Clean Architecture dependency rule (dependencies flow inward only; inner layers define interfaces, outer layers implement them) that AGENTS.md mandates and that this branch otherwise respects everywhere except `jobs`, `chores`, `projects`.

Findings are grouped into 7 workstreams (A–G) by **shared-file/shared-concern collision**, not by priority — this is the PRD's own grouping, reproduced verbatim below because the pipeline's task instructions require this structure to carry through into `plan.md`/`tasks.md` rather than being re-invented.

## Workstream Grouping (carried from PRD, authoritative)

| Workstream | Findings | Why grouped | Sequencing |
|---|---|---|---|
| **A** — Restart & retention sweeps | FR-R2, FR-R3, FR-R9 | All add a startup-reconciliation or periodic-sweep loop wired into `cmd/nuimanbot`'s DI; FR-R9's own text says it can combine with FR-R3's sweep loop. | FR-R3 first (sweep scaffold) → FR-R9 (extends the same loop to `PendingDeletion`) → FR-R2 (separate startup-only reconciliation; can proceed in parallel with R3/R9 but coordinate on the shared `cmd/nuimanbot` wiring file). |
| **B** — Filesystem layering & sandboxing | FR-R5, FR-R6, FR-R18, FR-R13 | FR-R5/FR-R6/FR-R18 all touch `fsguard` call sites or the usecase/infrastructure boundary in `jobs`/`chores`/`projects`/`file_project_repository.go`. | FR-R5 first (relocates `fsguard` calls out of usecase into a new infrastructure interface), then FR-R6 (adds `EvalSymlinks` at the relocated call sites — don't fix it twice). FR-R18 (output-directory root confinement) can start in parallel but must rebase against FR-R5 before merging, since both touch `projects/service.go`. FR-R13 (`file_conversation_repository.go`) is fully independent. |
| **C** — Chore parity | FR-R8 | Chore soft-delete, mirroring Jobs. | Must merge *before* FR-R9 (workstream A) extends the cleanup sweep to Chores — FR-R9 assumes Chores already soft-delete correctly. |
| **D** — Deferred agent-facing surfaces | FR-R1, FR-R4 | Both are "implement, or explicitly document the deferral + UI notice" calls for agent-facing chat/reply surfaces. | Independent of every other workstream (touch `chats`/`jobs`/`chores`/`history`/`memories` handlers+templates only); ideally the same teammate, since the fallback documentation path is identical for both. |
| **E** — Live-update & observability plumbing | FR-R10, FR-R19 | Both wire an already-correct backend computation through to the browser/every page (WebSocket consumer JS; `UnviewedRunCount` on every `BaseData` build). | Independent of every other workstream; coordinate only if both touch `server.go`'s `BaseData` construction in the same edit window. |
| **F** — Settings & execution edge cases | FR-R11, FR-R12, FR-R14, FR-R15 | Narrow, independent fixes with no file overlap with each other or the groups above. | Fully parallelizable. |
| **G** — Consistency polish | FR-R16, FR-R17 | Template/adapter-layer polish; no security or data-integrity stakes. | Fully parallelizable; lowest scheduling priority. |

**Cross-workstream dependency graph:**

```
C (FR-R8) ──────────────► A.FR-R9 (must merge first)
B.FR-R5 ──────────────► B.FR-R6 (must merge first, same call sites)
B.FR-R5 ──────────────► B.FR-R18 (rebase before merge, shared file: projects/service.go)
A.FR-R3 ──────────────► A.FR-R9 (extends same sweep loop)
A.FR-R3 ── (parallel, coordinate on cmd/nuimanbot) ──► A.FR-R2
D, E, F, G: no dependencies on any other workstream
```

Suggested execution: one agent teammate per workstream (A–G), each in its own git worktree/branch, running TDD + quality gates + per-finding code review inside that worktree; merge in dependency order (A's internal R3→R9→R2 sequencing, B's R5→R6 sequencing, C landing before A's R9) — everything else merges whenever its workstream is green.

## Layer Responsibilities (affected by this fix pass)

- **Domain (`internal/domain`):** FR-R5 adds one new interface (confined file I/O). No other domain changes anticipated — zero non-stdlib imports must be preserved.
- **Usecase (`internal/usecase/{jobs,chores,projects,chats,history,memories}`):** FR-R5 (remove direct `os`/`fsguard` imports), FR-R8 (Chore soft-delete), FR-R9 (sweep-eligible query), FR-R1 (Chats agent-invocation call or documented deferral), FR-R7 (Memories `ConversationID` mapping), FR-R18 (Project root confinement), FR-R12 (Executor Project-existence check — technically infrastructure/`Executor` boundary, listed here since it's the same `Executor` interface usecase code depends on).
- **Adapter (`internal/adapter/web`):** FR-R4 (per-item chat UI, at least one template), FR-R10 (browser-side WebSocket consumer JS + DOM update), FR-R11 (Settings form field gating), FR-R16 (nav.html inclusion on legacy pages), FR-R17 (RunRepository-based read instead of raw `os.ReadFile`), FR-R19 (`BaseData` population on every authenticated render).
- **Infrastructure (`internal/infrastructure`):** FR-R2 (queue/pool restart reconciliation), FR-R3 (sweep loop implementation + `cmd/nuimanbot` DI wiring), FR-R6 (`fsguard` symlink mitigation), FR-R13 (`file_conversation_repository.go` fsguard routing), FR-R5's interface implementation.

## Data Flow (fix-pass specific)

**Restart reconciliation (FR-R2), on process startup:**
```
cmd/nuimanbot main() → new reconciliation step (before/alongside queue restore)
  → scan RunRepository for Runs in non-terminal state (Running, or Queued with no matching queue entry)
  → for each: re-enqueue (if idempotent-safe) OR mark Failed with Error="run interrupted by server restart"
  → History surfaces these distinguishably via the Error field
```

**Retention + cleanup sweep (FR-R3, FR-R9), periodic ticker:**
```
cmd/nuimanbot DI wires a ticker goroutine (pattern: ChoreScheduler's poll loop)
  → per user: chats.Service.SweepExpired, projects.Service.SweepExpired, history.Service.SweepExpired
  → same loop (or coordinated loop): scan PendingDeletion Jobs/Chores whose Run reached terminal state → hard delete
  → sweep-driven Run deletion must decrement notification badge / never reference a deleted Run (Edge Case #7)
```

**Symlink-safe confined I/O (FR-R5 + FR-R6), post-fix call path:**
```
usecase (jobs/chores/projects) → domain.ConfinedFileStore interface (no fsguard/os import)
  → infrastructure implementation → fsguard.ResolveWithin(baseDir, relPath)
  → filepath.EvalSymlinks (or equivalent) on the resolved path before any os open/write
  → applies at minimum to: file_project_repository.go, live Executor, jobs/chores/projects call sites
```

**Project output directory confinement (FR-R18):**
```
CreateProject(outputDirectory) → validate against allowed root
  (default: <storagePath>/users/<ownerUserID>/projects/, per research.md Q4)
  → fsguard.ResolveWithin (or equivalent) applied to the directory itself, not just paths relative to it
  → reject with validation error if outside allowed root (relative traversal or absolute path)
```

**Notification badge (FR-R19) + WebSocket consumer (FR-R10):**
```
Run completion → UnviewedRunCount computation (already correct)
  → FR-R19: populate BaseData.UnviewedRunCount on every authenticated page render, not just History
  → FR-R10: browser-side JS opens WS connection on Job/Chore/History detail pages,
     listens for run_status/run_log/notification_badge events, updates DOM without reload
```

## Sequence Diagrams

Not separately diagrammed — the data-flow ASCII sequences above are sufficient given the fix-pass nature (modifying existing flows, not introducing new multi-actor interactions). If any individual finding's fix proves to have non-obvious interaction ordering during implementation (e.g. Edge Case #7's sweep-vs-notification race), add a dedicated sequence diagram here at that time.

## Integration Points

- `cmd/nuimanbot`'s DI wiring file(s) are a shared edit surface for workstream A (FR-R2, FR-R3, FR-R9) — coordinate within the workstream, not across.
- `server.go`'s `BaseData` construction is a shared edit surface for workstream E (FR-R10, FR-R19) — both already in the same workstream.
- `projects/service.go` is a shared edit surface across workstream B (FR-R5, FR-R18) — explicit rebase-before-merge rule applies (see table above).
- `fsguard.go` and its 8 call sites are shared across FR-R6 (workstream B) and indirectly FR-R5 (workstream B, same workstream, sequenced).

## Architectural Decisions

1. **New domain interface for confined file I/O (FR-R5), not a repository-interface extension.** The PRD's acceptance criteria explicitly allows either "a small domain-level interface... or extend the existing repository interfaces" — the choice between these is left to implementation; document the actual choice in `implementation-notes.md` once made, and reflect the final shape in `data-dictionary.md`.
2. **Symlink mitigation applied at the relocated (post-FR-R5) call sites (FR-R6), not the pre-relocation ones.** Explicit PRD instruction: "don't fix it twice." Workstream B's internal sequencing (R5 before R6) exists specifically to enforce this.
3. **Sweep loop reuses the `ChoreScheduler` polling pattern (FR-R3, FR-R9) rather than introducing a second scheduling mechanism.** Keeps the codebase's periodic-execution pattern singular.
4. **Project output-directory default allowed root:** `<storagePath>/users/<ownerUserID>/projects/`, matching the existing per-user storage convention (FR-R18), unless an existing per-deployment "projects root" config value is found in `internal/config` during implementation (see `research.md` Q4).
5. **FR-R1's resolution (implement vs. defer) is not pre-decided here.** Per the PRD's Open Question 2 and this spec's `research.md`, the "implement" path is attempted first; if it falls back to "defer + document," that decision must be written into `implementation-notes.md`'s "Deviations from Plan" explicitly — this was the exact omission the original review flagged as the real gap (asymmetric treatment vs. the Jobs/Chores `StubExecutor` deferral, which *was* documented).

## References

- Source PRD "Fix-Pass Execution Guidance" section (mandatory): [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md), lines ~289–313.
- AGENTS.md — Clean Architecture layer structure and dependency rules.
