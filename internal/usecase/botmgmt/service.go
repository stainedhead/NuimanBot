package botmgmt

import (
	"context"
	"fmt"
	"time"

	"nuimanbot/internal/domain"
)

// Service provides bot management operations
type Service struct {
	repo domain.BotConfigRepository
}

// NewService creates a new bot management service
func NewService(repo domain.BotConfigRepository) *Service {
	return &Service{
		repo: repo,
	}
}

// Slack Bot Operations

// CreateSlackBot creates a new Slack bot configuration
func (s *Service) CreateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	// Validate bot configuration
	if err := bot.Validate(); err != nil {
		return fmt.Errorf("invalid bot configuration: %w", err)
	}

	// Set timestamps if not already set
	if bot.CreatedAt.IsZero() {
		bot.CreatedAt = time.Now()
	}
	bot.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.SaveSlackBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to save Slack bot: %w", err)
	}

	return nil
}

// GetSlackBot retrieves a Slack bot by ID
func (s *Service) GetSlackBot(ctx context.Context, botID string) (*domain.SlackBotConfig, error) {
	bot, err := s.repo.GetSlackBotByID(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Slack bot: %w", err)
	}
	return bot, nil
}

// ListSlackBots returns all Slack bots
func (s *Service) ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error) {
	bots, err := s.repo.ListSlackBots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Slack bots: %w", err)
	}
	return bots, nil
}

// ListSlackBotsByOwner returns all Slack bots owned by a specific user
func (s *Service) ListSlackBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error) {
	bots, err := s.repo.ListSlackBotsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to list Slack bots by owner: %w", err)
	}
	return bots, nil
}

// UpdateSlackBot updates an existing Slack bot configuration
func (s *Service) UpdateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	// Validate bot configuration
	if err := bot.Validate(); err != nil {
		return fmt.Errorf("invalid bot configuration: %w", err)
	}

	// Update timestamp
	bot.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.SaveSlackBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to update Slack bot: %w", err)
	}

	return nil
}

// DeleteSlackBot removes a Slack bot by ID
func (s *Service) DeleteSlackBot(ctx context.Context, botID string) error {
	if err := s.repo.DeleteSlackBot(ctx, botID); err != nil {
		return fmt.Errorf("failed to delete Slack bot: %w", err)
	}
	return nil
}

// EnableSlackBot enables a Slack bot
func (s *Service) EnableSlackBot(ctx context.Context, botID string) error {
	return s.setSlackBotEnabled(ctx, botID, true)
}

// DisableSlackBot disables a Slack bot
func (s *Service) DisableSlackBot(ctx context.Context, botID string) error {
	return s.setSlackBotEnabled(ctx, botID, false)
}

// setSlackBotEnabled is a helper method to set the enabled state of a Slack bot
func (s *Service) setSlackBotEnabled(ctx context.Context, botID string, enabled bool) error {
	bot, err := s.repo.GetSlackBotByID(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to get Slack bot: %w", err)
	}

	bot.Enabled = enabled
	bot.UpdatedAt = time.Now()

	action := "enable"
	if !enabled {
		action = "disable"
	}

	if err := s.repo.SaveSlackBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to %s Slack bot: %w", action, err)
	}

	return nil
}

// CheckSlackBotAccess checks if a user has access to a Slack bot
func (s *Service) CheckSlackBotAccess(ctx context.Context, botID, userID string) (bool, error) {
	bot, err := s.repo.GetSlackBotByID(ctx, botID)
	if err != nil {
		return false, fmt.Errorf("failed to get Slack bot: %w", err)
	}

	return bot.IsOwnedByUser(userID), nil
}

// Telegram Bot Operations

// CreateTelegramBot creates a new Telegram bot configuration
func (s *Service) CreateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	// Validate bot configuration
	if err := bot.Validate(); err != nil {
		return fmt.Errorf("invalid bot configuration: %w", err)
	}

	// Set timestamps if not already set
	if bot.CreatedAt.IsZero() {
		bot.CreatedAt = time.Now()
	}
	bot.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.SaveTelegramBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to save Telegram bot: %w", err)
	}

	return nil
}

// GetTelegramBot retrieves a Telegram bot by ID
func (s *Service) GetTelegramBot(ctx context.Context, botID string) (*domain.TelegramBotConfig, error) {
	bot, err := s.repo.GetTelegramBotByID(ctx, botID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Telegram bot: %w", err)
	}
	return bot, nil
}

// ListTelegramBots returns all Telegram bots
func (s *Service) ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error) {
	bots, err := s.repo.ListTelegramBots(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Telegram bots: %w", err)
	}
	return bots, nil
}

// ListTelegramBotsByOwner returns all Telegram bots owned by a specific user
func (s *Service) ListTelegramBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error) {
	bots, err := s.repo.ListTelegramBotsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to list Telegram bots by owner: %w", err)
	}
	return bots, nil
}

// UpdateTelegramBot updates an existing Telegram bot configuration
func (s *Service) UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	// Validate bot configuration
	if err := bot.Validate(); err != nil {
		return fmt.Errorf("invalid bot configuration: %w", err)
	}

	// Update timestamp
	bot.UpdatedAt = time.Now()

	// Save to repository
	if err := s.repo.SaveTelegramBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to update Telegram bot: %w", err)
	}

	return nil
}

// DeleteTelegramBot removes a Telegram bot by ID
func (s *Service) DeleteTelegramBot(ctx context.Context, botID string) error {
	if err := s.repo.DeleteTelegramBot(ctx, botID); err != nil {
		return fmt.Errorf("failed to delete Telegram bot: %w", err)
	}
	return nil
}

// EnableTelegramBot enables a Telegram bot
func (s *Service) EnableTelegramBot(ctx context.Context, botID string) error {
	return s.setTelegramBotEnabled(ctx, botID, true)
}

// DisableTelegramBot disables a Telegram bot
func (s *Service) DisableTelegramBot(ctx context.Context, botID string) error {
	return s.setTelegramBotEnabled(ctx, botID, false)
}

// setTelegramBotEnabled is a helper method to set the enabled state of a Telegram bot
func (s *Service) setTelegramBotEnabled(ctx context.Context, botID string, enabled bool) error {
	bot, err := s.repo.GetTelegramBotByID(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to get Telegram bot: %w", err)
	}

	bot.Enabled = enabled
	bot.UpdatedAt = time.Now()

	action := "enable"
	if !enabled {
		action = "disable"
	}

	if err := s.repo.SaveTelegramBot(ctx, bot); err != nil {
		return fmt.Errorf("failed to %s Telegram bot: %w", action, err)
	}

	return nil
}

// CheckTelegramBotAccess checks if a user has access to a Telegram bot
func (s *Service) CheckTelegramBotAccess(ctx context.Context, botID, userID string) (bool, error) {
	bot, err := s.repo.GetTelegramBotByID(ctx, botID)
	if err != nil {
		return false, fmt.Errorf("failed to get Telegram bot: %w", err)
	}

	return bot.IsOwnedByUser(userID), nil
}
