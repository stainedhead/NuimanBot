package scheduler

import (
	"context"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// fakeChoreRepository is a minimal in-memory domain.ChoreRepository for
// testing ChoreScheduler in isolation from the real file-based repository.
type fakeChoreRepository struct {
	chores map[string]*domain.Chore // choreID -> chore
}

func newFakeChoreRepository() *fakeChoreRepository {
	return &fakeChoreRepository{chores: make(map[string]*domain.Chore)}
}

func (f *fakeChoreRepository) SaveChore(_ context.Context, c *domain.Chore) error {
	f.chores[c.ID] = c
	return nil
}

func (f *fakeChoreRepository) GetChore(_ context.Context, ownerUserID, choreID string) (*domain.Chore, error) {
	c, ok := f.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeChoreRepository) ListChores(_ context.Context, ownerUserID string) ([]*domain.Chore, error) {
	var out []*domain.Chore
	for _, c := range f.chores {
		if c.OwnerUserID == ownerUserID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeChoreRepository) DeleteChore(_ context.Context, ownerUserID, choreID string) error {
	c, ok := f.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	delete(f.chores, choreID)
	return nil
}

func (f *fakeChoreRepository) UpdateNextFireTime(_ context.Context, ownerUserID, choreID string, next time.Time) error {
	c, ok := f.chores[choreID]
	if !ok || c.OwnerUserID != ownerUserID {
		return domain.ErrNotFound
	}
	c.NextFireTime = next
	return nil
}

func (f *fakeChoreRepository) ListAllDue(_ context.Context, now time.Time) ([]*domain.Chore, error) {
	var out []*domain.Chore
	for _, c := range f.chores {
		if c.IsDue(now) {
			out = append(out, c)
		}
	}
	return out, nil
}

var _ domain.ChoreRepository = (*fakeChoreRepository)(nil)

// fakeRunRecorder records every Run saved.
type fakeRunRecorder struct {
	runs []*domain.Run
}

func (f *fakeRunRecorder) SaveRun(_ context.Context, r *domain.Run) error {
	f.runs = append(f.runs, r)
	return nil
}

// fakeRunEnqueuer is a controllable stand-in for WorkerPool.
type fakeRunEnqueuer struct {
	enqueued []RunRequest
	running  map[string]bool
}

func newFakeRunEnqueuer() *fakeRunEnqueuer {
	return &fakeRunEnqueuer{running: make(map[string]bool)}
}

func (f *fakeRunEnqueuer) Enqueue(req RunRequest) error {
	f.enqueued = append(f.enqueued, req)
	return nil
}

func (f *fakeRunEnqueuer) IsSourceRunning(sourceID string) bool {
	return f.running[sourceID]
}

func TestChoreScheduler_FiresDueChore(t *testing.T) {
	chores := newFakeChoreRepository()
	runs := &fakeRunRecorder{}
	pool := newFakeRunEnqueuer()

	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	chore := &domain.Chore{
		ID: "chore-1", OwnerUserID: "user-a",
		Schedule:          domain.Schedule{CronExpression: "0 * * * *"},
		ScheduleConfirmed: true,
		NextFireTime:      now.Add(-time.Minute),
	}
	if err := chores.SaveChore(context.Background(), chore); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}

	s := NewChoreScheduler(chores, runs, pool, time.Hour)
	s.now = func() time.Time { return now }
	s.tick(context.Background())

	if len(pool.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued run, got %d", len(pool.enqueued))
	}
	if pool.enqueued[0].SourceID != "chore-1" {
		t.Fatalf("expected enqueued run for chore-1, got %+v", pool.enqueued[0])
	}
	if len(runs.runs) != 1 || runs.runs[0].Status != domain.RunStatusQueued {
		t.Fatalf("expected 1 queued run record, got %+v", runs.runs)
	}

	// NextFireTime must have advanced past `now`.
	updated := chores.chores["chore-1"]
	if !updated.NextFireTime.After(now) {
		t.Fatalf("expected NextFireTime to advance past now, got %v", updated.NextFireTime)
	}
}

func TestChoreScheduler_SkipsWhenPreviousRunActive(t *testing.T) {
	// FR-035: if the previous run is still active when the next one comes
	// due, the new run is skipped and logged, not enqueued.
	chores := newFakeChoreRepository()
	runs := &fakeRunRecorder{}
	pool := newFakeRunEnqueuer()
	pool.running["chore-1"] = true

	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	chore := &domain.Chore{
		ID: "chore-1", OwnerUserID: "user-a",
		Schedule:          domain.Schedule{CronExpression: "0 * * * *"},
		ScheduleConfirmed: true,
		NextFireTime:      now.Add(-time.Minute),
	}
	if err := chores.SaveChore(context.Background(), chore); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}

	s := NewChoreScheduler(chores, runs, pool, time.Hour)
	s.now = func() time.Time { return now }
	s.tick(context.Background())

	if len(pool.enqueued) != 0 {
		t.Fatalf("expected no new run enqueued while previous run active, got %d", len(pool.enqueued))
	}
	if len(runs.runs) != 1 || runs.runs[0].Status != domain.RunStatusSkipped {
		t.Fatalf("expected 1 skipped run record, got %+v", runs.runs)
	}
	if runs.runs[0].SkipReason == nil || *runs.runs[0].SkipReason != "skipped — previous run still active" {
		t.Fatalf("expected skip reason to be recorded, got %+v", runs.runs[0].SkipReason)
	}

	// Chore resumes at its next scheduled time even when skipped.
	updated := chores.chores["chore-1"]
	if !updated.NextFireTime.After(now) {
		t.Fatalf("expected NextFireTime to advance even when skipped, got %v", updated.NextFireTime)
	}
}

func TestChoreScheduler_UnconfirmedChoreNeverFires(t *testing.T) {
	chores := newFakeChoreRepository()
	runs := &fakeRunRecorder{}
	pool := newFakeRunEnqueuer()

	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	chore := &domain.Chore{
		ID: "chore-1", OwnerUserID: "user-a",
		Schedule:          domain.Schedule{CronExpression: "0 * * * *"},
		ScheduleConfirmed: false, // pending confirmation
		NextFireTime:      now.Add(-time.Hour),
	}
	if err := chores.SaveChore(context.Background(), chore); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}

	s := NewChoreScheduler(chores, runs, pool, time.Hour)
	s.now = func() time.Time { return now }
	s.tick(context.Background())

	if len(pool.enqueued) != 0 || len(runs.runs) != 0 {
		t.Fatalf("expected unconfirmed chore to never fire, got enqueued=%d runs=%d", len(pool.enqueued), len(runs.runs))
	}
}

func TestChoreScheduler_NoChoresDue(t *testing.T) {
	chores := newFakeChoreRepository()
	runs := &fakeRunRecorder{}
	pool := newFakeRunEnqueuer()

	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	chore := &domain.Chore{
		ID: "chore-1", OwnerUserID: "user-a",
		Schedule:          domain.Schedule{CronExpression: "0 * * * *"},
		ScheduleConfirmed: true,
		NextFireTime:      now.Add(time.Hour), // not due yet
	}
	if err := chores.SaveChore(context.Background(), chore); err != nil {
		t.Fatalf("SaveChore: %v", err)
	}

	s := NewChoreScheduler(chores, runs, pool, time.Hour)
	s.now = func() time.Time { return now }
	s.tick(context.Background())

	if len(pool.enqueued) != 0 || len(runs.runs) != 0 {
		t.Fatalf("expected nothing to fire, got enqueued=%d runs=%d", len(pool.enqueued), len(runs.runs))
	}
}
