package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"nuimanbot/internal/domain"
	"os"
	"path/filepath"
	"time"
)

const (
	// Directory names for storage organization
	usersDir  = "users"
	systemDir = "system"

	// File names
	usersFile = "users.json"

	// Default admin credentials
	defaultAdminID    = "admin"
	defaultAdminEmail = "admin@localhost"
	defaultAdminName  = "Administrator"
)

// Initialize creates the required directory structure and default data
// for the file-based storage system. It is idempotent and can be called
// multiple times safely.
func Initialize(baseDir string) error {
	if baseDir == "" {
		return errors.New("base directory cannot be empty")
	}

	slog.Info("Initializing storage",
		"base_dir", baseDir,
	)

	// 1. Create directory structure
	if err := EnsureDirectories(baseDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// 2. Create default admin user if none exists
	if err := createDefaultAdminIfNeeded(baseDir); err != nil {
		return fmt.Errorf("failed to create default admin user: %w", err)
	}

	slog.Info("Storage initialization complete",
		"base_dir", baseDir,
	)

	return nil
}

// EnsureDirectories creates the required directory structure for file-based storage
func EnsureDirectories(baseDir string) error {
	requiredDirs := []string{usersDir, systemDir}

	for _, dir := range requiredDirs {
		path := filepath.Join(baseDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
		slog.Debug("Directory ensured", "path", path)
	}

	return nil
}

// createDefaultAdminIfNeeded creates a default admin user if users.json doesn't exist
func createDefaultAdminIfNeeded(baseDir string) error {
	usersFilePath := filepath.Join(baseDir, usersFile)

	// Check if users.json already exists
	if fileExists(usersFilePath) {
		slog.Info("users.json already exists, skipping default admin creation")
		return nil
	}

	slog.Info("Creating default admin user")

	// Create default admin profile and users file
	adminProfile := createDefaultAdminProfile()
	usersData := createUsersFileWithAdmin(adminProfile)

	// Write to disk
	if err := writeUsersFile(usersFilePath, usersData); err != nil {
		return err
	}

	slog.Info("Default admin user created",
		"user_id", defaultAdminID,
		"email", defaultAdminEmail,
	)

	return nil
}

// createDefaultAdminProfile creates a default admin user profile
func createDefaultAdminProfile() *domain.UserProfile {
	now := time.Now().UTC()
	return &domain.UserProfile{
		UserID:        defaultAdminID,
		Moniker:       defaultAdminName,
		FirstName:     "Admin",
		PrimaryEmail:  defaultAdminEmail,
		Role:          domain.RoleAdmin,
		UserType:      domain.UserTypeAdmin,
		Enabled:       true,
		DataDirectory: filepath.Join(usersDir, defaultAdminID),
		CreatedAt:     now,
		UpdatedAt:     now,
		LastVerified:  now,
		PlatformIDs:   domain.PlatformIdentifiers{},
	}
}

// createUsersFileWithAdmin creates a UserProfilesFile with the admin user
func createUsersFileWithAdmin(adminProfile *domain.UserProfile) *UserProfilesFile {
	return &UserProfilesFile{
		Version:     "1.0",
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
		Users:       []*domain.UserProfile{adminProfile},
		Indexes: UserProfileIndexes{
			ByUsername: map[string]string{
				defaultAdminID: defaultAdminID,
			},
			ByEmail: map[string]string{
				defaultAdminEmail: defaultAdminID,
			},
			ByAPIKey: make(map[string]string),
			ByPlatform: UserProfilePlatformIndexes{
				Slack:    make(map[string]string),
				Telegram: make(map[string]string),
				CLI:      make(map[string]string),
			},
		},
	}
}

// writeUsersFile marshals and writes the users file to disk
func writeUsersFile(filePath string, usersData *UserProfilesFile) error {
	data, err := json.MarshalIndent(usersData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users data: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write users.json: %w", err)
	}

	return nil
}

// fileExists checks if a file exists and is not a directory
func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}
