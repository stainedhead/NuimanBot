package buzz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
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

// sampleWireEventJSON builds a minimal kind:9 wire-shape event (an unverified
// signature is fine — tests using this only care about message volume/
// timing, not whether the gateway forwards the message).
func sampleWireEventJSON(t *testing.T, i int) map[string]any {
	t.Helper()
	return map[string]any{
		"id":         fmt.Sprintf("evt-%d", i),
		"pubkey":     "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459",
		"created_at": 1700000000,
		"kind":       nostr.KindChannelMessage,
		"tags":       [][]string{{"h", "channel-uuid-1"}},
		"content":    fmt.Sprintf("msg %d", i),
		"sig":        "deadbeef",
	}
}

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

func signedMembershipEvent(t *testing.T, channelID, targetPubkey, role string) nostr.Event {
	t.Helper()
	privHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	tags := [][]string{{nostr.ChannelTagName, channelID}, {nostr.PubkeyTagName, targetPubkey}}
	if role != "" {
		tags = append(tags, []string{nostr.RoleTagName, role})
	}
	e := nostr.Event{CreatedAt: 1700000000, Kind: nostr.KindChannelMembership, Tags: tags}
	if err := nostr.Sign(&e, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	return e
}

// signedAgentProfileEvent returns a signed kind:10100 event along with the
// hex pubkey that signed (and thus "owns") it.
func signedAgentProfileEvent(t *testing.T) (nostr.Event, string) {
	t.Helper()
	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	e := nostr.Event{CreatedAt: 1700000000, Kind: nostr.KindAgentProfile, Content: `{"channel_add_policy":"owner_only"}`}
	if err := nostr.Sign(&e, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	return e, pubHex
}

func signedChannelEventFrom(t *testing.T, privHex, channelID, content string) nostr.Event {
	t.Helper()
	e := nostr.Event{CreatedAt: 1700000000, Kind: nostr.KindChannelMessage, Tags: [][]string{{nostr.ChannelTagName, channelID}}, Content: content}
	if err := nostr.Sign(&e, privHex); err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	return e
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

func TestGateway_Send_NilClient_ReturnsErrorNotPanic(t *testing.T) {
	gw, _ := newTestGateway(t) // constructed but never Start()ed: g.client is nil
	err := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "hello",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	})
	if err == nil {
		t.Error("Send() error = nil, want a descriptive error when g.client is nil (FR-001)")
	}
}

func TestGateway_Send_MissingChannelID_ReturnsError(t *testing.T) {
	gw, _ := newTestGateway(t)
	err := gw.Send(context.Background(), domain.OutgoingMessage{Content: "no channel"})
	if err == nil {
		t.Error("Send() error = nil, want error when metadata[\"channel_id\"] is missing")
	}
}

// fakeFailingNostrClient's Start always fails, letting tests exercise
// Start()'s failed-client-Start wrap-and-return path (FR-003) without
// needing an unreachable real error condition.
type fakeFailingNostrClient struct{}

func (fakeFailingNostrClient) Start(ctx context.Context, subscriptionID string, filters ...nostr.Filter) error {
	return fmt.Errorf("simulated Nostr client start failure")
}
func (fakeFailingNostrClient) Stop() {}
func (fakeFailingNostrClient) Publish(ctx context.Context, event nostr.Event) (int, error) {
	return 0, nil
}
func (fakeFailingNostrClient) Events() <-chan nostr.ReceivedEvent { return nil }
func (fakeFailingNostrClient) ConnectedRelayCount() int           { return 0 }

func TestGateway_Start_NoRelays_ReturnsErrorAndLeavesClientNil(t *testing.T) {
	privHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	cfg := &config.BuzzConfig{
		Relays:     nil,
		PrivateKey: domain.NewSecureStringFromString(privHex),
		ChannelIDs: []string{"channel-uuid-1"},
	}
	gw, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = gw.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want an error for a gateway with no configured relays")
	}

	// FR-003: must not leave the gateway partially initialized such that a
	// subsequent Send()/Stop() call could panic (ties to FR-001).
	if sendErr := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "x",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	}); sendErr == nil {
		t.Error("Send() after a failed no-relays Start() error = nil, want error")
	}
	if stopErr := gw.Stop(context.Background()); stopErr != nil {
		t.Errorf("Stop() after a failed no-relays Start() error = %v, want nil", stopErr)
	}
}

