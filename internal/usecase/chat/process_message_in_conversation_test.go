package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// TestProcessMessageInConversation_UsesGivenConversationIDNotDerived proves
// the defining difference from ProcessMessage: the conversation thread is
// whatever the caller passes, not getConversationID(platform, platformUID).
// This is what lets one web Chats user have many independent named Chat
// threads sharing the same (platform, platformUID) — ProcessMessage's
// automatic derivation collapses them all into one thread.
func TestProcessMessageInConversation_UsesGivenConversationIDNotDerived(t *testing.T) {
	var seenConvID string
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			seenConvID = convID
			return []domain.StoredMessage{}, nil
		},
	}
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "reply", FinishReason: "end_turn"}, nil
		},
	}
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
			return []domain.Tool{}, nil
		},
	}
	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})

	explicitConvID := "chat-uuid-abc123"
	user := &domain.User{ID: "user-1", Role: domain.RoleUser}
	incomingMsg := &domain.IncomingMessage{
		ID:          "msg-1",
		Platform:    domain.PlatformWeb,
		PlatformUID: "alice",
		Text:        "hello",
		Timestamp:   time.Now(),
	}

	out, err := service.ProcessMessageInConversation(context.Background(), explicitConvID, user, incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessageInConversation failed: %v", err)
	}
	if out.Content != "reply" {
		t.Errorf("expected reply content %q, got %q", "reply", out.Content)
	}

	// getConversationID(PlatformWeb, "alice") would be "web:alice" — the
	// point of this test is that it must NOT be what was used.
	derivedConvID := getConversationID(domain.PlatformWeb, "alice")
	if seenConvID != explicitConvID {
		t.Errorf("expected GetRecentMessages to receive the explicit conversationID %q, got %q", explicitConvID, seenConvID)
	}
	if seenConvID == derivedConvID {
		t.Errorf("explicit conversationID unexpectedly matched the derived one (%q) — test isn't distinguishing the two paths", derivedConvID)
	}
}

// TestProcessMessageInConversation_UsesGivenUserNotResolveUser proves the
// second half of the decoupling: the RBAC identity is whatever the caller
// passes, never looked up via resolveUser/UserService — a mockUserService
// with no functions set would otherwise silently return a RoleAdmin user
// (see its doc comment), masking this; asserting on the *tools* a
// RoleGuest-vs-RoleAdmin distinction would produce is a more direct check
// than trying to prove a negative (that UserService was never called).
func TestProcessMessageInConversation_UsesGivenUserNotResolveUser(t *testing.T) {
	var receivedUser *domain.User
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
			receivedUser = user
			return []domain.Tool{}, nil
		},
	}
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "reply", FinishReason: "end_turn"}, nil
		},
	}
	memoryRepo := &mockMemoryRepository{}
	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})

	explicitUser := &domain.User{ID: "explicit-user-id", Role: domain.RoleGuest}
	incomingMsg := &domain.IncomingMessage{
		ID:          "msg-1",
		Platform:    domain.PlatformWeb,
		PlatformUID: "bob",
		Text:        "hi",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessageInConversation(context.Background(), "some-chat-id", explicitUser, incomingMsg)
	if err != nil {
		t.Fatalf("ProcessMessageInConversation failed: %v", err)
	}

	if receivedUser == nil {
		t.Fatal("expected ListTools to be called with a user")
	}
	if receivedUser.ID != explicitUser.ID || receivedUser.Role != explicitUser.Role {
		t.Errorf("expected the explicitly-passed user %+v, got %+v — resolveUser's mock default (RoleAdmin) would indicate the wrong path ran", explicitUser, receivedUser)
	}
}

// TestProcessMessageInConversation_InputValidationError mirrors
// TestProcessMessage_InputValidationError for the new entry point.
func TestProcessMessageInConversation_InputValidationError(t *testing.T) {
	securityService := &mockSecurityService{
		validateInputFunc: func(ctx context.Context, input string, maxLength int) (string, error) {
			return "", errors.New("input validation failed: contains malicious content")
		},
	}
	service := createTestService(&mockLLMService{}, &mockMemoryRepository{}, &mockToolExecutionService{}, securityService)

	incomingMsg := &domain.IncomingMessage{
		ID:          "msg-1",
		Platform:    domain.PlatformWeb,
		PlatformUID: "carol",
		Text:        "bad input",
		Timestamp:   time.Now(),
	}

	_, err := service.ProcessMessageInConversation(context.Background(), "chat-id", &domain.User{ID: "carol", Role: domain.RoleUser}, incomingMsg)
	if err == nil {
		t.Fatal("expected an error from failed input validation")
	}
}

// TestProcessMessageInConversation_DistinctConversationsDoNotShareHistory
// exercises the actual motivating scenario: the same (platform,
// platformUID) pair used across two different conversationIDs must not
// collapse into one thread, unlike ProcessMessage.
func TestProcessMessageInConversation_DistinctConversationsDoNotShareHistory(t *testing.T) {
	seenConvIDs := map[string]int{}
	memoryRepo := &mockMemoryRepository{
		getRecentMessagesFunc: func(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
			seenConvIDs[convID]++
			return []domain.StoredMessage{}, nil
		},
	}
	llmService := &mockLLMService{
		completeFunc: func(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
			return &domain.LLMResponse{Content: "reply", FinishReason: "end_turn"}, nil
		},
	}
	toolExecService := &mockToolExecutionService{
		listSkillsFunc: func(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
			return []domain.Tool{}, nil
		},
	}
	service := createTestService(llmService, memoryRepo, toolExecService, &mockSecurityService{})

	user := &domain.User{ID: "dave", Role: domain.RoleUser}
	baseMsg := domain.IncomingMessage{Platform: domain.PlatformWeb, PlatformUID: "dave", Timestamp: time.Now()}

	msg1 := baseMsg
	msg1.ID, msg1.Text = "msg-1", "first chat"
	if _, err := service.ProcessMessageInConversation(context.Background(), "chat-1", user, &msg1); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	msg2 := baseMsg
	msg2.ID, msg2.Text = "msg-2", "second chat"
	if _, err := service.ProcessMessageInConversation(context.Background(), "chat-2", user, &msg2); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if seenConvIDs["chat-1"] != 1 || seenConvIDs["chat-2"] != 1 {
		t.Errorf("expected exactly one GetRecentMessages call per distinct chat, got %v", seenConvIDs)
	}
}
