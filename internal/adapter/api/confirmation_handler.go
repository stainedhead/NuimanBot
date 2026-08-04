package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"nuimanbot/internal/adapter/api/middleware"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
)

// ConfirmationStore is the subset of security.ConfirmationStore the REST API
// confirmation endpoints need: looking up a confirmation's current state by
// ID. Resolution is delegated to ConfirmationResolver rather than performed
// via ConfirmationStore.Resolve directly, since resolving a confirmation
// also requires re-invoking the original tool call and running a fresh chat
// turn — see ConfirmationResolver's doc comment.
type ConfirmationStore interface {
	// Get retrieves the confirmation identified by id without changing its
	// state. Returns an error satisfying errors.Is(err,
	// security.ErrConfirmationNotFound) if id does not exist.
	Get(ctx context.Context, id string) (security.ConfirmationRequest, error)
}

// ConfirmationResolver resolves a pending confirmation — approving or
// denying it — performing the tool re-invocation and fresh chat turn a bare
// security.ConfirmationStore.Resolve call cannot on its own (implemented by
// chat.Service.ResolveConfirmation; see specs/260802-improve-nuimanbot-security
// /implementation-notes.md's Phase 5 section for why this indirection
// exists).
type ConfirmationResolver interface {
	// ResolveConfirmation resolves the confirmation identified by id,
	// approving it if approved is true or denying it otherwise. Returns an
	// error satisfying errors.Is(err, security.ErrConfirmationNotFound) or
	// errors.Is(err, security.ErrConfirmationAlreadyResolved) for those
	// specific failure modes.
	ResolveConfirmation(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error)
}

