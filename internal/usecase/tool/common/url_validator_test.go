package common

import (
	"context"
	"errors"
	"net"
	"testing"
)

// fakeResolver implements IPResolver with a fixed, in-memory hostname -> IP
// table so tests never depend on real DNS/network access.
type fakeResolver struct {
	answers map[string][]net.IPAddr
	err     error
}

func (f *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if f.err != nil {
		return nil, f.err
	}
	addrs, ok := f.answers[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return addrs, nil
}

func ipAddrs(ips ...string) []net.IPAddr {
	out := make([]net.IPAddr, 0, len(ips))
	for _, s := range ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(s)})
	}
	return out
}

func TestValidateFetchURL_IPLiterals(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Loopback
		{"loopback IPv4", "http://127.0.0.1/", true},
		{"loopback IPv4 alt", "http://127.5.5.5/", true},
		{"loopback IPv6", "http://[::1]/", true},

		// RFC 1918 private
		{"private 10/8", "http://10.0.0.5/", true},
		{"private 172.16/12", "http://172.16.0.5/", true},
		{"private 172.31/12 boundary", "http://172.31.255.254/", true},
		{"private 192.168/16", "http://192.168.1.5/", true},

		// Link-local (including cloud metadata)
		{"link-local IPv4 (cloud metadata)", "http://169.254.169.254/", true},
		{"link-local IPv4 generic", "http://169.254.1.1/", true},
		{"link-local IPv6", "http://[fe80::1]/", true},

		// Multicast / reserved
		{"multicast IPv4", "http://224.0.0.1/", true},
		{"multicast IPv6", "http://[ff02::1]/", true},

		// Unspecified
		{"unspecified IPv4", "http://0.0.0.0/", true},
		{"unspecified IPv6", "http://[::]/", true},

		// Legitimate public addresses
		{"public IPv4", "http://8.8.8.8/", false},
		{"public IPv4 other", "http://93.184.216.34/", false},
		{"public IPv6", "http://[2001:4860:4860::8888]/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFetchURL(context.Background(), tt.url, URLValidationOptions{})
			if tt.wantErr && err == nil {
				t.Errorf("ValidateFetchURL(%q) = nil, want error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateFetchURL(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

func TestValidateFetchURL_SchemeRejected(t *testing.T) {
	for _, u := range []string{"ftp://8.8.8.8/", "file:///etc/passwd", "gopher://8.8.8.8/"} {
		if err := ValidateFetchURL(context.Background(), u, URLValidationOptions{}); err == nil {
			t.Errorf("ValidateFetchURL(%q) = nil, want scheme error", u)
		}
	}
}

func TestValidateFetchURL_InvalidURL(t *testing.T) {
	if err := ValidateFetchURL(context.Background(), "://not a url", URLValidationOptions{}); err == nil {
		t.Error("ValidateFetchURL() with malformed URL = nil, want error")
	}
}

func TestValidateFetchURL_HostnameResolution(t *testing.T) {
	resolver := &fakeResolver{
		answers: map[string][]net.IPAddr{
			"internal.example.test": ipAddrs("10.0.0.5"),
			"public.example.test":   ipAddrs("93.184.216.34"),
			"mixed.example.test":    ipAddrs("93.184.216.34", "10.0.0.5"), // any-bad -> reject
			"metadata.example.test": ipAddrs("169.254.169.254"),
		},
	}
	opts := URLValidationOptions{Resolver: resolver}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"private host", "http://internal.example.test/", true},
		{"public host", "http://public.example.test/", false},
		{"mixed resolution rejects if any address is disallowed", "http://mixed.example.test/", true},
		{"cloud metadata hostname", "http://metadata.example.test/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFetchURL(context.Background(), tt.url, opts)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateFetchURL(%q) = nil, want error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateFetchURL(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

func TestValidateFetchURL_DNSFailure(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("dns boom")}
	err := ValidateFetchURL(context.Background(), "http://anything.example.test/", URLValidationOptions{Resolver: resolver})
	if err == nil {
		t.Error("ValidateFetchURL() with resolver error = nil, want error")
	}
}

func TestResolveValidatedIP_ReturnsResolvedIP(t *testing.T) {
	resolver := &fakeResolver{
		answers: map[string][]net.IPAddr{
			"public.example.test": ipAddrs("93.184.216.34"),
		},
	}
	ip, err := ResolveValidatedIP(context.Background(), "http://public.example.test/", URLValidationOptions{Resolver: resolver})
	if err != nil {
		t.Fatalf("ResolveValidatedIP() error = %v", err)
	}
	if ip.String() != "93.184.216.34" {
		t.Errorf("ResolveValidatedIP() = %v, want 93.184.216.34", ip)
	}
}

func TestResolveValidatedIP_IPLiteral(t *testing.T) {
	ip, err := ResolveValidatedIP(context.Background(), "http://8.8.8.8/", URLValidationOptions{})
	if err != nil {
		t.Fatalf("ResolveValidatedIP() error = %v", err)
	}
	if ip.String() != "8.8.8.8" {
		t.Errorf("ResolveValidatedIP() = %v, want 8.8.8.8", ip)
	}
}
