package subagent

import (
	"context"
	"nuimanbot/internal/domain"
	"testing"
	"time"
)

// TestContextForker_Fork tests the context forking functionality
func TestContextForker_Fork(t *testing.T) {
	tests := []struct {
		name            string
		parentHistory   []domain.Message
		skillName       string
		allowedTools    []string
		resourceLimits  domain.ResourceLimits
		wantHistoryLen  int
		wantErr         bool
		validateContext func(*testing.T, *domain.SubagentContext)
	}{
		{
			name: "successful fork with conversation history",
			parentHistory: []domain.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "Debug this issue"},
			},
			skillName:      "debug-skill",
			allowedTools:   []string{"read_file", "grep"},
			resourceLimits: domain.DefaultResourceLimits(),
			wantHistoryLen: 3,
			wantErr:        false,
			validateContext: func(t *testing.T, ctx *domain.SubagentContext) {
				if ctx.ID == "" {
					t.Error("SubagentContext.ID should not be empty")
				}
				if ctx.ParentContextID == "" {
					t.Error("SubagentContext.ParentContextID should not be empty")
				}
				if ctx.SkillName != "debug-skill" {
					t.Errorf("SubagentContext.SkillName = %v, want debug-skill", ctx.SkillName)
				}
				if len(ctx.AllowedTools) != 2 {
					t.Errorf("SubagentContext.AllowedTools length = %v, want 2", len(ctx.AllowedTools))
				}
			},
		},
		{
			name:           "fork with empty history",
			parentHistory:  []domain.Message{},
			skillName:      "test-skill",
			allowedTools:   []string{"read_file"},
			resourceLimits: domain.DefaultResourceLimits(),
			wantHistoryLen: 0,
			wantErr:        false,
		},
		{
			name: "fork with nil allowed tools (all tools allowed)",
			parentHistory: []domain.Message{
				{Role: "user", Content: "Test"},
			},
			skillName:      "all-tools-skill",
			allowedTools:   nil,
			resourceLimits: domain.DefaultResourceLimits(),
			wantHistoryLen: 1,
			wantErr:        false,
			validateContext: func(t *testing.T, ctx *domain.SubagentContext) {
				if ctx.AllowedTools != nil {
					t.Error("SubagentContext.AllowedTools should be nil for all-tools-allowed")
				}
			},
		},
		{
			name: "fork with empty allowed tools (no tools allowed)",
			parentHistory: []domain.Message{
				{Role: "user", Content: "Test"},
			},
			skillName:      "no-tools-skill",
			allowedTools:   []string{},
			resourceLimits: domain.DefaultResourceLimits(),
			wantHistoryLen: 1,
			wantErr:        false,
			validateContext: func(t *testing.T, ctx *domain.SubagentContext) {
				if len(ctx.AllowedTools) != 0 {
					t.Errorf("SubagentContext.AllowedTools length = %v, want 0", len(ctx.AllowedTools))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forker := NewContextForker()

			parentCtxID := "parent-ctx-123"
			ctx := context.Background()

			subagentCtx, err := forker.Fork(ctx, parentCtxID, tt.parentHistory, tt.skillName, tt.allowedTools, tt.resourceLimits)

			if tt.wantErr {
				if err == nil {
					t.Error("Fork() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Fork() unexpected error = %v", err)
				return
			}

			if subagentCtx == nil {
				t.Fatal("Fork() returned nil context")
			}

			// Validate conversation history
			if len(subagentCtx.ConversationHistory) != tt.wantHistoryLen {
				t.Errorf("ConversationHistory length = %v, want %v", len(subagentCtx.ConversationHistory), tt.wantHistoryLen)
			}

			// Validate SubagentContext fields
			if err := subagentCtx.Validate(); err != nil {
				t.Errorf("SubagentContext.Validate() error = %v", err)
			}

			// Run custom validation if provided
			if tt.validateContext != nil {
				tt.validateContext(t, subagentCtx)
			}
		})
	}
}

// TestContextForker_Fork_Isolation tests that forked context is isolated from parent
func TestContextForker_Fork_Isolation(t *testing.T) {
	forker := NewContextForker()

	originalHistory := []domain.Message{
		{Role: "user", Content: "Original message"},
		{Role: "assistant", Content: "Original response"},
	}

	parentCtxID := "parent-isolation-test"
	ctx := context.Background()

	subagentCtx, err := forker.Fork(
		ctx,
		parentCtxID,
		originalHistory,
		"isolation-test-skill",
		[]string{"read_file"},
		domain.DefaultResourceLimits(),
	)

	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}

	// Modify parent history - should not affect subagent
	originalHistory[0].Content = "Modified message"

	// Verify subagent context still has original content
	if subagentCtx.ConversationHistory[0].Content != "Original message" {
		t.Errorf("Context isolation failed: parent modification affected subagent history")
	}

	// Modify subagent history - should not affect parent
	subagentCtx.ConversationHistory[0].Content = "Subagent modified"

	// Verify parent history is unaffected
	if originalHistory[0].Content != "Modified message" {
		t.Errorf("Context isolation failed: subagent modification affected parent history")
	}
}

