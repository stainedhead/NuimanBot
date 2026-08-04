package tool

import (
	"strings"
	"testing"
)

// TestDescribePendingAction_TruncatesLongParameterValues is the regression
// test for FR-015 (P2): describePendingAction caps the *number* of
// parameters shown (5) but, prior to this fix, did not cap the *length* of
// any individual parameter value. A large parameter (e.g. a full file body
// passed to a tool) could produce an oversized or sensitive confirmation
// message pushed to chat platforms/logs.
//
// This test proves both halves of the fix:
//  1. A parameter value longer than the truncation threshold is truncated
//     in the rendered summary, with a clear indicator that truncation
//     occurred.
//  2. A parameter value at/under the threshold is left untouched.
func TestDescribePendingAction_TruncatesLongParameterValues(t *testing.T) {
	longValue := strings.Repeat("A", 2000)
	shortValue := "short-value"

	summary := describePendingAction("write_file", "", map[string]any{
		"body": longValue,
		"path": shortValue,
	})

	if strings.Contains(summary, longValue) {
		t.Fatalf("expected long parameter value to be truncated, but the full %d-char value was present in summary: %q", len(longValue), summary)
	}
	if !strings.Contains(summary, "...[truncated, 2000 chars total]") {
		t.Fatalf("expected summary to contain a truncation indicator with the total length, got: %q", summary)
	}
	if !strings.Contains(summary, shortValue) {
		t.Fatalf("expected short parameter value to be present unmodified in summary, got: %q", summary)
	}
}
