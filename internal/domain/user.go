package domain

import (
	"context"
	"time"
)

// Role defines the role of a user in the system.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents a user of the NuimanBot system.
type User struct {
	ID            string
	Username      string
	Role          Role
	PlatformIDs   map[Platform]string // Telegram ID, Slack ID, etc.
	AllowedSkills []string            // Empty = all (admin only)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserRepository defines the contract for user data persistence.
type UserRepository interface {
	SaveUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByPlatformID(ctx context.Context, platform Platform, platformUID string) (*User, error)
}
