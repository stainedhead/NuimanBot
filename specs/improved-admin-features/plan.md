# Improved Admin Features - Implementation Plan

**Created:** 2026-02-08
**Version:** 1.0
**Status:** Planning
**Estimated Duration:** 140-190 hours (17-24 weeks at 8h/week)

---

## Table of Contents

1. [Development Approach](#development-approach)
2. [Phase Breakdown](#phase-breakdown)
3. [Critical Path](#critical-path)
4. [Dependency Graph](#dependency-graph)
5. [Testing Strategy](#testing-strategy)
6. [Rollout Strategy](#rollout-strategy)
7. [Success Metrics](#success-metrics)
8. [Risk Mitigation](#risk-mitigation)

---

## Development Approach

### Methodology

**TDD + Bottom-Up Clean Architecture**

```
Red → Green → Refactor (for each component)
  ↓
Domain → Infrastructure → Use Case → Adapter
  ↓
Integration → E2E → Documentation
```

**Why This Approach?**
- Domain-first ensures business logic is correct before infrastructure concerns
- TDD prevents regression and ensures high test coverage (>90%)
- Bottom-up reduces rework (foundation is solid before building higher layers)
- Clean Architecture maintains separation of concerns and testability
- Incremental milestones allow for early feedback and course correction

**Key Principles:**
1. **Test First**: Write failing test before implementation (Red-Green-Refactor)
2. **Layer Independence**: Domain has no external dependencies; inner layers define interfaces
3. **Dependency Inversion**: Outer layers implement interfaces defined by inner layers
4. **Atomic Commits**: Each task produces a deployable, tested increment
5. **Quality Gates**: All tests, linters, and builds must pass before proceeding

**Incremental Milestones:**
- Phase 1: Core architecture foundation (paths, config reload, audit logging)
- Phase 2: User profiles file storage and management
- Phase 3: Bot configuration database and gateway integration
- Phase 4: REST API with authentication and authorization
- Phase 5: Web admin interface with all management pages
- Phase 6: Documentation, migration tools, and deployment guides

---

## Phase Breakdown

### Phase 1: Core Architecture Foundation (30-40h)

**Goal:** Establish foundational architecture for multi-component system with configuration management and hot reload capabilities.

**Deliverables:**
- Binary rename (nuimanbot → nuimanbotd for server, nuimanbot for CLI)
- Path configuration system (config/data/logs separation)
- Configuration hot reload infrastructure
- File-based storage foundation with atomic writes
- Enhanced audit logging system
- STDIO simplification (/refresh and /exit only)

**Tasks:**

#### Task 1.1: Binary Rename and Project Structure (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Create new entry point `cmd/nuimanbotd/main.go` (server daemon)
2. Refactor existing `cmd/nuimanbot/main.go` to CLI tool only
3. Update build scripts and Makefile
4. Update systemd/docker configurations
5. Update documentation references

**Files to Create/Modify:**
- Create: `cmd/nuimanbotd/main.go`
- Modify: `cmd/nuimanbot/main.go`
- Modify: `Makefile`
- Modify: `README.md`
- Modify: `.github/workflows/*.yml` (if CI exists)

**Acceptance Criteria:**
- [ ] `go build -o bin/nuimanbotd ./cmd/nuimanbotd` succeeds
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds
- [ ] Server binary runs continuously as daemon
- [ ] CLI binary executes admin commands and exits
- [ ] All existing tests pass
- [ ] Documentation updated

**Quality Gates:**
- [ ] Unit tests: N/A (structural change)
- [ ] Integration tests: Build succeeds
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** None

---

#### Task 1.2: Path Configuration System (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Define `PathsConfig` struct in `internal/domain/config.go`
2. Add path fields: `ConfigDir`, `DataDir`, `LogsDir`
3. Implement path resolution utility `ResolvePath(pathType, relativePath string) string`
4. Add environment variable overrides (`NUIMANBOT_SERVER_PATHS_*`)
5. Implement path validation (exists, writable)
6. Update all file operations to use configured paths

**Files to Create/Modify:**
- Create: `internal/domain/config.go` (PathsConfig)
- Create: `internal/domain/config_test.go`
- Create: `internal/infrastructure/filesystem/paths.go`
- Create: `internal/infrastructure/filesystem/paths_test.go`
- Modify: All files performing file I/O

**Acceptance Criteria:**
- [ ] PathsConfig struct with ConfigDir, DataDir, LogsDir fields
- [ ] Default values: ./config/, ./data/, ./logs/
- [ ] Environment variable overrides work (NUIMANBOT_SERVER_PATHS_CONFIG, etc.)
- [ ] Path validation on startup (must exist and be writable)
- [ ] All file operations use configured paths
- [ ] Container deployment can mount separate volumes

**Test Coverage:**
- Unit tests: Path resolution, validation, environment variable parsing
- Integration tests: File operations in configured paths
- E2E tests: Server startup with custom paths

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Path operations succeed
- [ ] Linter passes
- [ ] Documentation updated

**Dependencies:** Task 1.1

---

#### Task 1.3: Configuration Hot Reload Infrastructure (6-8h)
**Estimated:** 7h

**Implementation Steps:**
1. Define configuration reload interface in domain layer
2. Implement file watcher for config files (config.yaml, users.json, bots.json)
3. Implement configuration validator
4. Implement atomic configuration swap (no downtime)
5. Add `/refresh` STDIO command handler
6. Add configuration reload API endpoint (for later use)
7. Implement rollback on invalid configuration

**Files to Create/Modify:**
- Create: `internal/domain/config_reloader.go` (interface)
- Create: `internal/infrastructure/config/watcher.go`
- Create: `internal/infrastructure/config/watcher_test.go`
- Create: `internal/infrastructure/config/validator.go`
- Create: `internal/infrastructure/config/validator_test.go`
- Create: `internal/usecase/config/reload_service.go`
- Create: `internal/usecase/config/reload_service_test.go`
- Modify: `cmd/nuimanbotd/main.go` (add /refresh handler)

**Acceptance Criteria:**
- [ ] `/refresh` command reloads config.yaml, users.json, bots.json
- [ ] Configuration changes take effect immediately
- [ ] Active conversations continue uninterrupted
- [ ] Invalid configurations rejected with error message
- [ ] File watcher detects external changes (optional auto-reload)
- [ ] Atomic swap prevents partial configuration states

**Test Coverage:**
- Unit tests: Validator logic, file watcher
- Integration tests: End-to-end reload with valid/invalid configs
- E2E tests: Active conversation continues during reload

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Reload succeeds/fails appropriately
- [ ] No memory leaks during repeated reloads
- [ ] Linter passes
- [ ] Documentation updated

**Dependencies:** Task 1.2

---

#### Task 1.4: File-Based Storage Foundation (5-6h)
**Estimated:** 5.5h

**Implementation Steps:**
1. Define file storage interface in domain layer
2. Implement atomic file write utility (temp file + rename)
3. Implement file locking for concurrent access
4. Implement JSON marshalling/unmarshalling with validation
5. Implement index management for fast lookups
6. Add file backup before modifications

**Files to Create/Modify:**
- Create: `internal/domain/storage.go` (interface)
- Create: `internal/infrastructure/storage/file_storage.go`
- Create: `internal/infrastructure/storage/file_storage_test.go`
- Create: `internal/infrastructure/storage/atomic_writer.go`
- Create: `internal/infrastructure/storage/atomic_writer_test.go`

**Acceptance Criteria:**
- [ ] Atomic file writes prevent corruption
- [ ] File locking prevents race conditions
- [ ] JSON validation before write
- [ ] Backup created before modifications
- [ ] Indexes maintained for fast lookups
- [ ] Graceful handling of corrupted files

**Test Coverage:**
- Unit tests: Atomic writes, locking, validation
- Integration tests: Concurrent writes, corruption recovery
- Performance tests: Write latency <10ms p95

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Concurrent access safe
- [ ] Performance benchmarks met
- [ ] Linter passes
- [ ] Documentation updated

**Dependencies:** Task 1.2

---

#### Task 1.5: Enhanced Audit Logging (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Define audit log entry struct in domain layer
2. Implement audit logger with structured logging (JSON)
3. Add audit log middleware for API calls
4. Implement log rotation (max size, max age)
5. Add audit log query API
6. Implement log filtering and search

**Files to Create/Modify:**
- Create: `internal/domain/audit.go`
- Create: `internal/infrastructure/logging/audit_logger.go`
- Create: `internal/infrastructure/logging/audit_logger_test.go`
- Create: `internal/infrastructure/logging/rotation.go`
- Create: `internal/infrastructure/logging/rotation_test.go`

**Acceptance Criteria:**
- [ ] Audit log records: timestamp, adminUserID, operation, resource, resourceID, changes
- [ ] Log stored in <data-dir>/logs/audit.log
- [ ] Log rotation enabled (max size 100MB, max age 90 days)
- [ ] JSON format for machine parsing
- [ ] Query API for retrieving logs
- [ ] Filter by date range, user, operation

**Test Coverage:**
- Unit tests: Log entry creation, rotation logic
- Integration tests: Log writing, rotation triggers
- E2E tests: Audit trail for admin operations

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Rotation works correctly
- [ ] Log performance: <1ms p95
- [ ] Linter passes
- [ ] Documentation updated

**Dependencies:** Task 1.2

---

#### Task 1.6: STDIO Simplification (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Remove all STDIO commands except /refresh and /exit
2. Update STDIO command parser to reject other commands
3. Update server startup to display available commands
4. Add warning messages for deprecated commands
5. Update user documentation

**Files to Create/Modify:**
- Modify: `cmd/nuimanbotd/main.go`
- Modify: STDIO command handlers
- Modify: Documentation

**Acceptance Criteria:**
- [ ] Only /refresh and /exit commands accepted
- [ ] Other commands show deprecation warning
- [ ] Startup message lists available commands
- [ ] User interactions happen through gateways
- [ ] Server can run headless without terminal

**Test Coverage:**
- Unit tests: Command parsing
- Integration tests: STDIO commands execute correctly
- E2E tests: Server runs without terminal

**Quality Gates:**
- [ ] Unit tests: Command parser coverage >90%
- [ ] Integration tests: Commands work
- [ ] Documentation updated
- [ ] Linter passes

**Dependencies:** Task 1.3

---

#### Task 1.7: Configuration File Restructuring (5-6h)
**Estimated:** 5.5h

**Implementation Steps:**
1. Update config.yaml structure with new sections
2. Add server.paths section
3. Add gateway enable/disable flags
4. Implement LLM provider inheritance
5. Add fallback model selection (primary/secondary/tertiary)
6. Implement configuration migration tool
7. Update configuration parser and validator

**Files to Create/Modify:**
- Modify: `config.yaml` (structure changes)
- Create: `internal/infrastructure/config/migration.go`
- Create: `internal/infrastructure/config/migration_test.go`
- Modify: `internal/infrastructure/config/parser.go`
- Modify: `internal/infrastructure/config/validator.go`

**Acceptance Criteria:**
- [ ] server.paths.config, server.paths.data, server.paths.logs in config.yaml
- [ ] gateways.*.enabled flags present
- [ ] llm.default_model.primary/secondary/tertiary use provider IDs
- [ ] Provider-specific sections (llm.anthropic, llm.openai, etc.)
- [ ] Provider inheritance reduces duplication
- [ ] Migration tool converts old config to new format
- [ ] Backward compatibility with deprecation warnings

**Test Coverage:**
- Unit tests: Parser, validator, migration logic
- Integration tests: Load old and new configs
- E2E tests: Server starts with new config

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Config loading works
- [ ] Migration tool tested on real configs
- [ ] Documentation updated
- [ ] Linter passes

**Dependencies:** Task 1.3

---

**Phase 1 Summary:**
- Total estimated hours: 30-40h
- Critical path: Tasks 1.1 → 1.2 → 1.3 → 1.7
- Parallel work: Tasks 1.4, 1.5, 1.6 can start after 1.2
- Quality gates: All tests >90% coverage, all linters pass, documentation complete

---

### Phase 2: User Profile Management (25-35h)

**Goal:** Implement comprehensive user profile system with file-based storage and multi-platform integration.

**Deliverables:**
- UserProfile domain entity with comprehensive fields
- AgentPreferences and NotesInformation value objects
- File-based UserProfile repository (users.json + user directories)
- UserProfile service with CRUD operations
- Multi-platform ID mapping (Slack, Telegram, CLI)
- User profile validation
- Migration from existing User model

**Tasks:**

#### Task 2.1: UserProfile Domain Entity (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Define UserProfile struct in `internal/domain/user_profile.go`
2. Add fields: userID, moniker, firstName, lastName, nickName, primaryEmail, backupEmail, mobilePhone
3. Add fields: primaryLanguage, secondaryLanguage, timezone, primaryLocation, jobRole, userType
4. Add fields: platformIDs (Slack, Telegram, CLI), enabled, dataDirectory
5. Add timestamps: createdAt, updatedAt, lastVerified
6. Define UserType enum (Individual, Enterprise, Developer, Admin)
7. Implement validation methods
8. Write comprehensive unit tests

**Files to Create/Modify:**
- Create: `internal/domain/user_profile.go`
- Create: `internal/domain/user_profile_test.go`
- Create: `internal/domain/user_type.go`

**Acceptance Criteria:**
- [ ] UserProfile struct with all required fields
- [ ] UserType enum with 4 values
- [ ] Validation methods for email, phone, timezone, language
- [ ] PlatformIDs nested struct (Slack, Telegram, CLI)
- [ ] Immutability patterns (value objects)
- [ ] Comprehensive unit tests (>95% coverage)

**Test Coverage:**
- Unit tests: Field validation, enum values, edge cases
- Property-based tests: Random valid/invalid inputs

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] All validation methods tested
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Phase 1 complete

---

#### Task 2.2: AgentPreferences Value Object (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Define AgentPreferences struct in `internal/domain/agent_preferences.go`
2. Add fields: communicationStyle, verbosity, responseFormat
3. Add fields: codeExamplesPreferred, explainDecisions, proactiveMode
4. Add nested: skillDefaults (map[string]interface{})
5. Add nested: notificationPreferences
6. Implement JSON marshalling/unmarshalling
7. Implement validation and defaults
8. Write unit tests

**Files to Create/Modify:**
- Create: `internal/domain/agent_preferences.go`
- Create: `internal/domain/agent_preferences_test.go`

**Acceptance Criteria:**
- [ ] AgentPreferences struct with all fields
- [ ] Enum values for communicationStyle, verbosity, responseFormat
- [ ] JSON serialization/deserialization
- [ ] Default values for new users
- [ ] Validation for enum fields
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: JSON marshalling, validation, defaults
- Integration tests: Preferences applied to agent behavior

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] JSON roundtrip tests pass
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 2.1

---

#### Task 2.3: NotesInformation Value Object (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Define NotesInformation struct in `internal/domain/notes_information.go`
2. Add nested: adminNotes array (timestamp, authorID, note)
3. Add nested: flags (betaTester, earlyAccess, supportTickets)
4. Add nested: restrictions (rateLimitOverride, featureAccess)
5. Add nested: customMetadata (flexible key-value)
6. Implement JSON marshalling
7. Write unit tests

**Files to Create/Modify:**
- Create: `internal/domain/notes_information.go`
- Create: `internal/domain/notes_information_test.go`

**Acceptance Criteria:**
- [ ] NotesInformation struct with all nested fields
- [ ] AdminNote struct with timestamp, authorID, note
- [ ] JSON serialization/deserialization
- [ ] Helper methods for adding/removing notes
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: JSON marshalling, note management
- Integration tests: Admin notes audit trail

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] JSON tests pass
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 2.1

---

#### Task 2.4: UserProfile Repository Interface (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Define UserProfileRepository interface in `internal/domain/user_profile_repository.go`
2. Add methods: Create, GetByID, GetByUsername, GetByEmail, GetByPlatformID
3. Add methods: Update, Delete, List, Search
4. Add methods: UpdatePartial (for REST API)
5. Define query filters and pagination
6. Write interface documentation

**Files to Create/Modify:**
- Create: `internal/domain/user_profile_repository.go`

**Acceptance Criteria:**
- [ ] Repository interface with all CRUD methods
- [ ] Query methods for username, email, platform IDs
- [ ] Partial update support
- [ ] Pagination support (limit, offset)
- [ ] Filter support (enabled, userType, etc.)
- [ ] Clear documentation of each method

**Test Coverage:**
- N/A (interface definition only)

**Quality Gates:**
- [ ] Interface documented
- [ ] Linter passes

**Dependencies:** Tasks 2.1, 2.2, 2.3

---

#### Task 2.5: File-Based UserProfile Repository Implementation (8-10h)
**Estimated:** 9h

**Implementation Steps:**
1. Implement FileUserProfileRepository in `internal/infrastructure/storage/user_profile_repository.go`
2. Implement users.json central registry with indexes
3. Implement user directory structure (<data-dir>/users/<user-id>/)
4. Implement user-specific files (profile.json, preferences.json)
5. Implement atomic writes and file locking
6. Implement index management (byUsername, byEmail, byPlatform)
7. Implement query and search operations
8. Implement partial update logic
9. Write comprehensive unit and integration tests

**Files to Create/Modify:**
- Create: `internal/infrastructure/storage/user_profile_repository.go`
- Create: `internal/infrastructure/storage/user_profile_repository_test.go`
- Create: `internal/infrastructure/storage/user_index.go`
- Create: `internal/infrastructure/storage/user_index_test.go`

**Acceptance Criteria:**
- [ ] users.json contains central registry with indexes
- [ ] User directories created automatically
- [ ] profile.json and preferences.json in user directories
- [ ] Atomic writes prevent corruption
- [ ] Indexes enable fast lookups (O(1) for username/email/platformID)
- [ ] Partial updates only modify specified fields
- [ ] Concurrent access safe (file locking)
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Index management, file operations
- Integration tests: CRUD operations, concurrent access
- Performance tests: Lookup latency <5ms p95

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: All CRUD operations work
- [ ] Concurrent access tests pass
- [ ] Performance benchmarks met
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Tasks 1.4, 2.4

---

#### Task 2.6: UserProfile Service (Use Case) (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Define UserProfileService in `internal/usecase/user/profile_service.go`
2. Implement business logic for user creation
3. Implement validation logic (email format, phone format, timezone, language)
4. Implement unique constraint checks (username, email, platformIDs)
5. Implement multi-platform ID mapping
6. Implement user search and filtering
7. Write unit tests with mocks

**Files to Create/Modify:**
- Create: `internal/usecase/user/profile_service.go`
- Create: `internal/usecase/user/profile_service_test.go`
- Create: `internal/usecase/user/validation.go`
- Create: `internal/usecase/user/validation_test.go`

**Acceptance Criteria:**
- [ ] CreateUserProfile with validation
- [ ] UpdateUserProfile with partial update support
- [ ] DeleteUserProfile with soft delete option
- [ ] GetUserProfile by various identifiers
- [ ] SearchUsers with filters and pagination
- [ ] Platform ID uniqueness enforced
- [ ] Email/username uniqueness enforced
- [ ] Unit tests >90% coverage

**Test Coverage:**
- Unit tests: All service methods, validation logic
- Integration tests: End-to-end user creation workflow

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] All validation scenarios tested
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Tasks 2.5

---

#### Task 2.7: User Migration from Existing Model (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Create migration script in `internal/infrastructure/migration/user_migration.go`
2. Read existing User entities from database
3. Map User fields to UserProfile fields
4. Generate default values for new fields
5. Create users.json registry
6. Create user directories and files
7. Implement dry-run mode
8. Write migration tests

**Files to Create/Modify:**
- Create: `internal/infrastructure/migration/user_migration.go`
- Create: `internal/infrastructure/migration/user_migration_test.go`
- Create: `cmd/nuimanbot/migrate_users.go` (CLI command)

**Acceptance Criteria:**
- [ ] Migration script reads existing users
- [ ] All User fields mapped to UserProfile
- [ ] Defaults provided for new fields (timezone=UTC, primaryLanguage=en)
- [ ] users.json created with indexes
- [ ] User directories created
- [ ] Dry-run mode shows what would be migrated
- [ ] Migration idempotent (can run multiple times)

**Test Coverage:**
- Unit tests: Field mapping logic
- Integration tests: End-to-end migration with test data

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Migration succeeds
- [ ] Tested on production-like data
- [ ] Rollback procedure documented
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Tasks 2.6

---

**Phase 2 Summary:**
- Total estimated hours: 25-35h
- Critical path: Tasks 2.1 → 2.4 → 2.5 → 2.6 → 2.7
- Parallel work: Tasks 2.2, 2.3 can run in parallel with 2.1
- Quality gates: All tests >90% coverage, migration tested, documentation complete

---

### Phase 3: Bot Management (20-30h)

**Goal:** Implement database-driven bot configuration management with public/private bot support and dynamic enable/disable.

**Deliverables:**
- BotConfig domain entity (Slack and Telegram)
- Bot repository (file-based in bots.json)
- Bot management service with CRUD operations
- Public/private bot logic
- Dynamic enable/disable without restart
- Gateway integration for bot lifecycle
- Bot credential encryption
- Migration from config files

**Tasks:**

#### Task 3.1: BotConfig Domain Entity (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Define BotConfig base interface in `internal/domain/bot_config.go`
2. Define SlackBotConfig struct with Slack-specific fields
3. Define TelegramBotConfig struct with Telegram-specific fields
4. Add common fields: botID, botName, botType, ownerUserID, enabled
5. Add Slack fields: slackBotToken, slackAppToken, slackSigningSecret, slackTeamID, slackBotUserID
6. Add Telegram fields: telegramBotToken, telegramBotUsername, telegramBotID
7. Add access control: allowedUserIDs, allowedChatIDs (Telegram)
8. Define BotType enum (Public, Private)
9. Implement validation methods
10. Write comprehensive unit tests

**Files to Create/Modify:**
- Create: `internal/domain/bot_config.go`
- Create: `internal/domain/bot_config_test.go`
- Create: `internal/domain/bot_type.go`

**Acceptance Criteria:**
- [ ] BotConfig interface for common operations
- [ ] SlackBotConfig struct with all Slack fields
- [ ] TelegramBotConfig struct with all Telegram fields
- [ ] BotType enum (Public, Private)
- [ ] Validation methods for tokens, IDs
- [ ] Access control fields (allowedUserIDs)
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: Validation, enum values, field constraints

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] Validation tested thoroughly
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Phase 2 complete

---

#### Task 3.2: Bot Repository Interface (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Define BotRepository interface in `internal/domain/bot_repository.go`
2. Add methods for Slack bots: CreateSlackBot, GetSlackBot, UpdateSlackBot, DeleteSlackBot, ListSlackBots
3. Add methods for Telegram bots: CreateTelegramBot, GetTelegramBot, UpdateTelegramBot, DeleteTelegramBot, ListTelegramBots
4. Add query methods: GetBotsByOwner, GetBotsByType, GetEnabledBots
5. Define query filters and pagination

**Files to Create/Modify:**
- Create: `internal/domain/bot_repository.go`

**Acceptance Criteria:**
- [ ] Repository interface with CRUD methods for Slack and Telegram
- [ ] Query methods for filtering by owner, type, enabled status
- [ ] Pagination support
- [ ] Clear documentation

**Test Coverage:**
- N/A (interface definition)

**Quality Gates:**
- [ ] Interface documented
- [ ] Linter passes

**Dependencies:** Task 3.1

---

#### Task 3.3: File-Based Bot Repository Implementation (6-8h)
**Estimated:** 7h

**Implementation Steps:**
1. Implement FileBotRepository in `internal/infrastructure/storage/bot_repository.go`
2. Implement bots.json structure with slackBots and telegramBots arrays
3. Implement indexes: byName, byOwner, byPlatformBotID
4. Implement atomic writes and file locking
5. Implement credential encryption at rest (AES-256)
6. Implement query and filter operations
7. Write comprehensive unit and integration tests

**Files to Create/Modify:**
- Create: `internal/infrastructure/storage/bot_repository.go`
- Create: `internal/infrastructure/storage/bot_repository_test.go`
- Create: `internal/infrastructure/storage/bot_index.go`
- Create: `internal/infrastructure/storage/bot_encryption.go`
- Create: `internal/infrastructure/storage/bot_encryption_test.go`

**Acceptance Criteria:**
- [ ] bots.json contains slackBots and telegramBots arrays
- [ ] Indexes enable fast lookups
- [ ] Credentials encrypted at rest (AES-256-GCM)
- [ ] Atomic writes prevent corruption
- [ ] Concurrent access safe
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Encryption, index management
- Integration tests: CRUD operations, concurrent access
- Security tests: Credential encryption verified

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: All operations work
- [ ] Security review complete
- [ ] Performance benchmarks met
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Tasks 1.4, 3.2

---

#### Task 3.4: Bot Management Service (5-6h)
**Estimated:** 5.5h

**Implementation Steps:**
1. Define BotManagementService in `internal/usecase/bot/management_service.go`
2. Implement bot creation with validation
3. Implement bot update with credential rotation
4. Implement bot deletion with cleanup
5. Implement public bot access control logic
6. Implement private bot ownership verification
7. Implement bot enable/disable logic
8. Write unit tests with mocks

**Files to Create/Modify:**
- Create: `internal/usecase/bot/management_service.go`
- Create: `internal/usecase/bot/management_service_test.go`
- Create: `internal/usecase/bot/access_control.go`
- Create: `internal/usecase/bot/access_control_test.go`

**Acceptance Criteria:**
- [ ] CreateBot with validation and access control
- [ ] UpdateBot with partial update support
- [ ] DeleteBot with orphaned conversation cleanup
- [ ] EnableBot / DisableBot operations
- [ ] Public bot access restricted to allowedUserIDs
- [ ] Private bot access restricted to owner
- [ ] Admin override for all operations
- [ ] Unit tests >90% coverage

**Test Coverage:**
- Unit tests: All service methods, access control logic
- Integration tests: End-to-end bot lifecycle

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Access control thoroughly tested
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 3.3

---

#### Task 3.5: Gateway Integration for Dynamic Bot Lifecycle (6-8h)
**Estimated:** 7h

**Implementation Steps:**
1. Implement bot lifecycle manager in `internal/infrastructure/gateway/bot_lifecycle.go`
2. Implement file watcher for bots.json changes
3. Implement bot connection management (connect/disconnect)
4. Implement graceful conversation closure on bot disable
5. Integrate with Slack gateway
6. Integrate with Telegram gateway
7. Implement bot status tracking (connected, disconnected, error)
8. Write integration tests

**Files to Create/Modify:**
- Create: `internal/infrastructure/gateway/bot_lifecycle.go`
- Create: `internal/infrastructure/gateway/bot_lifecycle_test.go`
- Modify: `internal/infrastructure/gateway/slack/gateway.go`
- Modify: `internal/infrastructure/gateway/telegram/gateway.go`

**Acceptance Criteria:**
- [ ] Bot lifecycle manager watches bots.json
- [ ] New enabled bots connect automatically
- [ ] Disabled bots disconnect gracefully
- [ ] Active conversations closed with notification
- [ ] Bot status tracked and queryable
- [ ] No gateway restart required
- [ ] Integration tests verify behavior

**Test Coverage:**
- Unit tests: Lifecycle logic
- Integration tests: Bot connect/disconnect, file watcher
- E2E tests: Bot enable/disable without restart

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Lifecycle works
- [ ] E2E tests: No downtime on bot changes
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Tasks 3.4

---

#### Task 3.6: Bot Migration from Config Files (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Create migration script in `internal/infrastructure/migration/bot_migration.go`
2. Read bot configurations from config.yaml
3. Map Slack gateway config to SlackBotConfig
4. Map Telegram gateway config to TelegramBotConfig
5. Create bots.json with migrated bots
6. Encrypt credentials during migration
7. Implement dry-run mode
8. Write migration tests

**Files to Create/Modify:**
- Create: `internal/infrastructure/migration/bot_migration.go`
- Create: `internal/infrastructure/migration/bot_migration_test.go`
- Create: `cmd/nuimanbot/migrate_bots.go` (CLI command)

**Acceptance Criteria:**
- [ ] Migration reads Slack/Telegram configs from config.yaml
- [ ] Bots created in bots.json with correct fields
- [ ] Credentials encrypted
- [ ] Dry-run mode shows changes
- [ ] Migration idempotent
- [ ] Rollback procedure documented

**Test Coverage:**
- Unit tests: Config parsing, field mapping
- Integration tests: End-to-end migration

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Integration tests: Migration succeeds
- [ ] Credentials properly encrypted
- [ ] Documentation complete
- [ ] Linter passes

**Dependencies:** Task 3.5

---

**Phase 3 Summary:**
- Total estimated hours: 20-30h
- Critical path: Tasks 3.1 → 3.2 → 3.3 → 3.4 → 3.5 → 3.6
- Parallel work: None (sequential dependencies)
- Quality gates: All tests >90% coverage, security review complete, migration tested

---

### Phase 4: REST API (20-25h)

**Goal:** Implement comprehensive REST API with authentication, authorization, and full CRUD operations for all admin functions.

**Deliverables:**
- API router setup with Chi framework
- Authentication middleware (Bearer token)
- Authorization middleware (RBAC)
- User CRUD endpoints
- Bot CRUD endpoints
- Config management endpoints
- Partial update implementation
- Audit logging for all operations
- API documentation (OpenAPI/Swagger)

**Tasks:**

#### Task 4.1: API Router Setup (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Set up Chi router in `internal/adapter/api/router.go`
2. Configure middleware stack (logging, recovery, CORS)
3. Define API versioning (/api/v1)
4. Implement health check endpoint
5. Implement metrics endpoint
6. Set up request/response logging
7. Write router tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/router.go`
- Create: `internal/adapter/api/router_test.go`
- Create: `internal/adapter/api/middleware/logging.go`
- Create: `internal/adapter/api/middleware/recovery.go`

**Acceptance Criteria:**
- [ ] Chi router configured
- [ ] API versioning (/api/v1)
- [ ] Health check at /api/v1/health
- [ ] Metrics at /api/v1/metrics
- [ ] Request/response logging
- [ ] CORS configured
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Router configuration
- Integration tests: Middleware stack, endpoints

**Quality Gates:**
- [ ] Integration tests: >90% coverage
- [ ] Health check responds
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Phase 3 complete

---

#### Task 4.2: Authentication Middleware (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Implement Bearer token authentication in `internal/adapter/api/middleware/auth.go`
2. Implement API key validation against users.json
3. Implement token extraction from Authorization header
4. Implement user lookup and caching
5. Add request context with authenticated user
6. Implement 401 Unauthorized responses
7. Write middleware tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/middleware/auth.go`
- Create: `internal/adapter/api/middleware/auth_test.go`

**Acceptance Criteria:**
- [ ] Bearer token authentication
- [ ] API key validated against users.json
- [ ] User loaded into request context
- [ ] Invalid tokens return 401 Unauthorized
- [ ] Token caching for performance
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: Token validation, user lookup
- Integration tests: Authenticated requests

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] Security review complete
- [ ] Performance benchmarks met (<5ms p95)
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 4.1

---

#### Task 4.3: Authorization Middleware (RBAC) (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Implement RBAC middleware in `internal/adapter/api/middleware/authz.go`
2. Define role permissions (Admin, User, Service)
3. Implement resource-level access control
4. Implement owner-based access (users can access own resources)
5. Add 403 Forbidden responses
6. Write middleware tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/middleware/authz.go`
- Create: `internal/adapter/api/middleware/authz_test.go`
- Create: `internal/domain/rbac.go` (permissions)

**Acceptance Criteria:**
- [ ] Admin role has full access
- [ ] User role can access only own resources
- [ ] Service role has read-only access
- [ ] Resource ownership verified
- [ ] 403 Forbidden for unauthorized access
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: Permission checks, role logic
- Integration tests: Access control scenarios

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] All RBAC scenarios tested
- [ ] Security review complete
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 4.2

---

#### Task 4.4: User CRUD Endpoints (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Implement user handlers in `internal/adapter/api/handlers/user_handler.go`
2. POST /api/v1/admin/users (create user)
3. GET /api/v1/admin/users (list users with pagination)
4. GET /api/v1/admin/users/:id (get user by ID)
5. PUT /api/v1/admin/users/:id (update user, partial updates)
6. DELETE /api/v1/admin/users/:id (delete user)
7. Implement request/response DTOs
8. Implement validation
9. Integrate with UserProfileService
10. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/handlers/user_handler.go`
- Create: `internal/adapter/api/handlers/user_handler_test.go`
- Create: `internal/adapter/api/dto/user_dto.go`

**Acceptance Criteria:**
- [ ] All CRUD endpoints implemented
- [ ] Partial update support (PUT updates only specified fields)
- [ ] Request validation
- [ ] Error responses with details
- [ ] Pagination for list endpoint
- [ ] Integration with UserProfileService
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Handler logic, validation
- Integration tests: End-to-end CRUD operations

**Quality Gates:**
- [ ] Integration tests: >90% coverage
- [ ] All error scenarios tested
- [ ] API documented
- [ ] Linter passes

**Dependencies:** Tasks 2.6, 4.3

---

#### Task 4.5: Bot CRUD Endpoints (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Implement bot handlers in `internal/adapter/api/handlers/bot_handler.go`
2. POST /api/v1/admin/bots/slack (create Slack bot)
3. GET /api/v1/admin/bots/slack (list Slack bots)
4. PUT /api/v1/admin/bots/slack/:id (update Slack bot)
5. DELETE /api/v1/admin/bots/slack/:id (delete Slack bot)
6. Repeat for Telegram bots
7. Implement credential masking in responses
8. Implement access control (owner or admin)
9. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/handlers/bot_handler.go`
- Create: `internal/adapter/api/handlers/bot_handler_test.go`
- Create: `internal/adapter/api/dto/bot_dto.go`

**Acceptance Criteria:**
- [ ] All CRUD endpoints for Slack and Telegram bots
- [ ] Credential masking in responses (show only last 4 chars)
- [ ] Access control enforced (owner or admin)
- [ ] Partial update support
- [ ] Integration with BotManagementService
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Handler logic, credential masking
- Integration tests: CRUD operations, access control

**Quality Gates:**
- [ ] Integration tests: >90% coverage
- [ ] Security review complete
- [ ] API documented
- [ ] Linter passes

**Dependencies:** Tasks 3.4, 4.3

---

#### Task 4.6: Config Management Endpoints (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Implement config handlers in `internal/adapter/api/handlers/config_handler.go`
2. GET /api/v1/admin/config (get full config)
3. PUT /api/v1/admin/config/llm (update LLM config)
4. PUT /api/v1/admin/config/server (update server config)
5. POST /api/v1/admin/config/reload (trigger reload)
6. GET /api/v1/admin/status (server status)
7. Implement config validation before write
8. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/handlers/config_handler.go`
- Create: `internal/adapter/api/handlers/config_handler_test.go`
- Create: `internal/adapter/api/dto/config_dto.go`

**Acceptance Criteria:**
- [ ] Config read/update endpoints
- [ ] Reload endpoint triggers configuration reload
- [ ] Validation before config write
- [ ] Status endpoint returns server metrics
- [ ] Admin-only access
- [ ] Integration tests >90% coverage

**Test Coverage:**
- Unit tests: Config validation
- Integration tests: Config reload workflow

**Quality Gates:**
- [ ] Integration tests: >90% coverage
- [ ] Config validation comprehensive
- [ ] API documented
- [ ] Linter passes

**Dependencies:** Tasks 1.3, 4.3

---

#### Task 4.7: Audit Logging Integration (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Add audit logging middleware in `internal/adapter/api/middleware/audit.go`
2. Log all POST/PUT/DELETE operations
3. Capture: timestamp, user, operation, resource, changes
4. Integrate with audit logger from Phase 1
5. Implement log filtering endpoint GET /api/v1/admin/logs
6. Write middleware tests

**Files to Create/Modify:**
- Create: `internal/adapter/api/middleware/audit.go`
- Create: `internal/adapter/api/middleware/audit_test.go`
- Create: `internal/adapter/api/handlers/log_handler.go`

**Acceptance Criteria:**
- [ ] All mutations logged
- [ ] Log entries include user, timestamp, operation, changes
- [ ] Log query endpoint with filtering
- [ ] Integration with Phase 1 audit logger
- [ ] Unit tests >90% coverage

**Test Coverage:**
- Unit tests: Log capture logic
- Integration tests: Audit trail verification

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Audit trail complete
- [ ] API documented
- [ ] Linter passes

**Dependencies:** Tasks 1.5, 4.3

---

#### Task 4.8: API Documentation (OpenAPI) (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Set up Swagger/OpenAPI documentation
2. Document all endpoints with request/response schemas
3. Add authentication documentation
4. Add example requests/responses
5. Set up Swagger UI endpoint
6. Generate API client libraries (optional)
7. Write API usage guide

**Files to Create/Modify:**
- Create: `api/openapi.yaml`
- Create: `documentation/api-guide.md`
- Modify: Router to serve Swagger UI

**Acceptance Criteria:**
- [ ] OpenAPI spec complete for all endpoints
- [ ] Swagger UI accessible at /api/docs
- [ ] Request/response schemas documented
- [ ] Authentication documented
- [ ] Example requests provided
- [ ] API usage guide written

**Test Coverage:**
- Manual tests: Swagger UI loads and works

**Quality Gates:**
- [ ] OpenAPI spec validates
- [ ] All endpoints documented
- [ ] Examples tested
- [ ] Documentation complete

**Dependencies:** Tasks 4.4, 4.5, 4.6, 4.7

---

**Phase 4 Summary:**
- Total estimated hours: 20-25h
- Critical path: Tasks 4.1 → 4.2 → 4.3 → 4.4/4.5/4.6 → 4.7 → 4.8
- Parallel work: Tasks 4.4, 4.5, 4.6 can run in parallel after 4.3
- Quality gates: All tests >90% coverage, security review complete, API documented

---

### Phase 5: Web Admin Interface (30-40h)

**Goal:** Implement browser-based administration interface with all management capabilities.

**Deliverables:**
- Web server setup with Chi and templates
- Session management and authentication
- CSRF protection
- Dashboard page
- LLM configuration page
- Server configuration page
- User management page
- Bot management page
- Activity log page
- Responsive UI with Tailwind CSS and HTMX

**Tasks:**

#### Task 5.1: Web Server Setup (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Set up web router in `internal/adapter/web/router.go`
2. Configure template rendering (html/template)
3. Set up static file serving (CSS, JS)
4. Implement base layout template
5. Configure admin port (default 8080)
6. Set up HTTPS support (optional)
7. Write integration tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/router.go`
- Create: `internal/adapter/web/templates/base.html`
- Create: `internal/adapter/web/static/css/style.css`
- Create: `cmd/nuimanbotd/web.go` (web server startup)

**Acceptance Criteria:**
- [ ] Web server runs on configurable port (default 8080)
- [ ] Template rendering works
- [ ] Static files served correctly
- [ ] Base layout with header/footer
- [ ] HTTPS support (with provided certs)
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Integration tests: Server startup, template rendering

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] Web server starts successfully
- [ ] Templates render
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Phase 4 complete

---

#### Task 5.2: Session Management and Authentication (5-6h)
**Estimated:** 5.5h

**Implementation Steps:**
1. Implement session management in `internal/adapter/web/session/manager.go`
2. Use secure HTTP-only cookies
3. Implement login page and handler
4. Implement logout handler
5. Implement session middleware (redirect to login if not authenticated)
6. Implement "remember me" functionality
7. Implement session timeout (24 hours)
8. Write session tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/session/manager.go`
- Create: `internal/adapter/web/session/manager_test.go`
- Create: `internal/adapter/web/handlers/auth_handler.go`
- Create: `internal/adapter/web/templates/login.html`
- Create: `internal/adapter/web/middleware/session.go`

**Acceptance Criteria:**
- [ ] Login page at /admin/login
- [ ] Session cookie secure, HTTP-only
- [ ] Session timeout 24 hours (configurable)
- [ ] Remember me extends timeout to 30 days
- [ ] Logout clears session
- [ ] Middleware redirects unauthenticated users
- [ ] Unit tests >90% coverage

**Test Coverage:**
- Unit tests: Session logic
- Integration tests: Login/logout flow

**Quality Gates:**
- [ ] Unit tests: >90% coverage
- [ ] Security review complete
- [ ] Sessions secure
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 5.1

---

#### Task 5.3: CSRF Protection (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Implement CSRF middleware in `internal/adapter/web/middleware/csrf.go`
2. Generate CSRF tokens per session
3. Add CSRF token to all forms
4. Validate CSRF token on POST/PUT/DELETE
5. Return 403 on invalid token
6. Write CSRF tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/middleware/csrf.go`
- Create: `internal/adapter/web/middleware/csrf_test.go`

**Acceptance Criteria:**
- [ ] CSRF tokens generated per session
- [ ] Tokens included in all forms
- [ ] Validation on mutations
- [ ] 403 Forbidden on invalid token
- [ ] Unit tests >95% coverage

**Test Coverage:**
- Unit tests: Token generation, validation
- Integration tests: CSRF protection in action

**Quality Gates:**
- [ ] Unit tests: >95% coverage
- [ ] Security review complete
- [ ] CSRF protection verified
- [ ] Linter passes

**Dependencies:** Task 5.2

---

#### Task 5.4: Dashboard Page (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Implement dashboard handler in `internal/adapter/web/handlers/dashboard_handler.go`
2. Create dashboard template with server status
3. Show active connections (Slack/Telegram bots, CLI sessions)
4. Show recent activity log (last 50 entries)
5. Show quick stats (users, bots, LLM requests, error rate)
6. Add reload configuration button
7. Implement with HTMX for dynamic updates
8. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/dashboard_handler.go`
- Create: `internal/adapter/web/handlers/dashboard_handler_test.go`
- Create: `internal/adapter/web/templates/dashboard.html`

**Acceptance Criteria:**
- [ ] Dashboard at /admin/
- [ ] Server status displayed (uptime, version, memory)
- [ ] Active connections listed
- [ ] Recent activity log (last 50 entries)
- [ ] Quick stats cards
- [ ] Reload config button
- [ ] Dynamic updates with HTMX
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Handler logic
- Integration tests: Dashboard renders correctly

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] Dashboard functional
- [ ] HTMX integration works
- [ ] Linter passes

**Dependencies:** Task 5.3

---

#### Task 5.5: LLM Configuration Page (5-6h)
**Estimated:** 5.5h

**Implementation Steps:**
1. Implement LLM config handler in `internal/adapter/web/handlers/llm_handler.go`
2. Create LLM config template
3. Section: Default model selection (dropdowns for primary/secondary/tertiary)
4. Section: Provider configuration forms (Anthropic, OpenAI, Bedrock, Ollama)
5. Section: Model instances table (list, add, edit, delete)
6. Implement form validation
7. Implement save changes with HTMX
8. Add test connection button
9. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/llm_handler.go`
- Create: `internal/adapter/web/handlers/llm_handler_test.go`
- Create: `internal/adapter/web/templates/llm_config.html`
- Create: `internal/adapter/web/templates/partials/model_form.html`

**Acceptance Criteria:**
- [ ] LLM config page at /admin/llm
- [ ] Default model dropdowns
- [ ] Provider config forms
- [ ] Model instance table with CRUD
- [ ] Form validation
- [ ] Save changes updates config.yaml
- [ ] Test connection verifies credentials
- [ ] HTMX for dynamic updates
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Form parsing, validation
- Integration tests: Config update workflow

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] Forms work correctly
- [ ] Config updates persist
- [ ] Linter passes
- [ ] Documentation complete

**Dependencies:** Task 5.3

---

#### Task 5.6: Server Configuration Page (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Implement server config handler in `internal/adapter/web/handlers/server_handler.go`
2. Create server config template
3. Section: Server paths (config, data, logs)
4. Section: Logging configuration (log level, debug mode)
5. Section: Gateway enable/disable toggles
6. Implement form validation
7. Implement save changes with reload prompt
8. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/server_handler.go`
- Create: `internal/adapter/web/handlers/server_handler_test.go`
- Create: `internal/adapter/web/templates/server_config.html`

**Acceptance Criteria:**
- [ ] Server config page at /admin/server
- [ ] Path configuration forms
- [ ] Logging configuration forms
- [ ] Gateway enable/disable toggles
- [ ] Form validation
- [ ] Save changes updates config.yaml
- [ ] Prompt to reload configuration
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Form parsing, validation
- Integration tests: Config update workflow

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] Forms work correctly
- [ ] Config updates persist
- [ ] Linter passes

