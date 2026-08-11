package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/auth"
)

// maxLoginAttempts bounds the interactive login retry loop so a REPL never
// hangs forever on repeated bad credentials (still allows a real user a
// reasonable number of typos before failing closed).
const maxLoginAttempts = 3

// defaultSessionFileName is used when CLIConfig.HistoryFile is unset —
// data-dictionary.md specifies the session file lives "alongside the
// existing HistoryFile path convention," which is undefined if that field
// is blank.
const defaultSessionFileName = ".nuimanbot_session"

// UserRoleService is the subset of internal/usecase/user.Service's surface
// needed to reconcile a logged-in CLI identity's domain.User record with the
// account/role internal/usecase/auth.Service just authenticated (AD-6:
// internal/usecase/chat's defaultRoleForPlatform(PlatformCLI) hardcodes
// RoleAdmin, and must never fire for an authenticated CLI user).
type UserRoleService interface {
	GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error)
	CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error)
	UpdateUserRole(ctx context.Context, userID string, role domain.Role) error
}

// PasswordReader reads a single password without echoing it back. It
// receives the same *bufio.Scanner used for username input: the default
// implementation reuses it for the non-terminal fallback path (see
// defaultPasswordReader) rather than opening an independent bufio.Reader
// over os.Stdin — two independent buffered readers over the same underlying
// file descriptor can silently drop bytes the first one already buffered
// ahead. Test implementations are free to ignore the scanner argument.
type PasswordReader func(scanner *bufio.Scanner) (string, error)

// AuthCommandHandler implements the CLI's login/logout flow (FR-001-FR-005)
// against the shared internal/usecase/auth.Service, plus AD-6's identity
// reconciliation against domain.User via UserRoleService.
type AuthCommandHandler struct {
	authService     *auth.Service
	userRoleService UserRoleService
	sessionFilePath string
	passwordReader  PasswordReader
}

// NewAuthCommandHandler creates a new AuthCommandHandler.
//
// sessionFilePath is where the CLI persists the session record between
// process restarts (0600 permissions, AD-2). If passwordReader is nil, it
// defaults to masked terminal input via golang.org/x/term when stdin is a
// real terminal, falling back to a plain (unmasked) line read otherwise
// (piped input, non-interactive shells, tests).
func NewAuthCommandHandler(authService *auth.Service, userRoleService UserRoleService, sessionFilePath string, passwordReader PasswordReader) *AuthCommandHandler {
	if passwordReader == nil {
		passwordReader = defaultPasswordReader
	}
	return &AuthCommandHandler{
		authService:     authService,
		userRoleService: userRoleService,
		sessionFilePath: sessionFilePath,
		passwordReader:  passwordReader,
	}
}

// SessionFilePath derives the on-disk location for the persisted CLI
// session record (AD-2), alongside the configured history file's directory.
// Falls back to a default filename if historyFile is unset.
func SessionFilePath(historyFile string) string {
	if strings.TrimSpace(historyFile) == "" {
		historyFile = defaultSessionFileName
	}
	return filepath.Join(filepath.Dir(historyFile), defaultSessionFileName)
}

// defaultPasswordReader masks input via the terminal when stdin is a real
// terminal (golang.org/x/term.ReadPassword), and falls back to reading a
// plain line from the shared scanner otherwise (piped input, non-interactive
// shells) — reusing the same scanner instance that already read the
// username, not a second independent reader over os.Stdin.
func defaultPasswordReader(scanner *bufio.Scanner) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytePw, err := term.ReadPassword(fd)
		if err != nil {
			return "", fmt.Errorf("cli auth: read password: %w", err)
		}
		fmt.Println() // ReadPassword suppresses the newline the user typed
		return string(bytePw), nil
	}

	if !scanner.Scan() {
		return "", fmt.Errorf("cli auth: unexpected end of input reading password")
	}
	return scanner.Text(), nil
}

// EnsureAuthenticated is called once, before the REPL accepts any command or
// chat input (FR-001): it tries to restore a persisted session
// (FR-003/FR-004), falling back to an interactive login prompt
// (FR-001/FR-002) on any failure — absent, corrupted, expired, or rejected
// file — and never crashing (Reliability NFR). On success it also runs
// AD-6's identity-reconciliation step against domain.User before returning,
// so internal/usecase/chat's RBAC auto-provisioning path is never reached
// for this identity.
func (h *AuthCommandHandler) EnsureAuthenticated(ctx context.Context, out io.Writer, scanner *bufio.Scanner) (*domain.User, *auth.Session, error) {
	if session := h.tryRestoreSession(); session != nil {
		user, err := h.reconcileIdentity(ctx, session)
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(out, "Welcome back, %s.\n", session.Username)
		return user, session, nil
	}

	session, err := h.login(out, scanner)
	if err != nil {
		return nil, nil, err
	}

	user, err := h.reconcileIdentity(ctx, session)
	if err != nil {
		return nil, nil, err
	}
	return user, session, nil
}

// tryRestoreSession reads the on-disk session file (if any) and asks the
// shared auth.Service to restore it. Any failure (missing file, corrupted
// JSON, overly permissive file mode, expired session, or a RestoreSession
// rejection) simply returns nil — the caller falls back to a fresh login
// prompt; this never returns an error to avoid crashing the REPL.
func (h *AuthCommandHandler) tryRestoreSession() *auth.Session {
	session, err := readSessionFile(h.sessionFilePath)
	if err != nil {
		return nil
	}
	if err := h.authService.RestoreSession(session); err != nil {
		return nil
	}
	return session
}

