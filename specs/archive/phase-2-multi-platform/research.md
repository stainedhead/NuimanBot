# Phase 2: Multi-Platform - Research

**Last Updated:** 2026-02-06

This document contains research findings, API documentation, and integration examples for Phase 2 components.

---

## 1. Telegram Bot Integration

### 1.1. Library: github.com/go-telegram/bot

**Repository:** https://github.com/go-telegram/bot

**Installation:**
```bash
go get github.com/go-telegram/bot
```

**Key Features:**
- Modern Go Telegram Bot API wrapper
- Supports long polling and webhook modes
- Full Bot API coverage
- Context-based handlers
- Middleware support

**Basic Example:**
```go
package main

import (
    "context"
    "github.com/go-telegram/bot"
    "github.com/go-telegram/bot/models"
)

func main() {
    ctx := context.Background()

    opts := []bot.Option{
        bot.WithDefaultHandler(handler),
    }

    b, err := bot.New("YOUR_BOT_TOKEN", opts...)
    if err != nil {
        panic(err)
    }

    b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
    if update.Message != nil {
        b.SendMessage(ctx, &bot.SendMessageParams{
            ChatID: update.Message.Chat.ID,
            Text:   "Hello from NuimanBot!",
        })
    }
}
```

**Gateway Implementation Notes:**
- Use long polling for MVP (simpler than webhooks)
- Extract `update.Message.From.ID` as `PlatformUID`
- Map message to `IncomingMessage` domain type
- Support markdown formatting in responses
- Handle `/start` command for initialization
- Handle rate limiting (Telegram: 30 msg/sec per chat)

---

## 2. Slack Bot Integration

### 2.1. Library: github.com/slack-go/slack

**Repository:** https://github.com/slack-go/slack

**Installation:**
```bash
go get github.com/slack-go/slack
```

**Key Features:**
- Official Slack SDK for Go
- Supports Socket Mode (no public URL needed)
- Web API client
- RTM (Real-Time Messaging) support
- Block Kit builder

**Socket Mode Example:**
```go
package main

import (
    "context"
    "github.com/slack-go/slack"
    "github.com/slack-go/slack/slackevents"
    "github.com/slack-go/slack/socketmode"
)

func main() {
    api := slack.New(
        "xoxb-YOUR-BOT-TOKEN",
        slack.OptionAppLevelToken("xapp-YOUR-APP-TOKEN"),
    )

    client := socketmode.New(api)

    go func() {
        for evt := range client.Events {
            switch evt.Type {
            case socketmode.EventTypeEventsAPI:
                eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
                if !ok {
                    continue
                }

                client.Ack(*evt.Request)

                switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
                case *slackevents.MessageEvent:
                    // Handle message
                    api.PostMessage(ev.Channel, slack.MsgOptionText("Hello!", false))
                }
            }
        }
    }()

    client.Run()
}
```

**Gateway Implementation Notes:**
- Use Socket Mode (avoids webhook setup)
- Requires both Bot Token and App Token
- Listen for `app_mention` and `message` events
- Extract `event.User` as `PlatformUID`
- Map event to `IncomingMessage` domain type
- Use Block Kit for rich formatting (optional MVP feature)
- Handle threading (reply in thread if message is in thread)

**Required Slack App Scopes:**
- `app_mentions:read` - Receive mentions
- `chat:write` - Send messages
- `im:history` - Read DMs
- `im:write` - Send DMs

---

## 3. OpenAI Provider

### 3.1. Library: github.com/sashabaranov/go-openai

**Repository:** https://github.com/sashabaranov/go-openai

**Installation:**
```bash
go get github.com/sashabaranov/go-openai
```

**Key Features:**
- Unofficial but well-maintained OpenAI SDK
- Supports GPT-4, GPT-3.5, embeddings, images
- Streaming support
- Function calling (tool use)
- Retries and error handling

**Basic Completion Example:**
```go
package main

import (
    "context"
    "github.com/sashabaranov/go-openai"
)

func main() {
    client := openai.NewClient("sk-YOUR-API-KEY")

    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4o,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "Hello, how are you?",
                },
            },
        },
    )

    if err != nil {
        panic(err)
    }

    println(resp.Choices[0].Message.Content)
}
```

**Streaming Example:**
```go
stream, err := client.CreateChatCompletionStream(
    context.Background(),
    openai.ChatCompletionRequest{
        Model: openai.GPT4o,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleUser, Content: "Count to 10"},
        },
        Stream: true,
    },
)
if err != nil {
    panic(err)
}
defer stream.Close()

for {
    response, err := stream.Recv()
    if errors.Is(err, io.EOF) {
        break
    }
    if err != nil {
        panic(err)
    }

    fmt.Print(response.Choices[0].Delta.Content)
}
```

