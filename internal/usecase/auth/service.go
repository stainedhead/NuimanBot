// Package auth provides credential verification and session-lifecycle
// management shared by internal/adapter/web (via a thin HTTP-transport
// wrapper) and internal/adapter/gateway/cli (directly). It has no knowledge
// of HTTP or terminal I/O — see specs/260811-cli-parity-for-nuimanbot-features
// /architecture.md AD-1 for the extraction rationale.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTimeout         = 24 * time.Hour
	sessionCleanupInterval = 5 * time.Minute
	sessionIDLength        = 32
	bcryptCost             = 12
)

// Service handles authentication and session management. Both the web
// admin UI and the CLI gateway construct or share a *Service so they
// authenticate against the same account/session state.
type Service struct {
	users          map[string]*AuthUser
	sessions       map[string]*Session
	mu             sync.RWMutex
	sessionTimeout time.Duration
}

// AuthUser represents an authenticated user account.
type AuthUser struct {
	Username     string
	PasswordHash string
	Role         string
}

// NewService creates a new authentication service and starts its background
// session-cleanup loop.
func NewService() *Service {
	svc := &Service{
		users:          make(map[string]*AuthUser),
		sessions:       make(map[string]*Session),
		sessionTimeout: sessionTimeout,
	}
	go svc.runCleanupLoop()
	return svc
}

// AddUser adds a user to the authentication service (for testing and setup).
func (s *Service) AddUser(username, password, role string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: add user: hash password: %w", err)
	}

	s.users[username] = &AuthUser{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         role,
	}

	return nil
}

// ValidateCredentials checks if username and password are valid.
func (s *Service) ValidateCredentials(username, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// GetUser returns a value copy of the account for username, if it exists.
// A value (not pointer) is returned so callers cannot mutate the service's
// internal state through the result.
func (s *Service) GetUser(username string) (AuthUser, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[username]
	if !exists {
		return AuthUser{}, false
	}
	return *user, true
}

// UpdatePassword updates the stored password hash for a user.
func (s *Service) UpdatePassword(username, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[username]
	if !exists {
		return fmt.Errorf("auth: update password: user %q not found", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: update password: hash: %w", err)
	}

	user.PasswordHash = string(hash)
	return nil
}

const (
	// defaultAdminUsername is the well-known default administrator account name.
	defaultAdminUsername = "admin"

	// defaultAdminPasswordHash is the bcrypt hash of the literal string "admin".
	// Cost 10 keeps the hash stable for detection purposes only (not for authentication).
	// Authentication always uses the stored hash from AddUser.
	defaultAdminPasswordHash = "$2a$10$Y6HD25IiXnjpqGnkUrK02uZBvmdpzv6vB3eGFCcEeIn1jSZlsrd2e" // #nosec G101 -- a bcrypt hash used only to detect the well-known default password, not a real credential
)

// IsDefaultCredentials reports whether the supplied plaintext password
// matches the default "admin" password for the "admin" user.
// Detection uses bcrypt.CompareHashAndPassword for constant-time comparison,
// preventing timing-based enumeration of whether a password matches "admin".
func (s *Service) IsDefaultCredentials(username, password string) bool {
	if username != defaultAdminUsername {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(defaultAdminPasswordHash), []byte(password)) == nil
}

// generateRandomString generates a random base64 encoded string.
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.URLEncoding.EncodeToString(bytes)
}
