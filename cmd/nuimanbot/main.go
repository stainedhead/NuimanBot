package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/gateway/slack"
	"nuimanbot/internal/adapter/gateway/telegram"
	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/audit"
	"nuimanbot/internal/infrastructure/cache"
	"nuimanbot/internal/infrastructure/crypto"
	"nuimanbot/internal/infrastructure/health"
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
	"nuimanbot/internal/usecase/skill/coding_agent"
	"nuimanbot/internal/usecase/skill/common"
	"nuimanbot/internal/usecase/skill/doc_summarize"
	"nuimanbot/internal/usecase/skill/executor"
	"nuimanbot/internal/usecase/skill/github"
	"nuimanbot/internal/usecase/skill/repo_search"
	"nuimanbot/internal/usecase/skill/summarize"
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
	HealthServer          *health.Server
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

	// Validate configuration on startup
	if err := config.Validate(cfg); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	slog.Info("Configuration validated successfully")

	// 2. Initialize Structured Logging
	logFormat := "json" // Production default
	if cfg.Server.Debug {
		logFormat = "text" // Human-readable for development
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

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(25)                 // Maximum number of open connections
	db.SetMaxIdleConns(5)                  // Maximum number of idle connections
	db.SetConnMaxLifetime(5 * time.Minute) // Maximum lifetime of a connection
	db.SetConnMaxIdleTime(1 * time.Minute) // Maximum idle time before closing
	slog.Info("Database connection pool configured",
		"max_open", 25,
		"max_idle", 5,
		"max_lifetime", "5m",
		"max_idle_time", "1m",
	)

	// Verify database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize database schema
	if err := initializeDatabase(db); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 5. Initialize Security Service with SQLite Auditor
	inputValidator := security.NewDefaultInputValidator()
	auditor, err := audit.NewSQLiteAuditor(db)
	if err != nil {
		log.Fatalf("Failed to create audit logger: %v", err)
	}
	securityService := security.NewService(vault, inputValidator, auditor)
	slog.Info("Security service initialized with SQLite auditor")

	// 6. Initialize Memory Repository
	memoryRepo := sqlite.NewMessageRepository(db)

	// 7. Initialize Notes Repository
	notesRepo := sqlite.NewNotesRepository(db)

	// 8. Initialize LLM Service
	llmService, err := initializeLLMService(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	// 8.5. Initialize Health Check Server
	healthServer := health.NewServer(db, llmService, vaultPath)
	healthServer.SetVersion("1.0.0") // TODO: Get from build info
	slog.Info("Health check server initialized")

	// 9. Initialize Skill System
	skillRegistry := skill.NewInMemoryRegistry()

	// Register built-in skills
	if err := registerBuiltInSkills(skillRegistry, notesRepo, llmService); err != nil {
		log.Fatalf("Failed to register skills: %v", err)
	}

	skillExecutionService := skill.NewService(&cfg.Skills, skillRegistry, securityService)

	// 10. Initialize Chat Service
	chatService := chat.NewService(llmService, memoryRepo, skillExecutionService, securityService)

	// Configure LLM response cache (optional)
	llmCache := cache.NewLLMCache(1000, 1*time.Hour) // Cache up to 1000 responses for 1 hour
	chatService.SetCache(llmCache)
	slog.Info("LLM response cache configured",
		"max_size", 1000,
		"ttl", "1h",
	)

	// 11. Create Application
	app := &application{
		Config:                cfg,
		Vault:                 vault,
		SecurityService:       securityService,
		Memory:                memoryRepo,
		LLMService:            llmService,
		SkillRegistry:         skillRegistry,
		ChatService:           chatService,
		SkillExecutionService: skillExecutionService,
		HealthServer:          healthServer,
		DB:                    db,
	}

	// 12. Run application in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Run(ctx)
	}()

	// 13. Wait for shutdown signal or error
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
		slog.Info("Initializing LLM provider", "provider", "openai", "source", "legacy_config")
		return openai.New(&cfg.LLM.OpenAI), nil
	}

	// Check Ollama
	if cfg.LLM.Ollama.BaseURL != "" {
		slog.Info("Initializing LLM provider", "provider", "ollama", "source", "legacy_config")
		return ollama.New(&cfg.LLM.Ollama), nil
	}

	// Check Anthropic
	if cfg.LLM.Anthropic.APIKey.Value() != "" {
		slog.Info("Initializing LLM provider", "provider", "anthropic", "source", "legacy_config")
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
func registerBuiltInSkills(registry skill.SkillRegistry, notesRepo *sqlite.NotesRepository, llmService domain.LLMService) error {
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
		slog.Info("Skill registered", "skill", "weather")
	} else {
		slog.Warn("Skill skipped", "skill", "weather", "reason", "OPENWEATHERMAP_API_KEY not set")
	}

	// Register WebSearch skill
	ws := websearch.NewWebSearch(10)
	if err := registry.Register(ws); err != nil {
		return fmt.Errorf("failed to register websearch skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "websearch")

	// Register Notes skill
	notesSkill := notes.NewNotes(notesRepo)
	if err := registry.Register(notesSkill); err != nil {
		return fmt.Errorf("failed to register notes skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "notes")

	// Register Developer Productivity Skills (Phase 5)
	if err := registerDeveloperProductivitySkills(registry, llmService); err != nil {
		return fmt.Errorf("failed to register developer productivity skills: %w", err)
	}

	slog.Info("Registered built-in skills successfully")
	return nil
}

// registerDeveloperProductivitySkills registers developer productivity skills.
func registerDeveloperProductivitySkills(registry skill.SkillRegistry, llmService domain.LLMService) error {
	// Create shared dependencies
	executorSvc := executor.NewExecutorService()
	rateLimiter := common.NewRateLimiter()
	sanitizer := common.NewOutputSanitizer()
	httpClient := &http.Client{Timeout: 60 * time.Second}

	// Default workspace paths (can be configured later)
	workspacePaths := []string{"."}
	if cwd, err := os.Getwd(); err == nil {
		workspacePaths = []string{cwd}
	}
	pathValidator := common.NewPathValidator(workspacePaths)

	// Register GitHubSkill
	githubConfig := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"timeout":     30,
			"rate_limit":  "30/minute",
		},
	}
	githubSkill := github.NewGitHubSkill(githubConfig, executorSvc, rateLimiter, sanitizer)
	if err := registry.Register(githubSkill); err != nil {
		return fmt.Errorf("failed to register github skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "github")

	// Register RepoSearchSkill
	repoSearchConfig := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_directories": workspacePaths,
		},
	}
	repoSearchSkill := repo_search.NewRepoSearchSkill(repoSearchConfig, executorSvc, pathValidator, sanitizer)
	if err := registry.Register(repoSearchSkill); err != nil {
		return fmt.Errorf("failed to register repo_search skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "repo_search")

	// Register DocSummarizeSkill
	docSummarizeConfig := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_domains":   []interface{}{"github.com", "docs.google.com", "notion.so"},
			"max_document_size": 5 * 1024 * 1024,
		},
	}
	docSummarizeSkill := doc_summarize.NewDocSummarizeSkill(docSummarizeConfig, llmService, httpClient)
	if err := registry.Register(docSummarizeSkill); err != nil {
		return fmt.Errorf("failed to register doc_summarize skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "doc_summarize")

	// Register SummarizeSkill
	summarizeConfig := domain.SkillConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"timeout":    90,
			"user_agent": "NuimanBot/1.0",
		},
	}
	summarizeSkill := summarize.NewSummarizeSkill(summarizeConfig, llmService, executorSvc, httpClient)
	if err := registry.Register(summarizeSkill); err != nil {
		return fmt.Errorf("failed to register summarize skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "summarize")

	// Register CodingAgentSkill
	codingAgentConfig := domain.SkillConfig{
		Enabled: false, // Admin must explicitly enable
		Params: map[string]interface{}{
			"allowed_tools":  []interface{}{"codex", "claude_code"},
			"default_mode":   "interactive",
			"pty_mode":       true,
		},
	}
	codingAgentSkill := coding_agent.NewCodingAgentSkill(codingAgentConfig, executorSvc, pathValidator)
	if err := registry.Register(codingAgentSkill); err != nil {
		return fmt.Errorf("failed to register coding_agent skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "coding_agent")

	slog.Info("Registered developer productivity skills successfully")
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
			tool_calls TEXT,
			tool_results TEXT,
			token_count INTEGER DEFAULT 0,
			timestamp TIMESTAMP NOT NULL,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
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

	// Create index on messages for efficient conversation message retrieval
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_messages_conversation_timestamp
		ON messages(conversation_id, timestamp)
	`)
	if err != nil {
		return fmt.Errorf("failed to create messages conversation index: %w", err)
	}

	// Create index on messages for efficient token-based retrieval (GetRecentMessages)
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_messages_conversation_tokens
		ON messages(conversation_id, timestamp DESC, token_count)
	`)
	if err != nil {
		return fmt.Errorf("failed to create messages token index: %w", err)
	}

	// Create index on conversations for efficient user conversation listing
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_conversations_user_updated
		ON conversations(user_id, updated_at DESC)
	`)
	if err != nil {
		return fmt.Errorf("failed to create conversations user index: %w", err)
	}

	// Create unique index on users for platform-specific user lookups
	// Note: This is redundant with the UNIQUE constraint in the table definition,
	// but explicit indexes can help with query planning
	_, err = db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_platform_uid
		ON users(platform, platform_uid)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users platform index: %w", err)
	}

	slog.Info("Database schema initialized successfully")
	return nil
}

// Run starts the main application services.
func (app *application) Run(ctx context.Context) error {
	// Start health check server on port 8080
	if err := app.HealthServer.Start(":8080"); err != nil {
		slog.Error("Failed to start health check server", "error", err)
	}
	defer func() {
		if err := app.HealthServer.Stop(); err != nil {
			slog.Error("Failed to stop health check server", "error", err)
		}
	}()

	// Track active gateways for proper shutdown
	var gateways []domain.Gateway

	// Initialize CLI gateway
	cliGateway := cli.NewGateway(&app.Config.Gateways.CLI)
	app.connectGateway(cliGateway)
	gateways = append(gateways, cliGateway) //nolint:staticcheck // Reserved for future shutdown handling
	_ = gateways                            // Prevent unused variable warning

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
				slog.Info("Starting gateway", "platform", "telegram")
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
			gateways = append(gateways, slackGateway) //nolint:ineffassign,staticcheck // Reserved for future shutdown handling

			// Start Slack gateway in background
			go func() {
				slog.Info("Starting gateway", "platform", "slack")
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
