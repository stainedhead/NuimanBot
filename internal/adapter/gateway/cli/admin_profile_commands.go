package cli

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// ProfileService defines the interface for user profile operations.
type ProfileService interface {
	CreateProfile(ctx context.Context, profile *domain.UserProfile) error
	GetProfile(ctx context.Context, userID string) (*domain.UserProfile, error)
	GetProfileByEmail(ctx context.Context, email string) (*domain.UserProfile, error)
	UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error
	DeleteProfile(ctx context.Context, userID string) error
	ListProfiles(ctx context.Context, offset, limit int) ([]*domain.UserProfile, error)
}

// AdminProfileCommandHandler handles administrative profile commands.
type AdminProfileCommandHandler struct {
	profileService ProfileService
}

// NewAdminProfileCommandHandler creates a new admin profile command handler.
func NewAdminProfileCommandHandler(profileService ProfileService) *AdminProfileCommandHandler {
	return &AdminProfileCommandHandler{
		profileService: profileService,
	}
}

// IsProfileCommand checks if the input is a profile admin command.
func IsProfileCommand(input string) bool {
	return strings.HasPrefix(input, "/admin profile ")
}

// HandleProfileCommand processes a profile admin command and returns the response.
func (h *AdminProfileCommandHandler) HandleProfileCommand(ctx context.Context, currentUser *domain.User, input string) (string, error) {
	// Check if user is admin
	if currentUser.Role != domain.RoleAdmin {
		return "", domain.ErrInsufficientPermissions
	}

	// Parse command
	parts := strings.Fields(input)
	if len(parts) < 3 {
		return h.showHelp(), nil
	}

	// Skip "/admin profile"
	subcommand := parts[2]

	switch subcommand {
	case "create":
		return h.createProfile(ctx, parts[3:])
	case "list":
		return h.listProfiles(ctx)
	case "view":
		return h.viewProfile(ctx, parts[3:])
	case "update":
		return h.updateProfile(ctx, parts[3:])
	case "delete":
		return h.deleteProfile(ctx, parts[3:])
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown profile command: %s\nUse '/admin profile help' for usage information.", subcommand), nil
	}
}

// createProfile creates a new user profile.
// Usage: /admin profile create <user_id> <email> [--moniker <value>] [--first-name <value>] [--last-name <value>]
func (h *AdminProfileCommandHandler) createProfile(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /admin profile create <user_id> <email> [--moniker <value>] [--first-name <value>] [--last-name <value>]", nil
	}

	userID := args[0]
	email := args[1]

	// Create profile with required fields
	profile := domain.NewUserProfile(userID, email, domain.UserTypeIndividual)

	// Parse optional flags
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return fmt.Sprintf("Missing value for flag: %s", args[i]), nil
		}

		flag := args[i]
		value := args[i+1]

		switch flag {
		case "--moniker":
			profile.Moniker = value
		case "--first-name":
			profile.FirstName = value
		case "--last-name":
			profile.LastName = value
		case "--nick-name":
			profile.NickName = value
		case "--mobile-phone":
			profile.MobilePhone = value
		case "--timezone":
			profile.Timezone = value
		case "--location":
			profile.PrimaryLocation = value
		case "--job-role":
			profile.JobRole = value
		case "--user-type":
			profile.UserType = domain.UserType(value)
		default:
			return fmt.Sprintf("Unknown flag: %s", flag), nil
		}
	}

	if err := h.profileService.CreateProfile(ctx, profile); err != nil {
		return "", fmt.Errorf("failed to create profile: %w", err)
	}

	return fmt.Sprintf("✓ Profile created successfully\nUser ID: %s\nEmail: %s\nMoniker: %s",
		profile.UserID, profile.PrimaryEmail, profile.Moniker), nil
}

// listProfiles lists all user profiles.
// Usage: /admin profile list
func (h *AdminProfileCommandHandler) listProfiles(ctx context.Context) (string, error) {
	profiles, err := h.profileService.ListProfiles(ctx, 0, 100)
	if err != nil {
		return "", fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(profiles) == 0 {
		return "No profiles found.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d profile(s):\n\n", len(profiles)))

	for i, p := range profiles {
		result.WriteString(fmt.Sprintf("%d. User ID: %s\n", i+1, p.UserID))
		result.WriteString(fmt.Sprintf("   Email: %s\n", p.PrimaryEmail))
		if p.Moniker != "" {
			result.WriteString(fmt.Sprintf("   Moniker: %s\n", p.Moniker))
		}
		if p.FirstName != "" || p.LastName != "" {
			result.WriteString(fmt.Sprintf("   Name: %s %s\n", p.FirstName, p.LastName))
		}
		result.WriteString(fmt.Sprintf("   Type: %s\n", p.UserType))
		result.WriteString("\n")
	}

	return result.String(), nil
}

