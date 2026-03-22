package middleware

import (
	"encoding/json"
	"net/http"
)

// apiErrorResponse is the standard JSON error body used by all middleware in this package.
type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// writeError writes a structured JSON error response with the given status code.
// It is the single error-writing function used by all middleware in this package.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiErrorResponse{Error: errCode, Message: message})
}
