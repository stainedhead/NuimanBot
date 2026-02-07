package domain

import (
	"testing"
	"time"
)

// TestSubagentContext_Validate tests SubagentContext validation
func TestSubagentContext_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ctx     SubagentContext
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid context",
			ctx: SubagentContext{
				ID:              "subagent-123",
				ParentContextID: "parent-456",
				SkillName:       "debug-skill",
				AllowedTools:    []string{"read_file", "grep"},
				ResourceLimits: ResourceLimits{
					MaxTokens:    100000,
					MaxToolCalls: 50,
					Timeout:      5 * time.Minute,
				},
				ConversationHistory: []Message{},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			ctx: SubagentContext{
				ParentContextID: "parent-456",
				SkillName:       "debug-skill",
			},
			wantErr: true,
			errMsg:  "subagent context ID is required",
		},
		{
			name: "missing parent context ID",
			ctx: SubagentContext{
				ID:        "subagent-123",
				SkillName: "debug-skill",
			},
			wantErr: true,
			errMsg:  "parent context ID is required",
		},
		{
			name: "missing skill name",
			ctx: SubagentContext{
				ID:              "subagent-123",
				ParentContextID: "parent-456",
			},
			wantErr: true,
			errMsg:  "skill name is required",
		},
		{
			name: "invalid resource limits - zero timeout",
			ctx: SubagentContext{
				ID:              "subagent-123",
				ParentContextID: "parent-456",
				SkillName:       "debug-skill",
				ResourceLimits: ResourceLimits{
					Timeout: 0,
				},
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid resource limits - negative max tokens",
			ctx: SubagentContext{
				ID:              "subagent-123",
				ParentContextID: "parent-456",
				SkillName:       "debug-skill",
				ResourceLimits: ResourceLimits{
					MaxTokens: -100,
					Timeout:   5 * time.Minute,
				},
			},
			wantErr: true,
			errMsg:  "max tokens must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestResourceLimits_IsWithinLimits tests resource limit checking
func TestResourceLimits_IsWithinLimits(t *testing.T) {
	limits := ResourceLimits{
		MaxTokens:    100000,
		MaxToolCalls: 50,
		Timeout:      5 * time.Minute,
	}

	tests := []struct {
		name          string
		tokensUsed    int
		toolCallsMade int
		elapsed       time.Duration
		want          bool
	}{
		{
			name:          "within all limits",
			tokensUsed:    50000,
			toolCallsMade: 25,
			elapsed:       2 * time.Minute,
			want:          true,
		},
		{
			name:          "exceeds token limit",
			tokensUsed:    150000,
			toolCallsMade: 25,
			elapsed:       2 * time.Minute,
			want:          false,
		},
		{
			name:          "exceeds tool call limit",
			tokensUsed:    50000,
			toolCallsMade: 75,
			elapsed:       2 * time.Minute,
			want:          false,
		},
		{
			name:          "exceeds timeout",
			tokensUsed:    50000,
			toolCallsMade: 25,
			elapsed:       6 * time.Minute,
			want:          false,
		},
		{
			name:          "at exact limits",
			tokensUsed:    100000,
			toolCallsMade: 50,
			elapsed:       5 * time.Minute,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limits.IsWithinLimits(tt.tokensUsed, tt.toolCallsMade, tt.elapsed)
			if got != tt.want {
				t.Errorf("IsWithinLimits() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSubagentResult_Validate tests SubagentResult validation
func TestSubagentResult_Validate(t *testing.T) {
	tests := []struct {
		name    string
		result  SubagentResult
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid result",
			result: SubagentResult{
				SubagentID: "subagent-123",
				Status:     SubagentStatusComplete,
				Output:     "Task completed successfully",
				TokensUsed: 50000,
			},
			wantErr: false,
		},
		{
			name: "missing subagent ID",
			result: SubagentResult{
				Status: SubagentStatusComplete,
				Output: "Done",
			},
			wantErr: true,
			errMsg:  "subagent ID is required",
		},
		{
			name: "invalid status",
			result: SubagentResult{
				SubagentID: "subagent-123",
				Status:     "invalid-status",
				Output:     "Done",
			},
			wantErr: true,
			errMsg:  "invalid subagent status",
		},
		{
			name: "error status with no error message",
			result: SubagentResult{
				SubagentID: "subagent-123",
				Status:     SubagentStatusError,
				Output:     "Done",
			},
			wantErr: true,
			errMsg:  "error message required when status is error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestSubagentStatus_IsTerminal tests terminal status detection
func TestSubagentStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status SubagentStatus
		want   bool
	}{
		{"complete is terminal", SubagentStatusComplete, true},
		{"error is terminal", SubagentStatusError, true},
		{"timeout is terminal", SubagentStatusTimeout, true},
		{"cancelled is terminal", SubagentStatusCancelled, true},
		{"running is not terminal", SubagentStatusRunning, false},
		{"pending is not terminal", SubagentStatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsTerminal()
			if got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDefaultResourceLimits tests default resource limit initialization
func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()

	if limits.MaxTokens <= 0 {
		t.Errorf("DefaultResourceLimits().MaxTokens = %d, want positive", limits.MaxTokens)
	}
	if limits.MaxToolCalls <= 0 {
		t.Errorf("DefaultResourceLimits().MaxToolCalls = %d, want positive", limits.MaxToolCalls)
	}
	if limits.Timeout <= 0 {
		t.Errorf("DefaultResourceLimits().Timeout = %v, want positive", limits.Timeout)
	}

	// Default should be 5 minutes
	if limits.Timeout != 5*time.Minute {
		t.Errorf("DefaultResourceLimits().Timeout = %v, want 5m", limits.Timeout)
	}
}
