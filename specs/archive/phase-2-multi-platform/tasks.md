# Phase 2: Multi-Platform - Tasks

**Last Updated:** 2026-02-06
**Status:** Ready for Implementation

This document breaks down Phase 2 work into concrete, testable tasks organized by sub-agent and priority.

---

## Task Status Legend

- `☐` Not started
- `▸` In progress
- `✅` Complete
- `❌` Blocked

---

## Priority 1: Security & Foundation (Week 1)

### 1. Security Agent: RBAC Enforcement

#### Task 1.1: Define Permission Matrix [✅]
**Description:** Create permission mapping for all skills
**Files:**
- Create `internal/usecase/skill/permissions.go`

**Acceptance Criteria:**
- [x] `SkillPermissions` map defined with all skills
- [x] Guest, User, and Admin roles properly assigned
- [x] Constants are exported and documented

**Tests:**
- [x] None (data structure only)

**Estimated Time:** 1 hour
**Actual Time:** 15 minutes

---

#### Task 1.2: Extend User Entity [✅]
**Description:** Add `AllowedSkills` field to User entity
**Files:**
- Modify `internal/domain/user.go`

**Acceptance Criteria:**
- [x] `AllowedSkills []string` field added (already existed)
- [x] RoleGuest constant added
- [x] Field documented with comment
- [x] Empty slice means all skills allowed for role
- [x] Added Role.Level() and Role.HasPermission() methods

**Tests:**
- [x] None (data structure only)

**Estimated Time:** 30 minutes
**Actual Time:** 20 minutes

---

#### Task 1.3: Database Migration for RBAC [✅]
**Description:** Add `allowed_skills` column to users table
**Files:**
- Modify `internal/adapter/repository/sqlite/schema.go`
- Modify `internal/adapter/repository/sqlite/user.go`

**Acceptance Criteria:**
- [x] Schema already includes `allowed_skills` TEXT column
- [x] Default value is '[]' (empty JSON array)
- [x] Migration runs automatically on startup
- [x] UserRepository handles JSON serialization/deserialization

**Tests:**
- [x] Integration test: Create user with allowed_skills (existing)
- [x] Integration test: Retrieve user with allowed_skills (existing)
- [x] Integration test: Update user allowed_skills (existing)

**Estimated Time:** 2 hours
**Actual Time:** 0 (already implemented)

---

#### Task 1.4: Implement Permission Check Logic [✅]
**Description:** Add permission checking to SkillExecutionService
**Files:**
- Modify `internal/usecase/skill/service.go`
- Modify `internal/domain/errors.go`

**Acceptance Criteria:**
- [x] `checkPermission()` method implemented
- [x] Checks user role against required role
- [x] Checks AllowedSkills override if present
- [x] Returns `ErrInsufficientPermissions` on denial
- [x] Added ErrInsufficientPermissions to domain/errors.go
- [x] Extracted isSkillWhitelisted() helper method (refactor)

**Tests:**
- [x] Unit test: Admin can execute all skills
- [x] Unit test: User can execute User+ skills, not Admin skills
- [x] Unit test: Guest can only execute Guest skills
- [x] Unit test: AllowedSkills whitelist works
- [x] Unit test: Empty AllowedSkills allows all for role

**Estimated Time:** 3 hours
**Actual Time:** 2 hours

---

#### Task 1.5: Add Permission Check to Execute() [✅]
**Description:** Call checkPermission() before skill execution
**Files:**
- Modify `internal/usecase/skill/service.go`

**Acceptance Criteria:**
- [x] Created new `ExecuteWithUser()` method that calls `checkPermission()`
- [x] Permission denied errors are returned early
- [x] Audit log records permission denials
- [x] Extracted auditPermissionDenial() helper method (refactor)

**Tests:**
- [x] Integration test: User denied admin skill
- [x] Integration test: Audit log contains denial

**Estimated Time:** 1 hour
**Actual Time:** 30 minutes

---

#### Task 1.6: Add Permission Error Handling [✅]
**Description:** Handle permission errors in gateways
**Files:**
- Modify `internal/domain/errors.go`

**Acceptance Criteria:**
- [x] `ErrInsufficientPermissions` defined in domain
- [x] `ErrSkillNotFound` also added
- [x] CLI gateway integration to be done when updating ChatService

**Tests:**
- [x] Covered by ExecuteWithUser tests

**Estimated Time:** 1 hour
**Actual Time:** 10 minutes

---

**Total Estimated Time: 8-9 hours (1-2 days)**

---

### 2. User Management Agent: Admin Commands

#### Task 2.1: Define UserService Interface [✅]
**Description:** Create UserService interface for user management
**Files:**
- Create `internal/usecase/user/service.go`

**Acceptance Criteria:**
- [x] `UserService` interface defined with CRUD methods
- [x] Methods: CreateUser, GetUser, GetUserByPlatformUID, ListUsers, UpdateUserRole, UpdateAllowedSkills, DeleteUser
- [x] Interface documented with comments
- [x] ExtendedUserRepository interface defined (adds ListAll and Delete)

**Tests:**
- [x] None (interface only)

**Estimated Time:** 1 hour
**Actual Time:** 30 minutes

---

#### Task 2.2: Implement UserService [✅]
**Description:** Implement UserService with business logic
**Files:**
- Modify `internal/usecase/user/service.go`

