# Improved Admin Features - Product Requirements Document

## Overview

This document defines requirements for enhanced user profile management and admin features in NuimanBot. The primary goal is to expand the current User model with comprehensive profile information and provide admin tools for managing user profiles effectively.

### Key Requirements

#### Architecture & Administration
1. **Multi-Component Architecture**: Core server (daemon) + CLI tool + embedded web admin interface
2. **STDIO Simplification**: Only /refresh and /exit commands for server control
3. **Web Admin Interface**: Browser-based configuration embedded in server (port 8080)
4. **CLI Tool**: Standalone utility for configuration and server management
5. **Hot Configuration Reload**: Server can reload config files without restart

#### Configuration & Infrastructure
1. **Configuration Restructuring**: Modern, container-friendly configuration with path management, enable/disable flags, and provider inheritance
2. **Path Separation**: Distinct paths for config, data, and logs to support containerized deployments
3. **Dynamic Enable/Disable**: Gateways and providers can be enabled/disabled without code changes or config restructuring
4. **Provider Flexibility**: Support multiple instances of same provider type with inheritance-based configuration

#### User Profiles & Personalization
5. **Comprehensive User Profiles**: Expand beyond basic authentication to capture full user identity, preferences, and organizational context
6. **Multi-Platform Integration**: Support Slack, Telegram, and other platform integrations via unique identifiers
7. **Agent Personalization**: Store user preferences to customize agent behavior and communication style

#### API & Access Control
8. **REST API Management**: **All data elements MUST be manageable via REST API with full CRUD operations for admin users**
9. **Partial Updates**: PUT operations support updating only specified fields without requiring complete object replacement
10. **Role-Based Access Control**: Admin users have full access; regular users can only manage their own profiles
11. **Audit Logging**: All admin modifications tracked with timestamp and admin user ID

#### Bot Gateway
12. **Database-Driven Bot Management**: Bot configurations stored in database (slack_bots, telegram_bots) instead of config files
13. **Public/Private Bots**: Support both shared public bots and user-specific private bots
14. **Dynamic Bot Control**: Enable/disable bots without gateway restart

## Current State

### Existing User Model

The system currently has:

**User Entity** (`internal/domain/user.go`):
- `ID` (UUID) - Unique identifier
- `Username` (string) - Login name
- `Email` (string) - Primary email
- `Role` (UserRole) - Admin/User/Service
- `APIKeys` ([]APIKey) - Authentication keys
- `Metadata` (map[string]string) - Flexible key-value storage
- Timestamps: CreatedAt, UpdatedAt, LastLoginAt

**UserPreferences Entity** (`internal/domain/user.go`):
- `UserID` (UUID) - References User
- `DefaultModel` (string) - Primary LLM model
- `PreferredModels` ([]string) - Fallback models
- `ConversationSettings` (ConversationSettings) - Chat preferences
- Timestamps: CreatedAt, UpdatedAt

**Current Limitations**:
- No structured storage for personal information (names, phone, location)
- No multi-language support tracking
- No integration IDs for external platforms (Slack, Telegram)
- No job role or organizational context
- Limited ability to customize agent behavior per user

## Proposed Enhancement: UserProfiles Object

### Design Goals

1. **Comprehensive Identity**: Capture full user identity information beyond authentication
2. **Multi-Platform Integration**: Support for multiple communication platforms
3. **Internationalization**: Track language preferences for multi-lingual support
4. **Agent Personalization**: Store preferences for how the agent interacts with each user
5. **Organizational Context**: Support enterprise use cases with job roles and user types
6. **Backward Compatibility**: Extend existing User model without breaking changes

### UserProfiles Data Model

**Primary Fields**:

| Field | Type | Description | Required | Constraints |
|-------|------|-------------|----------|-------------|
| `userID` | UUID | References User.ID (primary key) | Yes | Foreign key to User |
| `moniker` | string | Display name or handle | No | Max 50 chars |
| `firstName` | string | Given name | No | Max 100 chars |
| `lastName` | string | Family name | No | Max 100 chars |
| `nickName` | string | Preferred informal name | No | Max 50 chars |
| `primaryLanguage` | string | ISO 639-1 language code (e.g., "en", "es") | Yes | Default: "en" |
| `secondaryLanguage` | string | ISO 639-1 language code for fallback | No | - |
| `primaryLocation` | string | Geographic location or timezone identifier | No | Max 100 chars |
| `primaryEmail` | string | Primary contact email | Yes | Valid email format |
| `backupEmail` | string | Secondary contact email | No | Valid email format |
| `mobilePhone` | string | Mobile phone number | No | E.164 format preferred |
| `timezone` | string | IANA timezone (e.g., "America/New_York") | Yes | Default: "UTC" |
| `jobRole` | string | User's organizational role | No | Max 100 chars |
| `profileInfo` | JSON/Text | Freeform biographical information | No | Max 2000 chars |
| `userType` | UserType | Enum: Individual, Enterprise, Developer, Admin | Yes | Default: Individual |
| `slackID` | string | Slack user ID for integration | No | Unique if present |
| `telegramID` | string | Telegram user ID for integration | No | Unique if present |
| `agentPreferences` | JSON | Structured agent behavior preferences | No | See schema below |
| `notesInformation` | JSON | Admin notes and flags | No | See schema below |

**Timestamps**:
- `createdAt` - Profile creation time
- `updatedAt` - Last modification time
- `lastVerified` - Last time user verified their information

### Enum: UserType

```go
type UserType string

const (
    UserTypeIndividual UserType = "individual"
    UserTypeEnterprise UserType = "enterprise"
    UserTypeDeveloper  UserType = "developer"
    UserTypeAdmin      UserType = "admin"
)
```

### AgentPreferences Schema

Structured JSON object stored in `agentPreferences` field:

```json
{
  "communicationStyle": "professional|casual|technical|friendly",
  "verbosity": "concise|moderate|detailed",
  "responseFormat": "markdown|plain|structured",
  "codeExamplesPreferred": true,
  "explainDecisions": false,
  "proactiveMode": true,
  "skillDefaults": {
    "commit": {
      "autoStage": true,
      "signoff": true
    }
  },
  "notificationPreferences": {
    "taskCompletion": true,
    "errors": true,
    "longRunningOps": true
  }
}
```

### NotesInformation Schema

Structured JSON object stored in `notesInformation` field:

```json
{
  "adminNotes": [
    {
      "timestamp": "2026-02-07T10:30:00Z",
      "authorID": "uuid",
      "note": "User requested enterprise features access"
    }
  ],
  "flags": {
    "betaTester": true,
    "earlyAccess": false,
    "supportTickets": ["TICKET-123", "TICKET-456"]
  },
  "restrictions": {
    "rateLimitOverride": null,
    "featureAccess": ["bedrock", "advanced-tools"]
  },
  "customMetadata": {
    "department": "Engineering",
    "project": "AI Integration"
  }
}
```

## Application Architecture

### Runtime Model

NuimanBot operates as a continuously running server process with multiple interfaces for interaction and management.

#### Core Server (nuimanbotd)

**Purpose**: Main application process running continuously as a daemon/service.

**Responsibilities**:
- LLM request processing
- Gateway management (CLI REPL, Slack, Telegram)
- User session management
- Configuration hot-reload via `/refresh` command
- Embedded web server for admin interface

**STDIO Commands** (when running in foreground):
- `/refresh` - Reload configuration from files (config.yaml, users.json, bots.json)
- `/exit` - Graceful shutdown of server

**Note**: All other commands removed from core server STDIO. User interactions happen through gateways (CLI, Slack, Telegram), not server STDIO.

#### CLI Tool (nuimanbot)

**Purpose**: Standalone CLI utility for configuration management and server interaction.

**Responsibilities**:
- Modify configuration files (config.yaml, users.json, bots.json)
- Trigger server configuration reload via signal or API call
- User management operations
- Bot management operations
- Administrative tasks

**Example Usage**:
```bash
# Modify configuration
nuimanbot config set llm.default_model.primary anthropic-main

# Add user
nuimanbot admin user create --username alice --email alice@example.com

# Reload server configuration
nuimanbot server reload

# View server status
nuimanbot server status
```

#### Administration Web Interface

**Purpose**: Browser-based configuration and management interface embedded in core server.

