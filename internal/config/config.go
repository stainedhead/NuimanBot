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
	Primary string `yaml:"primary"`

	Fallbacks []string `yaml:"fallbacks"`
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

// LLMConfig encapsulates all LLM-related configurations.

type LLMConfig struct {
	DefaultModel LLMDefaultModelConfig `yaml:"default_model"`

	Models map[string]LLMModelConfig `yaml:"models"`

	Providers []LLMProviderConfig `yaml:"providers"`

	Anthropic AnthropicProviderConfig `yaml:"anthropic"`
	OpenAI    OpenAIProviderConfig    `yaml:"openai"`
	Ollama    OllamaProviderConfig    `yaml:"ollama"`

	Bedrock struct {
		AWSRegion string `yaml:"aws_region"`

		AWSProfile string `yaml:"aws_profile"`
	} `yaml:"bedrock"`
}

// MCPClientConfig holds MCP client-specific configuration.
type MCPClientConfig struct {
	AllowedServers []string `yaml:"allowed_servers"`
	Timeout        string   `yaml:"timeout"`
	MaxRetries     int      `yaml:"max_retries"`
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
	MemoryBackendBuiltin MemoryBackend = "builtin"
	MemoryBackendQMD     MemoryBackend = "qmd"
)

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
	Backend   MemoryBackend       `yaml:"backend"`
	Citations MemoryCitationsMode `yaml:"citations"`
	QMD       MemoryQMDConfig     `yaml:"qmd"`
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
