package cli

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/user"
)

// AdminCommandHandler handles administrative commands.
type AdminCommandHandler struct {
	userService *user.Service
}

// NewAdminCommandHandler creates a new admin command handler.
func NewAdminCommandHandler(userService *user.Service) *AdminCommandHandler {
	return &AdminCommandHandler{
		userService: userService,
	}
}

// IsAdminCommand checks if the input is an admin command.
func IsAdminCommand(input string) bool {
	return strings.HasPrefix(input, "/admin ")
}

// HandleAdminCommand processes an admin command and returns the response.
// Returns error if the user lacks admin permissions or command fails.
func (h *AdminCommandHandler) HandleAdminCommand(ctx context.Context, currentUser *domain.User, input string) (string, error) {
	// Check if user is admin
	if currentUser.Role != domain.RoleAdmin {
		return "", domain.ErrInsufficientPermissions
	}

	// Parse command
	parts := strings.Fields(input)
	if len(parts) < 2 {
		return h.showHelp(), nil
	}

	// Skip "/admin"
	subcommand := parts[1]

	switch subcommand {
	case "user":
		return h.handleUserCommand(ctx, parts[2:])
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown admin command: %s\nUse '/admin help' for usage information.", subcommand), nil
	}
}

// handleUserCommand handles user management subcommands.
func (h *AdminCommandHandler) handleUserCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /admin user <create|list|get|update|delete> [args...]", nil
	}

	action := args[0]

	switch action {
	case "create":
		return h.createUser(ctx, args[1:])
	case "list":
		return h.listUsers(ctx)
	case "get":
		return h.getUser(ctx, args[1:])
	case "update":
		return h.updateUser(ctx, args[1:])
	case "delete":
		return h.deleteUser(ctx, args[1:])
	default:
		return fmt.Sprintf("Unknown user command: %s", action), nil
	}
}

// createUser creates a new user.
// Usage: /admin user create <platform> <platform_uid> <role>
func (h *AdminCommandHandler) createUser(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "Usage: /admin user create <platform> <platform_uid> <role>\nExample: /admin user create cli alice user", nil
	}

	platform := domain.Platform(args[0])
	platformUID := args[1]
	role := domain.Role(args[2])

	// Validate role
	if role != domain.RoleGuest && role != domain.RoleUser && role != domain.RoleAdmin {
		return fmt.Sprintf("Invalid role: %s. Must be one of: guest, user, admin", role), nil
	}

	user, err := h.userService.CreateUser(ctx, platform, platformUID, role)
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return fmt.Sprintf("✓ User created successfully\nID: %s\nPlatform: %s\nPlatform UID: %s\nRole: %s",
		user.ID, platform, platformUID, role), nil
}

// listUsers lists all users in the system.
// Usage: /admin user list
func (h *AdminCommandHandler) listUsers(ctx context.Context) (string, error) {
	users, err := h.userService.ListUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		return "No users found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d user(s):\n\n", len(users)))

	for i, u := range users {
		result.WriteString(fmt.Sprintf("%d. ID: %s\n", i+1, u.ID))
		result.WriteString(fmt.Sprintf("   Username: %s\n", u.Username))
		result.WriteString(fmt.Sprintf("   Role: %s\n", u.Role))
		if len(u.AllowedSkills) > 0 {
			result.WriteString(fmt.Sprintf("   Allowed Skills: %v\n", u.AllowedSkills))
		}
		result.WriteString("\n")
	}

	return result.String(), nil
}

// getUser retrieves a user by ID.
// Usage: /admin user get <user_id>
func (h *AdminCommandHandler) getUser(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin user get <user_id>", nil
	}

	userID := args[0]
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	var result strings.Builder
	result.WriteString("User Details:\n")
	result.WriteString(fmt.Sprintf("ID: %s\n", user.ID))
	result.WriteString(fmt.Sprintf("Username: %s\n", user.Username))
	result.WriteString(fmt.Sprintf("Role: %s\n", user.Role))
	result.WriteString(fmt.Sprintf("Created: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("Updated: %s\n", user.UpdatedAt.Format("2006-01-02 15:04:05")))

	if len(user.PlatformIDs) > 0 {
		result.WriteString("Platform IDs:\n")
		for platform, uid := range user.PlatformIDs {
			result.WriteString(fmt.Sprintf("  %s: %s\n", platform, uid))
		}
	}

	if len(user.AllowedSkills) > 0 {
		result.WriteString(fmt.Sprintf("Allowed Skills: %v\n", user.AllowedSkills))
	} else {
		result.WriteString("Allowed Skills: all (for role)\n")
	}

	return result.String(), nil
}

// updateUser updates a user's properties.
// Usage: /admin user update <user_id> --role <role> | --skills <skill1,skill2,...>
func (h *AdminCommandHandler) updateUser(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /admin user update <user_id> --role <role> | --skills <skill1,skill2,...>", nil
	}

	userID := args[0]

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--role":
			if i+1 >= len(args) {
				return "Missing value for --role", nil
			}
			role := domain.Role(args[i+1])

			// Validate role
			if role != domain.RoleGuest && role != domain.RoleUser && role != domain.RoleAdmin {
				return fmt.Sprintf("Invalid role: %s. Must be one of: guest, user, admin", role), nil
			}

			if err := h.userService.UpdateUserRole(ctx, userID, role); err != nil {
				return "", fmt.Errorf("failed to update role: %w", err)
			}
			return fmt.Sprintf("✓ User role updated to %s", role), nil

		case "--skills":
			if i+1 >= len(args) {
				return "Missing value for --skills", nil
			}
			skillsStr := args[i+1]
			skills := strings.Split(skillsStr, ",")

			// Trim whitespace
			for j := range skills {
				skills[j] = strings.TrimSpace(skills[j])
			}

			if err := h.userService.UpdateAllowedSkills(ctx, userID, skills); err != nil {
				return "", fmt.Errorf("failed to update skills: %w", err)
			}
			return fmt.Sprintf("✓ User allowed skills updated to: %v", skills), nil

		default:
			return fmt.Sprintf("Unknown flag: %s", args[i]), nil
		}
	}

	return "No updates specified. Use --role or --skills", nil
}

// deleteUser deletes a user.
// Usage: /admin user delete <user_id>
func (h *AdminCommandHandler) deleteUser(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin user delete <user_id>", nil
	}

	userID := args[0]
	if err := h.userService.DeleteUser(ctx, userID); err != nil {
		return "", fmt.Errorf("failed to delete user: %w", err)
	}

	return fmt.Sprintf("✓ User %s deleted successfully", userID), nil
}

// showHelp returns help text for admin commands.
func (h *AdminCommandHandler) showHelp() string {
	return `Admin Commands:

User Management:
  /admin user create <platform> <platform_uid> <role>
    Create a new user
    Example: /admin user create cli alice user

  /admin user list
    List all users

  /admin user get <user_id>
    Get details for a specific user

  /admin user update <user_id> --role <role>
    Update a user's role
    Example: /admin user update abc123 --role admin

  /admin user update <user_id> --skills <skill1,skill2,...>
    Update a user's allowed skills
    Example: /admin user update abc123 --skills calculator,datetime

  /admin user delete <user_id>
    Delete a user

General:
  /admin help
    Show this help message
`
}
