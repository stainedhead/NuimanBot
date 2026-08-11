package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// FileProjectRepository implements domain.ProjectRepository using
// per-owner, atomic-write file storage, modeled on FileConversationRepository.
//
// Layout: <basePath>/users/<ownerUserID>/projects/<projectID>.json
//
// Because every path is derived from ownerUserID, a lookup for a projectID
// owned by a different user simply misses on disk — cross-owner access
// naturally resolves to ErrNotFound without any extra ownership check,
// satisfying spec.md's IDOR requirement (Edge Case #10) at the data-access
// layer.
type FileProjectRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileProjectRepository creates a new file-based Project repository.
func NewFileProjectRepository(basePath string) *FileProjectRepository {
	return &FileProjectRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

func (r *FileProjectRepository) userDir(ownerUserID string) string {
	return filepath.Join(r.basePath, "users", ownerUserID, "projects")
}

// recordPath resolves projectID's on-disk record path, confined to
// ownerUserID's own projects directory via fsguard.ResolveWithinNoEscape
// (FR-R6: also guards against a symlink planted at projectID+".json"
// redirecting the read/write outside the owner's projects directory — a
// Project record is reachable by ID alone via the web adapter, so this is
// on a more directly attacker-reachable path than the Job/Chore/Run
// record-path siblings). See FileJobRepository.recordPath's doc comment
// for why this confinement is required (projectID derives from a URL path
// segment) and why every resolution failure maps uniformly to
// domain.ErrNotFound.
func (r *FileProjectRepository) recordPath(ownerUserID, projectID string) (string, error) {
	path, err := fsguard.ResolveWithinNoEscape(r.userDir(ownerUserID), projectID+".json")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return path, nil
}

// SaveProject creates or updates a Project.
func (r *FileProjectRepository) SaveProject(_ context.Context, p *domain.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}
	path, err := r.recordPath(p.OwnerUserID, p.ID)
	if err != nil {
		return err
	}
	if err := r.writer.Write(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write project record: %w", err)
	}
	return nil
}

// GetProject retrieves a Project by ID, scoped to its owner.
func (r *FileProjectRepository) GetProject(_ context.Context, ownerUserID, projectID string) (*domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	path, err := r.recordPath(ownerUserID, projectID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to read project record: %w", err)
	}

	var p domain.Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse project record: %w", err)
	}
	return &p, nil
}

// ListProjects returns all Projects owned by ownerUserID.
func (r *FileProjectRepository) ListProjects(_ context.Context, ownerUserID string) ([]*domain.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, err := os.ReadDir(r.userDir(ownerUserID))
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Project{}, nil
		}
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	projects := make([]*domain.Project, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.userDir(ownerUserID), entry.Name()))
		if err != nil {
			continue // Skip unreadable entries rather than failing the whole list.
		}
		var p domain.Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		projects = append(projects, &p)
	}
	return projects, nil
}

// DeleteProject removes a Project by ID, scoped to its owner.
func (r *FileProjectRepository) DeleteProject(_ context.Context, ownerUserID, projectID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path, err := r.recordPath(ownerUserID, projectID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to stat project record: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete project record: %w", err)
	}
	return nil
}

var _ domain.ProjectRepository = (*FileProjectRepository)(nil)
