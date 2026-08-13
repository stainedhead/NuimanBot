package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chores"
)

// choreChatNotImplementedMessage documents FR-031's deliberate deferral
// (auto-review fix pass FR-003): "/chore chat" is a known, deferred
// command, not a typo — ChoresService has no chat/converse method to
// mirror, matching Settings' existing "not yet implemented" convention
// rather than falling through to the generic unrecognized-subcommand
// response, which was indistinguishable from a genuine typo.
const choreChatNotImplementedMessage = "'/chore chat' is not yet implemented (deferred, see spec.md FR-031). Use '/chore help' for available commands."

// ChoreCommandHandler handles the Chores environment's CLI commands
// (FR-028–FR-033; FR-031's "/chore chat" is deliberately DEFERRED — see
// HandleChoreCommand's "chat" case).
type ChoreCommandHandler struct {
	service *chores.Service
}

// NewChoreCommandHandler creates a new Chores environment command handler.
func NewChoreCommandHandler(service *chores.Service) *ChoreCommandHandler {
	return &ChoreCommandHandler{service: service}
}

// Handle satisfies EnvCommandHandler, delegating to HandleChoreCommand.
func (h *ChoreCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleChoreCommand(ctx, currentUser, ownerUserID, input)
}

// HandleChoreCommand processes a "/chore ..." command and returns the
// formatted terminal response. ownerUserID (AD-5: the authenticated
// session's Username) scopes every call into the Chores service — never
// currentUser.ID. Chores is available to any authenticated user; no
// admin-role gating is applied here.
func (h *ChoreCommandHandler) HandleChoreCommand(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	subcommand := parts[1]
	args := parts[2:]

	switch subcommand {
	case "list":
		return h.listChores(ctx, ownerUserID)
	case "create":
		return h.createChore(ctx, ownerUserID, args)
	case "show":
		return h.showChore(ctx, ownerUserID, args)
	case "delete":
		return h.deleteChore(ctx, ownerUserID, args)
	case "confirm-schedule":
		return h.confirmSchedule(ctx, ownerUserID, args)
	case "help":
		return h.showHelp(), nil
	case "chat":
		// FR-031 ("/chore chat <id> <message>") is explicitly DEFERRED —
		// ChoresService has no chat/converse method to mirror. A dedicated
		// case (rather than falling through to the generic
		// unrecognized-subcommand response), mirroring
		// ProjectCommandHandler's FR-020 deferral: guards against
		// accidentally building new CLI-only chat capability later, and
		// responds with a specific "not yet implemented" message (FR-003,
		// auto-review fix pass) instead of one indistinguishable from a
		// genuine typo.
		return choreChatNotImplementedMessage, nil
	default:
		return fmt.Sprintf("Unknown chore command: %s\nUse '/chore help' for usage information.", subcommand), nil
	}
}

// listChores lists ownerUserID's Chores (FR-028).
func (h *ChoreCommandHandler) listChores(ctx context.Context, ownerUserID string) (string, error) {
	list, err := h.service.ListChores(ctx, ownerUserID)
	if err != nil {
		return "", fmt.Errorf("failed to list chores: %w", err)
	}

	if len(list) == 0 {
		return "No chores found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d chore(s):\n\n", len(list)))
	for i, c := range list {
		result.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", i+1, c.Title, c.ID))
		result.WriteString(fmt.Sprintf("   Schedule: %s\n", formatChoreSchedule(c)))
		result.WriteString(fmt.Sprintf("   Next run: %s\n", formatChoreNextRun(c)))
		if c.PendingDeletion {
			result.WriteString("   Status: pending deletion (active run in progress)\n")
		}
	}
	return result.String(), nil
}