**Function Calling Example:**
```go
resp, err := client.CreateChatCompletion(
    context.Background(),
    openai.ChatCompletionRequest{
        Model: openai.GPT4o,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleUser, Content: "What's 5 + 3?"},
        },
        Tools: []openai.Tool{
            {
                Type: openai.ToolTypeFunction,
                Function: &openai.FunctionDefinition{
                    Name:        "calculator",
                    Description: "Perform basic arithmetic",
                    Parameters: map[string]interface{}{
                        "type": "object",
                        "properties": map[string]interface{}{
                            "operation": map[string]string{"type": "string"},
                            "a":         map[string]string{"type": "number"},
                            "b":         map[string]string{"type": "number"},
                        },
                        "required": []string{"operation", "a", "b"},
                    },
                },
            },
        },
    },
)
```

**Provider Implementation Notes:**
- Map `openai.ChatCompletionMessage` to `domain.Message`
- Convert NuimanBot skill definitions to OpenAI function definitions
- Handle tool calls: parse function call, execute skill, send result back
- Support streaming via channel
- Track token usage from response
- Handle errors: rate limits (429), context length (400), invalid API key (401)

**Supported Models:**
- `gpt-4o` - Most capable, multimodal
- `gpt-4-turbo` - Fast GPT-4
- `gpt-4` - Original GPT-4
- `gpt-3.5-turbo` - Fast and cheap

---

## 4. Ollama Provider

### 4.1. API: Ollama HTTP API

**Documentation:** https://github.com/ollama/ollama/blob/main/docs/api.md

**Base URL:** `http://localhost:11434` (default)

**Key Endpoints:**
- `POST /api/generate` - Generate completion
- `POST /api/chat` - Chat completion
- `GET /api/tags` - List models
- `POST /api/pull` - Pull a model
- `DELETE /api/delete` - Delete a model

**Generate Completion Example:**
```bash
curl http://localhost:11434/api/generate -d '{
  "model": "llama3",
  "prompt": "Why is the sky blue?",
  "stream": false
}'
```

**Response:**
```json
{
  "model": "llama3",
  "created_at": "2024-01-01T00:00:00Z",
  "response": "The sky appears blue because...",
  "done": true,
  "total_duration": 1234567890,
  "load_duration": 123456789,
  "prompt_eval_count": 10,
  "eval_count": 50
}
```

**Chat Completion Example:**
```bash
curl http://localhost:11434/api/chat -d '{
  "model": "llama3",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ],
  "stream": false
}'
```

**Response:**
```json
{
  "model": "llama3",
  "created_at": "2024-01-01T00:00:00Z",
  "message": {
    "role": "assistant",
    "content": "Hello! How can I help you today?"
  },
  "done": true
}
```

**Streaming Example:**
```bash
curl http://localhost:11434/api/chat -d '{
  "model": "llama3",
  "messages": [{"role": "user", "content": "Count to 5"}],
  "stream": true
}'
```

**Streaming Response (line-delimited JSON):**
```json
{"message":{"role":"assistant","content":"1"},"done":false}
{"message":{"role":"assistant","content":", 2"},"done":false}
{"message":{"role":"assistant","content":", 3"},"done":false}
{"message":{"role":"assistant","content":", 4"},"done":false}
{"message":{"role":"assistant","content":", 5"},"done":false}
{"message":{"role":"assistant","content":""},"done":true,"total_duration":1234567890}
```

**Provider Implementation Notes:**
- Use standard `net/http` client (no SDK needed)
- Map Ollama message format to `domain.Message`
- Support streaming via line-delimited JSON parsing
- Handle connection errors gracefully (Ollama not running)
- No API key required
- Track token counts from `prompt_eval_count` and `eval_count`
- Tool calling: Not supported by all models, may need to simulate

**Popular Models:**
- `llama3` - Meta's Llama 3 (8B, 70B)
- `mistral` - Mistral 7B
- `codellama` - Code-focused Llama
- `phi` - Microsoft's Phi models
- `gemma` - Google's Gemma

---

## 5. OpenWeatherMap API

### 5.1. API: Current Weather & Forecast

**Documentation:** https://openweathermap.org/api

**Free Tier:**
- 60 calls/minute
- 1,000,000 calls/month
- Current weather data
- 5-day forecast