**Dependencies:** Task 5.3

---

#### Task 5.7: User Management Page (6-8h)
**Estimated:** 7h

**Implementation Steps:**
1. Implement user handler in `internal/adapter/web/handlers/user_handler.go`
2. Create user list template with search and filters
3. Create user form template (add/edit)
4. Implement user creation form
5. Implement user edit form with tabs (Basic Info, Profile, Access)
6. Implement user deletion with confirmation
7. Implement platform ID management
8. Implement agent preferences editor
9. Add search and pagination
10. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/user_handler.go`
- Create: `internal/adapter/web/handlers/user_handler_test.go`
- Create: `internal/adapter/web/templates/users.html`
- Create: `internal/adapter/web/templates/user_form.html`
- Create: `internal/adapter/web/templates/partials/user_row.html`

**Acceptance Criteria:**
- [ ] User list page at /admin/users
- [ ] Search and filter functionality
- [ ] Add user form
- [ ] Edit user form with tabs
- [ ] Delete user with confirmation
- [ ] Platform ID linking
- [ ] Agent preferences editor
- [ ] Pagination (50 users per page)
- [ ] HTMX for dynamic updates
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Form parsing, validation
- Integration tests: User CRUD workflow, search

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] All forms work
- [ ] Search and pagination work
- [ ] Linter passes

**Dependencies:** Task 5.3

---

#### Task 5.8: Bot Management Page (6-8h)
**Estimated:** 7h

**Implementation Steps:**
1. Implement bot handler in `internal/adapter/web/handlers/bot_handler.go`
2. Create bot list template with tabs (Slack, Telegram)
3. Create bot form templates (add/edit for Slack and Telegram)
4. Implement bot creation forms
5. Implement bot edit forms
6. Implement bot deletion with confirmation
7. Implement public bot user access management
8. Add test connection button
9. Add bot status indicators
10. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/bot_handler.go`
- Create: `internal/adapter/web/handlers/bot_handler_test.go`
- Create: `internal/adapter/web/templates/bots.html`
- Create: `internal/adapter/web/templates/slack_bot_form.html`
- Create: `internal/adapter/web/templates/telegram_bot_form.html`
- Create: `internal/adapter/web/templates/partials/bot_row.html`

