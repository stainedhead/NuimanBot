package cli

import (
	"context"
	"errors"
	"nuimanbot/internal/domain"
	"strings"
	"testing"
)

// Mock BotManagementService for testing
type mockBotManagementService struct {
	slackBots    map[string]*domain.SlackBotConfig
	telegramBots map[string]*domain.TelegramBotConfig
	createError  error
	getError     error
	updateError  error
	deleteError  error
}

func newMockBotManagementService() *mockBotManagementService {
	return &mockBotManagementService{
		slackBots:    make(map[string]*domain.SlackBotConfig),
		telegramBots: make(map[string]*domain.TelegramBotConfig),
	}
}

func (m *mockBotManagementService) CreateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	if m.createError != nil {
		return m.createError
	}
	m.slackBots[bot.BotID] = bot
	return nil
}

func (m *mockBotManagementService) GetSlackBot(ctx context.Context, botID string) (*domain.SlackBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bot, exists := m.slackBots[botID]
	if !exists {
		return nil, errors.New("bot not found")
	}
	return bot, nil
}

func (m *mockBotManagementService) ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error) {
	bots := make([]*domain.SlackBotConfig, 0, len(m.slackBots))
	for _, bot := range m.slackBots {
		bots = append(bots, bot)
	}
	return bots, nil
}

func (m *mockBotManagementService) UpdateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error {
	if m.updateError != nil {
		return m.updateError
	}
	m.slackBots[bot.BotID] = bot
	return nil
}

func (m *mockBotManagementService) DeleteSlackBot(ctx context.Context, botID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.slackBots, botID)
	return nil
}

func (m *mockBotManagementService) EnableSlackBot(ctx context.Context, botID string) error {
	bot, exists := m.slackBots[botID]
	if !exists {
		return errors.New("bot not found")
	}
	bot.Enabled = true
	return nil
}

func (m *mockBotManagementService) DisableSlackBot(ctx context.Context, botID string) error {
	bot, exists := m.slackBots[botID]
	if !exists {
		return errors.New("bot not found")
	}
	bot.Enabled = false
	return nil
}

// Telegram methods
func (m *mockBotManagementService) CreateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	if m.createError != nil {
		return m.createError
	}
	m.telegramBots[bot.BotID] = bot
	return nil
}

func (m *mockBotManagementService) GetTelegramBot(ctx context.Context, botID string) (*domain.TelegramBotConfig, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	bot, exists := m.telegramBots[botID]
	if !exists {
		return nil, errors.New("bot not found")
	}
	return bot, nil
}

func (m *mockBotManagementService) ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error) {
	bots := make([]*domain.TelegramBotConfig, 0, len(m.telegramBots))
	for _, bot := range m.telegramBots {
		bots = append(bots, bot)
	}
	return bots, nil
}

func (m *mockBotManagementService) UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error {
	if m.updateError != nil {
		return m.updateError
	}
	m.telegramBots[bot.BotID] = bot
	return nil
}

func (m *mockBotManagementService) DeleteTelegramBot(ctx context.Context, botID string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.telegramBots, botID)
	return nil
}

func (m *mockBotManagementService) EnableTelegramBot(ctx context.Context, botID string) error {
	bot, exists := m.telegramBots[botID]
	if !exists {
		return errors.New("bot not found")
	}
	bot.Enabled = true
	return nil
}

func (m *mockBotManagementService) DisableTelegramBot(ctx context.Context, botID string) error {
	bot, exists := m.telegramBots[botID]
	if !exists {
		return errors.New("bot not found")
	}
	bot.Enabled = false
	return nil
}

func TestIsBotCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "slack bot command",
			input: "/admin bot slack list",
			want:  true,
		},
		{
			name:  "telegram bot command",
			input: "/admin bot telegram create",
			want:  true,
		},
		{
			name:  "not a bot command",
			input: "/admin profile list",
			want:  false,
		},
		{
			name:  "incomplete command",
			input: "/admin bot ",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBotCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsBotCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdminBotCommandHandler_CreateSlackBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantContain string
	}{
		{
			name:        "create slack bot successfully",
			input:       "/admin bot slack create bot-1 --name TestBot --type public --bot-token xoxb-test --app-token xapp-test",
			wantErr:     false,
			wantContain: "Slack bot created successfully",
		},
		{
			name:        "missing required arguments",
			input:       "/admin bot slack create",
			wantErr:     false,
			wantContain: "Usage:",
		},
		{
			name:        "invalid bot type",
			input:       "/admin bot slack create bot-2 --name Test --type invalid --bot-token xoxb --app-token xapp",
			wantErr:     false,
			wantContain: "Invalid bot type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := handler.HandleBotCommand(ctx, adminUser, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleBotCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantContain != "" && !strings.Contains(response, tt.wantContain) {
				t.Errorf("HandleBotCommand() response = %v, want to contain %v", response, tt.wantContain)
			}
		})
	}
}

func TestAdminBotCommandHandler_ListSlackBots(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create test bot
	bot := domain.NewSlackBotConfig("bot-1", "Test Bot", domain.BotTypePublic)
	service.CreateSlackBot(ctx, bot)

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack list")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}

	if !strings.Contains(response, "bot-1") {
		t.Errorf("HandleBotCommand() response should contain bot-1, got: %v", response)
	}
}

func TestAdminBotCommandHandler_ViewSlackBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create test bot
	bot := domain.NewSlackBotConfig("bot-1", "Test Bot", domain.BotTypePublic)
	service.CreateSlackBot(ctx, bot)

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack view bot-1")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}

	if !strings.Contains(response, "Test Bot") {
		t.Errorf("HandleBotCommand() response should contain bot name, got: %v", response)
	}
}

func TestAdminBotCommandHandler_EnableDisableSlackBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create test bot
	bot := domain.NewSlackBotConfig("bot-1", "Test Bot", domain.BotTypePublic)
	bot.Enabled = true
	service.CreateSlackBot(ctx, bot)

	// Disable bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack disable bot-1")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "disabled") {
		t.Errorf("HandleBotCommand() response should mention disabled, got: %v", response)
	}

	// Verify bot is disabled
	disabledBot, _ := service.GetSlackBot(ctx, "bot-1")
	if disabledBot.Enabled {
		t.Error("bot should be disabled")
	}

	// Enable bot
	response, err = handler.HandleBotCommand(ctx, adminUser, "/admin bot slack enable bot-1")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "enabled") {
		t.Errorf("HandleBotCommand() response should mention enabled, got: %v", response)
	}

	// Verify bot is enabled
	enabledBot, _ := service.GetSlackBot(ctx, "bot-1")
	if !enabledBot.Enabled {
		t.Error("bot should be enabled")
	}
}

func TestAdminBotCommandHandler_DeleteSlackBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create test bot
	bot := domain.NewSlackBotConfig("bot-1", "Test Bot", domain.BotTypePublic)
	service.CreateSlackBot(ctx, bot)

	// Delete bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack delete bot-1")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "deleted") {
		t.Errorf("HandleBotCommand() response should mention deleted, got: %v", response)
	}

	// Verify bot is deleted
	_, err = service.GetSlackBot(ctx, "bot-1")
	if err == nil {
		t.Error("expected error when getting deleted bot")
	}
}

func TestAdminBotCommandHandler_TelegramBots(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create Telegram bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram create tg-bot-1 --name TelegramBot --type public --token 123456:ABC-DEF")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Telegram bot created successfully") {
		t.Errorf("HandleBotCommand() response should indicate success, got: %v", response)
	}

	// List Telegram bots
	response, err = handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram list")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "tg-bot-1") {
		t.Errorf("HandleBotCommand() response should contain bot ID, got: %v", response)
	}
}

func TestAdminBotCommandHandler_NonAdminUser(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	regularUser := &domain.User{ID: "user1", Role: domain.RoleUser}

	_, err := handler.HandleBotCommand(ctx, regularUser, "/admin bot slack list")
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("HandleBotCommand() error = %v, want %v", err, domain.ErrInsufficientPermissions)
	}
}

func TestAdminBotCommandHandler_Help(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack help")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}

	if !strings.Contains(response, "Slack Bot Management") {
		t.Errorf("HandleBotCommand() help should contain command info, got: %v", response)
	}
}
