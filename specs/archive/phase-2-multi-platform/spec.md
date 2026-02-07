# Phase 2: Multi-Platform - Feature Specification

**Phase:** Phase 2
**Status:** In Progress
**Started:** 2026-02-06
**Target Completion:** TBD

---

## 1. Overview

Phase 2 extends the NuimanBot MVP with multi-platform support and enhanced capabilities. Building on the solid Phase 1 foundation (CLI gateway, Anthropic provider, basic skills, SQLite storage), Phase 2 adds:

- **Additional Gateways:** Telegram and Slack integration for broader user reach
- **Additional LLM Providers:** OpenAI and Ollama support for provider flexibility
- **Enhanced Security:** Full RBAC enforcement with per-user permissions
- **User Management:** Admin commands for user lifecycle operations
- **Expanded Skills:** weather, web_search, and notes skills

This phase transforms NuimanBot from a single-platform CLI tool into a multi-platform conversational agent with production-ready user management.

---

## 2. Goals

### Primary Goals
1. Enable Telegram and Slack as additional messaging platforms
2. Support OpenAI GPT models and Ollama local models
3. Implement comprehensive RBAC enforcement throughout the application
4. Provide admin tools for user management (CRUD operations)
5. Add practical skills: weather forecasting, web search, and note-taking

### Success Criteria
- [ ] Users can interact with NuimanBot via Telegram
- [ ] Users can interact with NuimanBot via Slack
- [ ] OpenAI and Ollama providers work alongside Anthropic
- [ ] RBAC prevents unauthorized skill execution
- [ ] Admin users can create, update, delete, and list users
- [ ] weather, web_search, and notes skills are fully functional
- [ ] All quality gates pass (fmt, tidy, vet, lint, test, build)
- [ ] Test coverage maintained at 75%+ overall

---

## 3. Features

### 3.1. Telegram Gateway

**Description:** Integrate Telegram Bot API using `github.com/go-telegram/bot` to enable users to interact with NuimanBot through Telegram.

**User Stories:**
- As a user, I want to send messages to NuimanBot via Telegram so I can access its features from my mobile device
- As an admin, I want to configure the Telegram bot token securely
- As a user, I want to receive formatted responses from NuimanBot in Telegram

**Acceptance Criteria:**
- [ ] Telegram gateway implements the `Gateway` interface
- [ ] Bot receives and processes text messages from users
- [ ] Bot sends responses back to users with markdown formatting
- [ ] Bot handles errors gracefully (e.g., invalid commands, rate limits)
- [ ] Configuration includes `telegram.bot_token` as a `SecureString`
- [ ] User platform ID is mapped to Telegram user ID
- [ ] Messages are persisted to the database with platform="telegram"

**Technical Notes:**
- Library: `github.com/go-telegram/bot`
- Authentication: Bot token stored in credential vault
- Message format: Support Telegram markdown
- Long polling or webhook mode (start with long polling for MVP)

---

### 3.2. Slack Gateway

**Description:** Integrate Slack Bot API using `github.com/slack-go/slack` Socket Mode to enable workspace interactions.

**User Stories:**
- As a user, I want to mention NuimanBot in Slack channels to ask questions
- As a user, I want to DM NuimanBot for private conversations
- As an admin, I want to configure Slack app credentials securely

**Acceptance Criteria:**
- [ ] Slack gateway implements the `Gateway` interface
- [ ] Bot responds to direct mentions in channels
- [ ] Bot handles direct messages
- [ ] Bot sends responses with Slack Block Kit formatting
- [ ] Configuration includes `slack.bot_token` and `slack.app_token` as `SecureString`
- [ ] User platform ID is mapped to Slack user ID
- [ ] Messages are persisted with platform="slack"

**Technical Notes:**
- Library: `github.com/slack-go/slack`
- Mode: Socket Mode (no public URL required)
- Events: Listen for `app_mention` and `message` events
- Formatting: Use Slack Block Kit for rich responses

---

### 3.3. OpenAI Provider

**Description:** Add OpenAI as an LLM provider to support GPT-4o, GPT-4, and other OpenAI models.

**User Stories:**
- As an admin, I want to configure NuimanBot to use OpenAI models
- As a user, I want my requests processed by GPT-4o when configured
- As an admin, I want to switch between Anthropic and OpenAI providers

