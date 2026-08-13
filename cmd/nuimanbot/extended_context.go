package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"nuimanbot/internal/adapter/web"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/scheduler"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/chats"
	"nuimanbot/internal/usecase/chores"
	"nuimanbot/internal/usecase/history"
	"nuimanbot/internal/usecase/jobs"
	"nuimanbot/internal/usecase/memories"
	"nuimanbot/internal/usecase/projects"
	"nuimanbot/internal/usecase/settings"
)

// extendedContextServices carries the per-user environment usecase services
// wireExtendedContextEnvironments constructs, in addition to wiring them
// into webServer, so the CLI gateway's environment-command wiring
// (specs/260811-cli-parity-for-nuimanbot-features, Phase C) can share the
// exact same instances rather than constructing a second, disconnected set
// (which would still work — these are stateless wrappers over shared
// repositories — but Jobs/Chores/History all share the notifying-decorator-
// wrapped RunRepository and the worker pool constructed here, so
// reconstructing them independently would mean CLI-side Jobs/Chores don't
// push WebSocket notifications and don't share the one live worker pool).
// All fields are nil if wireExtendedContextEnvironments was never called
// (web UI disabled) — the CLI gateway wiring must treat that as "these five
// environments aren't available this run," not attempt a fallback
// construction, since Jobs/Chores/History have no meaning without the
// worker pool this function also constructs.
type extendedContextServices struct {
	Projects *projects.Service
	Jobs     *jobs.Service
	Chores   *chores.Service
	History  *history.Service
	Memories *memories.Service
}

// defaultRetentionSweepInterval is how often RetentionSweeper (FR-R3)
// checks every user's Chats/Projects/History for expired data. Deliberately
// coarser than defaultChoreSchedulerInterval — retention is a slow-moving,
// day-granularity concern, not something that needs sub-minute responsiveness.
const defaultRetentionSweepInterval = 15 * time.Minute

// defaultChoreSchedulerInterval is how often the Chore cron evaluator polls
// for due Chores (FR-032/FR-035).
const defaultChoreSchedulerInterval = 30 * time.Second

// jobRunEnqueuerAdapter adapts *scheduler.WorkerPool to jobs.RunEnqueuer:
// jobs.Service defines its own minimal enqueue interface (Clean
// Architecture — no direct usecase dependency on internal/infrastructure/
// scheduler), so this small adapter is the only place that bridges the two
// concrete shapes.
type jobRunEnqueuerAdapter struct {
	pool *scheduler.WorkerPool
}

func (a *jobRunEnqueuerAdapter) Enqueue(_ context.Context, req jobs.EnqueueRequest) error {
	return a.pool.Enqueue(scheduler.RunRequest{
		RunID:       req.RunID,
		OwnerUserID: req.OwnerUserID,
		SourceType:  domain.SourceTypeJob,
		SourceID:    req.SourceID,
		EnqueuedAt:  time.Now(),
	})
}

// projectDirectoryLookupAdapter adapts domain.ProjectRepository to
// jobs.ProjectDirectoryLookup, resolving a Project-context Job's default
// working directory (FR-026) without jobs.Service depending on
// internal/usecase/projects directly.
type projectDirectoryLookupAdapter struct {
	repo domain.ProjectRepository
}

func (a *projectDirectoryLookupAdapter) OutputDirectoryFor(ctx context.Context, ownerUserID, projectID string) (string, error) {
	p, err := a.repo.GetProject(ctx, ownerUserID, projectID)
	if err != nil {
		return "", err
	}
	return p.OutputDirectory, nil
}

// chatOwnershipCheckAdapter adapts domain.ConversationRepository to
// jobs.ChatOwnershipCheck (FR-002, auto-review fix pass), verifying a
// --chat contextID belongs to ownerUserID without jobs.Service depending on
// internal/usecase/chats directly. Mirrors chats.Service.GetChat's own
// ownership check (fetch by ID, then compare UserID) rather than importing
// that usecase package, per the same Clean Architecture rationale as
// projectDirectoryLookupAdapter above.
type chatOwnershipCheckAdapter struct {
	repo domain.ConversationRepository
}

func (a *chatOwnershipCheckAdapter) VerifyChatOwnership(ctx context.Context, ownerUserID, chatID string) error {
	conv, err := a.repo.GetConversation(ctx, chatID)
	if err != nil {
		return err
	}
	if conv.UserID != ownerUserID {
		return domain.ErrNotFound
	}
	return nil
}

// scheduleEvaluatorAdapter adapts internal/infrastructure/scheduler's free
// functions to chores.ScheduleEvaluator (Clean Architecture: chores.Service
// depends on this small interface, not the scheduler package directly).
type scheduleEvaluatorAdapter struct{}

