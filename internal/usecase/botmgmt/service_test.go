package botmgmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// Mock repository for testing
type mockBotConfigRepository struct {
	slackBots    map[string]*domain.SlackBotConfig
	telegramBots map[string]*domain.TelegramBotConfig
	saveError    error
	getError     error
	deleteError  error
}

func newMockBotConfigRepository() *mockBotConfigRepository {
	return &mockBotConfigRepository{
		slackBots:    make(map[string]*domain.SlackBotConfig),
		telegramBots: make(map[string]*domain.TelegramBotConfig),
	}
}

func (m *mockBotConfigRepository) SaveSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.slackBots[bot.BotID] = bot
	return nil
}

func (m *mockBotConfigRepository) GetSlackBotByID(ctx context.Context, botID string) (*domain.SlackBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bot, exists := m.slackBots[botID]
	if !exists {
		return nil, errors.New("bot not found")
	}
	return bot, nil
}

func (m *mockBotConfigRepository) ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bots := make([]*domain.SlackBotConfig, 0, len(m.slackBots))
	for _, bot := range m.slackBots {
		bots = append(bots, bot)
	}
	return bots, nil
}

func (m *mockBotConfigRepository) ListSlackBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bots := make([]*domain.SlackBotConfig, 0)
	for _, bot := range m.slackBots {
		if bot.OwnerUserID == ownerUserID {
			bots = append(bots, bot)
		}
	}
	return bots, nil
}

func (m *mockBotConfigRepository) DeleteSlackBot(ctx context.Context, botID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.slackBots, botID)
	return nil
}

func (m *mockBotConfigRepository) SaveTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.telegramBots[bot.BotID] = bot
	return nil
}

func (m *mockBotConfigRepository) GetTelegramBotByID(ctx context.Context, botID string) (*domain.TelegramBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bot, exists := m.telegramBots[botID]
	if !exists {
		return nil, errors.New("bot not found")
	}
	return bot, nil
}

func (m *mockBotConfigRepository) ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bots := make([]*domain.TelegramBotConfig, 0, len(m.telegramBots))
	for _, bot := range m.telegramBots {
		bots = append(bots, bot)
	}
	return bots, nil
}

func (m *mockBotConfigRepository) ListTelegramBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bots := make([]*domain.TelegramBotConfig, 0)
	for _, bot := range m.telegramBots {
		if bot.OwnerUserID == ownerUserID {
			bots = append(bots, bot)
		}
	}
	return bots, nil
}

func (m *mockBotConfigRepository) DeleteTelegramBot(ctx context.Context, botID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.telegramBots, botID)
	return nil
}

// Test Slack Bot Operations
func TestService_CreateSlackBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	tests := []struct {
		name    string
		bot     *domain.SlackBotConfig
		wantErr bool
	}{
		{
			name: "create valid public bot",
			bot: &domain.SlackBotConfig{
				BotID:              "bot-1",
				BotName:            "Test Bot",
				BotType:            domain.BotTypePublic,
				SlackBotToken:      "xoxb-test",
				SlackAppToken:      "xapp-test",
				SlackSigningSecret: "secret",
				Enabled:            true,
				AllowedUserIDs:     []string{"user-1"},
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			},
			wantErr: false,
		},
		{
			name: "create valid private bot",
			bot: &domain.SlackBotConfig{
				BotID:              "bot-2",
				BotName:            "Private Bot",
				BotType:            domain.BotTypePrivate,
				OwnerUserID:        "user-1",
				SlackBotToken:      "xoxb-test",
				SlackAppToken:      "xapp-test",
				SlackSigningSecret: "secret",
				Enabled:            true,
				CreatedAt:          time.Now(),
				UpdatedAt:          time.Now(),
			},
			wantErr: false,
		},
		{
			name: "fail on invalid bot",
			bot: &domain.SlackBotConfig{
				BotID:   "",
				BotName: "Invalid Bot",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CreateSlackBot(ctx, tt.bot)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSlackBot() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify bot was saved
				saved, err := repo.GetSlackBotByID(ctx, tt.bot.BotID)
				if err != nil {
					t.Fatalf("failed to retrieve saved bot: %v", err)
				}
				if saved.BotID != tt.bot.BotID {
					t.Errorf("saved bot ID = %v, want %v", saved.BotID, tt.bot.BotID)
				}
			}
		})
	}
}

