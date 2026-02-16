package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// validPersonaFile returns a PersonaFile with all valid fields for testing.
func validPersonaFile() *PersonaFile {
	return &PersonaFile{
		UserID:     "user-123",
		Type:       PersonaFileSOUL,
		Path:       "/data/users/user-123/SOUL.md",
		Content:    "# My Soul\nI am a helpful assistant.",
		ModifiedAt: time.Now(),
		SizeBytes:  42,
	}
}

func TestPersonaFileType_String(t *testing.T) {
	tests := []struct {
		name     string
		fileType PersonaFileType
		want     string
	}{
		{name: "SOUL", fileType: PersonaFileSOUL, want: "SOUL"},
		{name: "USER", fileType: PersonaFileUSER, want: "USER"},
		{name: "RULES", fileType: PersonaFileRULES, want: "RULES"},
		{name: "invalid type", fileType: PersonaFileType(99), want: "PersonaFileType(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fileType.String()
			if got != tt.want {
				t.Errorf("PersonaFileType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersonaFileType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		fileType PersonaFileType
		want     bool
	}{
		{name: "SOUL is valid", fileType: PersonaFileSOUL, want: true},
		{name: "USER is valid", fileType: PersonaFileUSER, want: true},
		{name: "RULES is valid", fileType: PersonaFileRULES, want: true},
		{name: "negative is invalid", fileType: PersonaFileType(-1), want: false},
		{name: "out of range is invalid", fileType: PersonaFileType(3), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fileType.IsValid()
			if got != tt.want {
				t.Errorf("PersonaFileType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersonaFileType_Filename(t *testing.T) {
	tests := []struct {
		name     string
		fileType PersonaFileType
		want     string
	}{
		{name: "SOUL filename", fileType: PersonaFileSOUL, want: PersonaFilenameSOUL},
		{name: "USER filename", fileType: PersonaFileUSER, want: PersonaFilenameUSER},
		{name: "RULES filename", fileType: PersonaFileRULES, want: PersonaFilenameRULES},
		{name: "invalid type returns empty", fileType: PersonaFileType(-1), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fileType.Filename()
			if got != tt.want {
				t.Errorf("PersonaFileType.Filename() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersonaFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*PersonaFile)
		wantErr string
	}{
		{
			name:    "valid persona file",
			modify:  func(_ *PersonaFile) {},
			wantErr: "",
		},
		{
			name:    "empty UserID",
			modify:  func(pf *PersonaFile) { pf.UserID = "" },
			wantErr: "userID is required",
		},
		{
			name:    "UserID too long",
			modify:  func(pf *PersonaFile) { pf.UserID = strings.Repeat("a", 65) },
			wantErr: "userID must be <= 64 characters",
		},
		{
			name:    "UserID at max length is valid",
			modify:  func(pf *PersonaFile) { pf.UserID = strings.Repeat("a", 64) },
			wantErr: "",
		},
		{
			name:    "invalid PersonaFileType negative",
			modify:  func(pf *PersonaFile) { pf.Type = PersonaFileType(-1) },
			wantErr: "invalid persona file type",
		},
		{
			name:    "invalid PersonaFileType out of range",
			modify:  func(pf *PersonaFile) { pf.Type = PersonaFileType(99) },
			wantErr: "invalid persona file type",
		},
		{
			name:    "empty path",
			modify:  func(pf *PersonaFile) { pf.Path = "" },
			wantErr: "path is required",
		},
		{
			name:    "relative path",
			modify:  func(pf *PersonaFile) { pf.Path = "relative/path/SOUL.md" },
			wantErr: "path must be absolute",
		},
		{
			name:    "content exceeds 100KB",
			modify:  func(pf *PersonaFile) { pf.Content = strings.Repeat("a", MaxPersonaFileSize+1) },
			wantErr: "content must be <= 100KB",
		},
		{
			name:    "content at max size is valid",
			modify:  func(pf *PersonaFile) { pf.Content = strings.Repeat("a", MaxPersonaFileSize) },
			wantErr: "",
		},
		{
			name:    "invalid UTF-8 content",
			modify:  func(pf *PersonaFile) { pf.Content = string([]byte{0xff, 0xfe, 0xfd}) },
			wantErr: "content must be valid UTF-8",
		},
		{
			name:    "zero ModifiedAt",
			modify:  func(pf *PersonaFile) { pf.ModifiedAt = time.Time{} },
			wantErr: "modifiedAt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := validPersonaFile()
			tt.modify(pf)
			err := pf.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				return
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPersonaFile_IsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "empty string", content: "", want: true},
		{name: "whitespace only spaces", content: "   ", want: true},
		{name: "whitespace only tabs", content: "\t\t", want: true},
		{name: "whitespace only newlines", content: "\n\n\n", want: true},
		{name: "mixed whitespace", content: " \t\n \r\n ", want: true},
		{name: "non-empty content", content: "hello", want: false},
		{name: "content with leading whitespace", content: "  hello  ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PersonaFile{Content: tt.content}
			got := pf.IsEmpty()
			if got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersonaFile_TokenCount(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantMin int
		wantMax int
	}{
		{name: "empty content", content: "", wantMin: 0, wantMax: 0},
		{name: "short content", content: "hello world", wantMin: 1, wantMax: 10},
		{name: "longer content", content: strings.Repeat("word ", 100), wantMin: 50, wantMax: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &PersonaFile{Content: tt.content}
			got := pf.TokenCount()
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("TokenCount() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPersonaFile_TokenCount_Proportional(t *testing.T) {
	short := &PersonaFile{Content: "hello"}
	long := &PersonaFile{Content: strings.Repeat("hello ", 100)}

	if long.TokenCount() <= short.TokenCount() {
		t.Errorf("longer content should have more tokens: short=%d, long=%d",
			short.TokenCount(), long.TokenCount())
	}
}

// Ensure the test helper produces valid UTF-8
func TestValidPersonaFile_ContentIsValidUTF8(t *testing.T) {
	pf := validPersonaFile()
	if !utf8.ValidString(pf.Content) {
		t.Error("validPersonaFile() content is not valid UTF-8")
	}
}

func TestPersonaFileConstants(t *testing.T) {
	if MaxPersonaFileSize != 100*1024 {
		t.Errorf("MaxPersonaFileSize = %d, want %d", MaxPersonaFileSize, 100*1024)
	}
	if PersonaFilenameSOUL != "SOUL.md" {
		t.Errorf("PersonaFilenameSOUL = %q, want %q", PersonaFilenameSOUL, "SOUL.md")
	}
	if PersonaFilenameUSER != "USER.md" {
		t.Errorf("PersonaFilenameUSER = %q, want %q", PersonaFilenameUSER, "USER.md")
	}
	if PersonaFilenameRULES != "RULES.md" {
		t.Errorf("PersonaFilenameRULES = %q, want %q", PersonaFilenameRULES, "RULES.md")
	}
}

func TestPersonaFileErrors(t *testing.T) {
	if ErrPersonaFileNotFound == nil {
		t.Error("ErrPersonaFileNotFound should not be nil")
	}
	if ErrInvalidPersonaFileType == nil {
		t.Error("ErrInvalidPersonaFileType should not be nil")
	}
	if ErrPathTraversal == nil {
		t.Error("ErrPathTraversal should not be nil")
	}
}
