package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"nuimanbot/internal/domain"
)

// BotService defines the interface for bot management operations
type BotService interface {
	// Slack bot operations
	CreateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error
	GetSlackBot(ctx context.Context, botID string) (*domain.SlackBotConfig, error)
	ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error)
	ListSlackBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.SlackBotConfig, error)
	UpdateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error
	DeleteSlackBot(ctx context.Context, botID string) error
	EnableSlackBot(ctx context.Context, botID string) error
	DisableSlackBot(ctx context.Context, botID string) error

	// Telegram bot operations
	CreateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error
	GetTelegramBot(ctx context.Context, botID string) (*domain.TelegramBotConfig, error)
	ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error)
	ListTelegramBotsByOwner(ctx context.Context, ownerUserID string) ([]*domain.TelegramBotConfig, error)
	UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error
	DeleteTelegramBot(ctx context.Context, botID string) error
	EnableTelegramBot(ctx context.Context, botID string) error
	DisableTelegramBot(ctx context.Context, botID string) error
}

// BotHandler handles HTTP requests for bot operations
type BotHandler struct {
	service BotService
}

// NewBotHandler creates a new bot handler
func NewBotHandler(service BotService) *BotHandler {
	return &BotHandler{
		service: service,
	}
}

// RegisterRoutes registers all bot routes
func (h *BotHandler) RegisterRoutes(router *mux.Router) {
	// Admin Slack bot routes
	router.HandleFunc("/api/v1/admin/bots/slack", h.ListSlackBots).Methods("GET")
	router.HandleFunc("/api/v1/admin/bots/slack/{id}", h.GetSlackBot).Methods("GET")
	router.HandleFunc("/api/v1/admin/bots/slack", h.CreateSlackBot).Methods("POST")
	router.HandleFunc("/api/v1/admin/bots/slack/{id}", h.UpdateSlackBot).Methods("PUT")
	router.HandleFunc("/api/v1/admin/bots/slack/{id}", h.DeleteSlackBot).Methods("DELETE")
	router.HandleFunc("/api/v1/admin/bots/slack/{id}/enable", h.EnableSlackBot).Methods("POST")
	router.HandleFunc("/api/v1/admin/bots/slack/{id}/disable", h.DisableSlackBot).Methods("POST")

	// Admin Telegram bot routes
	router.HandleFunc("/api/v1/admin/bots/telegram", h.ListTelegramBots).Methods("GET")
	router.HandleFunc("/api/v1/admin/bots/telegram/{id}", h.GetTelegramBot).Methods("GET")
	router.HandleFunc("/api/v1/admin/bots/telegram", h.CreateTelegramBot).Methods("POST")
	router.HandleFunc("/api/v1/admin/bots/telegram/{id}", h.UpdateTelegramBot).Methods("PUT")
	router.HandleFunc("/api/v1/admin/bots/telegram/{id}", h.DeleteTelegramBot).Methods("DELETE")
	router.HandleFunc("/api/v1/admin/bots/telegram/{id}/enable", h.EnableTelegramBot).Methods("POST")
	router.HandleFunc("/api/v1/admin/bots/telegram/{id}/disable", h.DisableTelegramBot).Methods("POST")

	// User self-service bot routes (non-admin)
	router.HandleFunc("/api/v1/bots/slack", h.ListOwnSlackBots).Methods("GET")
	router.HandleFunc("/api/v1/bots/slack/{id}", h.GetOwnSlackBot).Methods("GET")
	router.HandleFunc("/api/v1/bots/telegram", h.ListOwnTelegramBots).Methods("GET")
	router.HandleFunc("/api/v1/bots/telegram/{id}", h.GetOwnTelegramBot).Methods("GET")
}

// Slack Bot Handlers

// ListSlackBots handles GET /api/v1/admin/bots/slack
func (h *BotHandler) ListSlackBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.service.ListSlackBots(r.Context())
	if err != nil {
		http.Error(w, "Failed to list Slack bots", http.StatusInternalServerError)
		return
	}

	// Mask tokens before returning
	for _, bot := range bots {
		bot.SlackBotToken = maskToken(bot.SlackBotToken)
		bot.SlackAppToken = maskToken(bot.SlackAppToken)
		bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bots":  bots,
		"total": len(bots),
	})
}

// GetSlackBot handles GET /api/v1/admin/bots/slack/{id}
func (h *BotHandler) GetSlackBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	bot, err := h.service.GetSlackBot(r.Context(), botID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Mask tokens
	bot.SlackBotToken = maskToken(bot.SlackBotToken)
	bot.SlackAppToken = maskToken(bot.SlackAppToken)
	bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}

