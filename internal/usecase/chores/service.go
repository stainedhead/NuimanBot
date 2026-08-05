// Package chores orchestrates the web admin's Chores environment
// (FR-031–FR-038): create/list/get/delete a recurring, cron-scheduled
// Chore backed by domain.ChoreRepository, plus agent-proposed schedule
// confirmation (FR-033). Modeled closely on internal/usecase/projects and
// internal/usecase/chats for consistency, per spec.md's guidance to model
// net-new entities on existing patterns.
package chores

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// descriptionFileName is the filename a Chore's Description is persisted
// under inside its HiddenDirectory, matching the convention documented on
// domain.Job.Description ("also persisted as JOB-DESCRIPTION.md in
// HiddenDirectory") — Chores reuse the same filename rather than inventing
// a parallel "CHORE-DESCRIPTION.md" convention, since both entities serve
// the same purpose (the task description an agent run reads on start) and
// any future run-execution code can read one filename regardless of
// whether it is running a Job or a Chore.
const descriptionFileName = "JOB-DESCRIPTION.md"

// ScheduleEvaluator validates and evaluates cron expressions. Defined here
// (rather than importing internal/infrastructure/scheduler directly) per
// AGENTS.md's Clean Architecture layering — this usecase package must not
// depend on infrastructure concretes; a concrete adapter wrapping
// internal/infrastructure/scheduler is wired in centrally (main.go).
type ScheduleEvaluator interface {
	// Validate reports whether cronExpr is a syntactically valid cron
	// expression.
	Validate(cronExpr string) error
	// NextFireTime returns the next time cronExpr fires strictly after
	// `after`.
	NextFireTime(cronExpr string, after time.Time) (time.Time, error)
}

// Service orchestrates Chore create/list/get/delete/schedule-confirmation.
type Service struct {
	chores     domain.ChoreRepository
	evaluator  ScheduleEvaluator
	runs       domain.RunRepository
	hiddenRoot string
	now        func() time.Time
}

// NewService creates a Chores Service backed by chores and evaluator. runs
// is used by DeleteChore (FR-R8) to check whether a Chore has an active
// (non-terminal) Run before deciding between a soft- and hard-delete —
// unlike Job, Chore carries no Status field of its own (see domain.Chore's
// doc comment: "the Chore itself remains Active until deleted"), so
// activeness must be derived from its Runs rather than read off a field.
//
// hiddenRoot is the storage root each Chore's HiddenDirectory is created
// under, at <hiddenRoot>/users/<ownerUserID>/chores/<choreID> — deliberately
// mirroring FileChoreRepository's own
// <basePath>/users/<ownerUserID>/chores/<choreID>.json record layout, since
// a Chore's WorkingDirectory is optional (unlike Project's
// OutputDirectory) and so cannot always host a nested hidden directory the
// way Projects' ".nuimanbot" convention does.
func NewService(chores domain.ChoreRepository, evaluator ScheduleEvaluator, runs domain.RunRepository, hiddenRoot string) *Service {
	return &Service{chores: chores, evaluator: evaluator, runs: runs, hiddenRoot: hiddenRoot, now: time.Now}
}

// CreateChore creates a new Chore (FR-031), validating schedule's cron
// expression and persisting Description as JOB-DESCRIPTION.md in a new
// HiddenDirectory (FR-031). userConfirmed distinguishes a user-set schedule
// (fires immediately, FR-033) from an agent-proposed one pending
// confirmation: when true, NextFireTime is computed now; when false,
// ScheduleConfirmed stays false and NextFireTime is left zero (irrelevant —
// domain.Chore.IsDue never fires an unconfirmed Chore, per Edge Case #6).
func (s *Service) CreateChore(ctx context.Context, ownerUserID, title, description, workingDirectory string, schedule domain.Schedule, userConfirmed bool) (*domain.Chore, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: title is required", domain.ErrInvalidInput)
	}
	if err := s.evaluator.Validate(schedule.CronExpression); err != nil {
		return nil, fmt.Errorf("%w: invalid cron expression: %v", domain.ErrInvalidInput, err)
	}

	now := s.now()
	id := uuid.NewString()
	hiddenDir := filepath.Join(s.hiddenRoot, "users", ownerUserID, "chores", id)
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hidden directory: %w", err)
	}
	descPath, err := fsguard.ResolveWithin(hiddenDir, descriptionFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve description file path: %w", err)
	}
	if err := os.WriteFile(descPath, []byte(description), 0644); err != nil {
		return nil, fmt.Errorf("failed to write description file: %w", err)
	}

	c := &domain.Chore{
		ID:                id,
		OwnerUserID:       ownerUserID,
		Title:             title,
		Description:       description,
		HiddenDirectory:   hiddenDir,
		WorkingDirectory:  workingDirectory,
		Schedule:          schedule,
		ScheduleConfirmed: userConfirmed,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if userConfirmed {
		next, err := s.evaluator.NextFireTime(schedule.CronExpression, now)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to compute next fire time: %v", domain.ErrInvalidInput, err)
		}
		c.NextFireTime = next
	}

	if err := s.chores.SaveChore(ctx, c); err != nil {
		return nil, fmt.Errorf("failed to save chore: %w", err)
	}
	return c, nil
}

