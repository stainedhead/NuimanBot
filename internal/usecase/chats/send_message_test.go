package chats

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

type mockChatProcessor struct {
	processFunc func(ctx context.Context, conversationID string, user *domain.User, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error)
	calls       []struct {
		conversationID string
		user           *domain.User
	}
}

func (m *mockChatProcessor) ProcessMessageInConversation(ctx context.Context, conversationID string, user *domain.User, incomingMsg *domain.IncomingMessage) (domain.OutgoingMessage, error) {
	m.calls = append(m.calls, struct {
		conversationID string
		user           *domain.User
	}{conversationID, user})
	if m.processFunc != nil {
		return m.processFunc(ctx, conversationID, user, incomingMsg)
	}
	return domain.OutgoingMessage{Content: "mock agent reply"}, nil
}

type mockUserService struct {
	users          map[string]*domain.User // keyed by platformUID
	getErr         error
	createErr      error
	updateRoleErr  error
	updateRoleCall struct {
		userID string
		role   domain.Role
	}
}

func newMockUserService() *mockUserService {
	return &mockUserService{users: map[string]*domain.User{}}
}

func (m *mockUserService) GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if u, ok := m.users[platformUID]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (m *mockUserService) CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	u := &domain.User{ID: "generated-" + platformUID, Role: role}
	m.users[platformUID] = u
	return u, nil
}

func (m *mockUserService) UpdateUserRole(ctx context.Context, userID string, role domain.Role) error {
	m.updateRoleCall.userID = userID
	m.updateRoleCall.role = role
	if m.updateRoleErr != nil {
		return m.updateRoleErr
	}
	for _, u := range m.users {
		if u.ID == userID {
			u.Role = role
		}
	}
	return nil
}

func newSendableTestService(t *testing.T, processor ChatProcessor, users UserService) *Service {
	t.Helper()
	repo := storage.NewFileConversationRepository(t.TempDir())
	return NewService(repo, processor, users)
}

func TestSendMessage_NotConfiguredReturnsError(t *testing.T) {
	s := newTestService(t) // chatProcessor/userService both nil
	ctx := context.Background()

	conv, err := s.CreateChat(ctx, "alice", "hi")
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}

	_, err = s.SendMessage(ctx, "alice", conv.ID, "hello", "user")
	if err == nil {
		t.Fatal("expected an error when chatProcessor/userService are unset")
	}
}

func TestSendMessage_EnforcesOwnership(t *testing.T) {
	processor := &mockChatProcessor{}
	users := newMockUserService()
	s := newSendableTestService(t, processor, users)
	ctx := context.Background()

	conv, err := s.CreateChat(ctx, "alice", "hi")
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}

	_, err = s.SendMessage(ctx, "mallory", conv.ID, "hello", "user")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a non-owner, got %v", err)
	}
	if len(processor.calls) != 0 {
		t.Errorf("expected the agent never to be invoked for a non-owner, got %d calls", len(processor.calls))
	}
}

func TestSendMessage_CreatesUserOnFirstMessage(t *testing.T) {
	processor := &mockChatProcessor{}
	users := newMockUserService()
	s := newSendableTestService(t, processor, users)
	ctx := context.Background()

	conv, err := s.CreateChat(ctx, "alice", "hi")
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}

	out, err := s.SendMessage(ctx, "alice", conv.ID, "hello", "admin")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if out.Content != "mock agent reply" {
		t.Errorf("expected the agent's reply to be returned, got %q", out.Content)
	}

	if len(processor.calls) != 1 {
		t.Fatalf("expected exactly one agent call, got %d", len(processor.calls))
	}
	call := processor.calls[0]
	if call.conversationID != conv.ID {
		t.Errorf("expected conversationID %q (the Chat's own ID), got %q", conv.ID, call.conversationID)
	}
	if call.user == nil || call.user.Role != domain.RoleAdmin {
		t.Errorf("expected a newly-created RoleAdmin user (from the passed role), got %+v", call.user)
	}
}

func TestSendMessage_SyncsRoleOnChange(t *testing.T) {
	processor := &mockChatProcessor{}
	users := newMockUserService()
	users.users["alice"] = &domain.User{ID: "alice-id", Role: domain.RoleGuest}
	s := newSendableTestService(t, processor, users)
	ctx := context.Background()

	conv, err := s.CreateChat(ctx, "alice", "hi")
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}

	// alice was Guest but has since been promoted to admin in the auth
	// session — SendMessage must reconcile the RBAC domain.User to match,
	// not silently keep using the stale Guest role.
	_, err = s.SendMessage(ctx, "alice", conv.ID, "hello", "admin")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if users.updateRoleCall.userID != "alice-id" || users.updateRoleCall.role != domain.RoleAdmin {
		t.Errorf("expected UpdateUserRole(alice-id, admin), got %+v", users.updateRoleCall)
	}
	if len(processor.calls) != 1 || processor.calls[0].user.Role != domain.RoleAdmin {
		t.Errorf("expected the agent to be called with the updated role, got %+v", processor.calls)
	}
}

func TestSendMessage_DoesNotDoubleAppendUserMessage(t *testing.T) {
	// SendMessage must not itself call AppendUserMessage/write the user's
	// message to the conversation repo — that's chatProcessor's job (mocked
	// out here, so it does nothing) — otherwise every message would be
	// persisted twice in production (once here, once by chat.Service's own
	// saveTurnMessages).
	processor := &mockChatProcessor{}
	users := newMockUserService()
	s := newSendableTestService(t, processor, users)
	ctx := context.Background()

	conv, err := s.CreateChat(ctx, "alice", "")
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}

	if _, err := s.SendMessage(ctx, "alice", conv.ID, "hello", "user"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	got, err := s.GetChat(ctx, "alice", conv.ID)
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if len(got.Messages) != 0 {
		t.Errorf("expected SendMessage to leave message persistence entirely to chatProcessor, got %d messages stored", len(got.Messages))
	}
}
