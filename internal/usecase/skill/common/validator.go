package common

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PathValidator validates file paths against workspace restrictions
type PathValidator struct {
	allowedDirs []string
}

// NewPathValidator creates a new PathValidator with allowed directories
func NewPathValidator(allowedDirs []string) *PathValidator {
	// Canonicalize allowed directories
	canonical := make([]string, len(allowedDirs))
	for i, dir := range allowedDirs {
		absDir, err := filepath.Abs(filepath.Clean(dir))
		if err != nil {
			// If canonicalization fails, use the original path
			canonical[i] = filepath.Clean(dir)
		} else {
			canonical[i] = absDir
		}
	}

	return &PathValidator{
		allowedDirs: canonical,
	}
}

// ValidatePath checks if a path is within allowed directories
func (v *PathValidator) ValidatePath(path string) error {
	// Check for path traversal attempts in original path (before cleaning)
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}

	// Clean and canonicalize the input path
	cleanPath := filepath.Clean(path)

	// Get absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path is within any allowed directory
	for _, allowedDir := range v.allowedDirs {
		if strings.HasPrefix(absPath, allowedDir) {
			return nil
		}
	}

	return fmt.Errorf("path outside allowed workspace: %s", path)
}

// IsAllowed checks if a path is allowed (returns bool instead of error)
func (v *PathValidator) IsAllowed(path string) bool {
	return v.ValidatePath(path) == nil
}
