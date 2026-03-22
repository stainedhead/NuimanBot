package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"nuimanbot/internal/usecase/security"
)

// Validate returns a middleware that sanitizes string fields in JSON request bodies.
// It only activates for requests with Content-Type: application/json.
// Injection patterns detected by security.DefaultInputValidator cause a 400 response.
// The validated body is re-injected into the request for downstream handlers.
func Validate() func(http.Handler) http.Handler {
	validator := security.NewDefaultInputValidator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			raw, err := io.ReadAll(r.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, "bad_request", "failed to read request body")
				return
			}
			_ = r.Body.Close()

			// Parse into a generic map so we can inspect all string fields.
			var payload map[string]interface{}
			if err = json.Unmarshal(raw, &payload); err != nil {
				writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON")
				return
			}

			if err = validateStringFields(payload, validator); err != nil {
				writeError(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}

			// Re-encode and re-inject the validated body so downstream handlers can read it.
			sanitized, err := json.Marshal(payload)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to re-encode body")
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(sanitized))
			r.ContentLength = int64(len(sanitized))

			next.ServeHTTP(w, r)
		})
	}
}

// validateStringFields recursively validates all string values in the map using the validator.
func validateStringFields(m map[string]interface{}, v *security.DefaultInputValidator) error {
	for _, val := range m {
		switch typed := val.(type) {
		case string:
			if _, err := v.ValidateInput(context.Background(), typed, 1<<20); err != nil {
				return err
			}
		case map[string]interface{}:
			if err := validateStringFields(typed, v); err != nil {
				return err
			}
		}
	}
	return nil
}
