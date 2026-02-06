package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// Gateway implements domain.Gateway for Slack using Socket Mode.
type Gateway struct {
	config         *config.SlackConfig
	client         *slack.Client
	socketClient   *socketmode.Client
	messageHandler domain.MessageHandler
	cancel         context.CancelFunc
}

// New creates a new Slack gateway.
func New(cfg *config.SlackConfig) (*Gateway, error) {
	return &Gateway{
		config: cfg,
	}, nil
}

// Platform returns the platform identifier for Slack.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformSlack
}

// Start begins the Slack Socket Mode connection.
func (g *Gateway) Start(ctx context.Context) error {
	if g.config.BotToken.Value() == "" {
		return fmt.Errorf("Slack bot token is required")
	}
	if g.config.AppToken.Value() == "" {
		return fmt.Errorf("Slack app token is required for Socket Mode")
	}

	// Create Slack API client
	g.client = slack.New(
		g.config.BotToken.Value(),
		slack.OptionAppLevelToken(g.config.AppToken.Value()),
	)

	// Create Socket Mode client
	g.socketClient = socketmode.New(
		g.client,
		socketmode.OptionDebug(false),
	)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel

	// Handle events
	go g.handleEvents(ctx)

	slog.Info("Gateway started", "platform", "slack", "mode", "socket_mode")

	// Start Socket Mode (this blocks)
	return g.socketClient.Run()
}

// Stop gracefully shuts down the Slack gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		slog.Info("Stopping gateway", "platform", "slack")
		g.cancel()
	}
	return nil
}

// Send sends a message to a Slack channel.
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	if g.client == nil {
		return fmt.Errorf("Slack client not initialized")
	}

	// Extract channel ID from metadata
	var channelID string
	if msg.Metadata != nil {
		if cid, ok := msg.Metadata["channel"]; ok {
			channelID, _ = cid.(string)
		}
	}

	// Fallback to RecipientID
	if channelID == "" {
		channelID = msg.RecipientID
	}

	if channelID == "" {
		return fmt.Errorf("no channel ID found in message metadata or RecipientID")
	}

	// Check for thread_ts to reply in thread
	opts := []slack.MsgOption{
		slack.MsgOptionText(msg.Content, false),
	}

	if msg.Metadata != nil {
		if threadTS, ok := msg.Metadata["thread_ts"].(string); ok && threadTS != "" {
			opts = append(opts, slack.MsgOptionTS(threadTS))
		}
	}

	// Send message
	_, _, err := g.client.PostMessage(channelID, opts...)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}

	return nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}

// handleEvents processes incoming Slack events
func (g *Gateway) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-g.socketClient.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}

				// Acknowledge the event
				g.socketClient.Ack(*evt.Request)

				// Process the inner event
				g.handleSlackEvent(ctx, eventsAPIEvent.InnerEvent)

			case socketmode.EventTypeHello:
				slog.Info("Connected to Socket Mode", "platform", "slack")
			}
		}
	}
}

// handleSlackEvent processes specific Slack event types
func (g *Gateway) handleSlackEvent(ctx context.Context, innerEvent slackevents.EventsAPIInnerEvent) {
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		g.handleMessage(ctx, ev.User, ev.Text, ev.Channel, ev.TimeStamp, ev.ThreadTimeStamp, "app_mention")
	case *slackevents.MessageEvent:
		// Only handle direct messages or messages in channels where bot is mentioned
		if ev.ChannelType == "im" {
			g.handleMessage(ctx, ev.User, ev.Text, ev.Channel, ev.TimeStamp, ev.ThreadTimeStamp, "direct_message")
		}
	}
}

// handleMessage converts Slack message to domain.IncomingMessage
func (g *Gateway) handleMessage(ctx context.Context, userID, text, channel, ts, threadTS, messageType string) {
	if text == "" {
		return
	}

	// Convert to domain.IncomingMessage
	incomingMsg := domain.IncomingMessage{
		ID:          ts,
		Platform:    domain.PlatformSlack,
		PlatformUID: userID,
		Text:        text,
		Timestamp:   parseSlackTimestamp(ts),
		Metadata: map[string]any{
			"channel":      channel,
			"message_ts":   ts,
			"thread_ts":    threadTS,
			"message_type": messageType,
		},
	}

	// Call message handler if registered
	if g.messageHandler != nil {
		if err := g.messageHandler(ctx, incomingMsg); err != nil {
			slog.Error("Error handling message",
				"platform", "slack",
				"error", err,
			)
		}
	}
}

// parseSlackTimestamp converts Slack timestamp to time.Time
func parseSlackTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	// Slack timestamps are in format "1234567890.123456"
	if tsFloat, err := strconv.ParseFloat(ts, 64); err == nil {
		return time.Unix(int64(tsFloat), 0)
	}
	return time.Now()
}
