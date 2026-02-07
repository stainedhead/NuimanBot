# Phase 2: Multi-Platform - Implementation Plan

**Last Updated:** 2026-02-06
**Status:** Planning Complete - Ready for Implementation

---

## 1. Executive Summary

Phase 2 extends NuimanBot from a single-platform CLI tool to a multi-platform conversational agent with:
- **3 gateways** (CLI, Telegram, Slack)
- **3 LLM providers** (Anthropic, OpenAI, Ollama)
- **5 skills** (calculator, datetime, weather, web_search, notes)
- **Full RBAC enforcement** with role-based skill access
- **User management** with admin commands

This plan follows the same sub-agent collaboration model proven successful in Phase 1.

---

## 2. Implementation Strategy

### 2.1. Principles

1. **Build on Phase 1 Foundation** - Reuse existing interfaces and patterns
2. **Follow Clean Architecture** - Maintain strict layer separation
3. **TDD Required** - Red-Green-Refactor for all new code
4. **Parallel Development** - Use sub-agents for independent components
5. **Quality Gates** - All gates must pass before completion

### 2.2. Development Order

**Rationale for ordering:**
- RBAC first (security foundation for all features)
- User management next (enables admin operations)
- LLM providers (enables provider diversity)
- Gateways (extends platform reach)
- Skills (adds practical capabilities)

**Priority 1: Security & Foundation (Week 1)**
1. RBAC enforcement
2. User management service

**Priority 2: Provider Expansion (Week 2)**
3. OpenAI provider
4. Ollama provider

**Priority 3: Gateway Expansion (Week 3-4)**
5. Telegram gateway
6. Slack gateway

**Priority 4: Skill Expansion (Week 5)**
7. Weather skill
8. Web search skill
9. Notes skill

---

## 3. Sub-Agent Assignments

### 3.1. Security Agent: RBAC Enforcement

**Responsibility:** Implement comprehensive RBAC throughout the application

**Tasks:**
1. Define `SkillPermissions` map in `internal/usecase/skill/permissions.go`
2. Implement `checkPermission()` in `SkillExecutionService`
3. Update `User` entity with `AllowedSkills` field
4. Add database migration for `allowed_skills` column
5. Implement permission denied error handling
6. Add audit logging for permission violations
7. Write unit tests for all permission scenarios
8. Write integration tests for RBAC flow

**Files to Modify:**
- `internal/domain/user.go` - Add `AllowedSkills` field
- `internal/usecase/skill/service.go` - Add permission checks
- `internal/usecase/skill/permissions.go` - Create with permission matrix
- `internal/adapter/repository/sqlite/user.go` - Handle `allowed_skills` column
- `internal/adapter/repository/sqlite/schema.go` - Add migration
- `internal/domain/errors.go` - Add permission-related errors

**Dependencies:** None (builds on Phase 1)

**Estimated Time:** 2-3 days

---

### 3.2. User Management Agent: Admin Commands

**Responsibility:** Implement user lifecycle management

