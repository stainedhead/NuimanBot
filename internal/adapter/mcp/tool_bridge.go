// Package mcp provides adapters that bridge MCP (Model Context Protocol) tools
// into NuimanBot's domain tool registry.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nuimanbot/internal/domain"
	infra "nuimanbot/internal/infrastructure/mcp"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool/common"
)

// defaultMCPToolTimeout is the maximum time an MCP tool call is allowed to run.
// If the MCP server does not respond within this duration, Execute cancels the
// call and returns a timeout error.  This prevents a single hanging MCP server
// from blocking all tool execution in the bot indefinitely.
const defaultMCPToolTimeout = 30 * time.Second

// AdapterOption is a functional option for MCPToolAdapter construction.
type AdapterOption func(*MCPToolAdapter)

// WithToolTimeout overrides the per-tool call timeout.  Use this in tests or
// when a specific tool is known to need a different timeout than the default.
func WithToolTimeout(d time.Duration) AdapterOption {
	return func(a *MCPToolAdapter) {
		a.timeout = d
	}
}

// WithOutputValidator overrides the OutputValidator used to scan MCP tool
// output for prompt-injection patterns. If never supplied, NewMCPToolAdapter's
// default (fail-closed reject) is used.
func WithOutputValidator(v security.OutputValidator) AdapterOption {
	return func(a *MCPToolAdapter) {
		a.validator = v
	}
}

// WithTrustLevel sets the MCP tool's resolved trust classification
// (infra.TrustReadOnly | infra.TrustWrite | infra.TrustUnknown; Phase 6 /
// Part F, FR-022/FR-023). Callers (connectAndRegisterServer) resolve this
// once per bridged tool via infra.ResolvedToolTrust — the server's mcp.json
// "trust" default, overridden by "tool_overrides" when the specific tool
// name has an entry — and pass it here at construction time. If never
// supplied, NewMCPToolAdapter defaults to infra.TrustUnknown, the same
// fail-closed default LoadMCPConfig itself applies for an omitted/invalid
// mcp.json trust value.
func WithTrustLevel(level string) AdapterOption {
	return func(a *MCPToolAdapter) {
		a.trust = level
	}
}

// MCPToolAdapter wraps an infra.MCPClient tool invocation and implements
// domain.Tool so MCP tools can participate in the standard tool registry.
type MCPToolAdapter struct {
	client     *infra.MCPClient
	toolDef    infra.MCPTool
	serverName string
	sanitizer  *common.OutputSanitizer
	validator  security.OutputValidator
	timeout    time.Duration
	trust      string
}

