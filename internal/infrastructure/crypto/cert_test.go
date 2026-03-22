package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadOrGenerateCert_CreatesFilesWhenAbsent verifies that cert and key files are
// created at the specified paths when they do not yet exist.
func TestLoadOrGenerateCert_CreatesFilesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	_, err := LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("LoadOrGenerateCert returned unexpected error: %v", err)
	}

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Errorf("cert file not created at %s", certPath)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Errorf("key file not created at %s", keyPath)
	}
}

// TestLoadOrGenerateCert_ReusesExistingFiles verifies that a second call with the same
// paths does not regenerate the cert (mtime must remain unchanged).
func TestLoadOrGenerateCert_ReusesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	_, err := LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("first call returned error: %v", err)
	}

	info1, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("stat after first call: %v", err)
	}
	mtime1 := info1.ModTime()

	// Small sleep to guarantee mtime difference if file is regenerated.
	time.Sleep(10 * time.Millisecond)

	_, err = LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("second call returned error: %v", err)
	}

	info2, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("stat after second call: %v", err)
	}

	if !info2.ModTime().Equal(mtime1) {
		t.Errorf("cert file was regenerated on second call: mtime changed from %v to %v", mtime1, info2.ModTime())
	}
}

// TestLoadOrGenerateCert_CertValidForLocalhost verifies the generated certificate
// passes TLS verification for localhost by dialing a local test server.
func TestLoadOrGenerateCert_CertValidForLocalhost(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	tlsCert, err := LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("LoadOrGenerateCert error: %v", err)
	}

	// Build a TLS server using the generated cert.
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsCfg)
	if err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	defer listener.Close()

	// Parse the cert for use as a root CA pool (self-signed).
	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(x509Cert)

	// Serve a minimal HTTP handler over TLS.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(listener) //nolint:errcheck
	defer srv.Close()

	addr := listener.Addr().String()

	// Connect with our cert pool so the self-signed cert is trusted.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				ServerName: "localhost",
			},
		},
	}

	resp, err := client.Get("https://" + addr + "/")
	if err != nil {
		t.Fatalf("TLS connection failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestLoadOrGenerateCert_FilePermissions verifies that the key file is written with
// mode 0600 and the cert file with mode 0644.
func TestLoadOrGenerateCert_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	_, err := LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("LoadOrGenerateCert error: %v", err)
	}

	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("stat cert file: %v", err)
	}
	if certInfo.Mode().Perm() != 0o644 {
		t.Errorf("cert file permissions: expected 0644, got %o", certInfo.Mode().Perm())
	}

	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if keyInfo.Mode().Perm() != 0o600 {
		t.Errorf("key file permissions: expected 0600, got %o", keyInfo.Mode().Perm())
	}
}

// TestLoadOrGenerateCert_HostsInDNSNames verifies that the provided hosts appear in
// the generated certificate's DNSNames.
func TestLoadOrGenerateCert_HostsInDNSNames(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	hosts := []string{"localhost", "myservice.local"}
	tlsCert, err := LoadOrGenerateCert(certPath, keyPath, hosts)
	if err != nil {
		t.Fatalf("LoadOrGenerateCert error: %v", err)
	}

	x509Cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse generated certificate: %v", err)
	}

	dnsSet := make(map[string]bool, len(x509Cert.DNSNames))
	for _, name := range x509Cert.DNSNames {
		dnsSet[name] = true
	}

	for _, host := range hosts {
		// Skip IP addresses — they go in IPAddresses, not DNSNames.
		if ip := net.ParseIP(host); ip != nil {
			continue
		}
		if !dnsSet[host] {
			t.Errorf("host %q not found in certificate DNSNames: %v", host, x509Cert.DNSNames)
		}
	}
}

// TestLoadOrGenerateCert_ErrorOnUnwritablePath verifies that an error is returned
// when the cert path cannot be written due to a permission-restricted directory.
func TestLoadOrGenerateCert_ErrorOnUnwritablePath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	// Create a read-only parent directory to force a write error.
	roDir := t.TempDir()
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("failed to make dir read-only: %v", err)
	}
	t.Cleanup(func() {
		// Restore write permission so TempDir cleanup succeeds.
		os.Chmod(roDir, 0o755) //nolint:errcheck
	})

	certPath := filepath.Join(roDir, "server.crt")
	keyPath := filepath.Join(roDir, "server.key")

	_, err := LoadOrGenerateCert(certPath, keyPath, []string{"localhost"})
	if err == nil {
		t.Error("expected error when writing to read-only directory, got nil")
	}
}
