# Tasks: NuimanBot — Extend Context & UI

**Feature:** nuimanbot-extend-context-and-ui
**Date:** 2026-08-05
**Status:** In Progress

## Phase 1: Domain Foundation

- **P1.1** `internal/domain/retention.go` — `RetentionPolicy` value object (`Period *time.Duration`, nil = Never), shared by Chat/Project/History.
- **P1.2** `internal/domain/schedule.go` — `Schedule` value object (`CronExpression string`, `Preset *string`), preset→cron resolution table.
- **P1.3** `internal/domain/network_access.go` — `AccessMode` enum, `NetworkAccessConfig` value object (nil vs. empty allowlist distinction).
- **P1.4** `internal/domain/worker_pool_config.go` — `WorkerPoolConfig` value object.
- **P1.5** `internal/domain/project.go` + `project_repository.go` — `Project` entity, `ProjectRepository` interface.
- **P1.6** `internal/domain/job.go` + `job_repository.go` — `Job` entity, `JobStatus`/`ContextType` enums, `JobRepository` interface.
- **P1.7** `internal/domain/chore.go` + `chore_repository.go` — `Chore` entity, `ChoreRepository` interface.
- **P1.8** `internal/domain/run.go` + `run_repository.go` — `Run` entity, `RunStatus`/`SourceType` enums, `RunFilter`, `RunRepository` interface.
- **P1.9** Extend `internal/domain/message.go` `Conversation` with `Name string`; extend `ConversationSummary` with `Name string`.

## Phase 2: Infrastructure Primitives

- **P2.1** `internal/infrastructure/fsguard/fsguard.go` — `ResolveWithin(baseDir, relPath string) (string, error)`; adversarial tests (`../`, absolute paths, symlink escape, empty/`.`/null-byte input).
- **P2.2** `internal/infrastructure/storage/file_project_repository.go` (+ test) — modeled on `FileConversationRepository`.
- **P2.3** `internal/infrastructure/storage/file_job_repository.go` (+ test).
- **P2.4** `internal/infrastructure/storage/file_chore_repository.go` (+ test).
- **P2.5** `internal/infrastructure/storage/file_run_repository.go` (+ test), including `AppendLog` and `ListRuns` filtering.
- **P2.6** Extend `internal/infrastructure/storage/file_conversation_repository.go` for auto-naming persistence + `UpdatedAt`-based retention sweep helper.

## Phase 3: Worker Pool + Scheduler

- **P3.1** Add `github.com/robfig/cron/v3` to `go.mod`.
- **P3.2** `internal/infrastructure/scheduler/queue.go` — persisted FIFO queue (Job run requests), `AtomicFileWriter`/`FileLock`-backed.
- **P3.3** `internal/infrastructure/scheduler/pool.go` — N-worker pool consuming the queue, `SetConcurrency`, in-flight tracking, crash-safe run-state writes.
- **P3.4** `internal/infrastructure/scheduler/cron.go` — `robfig/cron/v3`-backed `Scheduler`, `NextFireTime`, `DueChores`, skip-if-still-running (FR-035).
- **P3.5** Restart-recovery integration test: simulate process restart, verify queued Jobs / in-flight run records / Chore next-fire-times all survive.

## Phase 4: Network Access + Settings Foundation

- **P4.1** `internal/config` additions: `NetworkAccessConfig`, `WorkerPoolConfig`, per-user retention defaults, wired into `config.yaml` + loader/validation. Defaults per plan.md Phase 4.
- **P4.2** `internal/adapter/web/middleware.go` — allowlist middleware (pre-auth, fail-closed on empty-but-present list); tests for allow/deny/malformed/absent-vs-empty.
- **P4.3** Localhost-only vs. remote bind wiring in `cmd/nuimanbot/main.go` / server startup.

## Phase 5: Usecase Layer

- **P5.1** `internal/usecase/chat` extensions — auto-naming (Edge Case #1 fallback), retention sweep, output download via existing `ExportConversation`.
- **P5.2** `internal/usecase/project` — CRUD, `AGENTS.md` presence check, retention sweep.
- **P5.3** `internal/usecase/job` — CRUD, `JOB-DESCRIPTION.md` persistence, enqueue-on-create, stale-Project-reference handling (Edge Case #2).
- **P5.4** `internal/usecase/chore` — CRUD, schedule confirm/discard flow (Edge Case #6), stale-Project-reference handling.
- **P5.5** `internal/usecase/history` — list/filter runs, per-run chat context assembly, retention sweep, notification badge state (Edge Case #7).
- **P5.6** `internal/usecase/memories` — read-only browse/search wrapper over `memoryv2` repositories.
- **P5.7** Per-user isolation tests across all of the above (IDOR → not-found, not forbidden).

## Phase 6: Adapter/Web Layer

- **P6.1** `internal/adapter/web/chats_handler.go` + templates.
- **P6.2** `internal/adapter/web/projects_handler.go` + templates.
- **P6.3** `internal/adapter/web/jobs_handler.go` + templates.
- **P6.4** `internal/adapter/web/chores_handler.go` + templates.
- **P6.5** `internal/adapter/web/history_handler.go` + templates.
- **P6.6** `internal/adapter/web/memories_handler.go` + templates.
- **P6.7** `internal/adapter/web/settings_handler.go` + templates (system-wide vs. per-user scope split).
- **P6.8** `internal/adapter/web/websocket_handler.go` — hub, per-user channel, run status/log/notification push.
- **P6.9** Nav/branding pass: `nav.html`, distinct palette, Nuiman marks on nav/login/favicon.
- **P6.10** Route registration in `server.go`, DI wiring in `cmd/nuimanbot/main.go`.

## Phase 7: Hardening / Final Verification

- **P7.1** Path-traversal adversarial tests (fsguard + Job/Chore file ops).
- **P7.2** Cross-user isolation tests (admin-cannot-see-other-users, IDOR → 404).
- **P7.3** Full quality-gate run: `go fmt`, `go vet`, `golangci-lint run`, `go test ./...` (coverage check), `go build -o bin/nuimanbot ./cmd/nuimanbot`, `./bin/nuimanbot --help`.
- **P7.4** `status.md` final update.

## Notes

Given the size of this feature (47 FRs, net-new worker-pool/scheduler subsystem, six UI
environments), Phases 1-5 (domain/infrastructure/usecase) are prioritized for full TDD
rigor and coverage, since AGENTS.md's ≥90% coverage gate applies to Domain/Usecase.
Phase 6 (adapter/web) implements working handlers/templates for all six environments
plus Settings; UI polish is functional-minimal, not a full design pass, given the scope
— see the final implementation report for anything explicitly descoped.
