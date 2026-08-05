package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

const (
	// Server timeouts
	readTimeout  = 15 * time.Second
	writeTimeout = 15 * time.Second
	idleTimeout  = 60 * time.Second

	// Shutdown timeout
	shutdownTimeout = 10 * time.Second

	// Default version
	defaultVersion = "1.0.0"
)

// Server represents the web admin server
type Server struct {
	addr                string
	httpServer          *http.Server
	templates           *template.Template
	auth                *AuthService
	loginLimiter        *loginRateLimiterStore
	profileService      ProfileService
	botService          BotService
	confirmationService ConfirmationService
	chatsService        ChatsService
	projectsService     ProjectsService
	jobsService         JobsService
	choresService       ChoresService
	historyService      HistoryService
	memoriesService     MemoriesService
	settingsService     SettingsService
	networkAccess       *networkAccessState
	hub                 *Hub
}

// NewServer creates a new web admin server
func NewServer(addr string) *Server {
	server := &Server{
		addr:          addr,
		auth:          NewAuthService(), // Initialize auth service
		loginLimiter:  newLoginRateLimiterStore(),
		networkAccess: &networkAccessState{},
		hub:           NewHub(),
	}

	// Parse templates
	templates := server.parseTemplates()
	server.templates = templates

	// Create HTTP server with configured timeouts
	mux := http.NewServeMux()
	server.registerRoutes(mux)

	// networkAllowlistMiddleware wraps every route, including /health and
	// /static/ (FR-007's "rejected before reaching application handlers").
	// See its doc comment: a server that never calls SetNetworkAccessConfig
	// behaves exactly as before this field existed.
	var handler http.Handler = mux
	handler = server.networkAllowlistMiddleware(handler)

	server.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	return server
}

// parseTemplates parses all HTML templates from embedded filesystem
func (s *Server) parseTemplates() *template.Template {
	templates, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		slog.Error("Failed to parse templates", "error", err)
		// For testing, create empty template if parsing fails
		return template.New("empty")
	}
	return templates
}

