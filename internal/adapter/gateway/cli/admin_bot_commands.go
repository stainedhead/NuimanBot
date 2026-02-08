package cli

import (
	"context"
	"fmt"
	"strings"

	"nuimanbot/internal/domain"
)

// BotManagementService defines the interface for bot management operations.
type BotManagementService interface {
	CreateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error
	GetSlackBot(ctx context.Context, botID string) (*domain.SlackBotConfig, error)
	ListSlackBots(ctx context.Context) ([]*domain.SlackBotConfig, error)
	UpdateSlackBot(ctx context.Context, bot *domain.SlackBotConfig) error
	DeleteSlackBot(ctx context.Context, botID string) error
	EnableSlackBot(ctx context.Context, botID string) error
	DisableSlackBot(ctx context.Context, botID string) error

	CreateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error
	GetTelegramBot(ctx context.Context, botID string) (*domain.TelegramBotConfig, error)
	ListTelegramBots(ctx context.Context) ([]*domain.TelegramBotConfig, error)
	UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBotConfig) error
	DeleteTelegramBot(ctx context.Context, botID string) error
	EnableTelegramBot(ctx context.Context, botID string) error
	DisableTelegramBot(ctx context.Context, botID string) error
}

// AdminBotCommandHandler handles administrative bot commands.
type AdminBotCommandHandler struct {
	botService BotManagementService
}

// NewAdminBotCommandHandler creates a new admin bot command handler.
func NewAdminBotCommandHandler(botService BotManagementService) *AdminBotCommandHandler {
	return &AdminBotCommandHandler{
		botService: botService,
	}
}

// IsBotCommand checks if the input is a bot admin command.
func IsBotCommand(input string) bool {
	return strings.HasPrefix(input, "/admin bot ")
}

// HandleBotCommand processes a bot admin command and returns the response.
func (h *AdminBotCommandHandler) HandleBotCommand(ctx context.Context, currentUser *domain.User, input string) (string, error) {
	// Check if user is admin
	if currentUser.Role != domain.RoleAdmin {
		return "", domain.ErrInsufficientPermissions
	}

	// Parse command
	parts := strings.Fields(input)
	if len(parts) < 3 {
		return h.showHelp(), nil
	}

	// Skip "/admin bot"
	platform := parts[2]

	switch platform {
	case "slack":
		return h.handleSlackCommand(ctx, parts[3:])
	case "telegram":
		return h.handleTelegramCommand(ctx, parts[3:])
	case "help":
		return h.showHelp(), nil
	default:
		return fmt.Sprintf("Unknown platform: %s\nUse '/admin bot help' for usage information.", platform), nil
	}
}

// handleSlackCommand handles Slack bot subcommands
func (h *AdminBotCommandHandler) handleSlackCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return h.showSlackHelp(), nil
	}

	subcommand := args[0]

	switch subcommand {
	case "create":
		return h.createSlackBot(ctx, args[1:])
	case "list":
		return h.listSlackBots(ctx)
	case "view":
		return h.viewSlackBot(ctx, args[1:])
	case "delete":
		return h.deleteSlackBot(ctx, args[1:])
	case "enable":
		return h.enableSlackBot(ctx, args[1:])
	case "disable":
		return h.disableSlackBot(ctx, args[1:])
	case "help":
		return h.showSlackHelp(), nil
	default:
		return fmt.Sprintf("Unknown Slack command: %s\nUse '/admin bot slack help' for usage information.", subcommand), nil
	}
}

// handleTelegramCommand handles Telegram bot subcommands
func (h *AdminBotCommandHandler) handleTelegramCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return h.showTelegramHelp(), nil
	}

	subcommand := args[0]

	switch subcommand {
	case "create":
		return h.createTelegramBot(ctx, args[1:])
	case "list":
		return h.listTelegramBots(ctx)
	case "view":
		return h.viewTelegramBot(ctx, args[1:])
	case "delete":
		return h.deleteTelegramBot(ctx, args[1:])
	case "enable":
		return h.enableTelegramBot(ctx, args[1:])
	case "disable":
		return h.disableTelegramBot(ctx, args[1:])
	case "help":
		return h.showTelegramHelp(), nil
	default:
		return fmt.Sprintf("Unknown Telegram command: %s\nUse '/admin bot telegram help' for usage information.", subcommand), nil
	}
}

// Slack Bot Operations

