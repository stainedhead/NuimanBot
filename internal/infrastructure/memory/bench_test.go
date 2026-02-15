package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nuimanbot/internal/domain/memoryv2"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// setupBenchDB creates a database and populates it with test data
func setupBenchDB(b *testing.B, cellCount int) (*sql.DB, *SQLiteMemoryCellRepository, *SQLiteMemorySceneRepository) {
	b.Helper()

	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}

	// Apply migration
	migrationSQL, err := os.ReadFile("migrations/001_memory_tables.sql")
	if err != nil {
		b.Fatalf("Failed to read migration: %v", err)
	}

	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		b.Fatalf("Failed to apply migration: %v", err)
	}

	cellRepo := NewSQLiteMemoryCellRepository(db)
	sceneRepo := NewSQLiteMemorySceneRepository(db)
	ctx := context.Background()

	// Create scenes
	scenes := []string{
		"authentication",
		"database-setup",
		"api-development",
		"testing",
		"deployment",
		"monitoring",
		"user-management",
		"configuration",
	}

	for _, sceneName := range scenes {
		scene := &memoryv2.MemoryScene{
			Scene:      sceneName,
			Summary:    fmt.Sprintf("Summary for %s scene", sceneName),
			TokenCount: 150,
			UpdatedAt:  time.Now(),
		}
		err := sceneRepo.Upsert(ctx, scene)
		if err != nil {
			b.Fatalf("Failed to create scene: %v", err)
		}
	}

	// Create cells
	contents := []string{
		"User configured OAuth authentication with Google provider",
		"Database schema uses PostgreSQL with JSON columns",
		"API implements REST endpoints with JWT validation",
		"Unit tests cover 95% of codebase with edge cases",
		"Deployment uses Docker containers on Kubernetes cluster",
		"Monitoring alerts configured for error rates and latency",
		"User roles include admin, editor, and viewer permissions",
		"Configuration loaded from environment variables and files",
	}

	cellTypes := []memoryv2.CellType{
		memoryv2.CellTypeFact,
		memoryv2.CellTypeDecision,
		memoryv2.CellTypeTask,
		memoryv2.CellTypePreference,
		memoryv2.CellTypePlan,
		memoryv2.CellTypeRisk,
	}

	for i := 0; i < cellCount; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: fmt.Sprintf("conv-%d", i%10),
			Scene:          scenes[i%len(scenes)],
			CellType:       cellTypes[i%len(cellTypes)],
			Salience:       0.5 + float64(i%50)/100.0, // Range 0.5-0.99
			Content:        contents[i%len(contents)],
			Source:         fmt.Sprintf(`["msg-%d"]`, i),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := cellRepo.Create(ctx, cell)
		if err != nil {
			b.Fatalf("Failed to create cell %d: %v", i, err)
		}
	}

	return db, cellRepo, sceneRepo
}

// BenchmarkCellRepository_Create benchmarks cell creation
func BenchmarkCellRepository_Create(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 0) // Start with empty DB
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "bench-conv",
			Scene:          "bench-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			Content:        fmt.Sprintf("Benchmark cell %d", i),
			Source:         fmt.Sprintf(`["msg-%d"]`, i),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := cellRepo.Create(ctx, cell)
		if err != nil {
			b.Fatalf("Failed to create cell: %v", err)
		}
	}
}

// BenchmarkCellRepository_Get benchmarks cell retrieval by ID
func BenchmarkCellRepository_Get(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 1000)
	defer db.Close()

	ctx := context.Background()

	// Get a cell ID to use for benchmarking
	filter := memoryv2.MemoryCellFilter{Limit: 1}
	cells, err := cellRepo.List(ctx, filter)
	if err != nil || len(cells) == 0 {
		b.Fatal("Failed to get test cell")
	}
	testCellID := cells[0].ID

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cellRepo.Get(ctx, testCellID)
		if err != nil {
			b.Fatalf("Failed to get cell: %v", err)
		}
	}
}

