package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"nuimanbot/internal/domain/memoryv2"
	memory "nuimanbot/internal/infrastructure/memory"
	usecasememory "nuimanbot/internal/usecase/memoryv2"
)

// mockMemoryLLMClient implements usecasememory.LLMClient for deterministic E2E tests.
// It distinguishes extraction from consolidation calls by checking the system prompt content.
type mockMemoryLLMClient struct {
	extractionResponses []string // Queue of JSON responses for extraction calls (consumed in order)
	consolidationResp   string   // JSON response for consolidation calls (reusable)
	callCount           int
	lastSystemPrompt    string
	lastUserPrompt      string
}

func (m *mockMemoryLLMClient) GenerateJSON(ctx context.Context, systemPrompt, userPrompt string, responseSchema interface{}) (string, error) {
	m.callCount++
	m.lastSystemPrompt = systemPrompt
	m.lastUserPrompt = userPrompt

	// Consolidation calls contain "consolidation" in system prompt
	if containsSubstring(systemPrompt, "consolidation") {
		if m.consolidationResp != "" {
			return m.consolidationResp, nil
		}
		return `{"summary":"default summary","token_count":5}`, nil
	}

	// Extraction calls - consume from queue
	if len(m.extractionResponses) > 0 {
		resp := m.extractionResponses[0]
		if len(m.extractionResponses) > 1 {
			m.extractionResponses = m.extractionResponses[1:]
		}
		return resp, nil
	}

	return `{"cells":[]}`, nil
}

// containsSubstring checks if s contains substr (case-insensitive match on lowercase).
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// setupMemoryTestDB creates a SQLite database with the memory schema applied.
func setupMemoryTestDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "memory_e2e_test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Apply memory migration schema inline (same as 001_memory_tables.sql)
	schema := `
		CREATE TABLE IF NOT EXISTS memory_cells (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			scene TEXT NOT NULL,
			cell_type TEXT NOT NULL,
			salience REAL NOT NULL,
			content TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP,
			CONSTRAINT chk_salience CHECK (salience >= 0.0 AND salience <= 1.0),
			CONSTRAINT chk_cell_type CHECK (cell_type IN ('fact', 'decision', 'task', 'preference', 'plan', 'risk'))
		);

		CREATE INDEX IF NOT EXISTS idx_memory_cells_conversation ON memory_cells(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_memory_cells_scene ON memory_cells(scene);
		CREATE INDEX IF NOT EXISTS idx_memory_cells_salience ON memory_cells(salience DESC);
		CREATE INDEX IF NOT EXISTS idx_memory_cells_expires_at ON memory_cells(expires_at) WHERE expires_at IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_memory_cells_created_at ON memory_cells(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_memory_cells_conv_scene ON memory_cells(conversation_id, scene);

		CREATE TABLE IF NOT EXISTS memory_scenes (
			scene TEXT PRIMARY KEY,
			summary TEXT NOT NULL,
			token_count INTEGER NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			CONSTRAINT chk_token_count CHECK (token_count > 0 AND token_count <= 2000)
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS memory_cells_fts USING fts5(
			content,
			scene,
			cell_type,
			content='memory_cells',
			content_rowid='rowid'
		);

		CREATE TRIGGER IF NOT EXISTS memory_cells_ai
		AFTER INSERT ON memory_cells
		BEGIN
			INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
			VALUES (new.rowid, new.content, new.scene, new.cell_type);
		END;

		CREATE TRIGGER IF NOT EXISTS memory_cells_ad
		AFTER DELETE ON memory_cells
		BEGIN
			DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
		END;

		CREATE TRIGGER IF NOT EXISTS memory_cells_au
		AFTER UPDATE ON memory_cells
		BEGIN
			DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
			INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
			VALUES (new.rowid, new.content, new.scene, new.cell_type);
		END;
	`

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to apply memory schema: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// memoryTestEnv holds all components for memory E2E tests.
type memoryTestEnv struct {
	db        *sql.DB
	cellRepo  memoryv2.MemoryCellRepository
	sceneRepo memoryv2.MemorySceneRepository
	curator   *usecasememory.MemoryCuratorService
	recall    *usecasememory.MemoryRecallService
	mockLLM   *mockMemoryLLMClient
}

