package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/auth"

	"github.com/google/uuid"
)

// fakeUserRoleService is an in-memory stand-in for internal/usecase/user.Service,
// implementing just the UserRoleService surface AuthCommandHandler needs.
type fakeUserRoleService struct {
	byPlatformUID map[string]*domain.User // keyed by platformUID (session.Username)
	byID          map[string]*domain.User
}

func newFakeUserRoleService() *fakeUserRoleService {
	return &fakeUserRoleService{
		byPlatformUID: make(map[string]*domain.User),
		byID:          make(map[string]*domain.User),
	}
}

func (f *fakeUserRoleService) GetUserByPlatformUID(_ context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	if platform != domain.PlatformCLI {
		return nil, domain.ErrUserNotFound
	}
	u, ok := f.byPlatformUID[platformUID]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRoleService) CreateUser(_ context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error) {
	if _, exists := f.byPlatformUID[platformUID]; exists {
		return nil, domain.ErrConflict
	}
	u := &domain.User{
		ID:          uuid.New().String(),
		Username:    platformUID,
		Role:        role,
		PlatformIDs: map[domain.Platform]string{platform: platformUID},
	}
	f.byPlatformUID[platformUID] = u
	f.byID[u.ID] = u
	return u, nil
}

func (f *fakeUserRoleService) UpdateUserRole(_ context.Context, userID string, role domain.Role) error {
	u, ok := f.byID[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.Role = role
	return nil
}

// fixedPasswordReader returns each password in sequence, one per call,
// ignoring the scanner argument (tests supply passwords directly rather
// than routing them through stdin).
func fixedPasswordReader(passwords ...string) PasswordReader {
	i := 0
	return func(_ *bufio.Scanner) (string, error) {
		if i >= len(passwords) {
			return "", errors.New("no more passwords")
		}
		pw := passwords[i]
		i++
		return pw, nil
	}
}

func newTestAuthHandler(t *testing.T, passwords ...string) (*AuthCommandHandler, *auth.Service, *fakeUserRoleService, string) {
	t.Helper()
	authSvc := auth.NewService()
	if err := authSvc.AddUser("alice", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newFakeUserRoleService()
	sessionPath := filepath.Join(t.TempDir(), ".nuimanbot_session")
	handler := NewAuthCommandHandler(authSvc, userSvc, sessionPath, fixedPasswordReader(passwords...))
	return handler, authSvc, userSvc, sessionPath
}

// TestEnsureAuthenticated_FreshLoginSuccess verifies a successful fresh
// login persists a session file and reconciles a brand-new domain.User
// (P2.1, P2.3).
func TestEnsureAuthenticated_FreshLoginSuccess(t *testing.T) {
	handler, _, userSvc, sessionPath := newTestAuthHandler(t, "pw123")

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	user, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("expected session username alice, got %q", session.Username)
	}
	if user.Role != domain.RoleUser {
		t.Errorf("expected reconciled user role RoleUser, got %q", user.Role)
	}
	if _, ok := userSvc.byPlatformUID["alice"]; !ok {
		t.Error("expected domain.User to be created for alice")
	}

	if _, err := os.Stat(sessionPath); err != nil {
		t.Errorf("expected session file to be written: %v", err)
	}
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("stat session file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected session file mode 0600, got %v", info.Mode().Perm())
	}
}

// TestEnsureAuthenticated_InvalidCredentialsThenSuccess verifies the retry
// loop: a bad password is rejected with a generic message and the user can
// retry.
func TestEnsureAuthenticated_InvalidCredentialsThenSuccess(t *testing.T) {
	handler, _, _, _ := newTestAuthHandler(t, "wrongpw", "pw123")

	scanner := bufio.NewScanner(strings.NewReader("alice\nalice\n"))
	var out bytes.Buffer

	_, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("expected eventual success for alice, got %q", session.Username)
	}
	if !strings.Contains(out.String(), "Invalid username or password") {
		t.Error("expected a generic invalid-credentials message for the failed attempt")
	}
}

