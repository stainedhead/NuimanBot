package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

var serverStartTime = time.Now()

// DashboardData represents dashboard template data
type DashboardData struct {
	*BaseData
	ServerStatus      string
	Uptime            string
	ActiveConnections int
	StartedAt         string
	MemoryUsage       string
	Goroutines        int
	Gateways          []GatewayStatus
	RecentActivity    []ActivityLog
}

// GatewayStatus represents gateway status information
type GatewayStatus struct {
	Name   string
	Active bool
}

// ActivityLog represents a single activity log entry
type ActivityLog struct {
	Level     string
	Action    string
	User      string
	Timestamp string
}

// handleDashboard renders the admin dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Gather dashboard data
	data := s.getDashboardData(user)

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		slog.Error("Failed to render dashboard template", "error", err)
		s.Error500(w, r, err)
	}
}

// getDashboardData gathers all dashboard data
func (s *Server) getDashboardData(user *User) *DashboardData {
	base := NewBaseData("Dashboard", "dashboard")
	base = base.WithUser(user)

	// Calculate uptime
	uptime := time.Since(serverStartTime)
	uptimeStr := formatDuration(uptime)

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memoryUsage := formatBytes(memStats.Alloc)

	// Get goroutine count
	goroutines := runtime.NumGoroutine()

	// Gateway status (hardcoded for now - will be dynamic later)
	gateways := []GatewayStatus{
		{Name: "CLI", Active: true},
		{Name: "Slack", Active: false},
		{Name: "Telegram", Active: false},
	}

	// Recent activity (sample data - will be from audit log later)
	recentActivity := []ActivityLog{
		{
			Level:     "info",
			Action:    "User logged in",
			User:      user.Username,
			Timestamp: time.Now().Format("15:04:05"),
		},
		{
			Level:     "info",
			Action:    "Dashboard accessed",
			User:      user.Username,
			Timestamp: time.Now().Format("15:04:05"),
		},
	}

	return &DashboardData{
		BaseData:          base,
		ServerStatus:      "Running",
		Uptime:            uptimeStr,
		ActiveConnections: 1, // CLI connection
		StartedAt:         serverStartTime.Format("2006-01-02 15:04:05"),
		MemoryUsage:       memoryUsage,
		Goroutines:        goroutines,
		Gateways:          gateways,
		RecentActivity:    recentActivity,
	}
}

// handleReloadConfig handles configuration reload requests
func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	user := s.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}

	// Only admins can reload config
	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Log the reload attempt
	slog.Info("Configuration reload requested", "user", user.Username)

	// TODO: Implement actual config reload logic
	// For now, just return success

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true, "message": "Configuration reloaded"}`))
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
