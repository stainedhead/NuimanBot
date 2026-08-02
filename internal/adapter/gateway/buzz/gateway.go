// Package buzz implements domain.Gateway for Buzz, Block's Nostr-based
// multi-agent chat platform.
package buzz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/nostr"
	"nuimanbot/internal/usecase/user"
)

const (
	// buzzSubscriptionID is the fixed NIP-01 subscription id this gateway uses
	// for its single channel-message subscription (FR-002).
	buzzSubscriptionID = "buzz-channels"

	// buzzChannelAddPolicy is the value this agent declares in its kind:10100
	// profile event's channel_add_policy field (P2.2) — see
	// implementation-notes.md for why this is the actual, relay-consumed
	// content schema for kind:10100 (not the "agent metadata + owner
	// reference" description in Buzz's kind.rs doc comment, which no current
	// relay code path reads or requires). "owner_only" is deliberately
	// conservative: only the Buzz workspace owner may add this agent to new
	// channels, so it isn't pulled into arbitrary channels by any user.
	buzzChannelAddPolicy = "owner_only"

	// buzzProfilePublishMaxAttempts/InitialBackoff bound the best-effort retry
	// for publishing the kind:10100 profile event once at Start() — the relay
	// connection is dialed asynchronously and may not be up yet on the first
	// attempt.
	buzzProfilePublishMaxAttempts    = 5
	buzzProfilePublishInitialBackoff = 200 * time.Millisecond
)

// Gateway implements domain.Gateway for Buzz.
type Gateway struct {
	config      *config.BuzzConfig
	userService *user.Service

	client         *nostr.Client
	messageHandler domain.MessageHandler
	cancel         context.CancelFunc

	seenMu sync.Mutex
	seen   map[string]struct{} // dedupe by event ID (FR-004)
}

// New creates a new Buzz gateway. userService resolves/creates
// domain.User records for Buzz senders (FR-006); it may be nil in contexts
// (e.g. some tests) that don't exercise RBAC user resolution.
func New(cfg *config.BuzzConfig, userService *user.Service) (*Gateway, error) {
	return &Gateway{
		config:      cfg,
		userService: userService,
		seen:        make(map[string]struct{}),
	}, nil
}

// Platform returns the platform identifier for Buzz.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformBuzz
}

// Start connects to configured relays, subscribes to configured channels,
// and processes incoming events until ctx is canceled. Blocks (mirrors the
// Slack/Telegram gateways' Start pattern).
func (g *Gateway) Start(ctx context.Context) error {
	if len(g.config.Relays) == 0 {
		return fmt.Errorf("Buzz gateway requires at least one relay")
	}
	if g.config.PrivateKey.Value() == "" {
		return fmt.Errorf("Buzz private key is required")
	}

	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel

	g.client = nostr.NewClient(g.config.Relays)
	filter := nostr.NewChannelFilter(g.config.ChannelIDs)
	if err := g.client.Start(ctx, buzzSubscriptionID, filter); err != nil {
		cancel()
		return fmt.Errorf("failed to start Nostr client: %w", err)
	}

	slog.Info("Gateway started",
		"platform", "buzz",
		"relays", len(g.config.Relays),
		"channels", len(g.config.ChannelIDs),
	)

	go g.publishAgentProfileBestEffort(ctx)

	g.handleEvents(ctx)
	return nil
}

// publishAgentProfile publishes this agent's kind:10100 profile event once
// (P2.2). Not part of Send()'s per-message hot path.
func (g *Gateway) publishAgentProfile(ctx context.Context) error {
	content, err := json.Marshal(map[string]string{"channel_add_policy": buzzChannelAddPolicy})
	if err != nil {
		return fmt.Errorf("failed to marshal Buzz agent profile content: %w", err)
	}

	event := nostr.Event{
		CreatedAt: time.Now().Unix(),
		Kind:      nostr.KindAgentProfile,
		Content:   string(content),
	}
	if err := nostr.Sign(&event, g.config.PrivateKey.Value()); err != nil {
		return fmt.Errorf("failed to sign Buzz agent profile: %w", err)
	}
	if _, err := g.client.Publish(ctx, event); err != nil {
		return fmt.Errorf("failed to publish Buzz agent profile: %w", err)
	}
	return nil
}

