package main

import (
	"context"
	"errors"
	"testing"

	"nuimanbot/internal/domain"
)

// notFoundConversationRepo is a domain.ConversationRepository whose
// GetConversation always fails, simulating a brand-new conversation that
// has never had a message saved.
type notFoundConversationRepo struct{}

func (notFoundConversationRepo) SaveConversation(ctx context.Context, conv *domain.Conversation) error {
	return errors.New("unexpected call")
}

func (notFoundConversationRepo) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	return nil, domain.ErrNotFound
}

func (notFoundConversationRepo) ListConversations(ctx context.Context, userID string) ([]*domain.ConversationSummary, error) {
	return nil, errors.New("unexpected call")
}

func (notFoundConversationRepo) DeleteConversation(ctx context.Context, convID string) error {
	return errors.New("unexpected call")
}

func (notFoundConversationRepo) AppendMessage(ctx context.Context, convID string, message domain.StoredMessage) error {
	return errors.New("unexpected call")
}

func (notFoundConversationRepo) CountMessages(ctx context.Context, convID string) (int, error) {
	return 0, errors.New("unexpected call")
}

// TestConversationRepositoryAdapter_GetRecentMessages_NewConversation
// guards against the bug this feature's ACP smoke test surfaced: a brand
// new conversation's first message must not fail chat.Service.
// ProcessMessage with "failed to get recent messages: ...".
// domain.ErrNotFound specifically must be treated as zero prior messages —
// see TestConversationRepositoryAdapter_GetRecentMessages_RealErrorPropagates
// for why this must NOT be "any error means empty".
func TestConversationRepositoryAdapter_GetRecentMessages_NewConversation(t *testing.T) {
	adapter := &conversationRepositoryAdapter{repo: notFoundConversationRepo{}}

	messages, err := adapter.GetRecentMessages(context.Background(), "acp:some-new-session", 4096)
	if err != nil {
		t.Fatalf("expected no error for a brand-new conversation, got: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected zero messages for a brand-new conversation, got %d", len(messages))
	}
}

// brokenConversationRepo simulates a genuine failure (I/O error, corrupt
// index) distinct from "conversation doesn't exist yet".
type brokenConversationRepo struct{}

func (brokenConversationRepo) SaveConversation(ctx context.Context, conv *domain.Conversation) error {
	return errors.New("unexpected call")
}

func (brokenConversationRepo) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	return nil, errors.New("disk read failed")
}

func (brokenConversationRepo) ListConversations(ctx context.Context, userID string) ([]*domain.ConversationSummary, error) {
	return nil, errors.New("unexpected call")
}

func (brokenConversationRepo) DeleteConversation(ctx context.Context, convID string) error {
	return errors.New("unexpected call")
}

func (brokenConversationRepo) AppendMessage(ctx context.Context, convID string, message domain.StoredMessage) error {
	return errors.New("unexpected call")
}

func (brokenConversationRepo) CountMessages(ctx context.Context, convID string) (int, error) {
	return 0, errors.New("unexpected call")
}

// TestConversationRepositoryAdapter_GetRecentMessages_RealErrorPropagates
// guards against the naive fix for the bug above: treating *every*
// GetConversation error (not just domain.ErrNotFound) as "zero messages"
// would let a transiently unreadable-but-existing conversation look empty
// here, and SaveMessage (a few lines above GetRecentMessages in this same
// file) would then treat that same error as "doesn't exist, create fresh"
// — silently overwriting real conversation history. A non-ErrNotFound
// error must still fail loudly.
func TestConversationRepositoryAdapter_GetRecentMessages_RealErrorPropagates(t *testing.T) {
	adapter := &conversationRepositoryAdapter{repo: brokenConversationRepo{}}

	_, err := adapter.GetRecentMessages(context.Background(), "acp:some-session", 4096)
	if err == nil {
		t.Fatal("expected a real GetConversation error to propagate, got nil")
	}
	if errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected a non-ErrNotFound error, got one that matches ErrNotFound: %v", err)
	}
}
