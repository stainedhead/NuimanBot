# Improved Admin Features - System Architecture

**Created:** 2026-02-08
**Version:** 1.0
**Status:** Complete
**Last Updated:** 2026-02-08

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [System Context](#system-context)
3. [Component Architecture](#component-architecture)
4. [Layer Responsibilities](#layer-responsibilities)
5. [Data Flow](#data-flow)
6. [Sequence Diagrams](#sequence-diagrams)
7. [Integration Points](#integration-points)
8. [Architectural Decisions](#architectural-decisions)
9. [Trade-offs](#trade-offs)
10. [Performance Considerations](#performance-considerations)
11. [Security Architecture](#security-architecture)
12. [Scalability](#scalability)

---

## Architecture Overview

**High-Level Summary:**

This feature transforms NuimanBot from a monolithic CLI application into a multi-component distributed system with three primary components: a daemon server process (nuimanbotd), a standalone CLI management tool (nuimanbot), and an embedded web administration interface. The architecture follows Clean Architecture principles with strict layer separation and dependency inversion, enabling hot configuration reloading, file-based user profile management, database-driven bot configurations, and comprehensive REST API access for all administrative operations.

**Architectural Style:** Clean Architecture + File-Based Storage + Event-Driven Configuration Management

**Key Principles:**
- Dependency Inversion: Outer layers depend on inner layers, domain defines interfaces
- Single Responsibility: Each component has one clear purpose (separation of runtime vs administration)
- Open/Closed: Configuration is open for extension via enable/disable flags and provider inheritance
- Interface Segregation: Small, focused interfaces for repositories and services
- Immutability: Configuration objects are immutable after validation
- Fail-Safe: Invalid configurations rejected, server continues with last known good state

**Multi-Component Architecture Diagram:**
```
┌──────────────────────────────────────────────────────────────────────┐
│                       External Actors                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │  Admin   │  │  Users   │  │  Slack   │  │ Telegram │            │
│  │ (Browser)│  │  (CLI)   │  │ Platform │  │ Platform │            │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  └─────┬────┘            │
└────────┼─────────────┼─────────────┼─────────────┼──────────────────┘
         │             │             │             │
         │ HTTP        │ stdin/out   │ Webhook     │ Webhook
         │ :8080       │             │             │
         ▼             ▼             ▼             ▼
┌─────────────────────────────────────────────────────────────────────┐
│              Core Server Daemon (nuimanbotd)                         │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    Web Admin Interface                        │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │  Dashboard   │  │  LLM Config  │  │ User Mgmt    │       │  │
│  │  │  Handler     │  │  Handler     │  │ Handler      │       │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │  │
│  └─────────┼──────────────────┼──────────────────┼──────────────┘  │
│            │                  │                  │                  │
│  ┌─────────▼──────────────────▼──────────────────▼──────────────┐  │
│  │                    REST API Layer                             │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │ Config API   │  │  User API    │  │  Bot API     │       │  │
│  │  │ /api/config  │  │ /api/users   │  │ /api/bots    │       │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │  │
│  └─────────┼──────────────────┼──────────────────┼──────────────┘  │
│            │                  │                  │                  │
│  ┌─────────▼──────────────────▼──────────────────▼──────────────┐  │
│  │              Configuration Manager (Hot Reload)               │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │ File Watcher │  │  Validator   │  │  Notifier    │       │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘       │  │
│  └───────────────────────────┬───────────────────────────────────┘  │
│                              │                                      │
│  ┌───────────────────────────▼───────────────────────────────────┐  │
│  │                   Gateway Manager                              │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │ CLI Gateway  │  │Slack Gateway │  │Telegram GW   │       │  │
│  │  │  (REPL)      │  │  (Bots)      │  │  (Bots)      │       │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘       │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      LLM Engine                                │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │  │
│  │  │  Anthropic   │  │    OpenAI    │  │   Bedrock    │       │  │
│  │  │  Provider    │  │   Provider   │  │   Provider   │       │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘       │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  STDIO Interface: /refresh, /exit only                              │
└──────────────────────────┬───────────────────────────────────────────┘
                           │
                           │ File I/O
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         File System                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │  config/         │  │  data/           │  │  logs/           │  │
│  │  - config.yaml   │  │  - users.json    │  │  - server.log    │  │
│  │  - .env          │  │  - bots.json     │  │  - audit.log     │  │
│  │                  │  │  - users/<id>/   │  │                  │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
         ▲
         │ API Calls
         │
┌────────┴─────────────────────────────────────────────────────────────┐
│                  CLI Tool (nuimanbot)                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  config cmd  │  │  admin cmd   │  │  server cmd  │              │
│  │  - set       │  │  - user      │  │  - reload    │              │
│  │  - get       │  │  - bot       │  │  - status    │              │
│  │  - validate  │  │  - audit     │  │  - health    │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
└──────────────────────────────────────────────────────────────────────┘
```

---

## System Context

**External Systems:**
```
┌────────────────────────────────────────────────────────────────────┐
│                        External Actors                              │
│                                                                     │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐ │
│  │ Admin   │  │ User    │  │ Slack   │  │Telegram │  │   LLM   │ │
│  │ Browser │  │ CLI     │  │Platform │  │Platform │  │Provider │ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘ │
└───────┼────────────┼────────────┼────────────┼────────────┼───────┘
        │            │            │            │            │
        │HTTP :8080  │stdin/out   │Webhook     │Webhook     │HTTPS API
        │            │            │            │            │
        ▼            ▼            ▼            ▼            ▼
┌────────────────────────────────────────────────────────────────────┐
│                      NuimanBot System                               │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                 Core Server (nuimanbotd)                      │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐            │  │
│  │  │ Web Admin  │  │  Gateways  │  │ LLM Engine │            │  │
│  │  │ Interface  │  │  (3 types) │  │(4 providers)│           │  │
│  │  └────────────┘  └────────────┘  └────────────┘            │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │              CLI Tool (nuimanbot)                             │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐            │  │
│  │  │Config Mgmt │  │ User Mgmt  │  │Server Mgmt │            │  │
│  │  └────────────┘  └────────────┘  └────────────┘            │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                     │
└─────────────┬───────────────────────────┬───────────────────────────┘
              │                           │
              ▼                           ▼
┌─────────────────────────┐   ┌─────────────────────────┐
│     File System         │   │   SQLite Database       │
│  - config.yaml          │   │  - conversations.db     │
│  - users.json           │   │  - messages             │
│  - bots.json            │   │  - history              │
│  - user directories     │   │                         │
└─────────────────────────┘   └─────────────────────────┘
```

**System Boundaries:**

**Inputs:**
- Admin web requests (HTTP, port 8080)
- CLI commands (nuimanbot tool, API calls to server)
- User messages (CLI REPL, Slack, Telegram)
- Configuration files (YAML, JSON)
- Environment variables (config overrides)
- STDIO commands (/refresh, /exit)

**Outputs:**
- HTTP responses (web UI, REST API)
- LLM-generated responses (to users via gateways)
- Log files (server logs, audit logs)
- Updated configuration files
- Metrics and health status

**External Dependencies:**
- LLM Provider APIs (Anthropic, OpenAI, AWS Bedrock, Ollama)
- Slack API (for Slack gateway)
- Telegram Bot API (for Telegram gateway)
- File system (config, data, logs storage)
- SQLite database (conversation history)
- Operating system (for daemon process management)

**Integration Points:**

| System | Type | Protocol | Purpose |
|--------|------|----------|---------|
| Web Browser | UI Client | HTTP/HTTPS | Admin interface access |
| CLI Tool | Management Client | REST API/File System | Configuration and user management |
| Slack Platform | Gateway | Webhook/HTTP | Bot message routing |
| Telegram Platform | Gateway | Webhook/HTTP | Bot message routing |
| Anthropic API | LLM Provider | HTTPS/JSON | Claude model access |
| OpenAI API | LLM Provider | HTTPS/JSON | GPT model access |
| AWS Bedrock | LLM Provider | AWS SDK | Bedrock model access |
| Ollama | LLM Provider | HTTP/JSON | Local model access |
| File System | Storage | Local I/O | Configuration and user data persistence |
| SQLite | Database | SQL | Conversation history storage |

---

## Component Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                   Improved Admin Features                            │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │            1. Configuration Manager                           │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ FileLoader │  │ Validator  │  │  Notifier  │             │  │
│  │  │- LoadYAML()│  │-Validate() │  │-Notify()   │             │  │
│  │  │- LoadJSON()│  │-Sanitize() │  │-Subscribe()│             │  │
│  │  │- Watch()   │  │-Rollback() │  │            │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  └───────────────────────┬──────────────────────────────────────┘  │
│                          │ uses                                     │
│  ┌───────────────────────▼──────────────────────────────────────┐  │
│  │            2. User Profile System                             │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │UserProfile │  │ProfileRepo │  │AgentPrefs  │             │  │
│  │  │ Service    │  │(File-based)│  │  Engine    │             │  │
│  │  │-Create()   │  │-Save()     │  │-Apply()    │             │  │
│  │  │-Update()   │  │-Load()     │  │-Customize()│             │  │
│  │  │-Delete()   │  │-Index()    │  │            │             │  │
│  │  │-Search()   │  │-Query()    │  │            │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  └───────────────────────┬──────────────────────────────────────┘  │
│                          │ uses                                     │
│  ┌───────────────────────▼──────────────────────────────────────┐  │
│  │            3. Bot Management System                           │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ BotConfig  │  │  BotRepo   │  │  Gateway   │             │  │
│  │  │  Service   │  │(JSON-based)│  │  Adapter   │             │  │
│  │  │-Create()   │  │-Save()     │  │-Connect()  │             │  │
│  │  │-Enable()   │  │-Load()     │  │-Disconnect()│            │  │
│  │  │-Disable()  │  │-List()     │  │-Route()    │             │  │
│  │  │-Update()   │  │            │  │            │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  └───────────────────────┬──────────────────────────────────────┘  │
│                          │ uses                                     │
│  ┌───────────────────────▼──────────────────────────────────────┐  │
│  │            4. REST API Layer                                  │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ Config API │  │  User API  │  │  Bot API   │             │  │
│  │  │ Handlers   │  │  Handlers  │  │  Handlers  │             │  │
│  │  │-Reload()   │  │-CRUD ops   │  │-CRUD ops   │             │  │
│  │  │-Validate() │  │-PartialUpd │  │-Enable()   │             │  │
│  │  │-Export()   │  │-Search()   │  │-Disable()  │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  │                                                               │  │
│  │  ┌────────────┐  ┌────────────┐                              │  │
│  │  │    Auth    │  │   Audit    │                              │  │
│  │  │ Middleware │  │   Logger   │                              │  │
│  │  │-Verify()   │  │-Log()      │                              │  │
│  │  │-RBAC()     │  │-Query()    │                              │  │
│  │  └────────────┘  └────────────┘                              │  │
│  └───────────────────────┬──────────────────────────────────────┘  │
│                          │ uses                                     │
│  ┌───────────────────────▼──────────────────────────────────────┐  │
│  │            5. Web Admin Interface                             │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ HTTP Server│  │  Session   │  │  Template  │             │  │
│  │  │            │  │  Manager   │  │   Engine   │             │  │
│  │  │-Route()    │  │-Create()   │  │-Render()   │             │  │
│  │  │-Serve()    │  │-Validate() │  │-Cache()    │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  │                                                               │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │ Dashboard  │  │LLM Config  │  │ User Mgmt  │             │  │
│  │  │  Handler   │  │  Handler   │  │  Handler   │             │  │
│  │  │            │  │            │  │            │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │            6. CLI Tool (Separate Binary)                      │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐             │  │
│  │  │Config Cmds │  │ Admin Cmds │  │Server Cmds │             │  │
│  │  │-set        │  │-user       │  │-reload     │             │  │
│  │  │-get        │  │-bot        │  │-status     │             │  │
│  │  │-validate   │  │-audit      │  │-health     │             │  │
│  │  └────────────┘  └────────────┘  └────────────┘             │  │
│  └──────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

### Component Descriptions

#### Component 1: Configuration Manager

**Responsibility:**
- Load configuration from files (config.yaml, users.json, bots.json)
- Validate configuration integrity and business rules
- Support hot reload without server restart
- Notify dependent services of configuration changes
- Rollback to last known good configuration on failure
- Provide thread-safe access to current configuration

**Dependencies:**
- File system (for reading configuration files)
- Domain validation interfaces
- Service notification interfaces

**Provides:**
- ConfigLoader interface (for loading from files)
- ConfigValidator interface (for validation)
- ConfigNotifier interface (for change notifications)
- CurrentConfig accessor (thread-safe read-only access)

**Lifecycle:**
- Created during server initialization
- Lifespan: Singleton (single instance per server process)
- Persists for entire server lifetime

**Concurrency:**
- Thread-safe: Yes
- Synchronization: RWMutex for configuration reads, exclusive lock for updates
- Reload operations are serialized to prevent race conditions
- Notification dispatch is concurrent via goroutines

**Key Patterns:**
- Observer pattern (for change notifications)
- Strategy pattern (for different file loaders)
- Command pattern (for rollback capability)

---

#### Component 2: User Profile System

**Responsibility:**
- Manage comprehensive user profiles (identity, preferences, organizational context)
- Store profiles in users.json central registry
- Maintain user-specific directories (<data-dir>/users/<user-id>/)
- Support multi-platform integration (Slack ID, Telegram ID mapping)
- Provide agent personalization preferences
- Enable fast lookup by username, email, or platform ID

**Dependencies:**
- File storage infrastructure (JSON serialization)
- Domain UserProfile entity
- Encryption service (for sensitive data)
- Validation service

**Provides:**
- UserProfileRepository interface
- UserProfileService (use case orchestration)
- PlatformIDResolver (for cross-platform user lookup)
- AgentPreferenceEngine (for behavior customization)

**Lifecycle:**
- Created during server initialization
- Lifespan: Singleton
- Lazy-loads user profiles on first access
- Caches frequently accessed profiles

**Concurrency:**
- Thread-safe: Yes
- Synchronization: Per-user locks for profile updates, shared lock for reads
- Index updates are atomic
- File writes use temp file + atomic rename pattern

**Data Storage:**
```
data/
├── users.json              # Central registry with indexes
└── users/
    ├── <user-id-1>/
    │   ├── profile.json    # Detailed profile
    │   ├── preferences.json
    │   └── history.json
    └── <user-id-2>/
        └── ...
```

---

#### Component 3: Bot Management System

**Responsibility:**
- Manage bot configurations for Slack and Telegram
- Store bot credentials in bots.json
- Support public (shared) and private (user-specific) bot types
- Enable/disable bots without gateway restart
- Manage bot-user relationships and permissions
- Notify gateways of bot configuration changes

**Dependencies:**
- File storage infrastructure
- Domain BotConfig entity
- Encryption service (for bot credentials)
- Gateway notification interfaces

**Provides:**
- BotConfigRepository interface
- BotManagementService (use case orchestration)
- GatewayAdapter (for gateway integration)

**Lifecycle:**
- Created during server initialization
- Lifespan: Singleton
- Monitors bots.json for external changes

**Concurrency:**
- Thread-safe: Yes
- Synchronization: RWMutex for bot list, per-bot locks for updates
- Gateway notifications are asynchronous
- Bot credential encryption/decryption is serialized

**Data Storage:**
```json
// bots.json
{
  "bots": [
    {
      "id": "bot-uuid",
      "type": "slack",
      "name": "Team Assistant",
      "botType": "public",
      "enabled": true,
      "credentials": {
        "botToken": "encrypted-value",
        "appToken": "encrypted-value"
      },
      "allowedUsers": ["user-id-1", "user-id-2"]
    }
  ],
  "indexes": {
    "byType": {
      "slack": ["bot-uuid-1", "bot-uuid-2"],
      "telegram": ["bot-uuid-3"]
    }
  }
}
```

---

#### Component 4: REST API Layer

**Responsibility:**
- Expose HTTP endpoints for all administrative operations
- Implement CRUD operations for users, profiles, bots, configuration
- Support partial updates (PUT updates only specified fields)
- Enforce role-based access control (RBAC)
- Log all administrative actions to audit log
- Provide OpenAPI/Swagger documentation

**Dependencies:**
- Configuration Manager
- User Profile System
- Bot Management System
- Authentication middleware
- Audit logging service

**Provides:**
- HTTP handlers for all API endpoints
- Request/response validation
- Error handling and standardized responses
- API versioning support

**Lifecycle:**
- Created during server initialization
- Lifespan: Singleton
- Shares HTTP server with Web Admin Interface

**Concurrency:**
- Thread-safe: Yes
- Each request handled in separate goroutine
- Services called from handlers are thread-safe
- Database transactions for multi-operation requests

**API Structure:**
```
/api/v1/
├── config/
│   ├── GET     /         # Get current config
│   ├── POST    /reload   # Trigger reload
│   ├── PUT     /         # Update config (partial)
│   └── POST    /validate # Validate without applying
├── users/
│   ├── GET     /         # List users
│   ├── POST    /         # Create user
│   ├── GET     /:id      # Get user
│   ├── PUT     /:id      # Update user (partial)
│   ├── DELETE  /:id      # Delete user
│   └── GET     /search   # Search users
├── bots/
│   ├── GET     /         # List bots
│   ├── POST    /         # Create bot
│   ├── GET     /:id      # Get bot
│   ├── PUT     /:id      # Update bot
│   ├── DELETE  /:id      # Delete bot
│   ├── POST    /:id/enable   # Enable bot
│   └── POST    /:id/disable  # Disable bot
└── audit/
    ├── GET     /         # List audit entries
    └── GET     /search   # Search audit log
```

---

#### Component 5: Web Admin Interface

**Responsibility:**
- Provide browser-based UI for all administrative operations
- Render server-side HTML templates with HTMX for dynamic updates
- Manage admin user sessions with secure cookies
- Implement CSRF protection on all forms
- Display real-time server status and metrics
- Serve static assets (CSS, JavaScript, images)

**Dependencies:**
- REST API Layer (for data operations)
- Session management service
- Template rendering engine
- Authentication middleware

**Provides:**
- HTTP handlers for all UI pages
- Session cookie management
- CSRF token generation and validation
- Static file serving

**Lifecycle:**
- Created during server initialization
- Lifespan: Singleton
- Shares HTTP server on port 8080

**Concurrency:**
- Thread-safe: Yes
- Each request handled in separate goroutine
- Sessions stored in thread-safe session store
- Template rendering is stateless

**Page Structure:**
```
/admin/
├── /                      # Dashboard (login required)
├── /login                 # Login page
├── /logout                # Logout handler
├── /llm                   # LLM configuration
├── /server                # Server configuration
├── /users                 # User management
├── /users/create          # Create user form
├── /users/:id/edit        # Edit user form
├── /bots                  # Bot management
├── /bots/create           # Create bot form
├── /bots/:id/edit         # Edit bot form
└── /audit                 # Audit log viewer
```

**Security:**
- Session-based authentication (not API key)
- Admin role required for access
- CSRF protection on all POST/PUT/DELETE requests
- Secure cookies (HttpOnly, Secure, SameSite=Strict)
- Content Security Policy headers
- Rate limiting on authentication endpoints

---

#### Component 6: CLI Tool (nuimanbot)

**Responsibility:**
- Provide command-line interface for configuration and server management
- Communicate with server via REST API or direct file editing
- Validate configuration files before applying
- Trigger server configuration reload
- Display server status and health metrics
- Support scripting and automation

**Dependencies:**
- REST API client (for server communication)
- File system (for direct configuration editing)
- YAML/JSON parsers
- Configuration validation library

**Provides:**
- CLI command handlers
- Configuration file editor
- Server API client
- Output formatting (table, JSON, YAML)

**Lifecycle:**
- Created on each CLI invocation
- Lifespan: Single command execution
- No persistent state

**Concurrency:**
- Thread-safe: N/A (single-threaded CLI tool)
- Each invocation is independent

**Command Structure:**
```bash
nuimanbot
├── config
│   ├── get <key>                # Get config value
│   ├── set <key> <value>        # Set config value
│   ├── validate                 # Validate config file
│   └── export                   # Export config to stdout
├── admin
│   ├── user
│   │   ├── create               # Create user
│   │   ├── update <id>          # Update user
│   │   ├── delete <id>          # Delete user
│   │   ├── list                 # List users
│   │   └── search <query>       # Search users
│   ├── bot
│   │   ├── create               # Create bot
│   │   ├── update <id>          # Update bot
│   │   ├── delete <id>          # Delete bot
│   │   ├── enable <id>          # Enable bot
│   │   ├── disable <id>         # Disable bot
│   │   └── list                 # List bots
│   └── audit
│       ├── list                 # List audit entries
│       └── search <query>       # Search audit log
└── server
    ├── reload                   # Trigger config reload
    ├── status                   # Show server status
    └── health                   # Health check
```

---

## Layer Responsibilities

### Domain Layer

**Location:** `internal/domain/`

**Responsibility:**
- Define core business entities (UserProfile, BotConfig, ServerConfig, AuditEntry)
- Specify business rules and validation logic
- Define repository and service interfaces (no implementations)
- Declare domain errors and constants
- Pure domain logic with no external dependencies

**Contains:**

**Entities:**
- `UserProfile` - Comprehensive user identity and preferences
- `BotConfig` - Bot configuration and credentials
- `ServerConfig` - Server-wide configuration
- `AuditEntry` - Audit log record
- `AgentPreferences` - Agent behavior customization
- `PlatformIDs` - Multi-platform user identifiers

**Interfaces:**
- `UserProfileRepository` - User profile persistence
- `BotConfigRepository` - Bot configuration persistence
- `ConfigLoader` - Configuration loading
- `ConfigValidator` - Configuration validation
- `EncryptionService` - Credential encryption
- `AuditLogger` - Audit logging

**Value Objects:**
- `PlatformID` - Platform-specific user identifier
- `Email` - Email address with validation
- `Timezone` - IANA timezone identifier
- `LanguageCode` - ISO 639-1 language code

**Enums:**
- `UserType` - Individual, Enterprise, Developer, Admin
- `UserRole` - Admin, User, Service
- `BotType` - Public, Private
- `PlatformType` - Slack, Telegram, CLI

**Dependencies:** None (stdlib only)

**Example:**
```go
// Domain entity
type UserProfile struct {
    UserID            string
    Username          string
    Moniker           string
    FirstName         string
    LastName          string
    NickName          string
    PrimaryLanguage   string
    SecondaryLanguage string
    PrimaryLocation   string
    PrimaryEmail      string
    BackupEmail       string
    MobilePhone       string
    Timezone          string
    JobRole           string
    UserType          UserType
    Role              UserRole
    PlatformIDs       PlatformIDs
    AgentPreferences  AgentPreferences
    NotesInformation  NotesInformation
    Enabled           bool
    DataDirectory     string
    CreatedAt         time.Time
    UpdatedAt         time.Time
    LastVerified      time.Time
}

// Domain interface
type UserProfileRepository interface {
    Create(ctx context.Context, profile *UserProfile) error
    Get(ctx context.Context, userID string) (*UserProfile, error)
    Update(ctx context.Context, profile *UserProfile) error
    Delete(ctx context.Context, userID string) error
    List(ctx context.Context, filter Filter) ([]*UserProfile, error)
    FindByUsername(ctx context.Context, username string) (*UserProfile, error)
    FindByEmail(ctx context.Context, email string) (*UserProfile, error)
    FindByPlatformID(ctx context.Context, platform PlatformType, platformID string) (*UserProfile, error)
}

// Validation method
func (u *UserProfile) Validate() error {
    if u.UserID == "" {
        return ErrInvalidUserID
    }
    if u.Username == "" {
        return ErrInvalidUsername
    }
    if u.PrimaryEmail == "" || !isValidEmail(u.PrimaryEmail) {
        return ErrInvalidEmail
    }
    // ... more validation
    return nil
}
```

---

### Use Case Layer

**Location:** `internal/usecase/admin/`

**Responsibility:**
- Orchestrate business logic for admin operations
- Coordinate between domain entities and repositories
- Implement application-specific workflows
- Handle partial updates and complex operations
- Trigger audit logging for all admin actions
- Manage transactions across multiple repositories

**Contains:**

**Services:**
- `ConfigService` - Configuration management use cases
- `UserProfileService` - User profile management use cases
- `BotManagementService` - Bot configuration use cases
- `AuditService` - Audit log querying use cases

**Use Cases:**
- Create user with validation and directory setup
- Update user with partial field support
- Hot reload configuration with validation and rollback
- Enable/disable bot with gateway notification
- Search users by multiple criteria
- Export/import users in bulk

**Dependencies:**
- Domain layer (entities, interfaces)
- No dependencies on infrastructure or adapters

**Example:**
```go
// Use case orchestrator
type UserProfileService struct {
    repo       domain.UserProfileRepository
    validator  domain.Validator
    encryptor  domain.EncryptionService
    auditor    domain.AuditLogger
}

func NewUserProfileService(
    repo domain.UserProfileRepository,
    validator domain.Validator,
    encryptor domain.EncryptionService,
    auditor domain.AuditLogger,
) *UserProfileService {
    return &UserProfileService{
        repo:      repo,
        validator: validator,
        encryptor: encryptor,
        auditor:   auditor,
    }
}

// Create user use case
func (s *UserProfileService) CreateUser(
    ctx context.Context,
    input CreateUserInput,
) (*UserProfile, error) {
    // 1. Validate input
    if err := input.Validate(); err != nil {
        return nil, fmt.Errorf("invalid input: %w", err)
    }

    // 2. Create domain entity
    profile := &UserProfile{
        UserID:          generateUUID(),
        Username:        input.Username,
        PrimaryEmail:    input.PrimaryEmail,
        FirstName:       input.FirstName,
        LastName:        input.LastName,
        PrimaryLanguage: input.PrimaryLanguage,
        Timezone:        input.Timezone,
        Role:            input.Role,
        UserType:        input.UserType,
        Enabled:         true,
        CreatedAt:       time.Now(),
        UpdatedAt:       time.Now(),
    }

    // 3. Validate domain entity
    if err := profile.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // 4. Persist to repository
    if err := s.repo.Create(ctx, profile); err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }

    // 5. Audit log
    s.auditor.Log(ctx, AuditEntry{
        Action:    "user.created",
        ActorID:   getActorFromContext(ctx),
        TargetID:  profile.UserID,
        Timestamp: time.Now(),
    })

    return profile, nil
}

// Partial update use case
func (s *UserProfileService) UpdateUser(
    ctx context.Context,
    userID string,
    updates map[string]interface{},
) (*UserProfile, error) {
    // 1. Load existing profile
    profile, err := s.repo.Get(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to load user: %w", err)
    }

    // 2. Apply partial updates
    for key, value := range updates {
        switch key {
        case "firstName":
            profile.FirstName = value.(string)
        case "lastName":
            profile.LastName = value.(string)
        case "primaryEmail":
            profile.PrimaryEmail = value.(string)
        // ... handle all updatable fields
        }
    }

    profile.UpdatedAt = time.Now()

    // 3. Validate updated entity
    if err := profile.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // 4. Persist changes
    if err := s.repo.Update(ctx, profile); err != nil {
        return nil, fmt.Errorf("failed to update user: %w", err)
    }

    // 5. Audit log
    s.auditor.Log(ctx, AuditEntry{
        Action:    "user.updated",
        ActorID:   getActorFromContext(ctx),
        TargetID:  profile.UserID,
        Details:   updates,
        Timestamp: time.Now(),
    })

    return profile, nil
}
```

---

### Infrastructure Layer

**Location:** `internal/infrastructure/admin/`

**Responsibility:**
- Implement domain interfaces with concrete implementations
- Handle external system interactions (file I/O, encryption)
- Provide technical capabilities (JSON serialization, file watching)
- Manage file-based storage with atomic writes
- Implement encryption/decryption for sensitive data

**Contains:**

**Implementations:**
- `FileUserProfileRepository` - File-based user profile storage
- `FileBotConfigRepository` - File-based bot configuration storage
- `YAMLConfigLoader` - YAML file loader
- `JSONConfigLoader` - JSON file loader
- `AESEncryptionService` - AES-256 encryption for credentials
- `FileAuditLogger` - File-based audit log storage

**Infrastructure Services:**
- `FileWatcher` - Watch configuration files for changes
- `AtomicFileWriter` - Atomic file write with temp file + rename
- `IndexBuilder` - Build and maintain JSON indexes

**Dependencies:**
- Domain layer (interfaces to implement)
- External libraries (YAML parser, encryption libs)
- File system

**Example:**
```go
// Infrastructure implementation
type FileUserProfileRepository struct {
    dataDir    string
    usersFile  string
    encryptor  domain.EncryptionService
    mu         sync.RWMutex
    index      *UserIndex
}

func NewFileUserProfileRepository(
    dataDir string,
    encryptor domain.EncryptionService,
) *FileUserProfileRepository {
    repo := &FileUserProfileRepository{
        dataDir:   dataDir,
        usersFile: filepath.Join(dataDir, "users.json"),
        encryptor: encryptor,
    }
    repo.loadIndex()
    return repo
}

func (r *FileUserProfileRepository) Create(
    ctx context.Context,
    profile *UserProfile,
) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // 1. Check for duplicates
    if r.index.ExistsByUsername(profile.Username) {
        return domain.ErrUserAlreadyExists
    }

    // 2. Create user directory
    userDir := filepath.Join(r.dataDir, "users", profile.UserID)
    if err := os.MkdirAll(userDir, 0755); err != nil {
        return fmt.Errorf("failed to create user directory: %w", err)
    }

    // 3. Write profile to user directory
    profileFile := filepath.Join(userDir, "profile.json")
    if err := atomicWriteJSON(profileFile, profile); err != nil {
        return fmt.Errorf("failed to write profile: %w", err)
    }

    // 4. Update central registry
    r.index.Add(profile)
    if err := r.saveIndex(); err != nil {
        return fmt.Errorf("failed to update index: %w", err)
    }

    return nil
}

func (r *FileUserProfileRepository) FindByPlatformID(
    ctx context.Context,
    platform PlatformType,
    platformID string,
) (*UserProfile, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Use index for fast lookup
    userID := r.index.FindByPlatform(platform, platformID)
    if userID == "" {
        return nil, domain.ErrUserNotFound
    }

    return r.loadProfile(userID)
}

// Atomic file write helper
func atomicWriteJSON(path string, data interface{}) error {
    // 1. Marshal to JSON
    jsonData, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal failed: %w", err)
    }

    // 2. Write to temp file
    tempFile := path + ".tmp"
    if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
        return fmt.Errorf("write temp file failed: %w", err)
    }

    // 3. Atomic rename
    if err := os.Rename(tempFile, path); err != nil {
        os.Remove(tempFile)
        return fmt.Errorf("rename failed: %w", err)
    }

    return nil
}
```

---

### Adapter Layer

**Location:** `internal/adapter/http/` and `internal/adapter/cli/`

**Responsibility:**
- Adapt external interfaces (HTTP, CLI) to internal use cases
- Handle protocol conversions (HTTP requests ↔ use case inputs)
- Manage request/response transformations
- Implement authentication and authorization
- Format user-friendly error messages

**Contains:**

**HTTP Adapters:**
- `ConfigHandler` - Configuration API endpoints
- `UserHandler` - User management API endpoints
- `BotHandler` - Bot management API endpoints
- `AuditHandler` - Audit log API endpoints
- `WebUIHandler` - Web admin page handlers
- `AuthMiddleware` - Authentication verification
- `RBACMiddleware` - Role-based access control
- `AuditMiddleware` - Request/response logging

**CLI Adapters:**
- `ConfigCommand` - Configuration CLI commands
- `AdminCommand` - Admin CLI commands
- `ServerCommand` - Server management CLI commands
- `OutputFormatter` - Format output (table, JSON, YAML)

**Dependencies:**
- Use case layer (services)
- Infrastructure layer (implementations for dependency injection)
- HTTP libraries (net/http, gorilla/mux, etc.)
- CLI libraries (cobra, etc.)

**Example:**
```go
// HTTP adapter
type UserHandler struct {
    service *usecase.UserProfileService
}

func NewUserHandler(service *usecase.UserProfileService) *UserHandler {
    return &UserHandler{service: service}
}

// POST /api/v1/users - Create user
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // 2. Convert to use case input
    input := usecase.CreateUserInput{
        Username:        req.Username,
        PrimaryEmail:    req.Email,
        FirstName:       req.FirstName,
        LastName:        req.LastName,
        PrimaryLanguage: req.Language,
        Timezone:        req.Timezone,
        Role:            req.Role,
        UserType:        req.UserType,
    }

    // 3. Execute use case
    profile, err := h.service.CreateUser(r.Context(), input)
    if err != nil {
        if errors.Is(err, domain.ErrUserAlreadyExists) {
            http.Error(w, "User already exists", http.StatusConflict)
        } else {
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }

    // 4. Convert to response
    resp := UserResponse{
        ID:       profile.UserID,
        Username: profile.Username,
        Email:    profile.PrimaryEmail,
        // ... map all fields
    }

    // 5. Send response
    w.Header().Set("Content-Type", "application/json")
    w.WriteStatus(http.StatusCreated)
    json.NewEncoder(w).Encode(resp)
}

// PUT /api/v1/users/:id - Update user (partial)
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
    // 1. Get user ID from path
    userID := mux.Vars(r)["id"]

    // 2. Parse partial update request
    var updates map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // 3. Execute use case
    profile, err := h.service.UpdateUser(r.Context(), userID, updates)
    if err != nil {
        // ... error handling
    }

    // 4. Send response
    // ... similar to CreateUser
}
```

---

## Data Flow

### 1. Hot Configuration Reload Flow

**Scenario:** Admin triggers configuration reload via web UI button

```
Admin → Web UI → REST API → Config Manager → Services → Audit Log
  ↓                                ↓
  ←─────────────────────Response───┘
```

**Step-by-Step:**

1. **Admin clicks "Reload Configuration" button** in web UI
2. **Web UI sends POST /api/v1/config/reload** HTTP request
3. **Auth middleware verifies** admin session and RBAC permissions
4. **Config API handler** calls ConfigService.Reload()
5. **ConfigService orchestrates reload**:
   - Load config.yaml, users.json, bots.json from disk
   - Validate all configuration integrity
   - Check business rules (e.g., default model exists)
   - If validation fails, reject and return error
   - If validation succeeds, apply new configuration atomically
6. **Config Manager notifies** all subscribed services:
   - LLM Engine (reload provider configurations)
   - Gateway Manager (enable/disable gateways)
   - User Profile System (reload user index)
   - Bot Management System (reload bot configurations)
7. **Audit log records** reload event with timestamp and admin ID
8. **Response flows back** to admin with success/failure status
9. **Web UI displays** notification and updates dashboard

**Error Handling:**
- If any file is missing, return error without applying changes
- If validation fails, return specific validation errors
- If notification to service fails, log warning but continue
- Rollback not needed (current config remains active until new config validates)

**Data Flow Diagram:**
```
┌──────┐
│Admin │
└──┬───┘
   │ Click "Reload Config"
   ▼
┌────────────────┐
│  Web UI        │
│  (JavaScript)  │
└──────┬─────────┘
       │ POST /api/v1/config/reload
       ▼
┌────────────────────────┐
│  Auth Middleware       │
│  - Verify session      │
│  - Check RBAC (admin)  │
└──────┬─────────────────┘
       │
       ▼
┌────────────────────────┐
│  Config API Handler    │
│  - Parse request       │
│  - Call service        │
└──────┬─────────────────┘
       │
       ▼
┌───────────────────────────────────────┐
│  ConfigService.Reload()                │
│  ┌─────────────────────────────────┐  │
│  │ 1. Load files                   │  │
│  │    - config.yaml                │  │
│  │    - users.json                 │  │
│  │    - bots.json                  │  │
│  └──────────────┬──────────────────┘  │
│                 ▼                      │
│  ┌─────────────────────────────────┐  │
│  │ 2. Validate                     │  │
│  │    - Schema validation          │  │
│  │    - Business rules             │  │
│  │    - Cross-references           │  │
│  └──────────────┬──────────────────┘  │
│                 ▼                      │
│         [Valid?]───No──> Return Error │
│            │ Yes                       │
│            ▼                           │
│  ┌─────────────────────────────────┐  │
│  │ 3. Apply atomically             │  │
│  │    - Swap config pointer        │  │
│  │    - Update indexes             │  │
│  └──────────────┬──────────────────┘  │
│                 ▼                      │
│  ┌─────────────────────────────────┐  │
│  │ 4. Notify services              │  │
│  │    ├─> LLM Engine               │  │
│  │    ├─> Gateway Manager          │  │
│  │    ├─> User Profile System      │  │
│  │    └─> Bot Management System    │  │
│  └──────────────┬──────────────────┘  │
└─────────────────┼─────────────────────┘
                  ▼
       ┌─────────────────────┐
       │  Audit Logger       │
       │  - Record reload    │
       │  - Timestamp        │
       │  - Admin ID         │
       └─────────────────────┘
```

---

### 2. User Creation Flow

**Scenario:** Admin creates new user via web UI

```
Admin → Web UI Form → REST API → UserService → Repository → File System
                                      ↓
                                  Audit Log
```

**Step-by-Step:**

1. **Admin fills out "Create User" form** with username, email, name, timezone, etc.
2. **Web UI sends POST /api/v1/users** with form data as JSON
3. **Auth middleware verifies** admin session and RBAC permissions
4. **User API handler** parses request and converts to CreateUserInput
5. **UserProfileService.CreateUser()**:
   - Validate input (required fields, format checks)
   - Generate new user ID (UUID)
   - Create UserProfile domain entity
   - Validate entity (business rules)
   - Call repository.Create()
6. **FileUserProfileRepository.Create()**:
   - Check for duplicate username/email in index
   - Create user directory: data/users/<user-id>/
   - Write profile.json to user directory
   - Update central users.json registry with new user
   - Update indexes (byUsername, byEmail, byPlatform)
   - Atomic file writes (temp file + rename)
7. **AuditLogger records** user.created event
8. **Response flows back** with created user details
9. **Web UI displays** success message and redirects to user list

**Data Flow Diagram:**
```
┌──────┐
│Admin │
└──┬───┘
   │ Fill form: username, email, firstName, lastName, etc.
   ▼
┌────────────────┐
│  Web UI Form   │
│  /admin/users/ │
│  create        │
└──────┬─────────┘
       │ POST /api/v1/users
       │ {username, email, firstName, ...}
       ▼
┌────────────────────────┐
│  User API Handler      │
│  - Validate request    │
│  - Convert to input    │
└──────┬─────────────────┘
       │
       ▼
┌───────────────────────────────────────────────────┐
│  UserProfileService.CreateUser()                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 1. Validate input                           │  │
│  │    - Required fields present?               │  │
│  │    - Email format valid?                    │  │
│  │    - Username format valid?                 │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 2. Create domain entity                     │  │
│  │    - Generate UUID                          │  │
│  │    - Set timestamps                         │  │
│  │    - Populate fields                        │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 3. Validate entity                          │  │
│  │    - Business rules                         │  │
│  │    - Cross-field validation                 │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 4. Call repository.Create()                 │  │
│  └──────────────┬──────────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  ▼
┌───────────────────────────────────────────────────┐
│  FileUserProfileRepository.Create()               │
│  ┌─────────────────────────────────────────────┐  │
│  │ 1. Check duplicates in index                │  │
│  │    - Username exists? → Error               │  │
│  │    - Email exists? → Error                  │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 2. Create user directory                    │  │
│  │    mkdir data/users/<user-id>/              │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 3. Write profile.json                       │  │
│  │    - Temp file write                        │  │
│  │    - Atomic rename                          │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 4. Update users.json registry               │  │
│  │    - Add to users array                     │  │
│  │    - Update indexes                         │  │
│  │    - Atomic write                           │  │
│  └──────────────┬──────────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  ▼
       ┌─────────────────────┐
       │  File System        │
       │  data/              │
       │  ├── users.json     │
       │  └── users/         │
       │      └── <id>/      │
       │          └── profile│
       └─────────────────────┘
                  │
                  ▼
       ┌─────────────────────┐
       │  Audit Logger       │
       │  - user.created     │
       │  - admin ID         │
       │  - timestamp        │
       └─────────────────────┘
```

---

### 3. Bot Enable/Disable Flow

**Scenario:** Admin enables a disabled Slack bot via web UI

```
Admin → Web UI → REST API → BotService → Repository → Gateway Adapter
                                              ↓
                                         Audit Log
```

**Step-by-Step:**

1. **Admin clicks "Enable" button** next to disabled bot in bot list
2. **Web UI sends POST /api/v1/bots/:id/enable**
3. **Auth middleware verifies** admin permissions
4. **Bot API handler** calls BotManagementService.EnableBot(id)
5. **BotManagementService.EnableBot()**:
   - Load bot configuration from repository
   - Validate bot configuration is complete (has credentials)
   - Set enabled=true on bot entity
   - Call repository.Update()
6. **FileBotConfigRepository.Update()**:
   - Update bot in bots.json
   - Update indexes
   - Atomic file write
7. **BotManagementService notifies** GatewayAdapter
8. **GatewayAdapter.ConnectBot()**:
   - If Slack bot, initialize Slack client with bot token
   - Establish WebSocket connection to Slack
   - Register event handlers
   - Start message polling
9. **AuditLogger records** bot.enabled event
10. **Response flows back** with updated bot status
11. **Web UI updates** button to "Disable" and shows bot as active

**Data Flow Diagram:**
```
┌──────┐
│Admin │
└──┬───┘
   │ Click "Enable Bot"
   ▼
┌────────────────┐
│  Web UI        │
│  /admin/bots   │
└──────┬─────────┘
       │ POST /api/v1/bots/:id/enable
       ▼
┌────────────────────────┐
│  Bot API Handler       │
│  - Parse bot ID        │
│  - Call service        │
└──────┬─────────────────┘
       │
       ▼
┌───────────────────────────────────────────────────┐
│  BotManagementService.EnableBot(id)               │
│  ┌─────────────────────────────────────────────┐  │
│  │ 1. Load bot config from repository          │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 2. Validate bot config                      │  │
│  │    - Has bot token?                         │  │
│  │    - Has app token? (if Slack)              │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 3. Update bot entity                        │  │
│  │    bot.Enabled = true                       │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 4. Persist via repository.Update()          │  │
│  └──────────────┬──────────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  ▼
┌───────────────────────────────────────────────────┐
│  FileBotConfigRepository.Update()                 │
│  ┌─────────────────────────────────────────────┐  │
│  │ 1. Update bots.json                         │  │
│  │    - Find bot by ID                         │  │
│  │    - Set enabled=true                       │  │
│  │    - Atomic write                           │  │
│  └──────────────┬──────────────────────────────┘  │
└─────────────────┼────────────────────────────────┘
                  ▼
       ┌─────────────────────┐
       │  File System        │
       │  data/bots.json     │
       │  (updated)          │
       └─────────────────────┘
                  │
                  ▼
┌─────────────────┴──────────────────────────────────┐
│  BotManagementService.NotifyGateway()              │
│  ┌─────────────────────────────────────────────┐   │
│  │ Call GatewayAdapter.ConnectBot(bot)         │   │
│  └──────────────┬──────────────────────────────┘   │
└─────────────────┼─────────────────────────────────┘
                  ▼
┌───────────────────────────────────────────────────┐
│  GatewayAdapter.ConnectBot()                      │
│  ┌─────────────────────────────────────────────┐  │
│  │ 1. Decrypt bot credentials                  │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 2. Initialize Slack/Telegram client        │  │
│  │    - Create client with token               │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 3. Establish connection                     │  │
│  │    - WebSocket for Slack                    │  │
│  │    - Webhook for Telegram                   │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 4. Register event handlers                  │  │
│  │    - Message received                       │  │
│  │    - App mention                            │  │
│  └──────────────┬──────────────────────────────┘  │
│                 ▼                                  │
│  ┌─────────────────────────────────────────────┐  │
│  │ 5. Start message polling                    │  │
│  │    - Launch goroutine                       │  │
│  └─────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────┘
                  │
                  ▼
       ┌─────────────────────┐
       │  Slack/Telegram     │
       │  Platform           │
       │  (connected)        │
       └─────────────────────┘
                  │
                  ▼
       ┌─────────────────────┐
       │  Audit Logger       │
       │  - bot.enabled      │
       │  - admin ID         │
       │  - bot ID           │
       └─────────────────────┘
```

---

### 4. REST API Request Flow (Generic)

**Scenario:** Any REST API request with authentication and RBAC

```
Client → HTTP Server → Auth → RBAC → Handler → Service → Repository
                         ↓      ↓        ↓         ↓
                      Audit  Audit   Audit    Audit
```

**Middleware Chain:**
```
Request
  ↓
┌─────────────────────┐
│ 1. CORS Middleware  │
│    - Allow origins  │
│    - Allow headers  │
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 2. Auth Middleware  │
│    - Verify session │
│    - Load user      │
│    - Set context    │
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 3. RBAC Middleware  │
│    - Check role     │
│    - Check endpoint │
│    - Allow/deny     │
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 4. Audit Middleware │
│    - Log request    │
│    - Log response   │
│    - Record timing  │
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 5. Handler          │
│    - Parse request  │
│    - Call service   │
│    - Format response│
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 6. Service          │
│    - Business logic │
│    - Validation     │
│    - Orchestration  │
└──────────┬──────────┘
           ▼
┌─────────────────────┐
│ 7. Repository       │
│    - Data access    │
│    - Persistence    │
└──────────┬──────────┘
           ▼
        Response
```

---

### 5. Web Admin Authentication Flow

**Scenario:** Admin logs into web admin interface

```
Admin → Login Page → Auth Handler → Session Manager → Cookie → Dashboard
```

**Step-by-Step:**

1. **Admin navigates** to http://localhost:8080/admin/
2. **Server detects** no valid session cookie, redirects to /admin/login
3. **Admin enters** username and password in login form
4. **Form submits** POST /admin/login with credentials
5. **Auth handler**:
   - Validate credentials against user repository
   - Check user.Role == "admin"
   - If valid, create session
6. **Session manager**:
   - Generate session ID (UUID)
   - Generate CSRF token
   - Store session in memory store with timeout (24h default)
   - Create secure session cookie
7. **Response** sets cookies and redirects to /admin/
8. **Dashboard loads** with session authenticated
9. **All subsequent requests** include session cookie for authentication

**Cookie Security:**
- HttpOnly: true (JavaScript cannot access)
- Secure: true (HTTPS only, if TLS enabled)
- SameSite: Strict (CSRF protection)
- Path: /admin/ (limited scope)
- MaxAge: 86400 (24 hours)

**CSRF Protection:**
- CSRF token generated per session
- Included in all forms as hidden field
- Verified on all POST/PUT/DELETE requests
- Token mismatch → 403 Forbidden

---

## Sequence Diagrams

### Sequence 1: Hot Configuration Reload

**Scenario:** Admin triggers /refresh command from server STDIO

```
Admin     Server     ConfigMgr   Validator   Services    FileSystem   AuditLog
 |           |           |           |           |            |           |
 |--/refresh>|           |           |           |            |           |
 |           |--Reload()->|           |           |            |           |
 |           |           |--Load-----|-----------|--------->  |           |
 |           |           |           |           |  <--files--|           |
 |           |           |--Validate>|           |            |           |
 |           |           |           |--Check--> |            |           |
 |           |           |           | <--OK---- |            |           |
 |           |           | <--Valid--|           |            |           |
 |           |           |--Apply----|           |            |           |
 |           |           |--Notify--------------->|           |           |
 |           |           |           |           |--Update--> |           |
 |           |           |           |           |            |           |
 |           |           |--Log------|-----------|-----------|---------->|
 |           | <--Success|           |           |            |           |
 | <--OK-----|           |           |           |            |           |
```

**Steps:**
1. Admin types `/refresh` in server STDIO
2. Server calls ConfigManager.Reload()
3. ConfigManager loads all config files from file system
4. ConfigManager validates configuration via Validator
5. Validator checks schema, business rules, cross-references
6. If valid, ConfigManager applies configuration atomically
7. ConfigManager notifies all subscribed services
8. Services update their internal state
9. AuditLog records reload event
10. Success message returned to admin

---

### Sequence 2: User Profile Update (Partial)

**Scenario:** Admin updates user's timezone via REST API

```
Admin   HTTP    Auth    RBAC   UserAPI  UserSvc   Repo   FileSystem  AuditLog
 |       |       |       |       |        |        |         |          |
 |--PUT->|       |       |       |        |        |         |          |
 |       |--Verify>|      |       |        |        |         |          |
 |       | <-User-|      |       |        |        |         |          |
 |       |--Check------->|       |        |        |         |          |
 |       | <--Allow------|       |        |        |         |          |
 |       |--Handle-------------->|        |        |         |          |
 |       |       |       |       |--Update>|       |         |          |
 |       |       |       |       |        |--Get-->|         |          |
 |       |       |       |       |        |        |--Read-->|          |
 |       |       |       |       |        |        | <-Profile          |
 |       |       |       |       |        | <-User-|         |          |
 |       |       |       |       |        |--Apply>|         |          |
 |       |       |       |       |        |--Save->|         |          |
 |       |       |       |       |        |        |--Write->|          |
 |       |       |       |       |        |        | <-OK----|          |
 |       |       |       |       |        | <-OK---|         |          |
 |       |       |       |       |        |--Log-------------|--------->|
 |       |       |       |       | <-User-|        |         |          |
 |       | <--Response-----------|        |        |         |          |
 | <-200-|       |       |       |        |        |         |          |
```

**Steps:**
1. Admin sends PUT /api/v1/users/:id with partial update
2. Auth middleware verifies session and loads user
3. RBAC middleware checks user is admin
4. UserAPI handler parses request and calls service
5. UserService.Update() loads existing profile from repository
6. Repository reads profile.json from file system
7. Service applies partial updates to profile entity
8. Service validates updated profile
9. Repository writes updated profile to file system (atomic)
10. AuditLog records user.updated event
11. Response sent back to admin with updated user

---

### Sequence 3: Bot Enable with Gateway Notification

**Scenario:** Admin enables Slack bot, gateway establishes connection

```
Admin  HTTP  BotAPI  BotSvc  Repo  FileSystem  Gateway  Slack  AuditLog
 |      |      |       |      |        |          |       |       |
 |--POST>|      |       |      |        |          |       |       |
 |      |--Handle>      |      |        |          |       |       |
 |      |      |--Enable>|     |        |          |       |       |
 |      |      |       |--Get->|        |          |       |       |
 |      |      |       |      |--Read-->|          |       |       |
 |      |      |       |      | <-Bot---|          |       |       |
 |      |      |       | <-Bot|        |          |       |       |
 |      |      |       |--SetEnabled--->|          |       |       |
 |      |      |       |--Update>|      |          |       |       |
 |      |      |       |      |--Write->|          |       |       |
 |      |      |       |      | <-OK----|          |       |       |
 |      |      |       | <-OK-|        |          |       |       |
 |      |      |       |--NotifyGateway----------->|       |       |
 |      |      |       |      |        |          |--Connect>     |
 |      |      |       |      |        |          |       | <-OK--|
 |      |      |       |      |        |          | <-Connected   |
 |      |      |       |--Log-|--------|----------|-------|------>|
 |      |      | <-Bot-|      |        |          |       |       |
 |      | <--Response--|      |        |          |       |       |
 | <-200|      |       |      |        |          |       |       |
```

**Steps:**
1. Admin sends POST /api/v1/bots/:id/enable
2. BotAPI handler calls BotManagementService.EnableBot()
3. Service loads bot config from repository
4. Repository reads bots.json from file system
5. Service sets bot.Enabled = true
6. Service updates bot via repository
7. Repository writes updated bots.json (atomic)
8. Service notifies GatewayAdapter of enabled bot
9. GatewayAdapter establishes connection to Slack platform
10. Slack platform acknowledges connection
11. AuditLog records bot.enabled event
12. Response sent back to admin

---

## Integration Points

### Integration 1: File System

**Type:** Local File System
**Purpose:** Persistent storage for configuration and user data
**Protocol:** POSIX file I/O

**File Structure:**
```
<config-dir>/               # Default: ./config/
├── config.yaml             # Server configuration
└── .env                    # Environment variables (optional)

<data-dir>/                 # Default: ./data/
├── users.json              # Central user registry with indexes
├── bots.json               # Bot configurations
└── users/                  # User-specific directories
    └── <user-id>/
        ├── profile.json    # Detailed user profile
        ├── preferences.json # User preferences
        ├── todos.json      # User todos
        ├── repeated-actions.json # Repeated actions
        └── history.json    # User history

<logs-dir>/                 # Default: ./logs/
├── server.log              # Server application log
└── audit.log               # Audit trail log
```

**Operations:**
- Read configuration files (config.yaml, users.json, bots.json)
- Write user profiles and bot configurations
- Atomic writes using temp file + rename pattern
- Watch files for external changes (optional)

**Error Handling:**
- File not found: Create with defaults or return error
- Permission denied: Log error and fail gracefully
- Disk full: Log error and reject writes
- Corrupted file: Log error, attempt recovery, fallback to backup

**Concurrency:**
- File-level locking for writes (flock on Unix)
- Atomic writes prevent partial reads
- Read-write mutex in application layer

---

### Integration 2: REST API (Internal)

**Type:** HTTP REST API
**Purpose:** Communication between CLI tool and server
**Protocol:** HTTP/JSON

**Base URL:** `http://localhost:8080/api/v1`

**Authentication:** Session cookie or API key (header: X-API-Key)

**Endpoints:**

**Configuration:**
- `GET /config` - Get current configuration
- `POST /config/reload` - Trigger configuration reload
- `PUT /config` - Update configuration (partial)
- `POST /config/validate` - Validate configuration without applying

**Users:**
- `GET /users` - List users (pagination, filtering)
- `POST /users` - Create user
- `GET /users/:id` - Get user by ID
- `PUT /users/:id` - Update user (partial)
- `DELETE /users/:id` - Delete user
- `GET /users/search?q=<query>` - Search users

**Bots:**
- `GET /bots` - List bots
- `POST /bots` - Create bot
- `GET /bots/:id` - Get bot by ID
- `PUT /bots/:id` - Update bot
- `DELETE /bots/:id` - Delete bot
- `POST /bots/:id/enable` - Enable bot
- `POST /bots/:id/disable` - Disable bot

**Audit:**
- `GET /audit` - List audit entries
- `GET /audit/search?q=<query>` - Search audit log

**Error Responses:**
```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "User validation failed",
    "details": {
      "email": "Invalid email format"
    }
  }
}
```

**Status Codes:**
- 200: Success
- 201: Created
- 400: Bad Request (validation error)
- 401: Unauthorized (authentication failed)
- 403: Forbidden (RBAC denied)
- 404: Not Found
- 409: Conflict (duplicate)
- 500: Internal Server Error

---

### Integration 3: LLM Provider APIs

**Type:** External HTTP APIs
**Purpose:** LLM model inference
**Protocol:** HTTPS/JSON

**Providers:**

**1. Anthropic API**
- Endpoint: https://api.anthropic.com/v1/messages
- Authentication: X-API-Key header
- Models: claude-3-5-sonnet, claude-3-haiku, etc.

**2. OpenAI API**
- Endpoint: https://api.openai.com/v1/chat/completions
- Authentication: Bearer token
- Models: gpt-4-turbo, gpt-4, gpt-3.5-turbo

**3. AWS Bedrock**
- Endpoint: AWS SDK (varies by region)
- Authentication: AWS credentials (IAM)
- Models: anthropic.claude-v2, meta.llama2, etc.

**4. Ollama**
- Endpoint: http://localhost:11434/api/generate
- Authentication: None (local)
- Models: llama2, mistral, codellama, etc.

**Configuration Reload Impact:**
- Provider settings (API keys, base URLs) can be updated
- Model instances can be added/removed
- Default model fallback order can be changed
- Changes take effect immediately after /refresh
- Active requests complete with old configuration
- New requests use new configuration

**Error Handling:**
- API key invalid: Log error, try fallback model
- Rate limit exceeded: Exponential backoff, try fallback
- Network error: Retry with backoff, try fallback
- Model unavailable: Skip to next fallback model
- All models failed: Return error to user

---

### Integration 4: Gateway Platforms

**Type:** External APIs (Slack, Telegram)
**Purpose:** Bot message routing
**Protocol:** Webhook/WebSocket

**1. Slack Platform**
- Socket Mode: WebSocket connection with app token
- Events API: Webhook for events (app_mention, message, etc.)
- Web API: HTTP API for sending messages, uploading files, etc.
- Authentication: Bot token (xoxb-...) and App token (xapp-...)

**2. Telegram Platform**
- Bot API: HTTP API for sending/receiving messages
- Webhook: HTTPS endpoint for receiving updates
- Long Polling: Alternative to webhook
- Authentication: Bot token

**Bot Management Integration:**
- Bot configurations stored in bots.json
- Gateway adapter reads bot configs on initialization
- Enable bot: Gateway establishes connection
- Disable bot: Gateway closes connection gracefully
- Configuration reload: Gateway reconnects with new credentials
- Public bots: Available to all users
- Private bots: Available only to specified users

**Message Routing:**
1. Platform sends message to gateway
2. Gateway extracts platform user ID (Slack ID or Telegram ID)
3. Gateway looks up NuimanBot user via UserProfileRepository.FindByPlatformID()
4. Gateway routes message to LLM engine with user context
5. LLM engine generates response using user's agent preferences
6. Gateway sends response back to platform

---

### Integration 5: SQLite Database

**Type:** Embedded SQL database
**Purpose:** Conversation history and message storage
**Protocol:** SQL

**Database File:** `<data-dir>/conversations.db`

**Tables:**
- `conversations` - Conversation metadata
- `messages` - Individual messages
- `attachments` - File attachments

**Integration with User Profiles:**
- Messages linked to user via user_id foreign key
- User profiles stored separately in users.json
- Database contains only conversation history, not user profiles
- Rationale: User profiles need multi-platform indexing, better suited for JSON

**Schema:**
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    gateway TEXT NOT NULL,  -- cli, slack, telegram
    started_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,  -- user, assistant, system
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);
```

**Note:** User table in SQLite is separate from UserProfile in users.json. SQLite user table is minimal (auth only), UserProfile is comprehensive (identity, preferences, etc.).

---

## Architectural Decisions

### ADR-001: File-Based Storage for User Profiles

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
User profiles need to be stored persistently with support for multi-platform ID indexing, fast lookups by username/email/platform ID, and easy backup/versioning. Options include SQLite database (existing infrastructure) or JSON files (new approach).

**Decision:**
Use file-based storage with central users.json registry and per-user directories for detailed data.

**Rationale:**
- **Simplicity**: JSON files are human-readable and easily editable
- **Version Control**: Files can be committed to git for audit trail
- **Backup**: Simple file copy for backup, no database dump needed
- **Portability**: JSON is universal, no database schema migration
- **Indexing**: Central users.json contains indexes for fast lookup
- **Scalability**: Sufficient for hundreds of users, can migrate to DB later if needed
- **Container-Friendly**: Easy to mount as volumes, no database initialization

**Consequences:**

**Positive:**
- Easy to inspect and debug user data
- No database migrations for schema changes
- Simple backup and restore (copy files)
- Can edit manually in emergency
- Git-friendly for configuration management

**Negative:**
- No ACID transactions (mitigated by atomic file writes)
- No complex queries (mitigated by in-memory indexes)
- File locking complexity (mitigated by per-user locks)
- Not suitable for thousands of users (acceptable for current scope)

**Alternatives Considered:**

1. **SQLite Database**
   - Pros: ACID transactions, SQL queries, existing infrastructure
   - Cons: Requires migrations, harder to inspect, backup complexity
   - Why rejected: Overkill for current scale, harder to version control

2. **PostgreSQL/MySQL**
   - Pros: Full SQL capabilities, high scalability, strong consistency
   - Cons: External dependency, deployment complexity, overkill
   - Why rejected: Not needed for current scale, violates simplicity goal

---

### ADR-002: Multi-Component Architecture (Daemon + CLI)

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
Current monolithic architecture combines server runtime and administration in single binary. This creates confusion (is it a server or a tool?), limits deployment options, and mixes concerns. Need to separate runtime (long-running server) from administration (short-lived commands).

**Decision:**
Split into two binaries: nuimanbotd (daemon server) and nuimanbot (CLI tool).

**Rationale:**
- **Separation of Concerns**: Server runtime vs administration are distinct responsibilities
- **Deployment Clarity**: Server runs as daemon/service, CLI runs on-demand
- **Container-Friendly**: Server runs in container, CLI can connect remotely
- **STDIO Simplification**: Server STDIO only for /refresh and /exit, not cluttered with admin commands
- **Remote Management**: CLI can manage server via API, doesn't need same host
- **Security**: Server can run with minimal permissions, CLI used only by admins

**Consequences:**

**Positive:**
- Clear deployment model (systemd service for daemon, CLI in PATH)
- Better security model (daemon doesn't need interactive terminal)
- Easier containerization (daemon in container, CLI outside)
- Simpler STDIO interface (only essential commands)
- Remote administration possible (CLI → API → server)

**Negative:**
- Two binaries to maintain and distribute
- Breaking change (existing users must migrate)
- CLI must communicate with server (API dependency)
- Documentation must explain both components

**Alternatives Considered:**

1. **Single Binary with Subcommands**
   - Pros: One binary, simpler distribution
   - Cons: Confusion (is `nuimanbot serve` a server or a command?), STDIO still cluttered
   - Why rejected: Doesn't solve separation of concerns problem

2. **Server-Only with Web UI Admin**
   - Pros: No CLI binary needed, all admin via web
   - Cons: No scripting support, harder automation, requires browser
   - Why rejected: CLI is essential for automation and scripting

---

### ADR-003: Hot Configuration Reload

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
Configuration changes currently require server restart, which disrupts active user conversations and causes downtime. For production use, need ability to update configuration (LLM providers, user profiles, bot credentials) without restarting server.

**Decision:**
Implement hot configuration reload triggered by /refresh command or API endpoint.

**Rationale:**
- **Zero Downtime**: Active conversations continue during configuration update
- **Fast Iteration**: Admins can test configuration changes quickly
- **Production Friendly**: No service interruption for config updates
- **Validation Before Apply**: Invalid config rejected, server continues with current config
- **Rollback Safety**: If reload fails, previous config remains active

**Consequences:**

**Positive:**
- Configuration changes apply immediately
- No conversation interruption
- Better production operational experience
- Faster debugging and testing
- Auditability (all reloads logged)

**Negative:**
- Complexity in config manager (thread-safe access, atomic updates)
- Service notification required (services must handle config changes)
- Potential race conditions (in-flight requests use old config)
- Memory usage (both old and new config in memory briefly)

**Alternatives Considered:**

1. **Require Restart**
   - Pros: Simplicity, no concurrency issues
   - Cons: Conversation interruption, downtime, poor UX
   - Why rejected: Unacceptable for production use

2. **File Watching with Auto-Reload**
   - Pros: Automatic, no manual trigger needed
   - Cons: Surprising behavior, hard to control timing, accidental reloads
   - Why rejected: Too magical, admins want explicit control

---

### ADR-004: Partial Update Support for REST API

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
REST APIs traditionally require full object replacement on PUT requests. For user profiles with many fields, this is cumbersome (must send all fields even if changing one). Need ability to update only specific fields without affecting others.

**Decision:**
Support partial updates via PUT requests with only specified fields in request body.

**Rationale:**
- **Usability**: Admins can update single field without knowing all other fields
- **Efficiency**: Less data transferred over network
- **Safety**: Reduces risk of accidentally overwriting fields with default values
- **Flexibility**: Supports both full and partial updates
- **Standard Practice**: Common in modern REST APIs (PATCH semantics on PUT)

**Consequences:**

**Positive:**
- Easier to use (change timezone without knowing all other fields)
- Fewer errors (no accidental overwrites)
- Better API ergonomics
- Supports incremental updates

**Negative:**
- Implementation complexity (must load, merge, validate)
- Potential for inconsistent state if validation fails mid-merge
- Ambiguity (is null a value or field omission?)
- Must document which fields are updatable

**Implementation Strategy:**
- Load existing entity from repository
- Apply updates from request (only present fields)
- Validate merged entity
- Save if valid, reject if invalid

**Null Handling:**
- Absent field: No change
- Null value: Clear/reset to default (if field is optional)
- Empty string: Set to empty (if valid)

**Alternatives Considered:**

1. **PATCH with JSON Patch**
   - Pros: Standard RFC 6902, explicit operations
   - Cons: Complex, requires array of operations, overkill
   - Why rejected: Too complex for simple field updates

2. **Require Full Object on PUT**
   - Pros: Simplicity, no ambiguity
   - Cons: Poor UX, error-prone, inefficient
   - Why rejected: Unacceptable UX for large objects

---

### ADR-005: Session-Based Auth for Web UI, API Keys for REST API

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
Web admin interface needs authentication. Options include session cookies (traditional web) or API keys (REST API pattern). REST API also needs authentication for CLI tool access.

**Decision:**
- Web UI: Session-based authentication with secure cookies
- REST API: Support both session cookies (for web UI) and API keys (for CLI/automation)

**Rationale:**
- **Web UI Best Practice**: Session cookies are standard for web apps, better UX (auto-logout on browser close)
- **API Key for Automation**: CLI tool and scripts need long-lived credentials
- **Security**: Sessions can be revoked, API keys can be rotated
- **CSRF Protection**: Sessions support CSRF tokens, API keys don't need CSRF protection

**Consequences:**

**Positive:**
- Standard web authentication pattern (familiar to users)
- Supports both interactive (web) and automated (CLI/scripts) use cases
- CSRF protection for web UI
- Session timeout enforces re-authentication
- API keys enable headless automation

**Negative:**
- Two authentication mechanisms to maintain
- Session storage required (in-memory or Redis)
- Must handle session expiration and renewal
- API key management UI needed

**Session Details:**
- Storage: In-memory (acceptable for single instance)
- Timeout: 24 hours (configurable)
- Cookies: HttpOnly, Secure (if HTTPS), SameSite=Strict
- CSRF: Token generated per session, validated on mutations

**API Key Details:**
- Storage: User.APIKeys array in user profile
- Format: nuiman_<random-base64>
- Rotation: Manual via web UI or CLI
- Scope: Per-user (inherits user's role and permissions)

**Alternatives Considered:**

1. **OAuth/OIDC**
   - Pros: Standard protocol, SSO support, delegated auth
   - Cons: Complex, requires auth provider, overkill for local tool
   - Why rejected: Too complex for initial release

2. **API Keys Only**
   - Pros: Simplicity, one auth mechanism
   - Cons: Poor web UI UX (must copy-paste key), no CSRF protection
   - Why rejected: Bad UX for web interface

---

### ADR-006: STDIO Simplification (/refresh and /exit Only)

**Date:** 2026-02-08
**Status:** Accepted

**Context:**
Current server STDIO interface accepts many commands (/user add, /bot enable, etc.), creating confusion about server's purpose. With web UI and CLI tool, server STDIO should be minimal.

**Decision:**
Limit server STDIO to only /refresh (reload config) and /exit (graceful shutdown).

**Rationale:**
- **Clarity**: Server is a daemon, not an interactive tool
- **Separation of Concerns**: Admin commands belong in CLI tool or web UI
- **Container-Friendly**: Containers expect minimal STDIO
- **Simplicity**: Fewer code paths to maintain
- **Essential Operations Only**: Reload and shutdown are system-level, not admin operations

**Consequences:**

**Positive:**
- Clear purpose (daemon server, not admin tool)
- Better container compatibility
- Simpler code (no STDIO command parser for admin commands)
- Forces use of proper admin interfaces (web UI, CLI tool)
- Easier to run as systemd service

**Negative:**
- Breaking change (users must use CLI tool for admin commands)
- Less convenient for quick admin tasks (must use separate tool)
- Requires documentation update

**Migration Path:**
- Document /refresh and /exit as only STDIO commands
- Provide CLI tool for all other operations
- Update tutorials to show CLI tool usage
- Add deprecation warnings in old version

**Alternatives Considered:**

1. **Keep All STDIO Commands**
   - Pros: Backward compatibility, convenience
   - Cons: Mixing concerns, confusing purpose, container-unfriendly
   - Why rejected: Conflicts with multi-component architecture goal

2. **Remove All STDIO Commands**
   - Pros: Pure daemon, no STDIO parsing needed
   - Cons: No emergency reload or shutdown, requires signals
   - Why rejected: /refresh and /exit are useful for operators

---

## Trade-offs

### Trade-off 1: File-Based Storage vs Database

**Choice:** File-based storage (users.json, bots.json)

**Benefits:**
- Simple backup and restore (copy files)
- Human-readable and editable
- Git-friendly (can version control)
- No database migrations needed
- Container-friendly (mount volumes)
- Easy inspection and debugging

**Costs:**
- No ACID transactions (partial writes possible on crash)
- Limited query capabilities (no SQL)
- File locking complexity for concurrent access
- Not suitable for large scale (thousands of users)
- Index maintenance required (manual updates)

**Mitigation:**
- Atomic file writes (temp file + rename) prevent partial writes
- In-memory indexes for fast lookups
- Per-entity locks for concurrency
- Document scale limits (recommended <1000 users)
- Provide migration path to database for large deployments

---

### Trade-off 2: Hot Reload vs Restart Required

**Choice:** Hot reload without restart

**Benefits:**
- Zero downtime for configuration changes
- Active conversations continue uninterrupted
- Faster iteration for admins
- Better production operational experience
- Immediate validation feedback

**Costs:**
- Increased code complexity (thread-safe config access)
- Service notification mechanism needed
- Potential for race conditions (in-flight requests)
- Memory overhead (old and new config in memory)
- More failure modes (reload can fail)

**Mitigation:**
- RWMutex for thread-safe config access
- Atomic config pointer swap
- Validation before apply (reject invalid configs)
- Rollback to previous config on failure
- Comprehensive error messages for debugging

---

### Trade-off 3: Multi-Component Architecture vs Monolith

**Choice:** Multi-component (daemon + CLI tool + embedded web UI)

**Benefits:**
- Clear separation of concerns (runtime vs administration)
- Better deployment model (daemon as service)
- Container-friendly (daemon in container)
- Remote administration possible
- Simpler STDIO interface
- Security (daemon doesn't need interactive terminal)

**Costs:**
- Two binaries to distribute and maintain
- Breaking change for existing users
- CLI must communicate with server (API dependency)
- More complex deployment (two binaries)
- Documentation must explain both components

**Mitigation:**
- Provide migration guide for existing users
- Bundle both binaries in releases
- Document deployment patterns (systemd, docker)
- Provide examples for common workflows
- CLI gracefully handles server unreachable

---

### Trade-off 4: Partial Updates vs Full Replacement

**Choice:** Support partial updates (PUT with subset of fields)

**Benefits:**
- Better UX (update one field without knowing others)
- Less data transferred
- Fewer errors (no accidental overwrites)
- More flexible API

**Costs:**
- Implementation complexity (load, merge, validate)
- Ambiguity (null vs absent field)
- Potential for inconsistent state
- Must document updatable fields

**Mitigation:**
- Clear null handling rules (absent = no change, null = clear)
- Validation after merge (catch inconsistencies)
- Document which fields are updatable
- Provide examples in API documentation

---

### Trade-off 5: Embedded Web UI vs Separate SPA

**Choice:** Embedded web UI (server-side rendering + HTMX)

**Benefits:**
- No separate deployment needed
- Simpler build process
- Faster page loads (no JS bundle download)
- Progressive enhancement (works without JS)
- Single process (no CORS issues)

**Costs:**
- Less rich interactivity than SPA
- Page reloads for navigation
- Harder to build complex UI
- Limited frontend framework ecosystem

**Mitigation:**
- HTMX for dynamic updates without full reload
- Minimal JavaScript for enhancement
- Focus on simple, functional UI
- Can migrate to SPA later if needed

---

## Performance Considerations

**Bottlenecks:**

1. **File I/O for User Profile Lookups**
   - Mitigation: In-memory index for fast lookups, lazy-load profiles, cache frequently accessed profiles

2. **Configuration Reload Under Load**
   - Mitigation: RWMutex allows concurrent reads during reload, atomic pointer swap minimizes lock time

3. **JSON Parsing for Large User Registry**
   - Mitigation: Incremental parsing, stream processing for large files, compression for storage

4. **Gateway Message Routing**
   - Mitigation: Concurrent goroutines per bot, platform ID index for fast user lookup

**Optimization Strategies:**

1. **Caching:**
   - User profiles cached in memory after first load
   - Configuration cached, only reload on /refresh
   - Template rendering cached (pre-compiled templates)
   - Static assets served with cache headers

2. **Indexing:**
   - Username index (hash map)
   - Email index (hash map)
   - Platform ID index (nested: platform -> platformID -> userID)
   - Bot type index (public/private)

3. **Lazy Loading:**
   - User directories loaded on demand
   - Preferences and history loaded only when needed
   - Bot credentials decrypted on gateway connection

4. **Batch Operations:**
   - Bulk user import (batch file writes)
   - Batch audit log writes (buffered)
   - Batch index updates (accumulate, write once)

**Concurrency:**

1. **Read-Write Mutex for Configuration:**
   ```go
   type ConfigManager struct {
       mu     sync.RWMutex
       config *Config
   }

   func (m *ConfigManager) Get() *Config {
       m.mu.RLock()
       defer m.mu.RUnlock()
       return m.config
   }

   func (m *ConfigManager) Reload() error {
       m.mu.Lock()
       defer m.mu.Unlock()
       // Load and validate new config
       newConfig := loadConfig()
       // Atomic swap
       m.config = newConfig
   }
   ```

2. **Per-User Locks for Profile Updates:**
   ```go
   type UserProfileRepository struct {
       mu    sync.RWMutex  // Global lock for index
       locks sync.Map      // Per-user locks
   }

   func (r *UserProfileRepository) Update(userID string, profile *UserProfile) error {
       // Acquire per-user lock
       lock := r.getUserLock(userID)
       lock.Lock()
       defer lock.Unlock()

       // Update profile
       // ...
   }
   ```

3. **Concurrent Gateway Handlers:**
   ```go
   func (g *Gateway) Start() {
       for _, bot := range g.bots {
           go g.handleBot(bot)  // One goroutine per bot
       }
   }
   ```

**Benchmarks:**

Target performance (single instance):
- User profile lookup: <10ms (with cache)
- Configuration reload: <100ms (for typical config)
- API request latency: <50ms (excluding LLM call)
- Concurrent users: 100+ (limited by LLM API rate limits)
- User profiles: 1000+ (file-based storage limit)

---

## Security Architecture

**Security Layers:**

1. **Input Validation (Adapter Layer)**
   - All HTTP requests validated against schema
   - Path parameters sanitized (prevent directory traversal)
   - JSON body validated (prevent injection)
   - File uploads restricted (if added in future)

2. **Authentication (Middleware)**
   - Session-based for web UI (secure cookies)
   - API key-based for REST API (X-API-Key header)
   - Invalid credentials → 401 Unauthorized
   - Rate limiting on auth endpoints (prevent brute force)

3. **Authorization (RBAC Middleware)**
   - Role-based access control (Admin, User, Service)
   - Endpoint-level permissions check
   - Admin-only endpoints: /api/v1/config, /api/v1/users, /api/v1/bots
   - User endpoints: /api/v1/profile (own profile only)
   - Unauthorized access → 403 Forbidden

4. **Data Encryption (Infrastructure Layer)**
   - Bot credentials encrypted at rest (AES-256)
   - API keys hashed (bcrypt or argon2)
   - Sensitive fields masked in logs
   - TLS for HTTPS (if enabled)

5. **Audit Logging (Cross-Cutting)**
   - All administrative actions logged
   - Authentication attempts logged (success and failure)
   - Configuration changes logged
   - Audit log immutable (append-only)

**Threat Model:**

1. **Unauthorized Configuration Access**
   - Threat: Attacker modifies config.yaml to inject malicious settings
   - Mitigation: File permissions (600), admin-only access, validation on load

2. **Credential Theft**
   - Threat: Attacker reads bots.json to steal bot tokens
   - Mitigation: Encryption at rest, file permissions, no plaintext credentials

3. **Session Hijacking**
   - Threat: Attacker steals session cookie to access admin UI
   - Mitigation: HttpOnly cookies, Secure flag (HTTPS), SameSite=Strict, session timeout

4. **CSRF Attacks**
   - Threat: Attacker tricks admin into submitting malicious form
   - Mitigation: CSRF tokens on all mutations, SameSite cookies

5. **API Key Leakage**
   - Threat: API key logged or exposed in error messages
   - Mitigation: Mask keys in logs, no keys in URLs, secure storage

6. **Audit Log Tampering**
   - Threat: Attacker deletes audit log to hide actions
   - Mitigation: Append-only log, separate from application logs, forward to SIEM

**Security Controls:**

1. **Input Sanitization:**
   ```go
   func sanitizeFilename(name string) string {
       // Remove directory traversal attempts
       name = filepath.Base(name)
       // Remove non-alphanumeric (except dash, underscore)
       name = regexp.MustCompile(`[^a-zA-Z0-9-_]`).ReplaceAllString(name, "")
       return name
   }
   ```

2. **Credential Encryption:**
   ```go
   func encryptCredential(plaintext string) (string, error) {
       key := getEncryptionKey()  // From environment or secure storage
       block, _ := aes.NewCipher(key)
       gcm, _ := cipher.NewGCM(block)
       nonce := make([]byte, gcm.NonceSize())
       rand.Read(nonce)
       ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
       return base64.StdEncoding.EncodeToString(ciphertext), nil
   }
   ```

3. **CSRF Protection:**
   ```go
   func csrfMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           if r.Method != "GET" {
               token := r.FormValue("csrf_token")
               expectedToken := getSessionToken(r)
               if token != expectedToken {
                   http.Error(w, "CSRF token mismatch", http.StatusForbidden)
                   return
               }
           }
           next.ServeHTTP(w, r)
       })
   }
   ```

4. **Rate Limiting:**
   ```go
   func rateLimitMiddleware(next http.Handler) http.Handler {
       limiter := rate.NewLimiter(rate.Limit(10), 20)  // 10 req/s, burst 20
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           if !limiter.Allow() {
               http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
               return
           }
           next.ServeHTTP(w, r)
       })
   }
   ```

---

## Scalability

**Current Limits:**

- **Users:** ~1000 (file-based storage, in-memory index)
- **Concurrent Requests:** 100+ (goroutine per request)
- **Bots:** ~50 (concurrent connections, platform limits)
- **Configuration Size:** ~10MB (YAML parsing overhead)
- **Audit Log:** Append-only file (rotation recommended)

**Scaling Strategy:**

**Vertical Scaling (Scale Up):**
- Increase server RAM for larger in-memory caches
- Faster disk for quicker file I/O
- More CPU cores for concurrent request handling
- Sufficient for <1000 users, <50 bots

**Horizontal Scaling (Scale Out):**
- Not supported in initial release (single instance architecture)
- Future consideration: Load balancer + multiple instances
- Requires: Shared storage (NFS/S3), distributed session store (Redis), leader election

**Future Considerations:**

1. **Database Migration:**
   - When user count exceeds 1000, migrate to PostgreSQL
   - Keep file-based config for simplicity
   - Use database for users, profiles, bots, audit log
   - Provide migration tool

2. **Distributed Cache:**
   - Add Redis for shared cache across instances
   - Store user profiles, session data, configuration
   - Reduce file I/O overhead

3. **Message Queue:**
   - Add message queue (RabbitMQ, Kafka) for bot events
   - Decouple gateway from LLM engine
   - Enable horizontal scaling of processing

4. **Observability:**
   - Add Prometheus metrics (request count, latency, error rate)
   - Add distributed tracing (OpenTelemetry)
   - Add structured logging (JSON logs for parsing)

---

## References

- [spec.md](spec.md) - Feature specification and requirements
- [data-dictionary.md](data-dictionary.md) - Data structures and types
- [plan.md](plan.md) - Implementation plan and phases
- [tasks.md](tasks.md) - Detailed task breakdown
- [PRD](/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/improved-admin-features-prd.md) - Product requirements document
- Clean Architecture: https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- REST API Best Practices: https://restfulapi.net/
- OWASP Security Guide: https://owasp.org/www-project-web-security-testing-guide/
