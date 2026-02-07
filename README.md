# NuimanBot

An AI agent framework built with Clean Architecture principles, featuring LLM integration, extensible skill system, and multiple messaging gateway support.

## Features

### Core Architecture
- **Clean Architecture**: Strict layer separation (Domain, Use Case, Adapter, Infrastructure)
- **Multi-LLM Support**: Anthropic Claude, OpenAI GPT, and Ollama (local models)
- **Rich Skill Library**: 5 built-in skills (calculator, datetime, weather, web search, notes)
- **Multiple Gateways**: CLI, Telegram, and Slack interfaces with concurrent operation

### Security & Access Control
- **RBAC System**: Role-based access control with user management
- **Rate Limiting**: Token bucket algorithm with per-user and per-skill limits
- **Secure Credentials**: AES-256-GCM encrypted credential vault with key rotation support
- **Input Validation**: Comprehensive protection against injection attacks and malicious input
- **Audit Logging**: Security event tracking for compliance and monitoring

### Data Management
- **SQLite Storage**: Persistent conversations, users, and notes with full CRUD
- **Conversation Summarization**: Automatic LLM-based summarization when context limits approached
- **Token Window Management**: Dynamic context sizing based on provider limits (200k Claude, 128k GPT-4, 32k Ollama)

### Performance & Observability
- **Connection Pooling**: Optimized database connections (25 max open, 5 idle, lifecycle management)
- **LLM Response Caching**: In-memory cache with SHA256 hashing (1000 entries, 1-hour TTL, 100% test coverage)
- **Message Batching**: Buffered writes with dual flush strategy (size-based + time-based)
- **Prometheus Metrics**: 14+ metric types exposed at `/metrics` endpoint
- **Health Checks**: Liveness, readiness, and version endpoints
- **Request Tracing**: Request ID propagation for distributed debugging

### Development & Quality
- **Configuration**: YAML file + environment variable override support with validation
- **Test Coverage**: ~85% coverage with comprehensive unit, integration, and E2E tests
- **TDD Methodology**: Strict Red-Green-Refactor cycles with mandatory refactoring phase

## Quick Start

📖 **For detailed installation and configuration instructions, see [ONBOARDING.md](support_docs/ONBOARDING.md)**

### Prerequisites

- Go 1.21 or later
- SQLite3
- At least one LLM provider API key:
  - Anthropic Claude (recommended)
  - OpenAI GPT
  - Ollama (for local models, no API key needed)
- Optional: OpenWeatherMap API key (for weather skill)
- Optional: Telegram Bot Token (for Telegram gateway)
- Optional: Slack Bot/App Tokens (for Slack gateway)

### Installation

```bash
# Clone the repository
git clone https://github.com/stainedhead/NuimanBot.git
cd NuimanBot

# Install dependencies
go mod download

# Build the application
go build -o bin/nuimanbot ./cmd/nuimanbot
```

### Configuration

#### Option 1: Configuration File

Create a `config.yaml` file in the project root:

```yaml
server:
  log_level: info
  debug: false

security:
  input_max_length: 4096
  vault_path: "./data/vault.enc"

storage:
  type: sqlite
  dsn: "./data/nuimanbot.db"

llm:
  # Provider-specific configurations (recommended)
  anthropic:
    api_key: "sk-ant-your-key-here"
  openai:
    api_key: "sk-your-openai-key"
    base_url: "https://api.openai.com/v1"  # optional
  ollama:
    base_url: "http://localhost:11434"  # for local models

gateways:
  cli:
    debug_mode: false
  telegram:
    enabled: true
    token: "your-telegram-bot-token"
    allowed_ids: [123456789]  # optional: restrict to specific users
  slack:
    enabled: true
    bot_token: "xoxb-your-bot-token"
    app_token: "xapp-your-app-token"  # required for Socket Mode

skills:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
    weather:
      enabled: true
      # Set OPENWEATHERMAP_API_KEY environment variable
    websearch:
      enabled: true
    notes:
      enabled: true
```

