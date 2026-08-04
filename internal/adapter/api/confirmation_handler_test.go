package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/adapter/api/middleware"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/security"
)

// confirmationTestJWTSecret is the HS256 signing secret used by every
// confirmation-endpoint test server in this file. Must be >=32 bytes (see
// minJWTSecretLength).
const confirmationTestJWTSecret = "confirmation-handler-test-secret!!"

// stubConfirmationStore implements ConfirmationStore for handler tests.
type stubConfirmationStore struct {
	getFunc func(ctx context.Context, id string) (security.ConfirmationRequest, error)
}

func (s *stubConfirmationStore) Get(ctx context.Context, id string) (security.ConfirmationRequest, error) {
	return s.getFunc(ctx, id)
}

// stubConfirmationResolver implements ConfirmationResolver for handler tests.
type stubConfirmationResolver struct {
	resolveFunc func(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error)
	calls       int
}

func (s *stubConfirmationResolver) ResolveConfirmation(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error) {
	s.calls++
	if s.resolveFunc != nil {
		return s.resolveFunc(ctx, id, approved)
	}
	return domain.OutgoingMessage{}, nil
}

// buildConfirmationTestServer creates a REST API server wired with the given
// store/resolver stubs.
func buildConfirmationTestServer(t *testing.T, store ConfirmationStore, resolver ConfirmationResolver) *Server {
	t.Helper()
	cfg := config.ExternalAPIRestConfig{
		Enabled: true,
		APIKey:  domain.NewSecureStringFromString("test-api-key"),
	}
	srv, err := NewServer(cfg, confirmationTestJWTSecret, store, resolver)
	require.NoError(t, err)
	return srv
}

// makeConfirmationTestToken builds a JWT (signed with
// confirmationTestJWTSecret) with the given subject and role claims,
// bypassing the normal POST /api/v1/auth/token endpoint — which always
// issues an admin-role token for the sole shared operator API key (see
// claims.go's newClaims) — so tests can exercise the non-admin
// ownership-check path directly. role == "" omits the claim entirely.
func makeConfirmationTestToken(t *testing.T, principalID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": principalID,
		"iss": "nuimanbot",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	if role != "" {
		claims["role"] = role
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(confirmationTestJWTSecret))
	require.NoError(t, err)
	return signed
}

// doConfirmationRequest issues method/path against srv's handler, optionally
// with a bearer token and a JSON body.
func doConfirmationRequest(srv *Server, method, path, token, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rr, req)
	return rr
}

func pendingConfirmationFixture(id, ownerUserID string) security.ConfirmationRequest {
	return security.ConfirmationRequest{
		ID:             id,
		UserID:         ownerUserID,
		ConversationID: "cli:" + ownerUserID,
		ToolName:       "github",
		Summary:        "Merge PR #42?",
		Status:         security.ConfirmationStatusPending,
		CreatedAt:      time.Now().Add(-time.Minute),
		ExpiresAt:      time.Now().Add(time.Hour),
	}
}

// --- GET /api/v1/confirmations/{id} ---------------------------------------

func TestConfirmationGet_OwnerCanRetrieve(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			assert.Equal(t, "conf-1", id)
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", token, "")

	require.Equal(t, http.StatusOK, rr.Code)
	var resp confirmationResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "conf-1", resp.ID)
	assert.Equal(t, "github", resp.ToolName)
	assert.Equal(t, "Merge PR #42?", resp.Summary)
	assert.Equal(t, "pending", resp.Status)
	assert.False(t, resp.ExpiresAt.IsZero())
}

func TestConfirmationGet_AdminCanRetrieveAnyUsersConfirmation(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "someone-else"), nil
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	// A real /auth/token-issued token is always admin-role for the sole
	// shared API key (see claims.go newClaims) — exercise that path too.
	token := issueRealAuthToken(t, srv, "test-api-key")
	rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", token, "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestConfirmationGet_WrongUser_Returns403(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "someone-else", "")
	rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", token, "")

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestConfirmationGet_NotFound_Returns404(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/does-not-exist", token, "")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestConfirmationGet_NoAuth_Returns401(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", "", "")

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- POST /api/v1/confirmations/{id}/resolve ------------------------------

func TestConfirmationResolve_Approve_OwnerCanResolve(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	resolver := &stubConfirmationResolver{
		resolveFunc: func(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error) {
			assert.Equal(t, "conf-1", id)
			assert.True(t, approved)
			return domain.OutgoingMessage{Content: "Done, merged PR #42."}, nil
		},
	}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp resolveConfirmationResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "conf-1", resp.ID)
	assert.Equal(t, "approved", resp.Status)
	assert.Equal(t, "Done, merged PR #42.", resp.Message)
	assert.Equal(t, 1, resolver.calls)
}

