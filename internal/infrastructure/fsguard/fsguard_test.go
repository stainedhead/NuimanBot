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
