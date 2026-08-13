package auth

import (
	"fmt"
	"time"
)

// Session represents a user session.
type Session struct {
	ID                  string
	Username            string
	Role                string
	CreatedAt           time.Time
	ExpiresAt           time.Time
	ForcePasswordChange bool // set when user must change default credentials
}

// CreateSession creates a new session for the user.
//
// Deliberately not implemented as CreateSessionWithFlags(username, role,
// false): that method spawns a background cleanup goroutine per call (see
// its doc comment), whereas CreateSession does not — TestSessionCleanup
// GoroutineCount depends on CreateSession alone not doing so, matching the
// pre-extraction behavior in internal/adapter/web/auth.go exactly (AD-1: no
// behavior change).
func (s *Service) CreateSession(username, role string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := generateRandomString(sessionIDLength)

	session := &Session{
		ID:        sessionID,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.sessionTimeout),
	}

	s.sessions[sessionID] = session

	return sessionID
}

// CreateSessionWithFlags creates a new session, optionally setting the
// ForcePasswordChange flag for accounts using default credentials.
func (s *Service) CreateSessionWithFlags(username, role string, forcePasswordChange bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := generateRandomString(sessionIDLength)

	session := &Session{
		ID:                  sessionID,
		Username:            username,
		Role:                role,
		CreatedAt:           time.Now(),
		ExpiresAt:           time.Now().Add(s.sessionTimeout),
		ForcePasswordChange: forcePasswordChange,
	}

	s.sessions[sessionID] = session

	go s.cleanupExpiredSessions()

	return sessionID
}

// ValidateSession checks if a session ID is valid.
func (s *Service) ValidateSession(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return false
	}

	return !time.Now().After(session.ExpiresAt)
}

// GetSession retrieves session information.
func (s *Service) GetSession(sessionID string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		return nil
	}

	return session
}

// DestroySession removes a session.
func (s *Service) DestroySession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
}

// ClearForcePasswordChange removes the ForcePasswordChange flag from the session.
func (s *Service) ClearForcePasswordChange(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.sessions[sessionID]; ok {
		session.ForcePasswordChange = false
	}
}

// RestoreSession re-hydrates a previously-issued session record (typically
// read from a CLI-persisted disk file, see architecture.md AD-2) into the
// in-memory session store, so a new process can resume without a fresh
// login prompt.
//
// It independently re-validates two things rather than trusting the
// caller's own pre-check:
//  1. ExpiresAt is still in the future.
//  2. Username still exists in the live user store.
//
// This is defense-in-depth against a caller-side bug (e.g. clock skew), not
// a defense against a malicious local user forging their own session file —
// that threat is explicitly out of scope per AD-2's decided threat model
// (the trust boundary is OS file permissions on the session file itself).
func (s *Service) RestoreSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("auth: restore session: session is nil")
	}
	if time.Now().After(session.ExpiresAt) {
		return fmt.Errorf("auth: restore session: session %q expired at %s", session.ID, session.ExpiresAt)
	}
	if _, exists := s.GetUser(session.Username); !exists {
		return fmt.Errorf("auth: restore session: user %q no longer exists", session.Username)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	restored := *session
	s.sessions[session.ID] = &restored
	return nil
}

// runCleanupLoop runs a single background goroutine that periodically removes
// expired sessions on a fixed interval. Using a timer-based loop (rather than
// spawning a goroutine on every CreateSession call) prevents goroutine
// accumulation under load: regardless of how many sessions are created, exactly
// one cleanup goroutine exists for the lifetime of the Service.
func (s *Service) runCleanupLoop() {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanupExpiredSessions()
	}
}

// cleanupExpiredSessions removes expired sessions.
func (s *Service) cleanupExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
}