**Tasks:**
1. Define `UserService` interface in `internal/usecase/user/service.go`
2. Implement `UserService` with CRUD operations
3. Add business logic (e.g., can't delete last admin)
4. Add admin command parser to CLI gateway
5. Implement `/admin user create|list|get|update|delete` commands
6. Add audit logging for all user management operations
7. Write unit tests for `UserService`
8. Write integration tests for admin commands

**Files to Create:**
- `internal/usecase/user/service.go` - Service interface and implementation
- `internal/usecase/user/service_test.go` - Unit tests

**Files to Modify:**
- `internal/adapter/gateway/cli/gateway.go` - Add admin command parsing
- `cmd/nuimanbot/main.go` - Initialize UserService

**Dependencies:** Security Agent (for permission checks)

**Estimated Time:** 2-3 days

---

### 3.3. LLM Provider Agent: OpenAI & Ollama

**Responsibility:** Implement OpenAI and Ollama LLM providers

**Tasks:**

**OpenAI:**
1. Define `OpenAIProviderConfig` in `internal/config/llm_config.go`
2. Implement `internal/infrastructure/llm/openai/client.go`
3. Implement `Complete()` method (non-streaming)
4. Implement `Stream()` method (streaming)
5. Implement tool calling (convert NuimanBot skills to OpenAI functions)
6. Handle errors (rate limits, context length, invalid key)
7. Write unit tests with mocked OpenAI API
8. Write integration tests with real OpenAI API

**Ollama:**
1. Define `OllamaProviderConfig` in `internal/config/llm_config.go`
2. Implement `internal/infrastructure/llm/ollama/client.go`
3. Implement `Complete()` method using `/api/chat` endpoint
4. Implement `Stream()` method (line-delimited JSON parsing)
5. Handle connection errors (Ollama not running)
6. Write unit tests with mocked Ollama API
7. Write integration tests with local Ollama server

**Files to Create:**
- `internal/infrastructure/llm/openai/client.go`
- `internal/infrastructure/llm/openai/client_test.go`
- `internal/infrastructure/llm/ollama/client.go`
- `internal/infrastructure/llm/ollama/client_test.go`

**Files to Modify:**
- `internal/config/llm_config.go` - Add new provider configs
- `cmd/nuimanbot/main.go` - Register new providers

**Dependencies:** None (builds on Phase 1 LLM interfaces)

**Estimated Time:** 4-5 days (2-3 per provider)

---

### 3.4. Gateway Agent: Telegram & Slack

**Responsibility:** Implement Telegram and Slack gateways

**Tasks:**

**Telegram:**
1. Install `github.com/go-telegram/bot` dependency
2. Define `TelegramConfig` in `internal/config/gateway_config.go`
3. Implement `internal/adapter/gateway/telegram/gateway.go`
4. Implement `Gateway` interface (Start, Stop, Send, OnMessage)
5. Use long polling mode
6. Parse incoming messages to `IncomingMessage`
7. Format outgoing messages with Telegram markdown
8. Handle `/start` command
9. Handle errors gracefully (rate limits, invalid tokens)
10. Write unit tests with mocked Telegram API
11. Write integration tests with test bot token

**Slack:**
1. Install `github.com/slack-go/slack` dependency
2. Define `SlackConfig` in `internal/config/gateway_config.go`
3. Implement `internal/adapter/gateway/slack/gateway.go`
4. Implement `Gateway` interface using Socket Mode
5. Listen for `app_mention` and `message` events
6. Parse events to `IncomingMessage`
7. Format outgoing messages (markdown or Block Kit)
8. Handle threading (reply in thread if message is in thread)
9. Write unit tests with mocked Slack API
10. Write integration tests with test workspace

**Files to Create:**
- `internal/adapter/gateway/telegram/gateway.go`
- `internal/adapter/gateway/telegram/gateway_test.go`
- `internal/adapter/gateway/slack/gateway.go`
- `internal/adapter/gateway/slack/gateway_test.go`

**Files to Modify:**
- `internal/config/gateway_config.go` - Add new gateway configs
- `cmd/nuimanbot/main.go` - Initialize and start new gateways
- `go.mod` - Add new dependencies

**Dependencies:** None (implements Phase 1 Gateway interface)

**Estimated Time:** 6-8 days (3-4 per gateway)

---

### 3.5. Skills Agent: Weather, Web Search, Notes

**Responsibility:** Implement three new skills

**Tasks:**

**Weather Skill:**
1. Define `WeatherSkillConfig` in `internal/config/skills_config.go`
2. Implement `internal/skills/weather/weather.go`
3. Implement OpenWeatherMap API client
4. Implement `Execute()` with parameters (location, days, units)
5. Implement response caching (30 min TTL)
6. Handle errors (invalid location, rate limits, invalid API key)
7. Write unit tests with mocked weather API
8. Write integration tests with real API

**Web Search Skill:**
1. Define `WebSearchSkillConfig` in `internal/config/skills_config.go`
2. Implement `internal/skills/web_search/web_search.go`
3. Implement DuckDuckGo Instant Answer API client
4. Implement `Execute()` with parameters (query, num_results)
5. Parse and format search results
6. Handle no results gracefully
7. Write unit tests with mocked search API
8. Write integration tests with real API

**Notes Skill:**
1. Define `NotesSkillConfig` in `internal/config/skills_config.go`
2. Define `Note` entity in `internal/domain/note.go`
3. Define `NotesRepository` interface in `internal/usecase/notes/repository.go`
4. Implement `internal/adapter/repository/sqlite/notes.go`
5. Implement `internal/skills/notes/notes.go`
6. Implement `Execute()` with commands (create, list, get, update, delete)
7. Enforce user isolation (user can only access their own notes)
8. Enforce note limits (max length, max notes per user)
9. Add `notes` table to database schema
10. Write unit tests for NotesRepository
11. Write unit tests for notes skill
12. Write integration tests for full note lifecycle

**Files to Create:**
- `internal/skills/weather/weather.go`
- `internal/skills/weather/weather_test.go`
- `internal/skills/web_search/web_search.go`
- `internal/skills/web_search/web_search_test.go`
- `internal/skills/notes/notes.go`
- `internal/skills/notes/notes_test.go`
- `internal/domain/note.go`
- `internal/usecase/notes/repository.go`
- `internal/adapter/repository/sqlite/notes.go`
- `internal/adapter/repository/sqlite/notes_test.go`

**Files to Modify:**
- `internal/config/skills_config.go` - Add new skill configs
- `internal/adapter/repository/sqlite/schema.go` - Add notes table
- `cmd/nuimanbot/main.go` - Register new skills

**Dependencies:**
- Notes skill depends on NotesRepository
- Weather skill independent
- Web search skill independent

**Estimated Time:** 7-10 days (2-3 per skill)

---

### 3.6. Integration Agent: Final Assembly

**Responsibility:** Integrate all components and ensure everything works together

**Tasks:**
1. Update `cmd/nuimanbot/main.go` with dependency injection for all new components
2. Test gateway switching (CLI, Telegram, Slack all running simultaneously)
3. Test provider switching (Anthropic, OpenAI, Ollama)
4. Test RBAC enforcement across all gateways
5. Test skill execution from all gateways
6. Verify graceful shutdown for all gateways
7. Update configuration file examples
8. Verify all quality gates pass

**Files to Modify:**
- `cmd/nuimanbot/main.go` - Wire up all new components

**Dependencies:** All other agents

**Estimated Time:** 2-3 days

---

### 3.7. QA Agent: Testing & Documentation

**Responsibility:** Ensure quality standards and documentation

**Tasks:**
1. Run all unit tests (`go test ./...`)
2. Run all integration tests
3. Measure test coverage (target: 75%+)
4. Run linter (`golangci-lint run`)
5. Fix any linter warnings
6. Update README.md with Phase 2 features
7. Update STATUS.md with current metrics
8. Create user guide for Telegram/Slack setup
9. Document admin commands
10. Document new skills

**Files to Modify:**
- `README.md` - Add Phase 2 features
- `STATUS.md` - Update metrics
- `PRODUCT_REQUIREMENT_DOC.md` - Mark Phase 2 complete

**Dependencies:** All other agents

**Estimated Time:** 2-3 days

---

## 4. Detailed Implementation Plans

### 4.1. RBAC Enforcement Implementation

**Step 1: Define Permission Matrix**
```go
// internal/usecase/skill/permissions.go
package skill

import "internal/domain"

var SkillPermissions = map[string]domain.Role{
    "calculator":   domain.RoleGuest,
    "datetime":     domain.RoleGuest,
    "weather":      domain.RoleUser,
    "web_search":   domain.RoleUser,
    "notes":        domain.RoleUser,
    "admin.user":   domain.RoleAdmin,
}
```

**Step 2: Add Permission Check**
```go
// internal/usecase/skill/service.go
func (s *SkillExecutionService) Execute(ctx context.Context, user *domain.User, skillName string, params map[string]interface{}) (*domain.SkillResult, error) {
    // Check permissions
    if err := s.checkPermission(user, skillName); err != nil {
        s.securityService.Audit(ctx, domain.AuditEvent{
            UserID:    user.ID,
            Action:    "skill_execution_denied",
            Resource:  skillName,
            Timestamp: time.Now(),
        })
        return nil, err
    }

    // Rest of existing logic...
}

func (s *SkillExecutionService) checkPermission(user *domain.User, skillName string) error {
    // Get required role for skill
    required, ok := SkillPermissions[skillName]
    if !ok {
        required = domain.RoleUser // Default: require User+
    }

    // Check user role
    if user.Role < required {
        return domain.ErrInsufficientPermissions
    }

    // Check AllowedSkills override
    if len(user.AllowedSkills) > 0 {
        allowed := false
        for _, s := range user.AllowedSkills {
            if s == skillName {
                allowed = true
                break
            }
        }
        if !allowed {
            return domain.ErrInsufficientPermissions
        }
    }

    return nil
}
```

**Step 3: Update User Entity**
```go
// internal/domain/user.go
type User struct {
    ID            string
    Platform      Platform
    PlatformUID   string
    Role          Role
    AllowedSkills []string  // NEW FIELD
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

**Step 4: Database Migration**
```go
// internal/adapter/repository/sqlite/schema.go
const migrationV2 = `
ALTER TABLE users ADD COLUMN allowed_skills TEXT DEFAULT '[]';
`
```

---

### 4.2. Telegram Gateway Implementation

**Step 1: Install Dependency**
```bash
go get github.com/go-telegram/bot
```

**Step 2: Define Config**
```go
// internal/config/gateway_config.go
type TelegramConfig struct {
    Enabled      bool         `mapstructure:"enabled" yaml:"enabled"`
    BotToken     SecureString `mapstructure:"bot_token" yaml:"bot_token"`
    AllowedUsers []string     `mapstructure:"allowed_users" yaml:"allowed_users"`
}
```

**Step 3: Implement Gateway**
```go
// internal/adapter/gateway/telegram/gateway.go
package telegram

import (
    "context"
    "github.com/go-telegram/bot"
    "internal/domain"
)

type Gateway struct {
    config  *TelegramConfig
    bot     *bot.Bot
    handler domain.MessageHandler
}

func New(config *TelegramConfig) (*Gateway, error) {
    opts := []bot.Option{
        bot.WithDefaultHandler(g.handleUpdate),
    }

    b, err := bot.New(config.BotToken.String(), opts...)
    if err != nil {
        return nil, err
    }

    return &Gateway{
        config: config,
        bot:    b,
    }, nil
}

func (g *Gateway) Platform() domain.Platform {
    return domain.PlatformTelegram
}

func (g *Gateway) Start(ctx context.Context) error {
    go g.bot.Start(ctx)
    return nil
}

func (g *Gateway) Stop(ctx context.Context) error {
    // Graceful shutdown
    return nil
}

func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
    _, err := g.bot.SendMessage(ctx, &bot.SendMessageParams{
        ChatID:    msg.RecipientID,
        Text:      msg.Text,
        ParseMode: models.ParseModeMarkdown,
    })
    return err
}