// publishAgentProfileBestEffort retries publishAgentProfile with bounded
// backoff, since the relay connection is dialed asynchronously by
// nostr.Client.Start and may not be up yet on the first attempt. A
// persistent failure is logged, not escalated — this is a low-stakes
// self-declaration, not something message send/receive correctness depends
// on.
func (g *Gateway) publishAgentProfileBestEffort(ctx context.Context) {
	backoff := buzzProfilePublishInitialBackoff
	for attempt := 1; attempt <= buzzProfilePublishMaxAttempts; attempt++ {
		if err := g.publishAgentProfile(ctx); err == nil {
			return
		} else if attempt == buzzProfilePublishMaxAttempts {
			slog.Warn("Failed to publish Buzz agent profile after retries", "attempts", attempt, "error", err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// Stop gracefully shuts down the Buzz gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		slog.Info("Stopping gateway", "platform", "buzz")
		g.cancel()
	}
	if g.client != nil {
		g.client.Stop()
	}
	return nil
}

// Send publishes msg as a signed kind:9 Buzz channel message (FR-008). The
// target channel comes from msg.Metadata["channel_id"] (data-dictionary.md's
// documented OutgoingMessage.Metadata contract for Buzz).
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	channelID, _ := msg.Metadata["channel_id"].(string)
	if channelID == "" {
		return fmt.Errorf(`Buzz Send requires metadata["channel_id"]`)
	}

	event, err := g.buildSignedChannelMessage(channelID, msg.Content)
	if err != nil {
		return err
	}

	if _, err := g.client.Publish(ctx, event); err != nil {
		metrics.BuzzEventsPublishedTotal.WithLabelValues("failure").Inc()
		return fmt.Errorf("failed to publish Buzz message: %w", err)
	}
	metrics.BuzzEventsPublishedTotal.WithLabelValues("success").Inc()
	return nil
}

// buildSignedChannelMessage constructs and signs a kind:9 Buzz channel
// message event carrying the required #h channel tag.
func (g *Gateway) buildSignedChannelMessage(channelID, content string) (nostr.Event, error) {
	event := nostr.Event{
		CreatedAt: time.Now().Unix(),
		Kind:      nostr.KindChannelMessage,
		Tags:      [][]string{{nostr.ChannelTagName, channelID}},
		Content:   content,
	}
	if err := nostr.Sign(&event, g.config.PrivateKey.Value()); err != nil {
		return nostr.Event{}, fmt.Errorf("failed to sign Buzz message: %w", err)
	}
	return event, nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}

// handleEvents reads from the Nostr client's merged event stream until ctx
// is canceled or the stream closes.
func (g *Gateway) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case recv, ok := <-g.client.Events():
			if !ok {
				return
			}
			g.processEvent(ctx, recv)
		}
	}
}

// processEvent runs a single received event through the Phase 1 pipeline:
// dedupe (FR-004) → verify (FR-003) → map to domain.IncomingMessage (FR-005)
// → RBAC user resolution (FR-006) → message handler dispatch.
func (g *Gateway) processEvent(ctx context.Context, recv nostr.ReceivedEvent) {
	if g.isDuplicate(recv.Event.ID) {
		return
	}

	valid, err := nostr.Verify(recv.Event)
	if err != nil || !valid {
		metrics.BuzzSignatureVerificationFailuresTotal.Inc()
		slog.Warn("Dropped Buzz event failing signature verification",
			"event_id", recv.Event.ID,
			"relay_url", recv.RelayURL,
			"error", err,
		)
		return
	}

	channelID := extractChannelID(recv.Event)
	// Best-effort default: Buzz has no per-message agent-identity tag —
	// accurate detection requires NIP-29 membership-role tracking or a
	// kind:10100 profile lookup, out of scope for Phase 1's infrastructure
	// (see research.md Q5). Flagged for Phase 2's loop-prevention guard.
	senderIsAgent := false

	metrics.BuzzEventsReceivedTotal.WithLabelValues(channelID, strconv.FormatBool(senderIsAgent)).Inc()

	if g.userService != nil {
		if err := g.resolveUser(ctx, recv.Event.PubKey); err != nil {
			slog.Error("Failed to resolve Buzz user", "pubkey", recv.Event.PubKey, "error", err)
		}
	}

	incomingMsg := domain.IncomingMessage{
		ID:          recv.Event.ID,
		Platform:    domain.PlatformBuzz,
		PlatformUID: recv.Event.PubKey,
		Text:        recv.Event.Content,
		Timestamp:   time.Unix(recv.Event.CreatedAt, 0),
		Metadata: map[string]any{
			"event_id":        recv.Event.ID,
			"relay_url":       recv.RelayURL,
			"sender_pubkey":   recv.Event.PubKey,
			"sender_is_agent": senderIsAgent,
			"channel_id":      channelID,
			"signature":       recv.Event.Sig,
		},
	}

	if g.messageHandler != nil {
		if err := g.messageHandler(ctx, incomingMsg); err != nil {
			slog.Error("Error handling message", "platform", "buzz", "error", err)
		}
	}
}

// isDuplicate reports whether eventID has already been processed, recording
// it as seen if not (FR-004: dedupe events by event ID across relays).
func (g *Gateway) isDuplicate(eventID string) bool {
	g.seenMu.Lock()
	defer g.seenMu.Unlock()
	if _, exists := g.seen[eventID]; exists {
		return true
	}
	g.seen[eventID] = struct{}{}
	return false
}

// resolveUser resolves or creates a domain.User for (PlatformBuzz, pubkey),
// defaulting new users to RoleGuest (FR-006). A conflict (another event for
// the same brand-new pubkey won the race to create it first) is treated as
// success, not an error.
func (g *Gateway) resolveUser(ctx context.Context, pubkey string) error {
	if _, err := g.userService.GetUserByPlatformUID(ctx, domain.PlatformBuzz, pubkey); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return fmt.Errorf("failed to look up Buzz user: %w", err)
	}

	_, err := g.userService.CreateUser(ctx, domain.PlatformBuzz, pubkey, domain.RoleGuest)
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		return fmt.Errorf("failed to create Buzz user: %w", err)
	}
	return nil
}

// extractChannelID returns the value of e's channel tag (nostr.ChannelTagName),
// or "" if absent.
func extractChannelID(e nostr.Event) string {
	channelID, _ := e.Tag(nostr.ChannelTagName)
	return channelID
}
