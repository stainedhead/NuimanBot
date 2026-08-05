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

func TestFileConfinedFileStore_FileExists_RejectsEscape(t *testing.T) {
	root := t.TempDir()
	s := NewFileConfinedFileStore()

	_, err := s.FileExists(root, "../../etc/passwd")
	if !errors.Is(err, fsguard.ErrPathEscape) {
		t.Fatalf("expected ErrPathEscape, got %v", err)
	}
}