func TestConfirmationResolve_Deny_OwnerCanResolve(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	resolver := &stubConfirmationResolver{
		resolveFunc: func(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error) {
			assert.False(t, approved)
			return domain.OutgoingMessage{Content: "Cancelled: Merge PR #42?"}, nil
		},
	}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": false}`)

	require.Equal(t, http.StatusOK, rr.Code)
	var resp resolveConfirmationResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "denied", resp.Status)
	assert.Equal(t, 1, resolver.calls)
}

func TestConfirmationResolve_WrongUser_Returns403_AndDoesNotResolve(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	resolver := &stubConfirmationResolver{}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "someone-else", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Equal(t, 0, resolver.calls, "resolution must not be attempted for a non-owner, non-admin caller")
}

func TestConfirmationResolve_AdminCanResolveAnyUsersConfirmation(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "someone-else"), nil
		},
	}
	resolver := &stubConfirmationResolver{
		resolveFunc: func(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error) {
			return domain.OutgoingMessage{Content: "ok"}, nil
		},
	}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := issueRealAuthToken(t, srv, "test-api-key")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, resolver.calls)
}

func TestConfirmationResolve_MissingConfirmation_Returns404(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return security.ConfirmationRequest{}, security.ErrConfirmationNotFound
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/does-not-exist/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestConfirmationResolve_AlreadyResolved_Returns409(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			fixture := pendingConfirmationFixture("conf-1", "owner-user")
			fixture.Status = security.ConfirmationStatusApproved
			return fixture, nil
		},
	}
	resolver := &stubConfirmationResolver{}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, 0, resolver.calls, "an already-resolved confirmation must not be re-resolved")
}

func TestConfirmationResolve_Expired_Returns409(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			fixture := pendingConfirmationFixture("conf-1", "owner-user")
			fixture.ExpiresAt = time.Now().Add(-time.Minute) // past deadline, still "pending" (background job hasn't flipped it yet)
			return fixture, nil
		},
	}
	resolver := &stubConfirmationResolver{}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, 0, resolver.calls)
}

