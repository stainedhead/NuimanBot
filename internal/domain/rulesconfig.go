package domain

import (
	"fmt"
	"regexp"
)

// identifierRegex matches valid identifiers: alphanumeric and underscore, non-empty.
var identifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// RulesConfig represents parsed YAML frontmatter from RULES.md.
type RulesConfig struct {
	// Actions requiring user confirmation
	RequiresConfirmation []string `yaml:"requires_confirmation"`

	// Blocked tools/actions (cannot execute)
	BlockedTools []string `yaml:"blocked_tools"`

	// Privacy settings
	Privacy PrivacyConfig `yaml:"privacy"`

	// Raw frontmatter (for validation)
	RawYAML string
}

// PrivacyConfig defines privacy-related rules.
type PrivacyConfig struct {
	// Data that should never be stored
	NeverStore []string `yaml:"never_store"`
}

// Validate checks if rules config is valid.
// All list entries must be valid identifiers (alphanumeric + underscore) with no duplicates.
func (r *RulesConfig) Validate() error {
	if err := validateIdentifierList(r.RequiresConfirmation, "requires_confirmation"); err != nil {
		return err
	}
	if err := validateIdentifierList(r.BlockedTools, "blocked_tools"); err != nil {
		return err
	}
	if err := validateIdentifierList(r.Privacy.NeverStore, "privacy.never_store"); err != nil {
		return err
	}
	return nil
}

// RequiresConfirmationFor checks if the given action requires user confirmation.
func (r *RulesConfig) RequiresConfirmationFor(action string) bool {
	for _, a := range r.RequiresConfirmation {
		if a == action {
			return true
		}
	}
	return false
}

// IsToolBlocked checks if the given tool is blocked from execution.
func (r *RulesConfig) IsToolBlocked(tool string) bool {
	for _, t := range r.BlockedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// validateIdentifierList checks that all entries in a list are valid identifiers
// (alphanumeric + underscore) and that there are no duplicates.
func validateIdentifierList(items []string, fieldName string) error {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !identifierRegex.MatchString(item) {
			return fmt.Errorf("%s: invalid identifier %q (must be alphanumeric and underscore only)", fieldName, item)
		}
		if seen[item] {
			return fmt.Errorf("%s: duplicate entry %q", fieldName, item)
		}
		seen[item] = true
	}
	return nil
}
