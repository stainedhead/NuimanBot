package storage

import (
	"bytes"
	"context"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/security"
	"os"
	"path/filepath"
	"testing"
)

const testEncryptionKey = "12345678901234567890123456789012" // 32 bytes

func TestFileBotConfigRepository_SlackBots(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bot-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	filePath := filepath.Join(tmpDir, "bots.json")
	encryption := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, encryption)

	ctx := context.Background()

	t.Run("create and get slack bot", func(t *testing.T) {
		bot := domain.NewSlackBotConfig("bot-1", "Test Bot", domain.BotTypePublic)
		bot.SlackBotToken = "xoxb-test-token"
		bot.SlackAppToken = "xapp-test-token"
		bot.SlackSigningSecret = "test-secret"
		bot.AllowedUserIDs = []string{"user-1", "user-2"}

		// Save bot
		err := repo.SaveSlackBot(ctx, bot)
		if err != nil {
			t.Fatalf("SaveSlackBot() error = %v", err)
		}

		// Retrieve bot
		retrieved, err := repo.GetSlackBotByID(ctx, "bot-1")
		if err != nil {
			t.Fatalf("GetSlackBotByID() error = %v", err)
		}

		// Verify fields
		if retrieved.BotID != bot.BotID {
			t.Errorf("BotID = %v, want %v", retrieved.BotID, bot.BotID)
		}
		if retrieved.BotName != bot.BotName {
			t.Errorf("BotName = %v, want %v", retrieved.BotName, bot.BotName)
		}
		if retrieved.SlackBotToken != bot.SlackBotToken {
			t.Errorf("SlackBotToken = %v, want %v", retrieved.SlackBotToken, bot.SlackBotToken)
		}
		if len(retrieved.AllowedUserIDs) != 2 {
			t.Errorf("AllowedUserIDs length = %v, want 2", len(retrieved.AllowedUserIDs))
		}
	})

	t.Run("list slack bots", func(t *testing.T) {
		// Create additional bot
		bot2 := domain.NewSlackBotConfig("bot-2", "Second Bot", domain.BotTypePrivate)
		bot2.OwnerUserID = "user-1"
		bot2.SlackBotToken = "xoxb-token-2"
		bot2.SlackAppToken = "xapp-token-2"
		bot2.SlackSigningSecret = "secret-2"

		err := repo.SaveSlackBot(ctx, bot2)
		if err != nil {
			t.Fatalf("SaveSlackBot() error = %v", err)
		}

		// List all bots
		bots, err := repo.ListSlackBots(ctx)
		if err != nil {
			t.Fatalf("ListSlackBots() error = %v", err)
		}

		if len(bots) != 2 {
			t.Errorf("ListSlackBots() returned %d bots, want 2", len(bots))
		}
	})

	t.Run("update slack bot", func(t *testing.T) {
		// Get existing bot
		bot, err := repo.GetSlackBotByID(ctx, "bot-1")
		if err != nil {
			t.Fatalf("GetSlackBotByID() error = %v", err)
		}

		// Update fields
		bot.BotName = "Updated Name"
		bot.Enabled = false

		// Save update
		err = repo.SaveSlackBot(ctx, bot)
		if err != nil {
			t.Fatalf("SaveSlackBot() error = %v", err)
		}

		// Retrieve and verify
		updated, err := repo.GetSlackBotByID(ctx, "bot-1")
		if err != nil {
			t.Fatalf("GetSlackBotByID() error = %v", err)
		}

		if updated.BotName != "Updated Name" {
			t.Errorf("BotName = %v, want 'Updated Name'", updated.BotName)
		}
		if updated.Enabled != false {
			t.Errorf("Enabled = %v, want false", updated.Enabled)
		}
	})

	t.Run("delete slack bot", func(t *testing.T) {
		// Delete bot
		err := repo.DeleteSlackBot(ctx, "bot-2")
		if err != nil {
			t.Fatalf("DeleteSlackBot() error = %v", err)
		}

		// Verify deletion
		_, err = repo.GetSlackBotByID(ctx, "bot-2")
		if err == nil {
			t.Error("expected error when getting deleted bot")
		}

		// Verify remaining bots
		bots, err := repo.ListSlackBots(ctx)
		if err != nil {
			t.Fatalf("ListSlackBots() error = %v", err)
		}

		if len(bots) != 1 {
			t.Errorf("ListSlackBots() returned %d bots, want 1", len(bots))
		}
	})
}

