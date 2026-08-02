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

	// buzzRelayConnectionsPollInterval bounds how often Start's background
	// loop samples client.ConnectedRelayCount() to refresh the
	// buzz_relay_connections gauge (FR-004). nostrClient exposes only an
	// aggregate connected-relay count, not connect/disconnect callbacks, so
	// periodic polling — not event-driven updates — is what's available
	// from the adapter layer.
	buzzRelayConnectionsPollInterval = 250 * time.Millisecond
)

// nostrClient is the subset of *nostr.Client's methods Gateway depends on.
// Defined here (rather than referencing *nostr.Client directly) so tests can
// substitute a fake whose Start fails — nostr.Client.Start only fails when
// its internal REQ-frame marshaling errors, which no valid Filter built from
// gateway.go's own inputs can trigger, making that error-wrap path
// (FR-003) otherwise untestable.
type nostrClient interface {
	Start(ctx context.Context, subscriptionID string, filters ...nostr.Filter) error
	Stop()
	Publish(ctx context.Context, event nostr.Event) (int, error)
	Events() <-chan nostr.ReceivedEvent
	ConnectedRelayCount() int
}

// newNostrClient constructs the nostrClient used by Start. A package
// variable so tests can substitute a fake (see nostrClient's doc comment).
var newNostrClient = func(relayURLs []string) nostrClient {
	return nostr.NewClient(relayURLs)
}

// buzzUserService is the subset of *user.Service's methods Gateway depends
// on for RBAC user resolution (FR-006). A small consumer-side interface
// (FR-011) rather than the concrete *user.Service type, matching the
// interface-segregation pattern chat.UserService already uses in
// usecase/chat/service.go for the same two methods.
type buzzUserService interface {
	GetUserByPlatformUID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error)
	CreateUser(ctx context.Context, platform domain.Platform, platformUID string, role domain.Role) (*domain.User, error)
}

// Gateway implements domain.Gateway for Buzz.
type Gateway struct {
	config *config.BuzzConfig

	// userService is used solely by resolveUser (FR-006), which is a
	// deliberately retained duplicate of chat.Service.resolveUser
	// (usecase/chat/service.go) — see resolveUser's doc comment (FR-016)
	// for why it wasn't removed in the auto-review remediation pass.
	userService buzzUserService

	// mu guards client, messageHandler, cancel, and stopped — written from
	// Start()/OnMessage()/Stop() and read from Send()/Stop()/handleEvents(),
	// which can run concurrently with Start() (FR-008: previously plain
	// fields, a latent data race under any caller that doesn't serialize
	// Start() before Send()/Stop()/OnMessage()).
	mu             sync.RWMutex
	client         nostrClient
	messageHandler domain.MessageHandler
	cancel         context.CancelFunc
	stopped        bool // guards Stop() so a second call is a safe no-op (FR-002) rather than a double-close panic in nostr.Client.Stop()

	seenMu sync.Mutex
	seen   map[string]struct{} // dedupe by event ID (FR-004)

	agentCache *agentCache // pubkey→is_agent, from kind:9000/kind:10100 (P2.3)
	loopGuard  *loopGuard  // runaway agent-to-agent reply prevention (P2.4/FR-009)
}

// New creates a new Buzz gateway. userService resolves/creates
// domain.User records for Buzz senders (FR-006); it may be nil in contexts
// (e.g. some tests) that don't exercise RBAC user resolution. Callers
// typically pass a *user.Service, which satisfies buzzUserService.
func New(cfg *config.BuzzConfig, userService buzzUserService) (*Gateway, error) {
	return &Gateway{
		config:      cfg,
		userService: userService,
		seen:        make(map[string]struct{}),
		agentCache:  newAgentCache(),
		loopGuard:   newLoopGuard(buzzLoopGuardMaxConsecutiveAgent, buzzLoopGuardWindow),
	}, nil
}

// Platform returns the platform identifier for Buzz.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformBuzz
}

// getClient returns the current nostrClient, or nil if Start() hasn't
// completed (or the gateway was never started). Safe to call concurrently
// with Start()/Stop() (FR-008).
func (g *Gateway) getClient() nostrClient {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.client
}

