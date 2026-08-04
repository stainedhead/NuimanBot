package security_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/usecase/security"
)

// TestConfirmationStatus_ConstantsAreDistinctStrings verifies the four
// documented ConfirmationStatus values are distinct, non-empty strings, so a
// zero-value ConfirmationRequest.Status ("") can never be confused with a
// real, meaningful status.
func TestConfirmationStatus_ConstantsAreDistinctStrings(t *testing.T) {
	statuses := map[security.ConfirmationStatus]string{
		security.ConfirmationStatusPending:  "pending",
		security.ConfirmationStatusApproved: "approved",
		security.ConfirmationStatusDenied:   "denied",
		security.ConfirmationStatusExpired:  "expired",
	}

	seen := make(map[security.ConfirmationStatus]bool, len(statuses))
	for status, name := range statuses {
		if string(status) == "" {
			t.Errorf("%s: expected non-empty ConfirmationStatus value", name)
		}
		if seen[status] {
			t.Errorf("%s: ConfirmationStatus value %q collides with another constant", name, status)
		}
		seen[status] = true
	}

	if len(seen) != 4 {
		t.Fatalf("expected 4 distinct ConfirmationStatus constants, got %d", len(seen))
	}
}

// TestConfirmationRequest_ZeroValue documents the zero-value behavior of
// ConfirmationRequest: an unset Status is not equal to any defined status
// constant, so code must not treat a zero-value request as "pending" by
// accident.
func TestConfirmationRequest_ZeroValue(t *testing.T) {
	var req security.ConfirmationRequest

	if req.Status == security.ConfirmationStatusPending {
		t.Error("zero-value ConfirmationRequest.Status should not equal ConfirmationStatusPending")
	}
	if req.Status == security.ConfirmationStatusApproved {
		t.Error("zero-value ConfirmationRequest.Status should not equal ConfirmationStatusApproved")
	}
	if req.Status == security.ConfirmationStatusDenied {
		t.Error("zero-value ConfirmationRequest.Status should not equal ConfirmationStatusDenied")
	}
	if req.Status == security.ConfirmationStatusExpired {
		t.Error("zero-value ConfirmationRequest.Status should not equal ConfirmationStatusExpired")
	}
	if req.ID != "" || req.UserID != "" || req.ConversationID != "" || req.ToolName != "" {
		t.Error("zero-value ConfirmationRequest should have empty string fields")
	}
	if req.Params != nil {
		t.Error("zero-value ConfirmationRequest.Params should be nil")
	}
	if !req.CreatedAt.IsZero() || !req.ExpiresAt.IsZero() {
		t.Error("zero-value ConfirmationRequest timestamps should be zero")
	}
}

// TestConfirmationStoreErrors_AreDistinctAndDescriptive verifies the three
// sentinel errors are non-nil, mutually distinct (so errors.Is
// discriminates between them), and carry a human-readable message.
func TestConfirmationStoreErrors_AreDistinctAndDescriptive(t *testing.T) {
	sentinels := []error{
		security.ErrConfirmationNotFound,
		security.ErrConfirmationAlreadyOpen,
		security.ErrConfirmationAlreadyResolved,
	}

	for _, err := range sentinels {
		if err == nil {
			t.Fatal("expected non-nil sentinel error")
		}
		if err.Error() == "" {
			t.Errorf("expected non-empty error message for %v", err)
		}
	}

	for i := range sentinels {
		for j := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinel errors must be distinct: %v should not match %v", sentinels[i], sentinels[j])
			}
		}
	}

	// A wrapped sentinel must still be discoverable via errors.Is, since
	// implementations (internal/infrastructure/security) are expected to
	// wrap these with additional context.
	wrapped := &wrappedErr{inner: security.ErrConfirmationAlreadyOpen}
	if !errors.Is(wrapped, security.ErrConfirmationAlreadyOpen) {
		t.Error("expected errors.Is to unwrap to ErrConfirmationAlreadyOpen")
	}
}

type wrappedErr struct{ inner error }

func (w *wrappedErr) Error() string { return "wrapped: " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }

// fakeConfirmationStore is a minimal, in-memory ConfirmationStore used solely
// to prove the interface's exact shape (including the P5.5-motivated
// GetOpenByKey addition) compiles and behaves per its documented contract at
// a basic level. The real, concurrency-hardened implementation is
// internal/infrastructure/security.FileConfirmationStore (P5.2), which has
// its own dedicated test suite including the concurrency tests tasks.md
// P5.2 requires.
type fakeConfirmationStore struct {
	byID      map[string]security.ConfirmationRequest
	openByKey map[string]string
	nextID    int
}

var _ security.ConfirmationStore = (*fakeConfirmationStore)(nil)

func newFakeConfirmationStore() *fakeConfirmationStore {
	return &fakeConfirmationStore{
		byID:      make(map[string]security.ConfirmationRequest),
		openByKey: make(map[string]string),
	}
}

func (f *fakeConfirmationStore) key(userID, conversationID string) string {
	return userID + "\x00" + conversationID
}

