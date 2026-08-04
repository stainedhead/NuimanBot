package tool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool/github"
)

// githubToolName is the registered tool name for
// internal/usecase/tool/github.GitHubSkill (see its Name() method). Used to
// scope the action-aware permission check in resolveRequiredRole to the
// github tool specifically.
const githubToolName = "github"

// RulesEnforcer defines the interface for enforcing RULES.md restrictions.
type RulesEnforcer interface {
	Enforce(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error)
}

// EnforcerInput represents input for rule enforcement.
type EnforcerInput struct {
	UserID string
	Action string
	Tool   string
}

// EnforcerOutput represents enforcement result.
type EnforcerOutput struct {
	Allowed              bool
	RequiresConfirmation bool
	Reason               string
}

// Service implements the ToolExecutionService.
type Service struct {
	cfg           *config.ToolsSystemConfig
	registry      ToolRegistry
	securitySvc   domain.SecurityService // Use domain.SecurityService
	rateLimiter   *ratelimit.RateLimiter // Optional rate limiter
	rulesEnforcer RulesEnforcer          // Optional persona rules enforcer
	// timeout      time.Duration // Default timeout for tool execution

	// confirmationStore and confirmationCfg back Part C's side-effecting
	// action confirmation gate (specs/260802-improve-nuimanbot-security,
	// FR-009..FR-015). Both are optional: a nil confirmationStore means any
	// tool/action that would otherwise require confirmation fails closed
	// (denied outright) rather than executing unconfirmed — see
	// createPendingConfirmation.
	confirmationStore security.ConfirmationStore
	confirmationCfg   config.ConfirmationConfig
}

// NewService creates a new ToolExecutionService instance.
func NewService(cfg *config.ToolsSystemConfig, registry ToolRegistry, securitySvc domain.SecurityService) *Service {
	// TODO: Load default timeout from config
	return &Service{
		cfg:         cfg,
		registry:    registry,
		securitySvc: securitySvc,
		// timeout:      time.Duration(cfg.DefaultToolTimeoutSeconds) * time.Second,
	}
}

// Execute runs a registered tool with given parameters.
// This method does not perform RBAC/permission checks - use ExecuteWithUser
// for that. It DOES, however, apply Part C's confirmation gate
// (specs/260802-improve-nuimanbot-security, FR-010/FR-012) when ctx carries a
// security.ConfirmationIdentity (see security.WithConfirmationIdentity) —
// this lets callers that only have a userID/conversationID and not a full
// RBAC-capable *domain.User (chiefly ChatService.ProcessMessage's
// tool-calling loop) still benefit from confirmation gating without the
// separately-scoped, larger change of resolving a *domain.User in that hot
// path. See implementation-notes.md's "Known Gap" note for the RBAC
// implication of this split.
func (s *Service) Execute(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	if identity, ok := security.ConfirmationIdentityFromContext(ctx); ok {
		pending, err := s.enforceRulesAndConfirmation(ctx, identity.UserID, identity.ConversationID, toolName, params)
		if err != nil {
			return nil, err
		}
		if pending != nil {
			return pending, nil
		}
	}

	tool, err := s.registry.Get(toolName)
	if err != nil {
		return nil, fmt.Errorf("tool '%s' not found: %w", toolName, err)
	}

	// TODO: Implement timeout logic for tool execution (from config).
	// Currently, the tool's own context will manage its timeout.

	// Audit the tool execution
	if err := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    fmt.Sprintf("tool_execute:%s", toolName),
		Resource:  toolName,
		Outcome:   "attempt",
		Details:   map[string]any{"params": params},
	}); err != nil {
		slog.Error("Error auditing tool execution attempt", "error", err)
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		// Audit failure. When the tool failed closed because OutputValidator
		// flagged its fetched/returned content as a likely prompt injection
		// (the reject action), surface injection_flagged/matched_patterns
		// alongside the error for security monitoring.
		details := map[string]any{"params": params, "error": err.Error()}
		addInjectionAuditFieldsFromError(details, err)
		if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
			Timestamp: time.Now(),
			Action:    fmt.Sprintf("tool_execute:%s", toolName),
			Resource:  toolName,
			Outcome:   "failure",
			Details:   details,
		}); auditErr != nil {
			slog.Error("Error auditing tool execution failure", "error", auditErr)
		}
		return nil, fmt.Errorf("failed to execute tool '%s': %w", toolName, err)
	}

	// Audit success. When the tool's result carries injection_flagged metadata
	// (set when OutputValidator flagged content but the configured action is
	// annotate rather than reject), surface it alongside the output summary.
	details := map[string]any{"params": params, "output_summary": result.Output}
	addInjectionAuditFieldsFromMetadata(details, result.Metadata)
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    fmt.Sprintf("tool_execute:%s", toolName),
		Resource:  toolName,
		Outcome:   "success",
		Details:   details,
	}); auditErr != nil {
		slog.Error("Error auditing tool execution success", "error", auditErr)
	}

	return result, nil
}

