package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// FileRunRepository implements domain.RunRepository using per-owner,
// atomic-write file storage. Layout mirrors the other new repositories:
// <basePath>/users/<ownerUserID>/runs/<runID>.json
//
// AppendLog writes to a companion file (<runID>.log) rather than the JSON
// metadata record itself, so a long-running Job/Chore's log can be
// appended to incrementally without re-marshaling/re-writing the entire
// run record on every chunk.
type FileRunRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileRunRepository creates a new file-based Run repository.
func NewFileRunRepository(basePath string) *FileRunRepository {
	return &FileRunRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

func (r *FileRunRepository) userDir(ownerUserID string) string {
	return filepath.Join(r.basePath, "users", ownerUserID, "runs")
}

// recordPath resolves runID's on-disk record path, confined to
// ownerUserID's own runs directory via fsguard.ResolveWithin. See
// FileJobRepository.recordPath's doc comment for why this confinement is
// required (runID derives from a URL path segment) and why every
// resolution failure maps uniformly to domain.ErrNotFound.
func (r *FileRunRepository) recordPath(ownerUserID, runID string) (string, error) {
	path, err := fsguard.ResolveWithin(r.userDir(ownerUserID), runID+".json")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return path, nil
}

// logPath resolves runID's on-disk log path, confined the same way as
// recordPath.
func (r *FileRunRepository) logPath(ownerUserID, runID string) (string, error) {
	path, err := fsguard.ResolveWithin(r.userDir(ownerUserID), runID+".log")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return path, nil
}

// SaveRun creates or updates a Run.
func (r *FileRunRepository) SaveRun(_ context.Context, run *domain.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeLocked(run)
}

func (r *FileRunRepository) writeLocked(run *domain.Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run: %w", err)
	}
	path, err := r.recordPath(run.OwnerUserID, run.ID)
	if err != nil {
		return err
	}
	if err := r.writer.Write(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write run record: %w", err)
	}
	return nil
}

func (r *FileRunRepository) readLocked(ownerUserID, runID string) (*domain.Run, error) {
	path, err := r.recordPath(ownerUserID, runID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to read run record: %w", err)
	}
	var run domain.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("failed to parse run record: %w", err)
	}
	return &run, nil
}

// GetRun retrieves a Run by ID, scoped to its owner.
func (r *FileRunRepository) GetRun(_ context.Context, ownerUserID, runID string) (*domain.Run, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readLocked(ownerUserID, runID)
}

func (r *FileRunRepository) listUserRunsLocked(ownerUserID string) ([]*domain.Run, error) {
	entries, err := os.ReadDir(r.userDir(ownerUserID))
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Run{}, nil
		}
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}

	runs := make([]*domain.Run, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.userDir(ownerUserID), entry.Name()))
		if err != nil {
			continue
		}
		var run domain.Run
		if err := json.Unmarshal(data, &run); err != nil {
			continue
		}
		runs = append(runs, &run)
	}
	return runs, nil
}

// runMatchesFilter reports whether run satisfies every non-nil dimension of filter.
func runMatchesFilter(run *domain.Run, filter domain.RunFilter) bool {
	if filter.SourceType != nil && run.SourceType != *filter.SourceType {
		return false
	}
	if filter.SourceID != nil && run.SourceID != *filter.SourceID {
		return false
	}
	if filter.Status != nil && run.Status != *filter.Status {
		return false
	}
	if filter.Since != nil && run.CreatedAt.Before(*filter.Since) {
		return false
	}
	if filter.Until != nil && run.CreatedAt.After(*filter.Until) {
		return false
	}
	return true
}

// ListRuns returns Runs owned by ownerUserID matching filter, most recent first.
func (r *FileRunRepository) ListRuns(_ context.Context, ownerUserID string, filter domain.RunFilter) ([]*domain.Run, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all, err := r.listUserRunsLocked(ownerUserID)
	if err != nil {
		return nil, err
	}

	matched := make([]*domain.Run, 0, len(all))
	for _, run := range all {
		if runMatchesFilter(run, filter) {
			matched = append(matched, run)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	return matched, nil
}

// AppendLog appends a chunk of log content to runID's durable log, scoped
// to its owner.
func (r *FileRunRepository) AppendLog(_ context.Context, ownerUserID, runID, chunk string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.readLocked(ownerUserID, runID); err != nil {
		return err
	}

	path, err := r.logPath(ownerUserID, runID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create runs directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open run log: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(chunk); err != nil {
		return fmt.Errorf("failed to append run log: %w", err)
	}
	return nil
}

// MarkNotified sets NotifiedAt on runID (FR-044's badge clear-on-view),
// scoped to its owner.
func (r *FileRunRepository) MarkNotified(_ context.Context, ownerUserID, runID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, err := r.readLocked(ownerUserID, runID)
	if err != nil {
		return err
	}
	now := time.Now()
	run.NotifiedAt = &now
	return r.writeLocked(run)
}

// CountUnnotified returns the number of terminal, unviewed runs for
// ownerUserID (the History notification badge count, FR-044).
func (r *FileRunRepository) CountUnnotified(_ context.Context, ownerUserID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all, err := r.listUserRunsLocked(ownerUserID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, run := range all {
		if run.IsUnviewed() {
			count++
		}
	}
	return count, nil
}

// DeleteRun removes a Run by ID (and its log file, if any), scoped to its owner.
func (r *FileRunRepository) DeleteRun(_ context.Context, ownerUserID, runID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path, err := r.recordPath(ownerUserID, runID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to stat run record: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete run record: %w", err)
	}
	if logPath, err := r.logPath(ownerUserID, runID); err == nil {
		_ = os.Remove(logPath) // Best-effort; log may not exist.
	}
	return nil
}

var _ domain.RunRepository = (*FileRunRepository)(nil)
