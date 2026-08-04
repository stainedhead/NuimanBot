package doc_summarize

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nuimanbot/internal/domain"
	"nuimanbot/internal/usecase/tool/common"
)

// TestDocSummarizeSkill_Execute_LoopbackRejectedWithoutAllowlist confirms the
// SSRF check FR-020 adds is always-on, layered UNDER the (optional) domain
// allowlist: even with no allowlist configured at all (previously
// "unrestricted"), a loopback target is now rejected.
func TestDocSummarizeSkill_Execute_LoopbackRejectedWithoutAllowlist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("secret"))
	}))
	defer server.Close()

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, server.Client())

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": server.URL,
	})
	if err == nil {
		t.Fatal("expected loopback target to be rejected even with no domain allowlist configured")
	}
}

// TestDocSummarizeSkill_Execute_IPv6LoopbackRejected confirms IPv6 loopback,
// entirely unchecked before this phase (doc_summarize had no SSRF check at
// all beyond the optional allowlist), is now blocked.
func TestDocSummarizeSkill_Execute_IPv6LoopbackRejected(t *testing.T) {
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": "http://[::1]:8080/secret",
	})
	if err == nil {
		t.Error("expected error for ::1 (IPv6 loopback)")
	}
}

// TestDocSummarizeSkill_Execute_CloudMetadataRejected confirms the link-local
// cloud-metadata address is blocked.
func TestDocSummarizeSkill_Execute_CloudMetadataRejected(t *testing.T) {
	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": "http://169.254.169.254/latest/meta-data/",
	})
	if err == nil {
		t.Error("expected error for 169.254.169.254 (link-local cloud metadata)")
	}
}

// TestDocSummarizeSkill_Execute_AllowlistedButPrivateStillRejected confirms
// the SSRF check applies even when the target host IS present in the domain
// allowlist — the allowlist controls *which domains* may be fetched, not
// whether the domain's resolved address is safe to fetch from.
func TestDocSummarizeSkill_Execute_AllowlistedButPrivateStillRejected(t *testing.T) {
	config := domain.ToolConfig{
		Params: map[string]interface{}{
			"allowed_domains": []interface{}{"127.0.0.1"},
		},
	}
	skill := NewDocSummarizeSkill(config, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"source": "http://127.0.0.1:8080/secret",
	})
	if err == nil {
		t.Error("expected loopback target to be rejected even when explicitly allowlisted by domain")
	}
}

// TestDocSummarizeSkill_SetSSRFProtection_Disabled_SkipsValidation confirms
// the escape hatch wired from security.fetch.ssrf_protection=false disables
// the check.
func TestDocSummarizeSkill_SetSSRFProtection_Disabled_SkipsValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, server.Client())
	skill.SetSSRFProtection(false)

	if _, _, err := skill.validateURL(context.Background(), server.URL); err != nil {
		t.Errorf("validateURL() with SSRF protection disabled = %v, want nil", err)
	}
}

func stubDialForHost(pinnedHost, target string) common.DialContextFunc {
	var d net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, _, err := net.SplitHostPort(addr); err == nil && host == pinnedHost {
			return d.DialContext(ctx, network, target)
		}
		return d.DialContext(ctx, network, addr)
	}
}

func defaultFetchPolicyClient(baseDial common.DialContextFunc) *http.Client {
	return &http.Client{
		Timeout:       5 * time.Second,
		Transport:     common.NewSSRFSafeTransport(baseDial),
		CheckRedirect: common.NewCheckRedirect(common.FetchPolicy{SSRFProtection: true, FollowRedirects: true}, common.URLValidationOptions{}),
	}
}

// TestDocSummarizeSkill_FetchURL_RedirectToDisallowedTargetFailsClosed
// confirms fetchURL's http.Client re-validates every redirect hop and fails
// closed when the target is disallowed.
func TestDocSummarizeSkill_FetchURL_RedirectToDisallowedTargetFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/secret", http.StatusFound)
	}))
	defer server.Close()

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, defaultFetchPolicyClient(nil))

	_, err := skill.fetchURL(context.Background(), server.URL)
	if err == nil {
		t.Fatal("fetchURL() through malicious redirect = nil error, want failure")
	}
}