**Acceptance Criteria:**
- [x] All interface methods implemented
- [x] CreateUser validates platform+platformUID uniqueness
- [x] DeleteUser prevents deleting last admin
- [x] UpdateUserRole prevents demoting last admin
- [x] All operations call Audit() for logging
- [x] Extracted auditSuccess() helper method (refactored)

**Tests:**
- [x] Unit test: CreateUser creates user
- [x] Unit test: CreateUser rejects duplicate platform+platformUID
- [x] Unit test: GetUser retrieves user
- [x] Unit test: GetUserByPlatformUID works
- [x] Unit test: ListUsers returns all users
- [x] Unit test: UpdateUserRole updates role
- [x] Unit test: UpdateUserRole prevents demoting last admin
- [x] Unit test: UpdateAllowedSkills updates skills list
- [x] Unit test: DeleteUser deletes user
- [x] Unit test: DeleteUser prevents deleting last admin
- [x] Unit test: All operations are audited

**Estimated Time:** 6 hours
**Actual Time:** 3 hours

---

#### Task 2.3: Add Admin Command Parser [✅]
**Description:** Parse `/admin user` commands in CLI gateway
**Files:**
- Create `internal/adapter/gateway/cli/admin_commands.go`
- Modify `internal/adapter/gateway/cli/gateway.go`

**Acceptance Criteria:**
- [x] Recognizes `/admin user create <platform> <platform_uid> <role>`
- [x] Recognizes `/admin user list`
- [x] Recognizes `/admin user get <user_id>`
- [x] Recognizes `/admin user update <user_id> --role <role>`
- [x] Recognizes `/admin user update <user_id> --skills <skill1,skill2>`
- [x] Recognizes `/admin user delete <user_id>`
- [x] Shows usage help if command malformed
- [x] Created AdminCommandHandler with full command parsing
- [x] Admin permission checks enforced

**Tests:**
- [x] Unit test: Parse create command
- [x] Unit test: Parse list command
- [x] Unit test: Parse get command
- [x] Unit test: Parse update role command
- [x] Unit test: Parse update skills command
- [x] Unit test: Parse delete command
- [x] Unit test: Invalid command shows help
- [x] Unit test: Non-admin denied admin commands

**Estimated Time:** 4 hours
**Actual Time:** 2 hours

---

#### Task 2.4: Wire UserService to Admin Commands [✅]
**Description:** Connect admin commands to UserService
**Files:**
- Modify `internal/adapter/gateway/cli/gateway.go`
- Create `internal/adapter/gateway/cli/admin_commands_test.go`

**Acceptance Criteria:**
- [x] CLI gateway has reference to AdminCommandHandler
- [x] AdminCommandHandler has reference to UserService
- [x] Admin commands call appropriate UserService methods
- [x] Errors are displayed to user
- [x] Success messages are displayed to user
- [x] Only admin users can execute admin commands
- [x] Admin commands intercepted before normal message flow
- [x] Gateway has SetAdminHandler() and SetCurrentUser() methods

**Tests:**
- [x] Integration test: Create user via CLI command
- [x] Integration test: List users via CLI command
- [x] Integration test: Get user via CLI command
- [x] Integration test: Update user role via CLI command
- [x] Integration test: Update user skills via CLI command
- [x] Integration test: Delete user via CLI command
- [x] Integration test: Non-admin denied admin commands
- [x] Unit test: IsAdminCommand() detects admin commands

**Estimated Time:** 3 hours
**Actual Time:** 2 hours

---

**Total Estimated Time: 14 hours (2 days)**

---

## Priority 2: Provider Expansion (Week 2)

### 3. LLM Provider Agent: OpenAI

#### Task 3.1: Add OpenAI Dependency [✅]
**Description:** Install OpenAI SDK
**Files:**
- Modify `go.mod`

**Acceptance Criteria:**
- [x] `github.com/sashabaranov/go-openai` added
- [x] `go mod tidy` runs successfully

**Tests:**
- [x] None

**Estimated Time:** 15 minutes
**Actual Time:** 10 minutes

---

#### Task 3.2: Define OpenAI Config [✅]
**Description:** Add OpenAI provider configuration
**Files:**
- Modify `internal/config/config.go`
- Modify `internal/config/loader.go`
- Modify `internal/config/loader_test.go`

**Acceptance Criteria:**
- [x] `OpenAIProviderConfig` struct defined
- [x] Fields: APIKey, DefaultModel, Organization, BaseURL
- [x] Also defined `OllamaProviderConfig` and `AnthropicProviderConfig`
- [x] Struct has yaml tags
- [x] Loader manually handles provider configs to support SecureString
- [x] Excluded provider configs from automatic decoding

**Tests:**
- [x] Unit test: Config loads from YAML (TestLoadConfig_OpenAIConfig)
- [x] Unit test: All existing config tests still pass

**Estimated Time:** 1 hour
**Actual Time:** 45 minutes

---

#### Task 3.3: Implement OpenAI Client Structure [✅]
**Description:** Create OpenAI client skeleton
**Files:**
- Create `internal/infrastructure/llm/openai/client.go`
- Create `internal/infrastructure/llm/openai/client_test.go`

**Acceptance Criteria:**
- [x] `Client` struct defined
- [x] `New()` constructor implemented
- [x] Client holds openai.Client and config
- [x] Constructor sets BaseURL if provided
- [x] Constructor sets Organization if provided

