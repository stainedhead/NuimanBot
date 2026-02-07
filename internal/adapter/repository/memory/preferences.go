package memory

import (
	"context"
	"sync"

	"nuimanbot/internal/domain"
)

// PreferencesRepository is an in-memory implementation of domain.PreferencesRepository.
type PreferencesRepository struct {
	prefs map[string]domain.UserPreferences
	mu    sync.RWMutex
}

// NewPreferencesRepository creates a new in-memory preferences repository.
func NewPreferencesRepository() *PreferencesRepository {
	return &PreferencesRepository{
		prefs: make(map[string]domain.UserPreferences),
	}
}

// Get retrieves user preferences by user ID.
func (r *PreferencesRepository) Get(ctx context.Context, userID string) (domain.UserPreferences, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	prefs, exists := r.prefs[userID]
	if !exists {
		return domain.UserPreferences{}, domain.ErrNotFound
	}

	return prefs, nil
}

// Save stores user preferences.
func (r *PreferencesRepository) Save(ctx context.Context, userID string, prefs domain.UserPreferences) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.prefs[userID] = prefs
	return nil
}

// Delete removes user preferences.
func (r *PreferencesRepository) Delete(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.prefs, userID)
	return nil
}
