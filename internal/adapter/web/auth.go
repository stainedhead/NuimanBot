package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionTimeout    = 24 * time.Hour
	sessionCookieName = "session_id"
	csrfTokenLength   = 32
	sessionIDLength   = 32
	bcryptCost        = 12
)

// AuthService handles authentication and session management
type AuthService struct {
	users          map[string]*AuthUser
	sessions       map[string]*Session
	csrfTokens     map[string]bool
	mu             sync.RWMutex
	sessionTimeout time.Duration
	secureCookies  bool // set to true when running under TLS
}

// AuthUser represents an authenticated user
type AuthUser struct {
	Username     string
	PasswordHash string
	Role         string
}

// Session represents a user session
type Session struct {
	ID                  string
	Username            string
	Role                string
	CreatedAt           time.Time
	ExpiresAt           time.Time
	ForcePasswordChange bool // set when user must change default credentials
}

// NewAuthService creates a new authentication service
func NewAuthService() *AuthService {
	return &AuthService{
		users:          make(map[string]*AuthUser),
		sessions:       make(map[string]*Session),
		csrfTokens:     make(map[string]bool),
		sessionTimeout: sessionTimeout,
	}
}

// AddUser adds a user to the authentication service (for testing and setup)
func (a *AuthService) AddUser(username, password, role string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	a.users[username] = &AuthUser{
		Username:     username,
		PasswordHash: string(hashedPassword),
		Role:         role,
	}

	return nil
}

// ValidateCredentials checks if username and password are valid
func (a *AuthService) ValidateCredentials(username, password string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	user, exists := a.users[username]
	if !exists {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// CreateSession creates a new session for the user
func (a *AuthService) CreateSession(username, role string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Generate random session ID
	sessionID := generateRandomString(sessionIDLength)

	// Create session
	session := &Session{
		ID:        sessionID,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(a.sessionTimeout),
	}

	a.sessions[sessionID] = session

	// Clean up expired sessions periodically
	go a.cleanupExpiredSessions()

	return sessionID
}

// ValidateSession checks if a session ID is valid
func (a *AuthService) ValidateSession(sessionID string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		return false
	}

	return true
}

// GetSession retrieves session information
func (a *AuthService) GetSession(sessionID string) *Session {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return nil
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		return nil
	}

	return session
}

// DestroySession removes a session
func (a *AuthService) DestroySession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.sessions, sessionID)
}

// GenerateCSRFToken generates a new CSRF token
func (a *AuthService) GenerateCSRFToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	token := generateRandomString(csrfTokenLength)
	a.csrfTokens[token] = true

	return token
}

// ValidateCSRFToken validates and consumes a CSRF token
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

// cleanupExpiredSessions removes expired sessions
func (a *AuthService) cleanupExpiredSessions() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	for sessionID, session := range a.sessions {
		if now.After(session.ExpiresAt) {
			delete(a.sessions, sessionID)
		}
	}
}

// generateRandomString generates a random base64 encoded string
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
		user, exists := s.auth.users[username]
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

// UpdatePassword updates the stored password hash for a user.
func (a *AuthService) UpdatePassword(username, newPassword string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	user, exists := a.users[username]
	if !exists {
		return fmt.Errorf("web: update password: user %q not found", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("web: update password: hash: %w", err)
	}

	user.PasswordHash = string(hash)
	return nil
}

// ClearForcePasswordChange removes the ForcePasswordChange flag from the session.
func (a *AuthService) ClearForcePasswordChange(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session, ok := a.sessions[sessionID]; ok {
		session.ForcePasswordChange = false
	}
}

// createSessionWithFlags creates a new session, optionally setting the
// ForcePasswordChange flag for accounts using default credentials.
func (a *AuthService) createSessionWithFlags(username, role string, forcePasswordChange bool) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	sessionID := generateRandomString(sessionIDLength)

	session := &Session{
		ID:                  sessionID,
		Username:            username,
		Role:                role,
		CreatedAt:           time.Now(),
		ExpiresAt:           time.Now().Add(a.sessionTimeout),
		ForcePasswordChange: forcePasswordChange,
	}

	a.sessions[sessionID] = session

	go a.cleanupExpiredSessions()

	return sessionID
}

// isDefaultCredentials reports whether the supplied plaintext password matches
// the default "admin" password for the "admin" user.
// Detection uses bcrypt.CompareHashAndPassword for constant-time comparison,
// preventing timing-based enumeration of whether a password matches "admin".
func (a *AuthService) isDefaultCredentials(username, password string) bool {
	if username != defaultAdminUsername {
		return false
	}
	// Use bcrypt to verify against the known default password hash.
	// This is constant-time by design.
	return bcrypt.CompareHashAndPassword([]byte(defaultAdminPasswordHash), []byte(password)) == nil
}

const (
	// defaultAdminUsername is the well-known default administrator account name.
	defaultAdminUsername = "admin"

	// defaultAdminPasswordHash is the bcrypt hash of the literal string "admin".
	// Cost 10 keeps the hash stable for detection purposes only (not for authentication).
	// Authentication always uses the stored hash from AddUser.
	defaultAdminPasswordHash = "$2a$10$Y6HD25IiXnjpqGnkUrK02uZBvmdpzv6vB3eGFCcEeIn1jSZlsrd2e"
)

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