// TestEnsureAuthenticated_TooManyFailedAttempts verifies the login loop
// fails closed after maxLoginAttempts rather than looping forever.
func TestEnsureAuthenticated_TooManyFailedAttempts(t *testing.T) {
	handler, _, _, _ := newTestAuthHandler(t, "bad1", "bad2", "bad3")

	scanner := bufio.NewScanner(strings.NewReader("alice\nalice\nalice\n"))
	var out bytes.Buffer

	if _, _, err := handler.EnsureAuthenticated(context.Background(), &out, scanner); err == nil {
		t.Error("expected an error after too many failed login attempts")
	}
}

// TestEnsureAuthenticated_DoesNotLeakUsernameExistence verifies an unknown
// username produces the same generic message as a known username with a
// wrong password (P2.1 acceptance criterion).
func TestEnsureAuthenticated_DoesNotLeakUsernameExistence(t *testing.T) {
	handler, _, _, _ := newTestAuthHandler(t, "anything")

	scanner := bufio.NewScanner(strings.NewReader("nobody\n"))
	var out bytes.Buffer

	_, _, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err == nil {
		t.Fatal("expected failure for unknown user after exhausting attempts is fine, but this single attempt should not succeed")
	}
	if !strings.Contains(out.String(), "Invalid username or password") {
		t.Error("expected the same generic message for an unknown username")
	}
}

// TestEnsureAuthenticated_RestoreValidSession verifies a valid, unexpired
// on-disk session skips the login prompt entirely (FR-003).
func TestEnsureAuthenticated_RestoreValidSession(t *testing.T) {
	handler, authSvc, userSvc, sessionPath := newTestAuthHandler(t)

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)
	if err := writeSessionFile(sessionPath, session); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}
	// Pre-seed the domain.User so reconciliation finds an existing record.
	if _, err := userSvc.CreateUser(context.Background(), domain.PlatformCLI, "alice", domain.RoleUser); err != nil {
		t.Fatalf("seed domain user: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader("")) // must not be read from
	var out bytes.Buffer

	user, restoredSession, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	if restoredSession.ID != sessionID {
		t.Errorf("expected restored session ID %q, got %q", sessionID, restoredSession.ID)
	}
	if user.Username != "alice" {
		t.Errorf("expected alice, got %q", user.Username)
	}
	if strings.Contains(out.String(), "Username:") {
		t.Error("expected no login prompt when a valid session is restored")
	}
}

// TestEnsureAuthenticated_RestoreCorrectsStaleRole is FR-007's (auto-review
// fix pass) mandatory end-to-end test: TestReconcileIdentity_UpdatesStaleRole
// already proves reconcileIdentity corrects a stale role when called
// directly, and TestEnsureAuthenticated_RestoreValidSession already proves
// the restore path works when the pre-seeded role already matches — but no
// existing test exercised EnsureAuthenticated's restore path specifically
// with a MISMATCHED pre-seeded role, to confirm the correction happens
// end-to-end through the public entry point (not just the internal method
// in isolation). This pre-seeds a domain.User for (PlatformCLI, "alice")
// with a stale RoleAdmin, writes a valid session file whose Role is "user"
// (the account's real, current role), and asserts both:
//  1. EnsureAuthenticated's returned user has the corrected RoleUser, and
//  2. the correction is durably visible via GetUserByPlatformUID afterward
//     — the same lookup internal/usecase/chat.Service's resolveUser would
//     perform on the first chat message (mirroring
//     TestReconcileIdentity_NonAdminChatDoesNotBecomeAdmin's pattern) —
//     which is what makes this end-to-end rather than a restatement of
//     TestReconcileIdentity_UpdatesStaleRole's direct-call assertion.
func TestEnsureAuthenticated_RestoreCorrectsStaleRole(t *testing.T) {
	handler, authSvc, userSvc, sessionPath := newTestAuthHandler(t)

	// Seed a stale admin record for alice, though her real account role
	// (as authSvc, the source of truth, has it) is "user".
	if _, err := userSvc.CreateUser(context.Background(), domain.PlatformCLI, "alice", domain.RoleAdmin); err != nil {
		t.Fatalf("seed stale domain user: %v", err)
	}

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)
	if session.Role != "user" {
		t.Fatalf("test setup: expected session Role %q, got %q", "user", session.Role)
	}
	if err := writeSessionFile(sessionPath, session); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader("")) // must not be read from — restore path only
	var out bytes.Buffer

	user, restoredSession, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	if restoredSession.ID != sessionID {
		t.Fatalf("expected restored session ID %q, got %q", sessionID, restoredSession.ID)
	}
	if strings.Contains(out.String(), "Username:") {
		t.Error("expected no login prompt when a valid session is restored")
	}

	// (1) EnsureAuthenticated's own return value must carry the corrected role.
	if user.Role != domain.RoleUser {
		t.Errorf("expected EnsureAuthenticated's restore path to correct the stale RoleAdmin to RoleUser, got %q", user.Role)
	}

	// (2) The correction must be durably visible via the same lookup
	// internal/usecase/chat.Service.resolveUser performs — proving this is
	// end-to-end, not just a return-value artifact.
	found, err := userSvc.GetUserByPlatformUID(context.Background(), domain.PlatformCLI, "alice")
	if err != nil {
		t.Fatalf("GetUserByPlatformUID: %v", err)
	}
	if found.Role != domain.RoleUser {
		t.Errorf("expected the stored domain.User's role to be corrected to RoleUser after restore, got %q", found.Role)
	}
}

