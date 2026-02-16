package domain

import (
	"strings"
	"testing"
)

func TestRulesConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *RulesConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid empty config",
			config:  &RulesConfig{},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: &RulesConfig{
				RequiresConfirmation: []string{"delete_file", "send_message"},
				BlockedTools:         []string{"rm_rf", "drop_table"},
				Privacy: PrivacyConfig{
					NeverStore: []string{"password", "api_key"},
				},
				RawYAML: "requires_confirmation:\n  - delete_file\n",
			},
			wantErr: false,
		},
		{
			name: "valid identifiers with numbers",
			config: &RulesConfig{
				RequiresConfirmation: []string{"action1", "action_2"},
				BlockedTools:         []string{"tool3"},
			},
			wantErr: false,
		},
		{
			name: "invalid identifier in requires_confirmation",
			config: &RulesConfig{
				RequiresConfirmation: []string{"valid_action", "invalid-action"},
			},
			wantErr: true,
			errMsg:  "requires_confirmation",
		},
		{
			name: "invalid identifier with spaces in requires_confirmation",
			config: &RulesConfig{
				RequiresConfirmation: []string{"has space"},
			},
			wantErr: true,
			errMsg:  "requires_confirmation",
		},
		{
			name: "invalid identifier in blocked_tools",
			config: &RulesConfig{
				BlockedTools: []string{"valid_tool", "invalid.tool"},
			},
			wantErr: true,
			errMsg:  "blocked_tools",
		},
		{
			name: "invalid identifier in privacy never_store",
			config: &RulesConfig{
				Privacy: PrivacyConfig{
					NeverStore: []string{"valid_item", "invalid/item"},
				},
			},
			wantErr: true,
			errMsg:  "privacy.never_store",
		},
		{
			name: "empty string identifier in requires_confirmation",
			config: &RulesConfig{
				RequiresConfirmation: []string{""},
			},
			wantErr: true,
			errMsg:  "requires_confirmation",
		},
		{
			name: "duplicate entries in requires_confirmation",
			config: &RulesConfig{
				RequiresConfirmation: []string{"delete_file", "delete_file"},
			},
			wantErr: true,
			errMsg:  "requires_confirmation",
		},
		{
			name: "duplicate entries in blocked_tools",
			config: &RulesConfig{
				BlockedTools: []string{"rm_rf", "rm_rf"},
			},
			wantErr: true,
			errMsg:  "blocked_tools",
		},
		{
			name: "duplicate entries in privacy never_store",
			config: &RulesConfig{
				Privacy: PrivacyConfig{
					NeverStore: []string{"password", "password"},
				},
			},
			wantErr: true,
			errMsg:  "privacy.never_store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RulesConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("RulesConfig.Validate() error = %q, want it to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestRulesConfig_RequiresConfirmationFor(t *testing.T) {
	config := &RulesConfig{
		RequiresConfirmation: []string{"delete_file", "send_message", "drop_table"},
	}

	tests := []struct {
		name   string
		action string
		want   bool
	}{
		{
			name:   "action requires confirmation",
			action: "delete_file",
			want:   true,
		},
		{
			name:   "another action requires confirmation",
			action: "send_message",
			want:   true,
		},
		{
			name:   "action does not require confirmation",
			action: "read_file",
			want:   false,
		},
		{
			name:   "empty action",
			action: "",
			want:   false,
		},
		{
			name:   "case sensitive match",
			action: "Delete_File",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.RequiresConfirmationFor(tt.action)
			if got != tt.want {
				t.Errorf("RequiresConfirmationFor(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestRulesConfig_RequiresConfirmationFor_EmptyList(t *testing.T) {
	config := &RulesConfig{}
	if config.RequiresConfirmationFor("any_action") {
		t.Error("RequiresConfirmationFor() should return false when list is empty")
	}
}

func TestRulesConfig_IsToolBlocked(t *testing.T) {
	config := &RulesConfig{
		BlockedTools: []string{"rm_rf", "drop_table", "format_disk"},
	}

	tests := []struct {
		name string
		tool string
		want bool
	}{
		{
			name: "tool is blocked",
			tool: "rm_rf",
			want: true,
		},
		{
			name: "another tool is blocked",
			tool: "drop_table",
			want: true,
		},
		{
			name: "tool is not blocked",
			tool: "read_file",
			want: false,
		},
		{
			name: "empty tool name",
			tool: "",
			want: false,
		},
		{
			name: "case sensitive match",
			tool: "Rm_Rf",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsToolBlocked(tt.tool)
			if got != tt.want {
				t.Errorf("IsToolBlocked(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}

func TestRulesConfig_IsToolBlocked_EmptyList(t *testing.T) {
	config := &RulesConfig{}
	if config.IsToolBlocked("any_tool") {
		t.Error("IsToolBlocked() should return false when list is empty")
	}
}

func TestPrivacyConfig_ZeroValue(t *testing.T) {
	config := &RulesConfig{}
	if config.Privacy.NeverStore != nil {
		t.Error("PrivacyConfig.NeverStore should be nil by default")
	}
}
