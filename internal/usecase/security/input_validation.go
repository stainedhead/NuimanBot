package security

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"nuimanbot/internal/domain"
)

// DefaultInputValidator implements the InputValidator interface.
type DefaultInputValidator struct {
	// Pre-combined prompt injection patterns, shared with DefaultOutputValidator
	// via detectPromptInjectionPatterns (see prompt_injection_patterns.go).
	promptPatterns []string

	// Pre-allocated patterns for command injection detection
	metacharacters    []string
	dangerousCommands []string
	sensitivePaths    []string
}

// NewDefaultInputValidator creates a new instance of DefaultInputValidator.
func NewDefaultInputValidator() *DefaultInputValidator {
	v := &DefaultInputValidator{}

	// Initialize prompt injection patterns once, from the shared pattern set.
	v.promptPatterns = defaultPromptInjectionPatterns().all()

	// Initialize command injection patterns once. Deliberately excludes a
	// bare ")" and "\n"/"\r" — extremely common in ordinary conversational
	// text (parentheticals, ":)" emoticons, any multi-line message) and not
	// meaningful shell-injection signals on their own. Confirmed via a real
	// production false positive: a normal multi-sentence chat message sent
	// through the Buzz ACP harness was rejected outright as "potential
	// command injection" purely for containing a newline, with no actual
	// shell content anywhere in it — and since Buzz's delivery queue is
	// per-channel FIFO with retry, that one rejected message blocked every
	// later message in the same channel too.
	//
	// "<"/">"/"<<"/">>" are NOT excluded, despite being common in casual
	// chat (comparisons, ">> quoted" reply style) — internal/adapter/api/
	// middleware.Validate() reuses this same DefaultInputValidator for REST
	// API request bodies, where "<"/">" are load-bearing for rejecting HTML/
	// script-tag payloads (see validate_test.go's <script>/<img onerror=...>
	// cases) — a genuinely different threat model (data that may later be
	// rendered as HTML) than natural-language chat text. Splitting these two
	// call sites onto separately-configured validator instances would let
	// chat text drop "<"/">" too without weakening the API's XSS check, but
	// wasn't undertaken here — this fix targets the confirmed, actively
	// user-blocking newline false positive specifically.
	v.metacharacters = []string{
		";", "&&", "||", "|", "`", "$(", "${", ">>", ">", "<<", "<",
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
		return "", domain.NewUserError(
			"INPUT_TOO_LONG",
			fmt.Sprintf("input exceeds maximum length of %d bytes", maxLength),
			fmt.Sprintf("Your message is too long. Please keep it under %d characters.", maxLength),
		)
	}

	// Rule 2: No null bytes
	if strings.ContainsRune(input, '\x00') {
		return "", domain.NewUserError(
			"INVALID_CHARACTERS",
			"input contains null bytes",
			"Your message contains invalid characters. Please remove them and try again.",
		)
	}

	// Rule 3: UTF-8 validation
	if !utf8.ValidString(input) {
		return "", domain.NewUserError(
			"INVALID_ENCODING",
			"input is not valid UTF-8",
			"Your message contains invalid text encoding. Please check your message and try again.",
		)
	}

	// Rule 4: Prompt injection pattern detection
	if v.detectPromptInjection(input) {
		return "", domain.NewUserError(
			"SUSPICIOUS_INPUT",
			"input detected as potential prompt injection",
			"Your message appears to contain potentially harmful content. Please rephrase and try again.",
		)
	}

	// Rule 5: Command injection pattern detection
	if v.detectCommandInjection(input) {
		return "", domain.NewUserError(
			"SUSPICIOUS_INPUT",
			"input detected as potential command injection",
			"Your message appears to contain potentially harmful content. Please rephrase and try again.",
		)
	}

	// Basic sanitization: trim whitespace
	sanitizedInput := strings.TrimSpace(input)

	return sanitizedInput, nil
}

// detectPromptInjection detects prompt injection patterns using comprehensive keyword matching.
// This implementation uses pattern matching for common jailbreak and manipulation attempts.
// It delegates to the shared detectPromptInjectionPatterns helper (see
// prompt_injection_patterns.go), which also backs DefaultOutputValidator.
func (v *DefaultInputValidator) detectPromptInjection(input string) bool {
	flagged, _ := detectPromptInjectionPatterns(v.promptPatterns, input)
	return flagged
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
