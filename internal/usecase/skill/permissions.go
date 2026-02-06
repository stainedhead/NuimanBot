package skill

import "nuimanbot/internal/domain"

// SkillPermissions maps skill names to the minimum role required to execute them.
// Skills not in this map default to requiring RoleUser.
//
// Permission Levels:
//   - RoleGuest: Available to all users (including unauthenticated)
//   - RoleUser: Available to registered users
//   - RoleAdmin: Available only to administrators
var SkillPermissions = map[string]domain.Role{
	// Built-in skills (Phase 1) - Available to all
	"calculator": domain.RoleGuest,
	"datetime":   domain.RoleGuest,

	// Extended skills (Phase 2) - Require registered user
	"weather":    domain.RoleUser,
	"web_search": domain.RoleUser,
	"notes":      domain.RoleUser,

	// Admin commands (Phase 2) - Require admin
	"admin.user": domain.RoleAdmin,
}

// DefaultSkillPermission is the role required for skills not explicitly listed
// in SkillPermissions. This provides a safe default of requiring user registration.
const DefaultSkillPermission = domain.RoleUser
