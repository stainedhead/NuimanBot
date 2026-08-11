# Data Dictionary: NuimanBot Extend Context & UI — Auto-Review Fix Pass

**Created:** 2026-08-05
**Purpose:** Document data structures introduced or modified by the 19-finding fix pass. This is a delta against the original feature's data dictionary (`specs/archive/260805-nuimanbot-extend-context-and-ui/data-dictionary.md`), not a restatement of it — existing entities (`Run`, `Job`, `Chore`, `Project`, `Conversation`, `MemoryCell`, `RetentionPolicy`, `WorkerPoolConfig`, etc.) are unchanged in shape unless noted below.

## New Entities / Interfaces

### `domain.HiddenFileWriter` (or equivalent name) — FR-R5

A small domain-level interface capturing "write/read a file confined to this entity's hidden/output directory," to be implemented by an infrastructure package using `fsguard.ResolveWithin` + `os`. Exact method set TBD during implementation; must be sufficient to replace all direct `os.MkdirAll`/`os.WriteFile`/`os.Stat` + `fsguard` imports currently in `jobs/service.go`, `chores/service.go`, `projects/service.go`.

Candidate shape (subject to implementation refinement):
```go
type ConfinedFileStore interface {
    WriteFile(baseDir, relPath string, data []byte, perm os.FileMode) error
    ReadFile(baseDir, relPath string) ([]byte, error)
    EnsureDir(baseDir, relPath string) error
    Stat(baseDir, relPath string) (os.FileInfo, error)
}
```

### `RunRepository.ReadLog` / `ReadResults` — FR-R17

New repository methods encapsulating what `history_handler.go`'s `readFileContentOrEmpty` currently does via raw `os.ReadFile`. Signature TBD; must accept a `Run` (or its log/results path fields) and return content + error, replacing direct filesystem access from the adapter layer.

## Modified Entities

### `domain.Run` — FR-R2, FR-R3

- `Error` field (already exists) gains a new conventional value on restart-interrupted runs: `"run interrupted by server restart"` (exact string per FR-R2's acceptance criteria).
- No new fields required for retention (FR-R3) — `RetentionPolicy.IsExpired` already exists and is correct; the fix wires an existing sweep to existing data.

### `domain.Job` / `domain.Chore` — FR-R8, FR-R9, FR-R14

- `Chore` gains soft-delete parity with `Job`: `PendingDeletion` status transition on `DeleteChore` when a run is active (mirrors `Job`'s existing `PendingDeletion` state — no new enum value needed if `Chore` already shares the same status type as `Job`; confirm during implementation).
- `Job.IsQueueable()` (FR-R14): either wired into the actual enqueue/dispatch path or removed along with its tests — no data shape change, a call-site/dead-code decision.

### `domain.WorkerPoolConfig` — FR-R15

- `Validate()` gains an upper-bound check on `worker_pool_size` (currently only rejects `<= 0`). New constant (e.g. `MaxWorkerPoolSize`, value TBD — "documented constant, or a multiple of `runtime.NumCPU()`" per the finding).

### `BaseData` (`server.go`) — FR-R19

- `UnviewedRunCount` population moves from History-page-only (`withUnviewedRunCount` called at 2 sites) to every authenticated page render — likely via a change to whatever shared "build BaseData for an authenticated request" constructor exists, or a per-handler composition change. No field shape change, only where/how often it's populated.

### `internal/usecase/projects/service.go`'s `CreateProject` input validation — FR-R18

- `outputDirectory` input now validated against a configured allowed root before use (via `fsguard.ResolveWithin` or equivalent on the directory itself, not just paths relative to it). Default allowed root: `<storagePath>/users/<ownerUserID>/projects/` (see `research.md` Question 4). No `Project` entity field shape change — this is an input-validation-path change.

### `fsguard.ResolveWithin` call sites — FR-R6

- No signature change implied by the finding itself, but acceptance criteria requires `filepath.EvalSymlinks` (or equivalent guard) applied to resolved paths before use, at minimum at the Project-output-directory call sites. If implemented as a new `fsguard` function/option rather than inline at each call site, document the new signature here once chosen.

### `MemoryCell.ConversationID` semantics — FR-R7

- No field shape change. The *meaning* of the value populated into `conversationIDFor(ownerUserID)` may change from "pass-through of `ownerUserID`" to "a real per-user mapping" depending on what FR-R7's gateway trace finds. If a mapping table is introduced, document its shape here once implemented.

## API Request/Response Types

No new HTTP-level request/response types are anticipated except:
- **FR-R11:** possible new POST form fields on the Settings update handler (allowlist entries, bind address) if the "extend the handler" resolution path is taken rather than the "grey out read-only fields" path (see `research.md` Question 3).
- **FR-R18:** `CreateProject`'s existing request shape is unchanged; only server-side validation behavior changes (previously-accepted out-of-root paths are now rejected with a validation error).

## Enumerations

No new enumerations anticipated. `Chore` status values should be confirmed to already include (or be extended to include) a `PendingDeletion` equivalent to `Job`'s, as part of FR-R8.

## References

- Original feature's full data dictionary: `specs/archive/260805-nuimanbot-extend-context-and-ui/data-dictionary.md`
- Source PRD findings (authoritative detail): [`nuimanbot-extend-context-and-ui-auto-review-PRD.md`](./nuimanbot-extend-context-and-ui-auto-review-PRD.md)