// BenchmarkCellRepository_List benchmarks listing cells with filters
func BenchmarkCellRepository_List(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 1000)
	defer db.Close()

	ctx := context.Background()
	filter := memoryv2.MemoryCellFilter{
		ConversationID: "conv-1",
		Limit:          10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cellRepo.List(ctx, filter)
		if err != nil {
			b.Fatalf("Failed to list cells: %v", err)
		}
	}
}

// BenchmarkCellRepository_SearchFTS benchmarks full-text search
func BenchmarkCellRepository_SearchFTS(b *testing.B) {
	sizes := []int{100, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			db, cellRepo, _ := setupBenchDB(b, size)
			defer db.Close()

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := cellRepo.SearchFTS(ctx, "authentication", 10)
				if err != nil {
					b.Fatalf("Failed to search: %v", err)
				}
			}
		})
	}
}

// BenchmarkCellRepository_GetByScene benchmarks scene-based retrieval
func BenchmarkCellRepository_GetByScene(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 1000)
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cellRepo.GetByScene(ctx, "authentication", 10)
		if err != nil {
			b.Fatalf("Failed to get cells by scene: %v", err)
		}
	}
}

// BenchmarkCellRepository_GetHighSalience benchmarks salience-based retrieval
func BenchmarkCellRepository_GetHighSalience(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 1000)
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cellRepo.GetHighSalience(ctx, "conv-1", 0.8, 10)
		if err != nil {
			b.Fatalf("Failed to get high salience cells: %v", err)
		}
	}
}

// BenchmarkSceneRepository_Upsert benchmarks scene upsert
func BenchmarkSceneRepository_Upsert(b *testing.B) {
	db, _, sceneRepo := setupBenchDB(b, 0)
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scene := &memoryv2.MemoryScene{
			Scene:      fmt.Sprintf("bench-scene-%d", i%10),
			Summary:    fmt.Sprintf("Benchmark scene %d", i),
			TokenCount: 150,
			UpdatedAt:  time.Now(),
		}

		err := sceneRepo.Upsert(ctx, scene)
		if err != nil {
			b.Fatalf("Failed to upsert scene: %v", err)
		}
	}
}

// BenchmarkSceneRepository_Get benchmarks scene retrieval
func BenchmarkSceneRepository_Get(b *testing.B) {
	db, _, sceneRepo := setupBenchDB(b, 100)
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sceneRepo.Get(ctx, "authentication")
		if err != nil {
			b.Fatalf("Failed to get scene: %v", err)
		}
	}
}

// BenchmarkSceneRepository_List benchmarks listing all scenes
func BenchmarkSceneRepository_List(b *testing.B) {
	db, _, sceneRepo := setupBenchDB(b, 100)
	defer db.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sceneRepo.List(ctx)
		if err != nil {
			b.Fatalf("Failed to list scenes: %v", err)
		}
	}
}

// BenchmarkCellRepository_DeleteExpired benchmarks expiration cleanup
func BenchmarkCellRepository_DeleteExpired(b *testing.B) {
	db, cellRepo, _ := setupBenchDB(b, 1000)
	defer db.Close()

	ctx := context.Background()

	// Add some expired cells
	now := time.Now()
	pastCreated := now.Add(-48 * time.Hour)
	pastExpired := now.Add(-1 * time.Hour)

	for i := 0; i < 100; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "expired-conv",
			Scene:          "expired-scene",
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			Content:        fmt.Sprintf("Expired cell %d", i),
			Source:         fmt.Sprintf(`["msg-%d"]`, i),
			CreatedAt:      pastCreated,
			UpdatedAt:      pastCreated,
			ExpiresAt:      &pastExpired,
		}

		err := cellRepo.Create(ctx, cell)
		if err != nil {
			b.Fatalf("Failed to create expired cell: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cellRepo.DeleteExpired(ctx)
		if err != nil {
			b.Fatalf("Failed to delete expired cells: %v", err)
		}
	}
}
