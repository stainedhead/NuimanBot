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
- User-input injection detection: 80+ attack pattern rules (30+ prompt injection, 50+ command injection) applied to direct chat input
- Tool-output injection filtering: a separate `OutputValidator` scans fetched web content, search results, and MCP tool output for injection patterns before it reaches the LLM (fail-closed reject by default)
- Side-effecting action confirmation: default-configured actions (e.g. GitHub PR merge, issue close) require human yes/no confirmation before executing
- Role-based access control (RBAC) with audit logging, enforced end-to-end for every gateway including live chat
- No external tool imports (100% custom, vetted tools)

**🤖 Multi-LLM Support**
- Anthropic Claude, OpenAI GPT, AWS Bedrock, Ollama (local models)
- Automatic failover across providers
- Streaming responses with graceful degradation

**🛠️ Rich Tool Ecosystem**
- 12 built-in tools (5 core + 7 developer productivity) + MCP external tools
- Agent Skills system following [Anthropic standard](https://github.com/anthropics/anthropic-skills)
- MCP (Model Context Protocol) client: HTTP and stdio transports, `mcp:<server>:<tool>` namespace
- Custom tool creation with RBAC enforcement

**💬 Multi-Gateway Support**
- CLI, Telegram, Slack, and Buzz (Nostr-based multi-agent chat) interfaces
- Concurrent operation with unified conversation history
- Per-user customization via persona files

**🧠 Self-Organizing Memory**
- Automatic knowledge extraction from conversations (LLM-based)
- Full-text search (FTS5) with scene-based organization and salience scoring
- Ingatan backend or built-in file-based storage (configurable)
- Graceful degradation (memory failures never block chat)

**📊 Production-Grade**
- Prometheus metrics, distributed tracing, real-time alerting
- Auto-initialization (zero-config first run)
- TLS auto-generation (self-signed ECDSA P-256, no config required)
- CI/CD automation with security scanning
- 72%+ test coverage (unit + integration)

**🗂️ Persistent Agent Workspace (Web UI, New — In Progress)**
- Six new web environments: Chats, Projects, Jobs, Chores, History, Memories, plus Settings
- Durable, directory-scoped Projects with an agent-steerable `AGENTS.md`; one-time Jobs and cron-scheduled Chores run through a shared, configurable FIFO worker pool; full run history with per-user notification badges
- Configurable network exposure: localhost-only (default) or remote with an optional IP/hostname allowlist, enforced pre-authentication
- Strict per-user isolation on every resource (cross-user access by ID returns 404, never 403 — including for admins)
- **Known limitation:** Job/Chore execution and the web Chats environment do not yet invoke the agent — see [Known Limitations](#known-limitations) and the [Web Workspace Guide](support_docs/web-workspace-guide.md)

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
- [Web Workspace Guide](support_docs/web-workspace-guide.md) - Using Chats, Projects, Jobs, Chores, History, and Memories from the web UI
- [Agent Skills](support_docs/skills-guide.md) - Creating and using reusable prompt templates
- [Persona Customization](support_docs/user-onboarding.md#persona-customization) - Per-user AI personality files
- [Self-Organizing Memory](support_docs/self-organizing-memory-guide.md) - Long-term memory system
- [Buzz Gateway](support_docs/buzz-guide.md) - Joining Buzz (Nostr-based multi-agent chat) channels
- [Security Hardening Guide](support_docs/security-hardening-guide.md) - Action confirmations, tool-output scanning, SSRF protection, and RBAC config
- [Advanced Skills: Subagents](support_docs/subagents-guide.md), [Preprocessing](support_docs/preprocessing-guide.md), [Plugins](support_docs/plugins-guide.md), [Versioning](support_docs/versioning-guide.md)

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
- Maintain or improve test coverage (currently 72%+)

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
│  Adapter Layer                      │  Gateways (CLI, Telegram, Slack, Buzz)
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
- ✅ User-input prompt injection → 30+ pattern detection on direct chat input, input sanitization
- ✅ Tool-output prompt injection → `OutputValidator` scans fetched web pages, search results, and MCP tool output for injection patterns before it re-enters the LLM's context; flagged content is rejected by default
- ✅ Command injection → 50+ pattern detection, output sandboxing
- ✅ Malicious tools → Custom tools only, no external imports
- ✅ Supply chain attacks → Minimal dependencies, security scanning
- ✅ Privilege escalation → RBAC roles are defined and enforced per tool via `ExecuteWithUser`/`checkPermission`, backed by a CI guard ensuring every registered tool has an explicit role; the live chat tool-calling loop (both the main tool-calling loop and the confirmation-approval re-invocation path) calls `ExecuteWithUser` directly, resolving each user's persisted `domain.User` via `UserService` (`GetUserByPlatformUID`/`CreateUser`) before enforcing permissions — the side-effecting-action confirmation gate remains an additional, independent safeguard on top of this

**Security Features**
- Two distinct, separately-implemented injection defenses: a ~30-pattern detector on direct user chat input (`internal/usecase/security/input_validation.go`), and a separate `OutputValidator` that scans all tool-sourced content — fetched web/document pages, web-search results, and MCP tool output — before it reaches the LLM (`internal/usecase/security/output_validation.go`)
- Prompt-boundary guardrail: every tool result is wrapped in `<tool_output source="...">` delimiters, and a fixed, non-overridable system-prompt instruction tells the model to treat tool output as data, never as instructions
- Side-effecting action confirmation: plain-text yes/no by default in every chat gateway, with native Slack Block Kit / Telegram inline-keyboard buttons where those integrations support it (not a universal rich UI); unresolved confirmations expire (default 5m) and are treated as denied
- SSRF hardening: fetch tools (`summarize`, `doc_summarize`) resolve and validate the target IP — rejecting loopback, RFC 1918 private ranges, link-local addresses (including the `169.254.169.254` cloud metadata endpoint), and multicast/reserved ranges — before dialing, and re-validate on every redirect hop by dialing the validated IP directly
- MCP tool trust classification: each MCP server (and optionally individual tools) is tagged `read_only` / `write` / `unknown` in `mcp.json`; `write`/`unknown`-trust tools are treated as admin-only and confirmation-required, the same as built-in write tools
- MCP (and all other) tool output passes through both secret redaction (`OutputSanitizer`) and injection-pattern detection (`OutputValidator`) — two distinct mechanisms, neither of which alone covers what the other does
- Encrypted credential vault with key rotation support
- Comprehensive audit logging for compliance, including `injection_flagged`/`matched_patterns` fields on flagged tool calls
- Rate limiting (token bucket algorithm): per-user tool limits, per-IP login limits, per-client REST API limits
- REST API JWT authentication (HS256, minimum 32-byte secret enforced)
- TLS auto-generation for admin web and health servers
- Web admin: forced password change on default credentials, CSRF protection, role middleware

**RBAC enforcement in live chat (fixed)**: the live chat conversation loop (`chat.Service.ProcessMessage`) executes tools via `ToolExecutionService.ExecuteWithUser` — both the main tool-calling loop and the confirmation-approval re-invocation path call this method, which runs the role-based RBAC check (`checkPermission`). Each incoming message's role-bearing `*domain.User` is resolved via `UserService` (`GetUserByPlatformUID`/`CreateUser`), defaulting new non-CLI identities to `RoleGuest` and CLI to `RoleAdmin`; an unresolvable or unregistered platform identity fails closed to `domain.RoleGuest` rather than failing open. The tool list offered to the LLM is also role-filtered (`ListTools` now filters `registry.List()` by the resolved caller's role, rather than returning every registered tool). Role-based restrictions such as `coding_agent`/`github` writes being admin-only are defined in code, unit-tested, CI-guarded, and enforced end-to-end for a live conversation — see the regression test `TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge` in `internal/usecase/chat/rbac_test.go`, which proves a non-admin chat user's attempt to invoke `github.pr_merge` is denied at the RBAC layer. The side-effecting-action confirmation gate described above remains independently wired and is fully live for real conversations as an additional layer on top of this.

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

### MCP External Tools (dynamic)
- Any tool exposed by an MCP-compatible server defined in `mcp.json`
- Registered at startup under the namespace `mcp:<server>:<tool>`
- HTTP and stdio transports; bad servers skipped with logged warning
- 30-second per-tool timeout; output passes through secret redaction (`OutputSanitizer`) and injection-pattern detection (`OutputValidator`)
- Each server (and optionally individual tools) carries a trust classification (`read_only`/`write`/`unknown`); `write`/`unknown`-trust tools are treated as admin-only and confirmation-required

All built-in tools have an assigned RBAC role, rate limiting, timeout enforcement, and output sanitization (secret redaction + injection-pattern filtering); see [Security](#security) for how role enforcement is wired into live chat via `ExecuteWithUser`.

See [User Onboarding Guide](support_docs/user-onboarding.md) for usage examples.

---

## Known Limitations

**Persistent Agent Workspace (Chats, Projects, Jobs, Chores, History, Memories, Settings) — new, 2026-08-05, hardened by a review-fix pass the same day, functional but partial:**
- Job/Chore execution runs through a real queueing/scheduling/persistence pipeline but does not yet invoke the agent — it uses a placeholder `StubExecutor` that produces no real work product
- The web Chats environment persists messages but does not yet generate agent replies (the CLI/Telegram/Slack/Buzz gateways are unaffected)
- Per-Job/Chore/Run "chat with the agent" interfaces are not built yet; Memories now has one (a minimal per-item Q&A), as the reference implementation the other three are meant to follow
- Configured retention windows (Chats/Projects/History) are now automatically enforced by a 15-minute sweep, and a soft-deleted Job or Chore is automatically cleaned up once its run finishes
- The WebSocket push transport for live run status/log/notification updates now has a browser-side consumer (`run-events.js`) — Job/Chore/Run detail pages update live without a manual refresh
- A run already dispatched to a worker at the moment of a server crash is now reconciled on restart (marked `Failed` rather than left silently stuck)

See [Product Summary](documentation/product-summary.md#persistent-agent-workspace-chats-projects-jobs-chores-history-memories--in-progress) for the complete, itemized status and the [Web Workspace Guide](support_docs/web-workspace-guide.md) for what to expect when using it today.

---

## Status

✅ **Production Ready** - 100% Complete

**Recently Completed**
- 🔶 Persistent Agent Workspace (web UI): Chats, Projects, Jobs, Chores, History, Memories, Settings, configurable network access — real queueing/scheduling/persistence pipeline with a scheduled retention sweep, restart recovery, and live browser-side WebSocket updates; agent-invoking execution still pending (2026-08-05, hardened same day by a review-fix pass; see [Known Limitations](#known-limitations))
- ✅ Buzz Gateway (Nostr-based multi-agent chat: relay client, RBAC, tool integration; cross-platform RBAC enforcement fix) (2026-08-02)
- ✅ Security Hardening — Parts A-G: tool-output injection filtering (`OutputValidator`), prompt-boundary guardrails, side-effecting action confirmation flow, RBAC correction with CI guard, SSRF hardening (IP-resolution + redirect revalidation), MCP trust classification, documentation parity (2026-08-02)
- ✅ FR-001/FR-002 — RBAC bypass in live chat fixed (P0), reconciled with the Buzz gateway's independent cross-platform RBAC fix onto one unified model: `chat.Service` resolves each user's persisted `domain.User` via `UserService` (`GetUserByPlatformUID`/`CreateUser`, defaulting new non-CLI identities to `RoleGuest`) and calls `ExecuteWithUser` — with `conversationID` threaded through for Part C's confirmation gate — for both the main tool-calling loop and the confirmation-approval re-invocation path; `ListTools` filters by caller role, including the action-aware `github` split and MCP trust classification; confirmation-reply detection is keyed on the resolved user's persisted ID, not raw platform UID (2026-08-04)
- ✅ MCP Client (HTTP + stdio, JSON-RPC 2.0, startup wiring) (2026-03-22)
- ✅ REST API Security (JWT, per-client rate limiting, body-size limit) (2026-03-22)
- ✅ Web Admin Security (role middleware, login rate limiter, forced password change) (2026-03-22)
- ✅ TLS Auto-Generation (self-signed ECDSA P-256 cert, StartTLS) (2026-03-22)
- ✅ Ingatan Memory Backend (JWT token exchange, graceful degradation fallback) (2026-03-22)
- ✅ Integration Test Suite (//go:build integration tagged tests) (2026-03-22)
- ✅ Auto-Initialization (2026-02-16)
- ✅ Self-Organizing Memory v2 (2026-02-15)
- ✅ Persona Customization System (2026-02-15)

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