**Tests:**
- [x] Unit test: New() creates client

**Estimated Time:** 1 hour
**Actual Time:** 20 minutes

---

#### Task 3.4: Implement OpenAI Complete() [✅]
**Description:** Implement non-streaming completion
**Files:**
- Modify `internal/infrastructure/llm/openai/client.go`
- Modify `internal/infrastructure/llm/openai/client_test.go`

**Acceptance Criteria:**
- [x] `Complete()` method implemented
- [x] Converts domain.LLMRequest to openai.ChatCompletionRequest
- [x] Converts openai.ChatCompletionResponse to domain.LLMResponse
- [x] Handles errors (returns error on API failure)
- [x] Tracks token usage
- [x] Handles system prompts
- [x] Uses default model from config if not specified
- [x] Refactored to extract helper methods (convertRequest, convertResponse, convertTools, convertToolCalls)
- [x] Properly parses JSON tool arguments

**Tests:**
- [x] Unit test: Complete() with invalid key (tests error handling)

**Estimated Time:** 4 hours
**Actual Time:** 1.5 hours

---

#### Task 3.5: Implement OpenAI Stream() [✅]
**Description:** Implement streaming completion
**Files:**
- Modify `internal/infrastructure/llm/openai/client.go`
- Modify `internal/infrastructure/llm/openai/client_test.go`

**Acceptance Criteria:**
- [x] `Stream()` method implemented
- [x] Returns channel of domain.StreamChunk
- [x] Handles streaming errors gracefully (checks for io.EOF)
- [x] Closes channel when done
- [x] Processes content deltas
- [x] Processes tool call deltas
- [x] Checks finish reason

**Tests:**
- [x] Unit test: Stream() with invalid key (tests error handling)

**Estimated Time:** 3 hours
**Actual Time:** 45 minutes

---

#### Task 3.6: Implement OpenAI Tool Calling [✅]
**Description:** Convert NuimanBot skills to OpenAI functions
**Files:**
- Already implemented in Task 3.4

**Acceptance Criteria:**
- [x] `convertTools()` converts domain.ToolDefinition to openai.Tool
- [x] Tool calls in response are parsed correctly (convertToolCalls)
- [x] JSON arguments properly parsed with error handling
- [x] Tool parameters mapped to OpenAI FunctionDefinition

**Tests:**
- [x] Covered by Complete() tests

**Estimated Time:** 4 hours
**Actual Time:** 0 (completed in Task 3.4)

---

#### Task 3.7: Wire OpenAI Provider to Main [✅]
**Description:** Register OpenAI provider in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`
- Modify `internal/infrastructure/llm/openai/client.go` (added ListModels)

**Acceptance Criteria:**
- [x] OpenAI provider registered if configured
- [x] Provider selection logic updated (checks llm.openai.api_key first)
- [x] Graceful degradation if API key missing (clear error message)
- [x] Supports both new config format (llm.openai) and old format (llm.providers array)
- [x] Implemented ListModels() method to complete LLMService interface
- [x] Added logging for provider initialization

**Tests:**
- [x] Build succeeds, all existing tests pass

**Estimated Time:** 1 hour
**Actual Time:** 30 minutes

---

**Total Estimated Time: 14-15 hours (2-3 days)**

---

### 4. LLM Provider Agent: Ollama

#### Task 4.1: Define Ollama Config [✅]
**Description:** Add Ollama provider configuration
**Files:**
- Already completed in Task 3.2

**Acceptance Criteria:**
- [x] `OllamaProviderConfig` struct defined
- [x] Fields: BaseURL, DefaultModel
- [x] Struct has yaml tags
- [x] Loader manually handles config

**Tests:**
- [x] Unit test: Config loads from YAML (TestLoadConfig_OpenAIConfig covers Ollama too)

**Estimated Time:** 1 hour
**Actual Time:** 0 (completed in Task 3.2)

---

#### Task 4.2: Implement Ollama Client Structure [✅]
**Description:** Create Ollama client skeleton
**Files:**
- Create `internal/infrastructure/llm/ollama/client.go`
- Create `internal/infrastructure/llm/ollama/client_test.go`

**Acceptance Criteria:**
- [x] `Client` struct defined
- [x] `New()` constructor implemented
- [x] Client holds http.Client and config
- [x] HTTP client has 120s timeout for slow models

**Tests:**
- [x] Unit test: New() creates client

**Estimated Time:** 1 hour
**Actual Time:** 10 minutes

---

#### Task 4.3: Implement Ollama Complete() [✅]
**Description:** Implement non-streaming completion via /api/chat
**Files:**
- Modify `internal/infrastructure/llm/ollama/client.go`
- Modify `internal/infrastructure/llm/ollama/client_test.go`

**Acceptance Criteria:**
- [x] `Complete()` method implemented
- [x] Makes HTTP POST to /api/chat endpoint
- [x] Converts domain.LLMRequest to Ollama format (messages, options)
- [x] Parses Ollama response to domain.LLMResponse
- [x] Handles connection errors and HTTP errors
- [x] Supports system prompts
- [x] Uses default model from config if not specified
- [x] Maps temperature and max_tokens to Ollama options

**Tests:**
- [x] Unit test: Complete() with mock HTTP server

**Estimated Time:** 3 hours
**Actual Time:** 45 minutes

---

#### Task 4.4: Implement Ollama Stream() [✅]
**Description:** Implement streaming completion via line-delimited JSON
**Files:**
- Modify `internal/infrastructure/llm/ollama/client.go`
- Modify `internal/infrastructure/llm/ollama/client_test.go`

**Acceptance Criteria:**
- [x] `Stream()` method implemented
- [x] Parses line-delimited JSON responses
- [x] Returns channel of domain.StreamChunk
- [x] Handles streaming errors gracefully
- [x] Closes channel on completion
- [x] Checks for done flag in response

**Tests:**
- [x] Unit test: Stream() with mock HTTP server

**Estimated Time:** 3 hours
**Actual Time:** 30 minutes

---

#### Task 4.5: Wire Ollama Provider to Main [✅]
**Description:** Register Ollama provider in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`
- Modify `internal/infrastructure/llm/ollama/client.go` (added ListModels)

