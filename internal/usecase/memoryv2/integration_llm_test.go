package memoryv2

import (
	"context"
	"os"
	"testing"
	"time"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	domainmemory "nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/llm/anthropic"
)

// skipIfNoAPIKey skips the test if no Anthropic API key is available
func skipIfNoAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: ANTHROPIC_API_KEY not set")
	}
}

// setupRealLLMClient creates a real Anthropic LLM client for testing
func setupRealLLMClient(t *testing.T) LLMClient {
	t.Helper()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Fatal("ANTHROPIC_API_KEY not set")
	}

	cfg := &config.LLMProviderConfig{
		Type:   domain.LLMProviderAnthropic,
		APIKey: domain.NewSecureStringFromString(apiKey),
	}

	client, err := anthropic.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create Anthropic client: %v", err)
	}

	// Wrap in adapter
	adapter := NewLLMServiceAdapter(client, domain.LLMProviderAnthropic, "claude-3-haiku-20240307")
	return adapter
}

func TestMemoryCuratorService_Integration_RealLLM(t *testing.T) {
	skipIfNoAPIKey(t)

	llmClient := setupRealLLMClient(t)
	mockCellRepo := &MockCellRepository{}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{
		Enabled:               true,
		ExtractionModel:       "claude-3-haiku-20240307",
		ConsolidationModel:    "claude-3-haiku-20240307",
		MaxCellsPerExtraction: 10,
		RetryOnInvalidJSON:    true,
		SceneSummaryMaxTokens: 500,
	}

	curator := NewMemoryCuratorService(llmClient, mockCellRepo, mockSceneRepo, config)

	t.Run("extract_cells_from_real_interaction", func(t *testing.T) {
		interaction := InteractionContext{
			ConversationID: "conv-integration-test",
			UserMessage:    "I've decided to use Go for my new microservice, and I prefer using Clean Architecture with TDD. Also, I want to deploy to AWS using Docker containers.",
			AssistantReply: "Great choices! Go is excellent for microservices with its performance and concurrency. Clean Architecture will help maintain separation of concerns, and TDD ensures code quality. Docker containers on AWS provide scalable deployment options.",
			MessageIDs:     []string{"msg-test-1", "msg-test-2"},
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		result, err := curator.ExtractCells(ctx, interaction)
		if err != nil {
			t.Fatalf("Expected successful extraction, got error: %v", err)
		}

		if result.CellsCreated == 0 {
			t.Error("Expected at least 1 cell to be extracted")
		}

		t.Logf("Extracted %d cells, updated %d scenes", result.CellsCreated, result.ScenesUpdated)

		// Verify cells have valid data
		for i, cell := range mockCellRepo.Cells {
			t.Logf("Cell %d: scene=%s, type=%s, salience=%.2f, content=%s",
				i+1, cell.Scene, cell.CellType.String(), cell.Salience, cell.Content)

			if cell.Scene == "" {
				t.Error("Cell should have a scene")
			}
			if cell.Content == "" {
				t.Error("Cell should have content")
			}
			if cell.Salience < 0 || cell.Salience > 1.0 {
				t.Errorf("Cell salience should be between 0 and 1, got %.2f", cell.Salience)
			}
		}

		// Verify scenes were created
		if result.ScenesUpdated > 0 {
			if len(mockSceneRepo.Scenes) == 0 {
				t.Error("Expected scenes to be created")
			}

			for i, scene := range mockSceneRepo.Scenes {
				t.Logf("Scene %d: name=%s, summary=%s, tokens=%d",
					i+1, scene.Scene, scene.Summary, scene.TokenCount)

				if scene.Summary == "" {
					t.Error("Scene should have a summary")
				}
				if scene.TokenCount == 0 {
					t.Error("Scene should have a token count")
				}
			}
		}
	})

	t.Run("consolidate_scene_with_real_llm", func(t *testing.T) {
		// Reset repos
		mockCellRepo = &MockCellRepository{}
		mockSceneRepo = &MockSceneRepository{}
		curator = NewMemoryCuratorService(llmClient, mockCellRepo, mockSceneRepo, config)

		// Create test cells manually
		now := time.Now()
		testCells := []*domainmemory.MemoryCell{
			{
				ID:             "cell-1",
				ConversationID: "conv-test",
				Scene:          "project-architecture",
				CellType:       domainmemory.CellTypeDecision,
				Salience:       0.9,
				Content:        "User decided to use Clean Architecture pattern",
				Source:         `["msg-1"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "cell-2",
				ConversationID: "conv-test",
				Scene:          "project-architecture",
				CellType:       domainmemory.CellTypePreference,
				Salience:       0.85,
				Content:        "User prefers Test-Driven Development workflow",
				Source:         `["msg-2"]`,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}
		mockCellRepo.Cells = testCells

		ctx := context.Background()
		err := curator.ConsolidateScene(ctx, "project-architecture")
		if err != nil {
			t.Fatalf("Expected successful consolidation, got error: %v", err)
		}

		// Verify scene was created
		if len(mockSceneRepo.Scenes) != 1 {
			t.Errorf("Expected 1 scene, got %d", len(mockSceneRepo.Scenes))
		}

		if len(mockSceneRepo.Scenes) > 0 {
			scene := mockSceneRepo.Scenes[0]
			t.Logf("Consolidated scene: name=%s, summary=%s, tokens=%d",
				scene.Scene, scene.Summary, scene.TokenCount)

			if scene.Summary == "" {
				t.Error("Scene should have a summary")
			}
			if scene.TokenCount == 0 {
				t.Error("Scene should have a token count")
			}
			if scene.TokenCount > config.SceneSummaryMaxTokens {
				t.Errorf("Scene token count (%d) exceeds max (%d)", scene.TokenCount, config.SceneSummaryMaxTokens)
			}

			// Check that summary doesn't have ephemeral phrasing
			summary := scene.Summary
			if contains(summary, "recently") || contains(summary, "just") || contains(summary, "this week") {
				t.Logf("Warning: Scene summary may contain ephemeral phrasing: %s", summary)
			}
		}
	})
}

func TestMemoryCuratorService_Integration_ErrorHandling(t *testing.T) {
	skipIfNoAPIKey(t)

	llmClient := setupRealLLMClient(t)
	mockCellRepo := &MockCellRepository{
		CreateErr: domainmemory.ErrAlreadyExists,
	}
	mockSceneRepo := &MockSceneRepository{}

	config := CuratorConfig{
		Enabled:            true,
		ExtractionModel:    "claude-3-haiku-20240307",
		RetryOnInvalidJSON: false,
	}

	curator := NewMemoryCuratorService(llmClient, mockCellRepo, mockSceneRepo, config)

	t.Run("handles_repository_errors_gracefully", func(t *testing.T) {
		interaction := InteractionContext{
			ConversationID: "conv-error-test",
			UserMessage:    "I want to use PostgreSQL for my database",
			AssistantReply: "PostgreSQL is a great choice for relational data.",
			Timestamp:      time.Now(),
		}

		ctx := context.Background()
		result, err := curator.ExtractCells(ctx, interaction)

		// Should get an error because repository fails
		if err == nil {
			t.Error("Expected error due to repository failure")
		}

		// But result should still be returned with error details
		if result != nil && len(result.Errors) == 0 {
			t.Error("Expected errors to be recorded in result")
		}
	})
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
