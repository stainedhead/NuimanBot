package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nuimanbot/internal/adapter/api"
	cliadapter "nuimanbot/internal/adapter/cli"
	memoryfactory "nuimanbot/internal/adapter/factory"
	"nuimanbot/internal/adapter/gateway/buzz"
	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/gateway/slack"
	"nuimanbot/internal/adapter/gateway/telegram"
	mcpadapter "nuimanbot/internal/adapter/mcp"
	"nuimanbot/internal/adapter/web"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/domain/memoryv2"
	"nuimanbot/internal/infrastructure/alerting"
	"nuimanbot/internal/infrastructure/cache"
	infraconfig "nuimanbot/internal/infrastructure/config"
	"nuimanbot/internal/infrastructure/crypto"
	"nuimanbot/internal/infrastructure/health"
	anthropic "nuimanbot/internal/infrastructure/llm/anthropic"
	bedrock "nuimanbot/internal/infrastructure/llm/bedrock"
	ollama "nuimanbot/internal/infrastructure/llm/ollama"
	openai "nuimanbot/internal/infrastructure/llm/openai"
	"nuimanbot/internal/infrastructure/logger"
	inframcp "nuimanbot/internal/infrastructure/mcp"
	personainfra "nuimanbot/internal/infrastructure/persona"
	infrasecurity "nuimanbot/internal/infrastructure/security"
	skillinfra "nuimanbot/internal/infrastructure/skill"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/tools/calculator"
	"nuimanbot/internal/tools/datetime"
	"nuimanbot/internal/tools/notes"
	"nuimanbot/internal/tools/weather"
	"nuimanbot/internal/tools/websearch"
	"nuimanbot/internal/usecase/botmgmt"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/chats"
	usecaseconfig "nuimanbot/internal/usecase/config"
	llm "nuimanbot/internal/usecase/llm"
	"nuimanbot/internal/usecase/memory"
	memoryv2uc "nuimanbot/internal/usecase/memoryv2"
	"nuimanbot/internal/usecase/profile"
	"nuimanbot/internal/usecase/security"
	skillusecase "nuimanbot/internal/usecase/skill"
	"nuimanbot/internal/usecase/tool"
	"nuimanbot/internal/usecase/tool/coding_agent"
	"nuimanbot/internal/usecase/tool/common"
	"nuimanbot/internal/usecase/tool/doc_summarize"
	"nuimanbot/internal/usecase/tool/executor"
	"nuimanbot/internal/usecase/tool/github"
	"nuimanbot/internal/usecase/tool/repo_search"
	"nuimanbot/internal/usecase/tool/summarize"
	"nuimanbot/internal/usecase/user"
)

// application represents the core NuimanBot application.
// It holds all the dependencies that different parts of the application need.
type application struct {
	Config               *config.NuimanBotConfig
	ConfigManager        *usecaseconfig.ConfigManager
	ChatService          *chat.Service
	LLMService           domain.LLMService
	Memory               memory.MemoryRepository
	SecurityService      *security.Service
	ToolRegistry         tool.ToolRegistry
	Vault                domain.CredentialVault
	ToolExecutionService *tool.Service
	DomainUserService    *user.Service // Resolves/creates the domain.User used for RBAC (FR-006, FR-011), shared by ChatService and the Buzz gateway
	HealthServer         *health.Server
	WebServer            *web.Server
	RESTServer           *api.Server                    // REST API server (optional)
	MemoryCellRepo       memoryv2.MemoryCellRepository  // Memory v2 cell repository
	MemorySceneRepo      memoryv2.MemorySceneRepository // Memory v2 scene repository
	MemoryAdmin          cliadapter.MemoryAdmin         // Optional admin operations for memory CLI
	ConfirmationStore    security.ConfirmationStore     // Part C confirmation store, shared with web/REST UI wiring (P5.8/P5.9)
	ConversationRepo     domain.ConversationRepository  // Backs the web admin's Chats environment (specs/260805-nuimanbot-extend-context-and-ui)
}

