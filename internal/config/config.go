package config

import "nuimanbot/internal/domain"

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Environment Environment `yaml:"environment"`
	LogLevel    string      `yaml:"log_level"`
	Debug       bool        `yaml:"debug"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	InputMaxLength     int    `yaml:"input_max_length"`
	TokenRotationHours int    `yaml:"token_rotation_hours"`
	VaultPath          string `yaml:"vault_path"`
	EncryptionKey      string `yaml:"encryption_key"`
}

// LLMProviderConfig configures a specific LLM provider instance.
type LLMProviderConfig struct {
	ID      string              `yaml:"id"`
	Type    domain.LLMProvider  `yaml:"type"`
	APIKey  domain.SecureString `yaml:"api_key"`
	BaseURL string              `yaml:"base_url"`
	Name    string              `yaml:"name"`
}

// LLMModelConfig holds configuration for a specific LLM model.
type LLMModelConfig struct {
	Alias            string                 `yaml:"alias"`
	ProviderConfigID string                 `yaml:"provider_config_id"`
	Params           map[string]interface{} `yaml:"params"`
}

// LLMDefaultModelConfig holds default LLM model configuration.
type LLMDefaultModelConfig struct {
	Primary     string   `yaml:"primary"`
	Fallbacks   []string `yaml:"fallbacks"`
	MaxTokens   int      `yaml:"max_tokens"`
	Temperature float64  `yaml:"temperature"`
}

// AnthropicProviderConfig holds Anthropic-specific provider configuration.
type AnthropicProviderConfig struct {
	APIKey domain.SecureString `yaml:"api_key"`
}

// OpenAIProviderConfig holds OpenAI-specific provider configuration.
type OpenAIProviderConfig struct {
	APIKey       domain.SecureString `yaml:"api_key"`
	BaseURL      string              `yaml:"base_url"`
	DefaultModel string              `yaml:"default_model"`
	Organization string              `yaml:"organization"`
}

// OllamaProviderConfig holds Ollama-specific provider configuration.
type OllamaProviderConfig struct {
	BaseURL      string `yaml:"base_url"`
	DefaultModel string `yaml:"default_model"`
}

// BedrockProviderConfig holds AWS Bedrock-specific provider configuration.
type BedrockProviderConfig struct {
	AWSRegion      string `yaml:"aws_region"`      // Required: AWS region (e.g., us-east-1)
	AWSProfile     string `yaml:"aws_profile"`     // Optional: AWS profile name
	DefaultModel   string `yaml:"default_model"`   // Optional: Default Bedrock model ID
	MaxRetries     int    `yaml:"max_retries"`     // Default: 3
	RequestTimeout int    `yaml:"request_timeout"` // Default: 120 seconds
}

// LLMConfig encapsulates all LLM-related configurations.

type LLMConfig struct {
	DefaultModel LLMDefaultModelConfig `yaml:"default_model"`

	Models map[string]LLMModelConfig `yaml:"models"`

	Providers []LLMProviderConfig `yaml:"providers"`

	Anthropic AnthropicProviderConfig `yaml:"anthropic"`
	OpenAI    OpenAIProviderConfig    `yaml:"openai"`
	Ollama    OllamaProviderConfig    `yaml:"ollama"`
	Bedrock   BedrockProviderConfig   `yaml:"bedrock"`
}

// MCPClientConfig holds MCP client-specific configuration.
type MCPClientConfig struct {
	// AllowedServers is the list of MCP server names permitted for use.
	AllowedServers []string `yaml:"allowed_servers"`
	// Timeout is the per-request timeout for MCP calls (e.g. "30s").
	Timeout string `yaml:"timeout"`
	// MaxRetries is the number of retries on transient errors.
	MaxRetries int `yaml:"max_retries"`
	// ConfigFile is the path to the mcp.json server configuration file (default: "mcp.json").
	ConfigFile string `yaml:"config_file"`
	// Enabled activates the MCP client at startup.
	Enabled bool `yaml:"enabled"`
}

// MCPServerConfig holds MCP server-specific configuration.
type MCPServerConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
	TLS     bool `yaml:"tls"`
}

// MCPConfig configures the Model Context Protocol (MCP) server and client.
type MCPConfig struct {
	Server MCPServerConfig `yaml:"server"`
	Client MCPClientConfig `yaml:"client"`
}

// ToolConfig configures an individual tool.
type ToolConfig struct {
	Enabled bool                   `yaml:"enabled"`
	APIKey  domain.SecureString    `yaml:"api_key"`
	Env     map[string]string      `yaml:"env"`
	Params  map[string]interface{} `yaml:"params"`
}

// ToolsSystemConfig defines global settings for the tool system.
type ToolsSystemConfig struct {
	Entries map[string]ToolConfig `yaml:"entries"`
	Load    struct {
		ExtraDirs []string `yaml:"extra_dirs"`
		Watch     bool     `yaml:"watch"`
	} `yaml:"load"`
}

// StorageConfig holds storage-related configuration.
type StorageConfig struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
	DSN  string `yaml:"dsn"`
}

// MemoryBackend defines the type of memory backend to use.
type MemoryBackend string

const (
	// MemoryBackendBuiltin uses the built-in file-based memory storage.
	MemoryBackendBuiltin MemoryBackend = "builtin"
	// MemoryBackendQMD uses the Queryable Memory Document backend.
	MemoryBackendQMD MemoryBackend = "qmd"
	// MemoryBackendIngatan uses the Ingatan hybrid-search memory backend.
	MemoryBackendIngatan MemoryBackend = "ingatan"
)

// IngatanConfig holds configuration for the Ingatan memory backend.
type IngatanConfig struct {
	// URL is the base URL of the Ingatan server (e.g. "https://localhost:8443").
	URL string `yaml:"url"`
	// APIKey is the API key used to exchange for a JWT. Stored as SecureString to prevent log leakage.
	APIKey domain.SecureString `yaml:"api_key"`
	// StorePrefix is the prefix for Ingatan store names (default: "nuiman").
	StorePrefix string `yaml:"store_prefix"`
	// TLSSkipVerify skips TLS certificate verification. For development only; logs a warning at startup.
	TLSSkipVerify bool `yaml:"tls_skip_verify"`
	// TokenTTL is the duration before proactively refreshing the JWT (e.g. "23h").
	TokenTTL string `yaml:"token_ttl"`
	// FallbackToBuiltin enables graceful fallback to the built-in backend when Ingatan is unreachable.
	FallbackToBuiltin bool `yaml:"fallback_to_builtin"`
}

// TLSConfig holds TLS configuration for the server.
type TLSConfig struct {
	// Enabled activates TLS for the server.
	Enabled bool `yaml:"enabled"`
	// AutoGenerate generates a self-signed certificate if no cert files are present.
	AutoGenerate bool `yaml:"auto_generate"`
	// CertFile is the path to the TLS certificate file (e.g. "data/certs/server.crt").
	CertFile string `yaml:"cert_file"`
	// KeyFile is the path to the TLS private key file (e.g. "data/certs/server.key").
	KeyFile string `yaml:"key_file"`
	// Hosts is the list of hostnames to include in the self-signed certificate.
	Hosts []string `yaml:"hosts"`
}

// MemoryCitationsMode defines how citations are handled.
type MemoryCitationsMode string

const (
	MemoryCitationsModeAuto MemoryCitationsMode = "auto"
	MemoryCitationsModeOn   MemoryCitationsMode = "on"
	MemoryCitationsModeOff  MemoryCitationsMode = "off"
)

// MemoryQMDIndexPath defines a path to a memory document or directory for QMD.
type MemoryQMDIndexPath struct {
	Path    string `yaml:"path"`
	Name    string `yaml:"name"`
	Pattern string `yaml:"pattern"`
}

// MemoryQMDSessionsConfig holds QMD session-related configuration.
type MemoryQMDSessionsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	ExportDir     string `yaml:"export_dir"`
	RetentionDays int    `yaml:"retention_days"`
}

// MemoryQMDUpdateConfig holds QMD update-related configuration.
type MemoryQMDUpdateConfig struct {
	Interval   string `yaml:"interval"`
	DebounceMs int    `yaml:"debounce_ms"`
	OnBoot     bool   `yaml:"on_boot"`
}

// MemoryQMDLimitsConfig holds QMD limits-related configuration.
type MemoryQMDLimitsConfig struct {
	MaxResults       int `yaml:"max_results"`
	MaxSnippetChars  int `yaml:"max_snippet_chars"`
	MaxInjectedChars int `yaml:"max_injected_chars"`
	TimeoutMs        int `yaml:"timeout_ms"`
}

// MemoryQMDConfig configures the Queryable Memory Document (QMD) backend.
type MemoryQMDConfig struct {
	Command              string                  `yaml:"command"`
	IncludeDefaultMemory bool                    `yaml:"include_default_memory"`
	Paths                []MemoryQMDIndexPath    `yaml:"paths"`
	Sessions             MemoryQMDSessionsConfig `yaml:"sessions"`
	Update               MemoryQMDUpdateConfig   `yaml:"update"`
	Limits               MemoryQMDLimitsConfig   `yaml:"limits"`
}

// MemoryConfig defines the configuration for the agent's long-term memory.
type MemoryConfig struct {
	// Backend selects the memory storage backend ("builtin", "qmd", or "ingatan").
	Backend MemoryBackend `yaml:"backend"`
	// Citations controls how memory citations are presented.
	Citations MemoryCitationsMode `yaml:"citations"`
	// QMD configures the Queryable Memory Document backend.
	QMD MemoryQMDConfig `yaml:"qmd"`
	// Ingatan configures the Ingatan hybrid-search memory backend.
	Ingatan IngatanConfig `yaml:"ingatan"`
}

// ExternalAPIOpenAIConfig holds OpenAI-compatible API specific configuration.
type ExternalAPIOpenAIConfig struct {
	Enabled      bool                `yaml:"enabled"`
	Port         int                 `yaml:"port"`
	APIKey       domain.SecureString `yaml:"api_key"`
	DefaultModel string              `yaml:"default_model"`
}

// ExternalAPIRestConfig holds REST API specific configuration.
type ExternalAPIRestConfig struct {
	Enabled bool                `yaml:"enabled"`
	Port    int                 `yaml:"port"`
	APIKey  domain.SecureString `yaml:"api_key"`
}

// ExternalAPIConfig holds external API configurations.
type ExternalAPIConfig struct {
	OpenAI ExternalAPIOpenAIConfig `yaml:"openai"`
	REST   ExternalAPIRestConfig   `yaml:"rest"`
}

// ToolsWebSearchConfig holds web search tool configuration.
type ToolsWebSearchConfig struct {
	APIKey     domain.SecureString `yaml:"api_key"`
	MaxResults int                 `yaml:"max_results"`
}

// ToolsExecConfig holds execution tool configuration.
type ToolsExecConfig struct {
	Timeout             int  `yaml:"timeout"`
	RestrictToWorkspace bool `yaml:"restrict_to_workspace"`
}

// ToolSettings holds all tool-specific configurations (API keys, limits, etc).
type ToolSettings struct {
	WebSearch ToolsWebSearchConfig `yaml:"web_search"`
	Exec      ToolsExecConfig      `yaml:"exec"`
}

// AlertingConfig holds alerting system configuration.
type AlertingConfig struct {
	Enabled        bool                   `yaml:"enabled"`
	ServiceName    string                 `yaml:"service_name"`
	ThrottleWindow int                    `yaml:"throttle_window"` // Seconds
	Channels       AlertingChannelsConfig `yaml:"channels"`
}

// AlertingChannelsConfig holds configuration for individual alerting channels.
type AlertingChannelsConfig struct {
	Log   AlertingLogConfig   `yaml:"log"`
	Slack AlertingSlackConfig `yaml:"slack"`
	Email AlertingEmailConfig `yaml:"email"`
}

// AlertingLogConfig holds log-based alerting configuration.
type AlertingLogConfig struct {
	Enabled bool `yaml:"enabled"`
}

// AlertingSlackConfig holds Slack webhook alerting configuration.
type AlertingSlackConfig struct {
	Enabled    bool   `yaml:"enabled"`
	WebhookURL string `yaml:"webhook_url"`
	Channel    string `yaml:"channel"`
	Username   string `yaml:"username"`
}

// AlertingEmailConfig holds email SMTP alerting configuration.
type AlertingEmailConfig struct {
	Enabled    bool   `yaml:"enabled"`
	SMTPHost   string `yaml:"smtp_host"`
	SMTPPort   int    `yaml:"smtp_port"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	From       string `yaml:"from"`
	Recipients string `yaml:"recipients"` // Comma-separated email addresses
}