// addInjectionAuditFieldsFromMetadata copies injection_flagged/matched_patterns
// into details when a successful ExecutionResult's Metadata reports flagged
// content (see internal/usecase/security.OutputValidator). It is a no-op when
// Metadata is nil or does not report a flag.
func addInjectionAuditFieldsFromMetadata(details map[string]any, metadata map[string]any) {
	if metadata == nil {
		return
	}
	flagged, ok := metadata["injection_flagged"].(bool)
	if !ok || !flagged {
		return
	}
	details["injection_flagged"] = true
	if patterns, ok := metadata["matched_patterns"].([]string); ok {
		details["matched_patterns"] = patterns
	}
}

// addInjectionAuditFieldsFromError copies injection_flagged/matched_patterns into
// details when err wraps a *security.FlaggedOutputError — the error a tool
// returns when OutputValidator's configured action is reject (fail closed). It
// is a no-op for any other error.
func addInjectionAuditFieldsFromError(details map[string]any, err error) {
	var flaggedErr *security.FlaggedOutputError
	if !errors.As(err, &flaggedErr) {
		return
	}
	details["injection_flagged"] = true
	details["matched_patterns"] = flaggedErr.MatchedPatterns
}

// SetRateLimiter sets the rate limiter for tool execution.
// This is optional - if not set, no rate limiting is applied.
func (s *Service) SetRateLimiter(limiter *ratelimit.RateLimiter) {
	s.rateLimiter = limiter
}

// SetRulesEnforcer sets the persona rules enforcer for tool execution.
// This is optional - if not set, no persona rule enforcement is applied.
func (s *Service) SetRulesEnforcer(enforcer RulesEnforcer) {
	s.rulesEnforcer = enforcer
}

// SetConfirmationStore sets the ConfirmationStore backing Part C's
// side-effecting-action confirmation gate. This is optional, but when a
// tool/action requires confirmation (per RulesEnforcer and/or
// SetConfirmationConfig) and no store is configured, the call fails closed
// (denied) rather than executing unconfirmed — see createPendingConfirmation.
func (s *Service) SetConfirmationStore(store security.ConfirmationStore) {
	s.confirmationStore = store
}

// SetConfirmationConfig sets the security.confirmation config (FR-012,
// FR-015) used to union deployment-wide default_required_actions with
// whatever the per-user RulesEnforcer already returns, and to resolve each
// new confirmation's TTL. This is optional; the zero value disables the
// config-level default list (RulesEnforcer-driven confirmation still works).
func (s *Service) SetConfirmationConfig(cfg config.ConfirmationConfig) {
	s.confirmationCfg = cfg
}

