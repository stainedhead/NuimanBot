package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialWebSocket upgrades a connection to srv's /ws endpoint using cookie as
// the session cookie (nil for an unauthenticated dial). srv must be an
// httptest.Server wrapping a *Server's real HTTP handler — a websocket
// handshake needs a real hijackable net.Conn, which httptest.NewRecorder
// cannot provide.
func dialWebSocket(t *testing.T, srv *httptest.Server, cookie *http.Cookie) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	header := http.Header{}
	if cookie != nil {
		header.Set("Cookie", cookie.String())
	}
	return websocket.DefaultDialer.Dial(wsURL, header)
}

func TestHandleWebSocket_RequiresAuth(t *testing.T) {
	server := NewServer(":0")
	httpSrv := httptest.NewServer(server.httpServer.Handler)
	defer httpSrv.Close()

	conn, resp, err := dialWebSocket(t, httpSrv, nil)
	if err == nil {
		conn.Close()
		t.Fatal("expected the handshake to fail for an unauthenticated request")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		status := -1
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("expected HTTP 401 on the rejected handshake, got %d", status)
	}
}

func TestHandleWebSocket_UpgradesAndReceivesPublishedEvent(t *testing.T) {
	server := NewServer(":0")
	httpSrv := httptest.NewServer(server.httpServer.Handler)
	defer httpSrv.Close()

	cookie := sessionCookieFor(server, "alice", "user")
	conn, _, err := dialWebSocket(t, httpSrv, cookie)
	if err != nil {
		t.Fatalf("expected the handshake to succeed for an authenticated request: %v", err)
	}
	defer conn.Close()

	// Wait for registration to land before publishing, to avoid a race
	// against the handler goroutine's hub.register call.
	deadline := time.Now().Add(2 * time.Second)
	for server.Hub().ConnectionCount("alice") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the connection to register in the hub")
		}
		time.Sleep(5 * time.Millisecond)
	}

	count := 3
	server.Hub().Publish("alice", RunEvent{Type: "notification_badge", UnnotifiedCount: &count})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected to receive the published event, got error: %v", err)
	}
	if !strings.Contains(string(msg), `"notification_badge"`) || !strings.Contains(string(msg), `"unnotifiedCount":3`) {
		t.Fatalf("unexpected message payload: %s", msg)
	}
}

func TestHub_PerUserIsolation(t *testing.T) {
	server := NewServer(":0")
	httpSrv := httptest.NewServer(server.httpServer.Handler)
	defer httpSrv.Close()

	aliceConn, _, err := dialWebSocket(t, httpSrv, sessionCookieFor(server, "alice", "user"))
	if err != nil {
		t.Fatalf("alice dial failed: %v", err)
	}
	defer aliceConn.Close()

	bobConn, _, err := dialWebSocket(t, httpSrv, sessionCookieFor(server, "bob", "user"))
	if err != nil {
		t.Fatalf("bob dial failed: %v", err)
	}
	defer bobConn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for server.Hub().ConnectionCount("alice") == 0 || server.Hub().ConnectionCount("bob") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for both connections to register")
		}
		time.Sleep(5 * time.Millisecond)
	}

	server.Hub().Publish("alice", RunEvent{Type: "run_status", RunID: "run-1", Status: "completed"})

	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := aliceConn.ReadMessage()
	if err != nil {
		t.Fatalf("expected alice to receive her own event: %v", err)
	}
	if !strings.Contains(string(msg), "run-1") {
		t.Fatalf("unexpected payload for alice: %s", msg)
	}

	// bob must never see alice's event: per-user isolation (mirrors the
	// repository-layer IDOR->ErrNotFound posture applied everywhere else
	// in this codebase's Job/Chore/Run access paths).
	_ = bobConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := bobConn.ReadMessage(); err == nil {
		t.Fatal("expected bob's connection to receive nothing (no cross-user delivery), but a message arrived")
	}
}

func TestHub_PublishOnNilHubIsNoOp(t *testing.T) {
	var hub *Hub
	// Must not panic.
	hub.Publish("alice", RunEvent{Type: "run_status"})
	if got := hub.ConnectionCount("alice"); got != 0 {
		t.Fatalf("expected 0 connections from a nil hub, got %d", got)
	}
}

func TestHub_PublishDropsForSlowClientWithoutBlocking(t *testing.T) {
	hub := NewHub()
	client := &wsClient{ownerUserID: "alice", send: make(chan []byte, 1)}
	hub.register(client)
	defer hub.unregister(client)

	// Fill the buffer, then publish clientSendBuffer+ events. None of these
	// calls may block, even though nothing ever drains client.send.
	done := make(chan struct{})
	go func() {
		for i := 0; i < clientSendBuffer+5; i++ {
			hub.Publish("alice", RunEvent{Type: "run_log", LogChunk: "x"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow/unread client instead of dropping the event")
	}
}
