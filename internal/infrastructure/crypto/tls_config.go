package crypto

import (
	"crypto/tls"
	"fmt"

	"nuimanbot/internal/config"
)

// BuildTLSConfig constructs a *tls.Config from a TLSConfig using LoadOrGenerateCert
// to obtain the certificate.
func BuildTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
	cert, err := LoadOrGenerateCert(cfg.CertFile, cfg.KeyFile, cfg.Hosts)
	if err != nil {
		return nil, fmt.Errorf("crypto: build TLS config: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
