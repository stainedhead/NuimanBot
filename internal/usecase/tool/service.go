package tool

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/ratelimit"
)

// Service implements the ToolExecutionService.
type Service struct {
	cfg         *config.ToolsSystemConfig
	registry    ToolRegistry
	securitySvc domain.SecurityService // Use domain.SecurityService
	rateLimiter *ratelimit.RateLimiter // Optional rate limiter
	// timeout      time.Duration // Default timeout for tool execution
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
// This method does not perform permission checks - use ExecuteWithUser for RBAC.
func (s *Service) Execute(ctx context.Context, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
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
		// Audit failure
		if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
			Timestamp: time.Now(),
			Action:    fmt.Sprintf("tool_execute:%s", toolName),
			Resource:  toolName,
			Outcome:   "failure",
			Details:   map[string]any{"params": params, "error": err.Error()},
		}); auditErr != nil {
			slog.Error("Error auditing tool execution failure", "error", auditErr)
		}
		return nil, fmt.Errorf("failed to execute tool '%s': %w", toolName, err)
	}

	// Audit success
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    fmt.Sprintf("tool_execute:%s", toolName),
		Resource:  toolName,
		Outcome:   "success",
		Details:   map[string]any{"params": params, "output_summary": result.Output},
	}); auditErr != nil {
		slog.Error("Error auditing tool execution success", "error", auditErr)
	}

	return result, nil
}

// SetRateLimiter sets the rate limiter for tool execution.
// This is optional - if not set, no rate limiting is applied.
func (s *Service) SetRateLimiter(limiter *ratelimit.RateLimiter) {
	s.rateLimiter = limiter
}

// ExecuteWithUser runs a registered tool with given parameters after checking permissions and rate limits.
// This method enforces RBAC based on the user's role and AllowedTools whitelist.
func (s *Service) ExecuteWithUser(ctx context.Context, user *domain.User, toolName string, params map[string]any) (*domain.ExecutionResult, error) {
	// Check permissions first
	if err := s.checkPermission(user, toolName); err != nil {
		// Audit permission denial for security monitoring
		s.auditPermissionDenial(ctx, user, toolName, err)
		return nil, err
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

	// Permission check and rate limit passed, execute the tool
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

// checkPermission checks if a user has permission to execute a tool.
// Permission is granted if:
//  1. The user's role meets or exceeds the required role for the tool
//  2. If the user has an AllowedTools whitelist, the tool must be in it
func (s *Service) checkPermission(user *domain.User, toolName string) error {
	// Get required role for this tool (default to RoleUser if not specified)
	requiredRole := DefaultToolPermission
	if role, ok := ToolPermissions[toolName]; ok {
		requiredRole = role
	}

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

// isToolWhitelisted checks if a tool is in the user's AllowedTools whitelist.
func (s *Service) isToolWhitelisted(toolName string, allowedTools []string) bool {
	for _, allowed := range allowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// ListTools returns all registered tools for a given user.
func (s *Service) ListTools(ctx context.Context, userID string) ([]domain.Tool, error) {
	// TODO: Implement user-specific tool filtering using registry.ListForUser
	return s.registry.List(), nil
}