**Acceptance Criteria:**
- [x] Ollama provider registered if configured
- [x] Provider selection logic updated (checks llm.ollama.base_url)
- [x] Graceful degradation if Ollama unavailable
- [x] Default BaseURL to http://localhost:11434 if not specified
- [x] Implemented ListModels() via /api/tags endpoint
- [x] Added logging for provider initialization

**Tests:**
- [x] Build succeeds, all existing tests pass

**Estimated Time:** 1 hour
**Actual Time:** 20 minutes

---

**Total Estimated Time: 9 hours (1-2 days)**

---

## Priority 3: Gateway Expansion (Week 3-4)

### 5. Gateway Agent: Telegram

#### Task 5.1: Add Telegram Dependency [☐]
**Description:** Install Telegram bot SDK
**Files:**
- Modify `go.mod`

**Acceptance Criteria:**
- [ ] `github.com/go-telegram/bot` added
- [ ] `go mod tidy` runs successfully

**Tests:**
- [ ] None

**Estimated Time:** 15 minutes

---

#### Task 5.2: Define Telegram Config [☐]
**Description:** Add Telegram gateway configuration
**Files:**
- Modify `internal/config/gateway_config.go`

**Acceptance Criteria:**
- [ ] `TelegramConfig` struct defined
- [ ] Fields: Enabled, BotToken, AllowedUsers
- [ ] Struct has mapstructure tags

**Tests:**
- [ ] Unit test: Config loads from YAML
- [ ] Unit test: Config loads from env vars

**Estimated Time:** 1 hour

---

#### Task 5.3: Implement Telegram Gateway Structure [☐]
**Description:** Create Telegram gateway skeleton
**Files:**
- Create `internal/adapter/gateway/telegram/gateway.go`

**Acceptance Criteria:**
- [ ] `Gateway` struct defined
- [ ] Implements `domain.Gateway` interface
- [ ] `New()` constructor creates bot client

**Tests:**
- [ ] Unit test: New() creates gateway

**Estimated Time:** 1 hour

---

#### Task 5.4: Implement Telegram Start/Stop [☐]
**Description:** Implement gateway lifecycle methods
**Files:**
- Modify `internal/adapter/gateway/telegram/gateway.go`

**Acceptance Criteria:**
- [ ] `Start()` begins long polling
- [ ] `Stop()` gracefully shuts down bot
- [ ] `Platform()` returns PlatformTelegram

**Tests:**
- [ ] Unit test: Start() starts polling
- [ ] Unit test: Stop() stops polling

**Estimated Time:** 2 hours

---

#### Task 5.5: Implement Telegram Message Receiving [☐]
**Description:** Parse incoming Telegram messages
**Files:**
- Modify `internal/adapter/gateway/telegram/gateway.go`

**Acceptance Criteria:**
- [ ] `OnMessage()` registers handler
- [ ] Incoming updates are parsed to domain.IncomingMessage
- [ ] PlatformUID is Telegram user ID
- [ ] Metadata includes message_id, chat_id, chat_type
- [ ] Handler is called for each message

**Tests:**
- [ ] Unit test: Message parsing
- [ ] Unit test: Handler is called
- [ ] Integration test: Receive real message (manual)

**Estimated Time:** 3 hours

---

#### Task 5.6: Implement Telegram Message Sending [☐]
**Description:** Send responses to Telegram users
**Files:**
- Modify `internal/adapter/gateway/telegram/gateway.go`

**Acceptance Criteria:**
- [ ] `Send()` sends message to chat
- [ ] Supports markdown formatting
- [ ] Handles errors (chat not found, bot blocked)

**Tests:**
- [ ] Unit test: Send() calls bot API
- [ ] Integration test: Send real message (manual)

**Estimated Time:** 2 hours

---

#### Task 5.7: Wire Telegram Gateway to Main [☐]
**Description:** Initialize Telegram gateway in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Acceptance Criteria:**
- [ ] Telegram gateway created if enabled
- [ ] OnMessage wired to ChatService
- [ ] Gateway starts with application
- [ ] Gateway stops on shutdown

**Tests:**
- [ ] Integration test: Full message flow via Telegram

**Estimated Time:** 2 hours

---

**Total Estimated Time: 11-12 hours (2-3 days)**

---

### 6. Gateway Agent: Slack