func TestGateway_Start_NoPrivateKey_ReturnsErrorAndLeavesClientNil(t *testing.T) {
	cfg := &config.BuzzConfig{
		Relays:     []string{"wss://relay.example.com"},
		PrivateKey: domain.NewSecureStringFromString(""),
		ChannelIDs: []string{"channel-uuid-1"},
	}
	gw, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = gw.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want an error for a gateway with no configured private key")
	}

	if sendErr := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "x",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	}); sendErr == nil {
		t.Error("Send() after a failed no-private-key Start() error = nil, want error")
	}
	if stopErr := gw.Stop(context.Background()); stopErr != nil {
		t.Errorf("Stop() after a failed no-private-key Start() error = %v, want nil", stopErr)
	}
}

func TestGateway_Start_NostrClientStartFailure_ReturnsWrappedErrorAndLeavesClientNil(t *testing.T) {
	origFactory := newNostrClient
	newNostrClient = func(relayURLs []string) nostrClient { return fakeFailingNostrClient{} }
	t.Cleanup(func() { newNostrClient = origFactory })

	privHex, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	cfg := &config.BuzzConfig{
		Relays:     []string{"wss://relay.example.com"},
		PrivateKey: domain.NewSecureStringFromString(privHex),
		ChannelIDs: []string{"channel-uuid-1"},
	}
	gw, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = gw.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want the wrapped Nostr client Start() failure")
	}
	if !strings.Contains(err.Error(), "simulated Nostr client start failure") {
		t.Errorf("Start() error = %q, want it to wrap the underlying client Start() error", err.Error())
	}

	if sendErr := gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "x",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	}); sendErr == nil {
		t.Error("Send() after a failed Start() (client.Start error) = nil, want error")
	}
	if stopErr := gw.Stop(context.Background()); stopErr != nil {
		t.Errorf("Stop() after a failed Start() (client.Start error) = %v, want nil", stopErr)
	}
}

// TestGateway_ConcurrentSendStopDuringStart_NoRace exercises Send() and
// Stop() called concurrently with a live Start() (FR-008). Run with -race:
// the assertion is the absence of a data race/panic, not any particular
// return value from Send()/Stop() (outcomes are inherently timing-dependent
// here).
func TestGateway_ConcurrentSendStopDuringStart_NoRace(t *testing.T) {
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

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
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		_ = gw.Start(context.Background())
	}()
	go func() {
		defer wg.Done()
		_ = gw.Send(context.Background(), domain.OutgoingMessage{
			Content:  "concurrent",
			Metadata: map[string]any{"channel_id": "channel-uuid-1"},
		})
	}()
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		_ = gw.Stop(context.Background())
	}()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent Start/Send/Stop to complete")
	}
}

