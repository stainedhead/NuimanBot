package skill

import (
	"fmt"
	"os"
	"path/filepath"

	"nuimanbot/internal/domain"
)

// FilesystemSkillRepository scans directories for SKILL.md files.
// It implements the domain.SkillRepository interface.
type FilesystemSkillRepository struct {
	parser *SkillParser
}

// NewFilesystemSkillRepository creates a new filesystem-based skill repository.
func NewFilesystemSkillRepository() *FilesystemSkillRepository {
	return &FilesystemSkillRepository{
		parser: NewSkillParser(),
	}
}

// Scan discovers all skills in the provided root directories.
// Returns a list of skills with full metadata.
//
// Scan behavior:
//   - Only scans top-level subdirectories (no nested scanning)
//   - Skips directories without SKILL.md
//   - Logs warnings for invalid skills but continues scanning
//   - Gracefully handles nonexistent root directories
//   - Expands ~ in paths to user home directory
func (r *FilesystemSkillRepository) Scan(roots []domain.SkillRoot) ([]domain.Skill, error) {
	var skills []domain.Skill

	for _, root := range roots {
		// Expand ~ in path
		rootPath, err := expandHome(root.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to expand path %s: %w", root.Path, err)
		}

		// Check if root exists
		if _, err := os.Stat(rootPath); os.IsNotExist(err) {
			// Skip non-existent directories silently
			continue
		}

		// List subdirectories in root
		entries, err := os.ReadDir(rootPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", rootPath, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue // Skip files, only process directories
			}

			skillDir := filepath.Join(rootPath, entry.Name())
			skillFile := filepath.Join(skillDir, "SKILL.md")

			// Check if SKILL.md exists
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				continue // Skip directories without SKILL.md
			}

			// Parse skill
			skill, err := r.parser.Parse(skillFile, root.Scope)
			if err != nil {
				// Log warning but don't fail entire scan
				// In a production system, this would use structured logging
				fmt.Fprintf(os.Stderr, "Warning: Failed to parse skill at %s: %v\n", skillFile, err)
				continue
			}

			skills = append(skills, *skill)
		}
	}

	return skills, nil
}

// Load reads the full content of a specific skill by file path.
// This includes parsing the frontmatter and reading the body.
func (r *FilesystemSkillRepository) Load(skillPath string) (*domain.Skill, error) {
	// Determine scope from path
	// This is a simplified implementation; in production, scope would be
	// determined by matching the path against configured root directories
	scope := domain.ScopeProject // Default scope

	skill, err := r.parser.Parse(skillPath, scope)
	if err != nil {
		return nil, err
	}

	return skill, nil
}

// expandHome expands ~ to user home directory.
// Supports:
//   - ~ -> $HOME
//   - ~/path -> $HOME/path
//
// Does not support ~user notation.
func expandHome(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if len(path) == 1 {
		return home, nil
	}

	if path[1] == '/' || path[1] == filepath.Separator {
		return filepath.Join(home, path[2:]), nil
	}

	// ~user notation not supported, return as-is
	return path, nil
}
