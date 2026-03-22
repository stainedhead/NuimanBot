package domain

import (
	"testing"
)

func TestNewSlackBotConfig(t *testing.T) {
	bot := NewSlackBotConfig("bot-1", "Test Bot", BotTypePublic)

	if bot == nil {
		t.Fatal("NewSlackBotConfig returned nil")
	}
	if bot.BotID != "bot-1" {
		t.Errorf("BotID = %q, want %q", bot.BotID, "bot-1")
	}
	if bot.BotName != "Test Bot" {
		t.Errorf("BotName = %q, want %q", bot.BotName, "Test Bot")
	}
	if bot.BotType != BotTypePublic {
		t.Errorf("BotType = %q, want %q", bot.BotType, BotTypePublic)
	}
	if !bot.Enabled {
		t.Error("Enabled should default to true")
	}
	if bot.AllowedUserIDs == nil {
		t.Error("AllowedUserIDs should be initialized (not nil)")
	}
	if bot.Metadata == nil {
		t.Error("Metadata should be initialized (not nil)")
	}
	if bot.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if bot.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestNewTelegramBotConfig(t *testing.T) {
	bot := NewTelegramBotConfig("tg-bot-1", "TG Bot", BotTypePrivate)

	if bot == nil {
		t.Fatal("NewTelegramBotConfig returned nil")
	}
	if bot.BotID != "tg-bot-1" {
		t.Errorf("BotID = %q, want %q", bot.BotID, "tg-bot-1")
	}
	if bot.BotName != "TG Bot" {
		t.Errorf("BotName = %q, want %q", bot.BotName, "TG Bot")
	}
	if bot.BotType != BotTypePrivate {
		t.Errorf("BotType = %q, want %q", bot.BotType, BotTypePrivate)
	}
	if !bot.Enabled {
		t.Error("Enabled should default to true")
	}
	if bot.AllowedUserIDs == nil {
		t.Error("AllowedUserIDs should be initialized (not nil)")
	}
	if bot.AllowedChatIDs == nil {
		t.Error("AllowedChatIDs should be initialized (not nil)")
	}
	if bot.Metadata == nil {
		t.Error("Metadata should be initialized (not nil)")
	}
	if bot.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if bot.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestTelegramBotConfig_IsOwnedByUser(t *testing.T) {
	privateBot := &TelegramBotConfig{
		BotType:     BotTypePrivate,
		OwnerUserID: "user-123",
	}

	publicBot := &TelegramBotConfig{
		BotType:        BotTypePublic,
		AllowedUserIDs: []string{"user-123", "user-456"},
	}

	if !privateBot.IsOwnedByUser("user-123") {
		t.Error("expected private bot to be owned by user-123")
	}
	if privateBot.IsOwnedByUser("user-456") {
		t.Error("expected private bot NOT to be owned by user-456")
	}
	if !publicBot.IsOwnedByUser("user-123") {
		t.Error("expected public bot to allow user-123")
	}
	if publicBot.IsOwnedByUser("user-789") {
		t.Error("expected public bot NOT to allow user-789")
	}
}

func TestSlackBotConfig_Validate_PublicWithOwner(t *testing.T) {
	bot := &SlackBotConfig{
		BotID:         "bot-1",
		BotName:       "Test Bot",
		BotType:       BotTypePublic,
		OwnerUserID:   "user-1", // Should not be set for public bots
		SlackBotToken: "xoxb-test",
	}
	if err := bot.Validate(); err == nil {
		t.Error("expected error for public bot with OwnerUserID set")
	}
}

func TestTelegramBotConfig_Validate_PublicWithOwner(t *testing.T) {
	bot := &TelegramBotConfig{
		BotID:            "bot-1",
		BotName:          "Test Bot",
		BotType:          BotTypePublic,
		OwnerUserID:      "user-1", // Should not be set for public bots
		TelegramBotToken: "123456:ABC",
	}
	if err := bot.Validate(); err == nil {
		t.Error("expected error for public bot with OwnerUserID set")
	}
}

func TestTelegramBotConfig_Validate_BotIDTooLong(t *testing.T) {
	bot := &TelegramBotConfig{
		BotID:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 65 chars
		BotName:          "Test Bot",
		BotType:          BotTypePublic,
		TelegramBotToken: "123456:ABC",
	}
	if err := bot.Validate(); err == nil {
		t.Error("expected error for botID > 64 chars")
	}
}

func TestTelegramBotConfig_Validate_BotNameTooLong(t *testing.T) {
	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}
	bot := &TelegramBotConfig{
		BotID:            "bot-1",
		BotName:          string(longName),
		BotType:          BotTypePublic,
		TelegramBotToken: "123456:ABC",
	}
	if err := bot.Validate(); err == nil {
		t.Error("expected error for botName > 100 chars")
	}
}