func TestFileBotConfigRepository_TelegramBots(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bot-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	filePath := filepath.Join(tmpDir, "bots.json")
	encryption := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, encryption)

	ctx := context.Background()

	t.Run("create and get telegram bot", func(t *testing.T) {
		bot := domain.NewTelegramBotConfig("tg-bot-1", "Telegram Bot", domain.BotTypePublic)
		bot.TelegramBotToken = "123456:ABC-DEF1234"
		bot.TelegramBotUsername = "test_bot"
		bot.TelegramBotID = "123456"
		bot.AllowedUserIDs = []string{"user-1"}

		// Save bot
		err := repo.SaveTelegramBot(ctx, bot)
		if err != nil {
			t.Fatalf("SaveTelegramBot() error = %v", err)
		}

		// Retrieve bot
		retrieved, err := repo.GetTelegramBotByID(ctx, "tg-bot-1")
		if err != nil {
			t.Fatalf("GetTelegramBotByID() error = %v", err)
		}

		// Verify fields
		if retrieved.BotID != bot.BotID {
			t.Errorf("BotID = %v, want %v", retrieved.BotID, bot.BotID)
		}
		if retrieved.TelegramBotToken != bot.TelegramBotToken {
			t.Errorf("TelegramBotToken = %v, want %v", retrieved.TelegramBotToken, bot.TelegramBotToken)
		}
	})

	t.Run("list telegram bots", func(t *testing.T) {
		// Create another bot
		bot2 := domain.NewTelegramBotConfig("tg-bot-2", "Second Telegram Bot", domain.BotTypePrivate)
		bot2.OwnerUserID = "user-1"
		bot2.TelegramBotToken = "789012:XYZ-GHI5678"
		bot2.TelegramBotUsername = "second_bot"

		err := repo.SaveTelegramBot(ctx, bot2)
		if err != nil {
			t.Fatalf("SaveTelegramBot() error = %v", err)
		}

		// List all bots
		bots, err := repo.ListTelegramBots(ctx)
		if err != nil {
			t.Fatalf("ListTelegramBots() error = %v", err)
		}

		if len(bots) != 2 {
			t.Errorf("ListTelegramBots() returned %d bots, want 2", len(bots))
		}
	})
}

func TestFileBotConfigRepository_TokenEncryption(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bot-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	filePath := filepath.Join(tmpDir, "bots.json")
	encryption := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, encryption)

	ctx := context.Background()

	bot := domain.NewSlackBotConfig("bot-encrypt-test", "Encryption Test", domain.BotTypePublic)
	bot.SlackBotToken = "xoxb-secret-token"
	bot.SlackAppToken = "xapp-secret-token"
	bot.SlackSigningSecret = "very-secret"

	// Save bot
	err = repo.SaveSlackBot(ctx, bot)
	if err != nil {
		t.Fatalf("SaveSlackBot() error = %v", err)
	}

	// Read raw file to verify encryption
	rawData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read bots file: %v", err)
	}

	rawContent := string(rawData)

	// Verify tokens are not in plaintext
	if containsPlaintext(rawContent, "xoxb-secret-token") {
		t.Error("SlackBotToken found in plaintext in file")
	}
	if containsPlaintext(rawContent, "xapp-secret-token") {
		t.Error("SlackAppToken found in plaintext in file")
	}
	if containsPlaintext(rawContent, "very-secret") {
		t.Error("SlackSigningSecret found in plaintext in file")
	}

	// Retrieve bot and verify tokens are decrypted correctly
	retrieved, err := repo.GetSlackBotByID(ctx, "bot-encrypt-test")
	if err != nil {
		t.Fatalf("GetSlackBotByID() error = %v", err)
	}

	if retrieved.SlackBotToken != "xoxb-secret-token" {
		t.Errorf("SlackBotToken = %v, want 'xoxb-secret-token'", retrieved.SlackBotToken)
	}
	if retrieved.SlackAppToken != "xapp-secret-token" {
		t.Errorf("SlackAppToken = %v, want 'xapp-secret-token'", retrieved.SlackAppToken)
	}
	if retrieved.SlackSigningSecret != "very-secret" {
		t.Errorf("SlackSigningSecret = %v, want 'very-secret'", retrieved.SlackSigningSecret)
	}
}

func TestFileBotConfigRepository_ConcurrentAccess(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bot-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	filePath := filepath.Join(tmpDir, "bots.json")
	encryption := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, encryption)

	ctx := context.Background()

	// Create multiple bots concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(index int) {
			bot := domain.NewSlackBotConfig(
				fmt.Sprintf("bot-%d", index),
				fmt.Sprintf("Bot %d", index),
				domain.BotTypePublic,
			)
			bot.SlackBotToken = fmt.Sprintf("xoxb-token-%d", index)
			bot.SlackAppToken = fmt.Sprintf("xapp-token-%d", index)
			bot.SlackSigningSecret = fmt.Sprintf("secret-%d", index)

			err := repo.SaveSlackBot(ctx, bot)
			if err != nil {
				t.Errorf("SaveSlackBot() error = %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all bots were saved
	bots, err := repo.ListSlackBots(ctx)
	if err != nil {
		t.Fatalf("ListSlackBots() error = %v", err)
	}

	if len(bots) != 10 {
		t.Errorf("Expected 10 bots, got %d", len(bots))
	}
}

func TestFileBotConfigRepository_GetNonExistent(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bot-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	filePath := filepath.Join(tmpDir, "bots.json")
	encryption := security.NewEncryptionService(testEncryptionKey)
	repo := NewFileBotConfigRepository(filePath, encryption)

	ctx := context.Background()

	// Try to get non-existent bot
	_, err = repo.GetSlackBotByID(ctx, "non-existent")
	if err == nil {
		t.Error("expected error when getting non-existent bot")
	}
}

// Helper function to check if plaintext appears in content
func containsPlaintext(content, plaintext string) bool {
	return len(content) > 0 && len(plaintext) > 0 &&
		string(content) != "" &&
		string(plaintext) != "" &&
		bytes.Contains([]byte(content), []byte(plaintext))
}
