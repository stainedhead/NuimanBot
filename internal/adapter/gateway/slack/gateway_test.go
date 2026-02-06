package slack_test

import (
	"testing"

	"nuimanbot/internal/adapter/gateway/slack"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

func TestNew(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if gw == nil {
		t.Fatal("New() returned nil gateway")
	}
}

func TestPlatform(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, _ := slack.New(cfg)

	if gw.Platform() != domain.PlatformSlack {
		t.Errorf("Expected platform %s, got %s", domain.PlatformSlack, gw.Platform())
	}
}
