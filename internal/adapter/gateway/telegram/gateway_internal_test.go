package telegram

import (
	"context"
	"errors"
	"strings"
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

// --- Part C / P5.7: pending-confirmation rendering (inline keyboard) ---

func TestPendingConfirmationID(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantID   string
		wantOK   bool
	}{
		{"nil metadata", nil, "", false},
		{"no status key", map[string]any{"confirmation_id": "abc"}, "", false},
		{"wrong status", map[string]any{"status": "confirmation_denied", "confirmation_id": "abc"}, "", false},
		{"pending but no id", map[string]any{"status": "pending_confirmation"}, "", false},
		{"pending with id", map[string]any{"status": "pending_confirmation", "confirmation_id": "abc-123"}, "abc-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := pendingConfirmationID(tt.metadata)
			if ok != tt.wantOK || id != tt.wantID {
				t.Errorf("pendingConfirmationID(%v) = (%q, %v), want (%q, %v)", tt.metadata, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestConfirmationKeyboard_HasApproveDenyCallbackData(t *testing.T) {
	kb := confirmationKeyboard("conf-abc")

	if len(kb.InlineKeyboard) == 0 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected a single row with 2 buttons, got %+v", kb.InlineKeyboard)
	}

	approve := kb.InlineKeyboard[0][0]
	deny := kb.InlineKeyboard[0][1]

	if !strings.Contains(approve.CallbackData, "conf-abc") || !strings.Contains(approve.CallbackData, confirmActionApprove) {
		t.Errorf("approve button callback data = %q, want it to reference approve action + id", approve.CallbackData)
	}
	if !strings.Contains(deny.CallbackData, "conf-abc") || !strings.Contains(deny.CallbackData, confirmActionDeny) {
		t.Errorf("deny button callback data = %q, want it to reference deny action + id", deny.CallbackData)
	}
	if !strings.HasPrefix(approve.CallbackData, confirmCallbackPrefix) || !strings.HasPrefix(deny.CallbackData, confirmCallbackPrefix) {
		t.Errorf("callback data must start with prefix %q: approve=%q deny=%q", confirmCallbackPrefix, approve.CallbackData, deny.CallbackData)
	}
}

func TestConfirmationReplyText(t *testing.T) {
	kb := confirmationKeyboard("conf-xyz")
	approveData := kb.InlineKeyboard[0][0].CallbackData
	denyData := kb.InlineKeyboard[0][1].CallbackData

	tests := []struct {
		name     string
		data     string
		wantText string
		wantOK   bool
	}{
		{"approve", approveData, "yes", true},
		{"deny", denyData, "no", true},
		{"unrelated prefix", "something:else", "", false},
		{"empty", "", "", false},
		{"malformed confirm prefix", "confirm:unknown:conf-xyz", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok := confirmationReplyText(tt.data)
			if ok != tt.wantOK || text != tt.wantText {
				t.Errorf("confirmationReplyText(%q) = (%q, %v), want (%q, %v)", tt.data, text, ok, tt.wantText, tt.wantOK)
			}
		})
	}
}

func TestIsUserAllowed(t *testing.T) {
	tests := []struct {
		name       string
		allowedIDs []int64
		userID     int64
		want       bool
	}{
		{"no restriction configured", nil, 12345, true},
		{"allowed", []int64{111, 12345}, 12345, true},
		{"not allowed", []int64{111}, 12345, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &Gateway{config: &config.TelegramConfig{AllowedIDs: tt.allowedIDs}}
			if got := gw.isUserAllowed(tt.userID); got != tt.want {
				t.Errorf("isUserAllowed(%d) = %v, want %v", tt.userID, got, tt.want)
			}
		})
	}
}

func TestHandleConfirmationCallback_NilCallbackQuery(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{CallbackQuery: nil}

	var b *bot.Bot
	gw.handleConfirmationCallback(context.Background(), b, update)

	if handlerCalled {
		t.Error("handler should not be called when CallbackQuery is nil")
	}
}

func TestHandleConfirmationCallback_Approve(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}

	var received domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		received = msg
		return nil
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq1",
			From: models.User{ID: 12345},
			Data: confirmationKeyboard("conf-1").InlineKeyboard[0][0].CallbackData, // approve
		},
	}

	var b *bot.Bot // nil bot: AnswerCallbackQuery call must be guarded
	gw.handleConfirmationCallback(context.Background(), b, update)

	if received.Text != "yes" {
		t.Errorf("Text = %q, want %q", received.Text, "yes")
	}
	if received.PlatformUID != "12345" {
		t.Errorf("PlatformUID = %q, want %q", received.PlatformUID, "12345")
	}
	if received.Platform != domain.PlatformTelegram {
		t.Errorf("Platform = %q, want %q", received.Platform, domain.PlatformTelegram)
	}
}

func TestHandleConfirmationCallback_Deny(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}

	var received domain.IncomingMessage
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		received = msg
		return nil
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq2",
			From: models.User{ID: 12345},
			Data: confirmationKeyboard("conf-1").InlineKeyboard[0][1].CallbackData, // deny
		},
	}

	var b *bot.Bot
	gw.handleConfirmationCallback(context.Background(), b, update)

	if received.Text != "no" {
		t.Errorf("Text = %q, want %q", received.Text, "no")
	}
}

func TestHandleConfirmationCallback_UnauthorizedUser(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{AllowedIDs: []int64{111}}}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq3",
			From: models.User{ID: 99999}, // not in allowed list
			Data: confirmationKeyboard("conf-1").InlineKeyboard[0][0].CallbackData,
		},
	}

	var b *bot.Bot
	gw.handleConfirmationCallback(context.Background(), b, update)

	if handlerCalled {
		t.Error("handler should not be called for an unauthorized user")
	}
}

func TestHandleConfirmationCallback_NoHandler(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}
	// messageHandler is nil

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq4",
			From: models.User{ID: 12345},
			Data: confirmationKeyboard("conf-1").InlineKeyboard[0][0].CallbackData,
		},
	}

	var b *bot.Bot
	// Should not panic
	gw.handleConfirmationCallback(context.Background(), b, update)
}

func TestHandleConfirmationCallback_UnrecognizedData(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}

	handlerCalled := false
	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalled = true
		return nil
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq5",
			From: models.User{ID: 12345},
			Data: "not_a_confirmation_callback",
		},
	}

	var b *bot.Bot
	gw.handleConfirmationCallback(context.Background(), b, update)

	if handlerCalled {
		t.Error("handler should not be called for unrecognized callback data")
	}
}

func TestHandleConfirmationCallback_HandlerError(t *testing.T) {
	gw := &Gateway{config: &config.TelegramConfig{Enabled: true}}

	gw.messageHandler = func(ctx context.Context, msg domain.IncomingMessage) error {
		return errors.New("handler error")
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cbq6",
			From: models.User{ID: 12345},
			Data: confirmationKeyboard("conf-1").InlineKeyboard[0][0].CallbackData,
		},
	}

	var b *bot.Bot
	// Should not panic
	gw.handleConfirmationCallback(context.Background(), b, update)
}
