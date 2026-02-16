package persona

import (
	"context"
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

// RulesEnforcer enforces RULES.md hard rules.
type RulesEnforcer struct {
	repo        domain.PersonaFileRepository
	parser      FrontmatterParser
	adminPolicy *domain.RulesConfig // Optional admin policy that overrides user rules
}

// NewRulesEnforcer creates a new RulesEnforcer.
func NewRulesEnforcer(repo domain.PersonaFileRepository, parser FrontmatterParser, adminPolicy *domain.RulesConfig) *RulesEnforcer {
	return &RulesEnforcer{
		repo:        repo,
		parser:      parser,
		adminPolicy: adminPolicy,
	}
}

// Enforce checks if an action/tool is allowed according to rules.
func (e *RulesEnforcer) Enforce(ctx context.Context, input EnforcerInput) (*EnforcerOutput, error) {
	if input.UserID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	// Load user's RULES.md file
	var userRules *domain.RulesConfig
	rulesFile, err := e.repo.Get(ctx, input.UserID, domain.PersonaFileRULES)
	if err != nil {
		if err == domain.ErrPersonaFileNotFound {
			// No RULES file - use empty config
			userRules = &domain.RulesConfig{}
		} else {
			return nil, fmt.Errorf("failed to load RULES.md: %w", err)
		}
	} else {
		// Parse frontmatter
		config, _, parseErr := e.parser.ParseMarkdownWithFrontmatter(rulesFile.Content)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse RULES.md: %w", parseErr)
		}
		userRules = &config
	}

	// Apply admin policy overrides if present
	effectiveRules := userRules
	if e.adminPolicy != nil {
		effectiveRules = e.mergeWithAdminPolicy(userRules)
	}

	// Check blocked tools first (highest priority)
	if input.Tool != "" && effectiveRules.IsToolBlocked(input.Tool) {
		return &EnforcerOutput{
			Allowed:              false,
			RequiresConfirmation: false,
			Reason:               fmt.Sprintf("Tool %q is blocked by rules", input.Tool),
		}, nil
	}

	// Check if action requires confirmation
	if input.Action != "" && effectiveRules.RequiresConfirmationFor(input.Action) {
		return &EnforcerOutput{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               fmt.Sprintf("Action %q requires user confirmation", input.Action),
		}, nil
	}

	// Tool also check for requires confirmation
	if input.Tool != "" && effectiveRules.RequiresConfirmationFor(input.Tool) {
		return &EnforcerOutput{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               fmt.Sprintf("Tool %q requires user confirmation", input.Tool),
		}, nil
	}

	// Default: allowed without confirmation
	return &EnforcerOutput{
		Allowed:              true,
		RequiresConfirmation: false,
	}, nil
}

// mergeWithAdminPolicy merges admin policy with user rules.
// Admin blocked tools are always enforced.
func (e *RulesEnforcer) mergeWithAdminPolicy(userRules *domain.RulesConfig) *domain.RulesConfig {
	merged := *userRules

	// Merge blocked tools (admin + user)
	blockedMap := make(map[string]bool)
	for _, tool := range e.adminPolicy.BlockedTools {
		blockedMap[tool] = true
	}
	for _, tool := range userRules.BlockedTools {
		blockedMap[tool] = true
	}

	var blocked []string
	for tool := range blockedMap {
		blocked = append(blocked, tool)
	}
	merged.BlockedTools = blocked

	// Merge requires confirmation (admin + user)
	confirmMap := make(map[string]bool)
	for _, action := range e.adminPolicy.RequiresConfirmation {
		confirmMap[action] = true
	}
	for _, action := range userRules.RequiresConfirmation {
		confirmMap[action] = true
	}

	var confirm []string
	for action := range confirmMap {
		confirm = append(confirm, action)
	}
	merged.RequiresConfirmation = confirm

	return &merged
}
