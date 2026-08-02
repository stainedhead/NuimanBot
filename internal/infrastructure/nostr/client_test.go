package nostr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"nuimanbot/internal/infrastructure/nostr"
)

// newFakeRelay starts an in-process WebSocket server that upgrades every
// connection and hands it to handler. The server is closed automatically at
// test cleanup.
func newFakeRelay(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		handler(conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// wsURL converts an httptest server's http:// URL to ws://.
func wsURL(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	u.Scheme = "ws"
	return u.String()
}

func sampleWireEventJSON(id string) map[string]any {
	return map[string]any{
		"id":         id,
		"pubkey":     "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
		"created_at": 1700000000,
		"kind":       9,
		"tags":       [][]string{{"h", "channel-uuid-1"}},
		"content":    "hello channel",
		"sig":        "deadbeef",
	}
}

func TestClient_ConnectsAndReceivesEvents(t *testing.T) {
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		if _, _, err := conn.ReadMessage(); err != nil {
			return // REQ frame
		}
		frame, err := json.Marshal([]any{"EVENT", "sub-1", sampleWireEventJSON("abc123")})
		if err != nil {
			t.Errorf("failed to marshal fake EVENT frame: %v", err)
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, frame)
		time.Sleep(200 * time.Millisecond) // keep the connection open for the test to read
	})

	client := nostr.NewClient([]string{wsURL(t, relay)})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Start(ctx, "sub-1", nostr.NewChannelFilter([]string{"channel-uuid-1"})); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Stop()

	select {
	case recv := <-client.Events():
		if recv.Event.ID != "abc123" {
			t.Errorf("got event ID %q, want %q", recv.Event.ID, "abc123")
		}
		if recv.RelayURL == "" {
			t.Error("expected RelayURL to be populated")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestClient_ReconnectsOnDropWithBoundedBackoff(t *testing.T) {
	var connectCount int32
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		atomic.AddInt32(&connectCount, 1)
		_, _, _ = conn.ReadMessage() // read REQ, then let handler return (closes conn — simulates a drop)
	})

	client := nostr.NewClient([]string{wsURL(t, relay)}, nostr.WithBackoff(20*time.Millisecond, 100*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Start(ctx, "sub-1", nostr.NewChannelFilter([]string{"channel-uuid-1"})); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Stop()

	time.Sleep(300 * time.Millisecond)

	count := atomic.LoadInt32(&connectCount)
	if count < 2 {
		t.Errorf("expected at least 2 connection attempts (reconnect-on-drop), got %d", count)
	}
	// A tight reconnect loop (no backoff) would produce hundreds of attempts
	// in 300ms; a working bounded backoff caps it far lower.
	if count > 30 {
		t.Errorf("expected bounded reconnect attempts in 300ms, got %d (looks like a tight loop)", count)
	}
}

func TestClient_PartialRelayFailureDoesNotBlockOtherRelays(t *testing.T) {
	goodRelay := newFakeRelay(t, func(conn *websocket.Conn) {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		frame, err := json.Marshal([]any{"EVENT", "sub-1", sampleWireEventJSON("from-good-relay")})
		if err != nil {
			t.Errorf("failed to marshal fake EVENT frame: %v", err)
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, frame)
		time.Sleep(300 * time.Millisecond)
	})

	// Port 1 is a privileged/unassigned port that reliably refuses connections.
	unreachableRelayURL := "ws://127.0.0.1:1"

	client := nostr.NewClient(
		[]string{unreachableRelayURL, wsURL(t, goodRelay)},
		nostr.WithBackoff(20*time.Millisecond, 50*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := client.Start(ctx, "sub-1", nostr.NewChannelFilter([]string{"channel-uuid-1"})); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Stop()

	select {
	case recv := <-client.Events():
		if recv.Event.ID != "from-good-relay" {
			t.Errorf("got event ID %q, want %q", recv.Event.ID, "from-good-relay")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event from reachable relay while another relay is unreachable")
	}
}

func TestClient_BoundedGoroutinesNoLeak(t *testing.T) {
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	before := runtime.NumGoroutine()

	const relayCount = 5
	relayURLs := make([]string, relayCount)
	for i := range relayURLs {
		relayURLs[i] = wsURL(t, relay)
	}

	client := nostr.NewClient(relayURLs)
	ctx, cancel := context.WithCancel(context.Background())

	if err := client.Start(ctx, "sub-1", nostr.NewChannelFilter([]string{"channel-uuid-1"})); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond) // let connections settle

	during := runtime.NumGoroutine()
	if during-before > relayCount*3+5 {
		t.Errorf("goroutine count grew unbounded for %d relays: before=%d during=%d", relayCount, before, during)
	}

	cancel()
	client.Stop()

	time.Sleep(150 * time.Millisecond) // let the runtime reap exited goroutines

	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Errorf("goroutine leak after Stop(): before=%d after=%d", before, after)
	}
}
