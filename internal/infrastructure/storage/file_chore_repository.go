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
	"nuimanbot/internal/infrastructure/fsguard"
)

// FileChoreRepository implements domain.ChoreRepository using per-owner,
// atomic-write file storage. Layout mirrors FileJobRepository:
// <basePath>/users/<ownerUserID>/chores/<choreID>.json
type FileChoreRepository struct {
	basePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileChoreRepository creates a new file-based Chore repository.
func NewFileChoreRepository(basePath string) *FileChoreRepository {
	return &FileChoreRepository{
		basePath: basePath,
		writer:   NewAtomicFileWriter(),
	}
}

func (r *FileChoreRepository) usersRootDir() string {
	return filepath.Join(r.basePath, "users")
}

func (r *FileChoreRepository) userDir(ownerUserID string) string {
	return filepath.Join(r.usersRootDir(), ownerUserID, "chores")
}

// recordPath resolves choreID's on-disk record path, confined to
// ownerUserID's own chores directory via fsguard.ResolveWithin. See
// FileJobRepository.recordPath's doc comment for why this confinement is
// required (choreID derives from a URL path segment) and why every
// resolution failure maps uniformly to domain.ErrNotFound.
func (r *FileChoreRepository) recordPath(ownerUserID, choreID string) (string, error) {
	path, err := fsguard.ResolveWithin(r.userDir(ownerUserID), choreID+".json")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return path, nil
}

// SaveChore creates or updates a Chore.
func (r *FileChoreRepository) SaveChore(_ context.Context, c *domain.Chore) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writeLocked(c)
}

func (r *FileChoreRepository) writeLocked(c *domain.Chore) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal chore: %w", err)
	}
	path, err := r.recordPath(c.OwnerUserID, c.ID)
	if err != nil {
		return err
	}
	if err := r.writer.Write(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write chore record: %w", err)
	}
	return nil
}

func (r *FileChoreRepository) readLocked(ownerUserID, choreID string) (*domain.Chore, error) {
	path, err := r.recordPath(ownerUserID, choreID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to read chore record: %w", err)
	}
	var c domain.Chore
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse chore record: %w", err)
	}
	return &c, nil
}

// GetChore retrieves a Chore by ID, scoped to its owner.
func (r *FileChoreRepository) GetChore(_ context.Context, ownerUserID, choreID string) (*domain.Chore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.readLocked(ownerUserID, choreID)
}

// ListChores returns all Chores owned by ownerUserID.
func (r *FileChoreRepository) ListChores(_ context.Context, ownerUserID string) ([]*domain.Chore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listUserChoresLocked(ownerUserID)
}

func (r *FileChoreRepository) listUserChoresLocked(ownerUserID string) ([]*domain.Chore, error) {
	entries, err := os.ReadDir(r.userDir(ownerUserID))
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Chore{}, nil
		}
		return nil, fmt.Errorf("failed to read chores directory: %w", err)
	}

	chores := make([]*domain.Chore, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.userDir(ownerUserID), entry.Name()))
		if err != nil {
			continue
		}
		var c domain.Chore
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		chores = append(chores, &c)
	}
	return chores, nil
}

// DeleteChore removes a Chore by ID, scoped to its owner.
func (r *FileChoreRepository) DeleteChore(_ context.Context, ownerUserID, choreID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path, err := r.recordPath(ownerUserID, choreID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to stat chore record: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete chore record: %w", err)
	}
	return nil
}

// UpdateNextFireTime updates just the NextFireTime (and UpdatedAt) of a
// Chore, scoped to its owner.
func (r *FileChoreRepository) UpdateNextFireTime(_ context.Context, ownerUserID, choreID string, next time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, err := r.readLocked(ownerUserID, choreID)
	if err != nil {
		return err
	}
	c.NextFireTime = next
	c.UpdatedAt = time.Now()
	return r.writeLocked(c)
}

// ListAllDue returns every confirmed, non-pending-deletion Chore across all
// users whose NextFireTime has arrived as of now. See the interface doc
// comment (domain.ChoreRepository) for why this is the one intentionally
// cross-user query on this repository.
func (r *FileChoreRepository) ListAllDue(_ context.Context, now time.Time) ([]*domain.Chore, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userEntries, err := os.ReadDir(r.usersRootDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []*domain.Chore{}, nil
		}
		return nil, fmt.Errorf("failed to read users directory: %w", err)
	}

	due := make([]*domain.Chore, 0)
	for _, userEntry := range userEntries {
		if !userEntry.IsDir() {
			continue
		}
		chores, err := r.listUserChoresLocked(userEntry.Name())
		if err != nil {
			continue // Skip a user whose chores directory can't be read.
		}
		for _, c := range chores {
			if c.IsDue(now) {
				due = append(due, c)
			}
		}
	}
	return due, nil
}

var _ domain.ChoreRepository = (*FileChoreRepository)(nil)