// CreateSlackBot handles POST /api/v1/admin/bots/slack
func (h *BotHandler) CreateSlackBot(w http.ResponseWriter, r *http.Request) {
	var bot domain.SlackBotConfig
	if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateSlackBot(r.Context(), &bot); err != nil {
		http.Error(w, "Failed to create bot: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Mask tokens
	bot.SlackBotToken = maskToken(bot.SlackBotToken)
	bot.SlackAppToken = maskToken(bot.SlackAppToken)
	bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bot)
}

// UpdateSlackBot handles PUT /api/v1/admin/bots/slack/{id}
func (h *BotHandler) UpdateSlackBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	var bot domain.SlackBotConfig
	if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	bot.BotID = botID

	if err := h.service.UpdateSlackBot(r.Context(), &bot); err != nil {
		http.Error(w, "Failed to update bot: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Mask tokens
	bot.SlackBotToken = maskToken(bot.SlackBotToken)
	bot.SlackAppToken = maskToken(bot.SlackAppToken)
	bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}

// DeleteSlackBot handles DELETE /api/v1/admin/bots/slack/{id}
func (h *BotHandler) DeleteSlackBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.DeleteSlackBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to delete bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnableSlackBot handles POST /api/v1/admin/bots/slack/{id}/enable
func (h *BotHandler) EnableSlackBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.EnableSlackBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to enable bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

// DisableSlackBot handles POST /api/v1/admin/bots/slack/{id}/disable
func (h *BotHandler) DisableSlackBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.DisableSlackBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to disable bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

// Telegram Bot Handlers

// ListTelegramBots handles GET /api/v1/admin/bots/telegram
func (h *BotHandler) ListTelegramBots(w http.ResponseWriter, r *http.Request) {
	bots, err := h.service.ListTelegramBots(r.Context())
	if err != nil {
		http.Error(w, "Failed to list Telegram bots", http.StatusInternalServerError)
		return
	}

	// Mask tokens
	for _, bot := range bots {
		bot.TelegramBotToken = maskToken(bot.TelegramBotToken)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bots":  bots,
		"total": len(bots),
	})
}

// GetTelegramBot handles GET /api/v1/admin/bots/telegram/{id}
func (h *BotHandler) GetTelegramBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	bot, err := h.service.GetTelegramBot(r.Context(), botID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Mask token
	bot.TelegramBotToken = maskToken(bot.TelegramBotToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}

// CreateTelegramBot handles POST /api/v1/admin/bots/telegram
func (h *BotHandler) CreateTelegramBot(w http.ResponseWriter, r *http.Request) {
	var bot domain.TelegramBotConfig
	if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.CreateTelegramBot(r.Context(), &bot); err != nil {
		http.Error(w, "Failed to create bot: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Mask token
	bot.TelegramBotToken = maskToken(bot.TelegramBotToken)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bot)
}

// UpdateTelegramBot handles PUT /api/v1/admin/bots/telegram/{id}
func (h *BotHandler) UpdateTelegramBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	var bot domain.TelegramBotConfig
	if err := json.NewDecoder(r.Body).Decode(&bot); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	bot.BotID = botID

	if err := h.service.UpdateTelegramBot(r.Context(), &bot); err != nil {
		http.Error(w, "Failed to update bot: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Mask token
	bot.TelegramBotToken = maskToken(bot.TelegramBotToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}

// DeleteTelegramBot handles DELETE /api/v1/admin/bots/telegram/{id}
func (h *BotHandler) DeleteTelegramBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.DeleteTelegramBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to delete bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnableTelegramBot handles POST /api/v1/admin/bots/telegram/{id}/enable
func (h *BotHandler) EnableTelegramBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.EnableTelegramBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to enable bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
}

// DisableTelegramBot handles POST /api/v1/admin/bots/telegram/{id}/disable
func (h *BotHandler) DisableTelegramBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.service.DisableTelegramBot(r.Context(), botID); err != nil {
		http.Error(w, "Failed to disable bot", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
}

// maskToken masks sensitive tokens, showing only first/last 4 characters
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// User Self-Service Bot Handlers

// ListOwnSlackBots handles GET /api/v1/bots/slack (non-admin)
func (h *BotHandler) ListOwnSlackBots(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	bots, err := h.service.ListSlackBotsByOwner(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to list Slack bots", http.StatusInternalServerError)
		return
	}

	// Mask tokens before returning
	for _, bot := range bots {
		bot.SlackBotToken = maskToken(bot.SlackBotToken)
		bot.SlackAppToken = maskToken(bot.SlackAppToken)
		bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bots":  bots,
		"total": len(bots),
	})
}

// GetOwnSlackBot handles GET /api/v1/bots/slack/{id} (non-admin)
func (h *BotHandler) GetOwnSlackBot(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	botID := vars["id"]

	bot, err := h.service.GetSlackBot(r.Context(), botID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if bot.OwnerUserID != userID {
		http.Error(w, "Access denied: you do not own this bot", http.StatusForbidden)
		return
	}

	// Mask tokens before returning
	bot.SlackBotToken = maskToken(bot.SlackBotToken)
	bot.SlackAppToken = maskToken(bot.SlackAppToken)
	bot.SlackSigningSecret = maskToken(bot.SlackSigningSecret)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}

// ListOwnTelegramBots handles GET /api/v1/bots/telegram (non-admin)
func (h *BotHandler) ListOwnTelegramBots(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	bots, err := h.service.ListTelegramBotsByOwner(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to list Telegram bots", http.StatusInternalServerError)
		return
	}

	// Mask tokens before returning
	for _, bot := range bots {
		bot.TelegramBotToken = maskToken(bot.TelegramBotToken)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bots":  bots,
		"total": len(bots),
	})
}

// GetOwnTelegramBot handles GET /api/v1/bots/telegram/{id} (non-admin)
func (h *BotHandler) GetOwnTelegramBot(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		http.Error(w, "User ID not found in request context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	botID := vars["id"]

	bot, err := h.service.GetTelegramBot(r.Context(), botID)
	if err != nil {
		http.Error(w, "Bot not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if bot.OwnerUserID != userID {
		http.Error(w, "Access denied: you do not own this bot", http.StatusForbidden)
		return
	}

	// Mask tokens before returning
	bot.TelegramBotToken = maskToken(bot.TelegramBotToken)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bot)
}
