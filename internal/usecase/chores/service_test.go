package chores

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// fakeEvaluator is a table-driven test double for ScheduleEvaluator,
// avoiding a dependency on internal/infrastructure/scheduler per
// AGENTS.md's Clean Architecture layering (usecase must not import
// infrastructure concretes).
type fakeEvaluator struct {
	invalidExprs   map[string]bool
	nextFireErrors map[string]bool
	fixedNext      time.Time
}

func newFakeEvaluator() *fakeEvaluator {
	return &fakeEvaluator{
		invalidExprs:   map[string]bool{"bogus cron": true},
		nextFireErrors: map[string]bool{"unschedulable cron": true},
		fixedNext:      time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
	}
}

func (f *fakeEvaluator) Validate(cronExpr string) error {
	if f.invalidExprs[cronExpr] {
		return errors.New("invalid cron expression")
	}
	return nil
}

func (f *fakeEvaluator) NextFireTime(cronExpr string, _ time.Time) (time.Time, error) {
	if f.nextFireErrors[cronExpr] {
		return time.Time{}, errors.New("cannot compute next fire time")
	}
	return f.fixedNext, nil
}

func newTestService(t *testing.T) (*Service, *fakeEvaluator, domain.ChoreRepository, string) {
	t.Helper()
	repo := newInMemoryChoreRepository()
	eval := newFakeEvaluator()
	hiddenRoot := t.TempDir()
	svc := NewService(repo, eval, hiddenRoot)
	return svc, eval, repo, hiddenRoot
}

func dailySchedule(t *testing.T) domain.Schedule {
	t.Helper()
	s, err := domain.NewScheduleFromPreset(domain.SchedulePresetDaily)
	if err != nil {
		t.Fatalf("unexpected error building schedule: %v", err)
	}
	return s
}