**URL**: `http://localhost:<admin-port>/admin/` (default: http://localhost:8080/admin/)

**Responsibilities**:
- Configure LLM providers and models
- Manage server paths and logging settings
- Create and configure users
- Manage bot configurations
- View system status and metrics

**Access Control**: Admin users only (User.Role == "admin")

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                      Core Server (nuimanbotd)                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   LLM Engine │  │   Gateways   │  │  Web Admin   │          │
│  │              │  │  - CLI REPL  │  │  Interface   │          │
│  │  Anthropic   │  │  - Slack     │  │              │          │
│  │  OpenAI      │  │  - Telegram  │  │  :8080/admin │          │
│  │  Bedrock     │  │              │  │              │          │
│  │  Ollama      │  │              │  │              │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│         │                  │                  │                 │
│         └──────────────────┼──────────────────┘                 │
│                            │                                    │
│  ┌─────────────────────────▼────────────────────────┐          │
│  │         Configuration Manager                     │          │
│  │  - Hot-reload on /refresh or API trigger         │          │
│  │  - Watch files: config.yaml, users.json, bots.json│         │
│  └───────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  CLI Tool    │    │  Web Browser │    │  File System │
│  (nuimanbot) │    │              │    │              │
│              │    │  Admin UI    │    │ config.yaml  │
│  - Config    │    │  localhost:  │    │ users.json   │
│  - Users     │    │  8080/admin  │    │ bots.json    │
│  - Bots      │    │              │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

## Administration Web Interface

### Overview

Embedded web server running within the core NuimanBot server process, providing a browser-based interface for system configuration and management.

### Technical Specifications

**Web Framework**:
- Go `net/http` with standard library routing or lightweight framework (e.g., Chi, Echo)
- Server-side rendering with HTML templates
- HTMX for dynamic updates without full page reloads
- Minimal JavaScript (progressive enhancement)

**Port Configuration**:
- Default: 8080
- Configurable via `server.admin_port` in config.yaml
- Environment variable: `NUIMANBOT_SERVER_ADMIN_PORT`

**Authentication**:
- Session-based authentication with secure cookies
- Admin role required (`User.Role == "admin"`)
- Login page at `/admin/login`
- Session timeout: 24 hours (configurable)
- CSRF protection on all forms

**Security**:
- HTTPS support with TLS certificates
- Content Security Policy headers
- Rate limiting on authentication endpoints
- Audit logging of all configuration changes

### Web Interface Pages

#### 1. Dashboard (`/admin/`)

**Purpose**: System overview and quick access

**Content**:
- Server status (uptime, version, memory usage)
- Active connections (Slack bots, Telegram bots, CLI sessions)
- Recent activity log (last 50 entries)
- Quick stats:
  - Total users
  - Active bots
  - LLM requests (last 24h)
  - Error rate

**Actions**:
- Reload configuration button
- View full logs link
- System health indicators

#### 2. LLM Configuration (`/admin/llm`)

**Purpose**: Configure LLM providers and models

**Sections**:

**A. Default Model Selection**
```
┌─────────────────────────────────────────────┐
│ Default Model Configuration                  │
├─────────────────────────────────────────────┤
│ Primary Model:   [anthropic-main      ▼]    │
│ Secondary Model: [anthropic-fast      ▼]    │
│ Tertiary Model:  [openai-gpt4        ▼]    │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘
```

**B. Provider Configuration**
```
┌─────────────────────────────────────────────┐
│ Anthropic Configuration                      │
├─────────────────────────────────────────────┤
│ API Key:    [••••••••••••••••••••] [Edit]  │
│ Base URL:   [https://api.anthropic.com]     │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ OpenAI Configuration                         │
├─────────────────────────────────────────────┤
│ API Key:       [••••••••••••••••••••] [Edit]│
│ Base URL:      [https://api.openai.com/v1]  │
│ Organization:  [org-123456789]               │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ AWS Bedrock Configuration                    │
├─────────────────────────────────────────────┤
│ AWS Region:    [us-east-1           ▼]     │
│ AWS Profile:   [default]                     │
│ Max Retries:   [3]                           │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│ Ollama Configuration                         │
├─────────────────────────────────────────────┤
│ Base URL:      [http://localhost:11434]     │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘
```

**C. Model Instances**
```
┌─────────────────────────────────────────────────────────────────┐
│ Model Instances                              [+ Add New Model]  │
├─────────────────────────────────────────────────────────────────┤
│ ID              Type       Model Name              Enabled      │
│ anthropic-main  anthropic  claude-3-5-sonnet...   ✓  [Edit]    │
│ anthropic-fast  anthropic  claude-3-haiku...      ✓  [Edit]    │
│ openai-gpt4     openai     gpt-4-turbo-preview    ✓  [Edit]    │
│ ollama-local    ollama     llama2                 ✗  [Edit]    │
└─────────────────────────────────────────────────────────────────┘
```

**Add/Edit Model Form**:
```
┌─────────────────────────────────────────────┐
│ Add Model Instance                           │
├─────────────────────────────────────────────┤
│ ID:          [anthropic-opus]                │
│ Type:        [anthropic            ▼]       │
│ Enabled:     [✓]                             │
│ Model Name:  [claude-3-opus-20240229]        │
│                                              │
│ Override Settings (optional):                │
│ API Key:     [                    ]          │
│ Base URL:    [                    ]          │
│                                              │
│              [Cancel]  [Save Model]          │
└─────────────────────────────────────────────┘
```

#### 3. Server Configuration (`/admin/server`)

**Purpose**: Configure server paths and logging

**Sections**:

**A. Server Paths**
```
┌─────────────────────────────────────────────┐
│ Server Paths                                 │
├─────────────────────────────────────────────┤
│ Config Directory: [./config/]                │
│ Data Directory:   [./data/]                  │
│ Logs Directory:   [./logs/]                  │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘
```

**B. Logging Configuration**
```
┌─────────────────────────────────────────────┐
│ Logging Settings                             │
├─────────────────────────────────────────────┤
│ Log Level:  [info              ▼]           │
│             (debug, info, warn, error)       │
│                                              │
│ Debug Mode: [✗] Enable verbose logging       │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘
```

**C. Gateway Configuration**
```
┌─────────────────────────────────────────────┐
│ Gateways                                     │
├─────────────────────────────────────────────┤
│ CLI Gateway:      [✓] Enabled                │
│ Slack Gateway:    [✓] Enabled                │
│ Telegram Gateway: [✗] Enabled                │
│                                              │
│                          [Save Changes]      │
└─────────────────────────────────────────────┘
```

#### 4. User Management (`/admin/users`)

**Purpose**: Create and configure users

**User List**:
```
┌─────────────────────────────────────────────────────────────────┐
│ Users                                        [+ Add New User]    │
├─────────────────────────────────────────────────────────────────┤
│ Username     Name            Email                Role  Actions │
│ alice_admin  Alice Anderson  alice@example.com   Admin [Edit]   │
│ bob_user     Bob Builder     bob@example.com     User  [Edit]   │
│ charlie_dev  Charlie Chen    charlie@example.com User  [Edit]   │
└─────────────────────────────────────────────────────────────────┘
```

**Add User Form**:
```
┌─────────────────────────────────────────────┐
│ Add New User                                 │
├─────────────────────────────────────────────┤
│ Username:         [                ]         │
│ Email:            [                ]         │
│ Role:             [user            ▼]       │
│                                              │
│ Profile Information:                         │
│ First Name:       [                ]         │
│ Last Name:        [                ]         │
│ Primary Language: [en              ▼]       │
│ Timezone:         [UTC             ▼]       │
│                                              │
│ Platform IDs (optional):                     │
│ Slack ID:         [                ]         │
│ Telegram ID:      [                ]         │
│                                              │
│              [Cancel]  [Create User]         │
└─────────────────────────────────────────────┘
```

**Edit User Form** (similar to Add, with additional fields):
```
┌─────────────────────────────────────────────┐
│ Edit User: alice_admin                       │
├─────────────────────────────────────────────┤
│ [Basic Info Tab] [Profile Tab] [Access Tab] │
│                                              │
│ Basic Info:                                  │
│ Username:         [alice_admin]              │
│ Email:            [alice@example.com]        │
│ Role:             [admin           ▼]       │
│ Enabled:          [✓]                        │
│                                              │
│ [View Full Profile] [Delete User]            │
│                                              │
│              [Cancel]  [Save Changes]        │
└─────────────────────────────────────────────┘
```

#### 5. Bot Management (`/admin/bots`)

**Purpose**: Configure Slack and Telegram bots

**Bot List**:
```
┌─────────────────────────────────────────────────────────────────┐
│ Slack Bots                                   [+ Add Slack Bot]   │
├─────────────────────────────────────────────────────────────────┤
│ Name                Type    Owner        Enabled  Actions        │
│ Company Assistant   Public  -            ✓        [Edit]         │
│ Alice Personal Bot  Private alice_admin  ✓        [Edit]         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ Telegram Bots                              [+ Add Telegram Bot]  │
├─────────────────────────────────────────────────────────────────┤
│ Name                  Type    Owner      Enabled  Actions        │
│ Bob Personal Assistant Private bob_user  ✓        [Edit]         │
│ Team Support Bot      Public  -          ✓        [Edit]         │
└─────────────────────────────────────────────────────────────────┘
```

**Add Slack Bot Form**:
```
┌─────────────────────────────────────────────┐
│ Add Slack Bot                                │
├─────────────────────────────────────────────┤
│ Bot Name:         [                ]         │
│ Bot Type:         [private         ▼]       │
│                   (private, public)          │
│                                              │
│ Owner (if private):[alice_admin    ▼]       │
│                                              │
│ Slack Configuration:                         │
│ Bot Token:        [xoxb-...        ]         │
│ App Token:        [xapp-...        ]         │
│ Signing Secret:   [                ]         │
│ Team ID:          [T12345678       ]         │
│ Bot User ID:      [B12345678       ]         │
│                                              │
│ Enabled:          [✓]                        │
│                                              │
│              [Cancel]  [Create Bot]          │
└─────────────────────────────────────────────┘
```

**Allowed Users (for public bots)**:
```
┌─────────────────────────────────────────────┐
│ Allowed Users for: Company Assistant        │
├─────────────────────────────────────────────┤
│ ☑ alice_admin (Alice Anderson)               │
│ ☑ bob_user (Bob Builder)                     │
│ ☑ charlie_dev (Charlie Chen)                 │
│ ☐ dave_dev (Dave Developer)                  │
│                                              │
│              [Cancel]  [Save Changes]        │
└─────────────────────────────────────────────┘
```

#### 6. Activity Log (`/admin/logs`)

**Purpose**: View system activity and audit log

**Log Viewer**:
```
┌─────────────────────────────────────────────────────────────────┐
│ Activity Log                                                     │
├─────────────────────────────────────────────────────────────────┤
│ Filters: [All       ▼] [Last 24h  ▼] [Search...]               │
│                                                                  │
│ Timestamp           User         Action                         │
│ 2026-02-07 15:30:00 alice_admin  Updated LLM provider config    │
│ 2026-02-07 14:20:00 alice_admin  Created user: dave_dev         │
│ 2026-02-07 13:15:00 alice_admin  Enabled bot: Team Support Bot  │
│ 2026-02-07 12:00:00 alice_admin  Updated server paths           │
│ 2026-02-07 11:30:00 alice_admin  Logged in                      │
│                                                                  │
│                                      [← Prev] [Next →]          │
└─────────────────────────────────────────────────────────────────┘
```

### API Endpoints (for CLI integration)

Web admin interface also exposes JSON API endpoints for CLI tool:

```
POST   /api/admin/config/reload        # Trigger config reload
GET    /api/admin/status                # Server status
POST   /api/admin/users                 # Create user
PUT    /api/admin/users/:id             # Update user
DELETE /api/admin/users/:id             # Delete user
POST   /api/admin/bots/slack            # Create Slack bot
PUT    /api/admin/bots/slack/:id        # Update Slack bot
DELETE /api/admin/bots/slack/:id        # Delete Slack bot
POST   /api/admin/bots/telegram         # Create Telegram bot
PUT    /api/admin/bots/telegram/:id     # Update Telegram bot
DELETE /api/admin/bots/telegram/:id     # Delete Telegram bot
PUT    /api/admin/config/llm            # Update LLM config
PUT    /api/admin/config/server         # Update server config
GET    /api/admin/logs                  # Get activity logs
```

### Implementation Notes

**File Watching**:
- Server watches config.yaml, users.json, bots.json for external changes
- Automatic reload when files modified (debounced to prevent rapid reloads)
- `/refresh` command forces immediate reload

**Configuration Write**:
- Web interface writes to config files atomically (temp file + rename)
- Triggers automatic reload or prompts user to reload
- Validates configuration before writing

**Session Management**:
- Redis or in-memory session store
- Secure HTTP-only cookies
- CSRF tokens on all mutations

**UI Framework**:
- Tailwind CSS for styling
- HTMX for dynamic interactions
- Alpine.js for minimal client-side interactivity
- Server-side rendering for SEO and simplicity

## Configuration File Restructuring

### Overview

The current configuration file structure requires significant improvements to support:
1. **Dynamic enable/disable** of gateways and providers without code changes
2. **Flexible provider configuration** with multiple instances of the same provider type
3. **Container-friendly path management** for config, data, and logs
4. **Consistent provider referencing** using IDs instead of type/name combinations
5. **Inheritance-based configuration** to reduce duplication in provider settings

### Current Configuration Issues

1. **Gateways**: No enable/disable flag - requires commenting out entire sections
2. **LLM Providers**: No enable/disable flag or model specification per provider
3. **Default Model**: Uses inconsistent format `<type>/<model-name>` instead of provider ID
4. **Hardcoded Paths**: Paths scattered throughout codebase, not container-friendly
5. **Provider Duplication**: Cannot easily have multiple providers of same type (e.g., two Anthropic configs)
6. **No Model Inheritance**: Each provider instance requires full configuration

### Proposed Configuration Structure

#### 1. Server Section - Add Paths

**Current**:
```yaml
server:
  log_level: info
  debug: false
```

**Proposed**:
```yaml
server:
  log_level: info
  debug: false
  paths:
    config: "./config/"     # Default: <working-directory>/config/
    data: "./data/"         # Default: <working-directory>/data/
    logs: "./logs/"         # Default: <working-directory>/logs/
```

**Benefits**:
- Container environments can mount separate volumes for config, data, logs
- Easy to redirect logs to `/var/log/nuimanbot/` in production
- Clear separation of operational data
- Eliminates hardcoded paths in codebase

**Implementation Requirements**:
- Add `PathsConfig` struct to `internal/config/config.go`
- Update all file operations to use configured paths
- Create path resolution utility: `config.ResolvePath(type, relativePath)`
- Paths used for: database files, vault, skills, history, temp files, log files

#### 2. Gateway Section - Add Enabled Flags

**Current**:
```yaml
gateways:
  cli:
    debug_mode: false
    history_file: ".nuimanbot_history"
  # telegram:  # Must comment/uncomment to enable/disable
  #   bot_token: "..."
```

**Proposed**:
```yaml
gateways:
  cli:
    enabled: true                          # NEW
    debug_mode: false
    history_file: ".nuimanbot_history"

  telegram:
    enabled: false                         # NEW - disabled without commenting
    bot_token: "your-bot-token"

  slack:
    enabled: false                         # NEW
    bot_token: "xoxb-your-token"
    app_token: "xapp-your-token"
```

**Benefits**:
- Enable/disable gateways without editing YAML structure
- Easier for users to toggle features
- Configuration remains valid when disabled
- Better for environment-based configuration (e.g., `NUIMANBOT_GATEWAYS_SLACK_ENABLED=true`)

**Implementation Requirements**:
- Add `Enabled bool` field to each gateway config struct
- Update gateway initialization to check `enabled` flag
- Skip disabled gateways during startup

**Note**: With database-driven bot management (from Bot Gateway Integration section), the `telegram` and `slack` gateway configuration will eventually be deprecated in favor of `slack_bots` and `telegram_bots` tables. The `enabled` flag here controls the entire gateway subsystem, while per-bot `enabled` flags control individual bot connections.

#### 3. LLM Providers Section - Enhanced Configuration

**Current**:
```yaml
llm:
  default_model:
    primary: anthropic/claude-sonnet    # Inconsistent format

  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "your-api-key-here"
      # No enabled flag, no model specified
```

**Proposed**:
```yaml
llm:
  default_model:
    primary: anthropic-main              # NEW: Use provider ID
    secondary: anthropic-fast            # NEW: Fallback model
    tertiary: openai-gpt4                # NEW: Second fallback

  # Provider-specific default configurations
  anthropic:
    api_key: "your-default-anthropic-key"
    base_url: "https://api.anthropic.com"  # Optional override

  openai:
    api_key: "your-default-openai-key"
    base_url: "https://api.openai.com/v1"
    organization: "your-org-id"            # Optional

  ollama:
    base_url: "http://localhost:11434"

  bedrock:
    aws_region: "us-east-1"
    aws_profile: ""
    max_retries: 3

  # Provider instances (can have multiple of same type)
  providers:
    # Auto-generated default providers (implicit)
    # - id: anthropic (uses anthropic section config)
    # - id: openai (uses openai section config)
    # - id: ollama (uses ollama section config)
    # - id: bedrock (uses bedrock section config)

    # Explicit provider instances
    - id: anthropic-main
      type: anthropic
      enabled: true                       # NEW
      model_name: claude-3-5-sonnet-20241022  # NEW
      # api_key inherited from anthropic section
      # base_url inherited from anthropic section

    - id: anthropic-fast
      type: anthropic
      enabled: true                       # NEW
      model_name: claude-3-haiku-20240307    # NEW
      # Inherits api_key and base_url

    - id: anthropic-custom
      type: anthropic
      enabled: false                      # NEW - disabled
      model_name: claude-3-opus-20240229
      api_key: "different-api-key"        # Override for this instance
      base_url: "https://custom.api.com"  # Override for this instance

    - id: openai-gpt4
      type: openai
      enabled: true
      model_name: gpt-4-turbo-preview
      # Inherits api_key, base_url, organization from openai section

    - id: ollama-local
      type: ollama
      enabled: true
      model_name: llama2
      # Inherits base_url from ollama section

    - id: bedrock-sonnet
      type: bedrock
      enabled: true
      model_name: us.anthropic.claude-3-5-sonnet-20241022-v2:0
      # Inherits aws_region, aws_profile from bedrock section
```

**Configuration Inheritance Rules**:
1. **Auto-generated Default Providers**: For each provider type with a configuration section (anthropic, openai, ollama, bedrock), automatically create a provider instance with `id` matching the type name
2. **Explicit Providers Inherit**: Provider instances inherit `api_key`, `base_url`, and other settings from their type's section
3. **Explicit Overrides**: Provider instances can override inherited values by specifying them explicitly
4. **Required Fields**: `id`, `type`, `enabled`, `model_name` are required for explicit providers
5. **Optional Overrides**: `api_key`, `base_url`, and provider-specific fields are optional (inherit if not specified)

**Benefits**:
- Multiple providers of same type (e.g., different Anthropic models for different use cases)
- Enable/disable providers without removing configuration
- Clear model specification per provider
- Reduced duplication via inheritance
- Consistent ID-based referencing throughout system

**Example Use Cases**:
```yaml
# Use case 1: Fast responses for simple queries
anthropic-fast:
  type: anthropic
  model_name: claude-3-haiku-20240307
  enabled: true

# Use case 2: Deep analysis
anthropic-opus:
  type: anthropic
  model_name: claude-3-opus-20240229
  enabled: true

# Use case 3: Separate API key for billing isolation
anthropic-project-x:
  type: anthropic
  model_name: claude-3-5-sonnet-20241022
  api_key: "project-x-key"
  enabled: true
```

#### 4. Default Model Configuration

**Current**:
```yaml
llm:
  default_model:
    primary: anthropic/claude-sonnet  # Type/name format
```

**Proposed**:
```yaml
llm:
  default_model:
    primary: anthropic-main      # Provider ID
    secondary: anthropic-fast    # Fallback 1
    tertiary: openai-gpt4        # Fallback 2
```

**Fallback Behavior**:
1. Attempt request with `primary` provider
2. If `primary` fails (rate limit, error, disabled), try `secondary`
3. If `secondary` fails, try `tertiary`
4. If all fail, return error to user

**Benefits**:
- Consistent ID-based referencing
- Automatic failover for reliability
- Clear priority ordering
- Easy to add more fallback levels

#### 5. Path Resolution Implementation

**Config Struct Changes** (`internal/config/config.go`):
```go
// PathsConfig holds configurable paths for different data types
type PathsConfig struct {
    Config string `yaml:"config"` // Default: "./config/"
    Data   string `yaml:"data"`   // Default: "./data/"
    Logs   string `yaml:"logs"`   // Default: "./logs/"
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
    Environment Environment `yaml:"environment"`
    LogLevel    string      `yaml:"log_level"`
    Debug       bool        `yaml:"debug"`
    Paths       PathsConfig `yaml:"paths"` // NEW
}
```

**Path Resolution Utility**:
```go
// PathType defines types of paths
type PathType string

const (
    PathTypeConfig PathType = "config"
    PathTypeData   PathType = "data"
    PathTypeLogs   PathType = "logs"
)

// ResolvePath resolves a relative path against configured base paths
func (c *Config) ResolvePath(pathType PathType, relativePath string) string {
    var basePath string
    switch pathType {
    case PathTypeConfig:
        basePath = c.Server.Paths.Config
    case PathTypeData:
        basePath = c.Server.Paths.Data
    case PathTypeLogs:
        basePath = c.Server.Paths.Logs
    default:
        basePath = "./"
    }

    return filepath.Join(basePath, relativePath)
}
```

**Usage Examples**:
```go
// Database file
dbPath := config.ResolvePath(config.PathTypeData, "nuimanbot.db")
// Result: "./data/nuimanbot.db" (or custom data path)

// Vault file
vaultPath := config.ResolvePath(config.PathTypeData, "vault.enc")
// Result: "./data/vault.enc"

// Log file
logPath := config.ResolvePath(config.PathTypeLogs, "app.log")
// Result: "./logs/app.log" (or "/var/log/nuimanbot/app.log" in production)

// Skills directory
skillsPath := config.ResolvePath(config.PathTypeData, "skills/shared")
// Result: "./data/skills/shared"

// Config file (when reloading)
configPath := config.ResolvePath(config.PathTypeConfig, "config.yaml")
// Result: "./config/config.yaml"
```

**Files Requiring Path Updates**:
1. `internal/config/loader.go` - Configuration loading
2. `internal/adapter/repository/sqlite/sqlite.go` - Database path
3. `internal/infrastructure/security/vault.go` - Vault file path
4. `internal/infrastructure/skill/loader.go` - Skills directory paths
5. `internal/adapter/gateway/cli/repl.go` - History file path
6. Logging initialization - Log file paths
7. Any file I/O operations using hardcoded paths

#### 6. Configuration Migration Guide

**For Existing Deployments**:

1. **Update config.yaml**:
   ```yaml
   # Before
   llm:
     default_model:
       primary: anthropic/claude-sonnet

   # After
   llm:
     default_model:
       primary: anthropic-main  # or anthropic if using auto-generated default
   ```

2. **Add paths section**:
   ```yaml
   server:
     paths:
       config: "./config/"
       data: "./data/"
       logs: "./logs/"
   ```

3. **Add enabled flags to gateways**:
   ```yaml
   gateways:
     cli:
       enabled: true
     telegram:
       enabled: false  # Previously commented out
     slack:
       enabled: false  # Previously commented out
   ```

4. **Update provider configurations**:
   ```yaml
   llm:
     # Add provider-specific sections
     anthropic:
       api_key: "your-key"

     # Update providers to include enabled and model_name
     providers:
       - id: anthropic-main
         type: anthropic
         enabled: true
         model_name: claude-3-5-sonnet-20241022
   ```

### Configuration Schema Validation

**Add JSON Schema validation** for configuration files to catch errors early:

```yaml
# config.schema.json (excerpt)
{
  "properties": {
    "server": {
      "properties": {
        "paths": {
          "type": "object",
          "properties": {
            "config": {"type": "string"},
            "data": {"type": "string"},
            "logs": {"type": "string"}
          }
        }
      }
    },
    "llm": {
      "properties": {
        "default_model": {
          "properties": {
            "primary": {"type": "string"},
            "secondary": {"type": "string"},
            "tertiary": {"type": "string"}
          },
          "required": ["primary"]
        },
        "providers": {
          "type": "array",
          "items": {
            "properties": {
              "id": {"type": "string"},
              "type": {"enum": ["anthropic", "openai", "ollama", "bedrock"]},
              "enabled": {"type": "boolean"},
              "model_name": {"type": "string"}
            },
            "required": ["id", "type", "enabled", "model_name"]
          }
        }
      }
    }
  }
}
```

### Environment Variable Overrides

**Enhanced environment variable support** for all new configuration fields:

```bash
# Server paths
export NUIMANBOT_SERVER_PATHS_CONFIG="/etc/nuimanbot/config/"
export NUIMANBOT_SERVER_PATHS_DATA="/var/lib/nuimanbot/"
export NUIMANBOT_SERVER_PATHS_LOGS="/var/log/nuimanbot/"

# Gateway enabled flags
export NUIMANBOT_GATEWAYS_CLI_ENABLED=true
export NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED=false
export NUIMANBOT_GATEWAYS_SLACK_ENABLED=true

# Default models
export NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY="anthropic-main"
export NUIMANBOT_LLM_DEFAULTMODEL_SECONDARY="anthropic-fast"
export NUIMANBOT_LLM_DEFAULTMODEL_TERTIARY="openai-gpt4"

# Provider instances (array notation)
export NUIMANBOT_LLM_PROVIDERS_0_ID="anthropic-main"
export NUIMANBOT_LLM_PROVIDERS_0_TYPE="anthropic"
export NUIMANBOT_LLM_PROVIDERS_0_ENABLED=true
export NUIMANBOT_LLM_PROVIDERS_0_MODELNAME="claude-3-5-sonnet-20241022"
```

### Docker/Container Configuration Example

**docker-compose.yml**:
```yaml
services:
  nuimanbot:
    image: nuimanbot:latest
    environment:
      NUIMANBOT_SERVER_PATHS_CONFIG: "/config"
      NUIMANBOT_SERVER_PATHS_DATA: "/data"
      NUIMANBOT_SERVER_PATHS_LOGS: "/logs"
      NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY: "anthropic-main"
      NUIMANBOT_GATEWAYS_SLACK_ENABLED: "true"
    volumes:
      - ./config:/config:ro          # Read-only config
      - nuimanbot-data:/data         # Persistent data
      - nuimanbot-logs:/logs         # Logs (can be volume or bind mount)

volumes:
  nuimanbot-data:
  nuimanbot-logs:
```

**Benefits**:
- Clear separation of concerns (config vs data vs logs)
- Easy to mount separate volumes
- Logs can be redirected to host or log aggregator
- Config can be read-only for security

## Bot Gateway Integration

### Bot Configuration Tables

NuimanBot supports multiple gateway integrations through configurable bots. Each bot can be either **public** (shared across multiple users) or **private** (dedicated to a single user).

### SlackBots Data Model

**Primary Fields**:

| Field | Type | Description | Required | Constraints |
|-------|------|-------------|----------|-------------|
| `botID` | UUID | Unique bot identifier (primary key) | Yes | Auto-generated |
| `botName` | string | Human-readable bot name | Yes | Max 100 chars, unique |
| `botType` | BotType | Enum: Public, Private | Yes | Default: Private |
| `ownerUserID` | UUID | User who owns this bot (for private bots) | Conditional | Required if botType=Private, NULL if Public |
| `slackBotToken` | SecureString | Slack Bot User OAuth Token (xoxb-...) | Yes | Encrypted at rest |
| `slackAppToken` | SecureString | Slack App-Level Token (xapp-...) for Socket Mode | No | Encrypted at rest |
| `slackSigningSecret` | SecureString | Slack signing secret for request verification | Yes | Encrypted at rest |
| `slackTeamID` | string | Slack workspace team ID | Yes | Max 20 chars |
| `slackBotUserID` | string | Slack bot user ID (starts with 'B') | Yes | Max 20 chars |
| `enabled` | boolean | Whether bot connects to service on gateway startup | Yes | Default: true |
| `allowedUserIDs` | JSON Array | List of userIDs allowed to interact (public bots only) | Conditional | Required if botType=Public |
| `metadata` | JSON | Additional bot configuration and settings | No | - |

**Notes**:
- **Socket Mode Only**: NuimanBot only supports Slack Socket Mode for security and simplicity. Webhooks are not supported.
- **No Timestamps**: Creation/update timestamps omitted from bots.json (use file system mtime if needed)
- **Runtime State**: Connection status tracked in memory or separate runtime state file, not in bots.json

### TelegramBots Data Model

**Primary Fields**:

| Field | Type | Description | Required | Constraints |
|-------|------|-------------|----------|-------------|
| `botID` | UUID | Unique bot identifier (primary key) | Yes | Auto-generated |
| `botName` | string | Human-readable bot name | Yes | Max 100 chars, unique |
| `botType` | BotType | Enum: Public, Private | Yes | Default: Private |
| `ownerUserID` | UUID | User who owns this bot (for private bots) | Conditional | Required if botType=Private, NULL if Public |
| `telegramBotToken` | SecureString | Telegram Bot API token | Yes | Encrypted at rest |
| `telegramBotUsername` | string | Telegram bot username (without @) | Yes | Max 32 chars, unique |
| `telegramBotID` | string | Telegram bot ID | Yes | Unique |
| `enabled` | boolean | Whether bot connects to service on gateway startup | Yes | Default: true |
| `allowedUserIDs` | JSON Array | List of userIDs allowed to interact (public bots only) | Conditional | Required if botType=Public |
| `allowedChatIDs` | JSON Array | List of Telegram chat IDs allowed to interact | No | For additional access control |
| `metadata` | JSON | Additional bot configuration and settings | No | - |

**Notes**:
- **Long Polling Only**: NuimanBot only supports Telegram long polling for security and simplicity. Webhooks are not supported.
- **No Timestamps**: Creation/update timestamps omitted from bots.json (use file system mtime if needed)
- **Runtime State**: Connection status tracked in memory or separate runtime state file, not in bots.json

### Enum: BotType

```go
type BotType string

const (
    BotTypePrivate BotType = "private"
    BotTypePublic  BotType = "public"
)
```

### Bot Access Control Rules

**Private Bots**:
- Tied to a single user (`ownerUserID`)
- Only the owner can interact with the bot
- Owner can manage bot settings via API/CLI
- Admins can manage all private bots

**Public Bots**:
- Not tied to a specific user (`ownerUserID` is NULL)
- Multiple users can interact with the bot
- `allowedUserIDs` defines who can use the bot
- Only admins can create and manage public bots
- Regular users cannot modify `allowedUserIDs` list

### Bot Metadata Schema

Common metadata structure for both Slack and Telegram bots:

```json
{
  "description": "Primary customer support bot",
  "tags": ["support", "production"],
  "rateLimits": {
    "maxRequestsPerMinute": 60,
    "maxConcurrentConversations": 10
  },
  "features": {
    "fileUploads": true,
    "voiceMessages": false,
    "commandsEnabled": true
  },
  "notifications": {
    "connectionErrors": ["admin@example.com"],
    "rateLimitWarnings": ["ops@example.com"]
  },
  "customSettings": {
    "responseDelay": 500,
    "typingIndicator": true
  }
}
```

## Bot-User Relationships

### Relationship Models

**Private Bot → Single User (1:1)**:
```
┌──────────────┐          ┌──────────────┐
│  Private Bot │ owns_by  │     User     │
│              │─────────>│              │
│ botType:     │          │ UserProfile  │
│  "private"   │          │              │
└──────────────┘          └──────────────┘
```

**Public Bot → Multiple Users (1:N)**:
```
                           ┌──────────────┐
                    ┌─────>│   User 1     │
                    │      └──────────────┘
┌──────────────┐   │      ┌──────────────┐
│  Public Bot  │───┼─────>│   User 2     │
│              │   │      └──────────────┘
│ botType:     │   │      ┌──────────────┐
│  "public"    │   └─────>│   User 3     │
│              │          └──────────────┘
│ allowedUser  │
│  IDs: [...]  │
└──────────────┘
```

### Message Routing Logic

**Incoming Message from Slack**:
1. Gateway receives message from Slack workspace
2. Extract Slack user ID from message
3. Query `user_profiles` table for matching `slackID`
4. If match found, load user's profile and preferences
5. Route message to agent with user context
6. Agent responds using user's preferred settings
7. Send response back through Slack bot

**Incoming Message from Telegram**:
1. Gateway receives message from Telegram
2. Extract Telegram user ID from message
3. Query `user_profiles` table for matching `telegramID`
4. If match found, load user's profile and preferences
5. Route message to agent with user context
6. Agent responds using user's preferred settings
7. Send response back through Telegram bot

### Bot Selection Logic

When multiple bots exist for the same platform:

**For Private Bots**:
1. User sends message to bot on Slack/Telegram
2. Gateway identifies bot by bot's platform ID (Slack bot user ID or Telegram bot ID)
3. Gateway looks up bot in `slack_bots` or `telegram_bots` table
4. Verify bot is enabled and botType is "private"
5. Extract `ownerUserID` from bot configuration
6. Load owner's user profile
7. Process message in context of owner user

**For Public Bots**:
1. User sends message to bot on Slack/Telegram
2. Gateway identifies bot by bot's platform ID
3. Gateway looks up bot in database and verifies `enabled=true`
4. Extract sender's platform user ID (Slack user ID or Telegram user ID)
5. Query `user_profiles` to find NuimanBot user by platform ID
6. Verify user's ID is in bot's `allowedUserIDs` list
7. If authorized, load user profile and process message
8. If not authorized, send "Access denied" message

### Gateway Initialization Sequence

**On Gateway Startup**:
```
1. Gateway starts
2. Query database: SELECT * FROM slack_bots WHERE enabled = true
3. Query database: SELECT * FROM telegram_bots WHERE enabled = true
4. For each enabled bot:
   a. Load bot credentials (decrypt tokens/secrets)
   b. Initialize platform connection (Slack Socket Mode or Telegram long polling)
   c. Register message handlers
   d. Update lastConnectedAt timestamp
   e. Start listening for messages
5. Monitor for bot enable/disable events during runtime
```

**Runtime Bot Management**:
```
1. Admin disables bot via API: PUT /api/v1/admin/bots/slack/{botID} {"enabled": false}
2. System updates database: UPDATE slack_bots SET enabled = false WHERE bot_id = ?
3. Gateway detects change (polling or event notification)
4. Gateway disconnects bot from Slack
5. Updates lastDisconnectedAt timestamp
6. Bot stops processing messages (existing conversations gracefully closed)

7. Admin re-enables bot: PUT /api/v1/admin/bots/slack/{botID} {"enabled": true}
8. System updates database: UPDATE slack_bots SET enabled = true WHERE bot_id = ?
9. Gateway detects change
10. Gateway reconnects bot to Slack
11. Updates lastConnectedAt timestamp
12. Bot resumes processing messages
```

### Example Configurations

**Example 1: Small Team with Public Slack Bot**
```
Organization: Acme Corp (10 users)
Setup:
- 1 public Slack bot: "Acme Assistant"
- Bot enabled for all 10 users
- All users share the same bot for company-wide access
- Each user has individual profile with preferences
- Bot adapts responses based on each user's profile

Database:
slack_bots:
  - botID: "bot-001"
  - botName: "Acme Assistant"
  - botType: "public"
  - ownerUserID: null
  - allowedUserIDs: ["user-1", "user-2", ..., "user-10"]
  - enabled: true

user_profiles:
  - userID: "user-1", slackID: "U001ABC", firstName: "Alice", ...
  - userID: "user-2", slackID: "U002DEF", firstName: "Bob", ...
```

**Example 2: Enterprise with Multiple Private Bots**
```
Organization: BigCorp (1000 users)
Setup:
- 500 private Telegram bots (one per user who requested)
- Each bot is personal to one user
- Users manage their own bot credentials
- Admin can view all bots but users own their data

Database:
telegram_bots:
  - botID: "tg-001", botName: "Alice's Assistant", botType: "private", ownerUserID: "user-1", enabled: true
  - botID: "tg-002", botName: "Bob's Helper", botType: "private", ownerUserID: "user-2", enabled: true
  - ...

user_profiles:
  - userID: "user-1", telegramID: "123456", firstName: "Alice", ...
  - userID: "user-2", telegramID: "789012", firstName: "Bob", ...
```

**Example 3: Mixed Configuration**
```
Organization: TechStartup (50 users)
Setup:
- 1 public Slack bot for company-wide access (30 users authorized)
- 2 private Slack bots for executives (CEO, CTO)
- 5 private Telegram bots for remote workers
- Bots can be enabled/disabled independently

Database:
slack_bots:
  - botID: "slack-pub-001", botName: "Company Assistant", botType: "public", allowedUserIDs: [30 users], enabled: true
  - botID: "slack-prv-001", botName: "CEO Assistant", botType: "private", ownerUserID: "user-ceo", enabled: true
  - botID: "slack-prv-002", botName: "CTO Assistant", botType: "private", ownerUserID: "user-cto", enabled: true

telegram_bots:
  - botID: "tg-001", botName: "Remote Worker 1", botType: "private", ownerUserID: "user-15", enabled: true
  - ... (4 more private bots)
```

## File-Based Storage Design

### Storage Philosophy

User profile data and related information will be stored in **file-based JSON format** rather than database tables. This approach provides:

1. **Simplicity**: No database migrations, easy to inspect and modify
2. **Portability**: Easy backup/restore (copy directory structure)
3. **Human-Readable**: JSON files can be manually edited if needed
4. **Container-Friendly**: Mount user data directory as volume
5. **Version Control**: User data can be versioned with git if desired
6. **Isolation**: Each user's data in separate directory for clear boundaries

### Storage Structure

```
<data-directory>/
├── users.json              # Central user registry with profile summaries
├── bots.json               # Bot configurations (Slack, Telegram)
└── users/                  # User-specific data directories
    ├── <user-id-1>/
    │   ├── profile.json    # Extended profile information
    │   ├── preferences.json # LLM and agent preferences
    │   ├── todos.json      # User's task list
    │   ├── repeated-actions.json  # Cron jobs and scheduled tasks
    │   ├── history.json    # Command/action history
    │   ├── projects/       # User's project files
    │   │   ├── project-a/
    │   │   └── project-b/
    │   └── notes/          # User's personal notes
    │       ├── note-1.md
    │       └── note-2.md
    ├── <user-id-2>/
    │   └── ...
    └── <user-id-3>/
        └── ...
```

### users.json Structure

**Purpose**: Central registry of all users with summary information for quick lookups.

**Schema**:
```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-07T15:30:00Z",
  "users": [
    {
      "userID": "550e8400-e29b-41d4-a716-446655440000",
      "username": "alice_admin",
      "moniker": "alice",
      "firstName": "Alice",
      "lastName": "Anderson",
      "nickName": "Ali",
      "primaryEmail": "alice@example.com",
      "backupEmail": "alice.anderson@personal.com",
      "mobilePhone": "+14155551234",
      "primaryLanguage": "en",
      "secondaryLanguage": "es",
      "timezone": "America/Los_Angeles",
      "primaryLocation": "San Francisco, CA",
      "jobRole": "Senior Engineer",
      "userType": "enterprise",
      "role": "admin",
      "platformIDs": {
        "cli": "alice_admin",
        "slack": "U01ABC123",
        "telegram": "123456789"
      },
      "enabled": true,
      "dataDirectory": "users/550e8400-e29b-41d4-a716-446655440000"
    },
    {
      "userID": "660e8400-e29b-41d4-a716-446655440001",
      "username": "bob_user",
      "moniker": "bob",
      "firstName": "Bob",
      "lastName": "Builder",
      "nickName": "Bobby",
      "primaryEmail": "bob@example.com",
      "backupEmail": null,
      "mobilePhone": null,
      "primaryLanguage": "en",
      "secondaryLanguage": null,
      "timezone": "America/New_York",
      "primaryLocation": "New York, NY",
      "jobRole": "Developer",
      "userType": "individual",
      "role": "user",
      "platformIDs": {
        "cli": "bob_user",
        "telegram": "987654321"
      },
      "enabled": true,
      "dataDirectory": "users/660e8400-e29b-41d4-a716-446655440001"
    },
    {
      "userID": "770e8400-e29b-41d4-a716-446655440002",
      "username": "charlie_dev",
      "moniker": "charlie",
      "firstName": "Charlie",
      "lastName": "Chen",
      "nickName": "CC",
      "primaryEmail": "charlie@example.com",
      "backupEmail": "charlie.chen@gmail.com",
      "mobilePhone": "+12125559876",
      "primaryLanguage": "en",
      "secondaryLanguage": "zh",
      "timezone": "UTC",
      "primaryLocation": "Remote",
      "jobRole": "Technical Lead",
      "userType": "developer",
      "role": "user",
      "platformIDs": {
        "cli": "charlie_dev",
        "slack": "U02DEF456"
      },
      "enabled": true,
      "dataDirectory": "users/770e8400-e29b-41d4-a716-446655440002"
    }
  ],
  "indexes": {
    "byUsername": {
      "alice_admin": "550e8400-e29b-41d4-a716-446655440000",
      "bob_user": "660e8400-e29b-41d4-a716-446655440001",
      "charlie_dev": "770e8400-e29b-41d4-a716-446655440002"
    },
    "byEmail": {
      "alice@example.com": "550e8400-e29b-41d4-a716-446655440000",
      "bob@example.com": "660e8400-e29b-41d4-a716-446655440001",
      "charlie@example.com": "770e8400-e29b-41d4-a716-446655440002"
    },
    "byPlatform": {
      "slack": {
        "U01ABC123": "550e8400-e29b-41d4-a716-446655440000",
        "U02DEF456": "770e8400-e29b-41d4-a716-446655440002"
      },
      "telegram": {
        "123456789": "550e8400-e29b-41d4-a716-446655440000",
        "987654321": "660e8400-e29b-41d4-a716-446655440001"
      },
      "cli": {
        "alice_admin": "550e8400-e29b-41d4-a716-446655440000",
        "bob_user": "660e8400-e29b-41d4-a716-446655440001",
        "charlie_dev": "770e8400-e29b-41d4-a716-446655440002"
      }
    }
  }
}
```

**Field Descriptions**:
- `version` - Schema version for migration compatibility
- `lastUpdated` - Timestamp of last modification to users.json
- `users[]` - Array of user profile summaries
- `indexes` - Pre-built lookup indexes for fast searches
  - `byUsername` - Username → UserID mapping
  - `byEmail` - Email → UserID mapping
  - `byPlatform` - Platform → (PlatformID → UserID) nested mapping

### User Directory Structure

Each user has a dedicated subdirectory: `<data-directory>/users/<user-id>/`

#### profile.json
Extended profile information not in users.json summary.

```json
{
  "profileInfo": "Alice is a senior engineer with 10+ years of experience in distributed systems and cloud architecture.",
  "metadata": {
    "department": "Engineering",
    "team": "Platform",
    "manager": "david@example.com",
    "startDate": "2020-03-15",
    "employeeID": "EMP-1234"
  },
  "customFields": {
    "favoriteEditor": "vim",
    "preferredOS": "macOS",
    "githubHandle": "alice-dev"
  }
}
```

#### preferences.json
LLM and agent preferences.

```json
{
  "defaultModel": {
    "primary": "anthropic-main",
    "secondary": "anthropic-fast",
    "tertiary": "openai-gpt4"
  },
  "agentPreferences": {
    "communicationStyle": "professional",
    "verbosity": "concise",
    "responseFormat": "markdown",
    "codeExamplesPreferred": true,
    "explainDecisions": false,
    "proactiveMode": true,
    "skillDefaults": {
      "commit": {
        "autoStage": true,
        "signoff": true
      }
    },
    "notificationPreferences": {
      "taskCompletion": true,
      "errors": true,
      "longRunningOps": true
    }
  },
  "conversationSettings": {
    "maxContextTokens": 100000,
    "historyRetention": "30d",
    "autoSummarize": true
  }
}
```

#### todos.json
User's task list.

```json
{
  "tasks": [
    {
      "id": "task-001",
      "title": "Review PR #456",
      "description": "Code review for authentication refactor",
      "status": "in_progress",
      "priority": "high",
      "tags": ["code-review", "security"],
      "dueDate": "2026-02-10T17:00:00Z",
      "createdAt": "2026-02-07T09:00:00Z",
      "updatedAt": "2026-02-07T14:00:00Z"
    },
    {
      "id": "task-002",
      "title": "Update deployment docs",
      "description": "Document new Kubernetes deployment process",
      "status": "pending",
      "priority": "medium",
      "tags": ["documentation"],
      "dueDate": "2026-02-15T12:00:00Z",
      "createdAt": "2026-02-06T10:30:00Z",
      "updatedAt": "2026-02-06T10:30:00Z"
    }
  ]
}
```

#### repeated-actions.json
Cron jobs and scheduled tasks.

```json
{
  "scheduledActions": [
    {
      "id": "cron-001",
      "name": "Daily standup reminder",
      "description": "Send Slack message with standup template",
      "schedule": "0 9 * * 1-5",
      "timezone": "America/Los_Angeles",
      "enabled": true,
      "action": {
        "type": "slack_message",
        "channel": "#team-platform",
        "template": "Good morning! Time for standup. Please share: 1) What did you do yesterday? 2) What will you do today? 3) Any blockers?"
      },
      "createdAt": "2026-01-15T10:00:00Z",
      "lastRun": "2026-02-07T09:00:00Z",
      "nextRun": "2026-02-10T09:00:00Z"
    },
    {
      "id": "cron-002",
      "name": "Weekly report generation",
      "description": "Generate and email weekly activity report",
      "schedule": "0 17 * * 5",
      "timezone": "America/Los_Angeles",
      "enabled": true,
      "action": {
        "type": "generate_report",
        "reportType": "weekly_activity",
        "emailTo": "alice@example.com"
      },
      "createdAt": "2026-01-20T11:00:00Z",
      "lastRun": "2026-02-07T17:00:00Z",
      "nextRun": "2026-02-14T17:00:00Z"
    }
  ]
}
```

#### history.json
Command and action history (last N entries).

```json
{
  "maxEntries": 1000,
  "entries": [
    {
      "id": "hist-5432",
      "timestamp": "2026-02-07T15:30:00Z",
      "type": "command",
      "command": "nuimanbot chat 'explain kubernetes pods'",
      "platform": "cli",
      "status": "success",
      "duration": 2.3
    },
    {
      "id": "hist-5431",
      "timestamp": "2026-02-07T14:15:00Z",
      "type": "skill",
      "skill": "code-review",
      "args": "pr-456",
      "platform": "slack",
      "status": "success",
      "duration": 5.7
    },
    {
      "id": "hist-5430",
      "timestamp": "2026-02-07T13:00:00Z",
      "type": "api_call",
      "endpoint": "/api/v1/profile",
      "method": "PUT",
      "status": "success",
      "duration": 0.15
    }
  ]
}
```

### bots.json Structure

**Purpose**: Central registry of all bot configurations for Slack and Telegram gateways.

**Location**: `<data-directory>/bots.json`

**Schema**:
```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-07T16:00:00Z",
  "slackBots": [
    {
      "botID": "slack-bot-001",
      "botName": "Company Assistant",
      "botType": "public",
      "ownerUserID": null,
      "slackBotToken": "xoxb-***",
      "slackAppToken": "xapp-***",
      "slackSigningSecret": "***",
      "slackTeamID": "T12345678",
      "slackBotUserID": "B12345678",
      "enabled": true,
      "allowedUserIDs": [
        "550e8400-e29b-41d4-a716-446655440000",
        "660e8400-e29b-41d4-a716-446655440001",
        "770e8400-e29b-41d4-a716-446655440002"
      ],
      "metadata": {
        "description": "Primary company-wide assistant bot",
        "tags": ["support", "production"],
        "rateLimits": {
          "maxRequestsPerMinute": 60
        }
      }
    },
    {
      "botID": "slack-bot-002",
      "botName": "Alice Personal Bot",
      "botType": "private",
      "ownerUserID": "550e8400-e29b-41d4-a716-446655440000",
      "slackBotToken": "xoxb-***",
      "slackAppToken": "xapp-***",
      "slackSigningSecret": "***",
      "slackTeamID": "T12345678",
      "slackBotUserID": "B87654321",
      "enabled": true,
      "allowedUserIDs": null,
      "metadata": {
        "description": "Alice's personal assistant",
        "tags": ["private", "admin"]
      }
    }
  ],
  "telegramBots": [
    {
      "botID": "telegram-bot-001",
      "botName": "Bob Personal Assistant",
      "botType": "private",
      "ownerUserID": "660e8400-e29b-41d4-a716-446655440001",
      "telegramBotToken": "123456:ABC-***",
      "telegramBotUsername": "bob_assistant_bot",
      "telegramBotID": "123456789",
      "enabled": true,
      "allowedUserIDs": null,
      "allowedChatIDs": [],
      "metadata": {
        "description": "Bob's personal Telegram bot",
        "tags": ["private", "telegram"]
      }
    },
    {
      "botID": "telegram-bot-002",
      "botName": "Team Support Bot",
      "botType": "public",
      "ownerUserID": null,
      "telegramBotToken": "654321:DEF-***",
      "telegramBotUsername": "team_support_bot",
      "telegramBotID": "987654321",
      "enabled": true,
      "allowedUserIDs": [
        "550e8400-e29b-41d4-a716-446655440000",
        "660e8400-e29b-41d4-a716-446655440001"
      ],
      "allowedChatIDs": ["chat-123", "chat-456"],
      "metadata": {
        "description": "Team-wide support bot for Telegram",
        "tags": ["support", "production", "telegram"]
      }
    }
  ],
  "indexes": {
    "slackByName": {
      "Company Assistant": "slack-bot-001",
      "Alice Personal Bot": "slack-bot-002"
    },
    "slackByOwner": {
      "550e8400-e29b-41d4-a716-446655440000": ["slack-bot-002"]
    },
    "slackByBotUserID": {
      "B12345678": "slack-bot-001",
      "B87654321": "slack-bot-002"
    },
    "telegramByName": {
      "Bob Personal Assistant": "telegram-bot-001",
      "Team Support Bot": "telegram-bot-002"
    },
    "telegramByOwner": {
      "660e8400-e29b-41d4-a716-446655440001": ["telegram-bot-001"]
    },
    "telegramByBotID": {
      "123456789": "telegram-bot-001",
      "987654321": "telegram-bot-002"
    }
  }
}
```

**Security Notes**:
- Bot tokens and secrets should be encrypted before writing to bots.json
- Use application-level encryption with key from environment variable
- Never commit bots.json to version control (add to .gitignore)
- Recommend storing bots.json in secure vault in production

### Data Migration Strategy

**Current State**:
- Users stored in SQLite `users` table
- Minimal profile information

**Target State**:
- users.json with comprehensive profile summaries
- User-specific directories with extended data
- SQLite database kept for conversations, messages, notes (transactional data)

**Migration Approach**:

**Phase 1: Initialize File Structure**
- Create `users.json` with empty structure
- Create `bots.json` with empty structure
- Create `users/` directory for user subdirectories
- Implement file I/O utilities with locking

**Phase 2: Migrate Existing Users**
- Read all users from SQLite `users` table
- For each user:
  - Generate UUID if using simple ID
  - Create entry in users.json with default values
  - Create user directory: `users/<user-id>/`
  - Create default files: profile.json, preferences.json, todos.json, etc.
  - Extract platform IDs from database → platformIDs in users.json
- Build indexes in users.json (byUsername, byEmail, byPlatform)

**Phase 3: Dual-Write Period** (Optional Transition)
- Continue writing to both SQLite and users.json
- Read from users.json, fallback to SQLite if not found
- Allows rollback if issues detected
- Duration: 1-2 weeks in production

**Phase 4: Cutover**
- Switch all user operations to use users.json
- Keep SQLite `users` table for historical reference
- Archive old data: `users.archive.json`

**Phase 5: Cleanup**
- Remove dual-write code
- Optionally drop `users` table from SQLite (keep conversations, messages, notes)
- Update documentation

## Admin Features

### Profile Management Commands

**View Profile**:
```bash
nuimanbot admin profile view <user-id>
nuimanbot admin profile view --email user@example.com
```

**Create/Update Profile**:
```bash
nuimanbot admin profile set <user-id> --first-name "John" --last-name "Doe"
nuimanbot admin profile set <user-id> --slack-id "U12345678"
nuimanbot admin profile set <user-id> --user-type enterprise
```

**Bulk Operations**:
```bash
nuimanbot admin profile import --file profiles.json
nuimanbot admin profile export --format json --output profiles.json
nuimanbot admin profile export --format csv --output profiles.csv
```

**Search and Filter**:
```bash
nuimanbot admin profile list --user-type enterprise
nuimanbot admin profile list --slack-id --format table
nuimanbot admin profile search --query "Engineering"
```

### Agent Preferences Management

**Set Agent Preferences**:
```bash
nuimanbot admin profile preferences set <user-id> --style professional --verbosity concise
nuimanbot admin profile preferences set <user-id> --proactive-mode true
```

**View Agent Preferences**:
```bash
nuimanbot admin profile preferences view <user-id>
```

**Reset to Defaults**:
```bash
nuimanbot admin profile preferences reset <user-id>
```

### Admin Notes Management

**Add Admin Note**:
```bash
nuimanbot admin profile note add <user-id> "User requested access to beta features"
```

**Set Flags**:
```bash
nuimanbot admin profile flag set <user-id> --beta-tester true
nuimanbot admin profile flag set <user-id> --feature-access bedrock,advanced-tools
```

**View Notes and Flags**:
```bash
nuimanbot admin profile notes view <user-id>
```

## Use Cases

### UC-1: User Self-Service Profile Management

**Actor**: End User

**Preconditions**: User is authenticated

**Flow**:
1. User runs `nuimanbot profile view` to see their current profile
2. User runs `nuimanbot profile update --first-name "Jane" --timezone "America/Los_Angeles"`
3. System validates input and updates profile
4. System confirms changes to user

**Postconditions**: User profile is updated with new information

### UC-2: Admin Bulk User Import

**Actor**: System Administrator

**Preconditions**: Admin has CSV/JSON file with user data

**Flow**:
1. Admin prepares import file with columns matching UserProfile fields
2. Admin runs `nuimanbot admin profile import --file users.csv --dry-run`
3. System validates data and shows preview
4. Admin confirms and runs actual import
5. System creates/updates profiles and reports results

**Postconditions**: Multiple user profiles created/updated

### UC-3: Agent Personalization

**Actor**: Agent (NuimanBot)

**Preconditions**: User has completed profile with agent preferences

**Flow**:
1. User initiates conversation
2. Agent loads user profile including agentPreferences
3. Agent adapts communication style based on preferences:
   - Adjusts verbosity level
   - Uses preferred response format
   - Applies proactive mode settings
4. Agent interacts according to personalized settings

**Postconditions**: User receives personalized agent experience

### UC-4: Platform Integration Linking

**Actor**: End User or Admin

**Preconditions**: User exists in system, Slack/Telegram integration active

**Flow**:
1. User authenticates via Slack OAuth
2. System receives Slack user ID
3. System updates user profile with `slackID`
4. Future messages from Slack automatically route to correct user profile
5. Agent can reference Slack-specific context in conversations

**Postconditions**: User profile linked to external platform

### UC-5: Admin Creates Public Slack Bot

**Actor**: System Administrator

**Preconditions**: Admin has Slack bot credentials and workspace access

**Flow**:
1. Admin creates Slack app in Slack workspace and obtains bot tokens
2. Admin runs `nuimanbot admin bot slack create` with bot credentials
3. System validates tokens and creates bot configuration in database
4. Admin adds authorized users to bot's `allowedUserIDs` list
5. Admin enables bot (`enabled=true`)
6. Gateway starts bot connection on next cycle
7. Bot connects to Slack workspace and begins listening for messages
8. Users in `allowedUserIDs` can interact with bot; others receive access denied

**Postconditions**: Public Slack bot is operational and serving multiple authorized users

### UC-6: User Creates Private Telegram Bot

**Actor**: End User

**Preconditions**: User has created Telegram bot via BotFather

**Flow**:
1. User creates bot through Telegram's @BotFather
2. User receives bot token from BotFather
3. User runs `nuimanbot bot create telegram` (or calls API) with bot credentials
4. System creates private bot configuration with `ownerUserID` set to user
5. System enables bot and starts connection
6. User can now interact with their personal bot on Telegram
7. Only the owner can send messages to this bot

**Postconditions**: User has private Telegram bot for personal use

### UC-7: Admin Disables Misbehaving Bot

**Actor**: System Administrator

**Preconditions**: Bot is currently enabled and connected

**Flow**:
1. Admin receives alert about bot misbehavior (rate limiting, errors, abuse)
2. Admin runs `nuimanbot admin bot slack disable <bot-id>` (or calls API)
3. System sets `enabled=false` in database
4. Gateway detects disabled flag and disconnects bot from Slack/Telegram
5. Users attempting to interact with bot receive "bot unavailable" message
6. Admin investigates and fixes configuration issues
7. Admin re-enables bot when ready

**Postconditions**: Bot is disabled and no longer processing messages

### UC-8: Admin Updates Public Bot User Access

**Actor**: System Administrator

**Preconditions**: Public bot exists with existing `allowedUserIDs` list

**Flow**:
1. Admin receives request to add new user to bot access list
2. Admin runs `nuimanbot admin bot slack users add <bot-id> <user-id>`
3. System updates bot's `allowedUserIDs` in database
4. New user can immediately interact with bot (no restart required)
5. Admin can also remove users or replace entire list

**Postconditions**: Updated user list has access to public bot

## REST API Requirements

**Core Requirement**: All data elements defined in the UserProfiles object MUST be manageable via REST API. Admin users with appropriate permissions can perform all CRUD operations (Create, Read, Update, Delete) on user profiles.

### REST API Design Principles

1. **Full CRUD Access**: Admin users have complete CRUD access to all profile fields
2. **Partial Updates**: PUT operations support partial updates - only fields provided in the request body are modified
3. **Idempotency**: PUT and DELETE operations are idempotent
4. **RESTful Conventions**: Use standard HTTP methods (GET, POST, PUT, DELETE)
5. **Role-Based Access**: Regular users can only access their own profile; admins can access all profiles
6. **Validation**: All requests validated against field constraints before persistence
7. **Audit Logging**: All admin modifications logged with timestamp, admin ID, and changes made

### REST API Endpoints

#### User Profile CRUD Operations (Admin)

**Create User Profile**
```
POST /api/v1/admin/profiles
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body (all fields optional except userID, primaryEmail, primaryLanguage, timezone):
{
  "userID": "550e8400-e29b-41d4-a716-446655440000",
  "moniker": "jdoe",
  "firstName": "John",
  "lastName": "Doe",
  "nickName": "Johnny",
  "primaryLanguage": "en",
  "secondaryLanguage": "es",
  "primaryLocation": "San Francisco, CA",
  "primaryEmail": "john.doe@example.com",
  "backupEmail": "jdoe@personal.com",
  "mobilePhone": "+14155551234",
  "timezone": "America/Los_Angeles",
  "jobRole": "Senior Engineer",
  "profileInfo": "10+ years in software development",
  "userType": "enterprise",
  "slackID": "U12345678",
  "telegramID": "123456789",
  "agentPreferences": {
    "communicationStyle": "professional",
    "verbosity": "concise"
  },
  "notesInformation": {
    "adminNotes": [],
    "flags": {"betaTester": true}
  }
}

Response (201 Created):
{
  "success": true,
  "data": {
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "moniker": "jdoe",
    // ... all fields ...
    "createdAt": "2026-02-07T10:30:00Z",
    "updatedAt": "2026-02-07T10:30:00Z"
  }
}
```

**Read User Profile**
```
GET /api/v1/admin/profiles/{userID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": {
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "moniker": "jdoe",
    "firstName": "John",
    // ... all fields ...
    "createdAt": "2026-02-07T10:30:00Z",
    "updatedAt": "2026-02-07T10:35:00Z",
    "lastVerified": "2026-02-07T10:35:00Z"
  }
}
```

**Update User Profile (Partial Update)**
```
PUT /api/v1/admin/profiles/{userID}
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body (only include fields to update):
{
  "firstName": "Jane",
  "jobRole": "Principal Engineer",
  "timezone": "America/New_York"
}

Response (200 OK):
{
  "success": true,
  "data": {
    "userID": "550e8400-e29b-41d4-a716-446655440000",
    "moniker": "jdoe",
    "firstName": "Jane",          // UPDATED
    "lastName": "Doe",             // unchanged
    "nickName": "Johnny",          // unchanged
    // ... other unchanged fields ...
    "jobRole": "Principal Engineer", // UPDATED
    "timezone": "America/New_York",  // UPDATED
    "updatedAt": "2026-02-07T11:00:00Z" // reflects update time
  },
  "updated_fields": ["firstName", "jobRole", "timezone"]
}
```

**Delete User Profile**
```
DELETE /api/v1/admin/profiles/{userID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "message": "User profile deleted successfully",
  "deletedUserID": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### List and Search Operations

**List All Profiles (Paginated)**
```
GET /api/v1/admin/profiles?page=1&limit=50&sort=createdAt&order=desc
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": [
    {
      "userID": "...",
      "moniker": "...",
      // ... all fields ...
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 500,
    "totalPages": 10
  }
}
```

**Filter Profiles**
```
GET /api/v1/admin/profiles?userType=enterprise&primaryLanguage=en&timezone=America/Los_Angeles
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": [/* matching profiles */],
  "filters": {
    "userType": "enterprise",
    "primaryLanguage": "en",
    "timezone": "America/Los_Angeles"
  },
  "count": 15
}
```

**Search Profiles**
```
GET /api/v1/admin/profiles/search?q=Engineering&fields=jobRole,profileInfo
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "query": "Engineering",
  "searchFields": ["jobRole", "profileInfo"],
  "data": [/* matching profiles */],
  "count": 8
}
```

#### Field-Specific Update Endpoints

**Update Agent Preferences (Partial)**
```
PUT /api/v1/admin/profiles/{userID}/preferences
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body (only include preferences to update):
{
  "communicationStyle": "technical",
  "verbosity": "detailed",
  "notificationPreferences": {
    "taskCompletion": false
  }
}

