package config

import (
	"os"
	"path/filepath"
	"testing"

	"nuimanbot/internal/domain"
)

func TestSkillsConfig_Defaults(t *testing.T) {
	cfg := DefaultSkillsConfig()

	if !cfg.Enabled {
		t.Error("Skills should be enabled by default")
	}

	if len(cfg.Roots) == 0 {
		t.Error("Should have default skill roots")
	}

	// Should have at least user and project scopes
	hasUser := false
	hasProject := false
	for _, root := range cfg.Roots {
		if root.Scope == domain.ScopeUser {
			hasUser = true
		}
		if root.Scope == domain.ScopeProject {
			hasProject = true
		}
	}

	if !hasUser {
		t.Error("Should have user scope root by default")
	}

	if !hasProject {
		t.Error("Should have project scope root by default")
	}
}

func TestSkillRootConfig_ExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Tilde expansion",
			path:     "~/.claude/skills",
			expected: filepath.Join(home, ".claude/skills"),
		},
		{
			name:     "Absolute path unchanged",
			path:     "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "Relative path unchanged",
			path:     "./relative/path",
			expected: "./relative/path",
		},
		{
			name:     "Tilde only",
			path:     "~",
			expected: home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := SkillRootConfig{Path: tt.path}
			expanded, err := root.ExpandPath()
			if err != nil {
				t.Fatalf("ExpandPath() error: %v", err)
			}

			if expanded != tt.expected {
				t.Errorf("ExpandPath() = %q, want %q", expanded, tt.expected)
			}
		})
	}
}

func TestSkillRootConfig_ToDomain(t *testing.T) {
	root := SkillRootConfig{
		Path:  "~/.claude/skills",
		Scope: domain.ScopeUser,
	}

	domainRoot, err := root.ToDomain()
	if err != nil {
		t.Fatalf("ToDomain() error: %v", err)
	}

	if domainRoot.Scope != domain.ScopeUser {
		t.Errorf("Scope = %v, want %v", domainRoot.Scope, domain.ScopeUser)
	}

	// Path should be expanded
	if domainRoot.Path == "~/.claude/skills" {
		t.Error("Path should be expanded, got unexpanded tilde path")
	}
}

func TestSkillsConfig_GetRoots(t *testing.T) {
	cfg := &SkillsConfig{
		Enabled: true,
		Roots: []SkillRootConfig{
			{Path: "~/.claude/skills", Scope: domain.ScopeUser},
			{Path: "./.claude/skills", Scope: domain.ScopeProject},
		},
	}

	roots, err := cfg.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots() error: %v", err)
	}

	if len(roots) != 2 {
		t.Errorf("GetRoots() returned %d roots, want 2", len(roots))
	}

	// Verify paths are expanded
	for _, root := range roots {
		if root.Path == "" {
			t.Error("GetRoots() returned empty path")
		}
	}
}

func TestSkillsConfig_GetRoots_Disabled(t *testing.T) {
	cfg := &SkillsConfig{
		Enabled: false,
		Roots: []SkillRootConfig{
			{Path: "~/.claude/skills", Scope: domain.ScopeUser},
		},
	}

	roots, err := cfg.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots() error: %v", err)
	}

	if len(roots) != 0 {
		t.Errorf("GetRoots() returned %d roots for disabled config, want 0", len(roots))
	}
}

func TestSkillsConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       SkillsConfig
		wantError bool
	}{
		{
			name: "Valid config",
			cfg: SkillsConfig{
				Enabled: true,
				Roots: []SkillRootConfig{
					{Path: "~/.claude/skills", Scope: domain.ScopeUser},
				},
			},
			wantError: false,
		},
		{
			name: "Disabled config is valid",
			cfg: SkillsConfig{
				Enabled: false,
				Roots:   []SkillRootConfig{},
			},
			wantError: false,
		},
		{
			name: "Empty roots when enabled",
			cfg: SkillsConfig{
				Enabled: true,
				Roots:   []SkillRootConfig{},
			},
			wantError: true,
		},
		{
			name: "Invalid scope",
			cfg: SkillsConfig{
				Enabled: true,
				Roots: []SkillRootConfig{
					{Path: "~/.claude/skills", Scope: domain.SkillScope(999)},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantError && err == nil {
				t.Error("Validate() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}
