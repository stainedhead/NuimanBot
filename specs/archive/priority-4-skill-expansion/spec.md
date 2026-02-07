# Priority 4: Skill Expansion Specification

## Overview
Expand NuimanBot's skill library with three new production-ready skills: Weather, Web Search, and Notes.

## Goals
- Add Weather skill for real-time weather information via OpenWeatherMap API
- Add Web Search skill for internet searches via DuckDuckGo
- Add Notes skill for persistent user note-taking with SQLite storage

## Success Criteria
- All three skills implement the domain.Skill interface
- Comprehensive test coverage (TDD approach)
- Proper error handling and validation
- Clean Architecture compliance (no domain layer dependencies)
- All quality gates passing

## Skills to Implement

### 1. Weather Skill
**Purpose:** Get current weather and forecasts for any location

**Operations:**
- `current` - Get current weather for a location
- `forecast` - Get 5-day forecast for a location

**Parameters:**
- `operation`: string (required) - "current" or "forecast"
- `location`: string (required) - City name or coordinates
- `units`: string (optional) - "metric", "imperial", "standard" (default: metric)

**External API:** OpenWeatherMap API (https://openweathermap.org/api)
- Requires API key (stored in vault)
- Rate limits apply (60 calls/minute free tier)

**Example Usage:**
```json
{
  "operation": "current",
  "location": "London",
  "units": "metric"
}
```

### 2. Web Search Skill
**Purpose:** Perform web searches and return results

**Operations:**
- `search` - Search the web for a query
- `news` - Search for recent news articles

**Parameters:**
- `operation`: string (required) - "search" or "news"
- `query`: string (required) - Search query
- `limit`: int (optional) - Number of results (default: 5, max: 10)

**External API:** DuckDuckGo Instant Answer API
- No API key required
- Rate limits apply (be respectful)
- Returns instant answers and web results

**Example Usage:**
```json
{
  "operation": "search",
  "query": "golang clean architecture",
  "limit": 5
}
```

### 3. Notes Skill
**Purpose:** Create, read, update, delete user notes

**Operations:**
- `create` - Create a new note
- `read` - Read a note by ID or list all notes
- `update` - Update an existing note
- `delete` - Delete a note
- `list` - List all notes for a user

**Parameters:**
- `operation`: string (required) - "create", "read", "update", "delete", "list"
- `id`: string (optional) - Note ID (required for read, update, delete)
- `title`: string (optional) - Note title (required for create, optional for update)
- `content`: string (optional) - Note content (required for create, optional for update)
- `tags`: []string (optional) - Note tags

**Storage:** SQLite database (new notes table)

**Example Usage:**
```json
{
  "operation": "create",
  "title": "Meeting Notes",
  "content": "Discussed Q1 roadmap...",
  "tags": ["work", "meeting"]
}
```

## Architecture

### Layer Placement
- **Domain:** Skill interface (already exists)
- **Infrastructure:**
  - `internal/infrastructure/weather/openweathermap.go` - OpenWeatherMap client
  - `internal/infrastructure/search/duckduckgo.go` - DuckDuckGo client
- **Adapter:**
  - `internal/adapter/repository/sqlite/notes.go` - Notes repository
- **Skills:**
  - `internal/skills/weather/weather.go` - Weather skill implementation
  - `internal/skills/websearch/websearch.go` - Web search skill implementation
  - `internal/skills/notes/notes.go` - Notes skill implementation

### Dependencies
- Weather: `github.com/briandowns/openweathermap` (or native HTTP client)
- Web Search: Native HTTP client (DuckDuckGo HTML API)
- Notes: Existing SQLite infrastructure

## Testing Strategy
- Unit tests for each skill (TDD approach)
- Mock external APIs for testing
- Integration tests with real API calls (optional, rate-limited)
- Test error handling, validation, edge cases

## Security Considerations
- API keys stored in credential vault
- Input validation for all parameters
- Rate limiting to prevent abuse
- Sanitize user input for SQL queries (notes skill)
- Validate URLs from search results

## Configuration
Add to config.yaml:
```yaml
skills:
  weather:
    enabled: true
    api_key_vault_id: "openweathermap_api_key"
    default_units: "metric"
    timeout: 10s

  websearch:
    enabled: true
    max_results: 10
    timeout: 10s

  notes:
    enabled: true
    max_note_size: 10000 # characters
```