// setupMemoryTestEnv creates the full memory test environment with real SQLite repos.
func setupMemoryTestEnv(t *testing.T) *memoryTestEnv {
	t.Helper()

	db := setupMemoryTestDB(t)
	cellRepo := memory.NewSQLiteMemoryCellRepository(db)
	sceneRepo := memory.NewSQLiteMemorySceneRepository(db)

	mockLLM := &mockMemoryLLMClient{}

	curatorConfig := usecasememory.CuratorConfig{
		Enabled:               true,
		ExtractionModel:       "mock-model",
		ConsolidationModel:    "mock-model",
		MaxCellsPerExtraction: 10,
		RetryOnInvalidJSON:    false,
		SceneSummaryMaxTokens: 500,
	}

	recallConfig := usecasememory.RecallConfig{
		FTSResultLimit:    20,
		SalienceThreshold: 0.7,
		FallbackCellLimit: 10,
		MaxScenes:         10,
		TokenBudget:       1000,
	}

	curator := usecasememory.NewMemoryCuratorService(mockLLM, cellRepo, sceneRepo, curatorConfig)
	recall := usecasememory.NewMemoryRecallService(cellRepo, sceneRepo, recallConfig)

	return &memoryTestEnv{
		db:        db,
		cellRepo:  cellRepo,
		sceneRepo: sceneRepo,
		curator:   curator,
		recall:    recall,
		mockLLM:   mockLLM,
	}
}

// buildExtractionJSON builds a valid extraction response JSON from cell descriptors.
func buildExtractionJSON(cells []usecasememory.ExtractedCell) string {
	resp := usecasememory.ExtractionResponse{Cells: cells}
	data, _ := json.Marshal(resp)
	return string(data)
}