#### Option 2: Environment Variables

```bash
# Required
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-encryption-key-here"

# LLM Configuration (choose one or more)
# Anthropic
export NUIMANBOT_LLM_ANTHROPIC_APIKEY="sk-ant-your-key"

# OpenAI
export NUIMANBOT_LLM_OPENAI_APIKEY="sk-your-openai-key"
export NUIMANBOT_LLM_OPENAI_BASEURL="https://api.openai.com/v1"  # optional

# Ollama (local models)
export NUIMANBOT_LLM_OLLAMA_BASEURL="http://localhost:11434"

# Gateway Configuration
export NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED="true"
export NUIMANBOT_GATEWAYS_TELEGRAM_TOKEN="your-telegram-bot-token"

export NUIMANBOT_GATEWAYS_SLACK_ENABLED="true"
export NUIMANBOT_GATEWAYS_SLACK_BOTTOKEN="xoxb-your-bot-token"
export NUIMANBOT_GATEWAYS_SLACK_APPTOKEN="xapp-your-app-token"

# Skills Configuration
export OPENWEATHERMAP_API_KEY="your-openweathermap-key"  # for weather skill

# Optional overrides
export NUIMANBOT_SERVER_LOGLEVEL="debug"
export NUIMANBOT_SECURITY_INPUTMAXLENGTH="8192"
```

### Running

```bash
# Ensure encryption key is set
export NUIMANBOT_ENCRYPTION_KEY="12345678901234567890123456789012"

# Choose your LLM provider:

# Option A: Anthropic Claude
export NUIMANBOT_LLM_ANTHROPIC_APIKEY="sk-ant-your-key-here"

# Option B: OpenAI GPT
export NUIMANBOT_LLM_OPENAI_APIKEY="sk-your-openai-key"

# Option C: Ollama (local)
export NUIMANBOT_LLM_OLLAMA_BASEURL="http://localhost:11434"

# Optional: Weather skill
export OPENWEATHERMAP_API_KEY="your-weather-api-key"

# Run the application
./bin/nuimanbot
```

The CLI will start and you can interact with the bot:

```
NuimanBot starting...
Config file used: ./config.yaml
2026/02/06 12:00:00 Database schema initialized successfully
2026/02/06 12:00:00 Calculator skill registered
2026/02/06 12:00:00 DateTime skill registered
2026/02/06 12:00:00 Weather skill registered
2026/02/06 12:00:00 WebSearch skill registered
2026/02/06 12:00:00 Notes skill registered
2026/02/06 12:00:00 Registered built-in skills successfully
2026/02/06 12:00:00 NuimanBot initialized with:
2026/02/06 12:00:00   Log Level: info
2026/02/06 12:00:00   Debug Mode: false
2026/02/06 12:00:00   LLM Provider: anthropic
2026/02/06 12:00:00   Skills Registered: 5

Starting CLI Gateway...
Type your messages below. Commands:
  - Type 'exit' or 'quit' to stop
  - Type 'help' for available skills

> Hello!
Bot: Hi! I'm NuimanBot. How can I help you today?

> What's 25 * 4?
Bot: The result is 100.

> exit
NuimanBot stopped gracefully.
```

## Security Features

NuimanBot implements comprehensive security measures to protect against common attack vectors:

### Input Validation
- **Maximum Length Enforcement**: Configurable input length limits (default: 4096 bytes)
- **Null Byte Detection**: Prevents null byte injection attacks
- **UTF-8 Validation**: Ensures all input is valid UTF-8 encoded
- **Prompt Injection Protection**: Detects and blocks 30+ jailbreak patterns including:
  - Instruction override attempts ("ignore previous instructions", "system override")
  - Role manipulation ("you are now", "act as", "from now on")
  - Information disclosure attempts ("show your prompt", "reveal instructions")
  - Output manipulation ("bypass filter", "skip validation")
