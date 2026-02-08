# Improved Admin Features - Specification

**Version:** 1.0
**Created:** 2026-02-08
**Status:** Draft
**Priority:** P0 (Critical)
**Effort:** X-Large (>80h)
**PRD Source:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/improved-admin-features-prd.md`

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Goals and Non-Goals](#goals-and-non-goals)
4. [User Requirements](#user-requirements)
5. [System Architecture](#system-architecture)
6. [Scope of Changes](#scope-of-changes)
7. [Breaking Changes](#breaking-changes)
8. [Success Criteria](#success-criteria)
9. [Risks and Mitigation](#risks-and-mitigation)
10. [Timeline](#timeline)
11. [References](#references)

---

## Executive Summary

### What is this feature?

This feature represents a comprehensive overhaul of NuimanBot's administration capabilities, transforming it from a simple CLI tool into a multi-component system with robust user management, configuration flexibility, and enterprise-grade administration features.

**Core Transformation:**
- Multi-component architecture: Core server daemon (nuimanbotd) + CLI tool (nuimanbot) + embedded web admin interface
- File-based user profile storage (JSON) with comprehensive identity, preferences, and organizational context
- Database-driven bot management for Slack and Telegram gateways
- Hot configuration reload without server restart
- Container-friendly configuration with path separation (config/data/logs)
- REST API for complete programmatic access to all admin functions
- Role-based access control with admin and user permissions

**Why now?**

Current limitations prevent NuimanBot from scaling beyond simple personal use:
- No way to manage users at scale (bulk import, export, search)
- No multi-platform integration (Slack/Telegram IDs not captured)
- No agent personalization per user
- Configuration changes require server restart
- Bot configurations hardcoded in YAML, not manageable at runtime
- No web-based administration interface
- No programmatic API access for admin operations

This prevents enterprise adoption and makes administration painful for even small teams.

### Core components to build

1. **Multi-Component Architecture**
   - Core server daemon (nuimanbotd) with STDIO simplification (/refresh, /exit only)
   - Standalone CLI tool (nuimanbot) for configuration and server management
   - Embedded web admin interface (port 8080) with authentication and RBAC

2. **Configuration System Overhaul**
   - Path configuration (config/data/logs separation)
   - Enable/disable flags for gateways and providers
   - Provider configuration inheritance
   - Fallback model selection (primary/secondary/tertiary)
   - Hot configuration reload via /refresh command

3. **User Profile Management**
   - File-based storage (users.json + user directories)
   - Comprehensive profile fields (name, language, timezone, location, job role, etc.)
   - Multi-platform integration (Slack ID, Telegram ID)
   - Agent personalization preferences
   - Admin notes and flags

4. **Bot Gateway Management**
   - Database-driven bot configurations (bots.json)
   - Public and private bot types
   - Dynamic enable/disable without gateway restart
   - Bot-user relationship management

5. **REST API**
   - Full CRUD operations for all entities
   - Partial update support (PUT updates only specified fields)
   - Role-based access control
   - Audit logging

6. **Web Admin Interface**
   - Dashboard (server status, metrics, activity log)
   - LLM configuration (providers, models, defaults)
   - Server configuration (paths, logging, gateways)
   - User management (create, edit, search, bulk operations)
   - Bot management (Slack/Telegram bot CRUD)
   - Activity log viewer

### Impact

**Files/Packages Affected:**
- `internal/domain/`: New entities (UserProfile, BotConfig, ServerConfig)
- `internal/usecase/`: New use cases (profile management, bot management, config management)
- `internal/adapter/`: New adapters (file-based repositories, REST handlers, web UI handlers)
- `internal/infrastructure/`: New infrastructure (file storage, hot reload, encryption)
- `cmd/nuimanbotd/`: New server daemon entry point
- `cmd/nuimanbot/`: Enhanced CLI tool with admin commands
- Configuration files: `config.yaml` restructured, `users.json` and `bots.json` added

**Database Changes:**
- No SQL database changes required
- File-based storage for users and bots (JSON files)
- Existing SQLite database remains for conversations and messages

**Configuration Changes:**
- `config.yaml` structure enhanced with paths, enable flags, provider inheritance
- New `users.json` central registry
- New `bots.json` bot configurations
- Directory structure: `<data-dir>/users/<user-id>/` for user-specific data

**Documentation Updates:**
- Product docs (README, product-summary, product-details, technical-details)
- API documentation for REST endpoints
- Admin guide for web interface and CLI commands
- Configuration migration guide

### Compatibility

**Backward Compatibility:**
- Partially backward compatible with migration path
- Existing configuration files can be migrated automatically
- Existing SQLite database schema unchanged
- Users must migrate to new multi-component architecture

**Breaking Changes:**
- Server binary renamed from `nuimanbot` to `nuimanbotd`
- CLI tool remains `nuimanbot` but behavior changes (no longer runs server)
- Configuration file structure changes require migration
- STDIO commands simplified (only /refresh and /exit)

**Migration Path:**
1. Run configuration migration tool to update config.yaml
2. Initialize users.json and bots.json from existing database
3. Update systemd/docker configs to use new binary names
4. Test web admin interface and REST API
5. Update monitoring/alerting for new server architecture

---

## Problem Statement

### Current State

**Limitations:**

1. **No User Profile Management**
   - Current User entity has minimal fields (ID, Username, Email, Role, APIKeys)
   - No structured storage for personal information (names, phone, location)
   - No multi-language support tracking
   - No integration IDs for external platforms (Slack, Telegram)
   - No job role or organizational context
   - No agent personalization preferences

2. **Monolithic Architecture**
   - Single binary serves as both server and CLI
   - No separation of concerns between runtime and administration
   - STDIO interface cluttered with management commands
   - No web-based administration

3. **Static Configuration**
   - Configuration changes require server restart
   - No hot reload capability
   - Gateways cannot be enabled/disabled dynamically
   - Provider configurations require full specification (no inheritance)

4. **Bot Configuration in YAML**
   - Bot credentials hardcoded in config.yaml
   - Cannot add/remove bots without editing files and restarting
   - No public vs private bot distinction
   - No per-user bot ownership

5. **No Admin API**
   - No REST API for programmatic administration
   - All admin tasks require SSH access and file editing
   - No role-based access control
   - No audit logging

6. **Container-Unfriendly**
   - Paths hardcoded throughout codebase
   - Cannot separate config, data, and logs into different volumes
   - No environment variable overrides for paths

**Use Cases We Can't Support:**

- Enterprise user onboarding with bulk import
- Multi-platform user routing (Slack message → correct user profile)
- Agent personalization per user (communication style, verbosity)
- Runtime bot management (add bot without restart)
- Web-based administration for non-technical admins
- API-driven automation of admin tasks
- Container deployments with separate volumes

**Example Desired Workflow:**

```bash
# Admin creates new user via web interface
# Opens http://localhost:8080/admin/users
# Fills form: username, email, first name, last name, timezone, Slack ID
# Clicks "Create User"
# User immediately available, no restart needed

# Admin adds new Slack bot via CLI
nuimanbot admin bot slack create \
  --name "Team Assistant" \
  --type public \
  --bot-token "xoxb-..." \
  --app-token "xapp-..." \
  --allowed-users "user1,user2,user3"

