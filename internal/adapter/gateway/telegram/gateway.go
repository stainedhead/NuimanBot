package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
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
	}

	b, err := bot.New(g.config.Token.Value(), opts...)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	g.bot = b

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel

	log.Println("Telegram gateway started, beginning long polling...")

	// Start polling (this blocks)
	g.bot.Start(ctx)

	return nil
}

// Stop gracefully shuts down the Telegram gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		log.Println("Stopping Telegram gateway...")
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
	if len(g.config.AllowedIDs) > 0 {
		allowed := false
		for _, id := range g.config.AllowedIDs {
			if id == msg.From.ID {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Printf("Telegram: Ignoring message from unauthorized user %d", msg.From.ID)
			return
		}
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
			log.Printf("Telegram: Error handling message: %v", err)
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

	// Send message with Markdown formatting
	_, err := g.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      msg.Content,
		ParseMode: models.ParseModeMarkdown,
	})

	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}
