# NuimanBot

<div align="center">
  <img src="./img/NuimanWebImage.jpg" alt="NuimanBot">
  <p><strong>A security-hardened AI agent framework built with Clean Architecture</strong></p>
  <p>
    <a href="#quick-start">Quick Start</a> •
    <a href="#documentation">Documentation</a> •
    <a href="#development">Development</a> •
    <a href="#contributing">Contributing</a>
  </p>
</div>

## Overview

NuimanBot is a production-ready AI agent framework that prioritizes **security**, **extensibility**, and **clean architecture**. Unlike existing frameworks with security vulnerabilities, NuimanBot provides enterprise-grade protection against injection attacks, credential leakage, and supply chain risks.

### Key Features

**🔒 Security First**
- Zero credential leakage with AES-256-GCM encryption
- 80+ attack pattern detection (prompt injection, command injection)
- Role-based access control (RBAC) with audit logging
- No external tool imports (100% custom, vetted tools)

**🤖 Multi-LLM Support**
- Anthropic Claude, OpenAI GPT, AWS Bedrock, Ollama (local models)
- Automatic failover across providers
- Streaming responses with graceful degradation

**🛠️ Rich Tool Ecosystem**
- 12 built-in tools (5 core + 7 developer productivity)
- Agent Skills system following [Anthropic standard](https://github.com/anthropics/anthropic-skills)
- Custom tool creation with RBAC enforcement

**💬 Multi-Gateway Support**
- CLI, Telegram, and Slack interfaces
- Concurrent operation with unified conversation history
- Per-user customization via persona files

**🧠 Self-Organizing Memory**
- Automatic knowledge extraction from conversations
- Full-text search with scene-based organization
- Graceful degradation (memory failures never block chat)

**📊 Production-Grade**
- Prometheus metrics, distributed tracing, real-time alerting
- Auto-initialization (zero-config first run)
- CI/CD automation with security scanning
- 85%+ test coverage

---

## Quick Start

### For Adopters: Install and Run

**Prerequisites**
- Go 1.24+ (or download pre-built binary)
- One LLM provider API key (Anthropic, OpenAI, AWS Bedrock, or Ollama for local)

**Installation**

```bash
# Clone the repository
git clone https://github.com/stainedhead/NuimanBot.git
cd NuimanBot

# Build
go build -o bin/nuimanbot ./cmd/nuimanbot

# Run (auto-generates encryption key, creates directories, sets up admin user)
./bin/nuimanbot
```

**First Run**
- 🔐 Encryption key auto-generated and saved to `.env`
- 📁 Data directories created automatically
- 👤 Default admin user created

**Configuration**

Choose your LLM provider via environment variables:

```bash
# Required: Encryption key (auto-generated on first run)
export NUIMANBOT_ENCRYPTION_KEY="your-32-byte-key"

# Option A: Anthropic Claude
export NUIMANBOT_LLM_ANTHROPIC_APIKEY="sk-ant-your-key"

# Option B: OpenAI GPT
export NUIMANBOT_LLM_OPENAI_APIKEY="sk-your-openai-key"

# Option C: AWS Bedrock
export AWS_REGION="us-east-1"
export AWS_PROFILE="your-profile"  # optional

# Option D: Ollama (local models)
export NUIMANBOT_LLM_OLLAMA_BASEURL="http://localhost:11434"

# Run
./bin/nuimanbot
```

**📖 Detailed Guides**
- [Installation & Setup Guide](support_docs/install-and-setup.md) - Complete installation instructions
- [User Onboarding Guide](support_docs/user-onboarding.md) - How to use NuimanBot features
- [Configuration Reference](support_docs/configuration-reference.md) - All configuration options

---

## Documentation

### For Users & Administrators

**Getting Started**
- [Installation & Setup](support_docs/install-and-setup.md) - System installation and configuration
- [User Onboarding](support_docs/user-onboarding.md) - Feature overview and usage
- [CLI Administration](support_docs/cli-admin-guide.md) - User and permission management

**Feature Guides**
- [Agent Skills](support_docs/skills-guide.md) - Creating and using reusable prompt templates
- [Persona Customization](support_docs/user-onboarding.md#persona-customization) - Per-user AI personality files
- [Self-Organizing Memory](support_docs/self-organizing-memory-guide.md) - Long-term memory system
- [Advanced Skills](support_docs/README.md) - Subagents, preprocessing, plugins, versioning

**Administration**
- [Admin Guide](support_docs/admin-guide.md) - REST API, user/bot management, operations
- [Memory Admin Guide](support_docs/admin-guide-memory.md) - Memory system monitoring and troubleshooting
- [API Reference](support_docs/api-reference.md) - REST API endpoints
- [Configuration Reference](support_docs/configuration-reference.md) - Config file and environment variables

### For Developers

**Architecture & Design**
- [Product Summary](documentation/product-summary.md) - Executive overview and status
- [Product Details](documentation/product-details.md) - Requirements, workflows, constraints
- [Technical Details](documentation/technical-details.md) - Architecture, system design, API documentation
- [Development Guidelines](AGENTS.md) - Clean Architecture, TDD methodology, quality gates

---

## Development

### For Developers: Setup and Build

**Prerequisites**
- Go 1.24 or later
- golangci-lint (for linting)
- Git

**Clone and Build**

```bash
# Clone repository
git clone https://github.com/stainedhead/NuimanBot.git
cd NuimanBot

# Install dependencies
go mod download

# Build
go build -o bin/nuimanbot ./cmd/nuimanbot

# Run tests
go test ./...

# Run with race detection
go test -race ./...
```

**Project Structure**

```
internal/
├── domain/            # Entities and business rules (zero dependencies)
├── usecase/           # Application business logic
├── adapter/           # Interface adapters (CLI, gateways, repositories)
└── infrastructure/    # External services (LLM clients, encryption, storage)
```

**Quality Gates**

All gates must pass before committing:

```bash
go fmt ./...                    # Format code
go mod tidy                     # Tidy dependencies
go vet ./...                    # Check for suspicious constructs
golangci-lint run               # Run linter
go test ./...                   # Run all tests
go build -o bin/nuimanbot ./cmd/nuimanbot  # Build executable
```

**Development Methodology**

This project follows **Test-Driven Development (TDD)** with strict Red-Green-Refactor cycles:

1. **Red**: Write a failing test
2. **Green**: Write minimal code to pass
3. **Refactor**: Improve code quality (mandatory)

See [AGENTS.md](AGENTS.md) for detailed development guidelines.

---

## Contributing

### For Collaborators: How to Contribute

We welcome contributions! Please follow these guidelines:

**1. Development Standards**
- Follow [Clean Architecture](AGENTS.md#clean-architecture) principles
- Write tests first (TDD)
- Ensure all quality gates pass
- Maintain 85%+ test coverage

**2. Contribution Process**
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests and implementation
4. Run quality gates (`go fmt`, `go vet`, `golangci-lint`, `go test`)
5. Commit with descriptive messages
6. Push to your fork and submit a Pull Request

**3. Commit Message Format**
```
type(scope): brief description

Detailed explanation of changes.

Co-Authored-By: Your Name <your.email@example.com>
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`

**4. Testing Requirements**
- Unit tests for all new functions
- Integration tests for multi-component features
- E2E tests for user-facing features
- Maintain or improve coverage percentage

**5. Documentation**
- Update `support_docs/` for user-facing features
- Update `documentation/` for architecture changes
- Update README.md if adding major features
- Keep AGENTS.md current for development changes

**Where to Contribute**

- 🐛 **Bug Fixes**: Check [issues](https://github.com/stainedhead/NuimanBot/issues) labeled `bug`
- ✨ **Features**: Review [feature requests](https://github.com/stainedhead/NuimanBot/issues?q=is%3Aissue+is%3Aopen+label%3Aenhancement)
- 📖 **Documentation**: Improvements to guides and examples always welcome
- 🧪 **Tests**: Increase coverage or add edge case tests

**Resources**
- [Development Guidelines](AGENTS.md) - Complete development rules
- [Technical Details](documentation/technical-details.md) - Architecture deep-dive
- [Product Details](documentation/product-details.md) - Requirements and workflows

---

## Architecture

NuimanBot follows **Clean Architecture** with strict dependency rules:

```
┌─────────────────────────────────────┐
│  Infrastructure Layer               │  External services (LLM clients, encryption)
│  (Outermost)                        │
└────────────┬────────────────────────┘
             │ implements interfaces
┌────────────▼────────────────────────┐
│  Adapter Layer                      │  Gateways (CLI, Telegram, Slack)
│                                     │  Repositories (file-based storage)
└────────────┬────────────────────────┘
             │ implements interfaces
┌────────────▼────────────────────────┐
│  Use Case Layer                     │  Business logic orchestration
│                                     │  (ChatService, ToolService, SecurityService)
└────────────┬────────────────────────┘
             │ uses entities
┌────────────▼────────────────────────┐
│  Domain Layer                       │  Pure entities, interfaces, business rules
│  (Innermost - Zero dependencies)   │  (User, Message, Tool, LLM interfaces)
└─────────────────────────────────────┘
```

**Dependency Rule**: Dependencies flow inward only. Inner layers define interfaces; outer layers implement them.

See [Technical Details](documentation/technical-details.md) for comprehensive architecture documentation.

---

## Security

NuimanBot addresses critical vulnerabilities found in similar AI agent frameworks:

**Threat Protection**
- ✅ Credential leakage → AES-256-GCM encryption, no plaintext secrets
- ✅ Prompt injection → 30+ pattern detection, input sanitization
- ✅ Command injection → 50+ pattern detection, output sandboxing
- ✅ Malicious tools → Custom tools only, no external imports
- ✅ Supply chain attacks → Minimal dependencies, security scanning
- ✅ Privilege escalation → Strict RBAC enforcement at all layers

**Security Features**
- Input validation with comprehensive attack pattern detection
- Encrypted credential vault with key rotation support
- Comprehensive audit logging for compliance
- Rate limiting (token bucket algorithm)
- API authentication with bearer tokens

See [Product Details](documentation/product-details.md#security-requirements) for complete security documentation.

---

## Built-in Tools

### Core Tools (5)
- **calculator** - Arithmetic operations
- **datetime** - Time and date information
- **weather** - Weather forecasts (OpenWeatherMap)
- **websearch** - Web search (DuckDuckGo)
- **notes** - Personal note management

### Developer Productivity Tools (7)
- **github** - GitHub operations via `gh` CLI
- **repo_search** - Codebase search with ripgrep
- **doc_summarize** - LLM-powered document summarization
- **summarize** - Web page and YouTube video summarization
- **coding_agent** - Orchestrate external coding CLIs (admin-only)
- **executor** - Tool execution engine
- **common** - Shared utilities

All tools include RBAC enforcement, rate limiting, timeout enforcement, and comprehensive testing (85%+ coverage).

See [User Onboarding Guide](support_docs/user-onboarding.md) for usage examples.

---

## Status

✅ **Production Ready** - 95.6% Complete (43/45 features)

**Recently Completed**
- ✅ Auto-Initialization (2026-02-16)
- ✅ Self-Organizing Memory v2 (2026-02-15)
- ✅ Persona Customization System (2026-02-15)
- ✅ CI/CD Pipeline with Security Scanning (2026-02-07)
- ✅ Agent Skills Phase 3: Advanced Features (2026-02-07)

**On Hold** (not required for production)
- ⏸️ Docker image build & push
- ⏸️ Kubernetes deployment manifests

See [Product Summary](documentation/product-summary.md) for detailed status and roadmap.

---

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

Copyright 2026 NuimanBot Contributors

---

## Support

- **Issues**: [GitHub Issues](https://github.com/stainedhead/NuimanBot/issues)
- **Documentation**: See [Documentation](#documentation) section above
- **Discussions**: [GitHub Discussions](https://github.com/stainedhead/NuimanBot/discussions)

---

<div align="center">
  <p>Built using Clean Architecture and Test-Driven Development</p>
  <p>
    <a href="#overview">Back to Top</a>
  </p>
</div>
