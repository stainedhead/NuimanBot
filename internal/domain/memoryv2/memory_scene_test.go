package memoryv2

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validMemoryScene() *MemoryScene {
	return &MemoryScene{
		Scene:      "project-setup",
		Summary:    "This scene covers project setup decisions and configurations.",
		TokenCount: 100,
		UpdatedAt:  time.Now(),
	}
}

func TestMemoryScene_Validate_Valid(t *testing.T) {
	scene := validMemoryScene()
	if err := scene.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestMemoryScene_Validate_Scene(t *testing.T) {
	tests := []struct {
		name    string
		scene   string
		wantErr bool
	}{
		{"valid scene", "project-setup", false},
		{"valid with numbers", "project-123", false},
		{"valid minimum length", "abc", false},
		{"valid maximum length", strings.Repeat("a", MaxSceneNameLength), false},
		{"empty scene", "", true},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", MaxSceneNameLength+1), true},
		{"uppercase letters", "Project-Setup", true},
		{"spaces", "project setup", true},
		{"underscores", "project_setup", true},
		{"special chars", "project@setup", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := validMemoryScene()
			scene.Scene = tt.scene
			err := scene.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryScene_Validate_Summary(t *testing.T) {
	tests := []struct {
		name    string
		summary string
		wantErr bool
	}{
		{"valid summary", "A short summary.", false},
		{"minimum summary", "a", false},
		{"max summary", strings.Repeat("a", MaxSummaryLength), false},
		{"empty summary", "", true},
		{"exceeds max", strings.Repeat("a", MaxSummaryLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := validMemoryScene()
			scene.Summary = tt.summary
			err := scene.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryScene_Validate_TokenCount(t *testing.T) {
	tests := []struct {
		name       string
		tokenCount int
		wantErr    bool
	}{
		{"valid token count", 100, false},
		{"minimum token count", 1, false},
		{"maximum token count", MaxSummaryTokens, false},
		{"zero token count", 0, true},
		{"negative token count", -1, true},
		{"exceeds max", MaxSummaryTokens + 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := validMemoryScene()
			scene.TokenCount = tt.tokenCount
			err := scene.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryScene_Validate_Timestamp(t *testing.T) {
	tests := []struct {
		name      string
		updatedAt time.Time
		wantErr   bool
	}{
		{"valid timestamp", time.Now(), false},
		{"past timestamp", time.Now().Add(-time.Hour), false},
		{"zero timestamp", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := validMemoryScene()
			scene.UpdatedAt = tt.updatedAt
			err := scene.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryScene_String(t *testing.T) {
	scene := validMemoryScene()
	got := scene.String()

	expected := []string{
		"MemoryScene{",
		"Scene: project-setup",
		"TokenCount: 100",
	}

	for _, substr := range expected {
		if !strings.Contains(got, substr) {
			t.Errorf("String() = %q, missing %q", got, substr)
		}
	}
}

func TestMemoryScene_Validate_ErrorsWrapErrInvalidInput(t *testing.T) {
	scene := &MemoryScene{} // All fields invalid
	err := scene.Validate()
	if err == nil {
		t.Fatal("expected error for empty scene")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("error should wrap ErrInvalidInput, got: %v", err)
	}
}