// buildConsolidationJSON builds a valid consolidation response JSON.
func buildConsolidationJSON(summary string, tokenCount int) string {
	resp := usecasememory.SceneConsolidationResponse{
		Summary:    summary,
		TokenCount: tokenCount,
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

// =============================================================================
// E2E Tests: Full Memory Lifecycle
// =============================================================================

func TestMemoryE2E_FullLifecycle_ExtractStoreRecall(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Step 1: Configure mock LLM to return memory cells from a conversation
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "project-setup",
				CellType: "decision",
				Salience: 0.9,
				Content:  "User decided to use Go with Clean Architecture for the NuimanBot project",
				Source:   []string{"msg-1"},
			},
			{
				Scene:    "project-setup",
				CellType: "preference",
				Salience: 0.85,
				Content:  "User prefers TDD with strict Red-Green-Refactor cycle",
				Source:   []string{"msg-1", "msg-2"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"NuimanBot project uses Go with Clean Architecture. Development follows strict TDD methodology with Red-Green-Refactor cycle.",
		25,
	)

	// Step 2: Extract memory cells from an interaction
	interaction := usecasememory.InteractionContext{
		ConversationID: "conv-e2e-001",
		UserMessage:    "I want to build NuimanBot using Go with Clean Architecture and TDD",
		AssistantReply: "Great choices! Clean Architecture keeps dependencies clean and TDD ensures quality.",
		MessageIDs:     []string{"msg-1", "msg-2"},
		Timestamp:      time.Now(),
	}

	result, err := env.curator.ExtractCells(ctx, interaction)
	if err != nil {
		t.Fatalf("ExtractCells failed: %v", err)
	}

	if result.CellsCreated != 2 {
		t.Errorf("Expected 2 cells created, got %d", result.CellsCreated)
	}
	if result.ScenesUpdated != 1 {
		t.Errorf("Expected 1 scene updated, got %d", result.ScenesUpdated)
	}

	// Step 3: Verify cells are persisted in SQLite
	filter := memoryv2.MemoryCellFilter{
		ConversationID: "conv-e2e-001",
		Scene:          "project-setup",
	}
	storedCells, err := env.cellRepo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List cells failed: %v", err)
	}
	if len(storedCells) != 2 {
		t.Errorf("Expected 2 stored cells, got %d", len(storedCells))
	}

	// Step 4: Verify scene was consolidated
	scene, err := env.sceneRepo.Get(ctx, "project-setup")
	if err != nil {
		t.Fatalf("Get scene failed: %v", err)
	}
	if scene.Summary == "" {
		t.Error("Scene summary is empty")
	}
	if scene.TokenCount != 25 {
		t.Errorf("Expected scene token count 25, got %d", scene.TokenCount)
	}

	// Step 5: Recall memories using FTS search
	recallReq := usecasememory.RecallRequest{
		ConversationID: "conv-e2e-001",
		Query:          "Clean Architecture",
		MaxTokens:      500,
		MaxCells:       10,
	}

	recallResp, err := env.recall.RecallMemory(ctx, recallReq)
	if err != nil {
		t.Fatalf("RecallMemory failed: %v", err)
	}

	if len(recallResp.Cells) == 0 {
		t.Error("Expected recalled cells, got none")
	}
	if recallResp.FTSMatchCount == 0 {
		t.Error("Expected FTS matches, got 0")
	}
	if recallResp.FallbackUsed {
		t.Error("Expected no fallback when FTS has results")
	}

	// Step 6: Format memory for injection and verify output
	formatted := env.recall.FormatMemoryForInjection(recallResp)
	if formatted == "" {
		t.Error("Formatted memory is empty")
	}
	if !containsSubstring(formatted, "project-setup") {
		t.Error("Formatted memory should contain scene name 'project-setup'")
	}
	if !containsSubstring(formatted, "Clean Architecture") {
		t.Error("Formatted memory should contain recalled content")
	}

	t.Logf("Full lifecycle test passed: %d cells created, %d recalled, formatted output:\n%s",
		result.CellsCreated, len(recallResp.Cells), formatted)
}

func TestMemoryE2E_MultiInteraction_FTSAcrossConversations(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Interaction 1: User discusses authentication
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "authentication",
				CellType: "decision",
				Salience: 0.9,
				Content:  "User chose OAuth2 with JWT tokens for authentication",
				Source:   []string{"msg-auth-1"},
			},
			{
				Scene:    "authentication",
				CellType: "fact",
				Salience: 0.8,
				Content:  "JWT tokens will have 24-hour expiry with refresh token rotation",
				Source:   []string{"msg-auth-2"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Authentication uses OAuth2 with JWT tokens. Tokens expire after 24 hours with refresh token rotation.",
		18,
	)

	_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-002",
		UserMessage:    "Let's use OAuth2 with JWT for auth. Tokens should expire after 24 hours.",
		AssistantReply: "Good security practice. I'll set up JWT with 24h expiry and refresh rotation.",
		MessageIDs:     []string{"msg-auth-1", "msg-auth-2"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Interaction 1 extraction failed: %v", err)
	}

	// Interaction 2: User discusses database setup
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "database-setup",
				CellType: "decision",
				Salience: 0.85,
				Content:  "Using SQLite for development and PostgreSQL for production",
				Source:   []string{"msg-db-1"},
			},
			{
				Scene:    "database-setup",
				CellType: "task",
				Salience: 0.75,
				Content:  "Set up database migration system using golang-migrate",
				Source:   []string{"msg-db-2"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Database uses SQLite for development and PostgreSQL for production. Migration managed by golang-migrate.",
		15,
	)

	_, err = env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-002",
		UserMessage:    "For the database, use SQLite in dev and PostgreSQL in production. Set up migrations.",
		AssistantReply: "I'll configure SQLite for dev, PostgreSQL for prod with golang-migrate for schema management.",
		MessageIDs:     []string{"msg-db-1", "msg-db-2"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Interaction 2 extraction failed: %v", err)
	}

	// Interaction 3: More auth details in a separate conversation
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "authentication",
				CellType: "fact",
				Salience: 0.7,
				Content:  "API rate limiting set to 100 requests per minute per user",
				Source:   []string{"msg-rate-1"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Authentication uses OAuth2 with JWT tokens (24h expiry, refresh rotation). API rate limited to 100 req/min per user.",
		22,
	)

	_, err = env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-003",
		UserMessage:    "Add rate limiting to the API - 100 requests per minute per user",
		AssistantReply: "I'll implement rate limiting at 100 req/min per user using a token bucket.",
		MessageIDs:     []string{"msg-rate-1"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Interaction 3 extraction failed: %v", err)
	}

	// FTS search for "authentication" should find cells from interactions 1 and 3
	recallResp, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-002",
		Query:          "authentication",
		MaxTokens:      1000,
		MaxCells:       20,
	})
	if err != nil {
		t.Fatalf("Recall for 'authentication' failed: %v", err)
	}

	// Should find cells mentioning authentication across scenes
	if recallResp.FTSMatchCount == 0 {
		t.Error("Expected FTS matches for 'authentication'")
	}

	// FTS search for "SQLite" should find database cells
	recallRespDB, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-002",
		Query:          "SQLite",
		MaxTokens:      1000,
		MaxCells:       20,
	})
	if err != nil {
		t.Fatalf("Recall for 'SQLite' failed: %v", err)
	}

	if recallRespDB.FTSMatchCount == 0 {
		t.Error("Expected FTS matches for 'SQLite'")
	}

	// FTS search for "quantum computing" should find nothing
	recallRespEmpty, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-002",
		Query:          "quantum computing",
		MaxTokens:      1000,
		MaxCells:       20,
	})
	if err != nil {
		t.Fatalf("Recall for 'quantum computing' failed: %v", err)
	}

	if recallRespEmpty.FTSMatchCount != 0 {
		t.Errorf("Expected 0 FTS matches for 'quantum computing', got %d", recallRespEmpty.FTSMatchCount)
	}

	// With no FTS matches, fallback should be used
	if !recallRespEmpty.FallbackUsed {
		t.Error("Expected salience fallback when FTS returns no results")
	}

	t.Logf("Multi-interaction FTS test passed: auth=%d matches, sqlite=%d matches, quantum=%d matches (fallback=%v)",
		recallResp.FTSMatchCount, recallRespDB.FTSMatchCount,
		recallRespEmpty.FTSMatchCount, recallRespEmpty.FallbackUsed)
}

