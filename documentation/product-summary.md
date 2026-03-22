# NuimanBot Product Summary

**Version:** 1.1
**Last Updated:** 2026-03-22
**Status:** Production Ready (100% Complete)
**CI/CD Status:** ✅ All Pipelines Passing

---

## Executive Overview

NuimanBot is a **security-hardened personal AI agent** built in Go, designed as a secure alternative to existing AI agent frameworks. The project addresses critical security vulnerabilities found in similar platforms (26% of community tools contain security issues including credential leakage, prompt injection enabling RCE, and supply chain attacks) while providing enterprise-grade functionality.

### Current Status

**Production-Ready** - 100% Complete

- ✅ Core functionality complete
- ✅ Comprehensive security hardening (TLS, JWT, rate limiting, RBAC)
- ✅ Multi-platform support (CLI, Telegram, Slack)
- ✅ Multi-LLM integration (Anthropic, OpenAI, AWS Bedrock, Ollama)
- ✅ Self-organizing memory with Ingatan backend support
- ✅ MCP (Model Context Protocol) client for external tool integration
- ✅ Full observability stack
- ✅ CI/CD automation with security scanning
- ✅ Integration test suite (tagged //go:build integration)

---

## Key Differentiators

### 1. Security-First Design

**Problem:** Research shows existing AI agent frameworks have critical vulnerabilities:
- Plaintext API key storage
- External tool imports with unvetted code
- Prompt injection vectors leading to RCE
- Supply chain compromise risks

**NuimanBot Solution:**
- ✅ **Zero credential leakage**: AES-256-GCM encryption at rest
- ✅ **100% tool security**: Custom tools only, no external imports
- ✅ **Input sanitization**: 80+ attack pattern detection rules
- ✅ **Comprehensive audit logging**: All security events tracked
- ✅ **RBAC enforcement**: Role-based access control throughout

### 2. Multi-Platform Support

- **CLI Gateway**: Interactive REPL for development and admin tasks
- **Telegram Gateway**: Long-polling and webhook support with user allowlists
- **Slack Gateway**: Socket Mode (no public endpoint required)

All gateways support concurrent operation with unified conversation history.

### 3. Multi-LLM Provider Integration

**Provider Abstraction Layer** enables:
- Anthropic Claude (Opus, Sonnet, Haiku)
- OpenAI GPT (GPT-4, GPT-3.5)
- AWS Bedrock (Claude 3/3.5 family via Converse API)
- Ollama (local models: Llama, Mistral, etc.)
- **Multi-provider fallback**: Automatic failover for high availability
- **Streaming support**: Real-time token-by-token responses

### 4. Agent Skills System

**Reusable prompt templates** following [Anthropic Agent Skills](https://github.com/anthropics/anthropic-skills) open standard:

- ✅ **YAML frontmatter parsing**: name, description, allowed-tools, invocability
- ✅ **Argument substitution**: `$ARGUMENTS`, `$0`, `$1`, ... placeholders
- ✅ **Priority resolution**: Enterprise > User > Project > Plugin
- ✅ **Multi-user storage**: Shared and per-user skill directories
- ✅ **CLI integration**: `/help`, `/describe`, `/skill-name` commands
- ✅ **5 production-ready skills**: code-review, debugging, api-docs, refactoring, testing

**See**: [Agent Skills User Guide](../support_docs/skills-guide.md)

### 5. Production-Grade Features

**Performance Optimizations:**
- Database connection pooling (25 max open, 5 idle)
- LLM response caching (1000 entries, 1h TTL, SHA256 hashing)
- Message batching (100-message buffer, dual flush strategy)

**Observability Stack:**
- Prometheus metrics (14+ metric types)
- Distributed tracing (OpenTelemetry-style spans)
- Error tracking with structured context
- Real-time alerting (multi-channel with throttling)
- Usage analytics with event batching

**Data Management:**
- Conversation summarization (automatic LLM-based compression)
- Token window management (provider-aware limits: 200k Claude, 128k GPT-4)
- Conversation export (JSON, Markdown formats)
- User preferences (model selection, temperature, context windows)

---

## Architecture Principles

### Clean Architecture

**Strict dependency rules** with inward-only flow:

```
Infrastructure → Adapter → Use Case → Domain
    ↓             ↓          ↓         ↑
 External     Interfaces  Business  Entities
 Services                  Logic
```

- **Domain Layer**: Pure entities (User, Message, Tool) with zero external dependencies
- **Use Case Layer**: Business logic orchestration (Chat, Tool Execution, Security)
- **Adapter Layer**: Gateway implementations (CLI, Telegram, Slack) and repositories (SQLite)
- **Infrastructure Layer**: External service clients (LLM providers, encryption, APIs)

### Test-Driven Development

- **72%+ test coverage** across all layers (unit + integration)
- **TDD methodology**: Strict Red-Green-Refactor cycles
- **Race detection**: All tests pass with `-race` flag
- **Comprehensive testing**: Unit, integration (//go:build integration), and E2E tests

---

## Use Cases

### Personal AI Assistant

- Multi-platform access (desktop CLI, mobile Telegram, team Slack)
- Context-aware conversations with long-term memory
- Built-in tools: calculator, datetime, weather, web search, notes
- Secure credential storage for API integrations

### Team Automation

- Role-based access control (Admin, User roles)
- Tool allowlists per user
- Audit logging for compliance
- Rate limiting (per-user, per-tool)

### Developer Productivity

**Tools:**
- GitHub operations (issues, PRs, workflows via `gh` CLI)
- Codebase search (ripgrep integration with regex support)
- Document summarization (LLM-powered, files and URLs)
- Web/YouTube summarization (content extraction and analysis)
- Coding agent orchestration (Codex, Claude Code, OpenCode, etc.)

**Infrastructure:**
- CLI-first design for automation scripts
- OpenAI-compatible API endpoint
- RESTful management API (JWT-protected, rate-limited)
- MCP client: plug in any MCP-compatible server via `mcp.json`
- Extensible tool system with comprehensive testing (72%+ coverage)

---

## Built-in Tools

### Core Tools (5 - Infrastructure Layer)

| Tool | Description | Permissions | Status |
|-------|-------------|-------------|--------|
| **calculator** | Basic arithmetic operations | None | ✅ |
| **datetime** | Current time, formatting, timezones | None | ✅ |
| **weather** | Current weather and forecasts | Network | ✅ |
| **websearch** | DuckDuckGo web search | Network | ✅ |
| **notes** | CRUD operations for personal notes | Write | ✅ |

### Developer Productivity Tools (7 - Use Case Layer)

| Tool | Description | Permissions | Coverage | Status |
|-------|-------------|-------------|----------|--------|
| **github** | GitHub operations via `gh` CLI (issues, PRs, workflows) | Network, Shell | 95.0% | ✅ |
| **repo_search** | Fast codebase search using ripgrep | Read | 82.5% | ✅ |
| **doc_summarize** | LLM-powered doc summarization (files, URLs) | Read, Network | 50.5% | ✅ |
| **summarize** | Web page and YouTube video summarization | Network | 76.3% | ✅ |
| **coding_agent** | Orchestrate external coding CLIs (admin-only) | Shell | 85.4% | ✅ |
| **executor** | Tool execution engine and orchestration | Internal | 90%+ | ✅ |
| **common** | Shared utilities (rate limiting, validation, sanitization) | Internal | 95%+ | ✅ |

### MCP External Tools (Dynamic)

| Tool | Description | Permissions | Status |
|-------|-------------|-------------|--------|
| **mcp:\<server\>:\<tool\>** | Any tool from a configured MCP server | Network | ✅ |

MCP tools are loaded at startup from `mcp.json`. Servers that fail to initialize are skipped with a logged warning; the bot continues with remaining tools.

**Total Built-in Tools: 12** (5 infrastructure + 7 use case) + dynamic MCP tools

**Security Controls (All Tools):**
- Custom-built (no external imports for built-in tools)
- RBAC enforcement (permission-gated)
- Per-tool rate limiting
- Timeout enforcement (30s default; 30s hard limit for MCP tools)
- Output sanitization (secret redaction; MCP output sanitized before LLM injection)
- Path traversal prevention
- Comprehensive audit logging

---

## Deployment Options

### Current (MVP)

- **Single-server deployment**: file-based JSON storage
- **Concurrent users**: ~100
- **Messages/sec**: 50-100 (with batching)

### Scalability Path (Post-MVP)

- **Horizontal scaling**: Multiple instances with PostgreSQL
- **Distributed caching**: Redis for shared LLM cache
- **Multi-region**: Provider-aware routing

---

## Technology Stack

**Core:**
- Language: Go 1.24
- Database: File-based JSON storage (PostgreSQL-ready)
- Encryption: AES-256-GCM
- Logging: slog (stdlib)

**External Integrations:**
- LLM: Anthropic SDK, OpenAI SDK, AWS Bedrock SDK, Ollama HTTP API
- Gateways: go-telegram/bot, slack-go/slack
- Memory: Ingatan REST API (optional; falls back to file-based storage)
- MCP: HTTP and stdio transports (JSON-RPC 2.0, protocol version 2024-11-05)
- Monitoring: Prometheus client_golang

**CI/CD:**
- GitHub Actions (CI/CD Pipeline, Security Scanning, Deployment)
- golangci-lint (pragmatic configuration)
- gosec + Trivy (security scanning with SARIF)
- Codecov (coverage tracking)
- Race detection enabled

---

## Security Architecture

### Threat Model

| Threat | Mitigation |
|--------|-----------|
| **Credential leakage** | AES-256-GCM encryption at rest; no plaintext secrets |
| **Prompt injection** | 30+ pattern detection; input sanitization; MCP output sanitized before LLM injection |
| **Command injection** | 50+ pattern detection; output sandboxing |
| **Malicious tools** | Custom tools only; no external imports; MCP output sanitized |
| **Session hijacking** | Token rotation; secure credential vault; TLS enforced Secure cookies |
| **Privilege escalation** | Strict RBAC enforcement; `requireRole` middleware on web admin |
| **Supply chain attacks** | Minimal dependencies; audit logging |
| **Brute-force login** | Per-IP token bucket rate limiter on web admin login |
| **Weak API secrets** | JWT secret minimum 32 bytes enforced at server construction |
| **Insecure transport** | TLS auto-generated (ECDSA P-256) for admin web and health servers |
| **Default credentials** | Forced password change on first login with default admin credentials |

### Input Validation

- Maximum input length: 32KB (configurable)
- Null byte detection
- UTF-8 validation
- Prompt injection pattern matching (30+ patterns)
- Command injection pattern matching (50+ patterns)
- Parameterized queries (no raw SQL)

---

## Development Status

### Completed Phases (100%)

- ✅ **Phase 1**: Critical Fixes (Anthropic client, tool calling, DB schema)
- ✅ **Phase 2**: Test Coverage (85%+ across all packages, 2,979 lines of tests)
- ✅ **Phase 3**: Production Readiness (logging, health checks, graceful shutdown)
- ✅ **Phase 4**: Performance Optimization (connection pooling, caching, batching)
- ✅ **Phase 5**: Feature Completion (streaming, fallback, preferences, export, summarization)
- ✅ **Phase 6**: Observability (metrics, tracing, error tracking, alerting, analytics)
- ✅ **Phase 7.1**: CI/CD Pipeline (automated quality gates, security scanning)
- ✅ **Agent Skills**: Complete implementation (Phases 0-2 + Phase 3)
  - **Phase 0-2 (Basic System)**:
    - Domain layer, infrastructure (parser, repository)
    - Use case layer (registry with priority resolution, renderer with argument substitution)
    - Adapter layer (CLI command handler, gateway integration)
    - Configuration system with multi-user storage
    - E2E integration with chat orchestrator
    - 5 production-ready example skills
    - 90%+ test coverage across all layers
  - **Phase 3A - Subagent Execution** (6 tasks, 34h):
    - Context forking with deep copy isolation
    - Autonomous multi-step execution with LLM orchestration
    - Resource limits (tokens, tool calls, timeout)
    - Thread-safe lifecycle management
    - Background execution with status monitoring
  - **Phase 3B - Preprocessing** (5 tasks, 18h):
    - Command blocks with !command syntax
    - Sandboxed shell execution (5s timeout, 10KB limit)
    - Whitelist-only commands (git, gh, ls, cat, grep)
    - Shell metacharacter blocking
    - Real-time data substitution
  - **Phase 3C - Plugin System** (6 tasks, 10h):
    - Plugin discovery and namespace management (org/skill-name)
    - Dependency resolution with semantic versioning
    - Security validation (collision detection, reserved words)
    - Lifecycle management (install, uninstall, enable, disable)
  - **Phase 3D - Skill Versioning** (4 tasks, 4h):
    - Semantic version parsing and comparison
    - Version constraint resolution (^, ~, =)
    - Compatibility checking
    - Latest version resolution
  - **Phase 3E - Persistent Memory** (4 tasks, 4h):
    - SQLite storage with multiple scopes (skill/user/global/session)
    - JSON value serialization
    - TTL and expiration support
    - Automatic cleanup
  - **Phase 3 Total**: 25 tasks, 40 files, 91 tests, 70 hours (77% faster than estimate)

### Post-MVP Security & Integration Phases (100% Complete)

- ✅ **IMS Phase 1**: Ingatan Memory Backend
  - IngatanHTTPClient with JWT token exchange and transparent refresh
  - IngatanMemoryCellRepository and IngatanMemorySceneRepository
  - memory_factory.go backend selector (builtin / ingatan)
  - Graceful degradation fallback on startup health check failure
- ✅ **IMS Phase 2**: TLS Auto-Generation
  - LoadOrGenerateCert in crypto package (ECDSA P-256, 365-day validity)
  - StartTLS on health server and web admin server
  - Secure cookie enforcement when TLS is active
- ✅ **IMS Phase 3**: Web Admin Security
  - requireRole middleware for role-based page access
  - Per-IP login rate limiter (token bucket)
  - Input sanitization on all form fields
  - Forced password change on default credentials
- ✅ **IMS Phase 4**: REST API Security
  - POST /api/v1/auth/token with JWT issuance (HS256)
  - JWT middleware on all protected routes
  - Per-client rate limiting (token bucket, keyed by JWT subject)
  - 1 MiB body-size limit; injection-pattern validation on JSON string fields
- ✅ **IMS Phase 5+6**: MCP Client
  - mcp.json loader; HTTP and stdio transports; JSON-RPC 2.0
  - MCPToolAdapter with mcp:\<server\>:\<tool\> namespace
  - Startup wiring with bad-server skip; 30s per-tool timeout
  - Output sanitized via OutputSanitizer before LLM injection
- ✅ **IMS Phase 7**: Integration Test Suite
  - //go:build integration tagged tests across storage, web, memoryv2
- ✅ **Post-review fixes**: 17 hardening items (token validation, goroutine leak fix, JWT secret strength enforcement, rate limiter eviction, MCP timeout, JSON array input validation, store prefix validation)

---

## Next Steps

The project is **100% complete**. All planned phases have been implemented and verified. Key achievements:

1. ✅ All core functionality implemented and tested
2. ✅ Security hardening complete (TLS, JWT, RBAC, rate limiting, default-credential detection)
3. ✅ Full observability stack operational
4. ✅ CI/CD automation with all pipelines passing
5. ✅ MCP client for external tool integration
6. ✅ Ingatan memory backend with graceful fallback
7. ✅ Integration test suite
8. ✅ Comprehensive documentation maintained

Future optional enhancements (Docker/Kubernetes packaging, linting cleanup) can be picked up as needed but are not required for production operation.

---

## Documentation

| Document | Purpose |
|----------|---------|
| `README.md` | Quick start, installation, usage examples |
| `support_docs/user-onboarding.md` | **User guide** - how to use NuimanBot and customize your experience |
| `support_docs/install-and-setup.md` | Installation and system configuration |
| `support_docs/cli-admin-guide.md` | CLI administration - user management and permissions |
| `support_docs/skills-guide.md` | **Agent Skills user guide** - creating and using skills |
| `documentation/product-summary.md` | This document - executive overview |
| `documentation/product-details.md` | Detailed requirements, workflows, constraints |
| `documentation/technical-details.md` | Architecture, system design, API documentation |
| `AGENTS.md` | Development guidelines for AI agents |
| `POST_REVIEW_IMPROVEMENT_PLAN.md` | Implementation progress tracking |

---

## Support & Resources

- **Repository**: https://github.com/stainedhead/NuimanBot
- **Issues**: https://github.com/stainedhead/NuimanBot/issues
- **CI/CD**: GitHub Actions (all workflows passing)
- **Security**: Automated scanning with gosec + Trivy

---

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../LICENSE) file for details.

Copyright 2026 NuimanBot Contributors

---

**Built using Clean Architecture and Test-Driven Development**
