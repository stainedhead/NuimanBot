// Package fsguard provides a single choke-point for confining filesystem
// path resolution to a base directory. It exists because no reusable
// path-confinement helper exists elsewhere in the codebase (verified during
// spec review of specs/260805-nuimanbot-extend-context-and-ui/: the
// existing internal/infrastructure/preprocess.CommandSandbox sandboxes
// *command execution*, not filesystem path containment; internal/config's
// FetchSecurityConfig guards SSRF/network targets, not local paths).
//
// Every Job/Chore/Project filesystem operation added by that feature must
// resolve user- or agent-supplied relative paths through ResolveWithin
// rather than joining paths directly, to prevent path traversal outside the
// assigned Project's output directory (a security-critical requirement, not
// a stretch goal).
package fsguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscape is returned when a requested relative path would resolve
// outside the confining base directory.
var ErrPathEscape = errors.New("path escapes the confined base directory")

// ErrInvalidPath is returned when relPath is structurally invalid (e.g.
// contains a NUL byte).
var ErrInvalidPath = errors.New("invalid path")

// ResolveWithin resolves relPath against baseDir and returns the resulting
// absolute path, guaranteeing the result is baseDir itself or a descendant
// of it. It rejects:
//   - absolute relPath values (an absolute path is never "relative to
//     baseDir" by definition; treating it as such would let a caller
//     bypass confinement entirely),
//   - any relPath whose lexical resolution (after filepath.Clean) walks
//     above baseDir via "..",
//   - paths containing a NUL byte (invalid on all supported platforms and
//     a classic string-truncation attack vector in C-based syscalls).
//
// baseDir itself is not required to exist; ResolveWithin is a pure path
// computation and performs no filesystem I/O (in particular, it does not
// resolve symlinks — callers that subsequently open the returned path
// should use O_NOFOLLOW-equivalent care, or resolve baseDir itself via
// filepath.EvalSymlinks before calling ResolveWithin, if symlink escape
// inside an untrusted baseDir is a concern for that call site).
func ResolveWithin(baseDir, relPath string) (string, error) {
	if strings.ContainsRune(baseDir, 0) || strings.ContainsRune(relPath, 0) {
		return "", fmt.Errorf("%w: NUL byte in path", ErrInvalidPath)
	}

	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("fsguard: resolving base directory: %w", err)
	}

	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: %q is an absolute path", ErrPathEscape, relPath)
	}

	joined := filepath.Join(absBase, relPath)

	if joined != absBase && !strings.HasPrefix(joined, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q resolves outside %q", ErrPathEscape, relPath, absBase)
	}

	return joined, nil
}

// ResolveWithinNoEscape behaves like ResolveWithin, with one additional
// guarantee (FR-R6): it also rejects a relPath that would escape baseDir
// via a symlink planted somewhere along the resolved path — either an
// intermediate directory or the final path component itself. ResolveWithin
// alone is a pure lexical computation (per its own doc comment) and cannot
// see the filesystem, so a symlink written inside baseDir — e.g. by a
// coding agent operating in a Project's OutputDirectory — can silently
// redirect a later filesystem operation outside baseDir even though the
// lexical path never appears to leave it.
//
// baseDir need not exist: if it doesn't, nothing can have been planted
// beneath it, so the result is identical to ResolveWithin's plain lexical
// join (this matters for callers like a repository's first write for a
// brand-new owner, whose directory doesn't exist yet). Likewise, any
// trailing component of relPath that doesn't yet exist on disk is treated
// as a literal path segment, never as a symlink — it cannot be one, since
// nothing has been created there yet. This makes the function safe to call
// before creating a new file, unlike a naive filepath.EvalSymlinks on the
// full target path, which errors with ENOENT for every such call.
//
// Scope: applied at the confined-write call sites this fix pass targets
// (domain.ConfinedFileStore's implementation, FileProjectRepository, and
// the live Executor) — see specs/260805-nuimanbot-extend-context-and-ui-auto-review's
// FR-R6. Record-path resolution keyed purely by server-generated UUIDs
// (Job/Chore/Run repositories) still uses plain ResolveWithin, since an
// ID's only user-influenced content is that it must parse as a UUID.
func ResolveWithinNoEscape(baseDir, relPath string) (string, error) {
	joined, err := ResolveWithin(baseDir, relPath)
	if err != nil {
		return "", err
	}

	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("fsguard: resolving base directory: %w", err)
	}

	rel, err := filepath.Rel(absBase, joined)
	if err != nil {
		return "", fmt.Errorf("fsguard: computing relative path: %w", err)
	}
	if rel == "." {
		return joined, nil
	}

	// boundaryReal is the confinement boundary's real, symlink-resolved
	// location, used only to validate an encountered symlink (or the
	// fully-resolved final path below) — never to reshape the return
	// value when no symlink is actually involved, so an OS-level
	// indirection on baseDir itself (e.g. a symlinked temp/mount root)
	// can't produce a false-positive escape or an unexpectedly-rewritten
	// result for the common, symlink-free case.
	boundaryReal := absBase
	if realBase, evalErr := filepath.EvalSymlinks(absBase); evalErr == nil {
		boundaryReal = realBase
	} else if !os.IsNotExist(evalErr) {
		return "", fmt.Errorf("fsguard: resolving base directory: %w", evalErr)
	}

	current := absBase
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		candidate := filepath.Join(current, part)
		info, statErr := os.Lstat(candidate)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				// Nothing exists from here down; the remaining components
				// cannot be symlinks (there's nothing on disk yet for them
				// to redirect). Join them literally and return — safe by
				// construction, nothing left to compare against.
				return filepath.Join(append([]string{current}, parts[i:]...)...), nil
			}
			return "", fmt.Errorf("fsguard: checking %q: %w", candidate, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, evalErr := filepath.EvalSymlinks(candidate)
			if evalErr != nil {
				return "", fmt.Errorf("fsguard: resolving symlink %q: %w", candidate, evalErr)
			}
			if resolved != boundaryReal && !strings.HasPrefix(resolved, boundaryReal+string(filepath.Separator)) {
				return "", fmt.Errorf("%w: %q escapes %q via symlink", ErrPathEscape, candidate, boundaryReal)
			}
			current = resolved
		} else {
			current = candidate
		}
	}

	// Every component of relPath exists on disk (the loop above completed
	// without an early return). Resolve the fully literal result for the
	// final containment check, so a symlinked baseDir mount alone (with no
	// symlink anywhere in relPath) can't produce a false positive — but
	// still return the literal, pre-this-resolution path, matching
	// ResolveWithin's contract for the symlink-free case.
	realCurrent, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", fmt.Errorf("fsguard: resolving final path %q: %w", current, err)
	}
	if realCurrent != boundaryReal && !strings.HasPrefix(realCurrent, boundaryReal+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: resolved path %q escapes %q", ErrPathEscape, realCurrent, boundaryReal)
	}

	return current, nil
}

// MustBeWithin is a convenience wrapper for call sites that already have an
// absolute candidate path (e.g. derived from os.Getwd or an existing
// on-disk entity) and need to assert it falls within baseDir, without
// re-deriving it from a relative path. Returns ErrPathEscape if candidate
// is not baseDir or a descendant of it.
func MustBeWithin(baseDir, candidate string) error {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return fmt.Errorf("fsguard: resolving base directory: %w", err)
	}
	absCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return fmt.Errorf("fsguard: resolving candidate path: %w", err)
	}
	if absCandidate != absBase && !strings.HasPrefix(absCandidate, absBase+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q is not within %q", ErrPathEscape, candidate, absBase)
	}
	return nil
}
