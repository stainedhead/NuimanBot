package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

func TestScan_SingleDirectory(t *testing.T) {
	// Create temp directory with one valid skill
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: test-skill
description: A test skill
---

Body content
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Scan
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("Scan() returned %d skills, want 1", len(skills))
	}

	skill := skills[0]
	if skill.Name != "test-skill" {
		t.Errorf("Skill name = %q, want %q", skill.Name, "test-skill")
	}

	if skill.Scope != domain.ScopeProject {
		t.Errorf("Skill scope = %v, want %v", skill.Scope, domain.ScopeProject)
	}
}

func TestScan_MultipleDirectories(t *testing.T) {
	// Create temp directory with multiple skills
	tmpDir := t.TempDir()

	// Skill 1
	skillDir1 := filepath.Join(tmpDir, "skill-one")
	if err := os.Mkdir(skillDir1, 0755); err != nil {
		t.Fatalf("Failed to create skill directory 1: %v", err)
	}
	skillFile1 := filepath.Join(skillDir1, "SKILL.md")
	content1 := `---
name: skill-one
description: First skill
---

Body 1
`
	if err := os.WriteFile(skillFile1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md 1: %v", err)
	}

	// Skill 2
	skillDir2 := filepath.Join(tmpDir, "skill-two")
	if err := os.Mkdir(skillDir2, 0755); err != nil {
		t.Fatalf("Failed to create skill directory 2: %v", err)
	}
	skillFile2 := filepath.Join(skillDir2, "SKILL.md")
	content2 := `---
name: skill-two
description: Second skill
---

Body 2
`
	if err := os.WriteFile(skillFile2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md 2: %v", err)
	}

	// Scan
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeUser},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("Scan() returned %d skills, want 2", len(skills))
	}
}

func TestScan_MultipleRoots(t *testing.T) {
	// Create two temp directories
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	// Skill in root 1
	skillDir1 := filepath.Join(tmpDir1, "skill-one")
	if err := os.Mkdir(skillDir1, 0755); err != nil {
		t.Fatalf("Failed to create skill directory 1: %v", err)
	}
	skillFile1 := filepath.Join(skillDir1, "SKILL.md")
	content1 := `---
name: skill-one
description: First skill
---

Body 1
`
	if err := os.WriteFile(skillFile1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md 1: %v", err)
	}

	// Skill in root 2
	skillDir2 := filepath.Join(tmpDir2, "skill-two")
	if err := os.Mkdir(skillDir2, 0755); err != nil {
		t.Fatalf("Failed to create skill directory 2: %v", err)
	}
	skillFile2 := filepath.Join(skillDir2, "SKILL.md")
	content2 := `---
name: skill-two
description: Second skill
---

Body 2
`
	if err := os.WriteFile(skillFile2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md 2: %v", err)
	}

	// Scan with different scopes
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir1, Scope: domain.ScopeEnterprise},
		{Path: tmpDir2, Scope: domain.ScopeUser},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("Scan() returned %d skills, want 2", len(skills))
	}

	// Verify scopes
	for _, skill := range skills {
		if skill.Name == "skill-one" && skill.Scope != domain.ScopeEnterprise {
			t.Errorf("skill-one has scope %v, want %v", skill.Scope, domain.ScopeEnterprise)
		}
		if skill.Name == "skill-two" && skill.Scope != domain.ScopeUser {
			t.Errorf("skill-two has scope %v, want %v", skill.Scope, domain.ScopeUser)
		}
	}
}

func TestScan_NonExistentRoot(t *testing.T) {
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: "/nonexistent/path/to/skills", Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() should skip nonexistent roots gracefully, got error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Scan() returned %d skills from nonexistent root, want 0", len(skills))
	}
}

func TestScan_MissingSkillMD(t *testing.T) {
	// Create temp directory with directory but no SKILL.md
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "no-skill-md")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create other file (not SKILL.md)
	readmeFile := filepath.Join(skillDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# README"), 0644); err != nil {
		t.Fatalf("Failed to write README.md: %v", err)
	}

	// Scan
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Scan() returned %d skills, want 0 (should skip dirs without SKILL.md)", len(skills))
	}
}

func TestScan_InvalidSkill(t *testing.T) {
	// Create temp directory with invalid skill
	tmpDir := t.TempDir()

	skillDir := filepath.Join(tmpDir, "invalid-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: invalid-skill
---

Missing description should cause validation error
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Scan (should skip invalid skill with warning, not fail)
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() should not fail on invalid skill, got error: %v", err)
	}

	// Invalid skill should be skipped
	if len(skills) != 0 {
		t.Errorf("Scan() returned %d skills, want 0 (invalid skills should be skipped)", len(skills))
	}
}

func TestScan_SkipsFiles(t *testing.T) {
	// Create temp directory with a file (not directory) at top level
	tmpDir := t.TempDir()

	// Create a file (not directory)
	file := filepath.Join(tmpDir, "not-a-directory.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Scan
	repo := NewFilesystemSkillRepository()
	roots := []domain.SkillRoot{
		{Path: tmpDir, Scope: domain.ScopeProject},
	}

	skills, err := repo.Scan(roots)
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("Scan() returned %d skills, want 0 (should skip files)", len(skills))
	}
}

func TestLoad_ValidSkill(t *testing.T) {
	// Create temp skill
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
name: test-skill
description: A test skill
---

Body content
`
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Load
	repo := NewFilesystemSkillRepository()
	skill, err := repo.Load(skillFile)

	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if skill.Name != "test-skill" {
		t.Errorf("Skill name = %q, want %q", skill.Name, "test-skill")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	repo := NewFilesystemSkillRepository()
	_, err := repo.Load("/nonexistent/SKILL.md")

	if err == nil {
		t.Fatal("Load() expected error for nonexistent file, got nil")
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains string // Check if result contains this string
	}{
		{
			name:     "Empty path",
			input:    "",
			wantErr:  false,
			contains: "",
		},
		{
			name:     "Absolute path",
			input:    "/absolute/path",
			wantErr:  false,
			contains: "/absolute/path",
		},
		{
			name:     "Relative path",
			input:    "relative/path",
			wantErr:  false,
			contains: "relative/path",
		},
		{
			name:    "Tilde only",
			input:   "~",
			wantErr: false,
			// Result should be home dir (can't know exact value)
		},
		{
			name:    "Tilde with path",
			input:   "~/skills",
			wantErr: false,
			// Result should contain "skills" at the end
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandHome(tt.input)

			if tt.wantErr && err == nil {
				t.Error("expandHome() expected error, got nil")
				return
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expandHome() unexpected error: %v", err)
				return
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expandHome() = %q, want to contain %q", result, tt.contains)
			}

			// Special checks for tilde expansion
			if tt.input == "~" && result == "" {
				t.Error("expandHome(~) returned empty string, expected home directory")
			}

			if tt.input == "~/skills" && !strings.HasSuffix(result, "skills") {
				t.Errorf("expandHome(~/skills) = %q, expected to end with 'skills'", result)
			}
		})
	}
}