**Current Weather Example:**
```bash
curl "https://api.openweathermap.org/data/2.5/weather?q=San Francisco&appid=YOUR_API_KEY&units=metric"
```

**Response:**
```json
{
  "coord": {"lon": -122.42, "lat": 37.77},
  "weather": [
    {
      "id": 800,
      "main": "Clear",
      "description": "clear sky",
      "icon": "01d"
    }
  ],
  "base": "stations",
  "main": {
    "temp": 18.5,
    "feels_like": 17.8,
    "temp_min": 16.0,
    "temp_max": 20.0,
    "pressure": 1013,
    "humidity": 65
  },
  "wind": {
    "speed": 3.5,
    "deg": 270
  },
  "clouds": {"all": 0},
  "dt": 1234567890,
  "sys": {
    "country": "US",
    "sunrise": 1234567800,
    "sunset": 1234598000
  },
  "timezone": -28800,
  "name": "San Francisco"
}
```

**5-Day Forecast Example:**
```bash
curl "https://api.openweathermap.org/data/2.5/forecast?q=San Francisco&appid=YOUR_API_KEY&units=metric&cnt=8"
```

**Weather Skill Implementation Notes:**
- Use `weather` endpoint for current conditions
- Use `forecast` endpoint for multi-day forecast
- Parse `main.temp`, `main.humidity`, `wind.speed`, `weather[0].description`
- Handle errors: invalid city (404), rate limit (429), invalid API key (401)
- Cache results for 30 minutes to reduce API calls
- Default units to metric, allow user preference
- Format output as human-readable text

---

## 6. DuckDuckGo Search API

### 6.1. API: Instant Answer API

**Documentation:** https://duckduckgo.com/api

**Endpoint:** `https://api.duckduckgo.com/`

**Example:**
```bash
curl "https://api.duckduckgo.com/?q=Golang&format=json"
```

**Response:**
```json
{
  "Abstract": "Go is a statically typed, compiled programming language...",
  "AbstractText": "Go is a statically typed, compiled programming language...",
  "AbstractSource": "Wikipedia",
  "AbstractURL": "https://en.wikipedia.org/wiki/Go_(programming_language)",
  "Heading": "Go (programming language)",
  "RelatedTopics": [
    {
      "Text": "Concurrent computing - simultaneous execution...",
      "FirstURL": "https://en.wikipedia.org/wiki/Concurrent_computing"
    }
  ],
  "Results": []
}
```

**Web Search Skill Implementation Notes:**
- DuckDuckGo Instant Answer API returns summaries, not full web results
- For full web results, consider:
  - Option 1: SerpAPI (paid, $50/mo for 5000 searches)
  - Option 2: Scrape DuckDuckGo HTML (against ToS, not recommended)
  - Option 3: Use Instant Answer API + Related Topics (free, MVP approach)
- Parse `Abstract`, `AbstractURL`, `RelatedTopics`
- Return top 3-5 related topics as search results
- Handle no results gracefully
- No API key required for DuckDuckGo

**Alternative: SerpAPI**
```bash
curl "https://serpapi.com/search.json?q=Golang&api_key=YOUR_API_KEY"
```

---

## 7. Database Schema Extensions

### 7.1. Notes Table

For the notes skill, add a new table:

```sql
CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_created_at ON notes(created_at);
```

**Columns:**
- `id` - UUID primary key
- `user_id` - Foreign key to users table
- `title` - Note title (max 200 chars)
- `content` - Note content (max 10000 chars)
- `created_at` - Timestamp when note was created
- `updated_at` - Timestamp when note was last updated

---

## 8. RBAC Permission Matrix

### 8.1. Role-Based Skill Access

| Skill | Admin | User | Guest |
|-------|-------|------|-------|
| calculator | ✅ | ✅ | ✅ |
| datetime | ✅ | ✅ | ✅ |
| weather | ✅ | ✅ | ❌ |
| web_search | ✅ | ✅ | ❌ |
| notes | ✅ | ✅ | ❌ |
| admin commands | ✅ | ❌ | ❌ |

**Implementation:**
```go
var skillPermissions = map[string]Role{
    "calculator":     RoleGuest,  // Available to all
    "datetime":       RoleGuest,  // Available to all
    "weather":        RoleUser,   // Requires User+
    "web_search":     RoleUser,   // Requires User+
    "notes":          RoleUser,   // Requires User+
    "admin.user":     RoleAdmin,  // Admin only
}

func (s *SkillExecutionService) checkPermission(user *User, skillName string) error {
    required, ok := skillPermissions[skillName]
    if !ok {
        required = RoleUser // Default: require User+
    }

    if user.Role < required {
        return ErrInsufficientPermissions
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
            return ErrInsufficientPermissions
        }
    }

    return nil
}
```