func TestCreateChore_UserConfirmedSetsNextFireTime(t *testing.T) {
	svc, eval, _, _ := newTestService(t)
	c, err := svc.CreateChore(context.Background(), "alice", "Nightly backup", "run the backup script", "", dailySchedule(t), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.ScheduleConfirmed {
		t.Fatal("expected ScheduleConfirmed to be true for a user-set schedule")
	}
	if !c.NextFireTime.Equal(eval.fixedNext) {
		t.Fatalf("expected NextFireTime %v, got %v", eval.fixedNext, c.NextFireTime)
	}
	if c.OwnerUserID != "alice" {
		t.Fatalf("expected OwnerUserID alice, got %s", c.OwnerUserID)
	}
}

func TestCreateChore_AgentProposedLeavesUnconfirmedAndNoNextFireTime(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	c, err := svc.CreateChore(context.Background(), "alice", "Weekly digest", "summarize the week", "", dailySchedule(t), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScheduleConfirmed {
		t.Fatal("expected ScheduleConfirmed to be false for an agent-proposed schedule")
	}
	if !c.NextFireTime.IsZero() {
		t.Fatalf("expected zero NextFireTime for an unconfirmed schedule, got %v", c.NextFireTime)
	}
	if c.IsDue(time.Now().Add(365 * 24 * time.Hour)) {
		t.Fatal("expected an unconfirmed chore to never be due, regardless of elapsed time")
	}
}

func TestCreateChore_InvalidCronRejected(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	sched, err := domain.NewScheduleFromCron("bogus cron")
	if err != nil {
		t.Fatalf("unexpected error building schedule: %v", err)
	}
	_, err = svc.CreateChore(context.Background(), "alice", "Bad chore", "desc", "", sched, true)
	if err == nil {
		t.Fatal("expected an error for an invalid cron expression")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

func TestCreateChore_NextFireTimeComputationErrorPropagates(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	sched, err := domain.NewScheduleFromCron("unschedulable cron")
	if err != nil {
		t.Fatalf("unexpected error building schedule: %v", err)
	}
	_, err = svc.CreateChore(context.Background(), "alice", "Bad chore", "desc", "", sched, true)
	if err == nil {
		t.Fatal("expected an error when NextFireTime computation fails")
	}
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

func TestCreateChore_RequiresOwnerAndTitle(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	sched := dailySchedule(t)

	if _, err := svc.CreateChore(context.Background(), "", "Title", "desc", "", sched, true); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for missing ownerUserID, got %v", err)
	}
	if _, err := svc.CreateChore(context.Background(), "alice", "", "desc", "", sched, true); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for missing title, got %v", err)
	}
}

func TestCreateChore_WritesDescriptionFileInHiddenDirectory(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	c, err := svc.CreateChore(context.Background(), "alice", "Nightly backup", "run the backup script", "", dailySchedule(t), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.HiddenDirectory == "" {
		t.Fatal("expected a non-empty HiddenDirectory")
	}
	data, err := os.ReadFile(filepath.Join(c.HiddenDirectory, "JOB-DESCRIPTION.md"))
	if err != nil {
		t.Fatalf("expected JOB-DESCRIPTION.md to exist: %v", err)
	}
	if string(data) != "run the backup script" {
		t.Fatalf("expected description file to contain the description, got %q", string(data))
	}
}

func TestCreateChore_HiddenDirectoryCreationErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "users")
	if err := os.WriteFile(blockingFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to set up test fixture: %v", err)
	}
	repo := newInMemoryChoreRepository()
	eval := newFakeEvaluator()
	svc := NewService(repo, eval, dir)

	_, err := svc.CreateChore(context.Background(), "alice", "Title", "desc", "", dailySchedule(t), true)
	if err == nil {
		t.Fatal("expected an error when the hidden directory cannot be created")
	}
}

func TestListChores_OwnerScoped(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := svc.CreateChore(ctx, "alice", "A1", "d", "", dailySchedule(t), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.CreateChore(ctx, "bob", "B1", "d", "", dailySchedule(t), true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list, err := svc.ListChores(ctx, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 || list[0].Title != "A1" {
		t.Fatalf("expected alice to see only her own chore, got %+v", list)
	}
}

func TestGetChore_CrossOwnerReturnsNotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	c, err := svc.CreateChore(ctx, "alice", "A1", "d", "", dailySchedule(t), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.GetChore(ctx, "bob", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound for cross-owner access, got %v", err)
	}
	got, err := svc.GetChore(ctx, "alice", c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != c.ID {
		t.Fatalf("expected to retrieve the same chore, got %+v", got)
	}
}

func TestDeleteChore_CrossOwnerReturnsNotFoundAndDoesNotDelete(t *testing.T) {
	svc, _, repo, _ := newTestService(t)
	ctx := context.Background()
	c, err := svc.CreateChore(ctx, "alice", "A1", "d", "", dailySchedule(t), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.DeleteChore(ctx, "bob", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound for cross-owner delete, got %v", err)
	}
	if _, err := repo.GetChore(ctx, "alice", c.ID); err != nil {
		t.Fatalf("expected alice's chore to remain undeleted, got %v", err)
	}
}

func TestDeleteChore_OwnerSuccess(t *testing.T) {
	svc, _, repo, _ := newTestService(t)
	ctx := context.Background()
	c, err := svc.CreateChore(ctx, "alice", "A1", "d", "", dailySchedule(t), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.DeleteChore(ctx, "alice", c.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.GetChore(ctx, "alice", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected chore to be gone, got %v", err)
	}
}

func TestConfirmSchedule_SetsConfirmedAndComputesNextFireTime(t *testing.T) {
	svc, eval, _, _ := newTestService(t)
	ctx := context.Background()
	c, err := svc.CreateChore(ctx, "alice", "Agent proposal", "d", "", dailySchedule(t), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ScheduleConfirmed {
		t.Fatal("precondition: expected unconfirmed chore")
	}

	if err := svc.ConfirmSchedule(ctx, "alice", c.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := svc.GetChore(ctx, "alice", c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.ScheduleConfirmed {
		t.Fatal("expected ScheduleConfirmed to be true after confirmation")
	}
	if !got.NextFireTime.Equal(eval.fixedNext) {
		t.Fatalf("expected NextFireTime %v, got %v", eval.fixedNext, got.NextFireTime)
	}
}

func TestConfirmSchedule_NextFireTimeComputationErrorPropagates(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	sched, err := domain.NewScheduleFromCron("unschedulable cron")
	if err != nil {
		t.Fatalf("unexpected error building schedule: %v", err)
	}
	c, err := svc.CreateChore(ctx, "alice", "Agent proposal", "d", "", sched, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.ConfirmSchedule(ctx, "alice", c.ID); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected domain.ErrInvalidInput, got %v", err)
	}
}

func TestConfirmSchedule_CrossOwnerReturnsNotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()
	c, err := svc.CreateChore(ctx, "alice", "A1", "d", "", dailySchedule(t), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.ConfirmSchedule(ctx, "bob", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound for cross-owner confirm, got %v", err)
	}
}

// inMemoryChoreRepository is a minimal domain.ChoreRepository test double,
// avoiding a dependency on internal/infrastructure/storage from this
// usecase package's tests.
type inMemoryChoreRepository struct {
	byOwner map[string]map[string]*domain.Chore
}

func newInMemoryChoreRepository() *inMemoryChoreRepository {
	return &inMemoryChoreRepository{byOwner: make(map[string]map[string]*domain.Chore)}
}

func (r *inMemoryChoreRepository) SaveChore(_ context.Context, c *domain.Chore) error {
	if r.byOwner[c.OwnerUserID] == nil {
		r.byOwner[c.OwnerUserID] = make(map[string]*domain.Chore)
	}
	cp := *c
	r.byOwner[c.OwnerUserID][c.ID] = &cp
	return nil
}

func (r *inMemoryChoreRepository) GetChore(_ context.Context, ownerUserID, choreID string) (*domain.Chore, error) {
	c, ok := r.byOwner[ownerUserID][choreID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (r *inMemoryChoreRepository) ListChores(_ context.Context, ownerUserID string) ([]*domain.Chore, error) {
	out := make([]*domain.Chore, 0)
	for _, c := range r.byOwner[ownerUserID] {
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

func (r *inMemoryChoreRepository) DeleteChore(_ context.Context, ownerUserID, choreID string) error {
	if _, ok := r.byOwner[ownerUserID][choreID]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byOwner[ownerUserID], choreID)
	return nil
}

func (r *inMemoryChoreRepository) UpdateNextFireTime(_ context.Context, ownerUserID, choreID string, next time.Time) error {
	c, ok := r.byOwner[ownerUserID][choreID]
	if !ok {
		return domain.ErrNotFound
	}
	c.NextFireTime = next
	c.UpdatedAt = next
	return nil
}

func (r *inMemoryChoreRepository) ListAllDue(_ context.Context, now time.Time) ([]*domain.Chore, error) {
	out := make([]*domain.Chore, 0)
	for _, chores := range r.byOwner {
		for _, c := range chores {
			if c.IsDue(now) {
				cp := *c
				out = append(out, &cp)
			}
		}
	}
	return out, nil
}

var _ domain.ChoreRepository = (*inMemoryChoreRepository)(nil)
