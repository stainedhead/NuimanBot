package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/adapter/repository/sqlite"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
	"nuimanbot/internal/infrastructure/crypto"
	anthropic "nuimanbot/internal/infrastructure/llm/anthropic"
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
}

func main() {
	fmt.Println("NuimanBot starting...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Initialize application dependencies
	vault, err := crypto.NewFileCredentialVault(cfg.Security.VaultPath, []byte(cfg.Security.EncryptionKey))
	if err != nil {
		log.Fatalf("Failed to create credential vault: %v", err)
	}

	securityService := security.NewService(vault, security.NewDefaultInputValidator(), security.NewNoOpAuditor())

	db, err := sql.Open("sqlite3", cfg.Storage.DSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	memoryRepo := sqlite.NewMessageRepository(db)

	// TODO: This is not a robust way to select the LLM provider.
	// It should be based on a configuration setting.
	llmService, err := anthropic.NewClient(&cfg.LLM.Providers[0])
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	skillRegistry := skill.NewInMemoryRegistry()
	// TODO: Register skills here
	skillExecutionService := skill.NewService(&cfg.Skills, skillRegistry, securityService)

	chatService := chat.NewService(llmService, memoryRepo, skillExecutionService, securityService)

	app := &application{
		Config:                cfg,
		Vault:                 vault,
		SecurityService:       securityService,
		Memory:                memoryRepo,
		LLMService:            llmService,
		SkillRegistry:         skillRegistry,
		ChatService:           chatService,
		SkillExecutionService: skillExecutionService,
	}

	// 3. Run the application
	if err := app.Run(); err != nil {
		log.Fatalf("NuimanBot stopped with error: %v", err)
	}

	fmt.Println("NuimanBot stopped gracefully.")
}

// Run starts the main application services.
func (app *application) Run() error {
	// For MVP, we start the CLI gateway directly.
	// In a full application, this would involve dependency injection
	// and starting multiple services/gateways based on configuration.

	cliGateway := cli.NewGateway(&app.Config.Gateways.CLI)

	// Placeholder for message handling. A real implementation would connect
	// this to the core chat use-case service.
	cliGateway.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		response, err := app.ChatService.ProcessMessage(ctx, &msg)
		if err != nil {
			log.Printf("Error processing message: %v", err)
			// Optionally, send an error message back to the user
			if _, writeErr := fmt.Fprintf(cliGateway.Writer, "Error: %s\n", err.Error()); writeErr != nil {
				log.Printf("Error writing to CLI output: %v", writeErr)
				return writeErr
			}
			return err
		}

		if _, err := fmt.Fprintf(cliGateway.Writer, "%s\n", response.Content); err != nil {
			log.Printf("Error writing to CLI output: %v", err)
			return err
		}
		return nil
	})

	fmt.Printf("NuimanBot running with LogLevel: %s, Debug: %t\n", app.Config.Server.LogLevel, app.Config.Server.Debug)
	fmt.Println("Starting CLI Gateway...")
	return cliGateway.Start(context.Background())

}