// login runs the interactive username/password prompt (FR-001/FR-002),
// persisting the resulting session to disk on success (FR-003). It retries
// on invalid credentials up to maxLoginAttempts, showing the same generic
// error either way so a failed attempt never leaks whether the username
// exists versus the password being wrong (reuses auth.Service.
// ValidateCredentials's existing behavior; no new differential path).
func (h *AuthCommandHandler) login(out io.Writer, scanner *bufio.Scanner) (*auth.Session, error) {
	for attempt := 0; attempt < maxLoginAttempts; attempt++ {
		fmt.Fprint(out, "Username: ")
		if !scanner.Scan() {
			return nil, fmt.Errorf("cli auth: unexpected end of input reading username")
		}
		username := strings.TrimSpace(scanner.Text())

		fmt.Fprint(out, "Password: ")
		password, err := h.passwordReader(scanner)
		if err != nil {
			return nil, err
		}

		if !h.authService.ValidateCredentials(username, password) {
			fmt.Fprintln(out, "Invalid username or password. Please try again.")
			continue
		}

		authUser, _ := h.authService.GetUser(username)
		sessionID := h.authService.CreateSession(username, authUser.Role)
		session := h.authService.GetSession(sessionID)
		if session == nil {
			return nil, fmt.Errorf("cli auth: session unexpectedly missing immediately after creation")
		}

		if writeErr := writeSessionFile(h.sessionFilePath, session); writeErr != nil {
			fmt.Fprintf(out, "Warning: failed to persist session to disk (you will need to log in again next time): %v\n", writeErr)
		}

		return session, nil
	}

	return nil, fmt.Errorf("cli auth: too many failed login attempts")
}

// reconcileIdentity implements AD-6: it syncs the domain.User record for
// (PlatformCLI, session.Username) so its Role matches the just-authenticated
// session's real Role, guaranteeing internal/usecase/chat's resolveUser
// lookup always hits an existing, correctly-privileged record and its
// auto-create/defaultRoleForPlatform(PlatformCLI)=RoleAdmin branch is never
// exercised for an authenticated CLI user.
func (h *AuthCommandHandler) reconcileIdentity(ctx context.Context, session *auth.Session) (*domain.User, error) {
	role := domain.Role(session.Role)

	existing, err := h.userRoleService.GetUserByPlatformUID(ctx, domain.PlatformCLI, session.Username)
	if err == nil {
		if existing.Role != role {
			if updateErr := h.userRoleService.UpdateUserRole(ctx, existing.ID, role); updateErr != nil {
				return nil, fmt.Errorf("cli auth: reconcile identity: update role: %w", updateErr)
			}
			existing.Role = role
		}
		return existing, nil
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("cli auth: reconcile identity: lookup user: %w", err)
	}

	created, err := h.userRoleService.CreateUser(ctx, domain.PlatformCLI, session.Username, role)
	if err != nil {
		return nil, fmt.Errorf("cli auth: reconcile identity: create user: %w", err)
	}
	return created, nil
}

// Logout destroys the session (both in-memory and the persisted file),
// per FR-005.
func (h *AuthCommandHandler) Logout(sessionID string) error {
	h.authService.DestroySession(sessionID)
	return deleteSessionFile(h.sessionFilePath)
}

// IsLogoutCommand reports whether input is the /logout command.
func IsLogoutCommand(input string) bool {
	return strings.TrimSpace(input) == "/logout"
}

// sessionFileRecord is the on-disk JSON schema for a persisted CLI session
// (data-dictionary.md): mirrors auth.Session's fields exactly, minus
// ForcePasswordChange (the CLI doesn't implement the forced-default-
// credential-change UX — that's web-admin-only).
type sessionFileRecord struct {
	SessionID string    `json:"session_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// writeSessionFile persists session to disk with 0600 permissions.
//
// It removes any pre-existing file before writing: os.WriteFile's mode
// argument only applies at file *creation* and does not tighten the
// permissions of a file that already exists with a looser mode — and AD-2
// makes this file's permissions the entire trust boundary, so that matters.
func writeSessionFile(path string, session *auth.Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("cli auth: create session directory: %w", err)
	}

	rec := sessionFileRecord{
		SessionID: session.ID,
		Username:  session.Username,
		Role:      session.Role,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("cli auth: marshal session: %w", err)
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli auth: remove stale session file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("cli auth: write session file: %w", err)
	}
	return nil
}

// readSessionFile reads and parses the persisted session file, rejecting it
// outright if its on-disk permissions are looser than 0600 — AD-2's trust
// boundary is the file's OS permissions, so a file this process didn't just
// write with 0600 (e.g. group/world readable) is not trusted.
func readSessionFile(path string) (*auth.Session, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cli auth: stat session file: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("cli auth: session file %s has overly permissive mode %v, refusing to trust it", path, info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cli auth: read session file: %w", err)
	}

	var rec sessionFileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("cli auth: parse session file: %w", err)
	}

	return &auth.Session{
		ID:        rec.SessionID,
		Username:  rec.Username,
		Role:      rec.Role,
		CreatedAt: rec.CreatedAt,
		ExpiresAt: rec.ExpiresAt,
	}, nil
}

// deleteSessionFile removes the persisted session file, tolerating its
// absence.
func deleteSessionFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli auth: remove session file: %w", err)
	}
	return nil
}
