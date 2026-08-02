package buzz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/nostr"
	"nuimanbot/internal/usecase/user"
)

// newFakeRelay starts an in-process WebSocket server that upgrades every
// connection and hands it to handler. Mirrors
// internal/infrastructure/nostr/client_test.go's helper of the same shape;
// reimplemented locally since that one is unexported in another package.
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

// stubSecurityService is a no-op domain.SecurityService, sufficient to
// satisfy usecase/user.Service's audit-logging dependency in tests.
type stubSecurityService struct{}

func (stubSecurityService) Encrypt(ctx context.Context, userID string, plaintext []byte) ([]byte, error) {
	return plaintext, nil
}

func (stubSecurityService) Decrypt(ctx context.Context, userID string, ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}

func (stubSecurityService) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	return input, nil
}

func (stubSecurityService) GenerateAPIKey(ctx context.Context) (string, error) {
	return "stub-api-key", nil
}

func (stubSecurityService) Audit(ctx context.Context, event *domain.AuditEvent) error {
	return nil
}

// inMemoryUserRepository is a minimal in-memory implementation of
// user.ExtendedUserRepository for tests, mirroring the shape of
// usecase/user/service_test.go's MockUserRepository (unexported there, so
// reimplemented locally rather than imported across package boundaries).
type inMemoryUserRepository struct {
	users map[string]*domain.User
}

func newInMemoryUserRepository() *inMemoryUserRepository {
	return &inMemoryUserRepository{users: make(map[string]*domain.User)}
}

func (r *inMemoryUserRepository) SaveUser(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}

func (r *inMemoryUserRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *inMemoryUserRepository) GetUserByPlatformID(ctx context.Context, platform domain.Platform, platformUID string) (*domain.User, error) {
	for _, u := range r.users {
		if u.PlatformIDs != nil && u.PlatformIDs[platform] == platformUID {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *inMemoryUserRepository) ListAll(ctx context.Context) ([]*domain.User, error) {
	all := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		all = append(all, u)
	}
	return all, nil
}

func (r *inMemoryUserRepository) Delete(ctx context.Context, userID string) error {
	delete(r.users, userID)
	return nil
}

func newTestGateway(t *testing.T) (*Gateway, *inMemoryUserRepository) {
	t.Helper()
	repo := newInMemoryUserRepository()
	userSvc := user.NewService(repo, stubSecurityService{})
	gw, err := New(&config.BuzzConfig{}, userSvc)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return gw, repo
}

func signedChannelEvent(t *testing.T, channelID, content string) nostr.Event {
	t.Helper()
	privHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	e := nostr.Event{
		CreatedAt: 1700000000,
		Kind:      nostr.KindChannelMessage,
		Tags:      [][]string{{nostr.ChannelTagName, channelID}},
		Content:   content,
	}
	if err := nostr.Sign(&e, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	return e
}

func TestGateway_Platform_ReturnsBuzz(t *testing.T) {
	gw, _ := newTestGateway(t)
	if gw.Platform() != domain.PlatformBuzz {
		t.Errorf("Platform() = %q, want %q", gw.Platform(), domain.PlatformBuzz)
	}
}

func TestGateway_ProcessEvent_ValidSignedEvent_MapsToIncomingMessage(t *testing.T) {
	gw, _ := newTestGateway(t)

	var got domain.IncomingMessage
	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		got = msg
		return nil
	})

	e := signedChannelEvent(t, "channel-uuid-1", "hello channel")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if handlerCalls != 1 {
		t.Fatalf("handler called %d times, want 1", handlerCalls)
	}
	if got.Platform != domain.PlatformBuzz {
		t.Errorf("Platform = %q, want %q", got.Platform, domain.PlatformBuzz)
	}
	if got.PlatformUID != e.PubKey {
		t.Errorf("PlatformUID = %q, want %q", got.PlatformUID, e.PubKey)
	}
	if got.Text != "hello channel" {
		t.Errorf("Text = %q, want %q", got.Text, "hello channel")
	}

	wantMeta := map[string]any{
		"event_id":        e.ID,
		"relay_url":       "wss://relay.example.com",
		"sender_pubkey":   e.PubKey,
		"sender_is_agent": false,
		"channel_id":      "channel-uuid-1",
		"signature":       e.Sig,
	}
	for k, want := range wantMeta {
		if got.Metadata[k] != want {
			t.Errorf("Metadata[%q] = %v, want %v", k, got.Metadata[k], want)
		}
	}
	if len(got.Metadata) != len(wantMeta) {
		t.Errorf("Metadata has %d keys, want %d: %v", len(got.Metadata), len(wantMeta), got.Metadata)
	}
}

