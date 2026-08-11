# Data Dictionary: CLI Parity for NuimanBot Features — Auto-Review Fix Pass

**Feature:** cli-parity-for-nuimanbot-features-auto-review
**Created:** 2026-08-11

**Purpose:** This fix pass modifies existing behavior of already-defined domain/usecase types; it introduces no new domain entities. This document tracks the existing types touched by each finding, for reference during implementation.

## Entities Touched (existing, unmodified shape)

- **`domain.Job`** (`internal/domain/job` or equivalent) — `ContextID`/`ContextType` fields are the subject of FR-002's fix; no new fields added, only stricter validation before a `Job` is persisted with a foreign-owned `ContextID`.
- **`domain.User`** — referenced by FR-007's new test (pre-seeded with a stale role for `(PlatformCLI, "alice")`); no shape change.
- **`memoryv2.MemoryCell`** — FR-001 truncates/caps `Content` (existing field, `MaxContentLength` = 2000 chars) at the display layer only; no domain-model change.

## Value Objects

- **`memoryv2.MemoryCellFilter`** (`internal/domain/memoryv2/filter.go`) — existing `Limit int` field ("maximum number of results to return, 0 = no limit") must actually be set by `browse()` for FR-001; no new fields required.

## Interfaces

- **`jobs.Service.CreateJob`** (`internal/usecase/jobs/service.go`) — signature is not expected to change for FR-002; the fix is to the internal ownership-check logic before persisting, not to the public method contract. Confirm during implementation whether the error return needs a new sentinel error (e.g. `ErrContextNotOwned`) distinct from the existing not-found error, to let the CLI layer produce an accurate message.
- **`AuditLogger`** (`internal/infrastructure/audit/logger.go`) — existing interface/type, currently unused by `web/auth.go`; FR-005's follow-up FR (not this pass) will wire it into both `web/auth.go` and `cli/auth_commands.go`.

## Enumerations

- **`ContextType`** (`ContextTypeProject`, `ContextTypeChat`) — existing enum used by `jobs.Service.CreateJob`; FR-002 extends ownership-check coverage to `ContextTypeChat`, which currently has none.

## API Request/Response Types

Not applicable — this is a CLI-only remediation pass; no new HTTP request/response types are introduced. (The web-side `AuditLogger` wiring referenced in FR-005 is deferred to a separate, not-yet-filed follow-up FR.)

## New Types Anticipated (subject to implementation-time decision)

- Possibly a new sentinel error type for "context ID exists but is not owned by the caller" (FR-002) — distinct from "context ID does not exist / stale" to satisfy the acceptance criterion that the two cases be distinguishable. Decision left to the implementing workstream; document the final choice in `implementation-notes.md`.
- Possibly new "not yet implemented" message constants for the four deferred `chat` subcommands (FR-003), following the existing `retentionSetNotImplementedMessage`/`networkModeNotImplementedMessage` naming convention (e.g. `projectChatNotImplementedMessage`, `jobChatNotImplementedMessage`, `choreChatNotImplementedMessage`, `historyChatNotImplementedMessage`).
