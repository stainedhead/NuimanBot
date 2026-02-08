package web

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"nuimanbot/internal/domain"
)

// ProfileService interface for user profile operations
type ProfileService interface {
	CreateProfile(ctx context.Context, profile *domain.UserProfile) error
	GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error)
	UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error
	DeleteProfile(ctx context.Context, userID string) error
	ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
}

// SetProfileService sets the profile service for the server
func (s *Server) SetProfileService(service ProfileService) {
	s.profileService = service
}

// handleUsers lists all users
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Get users from service
	var users []*domain.UserProfile
	if s.profileService != nil {
		profiles, err := s.profileService.ListProfiles(r.Context(), 0, 100)
		if err != nil {
			slog.Error("Failed to list profiles", "error", err)
		} else {
			users = profiles
		}
	}

	// Render template
	data := struct {
		Title string
		Users []*domain.UserProfile
	}{
		Title: "Users",
		Users: users,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "users.html", data); err != nil {
		slog.Error("Failed to render users template", "error", err)
		s.Error500(w, r, err)
	}
}

// handleUserCreate displays user creation form or processes creation
func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Only admins can create users
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		// Show create form
		data := struct {
			Title string
			User  *domain.UserProfile
		}{
			Title: "Create User",
			User:  &domain.UserProfile{}, // Empty user for form
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "user_form.html", data); err != nil {
			slog.Error("Failed to render user form template", "error", err)
			s.Error500(w, r, err)
		}
		return
	}

	if r.Method == http.MethodPost {
		// Process form submission
		userID := r.FormValue("userID")
		firstName := r.FormValue("firstName")
		lastName := r.FormValue("lastName")
		email := r.FormValue("primaryEmail")
		role := r.FormValue("role")

		// Create user profile
		profile := &domain.UserProfile{
			UserID:       userID,
			FirstName:    firstName,
			LastName:     lastName,
			PrimaryEmail: email,
			Role:         domain.Role(role),
		}

		if s.profileService != nil {
			if err := s.profileService.CreateProfile(r.Context(), profile); err != nil {
				slog.Error("Failed to create profile", "error", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
		}

		// Redirect to users list
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleUserEdit displays user edit form or processes update
func (s *Server) handleUserEdit(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Extract user ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	userID := pathParts[3]

	if r.Method == http.MethodGet {
		// Get user profile
		var profile *domain.UserProfile
		if s.profileService != nil {
			var err error
			profile, err = s.profileService.GetProfile(r.Context(), userID)
			if err != nil {
				slog.Error("Failed to get profile", "error", err, "userID", userID)
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
		}

		// Show edit form
		data := struct {
			Title string
			User  *domain.UserProfile
		}{
			Title: "Edit User",
			User:  profile,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.templates.ExecuteTemplate(w, "user_form.html", data); err != nil {
			slog.Error("Failed to render user form template", "error", err)
			s.Error500(w, r, err)
		}
		return
	}

	if r.Method == http.MethodPost {
		// Process form submission
		updates := map[string]interface{}{
			"firstName":    r.FormValue("firstName"),
			"lastName":     r.FormValue("lastName"),
			"primaryEmail": r.FormValue("primaryEmail"),
			"role":         r.FormValue("role"),
		}

		if s.profileService != nil {
			if err := s.profileService.UpdateProfile(r.Context(), userID, updates); err != nil {
				slog.Error("Failed to update profile", "error", err, "userID", userID)
				http.Error(w, "Failed to update user", http.StatusInternalServerError)
				return
			}
		}

		// Redirect to users list
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleUserDelete deletes a user
func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Only admins can delete users
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract user ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	userID := pathParts[3]

	if s.profileService != nil {
		if err := s.profileService.DeleteProfile(r.Context(), userID); err != nil {
			slog.Error("Failed to delete profile", "error", err, "userID", userID)
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}
	}

	// Redirect to users list
	http.Redirect(w, r, "/admin/users", http.StatusFound)
}
