package security

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DefaultInputValidator implements the InputValidator interface.
type DefaultInputValidator struct {
	// Add configurable patterns for injection detection if needed
}

// NewDefaultInputValidator creates a new instance of DefaultInputValidator.
func NewDefaultInputValidator() *DefaultInputValidator {
	return &DefaultInputValidator{}
}

// ValidateInput sanitizes and validates user input according to defined rules.
func (v *DefaultInputValidator) ValidateInput(ctx context.Context, input string, maxLength int) (string, error) {
	// Rule 1: Maximum input length
	if len(input) > maxLength {
		return "", fmt.Errorf("input exceeds maximum length of %d bytes", maxLength)
	}

	// Rule 2: No null bytes
	if strings.ContainsRune(input, '\x00') {
		return "", errors.New("input contains null bytes")
	}

	// Rule 3: UTF-8 validation
	if !utf8.ValidString(input) {
		return "", errors.New("input is not valid UTF-8")
	}

	// Rule 4: Prompt injection pattern detection
	if v.detectPromptInjection(input) {
		return "", errors.New("input detected as potential prompt injection")
	}

	// Rule 5: Command injection pattern detection
	if v.detectCommandInjection(input) {
		return "", errors.New("input detected as potential command injection")
	}

	// Basic sanitization: trim whitespace
	sanitizedInput := strings.TrimSpace(input)

	return sanitizedInput, nil
}

// detectPromptInjection is a placeholder for detecting prompt injection patterns.
// This would typically involve more sophisticated regex or ML-based detection.
func (v *DefaultInputValidator) detectPromptInjection(input string) bool {
	lowerInput := strings.ToLower(input)
	// Simple keywords for demonstration. In a real system, this would be robust.
	if strings.Contains(lowerInput, "ignore previous instructions") ||
		strings.Contains(lowerInput, "as an ai model") ||
		strings.Contains(lowerInput, "system override") {
		return true
	}
	return false
}

// detectCommandInjection is a placeholder for detecting command injection patterns.
// This would involve checking for shell-specific metacharacters and commands.
func (v *DefaultInputValidator) detectCommandInjection(input string) bool {
	lowerInput := strings.ToLower(input)
	// Simple checks for common shell commands/metacharacters.
	// This list needs to be comprehensive and carefully maintained.
	dangerousPatterns := []string{
		";", "&&", "||", "`", "$(",
		"rm ", "mv ", "cp ", "chmod ", "chown ", "sudo ",
		"wget ", "curl ", "nc ", "bash ", "sh ", "powershell ",
		"/etc/passwd", "/bin/bash",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}
	return false
}
