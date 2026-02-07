package config

// NuimanBotConfig encapsulates the entire application configuration.
type NuimanBotConfig struct {
	Server       ServerConfig      `yaml:"server"`
	Security     SecurityConfig    `yaml:"security"`
	LLM          LLMConfig         `yaml:"llm"`
	Gateways     GatewaysConfig    `yaml:"gateways"`
	MCP          MCPConfig         `yaml:"mcp"`
	Storage      StorageConfig     `yaml:"storage"`
	Tools        ToolsSystemConfig `yaml:"tools"` // Tool registry system (renamed from Skills)
	Memory       MemoryConfig      `yaml:"memory"`
	ExternalAPI  ExternalAPIConfig `yaml:"external_api"`
	ToolSettings ToolSettings      `yaml:"tool_settings"` // Tool-specific settings (renamed from Tools)
}
