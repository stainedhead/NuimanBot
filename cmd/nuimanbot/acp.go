package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"nuimanbot/internal/adapter/acp"
	"nuimanbot/internal/config"
	"nuimanbot/internal/infrastructure/crypto"
	"nuimanbot/internal/infrastructure/logger"
	infrasecurity "nuimanbot/internal/infrastructure/security"
	"nuimanbot/internal/infrastructure/storage"
	"nuimanbot/internal/usecase/chat"
	"nuimanbot/internal/usecase/security"
	"nuimanbot/internal/usecase/tool"
	"nuimanbot/internal/usecase/user"
)

// acpSubcommand is the os.Args[1] value that selects ACP mode. Registered
// via Buzz's --agent-command/--agent-args (e.g. --agent-command
// /path/to/bin/nuimanbot --agent-args acp), mirroring how Buzz already
// spawns `goose acp`/`codex-acp` as custom harnesses — see
// support_docs/buzz-acp-harness-guide.md.
const acpSubcommand = "acp"

// runACP boots the subset of NuimanBot's dependencies chat.Service.
// ProcessMessage actually needs (config, storage, security, LLM, tools,
// chat) and drives them via an acp.Server reading/writing stdio, instead of
// main()'s full Run() (health/web/REST servers, gateways). It deliberately
// starts none of those — Buzz's buzz-acp bridge spawns one NuimanBot
// subprocess per conversation (observed parallelism: 10+), so every one of
// them binding the same fixed health/web-admin ports would collbide.
//
// Every diagnostic message goes to stderr, never stdout: stdout is
// reserved exclusively for the ACP JSON-RPC stream (see acp.Server's doc
// comment) — a single stray byte there would corrupt it for the host.
func runACP(ctx context.Context) error {
	if err := ensureEncryptionKeyQuiet(); err != nil {
		return fmt.Errorf("acp: failed to initialize encryption key: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("acp: failed to load configuration: %w", err)
	}
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("acp: configuration validation failed: %w", err)
	}

	logFormat := "json"
	if cfg.Server.Debug {
		logFormat = "text"
	}
	logger.Initialize(logger.Config{
		Level:  logger.LogLevel(cfg.Server.LogLevel),
		Format: logFormat,
		Output: os.Stderr,
	})
	acpLogger := slog.Default()
	acpLogger.Info("ACP mode starting", "agent", acp.AgentName, "version", acp.AgentVersion)

	vaultPath := cfg.Security.VaultPath
	if vaultPath == "" {
		vaultPath = "./data/vault.enc"
	}
	vaultKey, err := crypto.DecodeKeyFromBase64(cfg.Security.EncryptionKey)
	if err != nil {
		return fmt.Errorf("acp: failed to decode encryption key: %w", err)
	}
	vault, err := crypto.NewFileCredentialVault(vaultPath, vaultKey)
	if err != nil {
		return fmt.Errorf("acp: failed to create credential vault: %w", err)
	}

	storagePath := cfg.Storage.DSN
	if storagePath == "" {
		storagePath = "./data"
	}
	if err := storage.Initialize(storagePath); err != nil {
		return fmt.Errorf("acp: failed to initialize storage: %w", err)
	}
	fileRepos, err := initializeFileStorage(storagePath, cfg.Security.EncryptionKey)
	if err != nil {
		return fmt.Errorf("acp: failed to initialize file storage: %w", err)
	}

	inputValidator := security.NewDefaultInputValidator()
	auditAdapter := &auditRepositoryAdapter{repo: fileRepos.Audit}
	securityService := security.NewService(vault, inputValidator, auditAdapter)

	domainUserRepo := storage.NewFileUserRepository(storagePath + "/domain_users.json")
	domainUserService := user.NewService(domainUserRepo, securityService)

	outputValidator := buildOutputValidator(cfg.Security.ToolOutputValidation)

	llmService, err := initializeLLMService(cfg)
	if err != nil {
		return fmt.Errorf("acp: failed to create LLM service: %w", err)
	}

	toolRegistry := tool.NewInMemoryRegistry()
	if err := registerBuiltInTools(toolRegistry, fileRepos.Notes, llmService, outputValidator, cfg.Security.Fetch); err != nil {
		return fmt.Errorf("acp: failed to register tools: %w", err)
	}
	if err := registerMCPTools(ctx, cfg, toolRegistry, outputValidator); err != nil {
		acpLogger.Warn("MCP tool registration encountered errors", "error", err)
	}

	toolExecutionService := tool.NewService(&cfg.Tools, toolRegistry, securityService)
	toolExecutionService.SetConfirmationConfig(cfg.Security.Confirmation)

	conversationAdapter := &conversationRepositoryAdapter{repo: fileRepos.Conversation}
	chatService := chat.NewService(llmService, conversationAdapter, toolExecutionService, securityService, domainUserService)
	chatService.SetLLMDefaults(chat.LLMDefaults{
		Model:       cfg.LLM.DefaultModel.Primary,
		MaxTokens:   cfg.LLM.DefaultModel.MaxTokens,
		Temperature: cfg.LLM.DefaultModel.Temperature,
	})

	confirmationStorePath := storagePath + "/confirmations.json"
	confirmationStore := infrasecurity.NewFileConfirmationStore(confirmationStorePath, cfg.Security.Confirmation.ResolvedTimeout())
	if err := wireConfirmationStore(chatService, toolExecutionService, confirmationStore); err != nil {
		return fmt.Errorf("acp: failed to wire confirmation store: %w", err)
	}

	server := acp.NewServer(chatService, acpLogger)
	acpLogger.Info("ACP server ready, reading stdio")
	return server.Run(ctx, os.Stdin, os.Stdout)
}

// ensureEncryptionKeyQuiet mirrors ensureEncryptionKey's behavior (generate
// a key on first run, persist it to .env) but writes its output to stderr
// instead of stdout — stdout must stay reserved for the ACP JSON-RPC stream
// from the moment this process starts. Kept as a separate function rather
// than adding a "quiet" flag to ensureEncryptionKey so the well-exercised
// interactive path (main()'s banner, which real users rely on to actually
// see and save a newly generated key) is untouched by this change.
func ensureEncryptionKeyQuiet() error {
	// See ensureEncryptionKey's matching comment in main.go: without loading
	// .env first, a key already saved from a prior run is invisible to
	// IsEncryptionKeySet in a freshly spawned process, and this would
	// regenerate (and corrupt the pairing with) an existing vault on every
	// single ACP subprocess spawn.
	_ = godotenv.Load() //nolint:errcheck // .env file is optional

	if crypto.IsEncryptionKeySet() {
		return nil
	}

	fmt.Fprintln(os.Stderr, "acp: no encryption key found, generating one for first-time setup")

	key, err := crypto.GenerateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	encodedKey := crypto.EncodeKeyToBase64(key)

	fmt.Fprintf(os.Stderr, "acp: %s=%s (saved to .env; back this up — losing it makes existing encrypted data unrecoverable)\n", crypto.EncryptionKeyEnvVar, encodedKey)

	if err := crypto.SaveEncryptionKeyToEnv(".env", key); err != nil {
		return fmt.Errorf("failed to save encryption key to .env: %w", err)
	}
	if err := os.Setenv(crypto.EncryptionKeyEnvVar, encodedKey); err != nil {
		return fmt.Errorf("failed to set encryption key environment variable: %w", err)
	}
	return nil
}
