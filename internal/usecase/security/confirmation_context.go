package security

import "context"

// confirmationIdentityKey is an unexported context.Context key type (per the
// standard library's recommended pattern) so ConfirmationIdentity values
// stored via WithConfirmationIdentity cannot collide with keys from other
// packages.
type confirmationIdentityKey struct{}

// ConfirmationIdentity carries the (UserID, ConversationID) pair a caller
// needs Part C's confirmation gating applied against, without requiring a
// full RBAC-capable *domain.User (see tool.Service.Execute vs. ExecuteWithUser
// in internal/usecase/tool/service.go, and the "Known Gap" note in
// specs/260802-improve-nuimanbot-security/implementation-notes.md: chat.Service
// only has a platform user id and a conversation id in its hot path, not a
// role-bearing domain.User).
type ConfirmationIdentity struct {
	UserID         string
	ConversationID string
}

// WithConfirmationIdentity returns a copy of ctx carrying identity, so that
// internal/usecase/tool.Service.Execute (the RBAC-free entry point
// ChatService's tool-calling loop actually calls in production) can still
// apply Part C's confirmation gate for that specific user+conversation.
func WithConfirmationIdentity(ctx context.Context, userID, conversationID string) context.Context {
	return context.WithValue(ctx, confirmationIdentityKey{}, ConfirmationIdentity{
		UserID:         userID,
		ConversationID: conversationID,
	})
}

// ConfirmationIdentityFromContext retrieves the ConfirmationIdentity
// previously stored by WithConfirmationIdentity. ok is false if ctx carries
// none — the common case for any caller (tests, CLI, other tools invoked
// internally) that hasn't opted into confirmation gating.
func ConfirmationIdentityFromContext(ctx context.Context) (ConfirmationIdentity, bool) {
	identity, ok := ctx.Value(confirmationIdentityKey{}).(ConfirmationIdentity)
	return identity, ok
}
