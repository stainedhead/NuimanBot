package config

import (
	"strings"
	"time"

	"nuimanbot/internal/domain"
)

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Environment Environment `yaml:"environment"`
	LogLevel    string      `yaml:"log_level"`
	Debug       bool        `yaml:"debug"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	InputMaxLength       int                        `yaml:"input_max_length"`
	TokenRotationHours   int                        `yaml:"token_rotation_hours"`
	VaultPath            string                     `yaml:"vault_path"`
	EncryptionKey        string                     `yaml:"encryption_key"`
	ToolOutputValidation ToolOutputValidationConfig `yaml:"tool_output_validation"`
	Fetch                FetchSecurityConfig        `yaml:"fetch"`
	Confirmation         ConfirmationConfig         `yaml:"confirmation"`
}

// ConfirmationConfig configures Part C's side-effecting-action confirmation
// gate (specs/260802-improve-nuimanbot-security, FR-012, FR-015). See
// internal/usecase/security.ConfirmationStore and
// internal/usecase/tool.Service, which the cmd/nuimanbot DI wiring drives
// from this config (config intentionally has no dependency on the usecase
// layer's security/tool packages).
type ConfirmationConfig struct {
	// Enabled activates the confirmation subsystem. A nil pointer (the
	// config key omitted) defaults to true — see IsEnabled. When disabled, a
	// tool/action that would otherwise require confirmation is denied
	// outright instead — disabling this subsystem never means "skip
	// confirmation and execute unconfirmed" (fail-closed default, matching
	// the security NFR in specs/260802-improve-nuimanbot-security/spec.md).
	Enabled *bool `yaml:"enabled"`
	// Timeout is how long an unresolved confirmation remains open before it
	// is treated as expired/denied (FR-015), as a duration string (e.g.
	// "5m"). Empty or unparseable resolves to DefaultConfirmationTimeout.
	Timeout string `yaml:"timeout"`
	// DefaultRequiredActions lists tool/action pairs that require
	// confirmation by default, formatted as "<tool>.<action>" (e.g.
	// "github.pr_merge") or, for tools with no action concept, just
	// "<tool>". Unioned with any per-user RULES.md requires_confirmation
	// entries (FR-012) — this list is never a replacement for RULES.md, only
	// an additional, deployment-wide floor.
	DefaultRequiredActions []string `yaml:"default_required_actions"`
}

// DefaultConfirmationTimeout is the fallback applied when Timeout is empty
// or fails to parse as a time.Duration.
const DefaultConfirmationTimeout = 5 * time.Minute

// IsEnabled reports whether the confirmation subsystem is active. Unset
// (nil) defaults to true (fail-closed / secure-by-default); only an explicit
// `enabled: false` disables it.
func (c ConfirmationConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// ResolvedTimeout parses Timeout as a time.Duration, defaulting to
// DefaultConfirmationTimeout for an empty or unparseable value.
func (c ConfirmationConfig) ResolvedTimeout() time.Duration {
	if c.Timeout == "" {
		return DefaultConfirmationTimeout
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return DefaultConfirmationTimeout
	}
	return d
}

// RequiresConfirmationByDefault reports whether toolName/action (action may
// be empty for tools with no action concept) appears in
// DefaultRequiredActions, matched case-insensitively against
// "<toolName>.<action>" (or bare "<toolName>" when action is empty).
// Returns false whenever the confirmation subsystem is disabled (IsEnabled
// == false), since a disabled subsystem must never be interpreted as "no
// action requires confirmation, so proceed" versus "confirmation isn't
// available, so this specific gate is inert" — callers combine this with
// RulesEnforcer output and are responsible for failing closed overall when
// disabled but a RulesEnforcer-driven confirmation was still requested.
func (c ConfirmationConfig) RequiresConfirmationByDefault(toolName, action string) bool {
	if !c.IsEnabled() {
		return false
	}
	key := toolName
	if action != "" {
		key = toolName + "." + action
	}
	for _, configured := range c.DefaultRequiredActions {
		if strings.EqualFold(strings.TrimSpace(configured), key) {
			return true
		}
	}
	return false
}

// FetchSecurityConfig configures SSRF protection for the tools that fetch
// third-party-controlled URLs (summarize, doc_summarize). See
// internal/usecase/tool/common.ValidateFetchURL, which the cmd/nuimanbot DI
// wiring drives from this config (config intentionally has no dependency on
// the usecase layer's tool/common package).
type FetchSecurityConfig struct {
	// SSRFProtection activates IP-resolution-based validation of fetch
	// targets — on the initial request and on every redirect hop — rejecting
	// loopback, RFC 1918 private, link-local (including cloud metadata), and
	// multicast/reserved ranges. A nil pointer (the config key omitted)
	// defaults to true — see SSRFProtectionEnabled. Fail-closed default per
	// the security NFR in specs/260802-improve-nuimanbot-security/spec.md.
	SSRFProtection *bool `yaml:"ssrf_protection"`
	// FollowRedirects controls whether the fetch HTTP clients follow HTTP
	// redirects at all. A nil pointer defaults to true. When explicitly set
	// to false, redirects are not followed: the 3xx response is returned to
	// the caller as-is rather than being validated and followed.
	FollowRedirects *bool `yaml:"follow_redirects"`
}

// SSRFProtectionEnabled reports whether SSRF validation is active. Unset
// (nil) defaults to true (fail-closed / secure-by-default); only an explicit
// `ssrf_protection: false` disables it.
func (c FetchSecurityConfig) SSRFProtectionEnabled() bool {
	return c.SSRFProtection == nil || *c.SSRFProtection
}

// FollowRedirectsEnabled reports whether the fetch HTTP clients follow
// redirects. Unset (nil) defaults to true; only an explicit
// `follow_redirects: false` disables redirect-following.
func (c FetchSecurityConfig) FollowRedirectsEnabled() bool {
	return c.FollowRedirects == nil || *c.FollowRedirects
}

// ToolOutputValidationConfig configures how third-party tool output (fetched web
// pages, search results, MCP responses) is scanned for prompt-injection patterns
// before it re-enters the LLM's conversation loop. See
// internal/usecase/security.OutputValidator, which the cmd/nuimanbot DI wiring
// constructs from this config (config intentionally has no dependency on the
// usecase layer's security package).
type ToolOutputValidationConfig struct {
	// Enabled activates tool-output injection scanning. A nil pointer (the
	// config key omitted) defaults to true — see IsEnabled. Fail-closed default
	// per the security NFR in specs/260802-improve-nuimanbot-security/spec.md.
	Enabled *bool `yaml:"enabled"`
	// Action is "reject" (default, fail closed) or "annotate" (pass flagged
	// content through wrapped with a visible warning marker instead of failing
	// the tool call). Any other/empty value resolves to "reject".
	Action string `yaml:"action"`
}

// IsEnabled reports whether tool-output validation is active. Unset (nil)
// defaults to true (fail-closed / secure-by-default); only an explicit
// `enabled: false` disables it.
func (c ToolOutputValidationConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// ResolvedAction returns the configured action ("reject" or "annotate"),
// defaulting to "reject" for any unset or unrecognized value (fail closed).
func (c ToolOutputValidationConfig) ResolvedAction() string {
	if c.Action == "annotate" {
		return "annotate"
	}
	return "reject"
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
	// ConfigFile is the path to the mcp.json server list; defaults to "mcp.json".
	ConfigFile string `yaml:"config_file"`
	// Enabled controls whether MCP client integration is active.
	Enabled bool `yaml:"enabled"`
	// AllowedServers is the list of MCP server names permitted for use.
	AllowedServers []string `yaml:"allowed_servers"`
	// Timeout is the per-request timeout for MCP calls (e.g. "30s").
	Timeout string `yaml:"timeout"`
	// MaxRetries is the number of retries on transient errors.
	MaxRetries int `yaml:"max_retries"`
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
	// Permissions is a per-tool override of the code-level ToolPermissions
	// defaults (internal/usecase/tool/permissions.go), keyed by tool name and
	// valued by role string ("guest" | "user" | "admin", case-insensitive).
	// When set for a tool, it applies uniformly to that tool's permission
	// check (including overriding github's action-aware read/write split),
	// letting an operator adjust RBAC without a code change — e.g.
	// `tools.permissions.github: user` reverts Phase 3's admin-only
	// github-write default for a deployment that relied on the old
	// permissive behavior (see FR-018a,
	// specs/260802-improve-nuimanbot-security/spec.md). An unrecognized role
	// string is ignored (falls back to the next precedence level) rather
	// than silently granting or denying access.
	Permissions map[string]string `yaml:"permissions"`
	Load        struct {
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
