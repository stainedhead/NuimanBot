# NuimanBot Admin API Reference

## Overview

The NuimanBot Admin API provides RESTful endpoints for managing user profiles, bot configurations, server settings, and monitoring. All endpoints require authentication via Bearer token and admin role permissions.

**Base URL:** `http://localhost:8080` (development) or `https://api.nuimanbot.example.com` (production)

**API Version:** 1.0.0

## Authentication

All API requests must include a valid Bearer token in the `Authorization` header:

```http
Authorization: Bearer <your-api-key>
```

The API key must be associated with a user profile that has the `admin` role and is enabled.

### Error Responses

| Status Code | Description |
|-------------|-------------|
| `200 OK` | Request succeeded |
| `201 Created` | Resource created successfully |
| `204 No Content` | Resource deleted successfully |
| `400 Bad Request` | Invalid request data |
| `401 Unauthorized` | Missing or invalid authentication |
| `403 Forbidden` | Insufficient permissions or disabled account |
| `404 Not Found` | Resource not found |
| `500 Internal Server Error` | Server error |

---

## User Profile Management

### List User Profiles

Retrieve a paginated list of all user profiles.

```http
GET /api/v1/admin/profiles
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `offset` | integer | 0 | Number of profiles to skip (for pagination) |
| `limit` | integer | 50 | Maximum number of profiles to return (1-500) |

**Example Request:**

```bash
curl -X GET "http://localhost:8080/api/v1/admin/profiles?offset=0&limit=10" \
  -H "Authorization: Bearer your-api-key"
```

**Example Response:**

```json
{
  "profiles": [
    {
      "userID": "user-123",
      "primaryEmail": "user@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "user",
      "userType": "individual",
      "enabled": true,
      "createdAt": "2026-01-15T10:00:00Z",
      "updatedAt": "2026-01-20T14:30:00Z"
    }
  ],
  "offset": 0,
  "limit": 10,
  "total": 1
}
```

### Get User Profile

Retrieve a specific user profile by ID.

```http
GET /api/v1/admin/profiles/{id}
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | User ID |

**Example Request:**

```bash
curl -X GET "http://localhost:8080/api/v1/admin/profiles/user-123" \
  -H "Authorization: Bearer your-api-key"
```

**Example Response:**

```json
{
  "userID": "user-123",
  "primaryEmail": "user@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "role": "user",
  "userType": "individual",
  "timezone": "America/New_York",
  "primaryLanguage": "en",
  "enabled": true,
  "createdAt": "2026-01-15T10:00:00Z",
  "updatedAt": "2026-01-20T14:30:00Z"
}
```

### Create User Profile

Create a new user profile.

```http
POST /api/v1/admin/profiles
```

**Request Body:**

```json
{
  "userID": "user-456",
  "primaryEmail": "new.user@example.com",
  "firstName": "Jane",
  "lastName": "Smith",
  "role": "user",
  "userType": "enterprise",
  "timezone": "Europe/London",
  "primaryLanguage": "en",
  "jobRole": "Product Manager"
}
```

**Response:** `201 Created` with the created profile in the response body.

### Update User Profile

Update an existing user profile (partial updates supported).

```http
PUT /api/v1/admin/profiles/{id}
```

**Request Body (all fields optional):**

```json
{
  "timezone": "Pacific/Auckland",
  "firstName": "Jane",
  "enabled": true
}
```

**Example Response:**

```json
{
  "profile": {
    "userID": "user-123",
    "primaryEmail": "user@example.com",
    "firstName": "Jane",
    "timezone": "Pacific/Auckland",
    "enabled": true
  },
  "updatedFields": ["timezone", "firstName", "enabled"]
}
```

### Delete User Profile

Delete a user profile by ID.

```http
DELETE /api/v1/admin/profiles/{id}
```

**Response:** `204 No Content`

---

## Slack Bot Management

### List Slack Bots

Retrieve all configured Slack bots. Tokens are masked in responses.

```http
GET /api/v1/admin/bots/slack
```

**Example Response:**

```json
{
  "bots": [
    {
      "botID": "slack-bot-1",
      "botName": "Production Bot",
      "botType": "private",
      "enabled": true,
      "slackBotToken": "xoxb...1234",
      "slackAppToken": "xapp...5678",
      "slackSigningSecret": "abcd...wxyz",
      "createdAt": "2026-01-10T08:00:00Z",
      "updatedAt": "2026-01-15T12:00:00Z"
    }
  ],
  "total": 1
}
```

### Get Slack Bot

