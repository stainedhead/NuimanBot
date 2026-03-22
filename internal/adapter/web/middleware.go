package web

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
	"nuimanbot/internal/usecase/security"
)

const (
	// loginRateLimitCapacity is the maximum number of failed login attempts before throttling.
	loginRateLimitCapacity = 5

	// loginRateLimitWindow is the refill window for the login rate limiter.
	loginRateLimitWindow = time.Minute

	// sanitizeMaxLength is the maximum input length passed to the security validator.
	sanitizeMaxLength = 1024
)

// loginRateLimiterStore manages per-IP token buckets for login rate limiting.
// It is safe for concurrent use.
type loginRateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*ratelimit.TokenBucket
}

// newLoginRateLimiterStore creates an empty per-IP rate limiter store.
func newLoginRateLimiterStore() *loginRateLimiterStore {
	return &loginRateLimiterStore{
		buckets: make(map[string]*ratelimit.TokenBucket),
	}
}

// allow returns true when the given IP is allowed to attempt a login.
// The first call for an IP creates a fresh bucket.
func (s *loginRateLimiterStore) allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	bucket, ok := s.buckets[ip]
	if !ok {
		refillInterval := loginRateLimitWindow / time.Duration(loginRateLimitCapacity)
		bucket = ratelimit.NewTokenBucket(loginRateLimitCapacity, refillInterval)
		s.buckets[ip] = bucket
	}
	return bucket.Allow()
}

// reset removes the rate-limit bucket for the given IP, effectively resetting
// the counter after a successful login.
func (s *loginRateLimiterStore) reset(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buckets, ip)
}

// inputValidator is a package-level validator used for form value sanitization.
// Initialized once at startup; safe for concurrent use.
var inputValidator = security.NewDefaultInputValidator()

// sanitizedFormValue reads a form value by key and returns it sanitized.
// If the value contains injection patterns, an empty string is returned.
// Whitespace is trimmed from safe values.
func sanitizedFormValue(r *http.Request, key string) string {
	raw := r.FormValue(key)
	if raw == "" {
		return ""
	}
	sanitized, err := inputValidator.ValidateInput(r.Context(), raw, sanitizeMaxLength)
	if err != nil {
		// Input failed validation — treat as empty to prevent injection.
		return ""
	}
	return sanitized
}

// requireRole returns middleware that enforces a minimum role level on a handler.
// Unauthenticated requests receive 401. Authenticated requests with insufficient
// role receive 403. The 403 body does not disclose which role is required.
func (s *Server) requireRole(required domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			session := s.auth.GetSession(cookie.Value)
			if session == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			sessionRole := domain.Role(session.Role)
			if !sessionRole.HasPermission(required) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requirePasswordChange returns middleware that redirects to /admin/change-password
// when the current session has the forcePasswordChange flag set. It passes through
// requests to /admin/change-password itself to avoid redirect loops.
func (s *Server) requirePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always allow access to the change-password page itself.
		if r.URL.Path == "/admin/change-password" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		session := s.auth.GetSession(cookie.Value)
		if session == nil {
			next.ServeHTTP(w, r)
			return
		}

		if hasForcePasswordChange(session) {
			http.Redirect(w, r, "/admin/change-password", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// hasForcePasswordChange reports whether the session requires a password change.
func hasForcePasswordChange(session *Session) bool {
	return session.ForcePasswordChange
}

// extractRemoteIP returns the client's IP address from RemoteAddr, stripping the port.
// X-Forwarded-For is intentionally NOT used because this server is not behind a
// trusted reverse proxy; trusting that header would allow IP spoofing.
func extractRemoteIP(r *http.Request) string {
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}