// TestEnsureAuthenticated_ExpiredSessionFallsBackToLogin verifies an expired
// on-disk session falls back to the login prompt rather than erroring out
// (FR-004, Reliability NFR).
func TestEnsureAuthenticated_ExpiredSessionFallsBackToLogin(t *testing.T) {
	handler, authSvc, _, sessionPath := newTestAuthHandler(t, "pw123")

	expired := &auth.Session{
		ID:        "expired-id",
		Username:  "alice",
		Role:      "user",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := writeSessionFile(sessionPath, expired); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}
	_ = authSvc // sanity: authSvc has "alice" seeded via newTestAuthHandler

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	_, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("expected fallback to login to succeed, got error: %v", err)
	}
	if session.ID == "expired-id" {
		t.Error("expired session must not be restored")
	}
}

// TestEnsureAuthenticated_CorruptedSessionFileFallsBackToLogin verifies a
// corrupted (unparseable) session file falls back to login without a
// panic/crash (FR-004, Reliability NFR).
func TestEnsureAuthenticated_CorruptedSessionFileFallsBackToLogin(t *testing.T) {
	handler, _, _, sessionPath := newTestAuthHandler(t, "pw123")

	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupted session file: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	_, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("expected fallback to login to succeed, got error: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("expected fresh login for alice, got %q", session.Username)
	}
}

// TestEnsureAuthenticated_MissingSessionFileFallsBackToLogin verifies no
// session file at all falls back to login (the common first-run case).
func TestEnsureAuthenticated_MissingSessionFileFallsBackToLogin(t *testing.T) {
	handler, _, _, _ := newTestAuthHandler(t, "pw123")

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	_, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("expected fresh login for alice, got %q", session.Username)
	}
}

// TestEnsureAuthenticated_OverlyPermissiveSessionFileRejected verifies a
// session file with group/world-readable permissions is not trusted, even
// if its contents are otherwise valid — AD-2's entire trust boundary is the
// file's OS permissions.
func TestEnsureAuthenticated_OverlyPermissiveSessionFileRejected(t *testing.T) {
	handler, authSvc, _, sessionPath := newTestAuthHandler(t, "pw123")

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)
	if err := writeSessionFile(sessionPath, session); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}
	// Loosen the permissions after the fact, simulating a file that wasn't
	// written by this process's writeSessionFile (or a misconfigured umask).
	if err := os.Chmod(sessionPath, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	_, restored, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("expected fallback to login to succeed, got error: %v", err)
	}
	if restored.ID == sessionID {
		t.Error("overly permissive session file must not be trusted/restored")
	}
}