// TestDocSummarizeSkill_FetchURL_RedirectToAllowedTargetSucceeds confirms a
// redirect to an allowed target is followed normally and the final content
// is returned.
func TestDocSummarizeSkill_FetchURL_RedirectToAllowedTargetSucceeds(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("final doc content"))
	}))
	defer final.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://93.184.216.34/landing", http.StatusFound)
	}))
	defer redirector.Close()

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil,
		defaultFetchPolicyClient(stubDialForHost("93.184.216.34", final.Listener.Addr().String())))

	content, err := skill.fetchURL(context.Background(), redirector.URL)
	if err != nil {
		t.Fatalf("fetchURL() through allowed redirect error = %v", err)
	}
	if content != "final doc content" {
		t.Errorf("content = %q, want %q", content, "final doc content")
	}
}

// fakeHostResolver implements common.IPResolver with a fixed, in-memory
// hostname -> IP mapping, letting a test simulate the "validation-time" DNS
// answer for a hostname that has no real DNS record, without any real
// network access.
type fakeHostResolver struct {
	answers map[string][]net.IPAddr
}

func (f *fakeHostResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if addrs, ok := f.answers[host]; ok {
		return addrs, nil
	}
	return nil, fmt.Errorf("fakeHostResolver: no answer configured for %q", host)
}

// TestDocSummarizeSkill_InitialRequest_PinnedToValidatedIP_NotReResolvedAddress
// is the regression test for the FR-003 P0 finding: validateURL used to
// resolve and validate a target's IP via common.ValidateFetchURL and then
// discard it; fetchURL built a fresh http.Request with no pinned IP, so the
// real dialer performed an INDEPENDENT second DNS lookup at connect time. A
// DNS-rebinding attacker (TTL=0 authoritative answer) can return an allowed
// public IP for the validation lookup and a different, disallowed address
// for the connect-time lookup moments later.
//
// See the analogous test in summarize_ssrf_test.go for the full mechanism
// explanation: a fake resolver answers the validation lookup for
// "rebind.example.test" with a fixed, allowed public IP; a base dial stub
// routes connections purely by the dial addr's host, so a pinned dial
// reaches a "legit" fixture server while an unpinned (re-resolved) dial
// would reach a different "attacker" fixture server.
//
// Before the fix: validateURL/validateSource returned the untouched input
// context (the discarded IP), so fetchURL's request was never pinned and
// the dial reached the attacker server — this test failed, demonstrating
// the vulnerability. After the fix: validateURL pins the validated IP into
// the returned context, fetchURL uses that context, and the dial reaches
// the legit server.
func TestDocSummarizeSkill_InitialRequest_PinnedToValidatedIP_NotReResolvedAddress(t *testing.T) {
	const (
		rebindHost  = "rebind.example.test"
		validatedIP = "93.184.216.34" // allowed public IP the validator resolves to
	)

	legit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("legit content"))
	}))
	defer legit.Close()

	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("attacker content"))
	}))
	defer attacker.Close()

	var d net.Dialer
	baseDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		switch host {
		case validatedIP:
			// pinnedDialContext rewrote addr's host to the pinned, validated
			// IP: this is what the fixed code must reach.
			return d.DialContext(ctx, network, legit.Listener.Addr().String())
		case rebindHost:
			// addr still carries the original hostname: no pinned IP was
			// found in the context, so pinnedDialContext fell through to
			// base dial unchanged — in production, the real dialer would
			// independently re-resolve this hostname, potentially landing
			// on a completely different, attacker-controlled address.
			return d.DialContext(ctx, network, attacker.Listener.Addr().String())
		}
		return nil, fmt.Errorf("unexpected dial address %q", addr)
	}

	skill := NewDocSummarizeSkill(domain.ToolConfig{}, nil, &http.Client{
		Timeout:   5 * time.Second,
		Transport: common.NewSSRFSafeTransport(baseDial),
	})
	skill.urlValidationOpts = common.URLValidationOptions{
		Resolver: &fakeHostResolver{answers: map[string][]net.IPAddr{
			rebindHost: {{IP: net.ParseIP(validatedIP)}},
		}},
	}

	urlStr, pinnedCtx, err := skill.validateURL(context.Background(), "http://"+rebindHost+"/page")
	if err != nil {
		t.Fatalf("validateURL() error = %v", err)
	}

	content, err := skill.fetchURL(pinnedCtx, urlStr)
	if err != nil {
		t.Fatalf("fetchURL() error = %v", err)
	}

	if content != "legit content" {
		t.Errorf("fetchURL() content = %q, want %q — the initial request must dial the pinned, validated IP, not a re-resolved (possibly attacker-controlled) address", content, "legit content")
	}
}
