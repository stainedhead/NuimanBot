package factory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/storage"
)

// healthProbeTimeout is the default timeout for the Ingatan startup health probe.
const healthProbeTimeout = 5 * time.Second

// BuildMemoryRepositoriesWithFallback builds memory repositories with optional graceful degradation.
//
// For the Ingatan backend:
//   - Performs a startup health probe (GET /api/v1/health).
//   - If the probe succeeds, returns Ingatan repositories.
//   - If the probe fails and fallback_to_builtin is true, logs a warning and returns built-in repositories.
//   - If the probe fails and fallback_to_builtin is false, returns an error.
//
// For all other backends, delegates to BuildMemoryRepositories without a health probe.
func BuildMemoryRepositoriesWithFallback(cfg *config.NuimanBotConfig) (memoryv2.MemoryCellRepository, memoryv2.MemorySceneRepository, error) {
	if cfg.Memory.Backend != config.MemoryBackendIngatan {
		return BuildMemoryRepositories(cfg)
	}

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

	// Startup health probe with configurable timeout.
	ctx, cancel := context.WithTimeout(context.Background(), healthProbeTimeout)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		if ingatanCfg.FallbackToBuiltin {
			slog.Warn("ingatan: health probe failed; falling back to built-in memory backend",
				"url", ingatanCfg.URL,
				"error", err,
			)
			return buildBuiltinRepositories(cfg)
		}
		return nil, nil, fmt.Errorf("ingatan: health probe failed and fallback_to_builtin is disabled: %w", err)
	}

	cellRepo := storage.NewIngatanMemoryCellRepository(client, prefix)
	sceneRepo := storage.NewIngatanMemorySceneRepository(client, prefix)
	return cellRepo, sceneRepo, nil
}
