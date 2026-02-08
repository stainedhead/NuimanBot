package middleware

import (
	"context"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// UserContextKey is the context key for storing authenticated user
const UserContextKey contextKey = "user"

// ProfileService defines the interface for retrieving user profiles
type ProfileService interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.UserProfile, error)
}

// AuthMiddleware handles Bearer token authentication for REST API
type AuthMiddleware struct {
	profileService ProfileService
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(profileService ProfileService) *AuthMiddleware {
	return &AuthMiddleware{
		profileService: profileService,
	}
}

// Authenticate is a middleware that validates Bearer tokens and adds user to context
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Validate token by looking up user profile
		profile, err := m.profileService.GetByAPIKey(r.Context(), token)
		if err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if user is enabled
		if !profile.Enabled {
			http.Error(w, "Forbidden: user account disabled", http.StatusForbidden)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), UserContextKey, profile)

		// Call next handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext retrieves the authenticated user from the request context
func GetUserFromContext(ctx context.Context) (*domain.UserProfile, bool) {
	user, ok := ctx.Value(UserContextKey).(*domain.UserProfile)
	return user, ok
}