// NewMCPToolAdapter constructs an MCPToolAdapter for the given client and tool.
// serverName is used in the tool name prefix and error messages.
// Optional AdapterOptions (e.g. WithToolTimeout, WithOutputValidator) may be
// supplied to override defaults.
func NewMCPToolAdapter(client *infra.MCPClient, toolDef infra.MCPTool, serverName string, opts ...AdapterOption) *MCPToolAdapter {
	a := &MCPToolAdapter{
		client:     client,
		toolDef:    toolDef,
		serverName: serverName,
		sanitizer:  common.NewOutputSanitizer(),
		validator:  security.NewDefaultOutputValidator(),
		timeout:    defaultMCPToolTimeout,
		trust:      infra.TrustUnknown,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns the namespaced tool identifier: "mcp:<serverName>:<toolName>".
func (a *MCPToolAdapter) Name() string {
	return "mcp:" + a.serverName + ":" + a.toolDef.Name
}

// Description returns the MCP tool's description.
func (a *MCPToolAdapter) Description() string {
	return a.toolDef.Description
}

// InputSchema returns a JSON-object schema derived from the MCP tool's inputSchema.
// If the MCP tool has no inputSchema, an empty object schema is returned.
func (a *MCPToolAdapter) InputSchema() map[string]any {
	if a.toolDef.InputSchema == nil {
		return map[string]any{"type": "object"}
	}
	var schema map[string]any
	if err := json.Unmarshal(a.toolDef.InputSchema, &schema); err != nil {
		return map[string]any{"type": "object"}
	}
	return schema
}

// RequiredPermissions returns [domain.PermissionNetwork] because all MCP tool
// invocations involve a network call to the MCP server (ADR-5).
func (a *MCPToolAdapter) RequiredPermissions() []domain.Permission {
	return []domain.Permission{domain.PermissionNetwork}
}

// Config returns the tool's configuration.  MCP tools have no local config.
func (a *MCPToolAdapter) Config() domain.ToolConfig {
	return domain.ToolConfig{Enabled: true}
}

// TrustLevel returns the tool's resolved MCP trust classification
// (infra.TrustReadOnly | infra.TrustWrite | infra.TrustUnknown), set at
// construction time via WithTrustLevel (default infra.TrustUnknown).
// Implements internal/usecase/tool.TrustClassifiedTool so the
// permission/confirmation layer (internal/usecase/tool.Service) can resolve
// RBAC and confirmation requirements for the dynamic "mcp:<server>:<tool>"
// namespace via a registry lookup + type assertion, without a static
// internal/usecase/tool/permissions.go entry (Phase 6 / Part F, FR-023).
func (a *MCPToolAdapter) TrustLevel() string {
	return a.trust
}

// Execute calls the MCP tool via tools/call and returns the concatenated text
// content.  Output passes through two independent defenses before being
// returned: OutputSanitizer redacts secrets/credentials, and OutputValidator
// scans for prompt-injection patterns. Both run on ALL MCP tool output
// regardless of trust classification (Phase 6 / Part F, FR-024) — a.trust
// only affects RBAC (Service.resolveRequiredRole) and the confirmation-
// required set (Service.requiresConfirmationForMCPTrust) via
// TrustLevel()/TrustClassifiedTool; it never bypasses this content-level
// scanning, including for a read_only-trust tool, since trust describes a
// tool's own capacity for side effects, not whether third-party content it
// returns can carry an injection payload.
//
// Execute enforces a per-tool timeout (default defaultMCPToolTimeout, overridable
// via WithToolTimeout).  If the MCP server does not respond within that duration,
// Execute returns an error naming the server, tool, and elapsed timeout.
func (a *MCPToolAdapter) Execute(ctx context.Context, params map[string]any) (*domain.ExecutionResult, error) {
	toolCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	result, err := a.client.CallTool(toolCtx, a.toolDef.Name, params)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("mcp: %s: %s: tool call timed out after %s",
				a.serverName, a.toolDef.Name, a.timeout)
		}
		return nil, fmt.Errorf("mcp: %s: %s: %w", a.serverName, a.toolDef.Name, err)
	}

	if result.IsError {
		errText := concatenateTextContent(result.Content)
		return nil, fmt.Errorf("mcp: %s: %s: tool error: %s", a.serverName, a.toolDef.Name, errText)
	}

	output := a.sanitizer.SanitizeOutput(concatenateTextContent(result.Content))

	return a.validateOutput(ctx, output)
}

// validateOutput scans sanitized MCP tool output for prompt-injection patterns
// via OutputValidator. Clean output is returned unchanged. Flagged output is
// handled per the validator's configured action: annotate wraps it with a
// visible warning marker and returns it with injection_flagged metadata;
// reject (the default) fails the tool call closed with a
// *security.FlaggedOutputError. A nil validator disables scanning.
func (a *MCPToolAdapter) validateOutput(ctx context.Context, output string) (*domain.ExecutionResult, error) {
	if a.validator == nil {
		return &domain.ExecutionResult{Output: output}, nil
	}

	vr, err := a.validator.ValidateToolOutput(ctx, a.Name(), output)
	if err != nil {
		return nil, fmt.Errorf("mcp: %s: %s: output validation failed: %w", a.serverName, a.toolDef.Name, err)
	}
	if !vr.Flagged {
		return &domain.ExecutionResult{Output: output}, nil
	}

	if vr.Action == security.ValidationActionAnnotate {
		return &domain.ExecutionResult{
			Output: security.AnnotateFlaggedContent(output),
			Metadata: map[string]any{
				"injection_flagged": true,
				"matched_patterns":  vr.MatchedPatterns,
			},
		}, nil
	}

	// Fail closed (default reject action).
	return nil, fmt.Errorf("mcp: %s: %s: %w", a.serverName, a.toolDef.Name, &security.FlaggedOutputError{
		Source:          a.Name(),
		MatchedPatterns: vr.MatchedPatterns,
	})
}

// concatenateTextContent joins all text-typed content items with a newline separator.
func concatenateTextContent(contents []infra.MCPContent) string {
	var parts []string
	for _, c := range contents {
		if c.Type == "text" && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}
