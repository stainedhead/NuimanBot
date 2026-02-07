package config

import (
	"fmt"
	"os"
	"path/filepath"

	"nuimanbot/internal/domain"
)

// SkillRootConfig configures a single skill root directory.
type SkillRootConfig struct {
	// Path to the skills directory (supports ~ expansion)
	Path string `yaml:"path"`

	// Scope determines the priority of skills in this root
	Scope domain.SkillScope `yaml:"scope"`
}

// SkillsConfig configures the Agent Skills system.
type SkillsConfig struct {
	// Enabled controls whether the skills system is active
	Enabled bool `yaml:"enabled"`

	// Roots defines directories to scan for skills
	Roots []SkillRootConfig `yaml:"roots"`
}

// DefaultSkillsConfig returns the default skills configuration.
func DefaultSkillsConfig() *SkillsConfig {
	return &SkillsConfig{
		Enabled: true,
		Roots: []SkillRootConfig{
			{
				Path:  "~/.claude/skills",
				Scope: domain.ScopeUser,
			},
			{
				Path:  "./.claude/skills",
				Scope: domain.ScopeProject,
			},
		},
	}
}

// ExpandPath expands ~ in the path to the user's home directory.
func (r *SkillRootConfig) ExpandPath() (string, error) {
	if len(r.Path) == 0 || r.Path[0] != '~' {
		return r.Path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if len(r.Path) == 1 {
		return home, nil
	}

	if r.Path[1] == '/' || r.Path[1] == filepath.Separator {
		return filepath.Join(home, r.Path[2:]), nil
	}

	// ~user notation not supported
	return r.Path, nil
}

// ToDomain converts SkillRootConfig to domain.SkillRoot with expanded path.
func (r *SkillRootConfig) ToDomain() (domain.SkillRoot, error) {
	expandedPath, err := r.ExpandPath()
	if err != nil {
		return domain.SkillRoot{}, err
	}

	return domain.SkillRoot{
		Path:  expandedPath,
		Scope: r.Scope,
	}, nil
}

// GetRoots returns domain.SkillRoot slices with expanded paths.
// Returns empty slice if skills are disabled.
func (c *SkillsConfig) GetRoots() ([]domain.SkillRoot, error) {
	if !c.Enabled {
		return []domain.SkillRoot{}, nil
	}

	roots := make([]domain.SkillRoot, 0, len(c.Roots))
	for _, rootCfg := range c.Roots {
		domainRoot, err := rootCfg.ToDomain()
		if err != nil {
			return nil, fmt.Errorf("failed to expand root path %s: %w", rootCfg.Path, err)
		}
		roots = append(roots, domainRoot)
	}

	return roots, nil
}

// Validate checks if the skills configuration is valid.
func (c *SkillsConfig) Validate() error {
	if !c.Enabled {
		return nil // Disabled config doesn't need validation
	}

	if len(c.Roots) == 0 {
		return fmt.Errorf("skills enabled but no roots configured")
	}

	for i, root := range c.Roots {
		if root.Path == "" {
			return fmt.Errorf("root[%d]: path is empty", i)
		}

		// Validate scope is a known value
		switch root.Scope {
		case domain.ScopeEnterprise, domain.ScopeUser, domain.ScopeProject, domain.ScopePlugin:
			// Valid
		default:
			return fmt.Errorf("root[%d]: invalid scope %d", i, root.Scope)
		}
	}

	return nil
}