func TestMemoryE2E_SceneConsolidation_IncrementalUpdates(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// First interaction: Initial cells for a scene
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "deployment",
				CellType: "decision",
				Salience: 0.85,
				Content:  "Deploying to AWS using ECS Fargate",
				Source:   []string{"msg-deploy-1"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Deployment targets AWS using ECS Fargate.",
		8,
	)

	_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-004",
		UserMessage:    "Deploy to AWS with ECS Fargate",
		AssistantReply: "Setting up ECS Fargate deployment.",
		MessageIDs:     []string{"msg-deploy-1"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("First extraction failed: %v", err)
	}

	// Verify initial scene summary
	scene1, err := env.sceneRepo.Get(ctx, "deployment")
	if err != nil {
		t.Fatalf("Get scene after first extraction failed: %v", err)
	}
	if scene1.TokenCount != 8 {
		t.Errorf("Expected initial token count 8, got %d", scene1.TokenCount)
	}

	// Second interaction: More cells for same scene
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "deployment",
				CellType: "decision",
				Salience: 0.8,
				Content:  "Using GitHub Actions for CI/CD pipeline",
				Source:   []string{"msg-deploy-2"},
			},
			{
				Scene:    "deployment",
				CellType: "task",
				Salience: 0.7,
				Content:  "Configure health checks and auto-scaling policies",
				Source:   []string{"msg-deploy-3"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Deployment targets AWS ECS Fargate with GitHub Actions CI/CD. Health checks and auto-scaling policies need configuration.",
		20,
	)

	_, err = env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-004",
		UserMessage:    "Use GitHub Actions for CI/CD and add health checks with auto-scaling",
		AssistantReply: "Configuring GitHub Actions pipeline with health checks and auto-scaling.",
		MessageIDs:     []string{"msg-deploy-2", "msg-deploy-3"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Second extraction failed: %v", err)
	}

	// Verify scene was updated (not duplicated)
	scenes, err := env.sceneRepo.List(ctx)
	if err != nil {
		t.Fatalf("List scenes failed: %v", err)
	}

	deploySceneCount := 0
	for _, s := range scenes {
		if s.Scene == "deployment" {
			deploySceneCount++
		}
	}
	if deploySceneCount != 1 {
		t.Errorf("Expected exactly 1 deployment scene, got %d", deploySceneCount)
	}

	// Verify updated summary
	scene2, err := env.sceneRepo.Get(ctx, "deployment")
	if err != nil {
		t.Fatalf("Get scene after second extraction failed: %v", err)
	}
	if scene2.TokenCount != 20 {
		t.Errorf("Expected updated token count 20, got %d", scene2.TokenCount)
	}
	if scene2.Summary == scene1.Summary {
		t.Error("Scene summary should have been updated")
	}

	// Verify all 3 cells exist in the scene
	cells, err := env.cellRepo.GetByScene(ctx, "deployment", 0)
	if err != nil {
		t.Fatalf("GetByScene failed: %v", err)
	}
	if len(cells) != 3 {
		t.Errorf("Expected 3 cells in deployment scene, got %d", len(cells))
	}

	t.Logf("Scene consolidation test passed: %d cells, summary updated from %d to %d tokens",
		len(cells), scene1.TokenCount, scene2.TokenCount)
}

