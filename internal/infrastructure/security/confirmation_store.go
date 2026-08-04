package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	usecasesecurity "nuimanbot/internal/usecase/security"
)

// DefaultConfirmationTTL is the fallback time-to-live applied to a
// ConfirmationRequest whose ExpiresAt is left zero by the caller, matching
// the PRD's documented default (specs/260802-improve-nuimanbot-security,
// FR-015 / §5.3.4).
const DefaultConfirmationTTL = 5 * time.Minute

// confirmationFileState is the on-disk JSON representation persisted by
// FileConfirmationStore. Only ByID is persisted — OpenByKey (the
// (UserID, ConversationID) -> ID index) is rebuilt on load from whichever
// records are still Pending and unexpired, so the two can never drift out of
// sync on disk.
type confirmationFileState struct {
	Version string                                         `json:"version"`
	ByID    map[string]usecasesecurity.ConfirmationRequest `json:"by_id"`
}

// FileConfirmationStore is a file-backed, TTL-expiring implementation of
// usecasesecurity.ConfirmationStore (specs/260802-improve-nuimanbot-security,
// Part C / FR-009, task P5.2).
//
// Concurrency: every operation (Create, Resolve, Get, GetOpenByKey,
// ListPending, ExpireStale) holds a single always-shared dataMu for its
// entire duration, including the synchronous file I/O (fsync + rename) that
// persists a mutation — so all operations are fully serialized regardless of
// which key/id they target. This is a deliberate simplicity-over-throughput
// tradeoff: ConfirmationStore sits on the confirmation-creation path (rare,
// human-gated actions), not the general tool-loop hot path (parent spec
// §8.6), so full serialization is not expected to matter in practice. An
// earlier revision additionally acquired a per-(UserID, ConversationID) or
// per-id lock before dataMu, intending to let operations on different
// keys/ids run concurrently — but since dataMu was never released mid-call,
// every operation still fully serialized on dataMu regardless, so those
// extra locks added complexity without delivering any actual parallelism.
// They were removed for that reason (specs/260803-improve-nuimanbot-security-
// auto-review, FR-009/OQ-3); see implementation-notes.md for the full
// decision rationale. The single-invariant guarantees this locking still
// provides are unaffected: at most one Create per (UserID, ConversationID)
// can observe no open confirmation and succeed, and at most one Resolve per
// id can observe Pending and transition it.
type FileConfirmationStore struct {
	filePath string
	ttl      time.Duration

	dataMu    sync.Mutex
	byID      map[string]usecasesecurity.ConfirmationRequest
	openByKey map[string]string // (UserID, ConversationID) key -> ID
	loaded    bool
}

// NewFileConfirmationStore creates a FileConfirmationStore persisting to
// filePath. ttl is the default expiry applied to a ConfirmationRequest whose
// ExpiresAt is left zero by the caller; a zero/negative ttl defaults to
// DefaultConfirmationTTL.
func NewFileConfirmationStore(filePath string, ttl time.Duration) *FileConfirmationStore {
	if ttl <= 0 {
		ttl = DefaultConfirmationTTL
	}
	return &FileConfirmationStore{
		filePath:  filePath,
		ttl:       ttl,
		byID:      make(map[string]usecasesecurity.ConfirmationRequest),
		openByKey: make(map[string]string),
	}
}

var _ usecasesecurity.ConfirmationStore = (*FileConfirmationStore)(nil)

// confirmationKey builds the (UserID, ConversationID) composite key used to
// enforce "at most one open confirmation per conversation" (FR-014). NUL is
// used as a separator since it cannot appear in either component in
// practice, avoiding key collisions between e.g. UserID="a", ConversationID
// ="b:c" and UserID="a:b", ConversationID="c".
func confirmationKey(userID, conversationID string) string {
	return userID + "\x00" + conversationID
}

