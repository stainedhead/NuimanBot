package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// confirmCallbackPrefix/confirmActionApprove/confirmActionDeny define the
// inline-keyboard callback_data format used to render and resolve a
// pending confirmation (specs/260802-improve-nuimanbot-security, Phase 5
// Part C / P5.7): "confirm:<approve|deny>:<confirmation_id>".
const (
	confirmCallbackPrefix = "confirm:"
	confirmActionApprove  = "approve"
	confirmActionDeny     = "deny"
)

// Gateway implements domain.Gateway for Telegram.
type Gateway struct {
	config         *config.TelegramConfig
	bot            *bot.Bot
	messageHandler domain.MessageHandler
	cancel         context.CancelFunc
}

// New creates a new Telegram gateway.
func New(cfg *config.TelegramConfig) (*Gateway, error) {
	return &Gateway{
		config: cfg,
	}, nil
}

// Platform returns the platform identifier for Telegram.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformTelegram
}

// Start begins the Telegram bot polling.
func (g *Gateway) Start(ctx context.Context) error {
	if g.config.Token.Value() == "" {
		return fmt.Errorf("Telegram bot token is required")
	}

	// Create bot instance
	opts := []bot.Option{
		bot.WithDefaultHandler(g.handleUpdate),
		bot.WithCallbackQueryDataHandler(confirmCallbackPrefix, bot.MatchTypePrefix, g.handleConfirmationCallback),
	}

	b, err := bot.New(g.config.Token.Value(), opts...)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	g.bot = b

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel

	slog.Info("Gateway started", "platform", "telegram", "mode", "long_polling")

	// Start polling (this blocks)
	g.bot.Start(ctx)

	return nil
}

// Stop gracefully shuts down the Telegram gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		slog.Info("Stopping gateway", "platform", "telegram")
		g.cancel()
	}
	return nil
}

// handleUpdate processes incoming Telegram updates
func (g *Gateway) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Only process messages (ignore other update types for now)
	if update.Message == nil {
		return
	}

	msg := update.Message

	// Check if message has text
	if msg.Text == "" {
		return
	}

	// Check if user is allowed (if AllowedIDs configured)
	if !g.isUserAllowed(msg.From.ID) {
		slog.Warn("Ignoring message from unauthorized user",
			"platform", "telegram",
			"user_id", msg.From.ID,
		)
		return
	}

	// Convert to domain.IncomingMessage
	incomingMsg := domain.IncomingMessage{
		ID:          strconv.FormatInt(int64(msg.ID), 10),
		Platform:    domain.PlatformTelegram,
		PlatformUID: strconv.FormatInt(msg.From.ID, 10),
		Text:        msg.Text,
		Timestamp:   time.Unix(int64(msg.Date), 0),
		Metadata: map[string]any{
			"message_id": msg.ID,
			"chat_id":    msg.Chat.ID,
			"chat_type":  msg.Chat.Type,
			"username":   msg.From.Username,
			"first_name": msg.From.FirstName,
			"last_name":  msg.From.LastName,
		},
	}

	// Call message handler if registered
	if g.messageHandler != nil {
		if err := g.messageHandler(ctx, incomingMsg); err != nil {
			slog.Error("Error handling message",
				"platform", "telegram",
				"error", err,
			)
		}
	}
}

// Send sends a message to a Telegram user.
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	if g.bot == nil {
		return fmt.Errorf("Telegram bot not initialized")
	}

	// Extract chat ID from metadata
	var chatID int64
	if msg.Metadata != nil {
		if cid, ok := msg.Metadata["chat_id"]; ok {
			switch v := cid.(type) {
			case int64:
				chatID = v
			case int:
				chatID = int64(v)
			case float64:
				chatID = int64(v)
			}
		}
	}

	// Fallback: try to parse RecipientID as chat ID
	if chatID == 0 {
		if id, err := strconv.ParseInt(msg.RecipientID, 10, 64); err == nil {
			chatID = id
		}
	}

	if chatID == 0 {
		return fmt.Errorf("no chat_id found in message metadata or PlatformUID")
	}

	// Send message with Markdown formatting. A pending confirmation (Part C
	// / P5.7) is rendered with an inline-keyboard Approve/Deny prompt
	// instead of plain text.
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      msg.Content,
		ParseMode: models.ParseModeMarkdown,
	}

	if confirmationID, ok := pendingConfirmationID(msg.Metadata); ok {
		params.Text = "⚠️ *Confirmation required*\n\n" + msg.Content
		params.ReplyMarkup = confirmationKeyboard(confirmationID)
	}

	_, err := g.bot.SendMessage(ctx, params)

	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}

