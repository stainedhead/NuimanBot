package config

// NuimanBotConfig encapsulates the entire application configuration.
type NuimanBotConfig struct {
	Server      ServerConfig       `yaml:"server"`
	Security    SecurityConfig     `yaml:"security"`
	LLM         LLMConfig          `yaml:"llm"`
	Gateways    GatewaysConfig     `yaml:"gateways"`
	MCP         MCPConfig          `yaml:"mcp"`
	Storage     StorageConfig      `yaml:"storage"`
	Skills      SkillsSystemConfig `yaml:"skills"`
	Memory      MemoryConfig       `yaml:"memory"`
	ExternalAPI ExternalAPIConfig  `yaml:"external_api"`
	Tools       ToolsConfig        `yaml:"tools"`
}
