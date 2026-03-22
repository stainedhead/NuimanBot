package cli

import (
	"context"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

func TestAdminBotCommandHandler_TelegramViewBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create a telegram bot first
	bot := domain.NewTelegramBotConfig("tg-1", "My TG Bot", domain.BotTypePublic)
	bot.TelegramBotUsername = "my_tg_bot"
	_ = service.CreateTelegramBot(ctx, bot)

	// View the bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram view tg-1")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "My TG Bot") {
		t.Errorf("Response should contain bot name, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramViewBot_NotFound(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// View non-existent bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram view nonexistent")
	if err == nil {
		t.Logf("Response: %s", response)
		// Service returns error, handler propagates
	}
}

func TestAdminBotCommandHandler_TelegramViewBot_MissingArgs(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// View without bot ID
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram view")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Usage:") {
		t.Errorf("Expected usage message, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramDeleteBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create bot first
	bot := domain.NewTelegramBotConfig("tg-del", "DeleteMe", domain.BotTypePublic)
	_ = service.CreateTelegramBot(ctx, bot)

	// Delete the bot
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram delete tg-del")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "deleted") {
		t.Errorf("Expected 'deleted' in response, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramDeleteBot_MissingArgs(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram delete")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Usage:") {
		t.Errorf("Expected usage message, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramEnableDisableBot(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Create bot first
	bot := domain.NewTelegramBotConfig("tg-toggle", "Toggle Bot", domain.BotTypePublic)
	bot.Enabled = true
	_ = service.CreateTelegramBot(ctx, bot)

	// Disable
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram disable tg-toggle")
	if err != nil {
		t.Fatalf("disable error = %v", err)
	}
	if !strings.Contains(response, "disabled") {
		t.Errorf("Expected 'disabled' in response, got: %v", response)
	}

	// Enable
	response, err = handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram enable tg-toggle")
	if err != nil {
		t.Fatalf("enable error = %v", err)
	}
	if !strings.Contains(response, "enabled") {
		t.Errorf("Expected 'enabled' in response, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramEnableBot_MissingArgs(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram enable")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Usage:") {
		t.Errorf("Expected usage message, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramDisableBot_MissingArgs(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram disable")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Usage:") {
		t.Errorf("Expected usage message, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramHelp(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram help")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Telegram Bot Management") {
		t.Errorf("Expected Telegram help info, got: %v", response)
	}
}

func TestAdminBotCommandHandler_GeneralHelp(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// General help
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot help")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Bot Management") {
		t.Errorf("Expected Bot Management help info, got: %v", response)
	}

	// Short command - triggers help
	response, err = handler.HandleBotCommand(ctx, adminUser, "/admin bot")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Bot Management") {
		t.Errorf("Expected help message for short command, got: %v", response)
	}
}

func TestAdminBotCommandHandler_UnknownPlatform(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot discord list")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Unknown platform") {
		t.Errorf("Expected 'Unknown platform' message, got: %v", response)
	}
}

func TestAdminBotCommandHandler_SlackNoSubcommand(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Slack with no subcommand
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Slack Bot Management") {
		t.Errorf("Expected Slack help, got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramNoSubcommand(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	// Telegram with no subcommand
	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Telegram Bot Management") {
		t.Errorf("Expected Telegram help, got: %v", response)
	}
}

func TestAdminBotCommandHandler_SlackUnknownSubcommand(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot slack frobnicate")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Unknown Slack command") {
		t.Errorf("Expected 'Unknown Slack command', got: %v", response)
	}
}

func TestAdminBotCommandHandler_TelegramUnknownSubcommand(t *testing.T) {
	service := newMockBotManagementService()
	handler := NewAdminBotCommandHandler(service)
	ctx := context.Background()
	adminUser := &domain.User{ID: "admin1", Role: domain.RoleAdmin}

	response, err := handler.HandleBotCommand(ctx, adminUser, "/admin bot telegram frobnicate")
	if err != nil {
		t.Fatalf("HandleBotCommand() error = %v", err)
	}
	if !strings.Contains(response, "Unknown Telegram command") {
		t.Errorf("Expected 'Unknown Telegram command', got: %v", response)
	}
}
