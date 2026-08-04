package security_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	infrasecurity "nuimanbot/internal/infrastructure/security"
	usecasesecurity "nuimanbot/internal/usecase/security"
)

func newTestStore(t *testing.T, ttl time.Duration) *infrasecurity.FileConfirmationStore {
	t.Helper()
	dir := t.TempDir()
	return infrasecurity.NewFileConfirmationStore(filepath.Join(dir, "confirmations.json"), ttl)
}

func TestFileConfirmationStore_CreateGetResolveLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID:         "user-1",
		ConversationID: "conv-1",
		ToolName:       "github",
		Action:         "pr_merge",
		Params:         map[string]any{"pr_number": 42},
		Summary:        "Merge PR #42?",
	})
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("Create: expected non-empty id")
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if got.Status != usecasesecurity.ConfirmationStatusPending {
		t.Errorf("expected Pending status, got %v", got.Status)
	}
	if got.ToolName != "github" || got.Action != "pr_merge" {
		t.Errorf("expected ToolName/Action to round-trip, got %+v", got)
	}
	if got.CreatedAt.IsZero() || got.ExpiresAt.IsZero() {
		t.Error("expected CreatedAt/ExpiresAt to be set by the store")
	}

	resolved, err := store.Resolve(ctx, id, true)
	if err != nil {
		t.Fatalf("Resolve: unexpected error: %v", err)
	}
	if resolved.Status != usecasesecurity.ConfirmationStatusApproved {
		t.Errorf("expected Approved status, got %v", resolved.Status)
	}
}

func TestFileConfirmationStore_ResolveDenied(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	resolved, err := store.Resolve(ctx, id, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Status != usecasesecurity.ConfirmationStatusDenied {
		t.Errorf("expected Denied status, got %v", resolved.Status)
	}
}

func TestFileConfirmationStore_Get_UnknownID(t *testing.T) {
	store := newTestStore(t, 5*time.Minute)
	if _, err := store.Get(context.Background(), "nope"); !errors.Is(err, usecasesecurity.ErrConfirmationNotFound) {
		t.Errorf("expected ErrConfirmationNotFound, got %v", err)
	}
}

func TestFileConfirmationStore_Resolve_UnknownID(t *testing.T) {
	store := newTestStore(t, 5*time.Minute)
	if _, err := store.Resolve(context.Background(), "nope", true); !errors.Is(err, usecasesecurity.ErrConfirmationNotFound) {
		t.Errorf("expected ErrConfirmationNotFound, got %v", err)
	}
}

func TestFileConfirmationStore_Resolve_AlreadyResolved(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := store.Resolve(ctx, id, true); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if _, err := store.Resolve(ctx, id, false); !errors.Is(err, usecasesecurity.ErrConfirmationAlreadyResolved) {
		t.Errorf("expected ErrConfirmationAlreadyResolved on second Resolve, got %v", err)
	}
}

// TestFileConfirmationStore_OneOpenConfirmationPerConversation covers FR-014:
// a second Create for the same (UserID, ConversationID) fails while the
// first is still open, but succeeds once the first is resolved.
func TestFileConfirmationStore_OneOpenConfirmationPerConversation(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "coding_agent"}); !errors.Is(err, usecasesecurity.ErrConfirmationAlreadyOpen) {
		t.Errorf("expected ErrConfirmationAlreadyOpen for second Create on same key, got %v", err)
	}

	// A different conversation for the same user is unaffected.
	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "other-conv", ToolName: "github"}); err != nil {
		t.Errorf("expected Create to succeed for a different conversation, got %v", err)
	}

	if _, err := store.Resolve(ctx, id, true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"}); err != nil {
		t.Errorf("expected Create to succeed after prior confirmation resolved, got %v", err)
	}
}

