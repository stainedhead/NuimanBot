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
