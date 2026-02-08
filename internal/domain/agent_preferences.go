package domain

import (
	"errors"
)

// CommunicationStyle represents the agent's communication style preference.
// Empty values are treated as valid and will use the default style.
type CommunicationStyle string

const (
	CommunicationStyleProfessional CommunicationStyle = "professional" // Formal, business-like
	CommunicationStyleCasual       CommunicationStyle = "casual"       // Informal, friendly
	CommunicationStyleTechnical    CommunicationStyle = "technical"    // Precise, technical jargon
	CommunicationStyleFriendly     CommunicationStyle = "friendly"     // Warm, personable
)

// String returns the string representation of the communication style.
func (cs CommunicationStyle) String() string {
	return string(cs)
}

// IsValid checks if the communication style is one of the defined constants.
// Empty string is considered valid and will use the default.
func (cs CommunicationStyle) IsValid() bool {
	if cs == "" {
		return true
	}
	return cs == CommunicationStyleProfessional ||
		cs == CommunicationStyleCasual ||
		cs == CommunicationStyleTechnical ||
		cs == CommunicationStyleFriendly
}

// Verbosity represents the level of detail in agent responses.
// Empty values are treated as valid and will use the default verbosity.
type Verbosity string

const (
	VerbosityConcise  Verbosity = "concise"  // Brief, to-the-point responses
	VerbosityModerate Verbosity = "moderate" // Balanced detail
	VerbosityDetailed Verbosity = "detailed" // Comprehensive, thorough responses
)

// String returns the string representation of the verbosity level.
func (v Verbosity) String() string {
	return string(v)
}

// IsValid checks if the verbosity level is one of the defined constants.
// Empty string is considered valid and will use the default.
func (v Verbosity) IsValid() bool {
	if v == "" {
		return true
	}
	return v == VerbosityConcise || v == VerbosityModerate || v == VerbosityDetailed
}

// ResponseFormat represents the format of agent responses.
// Empty values are treated as valid and will use the default format.
type ResponseFormat string

const (
	ResponseFormatMarkdown   ResponseFormat = "markdown"   // Markdown formatting
	ResponseFormatPlain      ResponseFormat = "plain"      // Plain text, no formatting
	ResponseFormatStructured ResponseFormat = "structured" // Structured data (JSON, tables)
)

// String returns the string representation of the response format.
func (rf ResponseFormat) String() string {
	return string(rf)
}

// IsValid checks if the response format is one of the defined constants.
// Empty string is considered valid and will use the default.
func (rf ResponseFormat) IsValid() bool {
	if rf == "" {
		return true
	}
	return rf == ResponseFormatMarkdown || rf == ResponseFormatPlain || rf == ResponseFormatStructured
}

// SkillConfig stores default configuration for a specific skill.
type SkillConfig struct {
	AutoExecute bool           `json:"autoExecute"` // Execute without confirmation
	Options     map[string]any `json:"options"`     // Skill-specific options
}

// NotificationPreferences controls when user receives notifications.
type NotificationPreferences struct {
	TaskCompletion bool `json:"taskCompletion"` // Notify on task completion
	Errors         bool `json:"errors"`         // Notify on errors
	LongRunningOps bool `json:"longRunningOps"` // Notify for operations longer than 30 seconds
}

// AgentPreferences stores user-specific preferences for agent behavior.
// Controls communication style, verbosity, formatting, and feature defaults.
// This is a value object that is part of UserProfile.
type AgentPreferences struct {
	// Communication Style
	CommunicationStyle CommunicationStyle `json:"communicationStyle"` // professional, casual, technical, friendly
	Verbosity          Verbosity          `json:"verbosity"`          // concise, moderate, detailed
	ResponseFormat     ResponseFormat     `json:"responseFormat"`     // markdown, plain, structured

	// Content Preferences
	CodeExamplesPreferred bool `json:"codeExamplesPreferred"` // Include code examples in responses
	ExplainDecisions      bool `json:"explainDecisions"`      // Explain reasoning behind answers
	ProactiveMode         bool `json:"proactiveMode"`         // Offer suggestions proactively

	// Skill Defaults
	SkillDefaults map[string]SkillConfig `json:"skillDefaults"` // Per-skill default settings

	// Notification Preferences
	NotificationPreferences NotificationPreferences `json:"notificationPreferences"` // When to notify user
}

// NewAgentPreferences creates a new AgentPreferences with sensible default values.
// Default: professional communication, moderate verbosity, markdown formatting.
func NewAgentPreferences() AgentPreferences {
	return AgentPreferences{
		CommunicationStyle:    CommunicationStyleProfessional,
		Verbosity:             VerbosityModerate,
		ResponseFormat:        ResponseFormatMarkdown,
		CodeExamplesPreferred: true,
		ExplainDecisions:      false,
		ProactiveMode:         true,
		SkillDefaults: map[string]SkillConfig{
			"commit": {
				AutoExecute: false,
				Options: map[string]any{
					"autoStage": true,
					"signoff":   true,
				},
			},
		},
		NotificationPreferences: NotificationPreferences{
			TaskCompletion: true,
			Errors:         true,
			LongRunningOps: true,
		},
	}
}

// Validate checks if the agent preferences are valid.
// Validates communication style, verbosity, and response format enums.
func (ap *AgentPreferences) Validate() error {
	if !ap.CommunicationStyle.IsValid() {
		return errors.New("invalid communication style")
	}
	if !ap.Verbosity.IsValid() {
		return errors.New("invalid verbosity")
	}
	if !ap.ResponseFormat.IsValid() {
		return errors.New("invalid response format")
	}
	return nil
}
