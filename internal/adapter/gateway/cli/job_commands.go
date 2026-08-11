package cli

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/jobs"
)

// JobService is the subset of internal/usecase/jobs.Service's exported
// surface JobCommandHandler depends on (FR-023, FR-024, FR-025, FR-027;
// FR-026's per-Job chat interface is deferred — see spec.md). Defined here,
// where it is used, rather than imported from internal/usecase/jobs, per
// AGENTS.md's "define interfaces where they are used" guidance — this also
// lets tests substitute a fake without depending on jobs.Service's concrete
// storage/enqueuer wiring. Satisfied by *jobs.Service.
type JobService interface {
	// CreateJob creates a Job (FR-024).
	CreateJob(ctx context.Context, ownerUserID, title, description string, contextType domain.ContextType, contextID string) (*domain.Job, error)
	// ListJobs returns ownerUserID's Jobs (FR-023).
	ListJobs(ctx context.Context, ownerUserID string) ([]*domain.Job, error)
	// GetJob retrieves a Job by ID, scoped to its owner (FR-025).
	GetJob(ctx context.Context, ownerUserID, jobID string) (*domain.Job, error)
	// DeleteJob deletes a Job, scoped to its owner (FR-027).
	DeleteJob(ctx context.Context, ownerUserID, jobID string) error
}

// jobChatNotImplementedMessage documents FR-026's deliberate deferral
// (auto-review fix pass FR-003): "/job chat" is a known, deferred command,
// not a typo — JobService has no chat/converse method to mirror, matching
// Settings' existing "not yet implemented" convention rather than falling
// through to the generic unrecognized-subcommand response, which was
// indistinguishable from a genuine typo.
const jobChatNotImplementedMessage = "'/job chat' is not yet implemented (deferred, see spec.md FR-026). Use '/job help' for available commands."

// JobCommandHandler handles the Jobs environment's CLI commands
// (FR-023–FR-027; FR-026 is deferred — see spec.md's Jobs section,
// architecture.md's "Scope correction", and HandleJobCommand's "chat"
// case).
type JobCommandHandler struct {
	service JobService
}

// NewJobCommandHandler creates a JobCommandHandler backed by service.
func NewJobCommandHandler(service *jobs.Service) *JobCommandHandler {
	return &JobCommandHandler{service: service}
}

// Handle implements EnvCommandHandler by delegating to HandleJobCommand.
func (h *JobCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleJobCommand(ctx, currentUser, ownerUserID, input)
}

// HandleJobCommand parses and executes a "/job ..." command, always scoping
// the underlying jobs.Service calls to ownerUserID (AD-5: the authenticated
// session's Username, passed explicitly by the caller) — never
// currentUser.ID or any other field of currentUser, which is available here
// only for future role/permission checks (Jobs has none today; every
// authenticated user may use these commands).
func (h *JobCommandHandler) HandleJobCommand(ctx context.Context, _ *domain.User, ownerUserID string, input string) (string, error) {
	tokens := splitJobCommandFields(input)
	if len(tokens) < 2 {
		return h.showHelp(), nil
	}

	subcommand := tokens[1]
	args := tokens[2:]

	switch subcommand {
	case "list":
		return h.list(ctx, ownerUserID, args)
	case "create":
		return h.create(ctx, ownerUserID, args)
	case "show":
		return h.show(ctx, ownerUserID, args)
	case "delete":
		return h.delete(ctx, ownerUserID, args)
	case "help":
		return h.showHelp(), nil
	case "chat":
		// FR-026 ("/job chat <id> <message>") is explicitly DEFERRED — no
		// per-Job chat interface exists. A dedicated case (rather than
		// falling through to the generic unrecognized-subcommand
		// response) guards against accidentally building new CLI-only
		// chat capability later, and responds with a specific "not yet
		// implemented" message (FR-003, auto-review fix pass) instead of
		// one indistinguishable from a genuine typo.
		return jobChatNotImplementedMessage, nil
	default:
		return fmt.Sprintf("Unknown job command: %s\nUse '/job help' for usage information.", subcommand), nil
	}
}

