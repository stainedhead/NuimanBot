package memoryv2

import (
	"context"
	"log/slog"
	"time"

	"nuimanbot/internal/infrastructure/alerting"
)

// Performance thresholds for logging and alerting.
const (
	// FTSSlowQueryThreshold triggers a warning log for slow FTS queries.
	FTSSlowQueryThreshold = 50 * time.Millisecond

	// RecallSlowThreshold triggers an alert when recall exceeds this duration.
	RecallSlowThreshold = 100 * time.Millisecond

	// ExtractionSlowThreshold triggers an alert when extraction exceeds this duration.
	ExtractionSlowThreshold = 5 * time.Second

	// ConsolidationSlowThreshold triggers an alert for slow scene consolidation.
	ConsolidationSlowThreshold = 5 * time.Second
)

// isSlowFTSQuery returns true if an FTS query exceeded the slow threshold.
func isSlowFTSQuery(duration time.Duration) bool {
	return duration > FTSSlowQueryThreshold
}

// isSlowRecall returns true if a recall operation exceeded the slow threshold.
func isSlowRecall(duration time.Duration) bool {
	return duration > RecallSlowThreshold
}

// isSlowExtraction returns true if an extraction operation exceeded the slow threshold.
func isSlowExtraction(duration time.Duration) bool {
	return duration > ExtractionSlowThreshold
}

// isSlowConsolidation returns true if a consolidation operation exceeded the slow threshold.
func isSlowConsolidation(duration time.Duration) bool {
	return duration > ConsolidationSlowThreshold
}

// alertExtractionFailed sends a structured log and alert for extraction failure.
func alertExtractionFailed(ctx context.Context, conversationID string, err error) {
	slog.ErrorContext(ctx, "Memory extraction failed",
		"component", "memory_curator",
		"conversation_id", conversationID,
		"error", err,
	)
	alerting.SendAlert(ctx, alerting.Alert{
		Severity: alerting.SeverityError,
		Title:    "Memory extraction failed",
		Message:  err.Error(),
		Tags:     map[string]string{"component": "memory_curator"},
		Details:  map[string]any{"conversation_id": conversationID},
	})
}

// alertConsolidationFailed sends a structured log and alert for consolidation failure.
func alertConsolidationFailed(ctx context.Context, sceneName string, err error) {
	slog.ErrorContext(ctx, "Scene consolidation failed",
		"component", "memory_curator",
		"scene", sceneName,
		"error", err,
	)
	alerting.SendAlert(ctx, alerting.Alert{
		Severity: alerting.SeverityWarning,
		Title:    "Scene consolidation failed",
		Message:  err.Error(),
		Tags:     map[string]string{"component": "memory_curator"},
		Details:  map[string]any{"scene": sceneName},
	})
}

// alertRecallFailed sends a structured log and alert for recall failure.
func alertRecallFailed(ctx context.Context, conversationID, query string, err error) {
	slog.ErrorContext(ctx, "Memory recall failed",
		"component", "memory_recall",
		"conversation_id", conversationID,
		"query", query,
		"error", err,
	)
	alerting.SendAlert(ctx, alerting.Alert{
		Severity: alerting.SeverityError,
		Title:    "Memory recall failed",
		Message:  err.Error(),
		Tags:     map[string]string{"component": "memory_recall"},
		Details: map[string]any{
			"conversation_id": conversationID,
			"query":           query,
		},
	})
}

// alertSlowRecall sends a structured log and alert for slow recall operations.
func alertSlowRecall(ctx context.Context, conversationID string, duration time.Duration) {
	slog.WarnContext(ctx, "Slow memory recall",
		"component", "memory_recall",
		"conversation_id", conversationID,
		"duration_ms", duration.Milliseconds(),
		"threshold_ms", RecallSlowThreshold.Milliseconds(),
	)
	alerting.SendAlert(ctx, alerting.Alert{
		Severity: alerting.SeverityWarning,
		Title:    "Slow memory recall",
		Message:  "Memory recall exceeded latency threshold",
		Tags:     map[string]string{"component": "memory_recall"},
		Details: map[string]any{
			"conversation_id": conversationID,
			"duration_ms":     duration.Milliseconds(),
			"threshold_ms":    RecallSlowThreshold.Milliseconds(),
		},
	})
}

// logSlowFTSQuery logs a warning for a slow FTS query.
func logSlowFTSQuery(ctx context.Context, query string, duration time.Duration) {
	slog.WarnContext(ctx, "Slow FTS query",
		"component", "memory_recall",
		"query", query,
		"duration_ms", duration.Milliseconds(),
		"threshold_ms", FTSSlowQueryThreshold.Milliseconds(),
	)
}

// logExtractionComplete logs structured extraction results.
func logExtractionComplete(conversationID string, cellsCreated, scenesUpdated, errorCount int, duration time.Duration) {
	slog.Info("Memory extraction complete",
		"component", "memory_curator",
		"conversation_id", conversationID,
		"cells_created", cellsCreated,
		"scenes_updated", scenesUpdated,
		"errors", errorCount,
		"duration_ms", duration.Milliseconds(),
	)
}

// logRecallComplete logs structured recall results.
func logRecallComplete(conversationID string, cellCount, sceneCount, totalTokens int, ftsMatchCount int, fallbackUsed bool, duration time.Duration) {
	slog.Info("Memory recall complete",
		"component", "memory_recall",
		"conversation_id", conversationID,
		"cell_count", cellCount,
		"scene_count", sceneCount,
		"total_tokens", totalTokens,
		"fts_match_count", ftsMatchCount,
		"fallback_used", fallbackUsed,
		"duration_ms", duration.Milliseconds(),
	)
}