Response (200 OK):
{
  "success": true,
  "data": {
    "agentPreferences": {
      "communicationStyle": "technical",     // UPDATED
      "verbosity": "detailed",               // UPDATED
      "responseFormat": "markdown",          // unchanged
      "codeExamplesPreferred": true,         // unchanged
      "notificationPreferences": {
        "taskCompletion": false,             // UPDATED
        "errors": true,                      // unchanged
        "longRunningOps": true               // unchanged
      }
    }
  },
  "updated_fields": ["communicationStyle", "verbosity", "notificationPreferences.taskCompletion"]
}
```

**Add Admin Note**
```
POST /api/v1/admin/profiles/{userID}/notes
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "note": "User requested access to Bedrock provider"
}

Response (200 OK):
{
  "success": true,
  "data": {
    "notesInformation": {
      "adminNotes": [
        {
          "timestamp": "2026-02-07T11:30:00Z",
          "authorID": "admin-user-id",
          "note": "User requested access to Bedrock provider"
        }
      ]
    }
  }
}
```

**Update Flags and Restrictions**
```
PUT /api/v1/admin/profiles/{userID}/flags
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body (partial update):
{
  "flags": {
    "betaTester": true,
    "earlyAccess": true
  },
  "restrictions": {
    "featureAccess": ["bedrock", "advanced-tools", "api-access"]
  }
}

