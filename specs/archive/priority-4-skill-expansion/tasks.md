# Priority 4: Skill Expansion - Task Breakdown

## Weather Skill (Tasks 7.1-7.5)

### Task 7.1: OpenWeatherMap Client Infrastructure
**Layer:** Infrastructure
**Files:**
- Create `internal/infrastructure/weather/openweathermap.go`
- Create `internal/infrastructure/weather/openweathermap_test.go`

**Subtasks:**
- [ ] Define WeatherClient interface
- [ ] Implement OpenWeatherMap API client
- [ ] Add GetCurrentWeather() method
- [ ] Add GetForecast() method
- [ ] Handle API errors and rate limits
- [ ] Write comprehensive tests with mocked HTTP responses

### Task 7.2: Weather Skill Domain Types
**Layer:** Domain (if needed, otherwise skip)
**Files:**
- Potentially add weather types to domain layer (optional)

**Subtasks:**
- [ ] Decide if weather types belong in domain or infrastructure
- [ ] If domain: add WeatherData, ForecastData types
- [ ] Keep minimal, focus on skill interface

### Task 7.3: Weather Skill Implementation
**Layer:** Skills
**Files:**
- Create `internal/skills/weather/weather.go`
- Create `internal/skills/weather/weather_test.go`

**Subtasks:**
- [ ] RED: Write failing tests for current weather operation
- [ ] RED: Write failing tests for forecast operation
- [ ] RED: Write failing tests for parameter validation
- [ ] GREEN: Implement Weather skill to pass all tests
- [ ] REFACTOR: Clean up code, eliminate duplication

### Task 7.4: Weather Skill Configuration
**Layer:** Config
**Files:**
- Modify `internal/config/skills_config.go`

**Subtasks:**
- [ ] Add WeatherSkillConfig struct
- [ ] Add API key vault ID field
- [ ] Add default units, timeout fields
- [ ] Update loader to handle weather config

### Task 7.5: Weather Skill Registration
**Layer:** Main
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Subtasks:**
- [ ] Register weather skill in registerBuiltInSkills()
- [ ] Load API key from vault
- [ ] Pass config to weather skill constructor
- [ ] Test end-to-end with real API (manual)
- [ ] Run all quality gates

---

## Web Search Skill (Tasks 8.1-8.5)

### Task 8.1: DuckDuckGo Client Infrastructure
**Layer:** Infrastructure
**Files:**
- Create `internal/infrastructure/search/duckduckgo.go`
- Create `internal/infrastructure/search/duckduckgo_test.go`

**Subtasks:**
- [ ] Define SearchClient interface
- [ ] Implement DuckDuckGo Instant Answer API client
- [ ] Add Search() method
- [ ] Add NewsSearch() method (optional)
- [ ] Parse HTML/JSON results
- [ ] Write comprehensive tests with mocked HTTP responses

### Task 8.2: Web Search Skill Implementation
**Layer:** Skills
**Files:**
- Create `internal/skills/websearch/websearch.go`
- Create `internal/skills/websearch/websearch_test.go`

**Subtasks:**
- [ ] RED: Write failing tests for search operation
- [ ] RED: Write failing tests for parameter validation
- [ ] RED: Write failing tests for result limiting
- [ ] GREEN: Implement WebSearch skill to pass all tests
- [ ] REFACTOR: Clean up code, eliminate duplication

### Task 8.3: Web Search Skill Configuration
**Layer:** Config
**Files:**
- Modify `internal/config/skills_config.go`

**Subtasks:**
- [ ] Add WebSearchSkillConfig struct
- [ ] Add max_results, timeout fields
- [ ] Update loader to handle websearch config

### Task 8.4: Web Search Skill Registration
**Layer:** Main
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Subtasks:**
- [ ] Register websearch skill in registerBuiltInSkills()
- [ ] Pass config to websearch skill constructor
- [ ] Test end-to-end (manual)
- [ ] Run all quality gates

### Task 8.5: Web Search Security Review
**Security:** Input validation
**Files:**
- Review and enhance input validation

**Subtasks:**
- [ ] Validate search query length
- [ ] Sanitize search query for URL encoding
- [ ] Validate result limit bounds
- [ ] Test injection attempts

---