// createChore creates a new Chore (FR-029). The schedule is confirmed
// immediately (userConfirmed=true), mirroring
// internal/adapter/web/chores_handler.go's handleChoreCreate: the web UI's
// create form always sets a user-set schedule this way, reserving an
// agent-proposed, pending-confirmation schedule (userConfirmed=false) for
// the chat interface — which is deferred (FR-031) and has no CLI
// equivalent in this pass.
// Usage: /chore create <title> <description> [--dir <path>] --schedule <preset-or-cron-expr>
func (h *ChoreCommandHandler) createChore(ctx context.Context, ownerUserID string, args []string) (string, error) {
	const usage = "Usage: /chore create <title> <description> [--dir <path>] --schedule <preset-or-cron-expr>"
	if len(args) < 2 {
		return usage, nil
	}

	title := args[0]
	description := args[1]
	rest := args[2:]

	var workingDirectory, scheduleExpr string
	scheduleFound := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--dir":
			if i+1 >= len(rest) {
				return "Missing value for --dir", nil
			}
			workingDirectory = rest[i+1]
			i++
		case "--schedule":
			// --schedule takes the rest of the arguments as its value
			// (joined by spaces) so a raw multi-field cron expression
			// (e.g. "*/5 * * * *") can be supplied, not just a single-word
			// preset. This means --schedule must come last.
			if i+1 >= len(rest) {
				return "Missing value for --schedule", nil
			}
			scheduleExpr = strings.Join(rest[i+1:], " ")
			scheduleFound = true
			i = len(rest)
		default:
			return usage, nil
		}
	}
	if !scheduleFound || scheduleExpr == "" {
		return usage, nil
	}

	schedule, err := resolveChoreSchedule(scheduleExpr)
	if err != nil {
		return "", fmt.Errorf("invalid schedule: %w", err)
	}

	c, err := h.service.CreateChore(ctx, ownerUserID, title, description, workingDirectory, schedule, true)
	if err != nil {
		return "", fmt.Errorf("failed to create chore: %w", err)
	}

	var result strings.Builder
	result.WriteString("✓ Chore created successfully\n")
	result.WriteString(fmt.Sprintf("ID: %s\n", c.ID))
	result.WriteString(fmt.Sprintf("Title: %s\n", c.Title))
	result.WriteString(fmt.Sprintf("Schedule: %s\n", formatChoreSchedule(c)))
	result.WriteString(fmt.Sprintf("Next run: %s\n", formatChoreNextRun(c)))
	return result.String(), nil
}

// showChore shows a Chore's schedule, confirmation status, and next run
// time (FR-030). Run history is not shown here: neither ChoresService
// (mirroring internal/adapter/web/chores_handler.go's interface) nor
// domain.Chore itself carries run-history data — the web UI's own
// ChoreDetailPageData has the same gap, so this mirrors it rather than
// inventing new capability.
// Usage: /chore show <id>
func (h *ChoreCommandHandler) showChore(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /chore show <id>", nil
	}

	id := args[0]
	c, err := h.service.GetChore(ctx, ownerUserID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Chore not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to get chore: %w", err)
	}

	var result strings.Builder
	result.WriteString("Chore Details:\n\n")
	result.WriteString(fmt.Sprintf("ID: %s\n", c.ID))
	result.WriteString(fmt.Sprintf("Title: %s\n", c.Title))
	result.WriteString(fmt.Sprintf("Description: %s\n", c.Description))
	if c.WorkingDirectory != "" {
		result.WriteString(fmt.Sprintf("Working Directory: %s\n", c.WorkingDirectory))
	}
	result.WriteString(fmt.Sprintf("Schedule: %s\n", formatChoreSchedule(c)))
	result.WriteString(fmt.Sprintf("Confirmed: %v\n", c.ScheduleConfirmed))
	result.WriteString(fmt.Sprintf("Next run: %s\n", formatChoreNextRun(c)))
	if c.PendingDeletion {
		result.WriteString("Status: pending deletion (active run in progress)\n")
	}
	result.WriteString(fmt.Sprintf("Created: %s\n", c.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("Updated: %s\n", c.UpdatedAt.Format("2006-01-02 15:04:05")))
	return result.String(), nil
}

