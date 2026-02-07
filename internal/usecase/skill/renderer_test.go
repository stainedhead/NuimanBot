package skill

import (
	"strings"
	"testing"

	"nuimanbot/internal/domain"
)

func TestSubstituteArguments_FullArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Process $ARGUMENTS"
	args := []string{"file1.go", "file2.go"}
	result := r.SubstituteArguments(body, args)
	expected := "Process file1.go file2.go"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_PositionalArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "First: $0, Second: $1, Third: $2"
	args := []string{"apple", "banana", "cherry"}
	result := r.SubstituteArguments(body, args)
	expected := "First: apple, Second: banana, Third: cherry"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_MixedPlaceholders(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Args: $ARGUMENTS. First: $0. Second: $1."
	args := []string{"foo", "bar"}
	result := r.SubstituteArguments(body, args)
	expected := "Args: foo bar. First: foo. Second: bar."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_EscapedDollar(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Price: $$100. Arg: $0."
	args := []string{"test"}
	result := r.SubstituteArguments(body, args)
	expected := "Price: $100. Arg: test."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_MissingArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "First: $0, Second: $1, Third: $2"
	args := []string{"only-one"}
	result := r.SubstituteArguments(body, args)
	// $1 and $2 should be left as empty string (placeholders removed)
	// Since there's no match, they stay as-is
	if !strings.Contains(result, "only-one") {
		t.Errorf("Expected to contain 'only-one', got %q", result)
	}
}

func TestSubstituteArguments_NoPlaceholders(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "No placeholders here, just text."
	args := []string{"foo", "bar"}
	result := r.SubstituteArguments(body, args)
	expected := "No placeholders here, just text."
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_MultiDigitIndex(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Arg 10: $10, Arg 0: $0"
	args := make([]string, 15)
	for i := range args {
		args[i] = string(rune('a' + i))
	}
	result := r.SubstituteArguments(body, args)
	// $10 should be replaced with args[10] (11th element = 'k')
	// $0 should be replaced with args[0] ('a')
	if !strings.Contains(result, "k") || !strings.Contains(result, "a") {
		t.Errorf("Multi-digit index substitution failed: %q", result)
	}
}

func TestSubstituteArguments_NoArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "No args: $ARGUMENTS"
	args := []string{}
	result := r.SubstituteArguments(body, args)
	expected := "No args: "
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_UnicodeArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Unicode: $0, $1"
	args := []string{"こんにちは", "世界"}
	result := r.SubstituteArguments(body, args)
	expected := "Unicode: こんにちは, 世界"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestSubstituteArguments_SpecialChars(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Special: $0"
	args := []string{"test@#$%^&*()_+-=[]{}|;':\",./<>?"}
	result := r.SubstituteArguments(body, args)
	if !strings.Contains(result, args[0]) {
		t.Errorf("Special characters not preserved: %q", result)
	}
}

func TestRender_ValidSkill(t *testing.T) {
	r := NewDefaultSkillRenderer()
	skill := &domain.Skill{
		Name:   "test-skill",
		BodyMD: "Process file: $0",
		Frontmatter: domain.SkillFrontmatter{
			AllowedTools: []string{"read_file", "grep"},
		},
	}
	args := []string{"test.go"}

	rendered, err := r.Render(skill, args)
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	if rendered.SkillName != "test-skill" {
		t.Errorf("SkillName = %q, want %q", rendered.SkillName, "test-skill")
	}

	if rendered.Prompt != "Process file: test.go" {
		t.Errorf("Prompt = %q, want %q", rendered.Prompt, "Process file: test.go")
	}

	if len(rendered.AllowedTools) != 2 {
		t.Errorf("AllowedTools len = %d, want 2", len(rendered.AllowedTools))
	}
}

func TestRender_EmptyBody(t *testing.T) {
	r := NewDefaultSkillRenderer()
	skill := &domain.Skill{
		Name:        "test-skill",
		BodyMD:      "",
		Frontmatter: domain.SkillFrontmatter{},
	}

	rendered, err := r.Render(skill, []string{})
	if err != nil {
		t.Fatalf("Render() should handle empty body: %v", err)
	}

	if rendered.Prompt != "" {
		t.Errorf("Expected empty prompt, got %q", rendered.Prompt)
	}
}

func TestRender_NilSkill(t *testing.T) {
	r := NewDefaultSkillRenderer()

	_, err := r.Render(nil, []string{})
	if err == nil {
		t.Fatal("Render() should return error for nil skill")
	}

	if !strings.Contains(err.Error(), "skill is nil") {
		t.Errorf("Expected 'skill is nil' error, got: %v", err)
	}
}

func TestRender_AllowedToolsExtracted(t *testing.T) {
	r := NewDefaultSkillRenderer()
	skill := &domain.Skill{
		Name:   "test-skill",
		BodyMD: "Test",
		Frontmatter: domain.SkillFrontmatter{
			AllowedTools: []string{"calculator", "datetime", "weather"},
		},
	}

	rendered, err := r.Render(skill, []string{})
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	if len(rendered.AllowedTools) != 3 {
		t.Errorf("AllowedTools len = %d, want 3", len(rendered.AllowedTools))
	}

	expected := map[string]bool{"calculator": true, "datetime": true, "weather": true}
	for _, tool := range rendered.AllowedTools {
		if !expected[tool] {
			t.Errorf("Unexpected tool in AllowedTools: %s", tool)
		}
	}
}

func TestRender_LargeBody(t *testing.T) {
	r := NewDefaultSkillRenderer()
	largeBody := strings.Repeat("a", 10*1024*1024) // 10MB
	skill := &domain.Skill{
		Name:        "test-skill",
		BodyMD:      largeBody,
		Frontmatter: domain.SkillFrontmatter{},
	}

	_, err := r.Render(skill, []string{})
	if err != nil {
		t.Fatalf("Render() should handle large bodies: %v", err)
	}
}

func TestSubstituteArguments_MalformedPlaceholder(t *testing.T) {
	r := NewDefaultSkillRenderer()
	body := "Test $abc $ $x"
	result := r.SubstituteArguments(body, []string{})
	// Malformed placeholders should be left as-is (no panic)
	if !strings.Contains(result, "$abc") || !strings.Contains(result, "$x") {
		t.Errorf("Malformed placeholders should be preserved: %q", result)
	}
}

func TestRender_NoArgs(t *testing.T) {
	r := NewDefaultSkillRenderer()
	skill := &domain.Skill{
		Name:        "test-skill",
		BodyMD:      "Simple prompt with no placeholders",
		Frontmatter: domain.SkillFrontmatter{},
	}

	rendered, err := r.Render(skill, []string{})
	if err != nil {
		t.Fatalf("Render() failed: %v", err)
	}

	if rendered.Prompt != "Simple prompt with no placeholders" {
		t.Errorf("Prompt = %q, unexpected value", rendered.Prompt)
	}
}