func TestGateway_Start_RelayConnectionsGauge_ReflectsConnectAndDisconnect(t *testing.T) {
	connCh := make(chan *websocket.Conn, 1)
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		connCh <- conn
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

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
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = gw.Start(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	deadline := time.Now().Add(2 * time.Second)
	for testutil.ToFloat64(metrics.BuzzRelayConnections) != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := testutil.ToFloat64(metrics.BuzzRelayConnections); got != 1 {
		t.Fatalf("BuzzRelayConnections = %v after relay connect, want 1", got)
	}

	var conn *websocket.Conn
	select {
	case conn = <-connCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the relay to record the connection")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("conn.Close() error = %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for testutil.ToFloat64(metrics.BuzzRelayConnections) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := testutil.ToFloat64(metrics.BuzzRelayConnections); got != 0 {
		t.Errorf("BuzzRelayConnections = %v after relay disconnect, want 0", got)
	}
}

func TestGateway_Stop_CancelsStartAndIsIdempotent(t *testing.T) {
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

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
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	startErrCh := make(chan error, 1)
	go func() { startErrCh <- gw.Start(context.Background()) }()

	// Give Start() time to connect before stopping.
	time.Sleep(300 * time.Millisecond)

	if err := gw.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	select {
	case err := <-startErrCh:
		if err != nil {
			t.Errorf("Start() returned error after Stop(): %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start() did not return after Stop() canceled its context")
	}

	// Idempotent: a second Stop() must not panic (nostr.Client.Stop() closes
	// its event channel, which panics on a repeat close) or return an error.
	if err := gw.Stop(context.Background()); err != nil {
		t.Errorf("second Stop() call error = %v, want nil (idempotent)", err)
	}

	// Post-stop state: no relays connected, so Send() must fail gracefully.
	err = gw.Send(context.Background(), domain.OutgoingMessage{
		Content:  "after stop",
		Metadata: map[string]any{"channel_id": "channel-uuid-1"},
	})
	if err == nil {
		t.Error("Send() after Stop() error = nil, want a graceful error (no relays connected)")
	}
}

func TestGateway_PublishAgentProfile_PublishesSignedKind10100Event(t *testing.T) {
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

	if err := gw.publishAgentProfile(context.Background()); err != nil {
		t.Fatalf("publishAgentProfile() error = %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case f := <-frames:
			if f.msgType != "EVENT" {
				continue // the initial REQ frame
			}
			if f.kind != nostr.KindAgentProfile {
				t.Errorf("published event kind = %d, want %d", f.kind, nostr.KindAgentProfile)
			}
			var content map[string]string
			if err := json.Unmarshal([]byte(f.content), &content); err != nil {
				t.Fatalf("failed to unmarshal profile content: %v", err)
			}
			if content["channel_add_policy"] != buzzChannelAddPolicy {
				t.Errorf(`content["channel_add_policy"] = %q, want %q`, content["channel_add_policy"], buzzChannelAddPolicy)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for published kind:10100 event at relay")
		}
	}
}

func TestGateway_PublishAgentProfile_ProducesVerifiableSignature(t *testing.T) {
	rawEvents := make(chan []byte, 4)
	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if decodeClientFrame(t, data).msgType == "EVENT" {
				rawEvents <- data
			}
		}
	})

	gw, _ := newConnectedTestGateway(t, relay)

	if err := gw.publishAgentProfile(context.Background()); err != nil {
		t.Fatalf("publishAgentProfile() error = %v", err)
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
			t.Error("Verify() = false for a published agent profile event, want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestGateway_Start_PublishesAgentProfileOnceNotPerMessage(t *testing.T) {
	var mu sync.Mutex
	profileFrames := 0

	relay := newFakeRelay(t, func(conn *websocket.Conn) {
		if _, _, err := conn.ReadMessage(); err != nil { // REQ frame
			return
		}
		// Simulate several channel messages arriving in quick succession —
		// none of these should trigger another profile publish.
		for i := 0; i < 3; i++ {
			frame, err := json.Marshal([]any{"EVENT", buzzSubscriptionID, sampleWireEventJSON(t, i)})
			if err != nil {
				t.Errorf("failed to marshal fake EVENT frame: %v", err)
				return
			}
			_ = conn.WriteMessage(websocket.TextMessage, frame)
		}
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if f := decodeClientFrame(t, data); f.msgType == "EVENT" && f.kind == nostr.KindAgentProfile {
				mu.Lock()
				profileFrames++
				mu.Unlock()
			}
		}
	})

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
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = gw.Start(ctx)
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	got := profileFrames
	mu.Unlock()
	if got != 1 {
		t.Errorf("published %d kind:10100 profile events during Start(), want exactly 1 (once, not per-message)", got)
	}
}

// --- P2.3: kind:9000/kind:10100 → agentCache ---

func TestGateway_ProcessEvent_MembershipBotRole_MarksTargetAsAgent(t *testing.T) {
	gw, _ := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	target := "target-pubkey-hex"
	e := signedMembershipEvent(t, "channel-uuid-1", target, nostr.RoleBot)
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if !gw.agentCache.IsAgent(target) {
		t.Error("agentCache.IsAgent(target) = false after a kind:9000 Bot-role event, want true")
	}
}

func TestGateway_ProcessEvent_MembershipNonBotRole_MarksTargetNotAgent(t *testing.T) {
	gw, _ := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	target := "target-pubkey-hex"
	gw.agentCache.Set(target, true) // previously known as an agent

	e := signedMembershipEvent(t, "channel-uuid-1", target, "member")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if gw.agentCache.IsAgent(target) {
		t.Error("agentCache.IsAgent(target) = true after a kind:9000 role=member event, want false")
	}
}

func TestGateway_ProcessEvent_MembershipNoRoleTag_DoesNotChangeCache(t *testing.T) {
	gw, _ := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	target := "target-pubkey-hex"
	gw.agentCache.Set(target, true)

	// No role tag = no role change (relay preserves current role).
	e := signedMembershipEvent(t, "channel-uuid-1", target, "")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if !gw.agentCache.IsAgent(target) {
		t.Error("agentCache.IsAgent(target) changed to false after a role-tag-less kind:9000 event, want unchanged (true)")
	}
}

func TestGateway_ProcessEvent_AgentProfileEvent_MarksPublisherAsAgent(t *testing.T) {
	gw, _ := newTestGateway(t)
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error { return nil })

	e, pubkey := signedAgentProfileEvent(t)
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if !gw.agentCache.IsAgent(pubkey) {
		t.Error("agentCache.IsAgent(pubkey) = false after that pubkey published a kind:10100 event, want true")
	}
}

func TestGateway_ProcessEvent_ChannelMessage_SenderIsAgentReflectsCache(t *testing.T) {
	gw, _ := newTestGateway(t)

	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	gw.agentCache.Set(pubHex, true)

	var got domain.IncomingMessage
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		got = msg
		return nil
	})

	e := signedChannelEventFrom(t, privHex, "channel-uuid-1", "hi")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if got.Metadata["sender_is_agent"] != true {
		t.Errorf(`Metadata["sender_is_agent"] = %v, want true (cache-backed)`, got.Metadata["sender_is_agent"])
	}
}