Response (200 OK):
{
  "success": true,
  "data": {
    "flags": {
      "betaTester": true,
      "earlyAccess": true
    },
    "restrictions": {
      "featureAccess": ["bedrock", "advanced-tools", "api-access"]
    }
  }
}
```

#### Platform Integration Endpoints

**Link Slack Account**
```
PUT /api/v1/admin/profiles/{userID}/integrations/slack
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "slackID": "U12345678"
}

Response (200 OK):
{
  "success": true,
  "data": {
    "slackID": "U12345678",
    "updatedAt": "2026-02-07T12:00:00Z"
  }
}
```

**Unlink Slack Account**
```
DELETE /api/v1/admin/profiles/{userID}/integrations/slack
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "message": "Slack integration removed",
  "data": {
    "slackID": null
  }
}
```

**Link Telegram Account**
```
PUT /api/v1/admin/profiles/{userID}/integrations/telegram
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "telegramID": "123456789"
}

Response (200 OK):
{
  "success": true,
  "data": {
    "telegramID": "123456789",
    "updatedAt": "2026-02-07T12:00:00Z"
  }
}
```

#### Bulk Operations

**Bulk Import Profiles**
```
POST /api/v1/admin/profiles/import
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "profiles": [
    {
      "userID": "...",
      "firstName": "...",
      // ... other fields ...
    }
  ],
  "options": {
    "updateExisting": true,
    "skipInvalid": false,
    "dryRun": false
  }
}

