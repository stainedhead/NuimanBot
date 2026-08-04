# NuimanBot Product Summary

**Version:** 1.2
**Last Updated:** 2026-08-02
**Status:** Production Ready (100% Complete)
**CI/CD Status:** ✅ All Pipelines Passing

---

## Executive Overview

NuimanBot is a **security-hardened personal AI agent** built in Go, designed as a secure alternative to existing AI agent frameworks. The project addresses critical security vulnerabilities found in similar platforms (26% of community tools contain security issues including credential leakage, prompt injection enabling RCE, and supply chain attacks) while providing enterprise-grade functionality.

### Current Status

**Production-Ready** - 100% Complete

- ✅ Core functionality complete
- ✅ Comprehensive security hardening (TLS, JWT, rate limiting, RBAC, tool-output injection filtering, side-effecting action confirmation, SSRF hardening, MCP trust classification)
- ✅ Multi-platform support (CLI, Telegram, Slack, Buzz)
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
- ✅ **User-input injection detection**: 80+ attack pattern detection rules (30+ prompt injection, 50+ command injection) applied to direct chat input
- ✅ **Tool-output injection filtering**: a separate `OutputValidator` scans all tool-sourced content (fetched web pages, search results, MCP output) for injection patterns before it reaches the LLM — a distinct mechanism from the input-side detector above, not the same coverage
- ✅ **Side-effecting action confirmation**: default-configured actions (e.g. `github.pr_merge`, `coding_agent.yolo_mode`) require human yes/no confirmation before executing
- ✅ **Comprehensive audit logging**: All security events tracked, including injection-flagged tool calls
- ✅ **RBAC enforcement**: role-based access control is defined and enforced per tool (`ExecuteWithUser`/`checkPermission`, CI-guarded); the live chat tool-calling loop calls that enforcement path at both of its tool-execution sites, resolving each user's persisted `domain.User` via `UserService` — see [Security Architecture](#security-architecture) below

### 2. Multi-Platform Support

