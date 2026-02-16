package persona

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nuimanbot/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewPromptComposer tests ---

func TestNewPromptComposer_DefaultBudget(t *testing.T) {
	repo := newMockRepo()
	pc := NewPromptComposer(repo, "You are a helpful assistant.")

	assert.NotNil(t, pc)
	assert.Equal(t, DefaultMaxTotal, pc.tokenBudget.MaxTotal)
	assert.Equal(t, DefaultMaxPerFile, pc.tokenBudget.MaxPerFile)
}

func TestNewPromptComposer_WithCustomBudget(t *testing.T) {
	repo := newMockRepo()
	budget := TokenBudget{MaxTotal: 2000, MaxPerFile: 800}
	pc := NewPromptComposer(repo, "policy", WithTokenBudget(budget))

	assert.Equal(t, 2000, pc.tokenBudget.MaxTotal)
	assert.Equal(t, 800, pc.tokenBudget.MaxPerFile)
}

// --- Compose tests ---

func TestCompose_AllFilesPresent(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "Be safe and secure.")
	repo.addFile("user1", domain.PersonaFileSOUL, "You are cheerful and helpful.")
	repo.addFile("user1", domain.PersonaFileUSER, "Name: Alice. Timezone: EST.")

	pc := NewPromptComposer(repo, "Global: Always be respectful.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	require.NotNil(t, out)

	// Global policy comes first
	assert.Contains(t, out.SystemPrompt, "Global: Always be respectful.")
	// RULES before SOUL before USER
	rulesIdx := strings.Index(out.SystemPrompt, "Be safe and secure.")
	soulIdx := strings.Index(out.SystemPrompt, "You are cheerful and helpful.")
	userIdx := strings.Index(out.SystemPrompt, "Name: Alice. Timezone: EST.")
	assert.Greater(t, soulIdx, rulesIdx, "SOUL should come after RULES")
	assert.Greater(t, userIdx, soulIdx, "USER should come after SOUL")
	assert.False(t, out.Truncated)
	assert.Greater(t, out.TokensUsed, 0)
}

func TestCompose_EmptyUserID(t *testing.T) {
	repo := newMockRepo()
	pc := NewPromptComposer(repo, "policy")

	_, err := pc.Compose(context.Background(), ComposerInput{UserID: ""})
	assert.Error(t, err)
}

func TestCompose_NoFiles(t *testing.T) {
	repo := newMockRepo()
	pc := NewPromptComposer(repo, "Global policy.")

	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "unknown-user"})
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Contains(t, out.SystemPrompt, "Global policy.")
	assert.False(t, out.Truncated)
}

func TestCompose_OnlyRulesFile(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "No external calls.")

	pc := NewPromptComposer(repo, "Admin policy.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, "Admin policy.")
	assert.Contains(t, out.SystemPrompt, "No external calls.")
	assert.False(t, out.Truncated)
}

func TestCompose_EmptyFileContent(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileSOUL, "   ")
	repo.addFile("user1", domain.PersonaFileUSER, "Some preferences.")

	pc := NewPromptComposer(repo, "Policy.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.NotContains(t, out.SystemPrompt, "## SOUL")
	assert.Contains(t, out.SystemPrompt, "Some preferences.")
}

func TestCompose_RepositoryError(t *testing.T) {
	repo := newMockRepo()
	repo.err = errors.New("disk failure")

	pc := NewPromptComposer(repo, "Policy.")
	_, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk failure")
}

// --- Token budget and truncation tests ---

func TestCompose_TruncatesWhenOverBudget(t *testing.T) {
	repo := newMockRepo()
	largeContent := generateContent(2000) // ~500 tokens

	repo.addFile("user1", domain.PersonaFileRULES, "Critical rules.")
	repo.addFile("user1", domain.PersonaFileSOUL, largeContent)
	repo.addFile("user1", domain.PersonaFileUSER, largeContent)

	budget := TokenBudget{MaxTotal: 200, MaxPerFile: 100}
	pc := NewPromptComposer(repo, "Policy.", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.True(t, out.Truncated)
	assert.LessOrEqual(t, out.TokensUsed, budget.MaxTotal)
	assert.NotEmpty(t, out.TruncatedFiles)
}

func TestCompose_TruncationPriority_RulesPreserved(t *testing.T) {
	repo := newMockRepo()
	rulesContent := "Critical: never share secrets."
	soulContent := generateContent(800)
	userContent := generateContent(800)

	repo.addFile("user1", domain.PersonaFileRULES, rulesContent)
	repo.addFile("user1", domain.PersonaFileSOUL, soulContent)
	repo.addFile("user1", domain.PersonaFileUSER, userContent)

	budget := TokenBudget{MaxTotal: 150, MaxPerFile: 250}
	pc := NewPromptComposer(repo, "", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, "Critical: never share secrets.")
	assert.True(t, out.Truncated)
}

func TestCompose_PerFileTruncation(t *testing.T) {
	repo := newMockRepo()
	largeContent := generateContent(4000) // ~1000 tokens

	repo.addFile("user1", domain.PersonaFileSOUL, largeContent)

	budget := TokenBudget{MaxTotal: 4000, MaxPerFile: 100}
	pc := NewPromptComposer(repo, "", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.True(t, out.Truncated)
	assert.Contains(t, out.TruncatedFiles, "SOUL")
}

func TestCompose_TruncationMarker(t *testing.T) {
	repo := newMockRepo()
	largeContent := generateContent(4000) // ~1000 tokens

	repo.addFile("user1", domain.PersonaFileSOUL, largeContent)

	budget := TokenBudget{MaxTotal: 4000, MaxPerFile: 50}
	pc := NewPromptComposer(repo, "", WithTokenBudget(budget))
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, "[truncated]")
}

// --- Section header tests ---

func TestCompose_SectionHeaders(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileRULES, "Rule one.")
	repo.addFile("user1", domain.PersonaFileSOUL, "Soul content.")
	repo.addFile("user1", domain.PersonaFileUSER, "User context.")

	pc := NewPromptComposer(repo, "Policy.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, "## RULES")
	assert.Contains(t, out.SystemPrompt, "## SOUL")
	assert.Contains(t, out.SystemPrompt, "## USER")
}

// --- TokensUsed calculation ---

func TestCompose_TokensUsedCalculation(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileSOUL, "Hello world.")

	pc := NewPromptComposer(repo, "Policy.")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	expectedTokens := estimateTokens(out.SystemPrompt)
	assert.Equal(t, expectedTokens, out.TokensUsed)
}

// --- GlobalPolicy tests ---

func TestCompose_EmptyGlobalPolicy(t *testing.T) {
	repo := newMockRepo()
	repo.addFile("user1", domain.PersonaFileSOUL, "Be nice.")

	pc := NewPromptComposer(repo, "")
	out, err := pc.Compose(context.Background(), ComposerInput{UserID: "user1"})

	require.NoError(t, err)
	assert.Contains(t, out.SystemPrompt, "Be nice.")
}

// --- Helper functions ---

func generateContent(charCount int) string {
	base := "Lorem ipsum dolor sit amet consectetur adipiscing elit. "
	var result strings.Builder
	for result.Len() < charCount {
		result.WriteString(base)
	}
	return result.String()[:charCount]
}
