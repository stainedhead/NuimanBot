package domain

import (
	"testing"
)

// TestSkillScope_String tests the String method of SkillScope
func TestSkillScope_String(t *testing.T) {
	tests := []struct {
		name     string
		scope    SkillScope
		expected string
	}{
		{"Enterprise scope", ScopeEnterprise, "enterprise"},
		{"User scope", ScopeUser, "user"},
		{"Project scope", ScopeProject, "project"},
		{"Plugin scope", ScopePlugin, "plugin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.String()
			if got != tt.expected {
				t.Errorf("SkillScope.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkillScope_Priority tests the Priority method of SkillScope
func TestSkillScope_Priority(t *testing.T) {
	tests := []struct {
		name     string
		scope    SkillScope
		expected int
	}{
		{"Enterprise has highest priority", ScopeEnterprise, 300},
		{"User has second priority", ScopeUser, 200},
		{"Project has third priority", ScopeProject, 100},
		{"Plugin has lowest priority", ScopePlugin, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Priority()
			if got != tt.expected {
				t.Errorf("SkillScope.Priority() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkillScope_PriorityOrdering ensures enterprise > user > project > plugin
func TestSkillScope_PriorityOrdering(t *testing.T) {
	if ScopeEnterprise.Priority() <= ScopeUser.Priority() {
		t.Error("Enterprise priority should be greater than User priority")
	}
	if ScopeUser.Priority() <= ScopeProject.Priority() {
		t.Error("User priority should be greater than Project priority")
	}
	if ScopeProject.Priority() <= ScopePlugin.Priority() {
		t.Error("Project priority should be greater than Plugin priority")
	}
}

// TestSkillFrontmatter_IsUserInvocable tests default behavior
func TestSkillFrontmatter_IsUserInvocable(t *testing.T) {
	tests := []struct {
		name     string
		fm       SkillFrontmatter
		expected bool
	}{
		{
			name:     "Defaults to true when nil",
			fm:       SkillFrontmatter{UserInvocable: nil},
			expected: true,
		},
		{
			name: "Respects explicit true",
			fm: SkillFrontmatter{
				UserInvocable: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "Respects explicit false",
			fm: SkillFrontmatter{
				UserInvocable: boolPtr(false),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fm.IsUserInvocable()
			if got != tt.expected {
				t.Errorf("IsUserInvocable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkillFrontmatter_IsModelInvocable tests model invocation flag
func TestSkillFrontmatter_IsModelInvocable(t *testing.T) {
	tests := []struct {
		name     string
		fm       SkillFrontmatter
		expected bool
	}{
		{
			name:     "Model can invoke by default",
			fm:       SkillFrontmatter{DisableModelInvocation: false},
			expected: true,
		},
		{
			name:     "Model cannot invoke when disabled",
			fm:       SkillFrontmatter{DisableModelInvocation: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fm.IsModelInvocable()
			if got != tt.expected {
				t.Errorf("IsModelInvocable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkillFrontmatter_Validate tests frontmatter validation
func TestSkillFrontmatter_Validate(t *testing.T) {
	tests := []struct {
		name      string
		fm        SkillFrontmatter
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid frontmatter",
			fm: SkillFrontmatter{
				Name:        "test-skill",
				Description: "A test skill",
			},
			wantError: false,
		},
		{
			name: "Missing name",
			fm: SkillFrontmatter{
				Description: "A test skill",
			},
			wantError: true,
			errorMsg:  "name is required in frontmatter",
		},
		{
			name: "Missing description",
			fm: SkillFrontmatter{
				Name: "test-skill",
			},
			wantError: true,
			errorMsg:  "description is required in frontmatter",
		},
		{
			name: "Name too long",
			fm: SkillFrontmatter{
				Name:        "this-is-a-very-long-skill-name-that-exceeds-the-maximum-allowed-length-of-sixty-four-characters",
				Description: "A test skill",
			},
			wantError: true,
			errorMsg:  "name must be <= 64 characters",
		},
		{
			name: "Invalid name - uppercase",
			fm: SkillFrontmatter{
				Name:        "TestSkill",
				Description: "A test skill",
			},
			wantError: true,
			errorMsg:  "name must be lowercase, hyphens, alphanumeric",
		},
		{
			name: "Invalid name - underscores",
			fm: SkillFrontmatter{
				Name:        "test_skill",
				Description: "A test skill",
			},
			wantError: true,
			errorMsg:  "name must be lowercase, hyphens, alphanumeric",
		},
		{
			name: "Description too long",
			fm: SkillFrontmatter{
				Name:        "test-skill",
				Description: string(make([]byte, 1025)),
			},
			wantError: true,
			errorMsg:  "description must be <= 1024 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fm.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if errSkill, ok := err.(ErrSkillInvalid); ok {
					if errSkill.Reason != tt.errorMsg {
						t.Errorf("Validate() error reason = %q, want %q", errSkill.Reason, tt.errorMsg)
					}
				} else {
					t.Errorf("Validate() expected ErrSkillInvalid, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSkill_CanBeInvokedByUser tests user invocation check
func TestSkill_CanBeInvokedByUser(t *testing.T) {
	tests := []struct {
		name     string
		skill    Skill
		expected bool
	}{
		{
			name: "User invocable when nil (default true)",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					UserInvocable: nil,
				},
			},
			expected: true,
		},
		{
			name: "User invocable when explicitly true",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					UserInvocable: boolPtr(true),
				},
			},
			expected: true,
		},
		{
			name: "Not user invocable when explicitly false",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					UserInvocable: boolPtr(false),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.skill.CanBeInvokedByUser()
			if got != tt.expected {
				t.Errorf("CanBeInvokedByUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkill_CanBeSelectedByModel tests model selection check
func TestSkill_CanBeSelectedByModel(t *testing.T) {
	tests := []struct {
		name     string
		skill    Skill
		expected bool
	}{
		{
			name: "Model can select by default",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					DisableModelInvocation: false,
				},
			},
			expected: true,
		},
		{
			name: "Model cannot select when disabled",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					DisableModelInvocation: true,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.skill.CanBeSelectedByModel()
			if got != tt.expected {
				t.Errorf("CanBeSelectedByModel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSkill_AllowedTools tests allowed tools retrieval
func TestSkill_AllowedTools(t *testing.T) {
	tests := []struct {
		name     string
		skill    Skill
		expected []string
	}{
		{
			name: "Empty list means all allowed",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					AllowedTools: []string{},
				},
			},
			expected: []string{},
		},
		{
			name: "Returns allowed tools list",
			skill: Skill{
				Frontmatter: SkillFrontmatter{
					AllowedTools: []string{"calculator", "datetime"},
				},
			},
			expected: []string{"calculator", "datetime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.skill.AllowedTools()
			if len(got) != len(tt.expected) {
				t.Errorf("AllowedTools() len = %v, want %v", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("AllowedTools()[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

// TestSkill_Validate tests skill validation
func TestSkill_Validate(t *testing.T) {
	tests := []struct {
		name      string
		skill     Skill
		wantError bool
	}{
		{
			name: "Valid skill",
			skill: Skill{
				Name:        "test-skill",
				Description: "A test skill",
				Frontmatter: SkillFrontmatter{
					Name:        "test-skill",
					Description: "A test skill",
				},
			},
			wantError: false,
		},
		{
			name: "Missing name",
			skill: Skill{
				Description: "A test skill",
				Frontmatter: SkillFrontmatter{
					Name:        "test-skill",
					Description: "A test skill",
				},
			},
			wantError: true,
		},
		{
			name: "Missing description",
			skill: Skill{
				Name: "test-skill",
				Frontmatter: SkillFrontmatter{
					Name:        "test-skill",
					Description: "A test skill",
				},
			},
			wantError: true,
		},
		{
			name: "Invalid frontmatter",
			skill: Skill{
				Name:        "test-skill",
				Description: "A test skill",
				Frontmatter: SkillFrontmatter{
					Name: "", // Invalid
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.skill.Validate()
			if tt.wantError && err == nil {
				t.Error("Validate() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

// TestErrSkillNotFound tests error type
func TestErrSkillNotFound(t *testing.T) {
	err := ErrSkillNotFound{SkillName: "test-skill"}
	expected := "skill not found: test-skill"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestErrSkillInvalid tests error type
func TestErrSkillInvalid(t *testing.T) {
	tests := []struct {
		name     string
		err      ErrSkillInvalid
		expected string
	}{
		{
			name:     "With skill name",
			err:      ErrSkillInvalid{SkillName: "test-skill", Reason: "invalid format"},
			expected: "skill test-skill is invalid: invalid format",
		},
		{
			name:     "Without skill name",
			err:      ErrSkillInvalid{Reason: "invalid format"},
			expected: "skill is invalid: invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.expected)
			}
		})
	}
}

// TestErrSkillConflict tests error type
func TestErrSkillConflict(t *testing.T) {
	err := ErrSkillConflict{
		SkillName: "test-skill",
		Scopes:    []SkillScope{ScopeUser, ScopeProject},
	}
	msg := err.Error()
	if msg != "skill test-skill found in multiple scopes: [user project]" {
		t.Errorf("Error() = %q, want conflict message", msg)
	}
}

// TestErrSkillPermissionDenied tests error type
func TestErrSkillPermissionDenied(t *testing.T) {
	err := ErrSkillPermissionDenied{
		SkillName: "test-skill",
		Reason:    "user lacks permission",
	}
	expected := "permission denied for skill test-skill: user lacks permission"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

// TestErrSkillParseError tests error type with unwrapping
func TestErrSkillParseError(t *testing.T) {
	baseErr := ErrSkillInvalid{Reason: "bad YAML"}
	err := ErrSkillParseError{
		FilePath: "/path/to/SKILL.md",
		Err:      baseErr,
	}

	msg := err.Error()
	if msg != "failed to parse skill at /path/to/SKILL.md: skill is invalid: bad YAML" {
		t.Errorf("Error() = %q, unexpected message", msg)
	}

	unwrapped := err.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