func TestMemoryE2E_TokenBudgetEnforcement(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Create many cells to exceed token budget
	for i := 0; i < 20; i++ {
		env.mockLLM.extractionResponses = []string{
			buildExtractionJSON([]usecasememory.ExtractedCell{
				{
					Scene:    "largeproject",
					CellType: "fact",
					Salience: 0.8,
					Content:  fmt.Sprintf("Important fact number %d about the large-project system design and architecture decisions that were made during the planning phase", i+1),
					Source:   []string{fmt.Sprintf("msg-%d", i+1)},
				},
			}),
		}
		env.mockLLM.consolidationResp = buildConsolidationJSON(
			fmt.Sprintf("Large project with %d documented facts about system design.", i+1),
			10+i,
		)

		_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
			ConversationID: "conv-e2e-005",
			UserMessage:    fmt.Sprintf("Fact %d about the project", i+1),
			AssistantReply: fmt.Sprintf("Noted fact %d.", i+1),
			MessageIDs:     []string{fmt.Sprintf("msg-%d", i+1)},
			Timestamp:      time.Now(),
		})
		if err != nil {
			t.Fatalf("Extraction %d failed: %v", i+1, err)
		}
	}

	// Verify all 20 cells were stored
	allCells, err := env.cellRepo.GetByScene(ctx, "largeproject", 0)
	if err != nil {
		t.Fatalf("GetByScene failed: %v", err)
	}
	if len(allCells) != 20 {
		t.Errorf("Expected 20 cells stored, got %d", len(allCells))
	}

	// Recall with a small token budget - should return fewer cells
	smallBudget := usecasememory.RecallRequest{
		ConversationID: "conv-e2e-005",
		Query:          "architecture",
		MaxTokens:      100, // Very small budget
		MaxCells:       20,
	}

	recallResp, err := env.recall.RecallMemory(ctx, smallBudget)
	if err != nil {
		t.Fatalf("RecallMemory with small budget failed: %v", err)
	}

	// Should have fewer cells than total due to budget
	if len(recallResp.Cells) >= 20 {
		t.Errorf("Expected token budget to limit cells, but got all %d", len(recallResp.Cells))
	}
	if recallResp.TotalTokens > 100 {
		t.Errorf("Expected total tokens <= 100, got %d", recallResp.TotalTokens)
	}

	// Recall with a large budget - should return more cells
	largeBudget := usecasememory.RecallRequest{
		ConversationID: "conv-e2e-005",
		Query:          "architecture",
		MaxTokens:      10000,
		MaxCells:       20,
	}

	recallRespLarge, err := env.recall.RecallMemory(ctx, largeBudget)
	if err != nil {
		t.Fatalf("RecallMemory with large budget failed: %v", err)
	}

	if len(recallRespLarge.Cells) <= len(recallResp.Cells) {
		t.Errorf("Expected more cells with larger budget: small=%d, large=%d",
			len(recallResp.Cells), len(recallRespLarge.Cells))
	}

	t.Logf("Token budget test passed: small budget=%d cells (%d tokens), large budget=%d cells (%d tokens)",
		len(recallResp.Cells), recallResp.TotalTokens,
		len(recallRespLarge.Cells), recallRespLarge.TotalTokens)
}