Response (200 OK):
{
  "success": true,
  "summary": {
    "total": 100,
    "created": 75,
    "updated": 20,
    "failed": 5
  },
  "errors": [
    {
      "index": 5,
      "userID": "...",
      "error": "Invalid email format"
    }
  ]
}
```

**Export Profiles**
```
GET /api/v1/admin/profiles/export?format=json&filter=userType:enterprise
Authorization: Bearer <admin-api-key>

Response (200 OK):
Content-Type: application/json
Content-Disposition: attachment; filename="profiles-2026-02-07.json"

{
  "exportedAt": "2026-02-07T12:30:00Z",
  "count": 150,
  "profiles": [/* all matching profiles */]
}
```

**Export Profiles (CSV)**
```
GET /api/v1/admin/profiles/export?format=csv
Authorization: Bearer <admin-api-key>

Response (200 OK):
Content-Type: text/csv
Content-Disposition: attachment; filename="profiles-2026-02-07.csv"

userID,moniker,firstName,lastName,primaryEmail,userType,createdAt
550e8400-e29b-41d4-a716-446655440000,jdoe,John,Doe,john@example.com,enterprise,2026-02-07T10:30:00Z
...
```

### User Self-Service Endpoints (Non-Admin)

Regular users can manage their own profile with limited access:

**Get Own Profile**
```
GET /api/v1/profile
Authorization: Bearer <user-api-key>

