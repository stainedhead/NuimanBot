package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"os"
	"sync"
)

// UserProfilesFile represents the structure of users.json
type UserProfilesFile struct {
	Version     string                `json:"version"`
	LastUpdated string                `json:"lastUpdated"`
	Users       []*domain.UserProfile `json:"users"`
	Indexes     UserProfileIndexes    `json:"indexes"`
}

// UserProfileIndexes contains lookup indexes for fast retrieval
type UserProfileIndexes struct {
	ByUsername map[string]string          `json:"byUsername"` // username -> userID
	ByEmail    map[string]string          `json:"byEmail"`    // email -> userID
	ByPlatform UserProfilePlatformIndexes `json:"byPlatform"` // platform-specific indexes
}

// UserProfilePlatformIndexes contains platform-specific ID lookups
type UserProfilePlatformIndexes struct {
	Slack    map[string]string `json:"slack"`    // slackID -> userID
	Telegram map[string]string `json:"telegram"` // telegramID -> userID
	CLI      map[string]string `json:"cli"`      // cliUsername -> userID
}

// FileUserProfileRepository implements UserProfileRepository using JSON file storage
type FileUserProfileRepository struct {
	filePath   string
	writer     *AtomicFileWriter
	mu         sync.RWMutex
	encryptKey string // For future use with sensitive data
}

// NewFileUserProfileRepository creates a new file-based user profile repository
func NewFileUserProfileRepository(filePath, encryptKey string) *FileUserProfileRepository {
	return &FileUserProfileRepository{
		filePath:   filePath,
		writer:     NewAtomicFileWriter(),
		encryptKey: encryptKey,
	}
}

// load reads and parses the users.json file
func (r *FileUserProfileRepository) load() (*UserProfilesFile, error) {
	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		// Return empty file structure
		return &UserProfilesFile{
			Version: "1.0",
			Users:   []*domain.UserProfile{},
			Indexes: UserProfileIndexes{
				ByUsername: make(map[string]string),
				ByEmail:    make(map[string]string),
				ByPlatform: UserProfilePlatformIndexes{
					Slack:    make(map[string]string),
					Telegram: make(map[string]string),
					CLI:      make(map[string]string),
				},
			},
		}, nil
	}

	// Read file
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}

	// Parse JSON
	var file UserProfilesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse users file: %w", err)
	}

	// Initialize maps if nil
	if file.Indexes.ByUsername == nil {
		file.Indexes.ByUsername = make(map[string]string)
	}
	if file.Indexes.ByEmail == nil {
		file.Indexes.ByEmail = make(map[string]string)
	}
	if file.Indexes.ByPlatform.Slack == nil {
		file.Indexes.ByPlatform.Slack = make(map[string]string)
	}
	if file.Indexes.ByPlatform.Telegram == nil {
		file.Indexes.ByPlatform.Telegram = make(map[string]string)
	}
	if file.Indexes.ByPlatform.CLI == nil {
		file.Indexes.ByPlatform.CLI = make(map[string]string)
	}

	return &file, nil
}

// save writes the users file atomically
func (r *FileUserProfileRepository) save(file *UserProfilesFile) error {
	// Update timestamp
	file.LastUpdated = formatTimestamp()

	// Marshal to JSON
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users file: %w", err)
	}

	// Write atomically
	if err := r.writer.Write(r.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}

	return nil
}