## Notes Skill (Tasks 9.1-9.8)

### Task 9.1: Notes Database Schema
**Layer:** Database
**Files:**
- Modify `cmd/nuimanbot/main.go` (initializeDatabase function)

**Subtasks:**
- [ ] Define notes table schema (id, user_id, title, content, tags, created_at, updated_at)
- [ ] Add CREATE TABLE statement to initializeDatabase()
- [ ] Add indexes for user_id, created_at

### Task 9.2: Notes Repository Interface
**Layer:** Use Case
**Files:**
- Create `internal/usecase/notes/repository.go`

**Subtasks:**
- [ ] Define NotesRepository interface
- [ ] Add Create() method
- [ ] Add Read() method
- [ ] Add Update() method
- [ ] Add Delete() method
- [ ] Add List() method

### Task 9.3: Notes Domain Types
**Layer:** Domain
**Files:**
- Create `internal/domain/note.go`

**Subtasks:**
- [ ] Define Note struct
- [ ] Add validation methods
- [ ] Keep simple, focused on data structure

### Task 9.4: SQLite Notes Repository Implementation
**Layer:** Adapter
**Files:**
- Create `internal/adapter/repository/sqlite/notes.go`
- Create `internal/adapter/repository/sqlite/notes_test.go`

**Subtasks:**
- [ ] Implement NotesRepository interface
- [ ] Implement Create() with SQL INSERT
- [ ] Implement Read() with SQL SELECT
- [ ] Implement Update() with SQL UPDATE
- [ ] Implement Delete() with SQL DELETE
- [ ] Implement List() with SQL SELECT + WHERE
- [ ] Write comprehensive tests

### Task 9.5: Notes Skill Implementation
**Layer:** Skills
**Files:**
- Create `internal/skills/notes/notes.go`
- Create `internal/skills/notes/notes_test.go`

**Subtasks:**
- [ ] RED: Write failing tests for create operation
- [ ] RED: Write failing tests for read operation
- [ ] RED: Write failing tests for update operation
- [ ] RED: Write failing tests for delete operation
- [ ] RED: Write failing tests for list operation
- [ ] GREEN: Implement Notes skill to pass all tests
- [ ] REFACTOR: Clean up code, eliminate duplication

### Task 9.6: Notes Skill Configuration
**Layer:** Config
**Files:**
- Modify `internal/config/skills_config.go`

**Subtasks:**
- [ ] Add NotesSkillConfig struct
- [ ] Add max_note_size field
- [ ] Update loader to handle notes config

### Task 9.7: Notes Skill Registration
**Layer:** Main
**Files:**
- Modify `cmd/nuimanbot/main.go`

**Subtasks:**
- [ ] Initialize notes repository
- [ ] Register notes skill in registerBuiltInSkills()
- [ ] Pass repository and config to notes skill constructor
- [ ] Test end-to-end (manual)
- [ ] Run all quality gates

### Task 9.8: Notes Security Review
**Security:** SQL injection prevention
**Files:**
- Review and enhance input validation

**Subtasks:**
- [ ] Ensure all SQL queries use parameterized statements
- [ ] Validate note title/content length
- [ ] Test SQL injection attempts
- [ ] Ensure user isolation (notes per user)

---

## Final Integration & Quality (Tasks 10.1-10.3)

### Task 10.1: Documentation Updates
**Files:**
- README.md
- STATUS.md
- SPEC_STATUS.md

**Subtasks:**
- [ ] Update README with new skills documentation
- [ ] Update STATUS.md with Priority 4 completion
- [ ] Add usage examples for each skill
- [ ] Update config examples

### Task 10.2: All Quality Gates
**Command:**
```bash
go fmt ./... && go mod tidy && go vet ./... && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

**Subtasks:**
- [ ] Run fmt, tidy, vet
- [ ] Run all tests
- [ ] Build executable
- [ ] Verify executable runs

### Task 10.3: Git Commit & Push
**Subtasks:**
- [ ] Create commits for completed work
- [ ] Push to main branch
- [ ] Tag release if appropriate

---

## Task Summary

- Weather Skill: 5 tasks
- Web Search Skill: 5 tasks
- Notes Skill: 8 tasks
- Final Integration: 3 tasks

**Total: 21 tasks**
