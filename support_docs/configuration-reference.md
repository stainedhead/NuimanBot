# NuimanBot Configuration Reference

**Version:** 1.0
**Last Updated:** 2026-02-08
**Configuration Format:** YAML

---

## Table of Contents

1. [Overview](#overview)
2. [Configuration Structure](#configuration-structure)
3. [Server Configuration](#server-configuration)
4. [Security Configuration](#security-configuration)
5. [Storage Configuration](#storage-configuration)
6. [LLM Configuration](#llm-configuration)
7. [Gateway Configuration](#gateway-configuration)
8. [Tools Configuration](#tools-configuration)
9. [Skills Configuration](#skills-configuration)
10. [MCP Configuration](#mcp-configuration)
11. [Environment Variables](#environment-variables)
12. [Examples](#examples)

---

## Overview

### Configuration Files

NuimanBot uses YAML configuration with environment variable override support:

| File | Location | Purpose |
|------|----------|---------|
| `config.yaml` | Project root | Primary configuration |
| `.env` | Project root | Environment variables (optional) |

### Configuration Priority

1. Environment variables (highest priority)
2. config.yaml file
3. Default values (lowest priority)

### Environment Variable Format

```
NUIMANBOT_{SECTION}_{SUBSECTION}_{KEY}
```

**Examples:**
- `NUIMANBOT_SERVER_LOG_LEVEL=debug`
- `NUIMANBOT_LLM_PROVIDERS_0_API_KEY=sk-...`
- `NUIMANBOT_SECURITY_VAULT_PATH=/secure/vault.enc`

---

## Configuration Structure

### Minimal config.yaml

```yaml
server:
  log_level: info

security:
  vault_path: "./data/vault.enc"

storage:
  type: sqlite
  dsn: "./data/nuimanbot.db"

llm:
  default_model:
    primary: anthropic/claude-sonnet
  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"

gateways:
  cli:
    enabled: true
```

### Complete config.yaml Template

See [Examples](#examples) section for full configuration templates.

---

## Server Configuration

### server Section

Controls server-level settings including logging, debugging, and path management.

```yaml
server:
  log_level: info              # Log verbosity
  debug: false                 # Debug mode
  port: 8080                   # HTTP server port (optional)
  paths:                       # Path configuration (optional)
    config: "./config/"        # Config directory
    data: "./data/"            # Data directory
    logs: "./logs/"            # Logs directory
```

### Fields

#### `server.log_level`

**Type:** string
**Default:** `info`
**Options:** `debug`, `info`, `warn`, `error`
**Description:** Sets the logging verbosity level.

**Environment Variable:** `NUIMANBOT_SERVER_LOG_LEVEL`

**Examples:**
```yaml
server:
  log_level: debug  # Verbose logging for development
```

```bash
export NUIMANBOT_SERVER_LOG_LEVEL=error  # Production: errors only
```

#### `server.debug`

**Type:** boolean
**Default:** `false`
**Description:** Enables debug mode with additional logging and diagnostics.

**Environment Variable:** `NUIMANBOT_SERVER_DEBUG`

**Examples:**
```yaml
server:
  debug: true  # Enable debug mode
```

#### `server.port`

**Type:** integer
**Default:** `8080`
**Description:** HTTP server port for REST API and web interface.

**Environment Variable:** `NUIMANBOT_SERVER_PORT`

**Examples:**
```yaml
server:
  port: 9090  # Custom port
```

#### `server.paths`

**Type:** object
**Description:** Configures separate paths for different file types (container-friendly).

**Subfields:**
- `config` (string): Configuration files directory
- `data` (string): Data files directory (users, bots, databases)
- `logs` (string): Log files directory

**Environment Variables:**
- `NUIMANBOT_SERVER_PATHS_CONFIG`
- `NUIMANBOT_SERVER_PATHS_DATA`
- `NUIMANBOT_SERVER_PATHS_LOGS`

**Examples:**
```yaml
server:
  paths:
    config: "/etc/nuimanbot"
    data: "/var/lib/nuimanbot"
    logs: "/var/log/nuimanbot"
```

```bash
# Container deployment
export NUIMANBOT_SERVER_PATHS_CONFIG=/config
export NUIMANBOT_SERVER_PATHS_DATA=/data
export NUIMANBOT_SERVER_PATHS_LOGS=/logs
```

---

## Security Configuration

### security Section

Configures security settings including input validation, encryption, and credential storage.

```yaml
security:
  input_max_length: 4096       # Max input length (characters)
  vault_path: "./data/vault.enc"  # Encrypted credential vault
```

### Fields

#### `security.input_max_length`

**Type:** integer
**Default:** `4096`
**Description:** Maximum allowed length for user input (prevents abuse).

**Environment Variable:** `NUIMANBOT_SECURITY_INPUT_MAX_LENGTH`

#### `security.vault_path`

**Type:** string
**Default:** `./data/vault.enc`
**Description:** Path to encrypted credential vault file.

**Environment Variable:** `NUIMANBOT_SECURITY_VAULT_PATH`

### Encryption Key (Required)

**Environment Variable:** `NUIMANBOT_ENCRYPTION_KEY`
**Type:** string (32 bytes)
**Required:** Yes
**Description:** Encryption key for bot tokens and sensitive data.

**Generate:**
```bash
export NUIMANBOT_ENCRYPTION_KEY=$(openssl rand -base64 32 | head -c 32)
```

**Security Notes:**
- Must be exactly 32 bytes for AES-256
- Store securely (secrets manager, environment)
- Never commit to version control
- Rotate periodically

---

## Storage Configuration

### storage Section

Configures database and persistent storage.

```yaml
storage:
  type: sqlite                 # Storage backend type
  dsn: "./data/nuimanbot.db"   # Data source name
```

### Fields

#### `storage.type`

**Type:** string
**Default:** `sqlite`
**Options:** `sqlite` (only supported type currently)
**Description:** Storage backend type.

**Environment Variable:** `NUIMANBOT_STORAGE_TYPE`

#### `storage.dsn`

**Type:** string
**Default:** `./data/nuimanbot.db`
**Description:** Data source name (database file path for SQLite).

**Environment Variable:** `NUIMANBOT_STORAGE_DSN`

**Examples:**
```yaml
storage:
  type: sqlite
  dsn: "/var/lib/nuimanbot/nuimanbot.db"
```

---

## LLM Configuration

### llm Section

Configures LLM providers and model settings.

```yaml
llm:
  default_model:
    primary: anthropic/claude-sonnet
    fallback: openai/gpt-4
  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"
    - id: openai-main
      type: openai
      api_key: "${OPENAI_API_KEY}"
```

### Fields

#### `llm.default_model`

**Type:** object
**Description:** Default model selection.

**Subfields:**
- `primary` (string): Primary model to use
- `fallback` (string): Fallback model if primary fails

**Format:** `{provider}/{model}`

**Examples:**
```yaml
llm:
  default_model:
    primary: anthropic/claude-sonnet-4-5
    fallback: openai/gpt-4
```

#### `llm.providers`

**Type:** array of objects
**Description:** LLM provider configurations.

**Provider Object Fields:**
- `id` (string): Unique provider identifier
- `type` (string): Provider type (anthropic, openai, bedrock, ollama)
- `api_key` (string): API key for authentication
- Additional provider-specific fields

**Environment Variables:**
```bash
# Provider-specific API keys
export NUIMANBOT_LLM_PROVIDERS_0_API_KEY=sk-...  # First provider
export NUIMANBOT_LLM_PROVIDERS_1_API_KEY=sk-...  # Second provider
```

### Provider Types

#### Anthropic

```yaml
llm:
  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"
      base_url: "https://api.anthropic.com"  # Optional
      max_tokens: 200000                      # Optional
```

#### OpenAI

```yaml
llm:
  providers:
    - id: openai-main
      type: openai
      api_key: "${OPENAI_API_KEY}"
      base_url: "https://api.openai.com/v1"  # Optional
      organization_id: "org-..."              # Optional
```

#### AWS Bedrock

```yaml
llm:
  providers:
    - id: bedrock-main
      type: bedrock
      region: "us-east-1"
      access_key_id: "${AWS_ACCESS_KEY_ID}"
      secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
```

#### Ollama (Local)

```yaml
llm:
  providers:
    - id: ollama-local
      type: ollama
      base_url: "http://localhost:11434"
      model: "llama2"
```

---

## Gateway Configuration

### gateways Section

Configures messaging gateways (CLI, Slack, Telegram).

```yaml
gateways:
  cli:
    enabled: true
    debug_mode: false
    history_file: ".nuimanbot_history"
  slack:
    enabled: false
    # Configuration moved to bots.json
  telegram:
    enabled: false
    # Configuration moved to bots.json
```

### Fields

#### `gateways.cli`

**Type:** object
**Description:** CLI gateway configuration.

**Subfields:**
- `enabled` (boolean): Enable CLI gateway
- `debug_mode` (boolean): Enable debug output
- `history_file` (string): Command history file path

**Environment Variables:**
- `NUIMANBOT_GATEWAYS_CLI_ENABLED`
- `NUIMANBOT_GATEWAYS_CLI_DEBUG_MODE`

**Examples:**
```yaml
gateways:
  cli:
    enabled: true
    debug_mode: false
    history_file: "~/.nuimanbot_history"
```

#### `gateways.slack`

**Type:** object
**Description:** Slack gateway enable/disable flag.

**Note:** Bot configurations (tokens, settings) are now managed via `bots.json` and the admin API/CLI.

**Subfields:**
- `enabled` (boolean): Enable Slack gateway

**Environment Variable:** `NUIMANBOT_GATEWAYS_SLACK_ENABLED`

**Examples:**
```yaml
gateways:
  slack:
    enabled: true
```

#### `gateways.telegram`

**Type:** object
**Description:** Telegram gateway enable/disable flag.

**Note:** Bot configurations (tokens, settings) are now managed via `bots.json` and the admin API/CLI.

**Subfields:**
- `enabled` (boolean): Enable Telegram gateway

**Environment Variable:** `NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED`

**Examples:**
```yaml
gateways:
  telegram:
    enabled: true
```

---

## Tools Configuration

### tools Section

Configures available tools and their settings.

```yaml
tools:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
    websearch:
      enabled: true
      api_key: "${SEARCH_API_KEY}"
    weather:
      enabled: true
      api_key: "${WEATHER_API_KEY}"
    notes:
      enabled: true
```

### Fields

#### `tools.entries`

**Type:** object (map)
**Description:** Map of tool name to tool configuration.

**Common Subfields:**
- `enabled` (boolean): Enable/disable tool
- Tool-specific configuration fields

**Environment Variables:**
```bash
export NUIMANBOT_TOOLS_ENTRIES_CALCULATOR_ENABLED=true
export NUIMANBOT_TOOLS_ENTRIES_WEBSEARCH_API_KEY=your-key
```

### Built-in Tools

#### calculator

```yaml
tools:
  entries:
    calculator:
      enabled: true
```

Basic arithmetic and math operations.

#### datetime

```yaml
tools:
  entries:
    datetime:
      enabled: true
```

Date/time queries and conversions.

#### websearch

```yaml
tools:
  entries:
    websearch:
      enabled: true
      api_key: "${SEARCH_API_KEY}"
      max_results: 10
```

Web search functionality (requires API key).

#### weather

```yaml
tools:
  entries:
    weather:
      enabled: true
      api_key: "${OPENWEATHERMAP_API_KEY}"
```

Weather information (requires OpenWeatherMap API key).

#### notes

```yaml
tools:
  entries:
    notes:
      enabled: true
      storage_path: "./data/notes"
```

User note-taking functionality.

---

## Skills Configuration

### skills Section

Configures the Agent Skills system (Anthropic-style file-based skills).

```yaml
skills:
  enabled: true
  roots:
    - path: "./data/skills/shared"
      scope: 2  # ScopeProject
    - path: "./data/skills/users/cli_user"
      scope: 1  # ScopeUser
```

### Fields

#### `skills.enabled`

**Type:** boolean
**Default:** `true`
**Description:** Enable the skills system.

**Environment Variable:** `NUIMANBOT_SKILLS_ENABLED`

#### `skills.roots`

**Type:** array of objects
**Description:** Skill directory roots with scopes.

**Root Object Fields:**
- `path` (string): Directory path containing skills
- `scope` (integer): Skill scope (0=Enterprise, 1=User, 2=Project)

**Scope Values:**
- `0`: Enterprise (highest priority)
- `1`: User-specific
- `2`: Project/shared (lowest priority)

**Examples:**
```yaml
skills:
  enabled: true
  roots:
    # Enterprise skills (highest priority)
    - path: "/etc/nuimanbot/skills/enterprise"
      scope: 0
    # User-specific skills
    - path: "./data/skills/users/cli_admin"
      scope: 1
    # Shared project skills
    - path: "./data/skills/shared"
      scope: 2
```

---

## MCP Configuration

### mcp Section

Configures Model Context Protocol client settings.

```yaml
mcp:
  client:
    timeout: "30s"
```

### Fields

#### `mcp.client.timeout`

**Type:** duration string
**Default:** `30s`
**Description:** Timeout for MCP client requests.

**Environment Variable:** `NUIMANBOT_MCP_CLIENT_TIMEOUT`

**Format:** Duration string (e.g., "30s", "1m", "500ms")

---

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `NUIMANBOT_ENCRYPTION_KEY` | 32-byte encryption key (required) |

### Server

| Variable | Default | Description |
|----------|---------|-------------|
| `NUIMANBOT_SERVER_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `NUIMANBOT_SERVER_DEBUG` | false | Debug mode |
| `NUIMANBOT_SERVER_PORT` | 8080 | HTTP server port |
| `NUIMANBOT_SERVER_PATHS_CONFIG` | ./config/ | Config directory |
| `NUIMANBOT_SERVER_PATHS_DATA` | ./data/ | Data directory |
| `NUIMANBOT_SERVER_PATHS_LOGS` | ./logs/ | Logs directory |

### Security

| Variable | Default | Description |
|----------|---------|-------------|
| `NUIMANBOT_SECURITY_INPUT_MAX_LENGTH` | 4096 | Max input length |
| `NUIMANBOT_SECURITY_VAULT_PATH` | ./data/vault.enc | Vault file path |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `NUIMANBOT_STORAGE_TYPE` | sqlite | Storage backend |
| `NUIMANBOT_STORAGE_DSN` | ./data/nuimanbot.db | Database path |

### LLM

| Variable | Description |
|----------|-------------|
| `NUIMANBOT_LLM_DEFAULT_MODEL_PRIMARY` | Primary model |
| `NUIMANBOT_LLM_DEFAULT_MODEL_FALLBACK` | Fallback model |
| `NUIMANBOT_LLM_PROVIDERS_0_API_KEY` | First provider API key |
| `NUIMANBOT_LLM_PROVIDERS_1_API_KEY` | Second provider API key |

### Gateways

| Variable | Default | Description |
|----------|---------|-------------|
| `NUIMANBOT_GATEWAYS_CLI_ENABLED` | true | Enable CLI gateway |
| `NUIMANBOT_GATEWAYS_SLACK_ENABLED` | false | Enable Slack gateway |
| `NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED` | false | Enable Telegram gateway |

---

## Examples

### Development Configuration

```yaml
server:
  log_level: debug
  debug: true
  paths:
    config: "./config/"
    data: "./data/"
    logs: "./logs/"

security:
  input_max_length: 4096
  vault_path: "./data/vault.enc"

storage:
  type: sqlite
  dsn: "./data/nuimanbot.db"

llm:
  default_model:
    primary: anthropic/claude-sonnet-4-5
  providers:
    - id: anthropic-dev
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"

gateways:
  cli:
    enabled: true
    debug_mode: true
    history_file: ".nuimanbot_history"
  slack:
    enabled: false
  telegram:
    enabled: false

tools:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
    notes:
      enabled: true

skills:
  enabled: true
  roots:
    - path: "./data/skills/shared"
      scope: 2

mcp:
  client:
    timeout: "30s"
```

### Production Configuration

```yaml
server:
  log_level: info
  debug: false
  port: 8080
  paths:
    config: "/etc/nuimanbot"
    data: "/var/lib/nuimanbot"
    logs: "/var/log/nuimanbot"

security:
  input_max_length: 4096
  vault_path: "/var/lib/nuimanbot/vault.enc"

storage:
  type: sqlite
  dsn: "/var/lib/nuimanbot/nuimanbot.db"

llm:
  default_model:
    primary: anthropic/claude-sonnet-4-5
    fallback: openai/gpt-4
  providers:
    - id: anthropic-prod
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"
    - id: openai-prod
      type: openai
      api_key: "${OPENAI_API_KEY}"

gateways:
  cli:
    enabled: true
    debug_mode: false
    history_file: "/var/lib/nuimanbot/.history"
  slack:
    enabled: true
  telegram:
    enabled: true

tools:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
    websearch:
      enabled: true
      api_key: "${SEARCH_API_KEY}"
    weather:
      enabled: true
      api_key: "${WEATHER_API_KEY}"
    notes:
      enabled: true

skills:
  enabled: true
  roots:
    - path: "/etc/nuimanbot/skills/enterprise"
      scope: 0
    - path: "/var/lib/nuimanbot/skills/shared"
      scope: 2

mcp:
  client:
    timeout: "30s"
```

### Container Configuration

```yaml
server:
  log_level: info
  debug: false
  paths:
    config: "/config"
    data: "/data"
    logs: "/logs"

security:
  input_max_length: 4096
  vault_path: "/data/vault.enc"

storage:
  type: sqlite
  dsn: "/data/nuimanbot.db"

llm:
  default_model:
    primary: anthropic/claude-sonnet-4-5
  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "${ANTHROPIC_API_KEY}"

gateways:
  cli:
    enabled: false  # Disabled in container
  slack:
    enabled: true
  telegram:
    enabled: true

tools:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true

skills:
  enabled: true
  roots:
    - path: "/data/skills/shared"
      scope: 2

mcp:
  client:
    timeout: "30s"
```

**Docker Compose:**
```yaml
version: '3.8'
services:
  nuimanbot:
    image: nuimanbot:latest
    environment:
      - NUIMANBOT_ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - NUIMANBOT_SERVER_PATHS_CONFIG=/config
      - NUIMANBOT_SERVER_PATHS_DATA=/data
      - NUIMANBOT_SERVER_PATHS_LOGS=/logs
    volumes:
      - ./config:/config:ro
      - ./data:/data
      - ./logs:/logs
    ports:
      - "8080:8080"
```

---

## Validation

### Validate Configuration

**Via REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/config/validate \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d @config.json
```

**Via YAML Linter:**
```bash
# Install yq
brew install yq  # macOS
# or: snap install yq

# Validate YAML syntax
yq eval config.yaml
```

---

## Hot Reload

Configuration can be reloaded without restarting:

**REST API:**
```bash
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer ${API_KEY}"
```

**STDIO (if server daemon running):**
```
/refresh
```

**What Gets Reloaded:**
- LLM provider settings
- Gateway enable/disable flags
- Tool enable/disable flags
- Logging configuration

**What Requires Restart:**
- Server port changes
- Storage backend changes
- Path configuration changes

---

## Best Practices

1. **Use Environment Variables for Secrets**
   - Never commit API keys to version control
   - Use environment variables or secrets management

2. **Separate Paths in Production**
   - Config: `/etc/nuimanbot`
   - Data: `/var/lib/nuimanbot`
   - Logs: `/var/log/nuimanbot`

3. **Enable Only Needed Gateways**
   - Disable unused gateways to save resources
   - Use bot management API/CLI for bot configuration

4. **Configure Log Levels Appropriately**
   - Development: `debug`
   - Production: `info` or `warn`
   - Error investigation: `debug` temporarily

5. **Backup Configuration**
   - Version control `config.yaml` (without secrets)
   - Backup encryption key securely
   - Document environment-specific settings

---

**Document Version:** 1.0
**Last Updated:** 2026-02-08
**Maintainer:** NuimanBot Development Team
