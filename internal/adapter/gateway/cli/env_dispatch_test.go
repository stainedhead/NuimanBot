package cli_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/auth"
)

// TestIsEnvCommand_PrefixMatching verifies each environment command family's
// prefix-detection function requires a word-boundary match (bare command or
// "prefix<space>..."), never a mere string-prefix match that would let a
// longer, unrelated word falsely trigger dispatch.
func TestIsEnvCommand_PrefixMatching(t *testing.T) {
	tests := []struct {
		name  string
		check func(string) bool
		want  map[string]bool
	}{
		{
			name:  "chat",
			check: cli.IsChatCommand,
			want: map[string]bool{
				"/chat":        true,
				"/chat list":   true,
				"/chatter":     false,
				"chat":         false,
				"/chatty type": false,
			},
		},
		{
			name:  "project",
			check: cli.IsProjectCommand,
			want: map[string]bool{
				"/project":       true,
				"/project list":  true,
				"/projects list": false,
			},
		},
		{
			name:  "job",
			check: cli.IsJobCommand,
			want: map[string]bool{
				"/job":       true,
				"/job list":  true,
				"/jobs list": false,
			},
		},
		{
			name:  "chore",
			check: cli.IsChoreCommand,
			want: map[string]bool{
				"/chore":       true,
				"/chore list":  true,
				"/chores list": false,
			},
		},
		{
			name:  "history",
			check: cli.IsHistoryCommand,
			want: map[string]bool{
				"/history":      true,
				"/history list": true,
				"/historyx":     false,
			},
		},
		{
			name:  "settings",
			check: cli.IsSettingsCommand,
			want: map[string]bool{
				"/settings":      true,
				"/settings show": true,
				"/settingsx":     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for input, want := range tt.want {
				if got := tt.check(input); got != want {
					t.Errorf("%s(%q) = %v, want %v", tt.name, input, got, want)
				}
			}
		})
	}
}

// TestMemoriesVsMemory_NoCollision verifies AD-3: the new plural "/memories"
// dispatch never collides with the existing singular "/memory " admin
// prefix in either direction, and a bare "/memories" (no args) still
// matches — it must route to the Memories handler's own help output, not
// fall through as unrecognized (FR-010).
func TestMemoriesVsMemory_NoCollision(t *testing.T) {
	tests := []struct {
		input         string
		wantMemories  bool
		wantMemorySvc bool
	}{
		{"/memories", true, false},
		{"/memories browse", true, false},
		{"/memories chat cell-1 hello", true, false},
		{"/memory stats", false, true},
		{"/memory help", false, true},
	}

	for _, tt := range tests {
		if got := cli.IsMemoriesCommand(tt.input); got != tt.wantMemories {
			t.Errorf("IsMemoriesCommand(%q) = %v, want %v", tt.input, got, tt.wantMemories)
		}
		if got := cli.IsMemoryCommand(tt.input); got != tt.wantMemorySvc {
			t.Errorf("IsMemoryCommand(%q) = %v, want %v", tt.input, got, tt.wantMemorySvc)
		}
		// The two must never both claim the same input.
		if cli.IsMemoriesCommand(tt.input) && cli.IsMemoryCommand(tt.input) {
			t.Errorf("input %q matched both IsMemoriesCommand and IsMemoryCommand", tt.input)
		}
	}
}

// stubEnvHandler is a minimal cli.EnvCommandHandler for scaffold dispatch
// tests — real environment logic is added per-handler in later commits.
type stubEnvHandler struct {
	gotUser        *domain.User
	gotOwnerUserID string
	gotInput       string
	response       string
	err            error
}

func (s *stubEnvHandler) Handle(_ context.Context, currentUser *domain.User, ownerUserID, input string) (string, error) {
	s.gotUser = currentUser
	s.gotOwnerUserID = ownerUserID
	s.gotInput = input
	return s.response, s.err
}

// TestGateway_EnvHandlerSetters verify nil-safety (must not panic) for all
// seven Set*Handler methods.
func TestGateway_EnvHandlerSetters(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	g.SetChatsHandler(nil)
	g.SetProjectsHandler(nil)
	g.SetJobsHandler(nil)
	g.SetChoresHandler(nil)
	g.SetHistoryHandler(nil)
	g.SetMemoriesHandler(nil)
	g.SetSettingsHandler(nil)
}

// runGatewayInput runs the gateway's REPL over inputLines (auth disabled —
// no SetAuthHandler call, matching how most pre-existing gateway_test.go
// cases exercise dispatch without a real login flow) and returns everything
// written to g.Writer.
func runGatewayInput(t *testing.T, configure func(g *cli.Gateway), inputLines ...string) string {
	t.Helper()
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)
	output := new(bytes.Buffer)
	g.Writer = output
	if configure != nil {
		configure(g)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	g.Reader = r

	if _, err := w.WriteString(strings.Join(append(inputLines, "exit"), "\n") + "\n"); err != nil {
		t.Fatalf("write input: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- g.Start(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for gateway to exit")
	}

	return output.String()
}

