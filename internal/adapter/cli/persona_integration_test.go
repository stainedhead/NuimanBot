package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/adapter/cli"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/persona"
	personausecase "nuimanbot/internal/usecase/persona"
)

// TestPersonaIntegration_FullWorkflow tests the complete persona initialization and usage workflow.
func TestPersonaIntegration_FullWorkflow(t *testing.T) {
	// Setup: Create temp directory for persona files
	tempDir := t.TempDir()
	userID := "test-user-1"

	// Step 1: Initialize FileRepository (real implementation)
	repo := persona.NewFileRepository(tempDir)

	// Step 2: Create template loader that loads from templates directory
	templateLoader := &fileTemplateLoader{
		templates: map[domain.PersonaFileType]string{
			domain.PersonaFileSOUL:  "You are a helpful and friendly AI assistant.",
			domain.PersonaFileUSER:  "User: Test User\nTimezone: UTC",
			domain.PersonaFileRULES: "---\nblocked_tools:\n  - dangerous_tool\nrequires_confirmation:\n  - external_api\n---\n# Rules\nBe safe and secure.",
		},
	}

	// Step 3: Initialize persona files using CLI command
	personaCmd := cli.NewPersonaCommand(repo, templateLoader, os.Stdout)
	err := personaCmd.Init(context.Background(), userID)
	if err != nil {
		t.Fatalf("Failed to initialize persona files: %v", err)
	}

	// Step 4: Verify files were created
	soulFile, err := repo.Get(context.Background(), userID, domain.PersonaFileSOUL)
	if err != nil {
		t.Fatalf("Failed to get SOUL file: %v", err)
	}
	if !strings.Contains(soulFile.Content, "helpful and friendly") {
		t.Errorf("SOUL file has unexpected content: %s", soulFile.Content)
	}

	userFile, err := repo.Get(context.Background(), userID, domain.PersonaFileUSER)
	if err != nil {
		t.Fatalf("Failed to get USER file: %v", err)
	}
	if !strings.Contains(userFile.Content, "Test User") {
		t.Errorf("USER file has unexpected content: %s", userFile.Content)
	}

	rulesFile, err := repo.Get(context.Background(), userID, domain.PersonaFileRULES)
	if err != nil {
		t.Fatalf("Failed to get RULES file: %v", err)
	}
	if !strings.Contains(rulesFile.Content, "blocked_tools") {
		t.Errorf("RULES file has unexpected content: %s", rulesFile.Content)
	}

	// Step 5: Use PromptComposer to build system prompt
	composer := personausecase.NewPromptComposer(repo, "Global: Always be respectful.")
	output, err := composer.Compose(context.Background(), personausecase.ComposerInput{
		UserID:   userID,
		Platform: "test",
	})
	if err != nil {
		t.Fatalf("Failed to compose prompt: %v", err)
	}

	// Verify composed prompt contains all sections
	if !strings.Contains(output.SystemPrompt, "Global: Always be respectful.") {
		t.Errorf("System prompt missing global policy")
	}
	if !strings.Contains(output.SystemPrompt, "helpful and friendly") {
		t.Errorf("System prompt missing SOUL content")
	}
	if !strings.Contains(output.SystemPrompt, "Test User") {
		t.Errorf("System prompt missing USER content")
	}
	if !strings.Contains(output.SystemPrompt, "Be safe and secure") {
		t.Errorf("System prompt missing RULES content")
	}

	// Verify ordering: Global → RULES → SOUL → USER
	globalIdx := strings.Index(output.SystemPrompt, "Global:")
	rulesIdx := strings.Index(output.SystemPrompt, "Be safe and secure")
	soulIdx := strings.Index(output.SystemPrompt, "helpful and friendly")
	userIdx := strings.Index(output.SystemPrompt, "Test User")

	if globalIdx < 0 || rulesIdx < 0 || soulIdx < 0 || userIdx < 0 {
		t.Fatal("System prompt missing expected sections")
	}
	if globalIdx > rulesIdx || rulesIdx > soulIdx || soulIdx > userIdx {
		t.Errorf("System prompt sections in wrong order. Expected: Global < RULES < SOUL < USER")
	}

	// Step 6: Use RulesEnforcer to check tool restrictions
	parser := persona.NewRulesParserAdapter()
	enforcer := personausecase.NewRulesEnforcer(repo, parser, nil)

	// Test blocked tool
	blockedResult, err := enforcer.Enforce(context.Background(), personausecase.EnforcerInput{
		UserID: userID,
		Tool:   "dangerous_tool",
	})
	if err != nil {
		t.Fatalf("Enforcer failed: %v", err)
	}
	if blockedResult.Allowed {
		t.Error("Expected dangerous_tool to be blocked, but it was allowed")
	}

	// Test confirmation-required tool
	confirmResult, err := enforcer.Enforce(context.Background(), personausecase.EnforcerInput{
		UserID: userID,
		Tool:   "external_api",
	})
	if err != nil {
		t.Fatalf("Enforcer failed: %v", err)
	}
	if !confirmResult.Allowed {
		t.Error("Expected external_api to be allowed (with confirmation)")
	}
	if !confirmResult.RequiresConfirmation {
		t.Error("Expected external_api to require confirmation")
	}

	// Test allowed tool
	allowedResult, err := enforcer.Enforce(context.Background(), personausecase.EnforcerInput{
		UserID: userID,
		Tool:   "calculator",
	})
	if err != nil {
		t.Fatalf("Enforcer failed: %v", err)
	}
	if !allowedResult.Allowed {
		t.Error("Expected calculator to be allowed")
	}
	if allowedResult.RequiresConfirmation {
		t.Error("Expected calculator to not require confirmation")
	}

	t.Logf("✓ Full workflow test passed: Init → Compose → Enforce")
}

