package buzz

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/metrics"
	"nuimanbot/internal/infrastructure/nostr"
	"nuimanbot/internal/usecase/user"
)

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
