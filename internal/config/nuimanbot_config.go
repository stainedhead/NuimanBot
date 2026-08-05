package config

// NuimanBotConfig encapsulates the entire application configuration.
type NuimanBotConfig struct {
	// Server holds server-level settings (log level, debug mode, environment).
	Server ServerConfig `yaml:"server"`
	// Security holds security-related settings (vault path, input limits).
	Security SecurityConfig `yaml:"security"`
	// TLS holds TLS certificate and listener configuration.
	TLS TLSConfig `yaml:"tls"`
	// LLM holds all LLM provider and model configuration.
	LLM LLMConfig `yaml:"llm"`
	// Gateways holds gateway (Telegram, Slack, CLI) configuration.
	Gateways GatewaysConfig `yaml:"gateways"`
	// MCP holds Model Context Protocol server/client configuration.
	MCP MCPConfig `yaml:"mcp"`
	// Storage holds file/database storage settings.
	Storage StorageConfig `yaml:"storage"`
	// Tools is the tool registry system configuration (renamed from Skills).
	Tools ToolsSystemConfig `yaml:"tools"`
	// Skills is the Anthropic-style agent skills system configuration.
	Skills SkillsConfig `yaml:"skills"`
	// Memory holds long-term memory backend configuration.
	Memory MemoryConfig `yaml:"memory"`
	// Alerting holds alerting channel configuration.
	Alerting AlertingConfig `yaml:"alerting"`
	// ExternalAPI holds external REST/OpenAI-compatible API configuration.
	ExternalAPI ExternalAPIConfig `yaml:"external_api"`
	// ToolSettings holds tool-specific API keys and limits (renamed from Tools).
	ToolSettings ToolSettings `yaml:"tool_settings"`
	// NetworkAccess configures the web admin's network exposure (FR-005–FR-008):
	// localhost-only vs. remote, and an optional IP/hostname allowlist.
	NetworkAccess NetworkAccessConfig `yaml:"network_access"`
	// WorkerPool configures the shared Job/Chore execution worker pool
	// (FR-004, FR-039).
	WorkerPool WorkerPoolConfig `yaml:"worker_pool"`
	// RetentionDefaults holds the system-wide default retention windows
	// (FR-003) applied to a user until they override them in Settings.
	RetentionDefaults RetentionDefaultsConfig `yaml:"retention_defaults"`
}
