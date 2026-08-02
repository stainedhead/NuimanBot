package nostr

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultEventBufferSize = 256

// ReceivedEvent pairs a Nostr event with the relay URL it arrived from, so
// callers can attribute/dedupe across relays (FR-004).
type ReceivedEvent struct {
	Event    Event
	RelayURL string
}

// ClientOption configures optional Client behavior.
type ClientOption func(*Client)

// WithBackoff overrides the reconnect backoff bounds (default 500ms..30s).
// Primarily useful for tests that need faster reconnect cycles than
// production defaults.
func WithBackoff(initial, max time.Duration) ClientOption {
	return func(c *Client) {
		c.initialBackoff = initial
		c.maxBackoff = max
	}
}

// Client manages WebSocket connections to one or more Nostr relays: connect,
// bounded-exponential-backoff reconnect-on-drop, and a merged inbound event
// stream. Each configured relay gets exactly one long-lived goroutine (plus,
// while connected, one short-lived watcher goroutine to unblock a pending
// read on Stop), so N configured relays cause bounded, not unbounded,
// resource growth.
type Client struct {
	relayURLs      []string
	events         chan ReceivedEvent
	initialBackoff time.Duration
	maxBackoff     time.Duration
	dialer         *websocket.Dialer

	cancel context.CancelFunc
	wg     sync.WaitGroup

	connsMu sync.Mutex
	conns   map[string]*relayConn
}

// relayConn pairs a live relay WebSocket connection with a write mutex.
// Gorilla's websocket.Conn supports one concurrent reader and one concurrent
// writer; the read loop (runConnection) is the sole reader, and writeMu
// serializes the initial subscription write against any concurrent Publish
// calls (the sole writer role, shared across possibly-concurrent Publish
// callers).
type relayConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// NewClient creates a Client for the given relay URLs. Call Start to begin
// connecting; events arrive on Events().
func NewClient(relayURLs []string, opts ...ClientOption) *Client {
	c := &Client{
		relayURLs:      relayURLs,
		events:         make(chan ReceivedEvent, defaultEventBufferSize),
		initialBackoff: 500 * time.Millisecond,
		maxBackoff:     30 * time.Second,
		dialer:         websocket.DefaultDialer,
		conns:          make(map[string]*relayConn),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Events returns the merged inbound event channel across all relays. The
// channel is closed once Stop has fully shut down all relay connections.
func (c *Client) Events() <-chan ReceivedEvent {
	return c.events
}

// Start connects to all configured relays and subscribes with filter,
// launching one goroutine per relay. Unreachable relays are retried with
// backoff in the background and do not fail Start — partial connectivity is
// not a startup failure (NFR).
func (c *Client) Start(ctx context.Context, subscriptionID string, filter Filter) error {
	reqFrame, err := NewSubscriptionRequest(subscriptionID, filter)
	if err != nil {
		return fmt.Errorf("failed to build subscription request: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	for _, relayURL := range c.relayURLs {
		c.wg.Add(1)
		go c.runRelay(ctx, relayURL, reqFrame)
	}
	return nil
}

// Stop cancels all relay connections and waits for their goroutines to exit
// before closing the event channel.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	close(c.events)
}

// runRelay owns the reconnect loop for a single relay: dial, subscribe, read
// until the connection drops or ctx is canceled, then retry with bounded
// exponential backoff.
func (c *Client) runRelay(ctx context.Context, relayURL string, reqFrame []byte) {
	defer c.wg.Done()

	backoff := c.initialBackoff
	for ctx.Err() == nil {
		conn, _, err := c.dialer.DialContext(ctx, relayURL, nil)
		if err != nil {
			if !sleepOrDone(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, c.maxBackoff)
			continue
		}

		backoff = c.initialBackoff // reset after a successful connect
		c.runConnection(ctx, conn, relayURL, reqFrame)

		if ctx.Err() != nil {
			return
		}
		if !sleepOrDone(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, c.maxBackoff)
	}
}

// runConnection writes the subscription request and reads events until the
// connection drops or ctx is canceled. A short-lived watcher goroutine
// closes conn on ctx cancellation to unblock a pending ReadMessage call.
func (c *Client) runConnection(ctx context.Context, conn *websocket.Conn, relayURL string, reqFrame []byte) {
	defer func() { _ = conn.Close() }()

	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-watcherDone:
		}
	}()

	rc := &relayConn{conn: conn}
	if err := conn.WriteMessage(websocket.TextMessage, reqFrame); err != nil {
		return
	}

	c.registerConn(relayURL, rc)
	defer c.unregisterConn(relayURL, rc)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		event, ok, err := parseEventFrame(data)
		if err != nil || !ok {
			continue // ignore malformed frames and non-EVENT relay messages (EOSE/NOTICE/OK)
		}

		select {
		case c.events <- ReceivedEvent{Event: event, RelayURL: relayURL}:
		case <-ctx.Done():
			return
		}
	}
}

