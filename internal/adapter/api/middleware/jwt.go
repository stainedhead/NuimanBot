// Package middleware provides HTTP middleware for the NuimanBot REST API.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported type for context keys in this package.
// Using a custom type prevents collisions with keys from other packages.
type contextKey int

const (
	// principalKey is the context key for the authenticated principal ID.
	principalKey contextKey = iota
)

// JWT returns a middleware that validates a Bearer JWT in the Authorization header.
// On success, the JWT subject claim is stored in the request context and the next
// handler is called. On failure, a structured 401 JSON response is returned.
func JWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, err := extractBearerToken(r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}

			sub, err := validateJWT(tokenStr, secret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
				return
			}

			ctx := ContextWithPrincipal(r.Context(), sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken extracts the token string from the "Authorization: Bearer <token>" header.
func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("authorization header missing")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return "", fmt.Errorf("authorization header must use Bearer scheme")
	}
	token := strings.TrimPrefix(auth, prefix)
	if token == "" {
		return "", fmt.Errorf("bearer token is empty")
	}
	return token, nil
}

// validateJWT parses and validates the token string, returning the subject claim.
func validateJWT(tokenStr, secret string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("token validation failed")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims type")
	}

	sub, err := claims.GetSubject()
	if err != nil || sub == "" {
		return "", fmt.Errorf("subject claim missing or empty")
	}

	return sub, nil
}

// ContextWithPrincipal returns a new context with the principal ID stored under principalKey.
func ContextWithPrincipal(ctx context.Context, principalID string) context.Context {
	return context.WithValue(ctx, principalKey, principalID)
}

// PrincipalFromContext retrieves the principal ID from the context.
// Returns an empty string if no principal is set.
func PrincipalFromContext(ctx context.Context) string {
	v, _ := ctx.Value(principalKey).(string)
	return v
}