func TestMemoryE2E_SalienceFallback_WhenFTSEmpty(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Store cells with specific content (not matching the query we'll use)
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "user-preferences",
				CellType: "preference",
				Salience: 0.95,
				Content:  "User strongly prefers dark mode in all applications",
				Source:   []string{"msg-pref-1"},
			},
			{
				Scene:    "user-preferences",
				CellType: "preference",
				Salience: 0.6,
				Content:  "User sometimes uses vim keybindings",
				Source:   []string{"msg-pref-2"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"User prefers dark mode and occasionally uses vim keybindings.",
		10,
	)

	_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-006",
		UserMessage:    "I prefer dark mode and sometimes use vim keybindings",
		AssistantReply: "Noted your preferences for dark mode and vim keybindings.",
		MessageIDs:     []string{"msg-pref-1", "msg-pref-2"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	// Search for something completely unrelated - FTS will find nothing
	recallResp, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-006",
		Query:          "banana smoothie recipe",
		MaxTokens:      1000,
		MaxCells:       10,
	})
	if err != nil {
		t.Fatalf("RecallMemory failed: %v", err)
	}

	// FTS should have 0 matches
	if recallResp.FTSMatchCount != 0 {
		t.Errorf("Expected 0 FTS matches for unrelated query, got %d", recallResp.FTSMatchCount)
	}

	// Fallback should be used
	if !recallResp.FallbackUsed {
		t.Error("Expected salience fallback to be used")
	}

	// Should still get high-salience cells (threshold 0.7, so only the 0.95 cell)
	if len(recallResp.Cells) == 0 {
		t.Error("Expected salience fallback to return high-salience cells")
	}

	// The high-salience cell (0.95) should be returned, low-salience (0.6) should not
	for _, cell := range recallResp.Cells {
		if cell.Salience < 0.7 {
			t.Errorf("Fallback returned cell with salience %.2f below threshold 0.7", cell.Salience)
		}
	}

	t.Logf("Salience fallback test passed: FTS=%d, fallback=%v, cells=%d",
		recallResp.FTSMatchCount, recallResp.FallbackUsed, len(recallResp.Cells))
}

func TestMemoryE2E_CuratorDisabled_NoCellsExtracted(t *testing.T) {
	db := setupMemoryTestDB(t)
	cellRepo := memory.NewSQLiteMemoryCellRepository(db)
	sceneRepo := memory.NewSQLiteMemorySceneRepository(db)
	mockLLM := &mockMemoryLLMClient{}

	// Create curator with Enabled=false
	disabledConfig := usecasememory.CuratorConfig{
		Enabled: false,
	}

	curator := usecasememory.NewMemoryCuratorService(mockLLM, cellRepo, sceneRepo, disabledConfig)
	ctx := context.Background()

	result, err := curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-disabled",
		UserMessage:    "This should not be extracted",
		AssistantReply: "Nothing to extract here",
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Expected no error when disabled, got: %v", err)
	}

	if result.CellsCreated != 0 {
		t.Errorf("Expected 0 cells when disabled, got %d", result.CellsCreated)
	}

	// LLM should not have been called
	if mockLLM.callCount != 0 {
		t.Errorf("Expected 0 LLM calls when disabled, got %d", mockLLM.callCount)
	}

	// Database should be empty
	allCells, err := cellRepo.List(ctx, memoryv2.MemoryCellFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(allCells) != 0 {
		t.Errorf("Expected empty database, got %d cells", len(allCells))
	}
}

