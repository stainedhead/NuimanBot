# NuimanBot

An AI agent framework built with Clean Architecture principles, featuring LLM integration, extensible skill system, and multiple messaging gateway support.

## Features

- **Clean Architecture**: Strict layer separation (Domain, Use Case, Adapter, Infrastructure)
- **LLM Integration**: Support for Anthropic Claude (OpenAI, Ollama planned)
- **Extensible Skills**: Plugin-based skill system with built-in calculator and datetime skills
- **Multiple Gateways**: CLI interface (Telegram, Slack planned)
- **Secure Credentials**: AES-256-GCM encrypted credential vault
- **SQLite Storage**: Persistent conversation history and user data
- **Configuration**: YAML file + environment variable override support
- **Test Coverage**: ~75% coverage with comprehensive unit and integration tests

## Quick Start

### Prerequisites

- Go 1.21 or later
- SQLite3
- Anthropic API key (for LLM functionality)

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
  default_model:
    primary: anthropic/claude-sonnet
  providers:
    - id: anthropic-main
      type: anthropic
      api_key: "your-api-key-here"

gateways:
  cli:
    debug_mode: false

skills:
  entries:
    calculator:
      enabled: true
    datetime:
      enabled: true
```

#### Option 2: Environment Variables

```bash
# Required
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-encryption-key-here"

# LLM Configuration
export NUIMANBOT_LLM_PROVIDERS_0_ID="anthropic-main"
export NUIMANBOT_LLM_PROVIDERS_0_TYPE="anthropic"
export NUIMANBOT_LLM_PROVIDERS_0_APIKEY="your-anthropic-api-key"

# Optional overrides
export NUIMANBOT_SERVER_LOGLEVEL="debug"
export NUIMANBOT_SECURITY_INPUTMAXLENGTH="8192"
```

### Running

```bash
# Ensure encryption key is set
export NUIMANBOT_ENCRYPTION_KEY="12345678901234567890123456789012"

# Set your Anthropic API key
export NUIMANBOT_LLM_PROVIDERS_0_APIKEY="sk-ant-your-key-here"

# Run the application
./bin/nuimanbot
```

The CLI will start and you can interact with the bot:

```
NuimanBot starting...
Config file used: ./config.yaml
2026/02/06 12:00:00 Database schema initialized successfully
2026/02/06 12:00:00 Registered 2 built-in skills
2026/02/06 12:00:00 NuimanBot initialized with:
2026/02/06 12:00:00   Log Level: info
2026/02/06 12:00:00   Debug Mode: false
2026/02/06 12:00:00   LLM Provider: anthropic
2026/02/06 12:00:00   Skills Registered: 2

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

## Built-in Skills

### Calculator
Performs basic arithmetic operations:
- **Operations**: add, subtract, multiply, divide
- **Usage**: "What is 5 plus 3?", "Calculate 20 divided by 4"

### DateTime
Provides current date and time information:
- **Operations**:
  - `now` - Current time in RFC3339 format
  - `format` - Custom time formatting
  - `unix` - Unix timestamp
- **Usage**: "What time is it?", "Give me the current date"

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
│   │   └── skill/         # Skill execution framework
│   ├── adapter/           # Interface adapters
│   │   ├── gateway/       # CLI, Telegram, Slack gateways
│   │   └── repository/    # SQLite repositories
│   └── infrastructure/    # External concerns
│       ├── crypto/        # AES encryption, vault
│       └── llm/           # LLM provider clients
├── internal/skills/       # Built-in skills
│   ├── calculator/
│   └── datetime/
├── config.yaml            # Configuration file
└── data/                  # Runtime data (gitignored)
    ├── nuimanbot.db       # SQLite database
    └── vault.enc          # Encrypted credentials
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/skills/calculator/... -v

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

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

✅ **MVP Complete** - Core features implemented and tested

- ✅ Clean Architecture foundation
- ✅ CLI Gateway
- ✅ Anthropic LLM integration
- ✅ Calculator & DateTime skills
- ✅ SQLite storage
- ✅ Configuration system (file + env vars)
- ✅ Security & encryption
- ✅ Graceful shutdown

**Coming Soon:**
- Additional LLM providers (OpenAI, Ollama)
- Telegram and Slack gateways
- More built-in skills
- MCP (Model Context Protocol) support

For detailed status, see `STATUS.md`.

## Support

- **Issues**: https://github.com/stainedhead/NuimanBot/issues
- **Documentation**: See `AGENTS.md`, `CLAUDE.md`, `PRODUCT_REQUIREMENT_DOC.md`

---

Built with ❤️ using Clean Architecture and Test-Driven Development