func (g *Gateway) OnMessage(handler domain.MessageHandler) {
    g.handler = handler
}

func (g *Gateway) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
    if update.Message == nil {
        return
    }

    msg := domain.IncomingMessage{
        ID:          fmt.Sprintf("%d", update.Message.ID),
        Platform:    domain.PlatformTelegram,
        PlatformUID: fmt.Sprintf("%d", update.Message.From.ID),
        Text:        update.Message.Text,
        Timestamp:   time.Unix(int64(update.Message.Date), 0),
        Metadata: map[string]interface{}{
            "telegram": map[string]interface{}{
                "message_id": update.Message.ID,
                "chat_id":    update.Message.Chat.ID,
                "chat_type":  update.Message.Chat.Type,
            },
        },
    }

    if g.handler != nil {
        g.handler(ctx, msg)
    }
}
```

**Step 4: Wire in main.go**
```go
// cmd/nuimanbot/main.go
if config.Gateways.Telegram.Enabled {
    telegramGateway, err := telegram.New(&config.Gateways.Telegram)
    if err != nil {
        log.Fatal("failed to create telegram gateway", "error", err)
    }

    telegramGateway.OnMessage(chatService.HandleMessage)

    if err := telegramGateway.Start(ctx); err != nil {
        log.Fatal("failed to start telegram gateway", "error", err)
    }

    defer telegramGateway.Stop(context.Background())
}
```

---

### 4.3. OpenAI Provider Implementation

**Step 1: Install Dependency**
```bash
go get github.com/sashabaranov/go-openai
```

**Step 2: Define Config**
```go
// internal/config/llm_config.go
type OpenAIProviderConfig struct {
    Type         string       `mapstructure:"type" yaml:"type"`
    APIKey       SecureString `mapstructure:"api_key" yaml:"api_key"`
    DefaultModel string       `mapstructure:"default_model" yaml:"default_model"`
    Organization string       `mapstructure:"organization" yaml:"organization"`
    BaseURL      string       `mapstructure:"base_url" yaml:"base_url"`
}
```

**Step 3: Implement Provider**
```go
// internal/infrastructure/llm/openai/client.go
package openai

