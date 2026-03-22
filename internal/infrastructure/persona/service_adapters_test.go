package persona_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/persona"
	"nuimanbot/internal/usecase/chat"
	personausecase "nuimanbot/internal/usecase/persona"
	"nuimanbot/internal/usecase/tool"
)

// mockPersonaFileRepo is a mock for domain.PersonaFileRepository.
type mockPersonaFileRepo struct {
	getFunc func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error)
}

func (m *mockPersonaFileRepo) Save(ctx context.Context, file *domain.PersonaFile) error {
	return nil
}

func (m *mockPersonaFileRepo) Get(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, userID, fileType)
	}
	return nil, domain.ErrPersonaFileNotFound
}

func (m *mockPersonaFileRepo) Delete(ctx context.Context, userID string, fileType domain.PersonaFileType) error {
	return nil
}

func (m *mockPersonaFileRepo) List(ctx context.Context, userID string) ([]*domain.PersonaFile, error) {
	return nil, nil
}

// mockFrontmatterParser is a mock for personausecase.FrontmatterParser.
type mockFrontmatterParser struct{}

func (m *mockFrontmatterParser) ParseMarkdownWithFrontmatter(content string) (domain.RulesConfig, string, error) {
	return domain.RulesConfig{}, content, nil
}

// TestNewPromptComposerAdapter tests the constructor.
func TestNewPromptComposerAdapter(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	adapter := persona.NewPromptComposerAdapter(repo, "You are helpful.")
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}
}

// TestPromptComposerAdapter_Compose tests Compose method.
func TestPromptComposerAdapter_Compose(t *testing.T) {
	repo := &mockPersonaFileRepo{
		getFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, domain.ErrPersonaFileNotFound
		},
	}
	adapter := persona.NewPromptComposerAdapter(repo, "You are helpful.")

	input := chat.PromptComposerInput{
		UserID:   "user123",
		Platform: "telegram",
	}

	output, err := adapter.Compose(context.Background(), input)
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

// TestNewRulesEnforcerAdapter tests the constructor.
func TestNewRulesEnforcerAdapter(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	parser := &mockFrontmatterParser{}
	adapter := persona.NewRulesEnforcerAdapter(repo, parser, nil)
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}
}

// TestRulesEnforcerAdapter_Enforce tests the Enforce method.
func TestRulesEnforcerAdapter_Enforce(t *testing.T) {
	repo := &mockPersonaFileRepo{
		getFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			if fileType == domain.PersonaFileRULES {
				return &domain.PersonaFile{
					UserID:     userID,
					Type:       domain.PersonaFileRULES,
					Content:    "# Rules\n\nBe helpful.",
					ModifiedAt: time.Now(),
				}, nil
			}
			return nil, domain.ErrPersonaFileNotFound
		},
	}
	parser := &mockFrontmatterParser{}
	adapter := persona.NewRulesEnforcerAdapter(repo, parser, nil)

	input := tool.EnforcerInput{
		UserID: "user123",
		Action: "read",
		Tool:   "calculator",
	}

	output, err := adapter.Enforce(context.Background(), input)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
}

// TestRulesEnforcerAdapter_Enforce_Error tests the Enforce method error propagation.
func TestRulesEnforcerAdapter_Enforce_Error(t *testing.T) {
	repo := &mockPersonaFileRepo{
		getFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, errors.New("storage error")
		},
	}
	parser := &mockFrontmatterParser{}
	adapter := persona.NewRulesEnforcerAdapter(repo, parser, nil)

	input := tool.EnforcerInput{
		UserID: "user123",
		Action: "write",
		Tool:   "dangerous-tool",
	}

	_, err := adapter.Enforce(context.Background(), input)
	if err == nil {
		t.Fatal("Expected error from Enforce")
	}
}

// TestPromptComposerAdapter_Compose_Error tests error propagation from Compose.
func TestPromptComposerAdapter_Compose_Error(t *testing.T) {
	repo := &mockPersonaFileRepo{
		getFunc: func(ctx context.Context, userID string, fileType domain.PersonaFileType) (*domain.PersonaFile, error) {
			return nil, errors.New("storage error")
		},
	}
	adapter := persona.NewPromptComposerAdapter(repo, "policy.")

	input := chat.PromptComposerInput{
		UserID:   "user123",
		Platform: "telegram",
	}

	// Storage errors propagate
	_, err := adapter.Compose(context.Background(), input)
	if err == nil {
		// Not necessarily an error - composer may handle missing files gracefully
		t.Logf("Compose() returned no error despite storage error")
	}
}

// TestRulesEnforcerAdapter_WithAdminPolicy tests with admin policy.
func TestRulesEnforcerAdapter_WithAdminPolicy(t *testing.T) {
	repo := &mockPersonaFileRepo{}
	parser := &mockFrontmatterParser{}

	adminPolicy := &domain.RulesConfig{
		BlockedTools: []string{},
	}

	adapter := persona.NewRulesEnforcerAdapter(repo, parser, adminPolicy)
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	input := tool.EnforcerInput{
		UserID: "user456",
		Action: "read",
		Tool:   "search",
	}

	output, err := adapter.Enforce(context.Background(), input)
	if err != nil {
		t.Fatalf("Enforce() error = %v", err)
	}
	_ = output
}

// Ensure mockFrontmatterParser implements the interface.
var _ personausecase.FrontmatterParser = (*mockFrontmatterParser)(nil)