// isUserAllowed reports whether userID may interact with this bot, per the
// configured AllowedIDs allowlist (an empty/unset list means unrestricted).
func (g *Gateway) isUserAllowed(userID int64) bool {
	if len(g.config.AllowedIDs) == 0 {
		return true
	}
	for _, id := range g.config.AllowedIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// pendingConfirmationID mirrors the Slack gateway's helper of the same
// purpose: returns the confirmation ID and true when the outgoing message
// metadata marks it as an open confirmation awaiting an Approve/Deny
// response (domain.OutgoingMessage.Metadata["status"] ==
// "pending_confirmation" with a populated "confirmation_id", per
// chat.Service.finishPendingConfirmationTurn).
func pendingConfirmationID(metadata map[string]any) (string, bool) {
	if metadata == nil {
		return "", false
	}
	status, _ := metadata["status"].(string)
	if status != "pending_confirmation" {
		return "", false
	}
	id, _ := metadata["confirmation_id"].(string)
	if id == "" {
		return "", false
	}
	return id, true
}

// confirmationKeyboard builds the inline keyboard (a single row of
// Approve/Deny buttons) rendered for a pending confirmation prompt.
func confirmationKeyboard(confirmationID string) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Approve", CallbackData: confirmCallbackPrefix + confirmActionApprove + ":" + confirmationID},
				{Text: "❌ Deny", CallbackData: confirmCallbackPrefix + confirmActionDeny + ":" + confirmationID},
			},
		},
	}
}

// confirmationReplyText parses an inline-keyboard callback_data string
// ("confirm:<approve|deny>:<id>") and returns the plain-text "yes"/"no"
// reply matching the button that was clicked — the same vocabulary
// chat.Service.classifyConfirmationReply's confirmationApproveWords /
// confirmationDenyWords already recognizes.
func confirmationReplyText(data string) (string, bool) {
	rest, ok := strings.CutPrefix(data, confirmCallbackPrefix)
	if !ok {
		return "", false
	}
	switch {
	case rest == confirmActionApprove || strings.HasPrefix(rest, confirmActionApprove+":"):
		return "yes", true
	case rest == confirmActionDeny || strings.HasPrefix(rest, confirmActionDeny+":"):
		return "no", true
	default:
		return "", false
	}
}

// handleConfirmationCallback resolves an Approve/Deny inline-keyboard
// button click on a pending-confirmation message (Part C / P5.7). Per
// specs/260802-improve-nuimanbot-security's implementation notes,
// ChatService has no dedicated "resolve by button click" entry point; the
// simplest and most consistent option is to simulate the exact plain-text
// "yes"/"no" reply the user could have typed instead —
// ChatService.ProcessMessage's existing confirmation-reply detection
// (classifyConfirmationReply) already resolves that end-to-end with zero
// gateway-specific resolution code, since it keys off (PlatformUID,
// conversationID) = (Platform + PlatformUID), both of which this
// synthesized message carries correctly.
func (g *Gateway) handleConfirmationCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cq := update.CallbackQuery

	// Acknowledge the callback query first so Telegram doesn't retry
	// delivering it (see the go-telegram/bot library's own inline_keyboard
	// example, which documents this same ordering).
	if b != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}

	if !g.isUserAllowed(cq.From.ID) {
		slog.Warn("Ignoring confirmation button click from unauthorized user",
			"platform", "telegram",
			"user_id", cq.From.ID,
		)
		return
	}

	reply, ok := confirmationReplyText(cq.Data)
	if !ok {
		return
	}

	if g.messageHandler == nil {
		return
	}

	incomingMsg := domain.IncomingMessage{
		ID:          cq.ID,
		Platform:    domain.PlatformTelegram,
		PlatformUID: strconv.FormatInt(cq.From.ID, 10),
		Text:        reply,
		Timestamp:   time.Now(),
		Metadata: map[string]any{
			"message_type": "confirmation_button",
		},
	}

	if err := g.messageHandler(ctx, incomingMsg); err != nil {
		slog.Error("Error handling Telegram confirmation button interaction",
			"platform", "telegram",
			"error", err,
		)
	}
}
