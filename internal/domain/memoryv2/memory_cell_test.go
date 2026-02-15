package memoryv2

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validMemoryCell() *MemoryCell {
	now := time.Now()
	return &MemoryCell{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		ConversationID: "conv-123",
		Scene:          "project-setup",
		CellType:       CellTypeDecision,
		Salience:       0.85,
		Content:        "User decided to use SQLite FTS5 for memory retrieval",
		Source:         `["msg-abc123", "msg-def456"]`,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      nil,
	}
}

func TestMemoryCell_Validate_Valid(t *testing.T) {
	cell := validMemoryCell()
	if err := cell.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestMemoryCell_Validate_ID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty id", "", true},
		{"invalid uuid", "not-a-uuid", true},
		{"short uuid", "550e8400", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.ID = tt.id
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Validate() error should wrap ErrInvalidInput")
			}
		})
	}
}

func TestMemoryCell_Validate_ConversationID(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		wantErr        bool
	}{
		{"valid conversation id", "conv-123", false},
		{"empty conversation id", "", true},
		{"max length", strings.Repeat("a", MaxConversationIDLen), false},
		{"exceeds max length", strings.Repeat("a", MaxConversationIDLen+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.ConversationID = tt.conversationID
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_Scene(t *testing.T) {
	tests := []struct {
		name    string
		scene   string
		wantErr bool
	}{
		{"valid scene", "project-setup", false},
		{"valid with numbers", "project-123-setup", false},
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
			cell := validMemoryCell()
			cell.Scene = tt.scene
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_CellType(t *testing.T) {
	tests := []struct {
		name     string
		cellType CellType
		wantErr  bool
	}{
		{"valid fact", CellTypeFact, false},
		{"valid decision", CellTypeDecision, false},
		{"valid task", CellTypeTask, false},
		{"valid preference", CellTypePreference, false},
		{"valid plan", CellTypePlan, false},
		{"valid risk", CellTypeRisk, false},
		{"invalid negative", CellType(-1), true},
		{"invalid large", CellType(99), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.CellType = tt.cellType
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_Salience(t *testing.T) {
	tests := []struct {
		name     string
		salience float64
		wantErr  bool
	}{
		{"minimum salience", 0.0, false},
		{"maximum salience", 1.0, false},
		{"mid salience", 0.5, false},
		{"negative salience", -0.1, true},
		{"above max", 1.1, true},
		{"way too high", 2.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.Salience = tt.salience
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_Content(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"valid content", "Some knowledge content", false},
		{"minimum content", "a", false},
		{"max content", strings.Repeat("a", MaxContentLength), false},
		{"empty content", "", true},
		{"exceeds max", strings.Repeat("a", MaxContentLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.Content = tt.content
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_Source(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{"valid array", `["msg-1", "msg-2"]`, false},
		{"empty array", `[]`, false},
		{"single element", `["msg-1"]`, false},
		{"empty source", "", true},
		{"invalid json", "not json", true},
		{"json object", `{"key": "value"}`, true},
		{"json string", `"string"`, true},
		{"json number", `123`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.Source = tt.source
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_Validate_Timestamps(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		createdAt time.Time
		updatedAt time.Time
		expiresAt *time.Time
		wantErr   bool
	}{
		{"valid timestamps", now, now, nil, false},
		{"updated after created", past, now, nil, false},
		{"with expiration", now, now, &future, false},
		{"zero created_at", time.Time{}, now, nil, true},
		{"updated before created", now, past, nil, true},
		{"expires same as created", now, now, &now, true},
		{"expires before created", now, now, &past, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.CreatedAt = tt.createdAt
			cell.UpdatedAt = tt.updatedAt
			cell.ExpiresAt = tt.expiresAt
			err := cell.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryCell_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"no expiration", nil, false},
		{"expired", &past, true},
		{"not expired", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := validMemoryCell()
			cell.ExpiresAt = tt.expiresAt
			if got := cell.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryCell_String(t *testing.T) {
	cell := validMemoryCell()
	got := cell.String()

	expected := []string{
		"MemoryCell{",
		"ID: 550e8400-e29b-41d4-a716-446655440000",
		"Scene: project-setup",
		"Type: decision",
		"Salience: 0.85",
	}

	for _, substr := range expected {
		if !strings.Contains(got, substr) {
			t.Errorf("String() = %q, missing %q", got, substr)
		}
	}
}

func TestMemoryCell_Validate_ErrorsWrapErrInvalidInput(t *testing.T) {
	cell := &MemoryCell{} // All fields invalid
	err := cell.Validate()
	if err == nil {
		t.Fatal("expected error for empty cell")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("error should wrap ErrInvalidInput, got: %v", err)
	}
}
