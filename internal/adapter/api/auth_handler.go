package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"nuimanbot/internal/config"
)

// tokenRequest is the JSON body for POST /api/v1/auth/token.
type tokenRequest struct {
	APIKey string `json:"api_key"`
}

// tokenResponse is the JSON body returned on successful token issuance.
type tokenResponse struct {
	Token string `json:"token"`
}

// errorResponse is a structured JSON error body.
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// AuthHandler handles the POST /api/v1/auth/token endpoint.
// It exchanges a valid API key for a signed JWT.
type AuthHandler struct {
	cfg       config.ExternalAPIRestConfig
	jwtSecret string
}

// NewAuthHandler creates an AuthHandler using cfg for API key validation and
// jwtSecret for signing issued JWTs.
func NewAuthHandler(cfg config.ExternalAPIRestConfig, jwtSecret string) *AuthHandler {
	return &AuthHandler{cfg: cfg, jwtSecret: jwtSecret}
}

// ServeHTTP implements http.Handler.
func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method_not_allowed"})
		return
	}

	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "request body required"})
		return
	}

	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid JSON"})
		return
	}

	if req.APIKey == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "api_key is required"})
		return
	}

	if !h.validAPIKey(req.APIKey) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized", Message: "invalid API key"})
		return
	}

	claims := newClaims(req.APIKey)
	signed, err := signToken(claims, h.jwtSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal_error"})
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{Token: signed})
}

// validAPIKey performs a constant-time comparison against the configured API key.
func (h *AuthHandler) validAPIKey(provided string) bool {
	expected := h.cfg.APIKey.Value()
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// writeJSON writes the status code and JSON-encoded body to w.
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