// ensureLoadedLocked lazily loads the on-disk file into memory the first
// time the store is used. Callers must hold dataMu.
func (s *FileConfirmationStore) ensureLoadedLocked() error {
	if s.loaded {
		return nil
	}
	s.loaded = true

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file yet: start with empty state
		}
		return fmt.Errorf("failed to read confirmation store file: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var state confirmationFileState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse confirmation store file: %w", err)
	}

	now := time.Now()
	for id, rec := range state.ByID {
		if rec.Status == usecasesecurity.ConfirmationStatusPending && now.After(rec.ExpiresAt) {
			rec.Status = usecasesecurity.ConfirmationStatusExpired
		}
		s.byID[id] = rec
		if rec.Status == usecasesecurity.ConfirmationStatusPending {
			s.openByKey[confirmationKey(rec.UserID, rec.ConversationID)] = id
		}
	}
	return nil
}

// persistLocked writes the current in-memory state to disk atomically.
// Callers must hold dataMu.
func (s *FileConfirmationStore) persistLocked() error {
	state := confirmationFileState{
		Version: "1",
		ByID:    s.byID,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal confirmation store state: %w", err)
	}
	if err := writeFileAtomic(s.filePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write confirmation store file: %w", err)
	}
	return nil
}

// writeFileAtomic writes data to targetPath via a temp-file-then-rename
// sequence in the same directory, so a reader never observes a partially
// written file. Deliberately self-contained (rather than reusing
// internal/infrastructure/storage.AtomicFileWriter, which already exists and
// does the same thing) because that package imports
// internal/infrastructure/security (for EncryptionService), and importing it
// back from here would create an import cycle.
func writeFileAtomic(targetPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-confirmation-store-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	tmpPath = ""

	return nil
}

// clearOpenIfMatchesLocked removes rec's (UserID, ConversationID) -> ID index
// entry, but only if it still points at rec.ID (defensive: avoids clobbering
// a newer confirmation's index entry in the unlikely event of a stale
// reference). Callers must hold dataMu.
func (s *FileConfirmationStore) clearOpenIfMatchesLocked(rec usecasesecurity.ConfirmationRequest) {
	key := confirmationKey(rec.UserID, rec.ConversationID)
	if s.openByKey[key] == rec.ID {
		delete(s.openByKey, key)
	}
}

// expireIfNeededLocked transitions rec to Expired (updating byID and the
// open-by-key index) if it is Pending and past its ExpiresAt deadline.
// Returns the possibly-updated record. Callers must hold dataMu.
func (s *FileConfirmationStore) expireIfNeededLocked(rec usecasesecurity.ConfirmationRequest) usecasesecurity.ConfirmationRequest {
	if rec.Status == usecasesecurity.ConfirmationStatusPending && time.Now().After(rec.ExpiresAt) {
		rec.Status = usecasesecurity.ConfirmationStatusExpired
		s.byID[rec.ID] = rec
		s.clearOpenIfMatchesLocked(rec)
	}
	return rec
}

// Create implements usecasesecurity.ConfirmationStore.
func (s *FileConfirmationStore) Create(_ context.Context, req usecasesecurity.ConfirmationRequest) (string, error) {
	key := confirmationKey(req.UserID, req.ConversationID)

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return "", err
	}

	// Lazily expire whatever currently occupies this key so a timed-out
	// confirmation never blocks a legitimate new request (FR-015).
	if existingID, open := s.openByKey[key]; open {
		s.expireIfNeededLocked(s.byID[existingID])
	}

	if _, open := s.openByKey[key]; open {
		return "", usecasesecurity.ErrConfirmationAlreadyOpen
	}

	now := time.Now()
	expiresAt := req.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(s.ttl)
	}

	rec := usecasesecurity.ConfirmationRequest{
		ID:             uuid.NewString(),
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		ToolName:       req.ToolName,
		Action:         req.Action,
		Params:         req.Params,
		Summary:        req.Summary,
		CreatedAt:      now,
		ExpiresAt:      expiresAt,
		Status:         usecasesecurity.ConfirmationStatusPending,
	}

	s.byID[rec.ID] = rec
	s.openByKey[key] = rec.ID

	if err := s.persistLocked(); err != nil {
		// Roll back the in-memory mutation so a failed persist can't leave
		// the store claiming a confirmation exists that isn't durable —
		// fail closed rather than accepting an unpersisted "success".
		delete(s.byID, rec.ID)
		delete(s.openByKey, key)
		return "", err
	}

	return rec.ID, nil
}

