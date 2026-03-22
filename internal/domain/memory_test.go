package domain

import (
	"testing"
	"time"
)

func TestSkillMemory_Validate(t *testing.T) {
	tests := []struct {
		name    string
		memory  *SkillMemory
		wantErr bool
	}{
		{
			name: "valid memory",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "last_result",
				Scope:     MemoryScopeSkill,
				Value:     "42",
			},
			wantErr: false,
		},
		{
			name: "missing skill name",
			memory: &SkillMemory{
				Key:   "some_key",
				Scope: MemoryScopeSkill,
			},
			wantErr: true,
		},
		{
			name: "missing key",
			memory: &SkillMemory{
				SkillName: "calculator",
				Scope:     MemoryScopeSkill,
			},
			wantErr: true,
		},
		{
			name: "empty scope defaults to skill",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "some_key",
				Scope:     "",
			},
			wantErr: false,
		},
		{
			name: "invalid scope",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "some_key",
				Scope:     "invalid_scope",
			},
			wantErr: true,
		},
		{
			name: "user scope is valid",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "some_key",
				Scope:     MemoryScopeUser,
			},
			wantErr: false,
		},
		{
			name: "global scope is valid",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "some_key",
				Scope:     MemoryScopeGlobal,
			},
			wantErr: false,
		},
		{
			name: "session scope is valid",
			memory: &SkillMemory{
				SkillName: "calculator",
				Key:       "some_key",
				Scope:     MemoryScopeSession,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.memory.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSkillMemory_IsExpired(t *testing.T) {
	t.Run("no expiry returns false", func(t *testing.T) {
		m := &SkillMemory{}
		if m.IsExpired() {
			t.Error("IsExpired() should return false when ExpiresAt is nil")
		}
	})

	t.Run("future expiry returns false", func(t *testing.T) {
		future := time.Now().Add(1 * time.Hour)
		m := &SkillMemory{ExpiresAt: &future}
		if m.IsExpired() {
			t.Error("IsExpired() should return false for future expiry")
		}
	})

	t.Run("past expiry returns true", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		m := &SkillMemory{ExpiresAt: &past}
		if !m.IsExpired() {
			t.Error("IsExpired() should return true for past expiry")
		}
	})
}