import (
    "context"
    "github.com/sashabaranov/go-openai"
    "internal/domain"
)

type Client struct {
    client       *openai.Client
    defaultModel string
}

func New(config *OpenAIProviderConfig) (*Client, error) {
    client := openai.NewClient(config.APIKey.String())

    if config.Organization != "" {
        client = openai.NewOrgClient(config.APIKey.String(), config.Organization)
    }

    return &Client{
        client:       client,
        defaultModel: config.DefaultModel,
    }, nil
}

func (c *Client) Complete(ctx context.Context, req *domain.LLMRequest) (*domain.LLMResponse, error) {
    messages := c.convertMessages(req.Messages)
    tools := c.convertTools(req.Tools)

    resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       c.getModel(req),
        Messages:    messages,
        Tools:       tools,
        Temperature: float32(req.Temperature),
        MaxTokens:   req.MaxTokens,
    })

    if err != nil {
        return nil, err
    }

    return c.convertResponse(&resp), nil
}

func (c *Client) convertMessages(msgs []domain.Message) []openai.ChatCompletionMessage {
    // Convert domain.Message to openai.ChatCompletionMessage
    // ...
}

func (c *Client) convertTools(tools []domain.ToolDefinition) []openai.Tool {
    // Convert domain.ToolDefinition to openai.Tool
    // ...
}