func TestService_GetSlackBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create a bot first
	bot := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Test Bot",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	repo.SaveSlackBot(ctx, bot)

	tests := []struct {
		name    string
		botID   string
		wantErr bool
	}{
		{
			name:    "get existing bot",
			botID:   "bot-1",
			wantErr: false,
		},
		{
			name:    "get non-existent bot",
			botID:   "bot-999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetSlackBot(ctx, tt.botID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSlackBot() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("GetSlackBot() returned nil bot")
			}
		})
	}
}

func TestService_ListSlackBots(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create test bots
	bot1 := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Bot 1",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	bot2 := &domain.SlackBotConfig{
		BotID:              "bot-2",
		BotName:            "Bot 2",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "user-1",
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            false,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	repo.SaveSlackBot(ctx, bot1)
	repo.SaveSlackBot(ctx, bot2)

	bots, err := service.ListSlackBots(ctx)
	if err != nil {
		t.Fatalf("ListSlackBots() error = %v", err)
	}

	if len(bots) != 2 {
		t.Errorf("ListSlackBots() returned %d bots, want 2", len(bots))
	}
}

func TestService_UpdateSlackBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create initial bot
	bot := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Original Name",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	repo.SaveSlackBot(ctx, bot)

	// Update bot
	bot.BotName = "Updated Name"
	bot.Enabled = false

	err := service.UpdateSlackBot(ctx, bot)
	if err != nil {
		t.Fatalf("UpdateSlackBot() error = %v", err)
	}

	// Verify update
	updated, err := repo.GetSlackBotByID(ctx, bot.BotID)
	if err != nil {
		t.Fatalf("failed to retrieve updated bot: %v", err)
	}

	if updated.BotName != "Updated Name" {
		t.Errorf("BotName = %v, want %v", updated.BotName, "Updated Name")
	}

	if updated.Enabled != false {
		t.Errorf("Enabled = %v, want false", updated.Enabled)
	}
}

func TestService_DeleteSlackBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create bot
	bot := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Test Bot",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	repo.SaveSlackBot(ctx, bot)

	// Delete bot
	err := service.DeleteSlackBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("DeleteSlackBot() error = %v", err)
	}

	// Verify deletion
	_, err = repo.GetSlackBotByID(ctx, "bot-1")
	if err == nil {
		t.Error("expected error when getting deleted bot")
	}
}

func TestService_EnableDisableSlackBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create bot
	bot := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Test Bot",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	repo.SaveSlackBot(ctx, bot)

	// Disable bot
	err := service.DisableSlackBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("DisableSlackBot() error = %v", err)
	}

	disabled, _ := repo.GetSlackBotByID(ctx, "bot-1")
	if disabled.Enabled {
		t.Error("bot should be disabled")
	}

	// Enable bot
	err = service.EnableSlackBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("EnableSlackBot() error = %v", err)
	}

	enabled, _ := repo.GetSlackBotByID(ctx, "bot-1")
	if !enabled.Enabled {
		t.Error("bot should be enabled")
	}
}

// Test Telegram Bot Operations
func TestService_CreateTelegramBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	tests := []struct {
		name    string
		bot     *domain.TelegramBotConfig
		wantErr bool
	}{
		{
			name: "create valid public bot",
			bot: &domain.TelegramBotConfig{
				BotID:               "bot-1",
				BotName:             "Test Bot",
				BotType:             domain.BotTypePublic,
				TelegramBotToken:    "123456:ABC-DEF",
				TelegramBotUsername: "test_bot",
				Enabled:             true,
				AllowedUserIDs:      []string{"user-1"},
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			wantErr: false,
		},
		{
			name: "create valid private bot",
			bot: &domain.TelegramBotConfig{
				BotID:               "bot-2",
				BotName:             "Private Bot",
				BotType:             domain.BotTypePrivate,
				OwnerUserID:         "user-1",
				TelegramBotToken:    "123456:ABC-DEF",
				TelegramBotUsername: "private_bot",
				Enabled:             true,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
			wantErr: false,
		},
		{
			name: "fail on invalid bot",
			bot: &domain.TelegramBotConfig{
				BotID:   "",
				BotName: "Invalid Bot",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CreateTelegramBot(ctx, tt.bot)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTelegramBot() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify bot was saved
				saved, err := repo.GetTelegramBotByID(ctx, tt.bot.BotID)
				if err != nil {
					t.Fatalf("failed to retrieve saved bot: %v", err)
				}
				if saved.BotID != tt.bot.BotID {
					t.Errorf("saved bot ID = %v, want %v", saved.BotID, tt.bot.BotID)
				}
			}
		})
	}
}

