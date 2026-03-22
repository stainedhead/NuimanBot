package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"nuimanbot/internal/adapter/api/middleware"
	"nuimanbot/internal/config"
)

const (
	// restReadTimeout is the maximum time to read an incoming request.
	restReadTimeout = 15 * time.Second

	// restWriteTimeout is the maximum time to write a response.
	restWriteTimeout = 15 * time.Second

	// restIdleTimeout is the maximum time to keep an idle connection alive.
	restIdleTimeout = 60 * time.Second

	// restRateLimitRequests is the number of requests allowed per rateLimitWindow per principal.
	restRateLimitRequests = 100

	// restRateLimitWindow is the rate limit window duration.
	restRateLimitWindow = time.Minute

	// restBodyLimitBytes is the maximum allowed request body size (1 MiB).
	restBodyLimitBytes = 1 << 20

	// minJWTSecretLength is the minimum acceptable byte length for the JWT signing secret.
	// HS256 requires at least 256 bits (32 bytes) of key material to be cryptographically sound.
	minJWTSecretLength = 32
)

// Server is the NuimanBot REST API server.
// It wires the middleware stack and routes in the correct order:
//
//  1. BodyLimit  — enforces 1 MiB request body cap before any auth work
//  2. JWT        — validates the Bearer token and stores the principal in context
//  3. RateLimit  — per-principal token bucket, keyed on the JWT subject claim
//  4. Validate   — sanitises JSON string fields against injection patterns
//  5. Handler    — the actual route handler
//
// The auth endpoint (POST /api/v1/auth/token) is exempt from JWT + RateLimit
// because it is the endpoint used to obtain a token in the first place.
type Server struct {
	httpServer *http.Server
}

// NewServer creates a new REST API Server.
// cfg provides the API key used by the auth endpoint.
// jwtSecret is the HS256 signing secret for issued JWTs.
// It returns an error if jwtSecret is shorter than minJWTSecretLength bytes.
func NewServer(cfg config.ExternalAPIRestConfig, jwtSecret string) (*Server, error) {
	if len(jwtSecret) < minJWTSecretLength {
		return nil, fmt.Errorf(
			"api server: jwt secret must be at least %d bytes, got %d; "+
				"set security.encryption_key to a sufficiently random value",
			minJWTSecretLength, len(jwtSecret),
		)
	}

	mux := http.NewServeMux()

	// Auth endpoint: body limit + validate only (no JWT required).
	authHandler := NewAuthHandler(cfg, jwtSecret)
	mux.Handle("POST /api/v1/auth/token",
		middleware.BodyLimit(restBodyLimitBytes)(
			middleware.Validate()(authHandler),
		),
	)

	// Middleware for protected routes (applied in order).
	protectedChain := func(h http.Handler) http.Handler {
		return middleware.BodyLimit(restBodyLimitBytes)(
			middleware.JWT(jwtSecret)(
				middleware.RateLimit(restRateLimitRequests, restRateLimitWindow)(
					middleware.Validate()(h),
				),
			),
		)
	}

	// Health check — no auth required.
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Protected routes placeholder — add handlers here as the API grows.
	mux.Handle("GET /api/v1/", protectedChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	})))

	_ = protectedChain // suppress unused if no protected routes added yet

	httpServer := &http.Server{
		Handler:      mux,
		ReadTimeout:  restReadTimeout,
		WriteTimeout: restWriteTimeout,
		IdleTimeout:  restIdleTimeout,
	}

	return &Server{httpServer: httpServer}, nil
}

// Start listens on addr and serves requests. It blocks until the server stops.
// Returns http.ErrServerClosed when Shutdown is called.
func (s *Server) Start(addr string) error {
	s.httpServer.Addr = addr
	if err := s.httpServer.ListenAndServe(); err != nil {
		return fmt.Errorf("api server: listen: %w", err)
	}
	return nil
}

// Shutdown gracefully drains active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("api server: shutdown: %w", err)
	}
	return nil
}