**Acceptance Criteria:**
- [ ] Bot list page at /admin/bots with tabs
- [ ] Add bot forms for Slack and Telegram
- [ ] Edit bot forms
- [ ] Delete bot with confirmation
- [ ] Public bot user access UI
- [ ] Test connection button
- [ ] Bot status indicators (connected, disconnected, error)
- [ ] HTMX for dynamic updates
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Form parsing, validation
- Integration tests: Bot CRUD workflow

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] All forms work
- [ ] Bot lifecycle management works
- [ ] Linter passes

**Dependencies:** Task 5.3

---

#### Task 5.9: Activity Log Page (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Implement log handler in `internal/adapter/web/handlers/log_handler.go`
2. Create log viewer template
3. Implement log table with columns (timestamp, user, action)
4. Add filters (log level, time range)
5. Add search functionality
6. Add pagination (50 entries per page)
7. Add export to CSV/JSON
8. Write handler tests

**Files to Create/Modify:**
- Create: `internal/adapter/web/handlers/log_handler.go`
- Create: `internal/adapter/web/handlers/log_handler_test.go`
- Create: `internal/adapter/web/templates/logs.html`

**Acceptance Criteria:**
- [ ] Log viewer at /admin/logs
- [ ] Log table with columns
- [ ] Filter by log level and time range
- [ ] Search functionality
- [ ] Pagination (50 per page)
- [ ] Export to CSV/JSON
- [ ] HTMX for dynamic filtering
- [ ] Integration tests >85% coverage