func TestGateway_ProcessEvent_UnsignedEvent_DroppedAndMetricIncremented(t *testing.T) {
	gw, _ := newTestGateway(t)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	before := testutil.ToFloat64(metrics.BuzzSignatureVerificationFailuresTotal)

	unsigned := nostr.Event{
		ID:      "fake-id-not-matching-content",
		PubKey:  "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
		Kind:    nostr.KindChannelMessage,
		Tags:    [][]string{{nostr.ChannelTagName, "channel-uuid-1"}},
		Content: "forged message",
		Sig:     "", // unsigned
	}
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: unsigned, RelayURL: "wss://relay.example.com"})

	if handlerCalls != 0 {
		t.Errorf("handler called %d times for unsigned event, want 0", handlerCalls)
	}

	after := testutil.ToFloat64(metrics.BuzzSignatureVerificationFailuresTotal)
	if after != before+1 {
		t.Errorf("BuzzSignatureVerificationFailuresTotal = %v, want %v", after, before+1)
	}
}

func TestGateway_ProcessEvent_ForgedEvent_DroppedAndMetricIncremented(t *testing.T) {
	gw, _ := newTestGateway(t)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	before := testutil.ToFloat64(metrics.BuzzSignatureVerificationFailuresTotal)

	e := signedChannelEvent(t, "channel-uuid-1", "original content")
	e.Content = "tampered content" // ID/Sig no longer match content
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if handlerCalls != 0 {
		t.Errorf("handler called %d times for tampered event, want 0", handlerCalls)
	}
	after := testutil.ToFloat64(metrics.BuzzSignatureVerificationFailuresTotal)
	if after != before+1 {
		t.Errorf("BuzzSignatureVerificationFailuresTotal = %v, want %v", after, before+1)
	}
}

func TestGateway_ProcessEvent_DuplicateEventID_HandlerInvokedOnce(t *testing.T) {
	gw, _ := newTestGateway(t)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	e := signedChannelEvent(t, "channel-uuid-1", "hello channel")

	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay-a.example.com"})
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay-b.example.com"})

	if handlerCalls != 1 {
		t.Errorf("handler called %d times for the same event ID from two relays, want 1", handlerCalls)
	}
}

func TestGateway_ProcessEvent_NewPubkey_CreatesRoleGuestUser(t *testing.T) {
	gw, repo := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	e := signedChannelEvent(t, "channel-uuid-1", "hi")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	u, err := repo.GetUserByPlatformID(context.Background(), domain.PlatformBuzz, e.PubKey)
	if err != nil {
		t.Fatalf("expected a user to be created for new pubkey, got error: %v", err)
	}
	if u.Role != domain.RoleGuest {
		t.Errorf("Role = %q, want %q", u.Role, domain.RoleGuest)
	}
	if u.PlatformIDs[domain.PlatformBuzz] != e.PubKey {
		t.Errorf("PlatformIDs[PlatformBuzz] = %q, want %q", u.PlatformIDs[domain.PlatformBuzz], e.PubKey)
	}
	if u.ID == "" || u.ID == e.PubKey {
		t.Errorf("expected a UUID-assigned User.ID distinct from the pubkey, got %q", u.ID)
	}
}

func TestGateway_ProcessEvent_RepeatPubkey_NoDuplicateUser(t *testing.T) {
	gw, repo := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	first := nostr.Event{CreatedAt: 1700000000, Kind: nostr.KindChannelMessage, Tags: [][]string{{nostr.ChannelTagName, "channel-uuid-1"}}, Content: "first"}
	if err := nostr.Sign(&first, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	second := nostr.Event{CreatedAt: 1700000001, Kind: nostr.KindChannelMessage, Tags: [][]string{{nostr.ChannelTagName, "channel-uuid-1"}}, Content: "second"}
	if err := nostr.Sign(&second, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: first, RelayURL: "wss://relay.example.com"})
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: second, RelayURL: "wss://relay.example.com"})

	all, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	matching := 0
	for _, u := range all {
		if u.PlatformIDs[domain.PlatformBuzz] == pubHex {
			matching++
		}
	}
	if matching != 1 {
		t.Errorf("found %d users for repeat pubkey, want exactly 1 (no duplicate)", matching)
	}
}

