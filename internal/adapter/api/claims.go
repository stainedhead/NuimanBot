// Package api implements the REST API adapter layer for NuimanBot's external REST API.
package api

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"nuimanbot/internal/adapter/api/middleware"
)

const (
	// jwtIssuer is the issuer claim value for all JWTs issued by NuimanBot.
	jwtIssuer = "nuimanbot"

	// jwtExpiry is the duration for which a JWT is valid after issuance.
	jwtExpiry = 24 * time.Hour
)

// NuimanClaims defines the JWT claims issued by the auth token endpoint.
type NuimanClaims struct {
	jwt.RegisteredClaims
	// Role is the principal's role, checked by resource-ownership guards
	// (e.g. the confirmation endpoints — see middleware.RoleFromContext /
	// middleware.RoleAdmin). Always "admin": the REST API currently issues
	// tokens for a single, shared operator API key rather than per-end-user
	// credentials (see newClaims), so the sole credential it recognizes is
	// treated as administrative.
	Role string `json:"role,omitempty"`
}

// newClaims creates a new NuimanClaims for the given API key.
// The subject is set to the SHA-256 hex digest of the API key so the raw key
// is never stored in the token payload.
func newClaims(apiKey string) NuimanClaims {
	sub := principalID(apiKey)
	now := time.Now()
	return NuimanClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtExpiry)),
		},
		Role: middleware.RoleAdmin,
	}
}

// signToken signs the claims with HS256 using the provided secret and returns
// the compact token string.
func signToken(claims NuimanClaims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("api: sign token: %w", err)
	}
	return signed, nil
}

// principalID returns a stable identifier derived from the API key.
// Uses SHA-256 so the raw key is never embedded in tokens or logs.
func principalID(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", h[:8]) // 16-char hex prefix — sufficient for a principal ID
}
