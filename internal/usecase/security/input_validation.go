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
	// Pre-allocated patterns for prompt injection detection
	jailbreakPatterns  []string
	rolePatterns       []string
	disclosurePatterns []string
	outputPatterns     []string
	promptPatterns     []string // Combined patterns for prompt injection

	// Pre-allocated patterns for command injection detection
	metacharacters    []string
	dangerousCommands []string
	sensitivePaths    []string
}

// NewDefaultInputValidator creates a new instance of DefaultInputValidator.
func NewDefaultInputValidator() *DefaultInputValidator {
	v := &DefaultInputValidator{}

	// Initialize prompt injection patterns once
	v.jailbreakPatterns = []string{
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

	v.rolePatterns = []string{
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

	v.disclosurePatterns = []string{
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

	v.outputPatterns = []string{
		"output raw",
		"return unfiltered",
		"bypass filter",
		"skip validation",
		"ignore safety",
		"disable safety",
		"without filter",
		"unfiltered response",
	}

	// Pre-combine all prompt patterns for faster lookup
	v.promptPatterns = make([]string, 0, len(v.jailbreakPatterns)+len(v.rolePatterns)+len(v.disclosurePatterns)+len(v.outputPatterns))
	v.promptPatterns = append(v.promptPatterns, v.jailbreakPatterns...)
	v.promptPatterns = append(v.promptPatterns, v.rolePatterns...)
	v.promptPatterns = append(v.promptPatterns, v.disclosurePatterns...)
	v.promptPatterns = append(v.promptPatterns, v.outputPatterns...)

	// Initialize command injection patterns once
	v.metacharacters = []string{
		";", "&&", "||", "|", "`", "$(",
		"$(", "${", ")", ">>", ">", "<<", "<",
		"\n", "\r",
	}

	v.dangerousCommands = []string{
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

	v.sensitivePaths = []string{
		"/etc/passwd", "/etc/shadow", "/etc/sudoers",
		"/root/", "/.ssh/", "~/.ssh/",
		"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
		"c:\\windows\\", "c:\\system32\\",
		"/proc/", "/sys/",
	}

	return v
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

	// Check pre-allocated combined patterns
	for _, pattern := range v.promptPatterns {
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

	// Check pre-allocated metacharacters
	for _, meta := range v.metacharacters {
		if strings.Contains(input, meta) {
			return true
		}
	}

	// Check pre-allocated dangerous commands
	for _, cmd := range v.dangerousCommands {
		if strings.Contains(lowerInput, cmd) {
			return true
		}
	}

	// Check pre-allocated sensitive paths
	for _, path := range v.sensitivePaths {
		if strings.Contains(lowerInput, path) {
			return true
		}
	}

	return false
}
