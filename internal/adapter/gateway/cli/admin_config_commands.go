package cli

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// ConfigReloader defines the interface for reloading configuration.
// This is implemented by usecase/config.ConfigManager.
type ConfigReloader interface {
	Reload(ctx context.Context) error
}

// AdminConfigCommandHandler handles administrative configuration commands.
type AdminConfigCommandHandler struct {
	reloader ConfigReloader
}

// NewAdminConfigCommandHandler creates a new admin config command handler.
func NewAdminConfigCommandHandler(reloader ConfigReloader) *AdminConfigCommandHandler {
	return &AdminConfigCommandHandler{
		reloader: reloader,
	}
}

// IsConfigCommand checks if the input is a config admin command.
func IsConfigCommand(input string) bool {
	return strings.HasPrefix(input, "/admin config ")
}

// HandleConfigCommand processes a config admin command and returns the response.
func (h *AdminConfigCommandHandler) HandleConfigCommand(ctx context.Context, currentUser *domain.User, input string) (string, error) {
	if currentUser.Role != domain.RoleAdmin {
		return "", domain.ErrInsufficientPermissions
	}

	parts := strings.Fields(input)
	// "/admin config" is 2 parts, need at least 3 for a subcommand
	if len(parts) < 3 {
		return h.showHelp(), nil
	}

	subcommand := parts[2]

	switch subcommand {
	case "reload":
		return h.handleReload(ctx)
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown config command: %s\nUse '/admin config help' for usage information.", subcommand), nil
	}
}

// handleReload triggers a configuration reload.
func (h *AdminConfigCommandHandler) handleReload(ctx context.Context) (string, error) {
	if err := h.reloader.Reload(ctx); err != nil {
		return fmt.Sprintf("Configuration reload failed: %v\nThe previous configuration remains active.", err), nil
	}
	return "Configuration reloaded successfully. All registered services have been notified.", nil
}

// showHelp returns the help text for config commands.
func (h *AdminConfigCommandHandler) showHelp() string {
	return `Configuration Management Commands:

Usage: /admin config <command>

Commands:
  reload    Reload configuration from files (hot-reload)
  help      Show this help message

Examples:
  /admin config reload
  /admin config help

Notes:
  - Reload validates the new configuration before applying
  - If validation fails, the previous configuration remains active
  - All registered services are notified of configuration changes`
}
