package persona

import (
	"context"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/chat"
	personausecase "nuimanbot/internal/usecase/persona"
	"nuimanbot/internal/usecase/tool"
)

// PromptComposerAdapter adapts personausecase.PromptComposer to chat.PromptComposer interface.
type PromptComposerAdapter struct {
	composer *personausecase.PromptComposer
}

// NewPromptComposerAdapter creates a new PromptComposerAdapter.
func NewPromptComposerAdapter(repo domain.PersonaFileRepository, globalPrompt string, opts ...personausecase.ComposerOption) *PromptComposerAdapter {
	return &PromptComposerAdapter{
		composer: personausecase.NewPromptComposer(repo, globalPrompt, opts...),
	}
}

// Compose implements chat.PromptComposer interface.
func (a *PromptComposerAdapter) Compose(ctx context.Context, input chat.PromptComposerInput) (*chat.PromptComposerOutput, error) {
	// Convert chat.PromptComposerInput to personausecase.ComposerInput
	composerInput := personausecase.ComposerInput{
		UserID:   input.UserID,
		Platform: input.Platform,
	}

	// Call the underlying composer
	output, err := a.composer.Compose(ctx, composerInput)
	if err != nil {
		return nil, err
	}

	// Convert personausecase.ComposerOutput to chat.PromptComposerOutput
	return &chat.PromptComposerOutput{
		SystemPrompt:   output.SystemPrompt,
		TokensUsed:     output.TokensUsed,
		Truncated:      output.Truncated,
		TruncatedFiles: output.TruncatedFiles,
	}, nil
}

// RulesEnforcerAdapter adapts personausecase.RulesEnforcer to tool.RulesEnforcer interface.
type RulesEnforcerAdapter struct {
	enforcer *personausecase.RulesEnforcer
}

// NewRulesEnforcerAdapter creates a new RulesEnforcerAdapter.
func NewRulesEnforcerAdapter(repo domain.PersonaFileRepository, parser personausecase.FrontmatterParser, adminPolicy *domain.RulesConfig) *RulesEnforcerAdapter {
	return &RulesEnforcerAdapter{
		enforcer: personausecase.NewRulesEnforcer(repo, parser, adminPolicy),
	}
}

// Enforce implements tool.RulesEnforcer interface.
func (a *RulesEnforcerAdapter) Enforce(ctx context.Context, input tool.EnforcerInput) (*tool.EnforcerOutput, error) {
	// Convert tool.EnforcerInput to personausecase.EnforcerInput
	enforcerInput := personausecase.EnforcerInput{
		UserID: input.UserID,
		Action: input.Action,
		Tool:   input.Tool,
	}

	// Call the underlying enforcer
	output, err := a.enforcer.Enforce(ctx, enforcerInput)
	if err != nil {
		return nil, err
	}

	// Convert personausecase.EnforcerOutput to tool.EnforcerOutput
	return &tool.EnforcerOutput{
		Allowed:              output.Allowed,
		RequiresConfirmation: output.RequiresConfirmation,
		Reason:               output.Reason,
	}, nil
}