# Server automatically detects new bot and starts connection
# No restart required
```

**Pain Points:**

- Editing YAML files error-prone and requires technical knowledge
- No way to manage users at scale (10+ users becomes painful)
- Bot credential management insecure (plain text in YAML)
- Cannot customize agent behavior per user
- No audit trail of configuration changes
- Server restarts disrupt active conversations

### Why This Matters

**User Impact:**

Without these features, NuimanBot remains a personal tool rather than a team/enterprise solution. Users cannot:
- Have personalized agent experiences
- Use multiple platforms seamlessly (Slack, Telegram, CLI)
- Manage bots without technical expertise
- Onboard teams efficiently
- Customize behavior per organizational role

**Business Impact:**

- Blocks enterprise adoption
- Increases support burden (manual user management)
- Limits revenue potential (cannot sell to teams)
- Competitive disadvantage (other AI assistants have admin interfaces)
- Operational overhead (manual configuration changes)

**Technical Debt:**

Current architecture tightly couples administration with runtime concerns. This prevents:
- Clean separation of responsibilities
- Automated testing of admin features
- API-driven integrations
- Scalability improvements
- Container-native deployments

---

## Goals and Non-Goals

### Goals

**Primary Goals:**

1. **Multi-Component Architecture**
   - Separate core server (nuimanbotd) from CLI tool (nuimanbot)
   - Embed web admin interface in server process
   - Simplify server STDIO to /refresh and /exit only
   - Enable headless server operation

2. **Comprehensive User Profiles**
   - Capture full user identity (name, language, timezone, location, job role)
   - Support multi-platform integration (Slack ID, Telegram ID)
   - Store agent personalization preferences
   - Enable admin notes and flags

3. **Dynamic Configuration Management**
   - Hot reload configuration without restart
   - Enable/disable gateways and providers dynamically
   - Inherit provider configurations to reduce duplication
   - Support fallback model selection

4. **Database-Driven Bot Management**
   - Store bot configurations in bots.json
   - Support public and private bot types
   - Enable/disable bots without restart
   - Manage bot-user relationships

5. **REST API for All Admin Operations**
   - Full CRUD for users, profiles, bots, configuration
   - Partial update support (PUT only updates specified fields)
   - Role-based access control
   - Audit logging

6. **Web Admin Interface**
   - Browser-based configuration management
   - User-friendly forms for user/bot creation
   - Real-time server status and metrics
   - Configuration refresh capability

7. **Container-Friendly Configuration**
   - Separate paths for config, data, logs
   - Environment variable overrides
   - Volume-mountable directories

**Secondary Goals:**

- Export/import capabilities for users and bots (JSON/CSV)
- Search and filter users by various criteria
- Bulk operations (import multiple users at once)
- Activity log with filtering and search
- Metrics dashboard (request counts, error rates)

### Non-Goals

**Explicitly Out of Scope:**

- **Database migration from SQLite to PostgreSQL/MySQL** - Rationale: File-based storage sufficient for current scale, database adds complexity
- **Multi-tenancy support** - Rationale: Single-organization focus for initial release, can be added later
- **OAuth/SAML authentication for web interface** - Rationale: API key-based auth sufficient, OAuth adds significant complexity
- **Real-time collaboration features** - Rationale: Admin tasks are typically single-user, real-time updates not critical
- **Mobile admin app** - Rationale: Web interface is responsive, native app not justified
- **Slack/Telegram OAuth flows for user linking** - Rationale: Manual ID entry sufficient, OAuth adds complexity
- **Automated bot credential rotation** - Rationale: Manual rotation acceptable for initial release
- **Multi-language support for web interface** - Rationale: English-only for admin interface, can be added later

**Future Considerations:**

- Advanced analytics and reporting (user activity trends, model usage statistics)
- Scheduled tasks and automation (cron jobs for admin operations)
- Webhook integrations for configuration changes
- Git-based configuration versioning
- Terraform/IaC support for configuration management
- Kubernetes operator for automated deployment

---

## User Requirements

### Functional Requirements

#### FR-001: Multi-Component Architecture
**Priority:** P0
**Description:** System must consist of separate server daemon and CLI tool

**Acceptance Criteria:**
- [ ] Server binary (nuimanbotd) runs continuously as daemon/service
- [ ] CLI tool (nuimanbot) operates independently for admin tasks
- [ ] Server STDIO accepts only /refresh and /exit commands
- [ ] All user interactions happen through gateways (CLI REPL, Slack, Telegram)
- [ ] Web admin interface embedded in server on configurable port (default 8080)
- [ ] Server can run headless without terminal interaction

**User Story:**
```
As a system administrator,
I want to run the NuimanBot server as a background daemon,
So that it operates continuously without terminal supervision.
```

#### FR-002: Hot Configuration Reload
**Priority:** P0
**Description:** Server must reload configuration files without restart

**Acceptance Criteria:**
- [ ] /refresh command reloads config.yaml, users.json, bots.json
- [ ] Configuration changes take effect immediately
- [ ] Active conversations continue uninterrupted
- [ ] Invalid configurations rejected with error message
- [ ] Web interface provides "Reload Config" button
- [ ] API endpoint POST /api/admin/config/reload available

**User Story:**
```
As a system administrator,
I want to update LLM provider settings without restarting the server,
So that active user conversations are not interrupted.
```

#### FR-003: Path Configuration
**Priority:** P0
**Description:** System must support configurable paths for config, data, and logs

**Acceptance Criteria:**
- [ ] server.paths.config, server.paths.data, server.paths.logs in config.yaml
- [ ] Default values: ./config/, ./data/, ./logs/
- [ ] Environment variable overrides (NUIMANBOT_SERVER_PATHS_CONFIG, etc.)
- [ ] All file operations use configured paths
- [ ] Paths validated on startup (must exist and be writable)
- [ ] Container deployments can mount separate volumes

**User Story:**
```
As a DevOps engineer,
I want to configure separate paths for config, data, and logs,
So that I can mount them as separate volumes in containerized deployments.
```

#### FR-004: Gateway Enable/Disable Flags
**Priority:** P0
**Description:** Gateways must be toggleable via configuration without code changes

**Acceptance Criteria:**
- [ ] gateways.cli.enabled flag in config.yaml (default: true)
- [ ] gateways.slack.enabled flag in config.yaml (default: false)
- [ ] gateways.telegram.enabled flag in config.yaml (default: false)
- [ ] Disabled gateways skipped during initialization
- [ ] Enabled gateways activated on /refresh command
- [ ] Environment variable overrides (NUIMANBOT_GATEWAYS_CLI_ENABLED, etc.)

**User Story:**
```
As a system administrator,
I want to enable or disable gateways via configuration flags,
So that I can toggle features without editing YAML structure.
```

#### FR-005: Provider Configuration Inheritance
**Priority:** P0
**Description:** LLM provider configurations must support inheritance to reduce duplication

**Acceptance Criteria:**
- [ ] Provider-specific sections (llm.anthropic, llm.openai, llm.ollama, llm.bedrock)
- [ ] Auto-generated default providers (id matches type name)
- [ ] Explicit provider instances inherit from type section
- [ ] Override support for api_key, base_url, and provider-specific fields
- [ ] Required fields: id, type, enabled, model_name
- [ ] Multiple providers of same type supported

**User Story:**
```
As a system administrator,
I want to define multiple Anthropic providers without duplicating API keys,
So that I can use different models with the same credentials.
```

#### FR-006: Fallback Model Selection
**Priority:** P0
**Description:** System must support primary, secondary, and tertiary model fallback

**Acceptance Criteria:**
- [ ] llm.default_model.primary references provider ID
- [ ] llm.default_model.secondary references provider ID (fallback 1)
- [ ] llm.default_model.tertiary references provider ID (fallback 2)
- [ ] Request attempts primary first
- [ ] On failure, attempts secondary, then tertiary
- [ ] Error returned only if all fallbacks fail
- [ ] User-specific model preferences override defaults

**User Story:**
```
As a user,
I want the system to automatically fallback to alternative models if primary fails,
So that I receive a response even when my preferred model is unavailable.
```

#### FR-007: File-Based User Profile Storage
**Priority:** P0
**Description:** User profiles must be stored in users.json with comprehensive fields

**Acceptance Criteria:**
- [ ] users.json contains central user registry
- [ ] User fields: userID, username, moniker, firstName, lastName, nickName, primaryEmail, backupEmail, mobilePhone, primaryLanguage, secondaryLanguage, timezone, primaryLocation, jobRole, userType, role, platformIDs, enabled, dataDirectory
- [ ] Indexes: byUsername, byEmail, byPlatform (nested: slack, telegram, cli)
- [ ] User directory structure: <data-dir>/users/<user-id>/
- [ ] User-specific files: profile.json, preferences.json, todos.json, repeated-actions.json, history.json
- [ ] Atomic file writes (temp file + rename)

**User Story:**
```
As a system administrator,
I want user profiles stored in JSON files,
So that I can easily backup, version control, and inspect user data.
```

#### FR-008: Multi-Platform Integration
**Priority:** P0
**Description:** User profiles must capture Slack, Telegram, and CLI identifiers

**Acceptance Criteria:**
- [ ] platformIDs.slack stores Slack user ID (e.g., "U01ABC123")
- [ ] platformIDs.telegram stores Telegram user ID (e.g., "123456789")
- [ ] platformIDs.cli stores CLI username
- [ ] Platform IDs used for message routing
- [ ] Indexes enable fast lookup by platform ID
- [ ] Platform IDs unique per platform (validated on create/update)

**User Story:**
```
As a user,
I want to use NuimanBot from Slack, Telegram, and CLI with the same profile,
So that my preferences and history are consistent across platforms.
```

#### FR-009: Agent Personalization Preferences
**Priority:** P1
**Description:** User profiles must store preferences for agent behavior customization

**Acceptance Criteria:**
- [ ] agentPreferences stored in user profile
- [ ] Fields: communicationStyle, verbosity, responseFormat, codeExamplesPreferred, explainDecisions, proactiveMode, skillDefaults, notificationPreferences
- [ ] Agent loads preferences on conversation start
- [ ] Agent adapts behavior based on preferences
- [ ] Preferences overridable per-request
- [ ] Default preferences for new users

**User Story:**
```
As a user,
I want to configure my preferred communication style and verbosity,
So that the agent responds in a way that matches my preferences.
```

#### FR-010: Bot Configuration Storage
**Priority:** P0
**Description:** Bot configurations must be stored in bots.json for dynamic management

**Acceptance Criteria:**
- [ ] bots.json contains slackBots and telegramBots arrays
- [ ] Slack bot fields: botID, botName, botType, ownerUserID, slackBotToken, slackAppToken, slackSigningSecret, slackTeamID, slackBotUserID, enabled, allowedUserIDs, metadata
- [ ] Telegram bot fields: botID, botName, botType, ownerUserID, telegramBotToken, telegramBotUsername, telegramBotID, enabled, allowedUserIDs, allowedChatIDs, metadata
- [ ] BotType enum: public, private
- [ ] Bot credentials encrypted at rest
- [ ] Indexes: byName, byOwner, byPlatformBotID

**User Story:**
```
As a system administrator,
I want to manage bot configurations in a database,
So that I can add, remove, and modify bots without editing YAML files.
```

#### FR-011: Public and Private Bots
**Priority:** P0
**Description:** Bot system must support both public (shared) and private (user-owned) bots

**Acceptance Criteria:**
- [ ] Private bots: botType=private, ownerUserID set, allowedUserIDs=null
- [ ] Public bots: botType=public, ownerUserID=null, allowedUserIDs contains user IDs
- [ ] Private bot access restricted to owner only
- [ ] Public bot access restricted to users in allowedUserIDs
- [ ] Admins can manage all bots
- [ ] Users can create and manage their own private bots

**User Story:**
```
As a user,
I want to create a private Telegram bot for personal use,
So that only I can interact with it and access my data.
```

#### FR-012: Dynamic Bot Enable/Disable
**Priority:** P0
**Description:** Bots must be enable/disable-able without gateway restart

**Acceptance Criteria:**
- [ ] Bot enabled flag in configuration
- [ ] Gateway polls bots.json for changes (or uses file watcher)
- [ ] Disabled bot disconnects from platform
- [ ] Enabled bot connects to platform
- [ ] Active conversations gracefully closed on disable
- [ ] No gateway restart required
- [ ] Status update visible in web interface

**User Story:**
```
As a system administrator,
I want to disable a misbehaving bot immediately,
So that I can stop it from processing messages without affecting other bots.
```

#### FR-013: REST API - Full CRUD Operations
**Priority:** P0
**Description:** All admin operations must be accessible via REST API

**Acceptance Criteria:**
- [ ] User profiles: GET, POST, PUT, DELETE at /api/v1/admin/profiles
- [ ] Bot configurations: GET, POST, PUT, DELETE at /api/v1/admin/bots/slack and /api/v1/admin/bots/telegram
- [ ] Configuration management: GET, PUT at /api/v1/admin/config/*
- [ ] Server control: POST /api/v1/admin/config/reload, GET /api/v1/admin/status
- [ ] All endpoints require authentication (Bearer token)
- [ ] Admin role required for all endpoints

**User Story:**
```
As an automation engineer,
I want to manage users and bots via REST API,
So that I can integrate NuimanBot administration into our provisioning scripts.
```

#### FR-014: REST API - Partial Updates
**Priority:** P0
**Description:** PUT operations must support updating only specified fields

**Acceptance Criteria:**
- [ ] Request body contains only fields to update
- [ ] Omitted fields remain unchanged
- [ ] Response includes updated_fields array
- [ ] Response includes full updated object
- [ ] Validation performed only on provided fields
- [ ] Timestamps (updatedAt) reflect change time

**User Story:**
```
As an API consumer,
I want to update only the user's timezone without sending the entire profile,
So that I can make targeted changes efficiently.
```

#### FR-015: REST API - Role-Based Access Control
**Priority:** P0
**Description:** API must enforce role-based access control

**Acceptance Criteria:**
- [ ] Admin role has full access to all endpoints
- [ ] User role can access only their own profile
- [ ] Service role has read-only access
- [ ] Authentication via API key in Authorization header
- [ ] API key validated against users.json
- [ ] Unauthorized requests return 403 Forbidden
- [ ] Unauthenticated requests return 401 Unauthorized

**User Story:**
```
As a security engineer,
I want API access controlled by user roles,
So that regular users cannot access admin functions.
```

#### FR-016: REST API - Audit Logging
**Priority:** P1
**Description:** All admin modifications must be logged with timestamp and admin ID

**Acceptance Criteria:**
- [ ] Audit log records: timestamp, adminUserID, operation, resource, resourceID, changes
- [ ] Log stored in <data-dir>/logs/audit.log
- [ ] Log rotation enabled (max size, max age)
- [ ] Web interface displays recent audit entries
- [ ] API endpoint GET /api/v1/admin/logs for programmatic access
- [ ] Log format: JSON for machine parsing

**User Story:**
```
As a compliance officer,
I want to review all administrative changes with timestamps and actor IDs,
So that I can audit system configuration changes.
```

#### FR-017: Web Admin Interface - Dashboard
**Priority:** P1
**Description:** Web interface must provide dashboard with system overview

**Acceptance Criteria:**
- [ ] Server status: uptime, version, memory usage
- [ ] Active connections: Slack bots, Telegram bots, CLI sessions
- [ ] Recent activity log (last 50 entries)
- [ ] Quick stats: total users, active bots, LLM requests (last 24h), error rate
- [ ] Reload configuration button
- [ ] System health indicators (green/yellow/red)

**User Story:**
```
As a system administrator,
I want to view server status and metrics at a glance,
So that I can quickly assess system health.
```

#### FR-018: Web Admin Interface - LLM Configuration
**Priority:** P1
**Description:** Web interface must provide LLM configuration management

**Acceptance Criteria:**
- [ ] Default model selection (primary, secondary, tertiary dropdowns)
- [ ] Provider configuration (Anthropic, OpenAI, Bedrock, Ollama)
- [ ] Model instance management (list, add, edit, delete)
- [ ] Enable/disable toggle for providers
- [ ] Test connection button for each provider
- [ ] Save changes button with validation

**User Story:**
```
As a system administrator,
I want to configure LLM providers through a web form,
So that I don't have to manually edit YAML files.
```

#### FR-019: Web Admin Interface - Server Configuration
**Priority:** P1
**Description:** Web interface must provide server configuration management

**Acceptance Criteria:**
- [ ] Server paths configuration (config, data, logs)
- [ ] Logging configuration (log level, debug mode)
- [ ] Gateway enable/disable toggles (CLI, Slack, Telegram)
- [ ] Admin port configuration
- [ ] Save changes button with validation
- [ ] Changes require explicit reload (button or automatic)

**User Story:**
```
As a system administrator,
I want to change server paths and logging settings via web interface,
So that I can configure the server without SSH access.
```

#### FR-020: Web Admin Interface - User Management
**Priority:** P0
**Description:** Web interface must provide user management UI

**Acceptance Criteria:**
- [ ] User list with search and filter
- [ ] Add user form (username, email, role, profile fields)
- [ ] Edit user form with tabs (Basic Info, Profile, Access)
- [ ] Delete user with confirmation
- [ ] Platform ID management (link/unlink Slack, Telegram)
- [ ] Agent preferences editor
- [ ] View full profile button

**User Story:**
```
As a system administrator,
I want to create and edit users through a web form,
So that I can onboard users without writing JSON files.
```

#### FR-021: Web Admin Interface - Bot Management
**Priority:** P0
**Description:** Web interface must provide bot management UI

**Acceptance Criteria:**
- [ ] Bot list (Slack and Telegram tabs)
- [ ] Add bot form (name, type, credentials)
- [ ] Edit bot form (modify credentials, enable/disable)
- [ ] Delete bot with confirmation
- [ ] Public bot user access management (add/remove users)
- [ ] Test bot connection button
- [ ] Bot status indicators (connected, disconnected, error)

**User Story:**
```
As a system administrator,
I want to add and configure Slack bots through a web form,
So that I can manage bot credentials securely.
```

#### FR-022: Web Admin Interface - Activity Log
**Priority:** P1
**Description:** Web interface must provide activity log viewer

**Acceptance Criteria:**
- [ ] Log table with columns: timestamp, user, action
- [ ] Filter by log level (all, info, warn, error)
- [ ] Filter by time range (last 1h, 24h, 7d, custom)
- [ ] Search by text
- [ ] Pagination (50 entries per page)
- [ ] Export log to CSV/JSON

**User Story:**
```
As a system administrator,
I want to view and search system activity logs,
So that I can troubleshoot issues and audit system usage.
```

#### FR-023: Web Admin Interface - Config Refresh UI
**Priority:** P1
**Description:** Web interface must provide configuration refresh capability

**Acceptance Criteria:**
- [ ] "Reload Configuration" button on dashboard
- [ ] Confirmation dialog before reload
- [ ] Progress indicator during reload
- [ ] Success/failure notification
- [ ] Error details displayed if reload fails
- [ ] No full page refresh required (AJAX/HTMX)

**User Story:**
```
As a system administrator,
I want to reload configuration from the web interface,
So that I can apply changes without SSH access to the server.
```

#### FR-024: CLI Tool - Admin User Commands
**Priority:** P0
**Description:** CLI tool must provide user management commands

**Acceptance Criteria:**
- [ ] nuimanbot admin user create --username <name> --email <email>
- [ ] nuimanbot admin user list [--format table|json|csv]
- [ ] nuimanbot admin user view <user-id>
- [ ] nuimanbot admin user update <user-id> --field <value>
- [ ] nuimanbot admin user delete <user-id>
- [ ] nuimanbot admin user import --file <path>
- [ ] nuimanbot admin user export --file <path>

**User Story:**
```
As a system administrator,
I want to manage users from the command line,
So that I can script user provisioning tasks.
```

#### FR-025: CLI Tool - Admin Bot Commands
**Priority:** P0
**Description:** CLI tool must provide bot management commands

**Acceptance Criteria:**
- [ ] nuimanbot admin bot slack create --name <name> --token <token> ...
- [ ] nuimanbot admin bot slack list [--format table|json]
- [ ] nuimanbot admin bot slack view <bot-id>
- [ ] nuimanbot admin bot slack update <bot-id> --field <value>
- [ ] nuimanbot admin bot slack delete <bot-id>
- [ ] nuimanbot admin bot slack enable <bot-id>
- [ ] nuimanbot admin bot slack disable <bot-id>
- [ ] Similar commands for telegram

**User Story:**
```
As a system administrator,
I want to manage bots from the command line,
So that I can automate bot provisioning and configuration.
```

#### FR-026: CLI Tool - Server Control Commands
**Priority:** P0
**Description:** CLI tool must provide server management commands

**Acceptance Criteria:**
- [ ] nuimanbot server reload (triggers /refresh via API)
- [ ] nuimanbot server status (shows uptime, version, active connections)
- [ ] nuimanbot server logs [--follow] [--level info|warn|error]
- [ ] nuimanbot config get <key> (reads config value)
- [ ] nuimanbot config set <key> <value> (updates config)
- [ ] nuimanbot config validate (validates config.yaml syntax)

**User Story:**
```
As a system administrator,
I want to control the server from the command line,
So that I can reload configuration and check status without web access.
```

### Non-Functional Requirements

#### NFR-001: Performance
- Hot configuration reload must complete within 5 seconds
- Web interface page load time < 2 seconds
- REST API response time < 500ms for CRUD operations (p95)
- File-based user lookup by ID < 50ms (p95)
- Bot connection startup < 10 seconds per bot
- Support 100+ users without performance degradation
- Support 50+ bots per gateway without performance degradation

#### NFR-002: Security
- Bot tokens and secrets encrypted at rest (AES-256)
- API authentication via Bearer token
- Web interface session timeout: 24 hours (configurable)
- CSRF protection on all web forms
- HTTPS support with TLS 1.2+
- Rate limiting on authentication endpoints (10 req/min)
- Audit logging of all admin operations
- Secure file permissions on users.json and bots.json (600)
- Environment variable support for sensitive configuration
- No secrets in logs or error messages

#### NFR-003: Scalability
- File-based storage supports 1000+ users
- Index-based lookups for O(1) user retrieval
- Pagination on all list endpoints (default 50, max 500)
- Batch operations for bulk user import (100+ users)
- Gateway supports 50+ concurrent bot connections
- Server handles 100+ concurrent LLM requests
- Log rotation prevents disk space exhaustion

#### NFR-004: Reliability
- Configuration validation before reload (reject invalid configs)
- Atomic file writes prevent data corruption (temp + rename)
- File locking prevents concurrent write conflicts
- Graceful degradation on missing optional configuration
- Active conversations continue during configuration reload
- Bot disconnections auto-reconnect with exponential backoff
- Database-backed message persistence (SQLite)
- Audit log durability (fsync on write)

#### NFR-005: Usability
- Web interface responsive (mobile-friendly)
- Web interface accessible (WCAG 2.1 AA compliance goal)
- CLI commands follow standard conventions (--help, --version)
- CLI output supports multiple formats (table, JSON, CSV)
- Error messages actionable and clear
- Configuration examples in documentation
- Migration guide for existing deployments
- API documentation (OpenAPI/Swagger spec)

#### NFR-006: Maintainability
- Clean architecture with clear layer boundaries
- TDD approach (tests before implementation)
- Code coverage > 80% for core logic
- API versioning (v1 prefix in routes)
- Database schema versioning (version field in JSON files)
- Backward-compatible configuration upgrades
- Deprecation warnings for old config syntax
- Logging at appropriate levels (debug, info, warn, error)

---

## System Architecture

### High-Level Design

**Architecture Diagram:**
```
┌──────────────────────────────────────────────────────────────────┐
│                      Core Server (nuimanbotd)                     │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐          │
│  │  LLM Engine  │  │  Gateways    │  │  Web Admin    │          │
│  │              │  │  - CLI REPL  │  │  Interface    │          │
│  │  Anthropic   │  │  - Slack     │  │               │          │
│  │  OpenAI      │  │  - Telegram  │  │  :8080/admin  │          │
│  │  Bedrock     │  │              │  │               │          │
│  │  Ollama      │  │              │  │  REST API     │          │
│  └──────────────┘  └──────────────┘  │  /api/v1/*    │          │
│         │                  │          └───────────────┘          │
│         └──────────────────┼──────────────────┘                  │
│                            │                                     │
│  ┌─────────────────────────▼──────────────────────┐             │
│  │        Configuration Manager                    │             │
│  │  - Hot-reload on /refresh or API trigger       │             │
│  │  - Watch: config.yaml, users.json, bots.json   │             │
│  └─────────────────────────────────────────────────┘             │
│                            │                                     │
│  ┌─────────────────────────▼──────────────────────┐             │
│  │        File Storage (JSON)                      │             │
│  │  - users.json (user registry + indexes)        │             │
│  │  - bots.json (bot configs + indexes)           │             │
│  │  - users/<id>/ (user-specific data)            │             │
│  └─────────────────────────────────────────────────┘             │
└──────────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼────────────────────┐
        │                   │                    │
        ▼                   ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  CLI Tool    │    │  Web Browser │    │  File System │
│  (nuimanbot) │    │              │    │              │
│              │    │  Admin UI    │    │  config/     │
│  - admin user│    │  localhost:  │    │  data/       │
│  - admin bot │    │  8080/admin  │    │  logs/       │
│  - server    │    │              │    │              │
│  - config    │    │              │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
```

**Key Components:**

1. **Core Server (nuimanbotd)**
   - Long-running daemon process
   - Embedded web server (port 8080)
   - LLM request orchestration
   - Gateway management
   - Configuration hot-reload

2. **CLI Tool (nuimanbot)**
   - Standalone admin utility
   - User/bot management
   - Server control (reload, status)
   - Configuration manipulation

3. **Web Admin Interface**
   - Server-side rendered HTML
   - HTMX for dynamic updates
   - Session-based authentication
   - Dashboard, users, bots, config, logs

4. **REST API**
   - JSON-based CRUD endpoints
   - Bearer token authentication
   - Role-based access control
   - Versioned (v1)

5. **Configuration Manager**
   - File watcher for config.yaml, users.json, bots.json
   - Hot reload logic
   - Validation before application
   - Rollback on error

6. **File Storage**
   - users.json: Central user registry with indexes
   - bots.json: Bot configurations with indexes
   - users/<id>/: User-specific directories
   - Atomic writes with file locking

**Data Flow:**

```
User Request → Gateway → Agent Engine → LLM Provider → Response
                  ↓
            User Profile Lookup (users.json)
                  ↓
            Agent Preferences Applied
                  ↓
            Personalized Response
```

**Admin Operation Flow:**

```
Admin Action (Web/CLI/API)
    ↓
Validation
    ↓
File Write (users.json or bots.json)
    ↓
Audit Log Entry
    ↓
[Optional] Trigger /refresh
    ↓
Configuration Reload
    ↓
Success Response
```

### Clean Architecture Layers

**Domain Layer:**
- `User` entity (existing, minimal changes)
- `UserProfile` entity (new, comprehensive profile fields)
- `BotConfig` entity (new, Slack/Telegram bot configurations)
- `ServerConfig` entity (enhanced, paths and enable flags)
- `AuditLog` entity (new, admin action tracking)
- Repository interfaces: `UserProfileRepository`, `BotConfigRepository`

**Use Case Layer:**
- `UserProfileService` (create, read, update, delete user profiles)
- `BotManagementService` (create, read, update, delete bots)
- `ConfigurationService` (read, update, reload configuration)
- `AdminService` (admin operations, audit logging)
- `AuthenticationService` (API key validation, session management)
- `PlatformRoutingService` (map platform ID to user)

**Infrastructure Layer:**
- `FileUserProfileRepository` (implements UserProfileRepository with JSON files)
- `FileBotConfigRepository` (implements BotConfigRepository with JSON files)
- `ConfigurationLoader` (loads and parses YAML/JSON configs)
- `ConfigurationWatcher` (watches files for changes)
- `EncryptionService` (encrypt/decrypt bot tokens)
- `AuditLogger` (append-only audit log)

**Adapter Layer:**

*REST Adapters:*
- `UserProfileHandler` (HTTP handlers for /api/v1/admin/profiles/*)
- `BotConfigHandler` (HTTP handlers for /api/v1/admin/bots/*)
- `ConfigHandler` (HTTP handlers for /api/v1/admin/config/*)
- `ServerControlHandler` (HTTP handlers for /api/v1/admin/status, /reload)

*Web UI Adapters:*
- `DashboardHandler` (renders dashboard page)
- `UserManagementHandler` (renders user list, forms)
- `BotManagementHandler` (renders bot list, forms)
- `ConfigUIHandler` (renders config forms)
- `LogViewerHandler` (renders activity log)

*CLI Adapters:*
- `AdminUserCommand` (CLI commands for user management)
- `AdminBotCommand` (CLI commands for bot management)
- `ServerControlCommand` (CLI commands for server control)
- `ConfigCommand` (CLI commands for config manipulation)

**For detailed architecture, see:** `architecture.md` (to be created in next phase)

---

## Scope of Changes

### New Files/Packages

**Domain Layer:**
- `internal/domain/user_profile.go` - UserProfile entity with comprehensive fields
- `internal/domain/user_profile_test.go` - Unit tests
- `internal/domain/bot_config.go` - BotConfig entity (Slack/Telegram)
- `internal/domain/bot_config_test.go` - Unit tests
- `internal/domain/server_config.go` - Enhanced ServerConfig with paths
- `internal/domain/server_config_test.go` - Unit tests
- `internal/domain/audit_log.go` - AuditLog entity
- `internal/domain/audit_log_test.go` - Unit tests

**Use Case Layer:**
- `internal/usecase/profile/service.go` - UserProfileService
- `internal/usecase/profile/service_test.go` - Unit tests
- `internal/usecase/botmgmt/service.go` - BotManagementService
- `internal/usecase/botmgmt/service_test.go` - Unit tests
- `internal/usecase/config/service.go` - ConfigurationService
- `internal/usecase/config/service_test.go` - Unit tests
- `internal/usecase/admin/service.go` - AdminService (audit, bulk ops)
- `internal/usecase/admin/service_test.go` - Unit tests
- `internal/usecase/routing/service.go` - PlatformRoutingService
- `internal/usecase/routing/service_test.go` - Unit tests

**Infrastructure Layer:**
- `internal/infrastructure/storage/file_user_profile_repository.go` - JSON file-based user storage
- `internal/infrastructure/storage/file_user_profile_repository_test.go` - Unit tests
- `internal/infrastructure/storage/file_bot_config_repository.go` - JSON file-based bot storage
- `internal/infrastructure/storage/file_bot_config_repository_test.go` - Unit tests
- `internal/infrastructure/config/loader.go` - Enhanced config loader with paths
- `internal/infrastructure/config/loader_test.go` - Unit tests
- `internal/infrastructure/config/watcher.go` - File watcher for hot reload
- `internal/infrastructure/config/watcher_test.go` - Unit tests
- `internal/infrastructure/security/encryption.go` - Encrypt/decrypt bot tokens
- `internal/infrastructure/security/encryption_test.go` - Unit tests
- `internal/infrastructure/audit/logger.go` - Audit log writer
- `internal/infrastructure/audit/logger_test.go` - Unit tests

**Adapter Layer - REST:**
- `internal/adapter/rest/profile_handler.go` - User profile CRUD endpoints
- `internal/adapter/rest/profile_handler_test.go` - Integration tests
- `internal/adapter/rest/bot_handler.go` - Bot CRUD endpoints
- `internal/adapter/rest/bot_handler_test.go` - Integration tests
- `internal/adapter/rest/config_handler.go` - Config management endpoints
- `internal/adapter/rest/config_handler_test.go` - Integration tests
- `internal/adapter/rest/server_handler.go` - Server control endpoints
- `internal/adapter/rest/server_handler_test.go` - Integration tests
- `internal/adapter/rest/middleware/auth.go` - Authentication middleware
- `internal/adapter/rest/middleware/auth_test.go` - Unit tests
- `internal/adapter/rest/middleware/rbac.go` - Role-based access control
- `internal/adapter/rest/middleware/rbac_test.go` - Unit tests

**Adapter Layer - Web UI:**
- `internal/adapter/web/dashboard_handler.go` - Dashboard page
- `internal/adapter/web/user_handler.go` - User management pages
- `internal/adapter/web/bot_handler.go` - Bot management pages
- `internal/adapter/web/config_handler.go` - Config management pages
- `internal/adapter/web/log_handler.go` - Activity log viewer
- `internal/adapter/web/templates/` - HTML templates directory
  - `base.html`, `dashboard.html`, `users.html`, `bots.html`, `config.html`, `logs.html`
- `internal/adapter/web/static/` - Static assets (CSS, JS)
  - `styles.css`, `htmx.min.js`, `alpine.min.js`

**Adapter Layer - CLI:**
- `internal/adapter/cli/admin_user_command.go` - User management commands
- `internal/adapter/cli/admin_user_command_test.go` - Unit tests
- `internal/adapter/cli/admin_bot_command.go` - Bot management commands
- `internal/adapter/cli/admin_bot_command_test.go` - Unit tests
- `internal/adapter/cli/server_command.go` - Server control commands
- `internal/adapter/cli/server_command_test.go` - Unit tests
- `internal/adapter/cli/config_command.go` - Config manipulation commands
- `internal/adapter/cli/config_command_test.go` - Unit tests

**Entry Points:**
- `cmd/nuimanbotd/main.go` - Server daemon entry point
- `cmd/nuimanbot/main.go` - CLI tool entry point (updated)

**Configuration & Data:**
- `config/config.yaml` - Enhanced configuration template
- `config/config.schema.json` - JSON schema for validation
- `data/users.json` - User registry (created at runtime)
- `data/bots.json` - Bot configurations (created at runtime)
- `data/users/` - User directories (created at runtime)

**Documentation:**
- `documentation/admin-guide.md` - Admin interface and CLI usage
- `documentation/api-reference.md` - REST API documentation
- `documentation/migration-guide.md` - Migration from old architecture
- `documentation/configuration-reference.md` - Config file reference

### Modified Files

**Configuration:**
- `internal/config/config.go` - Add PathsConfig, update GatewayConfig with enabled flags, update LLMConfig with inheritance
- `internal/config/loader.go` - Support new config structure, environment variable overrides

**Main Entry Points:**
- `cmd/nuimanbot/main.go` - Convert to CLI-only tool (remove server start logic)

**Gateway Initialization:**
- `internal/adapter/gateway/cli/cli.go` - Check enabled flag before initialization
- `internal/adapter/gateway/slack/slack.go` - Load bots from bots.json instead of config.yaml
- `internal/adapter/gateway/telegram/telegram.go` - Load bots from bots.json instead of config.yaml

**LLM Provider Management:**
- `internal/usecase/llm/provider_manager.go` - Support fallback model selection (primary/secondary/tertiary)
- `internal/infrastructure/llm/anthropic/provider.go` - Load config from provider-specific section
- `internal/infrastructure/llm/openai/provider.go` - Load config from provider-specific section
- `internal/infrastructure/llm/bedrock/provider.go` - Load config from provider-specific section
- `internal/infrastructure/llm/ollama/provider.go` - Load config from provider-specific section

**User Management:**
- `internal/domain/user.go` - Add platformIDs field (if not exists), deprecate old fields (if applicable)
- `internal/usecase/user/service.go` - Integrate with FileUserProfileRepository

**Logging:**
- `internal/infrastructure/logging/logger.go` - Use configured log path from server.paths.logs

**Documentation:**
- `README.md` - Update with new architecture, multi-component design, admin features
- `documentation/product-summary.md` - Update with admin capabilities
- `documentation/product-details.md` - Add user profile and bot management workflows
- `documentation/technical-details.md` - Update architecture diagrams and data flows

### Database Changes

**No SQL Database Changes:**
- Existing SQLite database (`nuimanbot.db`) remains for conversations and messages
- No schema migrations required
- File-based storage (JSON) for users and bots

**File-Based Storage Schema:**

**users.json:**
```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-08T12:00:00Z",
  "users": [
    {
      "userID": "uuid",
      "username": "string",
      "moniker": "string",
      "firstName": "string",
      "lastName": "string",
      "nickName": "string",
      "primaryEmail": "string",
      "backupEmail": "string",
      "mobilePhone": "string",
      "primaryLanguage": "string",
      "secondaryLanguage": "string",
      "timezone": "string",
      "primaryLocation": "string",
      "jobRole": "string",
      "userType": "string",
      "role": "string",
      "platformIDs": {
        "cli": "string",
        "slack": "string",
        "telegram": "string"
      },
      "enabled": true,
      "dataDirectory": "users/<uuid>"
    }
  ],
  "indexes": {
    "byUsername": {"username": "userID"},
    "byEmail": {"email": "userID"},
    "byPlatform": {
      "slack": {"slackID": "userID"},
      "telegram": {"telegramID": "userID"},
      "cli": {"username": "userID"}
    }
  }
}
```

**bots.json:**
```json
{
  "version": "1.0",
  "lastUpdated": "2026-02-08T12:00:00Z",
  "slackBots": [
    {
      "botID": "uuid",
      "botName": "string",
      "botType": "public|private",
      "ownerUserID": "uuid|null",
      "slackBotToken": "encrypted",
      "slackAppToken": "encrypted",
      "slackSigningSecret": "encrypted",
      "slackTeamID": "string",
      "slackBotUserID": "string",
      "enabled": true,
      "allowedUserIDs": ["uuid"]|null,
      "metadata": {}
    }
  ],
  "telegramBots": [
    {
      "botID": "uuid",
      "botName": "string",
      "botType": "public|private",
      "ownerUserID": "uuid|null",
      "telegramBotToken": "encrypted",
      "telegramBotUsername": "string",
      "telegramBotID": "string",
      "enabled": true,
      "allowedUserIDs": ["uuid"]|null,
      "allowedChatIDs": ["string"],
      "metadata": {}
    }
  ],
  "indexes": {
    "slackByName": {"name": "botID"},
    "slackByOwner": {"ownerID": ["botID"]},
    "slackByBotUserID": {"botUserID": "botID"},
    "telegramByName": {"name": "botID"},
    "telegramByOwner": {"ownerID": ["botID"]},
    "telegramByBotID": {"botID": "botID"}
  }
}
```

### Configuration Changes

**New Config Fields:**

```yaml
server:
  environment: development
  log_level: info
  debug: false
  admin_port: 8080  # NEW: Web admin interface port
  paths:  # NEW: Configurable paths
    config: "./config/"
    data: "./data/"
    logs: "./logs/"

gateways:
  cli:
    enabled: true  # NEW: Enable/disable flag
    debug_mode: false
    history_file: ".nuimanbot_history"

  slack:
    enabled: false  # NEW: Enable/disable flag
    # Bot configurations now in bots.json

  telegram:
    enabled: false  # NEW: Enable/disable flag
    # Bot configurations now in bots.json

llm:
  default_model:
    primary: anthropic-main  # CHANGED: Use provider ID instead of type/name
    secondary: anthropic-fast  # NEW: Fallback model
    tertiary: openai-gpt4  # NEW: Second fallback

  # NEW: Provider-specific default configurations
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    base_url: "https://api.anthropic.com"

  openai:
    api_key: "${OPENAI_API_KEY}"
    base_url: "https://api.openai.com/v1"
    organization: ""

  ollama:
    base_url: "http://localhost:11434"

  bedrock:
    aws_region: "us-east-1"
    aws_profile: ""
    max_retries: 3

  # ENHANCED: Provider instances with inheritance
  providers:
    - id: anthropic-main
      type: anthropic
      enabled: true  # NEW
      model_name: claude-3-5-sonnet-20241022  # NEW
      # Inherits api_key and base_url from anthropic section

    - id: anthropic-fast
      type: anthropic
      enabled: true  # NEW
      model_name: claude-3-haiku-20240307  # NEW
      # Inherits api_key and base_url from anthropic section

    - id: openai-gpt4
      type: openai
      enabled: true
      model_name: gpt-4-turbo-preview
      # Inherits api_key, base_url, organization from openai section

    - id: ollama-local
      type: ollama
      enabled: false
      model_name: llama2
      # Inherits base_url from ollama section
```

**Environment Variables:**

```bash
# Server paths
NUIMANBOT_SERVER_PATHS_CONFIG="/etc/nuimanbot/config/"
NUIMANBOT_SERVER_PATHS_DATA="/var/lib/nuimanbot/"
NUIMANBOT_SERVER_PATHS_LOGS="/var/log/nuimanbot/"

# Admin port
NUIMANBOT_SERVER_ADMIN_PORT=8080

# Gateway flags
NUIMANBOT_GATEWAYS_CLI_ENABLED=true
NUIMANBOT_GATEWAYS_SLACK_ENABLED=false
NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED=false

# Default models
NUIMANBOT_LLM_DEFAULTMODEL_PRIMARY="anthropic-main"
NUIMANBOT_LLM_DEFAULTMODEL_SECONDARY="anthropic-fast"
NUIMANBOT_LLM_DEFAULTMODEL_TERTIARY="openai-gpt4"

# Provider credentials
ANTHROPIC_API_KEY="sk-ant-..."
OPENAI_API_KEY="sk-..."
```

---

## Breaking Changes

### Breaking Change 1: Binary Rename and Architecture Split

**Impact:** All deployment scripts and systemd services

**Description:**
- Server binary renamed from `nuimanbot` to `nuimanbotd`
- CLI tool remains `nuimanbot` but no longer starts server
- Systemd services must be updated to use `nuimanbotd`
- Docker images must update ENTRYPOINT

**Migration Path:**
1. Build new binaries:
   ```bash
   go build -o bin/nuimanbotd ./cmd/nuimanbotd
   go build -o bin/nuimanbot ./cmd/nuimanbot
   ```

2. Update systemd service file (`/etc/systemd/system/nuimanbot.service`):
   ```ini
   [Service]
   ExecStart=/usr/local/bin/nuimanbotd
   # Previously: ExecStart=/usr/local/bin/nuimanbot server
   ```

3. Update Docker ENTRYPOINT:
   ```dockerfile
   ENTRYPOINT ["/usr/local/bin/nuimanbotd"]
   # Previously: ENTRYPOINT ["/usr/local/bin/nuimanbot", "server"]
   ```

4. Reload systemd and restart service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart nuimanbot
   ```

**Rollback Plan:**
- Keep old binary as `nuimanbot.v1.bak`
- Systemd service can be reverted by editing and reloading
- Docker rollback via image tag

### Breaking Change 2: Configuration File Structure

**Impact:** All existing config.yaml files

**Description:**
- `llm.default_model.primary` format changed from `type/name` to provider `id`
- `gateways` require `enabled` flag
- `server.paths` section required
- Provider configurations moved to type-specific sections with inheritance
- Bot configurations removed from config.yaml (moved to bots.json)

**Migration Path:**

1. Backup existing config:
   ```bash
   cp config.yaml config.yaml.backup
   ```

2. Run migration tool (to be created):
   ```bash
   nuimanbot config migrate --input config.yaml.backup --output config.yaml
   ```

3. Manual migration (if tool unavailable):
   ```yaml
   # OLD
   llm:
     default_model:
       primary: anthropic/claude-sonnet

   # NEW
   llm:
     default_model:
       primary: anthropic-main  # Or anthropic if using auto-generated default
   ```

4. Add required sections:
   ```yaml
   server:
     paths:
       config: "./config/"
       data: "./data/"
       logs: "./logs/"

   gateways:
     cli:
       enabled: true
   ```

5. Validate configuration:
   ```bash
   nuimanbot config validate
   ```

**Rollback Plan:**
- Keep backup config.yaml
- Restore from backup if issues arise
- Old binary can read old config format

### Breaking Change 3: Bot Configuration Storage

**Impact:** Slack and Telegram bot configurations

**Description:**
- Bot configurations no longer in config.yaml
- Bots now stored in data/bots.json
- Bot credentials encrypted at rest

**Migration Path:**

1. Extract bot configs from config.yaml
2. Run migration tool:
   ```bash
   nuimanbot admin bot migrate --from-config config.yaml.backup
   ```

3. Tool creates data/bots.json with encrypted credentials
4. Remove bot sections from config.yaml
5. Restart server with `/refresh`

**Rollback Plan:**
- Keep backup config.yaml with bot configs
- Tool can reverse migration (export bots.json → config.yaml)
- Old binary can use old config format

### Breaking Change 4: User Storage Location

**Impact:** Existing user data in SQLite

**Description:**
- User profiles moved from SQLite to file-based storage (users.json)
- Extended profile fields not backward compatible with old User entity
- Platform IDs (Slack, Telegram) stored in new format

**Migration Path:**

1. Run user migration tool:
   ```bash
   nuimanbot admin user migrate --from-db nuimanbot.db --to-file data/users.json
   ```

2. Tool extracts users from SQLite and creates:
   - data/users.json (central registry)
   - data/users/<user-id>/ (user directories)

3. SQLite database retained for conversations and messages (no changes)

4. Verify migration:
   ```bash
   nuimanbot admin user list
   ```

**Rollback Plan:**
- SQLite database unchanged (users table still present)
- Tool can reverse migration (users.json → SQLite)
- Old binary uses SQLite, new binary uses users.json

### Breaking Change 5: STDIO Command Simplification

**Impact:** Scripts that send commands to server STDIO

**Description:**
- Server STDIO accepts only `/refresh` and `/exit` commands
- All other commands removed (were admin commands, now in CLI/web/API)
- User interactions happen through gateways, not server STDIO

**Migration Path:**

1. Identify scripts sending STDIO commands to server
2. Replace with CLI commands:
   ```bash
   # OLD: echo "/reload-config" | nc localhost 5000
   # NEW: nuimanbot server reload
   ```

3. Replace with API calls:
   ```bash
   # NEW: curl -X POST http://localhost:8080/api/v1/admin/config/reload \
   #      -H "Authorization: Bearer <token>"
   ```

**Rollback Plan:**
- Old binary supports old STDIO commands
- Scripts can be updated incrementally (both old and new work during transition)

---

## Success Criteria

### Metrics

**Primary Metrics:**

- **Admin Efficiency:** Time to onboard new user reduced from 10 minutes (manual YAML editing) to 2 minutes (web form)
- **Configuration Flexibility:** 100% of configuration changes possible without server restart
- **API Coverage:** 100% of admin operations accessible via REST API
- **User Satisfaction:** Admin interface usability score > 8/10 (user survey)
- **Reliability:** Hot configuration reload success rate > 99%
- **Performance:** Configuration reload completes within 5 seconds (p95)

**Secondary Metrics:**

- **Security:** Zero secrets exposed in logs or error messages
- **Audit Coverage:** 100% of admin operations logged to audit.log
- **Documentation Quality:** Admin guide rated "helpful" by > 90% of readers
- **Migration Success:** 100% of existing deployments migrate without data loss
- **Test Coverage:** > 80% code coverage on core logic

### Acceptance Tests

**Test Scenario 1: Multi-Component Architecture**
```
Given the system is running
When I check running processes
Then I see "nuimanbotd" process (server daemon)
And I can execute "nuimanbot" command (CLI tool)
And I can access http://localhost:8080/admin/ (web interface)
And server STDIO accepts only "/refresh" and "/exit" commands
```

**Test Scenario 2: Hot Configuration Reload**
```
Given the server is running with configuration A
When I edit config.yaml to configuration B
And I send "/refresh" command via STDIO
Then server reloads configuration within 5 seconds
And active conversations continue without interruption
And new requests use configuration B
```

**Test Scenario 3: User Profile Management via Web Interface**
```
Given I am logged into web admin interface as admin
When I navigate to /admin/users
And I click "Add New User"
And I fill form with username "testuser", email "test@example.com"
And I click "Create User"
Then user "testuser" appears in user list
And data/users.json contains user entry
And data/users/<user-id>/ directory exists
And user can authenticate immediately (no restart)
```

**Test Scenario 4: Bot Management via API**
```
Given I have admin API key
When I POST /api/v1/admin/bots/slack with bot credentials
Then response is 201 Created with bot ID
And data/bots.json contains new bot entry
And bot tokens are encrypted in file
And gateway automatically connects bot to Slack
And bot appears as "connected" in web interface
```

**Test Scenario 5: Partial Profile Update**
```
Given user with ID "user-123" exists
When I PUT /api/v1/admin/profiles/user-123 with {"timezone": "America/New_York"}
Then response contains updated_fields: ["timezone"]
And timezone is changed to "America/New_York"
And all other fields remain unchanged
And updatedAt timestamp is current
```

**Test Scenario 6: Agent Personalization**
```
Given user "alice" has agentPreferences.communicationStyle = "technical"
And user "alice" has agentPreferences.verbosity = "concise"
When user "alice" sends message via Slack
Then agent loads user profile from users.json
And agent responds in technical, concise style
And response format matches user's preferences
```

**Test Scenario 7: Path Configuration for Containers**
```
Given Docker container with environment variables:
  NUIMANBOT_SERVER_PATHS_CONFIG=/config
  NUIMANBOT_SERVER_PATHS_DATA=/data
  NUIMANBOT_SERVER_PATHS_LOGS=/logs
When server starts
Then configuration loaded from /config/config.yaml
And users.json is at /data/users.json
And logs written to /logs/app.log
And separate volumes can be mounted for each path
```

**Test Scenario 8: Audit Logging**
```
Given I am admin user "admin-001"
When I update user "user-123" via API
Then audit.log contains entry with:
  - timestamp (current)
  - adminUserID: "admin-001"
  - operation: "update"
  - resource: "user"
  - resourceID: "user-123"
  - changes: {"timezone": "America/New_York"}
```

### Quality Gates

- [x] All unit tests passing (>90% coverage on domain/usecase layers)
- [x] Integration tests passing (REST API, web interface)
- [x] E2E tests passing (full workflows: create user → link Slack → send message)
- [x] Performance benchmarks met (hot reload < 5s, API < 500ms)
- [x] Security review complete (no secrets in logs, encryption validated)
- [x] Documentation complete (admin guide, API reference, migration guide)
- [x] Code review approved (clean architecture boundaries respected)
- [x] Configuration validation (schema enforced, invalid configs rejected)
- [x] Migration testing (old configs successfully migrated)
- [x] Rollback testing (can revert to old architecture)

---

## Risks and Mitigation

### Technical Risks

**Risk 1: File Locking Conflicts in users.json**
- **Likelihood:** Medium
- **Impact:** High (data corruption)
- **Mitigation:**
  - Implement robust file locking (flock) on all read/write operations
  - Use atomic writes (temp file + rename)
  - Retry logic with exponential backoff
  - Add integration tests for concurrent access
- **Contingency:**
  - If corruption detected, restore from backup (users.json.backup created on each write)
  - Alert admin via log and web interface

**Risk 2: Hot Reload Causes Service Disruption**
- **Likelihood:** Medium
- **Impact:** High (active conversations interrupted)
- **Mitigation:**
  - Validate configuration before applying (dry-run mode)
  - Use graceful reload (new config in separate goroutine)
  - Keep old config in memory if new config fails
  - Test reload under load (integration tests)
- **Contingency:**
  - Rollback to previous config automatically on error
  - Log detailed error message for debugging
  - Notify admin via web interface

**Risk 3: Bot Credential Encryption Key Loss**
- **Likelihood:** Low
- **Impact:** Critical (all bots inaccessible)
- **Mitigation:**
  - Document encryption key management in admin guide
  - Support key rotation (re-encrypt with new key)
  - Store key in environment variable (NUIMANBOT_ENCRYPTION_KEY)
  - Backup key separately from bots.json
- **Contingency:**
  - If key lost, require manual re-entry of bot credentials
  - Provide CLI tool to re-encrypt bots.json with new key

**Risk 4: API Rate Limiting Insufficient**
- **Likelihood:** Medium
- **Impact:** Medium (brute force attacks, DoS)
- **Mitigation:**
  - Implement rate limiting on authentication endpoints (10 req/min per IP)
  - Implement rate limiting on all API endpoints (100 req/min per API key)
  - Add CAPTCHA on repeated failed logins (web interface)
  - Monitor rate limit violations and alert admin
- **Contingency:**
  - Temporarily block offending IPs
  - Increase rate limits if legitimate traffic affected

**Risk 5: Large users.json Performance Degradation**
- **Likelihood:** Low (not expected at <1000 users)
- **Impact:** Medium (slow lookups)
- **Mitigation:**
  - Pre-build indexes in users.json (byUsername, byEmail, byPlatform)
  - Cache users.json in memory with TTL (invalidate on write)
  - Benchmark file load time (must be <100ms for 1000 users)
  - Document migration to database if >1000 users needed
- **Contingency:**
  - If performance unacceptable, provide migration tool to SQLite/PostgreSQL
  - Implement pagination on user list endpoints

### Operational Risks

**Risk 1: Migration Failures During Upgrade**
- **Likelihood:** Medium
- **Impact:** High (service down, data loss)
- **Mitigation:**
  - Provide automated migration tool (nuimanbot config migrate, nuimanbot admin user migrate)
  - Require backup before migration (tool checks and prompts)
  - Dry-run mode for migration (validate without applying)
  - Comprehensive migration testing on staging environments
- **Contingency:**
  - Rollback to old binary and old config/database
  - Provide step-by-step manual migration guide
  - Support team available during upgrade window

**Risk 2: Admin Forgetting Encryption Key**
- **Likelihood:** Low
- **Impact:** Critical (bots inaccessible)
- **Mitigation:**
  - Document key management prominently in admin guide
  - Warn admin to backup key during initial setup
  - Provide key validation command (nuimanbot config validate-encryption-key)
  - Store key in environment variable (less likely to lose)
- **Contingency:**
  - Manual re-entry of bot credentials via web interface
  - Tool to test decryption before critical operations

**Risk 3: Web Interface Security Vulnerabilities**
- **Likelihood:** Medium
- **Impact:** High (unauthorized access, data breach)
- **Mitigation:**
  - CSRF protection on all forms (tokens)
  - Input validation and sanitization
  - Session timeout (24 hours)
  - HTTPS enforcement in production (documented)
  - Security review of web handlers and middleware
- **Contingency:**
  - Disable web interface via config flag (gateways.web.enabled=false)
  - Revoke compromised API keys via CLI
  - Rotate session secrets

**Risk 4: Documentation Gaps Lead to Misconfiguration**
- **Likelihood:** Medium
- **Impact:** Medium (service misconfigured, support burden)
- **Mitigation:**
  - Comprehensive admin guide with examples
  - Configuration templates in config/ directory
  - Validation tool catches common mistakes (nuimanbot config validate)
  - Error messages include actionable guidance
- **Contingency:**
  - Support team provides quick-start guides
  - Community examples and tutorials
  - FAQ document for common issues

### Dependencies

**External Dependencies:**

- **Slack Socket Mode API** - Required for Slack bot connections
  - Status: Stable, widely used
  - Risk: API changes or deprecation
  - Mitigation: Monitor Slack API announcements, version lock SDK

- **Telegram Bot API (Long Polling)** - Required for Telegram bot connections
  - Status: Stable, official API
  - Risk: API changes or rate limiting
  - Mitigation: Monitor Telegram Bot API updates, implement retry logic

- **YAML Parser (gopkg.in/yaml.v3)** - Required for config.yaml parsing
  - Status: Mature library
  - Risk: YAML spec changes, parsing bugs
  - Mitigation: Version lock, comprehensive config validation

- **HTMX (htmx.org)** - Required for dynamic web UI updates
  - Status: Stable, widely adopted
  - Risk: CDN unavailability, breaking changes
  - Mitigation: Vendor HTMX locally (embed in binary), version lock

**Internal Dependencies:**

- **LLM Provider Infrastructure** - Required for agent functionality
  - Status: Already implemented
  - Risk: Changes to provider interfaces
  - Mitigation: Provider abstraction layer isolates changes

- **Gateway Infrastructure** - Required for multi-platform support
  - Status: Already implemented
  - Risk: Changes to gateway interfaces for bot loading
  - Mitigation: Gateway abstraction layer, comprehensive tests

- **Vault/Encryption Infrastructure** - Required for bot token encryption
  - Status: May need implementation
  - Risk: Encryption scheme changes
  - Mitigation: Versioned encryption format, migration path

---

## Timeline

### Estimated Duration

**Total: 120-160 hours (3-4 weeks of full-time work, or 6-8 weeks at 50% allocation)**

### Phases

**Phase 1: Core Architecture & Configuration (30-40 hours)**
- Restructure configuration (paths, enable flags, provider inheritance)
- Implement hot configuration reload
- Create file-based storage infrastructure (FileUserProfileRepository, FileBotConfigRepository)
- Implement encryption service for bot credentials
- Create server daemon entry point (cmd/nuimanbotd)
- Update CLI tool entry point (cmd/nuimanbot)
- Migration tools (config, users, bots)

**Milestones:**
- [ ] Configuration restructured and validated
- [ ] Hot reload working (integration test passes)
- [ ] File storage operational (unit tests pass)
- [ ] Encryption service validated (unit tests pass)
- [ ] Binaries build successfully (nuimanbotd, nuimanbot)
- [ ] Migration tools functional (tested on sample data)

**Phase 2: User Profile Management (25-35 hours)**
- Implement UserProfile domain entity
- Implement UserProfileService use case
- Create users.json schema and file structure
- Implement user directory management (profile.json, preferences.json, etc.)
- Add multi-platform integration (platformIDs)
- Implement agent personalization logic
- Create admin user commands (CLI)
- Add audit logging

**Milestones:**
- [ ] UserProfile entity complete (unit tests pass)
- [ ] UserProfileService operational (unit tests pass)
- [ ] users.json reads/writes working (integration tests pass)
- [ ] User directories created automatically
- [ ] Platform routing functional (Slack/Telegram ID → User)
- [ ] Agent loads and applies preferences
- [ ] CLI commands functional (nuimanbot admin user *)

**Phase 3: Bot Management (20-30 hours)**
- Implement BotConfig domain entity
- Implement BotManagementService use case
- Create bots.json schema and file structure
- Update gateways to load bots from bots.json
- Implement public vs private bot logic
- Add dynamic bot enable/disable
- Create admin bot commands (CLI)
- Add bot connection monitoring

**Milestones:**
- [ ] BotConfig entity complete (unit tests pass)
- [ ] BotManagementService operational (unit tests pass)
- [ ] bots.json reads/writes working (integration tests pass)
- [ ] Gateways load bots from bots.json
- [ ] Public/private bot access control working
- [ ] Dynamic enable/disable functional (no restart)
- [ ] CLI commands functional (nuimanbot admin bot *)

**Phase 4: REST API (20-25 hours)**
- Implement REST handlers (profile, bot, config, server)
- Add authentication middleware (Bearer token)
- Add RBAC middleware (admin/user roles)
- Implement partial update support (PUT)
- Add pagination on list endpoints
- Create API documentation (OpenAPI spec)
- Add integration tests for all endpoints

**Milestones:**
- [ ] All CRUD endpoints functional
- [ ] Authentication working (API key validation)
- [ ] RBAC enforced (admin/user access tested)
- [ ] Partial updates working (only specified fields changed)
- [ ] Pagination working on list endpoints
- [ ] API documentation complete (OpenAPI spec)
- [ ] Integration tests passing (>90% coverage)

**Phase 5: Web Admin Interface (30-40 hours)**
- Design HTML templates (base, dashboard, users, bots, config, logs)
- Implement web handlers (dashboard, user, bot, config, log)
- Add session-based authentication
- Implement CSRF protection
- Add HTMX for dynamic updates
- Style with Tailwind CSS
- Add responsive design (mobile-friendly)
- Integration testing (E2E tests)

**Milestones:**
- [ ] All pages rendering correctly
- [ ] Forms functional (create/edit/delete)
- [ ] Authentication working (session-based)
- [ ] CSRF protection validated
- [ ] HTMX updates working (no page refresh)
- [ ] Responsive design validated (mobile/tablet)
- [ ] E2E tests passing (user workflows)

**Phase 6: Documentation & Migration (15-20 hours)**
- Write admin guide (web interface, CLI commands)
- Write API reference (endpoint documentation)
- Write migration guide (old → new architecture)
- Write configuration reference (all fields documented)
- Update product docs (README, product-summary, technical-details)
- Create video tutorials (optional)
- Review and polish all documentation

**Milestones:**
- [ ] Admin guide complete and reviewed
- [ ] API reference complete (OpenAPI + examples)
- [ ] Migration guide tested on sample deployment
- [ ] Configuration reference comprehensive
- [ ] Product docs updated and consistent
- [ ] Documentation peer-reviewed

### Critical Path

**Blocking Dependencies:**

1. **Phase 1 blocks all others:** Configuration infrastructure must be complete before user/bot management can be built
2. **Phase 2 blocks Phase 4 & 5:** User profile system required for authentication in REST API and web interface
3. **Phase 3 blocks Phase 4 & 5:** Bot management required for bot admin features in API and web
4. **Phase 4 & 5 can proceed in parallel:** REST API and web interface can be developed simultaneously
5. **Phase 6 requires all others complete:** Documentation requires working system to document

**Parallel Work Opportunities:**

- Phase 2 and Phase 3 can partially overlap (independent features)
- Phase 4 and Phase 5 can fully overlap (different team members)
- Documentation can start during implementation (draft as you go)

**Time-Critical Items:**

- Configuration restructuring (breaks existing deployments, must be stable)
- Migration tools (required for smooth upgrade path)
- Encryption service (bot credentials must be secure from day 1)
- Hot reload (core value proposition, must be reliable)

**For detailed task breakdown, see:** `tasks.md` (to be created in next phase)

---

## References

### Internal Documents

- **PRD:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/improved-admin-features-prd.md` (source document)
- **AGENTS.md:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/AGENTS.md` (development guidelines)
- **Product Docs:** `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/documentation/` (existing product documentation)

### External Resources

- **Slack Socket Mode API:** https://api.slack.com/apis/connections/socket
- **Telegram Bot API:** https://core.telegram.org/bots/api
- **HTMX Documentation:** https://htmx.org/docs/
- **Tailwind CSS:** https://tailwindcss.com/docs
- **Clean Architecture (Go):** https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html

### Related Features

- **Phase 3: Agent Skills** (subagents, shared skills) - User profiles enable skill personalization
- **LLM Provider Infrastructure** (Anthropic, OpenAI, Bedrock, Ollama) - Provider inheritance builds on this
- **Gateway Infrastructure** (CLI, Slack, Telegram) - Bot management extends gateway capabilities
- **Vault System** (secure credential storage) - Encryption service extends vault pattern

---

**Next Steps:**

1. **Review and approve this specification** (stakeholder sign-off)
2. **Create research.md** (gather technical details, API documentation, examples)
3. **Create data-dictionary.md** (define all data structures, types, schemas)
4. **Create architecture.md** (detailed system design, component interactions)
5. **Create plan.md** (implementation approach, file-by-file breakdown)
6. **Create tasks.md** (breakdown into concrete, testable tasks with dependencies)
7. **Begin Phase 1 implementation** (configuration restructuring)

---

**Approval:**

- [ ] Product Owner: _______________________  Date: __________
- [ ] Tech Lead: ___________________________  Date: __________
- [ ] Security Review: _____________________  Date: __________
- [ ] Documentation Lead: __________________  Date: __________