#### Task 6.1: Add Slack Dependency [☐]
**Description:** Install Slack SDK
**Files:**
- Modify `go.mod`

**Acceptance Criteria:**
- [ ] `github.com/slack-go/slack` added
- [ ] `go mod tidy` runs successfully

**Tests:**
- [ ] None

**Estimated Time:** 15 minutes

---

#### Task 6.2: Define Slack Config [☐]
**Description:** Add Slack gateway configuration
**Files:**
- Modify `internal/config/gateway_config.go`

**Acceptance Criteria:**
- [ ] `SlackConfig` struct defined
- [ ] Fields: Enabled, BotToken, AppToken, AllowedChannels
- [ ] Struct has mapstructure tags

**Tests:**
- [ ] Unit test: Config loads from YAML
- [ ] Unit test: Config loads from env vars

**Estimated Time:** 1 hour

---

#### Task 6.3: Implement Slack Gateway Structure [☐]
**Description:** Create Slack gateway skeleton
**Files:**
- Create `internal/adapter/gateway/slack/gateway.go`

**Acceptance Criteria:**
- [ ] `Gateway` struct defined
- [ ] Implements `domain.Gateway` interface
- [ ] `New()` constructor creates Socket Mode client

**Tests:**
- [ ] Unit test: New() creates gateway

**Estimated Time:** 1 hour

---

#### Task 6.4: Implement Slack Start/Stop [☐]
**Description:** Implement gateway lifecycle with Socket Mode
**Files:**
- Modify `internal/adapter/gateway/slack/gateway.go`

**Acceptance Criteria:**
- [ ] `Start()` begins Socket Mode connection
- [ ] `Stop()` gracefully disconnects
- [ ] `Platform()` returns PlatformSlack

**Tests:**
- [ ] Unit test: Start() connects
- [ ] Unit test: Stop() disconnects

**Estimated Time:** 2 hours

---

#### Task 6.5: Implement Slack Message Receiving [☐]
**Description:** Parse incoming Slack events
**Files:**
- Modify `internal/adapter/gateway/slack/gateway.go`

**Acceptance Criteria:**
- [ ] Listens for `app_mention` and `message` events
- [ ] Parses events to domain.IncomingMessage
- [ ] PlatformUID is Slack user ID
- [ ] Metadata includes message_ts, channel, channel_type, thread_ts
- [ ] Handler is called for each message
- [ ] Acks events properly

**Tests:**
- [ ] Unit test: Event parsing for app_mention
- [ ] Unit test: Event parsing for direct message
- [ ] Integration test: Receive real event (manual)

**Estimated Time:** 4 hours

---

#### Task 6.6: Implement Slack Message Sending [☐]
**Description:** Send responses to Slack channels
**Files:**
- Modify `internal/adapter/gateway/slack/gateway.go`

**Acceptance Criteria:**
- [ ] `Send()` posts message to channel
- [ ] Replies in thread if original message was in thread
- [ ] Supports markdown formatting
- [ ] Handles errors (channel not found, not in channel)

**Tests:**
- [ ] Unit test: Send() calls Slack API
- [ ] Unit test: Thread reply works
- [ ] Integration test: Send real message (manual)

**Estimated Time:** 3 hours

---

#### Task 6.7: Wire Slack Gateway to Main [☐]
**Description:** Initialize Slack gateway in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Acceptance Criteria:**
- [ ] Slack gateway created if enabled
- [ ] OnMessage wired to ChatService
- [ ] Gateway starts with application
- [ ] Gateway stops on shutdown

**Tests:**
- [ ] Integration test: Full message flow via Slack

**Estimated Time:** 2 hours

---

**Total Estimated Time: 13-14 hours (2-3 days)**

---

## Priority 4: Skill Expansion (Week 5)

### 7. Skills Agent: Weather Skill

#### Task 7.1: Define Weather Skill Config [☐]
**Description:** Add weather skill configuration
**Files:**
- Modify `internal/config/skills_config.go`

**Acceptance Criteria:**
- [ ] `WeatherSkillConfig` struct defined
- [ ] Fields: Enabled, APIKey, DefaultLocation, CacheTTL
- [ ] Struct has mapstructure tags

**Tests:**
- [ ] Unit test: Config loads from YAML
- [ ] Unit test: Config loads from env vars

**Estimated Time:** 1 hour

---

#### Task 7.2: Implement Weather Skill Structure [☐]
**Description:** Create weather skill skeleton
**Files:**
- Create `internal/skills/weather/weather.go`

**Acceptance Criteria:**
- [ ] `Skill` struct defined
- [ ] Implements `domain.Skill` interface
- [ ] `New()` constructor initializes cache
- [ ] `Name()` returns "weather"
- [ ] `Description()` returns skill description
- [ ] `InputSchema()` returns JSON schema

**Tests:**
- [ ] Unit test: Name() returns "weather"
- [ ] Unit test: InputSchema() is valid

**Estimated Time:** 1 hour

---

#### Task 7.3: Implement Weather API Client [☐]
**Description:** Fetch weather data from OpenWeatherMap
**Files:**
- Modify `internal/skills/weather/weather.go`

**Acceptance Criteria:**
- [ ] `fetchWeather()` makes HTTP request to OpenWeatherMap
- [ ] Parses JSON response
- [ ] Handles errors (404 not found, 401 invalid key, 429 rate limit)
- [ ] Returns WeatherResponse struct

