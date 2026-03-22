package factory_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nuimanbot/internal/adapter/factory"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/storage"
)

// newHealthyIngatanServer creates a test Ingatan server that responds to the health endpoint.
func newHealthyIngatanServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, _ *http.Request) {
		exp := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"token": "test-jwt", "expires_at": exp,
		}); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	})
	return srv
}

// unreachableURL is a URL that nothing is listening on.
const unreachableURL = "http://127.0.0.1:19998"

func TestBuildMemoryRepositoriesWithFallback_IngatanHealthy(t *testing.T) {
	srv := newHealthyIngatanServer(t)

	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendIngatan,
			Ingatan: config.IngatanConfig{
				URL:               srv.URL,
				APIKey:            domain.NewSecureStringFromString("test-key"),
				StorePrefix:       "nuiman",
				FallbackToBuiltin: true,
			},
		},
		Storage: config.StorageConfig{DSN: t.TempDir()},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositoriesWithFallback(cfg)
	if err != nil {
		t.Fatalf("Expected no error when Ingatan is healthy, got: %v", err)
	}
	if _, ok := cellRepo.(*storage.IngatanMemoryCellRepository); !ok {
		t.Errorf("Expected IngatanMemoryCellRepository, got %T", cellRepo)
	}
	if _, ok := sceneRepo.(*storage.IngatanMemorySceneRepository); !ok {
		t.Errorf("Expected IngatanMemorySceneRepository, got %T", sceneRepo)
	}
}

func TestBuildMemoryRepositoriesWithFallback_FallbackOnUnreachable(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendIngatan,
			Ingatan: config.IngatanConfig{
				URL:               unreachableURL,
				APIKey:            domain.NewSecureStringFromString("test-key"),
				StorePrefix:       "nuiman",
				FallbackToBuiltin: true,
			},
		},
		Storage: config.StorageConfig{DSN: t.TempDir()},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositoriesWithFallback(cfg)
	if err != nil {
		t.Fatalf("Expected no error with fallback enabled, got: %v", err)
	}
	// Should have fallen back to file-based.
	if _, ok := cellRepo.(*storage.FileMemoryCellRepository); !ok {
		t.Errorf("Expected FileMemoryCellRepository on fallback, got %T", cellRepo)
	}
	if _, ok := sceneRepo.(*storage.FileMemorySceneRepository); !ok {
		t.Errorf("Expected FileMemorySceneRepository on fallback, got %T", sceneRepo)
	}
}

func TestBuildMemoryRepositoriesWithFallback_ErrorOnUnreachableNoFallback(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendIngatan,
			Ingatan: config.IngatanConfig{
				URL:               unreachableURL,
				APIKey:            domain.NewSecureStringFromString("test-key"),
				StorePrefix:       "nuiman",
				FallbackToBuiltin: false,
			},
		},
		Storage: config.StorageConfig{DSN: t.TempDir()},
	}

	_, _, err := factory.BuildMemoryRepositoriesWithFallback(cfg)
	if err == nil {
		t.Fatal("Expected error when Ingatan unreachable and fallback disabled, got nil")
	}
	if !strings.Contains(err.Error(), "ingatan") {
		t.Errorf("Expected error to mention 'ingatan', got: %v", err)
	}
}

func TestBuildMemoryRepositoriesWithFallback_BuiltinPassThrough(t *testing.T) {
	cfg := &config.NuimanBotConfig{
		Memory: config.MemoryConfig{
			Backend: config.MemoryBackendBuiltin,
		},
		Storage: config.StorageConfig{DSN: t.TempDir()},
	}

	cellRepo, sceneRepo, err := factory.BuildMemoryRepositoriesWithFallback(cfg)
	if err != nil {
		t.Fatalf("Expected no error for builtin, got: %v", err)
	}
	if _, ok := cellRepo.(*storage.FileMemoryCellRepository); !ok {
		t.Errorf("Expected FileMemoryCellRepository for builtin, got %T", cellRepo)
	}
	if _, ok := sceneRepo.(*storage.FileMemorySceneRepository); !ok {
		t.Errorf("Expected FileMemorySceneRepository for builtin, got %T", sceneRepo)
	}
}