// TestPersonaIntegration_FileModification tests updating persona files and seeing changes reflected.
func TestPersonaIntegration_FileModification(t *testing.T) {
	tempDir := t.TempDir()
	userID := "test-user-2"

	repo := persona.NewFileRepository(tempDir)

	// Create initial SOUL file
	initialFile := &domain.PersonaFile{
		UserID:  userID,
		Type:    domain.PersonaFileSOUL,
		Content: "Initial personality: formal and professional.",
	}
	err := repo.Save(context.Background(), initialFile)
	if err != nil {
		t.Fatalf("Failed to save initial file: %v", err)
	}

	// Compose prompt with initial content
	composer := personausecase.NewPromptComposer(repo, "")
	output1, err := composer.Compose(context.Background(), personausecase.ComposerInput{
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("Failed to compose initial prompt: %v", err)
	}
	if !strings.Contains(output1.SystemPrompt, "formal and professional") {
		t.Error("Initial prompt missing expected content")
	}

	// Modify SOUL file
	modifiedFile := &domain.PersonaFile{
		UserID:  userID,
		Type:    domain.PersonaFileSOUL,
		Content: "Updated personality: casual and friendly.",
	}
	err = repo.Save(context.Background(), modifiedFile)
	if err != nil {
		t.Fatalf("Failed to save modified file: %v", err)
	}

	// Compose prompt with modified content (should reflect change)
	output2, err := composer.Compose(context.Background(), personausecase.ComposerInput{
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("Failed to compose modified prompt: %v", err)
	}
	if !strings.Contains(output2.SystemPrompt, "casual and friendly") {
		t.Error("Modified prompt missing updated content")
	}
	if strings.Contains(output2.SystemPrompt, "formal and professional") {
		t.Error("Modified prompt still contains old content")
	}

	t.Logf("✓ File modification test passed")
}

// TestPersonaIntegration_TokenBudgetTruncation tests that long persona files are truncated correctly.
func TestPersonaIntegration_TokenBudgetTruncation(t *testing.T) {
	tempDir := t.TempDir()
	userID := "test-user-3"

	repo := persona.NewFileRepository(tempDir)

	// Create very long content (will exceed token budget)
	longContent := strings.Repeat("This is a very long personality description. ", 200) // ~1600 chars = ~400 tokens

	files := []*domain.PersonaFile{
		{
			UserID:  userID,
			Type:    domain.PersonaFileRULES,
			Content: "Critical rules that must be preserved.",
		},
		{
			UserID:  userID,
			Type:    domain.PersonaFileSOUL,
			Content: longContent,
		},
		{
			UserID:  userID,
			Type:    domain.PersonaFileUSER,
			Content: longContent,
		},
	}

	for _, file := range files {
		if err := repo.Save(context.Background(), file); err != nil {
			t.Fatalf("Failed to save file %s: %v", file.Type.String(), err)
		}
	}

	// Compose with strict token budget. MaxTotal is 300 rather than the
	// pre-Phase-2 200: PromptComposer.Compose() now always prepends a fixed,
	// non-truncatable prompt-boundary guardrail (~90 tokens, FR-007,
	// specs/260802-improve-nuimanbot-security), so a 200-token total budget
	// no longer leaves enough headroom for the token-estimation heuristic's
	// rounding margin. 300 still forces truncation (asserted below) while
	// accommodating the fixed guardrail overhead.
	composer := personausecase.NewPromptComposer(
		repo,
		"",
		personausecase.WithTokenBudget(personausecase.TokenBudget{
			MaxTotal:   300, // Low enough to force truncation, high enough for guardrail overhead
			MaxPerFile: 100,
		}),
	)

	output, err := composer.Compose(context.Background(), personausecase.ComposerInput{
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("Failed to compose prompt: %v", err)
	}

	// Verify truncation occurred
	if !output.Truncated {
		t.Error("Expected truncation to occur, but it didn't")
	}

	// Verify RULES content is preserved (highest priority)
	if !strings.Contains(output.SystemPrompt, "Critical rules that must be preserved") {
		t.Error("RULES content was truncated (should be preserved)")
	}

	// Verify truncation marker is present
	if !strings.Contains(output.SystemPrompt, "[truncated]") {
		t.Error("Expected [truncated] marker in output")
	}

	// Verify total tokens under budget
	if output.TokensUsed > 300 {
		t.Errorf("Token budget exceeded: %d > 300", output.TokensUsed)
	}

	t.Logf("✓ Token budget truncation test passed")
}

// fileTemplateLoader implements TemplateLoader for testing.
type fileTemplateLoader struct {
	templates map[domain.PersonaFileType]string
}

func (l *fileTemplateLoader) Load(fileType domain.PersonaFileType) (string, error) {
	if content, ok := l.templates[fileType]; ok {
		return content, nil
	}
	return "", os.ErrNotExist
}

// TestPersonaIntegration_RealTemplateFiles tests loading actual template files from disk.
func TestPersonaIntegration_RealTemplateFiles(t *testing.T) {
	// Check if templates directory exists
	templatesDir := filepath.Join("..", "..", "..", "templates")
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		t.Skip("Templates directory not found, skipping real template test")
	}

	// Try to load actual templates
	soulPath := filepath.Join(templatesDir, "SOUL.md")
	soulContent, err := os.ReadFile(soulPath)
	if err != nil {
		t.Skipf("Could not read SOUL.md template: %v", err)
	}

	userPath := filepath.Join(templatesDir, "USER.md")
	userContent, err := os.ReadFile(userPath)
	if err != nil {
		t.Skipf("Could not read USER.md template: %v", err)
	}

	rulesPath := filepath.Join(templatesDir, "RULES.md")
	rulesContent, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Skipf("Could not read RULES.md template: %v", err)
	}

	// Verify templates have expected structure
	if !strings.Contains(string(soulContent), "SOUL") {
		t.Error("SOUL.md template missing expected header")
	}
	if !strings.Contains(string(userContent), "USER") {
		t.Error("USER.md template missing expected header")
	}
	if !strings.Contains(string(rulesContent), "RULES") {
		t.Error("RULES.md template missing expected header")
	}

	// Verify RULES.md has YAML frontmatter
	if !strings.HasPrefix(string(rulesContent), "---") {
		t.Error("RULES.md template missing YAML frontmatter")
	}

	t.Logf("✓ Real template files validated")
}
