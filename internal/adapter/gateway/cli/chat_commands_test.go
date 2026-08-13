package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/chats"
)

func newChatTestHandler(t *testing.T) *cli.ChatCommandHandler {
	t.Helper()
	repo := storage.NewFileConversationRepository(t.TempDir())
	return cli.NewChatCommandHandler(chats.NewService(repo, nil, nil))
}

// chatTestUser is a currentUser whose ID/Username deliberately differ from
// any ownerUserID used in these tests, so a test that erroneously threads
// currentUser instead of ownerUserID into the service is caught rather than
// passing by coincidence.
var chatTestUser = &domain.User{ID: "u-1", Username: "bob", Role: domain.RoleUser}

func TestChatCommandHandler_BareCommandShowsHelp(t *testing.T) {
	h := newChatTestHandler(t)
	out, err := h.Handle(context.Background(), chatTestUser, "alice", "/chat")
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(out, "Chat Commands:") {
		t.Errorf("expected help output, got:\n%s", out)
	}
}

func TestChatCommandHandler_List_Empty(t *testing.T) {
	h := newChatTestHandler(t)
	out, err := h.HandleChatCommand(context.Background(), chatTestUser, "alice", "/chat list")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "No chats") {
		t.Errorf("expected empty-list message, got:\n%s", out)
	}
}

func TestChatCommandHandler_List_NonEmpty(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "Plan a trip to Kyoto")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat list")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	// NOTE: storage.FileConversationRepository's ConversationIndexEntry has
	// no Name field, so domain.Conversation.Name never survives a
	// save/reload round-trip (a pre-existing gap in shared infrastructure,
	// outside this package — see the handoff report). List output is
	// therefore only asserted on ID/count here, not the (currently always
	// blank) Name column.
	if !strings.Contains(out, id) {
		t.Errorf("expected chat ID in list output, got:\n%s", out)
	}
	if !strings.Contains(out, "Found 1 chat(s)") {
		t.Errorf("expected chat count in list output, got:\n%s", out)
	}
}

func TestChatCommandHandler_New_CreatesChatWithFullMessageText(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	// The "new" confirmation reflects the in-memory *domain.Conversation
	// CreateChat returns directly (not a repository round-trip), so it is
	// unaffected by the Name-persistence gap noted above.
	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat new  Help   me   plan   a trip")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "Chat created") {
		t.Errorf("expected creation confirmation, got:\n%s", out)
	}
	if !strings.Contains(out, "Help   me   plan   a trip") {
		t.Errorf("expected internal whitespace preserved in derived chat name, got:\n%s", out)
	}
}

func TestChatCommandHandler_New_RequiresMessage(t *testing.T) {
	h := newChatTestHandler(t)
	out, err := h.HandleChatCommand(context.Background(), chatTestUser, "alice", "/chat new")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected usage message, got:\n%s", out)
	}
}

func TestChatCommandHandler_ShowFound(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "hello there")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat show "+id)
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "hello there") {
		t.Errorf("expected first message in show output, got:\n%s", out)
	}
}

func TestChatCommandHandler_ShowNotFound(t *testing.T) {
	h := newChatTestHandler(t)
	_, err := h.HandleChatCommand(context.Background(), chatTestUser, "alice", "/chat show does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing chat")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestChatCommandHandler_Send_ToExistingChat(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "first message")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat send "+id+" a follow up message")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "sent") {
		t.Errorf("expected send confirmation, got:\n%s", out)
	}

	show, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat show "+id)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(show, "a follow up message") {
		t.Errorf("expected appended message text preserved, got:\n%s", show)
	}
}

func TestChatCommandHandler_Send_MissingChatReturnsError(t *testing.T) {
	h := newChatTestHandler(t)
	_, err := h.HandleChatCommand(context.Background(), chatTestUser, "alice", "/chat send does-not-exist hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

func TestChatCommandHandler_Delete(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "delete me")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat delete "+id)
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "deleted") {
		t.Errorf("expected deletion confirmation, got:\n%s", out)
	}

	if _, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat show "+id); err == nil {
		t.Fatal("expected chat to be gone after delete")
	}
}

func TestChatCommandHandler_Export_DefaultFormatReturnsTranscript(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "export me please")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat export "+id)
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty transcript text")
	}
}

func TestChatCommandHandler_Export_JSONFormat(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "export me as json")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat export "+id+" json")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "{") {
		t.Errorf("expected JSON-shaped output, got:\n%s", out)
	}
}

func TestChatCommandHandler_Export_InvalidFormat(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "export me")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat export "+id+" xml")
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, "Unknown export format") {
		t.Errorf("expected unknown-format message, got:\n%s", out)
	}
}

func TestChatCommandHandler_Export_WithPathWritesFile(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "export me to disk")
	path := filepath.Join(t.TempDir(), "transcript.md")

	out, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat export "+id+" markdown "+path)
	if err != nil {
		t.Fatalf("HandleChatCommand: %v", err)
	}
	if !strings.Contains(out, path) {
		t.Errorf("expected confirmation to mention file path, got:\n%s", out)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected export file to exist: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected non-empty export file content")
	}
}

