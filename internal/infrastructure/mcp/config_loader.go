package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// Trust-level values for an MCP server (and, via ToolOverrides, an individual
// MCP tool)'s capacity for side effects (Phase 6 / Part F, FR-022). These are
// plain strings — not a shared Go type imported across layers — matching the
// established pattern elsewhere in this codebase (e.g.
// security.ValidationAction's "annotate"/"reject" config strings) of keeping
// internal/infrastructure and internal/usecase decoupled at the type level;
// internal/adapter/mcp.MCPToolAdapter.TrustLevel() and
// internal/usecase/tool's mcp:<server>:<tool> RBAC/confirmation resolution
// both consume/compare these exact same literal values.
const (
	// TrustReadOnly means the server's tools (or, per ToolOverrides, a
	// specific tool) never cause side effects — permission-checked as
	// RoleUser and never auto-added to the confirmation-required set.
	TrustReadOnly = "read_only"
	// TrustWrite means the tool can cause side effects — permission-checked
	// as RoleAdmin-equivalent and auto-added to the confirmation-required set.
	TrustWrite = "write"
	// TrustUnknown is the default when "trust" is omitted from mcp.json (or
	// set to an unrecognized value — see normalizeTrustLevel) and is treated
	// identically to TrustWrite by the RBAC/confirmation layers: an
	// unclassified MCP tool is assumed capable of side effects until an
	// operator explicitly marks it "read_only".
	TrustUnknown = "unknown"
)

// MCPServerEntry describes a single MCP server in mcp.json.
type MCPServerEntry struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "http" | "stdio"
	URL       string            `json:"url"`       // required for http transport
	Command   string            `json:"command"`   // required for stdio transport
	Args      []string          `json:"args"`      // optional args for stdio transport
	Headers   map[string]string `json:"headers"`   // optional extra headers for http transport

	// Trust classifies this server's tools' capacity for side effects
	// (Phase 6 / Part F, FR-022): TrustReadOnly | TrustWrite | TrustUnknown.
	// Omitted, empty, or any unrecognized value normalizes to TrustUnknown
	// during LoadMCPConfig — a config typo here fails closed to the more
	// restrictive classification rather than aborting startup or silently
	// granting a looser default. See ResolvedToolTrust.
	Trust string `json:"trust"`
	// ToolOverrides maps a specific tool name (as reported by the MCP server
	// itself via tools/list, NOT the bridged "mcp:<server>:<tool>" name) to a
	// trust-level override for that tool alone, taking precedence over
	// Trust. Same normalization/fail-closed rules as Trust apply per entry.
	//
	// Known limitation (FR-R13): because the lookup key is the server's own
	// self-reported tool name rather than a stable, server-independent
	// identifier, a malicious or misconfigured MCP server could register a
	// write-capable tool under whatever name an operator has configured a
	// "read_only" override for, and thereby inherit that looser override.
	// This gap is narrow and confined to ToolOverrides — the server-wide
	// Trust default is unaffected, since it applies uniformly to every tool
	// the server reports regardless of name. Only use ToolOverrides against
	// MCP servers you fully trust to name their tools honestly.
	ToolOverrides map[string]string `json:"tool_overrides"`
}

// normalizeTrustLevel validates and normalizes a raw trust-level string from
// mcp.json (case-insensitive, whitespace-trimmed) to one of
// TrustReadOnly/TrustWrite/TrustUnknown. An empty string is treated as
// "not configured" and normalizes silently to TrustUnknown; any other
// unrecognized value ALSO normalizes to TrustUnknown but is logged as a
// warning (via context, identifying which server/tool_overrides entry was
// affected), since it most likely indicates an operator typo they'd want to
// know about. Either way, this never returns an error — a malformed trust
// value must fail closed to the most restrictive classification, not abort
// startup (Phase 6's edge-case requirement).
func normalizeTrustLevel(context, raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return TrustUnknown
	case TrustReadOnly:
		return TrustReadOnly
	case TrustWrite:
		return TrustWrite
	case TrustUnknown:
		return TrustUnknown
	default:
		slog.Warn("mcp: unrecognized trust value, failing closed to 'unknown'",
			"context", context, "value", raw)
		return TrustUnknown
	}
}

// normalizeTrustFields normalizes entry.Trust and every entry.ToolOverrides
// value in place via normalizeTrustLevel.
func normalizeTrustFields(entry *MCPServerEntry) {
	entry.Trust = normalizeTrustLevel(fmt.Sprintf("server %q trust", entry.Name), entry.Trust)
	if len(entry.ToolOverrides) == 0 {
		return
	}
	normalized := make(map[string]string, len(entry.ToolOverrides))
	for toolName, raw := range entry.ToolOverrides {
		normalized[toolName] = normalizeTrustLevel(
			fmt.Sprintf("server %q tool_overrides[%q]", entry.Name, toolName), raw)
	}
	entry.ToolOverrides = normalized
}

