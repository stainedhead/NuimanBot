package web

import (
	"log/slog"
	"net/http"

	"nuimanbot/internal/config"
	"nuimanbot/internal/infrastructure/crypto"
)

// StartTLS starts the web admin server over HTTPS using the provided TLSConfig.
// It enables Secure session cookies automatically when TLS is active.
// Shutdown semantics are identical to Start: call Stop() to gracefully shut down.
func (s *Server) StartTLS(cfg config.TLSConfig) error {
	tlsCfg, err := crypto.BuildTLSConfig(cfg)
	if err != nil {
		return err
	}

	// Enable Secure flag on session cookies when running under TLS.
	s.auth.setSecureCookies(true)

	s.httpServer.TLSConfig = tlsCfg

	slog.Info("Starting web admin server (TLS)", "addr", s.addr)

	go func() {
		if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			slog.Error("Web admin TLS server error", "error", err)
		}
	}()

	return nil
}