// TestContextForker_Fork_ToolRestrictions tests tool restriction application
func TestContextForker_Fork_ToolRestrictions(t *testing.T) {
	tests := []struct {
		name         string
		allowedTools []string
		wantTools    []string
		wantNil      bool
	}{
		{
			name:         "specific tools allowed",
			allowedTools: []string{"read_file", "write_file", "grep"},
			wantTools:    []string{"read_file", "write_file", "grep"},
			wantNil:      false,
		},
		{
			name:         "all tools allowed (nil)",
			allowedTools: nil,
			wantTools:    nil,
			wantNil:      true,
		},
		{
			name:         "no tools allowed (empty slice)",
			allowedTools: []string{},
			wantTools:    []string{},
			wantNil:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forker := NewContextForker()

			history := []domain.Message{
				{Role: "user", Content: "Test"},
			}

			subagentCtx, err := forker.Fork(
				context.Background(),
				"parent-tool-test",
				history,
				"tool-test-skill",
				tt.allowedTools,
				domain.DefaultResourceLimits(),
			)

			if err != nil {
				t.Fatalf("Fork() error = %v", err)
			}

			if tt.wantNil {
				if subagentCtx.AllowedTools != nil {
					t.Errorf("AllowedTools should be nil, got %v", subagentCtx.AllowedTools)
				}
			} else {
				if len(subagentCtx.AllowedTools) != len(tt.wantTools) {
					t.Errorf("AllowedTools length = %v, want %v", len(subagentCtx.AllowedTools), len(tt.wantTools))
				}

				// Verify tools match
				for i, tool := range tt.wantTools {
					if subagentCtx.AllowedTools[i] != tool {
						t.Errorf("AllowedTools[%d] = %v, want %v", i, subagentCtx.AllowedTools[i], tool)
					}
				}
			}
		})
	}
}

// TestContextForker_Fork_TimestampAndMetadata tests timestamp and metadata initialization
func TestContextForker_Fork_TimestampAndMetadata(t *testing.T) {
	forker := NewContextForker()

	history := []domain.Message{
		{Role: "user", Content: "Test"},
	}

	beforeFork := time.Now()

	subagentCtx, err := forker.Fork(
		context.Background(),
		"parent-timestamp-test",
		history,
		"timestamp-test-skill",
		[]string{"read_file"},
		domain.DefaultResourceLimits(),
	)

	afterFork := time.Now()

	if err != nil {
		t.Fatalf("Fork() error = %v", err)
	}

	// Verify CreatedAt is set and within expected range
	if subagentCtx.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if subagentCtx.CreatedAt.Before(beforeFork) || subagentCtx.CreatedAt.After(afterFork) {
		t.Errorf("CreatedAt = %v, should be between %v and %v", subagentCtx.CreatedAt, beforeFork, afterFork)
	}

	// Verify Metadata is initialized (not nil)
	if subagentCtx.Metadata == nil {
		t.Error("Metadata should be initialized, got nil")
	}
}

// TestContextForker_Fork_InvalidInputs tests error handling
func TestContextForker_Fork_InvalidInputs(t *testing.T) {
	tests := []struct {
		name           string
		parentCtxID    string
		skillName      string
		resourceLimits domain.ResourceLimits
		wantErr        bool
	}{
		{
			name:           "empty parent context ID",
			parentCtxID:    "",
			skillName:      "test-skill",
			resourceLimits: domain.DefaultResourceLimits(),
			wantErr:        true,
		},
		{
			name:           "empty skill name",
			parentCtxID:    "parent-123",
			skillName:      "",
			resourceLimits: domain.DefaultResourceLimits(),
			wantErr:        true,
		},
		{
			name:        "invalid resource limits (zero timeout)",
			parentCtxID: "parent-123",
			skillName:   "test-skill",
			resourceLimits: domain.ResourceLimits{
				MaxTokens:    100000,
				MaxToolCalls: 50,
				Timeout:      0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forker := NewContextForker()

			history := []domain.Message{
				{Role: "user", Content: "Test"},
			}

			_, err := forker.Fork(
				context.Background(),
				tt.parentCtxID,
				history,
				tt.skillName,
				[]string{"read_file"},
				tt.resourceLimits,
			)

			if tt.wantErr && err == nil {
				t.Error("Fork() expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Fork() unexpected error = %v", err)
			}
		})
	}
}