// Resolve implements usecasesecurity.ConfirmationStore.
func (s *FileConfirmationStore) Resolve(_ context.Context, id string, approved bool) (usecasesecurity.ConfirmationRequest, error) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return usecasesecurity.ConfirmationRequest{}, err
	}

	rec, ok := s.byID[id]
	if !ok {
		return usecasesecurity.ConfirmationRequest{}, usecasesecurity.ErrConfirmationNotFound
	}

	rec = s.expireIfNeededLocked(rec)
	if rec.Status != usecasesecurity.ConfirmationStatusPending {
		return usecasesecurity.ConfirmationRequest{}, usecasesecurity.ErrConfirmationAlreadyResolved
	}

	if approved {
		rec.Status = usecasesecurity.ConfirmationStatusApproved
	} else {
		rec.Status = usecasesecurity.ConfirmationStatusDenied
	}
	s.byID[id] = rec
	s.clearOpenIfMatchesLocked(rec)

	if err := s.persistLocked(); err != nil {
		return usecasesecurity.ConfirmationRequest{}, err
	}

	return rec, nil
}

// Get implements usecasesecurity.ConfirmationStore.
func (s *FileConfirmationStore) Get(_ context.Context, id string) (usecasesecurity.ConfirmationRequest, error) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return usecasesecurity.ConfirmationRequest{}, err
	}

	rec, ok := s.byID[id]
	if !ok {
		return usecasesecurity.ConfirmationRequest{}, usecasesecurity.ErrConfirmationNotFound
	}
	rec = s.expireIfNeededLocked(rec)
	return rec, nil
}

// GetOpenByKey implements usecasesecurity.ConfirmationStore.
func (s *FileConfirmationStore) GetOpenByKey(_ context.Context, userID, conversationID string) (usecasesecurity.ConfirmationRequest, bool, error) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return usecasesecurity.ConfirmationRequest{}, false, err
	}

	key := confirmationKey(userID, conversationID)
	id, open := s.openByKey[key]
	if !open {
		return usecasesecurity.ConfirmationRequest{}, false, nil
	}

	rec := s.expireIfNeededLocked(s.byID[id])
	if rec.Status != usecasesecurity.ConfirmationStatusPending {
		return usecasesecurity.ConfirmationRequest{}, false, nil
	}
	return rec, true, nil
}

// ListPending implements usecasesecurity.ConfirmationStore. Returns every
// confirmation still Pending after lazy-expiry, ordered by CreatedAt
// ascending so a listing UI (P5.8) shows the oldest, most-overdue requests
// first.
func (s *FileConfirmationStore) ListPending(_ context.Context) ([]usecasesecurity.ConfirmationRequest, error) {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return nil, err
	}

	pending := make([]usecasesecurity.ConfirmationRequest, 0, len(s.byID))
	for _, rec := range s.byID {
		rec = s.expireIfNeededLocked(rec)
		if rec.Status == usecasesecurity.ConfirmationStatusPending {
			pending = append(pending, rec)
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})

	return pending, nil
}

// ExpireStale implements usecasesecurity.ConfirmationStore.
func (s *FileConfirmationStore) ExpireStale(_ context.Context) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if err := s.ensureLoadedLocked(); err != nil {
		return err
	}

	changed := false
	for id, rec := range s.byID {
		if rec.Status == usecasesecurity.ConfirmationStatusPending && time.Now().After(rec.ExpiresAt) {
			rec.Status = usecasesecurity.ConfirmationStatusExpired
			s.byID[id] = rec
			s.clearOpenIfMatchesLocked(rec)
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return s.persistLocked()
}