// viewProfile retrieves and displays a user profile.
// Usage: /admin profile view <user_id>
func (h *AdminProfileCommandHandler) viewProfile(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin profile view <user_id>", nil
	}

	userID := args[0]
	profile, err := h.profileService.GetProfile(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get profile: %w", err)
	}

	var result strings.Builder
	result.WriteString("Profile Details:\n\n")
	result.WriteString(fmt.Sprintf("User ID: %s\n", profile.UserID))
	result.WriteString(fmt.Sprintf("Email: %s\n", profile.PrimaryEmail))

	if profile.Moniker != "" {
		result.WriteString(fmt.Sprintf("Moniker: %s\n", profile.Moniker))
	}
	if profile.FirstName != "" {
		result.WriteString(fmt.Sprintf("First Name: %s\n", profile.FirstName))
	}
	if profile.LastName != "" {
		result.WriteString(fmt.Sprintf("Last Name: %s\n", profile.LastName))
	}
	if profile.NickName != "" {
		result.WriteString(fmt.Sprintf("Nickname: %s\n", profile.NickName))
	}
	if profile.MobilePhone != "" {
		result.WriteString(fmt.Sprintf("Mobile: %s\n", profile.MobilePhone))
	}
	if profile.PrimaryLocation != "" {
		result.WriteString(fmt.Sprintf("Location: %s\n", profile.PrimaryLocation))
	}
	if profile.Timezone != "" {
		result.WriteString(fmt.Sprintf("Timezone: %s\n", profile.Timezone))
	}
	if profile.JobRole != "" {
		result.WriteString(fmt.Sprintf("Job Role: %s\n", profile.JobRole))
	}

	result.WriteString(fmt.Sprintf("User Type: %s\n", profile.UserType))
	result.WriteString(fmt.Sprintf("Enabled: %v\n", profile.Enabled))
	result.WriteString(fmt.Sprintf("Data Directory: %s\n", profile.DataDirectory))

	// Platform IDs
	if profile.PlatformIDs.Slack != "" || profile.PlatformIDs.Telegram != "" || profile.PlatformIDs.CLI != "" {
		result.WriteString("\nPlatform IDs:\n")
		if profile.PlatformIDs.Slack != "" {
			result.WriteString(fmt.Sprintf("  Slack: %s\n", profile.PlatformIDs.Slack))
		}
		if profile.PlatformIDs.Telegram != "" {
			result.WriteString(fmt.Sprintf("  Telegram: %s\n", profile.PlatformIDs.Telegram))
		}
		if profile.PlatformIDs.CLI != "" {
			result.WriteString(fmt.Sprintf("  CLI: %s\n", profile.PlatformIDs.CLI))
		}
	}

	result.WriteString(fmt.Sprintf("\nCreated: %s\n", profile.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("Updated: %s\n", profile.UpdatedAt.Format("2006-01-02 15:04:05")))

	return result.String(), nil
}

// updateProfile updates a user profile.
// Usage: /admin profile update <user_id> [--field value] ...
func (h *AdminProfileCommandHandler) updateProfile(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "Usage: /admin profile update <user_id> [--field value] ...\nExample: /admin profile update user-123 --moniker alice --first-name Alice", nil
	}

	userID := args[0]
	updates := make(map[string]interface{})

	// Parse flags
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return fmt.Sprintf("Missing value for flag: %s", args[i]), nil
		}

		flag := args[i]
		value := args[i+1]

		switch flag {
		case "--moniker":
			updates["moniker"] = value
		case "--first-name":
			updates["firstName"] = value
		case "--last-name":
			updates["lastName"] = value
		case "--nick-name":
			updates["nickName"] = value
		case "--mobile-phone":
			updates["mobilePhone"] = value
		case "--timezone":
			updates["timezone"] = value
		case "--location":
			updates["primaryLocation"] = value
		case "--job-role":
			updates["jobRole"] = value
		case "--enabled":
			updates["enabled"] = value == "true"
		default:
			return fmt.Sprintf("Unknown flag: %s", flag), nil
		}
	}

	if len(updates) == 0 {
		return "No updates specified. Use --field value flags.", nil
	}

	if err := h.profileService.UpdateProfile(ctx, userID, updates); err != nil {
		return "", fmt.Errorf("failed to update profile: %w", err)
	}

	return fmt.Sprintf("✓ Profile updated successfully\nUser ID: %s\nUpdated fields: %v", userID, getUpdateFields(updates)), nil
}

// deleteProfile deletes a user profile.
// Usage: /admin profile delete <user_id>
func (h *AdminProfileCommandHandler) deleteProfile(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin profile delete <user_id>", nil
	}

	userID := args[0]
	if err := h.profileService.DeleteProfile(ctx, userID); err != nil {
		return "", fmt.Errorf("failed to delete profile: %w", err)
	}

	return fmt.Sprintf("✓ Profile %s deleted successfully", userID), nil
}

// showHelp returns help text for profile admin commands.
func (h *AdminProfileCommandHandler) showHelp() string {
	return `Admin Profile Commands:

Profile Management:
  /admin profile create <user_id> <email> [flags]
    Create a new user profile
    Flags: --moniker, --first-name, --last-name, --nick-name, --mobile-phone,
           --timezone, --location, --job-role, --user-type
    Example: /admin profile create user-123 alice@example.com --moniker alice --first-name Alice

  /admin profile list
    List all user profiles

  /admin profile view <user_id>
    View detailed profile information

  /admin profile update <user_id> [flags]
    Update profile fields
    Flags: --moniker, --first-name, --last-name, --nick-name, --mobile-phone,
           --timezone, --location, --job-role, --enabled
    Example: /admin profile update user-123 --moniker alice-updated

  /admin profile delete <user_id>
    Delete a user profile

  /admin profile help
    Show this help message
`
}

// getUpdateFields extracts field names from updates map for display.
func getUpdateFields(updates map[string]interface{}) []string {
	fields := make([]string, 0, len(updates))
	for k := range updates {
		fields = append(fields, k)
	}
	return fields
}
