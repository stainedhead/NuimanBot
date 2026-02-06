package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/gateway/slack"
	"nuimanbot/internal/adapter/gateway/telegram"
	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
	anthropic "nuimanbot/internal/infrastructure/llm/anthropic"
	ollama "nuimanbot/internal/infrastructure/llm/ollama"
	openai "nuimanbot/internal/infrastructure/llm/openai"
	"nuimanbot/internal/infrastructure/logger"
	"nuimanbot/internal/skills/calculator"
	"nuimanbot/internal/skills/datetime"
	"nuimanbot/internal/skills/notes"
	"nuimanbot/internal/skills/weather"
	"nuimanbot/internal/skills/websearch"
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

	// 2. Initialize Structured Logging
	logFormat := "text"
	if cfg.Server.Debug {
		logFormat = "text" // Human-readable for development
	} else {
		logFormat = "json" // JSON for production
	}
	logger.Initialize(logger.Config{
		Level:  logger.LogLevel(cfg.Server.LogLevel),
		Format: logFormat,
	})
	slog.Info("Logger initialized",
		"level", cfg.Server.LogLevel,
		"format", logFormat,
	)

	// 3. Initialize Credential Vault
	vaultPath := cfg.Security.VaultPath
	if vaultPath == "" {
		vaultPath = "./data/vault.enc" // Default path
	}
	vault, err := crypto.NewFileCredentialVault(vaultPath, []byte(cfg.Security.EncryptionKey))
	if err != nil {
		log.Fatalf("Failed to create credential vault: %v", err)
	}

	// 4. Initialize Security Service
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

	// 5.5. Initialize Notes Repository
	notesRepo := sqlite.NewNotesRepository(db)

	// 6. Initialize LLM Service
	llmService, err := initializeLLMService(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	// 7. Initialize Skill System
	skillRegistry := skill.NewInMemoryRegistry()

	// Register built-in skills
	if err := registerBuiltInSkills(skillRegistry, notesRepo); err != nil {
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

// connectGateway connects a gateway to the chat service
func (app *application) connectGateway(gw domain.Gateway) {
	gw.OnMessage(func(msgCtx context.Context, msg domain.IncomingMessage) error {
		// Process message through chat service
		response, err := app.ChatService.ProcessMessage(msgCtx, &msg)
		if err != nil {
			slog.Error("Error processing message",
				"platform", gw.Platform(),
				"error", err,
			)
			// Send error message back to user
			errorMsg := domain.OutgoingMessage{
				RecipientID: msg.PlatformUID,
				Content:     fmt.Sprintf("Error: %s", err.Error()),
				Format:      "text",
				Metadata:    msg.Metadata, // Preserve metadata for response routing
			}
			return gw.Send(msgCtx, errorMsg)
		}

		// Send successful response
		return gw.Send(msgCtx, response)
	})
}

// initializeLLMService initializes the LLM service based on configuration.
func initializeLLMService(cfg *config.NuimanBotConfig) (domain.LLMService, error) {
	// Try provider-specific configs first (new way)
	// Check OpenAI
	if cfg.LLM.OpenAI.APIKey.Value() != "" {
		log.Println("Initializing OpenAI LLM provider")
		return openai.New(&cfg.LLM.OpenAI), nil
	}

	// Check Ollama
	if cfg.LLM.Ollama.BaseURL != "" {
		log.Println("Initializing Ollama LLM provider")
		return ollama.New(&cfg.LLM.Ollama), nil
	}

	// Check Anthropic
	if cfg.LLM.Anthropic.APIKey.Value() != "" {
		log.Println("Initializing Anthropic LLM provider")
		// Convert to generic provider config for Anthropic
		providerCfg := &config.LLMProviderConfig{
			Type:   domain.LLMProviderAnthropic,
			APIKey: cfg.LLM.Anthropic.APIKey,
		}
		return anthropic.NewClient(providerCfg)
	}

	// Fallback to generic Providers array (old way)
	if len(cfg.LLM.Providers) > 0 {
		// For MVP, use the first provider
		// TODO: Implement provider selection based on default_model config
		provider := &cfg.LLM.Providers[0]

		switch provider.Type {
		case domain.LLMProviderAnthropic:
			slog.Info("Initializing LLM provider", "provider", "anthropic", "source", "providers_array")
			return anthropic.NewClient(provider)
		case domain.LLMProviderOpenAI:
			slog.Info("Initializing LLM provider", "provider", "openai", "source", "providers_array")
			// Convert generic provider config to OpenAI-specific config
			openaiCfg := &config.OpenAIProviderConfig{
				APIKey:  provider.APIKey,
				BaseURL: provider.BaseURL,
			}
			return openai.New(openaiCfg), nil
		case domain.LLMProviderOllama:
			slog.Info("Initializing LLM provider", "provider", "ollama", "source", "providers_array")
			// Ollama doesn't need API key, just BaseURL
			ollamaCfg := &config.OllamaProviderConfig{
				BaseURL: provider.BaseURL,
			}
			if ollamaCfg.BaseURL == "" {
				ollamaCfg.BaseURL = "http://localhost:11434" // Default Ollama URL
			}
			return ollama.New(ollamaCfg), nil
		default:
			return nil, fmt.Errorf("unsupported LLM provider: %s", provider.Type)
		}
	}

	return nil, fmt.Errorf("no LLM providers configured (set llm.openai.api_key, llm.ollama.base_url, or llm.anthropic.api_key)")
}

// registerBuiltInSkills registers all built-in skills with the registry.
func registerBuiltInSkills(registry skill.SkillRegistry, notesRepo *sqlite.NotesRepository) error {
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

	// Register Weather skill (if API key is available)
	weatherAPIKey := os.Getenv("OPENWEATHERMAP_API_KEY")
	if weatherAPIKey != "" {
		w := weather.NewWeather(weatherAPIKey, 10)
		if err := registry.Register(w); err != nil {
			return fmt.Errorf("failed to register weather skill: %w", err)
		}
		log.Println("Weather skill registered")
	} else {
		log.Println("Weather skill skipped (OPENWEATHERMAP_API_KEY not set)")
	}

	// Register WebSearch skill
	ws := websearch.NewWebSearch(10)
	if err := registry.Register(ws); err != nil {
		return fmt.Errorf("failed to register websearch skill: %w", err)
	}
	log.Println("WebSearch skill registered")

	// Register Notes skill
	notesSkill := notes.NewNotes(notesRepo)
	if err := registry.Register(notesSkill); err != nil {
		return fmt.Errorf("failed to register notes skill: %w", err)
	}
	log.Println("Notes skill registered")

	slog.Info("Registered built-in skills successfully")
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

	// Create notes table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			tags TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create notes table: %w", err)
	}

	// Create index on user_id for faster note lookups
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes(user_id)
	`)
	if err != nil {
		return fmt.Errorf("failed to create notes index: %w", err)
	}

	log.Println("Database schema initialized successfully")
	return nil
}

// Run starts the main application services.
func (app *application) Run(ctx context.Context) error {
	// Track active gateways for proper shutdown
	var gateways []domain.Gateway

	// Initialize CLI gateway
	cliGateway := cli.NewGateway(&app.Config.Gateways.CLI)
	app.connectGateway(cliGateway)
	gateways = append(gateways, cliGateway)

	// Initialize Telegram gateway if enabled
	if app.Config.Gateways.Telegram.Enabled {
		telegramGateway, err := telegram.New(&app.Config.Gateways.Telegram)
		if err != nil {
			slog.Warn("Failed to create Telegram gateway", "error", err)
		} else {
			app.connectGateway(telegramGateway)
			gateways = append(gateways, telegramGateway)

			// Start Telegram gateway in background
			go func() {
				log.Println("Starting Telegram gateway...")
				if err := telegramGateway.Start(ctx); err != nil {
					slog.Error("Telegram gateway error", "error", err)
				}
			}()
		}
	}

	// Initialize Slack gateway if enabled
	if app.Config.Gateways.Slack.Enabled {
		slackGateway, err := slack.New(&app.Config.Gateways.Slack)
		if err != nil {
			slog.Warn("Failed to create Slack gateway", "error", err)
		} else {
			app.connectGateway(slackGateway)
			gateways = append(gateways, slackGateway)

			// Start Slack gateway in background
			go func() {
				log.Println("Starting Slack gateway...")
				if err := slackGateway.Start(ctx); err != nil {
					slog.Error("Slack gateway error", "error", err)
				}
			}()
		}
	}

	// Log startup information
	slog.Info("NuimanBot initialized",
		"log_level", app.Config.Server.LogLevel,
		"debug_mode", app.Config.Server.Debug,
		"llm_provider", app.Config.LLM.Providers[0].Type,
		"skills_registered", len(app.SkillRegistry.List()),
	)

	fmt.Println("\nStarting CLI Gateway...")
	fmt.Println("Type your messages below. Commands:")
	fmt.Println("  - Type 'exit' or 'quit' to stop")
	fmt.Println("  - Type 'help' for available skills")
	fmt.Println()

	// Start CLI gateway (blocks until shutdown)
	return cliGateway.Start(ctx)
}
