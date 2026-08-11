# NuimanBot Configuration Reference

**Version:** 1.2
**Last Updated:** 2026-08-05
**Configuration Format:** YAML

---

## Table of Contents

1. [Overview](#overview)
2. [Configuration Structure](#configuration-structure)
3. [Server Configuration](#server-configuration)
4. [Security Configuration](#security-configuration)
5. [Storage Configuration](#storage-configuration)
6. [Network Access & Workspace Configuration](#network-access--workspace-configuration)
7. [LLM Configuration](#llm-configuration)
8. [Gateway Configuration](#gateway-configuration)
9. [Tools Configuration](#tools-configuration)
10. [Skills Configuration](#skills-configuration)
11. [MCP Configuration](#mcp-configuration)
12. [Environment Variables](#environment-variables)
13. [Examples](#examples)

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
  type: file
  dsn: "./data"

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

#### `security.tool_output_validation`

**Type:** object
**Description:** Scans content fetched by tools (web pages, search results, MCP responses) for prompt-injection patterns before it reaches the LLM. See the [Security Hardening Guide](security-hardening-guide.md) for background.

**Subfields:**
- `enabled` (boolean): Enables tool-output injection scanning. Omitted/unset defaults to `true` (fail-closed/secure-by-default) — only an explicit `false` disables it.
- `action` (string): `reject` (default) fails the tool call when flagged content is detected; `annotate` passes the content through wrapped with a visible `[SECURITY WARNING: possible injected instructions detected]` marker instead of failing.

**Examples:**
```yaml
security:
  tool_output_validation:
    enabled: true
    action: reject
```

#### `security.confirmation`

**Type:** object
**Description:** Configures the side-effecting-action confirmation gate — certain tool actions (GitHub PR merges, issue create/close, `coding_agent` yolo mode, write/unknown-trust MCP tools) pause and require explicit human yes/no approval before executing. See the [Security Hardening Guide](security-hardening-guide.md) for the user-facing behavior.

**Subfields:**
- `enabled` (boolean): Activates the confirmation subsystem. Omitted/unset defaults to `true`. Disabling this does **not** mean gated actions run unconfirmed — a `RULES.md`-driven confirmation requirement is still denied, not silently allowed, when this is `false`.
- `timeout` (string): Duration string (e.g. `"5m"`, `"90s"`) for how long an unresolved confirmation stays open before it expires and is treated as denied. Empty or unparseable resolves to the default.
- `default_required_actions` (array of strings): Tool/action pairs requiring confirmation by default, formatted `"<tool>.<action>"` (e.g. `"github.pr_merge"`) or a bare `"<tool>"` for tools with no action concept. Unioned with (not a replacement for) any per-user `RULES.md` `requires_confirmation` entries.

**Default:** `timeout: "5m"`

**Examples:**
```yaml
security:
  confirmation:
    enabled: true
    timeout: "5m"
    default_required_actions:
      - "github.pr_merge"
      - "github.issue_close"
      - "github.issue_create"
      - "coding_agent.yolo_mode"
```

#### `security.fetch`

**Type:** object
**Description:** Configures SSRF (Server-Side Request Forgery) protection for tools that fetch third-party URLs (`summarize`, `doc_summarize`).

**Subfields:**
- `ssrf_protection` (boolean): Rejects fetch targets that resolve to loopback, RFC 1918 private, link-local (including cloud metadata addresses like `169.254.169.254`), or multicast/reserved IP ranges, on both the initial request and every redirect hop. Omitted/unset defaults to `true`.
- `follow_redirects` (boolean): Whether the fetch tools follow HTTP redirects at all. Omitted/unset defaults to `true`. Setting to `false` returns the 3xx response as-is instead of following it (restores Go's stock, non-SSRF-aware default redirect policy rather than a custom one).

**Examples:**
```yaml
security:
  fetch:
    ssrf_protection: true
    follow_redirects: true
```

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
  type: file                 # Storage backend type
  dsn: "./data"   # Data source name
```

### Fields

#### `storage.type`

**Type:** string
**Default:** `file`
**Options:** `file` (file-based JSON storage)
**Description:** Storage backend type.

**Environment Variable:** `NUIMANBOT_STORAGE_TYPE`

#### `storage.dsn`

**Type:** string
**Default:** `./data`
**Description:** Data source name (data directory path for file-based storage).

**Environment Variable:** `NUIMANBOT_STORAGE_DSN`

**Examples:**
```yaml
storage:
  type: file
  dsn: "/var/lib/nuimanbot"
```

---

## Network Access & Workspace Configuration

Configures the web workspace's network exposure (localhost-only vs. remote), the shared Job/Chore worker pool size, and default retention windows for Chats/Projects/History. See the [Web Workspace Guide](web-workspace-guide.md) for what these settings control from a user's perspective, and the [Admin Guide](admin-guide.md) for operational guidance.

**Note:** as of this writing, `worker_pool.max_concurrent_workers` and `network_access.mode` can also be changed live from the Settings page in the web UI; `network_access.allowlist` and `network_access.bind_address` are config-file-only — there is no UI control for either yet, and changing `mode` from the UI does not rebind the running server to a new address (the bind address is only read once, at startup).

### network_access Section

Controls whether the web admin server accepts connections from other machines, and if so, from which sources.

```yaml
network_access:
  mode: localhost_only        # localhost_only | remote
  # bind_address: "0.0.0.0:8443"   # only used when mode: remote
  # allowlist:
  #   - "203.0.113.9"
  #   - "trusted.example.com"
```

### Fields

#### `network_access.mode`

**Type:** string
**Default:** `localhost_only`
**Options:** `localhost_only` (binds `127.0.0.1` only), `remote` (binds `bind_address`)
**Description:** Controls whether the web admin server is reachable from other machines. Omitting this whole section is equivalent to explicitly setting `localhost_only` with no allowlist — existing single-machine deployments are unaffected by upgrading to a version that includes this feature. An unrecognized or malformed value is treated as `localhost_only`, never `remote` — a config typo must never silently open remote access.

**Environment Variable:** none yet — this section is `config.yaml`-only for now; unlike most sections in this reference, it has no per-field environment variable override wired up.

#### `network_access.bind_address`

**Type:** string
**Default:** (unset)
**Description:** The interface/port the server binds when `mode: remote` (e.g. `"0.0.0.0:8443"`). Ignored in `localhost_only` mode. Takes effect only at process startup — changing it requires a restart, and it cannot currently be changed from the Settings UI.

**Environment Variable:** none yet — `config.yaml`-only.

#### `network_access.allowlist`

**Type:** list of strings (IPs or hostnames)
**Default:** unset (absent)
**Description:** When `mode: remote`, restricts which client sources may reach the server. Enforced by a middleware layer ahead of every request — including unauthenticated endpoints like `/health` — before authentication runs; a rejected source gets HTTP 403 without reaching any application code. Cannot currently be changed from the Settings UI.

**IMPORTANT — absent vs. empty are NOT equivalent:**
- **Omitting `allowlist` entirely** (or leaving out the whole `network_access` section) = allow all remote sources once `mode: remote` is set. This is an explicit admin choice to open access with no source restriction.
- **`allowlist: []`** (present but empty) = deny **all** sources, fail-closed. Every remote request is rejected.
- **`allowlist: [...]`** with one or more entries = only those sources are allowed; everything else is rejected.

Double-check which of these three states you intend — the first and second look similar in a diff but have opposite effects.

**Environment Variable:** none yet — `config.yaml`-only.

---

### worker_pool Section

Configures the shared FIFO worker pool that executes Job and Chore runs.

```yaml
worker_pool:
  max_concurrent_workers: 3
```

#### `worker_pool.max_concurrent_workers`

**Type:** integer
**Default:** `3`
**Description:** Maximum number of Job/Chore runs that may execute concurrently, system-wide across all users, from one shared FIFO queue. An unset or non-positive value falls back to the default (3) rather than failing startup. Live-editable from the Settings page; reducing it never interrupts a run already in progress — it simply stops new runs from starting until the active count drops to the new limit.

**Environment Variable:** none yet — `config.yaml`-only, though it can also be changed live from the Settings page in the web UI.

---

### retention_defaults Section

Configures the default number of days Chats, Projects, and History (Job/Chore run) records are kept before automatic deletion.

```yaml
retention_defaults:
  chat_days: 90
  project_days: 180
  history_days: 90
```

#### `retention_defaults.chat_days` / `.project_days` / `.history_days`

**Type:** integer (days)
**Default:** `90` / `180` / `90` respectively
**Description:** Default retention window for each resource type. `0` (or omitting the field) means **"Never"** — no automatic expiry — not "expire immediately." Shown as the system-wide default on the Settings page.

**Current status:** these values are stored and displayed, but as of this writing no scheduled process actually sweeps and deletes expired records yet — see the [Web Workspace Guide](web-workspace-guide.md#what-to-expect-today) for the full list of what's operational today versus still pending.

**Environment Variable:** none yet — `config.yaml`-only.

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

#### `tools.permissions`

**Type:** object (map of tool name to role)
**Description:** Overrides the default RBAC role required to execute a specific tool, without a code change. Applies at whole-tool granularity (all of that tool's actions), taking precedence over both the built-in action-aware split (e.g. `github`'s read/write distinction) and the static default role table.

**Breaking change note:** `github` and `coding_agent` require the `admin` role by default as of the security-hardening update (previously they fell through to the general `user` default). Use this section to revert that for a specific deployment, or grant affected users the Admin role instead.

**Valid values:** `guest`, `user`, `admin` (case-insensitive, whitespace-trimmed). An unrecognized value is logged as a warning and ignored — it falls through to the built-in default rather than silently granting or denying access.

**Environment Variables:**
```bash
export NUIMANBOT_TOOLS_PERMISSIONS_GITHUB=user
```

**Examples:**
```yaml
tools:
  permissions:
    github: user          # revert github to the pre-hardening default
    coding_agent: admin    # explicit (already the default)
```

See the [Security Hardening Guide](security-hardening-guide.md) for full context.

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

### Per-Server Trust Classification (`mcp.json`)

MCP server definitions (tokens, transport, URL/command) live in a separate
`mcp.json` file, not `config.yaml` — see `mcp.client.timeout` above for the
one MCP-related setting that does live in `config.yaml`. Each server entry in
`mcp.json` supports an optional `trust` field and `tool_overrides` map, used
to determine RBAC role and confirmation requirements for that server's tools
under the dynamic `mcp:<server>:<tool>` namespace (built-in tools use
`tools.permissions` instead — see [Tools Configuration](#tools-configuration)).

#### `trust`

**Type:** string
**Options:** `read_only`, `write`, `unknown`
**Default:** `unknown` (omitted or any unrecognized value normalizes to this)
**Description:** Classifies this server's tools overall. `read_only` tools are available to regular users with no confirmation required. `write` and `unknown` tools are permission-checked as admin-only and require confirmation before executing — `unknown` exists specifically as the safe default for a server you haven't explicitly classified, so it is treated identically to `write`, not as a looser third tier.

#### `tool_overrides`

**Type:** object (map of tool name to trust level)
**Description:** Sets a per-tool trust exception overriding the server-wide `trust` value for one specific tool reported by that server (same `read_only`/`write`/`unknown` values and normalization rules as `trust`).

**Known limitation:** the map key is the tool name exactly as the MCP server self-reports it via `tools/list` — NuimanBot cannot independently verify that this name refers to the tool the operator actually intended. If you use `tool_overrides` to grant a specific MCP tool `read_only` trust, only do so for MCP servers you fully trust to correctly and honestly name their tools — a malicious server could exploit an override meant for a different tool by registering its own write-capable tool under that same name. The server-wide `trust` default (used when no `tool_overrides` entry matches) is unaffected by this, since it applies uniformly regardless of tool name.

**Example `mcp.json` entry:**
```json
{
  "servers": [
    {
      "name": "my-mcp-server",
      "transport": "http",
      "url": "https://mcp.example.com",
      "trust": "read_only",
      "tool_overrides": {
        "delete_record": "write"
      }
    }
  ]
}
```

A malformed/unrecognized `trust` or `tool_overrides` value is logged as a
warning and normalized to `unknown` — it never fails startup and never
silently resolves to a looser classification.

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
| `NUIMANBOT_SECURITY_TOOL_OUTPUT_VALIDATION_ENABLED` | true | Enable tool-output injection scanning |
| `NUIMANBOT_SECURITY_TOOL_OUTPUT_VALIDATION_ACTION` | reject | `reject` or `annotate` flagged tool output |
| `NUIMANBOT_SECURITY_CONFIRMATION_ENABLED` | true | Enable side-effecting-action confirmation gate |
| `NUIMANBOT_SECURITY_CONFIRMATION_TIMEOUT` | 5m | Confirmation expiry (auto-denied after) |
| `NUIMANBOT_SECURITY_FETCH_SSRF_PROTECTION` | true | Enable SSRF protection on fetch tools |
| `NUIMANBOT_SECURITY_FETCH_FOLLOW_REDIRECTS` | true | Follow (validated) redirects on fetch tools |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `NUIMANBOT_STORAGE_TYPE` | file | Storage backend |
| `NUIMANBOT_STORAGE_DSN` | ./data | Data directory path |

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
  type: file
  dsn: "./data"

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
  type: file
  dsn: "/var/lib/nuimanbot"

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
  type: file
  dsn: "/data"

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

**Document Version:** 1.1
**Last Updated:** 2026-08-03
**Maintainer:** NuimanBot Development Team
