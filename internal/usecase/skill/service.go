package skill

import (
	"context"
	"fmt"
	"log"
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
func (s *Service) Execute(ctx context.Context, skillName string, params map[string]any) (*domain.SkillResult, error) {
	skill, err := s.registry.Get(skillName)
	if err != nil {
		return nil, fmt.Errorf("skill '%s' not found: %w", skillName, err)
	}

	// TODO: Implement permission checks using securitySvc.
	// For MVP, assume all skills are allowed or permissions are handled by caller.

	// TODO: Implement timeout logic for skill execution (from config).
	// Currently, the skill's own context will manage its timeout.

	// Audit the skill execution
	if err := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(), // Added timestamp
		Action:    fmt.Sprintf("skill_execute:%s", skillName),
		Resource:  skillName,
		Outcome:   "attempt", // Will be updated to success/failure later
		Details:   map[string]any{"params": params},
		// UserID, Platform, etc. would come from context
	}); err != nil {
		log.Printf("Error auditing skill execution attempt: %v", err)
	}

	result, err := skill.Execute(ctx, params)
	if err != nil {
		// Audit failure
		if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
			Timestamp: time.Now(), // Added timestamp
			Action:    fmt.Sprintf("skill_execute:%s", skillName),
			Resource:  skillName,
			Outcome:   "failure",
			Details:   map[string]any{"params": params, "error": err.Error()},
		}); auditErr != nil {
			log.Printf("Error auditing skill execution failure: %v", auditErr)
		}
		return nil, fmt.Errorf("failed to execute skill '%s': %w", skillName, err)
	}

	// Audit success
	if auditErr := s.securitySvc.Audit(ctx, &domain.AuditEvent{
		Timestamp: time.Now(), // Added timestamp
		Action:    fmt.Sprintf("skill_execute:%s", skillName),
		Resource:  skillName,
		Outcome:   "success",
		Details:   map[string]any{"params": params, "output_summary": result.Output},
	}); auditErr != nil {
		log.Printf("Error auditing skill execution success: %v", auditErr)
	}

	return result, nil
}

// ListSkills returns all registered skills for a given user.
func (s *Service) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	// TODO: Implement user-specific skill filtering using registry.ListForUser
	return s.registry.List(), nil
}
