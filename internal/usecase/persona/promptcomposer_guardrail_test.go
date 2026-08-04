package persona

import (
	"context"
	"strings"
	"testing"

	"nuimanbot/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompose_ContainsToolOutputGuardrail covers P2.2 (FR-007,
// specs/260802-improve-nuimanbot-security): PromptComposer.Compose() must
// prepend a fixed, non-overridable guardrail instructing the model to treat
// <tool_output> content as data, never as instructions.
func TestCompose_ContainsToolOutputGuardrail(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "Be safe and secure.")
	repo.addFile("user1", domain.PersonaFileSOUL, "You are cheerful.")
	repo.addFile("user1", domain.PersonaFileUSER, "Name: Alice.")

	pc := NewPromptComposer(repo, "Global: Always be respectful.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, toolOutputGuardrail)

	// The guardrail must be positioned ahead of the sectionOrder-driven
	// persona layers (RULES/SOUL/USER) so user-editable persona files can
	// never override it.
	guardrailIdx := strings.Index(out.SystemPrompt, toolOutputGuardrail)
	rulesIdx := strings.Index(out.SystemPrompt, "Be safe and secure.")
	require.NotEqual(t, -1, guardrailIdx)
	require.NotEqual(t, -1, rulesIdx)
	assert.Less(t, guardrailIdx, rulesIdx, "guardrail must precede RULES content")
}

// TestCompose_GuardrailContainsToolOutputTagReference asserts the guardrail
// wording actually references the <tool_output> delimiter tag introduced by
// P2.1, so the model has a concrete structural marker to key off of.
func TestCompose_GuardrailContainsToolOutputTagReference(t *testing.T) {
	t.Parallel()

	assert.Contains(t, toolOutputGuardrail, "<tool_output>")
	assert.Contains(t, toolOutputGuardrail, "not an instruction")
}

// TestCompose_GuardrailPresentWithNoPersonaFiles ensures the guardrail is
// unconditional: it appears even when the user has no persona files at all.
func TestCompose_GuardrailPresentWithNoPersonaFiles(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	pc := NewPromptComposer(repo, "")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "unknown-user"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, toolOutputGuardrail)
}

// TestCompose_GuardrailSurvivesPerFileTruncation covers P2.2's second
// acceptance criterion: the guardrail must survive when persona files are
// large enough to trigger per-file truncation (applyPerFileTruncation).
func TestCompose_GuardrailSurvivesPerFileTruncation(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	largeContent := generateContent(4000) // ~1000 tokens, exceeds MaxPerFile below
	repo.addFile("user1", domain.PersonaFileSOUL, largeContent)

	budget := TokenBudget{MaxTotal: 4000, MaxPerFile: 100}
	pc := NewPromptComposer(repo, "Policy.", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	require.True(t, out.Truncated, "test setup should trigger per-file truncation")
	assert.Contains(t, out.SystemPrompt, toolOutputGuardrail, "guardrail must survive per-file truncation")
}

// TestCompose_GuardrailSurvivesTotalBudgetTruncation covers P2.2's second
// acceptance criterion for the total-budget truncation path
// (applyTotalBudget), including the extreme case where the total budget is
// smaller than the guardrail's own token cost — the guardrail is fixed and
// outside the truncatable section budget, so it must still appear in full.
func TestCompose_GuardrailSurvivesTotalBudgetTruncation(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	largeContent := generateContent(2000)
	repo.addFile("user1", domain.PersonaFileRULES, "Critical rules.")
	repo.addFile("user1", domain.PersonaFileSOUL, largeContent)
	repo.addFile("user1", domain.PersonaFileUSER, largeContent)

	budget := TokenBudget{MaxTotal: 200, MaxPerFile: 100}
	pc := NewPromptComposer(repo, "Policy.", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.True(t, out.Truncated)
	assert.Contains(t, out.SystemPrompt, toolOutputGuardrail, "guardrail must survive total-budget truncation")

	// Extreme case: total budget smaller than the guardrail's own token cost.
	tinyBudget := TokenBudget{MaxTotal: 5, MaxPerFile: 100}
	pcTiny := NewPromptComposer(repo, "Policy.", WithTokenBudget(tinyBudget))
	outTiny, err := pcTiny.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, outTiny.SystemPrompt, toolOutputGuardrail, "guardrail must appear in full even under a token budget smaller than its own size")
}