- **CLI Gateway**: Interactive REPL for development and admin tasks
- **Telegram Gateway**: Long-polling and webhook support with user allowlists
- **Slack Gateway**: Socket Mode (no public endpoint required)
- **Buzz Gateway**: Nostr-based, relay-transported multi-agent channel participation — decentralized transport (no single API endpoint) and cryptographically-identified agent/human participants, distinct from the other three request/response-style gateways

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
- **Adapter Layer**: Gateway implementations (CLI, Telegram, Slack, Buzz) and repositories (SQLite)
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
- RBAC role assigned per tool (permission-gated via `ExecuteWithUser`/`checkPermission`; not yet reached by the live chat tool-calling loop — see [Security Architecture](#security-architecture))
- Per-tool rate limiting
- Timeout enforcement (30s default; 30s hard limit for MCP tools)
- Output validation: secret redaction (`OutputSanitizer`) AND injection-pattern detection (`OutputValidator`) — two distinct mechanisms run on all tool output, including MCP output, regardless of MCP trust level
- Side-effecting actions (e.g. `github.pr_merge`, `coding_agent.yolo_mode`) require human yes/no confirmation by default
- MCP tools additionally carry a trust classification (`read_only`/`write`/`unknown`); `write`/`unknown` are treated as admin-only and confirmation-required
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
| **User-input prompt injection** | 30+ pattern detection on direct chat input; input sanitization |
| **Tool-output prompt injection** | `OutputValidator` scans fetched web pages, search results, and MCP output for injection patterns before it reaches the LLM (fail-closed reject by default) — a separate mechanism from the user-input detector above |
| **Command injection** | 50+ pattern detection; output sandboxing |
| **Malicious tools** | Custom tools only; no external imports; MCP output passes through both secret redaction and injection-pattern detection |
| **Unwanted side-effecting actions** | Human yes/no confirmation required before default-configured actions (e.g. `github.pr_merge`) execute; unresolved confirmations expire and are treated as denied |
| **SSRF via fetch tools** | `summarize`/`doc_summarize` resolve and validate the target IP (loopback, RFC 1918, link-local/cloud-metadata, multicast all rejected) before dialing, and re-validate on every redirect hop |
| **Untrusted MCP tools** | Per-server/per-tool trust classification (`read_only`/`write`/`unknown`); `write`/`unknown` treated as admin-only and confirmation-required |
| **Session hijacking** | Token rotation; secure credential vault; TLS enforced Secure cookies |
| **Privilege escalation** | RBAC roles defined and enforced per tool, CI-guarded; `requireRole` middleware on web admin; role enforcement is wired into the live chat tool-calling loop via `ExecuteWithUser` (see below) |
| **Supply chain attacks** | Minimal dependencies; audit logging |
| **Brute-force login** | Per-IP token bucket rate limiter on web admin login |
| **Weak API secrets** | JWT secret minimum 32 bytes enforced at server construction |
| **Insecure transport** | TLS auto-generated (ECDSA P-256) for admin web and health servers |
| **Default credentials** | Forced password change on first login with default admin credentials |

**RBAC in live chat (fixed)**: `chat.Service.ProcessMessage`'s tool-calling loop executes tools via `ToolExecutionService.ExecuteWithUser` — both this main tool-calling loop and the confirmation-approval re-invocation path call `ExecuteWithUser`, which performs the role-based `checkPermission` check. Each incoming message resolves a role-bearing `*domain.User` through `UserService` (`GetUserByPlatformUID`/`CreateUser`), defaulting new non-CLI identities to `RoleGuest` and CLI to `RoleAdmin`; an unresolvable or unregistered platform identity fails closed to `domain.RoleGuest` rather than failing open. `ListTools` (which builds the tool list offered to the LLM) now filters by the resolved caller's role. This means role-based restrictions (e.g. `coding_agent`/`github` writes being admin-only) are correctly defined, unit-tested, CI-guarded, and enforced end-to-end for a live conversation — see the regression test `TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge` (`internal/usecase/chat/rbac_test.go`), which proves a non-admin chat user's attempt to invoke `github.pr_merge` is denied. The side-effecting-action confirmation gate remains independently wired via a context-carried identity and is fully live for real conversations as an additional layer on top of RBAC.

### Input Validation (User Input)

- Maximum input length: 32KB (configurable)
- Null byte detection
- UTF-8 validation
- Prompt injection pattern matching (30+ patterns) — applied to direct chat input only
- Command injection pattern matching (50+ patterns)
- Parameterized queries (no raw SQL)

### Output Validation (Tool-Sourced Content)

- Separate from the input validation above: `OutputValidator` (`internal/usecase/security/output_validation.go`) reuses the same underlying pattern list against content the agent fetches on its own behalf — web pages and documents (`summarize`, `doc_summarize`), web-search results (`websearch`), and all MCP tool output — before that content re-enters the LLM's conversational context
- Configurable action: `reject` (default, fails the tool call closed) or `annotate` (passes content through wrapped with a visible `[SECURITY WARNING: possible injected instructions detected]` marker)
- Flagged content is excluded from memory-curation input, closing the stored/second-order injection path
- Audit trail gains `injection_flagged`/`matched_patterns` fields whenever content is flagged

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
  - Output passes through secret redaction (`OutputSanitizer`) and injection-pattern detection (`OutputValidator`) before reaching the LLM
  - Per-server/per-tool trust classification (`read_only`/`write`/`unknown`) enforced through the same RBAC/confirmation paths as built-in tools
- ✅ **IMS Phase 7**: Integration Test Suite
  - //go:build integration tagged tests across storage, web, memoryv2
- ✅ **Post-review fixes**: 17 hardening items (token validation, goroutine leak fix, JWT secret strength enforcement, rate limiter eviction, MCP timeout, JSON array input validation, store prefix validation)
- ✅ **Buzz Gateway** (3 phases): Nostr-based multi-agent chat gateway
  - **Phase 1 — Read-only participation**: hand-rolled NIP-01 client (`internal/infrastructure/nostr/`), multi-relay WebSocket transport with bounded exponential-backoff reconnect, signature verification, cross-relay event dedupe, generate-if-absent secp256k1 keypair via the existing credential vault
  - **Phase 2 — Full gateway**: signed `kind:9` channel message publishing, `kind:10100` agent-profile self-declaration, `kind:9000`/`kind:10100`-derived agent-identity cache, consecutive-message loop-prevention guard
  - **Phase 3 — Tool integration + cross-platform RBAC fix**: Buzz channel messages trigger existing tools under RBAC; fixed a pre-existing gap where tool execution was unenforced for *every* platform (`ExecuteWithUser` now used uniformly, `ListTools` now role-filtered)
- ✅ **Security Hardening (Parts A-G, 2026-08-02)**: tool-output injection filtering (`OutputValidator`); prompt-boundary guardrails (`<tool_output>` delimiters + non-overridable system-prompt guardrail); side-effecting action confirmation flow (file-backed `ConfirmationStore`, plain-text yes/no + Slack/Telegram buttons + web admin modal + REST endpoints); tool RBAC correction (explicit `ToolPermissions` map, action-aware `github` checks, CI guard); SSRF hardening (IP-resolution validation + redirect revalidation with pinned-IP dialing); MCP trust classification (`read_only`/`write`/`unknown`); documentation/code parity
- ✅ **FR-001/FR-002 — RBAC bypass in live chat fixed (P0), reconciled with the Buzz gateway's independent cross-platform RBAC fix (2026-08-04)**: the live chat tool-calling loop calls `ToolExecutionService.ExecuteWithUser`, not `Execute`, at both of `chat.Service`'s tool-execution sites, so full role-based RBAC — including the action-aware `github` split and MCP trust classification — is enforced end-to-end in live conversations, across every platform (CLI, Telegram, Slack, Buzz). Each incoming message resolves a persisted `domain.User` via `UserService` (`GetUserByPlatformUID`/`CreateUser`), defaulting new non-CLI identities to `RoleGuest` and CLI to `RoleAdmin`; `ListTools` filters the tool list by resolved role. `ExecuteWithUser`/`ListTools` carry `conversationID` so Part C's confirmation gate composes with RBAC, and confirmation-reply detection is keyed on the resolved user's persisted ID (not raw platform UID) to stay consistent with what `ExecuteWithUser` stores. Regression tests: `internal/usecase/chat/rbac_test.go`'s `TestProcessMessage_RBACEnforcedAcrossPlatforms` and `TestProcessMessage_RBAC_NonAdminCannotInvokeGitHubPRMerge`.

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
