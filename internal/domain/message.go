package domain

import (
	"time"
)

// Platform defines the messaging platform a message originated from or is destined for.
type Platform string

const (
	PlatformTelegram Platform = "telegram"
	PlatformSlack    Platform = "slack"
	PlatformCLI      Platform = "cli"
)

// IncomingMessage represents a message received from a platform.
type IncomingMessage struct {
	ID          string
	Platform    Platform
	PlatformUID string // Platform-specific user ID
	Text        string
	Timestamp   time.Time
	Metadata    map[string]any
}

// OutgoingMessage represents a message to be sent to a platform.
type OutgoingMessage struct {
	RecipientID string
	Content     string
	Format      string // "text", "markdown"
	Metadata    map[string]any
}

// StoredMessage represents a message stored in memory/database.
type StoredMessage struct {
	ID          string
	Role        string // "user", "assistant", "system"
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
	TokenCount  int
	Timestamp   time.Time
}

// Conversation represents a conversation in memory/database.
type Conversation struct {
	ID        string
	UserID    string
	Platform  Platform
	Messages  []StoredMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConversationSummary represents a summary of a conversation, used for listings.
type ConversationSummary struct {
	ID                 string
	UserID             string
	Platform           Platform
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastMessageSnippet string // A snippet of the last message for quick overview
}