**Test Coverage:**
- Unit tests: Filter logic
- Integration tests: Log display, filtering, export

**Quality Gates:**
- [ ] Integration tests: >85% coverage
- [ ] Filtering works
- [ ] Export works
- [ ] Linter passes

**Dependencies:** Task 5.3

---

#### Task 5.10: Responsive UI and Styling (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Set up Tailwind CSS
2. Create responsive layouts for all pages
3. Implement mobile-friendly navigation
4. Add loading indicators
5. Add success/error notifications
6. Implement consistent styling across pages
7. Add dark mode support (optional)
8. Test on multiple screen sizes

**Files to Create/Modify:**
- Modify: `internal/adapter/web/static/css/style.css`
- Create: `internal/adapter/web/templates/partials/notification.html`
- Modify: All templates for responsive design

**Acceptance Criteria:**
- [ ] Tailwind CSS integrated
- [ ] All pages responsive (mobile, tablet, desktop)
- [ ] Mobile-friendly navigation menu
- [ ] Loading indicators on async operations
- [ ] Toast notifications for success/error
- [ ] Consistent styling across pages
- [ ] Dark mode support (optional)

**Test Coverage:**
- Manual tests: UI on different screen sizes

**Quality Gates:**
- [ ] All pages responsive
- [ ] UI polished and professional
- [ ] Accessibility standards met
- [ ] Documentation complete