// getMessageHandler returns the currently registered handler, or nil if
// OnMessage() hasn't been called. Safe to call concurrently with OnMessage()
// (FR-008).
func (g *Gateway) getMessageHandler() domain.MessageHandler {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.messageHandler
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
	g.mu.Lock()
	g.cancel = cancel
	g.mu.Unlock()

	client := newNostrClient(g.config.Relays)
	if err := client.Start(ctx, buzzSubscriptionID,
		nostr.NewChannelFilter(g.config.ChannelIDs),
		nostr.NewMembershipFilter(g.config.ChannelIDs),
		nostr.NewAgentProfileFilter(),
	); err != nil {
		cancel()
		return fmt.Errorf("failed to start Nostr client: %w", err)
	}
	// g.client is only set once the underlying client has actually started
	// (FR-003) — a failed Start() above leaves g.client nil, so a subsequent
	// Send()/Stop() call hits FR-001's nil-guard instead of operating on a
	// half-initialized client.
	g.mu.Lock()
	g.client = client
	g.mu.Unlock()

	slog.Info("Gateway started",
		"platform", "buzz",
		"relays", len(g.config.Relays),
		"channels", len(g.config.ChannelIDs),
	)

	go g.publishAgentProfileBestEffort(ctx)
	go monitorRelayConnections(ctx, client)

	g.handleEvents(ctx, client)
	return nil
}

// monitorRelayConnections periodically refreshes the buzz_relay_connections
// gauge from client.ConnectedRelayCount() until ctx is canceled (FR-004).
func monitorRelayConnections(ctx context.Context, client nostrClient) {
	ticker := time.NewTicker(buzzRelayConnectionsPollInterval)
	defer ticker.Stop()
	for {
		metrics.BuzzRelayConnections.Set(float64(client.ConnectedRelayCount()))
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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
	client := g.getClient()
	if client == nil {
		return fmt.Errorf("Buzz client not initialized")
	}
	if _, err := client.Publish(ctx, event); err != nil {
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

// Stop gracefully shuts down the Buzz gateway: cancels Start()'s context and
// stops the underlying Nostr client. Idempotent (FR-002) — a second call is
// a safe no-op, since nostr.Client.Stop() itself is not safe to call twice
// (it closes its event channel, which would panic on a repeat close).
func (g *Gateway) Stop(ctx context.Context) error {
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		return nil
	}
	g.stopped = true
	cancel := g.cancel
	client := g.client
	g.mu.Unlock()

	if cancel != nil {
		slog.Info("Stopping gateway", "platform", "buzz")
		cancel()
	}
	if client != nil {
		client.Stop()
	}
	return nil
}

// Send publishes msg as a signed kind:9 Buzz channel message (FR-008). The
// target channel comes from msg.Metadata["channel_id"] (data-dictionary.md's
// documented OutgoingMessage.Metadata contract for Buzz).
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	client := g.getClient()
	if client == nil {
		return fmt.Errorf("Buzz client not initialized: gateway must be started before Send() (FR-001)")
	}

	channelID, _ := msg.Metadata["channel_id"].(string)
	if channelID == "" {
		return fmt.Errorf(`Buzz Send requires metadata["channel_id"]`)
	}

	event, err := g.buildSignedChannelMessage(channelID, msg.Content)
	if err != nil {
		return err
	}

	if _, err := client.Publish(ctx, event); err != nil {
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
	g.mu.Lock()
	g.messageHandler = handler
	g.mu.Unlock()
}

// handleEvents reads from client's merged event stream until ctx is canceled
// or the stream closes.
func (g *Gateway) handleEvents(ctx context.Context, client nostrClient) {
	events := client.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case recv, ok := <-events:
			if !ok {
				return
			}
			g.processEvent(ctx, recv)
		}
	}
}

// processEvent runs a single received event through dedupe (FR-004) and
// signature verification (FR-003) — applied uniformly to every event kind
// this gateway acts on, not just kind:9 channel messages, since trusting an
// unverified kind:9000/kind:10100 event would let a forged event corrupt
// agentCache (P2.3) or bypass the loop-prevention guard (P2.4) — then
// dispatches to the kind-specific handler.
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

	switch recv.Event.Kind {
	case nostr.KindChannelMembership, nostr.KindAgentProfile:
		g.handleAgentStatusEvent(recv.Event)
	case nostr.KindChannelMessage:
		g.processChannelMessage(ctx, recv)
	}
}

