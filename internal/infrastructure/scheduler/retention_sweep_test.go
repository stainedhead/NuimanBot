package scheduler

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/chats"
	"nuimanbot/internal/usecase/chores"
	"nuimanbot/internal/usecase/history"
	"nuimanbot/internal/usecase/jobs"
	"nuimanbot/internal/usecase/projects"
)

// noopEnqueuer is a jobs.RunEnqueuer that accepts every enqueue silently —
// RetentionSweeper's own tests never exercise Job creation/enqueue, only
// CleanupPendingDeletion, so nothing more than interface satisfaction is
// needed here.
type noopEnqueuer struct{}

func (noopEnqueuer) Enqueue(context.Context, jobs.EnqueueRequest) error { return nil }

// fixedEvaluator is a chores.ScheduleEvaluator that accepts every cron
// expression and always returns a fixed NextFireTime.
type fixedEvaluator struct{}

func (fixedEvaluator) Validate(string) error { return nil }
func (fixedEvaluator) NextFireTime(_ string, after time.Time) (time.Time, error) {
	return after.Add(24 * time.Hour), nil
}

// newTestJobsAndChoresServices constructs real jobs.Service/chores.Service
// backed by real file repositories under storagePath, sharing runRepo so
// CleanupPendingDeletion's active-run check sees the same Run records the
// rest of a test seeds.
func newTestJobsAndChoresServices(storagePath string, runRepo domain.RunRepository) (*jobs.Service, *chores.Service) {
	jobRepo := storage.NewFileJobRepository(storagePath)
	choreRepo := storage.NewFileChoreRepository(storagePath)
	files := storage.NewFileConfinedFileStore()

	jobsSvc := jobs.NewService(jobRepo, runRepo, noopEnqueuer{}, nil, nil, files, filepath.Join(storagePath, "jobs-hidden"))
	choresSvc := chores.NewService(choreRepo, fixedEvaluator{}, runRepo, files, filepath.Join(storagePath, "chores-hidden"))
	return jobsSvc, choresSvc
}

