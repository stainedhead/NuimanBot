package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestHandleUpdate_NilMessage(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		Message: nil,
	}

	// Should not panic and handler should not be called
	gw.handleUpdate(context.Background(), nil, update)
	if handlerCalled {
		t.Error("handler should not be called for nil message")
	}
}

func TestHandleUpdate_EmptyText(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "",
			From: &models.User{ID: 12345, FirstName: "Test"},
			Chat: models.Chat{ID: 67890},
			Date: int(time.Now().Unix()),
		},
	}

	gw.handleUpdate(context.Background(), nil, update)
	if handlerCalled {
		t.Error("handler should not be called for empty text message")
	}
}

func TestHandleUpdate_WithMessage(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true, AllowedIDs: []int64{}}
	gw := &Gateway{config: cfg}

	var receivedMsg domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		receivedMsg = msg
		return nil
	}

	update := &models.Update{
		Message: &models.Message{
			ID:   42,
			Text: "hello telegram",
			From: &models.User{
				ID:        12345,
				FirstName: "Alice",
				LastName:  "Smith",
				Username:  "alice123",
			},
			Chat: models.Chat{ID: 67890, Type: "private"},
			Date: int(time.Now().Unix()),
		},
	}

	gw.handleUpdate(context.Background(), nil, update)

	if receivedMsg.Text != "hello telegram" {
		t.Errorf("Text = %q, want %q", receivedMsg.Text, "hello telegram")
	}
	if receivedMsg.Platform != domain.PlatformTelegram {
		t.Errorf("Platform = %q, want %q", receivedMsg.Platform, domain.PlatformTelegram)
	}
	if receivedMsg.PlatformUID != "12345" {
		t.Errorf("PlatformUID = %q, want %q", receivedMsg.PlatformUID, "12345")
	}
}

func TestHandleUpdate_AllowedIDs_Allowed(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled:    true,
		AllowedIDs: []int64{12345, 67890},
	}
	gw := &Gateway{config: cfg}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "hello",
			From: &models.User{ID: 12345},
			Chat: models.Chat{ID: 111},
			Date: int(time.Now().Unix()),
		},
	}

	gw.handleUpdate(context.Background(), nil, update)
	if !handlerCalled {
		t.Error("handler should be called for allowed user ID")
	}
}

func TestHandleUpdate_AllowedIDs_NotAllowed(t *testing.T) {
	cfg := &config.TelegramConfig{
		Enabled:    true,
		AllowedIDs: []int64{12345},
	}
	gw := &Gateway{config: cfg}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "hello",
			From: &models.User{ID: 99999}, // not in allowed list
			Chat: models.Chat{ID: 111},
			Date: int(time.Now().Unix()),
		},
	}

	gw.handleUpdate(context.Background(), nil, update)
	if handlerCalled {
		t.Error("handler should not be called for non-allowed user ID")
	}
}

func TestHandleUpdate_HandlerError(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		return errors.New("handler error")
	}

	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "hello",
			From: &models.User{ID: 12345},
			Chat: models.Chat{ID: 111},
			Date: int(time.Now().Unix()),
		},
	}

	// Should not panic
	gw.handleUpdate(context.Background(), nil, update)
}

func TestHandleUpdate_NoHandler(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}
	// No message handler registered

	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "hello",
			From: &models.User{ID: 12345},
			Chat: models.Chat{ID: 111},
			Date: int(time.Now().Unix()),
		},
	}

	// Should not panic
	gw.handleUpdate(context.Background(), nil, update)
}

func TestStop_WithCancel(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	cancelCalled := false
	gw.cancel = func() {
		cancelCalled = true
	}

	if err := gw.Stop(context.Background()); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	if !cancelCalled {
		t.Error("cancel should be called when stopping with active cancel func")
	}
}

func TestSend_BotInitialized(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	// bot is nil - should return error about bot not initialized
	ctx := context.Background()
	msg := domain.OutgoingMessage{
		RecipientID: "123456",
		Content:     "Test message",
	}
	err := gw.Send(ctx, msg)
	if err == nil {
		t.Error("Send() should error when bot not initialized")
	}
}

// Placeholder to test that handleUpdate passes bot parameter correctly
func TestHandleUpdate_BotParamCanBeNil(t *testing.T) {
	cfg := &config.TelegramConfig{Enabled: true}
	gw := &Gateway{config: cfg}

	// The handleUpdate func signature accepts *bot.Bot but doesn't use it currently
	// This test verifies nil bot doesn't cause a panic
	update := &models.Update{
		Message: &models.Message{
			ID:   1,
			Text: "test",
			From: &models.User{ID: 1},
			Chat: models.Chat{ID: 2},
			Date: int(time.Now().Unix()),
		},
	}

	var b *bot.Bot
	gw.handleUpdate(context.Background(), b, update)
}
