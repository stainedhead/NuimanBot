package slack_test

import (
	"context"
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

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}

	if gw.Platform() != domain.PlatformSlack {
		t.Errorf("Expected platform %s, got %s", domain.PlatformSlack, gw.Platform())
	}
}

func TestOnMessage(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Test that OnMessage accepts a handler
	handlerCalled := false
	handler := func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	gw.OnMessage(handler)

	// We can't directly test the handler is called without starting the bot,
	// but we verify OnMessage doesn't panic and accepts the handler
	if handlerCalled {
		t.Error("Handler should not be called during registration")
	}
}

func TestSend_InvalidConfig(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "C123456",
		Content:     "Test message",
		Format:      "text",
	}

	// Send should fail because client is not initialized (Start not called)
	err = gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when client not initialized")
	}
}

func TestSend_MissingChannelID(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "", // Empty recipient
		Content:     "Test message",
		Format:      "text",
	}

	// Send should fail because no channel ID
	err = gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when channel ID missing")
	}
}

func TestSend_WithChannelInMetadata(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "",
		Content:     "Test message",
		Format:      "text",
		Metadata: map[string]any{
			"channel": "C123456",
		},
	}

	// Send should attempt to send (will fail without client, but tests channel extraction)
	err = gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when client not initialized")
	}
	// Error should be about client not initialized, not missing channel
	if err.Error() == "no channel ID found in message metadata or RecipientID" {
		t.Error("Should have extracted channel from metadata")
	}
}

func TestSend_WithThreadTS(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "C123456",
		Content:     "Test reply",
		Format:      "text",
		Metadata: map[string]any{
			"thread_ts": "1234567890.123456",
		},
	}

	// Send should attempt to send (will fail without client)
	err = gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when client not initialized")
	}
}

func TestStop(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	// Stop should not error even if not started
	err = gw.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestNew_ValidatesTokens(t *testing.T) {
	tests := []struct {
		name     string
		botToken string
		appToken string
		wantErr  bool
	}{
		{
			name:     "valid tokens",
			botToken: "xoxb-test-token",
			appToken: "xapp-test-token",
			wantErr:  false,
		},
		{
			name:     "empty tokens",
			botToken: "",
			appToken: "",
			wantErr:  false, // New doesn't validate, only Start does
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.SlackConfig{
				Enabled:  true,
				BotToken: domain.NewSecureStringFromString(tt.botToken),
				AppToken: domain.NewSecureStringFromString(tt.appToken),
			}

			gw, err := slack.New(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && gw == nil {
				t.Error("New() returned nil gateway")
			}
		})
	}
}

func TestSend_FallbackToRecipientID(t *testing.T) {
	cfg := &config.SlackConfig{
		Enabled:  true,
		BotToken: domain.NewSecureStringFromString("xoxb-test-token"),
		AppToken: domain.NewSecureStringFromString("xapp-test-token"),
	}

	gw, err := slack.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "C123456", // Should use this as channel ID
		Content:     "Test message",
		Format:      "text",
		Metadata:    nil, // No metadata, should fallback to RecipientID
	}

	// Send should attempt to send (will fail without client, but tests fallback logic)
	err = gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when client not initialized")
	}
	// Error should NOT be about missing channel ID
	if err.Error() == "no channel ID found in message metadata or RecipientID" {
		t.Error("Should have used RecipientID as fallback channel")
	}
}
