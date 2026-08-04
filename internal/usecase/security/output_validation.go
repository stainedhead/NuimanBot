package security

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// InjectionWarningMarker is prepended to flagged content when the configured
// action is ValidationActionAnnotate, making the presence of a possible
// injection attempt visible to the model and to a human reviewing transcripts.
const InjectionWarningMarker = "[SECURITY WARNING: possible injected instructions detected]"

// ValidationAction describes what a caller should do with content that
// OutputValidator has flagged as a likely prompt injection.
type ValidationAction string

const (
	// ValidationActionPass means the content was not flagged and needs no
	// special handling.
	ValidationActionPass ValidationAction = "pass"
	// ValidationActionAnnotate means flagged content should be passed through
	// wrapped with InjectionWarningMarker rather than rejected outright.
	ValidationActionAnnotate ValidationAction = "annotate"
	// ValidationActionReject means the tool call should fail closed: flagged
	// content must not reach the LLM. This is the fail-closed default.
	ValidationActionReject ValidationAction = "reject"
)

// ValidationResult is the outcome of scanning third-party-controlled tool
// output for prompt-injection patterns.
type ValidationResult struct {
	// Flagged is true when at least one injection pattern matched.
	Flagged bool
	// MatchedPatterns lists every pattern that matched. Empty when Flagged is false.
	MatchedPatterns []string
	// Action is what the caller should do about flagged content. Always
	// ValidationActionPass when Flagged is false.
	Action ValidationAction
}

// OutputValidator scans content that NuimanBot's tools fetch on the agent's own
// behalf (web pages, search results, MCP responses, and similar third-party
// content) for prompt-injection patterns before that content re-enters the
// LLM's conversation loop.
//
// OutputValidator is deliberately distinct from InputValidator/SecurityService's
// ValidateInput: InputValidator gates direct human chat input and fails the
// request outright. OutputValidator never mutates or rejects content itself; it
// reports a ValidationResult and lets the caller (per configured action) decide
// whether to reject the tool call, annotate the content with a visible warning,
// or pass it through unchanged.
type OutputValidator interface {
	// ValidateToolOutput scans content originating from source (a URL, search
	// result identifier, or "mcp:<server>:<tool>" name, for example) for
	// prompt-injection patterns. Empty, whitespace-only, and non-UTF8 content
	// always pass cleanly (Flagged: false, no error).
	ValidateToolOutput(ctx context.Context, source string, content string) (ValidationResult, error)
}

// DefaultOutputValidator implements OutputValidator using the same
// pattern-matching helper as DefaultInputValidator (detectPromptInjectionPatterns,
// see prompt_injection_patterns.go).
type DefaultOutputValidator struct {
	patterns      []string
	defaultAction ValidationAction
}

// OutputValidatorOption configures a DefaultOutputValidator at construction time.
type OutputValidatorOption func(*DefaultOutputValidator)

// WithDefaultAction overrides the action reported for flagged content. If never
// supplied, the default is ValidationActionReject (fail closed).
func WithDefaultAction(action ValidationAction) OutputValidatorOption {
	return func(v *DefaultOutputValidator) {
		if action != "" {
			v.defaultAction = action
		}
	}
}

// NewDefaultOutputValidator creates a DefaultOutputValidator. By default,
// flagged content is reported with Action: ValidationActionReject (fail closed
// per the security NFR in specs/260802-improve-nuimanbot-security/spec.md).
func NewDefaultOutputValidator(opts ...OutputValidatorOption) *DefaultOutputValidator {
	v := &DefaultOutputValidator{
		patterns:      defaultPromptInjectionPatterns().all(),
		defaultAction: ValidationActionReject,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidateToolOutput scans content for injection patterns. Empty, whitespace-only,
// and non-UTF8 content always pass cleanly (Flagged: false, no error) rather than
// being false-flagged or erroring: there's nothing to protect against in empty
// input, and non-UTF8 bytes cannot contain a matchable text pattern.
func (v *DefaultOutputValidator) ValidateToolOutput(_ context.Context, _ string, content string) (ValidationResult, error) {
	if strings.TrimSpace(content) == "" || !utf8.ValidString(content) {
		return ValidationResult{Flagged: false, Action: ValidationActionPass}, nil
	}

	flagged, matched := detectPromptInjectionPatterns(v.patterns, content)
	if !flagged {
		return ValidationResult{Flagged: false, Action: ValidationActionPass}, nil
	}

	return ValidationResult{
		Flagged:         true,
		MatchedPatterns: matched,
		Action:          v.defaultAction,
	}, nil
}

// NoopOutputValidator implements OutputValidator by never flagging anything.
// It exists so operators can disable tool-output injection scanning via
// security.tool_output_validation.enabled: false while keeping the same
// OutputValidator interface at every call site.
type NoopOutputValidator struct{}

// NewNoopOutputValidator creates a NoopOutputValidator.
func NewNoopOutputValidator() *NoopOutputValidator {
	return &NoopOutputValidator{}
}

// ValidateToolOutput always reports Flagged: false.
func (NoopOutputValidator) ValidateToolOutput(_ context.Context, _ string, _ string) (ValidationResult, error) {
	return ValidationResult{Flagged: false, Action: ValidationActionPass}, nil
}

// AnnotateFlaggedContent wraps flagged content with InjectionWarningMarker so a
// human, downstream guardrail, or the model itself can recognize that the
// content was flagged, per action: annotate.
func AnnotateFlaggedContent(content string) string {
	return InjectionWarningMarker + "\n" + content
}

// FlaggedOutputError indicates a tool call failed because OutputValidator
// flagged its content as a likely prompt injection and the configured action is
// ValidationActionReject. Callers can use errors.As to recover the matched
// patterns for auditing (see internal/usecase/tool/service.go Execute()).
type FlaggedOutputError struct {
	// Source identifies where the flagged content came from (a URL, search
	// result, or MCP tool name, for example).
	Source string
	// MatchedPatterns lists every injection pattern that matched.
	MatchedPatterns []string
}

// Error implements the error interface.
func (e *FlaggedOutputError) Error() string {
	return fmt.Sprintf(
		"content from %q rejected: possible prompt injection detected (%d pattern match(es))",
		e.Source, len(e.MatchedPatterns),
	)
}