func main() {
	fmt.Println("NuimanBot starting...")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Initialize encryption key if not set (first-time setup)
	if err := ensureEncryptionKey(); err != nil {
		log.Fatalf("Failed to initialize encryption key: %v", err)
	}

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

	// 2.5. Initialize Alerting System
	alertingCfg := buildAlertingConfig(cfg)
	if err := alerting.Initialize(alertingCfg); err != nil {
		log.Fatalf("Failed to initialize alerting: %v", err)
	}
	defer func() {
		if err := alerting.Shutdown(); err != nil {
			slog.Error("Failed to shutdown alerting", "error", err)
		}
	}()

	// 3. Initialize Credential Vault
	vaultPath := cfg.Security.VaultPath
	if vaultPath == "" {
		vaultPath = "./data/vault.enc" // Default path
	}
	vaultKey, err := crypto.DecodeKeyFromBase64(cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to decode encryption key: %v", err)
	}
	vault, err := crypto.NewFileCredentialVault(vaultPath, vaultKey)
	if err != nil {
		log.Fatalf("Failed to create credential vault: %v", err)
	}

	// 4. Initialize File-Based Storage
	storagePath := cfg.Storage.DSN
	if storagePath == "" {
		storagePath = "./data" // Default path
	}

	// 4.1. Auto-initialize storage (create directories and default admin)
	if err := storage.Initialize(storagePath); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	fileRepos, err := initializeFileStorage(storagePath, cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}

	// 5. Initialize Security Service with File-Based Auditor
	inputValidator := security.NewDefaultInputValidator()
	auditAdapter := &auditRepositoryAdapter{repo: fileRepos.Audit}
	securityService := security.NewService(vault, inputValidator, auditAdapter)
	slog.Info("Security service initialized with file-based auditor")

	// 5.5. Initialize domain.User resolution service. ChatService uses this to
	// resolve/create a RBAC-relevant domain.User for every incoming message,
	// on every platform (FR-006, FR-011) — not just Buzz, which was
	// previously the only caller of this pattern. Backed by a dedicated file
	// (domain_users.json, distinct from users.json's domain.UserProfile admin
	// data, a different schema).
	domainUserRepo := storage.NewFileUserRepository(storagePath + "/domain_users.json")
	domainUserService := user.NewService(domainUserRepo, securityService)

	// 5.6. Build the shared OutputValidator used by every tool that returns
	// third-party-controlled content (summarize, doc_summarize, websearch, MCP
	// tools) to scan for prompt-injection patterns before that content re-enters
	// the LLM's conversation loop. Fail-closed default: enabled, action=reject.
	outputValidator := buildOutputValidator(cfg.Security.ToolOutputValidation)
	slog.Info("Tool-output injection validator initialized",
		"enabled", cfg.Security.ToolOutputValidation.IsEnabled(),
		"action", cfg.Security.ToolOutputValidation.ResolvedAction(),
	)

	// 6. Initialize Memory Repository
	conversationAdapter := &conversationRepositoryAdapter{repo: fileRepos.Conversation}

	// 7. Initialize Notes Repository
	notesRepo := fileRepos.Notes

	// 8. Initialize LLM Service
	llmService, err := initializeLLMService(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	// 8.5. Initialize Health Check Server
	healthServer := health.NewServer(nil, llmService, vaultPath)
	healthServer.SetVersion("1.0.0") // TODO: Get from build info
	slog.Info("Health check server initialized")

	// 9. Initialize Skill System
	toolRegistry := tool.NewInMemoryRegistry()

	// Register built-in skills
	if err := registerBuiltInTools(toolRegistry, notesRepo, llmService, outputValidator, cfg.Security.Fetch); err != nil {
		log.Fatalf("Failed to register skills: %v", err)
	}

	// 9.5. Register MCP tools from mcp.json (if MCP client is enabled)
	if err := registerMCPTools(ctx, cfg, toolRegistry, outputValidator); err != nil {
		// Non-fatal: MCP errors are logged inside registerMCPTools; we carry on.
		slog.Warn("MCP tool registration encountered errors", "error", err)
	}

	toolExecutionService := tool.NewService(&cfg.Tools, toolRegistry, securityService)
	toolExecutionService.SetConfirmationConfig(cfg.Security.Confirmation)

	// 10. Initialize Chat Service
	chatService := chat.NewService(llmService, conversationAdapter, toolExecutionService, securityService, domainUserService)

	// 9.6. Wire Part C's side-effecting-action confirmation gate
	// (specs/260802-improve-nuimanbot-security, FR-009..FR-015). The same
	// FileConfirmationStore instance must be shared by ToolExecutionService
	// (which creates/checks confirmations when executing a flagged
	// tool/action) and ChatService (which detects and resolves
	// confirmation-reply messages — see ChatService.SetConfirmationStore's
	// doc comment). wireConfirmationStore (specs/260803-improve-nuimanbot-
	// security-auto-review, FR-014/P3.4) sets both services from this single
	// instance and fails fast rather than allowing one service to silently
	// end up unconfigured or pointed at a different instance.
	confirmationStorePath := storagePath + "/confirmations.json"
	confirmationStore := infrasecurity.NewFileConfirmationStore(confirmationStorePath, cfg.Security.Confirmation.ResolvedTimeout())
	if err := wireConfirmationStore(chatService, toolExecutionService, confirmationStore); err != nil {
		log.Fatalf("Failed to wire confirmation store: %v", err)
	}
	slog.Info("Side-effecting action confirmation gate initialized",
		"enabled", cfg.Security.Confirmation.IsEnabled(),
		"timeout", cfg.Security.Confirmation.ResolvedTimeout(),
		"default_required_actions", cfg.Security.Confirmation.DefaultRequiredActions,
	)

	// Configure LLM defaults from config
	chatService.SetLLMDefaults(chat.LLMDefaults{
		Model:       cfg.LLM.DefaultModel.Primary,
		MaxTokens:   cfg.LLM.DefaultModel.MaxTokens,
		Temperature: cfg.LLM.DefaultModel.Temperature,
	})

	// Configure LLM response cache (optional)
	llmCache := cache.NewLLMCache(1000, 1*time.Hour) // Cache up to 1000 responses for 1 hour
	chatService.SetCache(llmCache)
	slog.Info("LLM response cache configured",
		"max_size", 1000,
		"ttl", "1h",
	)

	// 10.4. Initialize Persona Customization System
	personaBasePath := os.ExpandEnv("${HOME}/.nuimanbot/personas")
	if envPath := os.Getenv("NUIMANBOT_PERSONA_PATH"); envPath != "" {
		personaBasePath = envPath
	}

	personaRepo := personainfra.NewFileRepository(personaBasePath)
	personaParser := personainfra.NewRulesParser()

	// Global admin policy (customize per organization)
	var adminPolicy *domain.RulesConfig
	// Example: Block dangerous tools globally
	// adminPolicy = &domain.RulesConfig{
	// 	BlockedTools: []string{"production_deploy", "database_migration"},
	// 	RequiresConfirmation: []string{"external_api", "filesystem_delete"},
	// }

	globalSystemPrompt := "You are a helpful AI assistant." // Default fallback
	promptComposer := personainfra.NewPromptComposerAdapter(personaRepo, globalSystemPrompt)
	chatService.SetPromptComposer(promptComposer)
	slog.Info("Persona customization enabled",
		"base_path", personaBasePath,
		"admin_policy_enabled", adminPolicy != nil,
	)

	// Wire up rules enforcement in tool execution
	rulesEnforcer := personainfra.NewRulesEnforcerAdapter(personaRepo, personaParser, adminPolicy)
	toolExecutionService.SetRulesEnforcer(rulesEnforcer)
	slog.Info("Persona rules enforcement enabled")

	// 10.5. Initialize Memory v2 (Self-Organizing Memory)
	// Backend is selected by cfg.Memory.Backend: "builtin" or "ingatan".
	// BuildMemoryRepositoriesWithFallback performs a health probe for Ingatan and
	// falls back to built-in storage if the probe fails and fallback_to_builtin is set.
	memoryCellRepo, memorySceneRepo, err := memoryfactory.BuildMemoryRepositoriesWithFallback(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize memory repositories: %v", err)
	}
	slog.Info("Memory v2 repositories initialized", "backend", cfg.Memory.Backend)

	// Initialize MemoryRecallService (no LLM dependency)
	recallConfig := memoryv2uc.RecallConfig{
		FTSResultLimit:    20,
		SalienceThreshold: 0.5,
		FallbackCellLimit: 10,
		MaxScenes:         10,
		TokenBudget:       2000,
	}
	recallService := memoryv2uc.NewMemoryRecallService(memoryCellRepo, memorySceneRepo, recallConfig)
	chatService.SetMemoryRecaller(&memoryRecallerAdapter{recaller: recallService})

	// Initialize MemoryCuratorService (requires LLM)
	curatorConfig := memoryv2uc.CuratorConfig{
		Enabled:               true,
		ExtractionModel:       "claude-3-haiku-20240307",
		ConsolidationModel:    "claude-3-haiku-20240307",
		MaxCellsPerExtraction: 5,
		RetryOnInvalidJSON:    false,
		SceneSummaryMaxTokens: 500,
	}
	llmAdapter := memoryv2uc.NewLLMServiceAdapter(llmService, domain.LLMProviderAnthropic, curatorConfig.ExtractionModel)
	curatorService := memoryv2uc.NewMemoryCuratorService(llmAdapter, memoryCellRepo, memorySceneRepo, curatorConfig)
	chatService.SetMemoryCurator(&memoryCuratorAdapter{curator: curatorService})

	slog.Info("Memory v2 (self-organizing memory) initialized",
		"backend", cfg.Memory.Backend,
		"curator_enabled", curatorConfig.Enabled,
		"recall_fts_limit", recallConfig.FTSResultLimit,
		"recall_token_budget", recallConfig.TokenBudget,
	)

	// 10.6. Initialize Config Hot-Reload Manager
	configLoader := infraconfig.NewViperConfigLoaderAdapter()
	configManager := usecaseconfig.NewConfigManager(cfg, configLoader, securityService)
	slog.Info("Config hot-reload manager initialized")

	// 11. Create Application
	// Build memory admin using concrete file-based types (optional — non-fatal if unavailable).
	// Only available when the memory backend is file-based (builtin). Ingatan does not support admin ops.
	var memoryAdmin cliadapter.MemoryAdmin
	if fileMemCellRepo, ok := memoryCellRepo.(*storage.FileMemoryCellRepository); ok {
		if fileMemSceneRepo, ok := memorySceneRepo.(*storage.FileMemorySceneRepository); ok {
			memoryAdmin = storage.NewFileMemoryAdmin(fileMemCellRepo, fileMemSceneRepo, storagePath)
		}
	}

	// Build REST API server if enabled.
	var restServer *api.Server
	if cfg.ExternalAPI.REST.Enabled {
		jwtSecret := cfg.Security.EncryptionKey // Use app encryption key as JWT signing secret.
		var err error
		// confirmationStore/chatService back the REST confirmation endpoints
		// (GET/POST /api/v1/confirmations/{id}[/resolve], Part C / FR-011,
		// P5.9) — chatService.ResolveConfirmation satisfies
		// api.ConfirmationResolver.
		restServer, err = api.NewServer(cfg.ExternalAPI.REST, jwtSecret, confirmationStore, chatService)
		if err != nil {
			slog.Error("REST API configuration error", "error", err)
			os.Exit(1)
		}
		slog.Info("REST API server configured", "port", cfg.ExternalAPI.REST.Port)
	}

	app := &application{
		Config:               cfg,
		ConfigManager:        configManager,
		Vault:                vault,
		SecurityService:      securityService,
		Memory:               conversationAdapter,
		LLMService:           llmService,
		ToolRegistry:         toolRegistry,
		ChatService:          chatService,
		ToolExecutionService: toolExecutionService,
		DomainUserService:    domainUserService,
		HealthServer:         healthServer,
		RESTServer:           restServer,
		MemoryCellRepo:       memoryCellRepo,
		MemorySceneRepo:      memorySceneRepo,
		MemoryAdmin:          memoryAdmin,
		ConfirmationStore:    confirmationStore,
		ConversationRepo:     fileRepos.Conversation,
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

// initializeLLMService initializes the LLM orchestration service, registering all configured providers.
func initializeLLMService(cfg *config.NuimanBotConfig) (domain.LLMService, error) {
	svc := llm.NewService(&cfg.LLM)
	registered := 0

	// Register legacy provider-specific configs
	if cfg.LLM.OpenAI.APIKey.Value() != "" {
		svc.RegisterProviderClient(domain.LLMProviderOpenAI, openai.New(&cfg.LLM.OpenAI))
		slog.Info("LLM provider registered", "provider", "openai", "source", "legacy_config")
		registered++
	}
	if cfg.LLM.Ollama.BaseURL != "" {
		svc.RegisterProviderClient(domain.LLMProviderOllama, ollama.New(&cfg.LLM.Ollama))
		slog.Info("LLM provider registered", "provider", "ollama", "source", "legacy_config")
		registered++
	}
	if cfg.LLM.Anthropic.APIKey.Value() != "" {
		providerCfg := &config.LLMProviderConfig{
			Type:   domain.LLMProviderAnthropic,
			APIKey: cfg.LLM.Anthropic.APIKey,
		}
		client, err := anthropic.NewClient(providerCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic client: %w", err)
		}
		svc.RegisterProviderClient(domain.LLMProviderAnthropic, client)
		slog.Info("LLM provider registered", "provider", "anthropic", "source", "legacy_config")
		registered++
	}
	if cfg.LLM.Bedrock.AWSRegion != "" {
		client, err := bedrock.NewClient(&cfg.LLM.Bedrock)
		if err != nil {
			return nil, fmt.Errorf("failed to create bedrock client: %w", err)
		}
		svc.RegisterProviderClient(domain.LLMProviderBedrock, client)
		slog.Info("LLM provider registered", "provider", "bedrock", "source", "legacy_config")
		registered++
	}

	// Register providers from Providers array (skip duplicates)
	for i := range cfg.LLM.Providers {
		p := &cfg.LLM.Providers[i]
		if _, err := svc.GetClientForProvider(p.Type); err == nil {
			continue
		}
		switch p.Type {
		case domain.LLMProviderAnthropic:
			client, err := anthropic.NewClient(p)
			if err != nil {
				return nil, fmt.Errorf("failed to create anthropic client: %w", err)
			}
			svc.RegisterProviderClient(domain.LLMProviderAnthropic, client)
		case domain.LLMProviderOpenAI:
			svc.RegisterProviderClient(domain.LLMProviderOpenAI, openai.New(&config.OpenAIProviderConfig{APIKey: p.APIKey, BaseURL: p.BaseURL}))
		case domain.LLMProviderOllama:
			ollamaCfg := &config.OllamaProviderConfig{BaseURL: p.BaseURL}
			if ollamaCfg.BaseURL == "" {
				ollamaCfg.BaseURL = "http://localhost:11434"
			}
			svc.RegisterProviderClient(domain.LLMProviderOllama, ollama.New(ollamaCfg))
		case domain.LLMProviderBedrock:
			client, err := bedrock.NewClient(&config.BedrockProviderConfig{AWSRegion: p.BaseURL})
			if err != nil {
				return nil, fmt.Errorf("failed to create bedrock client: %w", err)
			}
			svc.RegisterProviderClient(domain.LLMProviderBedrock, client)
		default:
			slog.Warn("Unsupported LLM provider, skipping", "type", p.Type)
			continue
		}
		slog.Info("LLM provider registered", "provider", p.Type, "id", p.ID, "source", "providers_array")
		registered++
	}

	if registered == 0 {
		return nil, fmt.Errorf("no LLM providers configured")
	}

	if dp, err := svc.DefaultProvider(); err == nil {
		slog.Info("LLM orchestration initialized", "providers_registered", registered, "default_provider", dp, "default_model", cfg.LLM.DefaultModel.Primary)
	} else {
		slog.Info("LLM orchestration initialized", "providers_registered", registered)
	}

	return svc, nil
}

// wireConfirmationStore configures chatSvc and toolSvc to share exactly one
// ConfirmationStore instance for Part C's side-effecting-action confirmation
// flow (specs/260803-improve-nuimanbot-security-auto-review, FR-014/P3.4).
//
// Both services expose SetConfirmationStore as an independent optional
// setter, so nothing at the type level stops a future change from wiring
// only one of them, or wiring them to two different instances. Either
// mistake is dangerous in a subtle way: tool.Service stays fail-closed (no
// unconfirmed action executes) so nothing unsafe happens, but chat.Service's
// confirmation-reply detection silently no-ops — pending confirmations then
// sit until they expire on their own TTL, with no error surfaced to the
// operator or the user who thinks they replied "yes".
//
// wireConfirmationStore closes that gap by construction: callers pass one
// store value and this function is the only place that calls either
// service's SetConfirmationStore, so both services always receive the
// identical instance. It additionally fails fast (rather than silently
// leaving both services unconfigured) if store is nil, since a nil store
// is never a valid steady-state configuration for this feature — the
// caller should not construct a ConfirmationStore at all if the
// confirmation gate is meant to be disabled outright.
func wireConfirmationStore(chatSvc *chat.Service, toolSvc *tool.Service, store security.ConfirmationStore) error {
	if store == nil {
		return fmt.Errorf("confirmation store must not be nil: chat.Service and tool.Service both require the same non-nil ConfirmationStore instance for Part C's confirmation flow to work end-to-end")
	}
	if chatSvc == nil {
		return fmt.Errorf("chat.Service must not be nil when wiring confirmation store")
	}
	if toolSvc == nil {
		return fmt.Errorf("tool.Service must not be nil when wiring confirmation store")
	}

	toolSvc.SetConfirmationStore(store)
	chatSvc.SetConfirmationStore(store)
	return nil
}

// buildOutputValidator constructs the shared OutputValidator used by every tool
// that returns third-party-controlled content, honoring
// security.tool_output_validation.enabled/action. A disabled config returns a
// NoopOutputValidator so every call site can depend on a non-nil
// security.OutputValidator unconditionally.
func buildOutputValidator(cfg config.ToolOutputValidationConfig) security.OutputValidator {
	if !cfg.IsEnabled() {
		return security.NewNoopOutputValidator()
	}

	action := security.ValidationActionReject
	if cfg.ResolvedAction() == "annotate" {
		action = security.ValidationActionAnnotate
	}
	return security.NewDefaultOutputValidator(security.WithDefaultAction(action))
}

// buildFetchHTTPClient constructs the shared *http.Client used by the fetch
// tools (summarize, doc_summarize), honoring
// security.fetch.{ssrf_protection,follow_redirects}. Every redirect hop is
// re-validated against disallowed IP ranges and dialed at the exact resolved
// IP that was validated (closing the DNS-rebinding TOCTOU window) unless
// ssrf_protection is explicitly disabled; when follow_redirects is
// explicitly disabled, redirects are not followed at all — the 3xx response
// is returned to the caller as-is.
func buildFetchHTTPClient(timeout time.Duration, cfg config.FetchSecurityConfig) *http.Client {
	policy := common.FetchPolicy{
		SSRFProtection:  cfg.SSRFProtectionEnabled(),
		FollowRedirects: cfg.FollowRedirectsEnabled(),
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     common.NewSSRFSafeTransport(nil),
		CheckRedirect: common.NewCheckRedirect(policy, common.URLValidationOptions{}),
	}
}

// registerBuiltInTools registers all built-in skills with the registry.
func registerBuiltInTools(registry tool.ToolRegistry, notesRepo domain.NotesRepository, llmService domain.LLMService, outputValidator security.OutputValidator, fetchCfg config.FetchSecurityConfig) error {
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
	ws.SetOutputValidator(outputValidator)
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
	if err := registerDeveloperProductivityTools(registry, llmService, outputValidator, fetchCfg); err != nil {
		return fmt.Errorf("failed to register developer productivity skills: %w", err)
	}

	slog.Info("Registered built-in skills successfully")
	return nil
}

// registerDeveloperProductivityTools registers developer productivity skills.
func registerDeveloperProductivityTools(registry tool.ToolRegistry, llmService domain.LLMService, outputValidator security.OutputValidator, fetchCfg config.FetchSecurityConfig) error {
	// Create shared dependencies
	executorSvc := executor.NewExecutorService()
	rateLimiter := common.NewRateLimiter()
	sanitizer := common.NewOutputSanitizer()
	httpClient := buildFetchHTTPClient(60*time.Second, fetchCfg)

	// Default workspace paths (can be configured later)
	workspacePaths := []string{"."}
	if cwd, err := os.Getwd(); err == nil {
		workspacePaths = []string{cwd}
	}
	pathValidator := common.NewPathValidator(workspacePaths)

	// Register GitHubSkill
	githubConfig := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"timeout":    30,
			"rate_limit": "30/minute",
		},
	}
	githubSkill := github.NewGitHubSkill(githubConfig, executorSvc, rateLimiter, sanitizer)
	if err := registry.Register(githubSkill); err != nil {
		return fmt.Errorf("failed to register github skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "github")

	// Register RepoSearchSkill
	repoSearchConfig := domain.ToolConfig{
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
	docSummarizeConfig := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"allowed_domains":   []interface{}{"github.com", "docs.google.com", "notion.so"},
			"max_document_size": 5 * 1024 * 1024,
		},
	}
	docSummarizeSkill := doc_summarize.NewDocSummarizeSkill(docSummarizeConfig, llmService, httpClient)
	docSummarizeSkill.SetOutputValidator(outputValidator)
	docSummarizeSkill.SetSSRFProtection(fetchCfg.SSRFProtectionEnabled())
	if err := registry.Register(docSummarizeSkill); err != nil {
		return fmt.Errorf("failed to register doc_summarize skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "doc_summarize")

	// Register SummarizeSkill
	summarizeConfig := domain.ToolConfig{
		Enabled: true,
		Params: map[string]interface{}{
			"timeout":    90,
			"user_agent": "NuimanBot/1.0",
		},
	}
	summarizeSkill := summarize.NewSummarizeSkill(summarizeConfig, llmService, executorSvc, httpClient)
	summarizeSkill.SetOutputValidator(outputValidator)
	summarizeSkill.SetSSRFProtection(fetchCfg.SSRFProtectionEnabled())
	if err := registry.Register(summarizeSkill); err != nil {
		return fmt.Errorf("failed to register summarize skill: %w", err)
	}
	slog.Info("Skill registered", "skill", "summarize")

	// Register CodingAgentSkill
	codingAgentConfig := domain.ToolConfig{
		Enabled: false, // Admin must explicitly enable
		Params: map[string]interface{}{
			"allowed_tools": []interface{}{"codex", "claude_code"},
			"default_mode":  "interactive",
			"pty_mode":      true,
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

// registerMCPTools loads mcp.json and registers MCP tools from every configured
// server.  Individual server failures are logged and skipped (non-fatal).
func registerMCPTools(ctx context.Context, cfg *config.NuimanBotConfig, registry tool.ToolRegistry, outputValidator security.OutputValidator) error {
	if !cfg.MCP.Client.Enabled {
		slog.Info("MCP client disabled; skipping MCP tool registration")
		return nil
	}

	cfgFile := cfg.MCP.Client.ConfigFile
	if cfgFile == "" {
		cfgFile = "mcp.json"
	}

	mcpCfg, err := inframcp.LoadMCPConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("mcp: load config %q: %w", cfgFile, err)
	}

	if len(mcpCfg.Servers) == 0 {
		slog.Info("MCP: no servers configured", "config_file", cfgFile)
		return nil
	}

	slog.Info("MCP: registering tools", "config_file", cfgFile, "servers", len(mcpCfg.Servers))
	return mcpadapter.BuildMCPTools(ctx, *mcpCfg, registry, mcpadapter.WithOutputValidator(outputValidator))
}

// confirmationServiceAdapter adapts security.ConfirmationStore (for listing
// and fetching pending confirmations) and *chat.Service (for resolving them)
// to web.ConfirmationService — the web admin UI's confirmation page
// (specs/260802-improve-nuimanbot-security, Part C, task P5.8). Composing
// the two here (rather than making either type implement web.ConfirmationService
// directly) keeps that interface's dependency minimal and avoids a
// usecase-layer package depending on the adapter layer.
type confirmationServiceAdapter struct {
	store security.ConfirmationStore
	chat  *chat.Service
}

func (a *confirmationServiceAdapter) ListPendingConfirmations(ctx context.Context) ([]web.PendingConfirmation, error) {
	reqs, err := a.store.ListPending(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]web.PendingConfirmation, len(reqs))
	for i, r := range reqs {
		out[i] = confirmationRequestToWeb(r)
	}
	return out, nil
}

func (a *confirmationServiceAdapter) GetConfirmation(ctx context.Context, id string) (web.PendingConfirmation, error) {
	r, err := a.store.Get(ctx, id)
	if err != nil {
		return web.PendingConfirmation{}, err
	}
	return confirmationRequestToWeb(r), nil
}

func (a *confirmationServiceAdapter) ResolveConfirmation(ctx context.Context, confirmationID string, approved bool) error {
	_, err := a.chat.ResolveConfirmation(ctx, confirmationID, approved)
	return err
}

// confirmationRequestToWeb converts a usecase-layer security.ConfirmationRequest
// into the web adapter's simplified display type.
func confirmationRequestToWeb(r security.ConfirmationRequest) web.PendingConfirmation {
	return web.PendingConfirmation{
		ID:             r.ID,
		UserID:         r.UserID,
		ConversationID: r.ConversationID,
		ToolName:       r.ToolName,
		Action:         r.Action,
		Summary:        r.Summary,
		CreatedAt:      r.CreatedAt,
		ExpiresAt:      r.ExpiresAt,
	}
}

// memoryCuratorAdapter adapts MemoryCuratorService to chat.MemoryCurator interface.
type memoryCuratorAdapter struct {
	curator *memoryv2uc.MemoryCuratorService
}

func (a *memoryCuratorAdapter) ExtractMemoryCells(ctx context.Context, conversationID, userMessage, assistantReply string, toolOutputs []string) error {
	interaction := memoryv2uc.InteractionContext{
		ConversationID: conversationID,
		UserMessage:    userMessage,
		AssistantReply: assistantReply,
		ToolOutputs:    toolOutputs,
	}
	_, err := a.curator.ExtractCells(ctx, interaction)
	return err
}

// memoryRecallerAdapter adapts MemoryRecallService to chat.MemoryRecaller interface.
type memoryRecallerAdapter struct {
	recaller *memoryv2uc.MemoryRecallService
}

func (a *memoryRecallerAdapter) RecallAndFormat(ctx context.Context, conversationID, query string, maxTokens int) (string, error) {
	request := memoryv2uc.RecallRequest{
		ConversationID: conversationID,
		Query:          query,
		MaxTokens:      maxTokens,
	}
	response, err := a.recaller.RecallMemory(ctx, request)
	if err != nil {
		return "", err
	}
	return a.recaller.FormatMemoryForInjection(response), nil
}

// initializeMemoryV2Schema creates memory_cells, memory_scenes, and FTS5 tables.
// auditRepositoryAdapter adapts domain.AuditRepository to security.Auditor interface
type auditRepositoryAdapter struct {
	repo domain.AuditRepository
}

func (a *auditRepositoryAdapter) Audit(ctx context.Context, event *domain.AuditEvent) error {
	return a.repo.Append(ctx, event)
}

// conversationRepositoryAdapter adapts domain.ConversationRepository to memory.MemoryRepository interface
type conversationRepositoryAdapter struct {
	repo domain.ConversationRepository
}

func (a *conversationRepositoryAdapter) SaveMessage(ctx context.Context, convID string, userID string, platform domain.Platform, msg domain.StoredMessage) error {
	// First, try to get the existing conversation.
	if _, err := a.repo.GetConversation(ctx, convID); err != nil {
		// Conversation doesn't exist; create a new one.
		conv := &domain.Conversation{
			ID:       convID,
			UserID:   userID,
			Platform: platform,
			Messages: []domain.StoredMessage{msg},
		}
		return a.repo.SaveConversation(ctx, conv)
	}

	// Conversation exists; append the message.
	return a.repo.AppendMessage(ctx, convID, msg)
}

func (a *conversationRepositoryAdapter) GetConversation(ctx context.Context, convID string) (*domain.Conversation, error) {
	return a.repo.GetConversation(ctx, convID)
}

func (a *conversationRepositoryAdapter) GetRecentMessages(ctx context.Context, convID string, maxTokens int) ([]domain.StoredMessage, error) {
	conv, err := a.repo.GetConversation(ctx, convID)
	if err != nil {
		return nil, err
	}

	// Get recent messages up to token limit (from the end, going backwards)
	var messages []domain.StoredMessage
	totalTokens := 0

	for i := len(conv.Messages) - 1; i >= 0; i-- {
		msg := conv.Messages[i]
		if totalTokens+msg.TokenCount > maxTokens && len(messages) > 0 {
			break
		}
		messages = append([]domain.StoredMessage{msg}, messages...)
		totalTokens += msg.TokenCount
	}

	return messages, nil
}

func (a *conversationRepositoryAdapter) DeleteConversation(ctx context.Context, convID string) error {
	return a.repo.DeleteConversation(ctx, convID)
}

func (a *conversationRepositoryAdapter) ListConversations(ctx context.Context, userID string) ([]domain.ConversationSummary, error) {
	summaries, err := a.repo.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Convert []*domain.ConversationSummary to []domain.ConversationSummary
	result := make([]domain.ConversationSummary, len(summaries))
	for i, s := range summaries {
		result[i] = *s
	}
	return result, nil
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

	// Initialize File Storage for Admin Features (Phase 2 & 3)
	dataDir := "./data"
	usersFilePath := dataDir + "/users.json"
	botsFilePath := dataDir + "/bots.json"

	// Initialize User Profile Repository and Service (Phase 2)
	profileRepo := storage.NewFileUserProfileRepository(usersFilePath, app.Config.Security.EncryptionKey)
	profileService := profile.NewService(profileRepo, app.SecurityService)
	slog.Info("User profile management initialized", "file", usersFilePath)

	// Note: chat.Service's RBAC resolution (specs/260803-improve-nuimanbot-
	// security-auto-review, FR-001/FR-002) is wired via domainUserService,
	// passed directly into chat.NewService in main() above — not via
	// profileService, which remains dedicated to the CLI/web admin surfaces'
	// own user-profile management and is unrelated to chat RBAC resolution.

	// Initialize Bot Config Repository and Service (Phase 3)
	// Get encryption key from security config. The config value is the
	// base64-encoded key as read from NUIMANBOT_ENCRYPTION_KEY, so it must be
	// decoded back to the raw 32-byte key before use.
	encryptionKey := app.Config.Security.EncryptionKey
	if decodedKey, err := crypto.DecodeKeyFromBase64(encryptionKey); err == nil && len(decodedKey) == 32 {
		encryptionKey = string(decodedKey)
	} else {
		slog.Warn("Encryption key must decode to 32 bytes for AES-256, using default (INSECURE)")
		encryptionKey = "default-32-byte-key-changeme!!!!"
	}
	botEncryption := infrasecurity.NewEncryptionService(encryptionKey)
	botConfigRepo := storage.NewFileBotConfigRepository(botsFilePath, botEncryption)
	botMgmtService := botmgmt.NewService(botConfigRepo)
	slog.Info("Bot management initialized", "file", botsFilePath)

	// Start Web UI server if enabled
	if app.Config.Gateways.WebUI.Enabled {
		addr := app.Config.Gateways.WebUI.Addr
		if addr == "" {
			addr = ":8081"
		}
		webServer := web.NewServer(addr)

		// Wire services into Web UI
		webServer.SetProfileService(profileService)

		// Wire the Chats environment (FR-011-FR-016, specs/260805-nuimanbot-extend-context-and-ui):
		// extends the existing Conversation/ConversationRepository rather
		// than a new entity/store.
		webServer.SetChatsService(chats.NewService(app.ConversationRepo))

		// Wire network access (FR-005-FR-008): an absent config section
		// resolves to the secure localhost-only default via ToDomain().
		webServer.SetNetworkAccessConfig(app.Config.NetworkAccess.ToDomain())

		// Wire Part C's confirmation admin UI (P5.8): lists pending
		// confirmations and resolves them via ChatService.ResolveConfirmation.
		if app.ChatService != nil && app.ConfirmationStore != nil {
			webServer.SetConfirmationService(&confirmationServiceAdapter{
				store: app.ConfirmationStore,
				chat:  app.ChatService,
			})
		}

		// Add default admin user for web login
		if err := webServer.GetAuth().AddUser("admin", "admin", "admin"); err != nil {
			slog.Warn("Failed to add default web admin user", "error", err)
		}

		app.WebServer = webServer

		go func() {
			slog.Info("Starting Web Admin UI", "addr", addr)
			if err := webServer.Start(); err != nil && err != http.ErrServerClosed {
				slog.Error("Web Admin UI error", "error", err)
			}
		}()

		defer func() {
			if app.WebServer != nil {
				if err := app.WebServer.Stop(); err != nil {
					slog.Error("Failed to stop Web Admin UI", "error", err)
				}
			}
		}()
	}

	// Start REST API server if configured.
	if app.RESTServer != nil {
		restAddr := fmt.Sprintf(":%d", app.Config.ExternalAPI.REST.Port)
		if app.Config.ExternalAPI.REST.Port == 0 {
			restAddr = ":8082"
		}
		go func() {
			slog.Info("Starting REST API server", "addr", restAddr)
			if err := app.RESTServer.Start(restAddr); err != nil && err != http.ErrServerClosed {
				slog.Error("REST API server error", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := app.RESTServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("Failed to stop REST API server", "error", err)
			}
		}()
	}

	// Initialize Agent Skills System (Phase 6: Config Integration)
	skillRepo := skillinfra.NewFilesystemSkillRepository()
	skillRegistry := skillusecase.NewInMemorySkillRegistry(skillRepo)
	skillRenderer := skillusecase.NewDefaultSkillRenderer()

	// Get skill roots from configuration
	skillRoots, err := app.Config.Skills.GetRoots()
	if err != nil {
		slog.Warn("Failed to get skill roots from config",
			"error", err,
			"note", "Continuing without skills (non-fatal)",
		)
	} else if len(skillRoots) > 0 {
		// Initialize skill registry with configured roots
		if err := skillRegistry.Initialize(ctx, skillRoots); err != nil {
			slog.Warn("Failed to initialize Agent Skills system",
				"error", err,
				"note", "Continuing without skills (non-fatal)",
			)
		} else {
			slog.Info("Agent Skills system initialized",
				"skills_loaded", len(skillRegistry.List()),
				"roots_configured", len(skillRoots),
			)
		}
	} else {
		slog.Info("Agent Skills system disabled (no roots configured)")
	}

	// Create skill CLI command handler
	skillCmd := cliadapter.NewSkillCommand(skillRegistry, skillRenderer, os.Stdout)
	skillHandler := cli.NewSkillHandler(skillCmd, os.Stdout)

	// Initialize CLI gateway
	cliGateway := cli.NewGateway(&app.Config.Gateways.CLI)
	cliGateway.SetSkillHandler(skillHandler) // Enable /skill-name command support

	// Initialize admin command handlers (Phase 2 & 3)
	profileHandler := cli.NewAdminProfileCommandHandler(profileService)
	cliGateway.SetProfileHandler(profileHandler)
	slog.Info("Profile admin commands initialized")

	botHandler := cli.NewAdminBotCommandHandler(botMgmtService)
	cliGateway.SetBotHandler(botHandler)
	slog.Info("Bot admin commands initialized")

	// Initialize config admin command handler (Phase 0.5)
	if app.ConfigManager != nil {
		configHandler := cli.NewAdminConfigCommandHandler(app.ConfigManager)
		cliGateway.SetConfigHandler(configHandler)
		slog.Info("Config admin commands initialized")
	}

	// Initialize memory CLI commands (if memory v2 is available)
	if app.MemoryCellRepo != nil && app.MemorySceneRepo != nil {
		memoryCmd := cliadapter.NewMemoryCommand(app.MemoryCellRepo, app.MemorySceneRepo, os.Stdout)
		if app.MemoryAdmin != nil {
			memoryCmd.SetAdmin(app.MemoryAdmin)
			slog.Info("Memory admin operations enabled")
		}
		memoryHandler := cli.NewMemoryCommandHandler(memoryCmd)
		cliGateway.SetMemoryHandler(memoryHandler)
		slog.Info("Memory CLI commands initialized")
	}

	// Set current user as admin for CLI (CLI users are trusted)
	cliGateway.SetCurrentUser(&domain.User{
		ID:       "cli_admin",
		Username: "cli_administrator",
		Role:     domain.RoleAdmin,
	})

	app.connectGateway(cliGateway)

	// Phase 7: Connect skill handler to chat service through gateway's message handler
	// This enables skills to process through the full chat pipeline (LLM + tools)
	messageHandler := func(ctx context.Context, msg domain.IncomingMessage) error {
		response, err := app.ChatService.ProcessMessage(ctx, &msg)
		if err != nil {
			return err
		}
		return cliGateway.Send(ctx, response)
	}
	skillHandler.SetMessageHandler(messageHandler, domain.PlatformCLI, "cli_user")

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

	// Initialize Buzz gateway if enabled
	if app.Config.Gateways.Buzz.Enabled {
		// Buzz's gateway.go still resolves/creates its own domain.User on
		// first message (FR-006, P1.8) using the same shared
		// app.DomainUserService ChatService now also uses (P3.1) — this is a
		// harmless, idempotent double lookup (ChatService.resolveUser finds
		// the user Buzz's gateway already created), kept as-is rather than
		// touching already-merged Phase 1 code.

		// Generate-if-absent secp256k1 keypair (FR-007), persisted via the
		// existing credential vault.
		ensuredKey, err := crypto.EnsureBuzzKeypair(ctx, app.Vault, "buzz_private_key", app.Config.Gateways.Buzz.PrivateKey)
		if err != nil {
			slog.Warn("Failed to ensure Buzz keypair", "error", err)
		} else {
			app.Config.Gateways.Buzz.PrivateKey = ensuredKey

			buzzGateway, err := buzz.New(&app.Config.Gateways.Buzz, app.DomainUserService)
			if err != nil {
				slog.Warn("Failed to create Buzz gateway", "error", err)
			} else {
				app.connectGateway(buzzGateway)
				gateways = append(gateways, buzzGateway) //nolint:ineffassign,staticcheck // Reserved for future shutdown handling

				// Start Buzz gateway in background
				go func() {
					slog.Info("Starting gateway", "platform", "buzz")
					if err := buzzGateway.Start(ctx); err != nil {
						slog.Error("Buzz gateway error", "error", err)
					}
				}()
			}
		}
	}

	// Log startup information
	slog.Info("NuimanBot initialized",
		"log_level", app.Config.Server.LogLevel,
		"debug_mode", app.Config.Server.Debug,
		"llm_provider", app.Config.LLM.Providers[0].Type,
		"skills_registered", len(app.ToolRegistry.List()),
	)

	fmt.Println("\nStarting CLI Gateway...")
	fmt.Println("Type your messages below. Commands:")
	fmt.Println("  - Type 'exit' or 'quit' to stop")
	fmt.Println("  - Type 'help' for available skills")
	fmt.Println("  - Type '/memory help' for memory commands")
	fmt.Println("  - Type '/admin config help' for config management")
	if app.WebServer != nil {
		fmt.Printf("  - Web Admin UI: http://localhost%s\n", app.Config.Gateways.WebUI.Addr)
	}
	fmt.Println()

	// Start CLI gateway (blocks until shutdown)
	return cliGateway.Start(ctx)
}

// buildAlertingConfig constructs an alerting.Config from the application configuration.
func buildAlertingConfig(cfg *config.NuimanBotConfig) alerting.Config {
	ac := cfg.Alerting
	var channels []alerting.ChannelConfig

	if ac.Channels.Log.Enabled {
		channels = append(channels, alerting.ChannelConfig{
			Type:    alerting.ChannelTypeLog,
			Enabled: true,
		})
	}

	if ac.Channels.Slack.Enabled {
		channels = append(channels, alerting.ChannelConfig{
			Type:    alerting.ChannelTypeSlack,
			Enabled: true,
			Config: map[string]string{
				"webhook_url": ac.Channels.Slack.WebhookURL,
				"channel":     ac.Channels.Slack.Channel,
				"username":    ac.Channels.Slack.Username,
			},
		})
	}

	if ac.Channels.Email.Enabled {
		channels = append(channels, alerting.ChannelConfig{
			Type:    alerting.ChannelTypeEmail,
			Enabled: true,
			Config: map[string]string{
				"smtp_host":  ac.Channels.Email.SMTPHost,
				"smtp_port":  fmt.Sprintf("%d", ac.Channels.Email.SMTPPort),
				"username":   ac.Channels.Email.Username,
				"password":   ac.Channels.Email.Password,
				"from":       ac.Channels.Email.From,
				"recipients": ac.Channels.Email.Recipients,
			},
		})
	}

	return alerting.Config{
		Enabled:        ac.Enabled,
		ServiceName:    ac.ServiceName,
		Channels:       channels,
		ThrottleWindow: ac.ThrottleWindow,
	}
}

// ensureEncryptionKey checks if the encryption key is set in the environment.
// If not, it generates a new key, displays it to the user with warnings,
// and saves it to the .env file for future use.
func ensureEncryptionKey() error {
	// Check if key is already set
	if crypto.IsEncryptionKeySet() {
		return nil
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("🔐 FIRST-TIME SETUP: Encryption Key")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("No encryption key found. Generating a new one...")
	fmt.Println()

	// Generate new encryption key
	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}

	encodedKey := crypto.EncodeKeyToBase64(key)

	// Display the key prominently
	fmt.Println("┌────────────────────────────────────────────────────────────┐")
	fmt.Println("│                                                            │")
	fmt.Println("│  🔑 YOUR ENCRYPTION KEY (SAVE THIS!)                      │")
	fmt.Println("│                                                            │")
	fmt.Println("│  NUIMANBOT_ENCRYPTION_KEY=" + encodedKey)
	fmt.Println("│                                                            │")
	fmt.Println("│  ⚠️  IMPORTANT WARNINGS:                                   │")
	fmt.Println("│  • This key encrypts all your credentials and secrets     │")
	fmt.Println("│  • If you lose this key, you CANNOT recover your data     │")
	fmt.Println("│  • Keep it safe and secure (password manager, secrets)    │")
	fmt.Println("│  • For production, use a secrets manager (not .env file)  │")
	fmt.Println("│                                                            │")
	fmt.Println("│  ✅ This key has been saved to your .env file             │")
	fmt.Println("│     and will be loaded automatically on next startup      │")
	fmt.Println("│                                                            │")
	fmt.Println("└────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Save to .env file
	envPath := ".env"
	if err := crypto.SaveEncryptionKeyToEnv(envPath, key); err != nil {
		return fmt.Errorf("failed to save encryption key to .env: %w", err)
	}

	fmt.Println("✓ Encryption key saved to .env file")
	fmt.Println()

	// Set the environment variable for this run
	if err := os.Setenv(crypto.EncryptionKeyEnvVar, encodedKey); err != nil {
		return fmt.Errorf("failed to set encryption key environment variable: %w", err)
	}

	fmt.Println("Continuing startup with generated encryption key...")
	fmt.Println()

	return nil
}
