package domain

import "fmt"

// Skill represents a file-based instruction package for agents.
// Skills provide reusable prompt templates and instructions that can be
// invoked by users (via /skill-name) or automatically selected by LLMs.
type Skill struct {
	// ID is a unique identifier in the format "scope:name"
	ID string

	// Name is the skill name from frontmatter
	Name string

	// Description provides a short summary of what the skill does
	Description string

	// Scope indicates where the skill was discovered
	Scope SkillScope

	// Priority determines resolution order when multiple skills have the same name
	Priority int

	// Frontmatter contains parsed YAML metadata from the skill file
	Frontmatter SkillFrontmatter

	// BodyMD contains the Markdown body content (the actual prompt)
	BodyMD string

	// Directory is the absolute path to the skill directory
	Directory string

	// FilePath is the absolute path to the SKILL.md file
	FilePath string
}

// SkillScope represents the location where a skill was discovered.
// Skills are resolved with priority: Enterprise > User > Project > Plugin
type SkillScope int

const (
	// ScopeEnterprise represents enterprise-managed skills (highest priority)
	ScopeEnterprise SkillScope = iota

	// ScopeUser represents user home directory skills
	ScopeUser

	// ScopeProject represents project-local skills
	ScopeProject

	// ScopePlugin represents plugin-provided skills (lowest priority)
	ScopePlugin
)

// String returns the string representation of the scope
func (s SkillScope) String() string {
	return [...]string{"enterprise", "user", "project", "plugin"}[s]
}

// Priority returns the resolution priority for this scope.
// Higher values indicate higher priority.
func (s SkillScope) Priority() int {
	return [...]int{300, 200, 100, 50}[s]
}