func (h *AdminBotCommandHandler) createSlackBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot slack create <bot_id> --name <value> --type <public|private> --bot-token <value> --app-token <value>", nil
	}

	botID := args[0]
	config := make(map[string]string)

	// Parse flags
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return fmt.Sprintf("Missing value for flag: %s", args[i]), nil
		}
		flag := args[i]
		value := args[i+1]
		config[strings.TrimPrefix(flag, "--")] = value
	}

	// Validate required fields
	if config["name"] == "" {
		return "Missing required flag: --name", nil
	}
	if config["type"] == "" {
		return "Missing required flag: --type", nil
	}
	if config["bot-token"] == "" {
		return "Missing required flag: --bot-token", nil
	}

	// Parse bot type
	var botType domain.BotType
	switch config["type"] {
	case "public":
		botType = domain.BotTypePublic
	case "private":
		botType = domain.BotTypePrivate
	default:
		return fmt.Sprintf("Invalid bot type: %s (must be 'public' or 'private')", config["type"]), nil
	}

	// Create bot
	bot := domain.NewSlackBotConfig(botID, config["name"], botType)
	bot.SlackBotToken = config["bot-token"]
	bot.SlackAppToken = config["app-token"]
	bot.SlackSigningSecret = config["signing-secret"]

	if botType == domain.BotTypePrivate {
		bot.OwnerUserID = config["owner"]
	}

	if err := h.botService.CreateSlackBot(ctx, bot); err != nil {
		return fmt.Sprintf("Failed to create Slack bot: %v", err), err
	}

	return fmt.Sprintf("Slack bot created successfully: %s (%s)", bot.BotName, bot.BotID), nil
}

func (h *AdminBotCommandHandler) listSlackBots(ctx context.Context) (string, error) {
	bots, err := h.botService.ListSlackBots(ctx)
	if err != nil {
		return fmt.Sprintf("Failed to list Slack bots: %v", err), err
	}

	if len(bots) == 0 {
		return "No Slack bots configured.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Slack Bots (%d):\n", len(bots)))
	for _, bot := range bots {
		status := "enabled"
		if !bot.Enabled {
			status = "disabled"
		}
		result.WriteString(fmt.Sprintf("- %s: %s (%s) [%s]\n", bot.BotID, bot.BotName, bot.BotType, status))
	}

	return result.String(), nil
}

func (h *AdminBotCommandHandler) viewSlackBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot slack view <bot_id>", nil
	}

	botID := args[0]
	bot, err := h.botService.GetSlackBot(ctx, botID)
	if err != nil {
		return fmt.Sprintf("Failed to get Slack bot: %v", err), err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Slack Bot: %s\n", bot.BotName))
	result.WriteString(fmt.Sprintf("  Bot ID: %s\n", bot.BotID))
	result.WriteString(fmt.Sprintf("  Type: %s\n", bot.BotType))
	result.WriteString(fmt.Sprintf("  Enabled: %t\n", bot.Enabled))
	if bot.OwnerUserID != "" {
		result.WriteString(fmt.Sprintf("  Owner: %s\n", bot.OwnerUserID))
	}
	if len(bot.AllowedUserIDs) > 0 {
		result.WriteString(fmt.Sprintf("  Allowed Users: %s\n", strings.Join(bot.AllowedUserIDs, ", ")))
	}

	return result.String(), nil
}

func (h *AdminBotCommandHandler) deleteSlackBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot slack delete <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.DeleteSlackBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to delete Slack bot: %v", err), err
	}

	return fmt.Sprintf("Slack bot deleted successfully: %s", botID), nil
}

func (h *AdminBotCommandHandler) enableSlackBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot slack enable <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.EnableSlackBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to enable Slack bot: %v", err), err
	}

	return fmt.Sprintf("Slack bot enabled successfully: %s", botID), nil
}

func (h *AdminBotCommandHandler) disableSlackBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot slack disable <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.DisableSlackBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to disable Slack bot: %v", err), err
	}

	return fmt.Sprintf("Slack bot disabled successfully: %s", botID), nil
}

// Telegram Bot Operations

func (h *AdminBotCommandHandler) createTelegramBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot telegram create <bot_id> --name <value> --type <public|private> --token <value>", nil
	}

	botID := args[0]
	config := make(map[string]string)

	// Parse flags
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return fmt.Sprintf("Missing value for flag: %s", args[i]), nil
		}
		flag := args[i]
		value := args[i+1]
		config[strings.TrimPrefix(flag, "--")] = value
	}

	// Validate required fields
	if config["name"] == "" {
		return "Missing required flag: --name", nil
	}
	if config["type"] == "" {
		return "Missing required flag: --type", nil
	}
	if config["token"] == "" {
		return "Missing required flag: --token", nil
	}

	// Parse bot type
	var botType domain.BotType
	switch config["type"] {
	case "public":
		botType = domain.BotTypePublic
	case "private":
		botType = domain.BotTypePrivate
	default:
		return fmt.Sprintf("Invalid bot type: %s (must be 'public' or 'private')", config["type"]), nil
	}

	// Create bot
	bot := domain.NewTelegramBotConfig(botID, config["name"], botType)
	bot.TelegramBotToken = config["token"]
	bot.TelegramBotUsername = config["username"]

	if botType == domain.BotTypePrivate {
		bot.OwnerUserID = config["owner"]
	}

	if err := h.botService.CreateTelegramBot(ctx, bot); err != nil {
		return fmt.Sprintf("Failed to create Telegram bot: %v", err), err
	}

	return fmt.Sprintf("Telegram bot created successfully: %s (%s)", bot.BotName, bot.BotID), nil
}

