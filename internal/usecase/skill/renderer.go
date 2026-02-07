package skill

import (
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// SkillRenderer renders skills with argument substitution.
// It transforms skill templates into ready-to-use prompts by replacing
// placeholders with actual argument values.
type SkillRenderer interface {
	// Render renders a skill with arguments substituted into the body
	Render(skill *domain.Skill, args []string) (*domain.RenderedSkill, error)

	// SubstituteArguments replaces placeholders in the body with argument values
	SubstituteArguments(body string, args []string) string
}

// DefaultSkillRenderer is the default implementation of SkillRenderer.
type DefaultSkillRenderer struct{}

// NewDefaultSkillRenderer creates a new default skill renderer.
func NewDefaultSkillRenderer() *DefaultSkillRenderer {
	return &DefaultSkillRenderer{}
}

// Render renders a skill with arguments substituted into the body.
// Returns a RenderedSkill with the processed prompt and allowed tools.
func (r *DefaultSkillRenderer) Render(skill *domain.Skill, args []string) (*domain.RenderedSkill, error) {
	if skill == nil {
		return nil, fmt.Errorf("skill is nil")
	}

	prompt := r.SubstituteArguments(skill.BodyMD, args)

	return &domain.RenderedSkill{
		SkillName:    skill.Name,
		Prompt:       prompt,
		AllowedTools: skill.AllowedTools(),
	}, nil
}

// SubstituteArguments replaces placeholders in the body with argument values.
//
// Supported placeholders:
//   - $ARGUMENTS - replaced with full argument string (space-separated)
//   - $0, $1, $N - replaced with positional arguments (0-indexed)
//   - $$ - escaped dollar sign (replaced with literal $)
//
// Behavior:
//   - Positional arguments that don't exist are left as-is
//   - Unknown placeholders (like $abc) are left as-is
//   - Escape sequences ($$) are processed last to avoid conflicts
//   - Supports multi-digit indices ($10, $100, etc.)
func (r *DefaultSkillRenderer) SubstituteArguments(body string, args []string) string {
	result := body

	// Build full arguments string
	fullArgs := strings.Join(args, " ")

	// Replace $ARGUMENTS with full argument string
	result = strings.ReplaceAll(result, "$ARGUMENTS", fullArgs)

	// Replace positional arguments ($0, $1, etc.)
	// Process in reverse order to handle multi-digit indices correctly
	// (e.g., $10 before $1 to avoid partial replacement)
	for i := len(args) - 1; i >= 0; i-- {
		placeholder := fmt.Sprintf("$%d", i)
		result = strings.ReplaceAll(result, placeholder, args[i])
	}

	// Handle $$ escape (replace with single $)
	// Must be done AFTER other substitutions to avoid conflicts
	result = strings.ReplaceAll(result, "$$", "$")

	return result
}