// SkillFrontmatter represents the parsed YAML frontmatter from a SKILL.md file.
// The frontmatter follows the Anthropic Agent Skills open standard.
type SkillFrontmatter struct {
	// Name is the skill identifier (required, lowercase-hyphenated)
	Name string `yaml:"name"`

	// Description provides a short summary (required, max 1024 chars)
	Description string `yaml:"description"`

	// DisableModelInvocation prevents the model from auto-selecting this skill
	DisableModelInvocation bool `yaml:"disable-model-invocation,omitempty"`

	// UserInvocable controls whether users can invoke via /skill-name
	// Defaults to true if not specified
	UserInvocable *bool `yaml:"user-invocable,omitempty"`

	// AllowedTools restricts which tools the skill can use (empty = all allowed)
	AllowedTools []string `yaml:"allowed-tools,omitempty"`

	// License specifies the skill's license (optional)
	License string `yaml:"license,omitempty"`

	// Compatibility specifies compatible agent versions (optional)
	Compatibility string `yaml:"compatibility,omitempty"`

	// Metadata contains additional custom metadata (optional)
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

// SkillCatalogEntry is a lightweight representation of a skill for catalog listings.
// It contains only the essential metadata without the full prompt body.
type SkillCatalogEntry struct {
	// Name is the skill identifier
	Name string `json:"name"`

	// Description provides a short summary
	Description string `json:"description"`

	// Scope indicates where the skill was discovered
	Scope SkillScope `json:"scope"`

	// Priority determines resolution order
	Priority int `json:"priority"`
}

// RenderedSkill represents a skill after argument substitution.
// This is the output of the rendering process before passing to the LLM.
type RenderedSkill struct {
	// SkillName is the original skill name
	SkillName string

	// Prompt is the rendered prompt with arguments substituted
	Prompt string

	// AllowedTools is the tool allowlist (empty = all allowed)
	AllowedTools []string
}

// SkillRepository defines operations for discovering and loading skills from the filesystem.
type SkillRepository interface {
	// Scan discovers all skills in the provided root directories.
	// Returns a list of skills with catalog metadata only (lazy loading).
	// Skills are not fully loaded until Load() is called.
	Scan(roots []SkillRoot) ([]Skill, error)

	// Load reads the full content of a specific skill by file path.
	// This includes parsing the frontmatter and reading the body.
	Load(skillPath string) (*Skill, error)
}

// SkillRoot represents a root directory to scan for skills.
type SkillRoot struct {
	// Path is the absolute path to the root directory
	Path string

	// Scope determines the priority of skills found in this root
	Scope SkillScope
}

// CanBeInvokedByUser returns true if the skill can be invoked by users via /skill-name.
// Defaults to true if not explicitly set to false.
func (s *Skill) CanBeInvokedByUser() bool {
	return s.Frontmatter.IsUserInvocable()
}

// CanBeSelectedByModel returns true if the model can automatically select this skill.
// Returns false if DisableModelInvocation is true.
func (s *Skill) CanBeSelectedByModel() bool {
	return !s.Frontmatter.DisableModelInvocation
}

// AllowedTools returns the list of tools this skill is allowed to use.
// An empty list means all tools are allowed.
func (s *Skill) AllowedTools() []string {
	return s.Frontmatter.AllowedTools
}

// Validate checks if the skill is valid according to domain rules.
func (s *Skill) Validate() error {
	if s.Name == "" {
		return ErrSkillInvalid{Reason: "name is required"}
	}
	if s.Description == "" {
		return ErrSkillInvalid{Reason: "description is required"}
	}
	return s.Frontmatter.Validate()
}

// IsUserInvocable checks if the skill can be invoked by users.
// Defaults to true if not explicitly set to false.
func (fm *SkillFrontmatter) IsUserInvocable() bool {
	if fm.UserInvocable == nil {
		return true // Default to true
	}
	return *fm.UserInvocable
}

// IsModelInvocable checks if the model can automatically select this skill.
func (fm *SkillFrontmatter) IsModelInvocable() bool {
	return !fm.DisableModelInvocation
}

// Validate checks if the frontmatter is valid according to the skill specification.
func (fm *SkillFrontmatter) Validate() error {
	if fm.Name == "" {
		return ErrSkillInvalid{Reason: "name is required in frontmatter"}
	}
	if len(fm.Name) > 64 {
		return ErrSkillInvalid{Reason: "name must be <= 64 characters"}
	}
	if !isValidSkillName(fm.Name) {
		return ErrSkillInvalid{Reason: "name must be lowercase, hyphens, alphanumeric"}
	}
	if fm.Description == "" {
		return ErrSkillInvalid{Reason: "description is required in frontmatter"}
	}
	if len(fm.Description) > 1024 {
		return ErrSkillInvalid{Reason: "description must be <= 1024 characters"}
	}
	return nil
}

// isValidSkillName checks if a skill name follows the naming rules.
// Rules: lowercase, hyphens, alphanumeric, 1-64 characters
func isValidSkillName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

// ErrSkillNotFound indicates a skill was not found in the catalog.
type ErrSkillNotFound struct {
	SkillName string
}

func (e ErrSkillNotFound) Error() string {
	return fmt.Sprintf("skill not found: %s", e.SkillName)
}

// ErrSkillInvalid indicates a skill failed validation.
type ErrSkillInvalid struct {
	SkillName string
	Reason    string
}

func (e ErrSkillInvalid) Error() string {
	if e.SkillName != "" {
		return fmt.Sprintf("skill %s is invalid: %s", e.SkillName, e.Reason)
	}
	return fmt.Sprintf("skill is invalid: %s", e.Reason)
}

// ErrSkillConflict indicates multiple skills with the same name were found in different scopes.
type ErrSkillConflict struct {
	SkillName string
	Scopes    []SkillScope
}

func (e ErrSkillConflict) Error() string {
	return fmt.Sprintf("skill %s found in multiple scopes: %v", e.SkillName, e.Scopes)
}

// ErrSkillPermissionDenied indicates the user lacks permission to execute a skill.
type ErrSkillPermissionDenied struct {
	SkillName string
	Reason    string
}

func (e ErrSkillPermissionDenied) Error() string {
	return fmt.Sprintf("permission denied for skill %s: %s", e.SkillName, e.Reason)
}

// ErrSkillParseError indicates a YAML or Markdown parsing error.
type ErrSkillParseError struct {
	FilePath string
	Err      error
}

func (e ErrSkillParseError) Error() string {
	return fmt.Sprintf("failed to parse skill at %s: %v", e.FilePath, e.Err)
}

// Unwrap returns the underlying error for error chain support.
func (e ErrSkillParseError) Unwrap() error {
	return e.Err
}