// processChannelMessage runs a verified kind:9 event through the rest of the
// Phase 1/2 pipeline: map to domain.IncomingMessage (FR-005) → RBAC user
// resolution (FR-006) → loop-prevention guard (FR-009) → message handler
// dispatch.
func (g *Gateway) processChannelMessage(ctx context.Context, recv nostr.ReceivedEvent) {
	channelID := extractChannelID(recv.Event)
	senderIsAgent := g.agentCache.IsAgent(recv.Event.PubKey)

	metrics.BuzzEventsReceivedTotal.WithLabelValues(channelID, strconv.FormatBool(senderIsAgent)).Inc()

	if g.userService != nil {
		if err := g.resolveUser(ctx, recv.Event.PubKey); err != nil {
			slog.Error("Failed to resolve Buzz user", "pubkey", recv.Event.PubKey, "error", err)
		}
	}

	timestamp := time.Unix(recv.Event.CreatedAt, 0)
	if !g.loopGuard.Allow(channelID, senderIsAgent, timestamp) {
		slog.Warn("Buzz loop-prevention guard suppressed message",
			"channel_id", channelID,
			"sender_pubkey", recv.Event.PubKey,
		)
		return
	}

	incomingMsg := domain.IncomingMessage{
		ID:          recv.Event.ID,
		Platform:    domain.PlatformBuzz,
		PlatformUID: recv.Event.PubKey,
		Text:        recv.Event.Content,
		Timestamp:   timestamp,
		Metadata: map[string]any{
			"event_id":        recv.Event.ID,
			"relay_url":       recv.RelayURL,
			"sender_pubkey":   recv.Event.PubKey,
			"sender_is_agent": senderIsAgent,
			"channel_id":      channelID,
			"signature":       recv.Event.Sig,
		},
	}

	if handler := g.getMessageHandler(); handler != nil {
		if err := handler(ctx, incomingMsg); err != nil {
			slog.Error("Error handling message", "platform", "buzz", "error", err)
		}
	}
}

// handleAgentStatusEvent updates agentCache from a verified kind:9000
// (channel membership) or kind:10100 (agent profile) event (P2.3).
func (g *Gateway) handleAgentStatusEvent(e nostr.Event) {
	switch e.Kind {
	case nostr.KindChannelMembership:
		g.handleMembershipEvent(e)
	case nostr.KindAgentProfile:
		// The mere presence of a kind:10100 event for a pubkey is the
		// agent-identity signal — see implementation-notes.md P2.2 for why
		// its content (channel_add_policy) is not itself an identity field.
		g.agentCache.Set(e.PubKey, true)
	}
}

// handleMembershipEvent updates agentCache from a kind:9000 event's target
// member (nostr.PubkeyTagName) and role (nostr.RoleTagName) tags. An event
// with no role tag requests no role change (the relay preserves the
// member's current role) and is ignored here rather than treated as a
// non-agent signal.
func (g *Gateway) handleMembershipEvent(e nostr.Event) {
	target, ok := e.Tag(nostr.PubkeyTagName)
	if !ok {
		return
	}
	role, ok := e.Tag(nostr.RoleTagName)
	if !ok {
		return
	}
	g.agentCache.Set(target, role == nostr.RoleBot)
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
//
// This duplicates chat.Service.resolveUser (usecase/chat/service.go), which
// now performs the identical (platform, platformUID) resolution/creation
// for every gateway's message once OnMessage's handler dispatches to
// ChatService.ProcessMessage (see cmd/nuimanbot/main.go's gw.OnMessage
// wiring) — so this call's return value is discarded and has no effect on
// message handling; the second resolveUser call typically just finds the
// user ChatService already created. FR-016 (see
// specs/260802-nuimanbot-support-buzz-auto-review/) flagged this as a minor
// architectural asymmetry worth removing. It's kept here rather than
// removed because doing so would require dropping New()'s userService
// parameter, which cmd/nuimanbot/main.go's construction call depends on —
// out of scope for this fix to touch. Safe to delete in a follow-up that
// also updates that call site.
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