func TestChatCommandHandler_Export_MissingChatReturnsError(t *testing.T) {
	h := newChatTestHandler(t)
	_, err := h.HandleChatCommand(context.Background(), chatTestUser, "alice", "/chat export does-not-exist")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

// TestChatCommandHandler_OwnerUserIDNotCurrentUser is the single
// highest-value test for AD-5: it proves the handler scopes every service
// call by the explicit ownerUserID parameter, never by currentUser's
// identity, by using a currentUser whose ID/Username never appear as an
// ownerUserID anywhere else in this file.
func TestChatCommandHandler_OwnerUserIDNotCurrentUser(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	// currentUser is "bob" but the data is scoped to ownerUserID "alice".
	id := createChat(t, ctx, h, "alice", "owned by alice, not bob")

	// The chat must be visible under ownerUserID "alice" even though
	// currentUser is "bob" throughout.
	show, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat show "+id)
	if err != nil {
		t.Fatalf("expected chat visible under ownerUserID alice, got err: %v", err)
	}
	if !strings.Contains(show, "owned by alice, not bob") {
		t.Errorf("expected chat content, got:\n%s", show)
	}

	// The same chat must NOT be visible if ownerUserID is switched to
	// "bob" (currentUser.Username), proving the handler didn't silently
	// substitute currentUser's identity for ownerUserID anywhere.
	if _, err := h.HandleChatCommand(ctx, chatTestUser, "bob", "/chat show "+id); err == nil {
		t.Fatal("expected chat to be invisible under ownerUserID bob (currentUser's own identity)")
	}
}

// TestChatCommandHandler_CrossUserIsolation verifies a CLI-level guarantee
// mirroring chats.Service's own ownership enforcement: user A cannot
// see/act on user B's chat via list/show/delete through the CLI handler.
func TestChatCommandHandler_CrossUserIsolation(t *testing.T) {
	h := newChatTestHandler(t)
	ctx := context.Background()

	id := createChat(t, ctx, h, "alice", "alice's private chat")

	// user B (ownerUserID "bob") must not see it in their list.
	list, err := h.HandleChatCommand(ctx, chatTestUser, "bob", "/chat list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(list, "alice's private chat") {
		t.Errorf("expected user bob's list to exclude alice's chat, got:\n%s", list)
	}

	// user B must not be able to show it.
	if _, err := h.HandleChatCommand(ctx, chatTestUser, "bob", "/chat show "+id); err == nil {
		t.Fatal("expected error showing another user's chat")
	}

	// user B must not be able to delete it.
	if _, err := h.HandleChatCommand(ctx, chatTestUser, "bob", "/chat delete "+id); err == nil {
		t.Fatal("expected error deleting another user's chat")
	}

	// It must still exist for its real owner.
	if _, err := h.HandleChatCommand(ctx, chatTestUser, "alice", "/chat show "+id); err != nil {
		t.Fatalf("expected chat to still exist for its owner, got: %v", err)
	}
}

// TestChatCommandHandler_CrossAdapterVisibility verifies the assumption
// cmd/nuimanbot/main.go's wiring depends on: the web admin's Chats
// environment and the CLI's /chat commands each construct their own
// chats.Service instance, but both wrap the *same* app.ConversationRepo
// (see main.go's "webServer.SetChatsService(chats.NewService(app.
// ConversationRepo))" and "cliGateway.SetChatsHandler(cli.
// NewChatCommandHandler(chats.NewService(app.ConversationRepo)))"). A chat
// created through one instance must be visible through the other — this is
// the hard "data created via CLI visible in web and vice versa" acceptance
// criterion, exercised directly against two independently constructed
// chats.Service values over one shared repository, rather than only
// inferred from both call sites passing the same ownerUserID convention.
func TestChatCommandHandler_CrossAdapterVisibility(t *testing.T) {
	repo := storage.NewFileConversationRepository(t.TempDir())
	cliHandler := cli.NewChatCommandHandler(chats.NewService(repo, nil, nil))
	ctx := context.Background()

	id := createChat(t, ctx, cliHandler, "alice", "created via CLI")

	// A second, independently constructed chats.Service over the same repo
	// stands in for the web admin's own instance (main.go constructs one
	// for each adapter; this test can't import internal/adapter/web without
	// violating the no-cross-adapter-import rule, so it exercises the
	// shared-repo assumption directly instead).
	webSideService := chats.NewService(repo, nil, nil)
	summaries, err := webSideService.ListChats(ctx, "alice")
	if err != nil {
		t.Fatalf("ListChats via the web-side service instance: %v", err)
	}
	found := false
	for _, s := range summaries {
		if s.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("chat %q created via the CLI handler is not visible through a separate chats.Service instance over the same repo; got summaries: %+v", id, summaries)
	}

	conv, err := webSideService.GetChat(ctx, "alice", id)
	if err != nil {
		t.Fatalf("GetChat via the web-side service instance: %v", err)
	}
	if len(conv.Messages) == 0 || conv.Messages[0].Content != "created via CLI" {
		t.Errorf("expected the CLI-created first message to be visible via the web-side instance, got: %+v", conv.Messages)
	}
}

// createChat is a test helper that creates a chat via the CLI handler and
// returns its ID, extracted from the handler's confirmation output.
func createChat(t *testing.T, ctx context.Context, h *cli.ChatCommandHandler, ownerUserID, message string) string {
	t.Helper()
	out, err := h.HandleChatCommand(ctx, chatTestUser, ownerUserID, "/chat new "+message)
	if err != nil {
		t.Fatalf("create chat: %v", err)
	}
	// Confirmation format: "✓ Chat created: <name> (<id>)"
	start := strings.LastIndex(out, "(")
	end := strings.LastIndex(out, ")")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("could not parse chat ID from confirmation: %q", out)
	}
	return strings.TrimSpace(out[start+1 : end])
}
