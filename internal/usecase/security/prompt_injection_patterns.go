package security

import "strings"

// promptInjectionPatternGroups holds the categorized keyword patterns used to
// detect likely prompt-injection / jailbreak attempts in text content. The same
// pattern set is shared by DefaultInputValidator (direct human chat input, which
// fails the request outright on a match) and DefaultOutputValidator (third-party
// tool-fetched content, which reports a ValidationResult and lets the caller
// decide what to do — see output_validation.go).
type promptInjectionPatternGroups struct {
	jailbreak  []string
	role       []string
	disclosure []string
	output     []string
}

// defaultPromptInjectionPatterns returns the built-in categorized pattern set.
func defaultPromptInjectionPatterns() promptInjectionPatternGroups {
	return promptInjectionPatternGroups{
		jailbreak: []string{
			"ignore previous instructions",
			"ignore all previous",
			"disregard previous",
			"forget previous",
			"new instructions:",
			"system override",
			"reset instructions",
			"clear instructions",
			"override previous",
		},
		role: []string{
			"you are now",
			"act as if you are",
			"pretend you are",
			"as an ai model",
			"you must now",
			"from now on",
			"act as",
			"behave as",
			"roleplay as",
		},
		disclosure: []string{
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
		},
		output: []string{
			"output raw",
			"return unfiltered",
			"bypass filter",
			"skip validation",
			"ignore safety",
			"disable safety",
			"without filter",
			"unfiltered response",
		},
	}
}

// all returns every pattern across all categories, combined into a single slice.
func (g promptInjectionPatternGroups) all() []string {
	combined := make([]string, 0, len(g.jailbreak)+len(g.role)+len(g.disclosure)+len(g.output))
	combined = append(combined, g.jailbreak...)
	combined = append(combined, g.role...)
	combined = append(combined, g.disclosure...)
	combined = append(combined, g.output...)
	return combined
}

// detectPromptInjectionPatterns scans input (case-insensitively) for any of the
// given patterns and returns whether at least one matched along with the list of
// every pattern that matched. It is the shared helper behind both
// DefaultInputValidator.detectPromptInjection and DefaultOutputValidator.
//
// Both the input and each pattern have runs of whitespace (spaces, tabs,
// newlines, etc.) collapsed to a single space before matching (FR-011). This
// closes a trivial evasion where an attacker inserts extra whitespace into an
// otherwise-known pattern (e.g. "ignore  previous instructions" with a double
// space) to slip past exact substring matching. It intentionally does not
// touch punctuation or attempt any broader fuzzy matching — the pattern list
// itself is never modified.
func detectPromptInjectionPatterns(patterns []string, input string) (bool, []string) {
	normalizedInput := normalizeWhitespace(strings.ToLower(input))

	var matched []string
	for _, pattern := range patterns {
		if strings.Contains(normalizedInput, normalizeWhitespace(pattern)) {
			matched = append(matched, pattern)
		}
	}

	return len(matched) > 0, matched
}

// normalizeWhitespace collapses every run of one or more whitespace
// characters (as defined by unicode.IsSpace: spaces, tabs, newlines, etc.)
// into a single space. It does not trim leading/trailing whitespace beyond
// what collapsing naturally produces, nor does it alter non-whitespace
// characters.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