**Tests:**
- [ ] Unit test: fetchWeather() with mock HTTP server
- [ ] Unit test: Error handling for 404
- [ ] Unit test: Error handling for 401
- [ ] Unit test: Error handling for 429

**Estimated Time:** 3 hours

---

#### Task 7.4: Implement Weather Skill Execution [☐]
**Description:** Execute weather skill with caching
**Files:**
- Modify `internal/skills/weather/weather.go`

**Acceptance Criteria:**
- [ ] `Execute()` validates parameters
- [ ] Checks cache before API call
- [ ] Fetches from API if not cached
- [ ] Caches results for CacheTTL minutes
- [ ] Formats output as human-readable text
- [ ] Returns domain.SkillResult

**Tests:**
- [ ] Unit test: Execute() with valid location
- [ ] Unit test: Cache hit avoids API call
- [ ] Unit test: Cache miss calls API
- [ ] Unit test: Cache expiration works
- [ ] Integration test: Real API call (CI only)

**Estimated Time:** 3 hours

---

#### Task 7.5: Register Weather Skill [☐]
**Description:** Register weather skill in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`
- Modify `internal/usecase/skill/permissions.go`

**Acceptance Criteria:**
- [ ] Weather skill registered if enabled
- [ ] Permission set to RoleUser
- [ ] Skill appears in skill list

**Tests:**
- [ ] Integration test: Execute weather skill

**Estimated Time:** 1 hour

---

**Total Estimated Time: 9 hours (1-2 days)**

---

### 8. Skills Agent: Web Search Skill

#### Task 8.1: Define Web Search Skill Config [☐]
**Description:** Add web search skill configuration
**Files:**
- Modify `internal/config/skills_config.go`

**Acceptance Criteria:**
- [ ] `WebSearchSkillConfig` struct defined
- [ ] Fields: Enabled, Provider, APIKey, MaxResults
- [ ] Struct has mapstructure tags

**Tests:**
- [ ] Unit test: Config loads from YAML
- [ ] Unit test: Config loads from env vars

**Estimated Time:** 1 hour

---

#### Task 8.2: Implement Web Search Skill Structure [☐]
**Description:** Create web search skill skeleton
**Files:**
- Create `internal/skills/web_search/web_search.go`

**Acceptance Criteria:**
- [ ] `Skill` struct defined
- [ ] Implements `domain.Skill` interface
- [ ] `New()` constructor
- [ ] `Name()` returns "web_search"
- [ ] `Description()` returns skill description
- [ ] `InputSchema()` returns JSON schema

**Tests:**
- [ ] Unit test: Name() returns "web_search"
- [ ] Unit test: InputSchema() is valid

**Estimated Time:** 1 hour

---

#### Task 8.3: Implement DuckDuckGo Search Client [☐]
**Description:** Search using DuckDuckGo Instant Answer API
**Files:**
- Modify `internal/skills/web_search/web_search.go`

**Acceptance Criteria:**
- [ ] `search()` makes HTTP request to DuckDuckGo
- [ ] Parses JSON response
- [ ] Extracts Abstract and RelatedTopics
- [ ] Returns search results

**Tests:**
- [ ] Unit test: search() with mock HTTP server
- [ ] Unit test: Parse valid response
- [ ] Unit test: Handle no results

**Estimated Time:** 2 hours

---

#### Task 8.4: Implement Web Search Skill Execution [☐]
**Description:** Execute web search skill
**Files:**
- Modify `internal/skills/web_search/web_search.go`

**Acceptance Criteria:**
- [ ] `Execute()` validates parameters
- [ ] Calls search API
- [ ] Formats results as JSON
- [ ] Returns top N results (configurable)
- [ ] Returns domain.SkillResult

**Tests:**
- [ ] Unit test: Execute() with valid query
- [ ] Unit test: MaxResults limit works
- [ ] Unit test: No results handled gracefully
- [ ] Integration test: Real search (CI only)

**Estimated Time:** 2 hours

---

#### Task 8.5: Register Web Search Skill [☐]
**Description:** Register web search skill in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`
- Modify `internal/usecase/skill/permissions.go`

**Acceptance Criteria:**
- [ ] Web search skill registered if enabled
- [ ] Permission set to RoleUser
- [ ] Skill appears in skill list

**Tests:**
- [ ] Integration test: Execute web_search skill

**Estimated Time:** 1 hour

---

**Total Estimated Time: 7 hours (1-2 days)**

---

### 9. Skills Agent: Notes Skill

#### Task 9.1: Define Note Entity [☐]
**Description:** Create Note domain entity
**Files:**
- Create `internal/domain/note.go`

**Acceptance Criteria:**
- [ ] `Note` struct defined
- [ ] Fields: ID, UserID, Title, Content, CreatedAt, UpdatedAt
- [ ] Validation rules documented

**Tests:**
- [ ] None (data structure only)

**Estimated Time:** 30 minutes

---

#### Task 9.2: Define Notes Repository Interface [☐]
**Description:** Create repository interface for notes
**Files:**
- Create `internal/usecase/notes/repository.go`

**Acceptance Criteria:**
- [ ] `NotesRepository` interface defined
- [ ] Methods: CreateNote, GetNote, ListNotes, UpdateNote, DeleteNote, CountUserNotes

