package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// UserType defines the type/tier of user account
type UserType string

const (
	UserTypeIndividual UserType = "individual" // Individual user
	UserTypeEnterprise UserType = "enterprise" // Enterprise/organization user
	UserTypeDeveloper  UserType = "developer"  // Developer account
	UserTypeAdmin      UserType = "admin"      // System administrator
)

// PlatformIdentifiers stores user IDs for different messaging platforms.
// Used for routing messages from platforms to correct user profile.
type PlatformIdentifiers struct {
	CLI      string `json:"cli"`      // CLI username
	Slack    string `json:"slack"`    // Slack user ID (e.g., "U01ABC123")
	Telegram string `json:"telegram"` // Telegram user ID (numeric string, e.g., "123456789")
}

// UserProfile represents comprehensive user identity and preferences
// beyond authentication. Contains personal information, platform integration,
// agent customization, and organizational context.
type UserProfile struct {
	// Core Identity
	UserID    string `json:"userID"`    // References User.ID (primary key)
	Moniker   string `json:"moniker"`   // Display name or handle
	FirstName string `json:"firstName"` // Given name
	LastName  string `json:"lastName"`  // Family name
	NickName  string `json:"nickName"`  // Preferred informal name

	// Contact Information
	PrimaryEmail string `json:"primaryEmail"` // Primary contact email
	BackupEmail  string `json:"backupEmail"`  // Secondary contact email
	MobilePhone  string `json:"mobilePhone"`  // Mobile phone number (E.164 format)

	// Localization
	PrimaryLanguage   string `json:"primaryLanguage"`   // ISO 639-1 code (e.g., "en", "es")
	SecondaryLanguage string `json:"secondaryLanguage"` // ISO 639-1 code for fallback
	Timezone          string `json:"timezone"`          // IANA timezone (e.g., "America/New_York")
	PrimaryLocation   string `json:"primaryLocation"`   // Geographic location or timezone identifier

	// Organizational Context
	JobRole  string   `json:"jobRole"`  // User's organizational role
	UserType UserType `json:"userType"` // Individual, Enterprise, Developer, Admin

	// Multi-Platform Integration
	PlatformIDs PlatformIdentifiers `json:"platformIDs"` // Slack, Telegram, CLI identifiers

	// Personalization
	ProfileInfo string `json:"profileInfo"` // Freeform biographical info (max 2000 chars)

	// Metadata
	Enabled       bool      `json:"enabled"`       // Account enabled/disabled
	DataDirectory string    `json:"dataDirectory"` // Path to user's data directory
	CreatedAt     time.Time `json:"createdAt"`     // Profile creation time
	UpdatedAt     time.Time `json:"updatedAt"`     // Last modification time
	LastVerified  time.Time `json:"lastVerified"`  // Last time user verified their info
}