func (c *Client) convertResponse(resp *openai.ChatCompletionResponse) *domain.LLMResponse {
    // Convert openai.ChatCompletionResponse to domain.LLMResponse
    // ...
}
```

---

### 4.4. Weather Skill Implementation

**Step 1: Define Config**
```go
// internal/config/skills_config.go
type WeatherSkillConfig struct {
    Enabled         bool         `mapstructure:"enabled" yaml:"enabled"`
    APIKey          SecureString `mapstructure:"api_key" yaml:"api_key"`
    DefaultLocation string       `mapstructure:"default_location" yaml:"default_location"`
    CacheTTL        int          `mapstructure:"cache_ttl" yaml:"cache_ttl"`
}
```

**Step 2: Implement Skill**
```go
// internal/skills/weather/weather.go
package weather

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    "internal/domain"
)

type Skill struct {
    config *WeatherSkillConfig
    cache  map[string]*cacheEntry
}

type cacheEntry struct {
    data      *WeatherResponse
    expiresAt time.Time
}

func New(config *WeatherSkillConfig) *Skill {
    return &Skill{
        config: config,
        cache:  make(map[string]*cacheEntry),
    }
}

func (s *Skill) Name() string {
    return "weather"
}

func (s *Skill) Description() string {
    return "Get current weather and forecast for a location"
}

func (s *Skill) InputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City name or coordinates",
            },
            "days": map[string]interface{}{
                "type":        "integer",
                "description": "Number of forecast days (1-3)",
                "minimum":     1,
                "maximum":     3,
                "default":     1,
            },
            "units": map[string]interface{}{
                "type":        "string",
                "description": "Temperature units",
                "enum":        []string{"metric", "imperial"},
                "default":     "metric",
            },
        },
        "required": []string{"location"},
    }
}

