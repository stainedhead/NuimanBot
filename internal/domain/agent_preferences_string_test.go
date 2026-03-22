package domain

import (
	"testing"
)

func TestCommunicationStyle_String(t *testing.T) {
	tests := []struct {
		cs   CommunicationStyle
		want string
	}{
		{CommunicationStyleProfessional, "professional"},
		{CommunicationStyleCasual, "casual"},
		{CommunicationStyleTechnical, "technical"},
		{CommunicationStyleFriendly, "friendly"},
		{CommunicationStyle(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.cs), func(t *testing.T) {
			if got := tt.cs.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerbosity_String(t *testing.T) {
	tests := []struct {
		v    Verbosity
		want string
	}{
		{VerbosityConcise, "concise"},
		{VerbosityModerate, "moderate"},
		{VerbosityDetailed, "detailed"},
		{Verbosity(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.v), func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResponseFormat_String(t *testing.T) {
	tests := []struct {
		rf   ResponseFormat
		want string
	}{
		{ResponseFormatMarkdown, "markdown"},
		{ResponseFormatPlain, "plain"},
		{ResponseFormatStructured, "structured"},
		{ResponseFormat(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.rf), func(t *testing.T) {
			if got := tt.rf.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
