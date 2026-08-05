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

// networkAccessState holds the server's current network-access configuration
// (FR-005–FR-008), guarded for concurrent access since it may be updated at
// runtime (Settings UI, system-wide/admin-only). The zero value has an empty
// Mode, which networkAllowlistMiddleware treats as "unconfigured — allow
// everything": this package must not silently start enforcing localhost-only
// just because a *Server exists (that would break every existing httptest-
// based caller, which never sets a loopback RemoteAddr). Production callers
// (cmd/nuimanbot's DI wiring) call SetNetworkAccessConfig with the loaded
// config's ToDomain() value — which itself defaults an absent/empty config
// section to AccessModeLocalhostOnly — so the secure-by-default behavior
// described in spec.md's Breaking Changes section is real once wired, without
// this package silently changing behavior for any caller that hasn't wired
// it in.
type networkAccessState struct {
	mu  sync.RWMutex
	cfg domain.NetworkAccessConfig
}

func (n *networkAccessState) get() domain.NetworkAccessConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.cfg
}

func (n *networkAccessState) set(cfg domain.NetworkAccessConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cfg = cfg
}

// SetNetworkAccessConfig updates the server's network-access configuration
// (FR-005–FR-008), taking effect for subsequent requests. Safe to call
// concurrently with request handling (e.g. from a Settings save handler).
func (s *Server) SetNetworkAccessConfig(cfg domain.NetworkAccessConfig) {
	s.networkAccess.set(cfg)
}

// networkAllowlistMiddleware enforces FR-005–FR-008 ahead of every other
// handler, including /health and /static/ — pre-auth, fail-closed per
// spec.md's Security NFR ("Remote-access allowlist enforcement happens at
// the network/middleware layer, before authentication, fail-closed"). A
// request from a host IsAllowed rejects receives 403 without reaching any
// application handler.
//
// An unconfigured server (Mode == "") passes every request through
// unchanged — see networkAccessState's doc comment for why.
func (s *Server) networkAllowlistMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.networkAccess.get()
		if cfg.Mode == "" {
			next.ServeHTTP(w, r)
			return
		}

		host := extractRemoteIP(r)
		if !cfg.IsAllowed(host) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

const (
	// loginRateLimitCapacity is the maximum number of failed login attempts before throttling.
	loginRateLimitCapacity = 5

	// loginRateLimitWindow is the refill window for the login rate limiter.
	loginRateLimitWindow = time.Minute

	// sanitizeMaxLength is the maximum input length passed to the security validator.
	sanitizeMaxLength = 1024

	// loginBucketEvictInterval is how often the background eviction goroutine runs.
	loginBucketEvictInterval = 10 * time.Minute

	// loginBucketIdleTimeout is the maximum time an IP bucket can be idle before eviction.
	loginBucketIdleTimeout = 1 * time.Hour
)

// ipBucket pairs a token bucket with the last time it was accessed.
type ipBucket struct {
	bucket   *ratelimit.TokenBucket
	lastUsed time.Time
}

// loginRateLimiterStore manages per-IP token buckets for login rate limiting.
// It is safe for concurrent use.
type loginRateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*ipBucket
}

// newLoginRateLimiterStore creates an empty per-IP rate limiter store and starts
// a background goroutine that periodically evicts idle buckets to prevent unbounded
// memory growth from IPs that never return (e.g., port scanners).
func newLoginRateLimiterStore() *loginRateLimiterStore {
	s := &loginRateLimiterStore{
		buckets: make(map[string]*ipBucket),
	}
	// Run eviction on a fixed interval rather than per-request to avoid lock
	// contention and to keep the eviction cost O(n) amortized over many requests.
	go func() {
		ticker := time.NewTicker(loginBucketEvictInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.evictIdle(loginBucketIdleTimeout)
		}
	}()
	return s
}

// allow returns true when the given IP is allowed to attempt a login.
// The first call for an IP creates a fresh bucket; subsequent calls stamp lastUsed.
func (s *loginRateLimiterStore) allow(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.buckets[ip]
	if !ok {
		refillInterval := loginRateLimitWindow / time.Duration(loginRateLimitCapacity)
		entry = &ipBucket{
			bucket:   ratelimit.NewTokenBucket(loginRateLimitCapacity, refillInterval),
			lastUsed: time.Now(),
		}
		s.buckets[ip] = entry
	}
	entry.lastUsed = time.Now()
	return entry.bucket.Allow()
}

// reset removes the rate-limit bucket for the given IP, effectively resetting
// the counter after a successful login.
func (s *loginRateLimiterStore) reset(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.buckets, ip)
}

// evictIdle removes buckets that have not been used within maxIdleTime.
// This prevents unbounded map growth from IPs that send a single login attempt
// and never return (e.g., automated scanners).
func (s *loginRateLimiterStore) evictIdle(maxIdleTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxIdleTime)
	for ip, entry := range s.buckets {
		if entry.lastUsed.Before(cutoff) {
			delete(s.buckets, ip)
		}
	}
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
