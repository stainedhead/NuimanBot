package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/settings"
)

// networkModeNotImplementedMessage documents FR-043's DEFERRED status (see
// specs/260811-cli-parity-for-nuimanbot-features/implementation-notes.md).
// This is deliberately deferred, not just an unwired stub: even the web
// UI's own network-mode setting doesn't actually rebind the running
// listener (FR-R11, a known, pre-existing, deliberately-deferred gap from
// the six-environments feature). Sharing state between the CLI and web
// adapters for this setting would make the value consistent between them
// but still not functionally effective — not worth the unplanned refactor
// (web/middleware.go, web/server.go, settings.Service, main.go) until
// FR-R11 itself is fixed in a future pass.
const networkModeNotImplementedMessage = "not yet implemented (deferred, see spec.md FR-043): even the web UI's network-mode setting doesn't currently rebind the running listener (a known, separately-tracked limitation, FR-R11) — revisit together with that fix in a future pass. Use the web admin Settings page for now."

// retentionSetNotImplementedMessage documents FR-040's scope deferral: see
// implementation-notes.md — no per-user retention override exists anywhere
// in the web UI to mirror, so this pass does not invent one (Non-Goal: "no
// new capabilities beyond what the web UI already has").
const retentionSetNotImplementedMessage = "not yet implemented: per-user retention overrides do not exist in the web UI to mirror (deferred, see spec.md FR-040). /settings show displays the system-wide default."

// SettingsCommandHandler handles the /settings CLI commands (FR-039-043).
// Per this pass's documented scope: /settings show displays the
// system-wide retention defaults read-only (not a per-user value — no
// per-user override storage exists, see implementation-notes.md);
// worker-pool-size is fully backed and admin-gated; network-mode and
// per-user retention "set" are stubbed with a clear not-yet-implemented
// message rather than silently doing nothing or inventing new capability.
type SettingsCommandHandler struct {
	service *settings.Service
}

// NewSettingsCommandHandler creates a new SettingsCommandHandler.
func NewSettingsCommandHandler(service *settings.Service) *SettingsCommandHandler {
	return &SettingsCommandHandler{service: service}
}

// Handle satisfies cli.EnvCommandHandler.
func (h *SettingsCommandHandler) Handle(ctx context.Context, currentUser *domain.User, ownerUserID string, input string) (string, error) {
	return h.HandleSettingsCommand(ctx, currentUser, ownerUserID, input)
}

// HandleSettingsCommand processes a /settings command and returns the
// formatted terminal response. ownerUserID is accepted for interface
// conformance and future per-user settings but unused this pass — see the
// per-user-retention scope note above.
func (h *SettingsCommandHandler) HandleSettingsCommand(_ context.Context, currentUser *domain.User, _ string, input string) (string, error) {
	if h.service == nil {
		return "", fmt.Errorf("settings service not configured")
	}

	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	switch parts[1] {
	case "show":
		if len(parts) >= 3 && parts[2] == "--system" {
			return h.showSystem(currentUser)
		}
		return h.showUser(), nil
	case "set":
		return h.handleSet(currentUser, parts[2:])
	default:
		return h.showHelp(), nil
	}
}

// showUser renders the read-only, system-wide retention defaults shown by
// "/settings show" (available to any authenticated user — see scope note).
func (h *SettingsCommandHandler) showUser() string {
	chatDays, projectDays, historyDays := h.service.RetentionDefaults()

	var b strings.Builder
	b.WriteString("Settings:\n")
	b.WriteString(fmt.Sprintf("  Chat retention:    %s\n", formatRetentionDays(chatDays)))
	b.WriteString(fmt.Sprintf("  Project retention: %s\n", formatRetentionDays(projectDays)))
	b.WriteString(fmt.Sprintf("  History retention: %s\n", formatRetentionDays(historyDays)))
	b.WriteString("  (system-wide defaults; per-user overrides are not yet available — see /settings set retention)")
	return b.String()
}

// showSystem renders "/settings show --system" (admin-only, FR-041).
func (h *SettingsCommandHandler) showSystem(currentUser *domain.User) (string, error) {
	if !isAdminUser(currentUser) {
		return "", domain.ErrInsufficientPermissions
	}

	var b strings.Builder
	b.WriteString("System Settings:\n")
	b.WriteString(fmt.Sprintf("  Worker pool size:  %d\n", h.service.WorkerPoolSize()))
	b.WriteString(fmt.Sprintf("  Skills registered: %d\n", len(h.service.SkillNames())))
	b.WriteString("  Network mode: " + networkModeNotImplementedMessage)
	return b.String(), nil
}

// handleSet dispatches "/settings set <target> ...".
func (h *SettingsCommandHandler) handleSet(currentUser *domain.User, args []string) (string, error) {
	if len(args) == 0 {
		return h.showHelp(), nil
	}

	switch args[0] {
	case "retention":
		return retentionSetNotImplementedMessage, nil
	case "worker-pool-size":
		return h.setWorkerPoolSize(currentUser, args[1:])
	case "network-mode":
		if !isAdminUser(currentUser) {
			return "", domain.ErrInsufficientPermissions
		}
		return networkModeNotImplementedMessage, nil
	default:
		return h.showHelp(), nil
	}
}

// setWorkerPoolSize implements "/settings set worker-pool-size <n>"
// (admin-only, FR-042).
func (h *SettingsCommandHandler) setWorkerPoolSize(currentUser *domain.User, args []string) (string, error) {
	if !isAdminUser(currentUser) {
		return "", domain.ErrInsufficientPermissions
	}
	if len(args) < 1 {
		return "", fmt.Errorf("usage: /settings set worker-pool-size <n>")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid worker pool size %q: %w", args[0], err)
	}
	if n > domain.MaxWorkerPoolSize {
		return "", fmt.Errorf("worker pool size must not exceed %d", domain.MaxWorkerPoolSize)
	}
	if err := h.service.SetWorkerPoolSize(n); err != nil {
		return "", fmt.Errorf("set worker pool size: %w", err)
	}

	return fmt.Sprintf("Worker pool size set to %d.", n), nil
}

// showHelp lists the available /settings subcommands.
func (h *SettingsCommandHandler) showHelp() string {
	return "Settings commands:\n" +
		"  /settings show                                        - show retention defaults\n" +
		"  /settings show --system                                - (admin) show worker pool size, skills, network mode\n" +
		"  /settings set retention <chat|project|history> <n|never> - (not yet implemented, per-user overrides don't exist)\n" +
		"  /settings set worker-pool-size <n>                     - (admin)\n" +
		"  /settings set network-mode <localhost|remote>          - (admin, not yet implemented)"
}

// isAdminUser reports whether user is a non-nil admin. Local helper (rather
// than reusing Gateway.hasAdminRole, which reads Gateway's own currentUser
// field) since SettingsCommandHandler receives currentUser as a parameter,
// not from a Gateway.
func isAdminUser(user *domain.User) bool {
	return user != nil && user.Role == domain.RoleAdmin
}

// formatRetentionDays renders a retention window in days as human-readable
// text, "Never" for 0/negative (matching domain's Never-retention
// convention used elsewhere, e.g. RetentionPolicy.IsNever).
func formatRetentionDays(days int) string {
	if days <= 0 {
		return "Never"
	}
	return fmt.Sprintf("%d days", days)
}