// TestEnsureAuthenticated_DeletedUserRejectsRestore verifies RestoreSession's
// fail-closed behavior propagates: a session file for a user that no longer
// exists in auth.Service falls back to login.
func TestEnsureAuthenticated_DeletedUserRejectsRestore(t *testing.T) {
	authSvc := auth.NewService()
	// Deliberately do not AddUser("ghost", ...).
	userSvc := newFakeUserRoleService()
	sessionPath := filepath.Join(t.TempDir(), ".nuimanbot_session")

	orphaned := &auth.Session{
		ID:        "orphaned-id",
		Username:  "ghost",
		Role:      "user",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := writeSessionFile(sessionPath, orphaned); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}

	if err := authSvc.AddUser("alice", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	handler := NewAuthCommandHandler(authSvc, userSvc, sessionPath, fixedPasswordReader("pw123"))

	scanner := bufio.NewScanner(strings.NewReader("alice\n"))
	var out bytes.Buffer

	_, session, err := handler.EnsureAuthenticated(context.Background(), &out, scanner)
	if err != nil {
		t.Fatalf("expected fallback to login to succeed, got error: %v", err)
	}
	if session.Username != "alice" {
		t.Errorf("expected fresh login for alice, got %q", session.Username)
	}
}

// TestReconcileIdentity_UpdatesStaleRole verifies AD-6: an existing
// domain.User whose Role no longer matches the just-authenticated session's
// real Role gets corrected (e.g. a demoted admin logging in again).
func TestReconcileIdentity_UpdatesStaleRole(t *testing.T) {
	handler, authSvc, userSvc, _ := newTestAuthHandler(t)
	// Seed a stale admin record for alice, though her real account role is "user".
	if _, err := userSvc.CreateUser(context.Background(), domain.PlatformCLI, "alice", domain.RoleAdmin); err != nil {
		t.Fatalf("seed stale domain user: %v", err)
	}

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)

	user, err := handler.reconcileIdentity(context.Background(), session)
	if err != nil {
		t.Fatalf("reconcileIdentity: %v", err)
	}
	if user.Role != domain.RoleUser {
		t.Errorf("expected stale RoleAdmin to be corrected to RoleUser, got %q", user.Role)
	}
}

// TestReconcileIdentity_NonAdminChatDoesNotBecomeAdmin verifies the concrete
// AD-6 bug scenario: a non-admin CLI login does not result in a domain.User
// with RoleAdmin, regardless of internal/usecase/chat's
// defaultRoleForPlatform(PlatformCLI)=RoleAdmin shortcut — because
// reconcileIdentity always creates/updates the record with the real role
// before any chat message is processed, resolveUser's auto-create path is
// never exercised.
func TestReconcileIdentity_NonAdminChatDoesNotBecomeAdmin(t *testing.T) {
	handler, authSvc, userSvc, _ := newTestAuthHandler(t)

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)

	user, err := handler.reconcileIdentity(context.Background(), session)
	if err != nil {
		t.Fatalf("reconcileIdentity: %v", err)
	}
	if user.Role == domain.RoleAdmin {
		t.Fatal("non-admin CLI login must not result in a RoleAdmin domain.User")
	}

	// Simulate resolveUser's lookup (what internal/usecase/chat.Service does
	// on the first chat message): it must find the reconciled record, not
	// fall through to defaultRoleForPlatform's auto-create-as-admin path.
	found, err := userSvc.GetUserByPlatformUID(context.Background(), domain.PlatformCLI, "alice")
	if err != nil {
		t.Fatalf("GetUserByPlatformUID: %v", err)
	}
	if found.Role == domain.RoleAdmin {
		t.Fatal("resolveUser's lookup must not observe RoleAdmin for a non-admin CLI login")
	}
}

