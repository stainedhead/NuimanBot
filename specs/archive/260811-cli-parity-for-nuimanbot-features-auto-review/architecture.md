# Architecture: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11
**Status:** Draft

## Architecture Overview

This fix pass makes targeted corrections within the existing Clean Architecture layering established by the original CLI-parity feature (`specs/archive/260811-cli-parity-for-nuimanbot-features/`). No new layers, packages, or cross-cutting components are introduced. Changes are confined to:

- **Usecase layer:** `internal/usecase/jobs` (FR-002) — the sole place this pass crosses the original spec's "reused as-is" Non-Goals boundary (spec.md line 141), by deliberate, PRD-mandated exception.
- **Adapter layer:** `internal/adapter/gateway/cli` (FR-001, FR-003, FR-007).
- **Infrastructure layer:** `internal/infrastructure/scheduler` (FR-002 doc comment only), `internal/infrastructure/audit` (referenced but not modified — FR-005's actual wiring is a separate, not-yet-filed follow-up FR).
- **Documentation:** `architecture.md`/code comments (FR-004), the original spec's NFR text (FR-005).

## Component Architecture

No new components. Existing components modified:

| Component | Layer | Finding | Change |
|---|---|---|---|
| `jobs.Service.CreateJob` | Usecase | FR-002 | Add ownership check for both `ContextTypeProject` and `ContextTypeChat`; reject foreign-owned context IDs instead of silently swallowing the lookup error. |
| `memories_commands.go` `browse()` | Adapter/CLI | FR-001 | Set `MemoryCellFilter.Limit`; add content truncation and a "N more not shown" trailer. |
| `project_commands.go`, `job_commands.go`, `chore_commands.go`, `history_commands.go` | Adapter/CLI | FR-003 | Replace generic "Unknown command" fallthrough for `chat` subcommand with a specific "not yet implemented" message, matching Settings' existing convention. |
| `auth_commands_test.go` | Adapter/CLI (test) | FR-007 | Add end-to-end restore-path test for stale-role correction. |
| `stub_executor.go` | Infrastructure | FR-002 | Doc comment only — flag future-executor ownership-recheck obligation. |
| `defaultRoleForPlatform` (chat.Service) | Usecase | FR-004 | Doc comment only — flag dead-in-practice-but-not-dead-in-code status of the `PlatformCLI → RoleAdmin` branch. |

## Layer Responsibilities

Unchanged from the original feature's architecture. This pass does not alter dependency direction or introduce new interfaces at layer boundaries, with one exception under review: FR-002 may introduce a new sentinel error type in the usecase layer (see `data-dictionary.md`) to let the adapter layer distinguish "not owned" from "not found" — if introduced, it follows the existing convention of usecase-layer-defined, adapter-layer-consumed error types.

## Data Flow

**FR-002 (the one flow-relevant change):** `CreateJob`'s current flow for `ContextTypeProject` is: look up via `projectLookup.OutputDirectoryFor(ctx, ownerUserID, contextID)` → on error, swallow and leave `WorkingDirectory` empty → job creation proceeds regardless. The fixed flow must be: look up ownership-scoped → on "not found for this owner" specifically, reject the `CreateJob` call before persisting → distinguish this from the already-correct "found but working directory unavailable" stale-reference tolerance (`TestCreateJob_StaleProjectReferenceStillCreatesJob`). `ContextTypeChat` currently has no lookup step at all in this flow; the fix adds one, matching (and extending) the Project pattern.

## Sequence Diagrams

[TBD] — a sequence diagram for FR-002's corrected `CreateJob` ownership-check flow (distinguishing "not owned" vs. "stale/deleted but owned" vs. "genuinely not found") would clarify the fix during implementation; add during Phase 3 (Architecture) if the implementing workstream finds the three-way branch logic ambiguous from prose alone.

## Integration Points

- **FR-002 ↔ `StubExecutor`:** `internal/infrastructure/scheduler/stub_executor.go`'s `checkProjectExists` is the only place a Project reference is actually dereferenced at runtime today; it is already owner-scoped. FR-002's fix moves the ownership check earlier (to `CreateJob` time) but does not change `StubExecutor`'s existing behavior — the doc-comment addition documents the *future* obligation on a real executor, it does not implement it.
- **FR-004 ↔ `Gateway.Start`/`AuthCommandHandler`:** No code change; the existing wiring-order dependency (auth gate must run before the REPL loop) is documented, not altered.
- **FR-005 ↔ `internal/infrastructure/audit`:** Referenced as the target package for a future follow-up FR; no integration work happens in this pass.

## Architectural Decisions

- **AD-F1 (FR-002 boundary-crossing):** `internal/usecase/jobs` was declared "reused as-is" by the original spec's Non-Goals (spec.md line 141). This pass deliberately crosses that boundary because the original spec's own line-157 acceptance criterion requires the behavior FR-002 delivers, and that criterion was never actually satisfied. Decision: cross the boundary, document it explicitly in the fix's commit/PR description, do not treat it as scope creep.
- **AD-F2 (FR-004 documentation-only, no structural fix in this pass):** The PRD offers two possible structural fixes (refuse-to-run-without-authHandler; change `defaultRoleForPlatform`'s default) but does not decide between them. Decision: this pass only documents the current mitigation's dependency on wiring order; the structural fix itself is optional, non-blocking follow-up work, left for a future pass if picked up.
- **AD-F3 (FR-005 two-part closure, not audit-logging implementation):** FR-005 is closed by (a) correcting spec.md's NFR text and (b) filing a new tracked FR — not by implementing audit logging in this pass. Decision: keep this pass's scope to "make the gap honest and tracked," not "close the gap," since implementing audit logging touches both `web/auth.go` and `cli/auth_commands.go` and was not scoped as part of the original CLI-parity feature.
- **AD-F4 (FR-005 edits a git-tracked archived file, by deliberate choice):** The PRD's Dependencies section refers to editing "spec.md" as if it were still the active spec; by the time this fix pass runs, that file has moved to `specs/archive/260811-cli-parity-for-nuimanbot-features/spec.md`, which — unlike this active spec's own `specs/` directory — is git-tracked (specs are gitignored except `specs/archive/`, per AGENTS.md). Decision: proceed with editing it anyway. An NFR that overstates what actually exists belongs corrected in the historical record, not left inaccurate because the spec has since been archived. This means C.2's fix produces a real, committed diff to a completed artifact — call this out explicitly in that commit's message so it isn't mistaken for an accidental edit to archived history. FR-002 (Workstream A), by contrast, only *satisfies* that same spec.md's line-157 acceptance criterion through code behavior — it does not edit the file — so there is no coordination needed between FR-002 and FR-005 on this point.
