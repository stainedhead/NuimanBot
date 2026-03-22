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
// and that an HTTPS server using that cert can be connected to with a trusting client.
func TestLoadOrGenerateCertRoundTrip(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "test.crt")
	keyFile := filepath.Join(dir, "test.key")

	cert, err := crypto.LoadOrGenerateCert(certFile, keyFile, []string{"localhost", "127.0.0.1"})
	require.NoError(t, err, "LoadOrGenerateCert should succeed")

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello tls")
	}))
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	ts.StartTLS()
	defer ts.Close()

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}

	resp, err := client.Get(ts.URL)
	require.NoError(t, err, "HTTPS GET should succeed with trusted cert")
	defer resp.Body.Close() //nolint:errcheck

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "hello tls")
}

// TestLoadOrGenerateCertReuse verifies that calling LoadOrGenerateCert twice returns the same cert.
func TestLoadOrGenerateCertReuse(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "test.crt")
	keyFile := filepath.Join(dir, "test.key")

	cert1, err := crypto.LoadOrGenerateCert(certFile, keyFile, []string{"localhost"})
	require.NoError(t, err)

	info, err := os.Stat(certFile)
	require.NoError(t, err)
	mtime1 := info.ModTime()

	cert2, err := crypto.LoadOrGenerateCert(certFile, keyFile, []string{"localhost"})
	require.NoError(t, err)

	info2, err := os.Stat(certFile)
	require.NoError(t, err)
	assert.Equal(t, mtime1, info2.ModTime(), "cert file should not be regenerated on second call")

	x509Cert1, err := x509.ParseCertificate(cert1.Certificate[0])
	require.NoError(t, err)
	x509Cert2, err := x509.ParseCertificate(cert2.Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, x509Cert1.SerialNumber, x509Cert2.SerialNumber, "both calls should return the same certificate")
}

// TestWebServerTLSStartup starts an HTTPS server on a free port and verifies the connection.
func TestWebServerTLSStartup(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server.crt")
	keyFile := filepath.Join(dir, "server.key")

	cert, err := crypto.LoadOrGenerateCert(certFile, keyFile, []string{"localhost", "127.0.0.1"})
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck

	addr := ln.Addr().String()

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
	go func() { _ = srv.Serve(tlsLn) }()
	defer srv.Close() //nolint:errcheck

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "localhost"},
		},
	}

	resp, err := client.Get("https://" + addr + "/health")
	require.NoError(t, err, "HTTPS connection to server should succeed")
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
