package common

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubDial returns a DialContextFunc that ignores the requested network
// address and always connects to target, simulating "the pinned validated IP
// happens to be where our test fixture server actually listens" without
// requiring real DNS/network access or a routable non-loopback address.
func stubDial(target string) DialContextFunc {
	var d net.Dialer
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		return d.DialContext(ctx, network, target)
	}
}

// stubDialForHost returns a DialContextFunc that redirects only connections
// whose host portion matches pinnedHost to target, dialing every other
// address normally. This lets a test simulate "the validated public IP
// pinned by CheckRedirect happens to be where our local fixture server
// listens" for the redirect hop specifically, while the initial (real
// loopback) request still reaches its own real local server unaffected.
func stubDialForHost(pinnedHost, target string) DialContextFunc {
	var d net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, _, err := net.SplitHostPort(addr); err == nil && host == pinnedHost {
			return d.DialContext(ctx, network, target)
		}
		return d.DialContext(ctx, network, addr)
	}
}

func TestNewCheckRedirect_SSRFProtectionDisabled_ReturnsNil(t *testing.T) {
	fn := NewCheckRedirect(FetchPolicy{SSRFProtection: false, FollowRedirects: true}, URLValidationOptions{})
	if fn != nil {
		t.Error("NewCheckRedirect() with SSRFProtection=false = non-nil, want nil (default Go redirect policy)")
	}
}

func TestNewCheckRedirect_RedirectsDisabled_ReturnsErrUseLastResponse(t *testing.T) {
	fn := NewCheckRedirect(FetchPolicy{SSRFProtection: true, FollowRedirects: false}, URLValidationOptions{})
	if fn == nil {
		t.Fatal("NewCheckRedirect() with FollowRedirects=false = nil, want a func")
	}
	req, _ := http.NewRequest(http.MethodGet, "http://8.8.8.8/", nil)
	if err := fn(req, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Errorf("CheckRedirect() = %v, want http.ErrUseLastResponse", err)
	}
}

func TestNewCheckRedirect_ValidatesAndPinsIP(t *testing.T) {
	resolver := &fakeResolver{
		answers: map[string][]net.IPAddr{
			"public.example.test": ipAddrs("93.184.216.34"),
		},
	}
	fn := NewCheckRedirect(FetchPolicy{SSRFProtection: true, FollowRedirects: true}, URLValidationOptions{Resolver: resolver})

	t.Run("allowed target pins resolved IP into context", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://public.example.test/", nil)
		if err := fn(req, nil); err != nil {
			t.Fatalf("CheckRedirect() error = %v", err)
		}
		ip, ok := resolvedIPFromContext(req.Context())
		if !ok {
			t.Fatal("request context has no pinned resolved IP after CheckRedirect")
		}
		if ip.String() != "93.184.216.34" {
			t.Errorf("pinned IP = %v, want 93.184.216.34", ip)
		}
	})

	t.Run("disallowed target fails closed", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/", nil)
		if err := fn(req, nil); err == nil {
			t.Error("CheckRedirect() to cloud-metadata target = nil, want error")
		}
	})

	t.Run("too many redirects", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://public.example.test/", nil)
		via := make([]*http.Request, 10)
		if err := fn(req, via); err == nil {
			t.Error("CheckRedirect() with 10 prior hops = nil, want error")
		}
	})
}

func TestPinnedDialContext_UsesResolvedIPFromContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	transport := NewSSRFSafeTransport(stubDial(server.Listener.Addr().String()))

	ctx := WithResolvedIP(context.Background(), net.ParseIP("93.184.216.34"))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://93.184.216.34/", nil)

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", body, "hello")
	}
}

// TestFullRedirectChain_DisallowedTargetFailsClosed exercises the whole
// http.Client (Transport + CheckRedirect) against a real redirect chain: the
// first hop is a normal (loopback) test server, which redirects to a target
// that ValidateFetchURL must reject. The tool call must fail closed.
func TestFullRedirectChain_DisallowedTargetFailsClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to a disallowed cloud-metadata-style target.
		http.Redirect(w, r, "http://169.254.169.254/secret", http.StatusFound)
	}))
	defer server.Close()

	client := &http.Client{
		Timeout:       5 * time.Second,
		Transport:     NewSSRFSafeTransport(nil),
		CheckRedirect: NewCheckRedirect(FetchPolicy{SSRFProtection: true, FollowRedirects: true}, URLValidationOptions{}),
	}

	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("client.Get() through malicious redirect = nil error, want failure")
	}
}

// TestFullRedirectChain_AllowedTargetSucceeds exercises the whole client
// against a redirect chain where the redirect target validates as allowed
// (a public IP literal, so no real DNS lookup is needed) and the actual
// dial is pinned to that exact validated IP. A stub base dial function
// redirects the pinned-IP connection to the real local fixture server that
// represents "where 93.184.216.34 actually is" for this test, without
// requiring real internet access.
func TestFullRedirectChain_AllowedTargetSucceeds(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("final content"))
	}))
	defer final.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://93.184.216.34/landing", http.StatusFound)
	}))
	defer redirector.Close()

	client := &http.Client{
		Timeout:       5 * time.Second,
		Transport:     NewSSRFSafeTransport(stubDialForHost("93.184.216.34", final.Listener.Addr().String())),
		CheckRedirect: NewCheckRedirect(FetchPolicy{SSRFProtection: true, FollowRedirects: true}, URLValidationOptions{}),
	}

	resp, err := client.Get(redirector.URL)
	if err != nil {
		t.Fatalf("client.Get() through allowed redirect error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "final content" {
		t.Errorf("body = %q, want %q", body, "final content")
	}
}
