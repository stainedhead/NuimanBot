package persona

import (
	"context"
	"errors"
	"fmt"

	"nuimanbot/internal/domain"
)

// FrontmatterParser defines the interface for parsing YAML frontmatter.
type FrontmatterParser interface {
	ParseMarkdownWithFrontmatter(content string) (domain.RulesConfig, string, error)
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

// RulesEnforcer enforces RULES.md hard rules against requested actions and tools.
// It loads the user's RULES.md, parses YAML frontmatter for blocked_tools and
// requires_confirmation lists, and merges with an optional admin policy that
// cannot be overridden by user rules.
type RulesEnforcer struct {
	repo        domain.PersonaFileRepository
	parser      FrontmatterParser
	adminPolicy *domain.RulesConfig
}

// NewRulesEnforcer creates a RulesEnforcer. adminPolicy may be nil if no
// admin-level overrides are needed.
func NewRulesEnforcer(repo domain.PersonaFileRepository, parser FrontmatterParser, adminPolicy *domain.RulesConfig) *RulesEnforcer {
	return &RulesEnforcer{
		repo:        repo,
		parser:      parser,
		adminPolicy: adminPolicy,
	}
}

// Enforce checks whether the given tool/action is allowed by the user's
// RULES.md and the admin policy.
//
// Precedence: blocked > requires_confirmation > allowed.
// Admin policy is merged with user rules and cannot be bypassed.
func (e *RulesEnforcer) Enforce(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
	if input.UserID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	userRules, err := e.loadUserRules(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	effective := userRules
	if e.adminPolicy != nil {
		effective = e.mergeWithAdminPolicy(userRules)
	}

	// Blocked tools take highest priority
	if input.Tool != "" && effective.IsToolBlocked(input.Tool) {
		return &EnforcerOutput{
			Allowed: false,
			Reason:  fmt.Sprintf("tool %q is blocked by rules", input.Tool),
		}, nil
	}

	// Check confirmation requirements for actions
	if input.Action != "" && effective.RequiresConfirmationFor(input.Action) {
		return &EnforcerOutput{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               fmt.Sprintf("action %q requires user confirmation", input.Action),
		}, nil
	}

	// Check confirmation requirements for tools
	if input.Tool != "" && effective.RequiresConfirmationFor(input.Tool) {
		return &EnforcerOutput{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               fmt.Sprintf("tool %q requires user confirmation", input.Tool),
		}, nil
	}

	return &EnforcerOutput{Allowed: true}, nil
}

// loadUserRules loads and parses the user's RULES.md. Returns an empty
// RulesConfig if the file does not exist (graceful degradation).
func (e *RulesEnforcer) loadUserRules(ctx context.Context, userID string) (*domain.RulesConfig, error) {
	file, err := e.repo.Get(ctx, userID, domain.PersonaFileRULES)
	if err != nil {
		if errors.Is(err, domain.ErrPersonaFileNotFound) {
			return &domain.RulesConfig{}, nil
		}
		return nil, fmt.Errorf("loading rules for user %q: %w", userID, err)
	}

	config, _, err := e.parser.ParseMarkdownWithFrontmatter(file.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing rules for user %q: %w", userID, err)
	}

	return &config, nil
}

// mergeWithAdminPolicy creates a new RulesConfig combining admin and user rules.
// Admin rules always apply and cannot be overridden by user configuration.
func (e *RulesEnforcer) mergeWithAdminPolicy(userRules *domain.RulesConfig) *domain.RulesConfig {
	merged := *userRules
	merged.BlockedTools = mergeUnique(e.adminPolicy.BlockedTools, userRules.BlockedTools)
	merged.RequiresConfirmation = mergeUnique(e.adminPolicy.RequiresConfirmation, userRules.RequiresConfirmation)
	return &merged
}

// mergeUnique combines two string slices, removing duplicates.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
