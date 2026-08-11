package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nuimanbot/internal/usecase/auth"
)

const (
	// sessionTimeout mirrors internal/usecase/auth.Service's default session
	// lifetime. It is duplicated here (rather than exported from auth) solely
	// so handleLogin can compute the session cookie's MaxAge without reaching
	// into the usecase layer for an HTTP-transport concern.
	sessionTimeout    = 24 * time.Hour
	sessionCookieName = "session_id"
	csrfTokenLength   = 32
)

// Session and AuthUser are type aliases for internal/usecase/auth's exported
// structs. A type alias is safe here — unlike AuthService below — because
// every field of both structs is exported; nothing in this package or its
// tests reaches into an unexported field or method of Session/AuthUser.
type (
	Session  = auth.Session
	AuthUser = auth.AuthUser
)

// AuthService adapts internal/usecase/auth.Service for HTTP-transport
// concerns (cookies, CSRF) that don't belong in the platform-agnostic
// usecase layer. It embeds *auth.Service, so all credential/session methods
// (ValidateCredentials, CreateSession, ValidateSession, GetSession,
// DestroySession, UpdatePassword, ClearForcePasswordChange,
// CreateSessionWithFlags, IsDefaultCredentials, GetUser, RestoreSession) are
// promoted and callable directly on *AuthService.
//
// This is a wrapper struct, not a type alias — see
// specs/260811-cli-parity-for-nuimanbot-features/architecture.md AD-1 for
// why a type alias does not compile against this package's existing
// white-box tests and production call sites.
type AuthService struct {
	*auth.Service
	csrfTokens    map[string]bool
	secureCookies bool         // set to true when running under TLS
	mu            sync.RWMutex // guards csrfTokens/secureCookies only; auth.Service has its own internal locking
}

// NewAuthService creates a new authentication service backed by its own
// private internal/usecase/auth.Service. Used when the web server owns
// authentication state exclusively (e.g. most tests).
func NewAuthService() *AuthService {
	return &AuthService{
		Service:    auth.NewService(),
		csrfTokens: make(map[string]bool),
	}
}

// NewAuthServiceWith wraps an existing, shared internal/usecase/auth.Service
// for HTTP-transport concerns. cmd/nuimanbot/main.go uses this so the web
// admin UI and the CLI gateway authenticate against the exact same
// account/session store, regardless of whether the web UI is enabled.
func NewAuthServiceWith(shared *auth.Service) *AuthService {
	return &AuthService{
		Service:    shared,
		csrfTokens: make(map[string]bool),
	}
}

// GenerateCSRFToken generates a new CSRF token. CSRF protects browser form
// submissions against cross-site forgery; it is meaningless for the CLI's
// terminal REPL (no browser/cookie surface), so it stays entirely in this
// package rather than moving to internal/usecase/auth.
func (a *AuthService) GenerateCSRFToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	token := generateRandomString(csrfTokenLength)
	a.csrfTokens[token] = true

	return token
}

// ValidateCSRFToken validates and consumes a CSRF token.
func (a *AuthService) ValidateCSRFToken(token string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	valid, exists := a.csrfTokens[token]
	if !exists || !valid {
		return false
	}

	// Token can only be used once
	delete(a.csrfTokens, token)

	return true
}

// generateRandomString generates a random base64 encoded string.
//
// Duplicated (unexported, identical implementation) from
// internal/usecase/auth rather than imported: GenerateCSRFToken needs it and
// CSRF stays in this package per AD-1's classification. Keeping two small
// copies also lets TestGenerateRandomString (auth_coverage_test.go) keep
// compiling unmodified, holding AD-1's "exactly two relocated tests" count.
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// SetAuthService sets the auth service for the server
func (s *Server) SetAuthService(auth *AuthService) {
	s.auth = auth
}

