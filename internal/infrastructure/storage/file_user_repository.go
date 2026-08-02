package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"nuimanbot/internal/domain"
)

// userRepoFile is the on-disk JSON shape for FileUserRepository.
type userRepoFile struct {
	Users []*domain.User `json:"users"`
}

// FileUserRepository implements usecase/user.ExtendedUserRepository
// (domain.UserRepository + ListAll/Delete) using JSON file storage,
// mirroring FileUserProfileRepository's load/save/atomic-write pattern.
//
// domain.User (RBAC identity: ID, Role, PlatformIDs) is distinct from
// domain.UserProfile (admin-managed profile data backing web/CLI admin
// features) — this repository backs the former, which usecase/user.Service
// depends on and no production repository previously implemented.
type FileUserRepository struct {
	filePath string
	writer   *AtomicFileWriter
	mu       sync.RWMutex
}

// NewFileUserRepository creates a new file-based domain.User repository.
func NewFileUserRepository(filePath string) *FileUserRepository {
	return &FileUserRepository{
		filePath: filePath,
		writer:   NewAtomicFileWriter(),
	}
}

func (r *FileUserRepository) load() (*userRepoFile, error) {
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return &userRepoFile{Users: []*domain.User{}}, nil
	}

	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}

	var f userRepoFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("failed to parse users file: %w", err)
	}
	return &f, nil
}

func (r *FileUserRepository) save(f *userRepoFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users file: %w", err)
	}
	if err := r.writer.Write(r.filePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}
	return nil
}

// SaveUser creates or updates a user (matched by ID).
func (r *FileUserRepository) SaveUser(ctx context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return err
	}

	idx := -1
	for i, existing := range f.Users {
		if existing.ID == user.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		f.Users[idx] = user
	} else {
		f.Users = append(f.Users, user)
	}

	return r.save(f)
}

// GetUserByID retrieves a user by ID. Returns domain.ErrUserNotFound if absent.
func (r *FileUserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, u := range f.Users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

// GetUserByPlatformID retrieves a user by (platform, platformUID). Returns
// domain.ErrUserNotFound if absent.
func (r *FileUserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, u := range f.Users {
		if u.PlatformIDs != nil && u.PlatformIDs[platform] == platformUID {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

// ListAll returns every stored user.
func (r *FileUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	f, err := r.load()
	if err != nil {
		return nil, err
	}
	return f.Users, nil
}

// Delete removes a user by ID. Returns domain.ErrUserNotFound if absent.
func (r *FileUserRepository) Delete(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.load()
	if err != nil {
		return err
	}

	newUsers := make([]*domain.User, 0, len(f.Users))
	found := false
	for _, u := range f.Users {
		if u.ID == userID {
			found = true
			continue
		}
		newUsers = append(newUsers, u)
	}
	if !found {
		return domain.ErrUserNotFound
	}

	f.Users = newUsers
	return r.save(f)
}