// list handles "/job list [--project <id>|--chat <id>]" (FR-023).
// jobs.Service.ListJobs has no context-filter parameter, so this filters
// the owner's already-scoped list client-side by the requested context
// rather than adding new usecase-layer filtering capability.
func (h *JobCommandHandler) list(ctx context.Context, ownerUserID string, args []string) (string, error) {
	contextType, contextID, err := parseContextFlag(args)
	if err != nil {
		return err.Error(), nil
	}

	allJobs, err := h.service.ListJobs(ctx, ownerUserID)
	if err != nil {
		return "", fmt.Errorf("failed to list jobs: %w", err)
	}

	filtered := allJobs
	if contextID != "" {
		filtered = make([]*domain.Job, 0, len(allJobs))
		for _, j := range allJobs {
			if j.ContextType == contextType && j.ContextID == contextID {
				filtered = append(filtered, j)
			}
		}
	}

	if len(filtered) == 0 {
		return "No jobs found.", nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d job(s):\n\n", len(filtered))
	for _, j := range filtered {
		writeJobSummary(&b, j)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// create handles "/job create <title> <description> [--project <id>|--chat <id>]"
// (FR-024). Title and description may be single tokens or double-quoted
// phrases (e.g. "Clean the inbox") to allow multi-word text.
func (h *JobCommandHandler) create(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /job create <title> <description> [--project <id>|--chat <id>]\nQuote title/description if they contain spaces.", nil
	}

	title := args[0]
	description := args[1]

	contextType, contextID, err := parseContextFlag(args[2:])
	if err != nil {
		return err.Error(), nil
	}
	if contextID == "" {
		contextType = domain.ContextTypeChat
	}

	job, err := h.service.CreateJob(ctx, ownerUserID, title, description, contextType, contextID)
	if err != nil {
		return "", fmt.Errorf("failed to create job: %w", err)
	}

	var b strings.Builder
	b.WriteString("Job created:\n\n")
	writeJobSummary(&b, job)
	return strings.TrimRight(b.String(), "\n"), nil
}

// show handles "/job show <id>" (FR-025).
func (h *JobCommandHandler) show(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /job show <id>", nil
	}

	job, err := h.service.GetJob(ctx, ownerUserID, args[0])
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	var b strings.Builder
	b.WriteString("Job Details:\n\n")
	writeJobDetail(&b, job)
	return strings.TrimRight(b.String(), "\n"), nil
}

// delete handles "/job delete <id>" (FR-027).
func (h *JobCommandHandler) delete(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /job delete <id>", nil
	}

	id := args[0]
	if err := h.service.DeleteJob(ctx, ownerUserID, id); err != nil {
		return "", fmt.Errorf("failed to delete job: %w", err)
	}
	return fmt.Sprintf("Job %s deleted.", id), nil
}

// showHelp returns help text for the Jobs environment commands.
func (h *JobCommandHandler) showHelp() string {
	return `Job Commands:

  /job list [--project <id>|--chat <id>]
    List your jobs, optionally filtered by context.

  /job create <title> <description> [--project <id>|--chat <id>]
    Create a new job. Quote title/description if they contain spaces.
    Example: /job create "Clean the inbox" "Archive anything older than 30 days." --project proj-1

  /job show <id>
    Show a job's details.

  /job delete <id>
    Delete a job.

  /job help
    Show this help message.
`
}

// parseContextFlag parses an optional trailing "--project <id>" or
// "--chat <id>" flag pair shared by "list" and "create". Returns a zero
// domain.ContextType and empty contextID if no flag is present.
func parseContextFlag(args []string) (domain.ContextType, string, error) {
	if len(args) == 0 {
		return "", "", nil
	}
	if len(args) != 2 {
		return "", "", fmt.Errorf("usage: [--project <id>|--chat <id>]")
	}

	switch args[0] {
	case "--project":
		return domain.ContextTypeProject, args[1], nil
	case "--chat":
		return domain.ContextTypeChat, args[1], nil
	default:
		return "", "", fmt.Errorf("unknown flag: %s\nUsage: [--project <id>|--chat <id>]", args[0])
	}
}

// writeJobSummary writes a short, one-job summary block (used by list and
// create output).
func writeJobSummary(b *strings.Builder, j *domain.Job) {
	fmt.Fprintf(b, "ID: %s\n", j.ID)
	fmt.Fprintf(b, "Title: %s\n", j.Title)
	fmt.Fprintf(b, "Status: %s\n", j.Status)
	if j.ContextType != "" && j.ContextID != "" {
		fmt.Fprintf(b, "Context: %s %s\n", j.ContextType, j.ContextID)
	}
	b.WriteString("\n")
}

// writeJobDetail writes the full detail block for a single job (used by
// show output).
func writeJobDetail(b *strings.Builder, j *domain.Job) {
	fmt.Fprintf(b, "ID: %s\n", j.ID)
	fmt.Fprintf(b, "Title: %s\n", j.Title)
	fmt.Fprintf(b, "Description: %s\n", j.Description)
	fmt.Fprintf(b, "Status: %s\n", j.Status)
	if j.ContextType != "" && j.ContextID != "" {
		fmt.Fprintf(b, "Context: %s %s\n", j.ContextType, j.ContextID)
	}
	if j.WorkingDirectory != "" {
		fmt.Fprintf(b, "Working Directory: %s\n", j.WorkingDirectory)
	}
	if j.PendingDeletion {
		b.WriteString("Pending Deletion: true\n")
	}
	fmt.Fprintf(b, "Created: %s\n", j.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(b, "Updated: %s\n", j.UpdatedAt.Format("2006-01-02 15:04:05"))
}

// splitJobCommandFields splits a "/job ..." input line into whitespace-
// separated tokens, treating a double-quoted span as a single token so
// multi-word title/description values (e.g. "Clean the inbox") survive as
// one field. Quote characters themselves are stripped from the output.
func splitJobCommandFields(input string) []string {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	hasCur := false

	for _, r := range input {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			hasCur = true
		case r == ' ' && !inQuotes:
			if hasCur {
				fields = append(fields, cur.String())
				cur.Reset()
				hasCur = false
			}
		default:
			cur.WriteRune(r)
			hasCur = true
		}
	}
	if hasCur {
		fields = append(fields, cur.String())
	}
	return fields
}