// ExecuteWithUser runs a registered tool with given parameters after checking
// permissions and rate limits. This method enforces RBAC based on the user's
// role and AllowedTools whitelist, and persona rules (RULES.md).
// conversationID scopes Part C's confirmation gate (FR-014's "at most one
// open confirmation per conversation" invariant) and may be empty for
// callers with no conversation context.
func (s *Service) ExecuteWithUser(ctx context.Context, user *domain.User, conversationID, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	// Check permissions first
	if err := s.checkPermission(user, toolName, params); err != nil {
		// Audit permission denial for security monitoring
		s.auditPermissionDenial(ctx, user, toolName, err)
		return nil, err
	}

	pending, err := s.enforceRulesAndConfirmation(ctx, user.ID, conversationID, toolName, params)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		return pending, nil
	}

	// Check rate limit if limiter is configured
	if s.rateLimiter != nil && !s.rateLimiter.Allow(user.ID, toolName) {
		// Audit rate limit exceeded
		if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
			Timestamp: time.Now(),
			Action:    "tool_rate_limit_exceeded",
			Resource:  toolName,
			Outcome:   "denied",
			Details: map[string]any{
				"user_id":   user.ID,
				"tool_name": toolName,
			},
		}); auditErr != nil {
			slog.Error("Error auditing rate limit denial", "error", auditErr)
		}
		return nil, domain.ErrRateLimitExceeded
	}

	// Permission check, rules enforcement, and rate limit passed - execute the tool
	return s.Execute(ctx, toolName, params)
}

// auditPermissionDenial logs a permission denial event for security monitoring.
func (s *Service) auditPermissionDenial(ctx context.Context, user *domain.User, toolName string, err error) {
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    "tool_execution_denied",
		Resource:  toolName,
		Outcome:   "denied",
		Details: map[string]any{
			"user_id":   user.ID,
			"user_role": string(user.Role),
			"reason":    err.Error(),
		},
	}); auditErr != nil {
		slog.Error("Error auditing permission denial", "error", auditErr)
	}
}

// enforceRulesAndConfirmation applies persona-rules enforcement
// (RulesEnforcer) and Part C's confirmation gate for a userID/conversationID/
// toolName/params combination (specs/260802-improve-nuimanbot-security,
// FR-009..FR-015). It is the single code path shared by ExecuteWithUser
// (which always has a userID, having already checked RBAC) and Execute
// (which only reaches this when ctx carries a security.ConfirmationIdentity —
// see security.WithConfirmationIdentity).
//
// Return values:
//   - (nil, nil): proceed to execute the tool normally.
//   - (a non-nil *domain.ExecutionResult with Status ==
//     domain.StatusPendingConfirmation, nil): the call is now paused pending
//     confirmation; the caller must return this result directly rather than
//     executing the tool.
//   - (nil, err): the tool is blocked by rules, RulesEnforcer errored, or a
//     required confirmation could not be safely created — fail closed in
//     every case (never proceed to execute).
func (s *Service) enforceRulesAndConfirmation(ctx context.Context, userID, conversationID, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	action, _ := params["action"].(string)
	requiresConfirmation := s.confirmationCfg.RequiresConfirmationByDefault(toolName, action)
	reason := ""
	if requiresConfirmation {
		reason = fmt.Sprintf("action %q is configured to require confirmation by default", confirmationActionKey(toolName, action))
	}

	// Phase 6 / Part F, FR-023: a write/unknown-trust MCP tool is added to
	// the effective confirmation-required set automatically, the same way a
	// config.ConfirmationConfig.DefaultRequiredActions entry would be —
	// gated on the confirmation subsystem itself being enabled, consistent
	// with RequiresConfirmationByDefault's own IsEnabled() check (a disabled
	// subsystem must never be silently bypassed by a different code path
	// that still creates pending confirmations).
	if !requiresConfirmation && s.confirmationCfg.IsEnabled() && s.requiresConfirmationForMCPTrust(toolName) {
		requiresConfirmation = true
		reason = fmt.Sprintf("MCP tool %q has a non-read_only trust classification and requires confirmation before executing", toolName)
	}

	if s.rulesEnforcer != nil {
		output, err := s.rulesEnforcer.Enforce(ctx, EnforcerInput{
			UserID: userID,
			Tool:   toolName,
		})
		if err != nil {
			s.auditRulesOutcome(ctx, "tool_rules_error", "error", userID, toolName, err.Error())
			return nil, fmt.Errorf("rules enforcement failed: %w", err)
		}

		if !output.Allowed {
			s.auditRulesOutcome(ctx, "tool_rules_denied", "denied", userID, toolName, output.Reason)
			return nil, fmt.Errorf("tool blocked by rules: %s", output.Reason)
		}

		if output.RequiresConfirmation {
			requiresConfirmation = true
			reason = output.Reason
		}
	}

	if !requiresConfirmation {
		return nil, nil
	}

	return s.createPendingConfirmation(ctx, userID, conversationID, toolName, action, params, reason)
}

