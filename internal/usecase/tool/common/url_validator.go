package common

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// IPResolver resolves a hostname to its IP addresses. *net.Resolver satisfies
// this interface; tests inject a fake implementation to avoid depending on
// real DNS/network access.
type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// URLValidationOptions configures ValidateFetchURL/ResolveValidatedIP.
type URLValidationOptions struct {
	// Resolver performs hostname -> IP resolution. Nil defaults to
	// net.DefaultResolver.
	Resolver IPResolver
}

func (o URLValidationOptions) resolver() IPResolver {
	if o.Resolver != nil {
		return o.Resolver
	}
	return net.DefaultResolver
}

// ValidateFetchURL validates that rawURL is safe to fetch from a
// server-side-request-forgery (SSRF) perspective: the scheme must be
// http/https, and every IP address the host resolves to (or, for an IP
// literal, the address itself) must fall outside disallowed ranges —
// loopback, RFC 1918 private, link-local (which covers cloud metadata
// 169.254.169.254), multicast, and unspecified addresses. If the host
// resolves to multiple addresses, ANY disallowed address rejects the whole
// URL (fail closed).
func ValidateFetchURL(ctx context.Context, rawURL string, opts URLValidationOptions) error {
	_, err := resolveAndValidate(ctx, rawURL, opts)
	return err
}

// ResolveValidatedIP validates rawURL exactly like ValidateFetchURL and, on
// success, also returns one of the validated (allowed) IP addresses it
// resolved to. Callers that need to dial the exact address that was
// validated — closing the DNS-rebinding TOCTOU window where a second,
// independent DNS lookup at connect time could return a different, disallowed
// address — should use the returned IP to dial directly instead of letting
// the HTTP client re-resolve the hostname.
func ResolveValidatedIP(ctx context.Context, rawURL string, opts URLValidationOptions) (net.IP, error) {
	return resolveAndValidate(ctx, rawURL, opts)
}

// resolveAndValidate parses rawURL, checks its scheme, resolves its host to
// IP address(es), rejects the URL if any resolved address is disallowed, and
// returns the first validated address.
func resolveAndValidate(ctx context.Context, rawURL string, opts URLValidationOptions) (net.IP, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q: only http and https are allowed", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL has no host: %s", rawURL)
	}

	ips, err := resolveHost(ctx, host, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q did not resolve to any IP address", host)
	}

	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return nil, fmt.Errorf("target address %s (resolved from %q) is not allowed: loopback, private, link-local, multicast, and unspecified addresses are blocked", ip, host)
		}
	}

	return ips[0], nil
}

// resolveHost returns host's IP address(es). If host is already an IP
// literal, it is returned as-is without a DNS lookup.
func resolveHost(ctx context.Context, host string, opts URLValidationOptions) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	addrs, err := opts.resolver().LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

// isDisallowedIP reports whether ip falls in a range that must never be
// reachable via an agent-initiated fetch: loopback (127.0.0.0/8, ::1/128),
// RFC 1918 private ranges, link-local unicast (169.254.0.0/16 — including the
// cloud-metadata address 169.254.169.254 — and fe80::/10), multicast, and
// unspecified (0.0.0.0, ::) addresses.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}