func TestFileConfirmationStore_GetOpenByKey(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	if _, ok, err := store.GetOpenByKey(ctx, "u", "c"); err != nil || ok {
		t.Fatalf("expected no open confirmation initially, got ok=%v err=%v", ok, err)
	}

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github", Summary: "Merge PR #42?"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req, ok, err := store.GetOpenByKey(ctx, "u", "c")
	if err != nil || !ok {
		t.Fatalf("expected an open confirmation, got ok=%v err=%v", ok, err)
	}
	if req.ID != id || req.Summary != "Merge PR #42?" {
		t.Errorf("expected GetOpenByKey to return the created confirmation, got %+v", req)
	}

	if _, err := store.Resolve(ctx, id, true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if _, ok, err := store.GetOpenByKey(ctx, "u", "c"); err != nil || ok {
		t.Errorf("expected no open confirmation after resolve, got ok=%v err=%v", ok, err)
	}
}

// TestFileConfirmationStore_ListPending_EmptyStore verifies ListPending
// returns an empty, non-nil slice (not an error) when nothing has ever been
// created (P5.8: the web admin UI's confirmations page).
func TestFileConfirmationStore_ListPending_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: unexpected error: %v", err)
	}
	if pending == nil {
		t.Error("expected non-nil empty slice from ListPending on an empty store")
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending confirmations, got %d", len(pending))
	}
}

// TestFileConfirmationStore_ListPending_ExcludesResolvedAndExpired verifies
// ListPending only returns confirmations still in ConfirmationStatusPending:
// resolved (approved/denied) and expired confirmations are excluded, and
// results are ordered by CreatedAt ascending.
func TestFileConfirmationStore_ListPending_ExcludesResolvedAndExpired(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	firstID, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID: "u1", ConversationID: "c1", ToolName: "github", Summary: "First (oldest)",
	})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	secondID, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID: "u2", ConversationID: "c2", ToolName: "coding_agent", Summary: "Second (still pending)",
	})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	resolvedID, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID: "u3", ConversationID: "c3", ToolName: "executor", Summary: "Resolved, must be excluded",
	})
	if err != nil {
		t.Fatalf("Create resolved: %v", err)
	}
	if _, err := store.Resolve(ctx, resolvedID, true); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Expired confirmation: explicit ExpiresAt in the past.
	expiredStore := newTestStore(t, 5*time.Minute)
	if _, err := expiredStore.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID: "u4", ConversationID: "c4", ToolName: "github", Summary: "Expired, must be excluded",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("Create expired: %v", err)
	}

	pending, err := store.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending: unexpected error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected exactly 2 pending confirmations, got %d: %+v", len(pending), pending)
	}
	if pending[0].ID != firstID || pending[1].ID != secondID {
		t.Errorf("expected ListPending ordered [%s, %s] (CreatedAt ascending), got [%s, %s]",
			firstID, secondID, pending[0].ID, pending[1].ID)
	}

	expiredPending, err := expiredStore.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending (expired store): unexpected error: %v", err)
	}
	if len(expiredPending) != 0 {
		t.Errorf("expected the past-TTL confirmation to be excluded as expired, got %d entries", len(expiredPending))
	}
}

// TestFileConfirmationStore_TTLExpiry covers FR-015: an unresolved
// confirmation past its TTL is treated as expired by ExpireStale, Get, and
// GetOpenByKey, and no longer blocks a new Create for the same key.
func TestFileConfirmationStore_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 20*time.Millisecond)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	if err := store.ExpireStale(ctx); err != nil {
		t.Fatalf("ExpireStale: %v", err)
	}

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != usecasesecurity.ConfirmationStatusExpired {
		t.Errorf("expected Expired status after ExpireStale, got %v", got.Status)
	}

	if _, ok, err := store.GetOpenByKey(ctx, "u", "c"); err != nil || ok {
		t.Errorf("expected no open confirmation after expiry, got ok=%v err=%v", ok, err)
	}

	// A new Create for the same key should now succeed.
	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"}); err != nil {
		t.Errorf("expected Create to succeed after TTL expiry, got %v", err)
	}
}

// TestFileConfirmationStore_TTLExpiry_LazyOnCreate verifies that Create
// itself lazily expires a stale open confirmation for its key without
// needing an explicit ExpireStale call first (so a background reaper is an
// optimization, not a correctness requirement).
func TestFileConfirmationStore_TTLExpiry_LazyOnCreate(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, 20*time.Millisecond)

	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	time.Sleep(40 * time.Millisecond)

	if _, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "coding_agent"}); err != nil {
		t.Errorf("expected second Create to succeed once the first has expired, got %v", err)
	}
}