// auditRulesOutcome logs a rules-enforcement audit event, matching the
// pre-existing "tool_rules_error"/"tool_rules_denied" audit shape.
func (s *Service) auditRulesOutcome(ctx context.Context, action, outcome, userID, toolName, reason string) {
	fieldName := "reason"
	if outcome == "error" {
		fieldName = "error"
	}
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    action,
		Resource:  toolName,
		Outcome:   outcome,
		Details: map[string]any{
			"user_id": userID,
			fieldName: reason,
		},
	}); auditErr != nil {
		slog.Error("Error auditing rules outcome", "action", action, "error", auditErr)
	}
}

// createPendingConfirmation records a new pending confirmation via
// s.confirmationStore and returns a StatusPendingConfirmation
// domain.ExecutionResult. Per PRD §8.3's fail-closed philosophy, an
// unconfigured store or a Create error (including
// security.ErrConfirmationAlreadyOpen) denies the call outright rather than
// allowing it to proceed unconfirmed.
func (s *Service) createPendingConfirmation(ctx context.Context, userID, conversationID, toolName, action string, params map[string]any, reason string) (*domain.ExecutionResult, error) {
	if s.confirmationStore == nil {
		s.auditConfirmationRequired(ctx, userID, toolName, reason, "")
		return nil, fmt.Errorf("tool requires user confirmation but no confirmation store is configured: %s", reason)
	}

	id, err := s.confirmationStore.Create(ctx, security.ConfirmationRequest{
		UserID:         userID,
		ConversationID: conversationID,
		ToolName:       toolName,
		Action:         action,
		Params:         params,
	})
	if err != nil {
		s.auditConfirmationRequired(ctx, userID, toolName, reason, "")
		if errors.Is(err, security.ErrConfirmationAlreadyOpen) {
			return nil, fmt.Errorf("a confirmation is already pending for this conversation; resolve it before requesting a new action: %w", err)
		}
		return nil, fmt.Errorf("failed to create confirmation: %w", err)
	}

	summary := describePendingAction(toolName, action, params)
	s.auditConfirmationRequired(ctx, userID, toolName, reason, id)

	return &domain.ExecutionResult{
		Status:         domain.StatusPendingConfirmation,
		ConfirmationID: id,
		Summary:        summary,
	}, nil
}

// auditConfirmationRequired logs the "tool_confirmation_required" audit
// event, matching the pre-existing shape plus a confirmation_id field once a
// confirmation was actually created.
func (s *Service) auditConfirmationRequired(ctx context.Context, userID, toolName, reason, confirmationID string) {
	details := map[string]any{
		"user_id": userID,
		"reason":  reason,
	}
	if confirmationID != "" {
		details["confirmation_id"] = confirmationID
	}
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    "tool_confirmation_required",
		Resource:  toolName,
		Outcome:   "pending",
		Details:   details,
	}); auditErr != nil {
		slog.Error("Error auditing confirmation requirement", "error", auditErr)
	}
}

// confirmationActionKey formats a tool/action pair as the
// "<toolName>.<action>" (or bare "<toolName>" when action is empty) key used
// by both config.ConfirmationConfig.RequiresConfirmationByDefault and
// describePendingAction's summary text.
func confirmationActionKey(toolName, action string) string {
	if action == "" {
		return toolName
	}
	return toolName + "." + action
}

// maxParamValueLen is the maximum number of characters of a single
// parameter's rendered value shown in a describePendingAction summary
// before it is truncated. This bounds the size of confirmation messages
// pushed to chat platforms/logs and avoids leaking large or sensitive
// payloads (e.g. a full file body passed as a tool parameter) into a
// human-readable summary. See FR-015.
const maxParamValueLen = 200