func TestGateway_ProcessEvent_UnknownSender_SenderIsAgentDefaultsFalse(t *testing.T) {
	gw, _ := newTestGateway(t)

	var got domain.IncomingMessage
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		got = msg
		return nil
	})

	e := signedChannelEvent(t, "channel-uuid-1", "hi")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})

	if got.Metadata["sender_is_agent"] != false {
		t.Errorf(`Metadata["sender_is_agent"] = %v, want false for a pubkey never seen in agentCache`, got.Metadata["sender_is_agent"])
	}
}

// --- P2.4: loop-prevention guard, wired through processEvent ---

func TestGateway_ProcessEvent_RunawayAgentChain_Terminates(t *testing.T) {
	gw, _ := newTestGateway(t)

	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	gw.agentCache.Set(pubHex, true)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	const messageCount = 50
	for i := 0; i < messageCount; i++ {
		// All within the same second: a real runaway agent-to-agent loop
		// fires in near-real-time (sub-second turnaround), not spread across
		// tens of real seconds — CreatedAt (NIP-01's second-resolution
		// timestamp) reflects that "rapid succession" here.
		e := nostr.Event{
			CreatedAt: 1700000000,
			Kind:      nostr.KindChannelMessage,
			Tags:      [][]string{{nostr.ChannelTagName, "channel-uuid-1"}},
			Content:   fmt.Sprintf("agent reply #%d", i),
		}
		if err := nostr.Sign(&e, privHex); err != nil {
			t.Fatalf("Sign() error = %v", err)
		}
		gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: e, RelayURL: "wss://relay.example.com"})
	}

	if handlerCalls != buzzLoopGuardMaxConsecutiveAgent {
		t.Errorf("handler invoked %d times for a %d-message runaway agent-to-agent chain, want exactly %d (guard threshold) — a runaway exchange must terminate, not run indefinitely",
			handlerCalls, messageCount, buzzLoopGuardMaxConsecutiveAgent)
	}
}

func TestGateway_ProcessEvent_SingleAgentReply_NotSuppressed(t *testing.T) {
	gw, _ := newTestGateway(t)

	privHex, pubHex, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	gw.agentCache.Set(pubHex, true)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	singleReply := signedChannelEventFrom(t, privHex, "channel-uuid-1", "a single agent reply")
	gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: singleReply, RelayURL: "wss://relay.example.com"})

	if handlerCalls != 1 {
		t.Errorf("handler invoked %d times for a single agent reply, want 1 (must not be suppressed)", handlerCalls)
	}
}

func TestGateway_ProcessEvent_HumanToAgentExchange_NotSuppressed(t *testing.T) {
	gw, _ := newTestGateway(t)

	humanPriv, _, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	agentPriv, agentPub, err := nostr.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}
	gw.agentCache.Set(agentPub, true)

	var handlerCalls int
	gw.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		handlerCalls++
		return nil
	})

	// A human message, then a single agent reply, repeated several times —
	// each human message resets the streak, so no exchange should ever
	// approach the runaway threshold.
	for i := 0; i < buzzLoopGuardMaxConsecutiveAgent+3; i++ {
		human := signedChannelEventFrom(t, humanPriv, "channel-uuid-1", fmt.Sprintf("human msg %d", i))
		gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: human, RelayURL: "wss://relay.example.com"})

		agentReply := signedChannelEventFrom(t, agentPriv, "channel-uuid-1", fmt.Sprintf("agent reply %d", i))
		gw.processEvent(context.Background(), nostr.ReceivedEvent{Event: agentReply, RelayURL: "wss://relay.example.com"})
	}

	want := 2 * (buzzLoopGuardMaxConsecutiveAgent + 3)
	if handlerCalls != want {
		t.Errorf("handler invoked %d times for a human/agent alternating exchange, want %d (guard must not suppress a legitimate human-to-agent exchange)", handlerCalls, want)
	}
}