func TestService_GetTelegramBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create a bot first
	bot := &domain.TelegramBotConfig{
		BotID:               "bot-1",
		BotName:             "Test Bot",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "test_bot",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	repo.SaveTelegramBot(ctx, bot)

	tests := []struct {
		name    string
		botID   string
		wantErr bool
	}{
		{
			name:    "get existing bot",
			botID:   "bot-1",
			wantErr: false,
		},
		{
			name:    "get non-existent bot",
			botID:   "bot-999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetTelegramBot(ctx, tt.botID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTelegramBot() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("GetTelegramBot() returned nil bot")
			}
		})
	}
}

func TestService_CheckBotAccess(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create private bot
	privateBot := &domain.SlackBotConfig{
		BotID:              "private-bot",
		BotName:            "Private Bot",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "user-1",
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Create public bot
	publicBot := &domain.SlackBotConfig{
		BotID:              "public-bot",
		BotName:            "Public Bot",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-test",
		SlackAppToken:      "xapp-test",
		SlackSigningSecret: "secret",
		Enabled:            true,
		AllowedUserIDs:     []string{"user-1", "user-2"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	repo.SaveSlackBot(ctx, privateBot)
	repo.SaveSlackBot(ctx, publicBot)

	tests := []struct {
		name       string
		botID      string
		userID     string
		wantAccess bool
	}{
		{
			name:       "owner has access to private bot",
			botID:      "private-bot",
			userID:     "user-1",
			wantAccess: true,
		},
		{
			name:       "non-owner denied access to private bot",
			botID:      "private-bot",
			userID:     "user-2",
			wantAccess: false,
		},
		{
			name:       "allowed user has access to public bot",
			botID:      "public-bot",
			userID:     "user-1",
			wantAccess: true,
		},
		{
			name:       "non-allowed user denied access to public bot",
			botID:      "public-bot",
			userID:     "user-3",
			wantAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAccess, err := service.CheckSlackBotAccess(ctx, tt.botID, tt.userID)
			if err != nil {
				t.Fatalf("CheckSlackBotAccess() error = %v", err)
			}
			if hasAccess != tt.wantAccess {
				t.Errorf("CheckSlackBotAccess() = %v, want %v", hasAccess, tt.wantAccess)
			}
		})
	}
}

func TestService_ListTelegramBots(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create test bots
	bot1 := &domain.TelegramBotConfig{
		BotID:               "bot-1",
		BotName:             "Bot 1",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "bot1",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	bot2 := &domain.TelegramBotConfig{
		BotID:               "bot-2",
		BotName:             "Bot 2",
		BotType:             domain.BotTypePrivate,
		OwnerUserID:         "user-1",
		TelegramBotToken:    "123456:GHI-JKL",
		TelegramBotUsername: "bot2",
		Enabled:             false,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	repo.SaveTelegramBot(ctx, bot1)
	repo.SaveTelegramBot(ctx, bot2)

	bots, err := service.ListTelegramBots(ctx)
	if err != nil {
		t.Fatalf("ListTelegramBots() error = %v", err)
	}

	if len(bots) != 2 {
		t.Errorf("ListTelegramBots() returned %d bots, want 2", len(bots))
	}
}

func TestService_UpdateTelegramBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create initial bot
	bot := &domain.TelegramBotConfig{
		BotID:               "bot-1",
		BotName:             "Original Name",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "original_bot",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	repo.SaveTelegramBot(ctx, bot)

	// Update bot
	bot.BotName = "Updated Name"
	bot.Enabled = false

	err := service.UpdateTelegramBot(ctx, bot)
	if err != nil {
		t.Fatalf("UpdateTelegramBot() error = %v", err)
	}

	// Verify update
	updated, err := repo.GetTelegramBotByID(ctx, bot.BotID)
	if err != nil {
		t.Fatalf("failed to retrieve updated bot: %v", err)
	}

	if updated.BotName != "Updated Name" {
		t.Errorf("BotName = %v, want %v", updated.BotName, "Updated Name")
	}

	if updated.Enabled != false {
		t.Errorf("Enabled = %v, want false", updated.Enabled)
	}
}

func TestService_DeleteTelegramBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create bot
	bot := &domain.TelegramBotConfig{
		BotID:               "bot-1",
		BotName:             "Test Bot",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "test_bot",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	repo.SaveTelegramBot(ctx, bot)

	// Delete bot
	err := service.DeleteTelegramBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("DeleteTelegramBot() error = %v", err)
	}

	// Verify deletion
	_, err = repo.GetTelegramBotByID(ctx, "bot-1")
	if err == nil {
		t.Error("expected error when getting deleted bot")
	}
}

func TestService_EnableDisableTelegramBot(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create bot
	bot := &domain.TelegramBotConfig{
		BotID:               "bot-1",
		BotName:             "Test Bot",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "test_bot",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	repo.SaveTelegramBot(ctx, bot)

	// Disable bot
	err := service.DisableTelegramBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("DisableTelegramBot() error = %v", err)
	}

	disabled, _ := repo.GetTelegramBotByID(ctx, "bot-1")
	if disabled.Enabled {
		t.Error("bot should be disabled")
	}

	// Enable bot
	err = service.EnableTelegramBot(ctx, "bot-1")
	if err != nil {
		t.Fatalf("EnableTelegramBot() error = %v", err)
	}

	enabled, _ := repo.GetTelegramBotByID(ctx, "bot-1")
	if !enabled.Enabled {
		t.Error("bot should be enabled")
	}
}

func TestService_CheckTelegramBotAccess(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create private bot
	privateBot := &domain.TelegramBotConfig{
		BotID:               "private-bot",
		BotName:             "Private Bot",
		BotType:             domain.BotTypePrivate,
		OwnerUserID:         "user-1",
		TelegramBotToken:    "123456:ABC-DEF",
		TelegramBotUsername: "private_bot",
		Enabled:             true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Create public bot
	publicBot := &domain.TelegramBotConfig{
		BotID:               "public-bot",
		BotName:             "Public Bot",
		BotType:             domain.BotTypePublic,
		TelegramBotToken:    "123456:GHI-JKL",
		TelegramBotUsername: "public_bot",
		Enabled:             true,
		AllowedUserIDs:      []string{"user-1", "user-2"},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	repo.SaveTelegramBot(ctx, privateBot)
	repo.SaveTelegramBot(ctx, publicBot)

	tests := []struct {
		name       string
		botID      string
		userID     string
		wantAccess bool
	}{
		{
			name:       "owner has access to private bot",
			botID:      "private-bot",
			userID:     "user-1",
			wantAccess: true,
		},
		{
			name:       "non-owner denied access to private bot",
			botID:      "private-bot",
			userID:     "user-2",
			wantAccess: false,
		},
		{
			name:       "allowed user has access to public bot",
			botID:      "public-bot",
			userID:     "user-1",
			wantAccess: true,
		},
		{
			name:       "non-allowed user denied access to public bot",
			botID:      "public-bot",
			userID:     "user-3",
			wantAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasAccess, err := service.CheckTelegramBotAccess(ctx, tt.botID, tt.userID)
			if err != nil {
				t.Fatalf("CheckTelegramBotAccess() error = %v", err)
			}
			if hasAccess != tt.wantAccess {
				t.Errorf("CheckTelegramBotAccess() = %v, want %v", hasAccess, tt.wantAccess)
			}
		})
	}
}

// Test error scenarios
func TestService_ErrorHandling(t *testing.T) {
	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	t.Run("repository save error", func(t *testing.T) {
		repo.saveError = errors.New("save failed")
		defer func() { repo.saveError = nil }()

		bot := &domain.SlackBotConfig{
			BotID:              "bot-1",
			BotName:            "Test Bot",
			BotType:            domain.BotTypePublic,
			SlackBotToken:      "xoxb-test",
			SlackAppToken:      "xapp-test",
			SlackSigningSecret: "secret",
			Enabled:            true,
		}

		err := service.CreateSlackBot(ctx, bot)
		if err == nil {
			t.Error("expected error from repository save failure")
		}
	})

	t.Run("repository get error", func(t *testing.T) {
		repo.getError = errors.New("get failed")
		defer func() { repo.getError = nil }()

		_, err := service.GetSlackBot(ctx, "bot-1")
		if err == nil {
			t.Error("expected error from repository get failure")
		}
	})

	t.Run("repository delete error", func(t *testing.T) {
		repo.deleteError = errors.New("delete failed")
		defer func() { repo.deleteError = nil }()

		err := service.DeleteSlackBot(ctx, "bot-1")
		if err == nil {
			t.Error("expected error from repository delete failure")
		}
	})

	t.Run("enable non-existent bot", func(t *testing.T) {
		err := service.EnableSlackBot(ctx, "non-existent")
		if err == nil {
			t.Error("expected error when enabling non-existent bot")
		}
	})

	t.Run("disable non-existent bot", func(t *testing.T) {
		err := service.DisableSlackBot(ctx, "non-existent")
		if err == nil {
			t.Error("expected error when disabling non-existent bot")
		}
	})
}