func (scheduleEvaluatorAdapter) Validate(cronExpr string) error {
	return scheduler.ValidateCronExpression(cronExpr)
}

func (scheduleEvaluatorAdapter) NextFireTime(cronExpr string, after time.Time) (time.Time, error) {
	return scheduler.NextFireTime(cronExpr, after)
}

// skillLister is the subset of *skillusecase.InMemorySkillRegistry's
// interface this package needs, named locally to avoid importing the skill
// usecase package into this small adapter file's signatures.
type skillLister interface {
	List() []domain.Skill
}

// skillNamesAdapter adapts a skillLister to settings.SkillsLister.
type skillNamesAdapter struct {
	registry skillLister
}

func (a skillNamesAdapter) SkillNames() []string {
	if a.registry == nil {
		return nil
	}
	list := a.registry.List()
	names := make([]string, len(list))
	for i, sk := range list {
		names[i] = sk.Name
	}
	return names
}

// wireExtendedContextEnvironments constructs and starts the Job/Chore
// worker pool + cron scheduler (specs/260805-nuimanbot-extend-context-and-ui's
// largest net-new subsystem) and wires the Projects/Jobs/Chores/History/
// Memories environments into webServer. Also runs FR-R2's startup restart-
// reconciliation and starts FR-R3's retention sweep loop, both from
// specs/260805-nuimanbot-extend-context-and-ui-auto-review. userProfileRepo
// backs the retention sweep's per-user iteration (every user's Chats/
// Projects/History are swept, not just one). Returns the constructed
// WorkerPool so a later call (once the skill registry exists) can wire
// Settings' worker-pool-size control against the same live instance.
func wireExtendedContextEnvironments(ctx context.Context, app *application, webServer *web.Server, userProfileRepo domain.UserProfileRepository) (*scheduler.WorkerPool, extendedContextServices, error) {
	// Shared confined filesystem I/O (FR-R5): the sole implementation
	// Projects/Jobs/Chores depend on via domain.ConfinedFileStore, keeping
	// "os"/internal/infrastructure/fsguard out of the usecase layer.
	confinedFiles := storage.NewFileConfinedFileStore()

	// Projects (FR-017-023): no dependency on the worker pool.
	projectsService := projects.NewService(app.ProjectRepo, confinedFiles, app.StoragePath)
	webServer.SetProjectsService(projectsService)

	// Wrap RunRepo so every status/log/badge write also pushes a RunEvent
	// over WebSocket to the owning user's connected tab(s) (P6.8), instead
	// of requiring History/Jobs/Chores pages to poll. Wrapped once, here,
	// so the executor (writer) and Jobs/Chores/History services (writers
	// via MarkNotified, readers otherwise) all observe the same live
	// notifications; webServer.Hub() is non-nil from NewServer onward.
	runRepo := web.NewNotifyingRunRepository(app.RunRepo, webServer.Hub())

	// Worker pool + queue (FR-004, FR-027, FR-039). Queue state persists
	// under <storagePath>/scheduler/queue.json so queued Jobs survive a
	// restart (Reliability NFR).
	queuePath := filepath.Join(app.StoragePath, "scheduler", "queue.json")
	queue := scheduler.NewQueue(queuePath)
	if err := queue.Load(); err != nil {
		return nil, extendedContextServices{}, err
	}

	// Restart-recovery (FR-R2, Reliability NFR): must run after queue.Load
	// (so the queue snapshot reflects on-disk state) and before pool.Start
	// (so no worker has had a chance to pick up new work yet). Any Run left
	// Running, or Queued with no matching queue entry, is a prior process's
	// crash victim — reconciled to Failed with a clear, visible error
	// rather than silently stranded forever.
	if n, err := scheduler.ReconcileInterruptedRuns(ctx, runRepo, queue, time.Now); err != nil {
		slog.Error("failed to reconcile interrupted runs at startup", "error", err)
	} else if n > 0 {
		slog.Info("reconciled runs interrupted by a prior restart", "count", n)
	}

	runsArtifactRoot := filepath.Join(app.StoragePath, "scheduler", "runs")
	executor := scheduler.NewStubExecutor(runRepo, app.JobRepo, app.ChoreRepo, app.ProjectRepo, runsArtifactRoot)

	poolConfig := app.Config.WorkerPool.ToDomain() // defaults an unset/invalid value to DefaultWorkerPoolSize
	pool := scheduler.NewWorkerPool(queue, executor, poolConfig.MaxConcurrentWorkers)
	pool.Start(ctx)

	// Jobs (FR-024-030).
	jobsService := jobs.NewService(
		app.JobRepo,
		runRepo,
		&jobRunEnqueuerAdapter{pool: pool},
		&projectDirectoryLookupAdapter{repo: app.ProjectRepo},
		&chatOwnershipCheckAdapter{repo: app.ConversationRepo},
		confinedFiles,
		filepath.Join(app.StoragePath, "jobs-hidden"),
	)
	webServer.SetJobsService(jobsService)

	// Chores (FR-031-038) + their cron scheduler driver (FR-032/FR-035).
	choresService := chores.NewService(app.ChoreRepo, scheduleEvaluatorAdapter{}, runRepo, confinedFiles, filepath.Join(app.StoragePath, "chores-hidden"))
	webServer.SetChoresService(choresService)

	choreScheduler := scheduler.NewChoreScheduler(app.ChoreRepo, runRepo, pool, defaultChoreSchedulerInterval)
	go choreScheduler.Run(ctx)

	// History (FR-040-044). Uses the same notifying decorator so
	// MarkViewed's badge-clear (usecase/history.Service.MarkViewed ->
	// RunRepository.MarkNotified) also pushes the refreshed count.
	historyService := history.NewService(runRepo)
	webServer.SetHistoryService(historyService)

	// Memories (FR-045-047), read-only browse/search over the existing
	// memoryv2 store, plus AskAboutCell's minimal per-item chat (FR-R4's
	// reference implementation — see memories.LLMService's doc comment for
	// why this is a single-turn grounded call, not the full chat
	// orchestration engine). LLM defaults mirror chat.LLMDefaults's config
	// source (cfg.LLM.DefaultModel) so both share the same configured model.
	memoriesService := memories.NewService(
		app.MemoryCellRepo,
		memories.WithLLM(app.LLMService),
		memories.WithLLMDefaults(memories.LLMDefaults{
			Model:       app.Config.LLM.DefaultModel.Primary,
			MaxTokens:   app.Config.LLM.DefaultModel.MaxTokens,
			Temperature: app.Config.LLM.DefaultModel.Temperature,
		}),
	)
	webServer.SetMemoriesService(memoriesService)

	// Retention sweep (FR-R3): three FRs (FR-014/FR-023/FR-043) promise
	// "Chats/Projects/runs older than a configured, non-Never period are
	// deleted automatically" — SweepExpired existed and was unit-tested on
	// all three services, but nothing ever called it. A second chats.Service
	// instance is constructed here rather than threading the one main.go
	// builds for webServer.SetChatsService through — both are stateless
	// wrappers over the same app.ConversationRepo, so this has no behavioral
	// difference and avoids a bigger DI-wiring reshuffle just to share one
	// pointer. Retention is a system-wide default (not yet per-user
	// configurable — see settings.Service.RetentionDefaults's doc comment).
	sweepChats := chats.NewService(app.ConversationRepo)
	retentionSweeper := scheduler.NewRetentionSweeper(
		userProfileRepo,
		sweepChats,
		projectsService,
		historyService,
		jobsService,
		choresService,
		scheduler.RetentionPolicyFromDays(app.Config.RetentionDefaults.ChatDays),
		scheduler.RetentionPolicyFromDays(app.Config.RetentionDefaults.ProjectDays),
		scheduler.RetentionPolicyFromDays(app.Config.RetentionDefaults.HistoryDays),
		defaultRetentionSweepInterval,
	)
	go retentionSweeper.Run(ctx)

	return pool, extendedContextServices{
		Projects: projectsService,
		Jobs:     jobsService,
		Chores:   choresService,
		History:  historyService,
		Memories: memoriesService,
	}, nil
}

// wireSettingsEnvironment wires Settings (FR-001-004). Split from
// wireExtendedContextEnvironments because the skill registry it needs to
// read isn't constructed until later in Run() (see main.go) — pool is the
// *scheduler.WorkerPool returned by wireExtendedContextEnvironments.
// retention is plan.md Phase 4's pinned default retention windows, in days.
// Returns the constructed *settings.Service so the CLI gateway's Settings
// command handler (specs/260811-cli-parity-for-nuimanbot-features P4.1-P4.2)
// can share the exact same instance rather than constructing a second one
// against a different PoolController.
func wireSettingsEnvironment(webServer *web.Server, pool *scheduler.WorkerPool, skillRegistry skillLister, retention settings.RetentionDefaults) *settings.Service {
	svc := settings.NewService(
		pool,
		skillNamesAdapter{registry: skillRegistry},
		retention,
	)
	webServer.SetSettingsService(svc)
	return svc
}
