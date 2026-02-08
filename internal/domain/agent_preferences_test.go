package domain

import (
	"encoding/json"
	"testing"
)

// TestNewAgentPreferences tests the constructor returns valid defaults
func TestNewAgentPreferences(t *testing.T) {
	prefs := NewAgentPreferences()

	// Verify default values
	if prefs.CommunicationStyle != CommunicationStyleProfessional {
		t.Errorf("expected CommunicationStyle=%s, got %s", CommunicationStyleProfessional, prefs.CommunicationStyle)
	}
	if prefs.Verbosity != VerbosityModerate {
		t.Errorf("expected Verbosity=%s, got %s", VerbosityModerate, prefs.Verbosity)
	}
	if prefs.ResponseFormat != ResponseFormatMarkdown {
		t.Errorf("expected ResponseFormat=%s, got %s", ResponseFormatMarkdown, prefs.ResponseFormat)
	}
	if !prefs.CodeExamplesPreferred {
		t.Error("expected CodeExamplesPreferred=true")
	}
	if prefs.ExplainDecisions {
		t.Error("expected ExplainDecisions=false")
	}
	if !prefs.ProactiveMode {
		t.Error("expected ProactiveMode=true")
	}
	if prefs.SkillDefaults == nil {
		t.Error("expected SkillDefaults to be initialized")
	}
	if !prefs.NotificationPreferences.TaskCompletion {
		t.Error("expected NotificationPreferences.TaskCompletion=true")
	}
	if !prefs.NotificationPreferences.Errors {
		t.Error("expected NotificationPreferences.Errors=true")
	}
	if !prefs.NotificationPreferences.LongRunningOps {
		t.Error("expected NotificationPreferences.LongRunningOps=true")
	}
}

