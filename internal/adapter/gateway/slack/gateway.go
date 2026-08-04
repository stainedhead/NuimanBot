package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
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

	opts := g.buildMessageOptions(msg)

	// Send message
	_, _, err := g.client.PostMessage(channelID, opts...)
	if err != nil {
		return fmt.Errorf("failed to send Slack message: %w", err)
	}

	return nil
}

// confirmActionApprove/confirmActionDeny are the Block Kit action_ids
// attached to the Approve/Deny buttons rendered for a pending confirmation
// (specs/260802-improve-nuimanbot-security, Phase 5 Part C / P5.7).
const (
	confirmActionApprove = "confirm_approve"
	confirmActionDeny    = "confirm_deny"
)

// buildMessageOptions builds the slack.MsgOption set for an outgoing
// message. When the message is a pending-confirmation prompt (Part C), it
// renders Block Kit Approve/Deny buttons instead of plain text — the
// underlying slack-go/slack dependency already supports Block Kit, so this
// adds no new dependency. Any other message (including a resolved
// "confirmation_denied"/normal reply) is sent as plain text, unchanged from
// pre-P5.7 behavior.
func (g *Gateway) buildMessageOptions(msg domain.OutgoingMessage) []slack.MsgOption {
	var opts []slack.MsgOption

	if confirmationID, ok := pendingConfirmationID(msg.Metadata); ok {
		opts = append(opts, confirmationBlockOptions(msg.Content, confirmationID)...)
	} else {
		opts = append(opts, slack.MsgOptionText(msg.Content, false))
	}

	// Check for thread_ts to reply in thread
	if msg.Metadata != nil {
		if threadTS, ok := msg.Metadata["thread_ts"].(string); ok && threadTS != "" {
			opts = append(opts, slack.MsgOptionTS(threadTS))
		}
	}

	return opts
}

// pendingConfirmationID returns the confirmation ID and true when the
// message metadata marks this reply as an open confirmation awaiting an
// Approve/Deny response (domain.OutgoingMessage.Metadata["status"] ==
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

// confirmationBlockOptions builds the Block Kit message options (a styled
// warning section plus Approve/Deny buttons, with a plain-text fallback for
// clients/notifications that can't render blocks) for a pending
// confirmation prompt.
func confirmationBlockOptions(content, confirmationID string) []slack.MsgOption {
	fallbackText := "⚠️ Confirmation required: " + content + " (reply \"yes\" or \"no\")"

	section := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, "⚠️ *Confirmation required*\n\n"+content, false, false),
		nil, nil,
	)

	approve := slack.NewButtonBlockElement(confirmActionApprove, confirmationID,
		slack.NewTextBlockObject(slack.PlainTextType, "Approve", true, false))
	approve.Style = slack.StylePrimary

	deny := slack.NewButtonBlockElement(confirmActionDeny, confirmationID,
		slack.NewTextBlockObject(slack.PlainTextType, "Deny", true, false))
	deny.Style = slack.StyleDanger

	actions := slack.NewActionBlock("confirmation_actions", approve, deny)

	return []slack.MsgOption{
		slack.MsgOptionText(fallbackText, false),
		slack.MsgOptionBlocks(section, actions),
	}
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

			case socketmode.EventTypeInteractive:
				callback, ok := evt.Data.(slack.InteractionCallback)
				if !ok {
					continue
				}

				// Acknowledge the interaction
				g.socketClient.Ack(*evt.Request)

				// Process Approve/Deny button clicks (Part C / P5.7)
				g.handleInteraction(ctx, callback)

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

// handleInteraction processes a Slack Block Kit interaction — specifically,
// an Approve/Deny button click on a pending-confirmation message (Part C /
// P5.7). Per specs/260802-improve-nuimanbot-security's implementation
// notes, ChatService has no dedicated "resolve by button click" entry
// point; the simplest and most consistent option is to simulate the exact
// plain-text "yes"/"no" reply the user could have typed instead —
// ChatService.ProcessMessage's existing confirmation-reply detection
// (classifyConfirmationReply) already resolves that end-to-end with zero
// gateway-specific resolution code, since it keys off (PlatformUID,
// conversationID) = (Platform + PlatformUID), both of which this
// synthesized message carries correctly.
func (g *Gateway) handleInteraction(ctx context.Context, callback slack.InteractionCallback) {
	if callback.Type != slack.InteractionTypeBlockActions {
		return
	}

	reply, ok := confirmationReplyText(callback.ActionCallback.BlockActions)
	if !ok {
		return
	}

	if g.messageHandler == nil {
		return
	}

	incomingMsg := domain.IncomingMessage{
		ID:          callback.ActionTs,
		Platform:    domain.PlatformSlack,
		PlatformUID: callback.User.ID,
		Text:        reply,
		Timestamp:   time.Now(),
		Metadata: map[string]any{
			"channel":      callback.Channel.ID,
			"message_type": "confirmation_button",
		},
	}

	if err := g.messageHandler(ctx, incomingMsg); err != nil {
		slog.Error("Error handling Slack confirmation button interaction",
			"platform", "slack",
			"error", err,
		)
	}
}

// confirmationReplyText inspects a Block Kit interaction's block actions
// and returns the plain-text "yes"/"no" reply matching the Approve/Deny
// button that was clicked — the same vocabulary
// chat.Service.classifyConfirmationReply's confirmationApproveWords /
// confirmationDenyWords already recognizes.
func confirmationReplyText(actions []*slack.BlockAction) (string, bool) {
	for _, action := range actions {
		if action == nil {
			continue
		}
		switch action.ActionID {
		case confirmActionApprove:
			return "yes", true
		case confirmActionDeny:
			return "no", true
		}
	}
	return "", false
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
