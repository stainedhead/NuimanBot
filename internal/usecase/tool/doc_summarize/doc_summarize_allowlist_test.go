package doc_summarize

import (
	"context"
	"testing"

	"nuimanbot/internal/domain"
)

// TestDocSummarizeSkill_ValidateURL_AllowlistExactOrSubdomainOnly is the
// regression test for FR-R08/FR-008 (P1): the domain allowlist check used
// strings.Contains(parsedURL.Host, domain), which matches a configured
// allowlist entry as a plain substring anywhere in the host — not just as
// the exact domain or one of its subdomains. With only "github.com"
// allowlisted, that let a host like "notgithub.com.attacker.net" (contains
// "github.com" as a substring) or "github.com.evil.example" (starts with
// "github.com" followed by an attacker-controlled suffix) sail through the
// allowlist meant to restrict fetches to GitHub.
//
// SSRF protection is disabled for this test via SetSSRFProtection(false) so
// only the allowlist-matching logic is exercised in isolation — these
// synthetic hostnames don't resolve via real DNS, and the IP-range SSRF
// check (common.ResolveValidatedIP) is a separate, additional layer this
// test is not meant to cover (see doc_summarize_ssrf_test.go for that).
func TestDocSummarizeSkill_ValidateURL_AllowlistExactOrSubdomainOnly(t *testing.T) {
	config := domain.ToolConfig{
		Params: map[string]interface{}{
			"allowed_domains": []interface{}{"github.com"},
		},
	}

	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name:      "exact allowlisted domain is accepted",
			url:       "http://github.com/user/repo",
			wantError: false,
		},
		{
			name:      "subdomain of allowlisted domain is accepted",
			url:       "http://sub.github.com/user/repo",
			wantError: false,
		},
		{
			name:      "domain containing allowlisted domain as substring prefix-of-suffix is rejected",
			url:       "http://notgithub.com.attacker.net/x",
			wantError: true,
		},
		{
			name:      "domain containing allowlisted domain as substring with attacker suffix is rejected",
			url:       "http://github.com.evil.example/x",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := NewDocSummarizeSkill(config, nil, nil)
			// Isolate the allowlist check from the IP-range SSRF layer: these
			// synthetic hosts don't resolve via real DNS, and this test is
			// specifically about the allowlist's own string-matching logic,
			// not the separate common.ResolveValidatedIP validation.
			skill.SetSSRFProtection(false)

			_, _, err := skill.validateURL(context.Background(), tt.url)

			if tt.wantError && err == nil {
				t.Errorf("validateURL(%q) with allowlist [github.com] = nil error, want rejection (substring match must not bypass exact-or-subdomain allowlist semantics)", tt.url)
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateURL(%q) with allowlist [github.com] = %v, want nil error", tt.url, err)
			}
		})
	}
}
