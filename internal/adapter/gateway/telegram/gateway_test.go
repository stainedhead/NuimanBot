package telegram_test

import (
	"testing"

	"nuimanbot/internal/adapter/gateway/telegram"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func TestNew(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, err := telegram.New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if gw == nil {
		t.Fatal("New() returned nil gateway")
	}
}

func TestPlatform(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)

	if gw.Platform() != domain.PlatformTelegram {
		t.Errorf("Expected platform %s, got %s", domain.PlatformTelegram, gw.Platform())
	}
}
