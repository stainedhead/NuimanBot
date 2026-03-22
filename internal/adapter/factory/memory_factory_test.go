package factory_test

import (
	"testing"

	"nuimanbot/internal/adapter/factory"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

func TestBuildMemoryRepositories_Builtin(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendBuiltin,
		},
		Storage: config.StorageConfig{
			DSN: t.TempDir(),
		},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositories(cfg)
	if err != nil {
		t.Fatalf("Expected no error for builtin backend, got: %v", err)
	}
	if cellRepo == nil {
		t.Error("Expected non-nil cell repository for builtin backend")
	}
	if sceneRepo == nil {
		t.Error("Expected non-nil scene repository for builtin backend")
	}

	// Verify it's a file-based repository.
	if _, ok := cellRepo.(*storage.FileMemoryCellRepository); !ok {
		t.Errorf("Expected *storage.FileMemoryCellRepository for builtin, got %T", cellRepo)
	}
}

func TestBuildMemoryRepositories_EmptyBackend_DefaultsToBuiltin(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: "", // empty — should default to builtin
		},
		Storage: config.StorageConfig{
			DSN: t.TempDir(),
		},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositories(cfg)
	if err != nil {
		t.Fatalf("Expected no error for empty backend, got: %v", err)
	}
	if cellRepo == nil || sceneRepo == nil {
		t.Error("Expected non-nil repositories for empty backend")
	}
}

func TestBuildMemoryRepositories_Ingatan_MissingURL(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendIngatan,
			Ingatan: config.IngatanConfig{
				URL:    "", // missing — must return error
				APIKey: domain.NewSecureStringFromString("some-key"),
			},
		},
	}

	_, _, err := factory.BuildMemoryRepositories(cfg)
	if err == nil {
		t.Fatal("Expected error when Ingatan URL is empty, got nil")
	}
}

func TestBuildMemoryRepositories_Ingatan_ValidConfig(t *testing.T) {
	// We can't connect to a real Ingatan in unit tests, but we can verify
	// that the factory returns Ingatan repositories (not file-based) when
	// configured. The health probe / graceful degradation is tested in Task 1.6.
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendIngatan,
			Ingatan: config.IngatanConfig{
				URL:               "http://127.0.0.1:19999", // unreachable — that's OK for factory type check
				APIKey:            domain.NewSecureStringFromString("test-key"),
				StorePrefix:       "nuiman",
				FallbackToBuiltin: false,
			},
		},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositories(cfg)
	if err != nil {
		t.Fatalf("Expected no error from factory for valid Ingatan config (no health probe), got: %v", err)
	}
	if cellRepo == nil {
		t.Error("Expected non-nil cell repository for Ingatan backend")
	}
	if sceneRepo == nil {
		t.Error("Expected non-nil scene repository for Ingatan backend")
	}

	// Verify it's an Ingatan repository.
	if _, ok := cellRepo.(*storage.IngatanMemoryCellRepository); !ok {
		t.Errorf("Expected *storage.IngatanMemoryCellRepository for ingatan, got %T", cellRepo)
	}
	if _, ok := sceneRepo.(*storage.IngatanMemorySceneRepository); !ok {
		t.Errorf("Expected *storage.IngatanMemorySceneRepository for ingatan, got %T", sceneRepo)
	}
}