**Dependencies:** Tasks 5.4-5.9

---

**Phase 5 Summary:**
- Total estimated hours: 30-40h
- Critical path: Tasks 5.1 → 5.2 → 5.3 → 5.4-5.9 → 5.10
- Parallel work: Tasks 5.4-5.9 can partially overlap after 5.3
- Quality gates: All tests >85% coverage, UI functional and responsive

---

### Phase 6: Documentation and Deployment (15-20h)

**Goal:** Complete product documentation, migration guides, and deployment resources.

**Deliverables:**
- Updated README
- Admin user guide
- API documentation
- Configuration guide
- Migration guide
- Deployment guide (Docker, systemd)
- Troubleshooting guide
- Architecture diagrams

**Tasks:**

#### Task 6.1: Update README (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Update project overview
2. Add quick start guide
3. Add installation instructions
4. Add basic usage examples
5. Add links to detailed documentation
6. Update screenshots

**Files to Create/Modify:**
- Modify: `README.md`

**Acceptance Criteria:**
- [ ] Overview reflects multi-component architecture
- [ ] Quick start guide updated
- [ ] Installation instructions for server and CLI
- [ ] Basic usage examples
- [ ] Links to detailed docs
- [ ] Screenshots of web interface

**Quality Gates:**
- [ ] README complete and accurate
- [ ] Links verified
- [ ] Examples tested