// newTestUserProfileRepo creates a real FileUserProfileRepository seeded
// with the given userIDs, so RetentionSweeper's per-user iteration is
// exercised against real storage, not a fake list.
func newTestUserProfileRepo(t *testing.T, storagePath string, userIDs ...string) *storage.FileUserProfileRepository {
	t.Helper()
	usersFile := filepath.Join(storagePath, "users.json")
	// Matches the 32-byte test key convention used by
	// file_user_profile_repository_test.go.
	repo := storage.NewFileUserProfileRepository(usersFile, "test-encryption-key-32bytes!ab")

	for _, id := range userIDs {
		p := &domain.UserProfile{
			UserID:        id,
			PrimaryEmail:  id + "@example.com",
			Role:          domain.RoleUser,
			Enabled:       true,
			DataDirectory: filepath.Join("users", id),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := repo.SaveProfile(context.Background(), p); err != nil {
			t.Fatalf("failed to seed user profile %q: %v", id, err)
		}
	}
	return repo
}

// TestRetentionSweep_DeletesExpiredDataAcrossRealRepositories is the FR-R3
// integration test: proves an expired Chat/Project/Run created via the
// real repositories is actually gone after a sweep cycle runs — not just
// that SweepExpired's isolated return value is correct (already covered by
// each usecase package's own unit tests), but that RetentionSweeper is
// actually the thing invoking it. Before this fix, nothing in the
// codebase ever called any of the three SweepExpired methods.
func TestRetentionSweep_DeletesExpiredDataAcrossRealRepositories(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()
	ownerUserID := "alice"

	userRepo := newTestUserProfileRepo(t, storagePath, ownerUserID)

	convRepo := storage.NewFileConversationRepository(storagePath)
	chatsSvc := chats.NewService(convRepo, nil, nil)

	projectRepo := storage.NewFileProjectRepository(storagePath)
	projectsSvc := projects.NewService(projectRepo, storage.NewFileConfinedFileStore(), storagePath)

	runRepo := storage.NewFileRunRepository(storagePath)
	historySvc := history.NewService(runRepo)

	// Seed one expired Chat, Project, and Run for alice.
	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	conv := &domain.Conversation{
		ID: "old-chat", UserID: ownerUserID, Platform: domain.PlatformCLI,
		CreatedAt: oldTime, UpdatedAt: oldTime,
	}
	if err := convRepo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	proj, err := projectsSvc.CreateProject(ctx, ownerUserID, "old project", filepath.Join(storagePath, "users", ownerUserID, "projects", "old"))
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj.UpdatedAt = oldTime
	if err := projectRepo.SaveProject(ctx, proj); err != nil {
		t.Fatalf("SaveProject (backdate): %v", err)
	}

	run := &domain.Run{
		ID: "old-run", OwnerUserID: ownerUserID, SourceType: domain.SourceTypeJob,
		SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: oldTime,
	}
	if err := runRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	jobsSvc, choresSvc := newTestJobsAndChoresServices(storagePath, runRepo)
	dayPolicy := domain.NewRetentionPolicy(24 * time.Hour)
	sweeper := NewRetentionSweeper(userRepo, chatsSvc, projectsSvc, historySvc, jobsSvc, choresSvc, dayPolicy, dayPolicy, dayPolicy, time.Hour)

	sweeper.tick(ctx)

	if _, err := convRepo.GetConversation(ctx, "old-chat"); err == nil {
		t.Fatal("expected expired chat to be deleted by the sweep")
	}
	if _, err := projectRepo.GetProject(ctx, ownerUserID, proj.ID); err == nil {
		t.Fatal("expected expired project to be deleted by the sweep")
	}
	if _, err := runRepo.GetRun(ctx, ownerUserID, "old-run"); err == nil {
		t.Fatal("expected expired run to be deleted by the sweep")
	}
}

// TestRetentionSweep_UnviewedRunSweepMarksNotifiedFirst is Edge Case #7,
// exercised through the actual sweep path (not just history.Service's own
// unit test): a swept, unviewed run must never leave the notification
// badge referencing a run that's since been deleted.
func TestRetentionSweep_UnviewedRunSweepMarksNotifiedFirst(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()
	ownerUserID := "alice"

	userRepo := newTestUserProfileRepo(t, storagePath, ownerUserID)
	convRepo := storage.NewFileConversationRepository(storagePath)
	chatsSvc := chats.NewService(convRepo, nil, nil)
	projectRepo := storage.NewFileProjectRepository(storagePath)
	projectsSvc := projects.NewService(projectRepo, storage.NewFileConfinedFileStore(), storagePath)
	runRepo := storage.NewFileRunRepository(storagePath)
	historySvc := history.NewService(runRepo)

	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	run := &domain.Run{
		ID: "unviewed-run", OwnerUserID: ownerUserID, SourceType: domain.SourceTypeJob,
		SourceID: "job-1", Status: domain.RunStatusCompleted, CreatedAt: oldTime,
	}
	if err := runRepo.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	before, err := runRepo.CountUnnotified(ctx, ownerUserID)
	if err != nil {
		t.Fatalf("CountUnnotified: %v", err)
	}
	if before != 1 {
		t.Fatalf("expected 1 unnotified run before sweep, got %d", before)
	}

	jobsSvc, choresSvc := newTestJobsAndChoresServices(storagePath, runRepo)
	dayPolicy := domain.NewRetentionPolicy(24 * time.Hour)
	sweeper := NewRetentionSweeper(userRepo, chatsSvc, projectsSvc, historySvc, jobsSvc, choresSvc, dayPolicy, dayPolicy, dayPolicy, time.Hour)
	sweeper.tick(ctx)

	after, err := runRepo.CountUnnotified(ctx, ownerUserID)
	if err != nil {
		t.Fatalf("CountUnnotified after sweep: %v", err)
	}
	if after != 0 {
		t.Fatalf("expected 0 unnotified runs after sweep (deleted run must not linger in the badge count), got %d", after)
	}
	if _, err := runRepo.GetRun(ctx, ownerUserID, "unviewed-run"); err == nil {
		t.Fatal("expected the run to actually be deleted")
	}
}

func TestRetentionSweep_NeverPolicyDeletesNothing(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()
	ownerUserID := "alice"

	userRepo := newTestUserProfileRepo(t, storagePath, ownerUserID)
	convRepo := storage.NewFileConversationRepository(storagePath)
	chatsSvc := chats.NewService(convRepo, nil, nil)
	projectRepo := storage.NewFileProjectRepository(storagePath)
	projectsSvc := projects.NewService(projectRepo, storage.NewFileConfinedFileStore(), storagePath)
	runRepo := storage.NewFileRunRepository(storagePath)
	historySvc := history.NewService(runRepo)

	oldTime := time.Now().Add(-1000 * 24 * time.Hour)
	conv := &domain.Conversation{ID: "ancient-chat", UserID: ownerUserID, Platform: domain.PlatformCLI, CreatedAt: oldTime, UpdatedAt: oldTime}
	if err := convRepo.SaveConversation(ctx, conv); err != nil {
		t.Fatalf("SaveConversation: %v", err)
	}

	jobsSvc, choresSvc := newTestJobsAndChoresServices(storagePath, runRepo)
	never := domain.NeverExpire()
	sweeper := NewRetentionSweeper(userRepo, chatsSvc, projectsSvc, historySvc, jobsSvc, choresSvc, never, never, never, time.Hour)
	sweeper.tick(ctx)

	if _, err := convRepo.GetConversation(ctx, "ancient-chat"); err != nil {
		t.Fatalf("expected chat to survive a Never-policy sweep, got err: %v", err)
	}
}

// TestRetentionSweep_CleansUpPendingDeletionJobsAndChores is the FR-R9
// integration test: a PendingDeletion Job/Chore whose Run has reached a
// terminal state is actually hard-deleted once RetentionSweeper's combined
// per-user loop runs — not just that jobs.Service/chores.Service's own
// CleanupPendingDeletion unit tests are correct in isolation, but that
// RetentionSweeper is actually the thing invoking them (mirroring FR-R3's
// integration test above).
func TestRetentionSweep_CleansUpPendingDeletionJobsAndChores(t *testing.T) {
	storagePath := t.TempDir()
	ctx := context.Background()
	ownerUserID := "alice"

	userRepo := newTestUserProfileRepo(t, storagePath, ownerUserID)
	convRepo := storage.NewFileConversationRepository(storagePath)
	chatsSvc := chats.NewService(convRepo, nil, nil)
	projectRepo := storage.NewFileProjectRepository(storagePath)
	projectsSvc := projects.NewService(projectRepo, storage.NewFileConfinedFileStore(), storagePath)
	runRepo := storage.NewFileRunRepository(storagePath)
	historySvc := history.NewService(runRepo)
	jobsSvc, choresSvc := newTestJobsAndChoresServices(storagePath, runRepo)

	job, err := jobsSvc.CreateJob(ctx, ownerUserID, "cleanup me", "d", domain.ContextTypeChat, "")
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := jobsSvc.DeleteJob(ctx, ownerUserID, job.ID); err != nil {
		t.Fatalf("DeleteJob (soft): %v", err)
	}
	sourceType := domain.SourceTypeJob
	jobRuns, err := runRepo.ListRuns(ctx, ownerUserID, domain.RunFilter{SourceType: &sourceType, SourceID: &job.ID})
	if err != nil || len(jobRuns) != 1 {
		t.Fatalf("expected exactly 1 run for job, got %v (err %v)", jobRuns, err)
	}
	jobRuns[0].Status = domain.RunStatusCompleted
	if err := runRepo.SaveRun(ctx, jobRuns[0]); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	schedule, err := domain.NewScheduleFromPreset(domain.SchedulePresetDaily)
	if err != nil {
		t.Fatalf("NewScheduleFromPreset: %v", err)
	}
	chore, err := choresSvc.CreateChore(ctx, ownerUserID, "cleanup me too", "d", "", schedule, true)
	if err != nil {
		t.Fatalf("CreateChore: %v", err)
	}
	if err := runRepo.SaveRun(ctx, &domain.Run{
		ID: "chore-run", OwnerUserID: ownerUserID, SourceType: domain.SourceTypeChore,
		SourceID: chore.ID, Status: domain.RunStatusCompleted, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := choresSvc.DeleteChore(ctx, ownerUserID, chore.ID); err != nil {
		t.Fatalf("DeleteChore (soft): %v", err)
	}

	never := domain.NeverExpire()
	sweeper := NewRetentionSweeper(userRepo, chatsSvc, projectsSvc, historySvc, jobsSvc, choresSvc, never, never, never, time.Hour)
	sweeper.tick(ctx)

	if _, err := jobsSvc.GetJob(ctx, ownerUserID, job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected PendingDeletion job to be hard-deleted by the sweep, got %v", err)
	}
	if _, err := choresSvc.GetChore(ctx, ownerUserID, chore.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected PendingDeletion chore to be hard-deleted by the sweep, got %v", err)
	}
}

// fakeSweeper is a minimal Sweeper test double for exercising RetentionSweeper's
// multi-user iteration and per-sweeper error resilience in isolation.
type fakeSweeper struct {
	calls []string // ownerUserIDs SweepExpired was called with
	err   error
}

func (f *fakeSweeper) SweepExpired(_ context.Context, ownerUserID string, _ domain.RetentionPolicy, _ time.Time) (int, error) {
	f.calls = append(f.calls, ownerUserID)
	if f.err != nil {
		return 0, f.err
	}
	return 0, nil
}

// fakeCleaner is a minimal Cleaner test double for exercising
// RetentionSweeper's multi-user iteration and per-cleaner error resilience
// in isolation, mirroring fakeSweeper.
type fakeCleaner struct {
	calls []string // ownerUserIDs CleanupPendingDeletion was called with
	err   error
}

func (f *fakeCleaner) CleanupPendingDeletion(_ context.Context, ownerUserID string) (int, error) {
	f.calls = append(f.calls, ownerUserID)
	if f.err != nil {
		return 0, f.err
	}
	return 0, nil
}

func TestRetentionSweep_SweepsEveryUser(t *testing.T) {
	storagePath := t.TempDir()
	userRepo := newTestUserProfileRepo(t, storagePath, "alice", "bob", "carol")

	chatsFake := &fakeSweeper{}
	projectsFake := &fakeSweeper{}
	historyFake := &fakeSweeper{}
	jobsFake := &fakeCleaner{}
	choresFake := &fakeCleaner{}
	never := domain.NeverExpire()
	sweeper := NewRetentionSweeper(userRepo, chatsFake, projectsFake, historyFake, jobsFake, choresFake, never, never, never, time.Hour)

	sweeper.tick(context.Background())

	if len(chatsFake.calls) != 3 {
		t.Fatalf("expected chats sweep called for 3 users, got %d: %v", len(chatsFake.calls), chatsFake.calls)
	}
	if len(projectsFake.calls) != 3 || len(historyFake.calls) != 3 {
		t.Fatalf("expected projects/history sweep called for 3 users each, got %d/%d", len(projectsFake.calls), len(historyFake.calls))
	}
	if len(jobsFake.calls) != 3 || len(choresFake.calls) != 3 {
		t.Fatalf("expected jobs/chores cleanup called for 3 users each, got %d/%d", len(jobsFake.calls), len(choresFake.calls))
	}
}

func TestRetentionSweep_OneSweeperErrorDoesNotBlockOthers(t *testing.T) {
	storagePath := t.TempDir()
	userRepo := newTestUserProfileRepo(t, storagePath, "alice")

	chatsFake := &fakeSweeper{err: os.ErrPermission}
	projectsFake := &fakeSweeper{err: os.ErrPermission}
	historyFake := &fakeSweeper{err: os.ErrPermission}
	jobsFake := &fakeCleaner{err: os.ErrPermission}
	choresFake := &fakeCleaner{err: os.ErrPermission}
	never := domain.NeverExpire()
	sweeper := NewRetentionSweeper(userRepo, chatsFake, projectsFake, historyFake, jobsFake, choresFake, never, never, never, time.Hour)

	sweeper.tick(context.Background())

	// All five are independently best-effort: every sweeper/cleaner is
	// still invoked once per user even though each one errors.
	if len(chatsFake.calls) != 1 || len(projectsFake.calls) != 1 || len(historyFake.calls) != 1 {
		t.Fatalf("expected chats/projects/history to each run once despite all erroring, got %d/%d/%d",
			len(chatsFake.calls), len(projectsFake.calls), len(historyFake.calls))
	}
	if len(jobsFake.calls) != 1 || len(choresFake.calls) != 1 {
		t.Fatalf("expected jobs/chores cleanup to each run once despite erroring, got %d/%d", len(jobsFake.calls), len(choresFake.calls))
	}
}

func TestRetentionSweep_ListUsersErrorStopsCleanly(t *testing.T) {
	chatsFake := &fakeSweeper{}
	projectsFake := &fakeSweeper{}
	historyFake := &fakeSweeper{}
	jobsFake := &fakeCleaner{}
	choresFake := &fakeCleaner{}
	never := domain.NeverExpire()
	sweeper := NewRetentionSweeper(errUserLister{}, chatsFake, projectsFake, historyFake, jobsFake, choresFake, never, never, never, time.Hour)

	sweeper.tick(context.Background()) // must not panic

	if len(chatsFake.calls) != 0 {
		t.Fatalf("expected no sweeps when listing users fails, got %d chats calls", len(chatsFake.calls))
	}
}

// errUserLister is a UserLister test double that always fails.
type errUserLister struct{}

func (errUserLister) ListProfiles(context.Context, int, int) ([]*domain.UserProfile, error) {
	return nil, os.ErrPermission
}

func TestRetentionPolicyFromDays(t *testing.T) {
	if !RetentionPolicyFromDays(0).IsNever() {
		t.Error("expected 0 days to mean Never")
	}
	if !RetentionPolicyFromDays(-5).IsNever() {
		t.Error("expected a negative days value to mean Never")
	}
	p := RetentionPolicyFromDays(30)
	if p.IsNever() {
		t.Error("expected 30 days to not be Never")
	}
	now := time.Now()
	if p.IsExpired(now.Add(-31*24*time.Hour), now) != true {
		t.Error("expected content older than 30 days to be expired")
	}
	if p.IsExpired(now.Add(-29*24*time.Hour), now) != false {
		t.Error("expected content newer than 30 days to not be expired")
	}

	// Defensive overflow clamp: an absurdly large days value must not wrap
	// time.Duration's multiplication into a bogus (e.g. negative) result.
	huge := RetentionPolicyFromDays(math.MaxInt32)
	if huge.IsNever() {
		t.Error("expected a huge days value to still be a finite (clamped) policy, not Never")
	}
	// Sanity: a policy clamped to a huge-but-finite window should not
	// consider anything created within the last century "expired" (a
	// silent int64 overflow could otherwise wrap this negative).
	if huge.IsExpired(now.Add(-100*365*24*time.Hour), now) {
		t.Error("expected the clamped huge policy to not flag recent content as expired")
	}
}
