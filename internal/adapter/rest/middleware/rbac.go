package middleware

import (
	"context"
	"net/http"

	"nuimanbot/internal/domain"
)

// RequireRole returns a middleware that checks if the user has any of the specified roles.
// The user must be in the request context (added by AuthMiddleware).
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			user, ok := GetUserFromContext(r.Context())
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check if user has required role
			hasRole := false
			for _, role := range roles {
				if user.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// User has required role, continue
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin is a convenience middleware that requires admin role
func RequireAdmin(next http.Handler) http.Handler {
	return RequireRole(domain.RoleAdmin)(next)
}

// RequireUser is a convenience middleware that requires user or admin role
// (admin has all user permissions)
func RequireUser(next http.Handler) http.Handler {
	return RequireRole(domain.RoleUser, domain.RoleAdmin)(next)
}

// WithUser is a helper function to add a user to a context (useful for testing)
func WithUser(ctx context.Context, user *domain.UserProfile) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}
