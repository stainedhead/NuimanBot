package common

import "regexp"

// OutputSanitizer sanitizes command output by redacting secrets
type OutputSanitizer struct {
	patterns []*regexp.Regexp
}

// NewOutputSanitizer creates a new OutputSanitizer with default secret patterns
func NewOutputSanitizer() *OutputSanitizer {
	return &OutputSanitizer{
		patterns: []*regexp.Regexp{
			// GitHub tokens (ghp_... with 36 chars after prefix)
			regexp.MustCompile(`ghp_[a-zA-Z0-9]{36,}`),
			// OpenAI API keys (sk-... with 48 chars after prefix)
			regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`),
			// Google API keys (AIza... with 35 chars after prefix)
			regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35,}`),
			// Generic passwords and tokens
			regexp.MustCompile(`(?i)(password|token)[\s]*[=:][\s]*["']?([a-zA-Z0-9+/=_-]{12,})["']?`),
			// API keys and secret keys (handles JSON format with quotes)
			regexp.MustCompile(`(?i)["']?(api[_-]?key|secret[_-]?key|aws_access_key_id|aws_secret_access_key)["']?[\s]*[:=][\s]*["']([a-zA-Z0-9+/=_-]{12,})["']`),
		},
	}
}

// SanitizeOutput sanitizes output by redacting matched secret patterns
func (s *OutputSanitizer) SanitizeOutput(output string) string {
	result := output
	for _, pattern := range s.patterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// AddPattern adds a custom pattern to the sanitizer
func (s *OutputSanitizer) AddPattern(pattern *regexp.Regexp) {
	s.patterns = append(s.patterns, pattern)
}