func TestMemoryE2E_FormatMemoryForInjection_StructuredOutput(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Create cells across multiple scenes
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "auth-design",
				CellType: "decision",
				Salience: 0.9,
				Content:  "Using OAuth2 with PKCE flow for authentication",
				Source:   []string{"msg-1"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON("OAuth2 PKCE flow for authentication.", 6)

	_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-007",
		UserMessage:    "Use OAuth2 with PKCE",
		AssistantReply: "Setting up PKCE flow.",
		MessageIDs:     []string{"msg-1"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	// Recall and format
	recallResp, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-007",
		Query:          "OAuth2",
		MaxTokens:      1000,
		MaxCells:       10,
	})
	if err != nil {
		t.Fatalf("RecallMemory failed: %v", err)
	}

	formatted := env.recall.FormatMemoryForInjection(recallResp)

	// Verify structured output format
	if !containsSubstring(formatted, "Relevant Long-Term Memory") {
		t.Error("Formatted output should contain header")
	}
	if !containsSubstring(formatted, "Scene: auth-design") {
		t.Error("Formatted output should contain scene name")
	}
	if !containsSubstring(formatted, "OAuth2 PKCE") {
		t.Error("Formatted output should contain scene summary")
	}
	if !containsSubstring(formatted, "Key Facts") {
		t.Error("Formatted output should contain key facts section")
	}
	if !containsSubstring(formatted, "Retrieved") {
		t.Error("Formatted output should contain retrieval stats")
	}

	t.Logf("Format test passed. Output:\n%s", formatted)
}

func TestMemoryE2E_EmptyRecall_ReturnsEmptyFormatted(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Recall from empty database
	recallResp, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-nonexistent",
		Query:          "anything",
		MaxTokens:      1000,
		MaxCells:       10,
	})
	if err != nil {
		t.Fatalf("RecallMemory on empty DB failed: %v", err)
	}

	if len(recallResp.Cells) != 0 {
		t.Errorf("Expected 0 cells from empty DB, got %d", len(recallResp.Cells))
	}

	formatted := env.recall.FormatMemoryForInjection(recallResp)
	if formatted != "" {
		t.Errorf("Expected empty formatted output for no memory, got: %s", formatted)
	}
}

func TestMemoryE2E_ExpiredCells_ExcludedFromRecall(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Manually create a cell with past expiration
	pastCreated := time.Now().Add(-48 * time.Hour)
	pastExpired := time.Now().Add(-24 * time.Hour)
	expiredCellID := uuid.New().String()

	expiredCell := &memoryv2.MemoryCell{
		ID:             expiredCellID,
		ConversationID: "conv-e2e-008",
		Scene:          "expired-scene",
		CellType:       memoryv2.CellTypeFact,
		Salience:       0.9,
		Content:        "This expired fact about kubernetes deployment should not appear",
		Source:         `["msg-expired"]`,
		CreatedAt:      pastCreated,
		UpdatedAt:      pastCreated,
		ExpiresAt:      &pastExpired,
	}

	err := env.cellRepo.Create(ctx, expiredCell)
	if err != nil {
		t.Fatalf("Failed to create expired cell: %v", err)
	}

	// Also create a non-expired cell
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{
				Scene:    "active-scene",
				CellType: "fact",
				Salience: 0.8,
				Content:  "This active fact about kubernetes networking should appear",
				Source:   []string{"msg-active"},
			},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON("Active kubernetes networking facts.", 5)

	_, err = env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-008",
		UserMessage:    "Tell me about kubernetes networking",
		AssistantReply: "Here's info about kubernetes networking.",
		MessageIDs:     []string{"msg-active"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	// Delete expired cells
	deleted, err := env.cellRepo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 expired cell deleted, got %d", deleted)
	}

	// Search for "kubernetes" - should only find the active cell
	recallResp, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-e2e-008",
		Query:          "kubernetes",
		MaxTokens:      1000,
		MaxCells:       10,
	})
	if err != nil {
		t.Fatalf("RecallMemory failed: %v", err)
	}

	for _, cell := range recallResp.Cells {
		if cell.ID == expiredCellID {
			t.Error("Expired cell should not appear in recall results")
		}
	}

	t.Logf("Expired cells test passed: %d deleted, %d recalled", deleted, len(recallResp.Cells))
}

