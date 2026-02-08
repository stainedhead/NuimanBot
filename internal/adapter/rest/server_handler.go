package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// ServerService defines the interface for server status and control
type ServerService interface {
	GetStatus(ctx context.Context) (*ServerStatus, error)
	GetMetrics(ctx context.Context) (*ServerMetrics, error)
	GetLogs(ctx context.Context, level string, limit int) ([]LogEntry, error)
}

// ServerStatus represents the server status
type ServerStatus struct {
	Uptime            time.Duration `json:"uptime"`
	Version           string        `json:"version"`
	MemoryUsageMB     float64       `json:"memoryUsageMB"`
	GoVersion         string        `json:"goVersion"`
	ActiveConnections struct {
		Slack    int `json:"slack"`
		Telegram int `json:"telegram"`
		CLI      int `json:"cli"`
	} `json:"activeConnections"`
}

// ServerMetrics represents server metrics
type ServerMetrics struct {
	RequestsLast24h int     `json:"requestsLast24h"`
	ErrorRate       float64 `json:"errorRate"`
	AvgResponseTime float64 `json:"avgResponseTime"`
	ActiveUsers     int     `json:"activeUsers"`
	ActiveBots      int     `json:"activeBots"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	UserID    string    `json:"userID,omitempty"`
}

// ServerHandler handles HTTP requests for server operations
type ServerHandler struct {
	service ServerService
}

// NewServerHandler creates a new server handler
func NewServerHandler(service ServerService) *ServerHandler {
	return &ServerHandler{
		service: service,
	}
}

// RegisterRoutes registers all server routes
func (h *ServerHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/admin/status", h.GetStatus).Methods("GET")
	router.HandleFunc("/api/v1/admin/metrics", h.GetMetrics).Methods("GET")
	router.HandleFunc("/api/v1/admin/logs", h.GetLogs).Methods("GET")
}

// GetStatus handles GET /api/v1/admin/status
func (h *ServerHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.GetStatus(r.Context())
	if err != nil {
		http.Error(w, "Failed to get server status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetMetrics handles GET /api/v1/admin/metrics
func (h *ServerHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.service.GetMetrics(r.Context())
	if err != nil {
		http.Error(w, "Failed to get server metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// GetLogs handles GET /api/v1/admin/logs
func (h *ServerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	level := r.URL.Query().Get("level")
	if level == "" {
		level = "all"
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 500 {
			limit = val
		}
	}

	logs, err := h.service.GetLogs(r.Context(), level, limit)
	if err != nil {
		http.Error(w, "Failed to get logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"level": level,
		"limit": limit,
		"total": len(logs),
	})
}
