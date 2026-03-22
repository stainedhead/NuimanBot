//go:build integration

package integration_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nuimanbot/internal/infrastructure/crypto"
)

// TestLoadOrGenerateCertRoundTrip verifies that LoadOrGenerateCert generates a cert,
// and that an HTTPS server using that cert can be connected to using a client
// that trusts the generated certificate.
func TestLoadOrGenerateCertRoundTrip(t *testing.T) {
	dir := t.TempDir()
	paths := crypto.CertPaths{
		CertFile: filepath.Join(dir, "test.crt"),
		KeyFile:  filepath.Join(dir, "test.key"),
	}

	cert, err := crypto.LoadOrGenerateCert(paths, []string{"localhost", "127.0.0.1"})
	require.NoError(t, err, "LoadOrGenerateCert should succeed")

	// Start an httptest TLS server using the generated certificate.
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello tls")
	}))
	ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	ts.StartTLS()
	defer ts.Close()

	// Build a client that trusts the generated cert.
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}

	resp, err := client.Get(ts.URL)
	require.NoError(t, err, "HTTPS GET should succeed with trusted cert")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "hello tls")
}

// TestLoadOrGenerateCertReuse verifies that calling LoadOrGenerateCert twice with
// the same paths returns the same certificate (second call loads from files).
func TestLoadOrGenerateCertReuse(t *testing.T) {
	dir := t.TempDir()
	paths := crypto.CertPaths{
		CertFile: filepath.Join(dir, "test.crt"),
		KeyFile:  filepath.Join(dir, "test.key"),
	}

	// First call — generates files.
	cert1, err := crypto.LoadOrGenerateCert(paths, []string{"localhost"})
	require.NoError(t, err)

	// Record file mtime.
	info, err := os.Stat(paths.CertFile)
	require.NoError(t, err)
	mtime1 := info.ModTime()

	// Second call — should load from existing files.
	cert2, err := crypto.LoadOrGenerateCert(paths, []string{"localhost"})
	require.NoError(t, err)

	// File mtime should not change (no regeneration).
	info2, err := os.Stat(paths.CertFile)
	require.NoError(t, err)
	assert.Equal(t, mtime1, info2.ModTime(), "cert file should not be regenerated on second call")

	// Both certs should have the same serial number (same cert data).
	x509Cert1, err := x509.ParseCertificate(cert1.Certificate[0])
	require.NoError(t, err)
	x509Cert2, err := x509.ParseCertificate(cert2.Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, x509Cert1.SerialNumber, x509Cert2.SerialNumber, "both calls should return the same certificate")
}

// TestWebServerTLSStartup starts the web.Server with TLS on a free port (:0)
// and verifies that an HTTPS connection can be established.
func TestWebServerTLSStartup(t *testing.T) {
	dir := t.TempDir()
	certPaths := crypto.CertPaths{
		CertFile: filepath.Join(dir, "server.crt"),
		KeyFile:  filepath.Join(dir, "server.key"),
	}

	// Generate the cert first so we can build a trusting client.
	cert, err := crypto.LoadOrGenerateCert(certPaths, []string{"localhost", "127.0.0.1"})
	require.NoError(t, err)

	// Start a TLS listener on a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	addr := ln.Addr().String()

	// Serve HTTPS using the generated cert.
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	tlsLn := tls.NewListener(ln, tlsCfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(tlsLn)
	}()
	defer srv.Close()

	// Build a trusting client.
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				ServerName: "localhost",
			},
		},
	}

	resp, err := client.Get("https://" + addr + "/health")
	require.NoError(t, err, "HTTPS connection to web server should succeed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
