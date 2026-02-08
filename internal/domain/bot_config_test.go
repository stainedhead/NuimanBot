package domain

import (
	"strings"
	"testing"
)

func TestSlackBotConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		bot     *SlackBotConfig
		wantErr bool
	}{
		{
			name: "valid public bot",
			bot: &SlackBotConfig{
				BotID:              "bot-123",
				BotName:            "Team Assistant",
				BotType:            BotTypePublic,
				SlackBotToken:      "xoxb-test-token",
				SlackAppToken:      "xapp-test-token",
				SlackSigningSecret: "signing-secret",
				Enabled:            true,
				AllowedUserIDs:     []string{"user-1", "user-2"},
			},
			wantErr: false,
		},
		{
			name: "valid private bot",
			bot: &SlackBotConfig{
				BotID:              "bot-123",
				BotName:            "Personal Bot",
				BotType:            BotTypePrivate,
				OwnerUserID:        "user-123",
				SlackBotToken:      "xoxb-test-token",
				SlackAppToken:      "xapp-test-token",
				SlackSigningSecret: "signing-secret",
				Enabled:            true,
			},
			wantErr: false,
		},
		{
			name: "missing bot ID",
			bot: &SlackBotConfig{
				BotName:       "Test Bot",
				BotType:       BotTypePublic,
				SlackBotToken: "xoxb-test-token",
			},
			wantErr: true,
		},
		{
			name: "missing bot token",
			bot: &SlackBotConfig{
				BotID:   "bot-123",
				BotName: "Test Bot",
				BotType: BotTypePublic,
			},
			wantErr: true,
		},
		{
			name: "name too long",
			bot: &SlackBotConfig{
				BotID:         "bot-123",
				BotName:       strings.Repeat("a", 101),
				BotType:       BotTypePublic,
				SlackBotToken: "xoxb-test-token",
			},
			wantErr: true,
		},
		{
			name: "private bot without owner",
			bot: &SlackBotConfig{
				BotID:         "bot-123",
				BotName:       "Test Bot",
				BotType:       BotTypePrivate,
				SlackBotToken: "xoxb-test-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bot.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SlackBotConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTelegramBotConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		bot     *TelegramBotConfig
		wantErr bool
	}{
		{
			name: "valid public bot",
			bot: &TelegramBotConfig{
				BotID:               "bot-123",
				BotName:             "Team Assistant",
				BotType:             BotTypePublic,
				TelegramBotToken:    "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				TelegramBotUsername: "team_assistant_bot",
				Enabled:             true,
				AllowedUserIDs:      []string{"user-1", "user-2"},
			},
			wantErr: false,
		},
		{
			name: "valid private bot",
			bot: &TelegramBotConfig{
				BotID:               "bot-123",
				BotName:             "Personal Bot",
				BotType:             BotTypePrivate,
				OwnerUserID:         "user-123",
				TelegramBotToken:    "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				TelegramBotUsername: "personal_bot",
				Enabled:             true,
			},
			wantErr: false,
		},
		{
			name: "missing bot ID",
			bot: &TelegramBotConfig{
				BotName:          "Test Bot",
				BotType:          BotTypePublic,
				TelegramBotToken: "123456:ABC-DEF",
			},
			wantErr: true,
		},
		{
			name: "missing bot token",
			bot: &TelegramBotConfig{
				BotID:   "bot-123",
				BotName: "Test Bot",
				BotType: BotTypePublic,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bot.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("TelegramBotConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBotConfig_IsOwnedByUser(t *testing.T) {
	privateBot := &SlackBotConfig{
		BotType:     BotTypePrivate,
		OwnerUserID: "user-123",
	}

	publicBot := &SlackBotConfig{
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
