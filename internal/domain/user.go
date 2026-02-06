package domain

import (
	"context"
	"time"
)

// Role defines the role of a user in the system.
type Role string

const (
	RoleGuest Role = "guest" // Limited access, unauthenticated users
	RoleUser  Role = "user"  // Standard access, registered users
	RoleAdmin Role = "admin" // Full access, administrators
)

// RoleLevel returns the numeric level of a role for comparison.
// Higher numbers = more permissions.
func (r Role) Level() int {
	switch r {
	case RoleGuest:
		return 0
	case RoleUser:
		return 1
	case RoleAdmin:
		return 2
	default:
		return -1 // Unknown role
	}
}

// HasPermission checks if this role has at least the permissions of the required role.
func (r Role) HasPermission(required Role) bool {
	return r.Level() >= required.Level()
}

// User represents a user of the NuimanBot system.
type User struct {
	ID            string
	Username      string
	Role          Role
	PlatformIDs   map[Platform]string // Telegram ID, Slack ID, etc.
	AllowedSkills []string            // Optional skill whitelist. Empty = all skills allowed for user's role
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserRepository defines the contract for user data persistence.
type UserRepository interface {
	SaveUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByPlatformID(ctx context.Context, platform Platform, platformUID string) (*User, error)
}