func (s *Skill) Execute(ctx context.Context, params map[string]interface{}) (*domain.SkillResult, error) {
    location := params["location"].(string)
    days := 1
    if d, ok := params["days"].(int); ok {
        days = d
    }
    units := "metric"
    if u, ok := params["units"].(string); ok {
        units = u
    }

    // Check cache
    cacheKey := fmt.Sprintf("%s:%d:%s", location, days, units)
    if entry, ok := s.cache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
        return s.formatResponse(entry.data, days, units), nil
    }

    // Fetch from API
    weather, err := s.fetchWeather(ctx, location, days, units)
    if err != nil {
        return nil, err
    }

    // Cache result
    s.cache[cacheKey] = &cacheEntry{
        data:      weather,
        expiresAt: time.Now().Add(time.Duration(s.config.CacheTTL) * time.Minute),
    }

    return s.formatResponse(weather, days, units), nil
}

func (s *Skill) fetchWeather(ctx context.Context, location string, days int, units string) (*WeatherResponse, error) {
    url := fmt.Sprintf(
        "https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=%s",
        location,
        s.config.APIKey.String(),
        units,
    )

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("weather API error: %d", resp.StatusCode)
    }

    var weather WeatherResponse
    if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
        return nil, err
    }

    return &weather, nil
}

func (s *Skill) formatResponse(weather *WeatherResponse, days int, units string) *domain.SkillResult {
    unitSymbol := "°C"
    if units == "imperial" {
        unitSymbol = "°F"
    }

    output := fmt.Sprintf(
        "%s: %.1f%s, %s. Humidity: %d%%, Wind: %.1f m/s",
        weather.Name,
        weather.Main.Temp,
        unitSymbol,
        weather.Weather[0].Description,
        weather.Main.Humidity,
        weather.Wind.Speed,
    )

    return &domain.SkillResult{
        Output: output,
    }
}

type WeatherResponse struct {
    Name string `json:"name"`
    Main struct {
        Temp     float64 `json:"temp"`
        Humidity int     `json:"humidity"`
    } `json:"main"`
    Weather []struct {
        Description string `json:"description"`
    } `json:"weather"`
    Wind struct {
        Speed float64 `json:"speed"`
    } `json:"wind"`
}
```

---

## 5. Testing Strategy

### 5.1. Unit Tests

Each component must have comprehensive unit tests:
- Mock external dependencies (APIs, databases)
- Test happy path and error cases
- Test edge cases (empty inputs, max limits)
- Target: 80%+ coverage per component

### 5.2. Integration Tests

Test component interactions:
- Gateway → ChatService → SkillService
- LLM Provider → External API
- Repository → Database
- Run with real (test) credentials where possible

### 5.3. E2E Tests (Manual)

Manual validation of full flows:
1. Send message via Telegram → receive response
2. Send message via Slack → receive response
3. Test RBAC: User tries admin command (should fail)
4. Execute weather skill → verify correct data
5. Execute web_search skill → verify results
6. Create/read/update/delete notes → verify persistence

---

## 6. Database Migrations

### 6.1. Migration V2: RBAC

```sql
-- Add allowed_skills column
ALTER TABLE users ADD COLUMN allowed_skills TEXT DEFAULT '[]';
```

### 6.2. Migration V3: Notes

```sql
-- Create notes table
CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL CHECK(LENGTH(title) <= 200),
    content TEXT NOT NULL CHECK(LENGTH(content) <= 10000),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_created_at ON notes(created_at DESC);
```

---

## 7. Configuration Updates

### 7.1. Environment Variables

**New environment variables needed:**
```bash
# Telegram Gateway
NUIMANBOT_GATEWAYS_TELEGRAM_ENABLED=true
NUIMANBOT_GATEWAYS_TELEGRAM_BOT_TOKEN=<token>

# Slack Gateway
NUIMANBOT_GATEWAYS_SLACK_ENABLED=true
NUIMANBOT_GATEWAYS_SLACK_BOT_TOKEN=<xoxb-token>
NUIMANBOT_GATEWAYS_SLACK_APP_TOKEN=<xapp-token>

# OpenAI Provider
NUIMANBOT_LLM_PROVIDERS_1_TYPE=openai
NUIMANBOT_LLM_PROVIDERS_1_API_KEY=<sk-key>
NUIMANBOT_LLM_PROVIDERS_1_DEFAULT_MODEL=gpt-4o