// rebuildIndexes rebuilds all indexes from the users list
func (r *FileUserProfileRepository) rebuildIndexes(file *UserProfilesFile) {
	file.Indexes.ByUsername = make(map[string]string)
	file.Indexes.ByEmail = make(map[string]string)
	file.Indexes.ByPlatform.Slack = make(map[string]string)
	file.Indexes.ByPlatform.Telegram = make(map[string]string)
	file.Indexes.ByPlatform.CLI = make(map[string]string)

	for _, user := range file.Users {
		// Index by email
		if user.PrimaryEmail != "" {
			file.Indexes.ByEmail[user.PrimaryEmail] = user.UserID
		}

		// Index by moniker (username)
		if user.Moniker != "" {
			file.Indexes.ByUsername[user.Moniker] = user.UserID
		}

		// Index by platform IDs
		if user.PlatformIDs.Slack != "" {
			file.Indexes.ByPlatform.Slack[user.PlatformIDs.Slack] = user.UserID
		}
		if user.PlatformIDs.Telegram != "" {
			file.Indexes.ByPlatform.Telegram[user.PlatformIDs.Telegram] = user.UserID
		}
		if user.PlatformIDs.CLI != "" {
			file.Indexes.ByPlatform.CLI[user.PlatformIDs.CLI] = user.UserID
		}
	}
}

// SaveProfile creates or updates a user profile
func (r *FileUserProfileRepository) SaveProfile(ctx context.Context, profile *domain.UserProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate profile
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}

	// Load current file
	file, err := r.load()
	if err != nil {
		return err
	}

	// Find existing profile index
	existingIndex := -1
	for i, existing := range file.Users {
		if existing.UserID == profile.UserID {
			existingIndex = i
			break
		}
	}

	// Update or append
	if existingIndex >= 0 {
		file.Users[existingIndex] = profile
	} else {
		file.Users = append(file.Users, profile)
	}

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}

// GetProfileByUserID retrieves a profile by user ID
func (r *FileUserProfileRepository) GetProfileByUserID(ctx context.Context, userID string) (*domain.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.load()
	if err != nil {
		return nil, err
	}

	for _, user := range file.Users {
		if user.UserID == userID {
			return user, nil
		}
	}

	return nil, errors.New("profile not found")
}

// GetProfileByEmail retrieves a profile by email address
func (r *FileUserProfileRepository) GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Use index for fast lookup
	userID, found := file.Indexes.ByEmail[email]
	if !found {
		return nil, errors.New("profile not found")
	}

	// Find user by ID
	for _, user := range file.Users {
		if user.UserID == userID {
			return user, nil
		}
	}

	return nil, errors.New("profile not found (index stale)")
}

// GetProfileByPlatformID retrieves a profile by platform-specific ID
func (r *FileUserProfileRepository) GetProfileByPlatformID(ctx context.Context, platform domain.Platform, platformID string) (*domain.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Use platform index for fast lookup
	var userID string
	var found bool

	switch platform {
	case domain.PlatformSlack:
		userID, found = file.Indexes.ByPlatform.Slack[platformID]
	case domain.PlatformTelegram:
		userID, found = file.Indexes.ByPlatform.Telegram[platformID]
	case domain.PlatformCLI:
		userID, found = file.Indexes.ByPlatform.CLI[platformID]
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	if !found {
		return nil, errors.New("profile not found")
	}

	// Find user by ID
	for _, user := range file.Users {
		if user.UserID == userID {
			return user, nil
		}
	}

	return nil, errors.New("profile not found (index stale)")
}

// ListProfiles returns all profiles (with pagination support)
func (r *FileUserProfileRepository) ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	file, err := r.load()
	if err != nil {
		return nil, err
	}

	// Apply pagination
	total := len(file.Users)
	if offset >= total {
		return []*domain.UserProfile{}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return file.Users[offset:end], nil
}

// DeleteProfile removes a profile by user ID
func (r *FileUserProfileRepository) DeleteProfile(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := r.load()
	if err != nil {
		return err
	}

	// Find and remove profile
	newUsers := make([]*domain.UserProfile, 0, len(file.Users))
	found := false
	for _, user := range file.Users {
		if user.UserID == userID {
			found = true
			continue
		}
		newUsers = append(newUsers, user)
	}

	if !found {
		return errors.New("profile not found")
	}

	file.Users = newUsers

	// Rebuild indexes
	r.rebuildIndexes(file)

	// Save file
	return r.save(file)
}

// formatTimestamp returns current timestamp in ISO 8601 format
func formatTimestamp() string {
	return fmt.Sprintf("%s", "2026-02-08T12:00:00Z") // Simplified for now
}