func (f *fakeConfirmationStore) Create(ctx context.Context, req security.ConfirmationRequest) (string, error) {
	key := f.key(req.UserID, req.ConversationID)
	if _, open := f.openByKey[key]; open {
		return "", security.ErrConfirmationAlreadyOpen
	}
	f.nextID++
	id := "fake-id"
	req.ID = id
	req.Status = security.ConfirmationStatusPending
	req.CreatedAt = time.Now()
	if req.ExpiresAt.IsZero() {
		req.ExpiresAt = req.CreatedAt.Add(5 * time.Minute)
	}
	f.byID[id] = req
	f.openByKey[key] = id
	return id, nil
}

func (f *fakeConfirmationStore) Resolve(ctx context.Context, id string, approved bool) (security.ConfirmationRequest, error) {
	req, ok := f.byID[id]
	if !ok {
		return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
	}
	if req.Status != security.ConfirmationStatusPending {
		return security.ConfirmationRequest{}, security.ErrConfirmationAlreadyResolved
	}
	if approved {
		req.Status = security.ConfirmationStatusApproved
	} else {
		req.Status = security.ConfirmationStatusDenied
	}
	f.byID[id] = req
	delete(f.openByKey, f.key(req.UserID, req.ConversationID))
	return req, nil
}

func (f *fakeConfirmationStore) Get(ctx context.Context, id string) (security.ConfirmationRequest, error) {
	req, ok := f.byID[id]
	if !ok {
		return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
	}
	return req, nil
}

func (f *fakeConfirmationStore) GetOpenByKey(ctx context.Context, userID, conversationID string) (security.ConfirmationRequest, bool, error) {
	id, ok := f.openByKey[f.key(userID, conversationID)]
	if !ok {
		return security.ConfirmationRequest{}, false, nil
	}
	return f.byID[id], true, nil
}

func (f *fakeConfirmationStore) ExpireStale(ctx context.Context) error {
	now := time.Now()
	for id, req := range f.byID {
		if req.Status == security.ConfirmationStatusPending && now.After(req.ExpiresAt) {
			req.Status = security.ConfirmationStatusExpired
			f.byID[id] = req
			delete(f.openByKey, f.key(req.UserID, req.ConversationID))
		}
	}
	return nil
}

func (f *fakeConfirmationStore) ListPending(ctx context.Context) ([]security.ConfirmationRequest, error) {
	pending := make([]security.ConfirmationRequest, 0, len(f.byID))
	for _, req := range f.byID {
		if req.Status == security.ConfirmationStatusPending {
			pending = append(pending, req)
		}
	}
	return pending, nil
}

// TestConfirmationStore_InterfaceContract exercises the documented
// interface-level contract using fakeConfirmationStore: one open confirmation
// per (UserID, ConversationID), and resolve-once semantics.
func TestConfirmationStore_InterfaceContract(t *testing.T) {
	ctx := context.Background()
	store := newFakeConfirmationStore()

	id, err := store.Create(ctx, security.ConfirmationRequest{
		UserID:         "u1",
		ConversationID: "c1",
		ToolName:       "github",
		Action:         "pr_merge",
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	// FR-014: a second Create for the same key fails closed.
	if _, err := store.Create(ctx, security.ConfirmationRequest{UserID: "u1", ConversationID: "c1", ToolName: "github"}); !errors.Is(err, security.ErrConfirmationAlreadyOpen) {
		t.Errorf("expected ErrConfirmationAlreadyOpen, got %v", err)
	}

	req, ok, err := store.GetOpenByKey(ctx, "u1", "c1")
	if err != nil || !ok {
		t.Fatalf("expected an open confirmation for (u1, c1), got ok=%v err=%v", ok, err)
	}
	if req.ID != id {
		t.Errorf("expected GetOpenByKey to return id %q, got %q", id, req.ID)
	}

	if _, err := store.Get(ctx, id); err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}

	resolved, err := store.Resolve(ctx, id, true)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if resolved.Status != security.ConfirmationStatusApproved {
		t.Errorf("expected ConfirmationStatusApproved, got %v", resolved.Status)
	}

	// Resolve-once: a second Resolve on the same id fails closed.
	if _, err := store.Resolve(ctx, id, false); !errors.Is(err, security.ErrConfirmationAlreadyResolved) {
		t.Errorf("expected ErrConfirmationAlreadyResolved, got %v", err)
	}

	// Resolving frees the (UserID, ConversationID) slot for a new Create.
	if _, err := store.Create(ctx, security.ConfirmationRequest{UserID: "u1", ConversationID: "c1", ToolName: "github"}); err != nil {
		t.Errorf("expected Create to succeed after prior confirmation resolved, got %v", err)
	}

	// An unrecognized id returns ErrConfirmationNotFound from both Get and Resolve.
	if _, err := store.Get(ctx, "does-not-exist"); !errors.Is(err, security.ErrConfirmationNotFound) {
		t.Errorf("Get: expected ErrConfirmationNotFound, got %v", err)
	}
	if _, err := store.Resolve(ctx, "does-not-exist", true); !errors.Is(err, security.ErrConfirmationNotFound) {
		t.Errorf("Resolve: expected ErrConfirmationNotFound, got %v", err)
	}
}
