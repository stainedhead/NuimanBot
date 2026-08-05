package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/infrastructure/fsguard"
)

func TestFileConfinedFileStore_EnsureDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b", "c")
	s := NewFileConfinedFileStore()

	if err := s.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected a directory")
	}

	// Idempotent: calling again on an already-existing dir is not an error.
	if err := s.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir (idempotent): %v", err)
	}
}

func TestFileConfinedFileStore_WriteFile(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()

	if err := s.WriteFile(root, "notes.md", []byte("hello")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "notes.md"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(got))
	}
}

func TestFileConfinedFileStore_WriteFile_RejectsEscape(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()

	err := s.WriteFile(root, "../outside.md", []byte("evil"))
	if !errors.Is(err, fsguard.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(root), "outside.md")); !os.IsNotExist(statErr) {
		t.Fatal("expected no file to have been written outside root")
	}
}

func TestFileConfinedFileStore_WriteFile_RejectsSymlinkEscape(t *testing.T) {
	// FR-R6 regression at the actual call site jobs/chores/projects
	// depend on (via domain.ConfinedFileStore): a symlink planted inside
	// root — e.g. by a coding agent writing into a Project's
	// OutputDirectory during a prior run — must not let a later write
	// escape root, even though the lexical relPath never leaves it.
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	outside := filepath.Join(parent, "outside")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	s := NewFileConfinedFileStore()
	err := s.WriteFile(root, filepath.Join("escape", "pwned.md"), []byte("evil"))
	if !errors.Is(err, fsguard.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "pwned.md")); !os.IsNotExist(statErr) {
		t.Fatal("expected no file to have been written outside root via the symlink")
	}
}

func TestFileConfinedFileStore_FileExists(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()

	exists, err := s.FileExists(root, "missing.md")
	if err != nil {
		t.Fatalf("FileExists: %v", err)
	}
	if exists {
		t.Fatal("expected missing.md to not exist")
	}

	if err := os.WriteFile(filepath.Join(root, "present.md"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to set up fixture: %v", err)
	}
	exists, err = s.FileExists(root, "present.md")
	if err != nil {
		t.Fatalf("FileExists: %v", err)
	}
	if !exists {
		t.Fatal("expected present.md to exist")
	}
}

func TestFileConfinedFileStore_Confine_AllowsWithinRoot(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()
	candidate := filepath.Join(root, "users", "alice", "projects", "p1")

	if err := s.Confine(root, candidate); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileConfinedFileStore_Confine_RejectsOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	outside := filepath.Join(parent, "outside")
	s := NewFileConfinedFileStore()

	if err := s.Confine(root, outside); !errors.Is(err, fsguard.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}

func TestFileConfinedFileStore_FileExists_RejectsEscape(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()

	_, err := s.FileExists(root, "../../etc/passwd")
	if !errors.Is(err, fsguard.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}
