package scheduler

import (
	"context"
	"log/slog"
	"time"

	"nuimanbot/internal/domain"
)

// Sweeper is anything with a per-owner SweepExpired method — the shape
// chats.Service, projects.Service, and history.Service all already share
// (FR-R3). Defined here rather than imported from those usecase packages,
// per AGENTS.md's Clean Architecture rule: this infrastructure-layer
// driver depends only on an interface it owns.
type Sweeper interface {
	SweepExpired(ctx context.Context, ownerUserID string, policy domain.RetentionPolicy, now time.Time) (int, error)
}

// Cleaner is anything with a per-owner CleanupPendingDeletion method — the
// shape jobs.Service and chores.Service already share (FR-R9). Combined
// into the same per-user loop as Sweeper's retention sweep (FR-R3) rather
// than a second scheduler.Run ticker loop, per the task's own guidance not
// to duplicate per-user iteration between the two.
type Cleaner interface {
	CleanupPendingDeletion(ctx context.Context, ownerUserID string) (int, error)
}

// UserLister is the minimal capability RetentionSweeper needs to enumerate
// every user in the system for its per-user sweep pass — satisfied
// structurally by internal/infrastructure/storage.FileUserProfileRepository.
type UserLister interface {
	ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
}

// listProfilesPageSize is how many profiles RetentionSweeper fetches per
// ListProfiles call while paging through every user.
const listProfilesPageSize = 100

// RetentionSweeper periodically, for every user in the system: calls
// SweepExpired for Chats, Projects, and History (FR-R3), deleting data
// older than each entity's configured system-wide retention window
// (FR-014/FR-023/FR-043's "auto-delete" promise — previously implemented
// and unit-tested in isolation, but never actually invoked by anything);
// and calls CleanupPendingDeletion for Jobs and Chores (FR-R9),
// hard-deleting a soft-deleted record once its Run reaches a terminal
// state. Modeled on ChoreScheduler's ticker-poll-loop pattern
// (Architectural Decision #3: reuse the codebase's single
// periodic-execution mechanism rather than introducing a second one).
type RetentionSweeper struct {
	users    UserLister
	chats    Sweeper
	projects Sweeper
	history  Sweeper
	jobs     Cleaner
	chores   Cleaner

	chatPolicy    domain.RetentionPolicy
	projectPolicy domain.RetentionPolicy
	historyPolicy domain.RetentionPolicy

	interval time.Duration
	now      func() time.Time
}

// NewRetentionSweeper creates a RetentionSweeper polling every interval,
// sweeping chats/projects/history against chatPolicy/projectPolicy/
// historyPolicy respectively, and cleaning up jobs/chores' PendingDeletion
// backlog, for every user users lists. Retention is a system-wide default
// (Settings/FR-003), not yet per-user configurable — see
// internal/usecase/settings.Service.RetentionDefaults's doc comment.
func NewRetentionSweeper(users UserLister, chats, projects, history Sweeper, jobs, chores Cleaner, chatPolicy, projectPolicy, historyPolicy domain.RetentionPolicy, interval time.Duration) *RetentionSweeper {
	return &RetentionSweeper{
		users:         users,
		chats:         chats,
		projects:      projects,
		history:       history,
		jobs:          jobs,
		chores:        chores,
		chatPolicy:    chatPolicy,
		projectPolicy: projectPolicy,
		historyPolicy: historyPolicy,
		interval:      interval,
		now:           time.Now,
	}
}

// Run polls until ctx is cancelled. Intended to be launched in its own
// goroutine by the caller (cmd/nuimanbot's DI wiring).
func (s *RetentionSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick sweeps every user once. Exported indirectly via Run, but kept
// callable directly in tests for determinism (no need to wait on a
// ticker).
func (s *RetentionSweeper) tick(ctx context.Context) {
	now := s.now()

	users, err := s.allUsers(ctx)
	if err != nil {
		slog.Error("retention sweep: failed to list users", "error", err)
		return
	}

	for _, userID := range users {
		s.sweepUser(ctx, userID, now)
	}
}

// allUsers pages through every UserProfile via ListProfiles, returning
// just the UserIDs (which double as ownerUserID across Chats/Projects/
// Jobs/Chores/History — see auth.go's session/User.Username wiring).
func (s *RetentionSweeper) allUsers(ctx context.Context) ([]string, error) {
	var ids []string
	for offset := 0; ; offset += listProfilesPageSize {
		page, err := s.users.ListProfiles(ctx, offset, listProfilesPageSize)
		if err != nil {
			return nil, err
		}
		for _, p := range page {
			ids = append(ids, p.UserID)
		}
		if len(page) < listProfilesPageSize {
			return ids, nil
		}
	}
}

// sweepUser runs all three retention sweeps plus both PendingDeletion
// cleanups for a single user. Each is independent and best-effort — one
// failing must not prevent the others from running, matching every other
// sweep/scheduler loop in this codebase (e.g. ChoreScheduler.fireOne,
// Service.SweepExpired's own per-item resilience).
func (s *RetentionSweeper) sweepUser(ctx context.Context, ownerUserID string, now time.Time) {
	if _, err := s.chats.SweepExpired(ctx, ownerUserID, s.chatPolicy, now); err != nil {
		slog.Error("retention sweep: chats sweep failed", "ownerUserID", ownerUserID, "error", err)
	}
	if _, err := s.projects.SweepExpired(ctx, ownerUserID, s.projectPolicy, now); err != nil {
		slog.Error("retention sweep: projects sweep failed", "ownerUserID", ownerUserID, "error", err)
	}
	if _, err := s.history.SweepExpired(ctx, ownerUserID, s.historyPolicy, now); err != nil {
		slog.Error("retention sweep: history sweep failed", "ownerUserID", ownerUserID, "error", err)
	}
	if _, err := s.jobs.CleanupPendingDeletion(ctx, ownerUserID); err != nil {
		slog.Error("retention sweep: jobs PendingDeletion cleanup failed", "ownerUserID", ownerUserID, "error", err)
	}
	if _, err := s.chores.CleanupPendingDeletion(ctx, ownerUserID); err != nil {
		slog.Error("retention sweep: chores PendingDeletion cleanup failed", "ownerUserID", ownerUserID, "error", err)
	}
}

// maxSafeRetentionDays is the largest days value that, multiplied out to a
// time.Duration in nanoseconds (24h * time.Hour per day), still fits
// within time.Duration's int64 range without silently wrapping — roughly
// Go's own ~292-year time.Duration ceiling, rounded down for a safety
// margin. Settings' own input validation keeps realistic values far below
// this, but RetentionPolicyFromDays clamps defensively rather than risk a
// silent overflow producing an arbitrary, wrong (and possibly much
// shorter) retention window.
const maxSafeRetentionDays = 100000

// RetentionPolicyFromDays converts a Settings-style days value (0 = Never)
// into a domain.RetentionPolicy, shared by cmd/nuimanbot's DI wiring for
// all three of Chat/Project/History's system-wide defaults.
func RetentionPolicyFromDays(days int) domain.RetentionPolicy {
	if days <= 0 {
		return domain.NeverExpire()
	}
	if days > maxSafeRetentionDays {
		days = maxSafeRetentionDays
	}
	return domain.NewRetentionPolicy(time.Duration(days) * 24 * time.Hour)
}