---

## 9. Tool Calling Format Mapping

### 9.1. NuimanBot Skill → OpenAI Function

**NuimanBot Skill:**
```go
type Skill interface {
    Name() string
    Description() string
    InputSchema() map[string]interface{}
    Execute(ctx context.Context, params map[string]interface{}) (*SkillResult, error)
}

// Example: Calculator skill
InputSchema: {
    "type": "object",
    "properties": {
        "operation": {"type": "string", "enum": ["add", "subtract", "multiply", "divide"]},
        "a": {"type": "number"},
        "b": {"type": "number"}
    },
    "required": ["operation", "a", "b"]
}
```

**OpenAI Function Definition:**
```go
openai.Tool{
    Type: openai.ToolTypeFunction,
    Function: &openai.FunctionDefinition{
        Name:        "calculator",
        Description: "Perform basic arithmetic operations",
        Parameters:  skill.InputSchema(), // Same schema format!
    },
}
```

**Conversion is trivial** - both use JSON Schema format!

---

## 10. Rate Limiting Considerations

### 10.1. External API Limits

| Service | Free Tier Limit | Strategy |
|---------|----------------|----------|
| Telegram Bot API | 30 msg/sec per chat | Built-in by Telegram |
| Slack API | ~1 req/sec per workspace | Use Socket Mode |
| OpenAI API | Varies by tier | Exponential backoff |
| Ollama | Unlimited (local) | N/A |
| OpenWeatherMap | 60 calls/min | Cache for 30 min |
| DuckDuckGo | Unofficial, unknown | Conservative use |

### 10.2. Internal Rate Limiting

Already implemented in Phase 1:
```go
type SkillConfig struct {
    RateLimit    int           // Max calls per minute
    Timeout      time.Duration // Execution timeout
}
```

Apply to new skills:
- weather: 10 calls/min per user
- web_search: 5 calls/min per user
- notes: 60 calls/min per user (local, fast)

---

## 11. Error Handling Patterns

### 11.1. Gateway Errors

```go
func (g *TelegramGateway) handleError(ctx context.Context, chatID int64, err error) {
    var msg string

    switch {
    case errors.Is(err, domain.ErrInsufficientPermissions):
        msg = "❌ You don't have permission to use this skill."
    case errors.Is(err, domain.ErrSkillNotFound):
        msg = "❌ Skill not found. Try /help to see available skills."
    case errors.Is(err, domain.ErrRateLimitExceeded):
        msg = "⏱️ Rate limit exceeded. Please wait a moment and try again."
    default:
        msg = "❌ An error occurred. Please try again later."
        // Log error for debugging
        log.Error("unhandled error", "error", err)
    }

    g.bot.SendMessage(ctx, &bot.SendMessageParams{
        ChatID: chatID,
        Text:   msg,
    })
}
```

---

## 12. Testing Strategy

### 12.1. Mock External Services

For unit tests, mock external APIs:

**Telegram:**
```go
type MockTelegramBot struct {
    SendMessageFunc func(ctx context.Context, params *bot.SendMessageParams) error
}

func (m *MockTelegramBot) SendMessage(ctx context.Context, params *bot.SendMessageParams) error {
    return m.SendMessageFunc(ctx, params)
}
```

**OpenAI:**
```go
type MockOpenAIClient struct {
    CreateChatCompletionFunc func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}
```

**OpenWeatherMap:**
```go
type MockWeatherClient struct {
    GetCurrentWeatherFunc func(ctx context.Context, location string) (*WeatherResponse, error)
}
```

### 12.2. Integration Tests

For integration tests, use real APIs in CI:
- Set API keys as GitHub secrets
- Run integration tests only on main branch
- Skip if API keys not available (for PRs)

---

## 13. Next Steps

1. ✅ Complete research.md
2. ⏳ Create data-dictionary.md (define new types)
3. ⏳ Create plan.md (implementation approach)
4. ⏳ Create tasks.md (concrete tasks)
5. ⏳ Begin implementation following TDD

---

## 14. References

- Telegram Bot API: https://core.telegram.org/bots/api
- Slack API: https://api.slack.com/
- OpenAI API: https://platform.openai.com/docs
- Ollama API: https://github.com/ollama/ollama/blob/main/docs/api.md
- OpenWeatherMap API: https://openweathermap.org/api
- DuckDuckGo API: https://duckduckgo.com/api