# Ollama Provider
NUIMANBOT_LLM_PROVIDERS_2_TYPE=ollama
NUIMANBOT_LLM_PROVIDERS_2_BASE_URL=http://localhost:11434
NUIMANBOT_LLM_PROVIDERS_2_DEFAULT_MODEL=llama3

# Weather Skill
NUIMANBOT_SKILLS_WEATHER_ENABLED=true
NUIMANBOT_SKILLS_WEATHER_API_KEY=<openweather-key>
```

---

## 8. Risk Mitigation

### 8.1. External API Dependencies

**Risk:** External APIs may be unavailable or rate-limited

**Mitigation:**
- Implement exponential backoff for retries
- Cache results where appropriate (weather)
- Graceful degradation (show error, don't crash)
- Monitor API usage to stay within limits

### 8.2. Permission Bypass

**Risk:** Users may find ways to bypass RBAC

**Mitigation:**
- Enforce permissions at skill execution time (not just UI)
- Audit all permission checks
- Test thoroughly with different roles
- Code review permission logic carefully

### 8.3. Cross-User Data Access

**Risk:** Users may access other users' notes

**Mitigation:**
- Always filter by user_id in NotesRepository
- Validate user_id matches authenticated user
- Add integration tests for unauthorized access attempts
- Code review data access logic

---

## 9. Quality Gates

All quality gates from Phase 1 must pass:

```bash
# 1. Format code
go fmt ./...

# 2. Tidy dependencies
go mod tidy

# 3. Vet for suspicious constructs
go vet ./...

# 4. Run linter
golangci-lint run

# 5. Run all tests
go test ./...

# 6. Build executable
go build -o bin/nuimanbot ./cmd/nuimanbot

# 7. Verify executable runs
./bin/nuimanbot --help
```

**All must pass with zero errors.**

---

## 10. Timeline

**Total Estimated Time: 20-25 working days (4-5 weeks)**

| Week | Agent | Tasks | Days |
|------|-------|-------|------|
| 1 | Security Agent | RBAC enforcement | 2-3 |
| 1 | User Management Agent | Admin commands | 2-3 |
| 2 | LLM Provider Agent | OpenAI + Ollama | 4-5 |
| 3 | Gateway Agent | Telegram gateway | 3-4 |
| 4 | Gateway Agent | Slack gateway | 3-4 |
| 5 | Skills Agent | Weather + Web Search + Notes | 7-10 |
| 5 | Integration Agent | Final assembly | 2-3 |
| 5 | QA Agent | Testing + Documentation | 2-3 |

**Parallel work opportunities:**
- RBAC + User Management can be done in parallel
- OpenAI + Ollama providers can be done separately
- Telegram + Slack gateways can be done separately
- Weather + Web Search + Notes skills can be done separately

---

## 11. Success Criteria

Phase 2 is complete when:

- [ ] All 3 gateways (CLI, Telegram, Slack) are operational
- [ ] All 3 LLM providers (Anthropic, OpenAI, Ollama) are operational
- [ ] All 5 skills (calculator, datetime, weather, web_search, notes) are operational
- [ ] RBAC prevents unauthorized skill execution (validated via tests)
- [ ] User management supports full CRUD (validated via tests)
- [ ] All quality gates pass
- [ ] Test coverage ≥75% overall
- [ ] README and documentation are updated
- [ ] Manual E2E testing is successful

---

## 12. Next Steps

1. ✅ Complete spec.md
2. ✅ Complete research.md
3. ✅ Complete data-dictionary.md
4. ✅ Complete plan.md (this document)
5. ⏳ Create tasks.md (concrete task breakdown)
6. ⏳ Begin implementation with Security Agent (RBAC)

---

## 13. References

- Phase 1 Plan: `specs/initial-mvp-spec/plan.md`
- Architecture Guide: `AGENTS.md`
- Specification: `specs/phase-2-multi-platform/spec.md`
- Data Dictionary: `specs/phase-2-multi-platform/data-dictionary.md`
- Research: `specs/phase-2-multi-platform/research.md`