func TestMemoryE2E_MultipleCellTypes_AllPersisted(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Extract one of each cell type
	env.mockLLM.extractionResponses = []string{
		buildExtractionJSON([]usecasememory.ExtractedCell{
			{Scene: "all-types", CellType: "fact", Salience: 0.8, Content: "Go is a statically typed language", Source: []string{"msg-1"}},
			{Scene: "all-types", CellType: "decision", Salience: 0.85, Content: "Chose Go over Rust for the project", Source: []string{"msg-1"}},
			{Scene: "all-types", CellType: "task", Salience: 0.7, Content: "Write unit tests for all handlers", Source: []string{"msg-2"}},
			{Scene: "all-types", CellType: "preference", Salience: 0.9, Content: "User prefers table-driven tests in Go", Source: []string{"msg-2"}},
			{Scene: "all-types", CellType: "plan", Salience: 0.75, Content: "Plan to migrate to gRPC in Q3", Source: []string{"msg-3"}},
			{Scene: "all-types", CellType: "risk", Salience: 0.65, Content: "SQLite may not scale beyond 10K concurrent users", Source: []string{"msg-3"}},
		}),
	}
	env.mockLLM.consolidationResp = buildConsolidationJSON(
		"Go project with various decisions, tasks, preferences, plans, and risks documented.",
		12,
	)

	result, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
		ConversationID: "conv-e2e-009",
		UserMessage:    "Let me tell you about the project details",
		AssistantReply: "I've noted all the project details.",
		MessageIDs:     []string{"msg-1", "msg-2", "msg-3"},
		Timestamp:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Extraction failed: %v", err)
	}

	if result.CellsCreated != 6 {
		t.Errorf("Expected 6 cells (one per type), got %d", result.CellsCreated)
	}

	// Verify all cell types are stored
	cells, err := env.cellRepo.GetByScene(ctx, "all-types", 0)
	if err != nil {
		t.Fatalf("GetByScene failed: %v", err)
	}

	typeCount := make(map[memoryv2.CellType]int)
	for _, cell := range cells {
		typeCount[cell.CellType]++
	}

	expectedTypes := []memoryv2.CellType{
		memoryv2.CellTypeFact,
		memoryv2.CellTypeDecision,
		memoryv2.CellTypeTask,
		memoryv2.CellTypePreference,
		memoryv2.CellTypePlan,
		memoryv2.CellTypeRisk,
	}

	for _, ct := range expectedTypes {
		if typeCount[ct] != 1 {
			t.Errorf("Expected 1 cell of type %s, got %d", ct.String(), typeCount[ct])
		}
	}

	t.Logf("All cell types test passed: %v", typeCount)
}

func TestMemoryE2E_RecallPerformance_Under50ms(t *testing.T) {
	env := setupMemoryTestEnv(t)
	ctx := context.Background()

	// Insert a moderate number of cells
	for i := 0; i < 50; i++ {
		env.mockLLM.extractionResponses = []string{
			buildExtractionJSON([]usecasememory.ExtractedCell{
				{
					Scene:    fmt.Sprintf("perf-scene-%d", i%5),
					CellType: "fact",
					Salience: 0.5 + float64(i%5)*0.1,
					Content:  fmt.Sprintf("Performance test cell %d with searchable content about software engineering topic %d", i, i),
					Source:   []string{fmt.Sprintf("msg-perf-%d", i)},
				},
			}),
		}
		env.mockLLM.consolidationResp = buildConsolidationJSON(
			fmt.Sprintf("Performance scene %d summary.", i%5),
			5,
		)

		_, err := env.curator.ExtractCells(ctx, usecasememory.InteractionContext{
			ConversationID: "conv-perf",
			UserMessage:    fmt.Sprintf("Perf test %d", i),
			AssistantReply: fmt.Sprintf("Perf reply %d", i),
			MessageIDs:     []string{fmt.Sprintf("msg-perf-%d", i)},
			Timestamp:      time.Now(),
		})
		if err != nil {
			t.Fatalf("Extraction %d failed: %v", i, err)
		}
	}

	// Measure recall performance
	start := time.Now()
	_, err := env.recall.RecallMemory(ctx, usecasememory.RecallRequest{
		ConversationID: "conv-perf",
		Query:          "software engineering",
		MaxTokens:      500,
		MaxCells:       20,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RecallMemory failed: %v", err)
	}

	if elapsed > 50*time.Millisecond {
		t.Errorf("Recall took %v, expected < 50ms", elapsed)
	}

	t.Logf("Performance test passed: recall took %v for 50 cells", elapsed)
}
