package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/history"
)

// historyListDisplayLimit caps how many runs /history list prints in one
// response (Performance NFR: list commands must not dump unbounded output
// to the terminal). Runs are already returned most-recent-first by the
// underlying service, so truncation simply keeps the most recent ones.
const historyListDisplayLimit = 20

// historyContentPreviewLimit caps how much of a run's log/results content
// /history show inlines, so a large captured log doesn't flood the
// terminal.
const historyContentPreviewLimit = 4000

// HistoryCommandHandler handles the History environment's CLI commands
// (FR-034-035; FR-036's chat sub-command is deferred, see spec.md and
// architecture.md's "Scope correction").
type HistoryCommandHandler struct {
	service *history.Service
}

// NewHistoryCommandHandler creates a History command handler backed by
// service.
func NewHistoryCommandHandler(service *history.Service) *HistoryCommandHandler {
	return &HistoryCommandHandler{service: service}
}

// Handle implements EnvCommandHandler, delegating to HandleHistoryCommand.
func (h *HistoryCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID, input string) (string, error) {
	return h.HandleHistoryCommand(ctx, currentUser, ownerUserID, input)
}

// HandleHistoryCommand parses and executes a /history command. ownerUserID
// (the authenticated session's Username, AD-5) is used for every call into
// the History service — never currentUser.ID or any other identifier.
// History has no admin-only sub-commands, so currentUser is unused beyond
// satisfying the shared EnvCommandHandler signature.
func (h *HistoryCommandHandler) HandleHistoryCommand(ctx context.Context, _ *domain.User, ownerUserID, input string) (string, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	switch parts[1] {
	case "list":
		return h.listRuns(ctx, ownerUserID, parts[2:])
	case "show":
		return h.showRun(ctx, ownerUserID, parts[2:])
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown history command: %s\nUse '/history help' for usage information.", parts[1]), nil
	}
}

// listRuns handles /history list [--job <id>] [--chore <id>] [--status
// <status>] [--since <duration-or-date>].
func (h *HistoryCommandHandler) listRuns(ctx context.Context, ownerUserID string, args []string) (string, error) {
	filter, errMsg := parseHistoryListArgs(args)
	if errMsg != "" {
		return errMsg, nil
	}

	runs, err := h.service.ListRuns(ctx, ownerUserID, filter)
	if err != nil {
		return "", fmt.Errorf("failed to list runs: %w", err)
	}

	if len(runs) == 0 {
		return "No runs found.", nil
	}

	shown := runs
	truncated := 0
	if len(shown) > historyListDisplayLimit {
		truncated = len(shown) - historyListDisplayLimit
		shown = shown[:historyListDisplayLimit]
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d run(s):\n\n", len(runs)))
	for i, run := range shown {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, run.ID))
		result.WriteString(fmt.Sprintf("   Source: %s/%s\n", run.SourceType, run.SourceID))
		result.WriteString(fmt.Sprintf("   Status: %s\n", run.Status))
		result.WriteString(fmt.Sprintf("   Created: %s\n", run.CreatedAt.Format(time.DateTime)))
		if run.StartedAt != nil && run.EndedAt != nil {
			result.WriteString(fmt.Sprintf("   Duration: %s\n", run.Duration()))
		}
		result.WriteString("\n")
	}
	if truncated > 0 {
		result.WriteString(fmt.Sprintf("... %d more result(s) not shown. Refine your filter (--job, --chore, --status, --since) to narrow the results.\n", truncated))
	}

	return strings.TrimRight(result.String(), "\n"), nil
}

// parseHistoryListArgs parses /history list's flags into a domain.RunFilter.
// Returns a non-empty errMsg (not an error) for a usage problem, matching
// this package's existing flag-parsing convention (see
// admin_profile_commands.go).
func parseHistoryListArgs(args []string) (filter domain.RunFilter, errMsg string) {
	var jobID, choreID string

	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return domain.RunFilter{}, fmt.Sprintf("Missing value for flag: %s", args[i])
		}
		flag, value := args[i], args[i+1]
		switch flag {
		case "--job":
			jobID = value
		case "--chore":
			choreID = value
		case "--status":
			status := domain.RunStatus(value)
			filter.Status = &status
		case "--since":
			since, err := parseHistorySince(value)
			if err != nil {
				return domain.RunFilter{}, fmt.Sprintf("Invalid --since value %q: %v", value, err)
			}
			filter.Since = &since
		default:
			return domain.RunFilter{}, fmt.Sprintf("Unknown flag: %s", flag)
		}
	}

	if jobID != "" && choreID != "" {
		return domain.RunFilter{}, "Specify at most one of --job or --chore, not both."
	}
	if jobID != "" {
		sourceType := domain.SourceTypeJob
		filter.SourceType = &sourceType
		filter.SourceID = &jobID
	}
	if choreID != "" {
		sourceType := domain.SourceTypeChore
		filter.SourceType = &sourceType
		filter.SourceID = &choreID
	}

	return filter, ""
}

