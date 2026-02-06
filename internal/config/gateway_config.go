package config

import "nuimanbot/internal/domain"

// DMPolicy defines the direct message policy for gateways.
type DMPolicy string

const (
	DMPolicyPairing   DMPolicy = "pairing"
	DMPolicyAllowlist DMPolicy = "allowlist"
	DMPolicyOpen      DMPolicy = "open"
)

// TelegramConfig configures the Telegram Gateway.
type TelegramConfig struct {
	Token      domain.SecureString `yaml:"token"`
	WebhookURL string              `yaml:"webhook_url"`
	AllowedIDs []int64             `yaml:"allowed_ids"`
	DMPolicy   DMPolicy
}

// SlackConfig configures the Slack Gateway.
type SlackConfig struct {
	BotToken    domain.SecureString `yaml:"bot_token"`
	AppToken    domain.SecureString `yaml:"app_token"`
	WorkspaceID string              `yaml:"workspace_id"`
}

// CLIConfig configures the CLI Gateway.
type CLIConfig struct {
	HistoryFile string `yaml:"history_file"`
	DebugMode   bool   `yaml:"debug_mode"`
}

// GatewaysConfig holds all gateway configurations.
type GatewaysConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Slack    SlackConfig    `yaml:"slack"`
	CLI      CLIConfig      `yaml:"cli"`
}
