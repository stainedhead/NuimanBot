package auth

import (
	"runtime"
	"testing"
	"time"
)

// TestSessionExpiry tests that sessions expire after timeout.
//
// Relocated verbatim from internal/adapter/web/auth_test.go per
// architecture.md AD-1's documented two-function exception: this test writes
// the unexported sessionTimeout field directly, which a struct-embedding
// shim cannot intercept.
func TestSessionExpiry(t *testing.T) {
	svc := NewService()
	svc.sessionTimeout = 100 * time.Millisecond // Short timeout for testing

	// Create session
	sessionID := svc.CreateSession("admin", "admin")

	// Session should be valid initially
	if !svc.ValidateSession(sessionID) {
		t.Error("session should be valid initially")
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Session should be expired
	if svc.ValidateSession(sessionID) {
		t.Error("session should be expired")
	}
}

// TestCleanupExpiredSessions verifies the cleanup removes expired sessions.
//
// Relocated verbatim from internal/adapter/web/auth_coverage_test.go per
// architecture.md AD-1's documented two-function exception: this test reads
// the unexported sessions map directly.
func TestCleanupExpiredSessions(t *testing.T) {
	svc := NewService()
	svc.sessionTimeout = 0 // immediately expired

	sessionID := svc.CreateSessionWithFlags("user", "admin", false)

	// Force expire by setting ExpiresAt in the past.
	svc.mu.Lock()
	if s, ok := svc.sessions[sessionID]; ok {
		s.ExpiresAt = s.CreatedAt.Add(-1)
	}
	svc.mu.Unlock()

	svc.cleanupExpiredSessions()

	svc.mu.RLock()
	_, still := svc.sessions[sessionID]
	svc.mu.RUnlock()

	if still {
		t.Error("expected expired session to be cleaned up")
	}
}

// TestSessionCleanupGoroutineCount verifies the cleanup loop is a single
// background goroutine, not one per CreateSession call.
func TestSessionCleanupGoroutineCount(t *testing.T) {
	svc := NewService()

	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		svc.CreateSession("user", "admin")
	}

	after := runtime.NumGoroutine()
	delta := after - baseline
	if delta > 3 {
		t.Errorf("goroutine count increased by %d after creating 10 sessions (baseline=%d, after=%d); expected delta <= 3", delta, baseline, after)
	}
}

// TestCreateSessionAndValidate verifies the basic session lifecycle.
func TestCreateSessionAndValidate(t *testing.T) {
	svc := NewService()
	id := svc.CreateSession("dave", "user")
	if !svc.ValidateSession(id) {
		t.Error("expected freshly created session to validate")
	}
	sess := svc.GetSession(id)
	if sess == nil || sess.Username != "dave" || sess.Role != "user" {
		t.Errorf("unexpected session: %+v", sess)
	}
	svc.DestroySession(id)
	if svc.ValidateSession(id) {
		t.Error("expected destroyed session to be invalid")
	}
	if svc.GetSession(id) != nil {
		t.Error("expected GetSession to return nil after DestroySession")
	}
}

// TestCreateSessionWithFlags verifies ForcePasswordChange is set correctly.
func TestCreateSessionWithFlags(t *testing.T) {
	svc := NewService()

	for _, forceChange := range []bool{true, false} {
		id := svc.CreateSessionWithFlags("user", "admin", forceChange)
		sess := svc.GetSession(id)
		if sess == nil {
			t.Fatal("expected session, got nil")
		}
		if sess.ForcePasswordChange != forceChange {
			t.Errorf("expected ForcePasswordChange=%v, got %v", forceChange, sess.ForcePasswordChange)
		}
	}
}

// TestClearForcePasswordChange verifies the flag is cleared, and clearing an
// unknown session does not panic.
func TestClearForcePasswordChange(t *testing.T) {
	svc := NewService()
	id := svc.CreateSessionWithFlags("user", "admin", true)

	s := svc.GetSession(id)
	if s == nil || !s.ForcePasswordChange {
		t.Fatal("expected ForcePasswordChange=true initially")
	}

	svc.ClearForcePasswordChange(id)

	s2 := svc.GetSession(id)
	if s2 == nil || s2.ForcePasswordChange {
		t.Error("expected ForcePasswordChange=false after ClearForcePasswordChange")
	}

	// Should not panic for an unknown session ID.
	svc.ClearForcePasswordChange("nonexistent")
}

// TestRestoreSession_Success verifies a valid, non-expired session for an
// existing user is re-hydrated so subsequent ValidateSession/GetSession
// calls succeed (P1.2 acceptance criterion).
func TestRestoreSession_Success(t *testing.T) {
	svc := NewService()
	if err := svc.AddUser("erin", "pw", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	persisted := &Session{
		ID:        "restored-session-id",
		Username:  "erin",
		Role:      "user",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := svc.RestoreSession(persisted); err != nil {
		t.Fatalf("RestoreSession: %v", err)
	}

	if !svc.ValidateSession(persisted.ID) {
		t.Error("expected restored session to validate")
	}
	got := svc.GetSession(persisted.ID)
	if got == nil || got.Username != "erin" {
		t.Errorf("unexpected restored session: %+v", got)
	}
}

// TestRestoreSession_RejectsExpired verifies RestoreSession independently
// re-checks expiry rather than trusting the caller's own pre-check (AD-2
// defense in depth).
func TestRestoreSession_RejectsExpired(t *testing.T) {
	svc := NewService()
	if err := svc.AddUser("frank", "pw", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}

	expired := &Session{
		ID:        "expired-session-id",
		Username:  "frank",
		Role:      "user",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // already expired
	}

	if err := svc.RestoreSession(expired); err == nil {
		t.Error("expected RestoreSession to reject an expired session")
	}
	if svc.ValidateSession(expired.ID) {
		t.Error("expired session must not be restored into the live store")
	}
}

// TestRestoreSession_RejectsUnknownUser verifies RestoreSession fails closed
// when the persisted session's username no longer exists in the live user
// store (AD-2 defense in depth: an account deleted since the session was
// persisted must not be restorable).
func TestRestoreSession_RejectsUnknownUser(t *testing.T) {
	svc := NewService()
	// Deliberately do not AddUser("ghost", ...).

	orphaned := &Session{
		ID:        "orphaned-session-id",
		Username:  "ghost",
		Role:      "user",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := svc.RestoreSession(orphaned); err == nil {
		t.Error("expected RestoreSession to reject a session for a nonexistent user")
	}
	if svc.ValidateSession(orphaned.ID) {
		t.Error("session for nonexistent user must not be restored into the live store")
	}
}

// TestRestoreSession_NilSession verifies RestoreSession fails closed rather
// than panicking on a nil session (defensive, not an explicit AD-2 case, but
// exercised because CLI disk I/O can plausibly produce a nil pointer on a
// malformed read).
func TestRestoreSession_NilSession(t *testing.T) {
	svc := NewService()
	if err := svc.RestoreSession(nil); err == nil {
		t.Error("expected RestoreSession(nil) to return an error")
	}
}
