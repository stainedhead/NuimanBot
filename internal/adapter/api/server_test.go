package api_test

import (
	"strings"
	"testing"

	"nuimanbot/internal/adapter/api"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// buildMinimalRESTConfig returns a minimal ExternalAPIRestConfig that won't fail
// for reasons unrelated to the JWT secret length check.
func buildMinimalRESTConfig() config.ExternalAPIRestConfig {
	return config.ExternalAPIRestConfig{
		Enabled: true,
		Port:    0,
		APIKey:  domain.NewSecureStringFromString("test-api-key"),
	}
}

// TestNewServer_EmptyJWTSecret_ReturnsError ensures NewServer rejects an empty JWT secret.
func TestNewServer_EmptyJWTSecret_ReturnsError(t *testing.T) {
	cfg := buildMinimalRESTConfig()

	srv, err := api.NewServer(cfg, "")

	if err == nil {
		t.Fatal("expected an error for empty JWT secret, got nil")
	}
	if srv != nil {
		t.Error("expected nil *Server when error is returned, got non-nil")
	}
}

// TestNewServer_ShortJWTSecret_ReturnsError ensures NewServer rejects a JWT secret
// that is shorter than the minimum of 32 bytes.
func TestNewServer_ShortJWTSecret_ReturnsError(t *testing.T) {
	cfg := buildMinimalRESTConfig()
	shortSecret := strings.Repeat("x", 31) // one byte short

	srv, err := api.NewServer(cfg, shortSecret)

	if err == nil {
		t.Fatalf("expected an error for a 31-byte JWT secret, got nil")
	}
	if srv != nil {
		t.Error("expected nil *Server when error is returned, got non-nil")
	}
}

// TestNewServer_MinimumLengthJWTSecret_Succeeds ensures NewServer accepts a JWT secret
// that is exactly 32 bytes long.
func TestNewServer_MinimumLengthJWTSecret_Succeeds(t *testing.T) {
	cfg := buildMinimalRESTConfig()
	validSecret := strings.Repeat("x", 32) // exactly 32 bytes

	srv, err := api.NewServer(cfg, validSecret)

	if err != nil {
		t.Fatalf("expected no error for a 32-byte JWT secret, got: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil *Server, got nil")
	}
}
