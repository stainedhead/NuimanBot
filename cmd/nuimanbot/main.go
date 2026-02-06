package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
	anthropic "nuimanbot/internal/infrastructure/llm/anthropic"
	"nuimanbot/internal/skills/calculator"
	"nuimanbot/internal/skills/datetime"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/memory"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/skill"
)

// application represents the core NuimanBot application.
// It holds all the dependencies that different parts of the application need.
type application struct {
	Config                *config.NuimanBotConfig
	ChatService           *chat.Service
	LLMService            domain.LLMService
	Memory                memory.MemoryRepository
	SecurityService       *security.Service
	SkillRegistry         skill.SkillRegistry
	Vault                 domain.CredentialVault
	SkillExecutionService *skill.Service
	DB                    *sql.DB
}

func main() {
	fmt.Println("NuimanBot starting...")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize Credential Vault
	vaultPath := cfg.Security.VaultPath
	if vaultPath == "" {
		vaultPath = "./data/vault.enc" // Default path
	}
	vault, err := crypto.NewFileCredentialVault(vaultPath, []byte(cfg.Security.EncryptionKey))
	if err != nil {
		log.Fatalf("Failed to create credential vault: %v", err)
	}

	// 3. Initialize Security Service
	inputValidator := security.NewDefaultInputValidator()
	auditor := security.NewNoOpAuditor()
	securityService := security.NewService(vault, inputValidator, auditor)

	// 4. Initialize Database
	dbPath := cfg.Storage.DSN
	if dbPath == "" {
		dbPath = "./data/nuimanbot.db" // Default path
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize database schema
	if err := initializeDatabase(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 5. Initialize Memory Repository
	memoryRepo := sqlite.NewMessageRepository(db)

	// 6. Initialize LLM Service
	llmService, err := initializeLLMService(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	// 7. Initialize Skill System
	skillRegistry := skill.NewInMemoryRegistry()

	// Register built-in skills
	if err := registerBuiltInSkills(skillRegistry); err != nil {
		log.Fatalf("Failed to register skills: %v", err)
	}

	skillExecutionService := skill.NewService(&cfg.Skills, skillRegistry, securityService)

	// 8. Initialize Chat Service
	chatService := chat.NewService(llmService, memoryRepo, skillExecutionService, securityService)

	// 9. Create Application
	app := &application{
		Config:                cfg,
		Vault:                 vault,
		SecurityService:       securityService,
		Memory:                memoryRepo,
		LLMService:            llmService,
		SkillRegistry:         skillRegistry,
		ChatService:           chatService,
		SkillExecutionService: skillExecutionService,
		DB:                    db,
	}

	// 10. Run application in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Run(ctx)
	}()

	// 11. Wait for shutdown signal or error
	select {
	case <-sigChan:
		fmt.Println("\nReceived shutdown signal, stopping gracefully...")
		cancel()
	case err := <-errChan:
		if err != nil {
			log.Fatalf("NuimanBot stopped with error: %v", err)
		}
	}

	fmt.Println("NuimanBot stopped gracefully.")
}

// initializeLLMService initializes the LLM service based on configuration.
func initializeLLMService(cfg *config.NuimanBotConfig) (domain.LLMService, error) {
	if len(cfg.LLM.Providers) == 0 {
		return nil, fmt.Errorf("no LLM providers configured")
	}

	// For MVP, use the first provider
	// TODO: Implement provider selection based on default_model config
	provider := &cfg.LLM.Providers[0]

	switch provider.Type {
	case domain.LLMProviderAnthropic:
		return anthropic.NewClient(provider)
	case domain.LLMProviderOpenAI:
		return nil, fmt.Errorf("OpenAI provider not yet implemented")
	case domain.LLMProviderOllama:
		return nil, fmt.Errorf("Ollama provider not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider.Type)
	}
}

// registerBuiltInSkills registers all built-in skills with the registry.
func registerBuiltInSkills(registry skill.SkillRegistry) error {
	// Register Calculator skill
	calc := calculator.NewCalculator()
	if err := registry.Register(calc); err != nil {
		return fmt.Errorf("failed to register calculator skill: %w", err)
	}

	// Register DateTime skill
	dt := datetime.NewDateTime()
	if err := registry.Register(dt); err != nil {
		return fmt.Errorf("failed to register datetime skill: %w", err)
	}

	log.Printf("Registered %d built-in skills", 2)
	return nil
}

// initializeDatabase creates necessary tables if they don't exist.
func initializeDatabase(db *sql.DB) error {
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
			token_count INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

	log.Println("Database schema initialized successfully")
	return nil
}

// Run starts the main application services.
func (app *application) Run(ctx context.Context) error {
	// For MVP, we start the CLI gateway directly.
	cliGateway := cli.NewGateway(&app.Config.Gateways.CLI)

	// Connect CLI gateway to chat service
	cliGateway.OnMessage(func(msgCtx context.Context, msg domain.IncomingMessage) error {
		// Process message through chat service
		response, err := app.ChatService.ProcessMessage(msgCtx, &msg)
		if err != nil {
			log.Printf("Error processing message: %v", err)
			// Send error message back to user
			errorMsg := domain.OutgoingMessage{
				RecipientID: msg.PlatformUID,
				Content:     fmt.Sprintf("Error: %s", err.Error()),
				Format:      "text",
			}
			return cliGateway.Send(msgCtx, errorMsg)
		}

		// Send successful response
		return cliGateway.Send(msgCtx, response)
	})

	// Log startup information
	log.Printf("NuimanBot initialized with:")
	log.Printf("  Log Level: %s", app.Config.Server.LogLevel)
	log.Printf("  Debug Mode: %t", app.Config.Server.Debug)
	log.Printf("  LLM Provider: %s", app.Config.LLM.Providers[0].Type)
	log.Printf("  Skills Registered: %d", len(app.SkillRegistry.List()))

	fmt.Println("\nStarting CLI Gateway...")
	fmt.Println("Type your messages below. Commands:")
	fmt.Println("  - Type 'exit' or 'quit' to stop")
	fmt.Println("  - Type 'help' for available skills")
	fmt.Println()

	// Start CLI gateway (blocks until shutdown)
	return cliGateway.Start(ctx)
}
