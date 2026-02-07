package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"nuimanbot/internal/domain"
)

// SkillParser parses SKILL.md files following the Anthropic Agent Skills format.
type SkillParser struct{}

// NewSkillParser creates a new skill parser.
func NewSkillParser() *SkillParser {
	return &SkillParser{}
}

// ParseFrontmatter extracts and parses YAML frontmatter from SKILL.md content.
// Expected format:
//
//	---
//	name: skill-name
//	description: description
//	---
//	Markdown body
//
// Returns the parsed frontmatter, body content, and any error encountered.
func (p *SkillParser) ParseFrontmatter(content []byte) (*domain.SkillFrontmatter, string, error) {
	// Split on "---" delimiters
	parts := bytes.SplitN(content, []byte("---"), 3)

	if len(parts) < 3 {
		return nil, "", fmt.Errorf("missing frontmatter delimiters (expected ---...---)")
	}

	// parts[0] should be empty (before first ---)
	// parts[1] is YAML frontmatter
	// parts[2] is markdown body

	yamlContent := bytes.TrimSpace(parts[1])
	if len(yamlContent) == 0 {
		return nil, "", fmt.Errorf("frontmatter is empty")
	}

	var fm domain.SkillFrontmatter
	if err := yaml.Unmarshal(yamlContent, &fm); err != nil {
		return nil, "", fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	body := string(bytes.TrimSpace(parts[2]))

	return &fm, body, nil
}

// ValidateFrontmatter checks frontmatter against skill naming rules.
// If expectedName is provided, it verifies that the name in frontmatter matches.
func (p *SkillParser) ValidateFrontmatter(fm *domain.SkillFrontmatter, expectedName string) error {
	if err := fm.Validate(); err != nil {
		return err
	}

	// Name in frontmatter must match directory name (if expectedName provided)
	if expectedName != "" && fm.Name != expectedName {
		return domain.ErrSkillInvalid{
			SkillName: fm.Name,
			Reason:    fmt.Sprintf("name in frontmatter (%s) does not match directory name (%s)", fm.Name, expectedName),
		}
	}

	return nil
}

// Parse parses a complete SKILL.md file and returns a domain.Skill.
// It performs the following:
//  1. Reads the file from disk
//  2. Parses the YAML frontmatter
//  3. Validates frontmatter against directory name
//  4. Creates and validates the Skill entity
func (p *SkillParser) Parse(filePath string, scope domain.SkillScope) (*domain.Skill, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, domain.ErrSkillParseError{FilePath: filePath, Err: err}
	}

	fm, body, err := p.ParseFrontmatter(content)
	if err != nil {
		return nil, domain.ErrSkillParseError{FilePath: filePath, Err: err}
	}

	// Extract directory name (expected skill name)
	dir := filepath.Dir(filePath)
	expectedName := filepath.Base(dir)

	if err := p.ValidateFrontmatter(fm, expectedName); err != nil {
		return nil, domain.ErrSkillParseError{FilePath: filePath, Err: err}
	}

	skill := &domain.Skill{
		ID:          fmt.Sprintf("%s:%s", scope, fm.Name),
		Name:        fm.Name,
		Description: fm.Description,
		Scope:       scope,
		Priority:    scope.Priority(),
		Frontmatter: *fm,
		BodyMD:      body,
		Directory:   dir,
		FilePath:    filePath,
	}

	if err := skill.Validate(); err != nil {
		return nil, domain.ErrSkillParseError{FilePath: filePath, Err: err}
	}

	return skill, nil
}
