package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// ConfigService defines the interface for configuration management
type ConfigService interface {
	GetConfig(ctx context.Context) (map[string]interface{}, error)
	UpdateConfig(ctx context.Context, updates map[string]interface{}) error
	ReloadConfig(ctx context.Context) error
	ValidateConfig(ctx context.Context, configData map[string]interface{}) error
}

// ConfigHandler handles HTTP requests for configuration operations
type ConfigHandler struct {
	service ConfigService
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(service ConfigService) *ConfigHandler {
	return &ConfigHandler{
		service: service,
	}
}

// RegisterRoutes registers all config routes
func (h *ConfigHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/admin/config", h.GetConfig).Methods("GET")
	router.HandleFunc("/api/v1/admin/config", h.UpdateConfig).Methods("PUT")
	router.HandleFunc("/api/v1/admin/config/reload", h.ReloadConfig).Methods("POST")
	router.HandleFunc("/api/v1/admin/config/validate", h.ValidateConfig).Methods("POST")
}

// GetConfig handles GET /api/v1/admin/config
func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.service.GetConfig(r.Context())
	if err != nil {
		http.Error(w, "Failed to get configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateConfig handles PUT /api/v1/admin/config
func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateConfig(r.Context(), updates); err != nil {
		http.Error(w, "Failed to update configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Configuration updated successfully",
	})
}

// ReloadConfig handles POST /api/v1/admin/config/reload
func (h *ConfigHandler) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ReloadConfig(r.Context()); err != nil {
		http.Error(w, "Failed to reload configuration: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Configuration reloaded successfully",
	})
}

// ValidateConfig handles POST /api/v1/admin/config/validate
func (h *ConfigHandler) ValidateConfig(w http.ResponseWriter, r *http.Request) {
	var configData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&configData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.ValidateConfig(r.Context(), configData); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":  false,
			"errors": []string{err.Error()},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"message": "Configuration is valid",
	})
}
