package domain

import (
	"strings"
	"time"
)

// Platform defines the messaging platform a message originated from or is destined for.
type Platform string

const (
	PlatformTelegram Platform = "telegram"
	PlatformSlack    Platform = "slack"
	PlatformCLI      Platform = "cli"
	PlatformBuzz     Platform = "buzz"
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
	ID     string
	UserID string
	// Name is the Chat's display name (FR-012), auto-derived from the text
	// of its first message. Empty for conversations created before this
	// field existed or via non-Chat gateways that don't set it; callers
	// needing a display name should fall back via ConversationDisplayName.
	Name      string
	Platform  Platform
	Messages  []StoredMessage
	CreatedAt time.Time
	UpdatedAt time.Time
	// Retention is this Chat's independently configurable retention policy
	// (FR-014). Zero value (RetentionPolicy{}) is equivalent to
	// NeverExpire() and should be treated as "no policy set / inherit
	// user default" by the usecase layer, not "expire immediately".
	Retention RetentionPolicy
}

// ConversationSummary represents a summary of a conversation, used for listings.
type ConversationSummary struct {
	ID                 string
	UserID             string
	Name               string
	Platform           Platform
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastMessageSnippet string // A snippet of the last message for quick overview
	MessageCount       int    // Total number of messages in conversation
}

// timestampNameLayout is the fallback Chat name format for spec.md's Edge
// Case #1 (empty/whitespace/non-text first message): "Chat — 2026-08-05
// 14:32".
const timestampNameLayout = "2006-01-02 15:04"

// FallbackConversationName returns the timestamp-based default Chat name
// used when the first message has no usable text content (spec.md Edge Case
// #1) — never an empty string.
func FallbackConversationName(at time.Time) string {
	return "Chat — " + at.Format(timestampNameLayout)
}

// DeriveConversationName returns a Chat's auto-name (FR-012) from the text
// of its first message, falling back to a timestamp-based default (Edge
// Case #1) when firstMessageText is empty or whitespace-only. createdAt is
// used for the fallback's timestamp.
func DeriveConversationName(firstMessageText string, createdAt time.Time) string {
	trimmed := strings.TrimSpace(firstMessageText)
	if trimmed == "" {
		return FallbackConversationName(createdAt)
	}
	const maxNameLength = 60
	runes := []rune(trimmed)
	if len(runes) > maxNameLength {
		return string(runes[:maxNameLength]) + "…"
	}
	return trimmed
}