// truncateParamValue renders a parameter value and, if the rendered string
// exceeds maxParamValueLen, truncates it and appends a clear indicator of
// how many characters the full value contained.
func truncateParamValue(v any) string {
	s := fmt.Sprintf("%v", v)
	if len(s) <= maxParamValueLen {
		return s
	}
	return fmt.Sprintf("%s...[truncated, %d chars total]", s[:maxParamValueLen], len(s))
}

// describePendingAction builds a human-readable summary of a pending
// confirmation for display to the user (tool name, action if any, and up to
// 5 other key parameters, sorted for determinism). Kept generic — not
// per-tool-hardcoded — since Part C applies uniformly to any tool/action a
// deployment configures or a user's RULES.md flags. Each parameter's
// rendered value is truncated to maxParamValueLen characters (see FR-015)
// to bound the size of confirmation messages surfaced to chat platforms
// and logs.
func describePendingAction(toolName, action string, params map[string]any) string {
	var b strings.Builder
	b.WriteString("Confirm ")
	b.WriteString(confirmationActionKey(toolName, action))

	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "action" {
			continue // already reflected in confirmationActionKey
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	const maxParams = 5
	if len(keys) > 0 {
		b.WriteString(" (")
		for i, k := range keys {
			if i >= maxParams {
				b.WriteString(", ...")
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s=%s", k, truncateParamValue(params[k]))
		}
		b.WriteString(")")
	}

	b.WriteString("?")
	return b.String()
}

// checkPermission checks if a user has permission to execute a tool with the
// given parameters. Permission is granted if:
//  1. The user's role meets or exceeds the required role for the tool/action
//     (see resolveRequiredRole)
//  2. If the user has an AllowedTools whitelist, the tool must be in it
func (s *Service) checkPermission(user *domain.User, toolName string, params map[string]any) error {
	requiredRole := s.resolveRequiredRole(toolName, params)

	// Check if user's role is sufficient
	if !user.Role.HasPermission(requiredRole) {
		return domain.ErrInsufficientPermissions
	}

	// If AllowedTools whitelist is set, verify tool is whitelisted
	if len(user.AllowedTools) > 0 && !s.isToolWhitelisted(toolName, user.AllowedTools) {
		return domain.ErrInsufficientPermissions
	}

	return nil
}

// resolveRequiredRole determines the minimum role required to execute
// toolName with the given params, applying the following precedence:
//
//  1. A per-tool `tools.permissions` config override (s.cfg.Permissions), if
//     present and parseable — applies uniformly regardless of action, so an
//     operator can fully revert a tool (e.g. "github") to a looser default
//     without a code change (FR-018a). An unrecognized role string is
//     ignored (falls through to the next precedence level) rather than
//     silently granting or denying access.
//  2. github's action-aware split (githubActionRole): read-only actions
//     resolve to RoleUser even though ToolPermissions["github"] is
//     RoleAdmin; write actions and unrecognized actions fall through to
//     that admin-only entry (fail closed).
//  3. The dynamic "mcp:<server>:<tool>" namespace (Phase 6 / Part F,
//     FR-023): trust classification (mcpToolTrustLevel) resolved via a
//     registry lookup + TrustClassifiedTool type assertion rather than a
//     static map entry, since MCP tool names aren't known at compile time.
//     read_only trust resolves to RoleUser; write/unknown (including an
//     unresolvable trust) resolves to RoleAdmin (fail closed).
//  4. The static ToolPermissions map (internal/usecase/tool/permissions.go).
//  5. DefaultToolPermission, for any tool with no explicit entry.
func (s *Service) resolveRequiredRole(toolName string, params map[string]any) domain.Role {
	if s.cfg != nil {
		if roleStr, ok := s.cfg.Permissions[toolName]; ok {
			if role, ok := parseConfiguredRole(roleStr); ok {
				return role
			}
			slog.Warn("Ignoring unrecognized tools.permissions override",
				"tool", toolName, "value", roleStr)
		}
	}

	if toolName == githubToolName {
		if role, ok := githubActionRole(params); ok {
			return role
		}
	}

	if isMCPTool(toolName) {
		if s.mcpToolTrustLevel(toolName) == TrustReadOnly {
			return domain.RoleUser
		}
		return domain.RoleAdmin
	}

	if role, ok := ToolPermissions[toolName]; ok {
		return role
	}

	return DefaultToolPermission
}

// githubActionRole returns the role required for a specific "github" tool
// action: read-only actions (issue_list, issue_view, pr_list, pr_view,
// repo_view) resolve to RoleUser; write actions (issue_create,
// issue_comment, issue_close, pr_create, pr_review, pr_merge, workflow_run)
// resolve to RoleAdmin. ok is false when params doesn't carry a recognized
// "action" string, in which case the caller falls back to the static
// admin-only "github" ToolPermissions entry (fail closed on an
// unrecognized/missing action).
func githubActionRole(params map[string]any) (domain.Role, bool) {
	action, _ := params["action"].(string)
	switch action {
	case github.ActionIssueList, github.ActionIssueView, github.ActionPRList,
		github.ActionPRView, github.ActionRepoView:
		return domain.RoleUser, true
	case github.ActionIssueCreate, github.ActionIssueComment, github.ActionIssueClose,
		github.ActionPRCreate, github.ActionPRReview, github.ActionPRMerge, github.ActionWorkflowRun:
		return domain.RoleAdmin, true
	default:
		return "", false
	}
}

// parseConfiguredRole parses a `tools.permissions` config value ("guest",
// "user", or "admin", case-insensitive, surrounding whitespace trimmed) into
// a domain.Role. ok is false for any other value.
func parseConfiguredRole(s string) (domain.Role, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(domain.RoleGuest):
		return domain.RoleGuest, true
	case string(domain.RoleUser):
		return domain.RoleUser, true
	case string(domain.RoleAdmin):
		return domain.RoleAdmin, true
	default:
		return "", false
	}
}

// isToolWhitelisted checks if a tool is in the user's AllowedTools whitelist.
func (s *Service) isToolWhitelisted(toolName string, allowedTools []string) bool {
	for _, allowed := range allowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// ListTools returns the tools user is permitted to execute: the registry's
// tools for user.ID, filtered by the same role/whitelist rule checkPermission
// applies to Execute/ExecuteWithUser (FR-012, and
// specs/260803-improve-nuimanbot-security-auto-review FR-002) — a tool that
// would be denied by ExecuteWithUser is never listed as available in the
// first place. A nil user is treated as the lowest-privilege
// domain.RoleGuest identity (fail closed — mirrors
// chat.Service.resolveUser's fallback for an unresolvable platform identity)
// rather than returning the full unfiltered registry, as the pre-fix
// implementation did.
//
// NOTE: a tool call's params (and therefore, for "github", its action)
// aren't known at listing time, so action-aware tools are filtered using
// their params-less (most conservative / ceiling) required role — see
// resolveRequiredRole's githubActionRole branch, which falls through to the
// static admin-only "github" ToolPermissions entry when no action is
// resolvable. A RoleUser is therefore not offered "github" in ListTools even
// though ExecuteWithUser would let them call its read-only actions — an
// intentional, conservative simplification (see implementation-notes.md).
func (s *Service) ListTools(ctx context.Context, user *domain.User) ([]domain.Tool, error) {
	effectiveUser := user
	if effectiveUser == nil {
		effectiveUser = &domain.User{Role: domain.RoleGuest}
	}

	allTools, err := s.registry.ListForUser(ctx, effectiveUser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools for user '%s': %w", effectiveUser.ID, err)
	}

	allowed := make([]domain.Tool, 0, len(allTools))
	for _, t := range allTools {
		if s.checkPermission(effectiveUser, t.Name(), nil) == nil {
			allowed = append(allowed, t)
		}
	}
	return allowed, nil
}