**Dependencies:** Phase 5 complete

---

#### Task 6.2: Admin User Guide (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Write web interface guide
2. Document all admin pages
3. Add step-by-step workflows (create user, add bot, etc.)
4. Add screenshots for each page
5. Document CLI commands
6. Add troubleshooting section

**Files to Create/Modify:**
- Create: `documentation/admin-guide.md`

**Acceptance Criteria:**
- [ ] Complete guide for web interface
- [ ] Step-by-step workflows
- [ ] Screenshots for all pages
- [ ] CLI command reference
- [ ] Troubleshooting section

**Quality Gates:**
- [ ] Guide complete and clear
- [ ] All workflows tested
- [ ] Screenshots current

**Dependencies:** Phase 5 complete

---

#### Task 6.3: API Documentation (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Write API overview
2. Document authentication
3. Document all endpoints with examples
4. Add error code reference
5. Add rate limiting documentation
6. Add API client examples (curl, Python, Go)

**Files to Create/Modify:**
- Create: `documentation/api-guide.md`
- Update: `api/openapi.yaml`

**Acceptance Criteria:**
- [ ] API overview with authentication
- [ ] All endpoints documented with examples
- [ ] Error code reference
- [ ] Rate limiting documented
- [ ] Client examples in multiple languages

**Quality Gates:**
- [ ] Documentation complete
- [ ] Examples tested
- [ ] OpenAPI spec current

**Dependencies:** Phase 4 complete

---

#### Task 6.4: Configuration Guide (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Document config.yaml structure
2. Document all configuration options
3. Document environment variable overrides
4. Add configuration examples for common scenarios
5. Document users.json and bots.json structure
6. Add validation rules

**Files to Create/Modify:**
- Create: `documentation/configuration-guide.md`

**Acceptance Criteria:**
- [ ] Complete config.yaml reference
- [ ] All options documented
- [ ] Environment variable overrides listed
- [ ] Example configurations
- [ ] users.json and bots.json documented
- [ ] Validation rules listed

**Quality Gates:**
- [ ] Guide complete
- [ ] Examples tested
- [ ] Validation documented

**Dependencies:** Phase 1 complete

---

#### Task 6.5: Migration Guide (3-4h)
**Estimated:** 3.5h

**Implementation Steps:**
1. Write migration overview
2. Document pre-migration checklist
3. Document step-by-step migration procedure
4. Document config file migration
5. Document user migration
6. Document bot migration
7. Add rollback procedure
8. Add common issues and solutions

**Files to Create/Modify:**
- Create: `documentation/migration-guide.md`

**Acceptance Criteria:**
- [ ] Migration overview
- [ ] Pre-migration checklist
- [ ] Step-by-step migration procedure
- [ ] Config/user/bot migration documented
- [ ] Rollback procedure
- [ ] Common issues and solutions

**Quality Gates:**
- [ ] Guide complete and tested
- [ ] Migration tested on real data
- [ ] Rollback verified

**Dependencies:** Tasks 2.7, 3.6

---

#### Task 6.6: Deployment Guide (4-5h)
**Estimated:** 4.5h

**Implementation Steps:**
1. Write deployment overview
2. Document systemd service setup
3. Create systemd service file
4. Document Docker deployment
5. Create Dockerfile and docker-compose.yaml
6. Document environment variables
7. Document health checks and monitoring
8. Document backup procedures
9. Add production checklist

**Files to Create/Modify:**
- Create: `documentation/deployment-guide.md`
- Create: `deploy/systemd/nuimanbotd.service`
- Create: `deploy/docker/Dockerfile`
- Create: `deploy/docker/docker-compose.yaml`

**Acceptance Criteria:**
- [ ] Deployment overview
- [ ] systemd service file and instructions
- [ ] Docker deployment with Dockerfile and compose file
- [ ] Environment variables documented
- [ ] Health checks documented
- [ ] Backup procedures
- [ ] Production checklist

**Quality Gates:**
- [ ] Guide complete
- [ ] systemd service tested
- [ ] Docker deployment tested
- [ ] Health checks work

**Dependencies:** All phases complete

---

#### Task 6.7: Architecture Documentation (2-3h)
**Estimated:** 2.5h

**Implementation Steps:**
1. Update technical-details.md
2. Add architecture diagrams
3. Document data flow
4. Document component interactions
5. Document security architecture
6. Add performance characteristics

**Files to Create/Modify:**
- Modify: `documentation/technical-details.md`
- Create: Architecture diagrams (Mermaid or images)

**Acceptance Criteria:**
- [ ] Technical details updated
- [ ] Architecture diagrams current
- [ ] Data flow documented
- [ ] Component interactions documented
- [ ] Security architecture documented
- [ ] Performance characteristics listed

**Quality Gates:**
- [ ] Documentation complete
- [ ] Diagrams accurate
- [ ] Security review complete

**Dependencies:** All phases complete

---

**Phase 6 Summary:**
- Total estimated hours: 15-20h
- Critical path: None (all tasks can run in parallel)
- Parallel work: All documentation tasks independent
- Quality gates: All documentation complete, examples tested, deployment verified

---

## Critical Path

**Critical Path Tasks:**
These tasks must complete in sequence and block other work:

```
Phase 1: Core Architecture
  Task 1.1 (Binary Rename) → Task 1.2 (Paths) → Task 1.3 (Hot Reload) → Task 1.7 (Config Restructure)
  [3.5h]                      [4.5h]             [7h]                     [5.5h]

Phase 2: User Profiles
  Task 2.1 (Domain Entity) → Task 2.4 (Repository Interface) → Task 2.5 (File Repository) → Task 2.6 (Service) → Task 2.7 (Migration)
  [4.5h]                      [3.5h]                            [9h]                         [4.5h]              [3.5h]

Phase 3: Bot Management
  Task 3.1 (Domain Entity) → Task 3.2 (Repository Interface) → Task 3.3 (File Repository) → Task 3.4 (Service) → Task 3.5 (Gateway) → Task 3.6 (Migration)
  [4.5h]                      [2.5h]                            [7h]                         [5.5h]              [7h]                [3.5h]

Phase 4: REST API
  Task 4.1 (Router) → Task 4.2 (Auth) → Task 4.3 (Authz) → Task 4.4/4.5/4.6 (Endpoints) → Task 4.7 (Audit) → Task 4.8 (Docs)
  [3.5h]             [4.5h]             [4.5h]             [12.5h combined]                 [2.5h]             [3.5h]

Phase 5: Web Interface
  Task 5.1 (Server) → Task 5.2 (Session) → Task 5.3 (CSRF) → Task 5.4-5.9 (Pages) → Task 5.10 (Styling)
  [4.5h]             [5.5h]               [2.5h]             [30h combined]        [4.5h]

Phase 6: Documentation
  All tasks can run in parallel (no critical path)

Total Critical Path Duration: ~120h minimum
```

**Parallel Work Opportunities:**
- Phase 1: Tasks 1.4, 1.5, 1.6 can run in parallel after 1.2
- Phase 2: Tasks 2.2, 2.3 can run in parallel with 2.1
- Phase 3: Sequential dependencies (no parallelization)
- Phase 4: Tasks 4.4, 4.5, 4.6 can run in parallel after 4.3
- Phase 5: Tasks 5.5-5.9 can partially overlap after 5.4
- Phase 6: All tasks independent (full parallelization)

**Blocking Dependencies:**
- Phase 2 blocked by Phase 1 completion
- Phase 3 blocked by Phase 2 completion
- Phase 4 blocked by Phases 2 and 3 completion
- Phase 5 blocked by Phase 4 completion
- Phase 6 can start during Phase 5

---

## Dependency Graph

**Visual Dependency Map:**