**Tests:**
- [ ] None (interface only)

**Estimated Time:** 30 minutes

---

#### Task 9.3: Database Migration for Notes [☐]
**Description:** Add notes table to database
**Files:**
- Modify `internal/adapter/repository/sqlite/schema.go`

**Acceptance Criteria:**
- [ ] Migration V3 creates notes table
- [ ] Columns: id, user_id, title, content, created_at, updated_at
- [ ] Foreign key constraint on user_id
- [ ] Indexes on user_id and created_at
- [ ] CHECK constraints on title and content length

**Tests:**
- [ ] Integration test: Migration runs successfully

**Estimated Time:** 1 hour

---

#### Task 9.4: Implement SQLite Notes Repository [☐]
**Description:** Implement NotesRepository for SQLite
**Files:**
- Create `internal/adapter/repository/sqlite/notes.go`

**Acceptance Criteria:**
- [ ] All repository methods implemented
- [ ] CreateNote generates UUID
- [ ] GetNote retrieves by ID
- [ ] ListNotes filters by user_id
- [ ] UpdateNote updates content/title and updated_at
- [ ] DeleteNote removes note
- [ ] CountUserNotes counts user's notes

**Tests:**
- [ ] Unit test: CreateNote inserts note
- [ ] Unit test: GetNote retrieves note
- [ ] Unit test: ListNotes filters by user
- [ ] Unit test: UpdateNote updates note
- [ ] Unit test: DeleteNote removes note
- [ ] Unit test: CountUserNotes returns count
- [ ] Integration test: Full CRUD lifecycle

**Estimated Time:** 4 hours

---

#### Task 9.5: Define Notes Skill Config [☐]
**Description:** Add notes skill configuration
**Files:**
- Modify `internal/config/skills_config.go`

**Acceptance Criteria:**
- [ ] `NotesSkillConfig` struct defined
- [ ] Fields: Enabled, MaxNoteLength, MaxNotesPerUser

**Tests:**
- [ ] Unit test: Config loads from YAML

**Estimated Time:** 30 minutes

---

#### Task 9.6: Implement Notes Skill Structure [☐]
**Description:** Create notes skill skeleton
**Files:**
- Create `internal/skills/notes/notes.go`

**Acceptance Criteria:**
- [ ] `Skill` struct defined
- [ ] Implements `domain.Skill` interface
- [ ] Holds reference to NotesRepository
- [ ] `Name()` returns "notes"
- [ ] `Description()` returns skill description
- [ ] `InputSchema()` returns JSON schema with commands

**Tests:**
- [ ] Unit test: Name() returns "notes"
- [ ] Unit test: InputSchema() is valid

**Estimated Time:** 1 hour

---

#### Task 9.7: Implement Notes Skill Commands [☐]
**Description:** Implement create, list, get, update, delete commands
**Files:**
- Modify `internal/skills/notes/notes.go`

**Acceptance Criteria:**
- [ ] `Execute()` dispatches to command handlers
- [ ] `handleCreate()` creates note
- [ ] `handleList()` lists user's notes
- [ ] `handleGet()` retrieves note by ID
- [ ] `handleUpdate()` updates note
- [ ] `handleDelete()` deletes note
- [ ] Enforces user isolation (can only access own notes)
- [ ] Enforces MaxNoteLength and MaxNotesPerUser limits

**Tests:**
- [ ] Unit test: Create command with valid params
- [ ] Unit test: List command returns user's notes
- [ ] Unit test: Get command retrieves note
- [ ] Unit test: Update command updates note
- [ ] Unit test: Delete command removes note
- [ ] Unit test: User cannot access another user's note
- [ ] Unit test: MaxNoteLength enforced
- [ ] Unit test: MaxNotesPerUser enforced
- [ ] Integration test: Full CRUD via skill

**Estimated Time:** 5 hours

---

#### Task 9.8: Register Notes Skill [☐]
**Description:** Register notes skill in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`
- Modify `internal/usecase/skill/permissions.go`

**Acceptance Criteria:**
- [ ] NotesRepository initialized
- [ ] Notes skill registered if enabled
- [ ] Permission set to RoleUser
- [ ] Skill appears in skill list

**Tests:**
- [ ] Integration test: Execute notes skill commands

**Estimated Time:** 1 hour

---

**Total Estimated Time: 13-14 hours (2-3 days)**

---

## Integration & QA Tasks

### 10. Integration Agent: Final Assembly

#### Task 10.1: Update Main Application Assembly [☐]
**Description:** Wire all new components in main.go
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Acceptance Criteria:**
- [ ] All new gateways initialized if enabled
- [ ] All new LLM providers registered
- [ ] All new skills registered
- [ ] UserService initialized
- [ ] Graceful shutdown handles all gateways

**Tests:**
- [ ] Integration test: All gateways start
- [ ] Integration test: All providers available
- [ ] Integration test: All skills registered

**Estimated Time:** 3 hours

---

#### Task 10.2: Test Multi-Gateway Operation [☐]
**Description:** Verify all gateways work simultaneously
**Files:**
- None (testing task)

**Acceptance Criteria:**
- [ ] CLI, Telegram, and Slack all running at once
- [ ] Messages from each gateway processed correctly
- [ ] No race conditions or conflicts

**Tests:**
- [ ] Manual test: Send message via CLI
- [ ] Manual test: Send message via Telegram
- [ ] Manual test: Send message via Slack

**Estimated Time:** 2 hours

---

#### Task 10.3: Test Multi-Provider Operation [☐]
**Description:** Verify provider switching works
**Files:**
- None (testing task)

**Acceptance Criteria:**
- [ ] Can switch between Anthropic, OpenAI, Ollama
- [ ] Tool calling works with each provider
- [ ] Graceful fallback if provider unavailable

**Tests:**
- [ ] Manual test: Use Anthropic provider
- [ ] Manual test: Use OpenAI provider
- [ ] Manual test: Use Ollama provider

**Estimated Time:** 2 hours

---

#### Task 10.4: Verify All Quality Gates Pass [☐]
**Description:** Run all quality gates
**Files:**
- None (testing task)

**Acceptance Criteria:**
- [ ] `go fmt ./...` - no changes
- [ ] `go mod tidy` - no changes
- [ ] `go vet ./...` - no warnings
- [ ] `golangci-lint run` - no errors
- [ ] `go test ./...` - all tests pass
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` - builds successfully
- [ ] `./bin/nuimanbot --help` - runs without errors

