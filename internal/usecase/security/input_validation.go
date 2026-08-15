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

	// skipCommandInjection disables Rule 5 entirely (see WithoutCommandInjectionDetection).
	skipCommandInjection bool
}

// InputValidatorOption configures a DefaultInputValidator at construction.
type InputValidatorOption func(*DefaultInputValidator)

// WithoutCommandInjectionDetection disables command-injection scanning (Rule
// 5) entirely for this validator instance — length/null-byte/UTF-8 checks
// and prompt-injection detection still apply.
//
// This exists for the Buzz ACP entrypoint (cmd/nuimanbot/acp.go) specifically
// — not for general use. ACP hosts (confirmed: buzz-acp) bundle a large,
// host-constructed system-context blob into every session/prompt call
// alongside the human's actual message: a "[Base]" system prompt, recent
// conversation history, and a markdown-formatted CLI reference table. That
// bundle is legitimate technical documentation, not arbitrary human chat,
// and it reliably contains characters command-injection scanning treats as
// suspicious purely as a side effect of ordinary technical writing — e.g. a
// semicolon in "`BUZZ_AUTH_TAG`; if it is missing, ..." or angle brackets as
// CLI placeholder syntax in "buzz <group> --help". Selectively removing
// individual characters from the metacharacter list (see
// NewDefaultInputValidator's own doc comment for that earlier, insufficient
// attempt) doesn't converge — technical documentation of any real size will
// always contain some subset of them. Scanning the actual human-authored
// portion specifically isn't done here either: that portion is structurally
// embedded inside the host's own bundle format (e.g. a "Content: ..." line
// within a larger event dump), which is undocumented and specific to
// buzz-acp's implementation, not a stable contract to parse against.
//
// The dangerousCommands/sensitivePaths checks (Rule 5's other two legs) are
// disabled along with the metacharacter check, not selectively — they were
// not confirmed to false-positive on real bundles the way metacharacters
// were, but they exist for the same category of "trust the tool-execution
// RBAC layer, not blind text scanning" reasoning, and a large bundle full of
// CLI documentation is exactly the kind of text likely to mention them
// (e.g. this very doc comment mentions "curl", which is itself in that
// list). Real protection against a message trying to get NuimanBot to
// actually run something dangerous lives in
// tool.Service.ExecuteWithUser's RBAC/confirmation gate, not here.
func WithoutCommandInjectionDetection() InputValidatorOption {
	return func(v *DefaultInputValidator) {
		v.skipCommandInjection = true
	}
}

// NewDefaultInputValidator creates a new instance of DefaultInputValidator.
func NewDefaultInputValidator(opts ...InputValidatorOption) *DefaultInputValidator {
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
	// Every other metacharacter here (";", "&&", "||", "|", "`", "$(",
	// "${", ">>" , ">", "<<", "<") is kept — including "|" and "`", despite
	// both also turning out to be common in ACP-bundled technical
	// documentation (markdown tables/inline code); that problem is handled
	// per-instance via WithoutCommandInjectionDetection (see its doc
	// comment for the full story) rather than by further loosening this
	// shared list, which every other platform (Telegram, Slack, CLI, web
	// Chats, REST API) also relies on. "<"/">" specifically are load-bearing
	// for internal/adapter/api/middleware.Validate(), which reuses this same
	// DefaultInputValidator for REST API request bodies to reject HTML/
	// script-tag payloads (see validate_test.go's <script>/<img onerror=...>
	// cases) — a genuinely different threat model than natural-language chat
	// text.
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

	for _, opt := range opts {
		opt(v)
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

	// Rule 5: Command injection pattern detection (see
	// WithoutCommandInjectionDetection's doc comment for why an instance
	// might skip this).
	if !v.skipCommandInjection && v.detectCommandInjection(input) {
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
