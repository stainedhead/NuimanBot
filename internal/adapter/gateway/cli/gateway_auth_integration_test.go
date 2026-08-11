package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	cliadapter "nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/auth"

	"github.com/google/uuid"
)

// integrationUserRoleService is a minimal in-memory implementation of
// cli.UserRoleService for end-to-end gateway auth tests, mirroring
// internal/usecase/user.Service's behavior closely enough for this purpose.
type integrationUserRoleService struct {
	mu            sync.Mutex
	byPlatformUID map[string]*domain.User
	byID          map[string]*domain.User
}

func newIntegrationUserRoleService() *integrationUserRoleService {
	return &integrationUserRoleService{
		byPlatformUID: make(map[string]*domain.User),
		byID:          make(map[string]*domain.User),
	}
}

func (s *integrationUserRoleService) GetUserByPlatformUID(_ context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byPlatformUID[platformUID]
	if !ok || platform != domain.PlatformCLI {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (s *integrationUserRoleService) CreateUser(_ context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := &domain.User{ID: uuid.New().String(), Username: platformUID, Role: role, PlatformIDs: map[domain.Platform]string{platform: platformUID}}
	s.byPlatformUID[platformUID] = u
	s.byID[u.ID] = u
	return u, nil
}

func (s *integrationUserRoleService) UpdateUserRole(_ context.Context, userID string, role domain.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.Role = role
	return nil
}

// sequencePasswordReader returns each queued password in order, ignoring
// the scanner argument.
func sequencePasswordReader(passwords ...string) cli.PasswordReader {
	i := 0
	return func(_ *bufio.Scanner) (string, error) {
		if i >= len(passwords) {
			return "", errors.New("no more test passwords queued")
		}
		pw := passwords[i]
		i++
		return pw, nil
	}
}

// TestGateway_AuthGatesInputAndAttributesMessages is an end-to-end test of
// FR-001 (login gates all input), FR-002 (shared credential verification),
// FR-007 (chat attributed to the real logged-in identity, not a hardcoded
// placeholder), and AD-6 (identity reconciliation runs before any chat
// message is processed).
func TestGateway_AuthGatesInputAndAttributesMessages(t *testing.T) {
	authSvc := auth.NewService()
	if err := authSvc.AddUser("alice", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newIntegrationUserRoleService()
	sessionPath := cli.SessionFilePath(t.TempDir() + "/.nuimanbot_history")
	authHandler := cli.NewAuthCommandHandler(authSvc, userSvc, sessionPath, sequencePasswordReader("pw123"))

	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	output := new(bytes.Buffer)
	g.Writer = output
	g.SetAuthHandler(authHandler)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	g.Reader = r

	if _, err := w.WriteString("alice\nhello there\nexit\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}

	messageReceived := make(chan domain.IncomingMessage, 1)
	g.OnMessage(func(_ context.Context, msg domain.IncomingMessage) error {
		messageReceived <- msg
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- g.Start(context.Background()) }()

	select {
	case msg := <-messageReceived:
		if msg.PlatformUID != "alice" {
			t.Errorf("expected message attributed to 'alice', got %q", msg.PlatformUID)
		}
		if msg.Text != "hello there" {
			t.Errorf("expected text 'hello there', got %q", msg.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for authenticated chat message")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gateway to exit")
	}

	if !strings.Contains(output.String(), "Username:") {
		t.Error("expected a login prompt in output")
	}

	// AD-6: the reconciled domain.User must not have been silently granted
	// admin via defaultRoleForPlatform's CLI shortcut.
	created, err := userSvc.GetUserByPlatformUID(context.Background(), domain.PlatformCLI, "alice")
	if err != nil {
		t.Fatalf("GetUserByPlatformUID: %v", err)
	}
	if created.Role == domain.RoleAdmin {
		t.Error("expected non-admin login to reconcile to a non-admin domain.User")
	}
}

// TestGateway_LogoutRequiresReLogin verifies FR-005: after /logout, the next
// command requires login again.
func TestGateway_LogoutRequiresReLogin(t *testing.T) {
	authSvc := auth.NewService()
	if err := authSvc.AddUser("alice", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newIntegrationUserRoleService()
	sessionPath := cli.SessionFilePath(t.TempDir() + "/.nuimanbot_history")
	authHandler := cli.NewAuthCommandHandler(authSvc, userSvc, sessionPath, sequencePasswordReader("pw123", "pw123"))

	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	output := new(bytes.Buffer)
	g.Writer = output
	g.SetAuthHandler(authHandler)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	g.Reader = r

	if _, err := w.WriteString("alice\n/logout\nalice\nexit\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- g.Start(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gateway to exit")
	}

	out := output.String()
	if !strings.Contains(out, "Logged out.") {
		t.Error("expected 'Logged out.' after /logout")
	}
	if strings.Count(out, "Username:") < 2 {
		t.Errorf("expected a second login prompt after /logout, output:\n%s", out)
	}
	// By the time Start() has returned, the re-login (fed by the test input)
	// has already written a fresh session file for the new login — so a
	// file existing here is expected, not evidence logout skipped deletion.
	// deleteSessionFile's actual behavior (file removed immediately on
	// logout) is covered directly by TestLogout in auth_commands_test.go.
}

// TestGateway_MemoryCommandsRequireAdmin verifies P2.6: memory admin
// commands are rejected for a non-admin logged-in user, previously ungated.
func TestGateway_MemoryCommandsRequireAdmin(t *testing.T) {
	authSvc := auth.NewService()
	if err := authSvc.AddUser("bob", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newIntegrationUserRoleService()
	sessionPath := cli.SessionFilePath(t.TempDir() + "/.nuimanbot_history")
	authHandler := cli.NewAuthCommandHandler(authSvc, userSvc, sessionPath, sequencePasswordReader("pw123"))

	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	output := new(bytes.Buffer)
	g.Writer = output
	g.SetAuthHandler(authHandler)
	g.SetMemoryHandler(nil) // handler absence isn't what we're testing; role check runs first regardless

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	g.Reader = r

	if _, err := w.WriteString("bob\n/memory list\nexit\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- g.Start(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gateway to exit")
	}

	if !strings.Contains(output.String(), "insufficient permissions") {
		t.Errorf("expected a permission error for non-admin /memory command, got:\n%s", output.String())
	}
}

// TestGateway_MemoryCommandsWorkForAdmin verifies P2.6's second criterion:
// existing admin memory commands still work end-to-end for a logged-in
// admin user (mirrors TestGateway_MemoryCommandsRequireAdmin's non-admin
// rejection case, using a real MemoryCommandHandler over file-backed repos
// instead of a nil handler, so the admin path is actually exercised rather
// than assumed from the role-check reordering alone).
func TestGateway_MemoryCommandsWorkForAdmin(t *testing.T) {
	authSvc := auth.NewService()
	if err := authSvc.AddUser("carol", "pw123", "admin"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newIntegrationUserRoleService()
	sessionPath := cli.SessionFilePath(t.TempDir() + "/.nuimanbot_history")
	authHandler := cli.NewAuthCommandHandler(authSvc, userSvc, sessionPath, sequencePasswordReader("pw123"))

	cellRepo := storage.NewFileMemoryCellRepository(t.TempDir())
	sceneRepo := storage.NewFileMemorySceneRepository(t.TempDir())
	output := new(bytes.Buffer)
	memCmd := cliadapter.NewMemoryCommand(cellRepo, sceneRepo, output)
	memHandler := cli.NewMemoryCommandHandler(memCmd)

	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	g.Writer = output
	g.SetAuthHandler(authHandler)
	g.SetMemoryHandler(memHandler)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	g.Reader = r

	if _, err := w.WriteString("carol\n/memory list\nexit\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- g.Start(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for gateway to exit")
	}

	if strings.Contains(output.String(), "insufficient permissions") {
		t.Errorf("admin's /memory command was rejected as insufficient permissions, got:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "No memory cells found") {
		t.Errorf("expected /memory list to reach the real handler and report an empty result, got:\n%s", output.String())
	}
}

func TestGateway_SetAuthHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	g.SetAuthHandler(nil) // must not panic
}
