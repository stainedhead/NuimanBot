package tool

import "nuimanbot/internal/domain"

// ToolPermissions maps tool names to the minimum role required to execute them.
// Tools not in this map default to requiring RoleUser.
//
// Permission Levels:
//   - RoleGuest: Available to all users (including unauthenticated)
//   - RoleUser: Available to registered users
//   - RoleAdmin: Available only to administrators
var ToolPermissions = map[string]domain.Role{
	// Built-in tools (Phase 1) - Available to all
	"calculator": domain.RoleGuest,
	"datetime":   domain.RoleGuest,

	// Extended tools (Phase 2) - Require registered user
	"weather":    domain.RoleUser,
	"web_search": domain.RoleUser,
	"notes":      domain.RoleUser,

	// Admin commands (Phase 2) - Require admin
	"admin.user": domain.RoleAdmin,
}

// DefaultToolPermission is the role required for tools not explicitly listed
// in ToolPermissions. This provides a safe default of requiring user registration.
const DefaultToolPermission = domain.RoleUser
