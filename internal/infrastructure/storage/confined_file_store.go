package storage

import (
	"fmt"
	"os"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/fsguard"
)

// FileConfinedFileStore implements domain.ConfinedFileStore using
// fsguard.ResolveWithin plus the calling filesystem's os package (FR-R5).
// This is the sole place jobs/chores/projects' filesystem writes now
// resolve through, restoring AGENTS.md's Clean Architecture dependency
// rule: those usecase packages depend only on domain.ConfinedFileStore,
// never on "os" or internal/infrastructure/fsguard directly.
type FileConfinedFileStore struct{}

// NewFileConfinedFileStore creates a FileConfinedFileStore.
func NewFileConfinedFileStore() *FileConfinedFileStore {
	return &FileConfinedFileStore{}
}

// EnsureDir creates dir (and any missing parents) if it does not already
// exist.
func (s *FileConfinedFileStore) EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}
	return nil
}

// WriteFile resolves relPath against baseDir via
// fsguard.ResolveWithinNoEscape (FR-R6: also rejects a symlink-based
// escape, not just a lexical one) and writes data to it, creating or
// truncating the file with mode 0644.
func (s *FileConfinedFileStore) WriteFile(baseDir, relPath string, data []byte) error {
	path, err := fsguard.ResolveWithinNoEscape(baseDir, relPath)
	if err != nil {
		return fmt.Errorf("failed to resolve confined path: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", path, err)
	}
	return nil
}

// FileExists resolves relPath against baseDir via
// fsguard.ResolveWithinNoEscape (FR-R6) and reports whether it currently
// exists.
func (s *FileConfinedFileStore) FileExists(baseDir, relPath string) (bool, error) {
	path, err := fsguard.ResolveWithinNoEscape(baseDir, relPath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve confined path: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, fmt.Errorf("failed to stat %q: %w", path, err)
	}
}

// Confine verifies candidate is confined within baseDir via
// fsguard.MustBeWithinNoEscape (FR-R18).
func (s *FileConfinedFileStore) Confine(baseDir, candidate string) error {
	return fsguard.MustBeWithinNoEscape(baseDir, candidate)
}

var _ domain.ConfinedFileStore = (*FileConfinedFileStore)(nil)
