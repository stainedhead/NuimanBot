# Research: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05
**Source PRD:** `nuimanbot-extend-context-and-ui-PRD.md` (this directory)

## Research Questions

Seeded from the PRD's Open Questions and Dependencies/Risks table. **Update (spec review, 2026-08-05):** Q1, Q2, Q5, Q6, and Q4 are resolved below (decisions also recorded in `spec.md`'s "Architecture decisions resolved during spec review" and `implementation-notes.md`). Q3 (default numeric values) remains intentionally open, to be pinned in `plan.md` during Phase 4 — see rationale under Q3.

1. **Cron scheduling — RESOLVED.** Use `github.com/robfig/cron/v3` (new `go.mod` dependency), not an in-house parser. It provides `Schedule.Next(time.Time) time.Time` directly, which both FR-035 (skip-if-still-running) and the restart-durability NFR need; it's the de facto standard, stable, MIT-licensed Go cron library, so an in-house parser would reinvent well-tested logic. Dependency-footprint concern is low — the repo already accepts comparable third-party deps.
2. **Worker pool crash recovery — RESOLVED.** `AtomicFileWriter` + `FileLock` (existing primitives, already used for `users.json`/`bots.json`) are sufficient for queue and in-flight-run persistence; no write-ahead log is needed. The worker pool is single-process/single-machine, so write volume and concurrency are bounded — the same atomic-rename + flock guarantee that protects `users.json` today (a write lands whole or not at all) is exactly what's needed here. Revisit only if Phase 2 integration testing surfaces write-contention the existing primitives can't handle.
3. **Default retention values — intentionally still open.** Sensible default auto-delete windows for Chat/Project/History and a default worker pool size N are not pinned here; the PRD explicitly defers this to spec/implementation planning, and `plan.md`'s Phase 4 (Implementation Plan) is the designated place to pin exact numbers with rationale (see spec.md's Risks table: "Default retention values set too aggressively" — Medium, mitigation is to document explicit defaults during planning). Not resolving it now is deliberate, not an oversight: picking real defaults benefits from architecture.md's Phase 3 output first (e.g., worker pool concurrency model informs a sensible N).
4. **Storage/disk quota policy — RESOLVED, out of scope.** No storage-usage surfacing, warning, or quota enforcement ships in this spec. "Never" retention is a valid, unenforced user choice (see spec.md's Non-Goals, added during spec review). Flagged as a candidate follow-up PRD if unbounded growth becomes an operational problem in practice.
5. **WebSocket connection lifecycle and per-user isolation — RESOLVED.** One WebSocket connection per browser tab, each subscribed server-side to a per-user broadcast channel (not per-run); a run-status/log/notification event publishes once per user and fans out to every open tab for that user. Reconnect is client-driven (simple backoff) with a full HTTP-API state resync on reconnect — no server-side replay buffer needed. The existing Buzz/Nostr gateway's `gorilla/websocket` usage (`internal/infrastructure/nostr/client.go`) is a different protocol context (Nostr relay semantics) and offers no directly reusable auth/framing pattern; this is net-new engineering, confirmed during spec review.
6. **Project directory sandboxing — RESOLVED.** No existing reusable path-confinement helper exists in the codebase (verified during spec review: `internal/infrastructure/preprocess/sandbox.go`'s `CommandSandbox` sandboxes *command execution*, not filesystem path containment; `internal/config.FetchSecurityConfig` guards SSRF/network targets, not local paths). A new dedicated helper (indicative name `internal/infrastructure/fsguard`, exact package/API finalized in `architecture.md`) providing a single `ResolveWithin(baseDir, relPath string) (string, error)` choke point must be built and used by every Job/Chore/Project file operation.

## Industry Standards

[TBD — research cron expression standards (POSIX cron vs. extended formats), WebSocket auth patterns for admin dashboards, and FIFO job-queue durability patterns during Phase 1.]

## Existing Implementations

- `internal/domain/conversation_repository.go` + `internal/infrastructure/storage/file_conversation_repository.go` — reference pattern for a per-user, file-based, atomic-write, restart-durable entity store. Study this file in detail before designing `ProjectRepository`/`JobRepository`/`ChoreRepository`/`RunRepository`.
- `internal/infrastructure/storage/atomic_file_writer.go` — `AtomicFileWriter` (temp-file+rename) and `FileLock` (flock-based, already used for `users.json`/`bots.json`) are the durable-persistence primitives to reuse for all new entities and worker-pool run-state.
- `internal/domain/memoryv2/` (`memory_cell.go`, `memory_scene.go`, `memory_cell_repository.go`, `memory_scene_repository.go`, `filter.go`) — reference for read-only query/filter patterns the Memories environment needs.
- `internal/domain/skill.go` + `internal/config/skills_config.go` (`SkillRepository`, `SkillsConfig`, `DefaultSkillsConfig()`) — existing model Settings' Skills/Plugins surface exposes as-is.
- `internal/adapter/web/server.go` — existing `net/http.ServeMux` routing composition (`adminHandler := requireRole(admin) + requirePasswordChange`) and `internal/adapter/web/middleware.go` (`requireRole(domain.Role)`, CSRF, login rate limiting via `ratelimit.TokenBucket`) — the auth/session/role middleware new environments must build on.
- `internal/infrastructure/nostr/client.go` — only existing consumer of `gorilla/websocket`; review for any reusable connection-handling idioms even though it is a different gateway/protocol context.
- `internal/usecase/chat/export.go` (`Service.ExportConversation`, `ExportFormatJSON`/`ExportFormatMarkdown`) — **found during spec review, not in the original PRD.** Already exports a `Conversation`'s full message history in both formats; the reference implementation for FR-016 (Chat output download) rather than new code.
- `internal/infrastructure/preprocess/sandbox.go` (`CommandSandbox`) — **checked during spec review and ruled out** as a reusable base for Project directory sandboxing (Q6): it sandboxes *command execution* (working dir + timeout + output capture), not filesystem path containment. No path-confinement (`filepath.Clean`/prefix-check against a base dir) logic exists there or elsewhere in the codebase today.

## API Documentation

[TBD — `gorilla/websocket` API reference relevant to server-side upgrade handling, ping/pong keepalive, and graceful close, to be captured once architecture.md's WebSocket design is drafted.]

## Best Practices

[TBD — Go cron library evaluation notes, FIFO queue-with-N-workers implementation patterns in Go (worker pool via buffered channel vs. explicit queue+dispatcher), and file-based job-queue durability best practices.]

## Open Questions

Carried from the PRD (two already-resolved items are reflected directly in `spec.md`'s FR-001–FR-004 and the WebSocket architecture note). **Update (spec review, 2026-08-05):** of the two items originally listed here, the storage/quota question is now resolved (see Research Questions Q4 above, and spec.md's Non-Goals) — only the retention-defaults question remains genuinely open:

- Default numeric values for Chat/Project/History retention and default worker pool size N — not pinned in the PRD; to be proposed in `plan.md` Phase 4, deliberately after `architecture.md`'s Phase 3 output is available (see Research Questions Q3 above for rationale).

## References

- Source PRD: `nuimanbot-extend-context-and-ui-PRD.md` (this directory), "Dependencies and Risks" and "Open Questions" sections.
- `AGENTS.md` — Clean Architecture layering and TDD workflow that all research conclusions must respect.
