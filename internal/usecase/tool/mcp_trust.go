package tool

import "strings"

// MCP tool trust-level values (Phase 6 / Part F, FR-022/FR-023). These
// exactly mirror internal/infrastructure/mcp's TrustReadOnly/TrustWrite/
// TrustUnknown constants as plain strings — not a shared Go type imported
// across layers, matching the established pattern elsewhere in this codebase
// (e.g. security.ValidationAction's "annotate"/"reject" config strings) of
// keeping internal/infrastructure and internal/usecase decoupled at the type
// level. internal/adapter/mcp.MCPToolAdapter.TrustLevel() returns one of
// these same three literal values.
const (
	// TrustReadOnly means the tool never causes side effects — permission-
	// checked as RoleUser and never auto-added to the confirmation-required
	// set.
	TrustReadOnly = "read_only"
	// TrustWrite means the tool can cause side effects — permission-checked
	// as RoleAdmin-equivalent and auto-added to the confirmation-required
	// set.
	TrustWrite = "write"
	// TrustUnknown is the fail-closed default for an unclassified MCP tool
	// (or, defensively, any TrustLevel() value other than the three
	// recognized ones) — treated identically to TrustWrite by
	// resolveRequiredRole and requiresConfirmationForMCPTrust.
	TrustUnknown = "unknown"
)

// mcpToolNamePrefix identifies dynamically-registered MCP-bridged tool names
// (internal/adapter/mcp.MCPToolAdapter.Name(), "mcp:<server>:<tool>") for the
// prefix-match/dynamic-lookup path in resolveRequiredRole and
// enforceRulesAndConfirmation — distinct from the static ToolPermissions map
// lookup Phase 3 built, since MCP tool names aren't known at compile time and
// so can never have a static entry there.
const mcpToolNamePrefix = "mcp:"

// isMCPTool reports whether toolName is a dynamically-registered MCP-bridged
// tool name.
func isMCPTool(toolName string) bool {
	return strings.HasPrefix(toolName, mcpToolNamePrefix)
}

// TrustClassifiedTool is implemented by tools whose RBAC
// (resolveRequiredRole) and confirmation-requirement
// (enforceRulesAndConfirmation) treatment depends on a runtime trust
// classification rather than solely a static ToolPermissions entry —
// currently only internal/adapter/mcp.MCPToolAdapter. domain.Tool itself is
// NOT extended with this method (its interface is fixed by earlier phases'
// scope and used well beyond MCP tools); instead, Service performs an
// optional type assertion against whatever the registry returns for a given
// "mcp:<server>:<tool>" name.
type TrustClassifiedTool interface {
	// TrustLevel returns the tool's resolved MCP trust classification
	// (TrustReadOnly | TrustWrite | TrustUnknown). Any other value is
	// treated identically to TrustUnknown (fail closed) by callers here.
	TrustLevel() string
}

// mcpToolTrustLevel resolves toolName's trust classification by looking it
// up in the registry and type-asserting TrustClassifiedTool. Fails closed to
// TrustUnknown (treated as write-equivalent by callers) whenever the tool
// can't be found, the registry itself is nil, or the returned domain.Tool
// doesn't implement TrustClassifiedTool — this should only happen for a name
// that merely looks like an MCP tool name but isn't actually a registered,
// bridged tool (e.g. a stale/forged reference), never for a genuinely
// bridged tool, since internal/adapter/mcp.MCPToolAdapter always implements
// TrustClassifiedTool. A TrustLevel() return value other than the three
// recognized constants is likewise treated as TrustUnknown, defensively,
// since a permission check must never interpret an unrecognized string as
// "read only".
func (s *Service) mcpToolTrustLevel(toolName string) string {
	if s.registry == nil {
		return TrustUnknown
	}
	t, err := s.registry.Get(toolName)
	if err != nil {
		return TrustUnknown
	}
	tc, ok := t.(TrustClassifiedTool)
	if !ok {
		return TrustUnknown
	}
	switch tc.TrustLevel() {
	case TrustReadOnly:
		return TrustReadOnly
	case TrustWrite:
		return TrustWrite
	default:
		return TrustUnknown
	}
}

// requiresConfirmationForMCPTrust reports whether toolName is an MCP-bridged
// tool whose resolved trust classification is write/unknown (i.e. anything
// other than read_only) — Phase 6 / FR-023's automatic addition to the
// effective confirmation-required set, alongside
// config.ConfirmationConfig.DefaultRequiredActions and any RulesEnforcer
// output (see enforceRulesAndConfirmation).
func (s *Service) requiresConfirmationForMCPTrust(toolName string) bool {
	return isMCPTool(toolName) && s.mcpToolTrustLevel(toolName) != TrustReadOnly
}
