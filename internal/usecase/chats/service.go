// Package chats orchestrates the web admin's Chats environment (FR-011–
// FR-016): create/list/get/delete a lightweight, directory-less Chat, and
// export its transcript. Named "chats" (plural) to avoid colliding with the
// existing internal/usecase/chat package, which is the full LLM/tool/RBAC
// orchestration engine — this package extends the existing
// domain.Conversation/ConversationRepository rather than introducing a new
// entity, per spec.md's "Key existing components to build on".
package chats

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chat"
)

// conversationExportAdapter adapts domain.ConversationRepository to
// chat.MemoryRepository's shape, so chat.Service.ExportConversation (FR-016)
// can be reused without constructing chat.Service's full LLM/tool/RBAC
// dependency graph — mirrors cmd/nuimanbot/main.go's
// conversationRepositoryAdapter (used there for a different interface).
type conversationExportAdapter struct {
	repo domain.ConversationRepository
}

func (a *conversationExportAdapter) SaveMessage(ctx context.Context, convID, _ string, _ domain.Platform, msg domain.StoredMessage) error {
	return a.repo.AppendMessage(ctx, convID, msg)
}

func (a *conversationExportAdapter) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	return a.repo.GetConversation(ctx, convID)
}

func (a *conversationExportAdapter) GetRecentMessages(ctx context.Context, convID string, _ int) ([]domain.StoredMessage, error) {
	conv, err := a.repo.GetConversation(ctx, convID)
	if err != nil {
		return nil, err
	}
	return conv.Messages, nil
}

func (a *conversationExportAdapter) DeleteConversation(ctx context.Context, convID string) error {
	return a.repo.DeleteConversation(ctx, convID)
}

func (a *conversationExportAdapter) ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error) {
	summaries, err := a.repo.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ConversationSummary, len(summaries))
	for i, s := range summaries {
		out[i] = *s
	}
	return out, nil
}

// Service orchestrates Chat create/list/get/delete/export.
type Service struct {
	conversations domain.ConversationRepository
	exporter      *chat.Service
	now           func() time.Time
}

// NewService creates a Chats Service backed by conversations.
func NewService(conversations domain.ConversationRepository) *Service {
	adapter := &conversationExportAdapter{repo: conversations}
	return &Service{
		conversations: conversations,
		// Only memoryRepo is ever touched by ExportConversation; the other
		// four chat.Service dependencies are intentionally nil here.
		exporter: chat.NewService(nil, adapter, nil, nil, nil),
		now:      time.Now,
	}
}

// CreateChat creates a new Chat (FR-011: no working directory exposed),
// auto-naming it from firstMessageText (FR-012), falling back to a
// timestamp-based name when firstMessageText is empty/whitespace/non-text
// (Edge Case #1). If firstMessageText has usable content, it is persisted
// as the Chat's first message.
func (s *Service) CreateChat(ctx context.Context, ownerUserID, firstMessageText string) (*domain.Conversation, error) {
	if ownerUserID == "" {
		return nil, fmt.Errorf("%w: ownerUserID is required", domain.ErrInvalidInput)
	}

	now := s.now()
	conv := &domain.Conversation{
		ID:        uuid.NewString(),
		UserID:    ownerUserID,
		Name:      domain.DeriveConversationName(firstMessageText, now),
		Platform:  domain.PlatformWeb,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if trimmed := strings.TrimSpace(firstMessageText); trimmed != "" {
		conv.Messages = []domain.StoredMessage{{
			ID:        uuid.NewString(),
			Role:      "user",
			Content:   firstMessageText,
			Timestamp: now,
		}}
	}

	if err := s.conversations.SaveConversation(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to save chat: %w", err)
	}
	return conv, nil
}

// ListChats returns Chat summaries owned by ownerUserID (FR-013's listing
// surface), most-recently-active first.
func (s *Service) ListChats(ctx context.Context, ownerUserID string) ([]*domain.ConversationSummary, error) {
	summaries, err := s.conversations.ListConversations(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	sortSummariesByUpdatedAtDesc(summaries)
	return summaries, nil
}

// GetChat retrieves a Chat by ID, enforcing ownership: a Chat owned by a
// different user resolves as domain.ErrNotFound (FR-010, Edge Case #10 —
// existence is never disclosed, including to admins, at this layer; a
// caller wanting an admin override must do so explicitly above this call,
// which the Chats environment does not).
func (s *Service) GetChat(ctx context.Context, ownerUserID, chatID string) (*domain.Conversation, error) {
	conv, err := s.conversations.GetConversation(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if conv.UserID != ownerUserID {
		return nil, domain.ErrNotFound
	}
	return conv, nil
}

// DeleteChat immediately and manually deletes a Chat (FR-015), enforcing ownership.
func (s *Service) DeleteChat(ctx context.Context, ownerUserID, chatID string) error {
	if _, err := s.GetChat(ctx, ownerUserID, chatID); err != nil {
		return err
	}
	return s.conversations.DeleteConversation(ctx, chatID)
}

// AppendUserMessage appends a user-authored message to an existing Chat the
// caller owns (FR-013's ongoing persistence), enforcing ownership.
func (s *Service) AppendUserMessage(ctx context.Context, ownerUserID, chatID, content string) error {
	if _, err := s.GetChat(ctx, ownerUserID, chatID); err != nil {
		return err
	}
	msg := domain.StoredMessage{
		ID:        uuid.NewString(),
		Role:      "user",
		Content:   content,
		Timestamp: s.now(),
	}
	return s.conversations.AppendMessage(ctx, chatID, msg)
}

// ExportChat exports a Chat's full transcript (FR-016), enforcing
// ownership, reusing chat.Service.ExportConversation rather than
// duplicating export logic.
func (s *Service) ExportChat(ctx context.Context, ownerUserID, chatID string, format chat.ExportFormat) (string, error) {
	if _, err := s.GetChat(ctx, ownerUserID, chatID); err != nil {
		return "", err
	}
	return s.exporter.ExportConversation(ctx, chatID, format)
}

// SweepExpired deletes every Chat owned by ownerUserID that is expired
// under policy (FR-014), measured from each Chat's UpdatedAt — not
// CreatedAt (Edge Case #12) — returning the count deleted. A "Never" policy
// deletes nothing.
func (s *Service) SweepExpired(ctx context.Context, ownerUserID string, policy domain.RetentionPolicy, now time.Time) (int, error) {
	if policy.IsNever() {
		return 0, nil
	}
	summaries, err := s.conversations.ListConversations(ctx, ownerUserID)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, summary := range summaries {
		if policy.IsExpired(summary.UpdatedAt, now) {
			if err := s.conversations.DeleteConversation(ctx, summary.ID); err != nil {
				continue // Best-effort sweep; one failure must not abort the rest.
			}
			deleted++
		}
	}
	return deleted, nil
}

// sortSummariesByUpdatedAtDesc sorts in place, most-recently-active first.
func sortSummariesByUpdatedAtDesc(summaries []*domain.ConversationSummary) {
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
}
