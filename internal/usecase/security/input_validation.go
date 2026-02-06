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

// detectPromptInjection detects prompt injection patterns using comprehensive keyword matching.
// This implementation uses pattern matching for common jailbreak and manipulation attempts.
func (v *DefaultInputValidator) detectPromptInjection(input string) bool {
	lowerInput := strings.ToLower(input)

	// Jailbreak attempts - trying to override previous instructions
	jailbreakPatterns := []string{
		"ignore previous instructions",
		"ignore all previous",
		"disregard previous",
		"forget previous",
		"new instructions:",
		"system override",
		"reset instructions",
		"clear instructions",
		"override previous",
	}

	// Role manipulation - trying to change the AI's behavior or role
	rolePatterns := []string{
		"you are now",
		"act as if you are",
		"pretend you are",
		"as an ai model",
		"you must now",
		"from now on",
		"act as",
		"behave as",
		"roleplay as",
	}

	// Information disclosure attempts - trying to reveal system prompts or instructions
	disclosurePatterns := []string{
		"reveal your prompt",
		"show your instructions",
		"what are your rules",
		"repeat your system prompt",
		"tell me your guidelines",
		"print your configuration",
		"show your system prompt",
		"repeat your instructions",
		"what is your system message",
		"show me your prompt",
	}

	// Output manipulation - trying to bypass filters or safety measures
	outputPatterns := []string{
		"output raw",
		"return unfiltered",
		"bypass filter",
		"skip validation",
		"ignore safety",
		"disable safety",
		"without filter",
		"unfiltered response",
	}

	// Check all pattern categories
	allPatterns := append(jailbreakPatterns, rolePatterns...)
	allPatterns = append(allPatterns, disclosurePatterns...)
	allPatterns = append(allPatterns, outputPatterns...)

	for _, pattern := range allPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// detectCommandInjection detects command injection patterns using comprehensive checks
// for shell metacharacters, dangerous commands, and sensitive file paths.
func (v *DefaultInputValidator) detectCommandInjection(input string) bool {
	lowerInput := strings.ToLower(input)

	// Shell metacharacters - characters used for command chaining and injection
	metacharacters := []string{
		";", "&&", "||", "|", "`", "$(",
		"$(", "${", ")", ">>", ">", "<<", "<",
		"\n", "\r",
	}

	// Dangerous commands - expanded list of commands that could be malicious
	dangerousCommands := []string{
		// File operations
		"rm ", "mv ", "cp ", "dd ", "shred ",
		// Permission changes
		"chmod ", "chown ", "chgrp ",
		// Privilege escalation
		"sudo ", "su ", "doas ",
		// Network operations
		"wget ", "curl ", "nc ", "netcat ", "telnet ", "ssh ", "scp ",
		// Shell invocations
		"bash ", "sh ", "zsh ", "fish ", "dash ",
		"powershell ", "pwsh ", "cmd ", "command.com ",
		// System manipulation
		"kill ", "pkill ", "systemctl ", "service ",
		"reboot ", "shutdown ", "halt ", "poweroff ",
		// Package management
		"apt ", "yum ", "dnf ", "pacman ", "brew ",
		// Encoding/decoding (often used in attacks)
		"base64 ", "xxd ", "od ",
		// Process inspection
		"ps ", "top ", "htop ",
		// File inspection
		"cat ", "less ", "more ", "head ", "tail ",
	}

	// Sensitive paths - file paths that should never be accessed
	sensitivePaths := []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"/root/", "/.ssh/", "~/.ssh/",
		"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
		"c:\\windows\\", "c:\\system32\\",
		"/proc/", "/sys/",
	}

	// Check metacharacters
	for _, meta := range metacharacters {
		if strings.Contains(input, meta) {
			return true
		}
	}

	// Check dangerous commands
	for _, cmd := range dangerousCommands {
		if strings.Contains(lowerInput, cmd) {
			return true
		}
	}

	// Check sensitive paths
	for _, path := range sensitivePaths {
		if strings.Contains(lowerInput, path) {
			return true
		}
	}

	return false
}
