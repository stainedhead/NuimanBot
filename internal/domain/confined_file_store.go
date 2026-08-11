package domain

// ConfinedFileStore provides filesystem operations for a base directory
// (baseDir) plus a caller-supplied relative path, with the confinement
// guarantee that the resolved path can never escape baseDir — including via
// symlink indirection (FR-R6). It exists so the usecase layer (jobs,
// chores, projects) never imports "os" or internal/infrastructure/fsguard
// directly, per AGENTS.md's Clean Architecture dependency rule: inner
// layers define interfaces, outer layers implement them (FR-R5).
//
// Implemented by internal/infrastructure/storage.FileConfinedFileStore,
// which resolves relPath via fsguard.ResolveWithin and closes the
// symlink-escape gap fsguard.ResolveWithin's own doc comment flags as the
// caller's responsibility.
type ConfinedFileStore interface {
	// EnsureDir creates dir, and any missing parents, if it does not
	// already exist. dir is a plain absolute directory path supplied by
	// the caller (not resolved against a separate base) — equivalent to
	// os.MkdirAll(dir, 0755).
	EnsureDir(dir string) error

	// WriteFile resolves relPath against baseDir with confinement and
	// writes data to it, creating or truncating the file. Returns
	// fsguard.ErrPathEscape (wrapped) if relPath would resolve outside
	// baseDir, including via a symlink encountered while resolving an
	// existing ancestor of the target path.
	WriteFile(baseDir, relPath string, data []byte) error

	// FileExists reports whether relPath, resolved against baseDir with
	// confinement, currently exists. Returns fsguard.ErrPathEscape
	// (wrapped) under the same conditions as WriteFile.
	FileExists(baseDir, relPath string) (bool, error)

	// Confine verifies that candidate — an absolute directory entirely
	// determined by the caller (e.g. a Project's user-requested
	// OutputDirectory), not a relative path this Service constructs
	// itself — is confined within baseDir, including symlink-escape
	// safety (FR-R18). Returns fsguard.ErrPathEscape (wrapped) if
	// candidate is not baseDir or a descendant of it, whether by lexical
	// traversal, an absolute path outside baseDir, or a symlink.
	Confine(baseDir, candidate string) error
}