// ResolvedToolTrust returns the effective, normalized trust level
// (TrustReadOnly | TrustWrite | TrustUnknown) for a specific tool bridged
// from this server entry: entry.ToolOverrides[toolName] if present (a
// per-tool exception), otherwise entry.Trust (the server-wide default). See
// internal/adapter/mcp's connectAndRegisterServer, which feeds this directly
// into MCPToolAdapter.TrustLevel() at bridge-construction time (P6.2).
//
// Re-normalizes the resolved value via normalizeTrustLevel rather than
// assuming entry was already passed through LoadMCPConfig's own
// normalization pass — this is a no-op for a config-loaded entry (whose
// fields are already normalized), but keeps this function safe/fail-closed
// for any entry constructed directly (e.g. in tests, or a future caller that
// builds an MCPConfig some other way).
//
// Known limitation (FR-R13): the entry.ToolOverrides[toolName] lookup below
// is keyed on toolName as self-reported by the MCP server via tools/list —
// not any stable, server-independent identifier. A malicious or
// misconfigured MCP server could therefore register a genuinely
// write-capable tool under a name an operator has configured a "read_only"
// override for, and inherit that override. This is a narrow,
// acceptable-to-document limitation of per-tool overrides: it requires the
// server itself to be dishonest about tool naming, and the server-wide
// entry.Trust default (the fallback path below, used when no per-tool
// override exists) is unaffected — it applies uniformly regardless of tool
// name and is purely operator-controlled. Operators should only configure
// ToolOverrides against MCP servers they fully trust to name their tools
// honestly.
func ResolvedToolTrust(entry MCPServerEntry, toolName string) string {
	if override, ok := entry.ToolOverrides[toolName]; ok {
		return normalizeTrustLevel(fmt.Sprintf("server %q tool_overrides[%q]", entry.Name, toolName), override)
	}
	return normalizeTrustLevel(fmt.Sprintf("server %q trust", entry.Name), entry.Trust)
}

// MCPConfig represents the parsed contents of an mcp.json file.
type MCPConfig struct {
	Servers []MCPServerEntry `json:"servers"`
}

// envVarPattern matches ${VAR_NAME} placeholders.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// substituteEnv replaces all ${VAR_NAME} occurrences in s with the corresponding
// environment variable value. If the variable is not set, it is replaced with an empty string.
func substituteEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract var name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		return os.Getenv(varName)
	})
}

// applyEnvSubstitution applies environment variable substitution to all string
// fields in a server entry.
func applyEnvSubstitution(entry *MCPServerEntry) {
	entry.Name = substituteEnv(entry.Name)
	entry.URL = substituteEnv(entry.URL)
	entry.Command = substituteEnv(entry.Command)
	for i, arg := range entry.Args {
		entry.Args[i] = substituteEnv(arg)
	}
	for k, v := range entry.Headers {
		entry.Headers[k] = substituteEnv(v)
	}
}

// validateServerEntry checks that a server entry has all required fields.
func validateServerEntry(entry MCPServerEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("mcp: server entry missing required field 'name'")
	}
	if entry.Transport == "" {
		return fmt.Errorf("mcp: server %q missing required field 'transport'", entry.Name)
	}
	switch entry.Transport {
	case "http":
		if entry.URL == "" {
			return fmt.Errorf("mcp: server %q (http transport) missing required field 'url'", entry.Name)
		}
	case "stdio":
		if entry.Command == "" {
			return fmt.Errorf("mcp: server %q (stdio transport) missing required field 'command'", entry.Name)
		}
	default:
		return fmt.Errorf("mcp: server %q has unknown transport %q (must be 'http' or 'stdio')", entry.Name, entry.Transport)
	}
	return nil
}

// LoadMCPConfig reads and parses an mcp.json file at the given path.
// Environment variable substitution (${VAR_NAME}) is applied to all string fields.
// Returns an error if the file cannot be read, parsed, or contains invalid entries.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp: load config: %w", err)
	}

	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mcp: parse config: %w", err)
	}

	for i := range cfg.Servers {
		applyEnvSubstitution(&cfg.Servers[i])
		normalizeTrustFields(&cfg.Servers[i])
		if err := validateServerEntry(cfg.Servers[i]); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}