**Tests:**
- [ ] All quality gates pass

**Estimated Time:** 1 hour

---

**Total Estimated Time: 8 hours (1 day)**

---

### 11. QA Agent: Documentation

#### Task 11.1: Update README.md [☐]
**Description:** Document Phase 2 features
**Files:**
- Modify `README.md`

**Acceptance Criteria:**
- [ ] Phase 2 features listed
- [ ] New gateways documented (Telegram, Slack)
- [ ] New LLM providers documented (OpenAI, Ollama)
- [ ] New skills documented (weather, web_search, notes)
- [ ] Admin commands documented
- [ ] Configuration examples updated

**Tests:**
- [ ] None

**Estimated Time:** 3 hours

---

#### Task 11.2: Update STATUS.md [☐]
**Description:** Update project status metrics
**Files:**
- Modify `STATUS.md`

**Acceptance Criteria:**
- [ ] Phase 2 completion noted
- [ ] Test coverage metrics updated
- [ ] New components listed

**Tests:**
- [ ] None

**Estimated Time:** 1 hour

---

#### Task 11.3: Create Telegram Setup Guide [☐]
**Description:** Document how to set up Telegram bot
**Files:**
- Create `docs/telegram-setup.md` (or add to README)

**Acceptance Criteria:**
- [ ] Instructions to create bot with @BotFather
- [ ] Instructions to get bot token
- [ ] Configuration example

**Tests:**
- [ ] None

**Estimated Time:** 1 hour

---

#### Task 11.4: Create Slack Setup Guide [☐]
**Description:** Document how to set up Slack bot
**Files:**
- Create `docs/slack-setup.md` (or add to README)

**Acceptance Criteria:**
- [ ] Instructions to create Slack app
- [ ] Instructions to enable Socket Mode
- [ ] Required scopes documented
- [ ] Configuration example

**Tests:**
- [ ] None

**Estimated Time:** 1 hour

---

#### Task 11.5: Update PRODUCT_REQUIREMENT_DOC.md [☐]
**Description:** Mark Phase 2 as complete
**Files:**
- Modify `PRODUCT_REQUIREMENT_DOC.md`

**Acceptance Criteria:**
- [ ] Phase 2 tasks marked complete
- [ ] Status updated

**Tests:**
- [ ] None

**Estimated Time:** 30 minutes

---

**Total Estimated Time: 6-7 hours (1 day)**

---

## Summary

### Total Task Count

| Agent | Tasks | Estimated Days |
|-------|-------|----------------|
| Security Agent (RBAC) | 6 | 1-2 |
| User Management Agent | 4 | 2 |
| LLM Provider Agent (OpenAI) | 7 | 2-3 |
| LLM Provider Agent (Ollama) | 5 | 1-2 |
| Gateway Agent (Telegram) | 7 | 2-3 |
| Gateway Agent (Slack) | 7 | 2-3 |
| Skills Agent (Weather) | 5 | 1-2 |
| Skills Agent (Web Search) | 5 | 1-2 |
| Skills Agent (Notes) | 8 | 2-3 |
| Integration Agent | 4 | 1 |
| QA Agent | 5 | 1 |
| **TOTAL** | **63** | **18-25** |

### Priority Summary

**Week 1 (Priority 1):** RBAC + User Management
**Week 2 (Priority 2):** OpenAI + Ollama Providers
**Week 3-4 (Priority 3):** Telegram + Slack Gateways
**Week 5 (Priority 4):** Weather + Web Search + Notes Skills

### Success Metrics

Phase 2 is complete when all 63 tasks are marked ✅ and:
- [ ] All quality gates pass
- [ ] Test coverage ≥75%
- [ ] All gateways operational
- [ ] All providers operational
- [ ] All skills operational
- [ ] Documentation updated

---

## Next Steps

1. ✅ Complete spec.md
2. ✅ Complete research.md
3. ✅ Complete data-dictionary.md
4. ✅ Complete plan.md
5. ✅ Complete tasks.md (this document)
6. ⏳ **BEGIN IMPLEMENTATION** - Start with Task 1.1 (RBAC Permission Matrix)

Ready to begin Phase 2 implementation! 🚀