var (
	// emailRegex is a simple email validation regex
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// Validate checks if profile is valid according to business rules
func (up *UserProfile) Validate() error {
	// UserID is required
	if up.UserID == "" {
		return errors.New("userID is required")
	}

	// UserID max length
	if len(up.UserID) > 64 {
		return errors.New("userID must be <= 64 characters")
	}

	// Moniker max length
	if len(up.Moniker) > 50 {
		return errors.New("moniker must be <= 50 characters")
	}

	// FirstName max length
	if len(up.FirstName) > 100 {
		return errors.New("firstName must be <= 100 characters")
	}

	// LastName max length
	if len(up.LastName) > 100 {
		return errors.New("lastName must be <= 100 characters")
	}

	// NickName max length
	if len(up.NickName) > 50 {
		return errors.New("nickName must be <= 50 characters")
	}

	// PrimaryEmail is required and must be valid
	if up.PrimaryEmail == "" {
		return errors.New("primaryEmail is required")
	}
	if !emailRegex.MatchString(up.PrimaryEmail) {
		return errors.New("primaryEmail must be valid email format")
	}
	if len(up.PrimaryEmail) > 254 {
		return errors.New("primaryEmail must be <= 254 characters")
	}

	// BackupEmail must be valid if set
	if up.BackupEmail != "" {
		if !emailRegex.MatchString(up.BackupEmail) {
			return errors.New("backupEmail must be valid email format")
		}
		if len(up.BackupEmail) > 254 {
			return errors.New("backupEmail must be <= 254 characters")
		}
		// BackupEmail must be different from PrimaryEmail
		if up.BackupEmail == up.PrimaryEmail {
			return errors.New("backupEmail must be different from primaryEmail")
		}
	}

	// MobilePhone validation (E.164 format) if set
	if up.MobilePhone != "" {
		if !strings.HasPrefix(up.MobilePhone, "+") {
			return errors.New("mobilePhone must be in E.164 format (start with +)")
		}
		if len(up.MobilePhone) < 8 || len(up.MobilePhone) > 16 {
			return errors.New("mobilePhone must be 8-16 characters in E.164 format")
		}
	}

	// PrimaryLocation max length
	if len(up.PrimaryLocation) > 100 {
		return errors.New("primaryLocation must be <= 100 characters")
	}

	// JobRole max length
	if len(up.JobRole) > 100 {
		return errors.New("jobRole must be <= 100 characters")
	}

	// ProfileInfo max length
	if len(up.ProfileInfo) > 2000 {
		return errors.New("profileInfo must be <= 2000 characters")
	}

	// DataDirectory is required
	if up.DataDirectory == "" {
		return errors.New("dataDirectory is required")
	}

	return nil
}

// GetDisplayName returns the best available display name
// Priority: NickName > Moniker > FirstName > UserID
func (up *UserProfile) GetDisplayName() string {
	if up.NickName != "" {
		return up.NickName
	}
	if up.Moniker != "" {
		return up.Moniker
	}
	if up.FirstName != "" {
		return up.FirstName
	}
	return up.UserID
}

// GetFullName returns "FirstName LastName", or empty string if both unset
func (up *UserProfile) GetFullName() string {
	parts := []string{}
	if up.FirstName != "" {
		parts = append(parts, up.FirstName)
	}
	if up.LastName != "" {
		parts = append(parts, up.LastName)
	}
	return strings.Join(parts, " ")
}

// GetPreferredLanguage returns PrimaryLanguage or "en" if not set
func (up *UserProfile) GetPreferredLanguage() string {
	if up.PrimaryLanguage == "" {
		return "en"
	}
	return up.PrimaryLanguage
}

// GetTimezone returns Timezone or "UTC" if not set
func (up *UserProfile) GetTimezone() string {
	if up.Timezone == "" {
		return "UTC"
	}
	return up.Timezone
}

// UserProfileRepository defines the contract for user profile data persistence.
type UserProfileRepository interface {
	// SaveProfile creates or updates a user profile
	SaveProfile(ctx context.Context, profile *UserProfile) error

	// GetProfileByUserID retrieves a profile by user ID
	GetProfileByUserID(ctx context.Context, userID string) (*UserProfile, error)

	// GetProfileByEmail retrieves a profile by email address
	GetProfileByEmail(ctx context.Context, email string) (*UserProfile, error)

	// GetProfileByPlatformID retrieves a profile by platform-specific ID
	GetProfileByPlatformID(ctx context.Context, platform Platform, platformID string) (*UserProfile, error)

	// ListProfiles returns all profiles (with pagination support)
	ListProfiles(ctx context.Context, offset, limit int) ([]*UserProfile, error)

	// DeleteProfile removes a profile by user ID
	DeleteProfile(ctx context.Context, userID string) error
}

// NewUserProfile creates a new UserProfile with default values and timestamp initialization
func NewUserProfile(userID, email string, userType UserType) *UserProfile {
	now := time.Now()
	return &UserProfile{
		UserID:          userID,
		PrimaryEmail:    email,
		UserType:        userType,
		PrimaryLanguage: "en",
		Timezone:        "UTC",
		Enabled:         true,
		DataDirectory:   fmt.Sprintf("users/%s", userID),
		CreatedAt:       now,
		UpdatedAt:       now,
		LastVerified:    now,
		PlatformIDs:     PlatformIdentifiers{},
	}
}
