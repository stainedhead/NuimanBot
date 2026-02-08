package web

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
)

// BotConfig represents a simplified bot configuration for web display
type BotConfig struct {
	ID       string
	Name     string
	Platform domain.Platform
	Enabled  bool
	IsPublic bool
}

// BotService interface for bot management operations
type BotService interface {
	CreateBot(ctx context.Context, bot *BotConfig) error
	GetBot(ctx context.Context, botID string) (*BotConfig, error)
	UpdateBot(ctx context.Context, botID string, updates map[string]interface{}) error
	DeleteBot(ctx context.Context, botID string) error
	ListBots(ctx context.Context) ([]*BotConfig, error)
}

// SetBotService sets the bot service for the server
func (s *Server) SetBotService(service BotService) {
	s.botService = service
}

// handleBots lists all bots
func (s *Server) handleBots(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Get bots from service
	var bots []*BotConfig
	if s.botService != nil {
		botsResult, err := s.botService.ListBots(r.Context())
		if err != nil {
			slog.Error("Failed to list bots", "error", err)
		} else {
			bots = botsResult
		}
	}

	// Render template
	data := struct {
		Title string
		Bots  []*BotConfig
	}{
		Title: "Bots",
		Bots:  bots,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "bots.html", data); err != nil {
		slog.Error("Failed to render bots template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleBotCreate displays bot creation form or processes creation
func (s *Server) handleBotCreate(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Only admins can create bots
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		// Show create form
		w.Write([]byte("<html><body><h1>Create Bot</h1><p>Bot creation form (simplified for testing)</p></body></html>"))
		return
	}

	if r.Method == http.MethodPost {
		// Simplified bot creation for testing
		botID := r.FormValue("botID")
		name := r.FormValue("name")

		bot := &BotConfig{
			ID:       botID,
			Name:     name,
			Platform: domain.PlatformSlack,
			Enabled:  true,
		}

		if s.botService != nil {
			if err := s.botService.CreateBot(r.Context(), bot); err != nil {
				slog.Error("Failed to create bot", "error", err)
				http.Error(w, "Failed to create bot", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/admin/bots", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleBotEdit displays bot edit form or processes update
func (s *Server) handleBotEdit(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Extract bot ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	botID := pathParts[3]

	if r.Method == http.MethodGet {
		// Simplified edit form for testing
		w.Write([]byte("<html><body><h1>Edit Bot</h1><p>Bot edit form for " + botID + "</p></body></html>"))
		return
	}

	if r.Method == http.MethodPost {
		// Simplified update for testing
		updates := map[string]interface{}{
			"name": r.FormValue("name"),
		}

		if s.botService != nil {
			if err := s.botService.UpdateBot(r.Context(), botID, updates); err != nil {
				slog.Error("Failed to update bot", "error", err, "botID", botID)
				http.Error(w, "Failed to update bot", http.StatusInternalServerError)
				return
			}
		}

		http.Redirect(w, r, "/admin/bots", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleBotDelete deletes a bot
func (s *Server) handleBotDelete(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Only admins can delete bots
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract bot ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	botID := pathParts[3]

	if s.botService != nil {
		if err := s.botService.DeleteBot(r.Context(), botID); err != nil {
			slog.Error("Failed to delete bot", "error", err, "botID", botID)
			http.Error(w, "Failed to delete bot", http.StatusInternalServerError)
			return
		}
	}

	// Redirect to bots list
	http.Redirect(w, r, "/admin/bots", http.StatusFound)
}
