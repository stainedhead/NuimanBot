package health

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/infrastructure/crypto"
)

// StartTLS starts the health check server over HTTPS on the specified address.
// It creates the TLS listener synchronously so that any bind or certificate
// error is returned immediately to the caller, rather than being swallowed
// inside a goroutine. The server then serves requests in a background goroutine.
// Shutdown semantics are identical to Start: call Stop() to gracefully shut down.
func (s *Server) StartTLS(addr string, cfg config.TLSConfig) error {
	tlsCfg, err := crypto.BuildTLSConfig(cfg)
	if err != nil {
		return fmt.Errorf("health: tls: build config: %w", err)
	}

	// Create the listener synchronously so bind errors surface to the caller immediately.
	ln, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("health: tls: listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		TLSConfig:    tlsCfg,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("Starting health check server (TLS)", "addr", addr)

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("health TLS server stopped unexpectedly", "error", err)
		}
	}()

	return nil
}
