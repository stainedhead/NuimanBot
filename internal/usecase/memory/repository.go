package memory

import (
	"context"

	"nuimanbot/internal/domain"
)

// MemoryConfig defines the configuration for the agent's long-term memory.
type MemoryConfig struct {
	Backend   MemoryBackend
	Citations MemoryCitationsMode
	QMD       MemoryQMDConfig
}

type MemoryBackend string

const (
	MemoryBackendBuiltin MemoryBackend = "builtin"
	MemoryBackendQMD     MemoryBackend = "qmd"
)

type MemoryCitationsMode string

const (
	MemoryCitationsModeAuto MemoryCitationsMode = "auto"
	MemoryCitationsModeOn   MemoryCitationsMode = "on"
	MemoryCitationsModeOff  MemoryCitationsMode = "off"
)

// MemoryQMDConfig defines configuration for the Queryable Memory Document (QMD) backend.
type MemoryQMDConfig struct {
	Command              string
	IncludeDefaultMemory bool
	Paths                []MemoryQMDIndexPath
	Sessions             struct {
		Enabled       bool
		ExportDir     string
		RetentionDays int
	}
	Update struct {
		Interval   string
		DebounceMs int
		OnBoot     bool
	}
	Limits struct {
		MaxResults       int
		MaxSnippetChars  int
		MaxInjectedChars int
		TimeoutMs        int
	}
}

// MemoryQMDIndexPath defines a path to a memory document or directory.
type MemoryQMDIndexPath struct {
	Path    string
	Name    string
	Pattern string
}

// MemoryRepository defines the contract for conversation memory persistence.
type MemoryRepository interface {
	// SaveMessage persists a message in a conversation.
	// If the conversation does not exist, it is created with the provided userID and platform.
	SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error

	// GetConversation retrieves a full conversation
	GetConversation(ctx context.Context, convID string) (*domain.Conversation, error)

	// GetRecentMessages retrieves messages up to a token limit
	GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error)

	// DeleteConversation removes a conversation
	DeleteConversation(ctx context.Context, convID string) error

	// ListConversations returns conversations for a user
	ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error)
}