// buzzRelayFrame is a helper to decode a client->relay NIP-01 frame's type
// and (for "EVENT" frames) the wrapped event's kind, without needing the
// full nostr.Event shape.
type buzzRelayFrame struct {
	msgType string
	kind    int
	tags    [][]string
	content string
}

func decodeClientFrame(t *testing.T, data []byte) buzzRelayFrame {
	t.Helper()
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal client frame: %v", err)
	}
	var msgType string
	if err := json.Unmarshal(raw[0], &msgType); err != nil {
		t.Fatalf("failed to unmarshal frame type: %v", err)
	}
	f := buzzRelayFrame{msgType: msgType}
	if msgType == "EVENT" && len(raw) >= 2 {
		var wire struct {
			Kind    int        `json:"kind"`
			Tags    [][]string `json:"tags"`
			Content string     `json:"content"`
		}
		if err := json.Unmarshal(raw[1], &wire); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		f.kind, f.tags, f.content = wire.Kind, wire.Tags, wire.Content
	}
	return f
}

func newConnectedTestGateway(t *testing.T, relay *httptest.Server) (*Gateway, string) {
	t.Helper()
	privHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	cfg := &config.BuzzConfig{
		Relays:     []string{wsURL(t, relay)},
		PrivateKey: domain.NewSecureStringFromString(privHex),
		ChannelIDs: []string{"channel-uuid-1"},
	}
	gw, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	gw.client = nostr.NewClient(cfg.Relays)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := gw.client.Start(ctx, buzzSubscriptionID, nostr.NewChannelFilter(cfg.ChannelIDs)); err != nil {
		t.Fatalf("client.Start() error = %v", err)
	}
	t.Cleanup(gw.client.Stop)

	deadline := time.Now().Add(2 * time.Second)
	for gw.client.ConnectedRelayCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if gw.client.ConnectedRelayCount() == 0 {
		t.Fatal("timed out waiting for gateway's relay connection")
	}

	return gw, privHex
}

func TestGateway_Send_PublishesSignedChannelMessage(t *testing.T) {
	frames := make(chan buzzRelayFrame, 4)
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			frames <- decodeClientFrame(t, data)
		}
	})

	gw, _ := newConnectedTestGateway(t, relay)

	before := testutil.ToFloat64(metrics.BuzzEventsPublishedTotal.WithLabelValues("success"))

	err := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "hello from the gateway",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	after := testutil.ToFloat64(metrics.BuzzEventsPublishedTotal.WithLabelValues("success"))
	if after != before+1 {
		t.Errorf("BuzzEventsPublishedTotal{success} = %v, want %v", after, before+1)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case f := <-frames:
			if f.msgType != "EVENT" {
				continue // the initial REQ frame
			}
			if f.kind != nostr.KindChannelMessage {
				t.Errorf("published event kind = %d, want %d", f.kind, nostr.KindChannelMessage)
			}
			if f.content != "hello from the gateway" {
				t.Errorf("published event content = %q, want %q", f.content, "hello from the gateway")
			}
			found := false
			for _, tag := range f.tags {
				if len(tag) == 2 && tag[0] == "h" && tag[1] == "channel-uuid-1" {
					found = true
				}
			}
			if !found {
				t.Errorf("published event missing #h channel-uuid-1 tag: %v", f.tags)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for published EVENT frame at relay")
		}
	}
}

func TestGateway_Send_ProducesVerifiableSignature(t *testing.T) {
	frames := make(chan buzzRelayFrame, 4)
	rawEvents := make(chan []byte, 4)
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			f := decodeClientFrame(t, data)
			frames <- f
			if f.msgType == "EVENT" {
				rawEvents <- data
			}
		}
	})

	gw, _ := newConnectedTestGateway(t, relay)

	if err := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "verify me",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	select {
	case data := <-rawEvents:
		var raw []json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("failed to unmarshal frame: %v", err)
		}
		var e nostr.Event
		if err := json.Unmarshal(raw[1], &e); err != nil {
			t.Fatalf("failed to unmarshal event: %v", err)
		}
		valid, err := nostr.Verify(e)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if !valid {
			t.Error("Verify() = false for a Send()-published event, want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestGateway_Send_MissingChannelID_ReturnsError(t *testing.T) {
	gw, _ := newTestGateway(t)
	err := gw.Send(context.Background(), domain.OutgoingMessage{Content: "no channel"})
	if err == nil {
		t.Error("Send() error = nil, want error when metadata[\"channel_id\"] is missing")
	}
}
