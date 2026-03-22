// Package factory provides constructors for swapping infrastructure implementations
// based on application configuration.
package factory

import (
	"fmt"
	"regexp"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

// validStorePrefixRE matches valid Ingatan store prefixes: 2–31 lowercase
// alphanumeric characters or hyphens, starting with a letter or digit.
// Compiled once at package level.
var validStorePrefixRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

// validateStorePrefix returns nil if prefix is empty (caller will use the default)
// or if it satisfies Ingatan store naming constraints. Returns a descriptive error
// otherwise.
func validateStorePrefix(prefix string) error {
	if prefix == "" {
		return nil // empty — factory will substitute default "nuiman"
	}
	if !validStorePrefixRE.MatchString(prefix) {
		return fmt.Errorf("ingatan: store_prefix %q is invalid: must be 2–31 lowercase alphanumeric characters or hyphens, starting with a letter or digit", prefix)
	}
	return nil
}

// BuildMemoryRepositories selects and constructs memory repositories based on cfg.Memory.Backend.
//
// Supported backends:
//   - "builtin" or "" — file-based repositories in cfg.Storage.DSN
//   - "ingatan"       — Ingatan REST API repositories; returns error if URL is missing
//
// The health probe and graceful degradation logic is applied separately via
// BuildMemoryRepositoriesWithFallback for startup use.
func BuildMemoryRepositories(cfg *config.NuimanBotConfig) (memoryv2.MemoryCellRepository, memoryv2.MemorySceneRepository, error) {
	switch cfg.Memory.Backend {
	case config.MemoryBackendIngatan:
		return buildIngatanRepositories(cfg)
	default:
		// "builtin", "", or unknown backend — use file-based storage.
		return buildBuiltinRepositories(cfg)
	}
}

// buildBuiltinRepositories creates file-based memory repositories.
func buildBuiltinRepositories(cfg *config.NuimanBotConfig) (memoryv2.MemoryCellRepository, memoryv2.MemorySceneRepository, error) {
	basePath := cfg.Storage.DSN
	if basePath == "" {
		basePath = "./data"
	}
	cellRepo := storage.NewFileMemoryCellRepository(basePath)
	sceneRepo := storage.NewFileMemorySceneRepository(basePath)
	return cellRepo, sceneRepo, nil
}

// buildIngatanRepositories creates Ingatan-backed memory repositories.
// Returns an error if the Ingatan URL is not configured or if the store_prefix
// does not satisfy Ingatan naming constraints.
func buildIngatanRepositories(cfg *config.NuimanBotConfig) (memoryv2.MemoryCellRepository, memoryv2.MemorySceneRepository, error) {
	ingatanCfg := cfg.Memory.Ingatan
	if ingatanCfg.URL == "" {
		return nil, nil, fmt.Errorf("ingatan backend selected but memory.ingatan.url is not configured")
	}

	if err := validateStorePrefix(ingatanCfg.StorePrefix); err != nil {
		return nil, nil, err
	}

	prefix := ingatanCfg.StorePrefix
	if prefix == "" {
		prefix = "nuiman"
	}

	client := storage.NewIngatanHTTPClient(storage.IngatanClientConfig{
		BaseURL:       ingatanCfg.URL,
		APIKey:        ingatanCfg.APIKey.Value(),
		StorePrefix:   prefix,
		TLSSkipVerify: ingatanCfg.TLSSkipVerify,
	})

	cellRepo := storage.NewIngatanMemoryCellRepository(client, prefix)
	sceneRepo := storage.NewIngatanMemorySceneRepository(client, prefix)
	return cellRepo, sceneRepo, nil
}
