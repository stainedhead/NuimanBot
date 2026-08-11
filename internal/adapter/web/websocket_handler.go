package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RunEvent is a near-real-time push notification about a Job/Chore Run's
// status, log output, or the History notification-badge count (FR-028,
// FR-036, FR-044). Hub.Publish delivers each event only to WebSocket
// connections belonging to the Run's owning user — see Hub's doc comment
// for the per-user isolation rationale.
type RunEvent struct {
	// Type identifies the event shape: "run_status" (Status changed),
	// "run_log" (a log chunk was appended), or "notification_badge" (the
	// History unviewed-run count changed).
	Type string `json:"type"`
	// RunID/SourceType/SourceID identify which Run this event concerns.
	// Empty for a bare "notification_badge" event, which is a per-user
	// count rather than a single-Run update.
	RunID      string `json:"runId,omitempty"`
	SourceType string `json:"sourceType,omitempty"`
	SourceID   string `json:"sourceId,omitempty"`
	// Status is set for "run_status" events.
	Status string `json:"status,omitempty"`
	// LogChunk is set for "run_log" events.
	LogChunk string `json:"logChunk,omitempty"`
	// UnnotifiedCount is set for "notification_badge" events. A pointer so
	// the zero count is still marshaled (omitempty would drop 0).
	UnnotifiedCount *int `json:"unnotifiedCount,omitempty"`
}

// wsClient is a single connected WebSocket client, identified by the
// session's owning user. Writes to conn are serialized entirely through
// send + writePump — gorilla/websocket connections are not safe for
// concurrent writes from multiple goroutines, so nothing else may call
// conn.WriteMessage directly.
type wsClient struct {
	ownerUserID string
	conn        *websocket.Conn
	send        chan []byte
}

// clientSendBuffer bounds how many pending outbound messages a slow client
// can accumulate before Hub.Publish starts dropping events for it rather
// than blocking the publisher (a disconnected-but-not-yet-reaped or slow
// browser tab must never stall Run status updates for other users/tabs).
const clientSendBuffer = 32

// wsWriteWait bounds how long a single WriteMessage call may block.
const wsWriteWait = 10 * time.Second

// wsPongWait is how long a connection may go without a pong before it is
// considered dead. wsPingPeriod must be comfortably less than wsPongWait.
const (
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
)

// Hub tracks connected WebSocket clients keyed by owning user, so a
// RunEvent is only ever delivered to the user who owns the underlying
// Job/Chore Run — never broadcast to every connected client. This mirrors
// every other Job/Chore/Run access path in this codebase (repository-layer
// ownerUserID scoping, IDOR->ErrNotFound semantics; see
// domain/run_repository.go), extended here to the push-notification path.
type Hub struct {
	mu      sync.Mutex
	clients map[string]map[*wsClient]struct{} // ownerUserID -> connections
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[*wsClient]struct{})}
}

// register adds client to the hub under ownerUserID.
func (h *Hub) register(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[client.ownerUserID]
	if !ok {
		set = make(map[*wsClient]struct{})
		h.clients[client.ownerUserID] = set
	}
	set[client] = struct{}{}
}

// unregister removes client from the hub and closes its send channel. Safe
// to call more than once for the same client.
func (h *Hub) unregister(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[client.ownerUserID]
	if !ok {
		return
	}
	if _, present := set[client]; !present {
		return
	}
	delete(set, client)
	if len(set) == 0 {
		delete(h.clients, client.ownerUserID)
	}
	close(client.send)
}

// Publish delivers event to every connection belonging to ownerUserID. A
// nil Hub is valid and Publish becomes a no-op, so callers (e.g.
// notifyingRunRepository) can wrap a repository with a Hub unconditionally,
// including in tests/wiring paths that never construct one.
//
// Delivery to any single slow/stalled client never blocks delivery to
// others: if a client's outbound buffer is full, its event is dropped
// rather than blocking this call (see clientSendBuffer).
func (h *Hub) Publish(ownerUserID string, event RunEvent) {
	if h == nil {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("websocket hub: failed to marshal RunEvent", "error", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients[ownerUserID] {
		select {
		case client.send <- payload:
		default:
			slog.Warn("websocket hub: dropping event for slow client", "ownerUserID", ownerUserID, "type", event.Type)
		}
	}
}

// ConnectionCount returns the number of currently connected clients for
// ownerUserID. Exposed for tests.
func (h *Hub) ConnectionCount(ownerUserID string) int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[ownerUserID])
}

// upgrader configures the WebSocket handshake. CheckOrigin enforces
// same-origin: this server is not behind a trusted reverse proxy (see
// extractRemoteIP's doc comment for the same posture) and has no
// cross-origin embedding use case, so a mismatched Origin header is
// rejected. Requests with no Origin header (non-browser clients, and
// httptest-based tests) are allowed through.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	},
}

// handleWebSocket upgrades an authenticated session to a WebSocket
// connection and registers it in s.hub under the session's owning user, so
// Job/Chore Run status/log/notification-badge updates (published by
// notifyingRunRepository as the worker pool executes runs) reach this
// user's browser tab(s) without polling. Per-user isolation is enforced
// entirely by keying on user.Username, the same stable identifier every
// other environment in this package uses for owned-resource scoping (see
// ChatsService's doc comment).
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if s.hub == nil {
		http.Error(w, "WebSocket hub not configured", http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade failed", "user", user.Username, "error", err)
		return
	}

	client := &wsClient{
		ownerUserID: user.Username,
		conn:        conn,
		send:        make(chan []byte, clientSendBuffer),
	}
	s.hub.register(client)

	go client.writePump()
	client.readPump(s.hub)
}

// readPump reads (and discards) inbound frames solely to detect the
// connection closing and to drive the pong handler's keepalive deadline
// extension. The protocol is server-push only; no client->server message
// type is defined. On return, the client is unregistered and the
// connection closed, which also causes writePump to exit (send channel
// closed).
func (c *wsClient) readPump(hub *Hub) {
	defer func() {
		hub.unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump serializes all writes to c.conn: outbound RunEvents from
// Hub.Publish and periodic pings. Exits when c.send is closed (by
// Hub.unregister) or a write fails.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				// Hub.unregister closed the channel: send a close frame and stop.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
