package web

import (
	"log/slog"
	"net/http"
	"strconv"

	"nuimanbot/internal/domain"
)

// SettingsService is the interface the web admin's Settings environment
// (FR-001–FR-004) depends on for the pieces not already exposed directly on
// *Server (network access is read/written via Server.NetworkAccessConfig/
// SetNetworkAccessConfig — see settings_handler.go's handlers below).
//
// Scope note: this is deliberately thin. Skills/Plugins/Gateways/Users are
// FR-001's "existing systems, surfaced as-is" (spec.md Non-Goals: "Rebuilding
// the skills/plugins config system" is explicitly out of scope) — Users
// management already exists at /admin/users and is linked from the
// Settings page rather than duplicated. Per-user retention override storage
// (beyond displaying the system-wide default) is deferred — see
// implementation-notes.md.
type SettingsService interface {
	// WorkerPoolSize returns the current live concurrency (FR-004).
	WorkerPoolSize() int
	// SetWorkerPoolSize updates the live worker pool concurrency. Per
	// spec.md Edge Case #4, reducing it never pre-empts in-flight runs —
	// that guarantee lives in the WorkerPool implementation this service
	// wraps, not here.
	SetWorkerPoolSize(n int) error
	// SkillNames lists the currently registered skill names (FR-001's
	// read-only Skills surface).
	SkillNames() []string
	// RetentionDefaults returns the system-wide default retention windows
	// in days (0 = Never) for Chat/Project/History (FR-003).
	RetentionDefaults() (chatDays, projectDays, historyDays int)
}

// SetSettingsService sets the Settings environment's service.
func (s *Server) SetSettingsService(svc SettingsService) {
	s.settingsService = svc
}

// SettingsPageData is the template data for /admin/settings.
type SettingsPageData struct {
	*BaseData
	IsAdmin              bool
	WorkerPoolSize       int
	NetworkMode          string
	NetworkBindAddress   string
	AllowlistCount       int
	HasAllowlist         bool
	SkillNames           []string
	ChatRetentionDays    int
	ProjectRetentionDays int
	HistoryRetentionDays int
	CSRFToken            string
}

// handleSettings renders Settings (GET) and applies system-wide changes
// (POST, admin-only — FR-001/FR-002/FR-004 are all explicitly "system-
// wide/admin-only" per spec.md; a non-admin POST is rejected with 403, the
// same fail-closed posture used throughout this package).
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	if s.settingsService == nil {
		http.Error(w, "Settings service not configured", http.StatusInternalServerError)
		return
	}

	isAdmin := user.Role == string(domain.RoleAdmin)

	if r.Method == http.MethodPost {
		if !isAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		s.handleSettingsUpdate(w, r)
		return
	}

	s.renderSettings(w, r, user, isAdmin, "", "")
}

// handleSettingsUpdate applies an admin's system-wide changes (worker pool
// size, network access mode) and re-renders the page with a flash message.
func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser(r)
	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var flashErr string

	if raw := r.FormValue("worker_pool_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			flashErr = "Worker pool size must be a positive integer."
		} else if err := s.settingsService.SetWorkerPoolSize(n); err != nil {
			slog.Error("Failed to update worker pool size", "error", err)
			flashErr = "Failed to update worker pool size."
		}
	}

	if mode := r.FormValue("network_mode"); mode != "" {
		current := s.NetworkAccessConfig()
		newMode := domain.AccessModeLocalhostOnly
		if mode == string(domain.AccessModeRemote) {
			newMode = domain.AccessModeRemote
		}
		current.Mode = newMode
		s.SetNetworkAccessConfig(current)
	}

	flashSuccess := ""
	if flashErr == "" {
		flashSuccess = "Settings updated."
	}
	s.renderSettings(w, r, user, true, flashSuccess, flashErr)
}

// renderSettings gathers current settings state and renders settings.html.
func (s *Server) renderSettings(w http.ResponseWriter, r *http.Request, user *User, isAdmin bool, flashSuccess, flashErr string) {
	net := s.NetworkAccessConfig()
	chatDays, projectDays, historyDays := s.settingsService.RetentionDefaults()

	base := s.baseDataFor(user, "Settings", "settings")
	if flashSuccess != "" {
		base.WithFlashSuccess(flashSuccess)
	}
	if flashErr != "" {
		base.WithFlashError(flashErr)
	}

	data := &SettingsPageData{
		BaseData:             base,
		IsAdmin:              isAdmin,
		WorkerPoolSize:       s.settingsService.WorkerPoolSize(),
		NetworkMode:          string(net.Mode),
		NetworkBindAddress:   net.BindAddress,
		AllowlistCount:       len(net.Allowlist),
		HasAllowlist:         net.Allowlist != nil,
		SkillNames:           s.settingsService.SkillNames(),
		ChatRetentionDays:    chatDays,
		ProjectRetentionDays: projectDays,
		HistoryRetentionDays: historyDays,
		CSRFToken:            s.auth.GenerateCSRFToken(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		slog.Error("Failed to render settings template", "error", err)
		s.Error500(w, r, err)
	}
}
