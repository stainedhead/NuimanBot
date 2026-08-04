package tool

import "nuimanbot/internal/domain"

// ToolPermissions maps tool names to the minimum role required to execute
// them. Every tool actually registered by cmd/nuimanbot (registerBuiltInTools
// / registerDeveloperProductivityTools) MUST have an explicit entry here —
// see permissions_test.go's CI guard, which fails the build if a newly
// registered tool has no deliberate permission decision recorded. There is
// intentionally no silent permissive fallback for registered tools;
// DefaultToolPermission below is a defensive last resort, not a design
// escape hatch (see its doc comment).
//
// Permission Levels:
//   - RoleGuest: Available to all users (including unauthenticated)
//   - RoleUser: Available to registered users
//   - RoleAdmin: Available only to administrators
//
// "github"'s permission requirement is action-aware (see
// Service.checkPermission / githubActionRole in service.go): read-only
// actions (issue_list, issue_view, pr_list, pr_view, repo_view) remain
// RoleUser even though the table below lists "github" itself as RoleAdmin —
// that entry is the ceiling applied to write actions (issue_create,
// issue_comment, issue_close, pr_create, pr_review, pr_merge, workflow_run)
// and to any call whose action can't be resolved.
//
// Operators can override any entry below without a code change via the
// `tools.permissions` config section (see config.ToolsSystemConfig.Permissions
// and Service.resolveRequiredRole) — e.g. to restore the pre-Phase-3
// permissive default for "github" in a deployment that needs it.
var ToolPermissions = map[string]domain.Role{
	// Read-only / harmless tools — available to all users, including guests.
	"calculator": domain.RoleGuest,
	"datetime":   domain.RoleGuest,

	// Tools requiring a registered user account.
	//
	// Dynamically-registered MCP tools ("mcp:<server>:<tool>", see
	// internal/adapter/mcp.MCPToolAdapter) are deliberately NOT listed in this
	// map — their RBAC treatment is trust-based, not a static entry (Phase 6 /
	// Part F, FR-023): see Service.resolveRequiredRole's dedicated
	// isMCPTool/mcpToolTrustLevel branch in service.go and mcp_trust.go.
	//
	// NOTE: internal/tools/websearch.WebSearch.Name() returns "websearch",
	// not "web_search" (the latter is only a config YAML key, see
	// config.ToolsWebSearchConfig `yaml:"web_search"`). The pre-Phase-3 map
	// had a "web_search" entry that never matched the registered tool name
	// and always fell through to DefaultToolPermission (RoleUser) — same
	// effective role, but the entry was silently dead. Fixed here to key on
	// the actual registered name; see implementation-notes.md.
	"weather":       domain.RoleUser,
	"websearch":     domain.RoleUser,
	"notes":         domain.RoleUser,
	"repo_search":   domain.RoleUser,
	"doc_summarize": domain.RoleUser,
	"summarize":     domain.RoleUser,

	// Side-effecting / high-privilege tools — admin only by default. This is
	// an intentional, security-motivated breaking change (Phase 3 / Part D):
	// these tools previously fell through to the permissive
	// DefaultToolPermission (RoleUser). Deployments needing the old behavior
	// must set an explicit `tools.permissions` override.
	//
	// "github" is admin-only as a ceiling for its write actions; its
	// read-only actions are downgraded to RoleUser by service.go's
	// action-aware check. "coding_agent" runs an external coding agent with
	// shell access and stays admin-only unconditionally (no read-only
	// actions). "executor" has no tool currently registered under this exact
	// name (internal/usecase/tool/executor.ExecutorService is a shared
	// dependency used *by* github/repo_search/summarize/coding_agent, not a
	// domain.Tool itself) — the entry is kept per the spec/PRD's explicit
	// RBAC table so a future tool literally named "executor" inherits an
	// admin-only default rather than silently falling through.
	"github":       domain.RoleAdmin,
	"coding_agent": domain.RoleAdmin,
	"executor":     domain.RoleAdmin,
	"admin.user":   domain.RoleAdmin,
}

// DefaultToolPermission is the role required for any tool NOT explicitly
// listed in ToolPermissions and not caught by resolveRequiredRole's other
// precedence steps (github's action-aware split, the "mcp:*" trust-based
// branch). permissions_test.go's CI guard ensures every tool actually
// registered by the running application (built-in and
// developer-productivity skills) has an explicit entry above, so in
// practice this fallback is reached only for tool names outside both that
// guard's coverage and the "mcp:" prefix — kept as domain.RoleUser (not
// Guest) as the safer of the two non-admin defaults for that residual case.
const DefaultToolPermission = domain.RoleUser