func TestConfirmationResolve_ResolverReturnsAlreadyResolvedError_Returns409(t *testing.T) {
	// Covers the race where the confirmation looked Pending at Get time but
	// was resolved by a concurrent request before ResolveConfirmation ran.
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	resolver := &stubConfirmationResolver{
		resolveFunc: func(ctx context.Context, id string, approved bool) (domain.OutgoingMessage, error) {
			return domain.OutgoingMessage{}, security.ErrConfirmationAlreadyResolved
		},
	}
	srv := buildConfirmationTestServer(t, store, resolver)

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": true}`)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestConfirmationResolve_MalformedJSON_Returns400(t *testing.T) {
	srv := buildConfirmationTestServer(t, &stubConfirmationStore{}, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{"approved": "yes"}`)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestConfirmationResolve_MissingApprovedField_Returns400(t *testing.T) {
	srv := buildConfirmationTestServer(t, &stubConfirmationStore{}, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, `{}`)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestConfirmationResolve_EmptyBody_Returns400(t *testing.T) {
	srv := buildConfirmationTestServer(t, &stubConfirmationStore{}, &stubConfirmationResolver{})

	token := makeConfirmationTestToken(t, "owner-user", "")
	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", token, "")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestConfirmationResolve_NoAuth_Returns401(t *testing.T) {
	srv := buildConfirmationTestServer(t, &stubConfirmationStore{}, &stubConfirmationResolver{})

	rr := doConfirmationRequest(srv, http.MethodPost, "/api/v1/confirmations/conf-1/resolve", "", `{"approved": true}`)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- Rate limiting ---------------------------------------------------------

// TestConfirmationGet_RateLimit_Returns429AfterLimit exercises the same
// per-route rate-limit pattern as the rest of the protected API (see
// protectedChain in server.go): each protectedChain-wrapped route gets its
// own token-bucket registry, so exceeding restRateLimitRequests requests
// against the confirmation GET route alone triggers 429.
func TestConfirmationGet_RateLimit_Returns429AfterLimit(t *testing.T) {
	store := &stubConfirmationStore{
		getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
			return pendingConfirmationFixture("conf-1", "owner-user"), nil
		},
	}
	srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})
	token := makeConfirmationTestToken(t, "owner-user", "")

	got429 := false
	for i := 0; i < restRateLimitRequests+1; i++ {
		rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", token, "")
		if rr.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	assert.True(t, got429, "should receive 429 after exceeding the per-route rate limit")
}

// issueRealAuthToken drives the actual POST /api/v1/auth/token endpoint to
// obtain a genuine, admin-role token for the given API key.
func issueRealAuthToken(t *testing.T, srv *Server, apiKey string) string {
	t.Helper()
	body := `{"api_key":"` + apiKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp tokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Token)
	return resp.Token
}

// --- FR-006: single-shared-key model — documented ownership limitation ----
//
// These tests lock in the ACTUAL current behavior of confirmationAuthorized
// and the REST confirmation endpoints under the REST API's single-shared-key
// auth model: every credential the API currently issues (via the sole
// POST /api/v1/auth/token flow, for the one shared operator API key) carries
// Role: middleware.RoleAdmin (see claims.go's newClaims), so the ownership
// comparison in confirmationAuthorized is unreachable for any real caller —
// any valid API credential can view or resolve ANY user's pending
// confirmation. This is intentionally NOT the previously-implied per-user-
// scoped behavior; see specs/260803-improve-nuimanbot-security-auto-review's
// OQ-2 for the decision to document (not fix) this for now.
//
// If the REST API's auth model is later extended to issue genuine per-user
// (non-admin-by-default) credentials, these tests SHOULD start failing —
// that is the intended signal that confirmationAuthorized's doc comment,
// this test, and support_docs/security-hardening-guide.md /
// support_docs/api-reference.md all need to be revisited together, rather
// than the auth-model change silently reintroducing (or silently fixing)
// this gap unnoticed.

// TestConfirmationAuthorized_SingleSharedKeyModel_AnyCredentialCanAccessAnyUsersConfirmation
// unit-tests confirmationAuthorized directly: a principal carrying the role
// every real REST credential currently carries (middleware.RoleAdmin) is
// authorized for a confirmation owned by a completely different, arbitrary
// user ID — proving the ownership check is non-discriminating today.
func TestConfirmationAuthorized_SingleSharedKeyModel_AnyCredentialCanAccessAnyUsersConfirmation(t *testing.T) {
	ctx := middleware.ContextWithPrincipal(context.Background(), "shared-key-principal")
	ctx = middleware.ContextWithRole(ctx, middleware.RoleAdmin)

	for _, ownerUserID := range []string{"owner-user", "someone-else", "a-totally-different-user", ""} {
		assert.True(t, confirmationAuthorized(ctx, ownerUserID),
			"under the current single-shared-key model, an admin-role principal (the only kind the REST API issues) must be authorized for owner %q — if this now fails, per-user REST credentials may have landed and this test/comment/docs need updating together", ownerUserID)
	}
}

// TestConfirmationAuthorized_NonAdminPrincipal_StillEnforcesOwnership documents
// the flip side: the ownership branch is not broken, merely unreachable
// today. A hypothetical non-admin principal (not obtainable via any current
// REST credential-issuance path, but exercised here directly against
// confirmationAuthorized, and via makeConfirmationTestToken at the handler
// level elsewhere in this file) is still correctly denied access to another
// user's confirmation and still correctly granted access to its own.
func TestConfirmationAuthorized_NonAdminPrincipal_StillEnforcesOwnership(t *testing.T) {
	ctx := middleware.ContextWithPrincipal(context.Background(), "owner-user")
	// No role set — RoleFromContext returns "" (non-admin).

	assert.True(t, confirmationAuthorized(ctx, "owner-user"), "a non-admin principal must still be authorized for its own confirmation")
	assert.False(t, confirmationAuthorized(ctx, "someone-else"), "a non-admin principal must still be denied access to another user's confirmation")
}

// TestConfirmationGet_SingleSharedKeyModel_RealTokenCrossesUserBoundary
// exercises the full HTTP path with a token obtained the way a real client
// would obtain one (POST /api/v1/auth/token with the shared operator API
// key) against confirmations owned by several different, arbitrary users —
// none of which match the token's principal ID — to confirm none of them
// are rejected. This is the end-to-end manifestation of FR-006's finding.
func TestConfirmationGet_SingleSharedKeyModel_RealTokenCrossesUserBoundary(t *testing.T) {
	for _, ownerUserID := range []string{"someone-else", "a-totally-different-user"} {
		store := &stubConfirmationStore{
			getFunc: func(ctx context.Context, id string) (security.ConfirmationRequest, error) {
				return pendingConfirmationFixture("conf-1", ownerUserID), nil
			},
		}
		srv := buildConfirmationTestServer(t, store, &stubConfirmationResolver{})

		token := issueRealAuthToken(t, srv, "test-api-key")
		rr := doConfirmationRequest(srv, http.MethodGet, "/api/v1/confirmations/conf-1", token, "")

		assert.Equal(t, http.StatusOK, rr.Code, "real shared-key-issued token must currently be able to read owner %q's confirmation (documented FR-006 limitation)", ownerUserID)
	}
}