// TestAgentPreferencesValidate_Valid tests validation passes for valid preferences
func TestAgentPreferencesValidate_Valid(t *testing.T) {
	tests := []struct {
		name  string
		prefs AgentPreferences
	}{
		{
			name:  "default preferences",
			prefs: NewAgentPreferences(),
		},
		{
			name: "casual technical style",
			prefs: AgentPreferences{
				CommunicationStyle: CommunicationStyleCasual,
				Verbosity:          VerbosityDetailed,
				ResponseFormat:     ResponseFormatPlain,
				SkillDefaults:      make(map[string]SkillConfig),
				NotificationPreferences: NotificationPreferences{
					TaskCompletion: false,
					Errors:         true,
					LongRunningOps: false,
				},
			},
		},
		{
			name: "friendly technical with skill defaults",
			prefs: AgentPreferences{
				CommunicationStyle: CommunicationStyleFriendly,
				Verbosity:          VerbosityConcise,
				ResponseFormat:     ResponseFormatStructured,
				SkillDefaults: map[string]SkillConfig{
					"commit": {
						AutoExecute: true,
						Options:     map[string]any{"autoStage": true},
					},
				},
				NotificationPreferences: NotificationPreferences{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.prefs.Validate(); err != nil {
				t.Errorf("expected valid preferences, got error: %v", err)
			}
		})
	}
}

// TestAgentPreferencesValidate_Invalid tests validation fails for invalid preferences
func TestAgentPreferencesValidate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		prefs   AgentPreferences
		wantErr string
	}{
		{
			name: "invalid communication style",
			prefs: AgentPreferences{
				CommunicationStyle: "invalid",
				Verbosity:          VerbosityModerate,
				ResponseFormat:     ResponseFormatMarkdown,
			},
			wantErr: "invalid communication style",
		},
		{
			name: "invalid verbosity",
			prefs: AgentPreferences{
				CommunicationStyle: CommunicationStyleProfessional,
				Verbosity:          "invalid",
				ResponseFormat:     ResponseFormatMarkdown,
			},
			wantErr: "invalid verbosity",
		},
		{
			name: "invalid response format",
			prefs: AgentPreferences{
				CommunicationStyle: CommunicationStyleProfessional,
				Verbosity:          VerbosityModerate,
				ResponseFormat:     "invalid",
			},
			wantErr: "invalid response format",
		},
		{
			name: "empty communication style allowed",
			prefs: AgentPreferences{
				CommunicationStyle: "",
				Verbosity:          VerbosityModerate,
				ResponseFormat:     ResponseFormatMarkdown,
			},
			wantErr: "", // Empty is valid - means use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prefs.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

// TestAgentPreferencesJSON tests JSON marshaling and unmarshaling
func TestAgentPreferencesJSON(t *testing.T) {
	original := AgentPreferences{
		CommunicationStyle:    CommunicationStyleTechnical,
		Verbosity:             VerbosityDetailed,
		ResponseFormat:        ResponseFormatStructured,
		CodeExamplesPreferred: true,
		ExplainDecisions:      true,
		ProactiveMode:         false,
		SkillDefaults: map[string]SkillConfig{
			"commit": {
				AutoExecute: true,
				Options: map[string]any{
					"autoStage": true,
					"signoff":   true,
				},
			},
		},
		NotificationPreferences: NotificationPreferences{
			TaskCompletion: true,
			Errors:         true,
			LongRunningOps: false,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal from JSON
	var decoded AgentPreferences
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Compare fields
	if decoded.CommunicationStyle != original.CommunicationStyle {
		t.Errorf("CommunicationStyle mismatch: got %s, want %s", decoded.CommunicationStyle, original.CommunicationStyle)
	}
	if decoded.Verbosity != original.Verbosity {
		t.Errorf("Verbosity mismatch: got %s, want %s", decoded.Verbosity, original.Verbosity)
	}
	if decoded.ResponseFormat != original.ResponseFormat {
		t.Errorf("ResponseFormat mismatch: got %s, want %s", decoded.ResponseFormat, original.ResponseFormat)
	}
	if decoded.CodeExamplesPreferred != original.CodeExamplesPreferred {
		t.Errorf("CodeExamplesPreferred mismatch: got %v, want %v", decoded.CodeExamplesPreferred, original.CodeExamplesPreferred)
	}
	if decoded.ExplainDecisions != original.ExplainDecisions {
		t.Errorf("ExplainDecisions mismatch: got %v, want %v", decoded.ExplainDecisions, original.ExplainDecisions)
	}
	if decoded.ProactiveMode != original.ProactiveMode {
		t.Errorf("ProactiveMode mismatch: got %v, want %v", decoded.ProactiveMode, original.ProactiveMode)
	}
	if decoded.NotificationPreferences.TaskCompletion != original.NotificationPreferences.TaskCompletion {
		t.Errorf("NotificationPreferences.TaskCompletion mismatch")
	}
	if decoded.NotificationPreferences.Errors != original.NotificationPreferences.Errors {
		t.Errorf("NotificationPreferences.Errors mismatch")
	}
	if decoded.NotificationPreferences.LongRunningOps != original.NotificationPreferences.LongRunningOps {
		t.Errorf("NotificationPreferences.LongRunningOps mismatch")
	}
}

// TestAgentPreferencesJSON_NilMaps tests JSON handling with nil maps
func TestAgentPreferencesJSON_NilMaps(t *testing.T) {
	original := AgentPreferences{
		CommunicationStyle: CommunicationStyleProfessional,
		Verbosity:          VerbosityModerate,
		ResponseFormat:     ResponseFormatMarkdown,
		SkillDefaults:      nil, // nil map should be handled gracefully
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal from JSON
	var decoded AgentPreferences
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Validation should still pass
	if err := decoded.Validate(); err != nil {
		t.Errorf("validation failed after unmarshal: %v", err)
	}
}

// TestCommunicationStyleValidation tests CommunicationStyle validation
func TestCommunicationStyleValidation(t *testing.T) {
	tests := []struct {
		style CommunicationStyle
		valid bool
	}{
		{CommunicationStyleProfessional, true},
		{CommunicationStyleCasual, true},
		{CommunicationStyleTechnical, true},
		{CommunicationStyleFriendly, true},
		{"", true}, // Empty is valid
		{"invalid", false},
		{"Professional", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.style), func(t *testing.T) {
			valid := tt.style.IsValid()
			if valid != tt.valid {
				t.Errorf("IsValid() = %v, want %v", valid, tt.valid)
			}
		})
	}
}

// TestVerbosityValidation tests Verbosity validation
func TestVerbosityValidation(t *testing.T) {
	tests := []struct {
		verbosity Verbosity
		valid     bool
	}{
		{VerbosityConcise, true},
		{VerbosityModerate, true},
		{VerbosityDetailed, true},
		{"", true}, // Empty is valid
		{"invalid", false},
		{"Concise", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.verbosity), func(t *testing.T) {
			valid := tt.verbosity.IsValid()
			if valid != tt.valid {
				t.Errorf("IsValid() = %v, want %v", valid, tt.valid)
			}
		})
	}
}

// TestResponseFormatValidation tests ResponseFormat validation
func TestResponseFormatValidation(t *testing.T) {
	tests := []struct {
		format ResponseFormat
		valid  bool
	}{
		{ResponseFormatMarkdown, true},
		{ResponseFormatPlain, true},
		{ResponseFormatStructured, true},
		{"", true}, // Empty is valid
		{"invalid", false},
		{"Markdown", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			valid := tt.format.IsValid()
			if valid != tt.valid {
				t.Errorf("IsValid() = %v, want %v", valid, tt.valid)
			}
		})
	}
}
