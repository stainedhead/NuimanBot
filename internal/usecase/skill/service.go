package skill

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// Service implements the SkillExecutionService.
type Service struct {
	cfg         *config.SkillsSystemConfig
	registry    SkillRegistry
	securitySvc domain.SecurityService // Use domain.SecurityService
	// timeout      time.Duration // Default timeout for skill execution
}

// NewService creates a new SkillExecutionService instance.
func NewService(cfg *config.SkillsSystemConfig, registry SkillRegistry, securitySvc domain.SecurityService) *Service {
	// TODO: Load default timeout from config
	return &Service{
		cfg:         cfg,
		registry:    registry,
		securitySvc: securitySvc,
		// timeout:      time.Duration(cfg.DefaultSkillTimeoutSeconds) * time.Second,
	}
}

// Execute runs a registered skill with given parameters.
// This method does not perform permission checks - use ExecuteWithUser for RBAC.
func (s *Service) Execute(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
	skill, err := s.registry.Get(skillName)
	if err != nil {
		return nil, fmt.Errorf("skill '%s' not found: %w", skillName, err)
	}

	// TODO: Implement timeout logic for skill execution (from config).
	// Currently, the skill's own context will manage its timeout.

	// Audit the skill execution
	if err := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    fmt.Sprintf("skill_execute:%s", skillName),
		Resource:  skillName,
		Outcome:   "attempt",
		Details:   map[string]any{"params": params},
	}); err != nil {
		slog.Error("Error auditing skill execution attempt", "error", err)
	}

	result, err := skill.Execute(ctx, params)
	if err != nil {
		// Audit failure
		if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
			Timestamp: time.Now(),
			Action:    fmt.Sprintf("skill_execute:%s", skillName),
			Resource:  skillName,
			Outcome:   "failure",
			Details:   map[string]any{"params": params, "error": err.Error()},
		}); auditErr != nil {
			slog.Error("Error auditing skill execution failure", "error", auditErr)
		}
		return nil, fmt.Errorf("failed to execute skill '%s': %w", skillName, err)
	}

	// Audit success
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    fmt.Sprintf("skill_execute:%s", skillName),
		Resource:  skillName,
		Outcome:   "success",
		Details:   map[string]any{"params": params, "output_summary": result.Output},
	}); auditErr != nil {
		slog.Error("Error auditing skill execution success", "error", auditErr)
	}

	return result, nil
}

// ExecuteWithUser runs a registered skill with given parameters after checking permissions.
// This method enforces RBAC based on the user's role and AllowedSkills whitelist.
func (s *Service) ExecuteWithUser(ctx context.Context, user *domain.User, skillName string, params map[string]any) (*domain.SkillResult, error) {
	// Check permissions first
	if err := s.checkPermission(user, skillName); err != nil {
		// Audit permission denial for security monitoring
		s.auditPermissionDenial(ctx, user, skillName, err)
		return nil, err
	}

	// Permission check passed, execute the skill
	return s.Execute(ctx, skillName, params)
}

// auditPermissionDenial logs a permission denial event for security monitoring.
func (s *Service) auditPermissionDenial(ctx context.Context, user *domain.User, skillName string, err error) {
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(),
		Action:    "skill_execution_denied",
		Resource:  skillName,
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

// checkPermission checks if a user has permission to execute a skill.
// Permission is granted if:
//  1. The user's role meets or exceeds the required role for the skill
//  2. If the user has an AllowedSkills whitelist, the skill must be in it
func (s *Service) checkPermission(user *domain.User, skillName string) error {
	// Get required role for this skill (default to RoleUser if not specified)
	requiredRole := DefaultSkillPermission
	if role, ok := SkillPermissions[skillName]; ok {
		requiredRole = role
	}

	// Check if user's role is sufficient
	if !user.Role.HasPermission(requiredRole) {
		return domain.ErrInsufficientPermissions
	}

	// If AllowedSkills whitelist is set, verify skill is whitelisted
	if len(user.AllowedSkills) > 0 && !s.isSkillWhitelisted(skillName, user.AllowedSkills) {
		return domain.ErrInsufficientPermissions
	}

	return nil
}

// isSkillWhitelisted checks if a skill is in the user's AllowedSkills whitelist.
func (s *Service) isSkillWhitelisted(skillName string, allowedSkills []string) bool {
	for _, allowed := range allowedSkills {
		if allowed == skillName {
			return true
		}
	}
	return false
}

// ListSkills returns all registered skills for a given user.
func (s *Service) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	// TODO: Implement user-specific skill filtering using registry.ListForUser
	return s.registry.List(), nil
}
