package middleware

import (
	"bytes"
	"io"
	"net/http"
)

// BodyLimit returns a middleware that limits the request body to maxBytes.
// Requests with a body exceeding maxBytes return 413 Request Entity Too Large.
// The error response never includes body contents.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Read up to maxBytes+1 bytes so we can detect when the body exceeds the limit.
			// Using LimitReader avoids buffering an unbounded body before checking.
			limited := io.LimitReader(r.Body, maxBytes+1)
			raw, err := io.ReadAll(limited)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to read body")
				return
			}

			if int64(len(raw)) > maxBytes {
				// Return 413 without leaking any body contents.
				writeError(w, http.StatusRequestEntityTooLarge, "request_entity_too_large", "request body exceeds size limit")
				return
			}

			// Re-inject the (fully read) body so downstream handlers can read it normally.
			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.ContentLength = int64(len(raw))

			next.ServeHTTP(w, r)
		})
	}
}