func (h *AdminBotCommandHandler) listTelegramBots(ctx context.Context) (string, error) {
	bots, err := h.botService.ListTelegramBots(ctx)
	if err != nil {
		return fmt.Sprintf("Failed to list Telegram bots: %v", err), err
	}

	if len(bots) == 0 {
		return "No Telegram bots configured.", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Telegram Bots (%d):\n", len(bots)))
	for _, bot := range bots {
		status := "enabled"
		if !bot.Enabled {
			status = "disabled"
		}
		result.WriteString(fmt.Sprintf("- %s: %s (%s) [%s]\n", bot.BotID, bot.BotName, bot.BotType, status))
	}

	return result.String(), nil
}

func (h *AdminBotCommandHandler) viewTelegramBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot telegram view <bot_id>", nil
	}

	botID := args[0]
	bot, err := h.botService.GetTelegramBot(ctx, botID)
	if err != nil {
		return fmt.Sprintf("Failed to get Telegram bot: %v", err), err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Telegram Bot: %s\n", bot.BotName))
	result.WriteString(fmt.Sprintf("  Bot ID: %s\n", bot.BotID))
	result.WriteString(fmt.Sprintf("  Type: %s\n", bot.BotType))
	result.WriteString(fmt.Sprintf("  Enabled: %t\n", bot.Enabled))
	if bot.OwnerUserID != "" {
		result.WriteString(fmt.Sprintf("  Owner: %s\n", bot.OwnerUserID))
	}
	if bot.TelegramBotUsername != "" {
		result.WriteString(fmt.Sprintf("  Username: @%s\n", bot.TelegramBotUsername))
	}

	return result.String(), nil
}

func (h *AdminBotCommandHandler) deleteTelegramBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot telegram delete <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.DeleteTelegramBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to delete Telegram bot: %v", err), err
	}

	return fmt.Sprintf("Telegram bot deleted successfully: %s", botID), nil
}

func (h *AdminBotCommandHandler) enableTelegramBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot telegram enable <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.EnableTelegramBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to enable Telegram bot: %v", err), err
	}

	return fmt.Sprintf("Telegram bot enabled successfully: %s", botID), nil
}

func (h *AdminBotCommandHandler) disableTelegramBot(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "Usage: /admin bot telegram disable <bot_id>", nil
	}

	botID := args[0]
	if err := h.botService.DisableTelegramBot(ctx, botID); err != nil {
		return fmt.Sprintf("Failed to disable Telegram bot: %v", err), err
	}

	return fmt.Sprintf("Telegram bot disabled successfully: %s", botID), nil
}

// Help functions

func (h *AdminBotCommandHandler) showHelp() string {
	return `Bot Management Commands:

Usage: /admin bot <platform> <command> [arguments]

Platforms:
  slack       Manage Slack bots
  telegram    Manage Telegram bots

Commands:
  create      Create a new bot
  list        List all bots
  view        View bot details
  delete      Delete a bot
  enable      Enable a bot
  disable     Disable a bot
  help        Show this help message

Examples:
  /admin bot slack list
  /admin bot telegram create my-bot --name "My Bot" --type public --token "123456:ABC"
  /admin bot slack help

For platform-specific help, use:
  /admin bot slack help
  /admin bot telegram help`
}

func (h *AdminBotCommandHandler) showSlackHelp() string {
	return `Slack Bot Management:

Commands:
  create <bot_id> --name <name> --type <public|private> --bot-token <token> --app-token <token> [--signing-secret <secret>] [--owner <user_id>]
    Create a new Slack bot

  list
    List all Slack bots

  view <bot_id>
    View details of a specific bot

  delete <bot_id>
    Delete a Slack bot

  enable <bot_id>
    Enable a Slack bot

  disable <bot_id>
    Disable a Slack bot

Examples:
  /admin bot slack create team-bot --name "Team Assistant" --type public --bot-token xoxb-123 --app-token xapp-456
  /admin bot slack list
  /admin bot slack enable team-bot`
}

func (h *AdminBotCommandHandler) showTelegramHelp() string {
	return `Telegram Bot Management:

Commands:
  create <bot_id> --name <name> --type <public|private> --token <token> [--username <username>] [--owner <user_id>]
    Create a new Telegram bot

  list
    List all Telegram bots

  view <bot_id>
    View details of a specific bot

  delete <bot_id>
    Delete a Telegram bot

  enable <bot_id>
    Enable a Telegram bot

  disable <bot_id>
    Disable a Telegram bot

Examples:
  /admin bot telegram create my-bot --name "My Bot" --type public --token "123456:ABC-DEF"
  /admin bot telegram list
  /admin bot telegram disable my-bot`
}