// parseHistorySince parses --since's value as either a Go duration (e.g.
// "24h", interpreted as "since that long ago") or a date/timestamp
// (RFC3339 or YYYY-MM-DD).
func parseHistorySince(value string) (time.Time, error) {
	if d, err := time.ParseDuration(value); err == nil {
		return time.Now().Add(-d), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("expected a Go duration (e.g. 24h) or a date (YYYY-MM-DD / RFC3339)")
}

// showRun handles /history show <run-id>: displays the run's details, log,
// and results, and marks it viewed as a side effect (mirroring the web
// admin's per-run detail page, history_handler.go's handleHistoryDetail).
func (h *HistoryCommandHandler) showRun(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /history show <run-id>", nil
	}
	runID := args[0]

	run, err := h.service.GetRun(ctx, ownerUserID, runID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Run not found: %s", runID), nil
		}
		return "", fmt.Errorf("failed to get run: %w", err)
	}

	// Best-effort: clearing the notification badge is a side effect of
	// viewing, not a reason to fail the command if it errors.
	_ = h.service.MarkViewed(ctx, ownerUserID, runID)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Run: %s\n", run.ID))
	result.WriteString(fmt.Sprintf("Source: %s/%s\n", run.SourceType, run.SourceID))
	result.WriteString(fmt.Sprintf("Status: %s\n", run.Status))
	result.WriteString(fmt.Sprintf("Created: %s\n", run.CreatedAt.Format(time.DateTime)))
	if run.StartedAt != nil {
		result.WriteString(fmt.Sprintf("Started: %s\n", run.StartedAt.Format(time.DateTime)))
	}
	if run.EndedAt != nil {
		result.WriteString(fmt.Sprintf("Ended: %s\n", run.EndedAt.Format(time.DateTime)))
	}
	if run.StartedAt != nil && run.EndedAt != nil {
		result.WriteString(fmt.Sprintf("Duration: %s\n", run.Duration()))
	}
	if run.SkipReason != nil {
		result.WriteString(fmt.Sprintf("Skip reason: %s\n", *run.SkipReason))
	}
	if run.Error != nil {
		result.WriteString(fmt.Sprintf("Error: %s\n", *run.Error))
	}

	if logContent, err := h.service.ReadLog(ctx, ownerUserID, runID); err == nil && logContent != "" {
		result.WriteString("\nLog:\n")
		result.WriteString(truncateForDisplay(logContent, historyContentPreviewLimit))
		result.WriteString("\n")
	}
	if resultsContent, err := h.service.ReadResults(ctx, ownerUserID, runID); err == nil && resultsContent != "" {
		result.WriteString("\nResults:\n")
		result.WriteString(truncateForDisplay(resultsContent, historyContentPreviewLimit))
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n"), nil
}

// truncateForDisplay caps content to at most limit characters, appending a
// note when truncated, so /history show cannot dump an unbounded amount of
// log/results text to the terminal.
func truncateForDisplay(content string, limit int) string {
	if len(content) <= limit {
		return content
	}
	return content[:limit] + fmt.Sprintf("\n... (truncated, %d more character(s))", len(content)-limit)
}

// showHelp returns help text for History commands.
func (h *HistoryCommandHandler) showHelp() string {
	return `History Commands:

  /history list [--job <id>] [--chore <id>] [--status <status>] [--since <duration-or-date>]
    List your Job/Chore runs, most recent first (shows up to 20; refine
    filters to narrow a larger result set).
    Example: /history list --status failed --since 24h

  /history show <run-id>
    Show a run's details, timing, log, and results.

  /history help
    Show this help message.
`
}