Retrieve a specific Slack bot configuration.

```http
GET /api/v1/admin/bots/slack/{id}
```

### Create Slack Bot

Create a new Slack bot configuration.

```http
POST /api/v1/admin/bots/slack
```

**Request Body:**

```json
{
  "botID": "slack-bot-2",
  "botName": "Test Bot",
  "botType": "private",
  "enabled": true,
  "slackBotToken": "xoxb-your-bot-token",
  "slackAppToken": "xapp-your-app-token",
  "slackSigningSecret": "your-signing-secret"
}
```

**Response:** `201 Created` with masked tokens

### Update Slack Bot

Update an existing Slack bot configuration.

```http
PUT /api/v1/admin/bots/slack/{id}
```

### Delete Slack Bot

Delete a Slack bot configuration.

```http
DELETE /api/v1/admin/bots/slack/{id}
```

**Response:** `204 No Content`

### Enable Slack Bot

Enable a Slack bot.

```http
POST /api/v1/admin/bots/slack/{id}/enable
```

**Response:**

```json
{
  "status": "enabled"
}
```

### Disable Slack Bot

Disable a Slack bot.

```http
POST /api/v1/admin/bots/slack/{id}/disable
```

**Response:**

```json
{
  "status": "disabled"
}
```

---

## Telegram Bot Management

The Telegram bot endpoints follow the same pattern as Slack bots, but use `/api/v1/admin/bots/telegram` instead.

### Endpoints

- `GET /api/v1/admin/bots/telegram` - List Telegram bots
- `GET /api/v1/admin/bots/telegram/{id}` - Get Telegram bot
- `POST /api/v1/admin/bots/telegram` - Create Telegram bot
- `PUT /api/v1/admin/bots/telegram/{id}` - Update Telegram bot
- `DELETE /api/v1/admin/bots/telegram/{id}` - Delete Telegram bot
- `POST /api/v1/admin/bots/telegram/{id}/enable` - Enable Telegram bot
- `POST /api/v1/admin/bots/telegram/{id}/disable` - Disable Telegram bot

**Telegram Bot Configuration:**

```json
{
  "botID": "telegram-bot-1",
  "botName": "Support Bot",
  "botType": "public",
  "enabled": true,
  "telegramBotToken": "1234567890:AAF..."
}
```

---

## Server Configuration

### Get Configuration

Retrieve the current server configuration.

```http
GET /api/v1/admin/config
```

**Example Response:**

```json
{
  "server": {
    "port": 8080,
    "host": "localhost"
  },
  "llm": {
    "provider": "anthropic",
    "model": "claude-3-5-sonnet-20241022"
  }
}
```

### Update Configuration

Update server configuration.

```http
PUT /api/v1/admin/config
```

**Request Body:**

```json
{
  "server": {
    "port": 9090
  }
}
```

**Response:**

```json
{
  "status": "success",
  "message": "Configuration updated successfully"
}
```

### Reload Configuration

Reload configuration from file.

```http
POST /api/v1/admin/config/reload
```

**Response:**

```json
{
  "status": "success",
  "message": "Configuration reloaded successfully"
}
```

### Validate Configuration

Validate a configuration without applying it.

```http
POST /api/v1/admin/config/validate
```

**Request Body:**

```json
{
  "server": {
    "port": 9090
  }
}
```

**Success Response:**

```json
{
  "valid": true,
  "message": "Configuration is valid"
}
```

**Error Response (400):**

```json
{
  "valid": false,
  "errors": [
    "Invalid port number: must be between 1 and 65535"
  ]
}
```

---

## Server Monitoring

### Get Server Status

Retrieve current server status including uptime, version, and resource usage.

```http
GET /api/v1/admin/status
```

**Example Response:**

```json
{
  "uptime": 86400000000000,
  "version": "1.0.0",
  "memoryUsageMB": 256.5,
  "goVersion": "1.21",
  "activeConnections": {
    "slack": 3,
    "telegram": 2,
    "cli": 1
  }
}
```

**Field Descriptions:**

- `uptime`: Server uptime in nanoseconds (86400000000000 = 24 hours)
- `memoryUsageMB`: Current memory usage in megabytes
- `activeConnections`: Number of active connections by platform

### Get Server Metrics

Retrieve server performance metrics.

```http
GET /api/v1/admin/metrics
```

**Example Response:**

```json
{
  "requestsLast24h": 1000,
  "errorRate": 0.01,
  "avgResponseTime": 150.5,
  "activeUsers": 25,
  "activeBots": 5
}
```

