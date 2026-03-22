package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// MCPServerEntry describes a single MCP server in mcp.json.
type MCPServerEntry struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "http" | "stdio"
	URL       string            `json:"url"`       // required for http transport
	Command   string            `json:"command"`   // required for stdio transport
	Args      []string          `json:"args"`      // optional args for stdio transport
	Headers   map[string]string `json:"headers"`   // optional extra headers for http transport
}

// MCPConfig represents the parsed contents of an mcp.json file.
type MCPConfig struct {
	Servers []MCPServerEntry `json:"servers"`
}

// envVarPattern matches ${VAR_NAME} placeholders.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// substituteEnv replaces all ${VAR_NAME} occurrences in s with the corresponding
// environment variable value. If the variable is not set, it is replaced with an empty string.
func substituteEnv(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract var name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		return os.Getenv(varName)
	})
}

// applyEnvSubstitution applies environment variable substitution to all string
// fields in a server entry.
func applyEnvSubstitution(entry *MCPServerEntry) {
	entry.Name = substituteEnv(entry.Name)
	entry.URL = substituteEnv(entry.URL)
	entry.Command = substituteEnv(entry.Command)
	for i, arg := range entry.Args {
		entry.Args[i] = substituteEnv(arg)
	}
	for k, v := range entry.Headers {
		entry.Headers[k] = substituteEnv(v)
	}
}

// validateServerEntry checks that a server entry has all required fields.
func validateServerEntry(entry MCPServerEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("mcp: server entry missing required field 'name'")
	}
	if entry.Transport == "" {
		return fmt.Errorf("mcp: server %q missing required field 'transport'", entry.Name)
	}
	switch entry.Transport {
	case "http":
		if entry.URL == "" {
			return fmt.Errorf("mcp: server %q (http transport) missing required field 'url'", entry.Name)
		}
	case "stdio":
		if entry.Command == "" {
			return fmt.Errorf("mcp: server %q (stdio transport) missing required field 'command'", entry.Name)
		}
	default:
		return fmt.Errorf("mcp: server %q has unknown transport %q (must be 'http' or 'stdio')", entry.Name, entry.Transport)
	}
	return nil
}

// LoadMCPConfig reads and parses an mcp.json file at the given path.
// Environment variable substitution (${VAR_NAME}) is applied to all string fields.
// Returns an error if the file cannot be read, parsed, or contains invalid entries.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mcp: load config: %w", err)
	}

	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("mcp: parse config: %w", err)
	}

	for i := range cfg.Servers {
		applyEnvSubstitution(&cfg.Servers[i])
		if err := validateServerEntry(cfg.Servers[i]); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}
