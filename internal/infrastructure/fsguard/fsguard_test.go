package fsguard

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveWithin_SimpleRelativePath(t *testing.T) {
	base := t.TempDir()
	got, err := ResolveWithin(base, "notes.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "notes.md")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWithin_NestedRelativePath(t *testing.T) {
	base := t.TempDir()
	got, err := ResolveWithin(base, filepath.Join("sub", "dir", "file.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "sub", "dir", "file.txt")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWithin_EmptyRelPathReturnsBase(t *testing.T) {
	base := t.TempDir()
	got, err := ResolveWithin(base, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs(base)
	if got != want {
		t.Fatalf("expected base dir %q, got %q", want, got)
	}
}

func TestResolveWithin_DotCurrentDirReturnsBase(t *testing.T) {
	base := t.TempDir()
	got, err := ResolveWithin(base, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := filepath.Abs(base)
	if got != want {
		t.Fatalf("expected base dir %q, got %q", want, got)
	}
}

func TestResolveWithin_ParentTraversalRejected(t *testing.T) {
	base := t.TempDir()
	cases := []string{
		"../escape.txt",
		"../../etc/passwd",
		filepath.Join("sub", "..", "..", "escape.txt"),
		"..",
	}
	for _, rel := range cases {
		_, err := ResolveWithin(base, rel)
		if !errors.Is(err, ErrPathEscape) {
			t.Errorf("relPath %q: expected ErrPathEscape, got %v", rel, err)
		}
	}
}

func TestResolveWithin_ParentTraversalThatStaysInsideIsAllowed(t *testing.T) {
	// sub/../file.txt lexically resolves to base/file.txt, which IS inside
	// base — this must be allowed (a naive substring-based ".." rejection
	// would incorrectly reject this).
	base := t.TempDir()
	got, err := ResolveWithin(base, filepath.Join("sub", "..", "file.txt"))
	if err != nil {
		t.Fatalf("unexpected error for traversal that nets out inside base: %v", err)
	}
	want := filepath.Join(base, "file.txt")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWithin_AbsolutePathRejected(t *testing.T) {
	base := t.TempDir()
	var abs string
	if runtime.GOOS == "windows" {
		abs = `C:\Windows\System32\config`
	} else {
		abs = "/etc/passwd"
	}
	_, err := ResolveWithin(base, abs)
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape for absolute relPath, got %v", err)
	}
}

func TestResolveWithin_NulByteRejected(t *testing.T) {
	base := t.TempDir()
	_, err := ResolveWithin(base, "file\x00.txt")
	if !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath for NUL byte, got %v", err)
	}
}

func TestResolveWithin_SiblingDirectoryPrefixNotConfused(t *testing.T) {
	// Regression test for a common bug: naive strings.HasPrefix(joined,
	// base) without a trailing separator would incorrectly allow
	// "/tmp/project-evil" to pass a check against base "/tmp/project".
	parent := t.TempDir()
	base := filepath.Join(parent, "project")
	sibling := filepath.Join(parent, "project-evil")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0755); err != nil {
		t.Fatal(err)
	}

	// Constructing a relPath that would lexically land in the sibling
	// directory requires escaping base first, which must be rejected.
	_, err := ResolveWithin(base, filepath.Join("..", "project-evil", "secret.txt"))
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape for sibling-directory escape, got %v", err)
	}
}

func TestMustBeWithin_InsideBase(t *testing.T) {
	base := t.TempDir()
	candidate := filepath.Join(base, "a", "b.txt")
	if err := MustBeWithin(base, candidate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMustBeWithin_BaseItself(t *testing.T) {
	base := t.TempDir()
	if err := MustBeWithin(base, base); err != nil {
		t.Fatalf("unexpected error for base dir itself: %v", err)
	}
}

func TestMustBeWithin_OutsideBase(t *testing.T) {
	parent := t.TempDir()
	base := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "other", "secret.txt")
	if err := MustBeWithin(base, outside); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}

// --- ResolveWithinNoEscape (FR-R6: symlink-escape mitigation) ---

func TestResolveWithinNoEscape_RejectsSymlinkEscapeViaIntermediateDir(t *testing.T) {
	// Adversarial case: an intermediate path component inside base is a
	// symlink pointing outside base. A pure lexical check (ResolveWithin)
	// cannot see this — the joined path never lexically leaves base — but
	// the OS would follow the symlink at open/write time and escape the
	// sandbox.
	parent := t.TempDir()
	base := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveWithinNoEscape(base, filepath.Join("escape", "pwned.txt"))
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "pwned.txt")); !os.IsNotExist(statErr) {
		t.Fatal("expected no file to have been created outside base via the symlink")
	}
}

func TestResolveWithinNoEscape_RejectsSymlinkAtFinalComponent(t *testing.T) {
	// The target filename itself is a symlink to outside base (e.g.
	// planted by a prior agent run) rather than an intermediate directory.
	parent := t.TempDir()
	base := filepath.Join(parent, "project")
	outsideFile := filepath.Join(parent, "secret.txt")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(base, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveWithinNoEscape(base, "AGENTS.md")
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}

func TestResolveWithinNoEscape_AllowsLegitimateInRootSymlink(t *testing.T) {
	// A symlink whose target is itself inside base must be allowed. The
	// returned path is asserted behaviorally (does it actually reach the
	// right file?) rather than by exact string match, since resolving a
	// real symlink necessarily yields the symlink's resolved target
	// location — which, on some platforms (e.g. macOS's /var ->
	// /private/var), legitimately differs textually from the literal
	// path the test constructed, without that being any kind of escape.
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(base, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWithinNoEscape(base, filepath.Join("alias", "file.txt"))
	if err != nil {
		t.Fatalf("expected in-root symlink to be allowed, got err: %v", err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("expected resolved path to reach the real file, got err: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected resolved path to read back %q, got %q", "hello", string(data))
	}
}

func TestResolveWithinNoEscape_AllowsCreatingNewFileAndNewSubdirs(t *testing.T) {
	// The advisor's trap: naively calling filepath.EvalSymlinks on the
	// full target path fails with ENOENT for any not-yet-created file —
	// which is every file-creation call site. Components that don't yet
	// exist on disk cannot be symlinks, so they must be treated as
	// literal path segments, not rejected or errored.
	base := t.TempDir()

	got, err := ResolveWithinNoEscape(base, "notes.md")
	if err != nil {
		t.Fatalf("expected new top-level file to be allowed, got err: %v", err)
	}
	if want := filepath.Join(base, "notes.md"); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	got, err = ResolveWithinNoEscape(base, filepath.Join("new", "nested", "dir", "file.txt"))
	if err != nil {
		t.Fatalf("expected new nested path to be allowed, got err: %v", err)
	}
	if want := filepath.Join(base, "new", "nested", "dir", "file.txt"); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWithinNoEscape_BaseDirDoesNotExistYet(t *testing.T) {
	// Nothing can have been planted inside a base directory that doesn't
	// exist yet (e.g. a brand-new user's first record write) — this must
	// behave exactly like the lexical-only ResolveWithin, not error.
	parent := t.TempDir()
	base := filepath.Join(parent, "brand-new-user", "projects")

	got, err := ResolveWithinNoEscape(base, "project-1.json")
	if err != nil {
		t.Fatalf("expected non-existent base dir to be allowed, got err: %v", err)
	}
	if want := filepath.Join(base, "project-1.json"); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveWithinNoEscape_ParentTraversalStillRejected(t *testing.T) {
	base := t.TempDir()
	_, err := ResolveWithinNoEscape(base, "../../etc/passwd")
	if !errors.Is(err, ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}

func TestResolveWithinNoEscape_SymlinkLoopAsBaseDirErrors(t *testing.T) {
	parent := t.TempDir()
	loop := filepath.Join(parent, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveWithinNoEscape(loop, "x.txt"); err == nil {
		t.Fatal("expected an error resolving a self-referential symlink base directory")
	}
}

func TestResolveWithinNoEscape_SymlinkLoopAtIntermediateComponentErrors(t *testing.T) {
	base := t.TempDir()
	loop := filepath.Join(base, "loop")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveWithinNoEscape(base, filepath.Join("loop", "file.txt")); err == nil {
		t.Fatal("expected an error resolving a self-referential intermediate symlink")
	}
}

func TestResolveWithinNoEscape_NonDirectoryIntermediateComponentErrors(t *testing.T) {
	// An intermediate path component that exists but is a regular file
	// (not a directory) hits a different OS-level error (ENOTDIR) than
	// "does not exist" (ENOENT) — must not be silently treated as the
	// safe not-yet-created case.
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "regular-file"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveWithinNoEscape(base, filepath.Join("regular-file", "child.txt")); err == nil {
		t.Fatal("expected an error when an intermediate component is not a directory")
	}
}

func TestResolveWithinNoEscape_ExistingRegularFileOverwriteAllowed(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "AGENTS.md")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveWithinNoEscape(base, "AGENTS.md")
	if err != nil {
		t.Fatalf("expected overwrite of existing regular file to be allowed, got err: %v", err)
	}
	if got != target {
		t.Fatalf("expected %q, got %q", target, got)
	}
}
