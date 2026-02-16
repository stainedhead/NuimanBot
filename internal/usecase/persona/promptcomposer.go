package persona

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// Default token budget constants.
const (
	DefaultMaxTotal   = 4000
	DefaultMaxPerFile = 1500
)

// TokenBudget defines token limits for prompt composition.
type TokenBudget struct {
	MaxTotal   int `yaml:"max_total"`
	MaxPerFile int `yaml:"max_per_file"`
}

// ComposerInput represents input for prompt composition.
type ComposerInput struct {
	UserID   string
	Platform string
}

// ComposerOutput represents the composed system prompt.
type ComposerOutput struct {
	SystemPrompt   string
	TokensUsed     int
	Truncated      bool
	TruncatedFiles []string
}

// ComposerOption configures a PromptComposer.
type ComposerOption func(*PromptComposer)

// WithTokenBudget sets a custom token budget.
func WithTokenBudget(budget TokenBudget) ComposerOption {
	return func(c *PromptComposer) {
		c.tokenBudget = budget
	}
}

// PromptComposer builds system prompts from persona files.
type PromptComposer struct {
	repo         domain.PersonaFileRepository
	tokenBudget  TokenBudget
	globalPolicy string
}

// NewPromptComposer creates a PromptComposer with the given repository and global policy.
func NewPromptComposer(repo domain.PersonaFileRepository, globalPolicy string, opts ...ComposerOption) *PromptComposer {
	pc := &PromptComposer{
		repo:         repo,
		globalPolicy: globalPolicy,
		tokenBudget: TokenBudget{
			MaxTotal:   DefaultMaxTotal,
			MaxPerFile: DefaultMaxPerFile,
		},
	}
	for _, opt := range opts {
		opt(pc)
	}
	return pc
}

// sectionOrder defines the composition order: RULES > SOUL > USER.
// RULES has highest priority for truncation preservation.
var sectionOrder = []domain.PersonaFileType{
	domain.PersonaFileRULES,
	domain.PersonaFileSOUL,
	domain.PersonaFileUSER,
}

