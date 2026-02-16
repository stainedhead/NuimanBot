package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nuimanbot/internal/domain"
)

// AllowedExtensions is the set of file extensions permitted for persona files.
var AllowedExtensions = []string{".md", ".txt", ".json"}

// ValidateUserPath ensures a user-relative path resolves within the base directory.
// It joins baseDir/userID/filename, then verifies the resolved path stays within
// baseDir. Returns the resolved absolute path or domain.ErrPathTraversal.
func ValidateUserPath(baseDir, userID, filename string) (string, error) {
	if baseDir == "" {
		return "", domain.ErrPathTraversal
	}
	if userID == "" {
		return "", domain.ErrPathTraversal
	}
	if filename == "" {
		return "", domain.ErrPathTraversal
	}

	// Reject null bytes in any component
	if containsNullByte(userID) || containsNullByte(filename) {
		return "", domain.ErrPathTraversal
	}

	// Reject ".." components in userID or filename (cross-platform defense)
	if containsTraversal(userID) || containsTraversal(filename) {
		return "", domain.ErrPathTraversal
	}

	// Resolve base directory to absolute
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", domain.ErrPathTraversal
	}

	// Join and resolve the full path
	joined := filepath.Join(absBaseDir, userID, filename)
	absResolved, err := filepath.Abs(joined)
	if err != nil {
		return "", domain.ErrPathTraversal
	}

	// The resolved path must be strictly inside the base directory
	if !strings.HasPrefix(absResolved, absBaseDir+string(filepath.Separator)) {
		return "", domain.ErrPathTraversal
	}

	return absResolved, nil
}

// ValidateExtension checks that the file has an allowed extension.
func ValidateExtension(path string) error {
	if path == "" {
		return fmt.Errorf("file path is required")
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, allowed := range AllowedExtensions {
		if ext == allowed {
			return nil
		}
	}
	return fmt.Errorf("file extension not allowed: %q (allowed: %v)", ext, AllowedExtensions)
}

// containsTraversal checks for ".." path components using both forward slash
// and backslash separators for cross-platform safety.
func containsTraversal(path string) bool {
	unified := strings.ReplaceAll(path, `\`, "/")
	for _, segment := range strings.Split(unified, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// containsNullByte returns true if the string contains a null byte.
func containsNullByte(s string) bool {
	return strings.ContainsRune(s, '\x00')
}

// ValidateNoSymlink checks if the given path is a symlink.
// Returns domain.ErrPathTraversal if the file is a symlink, to prevent
// symlink attacks that could read files outside the allowed directory.
func ValidateNoSymlink(path string) error {
	info, err := os.Lstat(path) // Lstat does NOT follow symlinks
	if err != nil {
		// If file doesn't exist yet, that's OK (not a symlink)
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return domain.ErrPathTraversal
	}

	return nil
}