// RequireAuth is middleware that requires authentication
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// Validate session
		if s.auth == nil || !s.auth.ValidateSession(cookie.Value) {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// Session is valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// handleLogin handles login page and authentication.
// It enforces per-IP rate limiting on POST requests and detects default credentials.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Show login form
		data := struct {
			Title     string
			CSRFToken string
			Error     string
		}{
			Title:     "Login",
			CSRFToken: s.auth.GenerateCSRFToken(),
			Error:     "",
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("Failed to render login template", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		clientIP := extractRemoteIP(r)

		// Check rate limit before processing credentials.
		if s.loginLimiter != nil && !s.loginLimiter.allow(clientIP) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Sanitize form inputs before processing.
		username := sanitizedFormValue(r, "username")
		password := sanitizedFormValue(r, "password")
		csrfToken := r.FormValue("csrf_token") // CSRF token is validated, not sanitized for injection

		// Validate CSRF token
		if s.auth != nil && !s.auth.ValidateCSRFToken(csrfToken) {
			// CSRF validation failed, but show login form again for usability
			data := struct {
				Title     string
				CSRFToken string
				Error     string
			}{
				Title:     "Login",
				CSRFToken: s.auth.GenerateCSRFToken(),
				Error:     "Invalid request. Please try again.",
			}
			w.WriteHeader(http.StatusUnauthorized)
			if err := s.templates.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("Failed to render login template", "error", err)
			}
			return
		}

		// Validate credentials
		if s.auth == nil || !s.auth.ValidateCredentials(username, password) {
			data := struct {
				Title     string
				CSRFToken string
				Error     string
			}{
				Title:     "Login",
				CSRFToken: s.auth.GenerateCSRFToken(),
				Error:     "Invalid username or password",
			}
			w.WriteHeader(http.StatusUnauthorized)
			if err := s.templates.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("Failed to render login template", "error", err)
			}
			return
		}

		// Credentials are valid — reset the rate-limit bucket for this IP.
		if s.loginLimiter != nil {
			s.loginLimiter.reset(clientIP)
		}

		// Get user role
		user, exists := s.auth.GetUser(username)
		if !exists {
			http.Error(w, "User not found", http.StatusInternalServerError)
			return
		}

		// Detect default credentials using constant-time comparison (via bcrypt).
		forceChange := s.auth.isDefaultCredentials(username, password)

		// Create session
		sessionID := s.auth.createSessionWithFlags(username, user.Role, forceChange)

		// Set session cookie — Secure flag is enabled when running under TLS.
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   int(sessionTimeout.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   s.auth.isSecureCookies(),
		})

		// Redirect default-credential users to change-password page.
		if forceChange {
			http.Redirect(w, r, "/admin/change-password", http.StatusFound)
			return
		}

		// Redirect to dashboard
		http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" && s.auth != nil {
		// Destroy session
		s.auth.DestroySession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Redirect to login
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

// handleChangePassword handles the forced password change page.
// On GET, it renders a simple form. On POST, it updates the password and clears
// the ForcePasswordChange flag so the user can proceed.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html><html><head><title>Change Password</title></head>` +
			`<body><h1>Change Your Password</h1>` +
			`<form method="POST"><input type="password" name="new_password" placeholder="New password" required>` +
			`<button type="submit">Change Password</button></form></body></html>`
		if _, err := w.Write([]byte(html)); err != nil {
			slog.Error("Failed to write change-password page", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		newPassword := sanitizedFormValue(r, "new_password")
		if newPassword == "" {
			http.Error(w, "Password required", http.StatusBadRequest)
			return
		}

		// Get current session
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		session := s.auth.GetSession(cookie.Value)
		if session == nil {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}

		// Update the user's password
		if updateErr := s.auth.UpdatePassword(session.Username, newPassword); updateErr != nil {
			slog.Error("Failed to update password", "error", updateErr)
			http.Error(w, "Failed to update password", http.StatusInternalServerError)
			return
		}

		// Clear the force-change flag on the session
		s.auth.ClearForcePasswordChange(cookie.Value)

		http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// getCurrentUser extracts current user from request context
func (s *Server) getCurrentUser(r *http.Request) *User {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}

	if s.auth == nil {
		return nil
	}

	session := s.auth.GetSession(cookie.Value)
	if session == nil {
		return nil
	}

	return &User{
		ID:       session.ID,
		Username: session.Username,
		Role:     session.Role,
	}
}

// setSecureCookies enables or disables the Secure flag on session cookies.
// It should be called with true when the server is running under TLS.
func (a *AuthService) setSecureCookies(secure bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.secureCookies = secure
}

// isSecureCookies returns whether the Secure flag should be set on session cookies.
func (a *AuthService) isSecureCookies() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.secureCookies
}

// createSessionWithFlags is a thin shim over the embedded auth.Service's
// exported CreateSessionWithFlags, declared here (lowercase, in package web)
// so existing white-box tests that call the pre-extraction unexported method
// name keep compiling unmodified.
func (a *AuthService) createSessionWithFlags(username, role string, forcePasswordChange bool) string {
	return a.CreateSessionWithFlags(username, role, forcePasswordChange)
}

// isDefaultCredentials is a thin shim over the embedded auth.Service's
// exported IsDefaultCredentials — see createSessionWithFlags's doc comment.
func (a *AuthService) isDefaultCredentials(username, password string) bool {
	return a.IsDefaultCredentials(username, password)
}
