package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nuimanbot/internal/domain"
)

// FileJobRepository implements domain.JobRepository using per-owner,
// atomic-write file storage. Layout mirrors FileProjectRepository:
// <basePath>/users/<ownerUserID>/jobs/<jobID>.json
type FileJobRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileJobRepository creates a new file-based Job repository.
func NewFileJobRepository(basePath string) *FileJobRepository {
	return &FileJobRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

func (r *FileJobRepository) userDir(ownerUserID string) string {
	return filepath.Join(r.basePath, "users", ownerUserID, "jobs")
}

func (r *FileJobRepository) recordPath(ownerUserID, jobID string) string {
	return filepath.Join(r.userDir(ownerUserID), jobID+".json")
}

// SaveJob creates or updates a Job.
func (r *FileJobRepository) SaveJob(_ context.Context, j *domain.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeLocked(j)
}

// writeLocked marshals and writes j; callers must hold r.mu.
func (r *FileJobRepository) writeLocked(j *domain.Job) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}
	if err := r.writer.Write(r.recordPath(j.OwnerUserID, j.ID), data, 0644); err != nil {
		return fmt.Errorf("failed to write job record: %w", err)
	}
	return nil
}

// GetJob retrieves a Job by ID, scoped to its owner.
func (r *FileJobRepository) GetJob(_ context.Context, ownerUserID, jobID string) (*domain.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readLocked(ownerUserID, jobID)
}

// readLocked reads and unmarshals a Job; callers must hold r.mu (read or write).
func (r *FileJobRepository) readLocked(ownerUserID, jobID string) (*domain.Job, error) {
	data, err := os.ReadFile(r.recordPath(ownerUserID, jobID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to read job record: %w", err)
	}
	var j domain.Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("failed to parse job record: %w", err)
	}
	return &j, nil
}

// ListJobs returns all Jobs owned by ownerUserID.
func (r *FileJobRepository) ListJobs(_ context.Context, ownerUserID string) ([]*domain.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, err := os.ReadDir(r.userDir(ownerUserID))
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Job{}, nil
		}
		return nil, fmt.Errorf("failed to read jobs directory: %w", err)
	}

	jobs := make([]*domain.Job, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.userDir(ownerUserID), entry.Name()))
		if err != nil {
			continue
		}
		var j domain.Job
		if err := json.Unmarshal(data, &j); err != nil {
			continue
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}

// DeleteJob removes a Job by ID, scoped to its owner.
func (r *FileJobRepository) DeleteJob(_ context.Context, ownerUserID, jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := r.recordPath(ownerUserID, jobID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to stat job record: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete job record: %w", err)
	}
	return nil
}

// UpdateStatus updates just the Status (and UpdatedAt) of a Job, scoped to
// its owner.
func (r *FileJobRepository) UpdateStatus(_ context.Context, ownerUserID, jobID string, status domain.JobStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	j, err := r.readLocked(ownerUserID, jobID)
	if err != nil {
		return err
	}
	j.Status = status
	j.UpdatedAt = time.Now()
	return r.writeLocked(j)
}

var _ domain.JobRepository = (*FileJobRepository)(nil)
