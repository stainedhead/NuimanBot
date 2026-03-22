// Package factory provides constructors for swapping infrastructure implementations
// based on application configuration.
package factory

import (
	"fmt"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

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
// Returns an error if the Ingatan URL is not configured.
func buildIngatanRepositories(cfg *config.NuimanBotConfig) (memoryv2.MemoryCellRepository, memoryv2.MemorySceneRepository, error) {
	ingatanCfg := cfg.Memory.Ingatan
	if ingatanCfg.URL == "" {
		return nil, nil, fmt.Errorf("ingatan backend selected but memory.ingatan.url is not configured")
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