```
Phase 1: Core Architecture (30-40h)
    ├─ Task 1.1 (Binary Rename) [3.5h] ───────┐
    │                                          ├─► Task 1.2 (Paths) [4.5h] ────────────┐
    │                                          │                                        │
    │                                          │                                        ├─► Task 1.3 (Hot Reload) [7h] ─────┐
    │                                          │                                        │                                     │
    │                                          │   Task 1.4 (File Storage) [5.5h] ─────┤                                     ├─► Task 1.7 (Config Restructure) [5.5h]
    │                                          │   Task 1.5 (Audit Log) [4.5h] ────────┤                                     │
    │                                          │   Task 1.6 (STDIO) [2.5h] ─────────────┘                                     │
    │                                          │                                                                              │
    ↓                                          ↓                                                                              ↓
Phase 2: User Profiles (25-35h)
    Task 2.1 (Domain Entity) [4.5h] ───┬─► Task 2.2 (AgentPrefs) [3.5h] ───┐
                                       │                                     ├─► Task 2.4 (Repo Interface) [3.5h] ─┐
                                       └─► Task 2.3 (NotesInfo) [2.5h] ────┘                                        │
                                                                                                                     ├─► Task 2.5 (File Repo) [9h] ─┐
                                           Task 1.4 (File Storage) [from Phase 1] ─────────────────────────────────┘                                │
                                                                                                                                                     ├─► Task 2.6 (Service) [4.5h] ─┐
                                                                                                                                                     │                              │
                                                                                                                                                     │                              ├─► Task 2.7 (Migration) [3.5h]
                                                                                                                                                     │                              │
Phase 3: Bot Management (20-30h)                                                                                                                     │                              │
    Task 3.1 (Domain Entity) [4.5h] ─► Task 3.2 (Repo Interface) [2.5h] ─┬─────────────────────────────────────────────────────────────────────────┘                              │
                                                                           │                                                                                                        │
                                       Task 1.4 (File Storage) [from Phase 1] ─► Task 3.3 (File Repo) [7h] ─► Task 3.4 (Service) [5.5h] ─► Task 3.5 (Gateway) [7h] ─► Task 3.6 (Migration) [3.5h]
                                                                                                                                                                                    │
Phase 4: REST API (20-25h)                                                                                                                                                          │
    Task 4.1 (Router) [3.5h] ─► Task 4.2 (Auth) [4.5h] ─► Task 4.3 (Authz) [4.5h] ──┬─► Task 4.4 (User CRUD) [4.5h] ◄───────────────────────────────────────────────────────────┤
                                                                                      ├─► Task 4.5 (Bot CRUD) [4.5h] ◄────────────────────────────────────────────────────────────┘
                                                                                      ├─► Task 4.6 (Config) [3.5h] ◄─ Task 1.3 (Hot Reload)
                                                                                      │
                                                                                      └─► Task 4.7 (Audit) [2.5h] ◄─ Task 1.5 (Audit Log)
                                                                                           │
                                                                                           └─► Task 4.8 (API Docs) [3.5h]

Phase 5: Web Interface (30-40h)
    Task 5.1 (Server) [4.5h] ─► Task 5.2 (Session) [5.5h] ─► Task 5.3 (CSRF) [2.5h] ──┬─► Task 5.4 (Dashboard) [3.5h]
                                                                                        ├─► Task 5.5 (LLM Config) [5.5h]
                                                                                        ├─► Task 5.6 (Server Config) [4.5h]
                                                                                        ├─► Task 5.7 (User Mgmt) [7h]
                                                                                        ├─► Task 5.8 (Bot Mgmt) [7h]
                                                                                        ├─► Task 5.9 (Logs) [3.5h]
                                                                                        │
                                                                                        └─► Task 5.10 (Styling) [4.5h]

Phase 6: Documentation (15-20h) - All parallel
    Task 6.1 (README) [2.5h]
    Task 6.2 (Admin Guide) [3.5h]
    Task 6.3 (API Docs) [2.5h]
    Task 6.4 (Config Guide) [3.5h]
    Task 6.5 (Migration Guide) [3.5h]
    Task 6.6 (Deployment Guide) [4.5h]
    Task 6.7 (Architecture Docs) [2.5h]
```

**External Dependencies:**
- Go 1.21+ required
- Chi web framework (go get github.com/go-chi/chi/v5)
- Tailwind CSS (npm install, or CDN)
- HTMX (CDN or local)
- No database migrations (file-based storage)
- Existing SQLite database unchanged

---

## Testing Strategy

### Unit Testing

**Approach:**
- Test-first (TDD): Write test before implementation (Red-Green-Refactor)
- One test file per implementation file (`_test.go` suffix)
- Use table-driven tests for multiple scenarios
- Mock external dependencies using interfaces
- Aim for >90% code coverage (>95% for domain layer)

**Test Organization:**
```
internal/domain/[entity]_test.go           # Domain entity tests
internal/usecase/[feature]/service_test.go # Use case tests with mocks
internal/infrastructure/[impl]_test.go     # Infrastructure tests
internal/adapter/[type]/handler_test.go    # Adapter tests
```

**Key Test Scenarios:**
- Domain: Validation rules, business logic, edge cases
- Use Case: Service methods, error handling, authorization
- Infrastructure: File operations, encryption, concurrency
- Adapter: Request parsing, response formatting, middleware

**Mocking Strategy:**
- Use interface-based mocking (no external mocking libraries)
- Mock repositories, external services, file system
- Use dependency injection for testability

---

### Integration Testing

**Approach:**
- Test component interactions across layers
- Use real implementations where possible (not mocks)
- Test with actual file system operations (temp directories)
- Test configuration loading and reloading
- Test API endpoints end-to-end

**Integration Test Suites:**

1. **File Storage Integration**
   - Test: users.json read/write operations
   - Test: bots.json read/write operations
   - Test: Atomic writes and file locking
   - Test: Concurrent access scenarios
   - Test: Index updates and lookups

2. **Configuration Reload Integration**
   - Test: Config file changes detected
   - Test: Configuration validated and loaded
   - Test: Invalid config rejected
   - Test: Active connections unaffected

3. **API Integration**
   - Test: Full CRUD workflows for users
   - Test: Full CRUD workflows for bots
   - Test: Authentication and authorization
   - Test: Audit logging triggered
   - Test: Error responses formatted correctly

4. **Gateway Integration**
   - Test: Bot lifecycle (connect/disconnect)
   - Test: Dynamic bot enable/disable
   - Test: Message routing to correct user

**Test Environment:**
- Use temp directories for file operations
- Use test configuration files
- Clean up after each test
- Parallel test execution safe

---

### End-to-End Testing

**Approach:**
- Full application lifecycle
- Real user workflows
- Production-like environment (no mocks)
- Test all interfaces (API, Web UI, CLI)

**E2E Test Scenarios:**

1. **User Onboarding Workflow**
   - Admin creates user via web interface
   - User profile persisted to files
   - User can authenticate via API
   - User receives welcome notification

2. **Bot Management Workflow**
   - Admin creates Slack bot via web interface
   - Bot configuration saved to bots.json
   - Bot connects to Slack automatically
   - User sends message to bot
   - Bot processes message and responds

3. **Configuration Update Workflow**
   - Admin updates LLM provider via web interface
   - Config saved to config.yaml
   - Admin triggers reload via /refresh command
   - New provider used for next request
   - Active conversations unaffected

4. **Multi-Platform User Workflow**
   - User profile created with Slack and Telegram IDs
   - User sends message from Slack
   - Conversation history saved
   - User sends message from Telegram
   - Conversation history accessible from both platforms

**E2E Test Location:**
- `e2e/admin_workflows_test.go`
- `e2e/user_workflows_test.go`
- `e2e/bot_lifecycle_test.go`
- `e2e/config_reload_test.go`

---

### Performance Testing

**Benchmarks:**

1. **File Operations**
   - User profile read: <5ms p95
   - User profile write: <10ms p95
   - Bot config read: <5ms p95
   - Bot config write: <10ms p95
   - Index lookup: <1ms p95

2. **API Operations**
   - User list endpoint: <100ms p95 (100 users)
   - User create endpoint: <50ms p95
   - Bot list endpoint: <100ms p95 (50 bots)
   - Config reload endpoint: <5s p95

3. **Web Interface**
   - Page load time: <1s p95
   - HTMX update: <500ms p95
   - Dashboard metrics: <200ms p95

**Load Testing:**

1. **Concurrent User Creation**
   - Scenario: 10 concurrent user creates
   - Expected: All succeed, no file corruption
   - Tool: Go test with goroutines

2. **Concurrent Bot Management**
   - Scenario: 5 concurrent bot enables/disables
   - Expected: All succeed, no race conditions
   - Tool: Go test with goroutines

3. **API Throughput**
   - Scenario: 100 requests/second for 1 minute
   - Expected: <1% error rate, <500ms p95 latency
   - Tool: Custom load generator or vegeta

**Benchmark Location:**
- `internal/infrastructure/storage/benchmark_test.go`
- `internal/adapter/api/benchmark_test.go`

---

### Security Testing

**Security Checks:**

1. **Input Validation**
   - [ ] All user inputs validated (email, phone, URLs)
   - [ ] SQL injection prevention (N/A, file-based)
   - [ ] Path traversal prevention in file operations
   - [ ] XSS prevention in web templates (HTML escaping)
   - [ ] Command injection prevention (no shell execution)

2. **Authentication and Authorization**
   - [ ] Bearer token validation
   - [ ] Session management secure (HTTP-only cookies)
   - [ ] CSRF protection on all mutations
   - [ ] RBAC enforced (admin vs user)
   - [ ] API key storage secure

3. **Sensitive Data Handling**
   - [ ] Bot credentials encrypted at rest (AES-256-GCM)
   - [ ] Credentials masked in API responses
   - [ ] Audit logs do not contain credentials
   - [ ] File permissions secure (0600 for sensitive files)

4. **Web Security**
   - [ ] HTTPS support with TLS 1.2+
   - [ ] Content Security Policy headers
   - [ ] Rate limiting on authentication endpoints
   - [ ] Session timeout enforced

**Security Testing Tools:**
- Manual code review for security issues
- Static analysis: `gosec` tool
- Dependency scanning: `go list -m all | nancy`
- Web security: Manual testing with security checklist

---

## Rollout Strategy

### Development Environment

**Phase:** Initial development and testing

**Setup:**
1. Clone repository
2. Install Go 1.21+
3. Run `go mod download`
4. Create `config/config.yaml` from template
5. Run `go build -o bin/nuimanbotd ./cmd/nuimanbotd`
6. Run `./bin/nuimanbotd`

**Criteria:**
- [ ] All unit tests passing (`go test ./...`)
- [ ] All integration tests passing
- [ ] Code coverage >90%
- [ ] Linters passing (`golangci-lint run`)
- [ ] Code review complete