- **Command Injection Protection**: Detects and blocks 50+ dangerous patterns including:
  - Shell metacharacters (`;`, `&&`, `||`, `` ` ``, `$()`)
  - Dangerous commands (`rm`, `sudo`, `wget`, `curl`, `bash`)
  - Sensitive file paths (`/etc/passwd`, `/bin/bash`, `c:\system32`)

### Credential Management
- **AES-256-GCM Encryption**: All API keys and secrets encrypted at rest
- **Secure Vault**: File-based credential vault with authenticated encryption
- **Environment Variable Support**: Sensitive data can be loaded from environment

### Audit Trail
- Security events logged for monitoring and compliance
- Input validation failures tracked
- Audit interface extensible for custom logging backends

## Built-in Skills

### Calculator
Performs basic arithmetic operations:
- **Operations**: add, subtract, multiply, divide
- **Permissions**: None required
- **Usage**: "What is 5 plus 3?", "Calculate 20 divided by 4"

### DateTime
Provides current date and time information:
- **Operations**:
  - `now` - Current time in RFC3339 format
  - `format` - Custom time formatting
  - `unix` - Unix timestamp
- **Permissions**: None required
- **Usage**: "What time is it?", "Give me the current date"

### Weather
Get current weather and forecasts for any location:
- **Operations**:
  - `current` - Current weather conditions
  - `forecast` - 5-day weather forecast
- **Parameters**: location (required), units (metric/imperial/standard)
- **Permissions**: Network
- **Requirements**: OPENWEATHERMAP_API_KEY environment variable
- **Usage**: "What's the weather in London?", "Give me the forecast for Tokyo"

### Web Search
Perform web searches using DuckDuckGo:
- **Operations**: search
- **Parameters**: query (required), limit (1-50, default: 5)
- **Permissions**: Network
- **Requirements**: None (uses public DuckDuckGo API)
- **Usage**: "Search for golang clean architecture", "Find information about AI agents"

### Notes
Create, read, update, and delete personal notes:
- **Operations**:
  - `create` - Create a new note
  - `read` - Read a note by ID
  - `update` - Update an existing note
  - `delete` - Delete a note
  - `list` - List all notes
- **Parameters**: title, content, tags (optional)
- **Permissions**: Write
- **Storage**: SQLite with user isolation
- **Usage**: "Create a note titled 'Meeting' with content 'Q1 planning session'", "List my notes"

## Development

### Project Structure

```
.
├── cmd/
│   └── nuimanbot/         # Application entry point
├── internal/
│   ├── domain/            # Business entities (no dependencies)
│   ├── usecase/           # Application business logic
│   │   ├── chat/          # Chat service orchestration
│   │   ├── security/      # Security & encryption
│   │   ├── user/          # User management
│   │   ├── notes/         # Notes repository interface
│   │   └── skill/         # Skill execution framework with RBAC
│   ├── adapter/           # Interface adapters
│   │   ├── gateway/       # CLI, Telegram, Slack gateways
│   │   └── repository/    # SQLite repositories (users, messages, notes)
│   └── infrastructure/    # External concerns
│       ├── crypto/        # AES encryption, vault
│       ├── llm/           # LLM provider clients (Anthropic, OpenAI, Ollama)
│       ├── weather/       # OpenWeatherMap client
│       └── search/        # DuckDuckGo search client
├── internal/skills/       # Built-in skills (calculator, datetime, weather, websearch, notes)
│   ├── calculator/
│   └── datetime/
├── config.yaml            # Configuration file
└── data/                  # Runtime data (gitignored)
    ├── nuimanbot.db       # SQLite database
    └── vault.enc          # Encrypted credentials
```

### Running Tests

```bash
# Run all tests (unit + integration + E2E)
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test types
go test ./internal/...            # Unit and integration tests
go test ./e2e/...                 # End-to-end tests
go test ./internal/skills/...     # Skill tests only

# Run specific package tests
go test ./internal/skills/calculator/... -v

# Run with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run E2E tests with verbose output
go test -v ./e2e/...
```

#### Test Types

**Unit Tests** (`*_test.go`)
- Fast, isolated tests for individual functions and methods
- Located in same package as code under test
- Example: `internal/skills/calculator/calculator_test.go`

**Integration Tests** (`*_test.go`)
- Test interactions between multiple components
- Example: `internal/config/loader_test.go` (config loading + file system)

**End-to-End Tests** (`e2e/*_test.go`)
- Test complete application flows from start to finish
- Full application initialization with all layers
- Test scenarios include:
  - Full application lifecycle (startup, operation, shutdown)
  - CLI to skill execution flow
  - Conversation persistence
  - Input validation rejection
  - Configuration loading
  - Graceful shutdown with active requests

**Test Coverage**: ~80% across all layers with comprehensive validation testing

### Quality Gates

All quality gates must pass before committing:

```bash
# Format code
go fmt ./...

# Tidy dependencies
go mod tidy

# Run vet
go vet ./...

# Run linter (requires golangci-lint)
golangci-lint run

# Run tests
go test ./...

# Build
go build -o bin/nuimanbot ./cmd/nuimanbot

# Combined quality check
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot
```

### Test-Driven Development (TDD)

This project follows strict TDD with Red-Green-Refactor cycles:

1. **Red**: Write a failing test first
2. **Green**: Write minimal code to pass the test
3. **Refactor**: Improve code quality while keeping tests green

See `AGENTS.md` for detailed development guidelines.

## Architecture

### Clean Architecture Layers

**Domain Layer** (`internal/domain/`)
- Pure business entities and interfaces
- No external dependencies (only stdlib)
- Defines: User, Message, Skill, LLM interfaces

**Use Case Layer** (`internal/usecase/`)
- Application business logic
- Orchestrates domain entities
- Defines repository/service interfaces
- Implements: ChatService, SkillExecutionService, SecurityService

**Adapter Layer** (`internal/adapter/`)
- Implements interfaces from use case layer
- Converts external data to domain models
- Includes: CLI Gateway, SQLite repositories

**Infrastructure Layer** (`internal/infrastructure/`)
- Concrete implementations for external services
- LLM clients (Anthropic, OpenAI, Ollama)
- Encryption, file I/O

### Dependency Flow

```
Infrastructure → Adapter → Use Case → Domain
      ↓             ↓          ↓          ↑
   External    Interfaces  Business   Entities
   Services                  Logic
```

Dependencies always flow inward. Inner layers define interfaces; outer layers implement them.

## Environment Variables

All configuration can be set via environment variables with the `NUIMANBOT_` prefix:

### Required
- `NUIMANBOT_ENCRYPTION_KEY` - 32-byte encryption key for credential vault

### Server
- `NUIMANBOT_SERVER_LOGLEVEL` - Log level (debug, info, warn, error)
- `NUIMANBOT_SERVER_DEBUG` - Debug mode (true/false)

### Security
- `NUIMANBOT_SECURITY_INPUTMAXLENGTH` - Max input length (default: 4096)
- `NUIMANBOT_SECURITY_VAULTPATH` - Path to encrypted vault file

### LLM Providers
- `NUIMANBOT_LLM_PROVIDERS_0_ID` - Provider ID
- `NUIMANBOT_LLM_PROVIDERS_0_TYPE` - Provider type (anthropic, openai, ollama)
- `NUIMANBOT_LLM_PROVIDERS_0_APIKEY` - API key for the provider

### Storage
- `NUIMANBOT_STORAGE_DSN` - Database connection string

### Skills
- `NUIMANBOT_SKILLS_ENTRIES_CALCULATOR_APIKEY` - API key for calculator skill
- `NUIMANBOT_SKILLS_ENTRIES_DATETIME_APIKEY` - API key for datetime skill

## Creating Custom Skills

Implement the `domain.Skill` interface:

```go
package myskill

import (
    "context"
    "nuimanbot/internal/domain"
)

type MySkill struct {
    config domain.SkillConfig
}

func NewMySkill() *MySkill {
    return &MySkill{
        config: domain.SkillConfig{Enabled: true},
    }
}

func (s *MySkill) Name() string {
    return "myskill"
}

func (s *MySkill) Description() string {
    return "Description of what my skill does"
}

func (s *MySkill) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "param": map[string]any{
                "type": "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"param"},
    }
}

func (s *MySkill) Execute(ctx context.Context, params map[string]any) (*domain.SkillResult, error) {
    // Skill logic here
    return &domain.SkillResult{
        Output: "result",
        Metadata: map[string]any{},
        Error: "",
    }, nil
}

func (s *MySkill) RequiredPermissions() []domain.Permission {
    return []domain.Permission{}
}

func (s *MySkill) Config() domain.SkillConfig {
    return s.config
}
```

Register in `cmd/nuimanbot/main.go`:

```go
import "nuimanbot/internal/skills/myskill"

func registerBuiltInSkills(registry skill.SkillRegistry) error {
    // ... existing skills ...

    myskill := myskill.NewMySkill()
    if err := registry.Register(myskill); err != nil {
        return fmt.Errorf("failed to register myskill: %w", err)
    }

    return nil
}
```

## Contributing

1. Follow Clean Architecture principles
2. Write tests first (TDD)
3. Ensure all quality gates pass
4. Update documentation
5. Follow commit message conventions

See `AGENTS.md` for detailed contribution guidelines.

## License

[Add your license here]

## Status

✅ **Production-Ready MVP** - 75% Complete (33/44 planned features)

### Completed Features ✅

**Phase 1-2: Core Functionality (100%)**
- ✅ Clean Architecture foundation with strict dependency rules
- ✅ Multi-gateway support (CLI, Telegram, Slack)
- ✅ Multi-LLM integration (Anthropic, OpenAI, Ollama)
- ✅ 5 built-in skills with RBAC permissions
- ✅ SQLite storage with migrations
- ✅ Configuration system with environment-aware validation
- ✅ Comprehensive test coverage (85%+ across all packages)

**Phase 3: Production Readiness (100%)**
- ✅ Structured logging with slog
- ✅ Request ID propagation for distributed tracing
- ✅ Error categorization (user/system/external/auth)
- ✅ Secret rotation with versioned vault
- ✅ Health check endpoints (liveness, readiness, version)
- ✅ Graceful shutdown with cleanup
- ✅ Audit logging for security events

**Phase 4: Performance Optimization (100%)**
- ✅ Database connection pooling (25 max open, 5 idle)
- ✅ LLM response caching (1000 entries, 1h TTL, 100% coverage)
- ✅ Message batching (100-message buffer, 5s flush interval)

**Phase 5: Feature Completion (43%)**
- ✅ Conversation summarization (automatic LLM-based compression)
- ✅ Rate limiting (token bucket with per-user/per-skill limits)
- ✅ Token window management (dynamic context sizing per provider)
- ⏳ Streaming response support
- ⏳ Multi-provider fallback
- ⏳ User preferences (model selection, temperature)
- ⏳ Conversation export (JSON, Markdown)

**Phase 6: Observability & Monitoring (20%)**
- ✅ Prometheus metrics (HTTP, LLM, skills, cache, database, security)
- ⏳ Distributed tracing (OpenTelemetry)
- ⏳ Error tracking (Sentry integration)
- ⏳ Real-time alerting
- ⏳ Usage analytics dashboard

### Remaining Work (25%)

**Phase 7: CI/CD & Automation (0%)**
- GitHub Actions pipeline (test, build, security scan)
- Automated deployments
- Release automation

For detailed progress tracking, see `POST_REVIEW_IMPROVEMENT_PLAN.md`.

## Support

- **Issues**: https://github.com/stainedhead/NuimanBot/issues
- **Documentation**: See `AGENTS.md`, `CLAUDE.md`, `PRODUCT_REQUIREMENT_DOC.md`

---

Built with ❤️ using Clean Architecture and Test-Driven Development