Response (200 OK): Returns user's own profile (excludes notesInformation)
```

**Update Own Profile (Partial)**
```
PUT /api/v1/profile
Authorization: Bearer <user-api-key>
Content-Type: application/json

Request Body (cannot modify: userID, userType, slackID, telegramID, notesInformation):
{
  "firstName": "Jane",
  "timezone": "America/New_York",
  "agentPreferences": {
    "communicationStyle": "casual"
  }
}

Response (200 OK): Returns updated profile
```

### Authentication and Authorization

**Admin Operations** (all `/api/v1/admin/*` endpoints):
- Require valid API key with Admin role (`User.Role == "admin"`)
- All operations logged with admin user ID
- 401 Unauthorized if no/invalid API key
- 403 Forbidden if non-admin attempts admin operation

**User Operations** (`/api/v1/profile`):
- Require valid API key (any role)
- Users can only access their own profile
- Attempting to access another user's profile returns 403 Forbidden

### Partial Update Semantics

**PUT Request Behavior**:
1. **Only provided fields are updated** - omitted fields remain unchanged
2. **Null values explicitly set field to NULL** (where allowed by schema)
3. **Empty strings** are treated as valid values (not ignored)
4. **Nested JSON objects** (agentPreferences, notesInformation):
   - Top-level merge: provided keys update, missing keys unchanged
   - To delete a nested key, explicitly set to `null`
5. **Validation** runs only on provided fields
6. **updatedAt timestamp** always updated on successful PUT

**Example - Nested JSON Partial Update**:
```json
// Current agentPreferences:
{
  "communicationStyle": "professional",
  "verbosity": "concise",
  "codeExamplesPreferred": true
}

// PUT Request:
{
  "agentPreferences": {
    "verbosity": "detailed"
  }
}

// Result:
{
  "communicationStyle": "professional",  // unchanged
  "verbosity": "detailed",               // updated
  "codeExamplesPreferred": true          // unchanged
}
```

### Error Responses

**400 Bad Request** - Invalid input:
```json
{
  "success": false,
  "error": "Validation failed",
  "details": [
    {
      "field": "primaryEmail",
      "error": "Invalid email format"
    },
    {
      "field": "primaryLanguage",
      "error": "Must be ISO 639-1 two-letter code"
    }
  ]
}
```

**404 Not Found** - User profile doesn't exist:
```json
{
  "success": false,
  "error": "User profile not found",
  "userID": "550e8400-e29b-41d4-a716-446655440000"
}
```

**409 Conflict** - Unique constraint violation:
```json
{
  "success": false,
  "error": "Conflict: slackID already in use",
  "field": "slackID",
  "value": "U12345678"
}
```

### Bot Management API Endpoints

#### Slack Bot CRUD Operations

**Create Slack Bot**
```
POST /api/v1/admin/bots/slack
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "botName": "Support Bot",
  "botType": "public",
  "slackBotToken": "xoxb-...",
  "slackAppToken": "xapp-...",
  "slackSigningSecret": "...",
  "slackTeamID": "T12345678",
  "slackBotUserID": "B12345678",
  "enabled": true,
  "socketMode": true,
  "allowedUserIDs": ["user-id-1", "user-id-2"],
  "metadata": {
    "description": "Primary support bot",
    "tags": ["support", "production"]
  }
}

Response (201 Created):
{
  "success": true,
  "data": {
    "botID": "550e8400-e29b-41d4-a716-446655440000",
    "botName": "Support Bot",
    "botType": "public",
    "enabled": true,
    "createdAt": "2026-02-07T13:00:00Z"
  }
}
```

**Get Slack Bot**
```
GET /api/v1/admin/bots/slack/{botID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": {
    "botID": "550e8400-e29b-41d4-a716-446655440000",
    "botName": "Support Bot",
    "botType": "public",
    "ownerUserID": null,
    "slackTeamID": "T12345678",
    "slackBotUserID": "B12345678",
    "enabled": true,
    "socketMode": true,
    "allowedUserIDs": ["user-id-1", "user-id-2"],
    "metadata": {...},
    "createdAt": "2026-02-07T13:00:00Z",
    "updatedAt": "2026-02-07T13:00:00Z",
    "lastConnectedAt": "2026-02-07T13:05:00Z"
  }
}

Note: Sensitive fields (tokens, secrets) are never returned in API responses
```

**Update Slack Bot (Partial)**
```
PUT /api/v1/admin/bots/slack/{botID}
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body (only include fields to update):
{
  "botName": "Enterprise Support Bot",
  "enabled": false,
  "allowedUserIDs": ["user-id-1", "user-id-2", "user-id-3"]
}

Response (200 OK):
{
  "success": true,
  "data": {
    "botID": "550e8400-e29b-41d4-a716-446655440000",
    "botName": "Enterprise Support Bot",
    "enabled": false,
    "allowedUserIDs": ["user-id-1", "user-id-2", "user-id-3"],
    "updatedAt": "2026-02-07T14:00:00Z"
  },
  "updated_fields": ["botName", "enabled", "allowedUserIDs"]
}
```

**Delete Slack Bot**
```
DELETE /api/v1/admin/bots/slack/{botID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "message": "Slack bot deleted successfully",
  "deletedBotID": "550e8400-e29b-41d4-a716-446655440000"
}
```

**List Slack Bots**
```
GET /api/v1/admin/bots/slack?botType=public&enabled=true
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": [
    {
      "botID": "...",
      "botName": "Support Bot",
      "botType": "public",
      "enabled": true,
      "lastConnectedAt": "2026-02-07T13:05:00Z"
    }
  ],
  "count": 5
}
```

**Enable/Disable Slack Bot**
```
PATCH /api/v1/admin/bots/slack/{botID}/enable
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "enabled": false
}

Response (200 OK):
{
  "success": true,
  "data": {
    "botID": "550e8400-e29b-41d4-a716-446655440000",
    "enabled": false,
    "updatedAt": "2026-02-07T14:30:00Z"
  },
  "message": "Bot disabled successfully. Will disconnect on next gateway cycle."
}
```

**Update Allowed Users (Public Bots)**
```
PUT /api/v1/admin/bots/slack/{botID}/users
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "allowedUserIDs": ["user-id-1", "user-id-2", "user-id-3"]
}

Response (200 OK):
{
  "success": true,
  "data": {
    "allowedUserIDs": ["user-id-1", "user-id-2", "user-id-3"],
    "updatedAt": "2026-02-07T15:00:00Z"
  }
}
```

#### Telegram Bot CRUD Operations

**Create Telegram Bot**
```
POST /api/v1/admin/bots/telegram
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "botName": "Personal Assistant",
  "botType": "private",
  "ownerUserID": "user-id-123",
  "telegramBotToken": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
  "telegramBotUsername": "my_assistant_bot",
  "telegramBotID": "123456789",
  "enabled": true,
  "webhookMode": false,
  "metadata": {
    "description": "Personal AI assistant",
    "tags": ["private", "production"]
  }
}

Response (201 Created):
{
  "success": true,
  "data": {
    "botID": "660e8400-e29b-41d4-a716-446655440001",
    "botName": "Personal Assistant",
    "botType": "private",
    "ownerUserID": "user-id-123",
    "enabled": true,
    "createdAt": "2026-02-07T13:00:00Z"
  }
}
```

**Get Telegram Bot**
```
GET /api/v1/admin/bots/telegram/{botID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": {
    "botID": "660e8400-e29b-41d4-a716-446655440001",
    "botName": "Personal Assistant",
    "botType": "private",
    "ownerUserID": "user-id-123",
    "telegramBotUsername": "my_assistant_bot",
    "telegramBotID": "123456789",
    "enabled": true,
    "webhookMode": false,
    "metadata": {...},
    "createdAt": "2026-02-07T13:00:00Z",
    "lastConnectedAt": "2026-02-07T13:05:00Z"
  }
}
```

**Update Telegram Bot (Partial)**
```
PUT /api/v1/admin/bots/telegram/{botID}
Authorization: Bearer <admin-api-key>
Content-Type: application/json

Request Body:
{
  "enabled": false,
  "allowedChatIDs": ["123456", "789012"]
}

Response (200 OK):
{
  "success": true,
  "data": {
    "botID": "660e8400-e29b-41d4-a716-446655440001",
    "enabled": false,
    "allowedChatIDs": ["123456", "789012"],
    "updatedAt": "2026-02-07T14:00:00Z"
  },
  "updated_fields": ["enabled", "allowedChatIDs"]
}
```

**Delete Telegram Bot**
```
DELETE /api/v1/admin/bots/telegram/{botID}
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "message": "Telegram bot deleted successfully",
  "deletedBotID": "660e8400-e29b-41d4-a716-446655440001"
}
```

**List Telegram Bots**
```
GET /api/v1/admin/bots/telegram?ownerUserID=user-id-123
Authorization: Bearer <admin-api-key>

Response (200 OK):
{
  "success": true,
  "data": [
    {
      "botID": "...",
      "botName": "Personal Assistant",
      "botType": "private",
      "ownerUserID": "user-id-123",
      "enabled": true,
      "lastConnectedAt": "2026-02-07T13:05:00Z"
    }
  ],
  "count": 2
}
```

#### User Self-Service Bot Endpoints (Private Bots Only)

**List Own Bots**
```
GET /api/v1/bots/slack
GET /api/v1/bots/telegram
Authorization: Bearer <user-api-key>

Response (200 OK):
{
  "success": true,
  "data": [
    {
      "botID": "...",
      "botName": "My Personal Bot",
      "botType": "private",
      "enabled": true,
      "lastConnectedAt": "2026-02-07T13:05:00Z"
    }
  ]
}
```

**Get Own Bot**
```
GET /api/v1/bots/slack/{botID}
GET /api/v1/bots/telegram/{botID}
Authorization: Bearer <user-api-key>

Response (200 OK): Returns bot if owned by user, 403 otherwise
```

**Update Own Bot (Limited Fields)**
```
PUT /api/v1/bots/slack/{botID}
PUT /api/v1/bots/telegram/{botID}
Authorization: Bearer <user-api-key>
Content-Type: application/json

Request Body (users can only update: botName, enabled, metadata):
{
  "botName": "Updated Bot Name",
  "enabled": false
}

Response (200 OK): Returns updated bot
```

### Bot Management CLI Commands

**Slack Bot Commands**:
```bash
# Create Slack bot
nuimanbot admin bot slack create --name "Support Bot" --type public \
  --bot-token "xoxb-..." --signing-secret "..." \
  --team-id "T12345678" --bot-user-id "B12345678"

# List Slack bots
nuimanbot admin bot slack list --enabled --type public

# Enable/Disable bot
nuimanbot admin bot slack disable <bot-id>
nuimanbot admin bot slack enable <bot-id>

# Update allowed users (public bot)
nuimanbot admin bot slack users add <bot-id> <user-id>
nuimanbot admin bot slack users remove <bot-id> <user-id>
nuimanbot admin bot slack users set <bot-id> <user-id-1> <user-id-2>

# View bot details
nuimanbot admin bot slack view <bot-id>

# Delete bot
nuimanbot admin bot slack delete <bot-id>
```

**Telegram Bot Commands**:
```bash
# Create Telegram bot
nuimanbot admin bot telegram create --name "Personal Assistant" --type private \
  --owner-user-id "user-id-123" --bot-token "123456:ABC..." \
  --username "my_bot" --bot-id "123456789"

# List Telegram bots
nuimanbot admin bot telegram list --owner "user-id-123"

# Enable/Disable bot
nuimanbot admin bot telegram disable <bot-id>
nuimanbot admin bot telegram enable <bot-id>

# Update allowed users (public bot)
nuimanbot admin bot telegram users add <bot-id> <user-id>

# View bot details
nuimanbot admin bot telegram view <bot-id>

# Delete bot
nuimanbot admin bot telegram delete <bot-id>
```

**User Self-Service Commands**:
```bash
# List own bots
nuimanbot bot list

# View own bot
nuimanbot bot view <bot-id>

# Enable/Disable own bot
nuimanbot bot enable <bot-id>
nuimanbot bot disable <bot-id>
```

### OpenAI-Compatible API Integration

User profile context automatically included in system prompts when using OpenAI-compatible endpoint:

```
System: You are assisting Jane Doe (preferred name: Jane), an Enterprise user in the America/Los_Angeles timezone.
Communication style: professional, concise responses preferred.
Primary language: English (en), Secondary: Spanish (es).
```

## Security Considerations

### Data Privacy

1. **Sensitive Fields**: Email, phone, location are PII - require encryption at rest
2. **Access Control**: Regular users can only view/edit their own profile
3. **Admin Audit Log**: All admin profile modifications logged with timestamp and admin ID
4. **External IDs**: Slack/Telegram IDs should not be exposed in public APIs
5. **Bot Tokens**: All bot tokens and secrets MUST be encrypted at rest
6. **Token Exposure**: Bot tokens and secrets NEVER returned in API responses
7. **Bot Access Isolation**: Private bots strictly isolated to owner; public bots enforce allowedUserIDs

### Authentication

1. **Profile Updates**: Require active session or valid API key
2. **Admin Operations**: Require Admin role (User.Role == "admin")
3. **Bulk Operations**: Require Admin role + additional confirmation
4. **Bot Management**:
   - Creating/deleting public bots: Admin role required
   - Creating/deleting private bots: User can manage their own, admins can manage all
   - Updating bot credentials: Bot owner or admin only
   - Viewing bot configurations: Bot owner or admin (sensitive fields redacted)
   - Managing allowedUserIDs on public bots: Admin only

### Validation

1. **Email Validation**: RFC 5322 compliant email format
2. **Phone Validation**: E.164 format for international compatibility
3. **Language Codes**: ISO 639-1 two-letter codes only
4. **Timezone**: IANA timezone database entries only
5. **JSON Schema**: Validate agentPreferences and notesInformation against schemas

## Performance Considerations

1. **Indexing**: Create indexes on frequently queried fields (slackID, telegramID, primaryEmail)
2. **Caching**: Cache user profiles in memory for active sessions
3. **Lazy Loading**: Load agentPreferences only when needed for agent interaction
4. **Bulk Operations**: Use transactions for consistency, batch inserts for performance

## Implementation Phases

### Phase 0: Configuration Restructuring (Week 1)
- [ ] Update config structs with new fields (PathsConfig, enabled flags, model_name)
- [ ] Implement path resolution utility (ResolvePath)
- [ ] Update config loader to support provider inheritance
- [ ] Add default model fallback logic (primary → secondary → tertiary)
- [ ] Update gateway initialization to check enabled flags
- [ ] Update provider initialization to check enabled flags
- [ ] Remove hardcoded paths throughout codebase
- [ ] Update all file operations to use config.ResolvePath()
- [ ] Add JSON schema validation for config files
- [ ] Update config.example.yaml with new structure
- [ ] Write configuration migration guide
- [ ] Test configuration loading with environment variable overrides
- [ ] Test container deployment with separated paths
- [ ] Write comprehensive config tests

### Phase 1: Foundation - File-Based Storage (Week 2-3)
- [ ] Define domain entities (UserProfile, UserType, AgentPreferences, NotesInformation)
- [ ] Design JSON schemas for users.json and user directory files
- [ ] Implement file I/O layer with proper locking (prevent concurrent writes)
- [ ] Implement users.json manager (read, write, update, index management)
- [ ] Implement user directory manager (create directories, manage user files)
- [ ] Implement bots.json manager
- [ ] Add encryption utilities for sensitive data (bot tokens, secrets)
- [ ] Implement atomic file writes (write to temp file, then rename)
- [ ] Write comprehensive unit tests for file operations (TDD)
- [ ] Test concurrent access scenarios
- [ ] Implement backup/restore functionality

### Phase 2: Use Cases (Week 4)
- [ ] Implement profile management use cases
- [ ] Implement agent preferences use cases
- [ ] Implement admin notes use cases
- [ ] Integration tests for use case layer

### Phase 3: CLI Adapter (Week 5)
- [ ] Add `profile` command group for user self-service
- [ ] Add `admin profile` command group for admin operations
- [ ] Add import/export functionality
- [ ] CLI integration tests

### Phase 4: Agent Integration (Week 6)
- [ ] Modify agent initialization to load user profile
- [ ] Implement agent preference application logic
- [ ] Test agent behavior customization
- [ ] End-to-end agent personalization tests

### Phase 5: Platform Integration (Week 7)
- [ ] Implement Slack OAuth flow and ID linking
- [ ] Implement Telegram bot authentication and ID linking
- [ ] Test cross-platform user identification
- [ ] Integration tests for platform linking

### Phase 6: REST API Implementation (Week 8-9)
- [ ] Implement all CRUD endpoints for user profiles (admin)
- [ ] Implement partial update logic for PUT operations
- [ ] Implement field-specific update endpoints (preferences, notes, flags)
- [ ] Implement platform integration endpoints (Slack, Telegram)
- [ ] Implement list, filter, and search endpoints
- [ ] Implement bulk import/export endpoints (JSON, CSV)
- [ ] Implement user self-service endpoints (non-admin)
- [ ] Add OpenAI-compatible API profile context injection
- [ ] Implement role-based access control (RBAC) middleware
- [ ] Implement admin audit logging for all modifications
- [ ] Add request validation and error handling
- [ ] Write comprehensive API integration tests
- [ ] Generate OpenAPI/Swagger documentation

### Phase 7: Data Migration (Week 10)
- [ ] Write migration script: SQLite users → users.json
- [ ] Implement dual-read logic (users.json first, fallback to SQLite)
- [ ] Implement dual-write logic (write to both during transition)
- [ ] Create user directories for all existing users
- [ ] Generate default profile.json, preferences.json for each user
- [ ] Test migration on development data
- [ ] Create backup of SQLite database before migration
- [ ] Execute migration with verification checks
- [ ] Monitor dual-write period for issues
- [ ] Verify data integrity (checksums, count validation)

### Phase 8: Bot Gateway Integration (Week 11-12)
- [ ] Define SlackBot and TelegramBot domain entities
- [ ] Implement bots.json manager with encryption for tokens/secrets
- [ ] Implement bot CRUD operations (create, read, update, delete)
- [ ] Implement bot enable/disable functionality
- [ ] Implement access control logic (private vs public bots)
- [ ] Add bot management CLI commands (admin bot slack/telegram)
- [ ] Implement REST API endpoints for bot management
- [ ] Migrate gateway from config-based to bots.json-driven bot loading
- [ ] Update gateway initialization to read from bots.json
- [ ] Implement hot-reload: detect bots.json changes and reconnect bots
- [ ] Implement dynamic bot connection/disconnection based on enabled flag
- [ ] Update connection status tracking in bots.json (lastConnectedAt, lastDisconnectedAt)
- [ ] Write comprehensive tests for bot management
- [ ] Test bot enable/disable without gateway restart
- [ ] Test concurrent bot configuration updates
- [ ] Deprecate gateway.telegram and gateway.slack config sections
- [ ] Add bots.json to .gitignore (contains sensitive tokens)

### Phase 9: Administration Web Interface (Week 13-15)
- [ ] Design web interface UI/UX (wireframes, page layouts)
- [ ] Set up web server framework (Chi or Echo)
- [ ] Implement authentication and session management
- [ ] Implement CSRF protection and security headers
- [ ] Build Dashboard page (server status, metrics)
- [ ] Build LLM Configuration page (providers, models, default selection)
- [ ] Build Server Configuration page (paths, logging, gateways)
- [ ] Build User Management page (list, create, edit, delete)
- [ ] Build Bot Management page (Slack and Telegram bots)
- [ ] Build Activity Log page (audit log viewer)
- [ ] Implement JSON API endpoints for CLI integration
- [ ] Add file watching for automatic configuration reload
- [ ] Implement atomic file writes (temp + rename)
- [ ] Add configuration validation before write
- [ ] Style with Tailwind CSS
- [ ] Add HTMX for dynamic updates
- [ ] Add Alpine.js for client-side interactivity
- [ ] Write integration tests for web interface
- [ ] Test all configuration changes via web UI
- [ ] Document web admin interface usage

### Phase 10: CLI Tool Development (Week 16-17)
- [ ] Design CLI command structure (subcommands, flags)
- [ ] Implement configuration management commands
  - [ ] `nuimanbot config get <key>`
  - [ ] `nuimanbot config set <key> <value>`
  - [ ] `nuimanbot config list`
- [ ] Implement user management commands
  - [ ] `nuimanbot admin user create`
  - [ ] `nuimanbot admin user list`
  - [ ] `nuimanbot admin user edit`
  - [ ] `nuimanbot admin user delete`
- [ ] Implement bot management commands
  - [ ] `nuimanbot admin bot slack create/list/edit/delete`
  - [ ] `nuimanbot admin bot telegram create/list/edit/delete`
- [ ] Implement server control commands
  - [ ] `nuimanbot server reload` (trigger /refresh)
  - [ ] `nuimanbot server status`
  - [ ] `nuimanbot server logs`
- [ ] Add API client for JSON endpoint communication
- [ ] Implement output formatting (table, JSON, YAML)
- [ ] Add command autocomplete (bash, zsh)
- [ ] Write comprehensive CLI tests
- [ ] Test configuration file modifications
- [ ] Test server reload triggering
- [ ] Document CLI commands and usage

### Phase 11: STDIO Command Cleanup (Week 18)
- [ ] Remove all STDIO commands except /refresh and /exit
- [ ] Update STDIO command handler to support only:
  - [ ] `/refresh` - Reload configuration from files
  - [ ] `/exit` - Graceful shutdown
- [ ] Add deprecation warnings for removed commands
- [ ] Update REPL prompt and help text
- [ ] Test /refresh command with all config file types
- [ ] Test graceful shutdown with /exit
- [ ] Document new STDIO command model

### Phase 12: Documentation & Polish (Week 19)
- [ ] Update technical documentation with all new features
- [ ] Document configuration restructuring and migration
- [ ] Write user guide for profile management
- [ ] Write admin guide for bot management and bulk operations
- [ ] Document bot setup process (Slack app creation, Telegram BotFather)
- [ ] Update config.example.yaml with complete examples
- [ ] Create Docker/container deployment guide
- [ ] Prepare release notes

## Success Metrics

### User Profiles
1. **Adoption Rate**: % of users who complete their profile within 30 days
2. **Agent Personalization**: User satisfaction scores improve by 20%
3. **Platform Integration**: % of Slack/Telegram users successfully linked
4. **Admin Efficiency**: Time to complete bulk user operations reduced by 50%
5. **Data Quality**: < 5% of profiles with invalid/incomplete required fields

### Bot Gateway
6. **Bot Uptime**: 99.9% uptime for enabled bots
7. **Bot Creation Time**: < 5 minutes from bot creation to first message processed
8. **Connection Success Rate**: > 98% successful bot connections on gateway startup
9. **Enable/Disable Latency**: < 30 seconds from disable command to bot disconnection
10. **Bot Adoption**: > 60% of active users using bot gateway within 90 days
11. **Public Bot Usage**: Average 10+ users per public bot
12. **Bot Error Rate**: < 1% of bot messages result in errors

## Open Questions

### User Profiles
1. Should profiles support multiple languages beyond primary/secondary?
2. Should we support profile visibility settings (public/private fields)?
3. How to handle profile data retention when users are deleted?
4. Should agent preferences be version-controlled for rollback?
5. What's the strategy for handling conflicts between User.Email and UserProfile.primaryEmail?

### Bot Gateway
6. Should we support bot-level rate limiting separate from user rate limiting?
7. How to handle bot credential rotation without service interruption?
8. Should disabled bots queue incoming messages for when they're re-enabled?
9. Should we support multiple bots per user (e.g., one for Slack, one for Telegram)?
10. How to handle bot ownership transfer (e.g., user leaves organization)?
11. Should public bots support role-based access (e.g., only enterprise users)?
12. What's the strategy for bot failover if primary bot fails?
13. Should we log all bot interactions for audit purposes?
14. How to handle Slack workspace migrations (team ID changes)?
15. Should we support bot scheduling (enable/disable on schedule)?

## References

- Current User model: `internal/domain/user.go`
- Current database schema: `internal/adapter/repository/sqlite/schema.go`
- ISO 639-1 language codes: https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
- IANA timezone database: https://www.iana.org/time-zones
- E.164 phone format: https://en.wikipedia.org/wiki/E.164

---

**Document Version**: 2.1
**Last Updated**: 2026-02-07
**Author**: NuimanBot Development Team
**Status**: Draft - Awaiting Review

**Changelog**:
- **v2.1 (2026-02-07)**: Added administration architecture and web interface
  - Application architecture: Core server (nuimanbotd) + CLI tool (nuimanbot) + Web admin interface
  - STDIO commands simplified to /refresh and /exit only
  - Comprehensive web admin interface specification (dashboard, LLM config, server config, user management, bot management, activity logs)
  - JSON API endpoints for CLI integration
  - CLI tool commands for configuration, user, bot, and server management
  - Implementation phases for web interface (Phase 9), CLI tool (Phase 10), and STDIO cleanup (Phase 11)
  - Total timeline extended to 19 weeks
- **v2.0 (2026-02-07)**: MAJOR CHANGE - Shifted from database-centric to file-based storage approach
  - User profiles stored in `users.json` (central registry) + user-specific directories
  - Bot configurations stored in `bots.json` instead of database tables
  - User data organized in subdirectories: `users/<user-id>/` containing profile.json, preferences.json, todos.json, repeated-actions.json, history.json, projects/, notes/
  - Rationale: Simplicity, portability, human-readability, container-friendliness, easier backup/restore
  - Updated migration strategy for SQLite → JSON file migration
  - Updated implementation phases to reflect file I/O operations with locking
  - Added JSON schemas and example data for users.json, bots.json, and user directory files
  - SQLite database retained for transactional data (conversations, messages, notes)
- v1.3 (2026-02-07): Added comprehensive configuration restructuring including: server paths (config/data/logs), gateway enabled flags, provider enabled flags and model_name, ID-based default models with fallback chain (primary/secondary/tertiary), provider inheritance model, path resolution utilities, container-friendly configuration, and Phase 0 implementation plan
- v1.2 (2026-02-07): Added SlackBots and TelegramBots tables with public/private bot support, enabled flag for gateway control, bot management REST API endpoints, CLI commands, use cases, and implementation phases
- v1.1 (2026-02-07): Added comprehensive REST API requirements with full CRUD operations, partial update semantics, and detailed endpoint specifications
- v1.0 (2026-02-07): Initial draft with UserProfiles data model, database schema, and implementation phases
