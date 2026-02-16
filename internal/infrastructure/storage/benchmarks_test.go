package storage

import (
	"context"
	"fmt"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ================================================================================
// Benchmark: FileUserProfileRepository
// ================================================================================

func BenchmarkFileUserProfileRepository_SaveProfile(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(filePath, "")

	profile := &domain.UserProfile{
		UserID:        "user-bench-1",
		Moniker:       "benchuser",
		PrimaryEmail:  "bench@example.com",
		DataDirectory: filepath.Join(tmpDir, "user-bench-1"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		PlatformIDs: domain.PlatformIdentifiers{
			CLI: "cli-bench-1",
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		profile.UserID = fmt.Sprintf("user-bench-%d", i)
		if err := repo.SaveProfile(ctx, profile); err != nil {
			b.Fatalf("SaveProfile failed: %v", err)
		}
	}
}

func BenchmarkFileUserProfileRepository_GetProfileByUserID(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(filePath, "")

	// Create test profiles
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		profile := &domain.UserProfile{
			UserID:        fmt.Sprintf("user-%d", i),
			Moniker:       fmt.Sprintf("user%d", i),
			PrimaryEmail:  fmt.Sprintf("user%d@example.com", i),
			DataDirectory: filepath.Join(tmpDir, fmt.Sprintf("user-%d", i)),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			PlatformIDs: domain.PlatformIdentifiers{
				CLI: fmt.Sprintf("cli-%d", i),
			},
		}
		if err := repo.SaveProfile(ctx, profile); err != nil {
			b.Fatalf("Setup SaveProfile failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("user-%d", i%100)
		_, err := repo.GetProfileByUserID(ctx, userID)
		if err != nil {
			b.Fatalf("GetProfileByUserID failed: %v", err)
		}
	}
}

func BenchmarkFileUserProfileRepository_GetProfileByEmail(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "users.json")
	repo := NewFileUserProfileRepository(filePath, "")

	// Create test profiles
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		profile := &domain.UserProfile{
			UserID:        fmt.Sprintf("user-%d", i),
			Moniker:       fmt.Sprintf("user%d", i),
			PrimaryEmail:  fmt.Sprintf("user%d@example.com", i),
			DataDirectory: filepath.Join(tmpDir, fmt.Sprintf("user-%d", i)),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			PlatformIDs: domain.PlatformIdentifiers{
				CLI: fmt.Sprintf("cli-%d", i),
			},
		}
		if err := repo.SaveProfile(ctx, profile); err != nil {
			b.Fatalf("Setup SaveProfile failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		email := fmt.Sprintf("user%d@example.com", i%100)
		_, err := repo.GetProfileByEmail(ctx, email)
		if err != nil {
			b.Fatalf("GetProfileByEmail failed: %v", err)
		}
	}
}

// ================================================================================
// Benchmark: FileConversationRepository
// ================================================================================

func BenchmarkFileConversationRepository_SaveConversation(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileConversationRepository(tmpDir)
	ctx := context.Background()

	conv := &domain.Conversation{
		ID:        "conv-bench-1",
		UserID:    "user-1",
		Platform:  domain.PlatformCLI,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []domain.StoredMessage{
			{
				ID:        "msg-1",
				Role:      "user",
				Content:   "Hello, this is a benchmark message.",
				Timestamp: time.Now(),
			},
			{
				ID:        "msg-2",
				Role:      "assistant",
				Content:   "Hello! This is a benchmark response.",
				Timestamp: time.Now(),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conv.ID = fmt.Sprintf("conv-bench-%d", i)
		if err := repo.SaveConversation(ctx, conv); err != nil {
			b.Fatalf("SaveConversation failed: %v", err)
		}
	}
}

func BenchmarkFileConversationRepository_GetConversation(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileConversationRepository(tmpDir)
	ctx := context.Background()

	// Create test conversations
	for i := 0; i < 10; i++ {
		conv := &domain.Conversation{
			ID:        fmt.Sprintf("conv-%d", i),
			UserID:    "user-1",
			Platform:  domain.PlatformCLI,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages: []domain.StoredMessage{
				{
					ID:        fmt.Sprintf("msg-%d-1", i),
					Role:      "user",
					Content:   "Test message",
					Timestamp: time.Now(),
				},
			},
		}
		if err := repo.SaveConversation(ctx, conv); err != nil {
			b.Fatalf("Setup SaveConversation failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		convID := fmt.Sprintf("conv-%d", i%10)
		_, err := repo.GetConversation(ctx, convID)
		if err != nil {
			b.Fatalf("GetConversation failed: %v", err)
		}
	}
}

func BenchmarkFileConversationRepository_GetConversation_LargeConversation(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileConversationRepository(tmpDir)
	ctx := context.Background()

	// Create conversation with 1000 messages
	conv := &domain.Conversation{
		ID:        "conv-large",
		UserID:    "user-1",
		Platform:  domain.PlatformCLI,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]domain.StoredMessage, 1000),
	}

	for i := 0; i < 1000; i++ {
		conv.Messages[i] = domain.StoredMessage{
			ID:        fmt.Sprintf("msg-%d", i),
			Role:      "user",
			Content:   fmt.Sprintf("This is test message number %d with some content", i),
			Timestamp: time.Now(),
		}
	}

	if err := repo.SaveConversation(ctx, conv); err != nil {
		b.Fatalf("Setup SaveConversation failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.GetConversation(ctx, "conv-large")
		if err != nil {
			b.Fatalf("GetConversation failed: %v", err)
		}
	}
}

func BenchmarkFileConversationRepository_ListConversations(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileConversationRepository(tmpDir)
	ctx := context.Background()

	// Create 50 conversations
	for i := 0; i < 50; i++ {
		conv := &domain.Conversation{
			ID:        fmt.Sprintf("conv-%d", i),
			UserID:    "user-1",
			Platform:  domain.PlatformCLI,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Messages: []domain.StoredMessage{
				{
					ID:        fmt.Sprintf("msg-%d", i),
					Role:      "user",
					Content:   "Test message",
					Timestamp: time.Now(),
				},
			},
		}
		if err := repo.SaveConversation(ctx, conv); err != nil {
			b.Fatalf("Setup SaveConversation failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.ListConversations(ctx, "user-1")
		if err != nil {
			b.Fatalf("ListConversations failed: %v", err)
		}
	}
}

// ================================================================================
// Benchmark: FileMemoryCellRepository
// ================================================================================

func BenchmarkFileMemoryCellRepository_Create(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemoryCellRepository(tmpDir)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "user-1",
			Scene:          "benchmark-scene",
			Content:        "This is a benchmark memory cell content",
			Source:         `["benchmark"]`,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

func BenchmarkFileMemoryCellRepository_Get(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemoryCellRepository(tmpDir)
	ctx := context.Background()

	// Create test memory cells
	cellIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "user-1",
			Scene:          "benchmark-scene",
			Content:        fmt.Sprintf("Memory content %d", i),
			Source:         `["benchmark"]`,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		cellIDs[i] = cell.ID
		if err := repo.Create(ctx, cell); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cellID := cellIDs[i%100]
		_, err := repo.Get(ctx, cellID)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

func BenchmarkFileMemoryCellRepository_Search_SmallResultSet(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemoryCellRepository(tmpDir)
	ctx := context.Background()

	// Create 100 memory cells with varying content
	for i := 0; i < 100; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "user-1",
			Scene:          "benchmark-scene",
			Content:        fmt.Sprintf("Memory about topic %d with some unique content", i),
			Source:         `["benchmark"]`,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5 + float64(i%50)/100.0,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := fmt.Sprintf("topic %d", i%100)
		_, err := repo.SearchFTS(ctx, query, 10)
		if err != nil {
			b.Fatalf("SearchFTS failed: %v", err)
		}
	}
}

func BenchmarkFileMemoryCellRepository_Search_LargeResultSet(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemoryCellRepository(tmpDir)
	ctx := context.Background()

	// Create 1000 memory cells
	for i := 0; i < 1000; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "user-1",
			Scene:          "benchmark-scene",
			Content:        "Memory with common keyword content for searching",
			Source:         `["benchmark"]`,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.5,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.SearchFTS(ctx, "common keyword", 100)
		if err != nil {
			b.Fatalf("SearchFTS failed: %v", err)
		}
	}
}

func BenchmarkFileMemoryCellRepository_GetByScene(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemoryCellRepository(tmpDir)
	ctx := context.Background()

	// Create memory cells across multiple scenes
	for i := 0; i < 100; i++ {
		cell := &memoryv2.MemoryCell{
			ID:             uuid.New().String(),
			ConversationID: "user-1",
			Scene:          fmt.Sprintf("benchmark-scene-%d", i%10),
			Content:        fmt.Sprintf("Memory %d", i),
			Source:         `["benchmark"]`,
			CellType:       memoryv2.CellTypeFact,
			Salience:       0.8,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := repo.Create(ctx, cell); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sceneID := fmt.Sprintf("benchmark-scene-%d", i%10)
		_, err := repo.GetByScene(ctx, sceneID, 20)
		if err != nil {
			b.Fatalf("GetByScene failed: %v", err)
		}
	}
}

// ================================================================================
// Benchmark: FileMemorySceneRepository
// ================================================================================

func BenchmarkFileMemorySceneRepository_Upsert(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemorySceneRepository(tmpDir)
	ctx := context.Background()

	scene := &memoryv2.MemoryScene{
		Scene:      "scene-bench-1",
		Summary:    "A scene for benchmarking",
		TokenCount: 10,
		UpdatedAt:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scene.Scene = fmt.Sprintf("scene-bench-%d", i)
		if err := repo.Upsert(ctx, scene); err != nil {
			b.Fatalf("Upsert failed: %v", err)
		}
	}
}

func BenchmarkFileMemorySceneRepository_Get(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileMemorySceneRepository(tmpDir)
	ctx := context.Background()

	// Create test scenes
	for i := 0; i < 50; i++ {
		scene := &memoryv2.MemoryScene{
			Scene:      fmt.Sprintf("scene-%d", i),
			Summary:    "Test scene",
			TokenCount: 10,
			UpdatedAt:  time.Now(),
		}
		if err := repo.Upsert(ctx, scene); err != nil {
			b.Fatalf("Setup Upsert failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sceneID := fmt.Sprintf("scene-%d", i%50)
		_, err := repo.Get(ctx, sceneID)
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// ================================================================================
// Benchmark: FileNotesRepository
// ================================================================================

func BenchmarkFileNotesRepository_Create(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileNotesRepository(tmpDir)
	ctx := context.Background()

	note := &domain.Note{
		ID:        "note-bench-1",
		UserID:    "user-1",
		Title:     "Benchmark Note",
		Content:   "This is a benchmark note with some content",
		Tags:      []string{"benchmark", "test"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		note.ID = fmt.Sprintf("note-bench-%d", i)
		if err := repo.Create(ctx, note); err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

func BenchmarkFileNotesRepository_GetByID(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileNotesRepository(tmpDir)
	ctx := context.Background()

	// Create test notes
	for i := 0; i < 100; i++ {
		note := &domain.Note{
			ID:        fmt.Sprintf("note-%d", i),
			UserID:    "user-1",
			Title:     fmt.Sprintf("Note %d", i),
			Content:   "Test note content",
			Tags:      []string{"test"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, note); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		noteID := fmt.Sprintf("note-%d", i%100)
		_, err := repo.GetByID(ctx, noteID)
		if err != nil {
			b.Fatalf("GetByID failed: %v", err)
		}
	}
}

func BenchmarkFileNotesRepository_List(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	repo := NewFileNotesRepository(tmpDir)
	ctx := context.Background()

	// Create 100 notes
	for i := 0; i < 100; i++ {
		note := &domain.Note{
			ID:        fmt.Sprintf("note-%d", i),
			UserID:    "user-1",
			Title:     fmt.Sprintf("Note %d", i),
			Content:   "Test note content",
			Tags:      []string{"test", fmt.Sprintf("tag-%d", i%10)},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Create(ctx, note); err != nil {
			b.Fatalf("Setup Create failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.List(ctx, "user-1")
		if err != nil {
			b.Fatalf("List failed: %v", err)
		}
	}
}

// ================================================================================
// Benchmark: FileAuditRepository
// ================================================================================

func BenchmarkFileAuditRepository_Append(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "audit.jsonl")
	repo := NewFileAuditRepository(filePath)
	ctx := context.Background()

	event := &domain.AuditEvent{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Action:    "benchmark.action",
		Resource:  "test-resource",
		Outcome:   "success",
		Details:   map[string]any{"test": "data"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.Timestamp = time.Now()
		if err := repo.Append(ctx, event); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
	}
}

func BenchmarkFileAuditRepository_Query_LargeLog(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "audit.jsonl")
	repo := NewFileAuditRepository(filePath)
	ctx := context.Background()

	// Create 1000 audit events
	for i := 0; i < 1000; i++ {
		event := &domain.AuditEvent{
			Timestamp: time.Now(),
			UserID:    "user-1",
			Action:    "test.action",
			Resource:  "test-resource",
			Outcome:   "success",
			Details:   map[string]any{"index": i},
		}
		if err := repo.Append(ctx, event); err != nil {
			b.Fatalf("Setup Append failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter := domain.AuditFilter{
			UserID: "user-1",
			Limit:  100,
		}
		// Read first 100 events
		events, err := repo.Query(ctx, filter)
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
		if len(events) > 100 {
			b.Fatalf("Expected <= 100 events, got %d", len(events))
		}
	}
}

// ================================================================================
// Helper: Clean up test files
// ================================================================================

func init() {
	// Ensure clean state for benchmarks
	os.Setenv("TESTING", "true")
}
