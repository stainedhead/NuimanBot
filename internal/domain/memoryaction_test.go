package domain

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryActionType_String(t *testing.T) {
	tests := []struct {
		name string
		mat  MemoryActionType
		want string
	}{
		{
			name: "write_file",
			mat:  MemoryActionWriteFile,
			want: "write_file",
		},
		{
			name: "persona_update",
			mat:  MemoryActionPersonaUpdate,
			want: "persona_update",
		},
		{
			name: "invalid type fallback",
			mat:  MemoryActionType(99),
			want: "MemoryActionType(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mat.String()
			if got != tt.want {
				t.Errorf("MemoryActionType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMemoryActionType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		mat  MemoryActionType
		want bool
	}{
		{
			name: "write_file is valid",
			mat:  MemoryActionWriteFile,
			want: true,
		},
		{
			name: "persona_update is valid",
			mat:  MemoryActionPersonaUpdate,
			want: true,
		},
		{
			name: "negative value is invalid",
			mat:  MemoryActionType(-1),
			want: false,
		},
		{
			name: "out of range is invalid",
			mat:  MemoryActionType(99),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mat.IsValid()
			if got != tt.want {
				t.Errorf("MemoryActionType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func validMemoryAction() *MemoryAction {
	return &MemoryAction{
		ID:                   "action-001",
		UserID:               "user-123",
		Type:                 MemoryActionWriteFile,
		FilePath:             "/data/users/user-123/SOUL.md",
		Operation:            "append",
		Content:              "Remember: user prefers concise answers",
		RequiresConfirmation: true,
		Confirmed:            false,
		CreatedAt:            time.Now(),
	}
}

func TestMemoryAction_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(m *MemoryAction)
		wantErr string
	}{
		{
			name:    "valid action",
			modify:  func(_ *MemoryAction) {},
			wantErr: "",
		},
		{
			name:    "valid with replace operation",
			modify:  func(m *MemoryAction) { m.Operation = "replace" },
			wantErr: "",
		},
		{
			name:    "valid with insert operation",
			modify:  func(m *MemoryAction) { m.Operation = "insert" },
			wantErr: "",
		},
		{
			name:    "valid persona_update type",
			modify:  func(m *MemoryAction) { m.Type = MemoryActionPersonaUpdate },
			wantErr: "",
		},
		{
			name:    "missing ID",
			modify:  func(m *MemoryAction) { m.ID = "" },
			wantErr: "ID is required",
		},
		{
			name:    "ID too long",
			modify:  func(m *MemoryAction) { m.ID = strings.Repeat("a", 129) },
			wantErr: "ID must be <= 128 characters",
		},
		{
			name:    "missing UserID",
			modify:  func(m *MemoryAction) { m.UserID = "" },
			wantErr: "userID is required",
		},
		{
			name:    "UserID too long",
			modify:  func(m *MemoryAction) { m.UserID = strings.Repeat("u", 65) },
			wantErr: "userID must be <= 64 characters",
		},
		{
			name:    "invalid type",
			modify:  func(m *MemoryAction) { m.Type = MemoryActionType(99) },
			wantErr: "invalid memory action type",
		},
		{
			name:    "missing file path",
			modify:  func(m *MemoryAction) { m.FilePath = "" },
			wantErr: "filePath is required",
		},
		{
			name:    "invalid operation",
			modify:  func(m *MemoryAction) { m.Operation = "delete" },
			wantErr: "operation must be one of: append, replace, insert",
		},
		{
			name:    "empty operation",
			modify:  func(m *MemoryAction) { m.Operation = "" },
			wantErr: "operation is required",
		},
		{
			name:    "missing content",
			modify:  func(m *MemoryAction) { m.Content = "" },
			wantErr: "content is required",
		},
		{
			name:    "zero CreatedAt",
			modify:  func(m *MemoryAction) { m.CreatedAt = time.Time{} },
			wantErr: "createdAt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMemoryAction()
			tt.modify(m)
			err := m.Validate()

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
				t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestMemoryAction_AwaitingConfirmation(t *testing.T) {
	tests := []struct {
		name                 string
		requiresConfirmation bool
		confirmed            bool
		want                 bool
	}{
		{
			name:                 "requires confirmation and not confirmed",
			requiresConfirmation: true,
			confirmed:            false,
			want:                 true,
		},
		{
			name:                 "requires confirmation and confirmed",
			requiresConfirmation: true,
			confirmed:            true,
			want:                 false,
		},
		{
			name:                 "does not require confirmation",
			requiresConfirmation: false,
			confirmed:            false,
			want:                 false,
		},
		{
			name:                 "does not require confirmation but somehow confirmed",
			requiresConfirmation: false,
			confirmed:            true,
			want:                 false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MemoryAction{
				RequiresConfirmation: tt.requiresConfirmation,
				Confirmed:            tt.confirmed,
			}
			got := m.AwaitingConfirmation()
			if got != tt.want {
				t.Errorf("AwaitingConfirmation() = %v, want %v", got, tt.want)
			}
		})
	}
}