**Rollback:** Not applicable (dev only)

---

### Staging Environment

**Phase:** Pre-production validation

**Deployment Steps:**
1. Build binaries for target platform
2. Deploy binaries to staging server
3. Deploy configuration files
4. Run migration scripts (users, bots, config)
5. Start server with systemd service
6. Verify health check endpoint
7. Run E2E tests against staging

**Validation:**
- [ ] Server starts successfully
- [ ] Health check returns 200 OK
- [ ] E2E tests passing
- [ ] Performance benchmarks met
- [ ] Security scan clean (`gosec`)
- [ ] Web UI accessible and functional
- [ ] API endpoints functional
- [ ] Bot connections working

**Rollback Plan:**
1. Stop systemd service
2. Restore previous binaries
3. Restore previous configuration files
4. Restore previous data files (users.json, bots.json)
5. Restart service
6. Verify rollback successful

**Monitoring:**
- Server logs (`tail -f logs/nuimanbot.log`)
- System metrics (CPU, memory, disk)
- Health check endpoint
- Error rate tracking

---

### Production Environment

**Phase:** Production release

**Deployment Strategy:**
- [ ] Feature flag enabled (if applicable)
- [ ] Blue-green deployment (recommended)
- [ ] Database backup (SQLite file)
- [ ] Configuration backup
- [ ] Gradual rollout (if multiple instances)

**Go-Live Checklist:**
- [ ] All tests passing in staging
- [ ] Documentation complete and reviewed
- [ ] Migration scripts tested on production-like data
- [ ] Monitoring configured (logs, metrics, alerts)
- [ ] Alerts configured (error rate, latency, disk space)
- [ ] Rollback plan tested
- [ ] Backup procedure verified
- [ ] Stakeholder approval obtained
- [ ] Communication plan for users
- [ ] On-call engineer assigned

**Deployment Steps:**
1. Notify users of maintenance window
2. Backup production database and configuration
3. Run migration scripts in dry-run mode
4. Review migration output
5. Deploy new binaries
6. Run migration scripts (live)
7. Start new server
8. Verify health check
9. Monitor for errors (10 minutes)
10. Gradually route traffic to new server
11. Announce completion to users

**Rollback Plan:**
1. Stop new server
2. Restore previous binaries
3. Restore backed-up database and configuration
4. Restart old server
5. Verify health check
6. Route traffic back to old server
7. Investigate and fix issues
8. Announce rollback to users

**Post-Deployment Monitoring (First 24h):**
- Error rate (target: <1%)
- API latency (target: <500ms p95)
- Web UI latency (target: <1s p95)
- File system disk usage
- Memory usage
- CPU usage
- User feedback
- Support ticket volume

---

## Success Metrics

### Development Metrics

**Code Quality:**
- Test coverage: Target >90% (>95% for domain layer)
- Linter warnings: Target 0
- Code review approvals: Target 2+ per PR
- Documentation coverage: 100% of public APIs

**Velocity:**
- Tasks completed per week: Track actual vs estimated
- Actual vs estimated hours: Target within 20%
- Blockers: Track and resolve within 24h
- PR merge time: Target <24h

**Quality:**
- Bug escape rate: Target <5% (bugs found in staging/prod)
- Test failure rate: Target <1%
- Build success rate: Target >99%
- Security vulnerabilities: Target 0 critical/high

---

### Release Metrics

**Adoption:**
- Web UI usage: Track daily active admins
- API usage: Track requests per day
- CLI usage: Track command executions
- User profile completeness: Target >80% fields filled

**Performance:**
- API response time: Target <500ms p95
- Web UI page load: Target <1s p95
- File operations: Target <10ms p95
- Config reload time: Target <5s

**Reliability:**
- Error rate: Target <1% of requests
- Uptime: Target >99.9% (8.76h downtime/year)
- Data corruption incidents: Target 0
- Security incidents: Target 0

---

### User Metrics

**Engagement:**
- Admin logins per week: Track trend
- Users created per week: Track trend
- Bots created per week: Track trend
- Config changes per week: Track trend

**Satisfaction:**
- User feedback: Target >80% positive
- Support tickets: Target <5 per week
- Feature requests: Track and prioritize
- Bug reports: Target <2 per week

**Efficiency:**
- Time to create user: Target <2 minutes
- Time to add bot: Target <3 minutes
- Time to update config: Target <1 minute
- Time to resolve issue: Target <1 hour

---

## Risk Mitigation

### Pre-Development Risks

**Risk: Incomplete requirements**
- **Probability:** Medium
- **Impact:** High
- **Mitigation:** Thorough PRD and spec review before starting
- **Contingency:** Pause development, clarify requirements with stakeholders

**Risk: Technical dependencies unavailable**
- **Probability:** Low
- **Impact:** High
- **Mitigation:** Verify all dependencies available (Chi, Tailwind, HTMX)
- **Contingency:** Find alternative libraries or implement from scratch

**Risk: Team capacity insufficient**
- **Probability:** Medium
- **Impact:** High
- **Mitigation:** Create detailed task breakdown, identify parallelizable work
- **Contingency:** Prioritize critical features, defer nice-to-haves

---

### Development Risks

**Risk: File-based storage performance issues**
- **Probability:** Medium
- **Impact:** Medium
- **Mitigation:** Performance benchmarks in Phase 1, optimization if needed
- **Contingency:** Implement caching layer or migrate to SQL database

**Risk: Concurrency bugs in file operations**
- **Probability:** Medium
- **Impact:** High
- **Mitigation:** Extensive integration tests, file locking, atomic writes
- **Contingency:** Implement stricter locking, add retries

**Risk: Configuration reload disrupts active connections**
- **Probability:** Low
- **Impact:** High
- **Mitigation:** Careful design of hot reload, integration tests
- **Contingency:** Add config reload scheduling, notify users before reload

**Risk: Security vulnerabilities in web interface**
- **Probability:** Medium
- **Impact:** Critical
- **Mitigation:** CSRF protection, input validation, security review
- **Contingency:** Security audit, penetration testing, fix before release

**Risk: Migration script data loss**
- **Probability:** Low
- **Impact:** Critical
- **Mitigation:** Dry-run mode, backups before migration, extensive testing
- **Contingency:** Rollback procedure, restore from backup

---

### Pre-Release Risks

**Risk: Staging environment differs from production**
- **Probability:** Medium
- **Impact:** Medium
- **Mitigation:** Staging mirrors production, test with production-like data
- **Contingency:** Additional testing in production-like environment

**Risk: Documentation incomplete or inaccurate**
- **Probability:** Low
- **Impact:** Medium
- **Mitigation:** Documentation as part of each phase, review before release
- **Contingency:** Documentation sprint before release

**Risk: Performance issues at scale**
- **Probability:** Medium
- **Impact:** High
- **Mitigation:** Performance testing, benchmarks
- **Contingency:** Performance optimization sprint, caching layer

---

### Post-Release Risks

**Risk: User confusion with new interface**
- **Probability:** Medium
- **Impact:** Medium
- **Mitigation:** Clear documentation, user guide, in-app help
- **Contingency:** Onboarding webinar, video tutorials

**Risk: Bugs discovered in production**
- **Probability:** Medium
- **Impact:** Medium-High
- **Mitigation:** Comprehensive testing, gradual rollout
- **Contingency:** Hotfix process, rollback if critical

**Risk: Performance degradation under load**
- **Probability:** Low
- **Impact:** High
- **Mitigation:** Load testing, performance monitoring
- **Contingency:** Scale vertically, optimize hot paths

---

## Timeline Summary

**Total Estimated Duration:** 140-190 hours

**Phase Breakdown:**
- Phase 1 (Core Architecture): 30-40h
- Phase 2 (User Profiles): 25-35h
- Phase 3 (Bot Management): 20-30h
- Phase 4 (REST API): 20-25h
- Phase 5 (Web Interface): 30-40h
- Phase 6 (Documentation): 15-20h

**Key Milestones:**
- Week 4: Phase 1 complete (Core architecture foundation)
- Week 9: Phase 2 complete (User profile management working)
- Week 12: Phase 3 complete (Bot management working)
- Week 15: Phase 4 complete (REST API fully functional)
- Week 20: Phase 5 complete (Web interface fully functional)
- Week 22: Phase 6 complete (Documentation complete)
- Week 23: Release (All testing and deployment complete)

**Contingency:**
- Buffer: 20% (28-38 hours for unknown unknowns)
- Total with buffer: 168-228 hours (21-29 weeks at 8h/week)

---

## Next Steps

1. **Review and approve this plan** (Stakeholder sign-off)
2. **Create detailed task breakdown** (tasks.md with daily breakdown)
3. **Set up feature branch** (`git checkout -b feature/improved-admin-features`)
4. **Create initial directory structure** (specs/, internal/domain/, etc.)
5. **Begin Phase 1, Task 1.1** (Binary rename and project structure)
6. **Daily progress tracking** (Update task status, blockers, actual hours)
7. **Weekly checkpoint** (Review progress, adjust estimates, resolve blockers)
8. **Update STATUS.md** as phases complete

---

## Notes

**Assumptions:**
- Go 1.21+ available
- Development environment has access to required dependencies
- Existing SQLite database schema remains unchanged
- File-based storage sufficient for expected scale (< 1000 users, < 100 bots)
- Single-server deployment (no distributed systems concerns)

**Open Questions:**
- Encryption key management: Where to store encryption key for bot credentials?
  - **Answer:** Environment variable or secure file with restricted permissions
- Session storage: In-memory or Redis?
  - **Answer:** Start with in-memory, add Redis option later if needed
- Web UI framework: Plain HTML+HTMX or consider Templ?
  - **Answer:** Start with html/template + HTMX, migrate to Templ if desired later

**Constraints:**
- Must maintain backward compatibility where possible
- Cannot require database migration (use files instead)
- Must support container deployment (separate config/data/logs)
- Must complete within 6 months (realistic timeline)
- Must follow TDD and Clean Architecture principles