// Compose builds a system prompt from persona files.
// Prompt structure: Global Policy -> RULES -> SOUL -> USER
// Truncation priority: RULES (preserved first) > SOUL > USER (truncated first).
func (c *PromptComposer) Compose(ctx context.Context, input ComposerInput) (*ComposerOutput, error) {
	if input.UserID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	sections, err := c.loadSections(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	c.applyPerFileTruncation(sections)

	prompt, truncated := c.buildPrompt(sections)

	return &ComposerOutput{
		SystemPrompt:   prompt,
		TokensUsed:     estimateTokens(prompt),
		Truncated:      truncated,
		TruncatedFiles: truncatedFileNames(sections),
	}, nil
}

// section holds a loaded and possibly truncated persona file section.
type section struct {
	fileType  domain.PersonaFileType
	content   string
	tokens    int
	truncated bool
}

// loadSections loads persona files from the repository, skipping missing/empty files.
func (c *PromptComposer) loadSections(ctx context.Context, userID string) ([]*section, error) {
	var sections []*section
	for _, ft := range sectionOrder {
		file, err := c.repo.Get(ctx, userID, ft)
		if err != nil {
			if errors.Is(err, domain.ErrPersonaFileNotFound) {
				continue
			}
			return nil, fmt.Errorf("loading %s: %w", ft.String(), err)
		}
		trimmed := strings.TrimSpace(file.Content)
		if trimmed == "" {
			continue
		}
		sections = append(sections, &section{
			fileType: ft,
			content:  trimmed,
			tokens:   estimateTokens(trimmed),
		})
	}
	return sections, nil
}

// applyPerFileTruncation truncates individual sections exceeding per-file budget.
func (c *PromptComposer) applyPerFileTruncation(sections []*section) {
	for _, s := range sections {
		if s.tokens > c.tokenBudget.MaxPerFile {
			s.content = truncateToTokens(s.content, c.tokenBudget.MaxPerFile)
			s.tokens = estimateTokens(s.content)
			s.truncated = true
		}
	}
}

// buildPrompt assembles the final prompt string, applying total budget truncation.
// Returns the prompt and whether any truncation occurred.
func (c *PromptComposer) buildPrompt(sections []*section) (string, bool) {
	var b strings.Builder

	// Global policy first (not subject to truncation)
	if c.globalPolicy != "" {
		b.WriteString(c.globalPolicy)
		b.WriteString("\n\n")
	}

	// Calculate overhead: policy + separator + all section headers/separators
	overhead := c.calcOverhead(sections)
	remaining := c.tokenBudget.MaxTotal - overhead

	// Apply total budget truncation in reverse priority (USER first, RULES last)
	truncated := c.applyTotalBudget(sections, remaining)

	// Write sections in order
	for _, s := range sections {
		if s.content == "" {
			continue
		}
		b.WriteString("## ")
		b.WriteString(s.fileType.String())
		b.WriteString("\n\n")
		b.WriteString(s.content)
		b.WriteString("\n\n")
	}

	return strings.TrimRight(b.String(), "\n"), truncated
}

// calcOverhead computes the token cost of non-content parts of the prompt:
// global policy, section headers ("## TYPE\n\n"), and separators ("\n\n").
// Includes a small safety margin per section to account for rounding in
// the token estimation heuristic.
func (c *PromptComposer) calcOverhead(sections []*section) int {
	overhead := 0
	if c.globalPolicy != "" {
		overhead += estimateTokens(c.globalPolicy + "\n\n")
	}
	for _, s := range sections {
		// "## TYPE\n\n" header + trailing "\n\n" after content + 1 rounding margin
		overhead += estimateTokens("## "+s.fileType.String()+"\n\n"+"\n\n") + 1
	}
	return overhead
}

// applyTotalBudget enforces the total token budget for section content only.
// Truncates in reverse priority order: USER -> SOUL -> RULES.
// Returns true if any truncation occurred.
func (c *PromptComposer) applyTotalBudget(sections []*section, remaining int) bool {
	totalNeeded := 0
	for _, s := range sections {
		totalNeeded += s.tokens
	}

	if totalNeeded <= remaining {
		return anyTruncated(sections)
	}

	// Need to truncate. Process in reverse priority: USER (last) -> SOUL -> RULES (first)
	excess := totalNeeded - remaining
	for i := len(sections) - 1; i >= 0 && excess > 0; i-- {
		s := sections[i]
		canTrim := s.tokens - 1 // Keep at least 1 token
		if canTrim <= 0 {
			continue
		}
		if excess >= canTrim {
			// Truncate this section to minimal
			s.content = truncateToTokens(s.content, 1)
			excess -= canTrim
		} else {
			// Partial truncation
			newTokens := s.tokens - excess
			s.content = truncateToTokens(s.content, newTokens)
			excess = 0
		}
		s.tokens = estimateTokens(s.content)
		s.truncated = true
	}

	return true
}

// estimateTokens approximates the token count for a string.
// Uses ~4 characters per token heuristic.
func estimateTokens(s string) int {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0
	}
	return (len(trimmed) + 3) / 4
}

// truncateToTokens truncates content to approximately the given token count,
// appending a "[truncated]" marker.
func truncateToTokens(content string, maxTokens int) string {
	marker := "\n...[truncated]"
	markerTokens := estimateTokens(marker)
	contentTokens := maxTokens - markerTokens
	if contentTokens <= 0 {
		return marker
	}

	maxChars := contentTokens * 4
	if maxChars >= len(content) {
		return content
	}

	// Try to truncate at a word boundary
	truncated := content[:maxChars]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxChars/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + marker
}

// truncatedFileNames returns the names of truncated sections.
func truncatedFileNames(sections []*section) []string {
	var names []string
	for _, s := range sections {
		if s.truncated {
			names = append(names, s.fileType.String())
		}
	}
	return names
}

// anyTruncated returns true if any section was truncated.
func anyTruncated(sections []*section) bool {
	return len(truncatedFileNames(sections)) > 0
}