// confirmationResponse is the JSON body for GET /api/v1/confirmations/{id}.
type confirmationResponse struct {
	ID        string    `json:"id"`
	ToolName  string    `json:"tool_name"`
	Summary   string    `json:"summary"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// resolveConfirmationRequest is the JSON body for
// POST /api/v1/confirmations/{id}/resolve. Approved is a pointer so a
// missing "approved" field can be distinguished from an explicit `false`.
type resolveConfirmationRequest struct {
	Approved *bool `json:"approved"`
}

// resolveConfirmationResponse is the JSON body returned on a successful
// POST /api/v1/confirmations/{id}/resolve.
type resolveConfirmationResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // "approved" | "denied"
	Message string `json:"message,omitempty"`
}

// confirmationHandler implements the REST API confirmation endpoints
// (specs/260802-improve-nuimanbot-security, Part C / FR-011, P5.9):
//
//   - GET /api/v1/confirmations/{id}
//   - POST /api/v1/confirmations/{id}/resolve
//
// Both endpoints run behind the same JWT + rate-limit + body-limit +
// validate middleware chain as every other protected route (see
// NewServer's protectedChain) and, in principle, additionally enforce that
// the authenticated principal owns the confirmation (matches
// ConfirmationRequest.UserID) unless it holds middleware.RoleAdmin.
//
// KNOWN LIMITATION (FR-006 / specs/260803-improve-nuimanbot-security-auto-
// review): under the REST API's current single-shared-key deployment model,
// every issued JWT carries Role: middleware.RoleAdmin (see claims.go's
// newClaims) because the API recognizes only one credential. That makes the
// ownership check below effectively non-discriminating today — see
// confirmationAuthorized's doc comment for the precise behavior this
// implies. Do not rely on these endpoints to keep one user's confirmations
// private from another; see support_docs/security-hardening-guide.md and
// support_docs/api-reference.md for the operator-facing caveat.
type confirmationHandler struct {
	store    ConfirmationStore
	resolver ConfirmationResolver
}

// newConfirmationHandler creates a confirmationHandler.
func newConfirmationHandler(store ConfirmationStore, resolver ConfirmationResolver) *confirmationHandler {
	return &confirmationHandler{store: store, resolver: resolver}
}

// handleGet implements GET /api/v1/confirmations/{id}.
func (h *confirmationHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error", Message: "confirmation store not configured"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "confirmation id is required"})
		return
	}

	req, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeConfirmationLookupError(w, err)
		return
	}

	if !confirmationAuthorized(r.Context(), req.UserID) {
		writeForbidden(w)
		return
	}

	writeJSON(w, http.StatusOK, confirmationResponse{
		ID:        req.ID,
		ToolName:  req.ToolName,
		Summary:   req.Summary,
		Status:    string(req.Status),
		ExpiresAt: req.ExpiresAt,
	})
}

// handleResolve implements POST /api/v1/confirmations/{id}/resolve.
func (h *confirmationHandler) handleResolve(w http.ResponseWriter, r *http.Request) {
	if h.store == nil || h.resolver == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error", Message: "confirmation store not configured"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "confirmation id is required"})
		return
	}

	body, ok := decodeResolveBody(w, r)
	if !ok {
		return
	}

	req, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeConfirmationLookupError(w, err)
		return
	}

	if !confirmationAuthorized(r.Context(), req.UserID) {
		writeForbidden(w)
		return
	}

	if confirmationIsSettled(req) {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "already_resolved", Message: "confirmation is no longer pending"})
		return
	}

	outgoing, err := h.resolver.ResolveConfirmation(r.Context(), id, *body.Approved)
	if err != nil {
		writeConfirmationResolveError(w, err)
		return
	}

	status := "denied"
	if *body.Approved {
		status = "approved"
	}
	writeJSON(w, http.StatusOK, resolveConfirmationResponse{
		ID:      id,
		Status:  status,
		Message: outgoing.Content,
	})
}

// decodeResolveBody decodes and validates the POST resolve request body. On
// failure it writes the appropriate 400 response itself and returns
// ok=false; the caller must return immediately without further processing.
func decodeResolveBody(w http.ResponseWriter, r *http.Request) (resolveConfirmationRequest, bool) {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "request body required"})
		return resolveConfirmationRequest{}, false
	}

	var body resolveConfirmationRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: `invalid JSON: expected {"approved": bool}`})
		return resolveConfirmationRequest{}, false
	}
	if body.Approved == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "approved (bool) is required"})
		return resolveConfirmationRequest{}, false
	}

	return body, true
}

// confirmationIsSettled reports whether req is no longer eligible for
// resolution: already Approved/Denied/Expired, or Pending but past its
// ExpiresAt deadline (FR-015) — checked directly rather than relying on the
// background ExpireStale job having already run.
func confirmationIsSettled(req security.ConfirmationRequest) bool {
	if req.Status != security.ConfirmationStatusPending {
		return true
	}
	return !req.ExpiresAt.IsZero() && time.Now().After(req.ExpiresAt)
}

// confirmationAuthorized reports whether the authenticated principal in ctx
// may access the confirmation owned by ownerUserID: either the principal
// holds middleware.RoleAdmin, or its ID matches ownerUserID exactly.
//
// CURRENT BEHAVIOR — NOT PER-USER-SCOPED (FR-006): the RoleAdmin branch
// short-circuits true before the ownerUserID comparison is ever reached.
// Under the REST API's present single-shared-key auth model, newClaims
// (claims.go) hardcodes Role: middleware.RoleAdmin on every issued JWT,
// because the API has exactly one credential to recognize and no concept of
// distinct end-user identity for REST callers. As a direct consequence,
// this function returns true unconditionally for any valid API credential,
// regardless of ownerUserID — the ownership branch below is presently dead
// code for every real caller. This is a known, documented limitation (see
// support_docs/security-hardening-guide.md and support_docs/api-reference.md),
// not an oversight: resolving it requires extending the REST API to issue
// genuine per-user credentials, which is out of scope for this fix pass
// (see specs/260803-improve-nuimanbot-security-auto-review, OQ-2). Do not
// remove this comment or the ownership branch below when refactoring —
// once per-user REST credentials exist, RoleFromContext will start
// returning non-admin roles and this branch will become live again.
func confirmationAuthorized(ctx context.Context, ownerUserID string) bool {
	if middleware.RoleFromContext(ctx) == middleware.RoleAdmin {
		return true
	}
	return middleware.PrincipalFromContext(ctx) == ownerUserID
}

// writeForbidden writes the standard 403 response for a confirmation
// ownership check failure.
func writeForbidden(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden", Message: "you do not have access to this confirmation"})
}

// writeConfirmationLookupError maps a ConfirmationStore.Get error to the
// appropriate HTTP status.
func writeConfirmationLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, security.ErrConfirmationNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not_found", Message: "confirmation not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
}

// writeConfirmationResolveError maps a
// ConfirmationResolver.ResolveConfirmation error to the appropriate HTTP
// status.
func writeConfirmationResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, security.ErrConfirmationNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not_found", Message: "confirmation not found"})
	case errors.Is(err, security.ErrConfirmationAlreadyResolved):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "already_resolved", Message: "confirmation is no longer pending"})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
	}
}