// TestReconcileIdentity_AdminNotDowngraded is a regression test: an admin
// user's role is preserved (not accidentally downgraded) across
// reconciliation.
func TestReconcileIdentity_AdminNotDowngraded(t *testing.T) {
	handler, authSvc, userSvc, _ := newTestAuthHandler(t)
	if err := authSvc.AddUser("admin-user", "pw", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	if _, err := userSvc.CreateUser(context.Background(), domain.PlatformCLI, "admin-user", domain.RoleAdmin); err != nil {
		t.Fatalf("seed domain user: %v", err)
	}

	sessionID := authSvc.CreateSession("admin-user", "admin")
	session := authSvc.GetSession(sessionID)

	user, err := handler.reconcileIdentity(context.Background(), session)
	if err != nil {
		t.Fatalf("reconcileIdentity: %v", err)
	}
	if user.Role != domain.RoleAdmin {
		t.Errorf("expected admin role preserved, got %q", user.Role)
	}
}

// TestLogout verifies Logout destroys the in-memory session and deletes the
// on-disk file (FR-005).
func TestLogout(t *testing.T) {
	handler, authSvc, _, sessionPath := newTestAuthHandler(t)

	sessionID := authSvc.CreateSession("alice", "user")
	session := authSvc.GetSession(sessionID)
	if err := writeSessionFile(sessionPath, session); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}

	if err := handler.Logout(sessionID); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if authSvc.ValidateSession(sessionID) {
		t.Error("expected session to be destroyed after logout")
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("expected session file to be removed after logout")
	}
}

// TestIsLogoutCommand verifies the exact-match /logout detection.
func TestIsLogoutCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/logout", true},
		{"  /logout  ", true},
		{"/logout now", false},
		{"/login", false},
		{"logout", false},
	}
	for _, tt := range tests {
		if got := IsLogoutCommand(tt.input); got != tt.want {
			t.Errorf("IsLogoutCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestSessionFilePath verifies the derived path sits alongside HistoryFile's
// directory, with a sane fallback when HistoryFile is unset.
func TestSessionFilePath(t *testing.T) {
	tests := []struct {
		historyFile string
		wantDir     string
	}{
		{"", "."},
		{".nuimanbot_history", "."},
		{"/var/lib/nuimanbot/.nuimanbot_history", "/var/lib/nuimanbot"},
	}
	for _, tt := range tests {
		got := SessionFilePath(tt.historyFile)
		if filepath.Dir(got) != tt.wantDir {
			t.Errorf("SessionFilePath(%q) dir = %q, want %q", tt.historyFile, filepath.Dir(got), tt.wantDir)
		}
		if filepath.Base(got) != defaultSessionFileName {
			t.Errorf("SessionFilePath(%q) base = %q, want %q", tt.historyFile, filepath.Base(got), defaultSessionFileName)
		}
	}
}

// TestWriteSessionFile_TightensExistingLoosePermissions verifies
// writeSessionFile does not merely rely on os.WriteFile's mode argument
// (which does not tighten an existing file's mode) — it must actively
// enforce 0600 even when a looser-permission file already exists at the
// target path.
func TestWriteSessionFile_TightensExistingLoosePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed loose-permission file: %v", err)
	}

	session := &auth.Session{
		ID:        "id",
		Username:  "alice",
		Role:      "user",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := writeSessionFile(path, session); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 after write, got %v", info.Mode().Perm())
	}
}

// TestDeleteSessionFile_ToleratesAbsence verifies deleting an already-absent
// session file is not an error.
func TestDeleteSessionFile_ToleratesAbsence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	if err := deleteSessionFile(path); err != nil {
		t.Errorf("expected no error deleting an absent file, got %v", err)
	}
}