// TestFileConfirmationStore_Persistence verifies confirmations survive
// across FileConfirmationStore instances backed by the same file (the store
// is not merely an in-memory cache).
func TestFileConfirmationStore_Persistence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "confirmations.json")

	store1 := infrasecurity.NewFileConfirmationStore(path, 5*time.Minute)
	id, err := store1.Create(ctx, usecasesecurity.ConfirmationRequest{
		UserID:         "u",
		ConversationID: "c",
		ToolName:       "github",
		Action:         "pr_merge",
		Summary:        "Merge PR #42?",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	store2 := infrasecurity.NewFileConfirmationStore(path, 5*time.Minute)
	got, err := store2.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get from second store instance: %v", err)
	}
	if got.ToolName != "github" || got.Action != "pr_merge" || got.Summary != "Merge PR #42?" {
		t.Errorf("expected persisted fields to round-trip, got %+v", got)
	}

	req, ok, err := store2.GetOpenByKey(ctx, "u", "c")
	if err != nil || !ok || req.ID != id {
		t.Errorf("expected GetOpenByKey to find the persisted confirmation from a fresh store instance, got ok=%v err=%v req=%+v", ok, err, req)
	}
}

// TestFileConfirmationStore_ConcurrentCreate_SameKey_ExactlyOneSucceeds is
// the P5.2-mandated concurrency test: N goroutines racing Create for the
// same (UserID, ConversationID) key must yield exactly one success and N-1
// ErrConfirmationAlreadyOpen failures — proving the check-then-create
// sequence is atomic under contention, not just correct for sequential
// calls.
func TestFileConfirmationStore_ConcurrentCreate_SameKey_ExactlyOneSucceeds(t *testing.T) {
	const n = 50
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		successCount int
		alreadyOpen  int
		otherErrs    int
	)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
				UserID:         "racer",
				ConversationID: "conv",
				ToolName:       "github",
				Action:         "pr_merge",
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, usecasesecurity.ErrConfirmationAlreadyOpen):
				alreadyOpen++
			default:
				otherErrs++
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful Create, got %d (alreadyOpen=%d, otherErrs=%d)", successCount, alreadyOpen, otherErrs)
	}
	if alreadyOpen != n-1 {
		t.Errorf("expected %d ErrConfirmationAlreadyOpen failures, got %d", n-1, alreadyOpen)
	}
	if otherErrs != 0 {
		t.Errorf("expected no other errors, got %d", otherErrs)
	}
}

// TestFileConfirmationStore_ConcurrentResolve_SameID_ExactlyOneSucceeds is
// the second P5.2-mandated concurrency test: N goroutines racing Resolve on
// the same confirmation ID must yield exactly one success and N-1
// ErrConfirmationAlreadyResolved failures — the property the underlying
// tool call being re-invoked at most once depends on.
func TestFileConfirmationStore_ConcurrentResolve_SameID_ExactlyOneSucceeds(t *testing.T) {
	const n = 50
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	id, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{UserID: "u", ConversationID: "c", ToolName: "github"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var (
		wg              sync.WaitGroup
		mu              sync.Mutex
		successCount    int
		alreadyResolved int
		otherErrs       int
	)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := store.Resolve(ctx, id, i%2 == 0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, usecasesecurity.ErrConfirmationAlreadyResolved):
				alreadyResolved++
			default:
				otherErrs++
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful Resolve, got %d (alreadyResolved=%d, otherErrs=%d)", successCount, alreadyResolved, otherErrs)
	}
	if alreadyResolved != n-1 {
		t.Errorf("expected %d ErrConfirmationAlreadyResolved failures, got %d", n-1, alreadyResolved)
	}
	if otherErrs != 0 {
		t.Errorf("expected no other errors, got %d", otherErrs)
	}

	final, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if final.Status != usecasesecurity.ConfirmationStatusApproved && final.Status != usecasesecurity.ConfirmationStatusDenied {
		t.Errorf("expected final status to be Approved or Denied, got %v", final.Status)
	}
}

// TestFileConfirmationStore_ConcurrentCreate_DifferentKeys_AllSucceed proves
// correctness (not parallelism — all operations are fully serialized by the
// single dataMu mutex, by design; see FileConfirmationStore's doc comment):
// concurrent Creates for distinct (UserID, ConversationID) keys must all
// still succeed independently, none spuriously failing due to contention.
func TestFileConfirmationStore_ConcurrentCreate_DifferentKeys_AllSucceed(t *testing.T) {
	const n = 20
	ctx := context.Background()
	store := newTestStore(t, 5*time.Minute)

	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := store.Create(ctx, usecasesecurity.ConfirmationRequest{
				UserID:         "user",
				ConversationID: filepath.Join("conv", string(rune('a'+i))),
				ToolName:       "github",
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Create %d: expected success for a distinct key, got %v", i, err)
		}
	}
}
