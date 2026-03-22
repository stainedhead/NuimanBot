package domain

import (
	"testing"
)

func TestSkill_ShouldFork(t *testing.T) {
	tests := []struct {
		name    string
		context string
		want    bool
	}{
		{
			name:    "fork context",
			context: "fork",
			want:    true,
		},
		{
			name:    "empty context",
			context: "",
			want:    false,
		},
		{
			name:    "other context",
			context: "inline",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Skill{
				Frontmatter: SkillFrontmatter{
					Context: tt.context,
				},
			}
			if got := s.ShouldFork(); got != tt.want {
				t.Errorf("ShouldFork() = %v, want %v", got, tt.want)
			}
		})
	}
}
