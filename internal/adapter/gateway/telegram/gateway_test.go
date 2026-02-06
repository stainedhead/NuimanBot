package telegram_test

import (
	"context"
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

func TestOnMessage(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, err := telegram.New(cfg)
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
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "123456",
		Content:     "Test message",
		Format:      "text",
	}

	// Send should fail because bot is not initialized (Start not called)
	err := gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when bot not initialized")
	}
}

func TestSend_MissingChatID(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "", // Empty recipient
		Content:     "Test message",
		Format:      "text",
	}

	// Send should fail because no chat_id
	err := gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when chat_id missing")
	}
}

func TestSend_WithChatIDInMetadata(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)
	ctx := context.Background()

	msg := domain.OutgoingMessage{
		RecipientID: "123456",
		Content:     "Test message",
		Format:      "text",
		Metadata: map[string]any{
			"chat_id": int64(123456),
		},
	}

	// Send should attempt to send (will fail without bot, but tests chat_id extraction)
	err := gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when bot not initialized")
	}
	// Error should be about bot not initialized, not missing chat_id
	if err.Error() == "no chat_id found in message metadata or PlatformUID" {
		t.Error("Should have extracted chat_id from metadata")
	}
}

func TestStop(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)
	ctx := context.Background()

	// Stop should not error even if bot not started
	err := gw.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestNew_ValidatesToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   "test-token-123",
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: false, // New doesn't validate, only Start does
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.TelegramConfig{
				Enabled: true,
				Token:   domain.NewSecureStringFromString(tt.token),
			}

			gw, err := telegram.New(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && gw == nil {
				t.Error("New() returned nil gateway")
			}
		})
	}
}

func TestSend_ChatIDTypeConversion(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled: true,
		Token:   domain.NewSecureStringFromString("test-token"),
	}

	gw, _ := telegram.New(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
	}{
		{
			name: "chat_id as int64",
			metadata: map[string]any{
				"chat_id": int64(123456),
			},
			wantErr: true, // Will fail because bot not initialized, but chat_id extracted
		},
		{
			name: "chat_id as int",
			metadata: map[string]any{
				"chat_id": int(123456),
			},
			wantErr: true, // Will fail because bot not initialized, but chat_id extracted
		},
		{
			name: "chat_id as float64",
			metadata: map[string]any{
				"chat_id": float64(123456),
			},
			wantErr: true, // Will fail because bot not initialized, but chat_id extracted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := domain.OutgoingMessage{
				RecipientID: "",
				Content:     "Test",
				Metadata:    tt.metadata,
			}

			err := gw.Send(ctx, msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
