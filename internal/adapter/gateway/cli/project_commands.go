package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/projects"
)

// ProjectCommandHandler handles the Projects environment's CLI commands
// (FR-017–FR-022; FR-020's "/project chat" is deliberately DEFERRED — see
// HandleProjectCommand's default case).
type ProjectCommandHandler struct {
	service *projects.Service
}

// NewProjectCommandHandler creates a new Projects environment command handler.
func NewProjectCommandHandler(service *projects.Service) *ProjectCommandHandler {
	return &ProjectCommandHandler{service: service}
}

// Handle satisfies EnvCommandHandler, delegating to HandleProjectCommand.
func (h *ProjectCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleProjectCommand(ctx, currentUser, ownerUserID, input)
}

// HandleProjectCommand processes a "/project ..." command and returns the
// formatted terminal response. ownerUserID (AD-5: the authenticated
// session's Username) scopes every call into the Projects service — never
// currentUser.ID. Projects is available to any authenticated user; no
// admin-role gating is applied here.
func (h *ProjectCommandHandler) HandleProjectCommand(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	subcommand := parts[1]
	args := parts[2:]

	switch subcommand {
	case "list":
		return h.listProjects(ctx, ownerUserID)
	case "create":
		return h.createProject(ctx, ownerUserID, args)
	case "show":
		return h.showProject(ctx, ownerUserID, args)
	case "add-agents-file":
		return h.addAgentsFile(ctx, ownerUserID, args)
	case "delete":
		return h.deleteProject(ctx, ownerUserID, args)
	case "help":
		return h.showHelp(), nil
	default:
		// FR-020 ("/project chat <id> <message>") is explicitly DEFERRED —
		// ProjectsService has no chat/converse method to mirror. Falling
		// through to this unrecognized-subcommand response (rather than a
		// dedicated case) is deliberate: it guards against accidentally
		// building new CLI-only chat capability later.
		return fmt.Sprintf("Unknown project command: %s\nUse '/project help' for usage information.", subcommand), nil
	}
}

// listProjects lists ownerUserID's Projects (FR-017).
func (h *ProjectCommandHandler) listProjects(ctx context.Context, ownerUserID string) (string, error) {
	list, err := h.service.ListProjects(ctx, ownerUserID)
	if err != nil {
		return "", fmt.Errorf("failed to list projects: %w", err)
	}

	if len(list) == 0 {
		return "No projects found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d project(s):\n\n", len(list)))
	for i, p := range list {
		result.WriteString(fmt.Sprintf("%d. %s (ID: %s)\n", i+1, p.Name, p.ID))
		result.WriteString(fmt.Sprintf("   Output Directory: %s\n", p.OutputDirectory))
	}
	return result.String(), nil
}

// createProject creates a new Project (FR-018).
// Usage: /project create <name> <output-directory>
func (h *ProjectCommandHandler) createProject(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /project create <name> <output-directory>", nil
	}

	name := args[0]
	outputDirectory := args[1]

	p, err := h.service.CreateProject(ctx, ownerUserID, name, outputDirectory)
	if err != nil {
		return "", fmt.Errorf("failed to create project: %w", err)
	}

	return fmt.Sprintf("✓ Project created successfully\nID: %s\nName: %s\nOutput Directory: %s", p.ID, p.Name, p.OutputDirectory), nil
}

// showProject shows a Project's output directory, AGENTS.md presence, and
// retention setting (FR-019).
// Usage: /project show <id>
func (h *ProjectCommandHandler) showProject(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /project show <id>", nil
	}

	id := args[0]
	p, err := h.service.GetProject(ctx, ownerUserID, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Project not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to get project: %w", err)
	}

	agentsFileExists := false
	if _, statErr := os.Stat(p.AgentsFilePath()); statErr == nil {
		agentsFileExists = true
	}

	var result strings.Builder
	result.WriteString("Project Details:\n\n")
	result.WriteString(fmt.Sprintf("ID: %s\n", p.ID))
	result.WriteString(fmt.Sprintf("Name: %s\n", p.Name))
	result.WriteString(fmt.Sprintf("Output Directory: %s\n", p.OutputDirectory))
	result.WriteString(fmt.Sprintf("AGENTS.md: %s\n", agentsFileStatus(agentsFileExists)))
	result.WriteString(fmt.Sprintf("Retention: %s\n", formatRetention(p.Retention)))
	result.WriteString(fmt.Sprintf("Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("Updated: %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05")))
	return result.String(), nil
}

// addAgentsFile adds a starter AGENTS.md to a Project that doesn't yet have
// one (FR-021).
// Usage: /project add-agents-file <id>
func (h *ProjectCommandHandler) addAgentsFile(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /project add-agents-file <id>", nil
	}

	id := args[0]
	if err := h.service.AddAgentsFile(ctx, ownerUserID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Project not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to add AGENTS.md: %w", err)
	}

	return fmt.Sprintf("✓ AGENTS.md added to project %s", id), nil
}

// deleteProject deletes a Project (FR-022).
// Usage: /project delete <id>
func (h *ProjectCommandHandler) deleteProject(ctx context.Context, ownerUserID string, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /project delete <id>", nil
	}

	id := args[0]
	if err := h.service.DeleteProject(ctx, ownerUserID, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Sprintf("Project not found: %s", id), nil
		}
		return "", fmt.Errorf("failed to delete project: %w", err)
	}

	return fmt.Sprintf("✓ Project %s deleted successfully", id), nil
}

// showHelp returns help text for Project environment commands.
func (h *ProjectCommandHandler) showHelp() string {
	return `Project Commands:

  /project list
    List your Projects

  /project create <name> <output-directory>
    Create a new Project with the given output directory

  /project show <id>
    Show a Project's details: output directory, AGENTS.md presence, retention setting

  /project add-agents-file <id>
    Add a starter AGENTS.md to a Project that doesn't yet have one

  /project delete <id>
    Delete a Project

  /project help
    Show this help message
`
}

// agentsFileStatus formats AGENTS.md presence for display.
func agentsFileStatus(exists bool) string {
	if exists {
		return "present"
	}
	return "not present"
}

// formatRetention formats a Project's retention policy for display.
func formatRetention(r domain.RetentionPolicy) string {
	if r.IsNever() {
		return "Never"
	}
	return r.Period.String()
}