// registerRoutes registers all HTTP routes, applying middleware to admin-only routes.
//
// Route groups:
//   - Public: /health, /static/, /admin/login, /admin/logout
//   - Change-password (authenticated, no role check): /admin/change-password
//   - Admin-only (requireRole(admin) + requirePasswordChange): all other /admin/* routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check endpoint — public
	mux.HandleFunc("/health", s.handleHealth)

	// Root redirect — public
	mux.HandleFunc("/", s.handleRootRedirect)

	// Static files — public
	staticFS := http.FileServer(http.FS(embeddedFS))
	mux.Handle("/static/", staticFS)

	// Authentication routes — public (rate limiter is inside handleLogin)
	mux.HandleFunc("/admin/login", s.handleLogin)
	mux.HandleFunc("/admin/logout", s.handleLogout)

	// Change-password — accessible to authenticated users regardless of force-change flag;
	// does not require admin role so that users with force-change can reach it.
	mux.HandleFunc("/admin/change-password", s.handleChangePassword)

	// adminHandler wraps a handler with admin role enforcement and password-change redirect.
	adminHandler := func(h http.HandlerFunc) http.Handler {
		return s.requireRole(domain.RoleAdmin)(s.requirePasswordChange(h))
	}

	// Admin dashboard routes
	mux.Handle("/admin/dashboard", adminHandler(s.handleDashboard))
	mux.Handle("/admin/dashboard/reload", adminHandler(s.handleReloadConfig))

	// User management routes
	mux.Handle("/admin/users", adminHandler(s.handleUsers))
	mux.Handle("/admin/users/create", adminHandler(s.handleUserCreate))
	mux.Handle("/admin/users/", adminHandler(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/edit") {
			s.handleUserEdit(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/delete") {
			s.handleUserDelete(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	// Bot management routes
	mux.Handle("/admin/bots", adminHandler(s.handleBots))
	mux.Handle("/admin/bots/create", adminHandler(s.handleBotCreate))
	mux.Handle("/admin/bots/", adminHandler(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/edit") {
			s.handleBotEdit(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/delete") {
			s.handleBotDelete(w, r)
		} else {
			http.NotFound(w, r)
		}
	}))

	// Configuration and logging routes
	mux.Handle("/admin/llm", adminHandler(s.handleLLMConfig))
	mux.Handle("/admin/config", adminHandler(s.handleServerConfig))
	mux.Handle("/admin/logs", adminHandler(s.handleLogs))

	// Confirmation routes (Part C, P5.8): RoleUser is sufficient here — access
	// is scoped to the requesting user's own pending confirmations, with
	// RoleAdmin additionally able to see/resolve everyone's (enforced inside
	// the handlers, not by this route-level role check).
	confirmationHandler := func(h http.HandlerFunc) http.Handler {
		return s.requireRole(domain.RoleUser)(s.requirePasswordChange(h))
	}
	mux.Handle("/admin/confirmations", confirmationHandler(s.handleConfirmations))
	mux.Handle("/admin/confirmations/", confirmationHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approve"):
			s.handleConfirmationApprove(w, r)
		case strings.HasSuffix(r.URL.Path, "/deny"):
			s.handleConfirmationDeny(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	// Chats environment routes (FR-011-FR-016): per-user, RoleUser is
	// sufficient — every handler scopes to the requesting session's own
	// Username, matching the confirmations routes' composition above.
	userHandler := func(h http.HandlerFunc) http.Handler {
		return s.requireRole(domain.RoleUser)(s.requirePasswordChange(h))
	}
	mux.Handle("/admin/chats", userHandler(s.handleChats))
	mux.Handle("/admin/chats/", userHandler(s.handleChatSubroutes))

	// Projects/Jobs/Chores/History/Memories environment routes (FR-017-047):
	// per-user, RoleUser, same composition as Chats above. Each
	// <env>_handler.go owns its own handler implementations; this file only
	// owns route registration.
	mux.Handle("/admin/projects", userHandler(s.handleProjects))
	mux.Handle("/admin/projects/", userHandler(s.handleProjectSubroutes))
	mux.Handle("/admin/jobs", userHandler(s.handleJobs))
	mux.Handle("/admin/jobs/", userHandler(s.handleJobSubroutes))
	mux.Handle("/admin/chores", userHandler(s.handleChores))
	mux.Handle("/admin/chores/", userHandler(s.handleChoreSubroutes))
	mux.Handle("/admin/history", userHandler(s.handleHistory))
	mux.Handle("/admin/history/", userHandler(s.handleHistorySubroutes))
	mux.Handle("/admin/memories", userHandler(s.handleMemories))
	mux.Handle("/admin/memories/", userHandler(s.handleMemorySubroutes))

	// Settings (FR-001-FR-004): viewable by any authenticated user (shows
	// per-user retention info), system-wide changes gated to admin inside
	// the handler itself (same fail-closed posture as elsewhere).
	mux.Handle("/admin/settings", userHandler(s.handleSettings))

	// WebSocket endpoint (P6.8): pushes Job/Chore Run status/log/
	// notification-badge updates to the connecting session's own user
	// (handleWebSocket enforces this via s.getCurrentUser, mirroring
	// userHandler's RoleUser posture without the redirect-on-force-
	// password-change behavior, which doesn't apply to a non-page route).
	mux.HandleFunc("/ws", s.handleWebSocket)

	mux.Handle("/admin/", adminHandler(s.handleAdminIndex))
}

// Hub returns the server's WebSocket hub, so DI wiring (cmd/nuimanbot) can
// wrap domain.RunRepository with a notifying decorator that publishes to
// it as the worker pool executes runs.
func (s *Server) Hub() *Hub {
	return s.hub
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode health response", "error", err)
	}
}

// handleRootRedirect redirects / to /admin/
func (s *Server) handleRootRedirect(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Redirect to admin interface
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

// Start starts the web server
func (s *Server) Start() error {
	slog.Info("Starting web admin server", "addr", s.addr)
	return s.httpServer.ListenAndServe()
}

// StartContext starts the web server with context for graceful shutdown
func (s *Server) StartContext(ctx context.Context) error {
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		slog.Info("Starting web admin server", "addr", s.addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return s.shutdown()
	}
}

// Stop gracefully stops the web server
func (s *Server) Stop() error {
	return s.shutdown()
}

// shutdown performs graceful shutdown of the HTTP server
func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	slog.Info("Shutting down web admin server")
	return s.httpServer.Shutdown(ctx)
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return s.addr
}

// GetAuth returns the authentication service for external configuration
func (s *Server) GetAuth() *AuthService {
	return s.auth
}

// BaseData represents common template data passed to all pages
type BaseData struct {
	Title           string
	ActivePage      string
	IsAuthenticated bool
	CurrentUser     *User
	FlashSuccess    string
	FlashError      string
	Version         string
	// UnviewedRunCount drives the History nav item's notification badge
	// (FR-044). Zero (the default) renders no badge. Populated by handlers
	// that have a HistoryService available; pages without one simply leave
	// it at zero rather than requiring every handler to know about History.
	UnviewedRunCount int
}

// User represents a simplified user for template rendering
type User struct {
	ID       string
	Username string
	Role     string
}

// NewBaseData creates a new BaseData with default values
func NewBaseData(title, activePage string) *BaseData {
	return &BaseData{
		Title:           title,
		ActivePage:      activePage,
		IsAuthenticated: false,
		Version:         defaultVersion,
	}
}

// WithUser sets the current user and marks as authenticated
func (bd *BaseData) WithUser(user *User) *BaseData {
	bd.CurrentUser = user
	bd.IsAuthenticated = true
	return bd
}

// WithFlashSuccess adds a success flash message
func (bd *BaseData) WithFlashSuccess(msg string) *BaseData {
	bd.FlashSuccess = msg
	return bd
}

// WithFlashError adds an error flash message
func (bd *BaseData) WithFlashError(msg string) *BaseData {
	bd.FlashError = msg
	return bd
}

// Error404 renders a 404 error page
func (s *Server) Error404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprintf(w, "404 - Page Not Found")
}

// Error500 renders a 500 error page
func (s *Server) Error500(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("Internal server error", "path", r.URL.Path, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "500 - Internal Server Error")
}