**Acceptance Criteria:**
- [ ] OpenAI client implements `LLMProvider` interface
- [ ] Supports non-streaming completion
- [ ] Supports streaming completion (SSE)
- [ ] Tool calling works with OpenAI function calling format
- [ ] Configuration includes `openai.api_key` as `SecureString`
- [ ] Provider selection works via config or per-request
- [ ] Token usage is tracked and logged

**Technical Notes:**
- Library: `github.com/sashabaranov/go-openai`
- Models: gpt-4o, gpt-4-turbo, gpt-4, gpt-3.5-turbo
- Tool format: Convert between NuimanBot skill format and OpenAI function format
- Error handling: Handle rate limits, context length errors

---

### 3.4. Ollama Provider

**Description:** Add Ollama support for running local LLM models (e.g., llama3, mistral, codellama).

**User Stories:**
- As a developer, I want to run NuimanBot with local models for privacy
- As an admin, I want to configure Ollama endpoint URL
- As a user, I want responses from local models without external API calls

**Acceptance Criteria:**
- [ ] Ollama client implements `LLMProvider` interface
- [ ] Supports non-streaming completion
- [ ] Supports streaming completion
- [ ] Tool calling works (if supported by model)
- [ ] Configuration includes `ollama.base_url` (default: http://localhost:11434)
- [ ] Model selection via `ollama.default_model` config
- [ ] Graceful degradation if Ollama service is unavailable

**Technical Notes:**
- API: Ollama HTTP API (REST)
- Endpoint: `/api/generate` for completion
- Models: llama3, mistral, codellama, etc.
- Tool calling: May need adaptation based on model capabilities
- Local only: No API key required

---

### 3.5. RBAC Enforcement

**Description:** Enforce role-based access control throughout the application to restrict skill execution based on user roles.

**User Stories:**
- As an admin, I want to define which skills each role can access
- As the system, I want to deny unauthorized skill execution attempts
- As an admin, I want audit logs for permission violations

**Acceptance Criteria:**
- [ ] `User.Role` (Admin, User) determines skill access
- [ ] `User.AllowedSkills` provides per-user override
- [ ] Permission checks occur before skill execution
- [ ] Permission denials are logged to audit log
- [ ] Error messages indicate insufficient permissions
- [ ] Admin role has access to all skills by default
- [ ] User role has access only to safe skills (calculator, datetime)

**Technical Notes:**
- Check permissions in `SkillExecutionService.Execute()`
- Use `SecurityService.Audit()` for permission violations
- Consider per-skill permission requirements (e.g., weather requires User+)
- Document permission model in README

---

### 3.6. User Management

**Description:** Add admin commands for user lifecycle management (create, read, update, delete).

**User Stories:**
- As an admin, I want to create new users with specific roles
- As an admin, I want to list all users in the system
- As an admin, I want to update user roles and permissions
- As an admin, I want to delete users who no longer need access

**Acceptance Criteria:**
- [ ] `/admin user create <platform> <platform_uid> <role>` command works
- [ ] `/admin user list` command shows all users
- [ ] `/admin user get <user_id>` command shows user details
- [ ] `/admin user update <user_id> --role <role>` updates user role
- [ ] `/admin user delete <user_id>` deletes user
- [ ] Only Admin role can execute user management commands
- [ ] User management operations are audit logged

**Technical Notes:**
- Add `UserService` interface in `internal/usecase/user/`
- Implement `UserService` using `UserRepository`
- Add admin command parser to CLI gateway
- Consider adding user management to Telegram/Slack gateways
- Validate platform+platform_uid uniqueness

---

### 3.7. Weather Skill

**Description:** Add a weather forecast skill using OpenWeatherMap API.

**User Stories:**
- As a user, I want to ask "What's the weather in San Francisco?"
- As a user, I want to get current temperature and conditions
- As a user, I want to get a 3-day forecast

**Acceptance Criteria:**
- [ ] Skill name: `weather`
- [ ] Parameters: `location` (string), `days` (int, optional, default 1)
- [ ] Returns: Current conditions, temperature, humidity, wind speed
- [ ] Returns: Forecast for specified days (max 3)
- [ ] Requires API key configured in `skills.weather.api_key`
- [ ] Handles API errors gracefully (invalid location, rate limit)
- [ ] Unit tests with mocked API responses

**Technical Notes:**
- API: OpenWeatherMap (free tier: current + 5-day forecast)
- Endpoint: `https://api.openweathermap.org/data/2.5/weather`
- Rate limit: 60 calls/min on free tier
- Permission: Requires User+ role

---

### 3.8. Web Search Skill

**Description:** Add web search capability using DuckDuckGo or similar API.

**User Stories:**
- As a user, I want to search the web for information
- As a user, I want to get top 3-5 search results with summaries

**Acceptance Criteria:**
- [ ] Skill name: `web_search`
- [ ] Parameters: `query` (string), `num_results` (int, optional, default 3)
- [ ] Returns: Title, URL, snippet for each result
- [ ] Uses DuckDuckGo or SerpAPI (no API key needed for DDG)
- [ ] Handles search errors gracefully
- [ ] Unit tests with mocked search responses

**Technical Notes:**
- Option 1: DuckDuckGo Instant Answer API (free, no key)
- Option 2: SerpAPI (paid, requires API key)
- Start with DuckDuckGo for MVP
- Permission: Requires User+ role
- Consider result caching to avoid duplicate searches

---

### 3.9. Notes Skill

**Description:** Add personal note-taking functionality with database persistence.

**User Stories:**
- As a user, I want to save notes for later retrieval
- As a user, I want to list all my notes
- As a user, I want to retrieve a specific note by ID
- As a user, I want to delete notes I no longer need

**Acceptance Criteria:**
- [ ] Skill name: `notes`
- [ ] Commands: `create`, `list`, `get`, `delete`
- [ ] Notes are stored in SQLite with user_id association
- [ ] Notes include: id, user_id, title, content, created_at, updated_at
- [ ] User can only access their own notes (not other users' notes)
- [ ] Unit and integration tests for all CRUD operations

**Technical Notes:**
- Add `notes` table to SQLite schema
- Create `NotesRepository` interface and implementation
- Notes skill uses `NotesRepository` for persistence
- Consider markdown support in note content
- Permission: Requires User+ role
- This is a stateful skill (modifies database)

---

## 4. Architecture Changes

### 4.1. Gateway Layer Expansion

**Before (Phase 1):**
```
internal/adapter/gateway/
└── cli/
    └── gateway.go
```

**After (Phase 2):**
```
internal/adapter/gateway/
├── cli/
│   └── gateway.go
├── telegram/
│   └── gateway.go
└── slack/
    └── gateway.go
```

All gateways implement the same `Gateway` interface defined in `internal/domain/`.

---

### 4.2. LLM Provider Expansion

**Before (Phase 1):**
```
internal/infrastructure/llm/
└── anthropic/
    └── client.go
```

**After (Phase 2):**
```
internal/infrastructure/llm/
├── anthropic/
│   └── client.go
├── openai/
│   └── client.go
└── ollama/
    └── client.go
```

All providers implement the `LLMProvider` interface.

---

### 4.3. Skills Registry Expansion

**Before (Phase 1):**
```
internal/skills/
├── calculator/
│   └── calculator.go
└── datetime/
    └── datetime.go
```

**After (Phase 2):**
```
internal/skills/
├── calculator/
│   └── calculator.go
├── datetime/
│   └── datetime.go
├── weather/
│   └── weather.go
├── web_search/
│   └── web_search.go
└── notes/
    └── notes.go
```

---

### 4.4. User Management Service

**New Addition:**
```
internal/usecase/user/
├── service.go      # UserService interface and implementation
└── service_test.go # Unit tests
```

**Integration:**
- `UserService` uses `UserRepository` (already exists)
- Admin commands route to `UserService`
- Permission checks occur before execution

---

## 5. Configuration Changes

### 5.1. Gateway Configuration

```yaml
gateways:
  telegram:
    enabled: true
    bot_token: ${TELEGRAM_BOT_TOKEN}  # SecureString
    allowed_users: []  # Empty = allow all registered users

  slack:
    enabled: true
    bot_token: ${SLACK_BOT_TOKEN}      # SecureString
    app_token: ${SLACK_APP_TOKEN}      # SecureString
    allowed_channels: []  # Empty = allow all channels

  cli:
    enabled: true
```

---

### 5.2. LLM Configuration

```yaml
llm:
  default_provider: anthropic  # or openai, ollama

  providers:
    - name: anthropic
      type: anthropic
      api_key: ${ANTHROPIC_API_KEY}
      default_model: claude-sonnet-4-20250514

    - name: openai
      type: openai
      api_key: ${OPENAI_API_KEY}
      default_model: gpt-4o

    - name: ollama
      type: ollama
      base_url: http://localhost:11434
      default_model: llama3
```

---

### 5.3. Skills Configuration

```yaml
skills:
  weather:
    enabled: true
    api_key: ${OPENWEATHER_API_KEY}  # SecureString
    default_location: "San Francisco, CA"

  web_search:
    enabled: true
    provider: duckduckgo  # or serpapi
    # api_key: ${SERPAPI_KEY}  # Only if using SerpAPI

  notes:
    enabled: true
    # No additional config needed (uses main database)
```

---

## 6. Testing Strategy

### 6.1. Unit Tests
- All new services, gateways, and skills have unit tests
- Mock external APIs (Telegram, Slack, OpenWeatherMap)
- Test permission checks in isolation
- Test user management CRUD operations

### 6.2. Integration Tests
- Test gateway message flow (incoming -> ChatService -> outgoing)
- Test LLM provider integration with real/mock APIs
- Test skills with database (especially notes skill)
- Test RBAC enforcement in full flow

### 6.3. E2E Tests (Manual)
- Send message via Telegram, verify response
- Send message via Slack, verify response
- Execute weather skill with real API
- Execute web_search skill with real API
- Create, list, retrieve, delete notes
- Test permission denial for unauthorized users

---

## 7. Rollout Plan

### 7.1. Implementation Order

**Priority 1 (Core Multi-Platform):**
1. OpenAI provider (enables provider diversity)
2. Ollama provider (enables local deployment)
3. RBAC enforcement (security foundation)
4. User management (enables role administration)

**Priority 2 (Gateway Expansion):**
5. Telegram gateway
6. Slack gateway

**Priority 3 (Enhanced Skills):**
7. Weather skill
8. Web search skill
9. Notes skill

### 7.2. Development Approach

Follow the same sub-agent collaboration model from Phase 1:
- **LLM Agent**: Implements OpenAI and Ollama providers
- **Gateway Agent**: Implements Telegram and Slack gateways
- **Skills Agent**: Implements weather, web_search, notes skills
- **Security Agent**: Implements RBAC enforcement
- **User Management Agent**: Implements UserService and admin commands
- **QA Agent**: Ensures all tests pass and coverage is maintained

---

## 8. Risk Assessment

### 8.1. Technical Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Telegram/Slack API changes | Medium | Pin library versions, monitor changelogs |
| OpenAI rate limits | Medium | Implement exponential backoff, caching |
| Ollama model compatibility | Low | Document supported models, test with popular ones |
| RBAC complexity | Medium | Start simple (role-based only), iterate |
| Weather API rate limits | Low | Cache results, use free tier initially |

### 8.2. Security Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Unauthorized skill access | High | Strict RBAC enforcement, audit logging |
| API key exposure | High | Store in credential vault, never log |
| Cross-user note access | High | Enforce user_id checks in NotesRepository |
| Admin command abuse | Medium | Require Admin role, audit all operations |

---

## 9. Success Metrics

### 9.1. Functionality
- [ ] All 3 gateways (CLI, Telegram, Slack) operational
- [ ] All 3 LLM providers (Anthropic, OpenAI, Ollama) operational
- [ ] All 5 skills (calculator, datetime, weather, web_search, notes) operational
- [ ] RBAC prevents unauthorized access (validated via tests)
- [ ] User management supports full CRUD (validated via tests)

### 9.2. Quality
- [ ] All quality gates pass (fmt, tidy, vet, lint, test, build)
- [ ] Test coverage ≥75% overall
- [ ] No high-severity linter warnings
- [ ] All E2E flows validated manually

### 9.3. Documentation
- [ ] README updated with Phase 2 features
- [ ] Configuration examples for all new features
- [ ] API documentation for new skills
- [ ] User guide for Telegram/Slack setup

---

## 10. Out of Scope (Phase 3+)

The following features are explicitly **not** part of Phase 2:
- MCP server/client integration (Phase 3)
- PostgreSQL backend (Phase 4)
- Monitoring/metrics (Phase 4)
- Conversation summarization (Phase 3)
- Token window management (Phase 3)
- Additional skills beyond weather/web_search/notes
- REST API for external management
- WebSocket streaming
- Multi-server deployment

---

## 11. References

- Phase 1 Spec: `specs/initial-mvp-spec/spec.md`
- Architecture Guide: `AGENTS.md`
- Product Requirements: `PRODUCT_REQUIREMENT_DOC.md`
- Implementation Plan: `specs/phase-2-multi-platform/plan.md` (to be created)
- Task Breakdown: `specs/phase-2-multi-platform/tasks.md` (to be created)
