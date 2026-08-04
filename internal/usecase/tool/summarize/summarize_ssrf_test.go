package summarize

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

// TestSummarizeSkill_Execute_IPv6LoopbackRejected confirms the previous
// substring-only validateURL bypass (::1 never matched "127.0.0.1" or
// "localhost" as a substring) is now blocked by IP-resolution-based
// validation.
func TestSummarizeSkill_Execute_IPv6LoopbackRejected(t *testing.T) {
	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "http://[::1]:8080/secret",
	})
	if err == nil {
		t.Error("expected error for ::1 (IPv6 loopback)")
	}
}

// TestSummarizeSkill_Execute_CloudMetadataRejected confirms the link-local
// cloud-metadata address is blocked; the old substring check never covered
// this range at all.
func TestSummarizeSkill_Execute_CloudMetadataRejected(t *testing.T) {
	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil)

	_, err := skill.Execute(context.Background(), map[string]any{
		"url": "http://169.254.169.254/latest/meta-data/",
	})
	if err == nil {
		t.Error("expected error for 169.254.169.254 (link-local cloud metadata)")
	}
}

// TestSummarizeSkill_Execute_PrivateRFC1918Rejected confirms RFC 1918 ranges
// beyond the old hardcoded substrings (10.0.0.0/8, 172.16.0.0/12) are now
// blocked.
func TestSummarizeSkill_Execute_PrivateRFC1918Rejected(t *testing.T) {
	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, nil)

	for _, u := range []string{"http://10.1.2.3/", "http://172.16.5.5/", "http://192.168.0.1/"} {
		_, err := skill.Execute(context.Background(), map[string]any{"url": u})
		if err == nil {
			t.Errorf("expected error for private URL %s", u)
		}
	}
}

// TestSummarizeSkill_SetSSRFProtection_Disabled_SkipsValidation confirms the
// escape hatch wired from security.fetch.ssrf_protection=false actually
// disables the initial-request check.
func TestSummarizeSkill_SetSSRFProtection_Disabled_SkipsValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, server.Client())
	skill.SetSSRFProtection(false)

	if _, _, err := skill.validateURL(context.Background(), map[string]any{"url": server.URL}); err != nil {
		t.Errorf("validateURL() with SSRF protection disabled = %v, want nil", err)
	}
}

// TestSummarizeSkill_SetSSRFProtection_Enabled_BlocksLoopback confirms the
// default (protection enabled) path still blocks loopback even when a
// caller-supplied httpClient is used.
func TestSummarizeSkill_SetSSRFProtection_Enabled_BlocksLoopback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, server.Client())

	if _, _, err := skill.validateURL(context.Background(), map[string]any{"url": server.URL}); err == nil {
		t.Error("validateURL() with SSRF protection enabled (default) = nil, want error for loopback target")
	}
}

// stubDialForHost redirects only connections whose host portion matches
// pinnedHost to target, dialing everything else normally. It lets a test
// simulate "the validated public IP pinned by CheckRedirect happens to be
// where our local fixture server listens" without real internet access.
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

// TestSummarizeSkill_FetchWebPage_RedirectToDisallowedTargetFailsClosed
// confirms fetchWebPage's http.Client re-validates every redirect hop and
// fails closed when the target is disallowed.
func TestSummarizeSkill_FetchWebPage_RedirectToDisallowedTargetFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/secret", http.StatusFound)
	}))
	defer server.Close()

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, defaultFetchPolicyClient(nil))

	_, err := skill.fetchWebPage(context.Background(), server.URL)
	if err == nil {
		t.Fatal("fetchWebPage() through malicious redirect = nil error, want failure")
	}
}

// TestSummarizeSkill_FetchWebPage_RedirectToAllowedTargetSucceeds confirms a
// redirect to an allowed target is followed normally and the final content
// is returned.
func TestSummarizeSkill_FetchWebPage_RedirectToAllowedTargetSucceeds(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("final page content"))
	}))
	defer final.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://93.184.216.34/landing", http.StatusFound)
	}))
	defer redirector.Close()

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil,
		defaultFetchPolicyClient(stubDialForHost("93.184.216.34", final.Listener.Addr().String())))

	content, err := skill.fetchWebPage(context.Background(), redirector.URL)
	if err != nil {
		t.Fatalf("fetchWebPage() through allowed redirect error = %v", err)
	}
	if content != "final page content" {
		t.Errorf("content = %q, want %q", content, "final page content")
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

// TestSummarizeSkill_InitialRequest_PinnedToValidatedIP_NotReResolvedAddress
// is the regression test for the FR-003 P0 finding: validateURL used to
// resolve and validate a target's IP via common.ValidateFetchURL and then
// discard it; fetchWebPage built a fresh http.Request with no pinned IP, so
// the real dialer performed an INDEPENDENT second DNS lookup at connect
// time. A DNS-rebinding attacker (TTL=0 authoritative answer) can return an
// allowed public IP for the validation lookup and a different, disallowed
// address for the connect-time lookup moments later.
//
// This test uses a fake resolver that answers the validation lookup for
// "rebind.example.test" with a fixed, allowed public IP, and a base dial
// stub that routes connections purely by the dial addr's host: a connection
// whose addr was rewritten to that validated IP (i.e., pinnedDialContext
// found a pinned IP in the request's context) reaches a "legit" fixture
// server; a connection that still carries the original hostname (i.e., no
// pin was found, so pinnedDialContext fell through to the base dialer
// unchanged — in production this is exactly where a second, independent,
// attacker-controlled DNS lookup would happen) reaches a completely
// different "attacker" fixture server instead.
//
// Before the fix: validateURL returned the untouched input context (the
// discarded IP), so fetchWebPage's request was never pinned and the dial
// reached the attacker server — this test failed, demonstrating the
// vulnerability. After the fix: validateURL pins the validated IP into the
// returned context, fetchWebPage uses that context, and the dial reaches
// the legit server.
func TestSummarizeSkill_InitialRequest_PinnedToValidatedIP_NotReResolvedAddress(t *testing.T) {
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

	skill := NewSummarizeSkill(domain.ToolConfig{}, nil, nil, &http.Client{
		Timeout:   5 * time.Second,
		Transport: common.NewSSRFSafeTransport(baseDial),
	})
	skill.urlValidationOpts = common.URLValidationOptions{
		Resolver: &fakeHostResolver{answers: map[string][]net.IPAddr{
			rebindHost: {{IP: net.ParseIP(validatedIP)}},
		}},
	}

	urlStr, pinnedCtx, err := skill.validateURL(context.Background(), map[string]any{
		"url": "http://" + rebindHost + "/page",
	})
	if err != nil {
		t.Fatalf("validateURL() error = %v", err)
	}

	content, err := skill.fetchWebPage(pinnedCtx, urlStr)
	if err != nil {
		t.Fatalf("fetchWebPage() error = %v", err)
	}

	if content != "legit content" {
		t.Errorf("fetchWebPage() content = %q, want %q — the initial request must dial the pinned, validated IP, not a re-resolved (possibly attacker-controlled) address", content, "legit content")
	}
}
