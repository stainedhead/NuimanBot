package common

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizer_RedactGitHubToken(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	// Real GitHub tokens: ghp_ + 36 alphanumeric characters
	input := "Authorization: ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	output := sanitizer.SanitizeOutput(input)

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "ghp_abcdefghijklmnopqrstuvwxyz1234567890")
}

func TestSanitizer_RedactOpenAIKey(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	input := "OPENAI_API_KEY=sk-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKL"
	output := sanitizer.SanitizeOutput(input)

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "sk-")
}

func TestSanitizer_RedactGoogleAPIKey(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	// Real Google API keys: AIza + 35 alphanumeric/dash/underscore characters
	input := "Google API Key: AIzaSyABCDEF1234567890abcdefghijklmnopqrs"
	output := sanitizer.SanitizeOutput(input)

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "AIzaSyABCDEF1234567890abcdefghijklmnopqrs")
}

func TestSanitizer_RedactGenericSecrets(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "api_key",
			input: `config: {"api_key": "secretvalue123456789"}`,
		},
		{
			name:  "password",
			input: "password=mysecretpassword123",
		},
		{
			name:  "token",
			input: "token: mytoken123456789012345678",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := sanitizer.SanitizeOutput(tc.input)
			assert.Contains(t, output, "[REDACTED]", "Should redact secret in: %s", tc.input)
		})
	}
}

func TestSanitizer_NoFalsePositives(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	testCases := []string{
		"This is normal text without secrets",
		"User ID: 12345",
		"HTTP status: 200 OK",
		"ghp is not a secret",
		"sk is not a secret",
	}

	for _, input := range testCases {
		output := sanitizer.SanitizeOutput(input)
		assert.Equal(t, input, output, "Should not modify innocent text: %s", input)
	}
}

func TestSanitizer_MultipleSecrets(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	input := `
		GitHub: ghp_abcdefghijklmnopqrstuvwxyz1234567890
		OpenAI: sk-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKL
		Google: AIzaSyABCDEF1234567890abcdefghijklmnopqrs
	`

	output := sanitizer.SanitizeOutput(input)

	// All secrets should be redacted
	assert.NotContains(t, output, "ghp_abcdefghijklmnopqrstuvwxyz1234567890")
	assert.NotContains(t, output, "sk-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKL")
	assert.NotContains(t, output, "AIzaSyABCDEF1234567890abcdefghijklmnopqrs")

	// Should contain redacted markers
	redactedCount := 0
	for i := 0; i < len(output)-len("[REDACTED]")+1; i++ {
		if i+len("[REDACTED]") <= len(output) && output[i:i+len("[REDACTED]")] == "[REDACTED]" {
			redactedCount++
		}
	}
	assert.GreaterOrEqual(t, redactedCount, 3, "Should redact all 3 secrets")
}

func TestSanitizer_AddPattern(t *testing.T) {
	sanitizer := NewOutputSanitizer()

	// Add custom pattern for credit card numbers (simplified)
	customPattern := regexp.MustCompile(`\d{4}-\d{4}-\d{4}-\d{4}`)
	sanitizer.AddPattern(customPattern)

	input := "Credit Card: 1234-5678-9012-3456"
	output := sanitizer.SanitizeOutput(input)

	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "1234-5678")
}