**Field Descriptions:**

- `requestsLast24h`: Total requests in the last 24 hours
- `errorRate`: Error rate (0.01 = 1%)
- `avgResponseTime`: Average response time in milliseconds
- `activeUsers`: Number of active users
- `activeBots`: Number of active bots

### Get Server Logs

Retrieve recent server logs with optional filtering.

```http
GET /api/v1/admin/logs
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `level` | string | `all` | Log level filter: `all`, `error`, `warn`, `info`, `debug` |
| `limit` | integer | 50 | Maximum number of log entries (1-500) |

**Example Request:**

```bash
curl -X GET "http://localhost:8080/api/v1/admin/logs?level=error&limit=100" \
  -H "Authorization: Bearer your-api-key"
```

**Example Response:**

```json
{
  "logs": [
    {
      "timestamp": "2026-02-08T10:30:00Z",
      "level": "error",
      "message": "Connection failed to Slack API",
      "userID": "user-123"
    },
    {
      "timestamp": "2026-02-08T10:25:00Z",
      "level": "error",
      "message": "Database query timeout"
    }
  ],
  "level": "error",
  "limit": 100,
  "total": 2
}
```

---

## Data Models

### UserProfile

```typescript
{
  userID: string              // Required: Unique user identifier
  primaryEmail: string        // Required: Primary email address
  role: "admin" | "user" | "service"  // Required: User role

  // Optional fields
  moniker?: string           // Preferred display name
  firstName?: string
  lastName?: string
  nickName?: string
  backupEmail?: string
  mobilePhone?: string
  primaryLanguage?: string   // Default: "en"
  secondaryLanguage?: string
  timezone?: string          // Default: "UTC"
  primaryLocation?: string
  jobRole?: string
  userType?: "individual" | "enterprise" | "academic"
  profileInfo?: string       // Additional context about user
  enabled?: boolean          // Default: true
  platformIDs?: {
    slackID?: string
    telegramID?: string
    discordID?: string
  }

  // Auto-generated
  createdAt: string          // ISO 8601 timestamp
  updatedAt: string          // ISO 8601 timestamp
}
```

### SlackBotConfig

```typescript
{
  botID: string               // Required: Unique bot identifier
  botName: string            // Required: Display name
  botType: "private" | "public"  // Required
  enabled: boolean           // Default: true

  slackBotToken: string      // Masked in responses: "xoxb...1234"
  slackAppToken: string      // Masked in responses
  slackSigningSecret: string // Masked in responses

  createdAt: string
  updatedAt: string
}
```

### TelegramBotConfig

```typescript
{
  botID: string
  botName: string
  botType: "private" | "public"
  enabled: boolean

  telegramBotToken: string   // Masked in responses

  createdAt: string
  updatedAt: string
}
```

---

## Common Patterns

### Pagination

List endpoints support pagination via `offset` and `limit` query parameters:

```bash
# Get first 20 profiles
GET /api/v1/admin/profiles?offset=0&limit=20

# Get next 20 profiles
GET /api/v1/admin/profiles?offset=20&limit=20
```

### Partial Updates

PUT endpoints support partial updates - only include fields you want to change:

```json
// Update only timezone
{
  "timezone": "Pacific/Auckland"
}

// Update multiple fields
{
  "firstName": "Jane",
  "lastName": "Doe",
  "enabled": true
}
```

### Token Masking

Sensitive tokens are automatically masked in API responses:

- **Input:** `<token-value-here>` (e.g., a 60+ character Slack bot token)
- **Response:** `xoxb...wxyz` (first 4 and last 4 characters shown)

---

## OpenAPI Specification

The complete OpenAPI 3.0 specification is available at:

- **YAML:** [`api/openapi.yaml`](../api/openapi.yaml)
- **Swagger UI:** Visit `http://localhost:8080/swagger` (when server is running)

---

## Rate Limiting

Currently, the API does not enforce rate limiting, but this may be added in future versions. Best practice is to:

- Limit concurrent requests to avoid overwhelming the server
- Use pagination for large result sets
- Cache responses when appropriate

---

## Changelog

### Version 1.0.0 (2026-02-08)

- Initial API release
- User profile management endpoints
- Slack and Telegram bot management
- Server configuration endpoints
- Server monitoring endpoints
- Bearer token authentication
- Role-based access control (RBAC)

---

## Support

For API support or to report issues:

- GitHub Issues: https://github.com/yourusername/nuimanbot/issues
- Email: admin@nuimanbot.example.com

---

**Last Updated:** 2026-02-08
