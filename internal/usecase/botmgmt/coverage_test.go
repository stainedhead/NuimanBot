package botmgmt

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
)

// TestListSlackBotsByOwner covers the 0.0% function
func TestListSlackBotsByOwner(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Populate with bots owned by different users
	bot1 := &domain.SlackBotConfig{
		BotID:              "bot-1",
		BotName:            "Bot 1",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "user-alice",
		SlackBotToken:      "xoxb-1",
		SlackAppToken:      "xapp-1",
		SlackSigningSecret: "secret1",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	bot2 := &domain.SlackBotConfig{
		BotID:              "bot-2",
		BotName:            "Bot 2",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "user-bob",
		SlackBotToken:      "xoxb-2",
		SlackAppToken:      "xapp-2",
		SlackSigningSecret: "secret2",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	bot3 := &domain.SlackBotConfig{
		BotID:              "bot-3",
		BotName:            "Bot 3",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "user-alice",
		SlackBotToken:      "xoxb-3",
		SlackAppToken:      "xapp-3",
		SlackSigningSecret: "secret3",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	_ = repo.SaveSlackBot(ctx, bot1)
	_ = repo.SaveSlackBot(ctx, bot2)
	_ = repo.SaveSlackBot(ctx, bot3)

	t.Run("returns_only_owner_bots", func(t *testing.T) {
		bots, err := service.ListSlackBotsByOwner(ctx, "user-alice")
		if err != nil {
			t.Fatalf("ListSlackBotsByOwner failed: %v", err)
		}
		if len(bots) != 2 {
			t.Errorf("Expected 2 bots for user-alice, got %d", len(bots))
		}
	})

	t.Run("returns_empty_for_unknown_owner", func(t *testing.T) {
		bots, err := service.ListSlackBotsByOwner(ctx, "user-charlie")
		if err != nil {
			t.Fatalf("ListSlackBotsByOwner failed: %v", err)
		}
		if len(bots) != 0 {
			t.Errorf("Expected 0 bots for unknown owner, got %d", len(bots))
		}
	})

	t.Run("repo_error_propagated", func(t *testing.T) {
		repo.getError = errors.New("database error")
		defer func() { repo.getError = nil }()

		_, err := service.ListSlackBotsByOwner(ctx, "user-alice")
		if err == nil {
			t.Error("Expected error when repository fails")
		}
	})
}

// TestListTelegramBotsByOwner covers the 0.0% function
func TestListTelegramBotsByOwner(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot1 := &domain.TelegramBotConfig{
		BotID:            "tbot-1",
		BotName:          "Telegram Bot 1",
		BotType:          domain.BotTypePrivate,
		OwnerUserID:      "user-alice",
		TelegramBotToken: "token1",
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	bot2 := &domain.TelegramBotConfig{
		BotID:            "tbot-2",
		BotName:          "Telegram Bot 2",
		BotType:          domain.BotTypePrivate,
		OwnerUserID:      "user-bob",
		TelegramBotToken: "token2",
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	_ = repo.SaveTelegramBot(ctx, bot1)
	_ = repo.SaveTelegramBot(ctx, bot2)

	t.Run("returns_only_owner_bots", func(t *testing.T) {
		bots, err := service.ListTelegramBotsByOwner(ctx, "user-alice")
		if err != nil {
			t.Fatalf("ListTelegramBotsByOwner failed: %v", err)
		}
		if len(bots) != 1 {
			t.Errorf("Expected 1 bot for user-alice, got %d", len(bots))
		}
	})

	t.Run("returns_empty_for_unknown_owner", func(t *testing.T) {
		bots, err := service.ListTelegramBotsByOwner(ctx, "user-charlie")
		if err != nil {
			t.Fatalf("ListTelegramBotsByOwner failed: %v", err)
		}
		if len(bots) != 0 {
			t.Errorf("Expected 0 bots for unknown owner, got %d", len(bots))
		}
	})

	t.Run("repo_error_propagated", func(t *testing.T) {
		repo.getError = errors.New("database error")
		defer func() { repo.getError = nil }()

		_, err := service.ListTelegramBotsByOwner(ctx, "user-alice")
		if err == nil {
			t.Error("Expected error when repository fails")
		}
	})
}

// TestSetSlackBotEnabled_SaveError covers uncovered branch
func TestSetSlackBotEnabled_SaveError(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot := &domain.SlackBotConfig{
		BotID:              "bot-save-err",
		BotName:            "Bot Save Error",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-1",
		SlackAppToken:      "xapp-1",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_ = repo.SaveSlackBot(ctx, bot)

	// Make the next save fail
	repo.saveError = errors.New("save error")
	defer func() { repo.saveError = nil }()

	err := service.EnableSlackBot(ctx, "bot-save-err")
	if err == nil {
		t.Error("Expected error when save fails during EnableSlackBot")
	}
}

// TestSetTelegramBotEnabled_SaveError covers uncovered branch
func TestSetTelegramBotEnabled_SaveError(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot := &domain.TelegramBotConfig{
		BotID:            "tbot-save-err",
		BotName:          "Telegram Bot Save Error",
		BotType:          domain.BotTypePublic,
		TelegramBotToken: "token",
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	_ = repo.SaveTelegramBot(ctx, bot)

	// Make the next save fail
	repo.saveError = errors.New("save error")
	defer func() { repo.saveError = nil }()

	err := service.EnableTelegramBot(ctx, "tbot-save-err")
	if err == nil {
		t.Error("Expected error when save fails during EnableTelegramBot")
	}
}

// TestUpdateSlackBot_SaveError covers the error path in UpdateSlackBot
func TestUpdateSlackBot_SaveError(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot := &domain.SlackBotConfig{
		BotID:              "bot-upd-err",
		BotName:            "Bot Update Error",
		BotType:            domain.BotTypePublic,
		SlackBotToken:      "xoxb-1",
		SlackAppToken:      "xapp-1",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_ = repo.SaveSlackBot(ctx, bot)

	repo.saveError = errors.New("save error")
	defer func() { repo.saveError = nil }()

	err := service.UpdateSlackBot(ctx, bot)
	if err == nil {
		t.Error("Expected error when save fails during UpdateSlackBot")
	}
}

// TestUpdateTelegramBot_Paths covers branches in UpdateTelegramBot
func TestUpdateTelegramBot_Paths(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	t.Run("invalid_bot_returns_error", func(t *testing.T) {
		invalidBot := &domain.TelegramBotConfig{
			BotID:   "", // invalid: no BotID
			BotName: "Invalid",
		}
		err := service.UpdateTelegramBot(ctx, invalidBot)
		if err == nil {
			t.Error("Expected error for invalid bot in UpdateTelegramBot")
		}
	})

	t.Run("save_error_returns_error", func(t *testing.T) {
		bot := &domain.TelegramBotConfig{
			BotID:            "tbot-upd",
			BotName:          "Telegram Bot Update",
			BotType:          domain.BotTypePublic,
			TelegramBotToken: "token",
			Enabled:          true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		_ = repo.SaveTelegramBot(ctx, bot)

		repo.saveError = errors.New("save error")
		defer func() { repo.saveError = nil }()

		err := service.UpdateTelegramBot(ctx, bot)
		if err == nil {
			t.Error("Expected error when save fails during UpdateTelegramBot")
		}
	})
}

// TestDeleteTelegramBot_ErrorPath covers the error path in DeleteTelegramBot
func TestDeleteTelegramBot_ErrorPath(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	repo.deleteError = errors.New("delete error")
	defer func() { repo.deleteError = nil }()

	err := service.DeleteTelegramBot(ctx, "some-bot-id")
	if err == nil {
		t.Error("Expected error when delete fails")
	}
}

// TestCheckSlackBotAccess_Paths covers branches in CheckSlackBotAccess
func TestCheckSlackBotAccess_Paths(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot := &domain.SlackBotConfig{
		BotID:              "bot-access",
		BotName:            "Access Bot",
		BotType:            domain.BotTypePrivate,
		OwnerUserID:        "owner-user",
		SlackBotToken:      "xoxb-1",
		SlackAppToken:      "xapp-1",
		SlackSigningSecret: "secret",
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_ = repo.SaveSlackBot(ctx, bot)

	t.Run("repo_error_returns_error", func(t *testing.T) {
		repo.getError = errors.New("get error")
		defer func() { repo.getError = nil }()

		_, err := service.CheckSlackBotAccess(ctx, "bot-access", "owner-user")
		if err == nil {
			t.Error("Expected error when repository fails")
		}
	})
}

// TestCheckTelegramBotAccess_Paths covers branches in CheckTelegramBotAccess
func TestCheckTelegramBotAccess_Paths(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	bot := &domain.TelegramBotConfig{
		BotID:            "tbot-access",
		BotName:          "Telegram Access Bot",
		BotType:          domain.BotTypePrivate,
		OwnerUserID:      "telegram-owner",
		TelegramBotToken: "token",
		Enabled:          true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	_ = repo.SaveTelegramBot(ctx, bot)

	t.Run("owner_has_access", func(t *testing.T) {
		hasAccess, err := service.CheckTelegramBotAccess(ctx, "tbot-access", "telegram-owner")
		if err != nil {
			t.Fatalf("CheckTelegramBotAccess failed: %v", err)
		}
		if !hasAccess {
			t.Error("Expected owner to have access")
		}
	})

	t.Run("non_owner_has_no_access", func(t *testing.T) {
		hasAccess, err := service.CheckTelegramBotAccess(ctx, "tbot-access", "other-user")
		if err != nil {
			t.Fatalf("CheckTelegramBotAccess failed: %v", err)
		}
		if hasAccess {
			t.Error("Expected non-owner to not have access")
		}
	})

	t.Run("repo_error_returns_error", func(t *testing.T) {
		repo.getError = errors.New("get error")
		defer func() { repo.getError = nil }()

		_, err := service.CheckTelegramBotAccess(ctx, "tbot-access", "telegram-owner")
		if err == nil {
			t.Error("Expected error when repository fails")
		}
	})
}

// TestListTelegramBots_Error covers error path
func TestListTelegramBots_Error(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	repo.getError = errors.New("list error")
	defer func() { repo.getError = nil }()

	_, err := service.ListTelegramBots(ctx)
	if err == nil {
		t.Error("Expected error when listing telegram bots fails")
	}
}

// TestCreateTelegramBot_Paths covers paths in CreateTelegramBot
func TestCreateTelegramBot_Paths(t *testing.T) {
	t.Parallel()

	repo := newMockBotConfigRepository()
	service := NewService(repo)
	ctx := context.Background()

	t.Run("invalid_bot_fails", func(t *testing.T) {
		bot := &domain.TelegramBotConfig{
			BotID: "", // invalid
		}
		err := service.CreateTelegramBot(ctx, bot)
		if err == nil {
			t.Error("Expected error for invalid bot")
		}
	})

	t.Run("save_error_propagated", func(t *testing.T) {
		bot := &domain.TelegramBotConfig{
			BotID:            "tbot-create-err",
			BotName:          "Create Error Bot",
			BotType:          domain.BotTypePublic,
			TelegramBotToken: "token",
			Enabled:          true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		repo.saveError = errors.New("save error")
		defer func() { repo.saveError = nil }()

		err := service.CreateTelegramBot(ctx, bot)
		if err == nil {
			t.Error("Expected error when save fails")
		}
	})
}
