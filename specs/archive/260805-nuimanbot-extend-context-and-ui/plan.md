# Plan: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05
**Status:** Implementation

## Development Approach

Strict TDD per AGENTS.md (Red-Green-Refactor per component), building inside-out per
Clean Architecture layer: domain entities/interfaces first, then infrastructure
(fsguard, file repositories, worker pool/scheduler), then usecase orchestration, then
adapter/web handlers+templates. The Job/Chore worker pool + scheduler is built and
proven durable (restart-recovery tests) before any UI depends on it.

## Phase 4: Pinned Default Values

Resolving research.md Q3 (deliberately deferred until this point):

- **Chat retention default:** 90 days (non-"Never"; user/admin may change per-user, including to "Never").
- **Project retention default:** 180 days (Projects are durable working context; longer default than Chats).
- **History retention default:** 90 days (matches Chat; run logs/results are the bulkiest artifact).
- **Worker pool size N default:** 3 concurrent workers. Rationale: safe for a single-machine deployment without operator tuning; small enough to bound LLM-API concurrent-call cost/rate-limit exposure, large enough that one long-running Chore doesn't starve the Job queue.
- **Network access mode default:** `localhost-only`, no allowlist — matches spec.md's Breaking Changes requirement that existing single-machine deployments are unaffected by upgrade.

## Phase Breakdown

1. **Foundation (domain):** `Project`, `Job`, `Chore`, `Run` entities + repository interfaces; `RetentionPolicy`, `Schedule`, `NetworkAccessConfig`, `WorkerPoolConfig` value objects; `Conversation` extensions (`Name`, retention-relevant `UpdatedAt` already present). Package: `internal/domain`.
2. **Foundation (infrastructure primitives):** `internal/infrastructure/fsguard` (`ResolveWithin`) with adversarial path-traversal tests; file-based repositories for Project/Job/Chore/Run in `internal/infrastructure/storage`, modeled on `FileConversationRepository`, using `AtomicFileWriter`/`FileLock`.
3. **Worker pool + scheduler:** `internal/infrastructure/scheduler` — FIFO queue, N-worker pool, `robfig/cron/v3`-backed schedule evaluation, crash-safe persistence, restart-recovery. Proven with integration tests before UI depends on it.
4. **Network access + Settings foundation:** `NetworkAccessConfig`/`WorkerPoolConfig`/retention-defaults additions to `internal/config`; allowlist middleware (pre-auth, fail-closed) in `internal/adapter/web/middleware.go`.
5. **Usecase layer:** per-environment packages (`internal/usecase/project`, `job`, `chore`, `history`, `memories`), plus `internal/usecase/chat` extensions (auto-naming, retention, reuse of `ExportConversation`).
6. **Adapter/web layer:** handlers + templates for all six environments + Settings; WebSocket hub for run status/log/notification push; nav/branding pass.
7. **Cross-cutting hardening:** retention sweeps, notification badges, per-user isolation tests (incl. admin-cannot-see-other-users, IDOR→404), allowlist tests, restart-durability tests, path-traversal tests.

## Critical Path

Phase 3 (worker pool + scheduler) blocks Phases 5/6 for Jobs/Chores/History. No existing
analog in the codebase; highest risk per spec.md's Risks table.

## Testing Strategy

Per AGENTS.md: strict TDD, Red-Green-Refactor (refactor mandatory), tests co-located
per package. Priority: restart-durability, per-user isolation (incl. IDOR→404),
path-traversal (fsguard), allowlist (allow/deny/malformed), cron/schedule (presets, raw
expression validation, skip-if-still-running), WebSocket per-user isolation. Domain and
usecase layers target ≥90% coverage per AGENTS.md quality gates.

## Rollout Strategy

All new environments require an authenticated session (same as `/admin/*` today).
Default network access mode is localhost-only with no allowlist — existing
single-machine deployments are unaffected until an admin opts into remote access.
Default retention values are conservative but not infinite (see Phase 4 above) —
"Never" is an explicit opt-in per resource type, not the shipped default.

## Success Metrics

All PRD acceptance criteria pass (spec.md); quality gates pass (`go fmt`, `go vet`,
`golangci-lint run`, `go test ./...`, `go build`); restart-durability integration tests
pass; branding/palette requirements visually confirmed.
