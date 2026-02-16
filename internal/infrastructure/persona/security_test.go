package persona

import (
	"errors"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

// --- ValidateUserPath Tests ---

func TestValidateUserPath_ValidPaths(t *testing.T) {
	baseDir := "/data/persona"

	tests := []struct {
		name     string
		userID   string
		filename string
		wantPath string
	}{
		{
			name:     "simple SOUL file",
			userID:   "user123",
			filename: "SOUL.md",
			wantPath: filepath.Join(baseDir, "user123", "SOUL.md"),
		},
		{
			name:     "USER file",
			userID:   "user-456",
			filename: "USER.md",
			wantPath: filepath.Join(baseDir, "user-456", "USER.md"),
		},
		{
			name:     "RULES file",
			userID:   "user_789",
			filename: "RULES.md",
			wantPath: filepath.Join(baseDir, "user_789", "RULES.md"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateUserPath(baseDir, tt.userID, tt.filename)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("got %s, want %s", got, tt.wantPath)
			}
		})
	}
}

func TestValidateUserPath_TraversalInUserID(t *testing.T) {
	baseDir := "/data/persona"

	tests := []struct {
		name   string
		userID string
	}{
		{name: "parent directory", userID: "../etc"},
		{name: "double parent", userID: "../../etc"},
		{name: "triple parent", userID: "../../../etc"},
		{name: "dot-dot only", userID: ".."},
		{name: "traversal in middle", userID: "safe/../../etc"},
		{name: "backslash traversal", userID: `..\..\etc`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateUserPath(baseDir, tt.userID, "SOUL.md")
			if err == nil {
				t.Errorf("expected error for userID %q, got nil", tt.userID)
			}
			if !errors.Is(err, domain.ErrPathTraversal) {
				t.Errorf("expected ErrPathTraversal, got %v", err)
			}
		})
	}
}

func TestValidateUserPath_TraversalInFilename(t *testing.T) {
	baseDir := "/data/persona"

	tests := []struct {
		name     string
		filename string
	}{
		{name: "parent in filename", filename: "../../../etc/passwd"},
		{name: "traversal at start", filename: "../SOUL.md"},
		{name: "traversal in middle", filename: "subdir/../../etc/passwd"},
		{name: "backslash filename", filename: `..\..\etc\passwd`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateUserPath(baseDir, "user123", tt.filename)
			if err == nil {
				t.Errorf("expected error for filename %q, got nil", tt.filename)
			}
			if !errors.Is(err, domain.ErrPathTraversal) {
				t.Errorf("expected ErrPathTraversal, got %v", err)
			}
		})
	}
}

func TestValidateUserPath_EmptyInputs(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		userID   string
		filename string
	}{
		{name: "empty base dir", baseDir: "", userID: "user1", filename: "SOUL.md"},
		{name: "empty userID", baseDir: "/data/persona", userID: "", filename: "SOUL.md"},
		{name: "empty filename", baseDir: "/data/persona", userID: "user1", filename: ""},
		{name: "all empty", baseDir: "", userID: "", filename: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateUserPath(tt.baseDir, tt.userID, tt.filename)
			if err == nil {
				t.Error("expected error for empty input")
			}
			if !errors.Is(err, domain.ErrPathTraversal) {
				t.Errorf("expected ErrPathTraversal, got %v", err)
			}
		})
	}
}

func TestValidateUserPath_NullBytes(t *testing.T) {
	baseDir := "/data/persona"

	tests := []struct {
		name     string
		userID   string
		filename string
	}{
		{name: "null in userID", userID: "user\x001", filename: "SOUL.md"},
		{name: "null in filename", userID: "user1", filename: "SOUL\x00.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateUserPath(baseDir, tt.userID, tt.filename)
			if err == nil {
				t.Error("expected error for null byte")
			}
			if !errors.Is(err, domain.ErrPathTraversal) {
				t.Errorf("expected ErrPathTraversal, got %v", err)
			}
		})
	}
}

func TestValidateUserPath_ReturnsExactError(t *testing.T) {
	// Verify direct equality (not just errors.Is) for callers that use !=
	_, err := ValidateUserPath("/data/persona", "../etc", "passwd")
	if err != domain.ErrPathTraversal {
		t.Errorf("expected exact ErrPathTraversal, got %v", err)
	}
}

// --- ValidateExtension Tests ---

func TestValidateExtension_AllowedExtensions(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "markdown", path: "SOUL.md"},
		{name: "text", path: "notes.txt"},
		{name: "json", path: "config.json"},
		{name: "uppercase markdown", path: "RULES.MD"},
		{name: "nested markdown", path: "subdir/file.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtension(tt.path)
			if err != nil {
				t.Errorf("unexpected error for path %q: %v", tt.path, err)
			}
		})
	}
}

func TestValidateExtension_BlockedExtensions(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "executable", path: "malware.exe"},
		{name: "shell script", path: "hack.sh"},
		{name: "binary", path: "data.bin"},
		{name: "go source", path: "main.go"},
		{name: "python", path: "script.py"},
		{name: "yaml", path: "config.yaml"},
		{name: "no extension", path: "Makefile"},
		{name: "hidden file", path: ".secret"},
		{name: "double extension", path: "file.md.exe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExtension(tt.path)
			if err == nil {
				t.Errorf("expected error for path %q, got nil", tt.path)
			}
		})
	}
}

func TestValidateExtension_EmptyPath(t *testing.T) {
	err := ValidateExtension("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

// --- containsTraversal helper tests ---

func TestContainsTraversal(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "safe", want: false},
		{path: "safe/path", want: false},
		{path: ".hidden", want: false},
		{path: "...", want: false},
		{path: "..", want: true},
		{path: "../etc", want: true},
		{path: "safe/../etc", want: true},
		{path: `..\..\etc`, want: true},
		{path: `safe\..\..\etc`, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := containsTraversal(tt.path)
			if got != tt.want {
				t.Errorf("containsTraversal(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestContainsNullByte(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "safe", want: false},
		{input: "", want: false},
		{input: "has\x00null", want: true},
		{input: "\x00", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := containsNullByte(tt.input)
			if got != tt.want {
				t.Errorf("containsNullByte(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
