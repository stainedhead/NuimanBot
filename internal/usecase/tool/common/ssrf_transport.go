package common

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DialContextFunc matches the signature of net.Dialer.DialContext and
// http.Transport.DialContext, letting tests substitute the underlying
// network dial without a real routable address.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// FetchPolicy controls how an SSRF-safe *http.Client behaves on redirects.
// It is derived from config.FetchSecurityConfig by the DI layer (cmd/nuimanbot)
// so this usecase-layer package has no dependency on the config package.
type FetchPolicy struct {
	// SSRFProtection activates IP-resolution-based validation of every
	// redirect hop. When false, redirects are followed with Go's normal
	// (unvalidated) policy — NewCheckRedirect returns nil in that case.
	SSRFProtection bool
	// FollowRedirects controls whether redirects are followed at all, when
	// SSRFProtection is true. When false, the client does not follow
	// redirects: the 3xx response is returned to the caller as-is.
	FollowRedirects bool
}

type resolvedIPContextKey struct{}

// WithResolvedIP returns a context carrying ip as the address that
// pinnedDialContext must dial for any request made with this context,
// instead of re-resolving the request's hostname.
func WithResolvedIP(ctx context.Context, ip net.IP) context.Context {
	return context.WithValue(ctx, resolvedIPContextKey{}, ip)
}

// resolvedIPFromContext returns the IP pinned via WithResolvedIP, if any.
func resolvedIPFromContext(ctx context.Context) (net.IP, bool) {
	ip, ok := ctx.Value(resolvedIPContextKey{}).(net.IP)
	return ip, ok
}

// NewSSRFSafeTransport returns an *http.Transport whose DialContext dials the
// IP address pinned into the request's context (via WithResolvedIP —
// NewCheckRedirect does this for every validated redirect hop) rather than
// letting the dialer re-resolve the request's hostname itself. This closes
// the DNS-rebinding TOCTOU window: the exact address that ValidateFetchURL
// validated is the exact address that gets connected to, not a second,
// independent DNS lookup that could return something different. The original
// Host header and TLS ServerName (SNI) are left untouched — http.Transport
// derives those from the request URL, not from the dial address — so TLS
// verification and virtual-hosted HTTP still target the intended hostname.
//
// If the request's context carries no pinned IP (e.g. the initial,
// non-redirect request, which callers validate separately before ever
// constructing the request), the dial falls back to normal resolution via
// baseDial. A nil baseDial defaults to a plain net.Dialer.
func NewSSRFSafeTransport(baseDial DialContextFunc) *http.Transport {
	if baseDial == nil {
		d := &net.Dialer{Timeout: 30 * time.Second}
		baseDial = d.DialContext
	}
	return &http.Transport{
		DialContext: pinnedDialContext(baseDial),
	}
}

// pinnedDialContext wraps base so that, when the dial's context carries a
// resolved IP (see WithResolvedIP), the connection is made directly to that
// IP — preserving the requested port — instead of re-resolving addr's host.
func pinnedDialContext(base DialContextFunc) DialContextFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if ip, ok := resolvedIPFromContext(ctx); ok {
			if _, port, err := net.SplitHostPort(addr); err == nil {
				addr = net.JoinHostPort(ip.String(), port)
			}
		}
		return base(ctx, network, addr)
	}
}

// maxRedirects mirrors Go's default http.Client redirect-following limit.
const maxRedirects = 10

// NewCheckRedirect returns an http.Client.CheckRedirect function implementing
// policy:
//
//   - !policy.SSRFProtection: returns nil, restoring Go's normal (permissive,
//     unvalidated) redirect-following behavior.
//   - policy.SSRFProtection && !policy.FollowRedirects: refuses every
//     redirect via http.ErrUseLastResponse, so the 3xx response is returned
//     to the caller as-is rather than being validated and followed.
//   - policy.SSRFProtection && policy.FollowRedirects (the default): every
//     redirect hop's target is re-validated with ResolveValidatedIP, failing
//     closed (returning an error, which aborts the request) on any
//     disallowed target; the validated IP is pinned into the request's
//     context so NewSSRFSafeTransport's DialContext dials it directly.
func NewCheckRedirect(policy FetchPolicy, opts URLValidationOptions) func(req *http.Request, via []*http.Request) error {
	if !policy.SSRFProtection {
		return nil
	}
	if !policy.FollowRedirects {
		return func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("stopped after 10 redirects")
		}

		ip, err := ResolveValidatedIP(req.Context(), req.URL.String(), opts)
		if err != nil {
			return fmt.Errorf("redirect target blocked by SSRF protection: %w", err)
		}

		// req is the exact pointer the Client will use for the next round
		// trip; mutate it in place (rather than reassigning the local
		// variable) so the pinned context actually reaches the dial.
		*req = *req.WithContext(WithResolvedIP(req.Context(), ip))
		return nil
	}
}
