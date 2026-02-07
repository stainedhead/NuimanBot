package skill

import (
	"testing"
)

// TestPreprocessParser_Parse tests parsing !command blocks
func TestPreprocessParser_Parse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCommands int
		wantFirst    string
	}{
		{
			name: "single command block",
			input: `# Test Skill

Some text before.

!command
git status

Some text after.`,
			wantCommands: 1,
			wantFirst:    "git status",
		},
		{
			name: "multiple command blocks",
			input: `# Test Skill

!command
git status

Some text.

!command
ls -la

More text.`,
			wantCommands: 2,
			wantFirst:    "git status",
		},
		{
			name: "multi-line command",
			input: `# Test Skill

!command
git log \
  --oneline \
  --max-count=10

Text.`,
			wantCommands: 1,
			wantFirst:    "git log \\\n  --oneline \\\n  --max-count=10",
		},
		{
			name: "command with flags",
			input: `!command
ls -la /tmp`,
			wantCommands: 1,
			wantFirst:    "ls -la /tmp",
		},
		{
			name: "no commands",
			input: `# Test Skill

Just regular markdown text.

No commands here.`,
			wantCommands: 0,
		},
		{
			name:         "code block (not command)",
			input:        "# Test Skill\n\n```bash\ngit status\n```\n\nNot a command block.",
			wantCommands: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPreprocessParser()
			commands, err := parser.Parse(tt.input)

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(commands) != tt.wantCommands {
				t.Errorf("Parse() found %d commands, want %d", len(commands), tt.wantCommands)
			}

			if tt.wantCommands > 0 && len(commands) > 0 {
				if commands[0].Command != tt.wantFirst {
					t.Errorf("First command = %q, want %q", commands[0].Command, tt.wantFirst)
				}
			}
		})
	}
}

// TestPreprocessParser_ValidateCommands tests command validation
func TestPreprocessParser_ValidateCommands(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "valid git command",
			input: `!command
git status`,
			wantErr: false,
		},
		{
			name: "invalid rm command",
			input: `!command
rm -rf /`,
			wantErr: true,
		},
		{
			name: "shell injection attempt",
			input: `!command
ls | rm -rf /`,
			wantErr: true,
		},
		{
			name: "command substitution attempt",
			input: `!command
ls $(whoami)`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPreprocessParser()
			commands, err := parser.Parse(tt.input)

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(commands) == 0 {
				t.Fatal("Parse() returned no commands")
			}

			err = commands[0].Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPreprocessParser_ExtractPosition tests position tracking
func TestPreprocessParser_ExtractPosition(t *testing.T) {
	input := `# Header

Some text.

!command
git status

More text.`

	parser := NewPreprocessParser()
	blocks, err := parser.ParseWithPositions(input)

	if err != nil {
		t.Fatalf("ParseWithPositions() error = %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("ParseWithPositions() found %d blocks, want 1", len(blocks))
	}

	block := blocks[0]

	// Position should point to the !command line
	if block.StartPos < 0 {
		t.Error("StartPos should be >= 0")
	}

	if block.EndPos <= block.StartPos {
		t.Error("EndPos should be > StartPos")
	}

	if block.Command.Command != "git status" {
		t.Errorf("Command = %q, want 'git status'", block.Command.Command)
	}
}

// TestPreprocessParser_EdgeCases tests edge cases
func TestPreprocessParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCommands int
	}{
		{
			name:         "empty input",
			input:        "",
			wantCommands: 0,
		},
		{
			name:         "only whitespace",
			input:        "   \n\n   ",
			wantCommands: 0,
		},
		{
			name: "command block with no command",
			input: `!command

`,
			wantCommands: 0,
		},
		{
			name: "consecutive command blocks",
			input: `!command
git status
!command
ls -la`,
			wantCommands: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewPreprocessParser()
			commands, err := parser.Parse(tt.input)

			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			if len(commands) != tt.wantCommands {
				t.Errorf("Parse() found %d commands, want %d", len(commands), tt.wantCommands)
			}
		})
	}
}

// TestPreprocessParser_RealWorldExample tests a realistic skill
func TestPreprocessParser_RealWorldExample(t *testing.T) {
	input := `---
name: project-status
description: Show current project status
---

# Project Status

Current branch and recent commits:

!command
git status --short

Recent activity:

!command
git log --oneline --max-count=5

Open pull requests:

!command
gh pr list --limit 5
`

	parser := NewPreprocessParser()
	commands, err := parser.Parse(input)

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(commands) != 3 {
		t.Fatalf("Parse() found %d commands, want 3", len(commands))
	}

	// Verify all commands
	expected := []string{
		"git status --short",
		"git log --oneline --max-count=5",
		"gh pr list --limit 5",
	}

	for i, cmd := range commands {
		if cmd.Command != expected[i] {
			t.Errorf("Command %d = %q, want %q", i, cmd.Command, expected[i])
		}

		// All should be valid
		if err := cmd.Validate(); err != nil {
			t.Errorf("Command %d validation failed: %v", i, err)
		}
	}
}