// TestGateway_EnvCommand_NilHandlerShowsNotAvailable verifies each of the
// seven environment command families produces a clear "not available"
// message (not a silent no-op, not falling through to the plain-chat
// message handler) when its handler hasn't been wired yet.
func TestGateway_EnvCommand_NilHandlerShowsNotAvailable(t *testing.T) {
	tests := []struct {
		input string
		label string
	}{
		{"/chat list", "Chat"},
		{"/project list", "Project"},
		{"/job list", "Job"},
		{"/chore list", "Chore"},
		{"/history list", "History"},
		{"/memories browse", "Memories"},
		{"/settings show", "Settings"},
	}
	for _, tt := range tests {
		out := runGatewayInput(t, nil, tt.input)
		want := "Error: " + tt.label + " commands not available."
		if !strings.Contains(out, want) {
			t.Errorf("input %q: expected output to contain %q, got:\n%s", tt.input, want, out)
		}
	}
}

// TestGateway_EnvCommand_DispatchesToWiredHandler verifies a wired handler
// receives the current user, the correct ownerUserID (AD-5), and the raw
// input line, and that its response is written to output.
func TestGateway_EnvCommand_DispatchesToWiredHandler(t *testing.T) {
	stub := &stubEnvHandler{response: "chat list: (empty)"}
	out := runGatewayInput(t, func(g *cli.Gateway) {
		g.SetChatsHandler(stub)
		g.SetCurrentUser(&domain.User{ID: "u1", Username: "alice", Role: domain.RoleUser})
	}, "/chat list")

	if !strings.Contains(out, "chat list: (empty)") {
		t.Errorf("expected handler response in output, got:\n%s", out)
	}
	if stub.gotInput != "/chat list" {
		t.Errorf("expected handler to receive raw input '/chat list', got %q", stub.gotInput)
	}
	if stub.gotUser == nil || stub.gotUser.Username != "alice" {
		t.Errorf("expected handler to receive currentUser alice, got %+v", stub.gotUser)
	}
	// No SetAuthHandler was wired in this test, so currentSession stays nil
	// and platformUID() falls back to its documented placeholder — AD-5's
	// real session.Username plumbing (ownerUserID == currentSession.Username)
	// is verified end-to-end, through a real login, by
	// TestGateway_EnvCommand_OwnerUserIDMatchesAuthenticatedSession below.
	const wantUnauthenticatedPlatformUID = "cli_unauthenticated"
	if stub.gotOwnerUserID != wantUnauthenticatedPlatformUID {
		t.Errorf("expected ownerUserID fallback %q with no auth flow wired, got %q", wantUnauthenticatedPlatformUID, stub.gotOwnerUserID)
	}
}

// TestGateway_EnvCommand_OwnerUserIDMatchesAuthenticatedSession verifies
// AD-5 end-to-end: after a real login, ownerUserID passed to an
// EnvCommandHandler is exactly the authenticated session's Username, not
// currentUser.ID or any other identifier.
func TestGateway_EnvCommand_OwnerUserIDMatchesAuthenticatedSession(t *testing.T) {
	authSvc := auth.NewService()
	if err := authSvc.AddUser("carol", "pw123", "user"); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	userSvc := newIntegrationUserRoleService()
	sessionPath := cli.SessionFilePath(t.TempDir() + "/.nuimanbot_history")
	authHandler := cli.NewAuthCommandHandler(authSvc, userSvc, sessionPath, sequencePasswordReader("pw123"))

	stub := &stubEnvHandler{response: "ok"}
	out := runGatewayInput(t, func(g *cli.Gateway) {
		g.SetAuthHandler(authHandler)
		g.SetProjectsHandler(stub)
	}, "carol", "/project list")

	if stub.gotOwnerUserID != "carol" {
		t.Errorf("expected ownerUserID 'carol' (session.Username), got %q", stub.gotOwnerUserID)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected handler response in output, got:\n%s", out)
	}
}

// TestGateway_BareMemoriesRoutesToHandler verifies FR-010/AD-3: a bare
// "/memories" (no args, no trailing space) still routes to the Memories
// handler (for its own help output), not an unrecognized-command/message
// fallback.
func TestGateway_BareMemoriesRoutesToHandler(t *testing.T) {
	stub := &stubEnvHandler{response: "Memories commands:\n  browse [query]\n  chat <cell-id> <message>"}
	out := runGatewayInput(t, func(g *cli.Gateway) {
		g.SetMemoriesHandler(stub)
	}, "/memories")

	if stub.gotInput != "/memories" {
		t.Errorf("expected Memories handler to receive bare '/memories', got %q", stub.gotInput)
	}
	if !strings.Contains(out, "Memories commands:") {
		t.Errorf("expected bare /memories to show handler help, got:\n%s", out)
	}
}