// deleteChore deletes a Chore (FR-032). Service.DeleteChore soft-deletes
// (sets PendingDeletion) instead of removing the record outright when the
// Chore has an active Run (spec.md Edge Case #3 / FR-R8) — it reports this
// via the record's continued existence rather than a distinct return
// value, so this re-fetches the Chore after a successful delete call to
// surface the correct outcome to the terminal, without reimplementing that
// decision here.
// Usage: /chore delete <id>
func (h *ChoreCommandHandler) deleteChore(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /chore delete <id>", nil
	}

	id := args[0]
	if err := h.service.DeleteChore(ctx, ownerUserID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Chore not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to delete chore: %w", err)
	}

	if c, err := h.service.GetChore(ctx, ownerUserID, id); err == nil && c.PendingDeletion {
		return fmt.Sprintf("Chore %s has an active run in progress — it will be deleted once the run completes.", id), nil
	}
	return fmt.Sprintf("✓ Chore %s deleted successfully", id), nil
}

// confirmSchedule confirms an agent-proposed schedule (FR-033), allowing
// the Chore to begin firing.
// Usage: /chore confirm-schedule <id>
func (h *ChoreCommandHandler) confirmSchedule(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /chore confirm-schedule <id>", nil
	}

	id := args[0]
	if err := h.service.ConfirmSchedule(ctx, ownerUserID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Chore not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to confirm chore schedule: %w", err)
	}

	return fmt.Sprintf("✓ Schedule confirmed for chore %s", id), nil
}

// showHelp returns help text for Chore environment commands.
func (h *ChoreCommandHandler) showHelp() string {
	return `Chore Commands:

  /chore list
    List your Chores

  /chore create <title> <description> [--dir <path>] --schedule <preset-or-cron-expr>
    Create a new Chore. <preset-or-cron-expr> is one of hourly, daily,
    weekly, monthly, or a raw 5-field cron expression (e.g. "*/5 * * * *").
    The schedule is confirmed immediately, matching the web UI's create form.

  /chore show <id>
    Show a Chore's details: schedule, confirmation status, next run time

  /chore delete <id>
    Delete a Chore. If a run is currently active, the Chore is marked for
    deletion instead and removed once the run completes.

  /chore confirm-schedule <id>
    Confirm a pending schedule, allowing the Chore to begin firing

  /chore help
    Show this help message
`
}

// resolveChoreSchedule resolves a single --schedule flag value into a
// domain.Schedule: a recognized preset name (see domain.KnownPresets) takes
// precedence; otherwise the value is treated as a raw cron expression.
// Mirrors internal/adapter/web/chores_handler.go's resolveScheduleFromForm,
// adapted from two separate form fields to a single CLI flag value.
func resolveChoreSchedule(expr string) (domain.Schedule, error) {
	for _, preset := range domain.KnownPresets() {
		if string(preset) == expr {
			return domain.NewScheduleFromPreset(preset)
		}
	}
	return domain.NewScheduleFromCron(expr)
}

// formatChoreSchedule formats a Chore's schedule for display, including its
// preset name (if any) alongside the underlying cron expression.
func formatChoreSchedule(c *domain.Chore) string {
	if c.Schedule.Preset != nil {
		return fmt.Sprintf("%s (%s)", *c.Schedule.Preset, c.Schedule.CronExpression)
	}
	return c.Schedule.CronExpression
}

// formatChoreNextRun formats a Chore's next scheduled firing for display.
// An unconfirmed schedule never fires (domain.Chore.IsDue), so its
// NextFireTime is meaningless — reported as "pending confirmation" instead.
func formatChoreNextRun(c *domain.Chore) string {
	if !c.ScheduleConfirmed {
		return "pending confirmation"
	}
	if c.NextFireTime.IsZero() {
		return "not scheduled"
	}
	return c.NextFireTime.Format("2006-01-02 15:04:05")
}