// registerConn records relayURL as currently connected, making it a target
// for Publish. Multiple relay URLs may repeat (reconnects); the latest
// connection for a URL wins.
func (c *Client) registerConn(relayURL string, rc *relayConn) {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()
	c.conns[relayURL] = rc
}

// unregisterConn removes relayURL from the connected set, but only if it
// still points at rc — guards against a fresh reconnect's registerConn being
// clobbered by an older connection's delayed unregister.
func (c *Client) unregisterConn(relayURL string, rc *relayConn) {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()
	if c.conns[relayURL] == rc {
		delete(c.conns, relayURL)
	}
}

// ConnectedRelayCount returns the number of relays currently connected. Safe
// to call concurrently.
func (c *Client) ConnectedRelayCount() int {
	c.connsMu.Lock()
	defer c.connsMu.Unlock()
	return len(c.conns)
}

// Publish serializes event as an outgoing NIP-01 ["EVENT", event] client
// frame and writes it to every currently connected relay. Partial failure
// (some relays down or slow) does not fail the whole call as long as at
// least one relay accepts the write — consistent with Start's
// partial-connectivity NFR. Publish returns the number of relays the write
// succeeded on, and an error only when every currently connected relay's
// write failed (including when zero relays are connected).
func (c *Client) Publish(ctx context.Context, event Event) (int, error) {
	frame, err := json.Marshal([]any{"EVENT", event})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal EVENT frame: %w", err)
	}

	c.connsMu.Lock()
	targets := make(map[string]*relayConn, len(c.conns))
	for relayURL, rc := range c.conns {
		targets[relayURL] = rc
	}
	c.connsMu.Unlock()

	if len(targets) == 0 {
		return 0, fmt.Errorf("no Buzz relays currently connected")
	}

	var succeeded int
	var lastErr error
	for relayURL, rc := range targets {
		rc.writeMu.Lock()
		writeErr := rc.conn.WriteMessage(websocket.TextMessage, frame)
		rc.writeMu.Unlock()
		if writeErr != nil {
			lastErr = fmt.Errorf("relay %s: %w", relayURL, writeErr)
			continue
		}
		succeeded++
	}
	if succeeded == 0 {
		return 0, lastErr
	}
	return succeeded, nil
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// parseEventFrame parses a NIP-01 relay message and, if it is an
// ["EVENT", <subscription_id>, <event>] frame, decodes the event. Other
// frame types (EOSE, NOTICE, OK) and malformed input are reported via the
// bool/error return without panicking.
func parseEventFrame(data []byte) (Event, bool, error) {
	var frame []json.RawMessage
	if err := json.Unmarshal(data, &frame); err != nil {
		return Event{}, false, err
	}
	if len(frame) < 3 {
		return Event{}, false, nil
	}

	var msgType string
	if err := json.Unmarshal(frame[0], &msgType); err != nil {
		return Event{}, false, err
	}
	if msgType != "EVENT" {
		return Event{}, false, nil
	}

	var wire struct {
		ID        string     `json:"id"`
		PubKey    string     `json:"pubkey"`
		CreatedAt int64      `json:"created_at"`
		Kind      int        `json:"kind"`
		Tags      [][]string `json:"tags"`
		Content   string     `json:"content"`
		Sig       string     `json:"sig"`
	}
	if err := json.Unmarshal(frame[2], &wire); err != nil {
		return Event{}, false, err
	}

	return Event{
		ID:        wire.ID,
		PubKey:    wire.PubKey,
		CreatedAt: wire.CreatedAt,
		Kind:      wire.Kind,
		Tags:      wire.Tags,
		Content:   wire.Content,
		Sig:       wire.Sig,
	}, true, nil
}