// ListChores returns ownerUserID's Chores.
func (s *Service) ListChores(ctx context.Context, ownerUserID string) ([]*domain.Chore, error) {
	return s.chores.ListChores(ctx, ownerUserID)
}

// GetChore retrieves a Chore by ID, scoped to its owner. Cross-owner access
// resolves as domain.ErrNotFound (enforced by the repository layer; see
// domain.ChoreRepository's doc comment).
func (s *Service) GetChore(ctx context.Context, ownerUserID, choreID string) (*domain.Chore, error) {
	return s.chores.GetChore(ctx, ownerUserID, choreID)
}

// DeleteChore deletes a Chore by ID, scoped to its owner (FR-R8, spec.md
// Edge Case #3, mirroring jobs.Service.DeleteJob): if the Chore has a Run in
// a non-terminal state (Queued or Running), the record is soft-marked
// PendingDeletion instead of removed, so the in-flight/queued run is not
// killed mid-write; otherwise the record is deleted outright.
//
// Sweep integration deferred: a background sweep that hard-deletes a
// PendingDeletion Chore once its run reaches a terminal state is out of
// scope for this task — see Workstream A's FR-R9, which lands after this
// task specifically because it assumes Chores already soft-delete
// correctly. No new runs should be enqueued for a Chore with
// PendingDeletion set; enforcing that at schedule-fire time also belongs to
// that future sweep/scheduler integration, not this Service.
func (s *Service) DeleteChore(ctx context.Context, ownerUserID, choreID string) error {
	c, err := s.chores.GetChore(ctx, ownerUserID, choreID)
	if err != nil {
		return err
	}

	active, err := s.hasActiveRun(ctx, ownerUserID, choreID)
	if err != nil {
		return err
	}
	if active {
		c.PendingDeletion = true
		c.UpdatedAt = s.now()
		if err := s.chores.SaveChore(ctx, c); err != nil {
			return fmt.Errorf("failed to soft-delete chore: %w", err)
		}
		return nil
	}

	return s.chores.DeleteChore(ctx, ownerUserID, choreID)
}

// hasActiveRun reports whether choreID has any Run in a non-terminal state
// (Queued or Running). Chore carries no Status field of its own to consult
// directly (unlike Job), so DeleteChore derives activeness from the Run
// history instead.
func (s *Service) hasActiveRun(ctx context.Context, ownerUserID, choreID string) (bool, error) {
	sourceType := domain.SourceTypeChore
	runs, err := s.runs.ListRuns(ctx, ownerUserID, domain.RunFilter{SourceType: &sourceType, SourceID: &choreID})
	if err != nil {
		return false, fmt.Errorf("failed to check chore's active runs: %w", err)
	}
	for _, r := range runs {
		if !r.Status.IsTerminal() {
			return true, nil
		}
	}
	return false, nil
}

// ConfirmSchedule confirms an agent-proposed schedule (FR-033), computing
// and persisting its first NextFireTime so the Chore begins firing.
// Cross-owner access resolves as domain.ErrNotFound.
func (s *Service) ConfirmSchedule(ctx context.Context, ownerUserID, choreID string) error {
	c, err := s.chores.GetChore(ctx, ownerUserID, choreID)
	if err != nil {
		return err
	}

	next, err := s.evaluator.NextFireTime(c.Schedule.CronExpression, s.now())
	if err != nil {
		return fmt.Errorf("%w: failed to compute next fire time: %v", domain.ErrInvalidInput, err)
	}

	c.ScheduleConfirmed = true
	c.NextFireTime = next
	c.UpdatedAt = s.now()
	if err := s.chores.SaveChore(ctx, c); err != nil {
		return fmt.Errorf("failed to save chore: %w", err)
	}
	return nil
}
