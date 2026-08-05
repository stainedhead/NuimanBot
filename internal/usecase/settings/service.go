// Package settings orchestrates the web admin's Settings environment
// (FR-001–FR-004): system-wide worker pool size, a read-only Skills
// surface, and system-wide default retention windows. Network access
// (FR-002/FR-005–FR-008) and Users management (FR-001) are handled
// elsewhere — see internal/adapter/web.SettingsService's doc comment for
// the full scope rationale.
package settings

import "fmt"

// PoolController is the subset of internal/infrastructure/scheduler.WorkerPool
// this package needs. Defined locally (rather than importing the scheduler
// package) so this usecase package depends only on an interface it owns,
// per AGENTS.md's Clean Architecture rule — the concrete *scheduler.WorkerPool
// already satisfies this interface structurally (SetConcurrency/Concurrency),
// no adapter type is needed at the call site.
type PoolController interface {
	// SetConcurrency updates the live worker pool concurrency.
	SetConcurrency(n int)
	// Concurrency returns the current concurrency.
	Concurrency() int
}

// SkillsLister is the subset of skill registry information this package
// needs — just the names, for FR-001's read-only Skills surface.
type SkillsLister interface {
	SkillNames() []string
}

// RetentionDefaults is the system-wide default retention configuration
// (FR-003), expressed in days (0 = Never), matching
// internal/config.RetentionDefaultsConfig's shape without importing the
// config package's YAML-tagged struct directly into the usecase layer.
type RetentionDefaults struct {
	ChatDays    int
	ProjectDays int
	HistoryDays int
}

// Service backs internal/adapter/web.SettingsService.
type Service struct {
	pool      PoolController
	skills    SkillsLister
	retention RetentionDefaults
}

// NewService creates a Settings Service.
func NewService(pool PoolController, skills SkillsLister, retention RetentionDefaults) *Service {
	return &Service{pool: pool, skills: skills, retention: retention}
}

// WorkerPoolSize returns the current live concurrency.
func (s *Service) WorkerPoolSize() int {
	return s.pool.Concurrency()
}

// SetWorkerPoolSize updates the live worker pool concurrency (FR-004).
// Rejects non-positive values rather than silently clamping — the caller
// (the web handler) should treat this as a validation failure to surface to
// the admin, not adjust it for them.
func (s *Service) SetWorkerPoolSize(n int) error {
	if n <= 0 {
		return fmt.Errorf("worker pool size must be positive, got %d", n)
	}
	s.pool.SetConcurrency(n)
	return nil
}

// SkillNames lists the currently registered skill names.
func (s *Service) SkillNames() []string {
	if s.skills == nil {
		return nil
	}
	return s.skills.SkillNames()
}

// RetentionDefaults returns the system-wide default retention windows in
// days (0 = Never) for Chat/Project/History.
func (s *Service) RetentionDefaults() (chatDays, projectDays, historyDays int) {
	return s.retention.ChatDays, s.retention.ProjectDays, s.retention.HistoryDays
}
