package domain

import "context"

// ConversationRepository defines operations for conversation persistence.
type ConversationRepository interface {
	// SaveConversation creates or updates a conversation.
	SaveConversation(ctx context.Context, conv *Conversation) error

	// GetConversation retrieves a conversation by ID.
	// Returns ErrNotFound if the conversation doesn't exist.
	GetConversation(ctx context.Context, convID string) (*Conversation, error)

	// ListConversations returns conversation summaries for a user.
	// Returns empty slice (never nil) if no conversations exist.
	ListConversations(ctx context.Context, userID string) ([]*ConversationSummary, error)

	// DeleteConversation removes a conversation by ID.
	// Returns ErrNotFound if the conversation doesn't exist.
	DeleteConversation(ctx context.Context, convID string) error

	// AppendMessage adds a message to an existing conversation.
	// Returns ErrNotFound if the conversation doesn't exist.
	AppendMessage(ctx context.Context, convID string, message StoredMessage) error

	// CountMessages returns the total number of messages in a conversation.
	// Returns ErrNotFound if the conversation doesn't exist.
	CountMessages(ctx context.Context, convID string) (int, error)
}
