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
	addr           string
	httpServer     *http.Server
	templates      *template.Template
	auth           *AuthService
	profileService ProfileService
	botService     BotService
}

// NewServer creates a new web admin server
func NewServer(addr string) *Server {
	server := &Server{
		addr: addr,
		auth: NewAuthService(), // Initialize auth service
	}

	// Parse templates
	templates := server.parseTemplates()
	server.templates = templates

	// Create HTTP server with configured timeouts
	mux := http.NewServeMux()
	server.registerRoutes(mux)

	server.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
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

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)

	// Root redirect
	mux.HandleFunc("/", s.handleRootRedirect)

	// Static files - serve from embedded filesystem
	staticFS := http.FileServer(http.FS(embeddedFS))
	mux.Handle("/static/", staticFS)

	// Authentication routes
	mux.HandleFunc("/admin/login", s.handleLogin)
	mux.HandleFunc("/admin/logout", s.handleLogout)

	// Admin routes
	mux.HandleFunc("/admin/dashboard", s.handleDashboard)
	mux.HandleFunc("/admin/dashboard/reload", s.handleReloadConfig)

	// User management routes
	mux.HandleFunc("/admin/users", s.handleUsers)
	mux.HandleFunc("/admin/users/create", s.handleUserCreate)
	mux.HandleFunc("/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		// Route to edit or delete based on path
		if strings.HasSuffix(r.URL.Path, "/edit") {
			s.handleUserEdit(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/delete") {
			s.handleUserDelete(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// Bot management routes
	mux.HandleFunc("/admin/bots", s.handleBots)
	mux.HandleFunc("/admin/bots/create", s.handleBotCreate)
	mux.HandleFunc("/admin/bots/", func(w http.ResponseWriter, r *http.Request) {
		// Route to edit or delete based on path
		if strings.HasSuffix(r.URL.Path, "/edit") {
			s.handleBotEdit(w, r)
		} else if strings.HasSuffix(r.URL.Path, "/delete") {
			s.handleBotDelete(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// Configuration and logging routes
	mux.HandleFunc("/admin/llm", s.handleLLMConfig)
	mux.HandleFunc("/admin/config", s.handleServerConfig)
	mux.HandleFunc("/admin/logs", s.handleLogs)
	mux.HandleFunc("/admin/", s.handleAdminIndex)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
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

// renderTemplate renders an HTML template with data
func (s *Server) renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(w, name, data)
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
	fmt.Fprintf(w, "404 - Page Not Found")
}

// Error500 renders a 500 error page
func (s *Server) Error500(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("Internal server error", "path", r.URL.Path, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "500 - Internal Server Error")
}
