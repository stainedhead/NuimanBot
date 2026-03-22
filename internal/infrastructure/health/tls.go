package health

import (
	"log/slog"
	"net/http"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/infrastructure/crypto"
)

// StartTLS starts the health check server over HTTPS on the specified address.
// It uses LoadOrGenerateCert to obtain a TLS certificate per the TLSConfig.
// Shutdown semantics are identical to Start: call Stop() to gracefully shut down.
func (s *Server) StartTLS(addr string, cfg config.TLSConfig) error {
	tlsCfg, err := crypto.BuildTLSConfig(cfg)
	if err != nil {
		return err
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
		if err := s.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("Health check TLS server error", "error", err)
		}
	}()

	return nil
}
