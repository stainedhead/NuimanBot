package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureHandler is a slog.Handler that captures log records for inspection in tests.
type captureHandler struct {
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

// hasWarnContaining reports true if any captured Warn-level record's message or
// attributes contain all of the given substrings.
func (h *captureHandler) hasWarnContaining(substrings ...string) bool {
	for _, r := range h.records {
		if r.Level != slog.LevelWarn {
			continue
		}
		// Build a single searchable string from message and all attrs.
		var sb strings.Builder
		sb.WriteString(r.Message)
		r.Attrs(func(a slog.Attr) bool {
			sb.WriteString(" ")
			sb.WriteString(a.Key)
			sb.WriteString("=")
			sb.WriteString(a.Value.String())
			return true
		})
		combined := sb.String()

		allFound := true
		for _, sub := range substrings {
			if !strings.Contains(combined, sub) {
				allFound = false
				break
			}
		}
		if allFound {
			return true
		}
	}
	return false
}

// withCaptureLogger swaps the default slog logger for a capture handler,
// runs fn, then restores the original logger.  It returns the handler so
// the caller can inspect captured records.
func withCaptureLogger(fn func()) *captureHandler {
	h := &captureHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	fn()
	slog.SetDefault(orig)
	return h
}

// ---- metaFloat64 tests ----

func TestMetaFloat64_Float64Value(t *testing.T) {
	meta := map[string]interface{}{"salience": 0.85}
	h := withCaptureLogger(func() {
		got := metaFloat64(meta, "salience")
		if got != 0.85 {
			t.Errorf("expected 0.85, got %v", got)
		}
	})
	if h.hasWarnContaining("salience") {
		t.Error("expected no warning for float64 value, but one was logged")
	}
}

func TestMetaFloat64_StringValue(t *testing.T) {
	meta := map[string]interface{}{"salience": "0.85"}
	h := withCaptureLogger(func() {
		got := metaFloat64(meta, "salience")
		if got != 0.85 {
			t.Errorf("expected 0.85, got %v", got)
		}
	})
	if h.hasWarnContaining("salience") {
		t.Error("expected no warning for string-encoded float, but one was logged")
	}
}

func TestMetaFloat64_JsonNumber(t *testing.T) {
	meta := map[string]interface{}{"salience": json.Number("0.85")}
	h := withCaptureLogger(func() {
		got := metaFloat64(meta, "salience")
		if got != 0.85 {
			t.Errorf("expected 0.85, got %v", got)
		}
	})
	if h.hasWarnContaining("salience") {
		t.Error("expected no warning for json.Number value, but one was logged")
	}
}

func TestMetaFloat64_NilValue_NoWarning(t *testing.T) {
	meta := map[string]interface{}{"salience": nil}
	h := withCaptureLogger(func() {
		got := metaFloat64(meta, "salience")
		if got != 0 {
			t.Errorf("expected 0 for nil value, got %v", got)
		}
	})
	if h.hasWarnContaining("salience") {
		t.Error("expected no warning for nil value, but one was logged")
	}
}

func TestMetaFloat64_AbsentKey_NoWarning(t *testing.T) {
	meta := map[string]interface{}{}
	h := withCaptureLogger(func() {
		got := metaFloat64(meta, "salience")
		if got != 0 {
			t.Errorf("expected 0 for absent key, got %v", got)
		}
	})
	if h.hasWarnContaining("salience") {
		t.Error("expected no warning for absent key, but one was logged")
	}
}

func TestMetaFloat64_UnexpectedType_WarnsWithKeyInMessage(t *testing.T) {
	meta := map[string]interface{}{"salience": true}
	var got float64
	h := withCaptureLogger(func() {
		got = metaFloat64(meta, "salience")
	})
	if got != 0 {
		t.Errorf("expected 0 for unexpected type, got %v", got)
	}
	if !h.hasWarnContaining("unexpected", "salience") {
		t.Error("expected a Warn log containing 'unexpected' and 'salience', none found")
	}
}

// ---- metaInt tests ----

func TestMetaInt_IntValue(t *testing.T) {
	meta := map[string]interface{}{"token_count": 42}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for int value, but one was logged")
	}
}

func TestMetaInt_Float64Value(t *testing.T) {
	meta := map[string]interface{}{"token_count": float64(42)}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for float64 value, but one was logged")
	}
}

func TestMetaInt_StringValue(t *testing.T) {
	meta := map[string]interface{}{"token_count": "42"}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for string-encoded int, but one was logged")
	}
}

func TestMetaInt_JsonNumber(t *testing.T) {
	meta := map[string]interface{}{"token_count": json.Number("42")}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for json.Number int, but one was logged")
	}
}

func TestMetaInt_NilValue_NoWarning(t *testing.T) {
	meta := map[string]interface{}{"token_count": nil}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 0 {
			t.Errorf("expected 0 for nil value, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for nil value, but one was logged")
	}
}

func TestMetaInt_AbsentKey_NoWarning(t *testing.T) {
	meta := map[string]interface{}{}
	h := withCaptureLogger(func() {
		got := metaInt(meta, "token_count")
		if got != 0 {
			t.Errorf("expected 0 for absent key, got %v", got)
		}
	})
	if h.hasWarnContaining("token_count") {
		t.Error("expected no warning for absent key, but one was logged")
	}
}

func TestMetaInt_UnexpectedType_WarnsWithKeyInMessage(t *testing.T) {
	meta := map[string]interface{}{"token_count": true}
	var got int
	h := withCaptureLogger(func() {
		got = metaInt(meta, "token_count")
	})
	if got != 0 {
		t.Errorf("expected 0 for unexpected type, got %v", got)
	}
	if !h.hasWarnContaining("unexpected", "token_count") {
		t.Error("expected a Warn log containing 'unexpected' and 'token_count', none found")
	}
}
