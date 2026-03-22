package health

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// StartTLS starts the health check server with TLS on the specified address.
// It creates the TLS listener synchronously so that any bind or certificate
// error is returned immediately to the caller, rather than being swallowed
// inside a goroutine. The server then serves requests in a background goroutine.
//
// certFile and keyFile are paths to PEM-encoded TLS certificate and key files.
// Both may be empty strings only in test scenarios where the bind is expected
// to fail before TLS configuration is needed.
func (s *Server) StartTLS(addr, certFile, keyFile string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	tlsCfg, err := buildTLSConfig(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("health: tls: load cert: %w", err)
	}

	ln, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("health: tls: listen %s: %w", addr, err)
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Starting health check TLS server", "addr", addr)

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health TLS server stopped unexpectedly", "error", err)
		}
	}()

	return nil
}

// buildTLSConfig constructs a *tls.Config from cert/key file paths.
// Returns an error if files are provided but cannot be loaded.
// Returns a minimal default config when both paths are empty (useful for
// testing scenarios where the listener is expected to fail at bind time).
func buildTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" && keyFile == "" {
		// No files provided: return a minimal config.
		// The caller is responsible for ensuring this is only used
		// in contexts where TLS negotiation will not actually occur
		// (e.g., bind-failure tests).
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil //nolint:gosec // caller controls usage
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load key pair %q / %q: %w", certFile, keyFile, err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
