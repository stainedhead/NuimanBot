package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
	"nuimanbot/internal/skills/calculator"
	"nuimanbot/internal/skills/datetime"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/memory"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/skill"
)

// testApplication represents a fully-initialized NuimanBot application for testing.
type testApplication struct {
	Config                *config.NuimanBotConfig
	ChatService           *chat.Service
	LLMService            domain.LLMService
	Memory                memory.MemoryRepository
	SecurityService       *security.Service
	SkillRegistry         skill.SkillRegistry
	Vault                 domain.CredentialVault
	SkillExecutionService *skill.Service
	DB                    *sql.DB
	CLIGateway            *cli.Gateway
	TempDir               string
}

// mockLLMService implements domain.LLMService for testing without real API calls.
type mockLLMService struct {
	responses map[string]string
	callCount int
}

func newMockLLMService() *mockLLMService {
	return &mockLLMService{
		responses: make(map[string]string),
		callCount: 0,
	}
}

func (m *mockLLMService) Complete(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (*domain.LLMResponse, error) {
	m.callCount++

	// Default behavior: echo the last user message
	var lastUserMsg string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			lastUserMsg = msg.Content
		}
	}

	// Check for mock responses
	if response, ok := m.responses[lastUserMsg]; ok {
		return &domain.LLMResponse{
			Content:      response,
			Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			FinishReason: "stop",
		}, nil
	}

	// Default: return a simple echo
	return &domain.LLMResponse{
		Content:      fmt.Sprintf("Mock LLM received: %s", lastUserMsg),
		Usage:        domain.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		FinishReason: "stop",
	}, nil
}

func (m *mockLLMService) SetResponse(input, output string) {
	m.responses[input] = output
}

func (m *mockLLMService) Stream(ctx context.Context, provider domain.LLMProvider, req *domain.LLMRequest) (<-chan domain.StreamChunk, error) {
	// Not implemented for E2E tests
	return nil, fmt.Errorf("streaming not implemented in mock")
}

func (m *mockLLMService) ListModels(ctx context.Context, provider domain.LLMProvider) ([]domain.ModelInfo, error) {
	// Return mock models
	return []domain.ModelInfo{
		{ID: "mock-model", Name: "Mock Model", Provider: "mock", ContextWindow: 100000},
	}, nil
}

// setupTestApp creates a fully initialized test application with all layers.
func setupTestApp(t *testing.T) (*testApplication, func()) {
	t.Helper()

	// Create temp directory for test data
	tempDir := t.TempDir()

	// Copy test config to temp directory
	testConfigPath := filepath.Join(tempDir, "config.yaml")
	configContent, err := os.ReadFile("testdata/config.yaml")
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}
	if err := os.WriteFile(testConfigPath, configContent, 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set test encryption key
	testEncKey := "12345678901234567890123456789012" // 32 bytes
	if err := os.Setenv("NUIMANBOT_ENCRYPTION_KEY", testEncKey); err != nil {
		t.Fatalf("Failed to set encryption key env var: %v", err)
	}

	// Set test LLM API key
	if err := os.Setenv("NUIMANBOT_LLM_PROVIDERS_0_APIKEY", "sk-test-key-e2e"); err != nil {
		t.Fatalf("Failed to set LLM API key env var: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Override paths to use temp directory
	vaultPath := filepath.Join(tempDir, "vault.enc")
	cfg.Security.VaultPath = vaultPath
	cfg.Security.EncryptionKey = testEncKey

	dbPath := filepath.Join(tempDir, "test.db")
	cfg.Storage.DSN = dbPath

	// Initialize credential vault
	vault, err := crypto.NewFileCredentialVault(vaultPath, []byte(testEncKey))
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Initialize security service
	inputValidator := security.NewDefaultInputValidator()
	auditor := security.NewNoOpAuditor()
	securityService := security.NewService(vault, inputValidator, auditor)

	// Initialize database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize database schema
	if err := initializeTestDatabase(db); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize memory repository
	memoryRepo := sqlite.NewMessageRepository(db)

	// Initialize mock LLM service
	llmService := newMockLLMService()

	// Initialize skill system
	skillRegistry := skill.NewInMemoryRegistry()

	// Register built-in skills
	calc := calculator.NewCalculator()
	if err := skillRegistry.Register(calc); err != nil {
		t.Fatalf("Failed to register calculator skill: %v", err)
	}

	dt := datetime.NewDateTime()
	if err := skillRegistry.Register(dt); err != nil {
		t.Fatalf("Failed to register datetime skill: %v", err)
	}

	skillExecutionService := skill.NewService(&cfg.Skills, skillRegistry, securityService)

	// Initialize chat service
	chatService := chat.NewService(llmService, memoryRepo, skillExecutionService, securityService)

	// Initialize CLI gateway
	cliGateway := cli.NewGateway(&cfg.Gateways.CLI)

	// Create test application
	app := &testApplication{
		Config:                cfg,
		Vault:                 vault,
		SecurityService:       securityService,
		Memory:                memoryRepo,
		LLMService:            llmService,
		SkillRegistry:         skillRegistry,
		ChatService:           chatService,
		SkillExecutionService: skillExecutionService,
		DB:                    db,
		CLIGateway:            cliGateway,
		TempDir:               tempDir,
	}

	// Cleanup function
	cleanup := func() {
		if db != nil {
			db.Close()
		}
		os.Unsetenv("NUIMANBOT_ENCRYPTION_KEY")
		os.Unsetenv("NUIMANBOT_LLM_PROVIDERS_0_APIKEY")
	}

	return app, cleanup
}

// initializeTestDatabase creates the database schema for testing.
func initializeTestDatabase(db *sql.DB) error {
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL,
			platform_uid TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(platform, platform_uid)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create messages table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			tool_calls TEXT,
			tool_results TEXT,
			token_count INTEGER DEFAULT 0,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %w", err)
	}

	// Create conversations table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create conversations table: %w", err)
	}

	return nil
}

// createTestMessage creates a test incoming message.
func createTestMessage(content string) domain.IncomingMessage {
	return domain.IncomingMessage{
		ID:          "test-msg-id",
		Platform:    domain.PlatformCLI,
		PlatformUID: "test-user-123",
		Text:        content,
	}
}
