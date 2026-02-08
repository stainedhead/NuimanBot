package domain

import (
	"context"
	"errors"
	"time"
)

// BotType defines the type of bot (public or private)
type BotType string

const (
	BotTypePublic  BotType = "public"  // Public bot shared across users
	BotTypePrivate BotType = "private" // Private bot owned by single user
)

// SlackBotConfig represents configuration for a Slack bot instance
type SlackBotConfig struct {
	// Core Identity
	BotID   string  `json:"botID"`   // Unique bot identifier
	BotName string  `json:"botName"` // Display name for the bot
	BotType BotType `json:"botType"` // public or private

	// Ownership
	OwnerUserID string `json:"ownerUserID,omitempty"` // User ID for private bots, empty for public

	// Slack Credentials (encrypted at rest)
	SlackBotToken      string `json:"slackBotToken"`      // Bot token (xoxb-...)
	SlackAppToken      string `json:"slackAppToken"`      // App token (xapp-...)
	SlackSigningSecret string `json:"slackSigningSecret"` // Signing secret for webhooks

	// Slack Platform IDs
	SlackTeamID    string `json:"slackTeamID,omitempty"`    // Workspace/team ID
	SlackBotUserID string `json:"slackBotUserID,omitempty"` // Bot user ID (from OAuth)

	// Access Control
	Enabled        bool     `json:"enabled"`        // Bot enabled/disabled
	AllowedUserIDs []string `json:"allowedUserIDs"` // For public bots, list of user IDs with access

	// Metadata
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Custom metadata
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// Validate checks if the Slack bot configuration is valid
func (b *SlackBotConfig) Validate() error {
	if b.BotID == "" {
		return errors.New("botID is required")
	}

	if len(b.BotID) > 64 {
		return errors.New("botID must be <= 64 characters")
	}

	if b.BotName == "" {
		return errors.New("botName is required")
	}

	if len(b.BotName) > 100 {
		return errors.New("botName must be <= 100 characters")
	}

	if b.BotType != BotTypePublic && b.BotType != BotTypePrivate {
		return errors.New("botType must be 'public' or 'private'")
	}

	if b.SlackBotToken == "" {
		return errors.New("slackBotToken is required")
	}

	// Private bots must have an owner
	if b.BotType == BotTypePrivate && b.OwnerUserID == "" {
		return errors.New("ownerUserID is required for private bots")
	}

	// Public bots should not have an owner
	if b.BotType == BotTypePublic && b.OwnerUserID != "" {
		return errors.New("ownerUserID must be empty for public bots")
	}

	return nil
}

// IsOwnedByUser checks if the given user has access to this bot
func (b *SlackBotConfig) IsOwnedByUser(userID string) bool {
	if b.BotType == BotTypePrivate {
		return b.OwnerUserID == userID
	}

	// Public bot: check allowed users list
	for _, allowedID := range b.AllowedUserIDs {
		if allowedID == userID {
			return true
		}
	}

	return false
}

// TelegramBotConfig represents configuration for a Telegram bot instance
type TelegramBotConfig struct {
	// Core Identity
	BotID   string  `json:"botID"`   // Unique bot identifier
	BotName string  `json:"botName"` // Display name for the bot
	BotType BotType `json:"botType"` // public or private

	// Ownership
	OwnerUserID string `json:"ownerUserID,omitempty"` // User ID for private bots, empty for public

	// Telegram Credentials (encrypted at rest)
	TelegramBotToken    string `json:"telegramBotToken"`        // Bot token from @BotFather
	TelegramBotUsername string `json:"telegramBotUsername"`     // Bot username (without @)
	TelegramBotID       string `json:"telegramBotID,omitempty"` // Bot's Telegram ID (numeric)

	// Access Control
	Enabled        bool     `json:"enabled"`        // Bot enabled/disabled
	AllowedUserIDs []string `json:"allowedUserIDs"` // For public bots, list of user IDs with access
	AllowedChatIDs []string `json:"allowedChatIDs"` // Allowed Telegram chat IDs

	// Metadata
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Custom metadata
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// Validate checks if the Telegram bot configuration is valid
func (b *TelegramBotConfig) Validate() error {
	if b.BotID == "" {
		return errors.New("botID is required")
	}

	if len(b.BotID) > 64 {
		return errors.New("botID must be <= 64 characters")
	}

	if b.BotName == "" {
		return errors.New("botName is required")
	}

	if len(b.BotName) > 100 {
		return errors.New("botName must be <= 100 characters")
	}

	if b.BotType != BotTypePublic && b.BotType != BotTypePrivate {
		return errors.New("botType must be 'public' or 'private'")
	}

	if b.TelegramBotToken == "" {
		return errors.New("telegramBotToken is required")
	}

	// Private bots must have an owner
	if b.BotType == BotTypePrivate && b.OwnerUserID == "" {
		return errors.New("ownerUserID is required for private bots")
	}

	// Public bots should not have an owner
	if b.BotType == BotTypePublic && b.OwnerUserID != "" {
		return errors.New("ownerUserID must be empty for public bots")
	}

	return nil
}

// IsOwnedByUser checks if the given user has access to this bot
func (b *TelegramBotConfig) IsOwnedByUser(userID string) bool {
	if b.BotType == BotTypePrivate {
		return b.OwnerUserID == userID
	}

	// Public bot: check allowed users list
	for _, allowedID := range b.AllowedUserIDs {
		if allowedID == userID {
			return true
		}
	}

	return false
}

// BotConfigRepository defines the contract for bot configuration persistence
type BotConfigRepository interface {
	// SaveSlackBot creates or updates a Slack bot configuration
	SaveSlackBot(ctx context.Context, bot *SlackBotConfig) error

	// GetSlackBotByID retrieves a Slack bot by ID
	GetSlackBotByID(ctx context.Context, botID string) (*SlackBotConfig, error)

	// ListSlackBots returns all Slack bots
	ListSlackBots(ctx context.Context) ([]*SlackBotConfig, error)

	// DeleteSlackBot removes a Slack bot by ID
	DeleteSlackBot(ctx context.Context, botID string) error

	// SaveTelegramBot creates or updates a Telegram bot configuration
	SaveTelegramBot(ctx context.Context, bot *TelegramBotConfig) error

	// GetTelegramBotByID retrieves a Telegram bot by ID
	GetTelegramBotByID(ctx context.Context, botID string) (*TelegramBotConfig, error)

	// ListTelegramBots returns all Telegram bots
	ListTelegramBots(ctx context.Context) ([]*TelegramBotConfig, error)

	// DeleteTelegramBot removes a Telegram bot by ID
	DeleteTelegramBot(ctx context.Context, botID string) error
}

// NewSlackBotConfig creates a new Slack bot configuration with defaults
func NewSlackBotConfig(botID, botName string, botType BotType) *SlackBotConfig {
	now := time.Now()
	return &SlackBotConfig{
		BotID:          botID,
		BotName:        botName,
		BotType:        botType,
		Enabled:        true,
		AllowedUserIDs: []string{},
		Metadata:       make(map[string]interface{}),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewTelegramBotConfig creates a new Telegram bot configuration with defaults
func NewTelegramBotConfig(botID, botName string, botType BotType) *TelegramBotConfig {
	now := time.Now()
	return &TelegramBotConfig{
		BotID:          botID,
		BotName:        botName,
		BotType:        botType,
		Enabled:        true,
		AllowedUserIDs: []string{},
		AllowedChatIDs: []string{},
		Metadata:       make(map[string]interface{}),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
