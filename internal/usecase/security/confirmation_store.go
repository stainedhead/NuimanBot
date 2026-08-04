package security

import (
	"context"
	"errors"
	"time"
)

// ConfirmationStatus describes the current state of a ConfirmationRequest.
type ConfirmationStatus string

const (
	// ConfirmationStatusPending means the confirmation is still awaiting a
	// yes/no reply and has not yet expired.
	ConfirmationStatusPending ConfirmationStatus = "pending"
	// ConfirmationStatusApproved means the user (or gateway UI) confirmed the
	// pending action.
	ConfirmationStatusApproved ConfirmationStatus = "approved"
	// ConfirmationStatusDenied means the user (or gateway UI) declined the
	// pending action.
	ConfirmationStatusDenied ConfirmationStatus = "denied"
	// ConfirmationStatusExpired means the confirmation was never resolved
	// before its ExpiresAt deadline and is treated as denied for the purpose
	// of gating the underlying tool call (FR-015), while remaining a
	// distinct, auditable status value from an explicit deny.
	ConfirmationStatusExpired ConfirmationStatus = "expired"
)

// ConfirmationRequest represents a single side-effecting tool call that has
// been paused pending explicit human confirmation
// (specs/260802-improve-nuimanbot-security, Part C / FR-009..FR-015).
//
// A ConfirmationRequest generalizes the NeedsConfirmation/ConfirmationID
// mechanism internal/usecase/persona/memorywriter.go already uses for persona
// memory-file writes to arbitrary tool calls: ToolName/Action/Params capture
// enough to re-invoke the original call verbatim once approved, without
// re-prompting the LLM for a fresh decision (FR-013).
type ConfirmationRequest struct {
	// ID uniquely identifies this confirmation request.
	ID string
	// UserID is the user who triggered the pending action.
	UserID string
	// ConversationID scopes the "at most one open confirmation" invariant
	// (FR-014) and is how ChatService.ProcessMessage locates an open
	// confirmation for an incoming message (FR-013).
	ConversationID string
	// ToolName is the registered domain.Tool name the pending call targets.
	ToolName string
	// Action is the tool-specific action being requested (e.g. "pr_merge"
	// for the "github" tool). May be empty for tools with no action concept.
	Action string
	// Params are the exact parameters the original tool call was invoked
	// with; re-used verbatim to re-invoke the call on approval (FR-013).
	Params map[string]any
	// Summary is a human-readable description of the pending action, shown
	// to the user by whichever gateway/UI surfaces the confirmation prompt.
	Summary string
	// CreatedAt is when the confirmation was created.
	CreatedAt time.Time
	// ExpiresAt is when an unresolved confirmation is treated as expired
	// (and denied) — see ConfirmationStatusExpired and FR-015.
	ExpiresAt time.Time
	// Status is the confirmation's current lifecycle state.
	Status ConfirmationStatus
}

// Errors returned by ConfirmationStore implementations. Callers should use
// errors.Is to check for these rather than comparing implementation-specific
// error values.
var (
	// ErrConfirmationNotFound is returned by Get/Resolve when no confirmation
	// exists for the given ID.
	ErrConfirmationNotFound = errors.New("confirmation not found")
	// ErrConfirmationAlreadyOpen is returned by Create when a Pending, not-yet-
	// expired confirmation already exists for the request's (UserID,
	// ConversationID) — enforcing "at most one open confirmation per
	// conversation" (FR-014). Callers must fail closed (deny/do not execute)
	// on this error, not retry into a duplicate side effect.
	ErrConfirmationAlreadyOpen = errors.New("a confirmation is already pending for this conversation")
	// ErrConfirmationAlreadyResolved is returned by Resolve when the
	// confirmation is no longer Pending (already Approved, Denied, or
	// Expired) — including the case where two concurrent Resolve calls race
	// on the same ID: exactly one succeeds and the other observes this error.
	ErrConfirmationAlreadyResolved = errors.New("confirmation has already been resolved")
)

// ConfirmationStore persists pending side-effecting-action confirmations and
// enforces the invariants FR-014/FR-015 depend on:
//
//   - At most one open (Pending, unexpired) confirmation per (UserID,
//     ConversationID): a second Create for the same key fails with
//     ErrConfirmationAlreadyOpen rather than silently creating a duplicate.
//   - A confirmation can be resolved at most once: concurrent Resolve calls
//     on the same ID must not both succeed (see
//     internal/infrastructure/security.FileConfirmationStore, which
//     guarantees this via a single mutex serializing all operations —
//     see that type's doc comment for the concurrency/throughput tradeoff
//     rationale).
//
// Implementations must fail closed: an internal error from Create/Resolve
// must never be interpreted by the caller as "safe to execute" (see
// internal/usecase/tool/service.go's handling of Create's return error).
type ConfirmationStore interface {
	// Create records a new pending confirmation and returns its generated ID.
	// req.ID/CreatedAt/Status are set by the store (any caller-supplied
	// values are ignored); req.ExpiresAt, if zero, is defaulted by the store
	// to its own configured TTL. Returns ErrConfirmationAlreadyOpen if an
	// open confirmation already exists for (req.UserID, req.ConversationID).
	Create(ctx context.Context, req ConfirmationRequest) (id string, err error)
	// Resolve marks the confirmation identified by id as Approved (if
	// approved is true) or Denied (if false) and returns the resolved
	// record. Returns ErrConfirmationNotFound if id does not exist, or
	// ErrConfirmationAlreadyResolved if it is no longer Pending (already
	// resolved, or expired) — including the losing side of two concurrent
	// Resolve calls on the same ID.
	Resolve(ctx context.Context, id string, approved bool) (ConfirmationRequest, error)
	// Get retrieves the confirmation identified by id without changing its
	// state. Returns ErrConfirmationNotFound if it does not exist.
	Get(ctx context.Context, id string) (ConfirmationRequest, error)
	// GetOpenByKey returns the currently open (Pending, not-yet-expired)
	// confirmation for (userID, conversationID), if one exists. ok is false
	// — with a zero-value ConfirmationRequest and nil error — when no such
	// confirmation exists; this is not an error condition, unlike Get, which
	// returns ErrConfirmationNotFound for an unrecognized id.
	//
	// Added for FR-013: ChatService.ProcessMessage calls this at the start
	// of every incoming message to determine whether the message should be
	// checked as a confirmation reply (see the yes/no heuristic in
	// internal/usecase/chat) before being treated as a new chat turn.
	GetOpenByKey(ctx context.Context, userID, conversationID string) (ConfirmationRequest, bool, error)
	// ExpireStale transitions every Pending confirmation whose ExpiresAt has
	// passed to ConfirmationStatusExpired, freeing its (UserID,
	// ConversationID) slot for a new Create.
	ExpireStale(ctx context.Context) error
	// ListPending returns every currently open (Pending, not-yet-expired)
	// confirmation across all users/conversations, ordered by CreatedAt
	// ascending (oldest first). An empty store returns an empty, non-nil
	// slice.
	//
	// Added for P5.8/P5.9: a gateway/web/REST UI that needs to show "what's
	// waiting on me" (or, for an admin, "everything currently pending")
	// has no other way to enumerate confirmations — Get requires an ID and
	// GetOpenByKey requires a (UserID, ConversationID) pair, neither of which
	// a listing UI has in hand up front.
	ListPending(ctx context.Context) ([]ConfirmationRequest, error)
}
