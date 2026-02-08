# Improved Admin Features - Task Breakdown

**Feature:** Improved Admin Features
**Version:** 1.0
**Created:** 2026-02-08
**Status:** Planning
**Estimated Total Duration:** 140-190 hours (17-24 weeks at 8h/week)

---

## Table of Contents

1. [Progress Summary](#progress-summary)
2. [Phase 0: Planning & Documentation](#phase-0-planning--documentation-10-15-hours)
3. [Phase 1: Core Architecture](#phase-1-core-architecture-30-40-hours)
4. [Phase 2: User Profile Management](#phase-2-user-profile-management-25-35-hours)
5. [Phase 3: Bot Management](#phase-3-bot-management-20-30-hours)
6. [Phase 4: REST API](#phase-4-rest-api-20-25-hours)
7. [Phase 5: Web Admin Interface](#phase-5-web-admin-interface-30-40-hours)
8. [Phase 6: Documentation & Polish](#phase-6-documentation--polish-15-20-hours)
9. [Task Dependencies Graph](#task-dependencies-graph)
10. [Quality Gate Checklist](#quality-gate-checklist)
11. [Common Issues & Solutions](#common-issues--solutions)

---

## Progress Summary

**Overall Progress:** 0/54 tasks complete (0%)

**By Phase:**
- Phase 0 (Planning): 0/9 tasks complete
- Phase 1 (Core Architecture): 0/7 tasks complete
- Phase 2 (User Profiles): 0/7 tasks complete
- Phase 3 (Bot Management): 0/6 tasks complete
- Phase 4 (REST API): 0/8 tasks complete
- Phase 5 (Web Admin): 0/10 tasks complete
- Phase 6 (Documentation): 0/7 tasks complete

**By Priority:**
- P0 (Critical): 0/28 tasks complete
- P1 (High): 0/20 tasks complete
- P2 (Medium): 0/6 tasks complete

**Current Blockers:** None (Phase 0 can start immediately)

---

## Phase 0: Planning & Documentation (10-15 hours)

### Task P0.1: Create Research Document

**ID:** P0.1
**Dependencies:** None
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Gather technical research, API documentation, and code examples for all external dependencies and internal patterns.

**Files to Create:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/research.md`

**Acceptance Criteria:**
- [ ] Slack Socket Mode API documentation summarized
- [ ] Telegram Bot API documentation summarized
- [ ] HTMX usage patterns documented
- [ ] Go file locking patterns researched (flock)
- [ ] Go encryption patterns researched (AES-256)
- [ ] Existing codebase patterns analyzed (gateway, provider, config)
- [ ] File structure conventions documented

**Verification Commands:**
```bash
# Verify file exists and is comprehensive
cat /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/research.md
```

---

### Task P0.2: Create Data Dictionary

**ID:** P0.2
**Dependencies:** None
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Define all data structures, types, and schemas for UserProfile, BotConfig, ServerConfig, and file formats.

**Files to Create:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/data-dictionary.md`

**Acceptance Criteria:**
- [ ] UserProfile struct fully defined with all fields and types
- [ ] BotConfig structs defined (SlackBot, TelegramBot)
- [ ] ServerConfig enhancements defined (PathsConfig)
- [ ] users.json schema documented with indexes
- [ ] bots.json schema documented with indexes
- [ ] User directory structure documented
- [ ] API request/response types defined
- [ ] All enums documented (UserType, BotType, Role)

**Verification Commands:**
```bash
# Verify file exists and covers all entities
cat /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/data-dictionary.md
```

---

### Task P0.3: Create Architecture Document

**ID:** P0.3
**Dependencies:** P0.1, P0.2
**Duration:** 3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Detailed system design with component interactions, data flows, and layer boundaries.

**Files to Create:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/architecture.md`

**Acceptance Criteria:**
- [ ] High-level architecture diagram
- [ ] Component interaction diagrams
- [ ] Data flow diagrams (user request, admin operation)
- [ ] Layer boundaries clearly defined
- [ ] Interface definitions documented
- [ ] Hot reload mechanism documented
- [ ] File locking strategy documented
- [ ] Encryption strategy documented

**Verification Commands:**
```bash
# Verify comprehensive architecture doc
cat /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/architecture.md
```

---

### Task P0.4: Review Existing Codebase

**ID:** P0.4
**Dependencies:** None (can run parallel with P0.1-P0.3)
**Duration:** 2 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Analyze existing codebase to identify files to modify, interfaces to extend, and patterns to follow.

**Files to Review:**
- `internal/config/config.go` - Current config structure
- `internal/domain/user.go` - Current User entity
- `cmd/nuimanbot/main.go` - Current entry point
- `internal/adapter/gateway/*/` - Gateway implementations
- `internal/infrastructure/llm/*/` - LLM provider implementations
- `internal/usecase/user/service.go` - User service patterns

**Acceptance Criteria:**
- [ ] List of files to modify documented
- [ ] Current config structure analyzed
- [ ] Current User entity analyzed
- [ ] Gateway initialization patterns understood
- [ ] Provider patterns understood
- [ ] Existing repository patterns identified
- [ ] Notes added to plan.md

**Verification Commands:**
```bash
# List key files
ls -la internal/config/
ls -la internal/domain/
ls -la cmd/nuimanbot/
```

---

### Task P0.5: Create Implementation Notes Template

**ID:** P0.5
**Dependencies:** None
**Duration:** 0.5 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Set up implementation-notes.md for tracking decisions and gotchas during development.

**Files to Create:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/implementation-notes.md`

**Acceptance Criteria:**
- [ ] Template created with sections for each phase
- [ ] Decision log section added
- [ ] Issues/gotchas section added
- [ ] Lessons learned section added

**Verification Commands:**
```bash
# Verify template exists
cat /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/implementation-notes.md
```

---

### Task P0.6: Define Test Strategy

**ID:** P0.6
**Dependencies:** P0.3
**Duration:** 1.5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Document testing approach for unit, integration, and E2E tests.

**Files to Update:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md` (add testing section)

**Acceptance Criteria:**
- [ ] Unit test patterns defined
- [ ] Integration test patterns defined
- [ ] E2E test scenarios defined
- [ ] Test data strategy documented
- [ ] Mock/stub patterns defined
- [ ] Coverage targets documented (>90% domain/usecase)

**Verification Commands:**
```bash
# Review test strategy in plan.md
grep -A 20 "Testing Strategy" /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md
```

---

### Task P0.7: Create Migration Scripts Plan

**ID:** P0.7
**Dependencies:** P0.2, P0.3
**Duration:** 1 hour
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Plan migration tools for config.yaml, users, and bots.

**Files to Update:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md` (add migration section)

**Acceptance Criteria:**
- [ ] Config migration tool designed
- [ ] User migration tool designed (SQLite → users.json)
- [ ] Bot migration tool designed (config.yaml → bots.json)
- [ ] Validation strategy defined
- [ ] Rollback strategy defined
- [ ] Dry-run mode designed

**Verification Commands:**
```bash
# Review migration plan
grep -A 30 "Migration" /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md
```

---

### Task P0.8: Set Up Development Environment

**ID:** P0.8
**Dependencies:** None
**Duration:** 1 hour
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Ensure development environment has all required tools and dependencies.

**Acceptance Criteria:**
- [ ] Go 1.21+ installed and verified
- [ ] golangci-lint installed
- [ ] SQLite tools available
- [ ] Test database created
- [ ] Sample config files created
- [ ] .gitignore updated for new files

**Verification Commands:**
```bash
# Verify tools
go version
golangci-lint version
sqlite3 --version

# Verify build
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot --help
```

---

### Task P0.9: Create Backup Strategy

**ID:** P0.9
**Dependencies:** None
**Duration:** 0.5 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Document backup and rollback procedures for development and production.

**Files to Update:**
- `/Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md` (add backup section)

**Acceptance Criteria:**
- [ ] Backup procedure documented
- [ ] Rollback procedure documented
- [ ] Git branching strategy defined
- [ ] Testing rollback documented

**Verification Commands:**
```bash
# Review backup strategy
grep -A 15 "Backup" /Users/iggybdda/Code/stainedhead/Golang/NuimanBot/specs/improved-admin-features/plan.md
```

---

## Phase 1: Core Architecture (30-40 hours)

### Task P1.1: Binary Rename and Project Structure

**ID:** P1.1
**Dependencies:** P0.4, P0.8
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Create new server daemon entry point (nuimanbotd) and update CLI tool entry point (nuimanbot).

**Files to Create:**
- `cmd/nuimanbotd/main.go` - New server daemon entry point

**Files to Modify:**
- `cmd/nuimanbot/main.go` - Remove server start logic, keep CLI only

**Acceptance Criteria:**
- [ ] cmd/nuimanbotd/main.go created with basic server initialization
- [ ] cmd/nuimanbot/main.go updated to CLI-only (no server start)
- [ ] Both binaries build successfully
- [ ] nuimanbotd runs as daemon
- [ ] nuimanbot shows CLI commands only
- [ ] Build instructions updated in README

**Implementation Details:**

```go
// cmd/nuimanbotd/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Signal handling
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

    // TODO: Initialize server components
    // - Load configuration
    // - Start gateways
    // - Start web admin interface
    // - Start STDIO listener

    log.Println("nuimanbotd starting...")

    <-sigCh
    log.Println("Shutting down gracefully...")
}
```

**Test Cases:**
- [ ] nuimanbotd binary builds without errors
- [ ] nuimanbotd starts and responds to signals
- [ ] nuimanbot binary builds without errors
- [ ] nuimanbot shows help text with commands

**Verification Commands:**
```bash
# Build both binaries
go build -o bin/nuimanbotd ./cmd/nuimanbotd
go build -o bin/nuimanbot ./cmd/nuimanbot

# Verify they run
./bin/nuimanbotd --help
./bin/nuimanbot --help

# Test daemon starts (will need Ctrl+C to stop)
./bin/nuimanbotd

# Run tests
go test ./cmd/... -v
```

---

### Task P1.2: Path Configuration System

**ID:** P1.2
**Dependencies:** P0.2, P0.3
**Duration:** 5-6 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Add configurable paths (config, data, logs) to ServerConfig with environment variable overrides.

**Files to Create:**
- `internal/domain/server_config.go` - PathsConfig struct
- `internal/domain/server_config_test.go` - Unit tests

**Files to Modify:**
- `internal/config/config.go` - Add Paths field to ServerConfig
- `internal/config/loader.go` - Support environment variable overrides

**Acceptance Criteria:**
- [ ] PathsConfig struct defined with Config, Data, Logs fields
- [ ] Default values: "./config/", "./data/", "./logs/"
- [ ] Environment variable support (NUIMANBOT_SERVER_PATHS_CONFIG, etc.)
- [ ] Validation on startup (paths exist and writable)
- [ ] All file operations use configured paths
- [ ] Unit tests cover all scenarios
- [ ] Integration test validates environment overrides

**Implementation Details:**

```go
// internal/domain/server_config.go
package domain

type PathsConfig struct {
    Config string `yaml:"config" json:"config"`
    Data   string `yaml:"data" json:"data"`
    Logs   string `yaml:"logs" json:"logs"`
}

func (p *PathsConfig) Validate() error {
    // Check paths exist and are writable
    return nil
}

func DefaultPathsConfig() PathsConfig {
    return PathsConfig{
        Config: "./config/",
        Data:   "./data/",
        Logs:   "./logs/",
    }
}
```

**Test Cases:**
- [ ] Default paths loaded correctly
- [ ] Environment variables override config file
- [ ] Invalid paths rejected with clear error
- [ ] Relative and absolute paths both work
- [ ] Non-existent paths detected

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/domain/ -v -run TestPathsConfig

# Test with environment variables
NUIMANBOT_SERVER_PATHS_DATA=/tmp/test-data go test ./internal/config/ -v

# Build and validate
go build -o bin/nuimanbotd ./cmd/nuimanbotd
```

---

### Task P1.3: Hot Reload Infrastructure

**ID:** P1.3
**Dependencies:** P1.2
**Duration:** 8-10 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement configuration hot reload without server restart.

**Files to Create:**
- `internal/infrastructure/config/watcher.go` - File watcher for hot reload
- `internal/infrastructure/config/watcher_test.go` - Unit tests
- `internal/usecase/config/service.go` - ConfigurationService
- `internal/usecase/config/service_test.go` - Unit tests

**Files to Modify:**
- `cmd/nuimanbotd/main.go` - Add /refresh STDIO command handler

**Acceptance Criteria:**
- [ ] File watcher detects changes to config.yaml
- [ ] /refresh command triggers reload
- [ ] Configuration validated before applying
- [ ] Invalid configs rejected with error message
- [ ] Active conversations continue during reload
- [ ] Reload completes within 5 seconds
- [ ] Unit tests cover all reload scenarios
- [ ] Integration test validates full reload cycle

**Implementation Details:**

```go
// internal/infrastructure/config/watcher.go
package config

import (
    "context"
    "log"
    "time"
)

type Watcher struct {
    configPath string
    reloadCh   chan struct{}
}

func NewWatcher(configPath string) *Watcher {
    return &Watcher{
        configPath: configPath,
        reloadCh:   make(chan struct{}, 1),
    }
}

func (w *Watcher) Watch(ctx context.Context) {
    // Watch for file changes
    // Debounce rapid changes
    // Send reload signal
}

func (w *Watcher) TriggerReload() {
    select {
    case w.reloadCh <- struct{}{}:
    default:
        // Already pending
    }
}
```

**Test Cases:**
- [ ] File change triggers reload
- [ ] /refresh command triggers reload
- [ ] Invalid config rejected
- [ ] Rollback on error
- [ ] No disruption to active requests
- [ ] Rapid changes debounced

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/infrastructure/config/ -v
go test ./internal/usecase/config/ -v

# Integration test
go test ./internal/adapter/gateway/ -v -run TestHotReload

# Build and test manually
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd &
echo "/refresh" | nc localhost 5000
```

---

### Task P1.4: File-Based Storage Foundation

**ID:** P1.4
**Dependencies:** P1.2
**Duration:** 6-8 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement atomic file writes, file locking, and JSON serialization for users.json and bots.json.

**Files to Create:**
- `internal/infrastructure/storage/file_storage.go` - Base file storage utilities
- `internal/infrastructure/storage/file_storage_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] Atomic write implementation (temp file + rename)
- [ ] File locking using flock
- [ ] JSON serialization with proper error handling
- [ ] Concurrent access safety tested
- [ ] Index management utilities
- [ ] Backup on write (*.backup files)
- [ ] Unit tests cover all scenarios
- [ ] Benchmark tests for performance

**Implementation Details:**

```go
// internal/infrastructure/storage/file_storage.go
package storage

import (
    "encoding/json"
    "os"
    "path/filepath"
    "syscall"
)

type FileStorage struct {
    basePath string
}

func NewFileStorage(basePath string) *FileStorage {
    return &FileStorage{basePath: basePath}
}

func (fs *FileStorage) AtomicWrite(filename string, data interface{}) error {
    // Marshal to JSON
    // Write to temp file
    // Fsync
    // Rename to target
    // Create backup
    return nil
}

func (fs *FileStorage) LockAndRead(filename string, dest interface{}) error {
    // Open file with flock
    // Read and unmarshal
    // Unlock
    return nil
}
```

**Test Cases:**
- [ ] Atomic write succeeds
- [ ] Concurrent writes don't corrupt
- [ ] Lock timeout handled
- [ ] Backup created correctly
- [ ] JSON marshal/unmarshal errors handled
- [ ] Large files handled efficiently

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/infrastructure/storage/ -v

# Run benchmark tests
go test ./internal/infrastructure/storage/ -bench=. -benchmem

# Build
go build -o bin/nuimanbotd ./cmd/nuimanbotd
```

---

### Task P1.5: Audit Logging Enhancement

**ID:** P1.5
**Dependencies:** P1.2, P1.4
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement audit logging for all admin operations.

**Files to Create:**
- `internal/domain/audit_log.go` - AuditLog entity
- `internal/domain/audit_log_test.go` - Unit tests
- `internal/infrastructure/audit/logger.go` - Audit log writer
- `internal/infrastructure/audit/logger_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] AuditLog entity with timestamp, adminUserID, operation, resource, resourceID, changes
- [ ] Append-only audit log file
- [ ] JSON format for machine parsing
- [ ] Log rotation support (max size, max age)
- [ ] Audit entries written atomically
- [ ] Unit tests cover all scenarios

**Implementation Details:**

```go
// internal/domain/audit_log.go
package domain

import "time"

type AuditLog struct {
    Timestamp   time.Time              `json:"timestamp"`
    AdminUserID string                 `json:"admin_user_id"`
    Operation   string                 `json:"operation"`
    Resource    string                 `json:"resource"`
    ResourceID  string                 `json:"resource_id"`
    Changes     map[string]interface{} `json:"changes"`
}
```

**Test Cases:**
- [ ] Audit entry created correctly
- [ ] Multiple entries written
- [ ] Log rotation works
- [ ] Concurrent writes safe
- [ ] Corrupt entries skipped on read

**Verification Commands:**
```bash
# Run tests
go test ./internal/domain/ -v -run TestAuditLog
go test ./internal/infrastructure/audit/ -v

# Build
go build -o bin/nuimanbotd ./cmd/nuimanbotd
```

---

### Task P1.6: STDIO Simplification

**ID:** P1.6
**Dependencies:** P1.1, P1.3
**Duration:** 2-3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Simplify server STDIO to accept only /refresh and /exit commands.

**Files to Modify:**
- `cmd/nuimanbotd/main.go` - STDIO command handler

**Acceptance Criteria:**
- [ ] STDIO accepts /refresh command
- [ ] STDIO accepts /exit command
- [ ] All other commands rejected with error message
- [ ] /refresh triggers hot reload
- [ ] /exit performs graceful shutdown
- [ ] Help message updated

**Implementation Details:**

```go
// cmd/nuimanbotd/main.go (STDIO handler)
func handleSTDIO(ctx context.Context, reloadFunc func()) {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        switch line {
        case "/refresh":
            log.Println("Reloading configuration...")
            reloadFunc()
        case "/exit":
            log.Println("Exiting...")
            os.Exit(0)
        default:
            log.Printf("Unknown command: %s (only /refresh and /exit supported)\n", line)
        }
    }
}
```

**Test Cases:**
- [ ] /refresh triggers reload
- [ ] /exit shuts down gracefully
- [ ] Unknown commands rejected
- [ ] Empty input ignored

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd &

# Test commands
echo "/refresh" | nc localhost 5000
echo "/exit" | nc localhost 5000
```

---

### Task P1.7: Configuration Restructuring

**ID:** P1.7
**Dependencies:** P1.2, P1.3
**Duration:** 3-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Restructure config.yaml with gateway enable flags and provider inheritance.

**Files to Modify:**
- `internal/config/config.go` - Add Enabled flags, provider sections
- `internal/config/loader.go` - Support inheritance
- `config/config.yaml` - Update template

**Acceptance Criteria:**
- [ ] gateways.cli.enabled flag added
- [ ] gateways.slack.enabled flag added
- [ ] gateways.telegram.enabled flag added
- [ ] llm.anthropic section added
- [ ] llm.openai section added
- [ ] llm.ollama section added
- [ ] llm.bedrock section added
- [ ] Provider instances inherit from type sections
- [ ] Validation ensures required fields present
- [ ] Migration tool created for old configs

**Implementation Details:**

```yaml
# config/config.yaml
server:
  paths:
    config: "./config/"
    data: "./data/"
    logs: "./logs/"
  admin_port: 8080

gateways:
  cli:
    enabled: true
  slack:
    enabled: false
  telegram:
    enabled: false

llm:
  default_model:
    primary: anthropic-main
    secondary: anthropic-fast
    tertiary: openai-gpt4

  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    base_url: "https://api.anthropic.com"

  providers:
    - id: anthropic-main
      type: anthropic
      enabled: true
      model_name: claude-3-5-sonnet-20241022
```

**Test Cases:**
- [ ] Config loads with new structure
- [ ] Environment variables override values
- [ ] Provider inheritance works
- [ ] Invalid configs rejected
- [ ] Migration tool converts old config

**Verification Commands:**
```bash
# Run tests
go test ./internal/config/ -v

# Validate config
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot config validate

# Test migration
./bin/nuimanbot config migrate --input config.yaml.old --output config.yaml
```

---

## Phase 2: User Profile Management (25-35 hours)

### Task P2.1: UserProfile Domain Entity

**ID:** P2.1
**Dependencies:** P0.2, P1.4
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Create UserProfile domain entity with comprehensive fields.

**Files to Create:**
- `internal/domain/user_profile.go` - UserProfile entity
- `internal/domain/user_profile_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] UserProfile struct with all fields from data dictionary
- [ ] Validation methods (ValidateEmail, ValidateTimezone, etc.)
- [ ] Constructor with required fields
- [ ] Update methods with validation
- [ ] Unit tests cover all validation scenarios
- [ ] Code coverage >90%

**Implementation Details:**

```go
// internal/domain/user_profile.go
package domain

import (
    "errors"
    "time"
)

type UserProfile struct {
    UserID            string                 `json:"userID"`
    Username          string                 `json:"username"`
    Moniker           string                 `json:"moniker,omitempty"`
    FirstName         string                 `json:"firstName"`
    LastName          string                 `json:"lastName"`
    NickName          string                 `json:"nickName,omitempty"`
    PrimaryEmail      string                 `json:"primaryEmail"`
    BackupEmail       string                 `json:"backupEmail,omitempty"`
    MobilePhone       string                 `json:"mobilePhone,omitempty"`
    PrimaryLanguage   string                 `json:"primaryLanguage"`
    SecondaryLanguage string                 `json:"secondaryLanguage,omitempty"`
    Timezone          string                 `json:"timezone"`
    PrimaryLocation   string                 `json:"primaryLocation,omitempty"`
    JobRole           string                 `json:"jobRole,omitempty"`
    UserType          string                 `json:"userType"`
    Role              string                 `json:"role"`
    PlatformIDs       PlatformIDs            `json:"platformIDs"`
    AgentPreferences  AgentPreferences       `json:"agentPreferences,omitempty"`
    Enabled           bool                   `json:"enabled"`
    DataDirectory     string                 `json:"dataDirectory"`
    CreatedAt         time.Time              `json:"createdAt"`
    UpdatedAt         time.Time              `json:"updatedAt"`
}

type PlatformIDs struct {
    CLI      string `json:"cli,omitempty"`
    Slack    string `json:"slack,omitempty"`
    Telegram string `json:"telegram,omitempty"`
}

type AgentPreferences struct {
    CommunicationStyle string `json:"communicationStyle,omitempty"`
    Verbosity          string `json:"verbosity,omitempty"`
    ResponseFormat     string `json:"responseFormat,omitempty"`
}

func NewUserProfile(username, email, firstName, lastName, role string) (*UserProfile, error) {
    // Validation and initialization
    return &UserProfile{}, nil
}

func (up *UserProfile) Validate() error {
    // Field validation
    return nil
}
```

**Test Cases:**
- [ ] NewUserProfile creates valid profile
- [ ] Validation rejects invalid email
- [ ] Validation rejects invalid timezone
- [ ] Update methods maintain consistency
- [ ] Required fields enforced
- [ ] Optional fields handled correctly

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/domain/ -v -run TestUserProfile

# Check coverage
go test ./internal/domain/ -cover -coverprofile=coverage.out
go tool cover -func=coverage.out | grep user_profile
```

---

### Task P2.2: UserProfile Repository Interface

**ID:** P2.2
**Dependencies:** P2.1
**Duration:** 2-3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Define UserProfileRepository interface in use case layer.

**Files to Create:**
- `internal/usecase/profile/repository.go` - Repository interface

**Acceptance Criteria:**
- [ ] Repository interface defined with CRUD methods
- [ ] Lookup methods defined (ByID, ByUsername, ByEmail, ByPlatformID)
- [ ] List methods with filtering
- [ ] Interface documented

**Implementation Details:**

```go
// internal/usecase/profile/repository.go
package profile

import (
    "context"
    "github.com/yourusername/nuimanbot/internal/domain"
)

type Repository interface {
    Create(ctx context.Context, profile *domain.UserProfile) error
    GetByID(ctx context.Context, userID string) (*domain.UserProfile, error)
    GetByUsername(ctx context.Context, username string) (*domain.UserProfile, error)
    GetByEmail(ctx context.Context, email string) (*domain.UserProfile, error)
    GetByPlatformID(ctx context.Context, platform, id string) (*domain.UserProfile, error)
    Update(ctx context.Context, profile *domain.UserProfile) error
    Delete(ctx context.Context, userID string) error
    List(ctx context.Context, filter *Filter) ([]*domain.UserProfile, error)
}

type Filter struct {
    Role    string
    Enabled *bool
    Limit   int
    Offset  int
}
```

**Verification Commands:**
```bash
# Build to ensure interface compiles
go build ./internal/usecase/profile/
```

---

### Task P2.3: File-Based UserProfile Repository

**ID:** P2.3
**Dependencies:** P2.2, P1.4
**Duration:** 8-10 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement file-based UserProfileRepository with users.json storage.

**Files to Create:**
- `internal/infrastructure/storage/file_user_profile_repository.go` - Implementation
- `internal/infrastructure/storage/file_user_profile_repository_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] All Repository interface methods implemented
- [ ] users.json with indexes (byUsername, byEmail, byPlatform)
- [ ] Atomic writes using FileStorage
- [ ] Index updates on create/update/delete
- [ ] User directories created automatically
- [ ] Concurrent access safe
- [ ] Unit tests cover all methods
- [ ] Integration tests validate file operations

**Implementation Details:**

```go
// internal/infrastructure/storage/file_user_profile_repository.go
package storage

import (
    "context"
    "encoding/json"
    "sync"

    "github.com/yourusername/nuimanbot/internal/domain"
    "github.com/yourusername/nuimanbot/internal/usecase/profile"
)

type UserRegistry struct {
    Version     string                 `json:"version"`
    LastUpdated string                 `json:"lastUpdated"`
    Users       []*domain.UserProfile  `json:"users"`
    Indexes     UserIndexes            `json:"indexes"`
}

type UserIndexes struct {
    ByUsername map[string]string            `json:"byUsername"`
    ByEmail    map[string]string            `json:"byEmail"`
    ByPlatform map[string]map[string]string `json:"byPlatform"`
}

type FileUserProfileRepository struct {
    storage    *FileStorage
    registryPath string
    mu         sync.RWMutex
    cache      *UserRegistry
}

func NewFileUserProfileRepository(storage *FileStorage) *FileUserProfileRepository {
    return &FileUserProfileRepository{
        storage:      storage,
        registryPath: "users.json",
    }
}

func (r *FileUserProfileRepository) Create(ctx context.Context, profile *domain.UserProfile) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Load registry
    // Add user
    // Update indexes
    // Save registry
    // Create user directory
    return nil
}
```

**Test Cases:**
- [ ] Create user adds to registry
- [ ] GetByID retrieves user
- [ ] GetByUsername uses index
- [ ] GetByEmail uses index
- [ ] GetByPlatformID uses index
- [ ] Update modifies user
- [ ] Delete removes user and indexes
- [ ] List with filters works
- [ ] Concurrent creates don't corrupt

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/infrastructure/storage/ -v -run TestFileUserProfileRepository

# Run integration tests
go test ./internal/infrastructure/storage/ -v -tags=integration

# Check coverage
go test ./internal/infrastructure/storage/ -cover
```

---

### Task P2.4: UserProfile Service

**ID:** P2.4
**Dependencies:** P2.3
**Duration:** 5-6 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement UserProfileService use case with business logic.

**Files to Create:**
- `internal/usecase/profile/service.go` - UserProfileService
- `internal/usecase/profile/service_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] Service with Create, Get, Update, Delete, List methods
- [ ] Business logic validation (unique username, unique email, unique platform IDs)
- [ ] Partial update support
- [ ] Audit logging on create/update/delete
- [ ] Unit tests with mocked repository
- [ ] Code coverage >90%

**Implementation Details:**

```go
// internal/usecase/profile/service.go
package profile

import (
    "context"
    "errors"

    "github.com/yourusername/nuimanbot/internal/domain"
)

type Service struct {
    repo   Repository
    logger AuditLogger
}

func NewService(repo Repository, logger AuditLogger) *Service {
    return &Service{repo: repo, logger: logger}
}

func (s *Service) Create(ctx context.Context, profile *domain.UserProfile) error {
    // Validate
    // Check uniqueness
    // Create
    // Audit log
    return nil
}

func (s *Service) Update(ctx context.Context, userID string, updates map[string]interface{}) error {
    // Load existing
    // Apply updates
    // Validate
    // Save
    // Audit log
    return nil
}
```

**Test Cases:**
- [ ] Create validates and saves
- [ ] Duplicate username rejected
- [ ] Duplicate email rejected
- [ ] Duplicate platform ID rejected
- [ ] Update applies partial changes
- [ ] Delete removes user
- [ ] Audit log entries created

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/usecase/profile/ -v

# Check coverage
go test ./internal/usecase/profile/ -cover
```

---

### Task P2.5: Platform Routing Service

**ID:** P2.5
**Dependencies:** P2.4
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement service to route platform IDs (Slack, Telegram) to user profiles.

**Files to Create:**
- `internal/usecase/routing/service.go` - PlatformRoutingService
- `internal/usecase/routing/service_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] MapPlatformToUser(platform, id) returns UserProfile
- [ ] Caching for performance
- [ ] Cache invalidation on profile updates
- [ ] Unit tests with mocked repository

**Implementation Details:**

```go
// internal/usecase/routing/service.go
package routing

import (
    "context"
    "sync"
    "time"

    "github.com/yourusername/nuimanbot/internal/domain"
    "github.com/yourusername/nuimanbot/internal/usecase/profile"
)

type Service struct {
    profileRepo profile.Repository
    cache       map[string]*cacheEntry
    cacheTTL    time.Duration
    mu          sync.RWMutex
}

type cacheEntry struct {
    profile   *domain.UserProfile
    expiresAt time.Time
}

func NewService(profileRepo profile.Repository, cacheTTL time.Duration) *Service {
    return &Service{
        profileRepo: profileRepo,
        cache:       make(map[string]*cacheEntry),
        cacheTTL:    cacheTTL,
    }
}

func (s *Service) MapPlatformToUser(ctx context.Context, platform, id string) (*domain.UserProfile, error) {
    // Check cache
    // If miss, lookup from repo
    // Cache result
    return nil, nil
}
```

**Test Cases:**
- [ ] Slack ID maps to user
- [ ] Telegram ID maps to user
- [ ] CLI username maps to user
- [ ] Cache hit returns cached value
- [ ] Cache miss loads from repo
- [ ] Cache expires after TTL

**Verification Commands:**
```bash
# Run tests
go test ./internal/usecase/routing/ -v

# Check coverage
go test ./internal/usecase/routing/ -cover
```

---

### Task P2.6: CLI Admin User Commands

**ID:** P2.6
**Dependencies:** P2.4
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement CLI commands for user management.

**Files to Create:**
- `internal/adapter/cli/admin_user_command.go` - CLI command handlers
- `internal/adapter/cli/admin_user_command_test.go` - Unit tests

**Files to Modify:**
- `cmd/nuimanbot/main.go` - Register admin user commands

**Acceptance Criteria:**
- [ ] nuimanbot admin user create
- [ ] nuimanbot admin user list
- [ ] nuimanbot admin user view <id>
- [ ] nuimanbot admin user update <id> --field value
- [ ] nuimanbot admin user delete <id>
- [ ] nuimanbot admin user import --file <path>
- [ ] nuimanbot admin user export --file <path>
- [ ] Output formats: table, JSON, CSV
- [ ] Unit tests cover all commands

**Implementation Details:**

```go
// internal/adapter/cli/admin_user_command.go
package cli

import (
    "github.com/spf13/cobra"
    "github.com/yourusername/nuimanbot/internal/usecase/profile"
)

type AdminUserCommand struct {
    service *profile.Service
}

func NewAdminUserCommand(service *profile.Service) *AdminUserCommand {
    return &AdminUserCommand{service: service}
}

func (c *AdminUserCommand) CreateCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "create",
        Short: "Create a new user",
        RunE:  c.create,
    }
    cmd.Flags().String("username", "", "Username (required)")
    cmd.Flags().String("email", "", "Email (required)")
    cmd.Flags().String("first-name", "", "First name (required)")
    cmd.Flags().String("last-name", "", "Last name (required)")
    cmd.Flags().String("role", "user", "Role (admin|user)")
    return cmd
}

func (c *AdminUserCommand) create(cmd *cobra.Command, args []string) error {
    // Parse flags
    // Call service.Create
    // Output result
    return nil
}
```

**Test Cases:**
- [ ] Create command parses flags correctly
- [ ] List command outputs table format
- [ ] List command outputs JSON format
- [ ] View command shows user details
- [ ] Update command modifies user
- [ ] Delete command removes user
- [ ] Import command loads from file
- [ ] Export command writes to file

**Verification Commands:**
```bash
# Run tests
go test ./internal/adapter/cli/ -v -run TestAdminUserCommand

# Build and test manually
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot admin user create --username test --email test@example.com --first-name Test --last-name User
./bin/nuimanbot admin user list
./bin/nuimanbot admin user view <user-id>
```

---

### Task P2.7: Agent Personalization Integration

**ID:** P2.7
**Dependencies:** P2.5
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Integrate agent preferences loading into gateway message handling.

**Files to Modify:**
- `internal/adapter/gateway/cli/cli.go` - Load preferences on message
- `internal/adapter/gateway/slack/slack.go` - Load preferences on message
- `internal/adapter/gateway/telegram/telegram.go` - Load preferences on message

**Acceptance Criteria:**
- [ ] Gateway loads user profile on message receipt
- [ ] Agent preferences applied to request context
- [ ] Fallback to defaults if no preferences set
- [ ] Integration tests validate preference application

**Verification Commands:**
```bash
# Run integration tests
go test ./internal/adapter/gateway/ -v -tags=integration

# Build and test manually
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
```

---

## Phase 3: Bot Management (20-30 hours)

### Task P3.1: BotConfig Domain Entity

**ID:** P3.1
**Dependencies:** P0.2, P1.4
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Create BotConfig domain entities for Slack and Telegram bots.

**Files to Create:**
- `internal/domain/bot_config.go` - SlackBot and TelegramBot entities
- `internal/domain/bot_config_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] SlackBot struct with all required fields
- [ ] TelegramBot struct with all required fields
- [ ] BotType enum (public, private)
- [ ] Validation methods
- [ ] Constructor with required fields
- [ ] Unit tests cover all scenarios

**Implementation Details:**

```go
// internal/domain/bot_config.go
package domain

import "time"

type BotType string

const (
    BotTypePublic  BotType = "public"
    BotTypePrivate BotType = "private"
)

type SlackBot struct {
    BotID              string    `json:"botID"`
    BotName            string    `json:"botName"`
    BotType            BotType   `json:"botType"`
    OwnerUserID        string    `json:"ownerUserID,omitempty"`
    SlackBotToken      string    `json:"slackBotToken"`
    SlackAppToken      string    `json:"slackAppToken"`
    SlackSigningSecret string    `json:"slackSigningSecret"`
    SlackTeamID        string    `json:"slackTeamID"`
    SlackBotUserID     string    `json:"slackBotUserID"`
    Enabled            bool      `json:"enabled"`
    AllowedUserIDs     []string  `json:"allowedUserIDs,omitempty"`
    Metadata           map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt          time.Time `json:"createdAt"`
    UpdatedAt          time.Time `json:"updatedAt"`
}

type TelegramBot struct {
    BotID              string    `json:"botID"`
    BotName            string    `json:"botName"`
    BotType            BotType   `json:"botType"`
    OwnerUserID        string    `json:"ownerUserID,omitempty"`
    TelegramBotToken   string    `json:"telegramBotToken"`
    TelegramBotUsername string   `json:"telegramBotUsername"`
    TelegramBotID      string    `json:"telegramBotID"`
    Enabled            bool      `json:"enabled"`
    AllowedUserIDs     []string  `json:"allowedUserIDs,omitempty"`
    AllowedChatIDs     []string  `json:"allowedChatIDs,omitempty"`
    Metadata           map[string]interface{} `json:"metadata,omitempty"`
    CreatedAt          time.Time `json:"createdAt"`
    UpdatedAt          time.Time `json:"updatedAt"`
}

func NewSlackBot(name string, botType BotType, tokens SlackTokens) (*SlackBot, error) {
    // Validation and initialization
    return &SlackBot{}, nil
}

func (sb *SlackBot) Validate() error {
    // Field validation
    return nil
}
```

**Test Cases:**
- [ ] NewSlackBot creates valid bot
- [ ] NewTelegramBot creates valid bot
- [ ] Validation rejects invalid tokens
- [ ] Public bot validation enforces allowedUserIDs
- [ ] Private bot validation enforces ownerUserID

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/domain/ -v -run TestBotConfig

# Check coverage
go test ./internal/domain/ -cover
```

---

### Task P3.2: BotConfig Repository Interface

**ID:** P3.2
**Dependencies:** P3.1
**Duration:** 2-3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Define BotConfigRepository interface in use case layer.

**Files to Create:**
- `internal/usecase/botmgmt/repository.go` - Repository interface

**Acceptance Criteria:**
- [ ] Repository interface with CRUD methods for SlackBot
- [ ] Repository interface with CRUD methods for TelegramBot
- [ ] Lookup methods (ByID, ByName, ByOwner, ByPlatformBotID)
- [ ] List methods with filtering

**Implementation Details:**

```go
// internal/usecase/botmgmt/repository.go
package botmgmt

import (
    "context"
    "github.com/yourusername/nuimanbot/internal/domain"
)

type Repository interface {
    // Slack bots
    CreateSlackBot(ctx context.Context, bot *domain.SlackBot) error
    GetSlackBotByID(ctx context.Context, botID string) (*domain.SlackBot, error)
    GetSlackBotByName(ctx context.Context, name string) (*domain.SlackBot, error)
    GetSlackBotByBotUserID(ctx context.Context, botUserID string) (*domain.SlackBot, error)
    UpdateSlackBot(ctx context.Context, bot *domain.SlackBot) error
    DeleteSlackBot(ctx context.Context, botID string) error
    ListSlackBots(ctx context.Context, filter *BotFilter) ([]*domain.SlackBot, error)

    // Telegram bots
    CreateTelegramBot(ctx context.Context, bot *domain.TelegramBot) error
    GetTelegramBotByID(ctx context.Context, botID string) (*domain.TelegramBot, error)
    GetTelegramBotByName(ctx context.Context, name string) (*domain.TelegramBot, error)
    GetTelegramBotByBotID(ctx context.Context, botID string) (*domain.TelegramBot, error)
    UpdateTelegramBot(ctx context.Context, bot *domain.TelegramBot) error
    DeleteTelegramBot(ctx context.Context, botID string) error
    ListTelegramBots(ctx context.Context, filter *BotFilter) ([]*domain.TelegramBot, error)
}

type BotFilter struct {
    BotType *domain.BotType
    OwnerUserID string
    Enabled *bool
    Limit   int
    Offset  int
}
```

**Verification Commands:**
```bash
# Build to ensure interface compiles
go build ./internal/usecase/botmgmt/
```

---

### Task P3.3: File-Based BotConfig Repository

**ID:** P3.3
**Dependencies:** P3.2, P1.4, P1.7
**Duration:** 8-10 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement file-based BotConfigRepository with bots.json storage and encryption.

**Files to Create:**
- `internal/infrastructure/storage/file_bot_config_repository.go` - Implementation
- `internal/infrastructure/storage/file_bot_config_repository_test.go` - Unit tests
- `internal/infrastructure/security/encryption.go` - Encryption service
- `internal/infrastructure/security/encryption_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] All Repository interface methods implemented
- [ ] bots.json with indexes
- [ ] Bot tokens encrypted at rest (AES-256)
- [ ] Atomic writes using FileStorage
- [ ] Index updates on create/update/delete
- [ ] Concurrent access safe
- [ ] Unit tests cover all methods
- [ ] Encryption/decryption tested

**Implementation Details:**

```go
// internal/infrastructure/storage/file_bot_config_repository.go
package storage

import (
    "context"
    "sync"

    "github.com/yourusername/nuimanbot/internal/domain"
    "github.com/yourusername/nuimanbot/internal/infrastructure/security"
    "github.com/yourusername/nuimanbot/internal/usecase/botmgmt"
)

type BotRegistry struct {
    Version      string               `json:"version"`
    LastUpdated  string               `json:"lastUpdated"`
    SlackBots    []*domain.SlackBot   `json:"slackBots"`
    TelegramBots []*domain.TelegramBot `json:"telegramBots"`
    Indexes      BotIndexes           `json:"indexes"`
}

type BotIndexes struct {
    SlackByName       map[string]string   `json:"slackByName"`
    SlackByOwner      map[string][]string `json:"slackByOwner"`
    SlackByBotUserID  map[string]string   `json:"slackByBotUserID"`
    TelegramByName    map[string]string   `json:"telegramByName"`
    TelegramByOwner   map[string][]string `json:"telegramByOwner"`
    TelegramByBotID   map[string]string   `json:"telegramByBotID"`
}

type FileBotConfigRepository struct {
    storage      *FileStorage
    encryption   *security.EncryptionService
    registryPath string
    mu           sync.RWMutex
    cache        *BotRegistry
}

func NewFileBotConfigRepository(storage *FileStorage, encryption *security.EncryptionService) *FileBotConfigRepository {
    return &FileBotConfigRepository{
        storage:      storage,
        encryption:   encryption,
        registryPath: "bots.json",
    }
}

func (r *FileBotConfigRepository) CreateSlackBot(ctx context.Context, bot *domain.SlackBot) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Load registry
    // Encrypt tokens
    // Add bot
    // Update indexes
    // Save registry
    return nil
}

// internal/infrastructure/security/encryption.go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "io"
)

type EncryptionService struct {
    key []byte
}

func NewEncryptionService(key []byte) (*EncryptionService, error) {
    if len(key) != 32 {
        return nil, errors.New("key must be 32 bytes for AES-256")
    }
    return &EncryptionService{key: key}, nil
}

func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
    // AES-256-GCM encryption
    return "", nil
}

func (s *EncryptionService) Decrypt(ciphertext string) (string, error) {
    // AES-256-GCM decryption
    return "", nil
}
```

**Test Cases:**
- [ ] Create Slack bot saves encrypted
- [ ] Get Slack bot decrypts tokens
- [ ] Create Telegram bot saves encrypted
- [ ] Get Telegram bot decrypts tokens
- [ ] Indexes updated correctly
- [ ] Concurrent creates safe
- [ ] Encryption key validation

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/infrastructure/storage/ -v -run TestFileBotConfigRepository
go test ./internal/infrastructure/security/ -v

# Check coverage
go test ./internal/infrastructure/storage/ -cover
go test ./internal/infrastructure/security/ -cover
```

---

### Task P3.4: BotManagement Service

**ID:** P3.4
**Dependencies:** P3.3
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement BotManagementService use case with business logic.

**Files to Create:**
- `internal/usecase/botmgmt/service.go` - BotManagementService
- `internal/usecase/botmgmt/service_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] Service with Create, Get, Update, Delete, List methods for both platforms
- [ ] Business logic validation (unique name, unique platform bot ID)
- [ ] Access control (public bot allowed users, private bot owner)
- [ ] Partial update support
- [ ] Audit logging on create/update/delete
- [ ] Unit tests with mocked repository

**Implementation Details:**

```go
// internal/usecase/botmgmt/service.go
package botmgmt

import (
    "context"
    "errors"

    "github.com/yourusername/nuimanbot/internal/domain"
)

type Service struct {
    repo   Repository
    logger AuditLogger
}

func NewService(repo Repository, logger AuditLogger) *Service {
    return &Service{repo: repo, logger: logger}
}

func (s *Service) CreateSlackBot(ctx context.Context, bot *domain.SlackBot) error {
    // Validate
    // Check uniqueness
    // Create
    // Audit log
    return nil
}

func (s *Service) UpdateSlackBot(ctx context.Context, botID string, updates map[string]interface{}) error {
    // Load existing
    // Apply updates
    // Validate
    // Save
    // Audit log
    return nil
}

func (s *Service) CanUserAccessBot(ctx context.Context, userID string, bot *domain.SlackBot) bool {
    // Check if user can access this bot
    // Private bot: only owner
    // Public bot: check allowedUserIDs
    return false
}
```

**Test Cases:**
- [ ] Create Slack bot validates and saves
- [ ] Create Telegram bot validates and saves
- [ ] Duplicate name rejected
- [ ] Update applies partial changes
- [ ] Delete removes bot
- [ ] Access control enforced
- [ ] Audit log entries created

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/usecase/botmgmt/ -v

# Check coverage
go test ./internal/usecase/botmgmt/ -cover
```

---

### Task P3.5: Gateway Bot Loading Integration

**ID:** P3.5
**Dependencies:** P3.4
**Duration:** 5-6 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Update gateways to load bots from bots.json instead of config.yaml.

**Files to Modify:**
- `internal/adapter/gateway/slack/slack.go` - Load from repository
- `internal/adapter/gateway/telegram/telegram.go` - Load from repository

**Acceptance Criteria:**
- [ ] Slack gateway loads bots from repository
- [ ] Telegram gateway loads bots from repository
- [ ] Only enabled bots loaded
- [ ] Dynamic reload on /refresh
- [ ] Connection monitoring
- [ ] Integration tests validate bot loading

**Implementation Details:**

```go
// internal/adapter/gateway/slack/slack.go (modifications)
func (g *Gateway) Initialize(ctx context.Context, botRepo botmgmt.Repository) error {
    // Load enabled Slack bots from repository
    bots, err := botRepo.ListSlackBots(ctx, &botmgmt.BotFilter{
        Enabled: ptr.Bool(true),
    })
    if err != nil {
        return err
    }

    // Connect each bot
    for _, bot := range bots {
        if err := g.connectBot(ctx, bot); err != nil {
            log.Printf("Failed to connect bot %s: %v", bot.BotName, err)
        }
    }

    return nil
}

func (g *Gateway) Reload(ctx context.Context) error {
    // Disconnect bots no longer enabled
    // Connect newly enabled bots
    return nil
}
```

**Test Cases:**
- [ ] Gateway loads enabled bots
- [ ] Gateway skips disabled bots
- [ ] Gateway reloads on /refresh
- [ ] Bot connections monitored
- [ ] Failed connections handled gracefully

**Verification Commands:**
```bash
# Run integration tests
go test ./internal/adapter/gateway/ -v -tags=integration

# Build and test manually
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
```

---

### Task P3.6: CLI Admin Bot Commands

**ID:** P3.6
**Dependencies:** P3.4
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement CLI commands for bot management.

**Files to Create:**
- `internal/adapter/cli/admin_bot_command.go` - CLI command handlers
- `internal/adapter/cli/admin_bot_command_test.go` - Unit tests

**Files to Modify:**
- `cmd/nuimanbot/main.go` - Register admin bot commands

**Acceptance Criteria:**
- [ ] nuimanbot admin bot slack create
- [ ] nuimanbot admin bot slack list
- [ ] nuimanbot admin bot slack view <id>
- [ ] nuimanbot admin bot slack update <id>
- [ ] nuimanbot admin bot slack delete <id>
- [ ] nuimanbot admin bot slack enable <id>
- [ ] nuimanbot admin bot slack disable <id>
- [ ] Similar commands for telegram
- [ ] Unit tests cover all commands

**Verification Commands:**
```bash
# Run tests
go test ./internal/adapter/cli/ -v -run TestAdminBotCommand

# Build and test
go build -o bin/nuimanbot ./cmd/nuimanbot
./bin/nuimanbot admin bot slack create --name "Test Bot" --type private --bot-token "xoxb-..." --app-token "xapp-..."
./bin/nuimanbot admin bot slack list
```

---

## Phase 4: REST API (20-25 hours)

### Task P4.1: Authentication Middleware

**ID:** P4.1
**Dependencies:** P2.4
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement Bearer token authentication middleware for REST API.

**Files to Create:**
- `internal/adapter/rest/middleware/auth.go` - Auth middleware
- `internal/adapter/rest/middleware/auth_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] Extract Bearer token from Authorization header
- [ ] Validate token against users.json (API key field)
- [ ] Set user context for downstream handlers
- [ ] Return 401 Unauthorized if no token
- [ ] Return 403 Forbidden if invalid token
- [ ] Unit tests cover all scenarios

**Implementation Details:**

```go
// internal/adapter/rest/middleware/auth.go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/yourusername/nuimanbot/internal/usecase/profile"
)

type contextKey string

const UserContextKey contextKey = "user"

type AuthMiddleware struct {
    profileService *profile.Service
}

func NewAuthMiddleware(profileService *profile.Service) *AuthMiddleware {
    return &AuthMiddleware{profileService: profileService}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract token from Authorization: Bearer <token>
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
            return
        }

        token := parts[1]

        // Validate token (lookup user by API key)
        user, err := m.profileService.GetByAPIKey(r.Context(), token)
        if err != nil {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        // Set user in context
        ctx := context.WithValue(r.Context(), UserContextKey, user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Test Cases:**
- [ ] Valid token allows access
- [ ] Missing token returns 401
- [ ] Invalid token returns 403
- [ ] User added to context
- [ ] Malformed header rejected

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/adapter/rest/middleware/ -v -run TestAuthMiddleware
```

---

### Task P4.2: RBAC Middleware

**ID:** P4.2
**Dependencies:** P4.1
**Duration:** 2-3 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement role-based access control middleware.

**Files to Create:**
- `internal/adapter/rest/middleware/rbac.go` - RBAC middleware
- `internal/adapter/rest/middleware/rbac_test.go` - Unit tests

**Acceptance Criteria:**
- [ ] RequireRole middleware checks user role
- [ ] Admin role has full access
- [ ] User role restricted to own resources
- [ ] Service role has read-only access
- [ ] Return 403 Forbidden if insufficient permissions
- [ ] Unit tests cover all scenarios

**Implementation Details:**

```go
// internal/adapter/rest/middleware/rbac.go
package middleware

import (
    "net/http"

    "github.com/yourusername/nuimanbot/internal/domain"
)

func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := r.Context().Value(UserContextKey).(*domain.UserProfile)

            // Check if user has required role
            hasRole := false
            for _, role := range roles {
                if user.Role == role {
                    hasRole = true
                    break
                }
            }

            if !hasRole {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func RequireAdmin(next http.Handler) http.Handler {
    return RequireRole("admin")(next)
}
```

**Test Cases:**
- [ ] Admin role passes RequireAdmin
- [ ] User role blocked by RequireAdmin
- [ ] Multiple roles supported
- [ ] User can access own resources

**Verification Commands:**
```bash
# Run unit tests
go test ./internal/adapter/rest/middleware/ -v -run TestRBACMiddleware
```

---

### Task P4.3: UserProfile REST Handlers

**ID:** P4.3
**Dependencies:** P4.2
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement REST API handlers for user profile CRUD operations.

**Files to Create:**
- `internal/adapter/rest/profile_handler.go` - HTTP handlers
- `internal/adapter/rest/profile_handler_test.go` - Integration tests

**Acceptance Criteria:**
- [ ] GET /api/v1/admin/profiles - List profiles
- [ ] GET /api/v1/admin/profiles/:id - Get profile
- [ ] POST /api/v1/admin/profiles - Create profile
- [ ] PUT /api/v1/admin/profiles/:id - Update profile (partial)
- [ ] DELETE /api/v1/admin/profiles/:id - Delete profile
- [ ] Pagination on list endpoint
- [ ] Response includes updated_fields on PUT
- [ ] Integration tests cover all endpoints

**Implementation Details:**

```go
// internal/adapter/rest/profile_handler.go
package rest

import (
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/yourusername/nuimanbot/internal/usecase/profile"
)

type ProfileHandler struct {
    service *profile.Service
}

func NewProfileHandler(service *profile.Service) *ProfileHandler {
    return &ProfileHandler{service: service}
}

func (h *ProfileHandler) RegisterRoutes(r *mux.Router, authMw, adminMw func(http.Handler) http.Handler) {
    r.Handle("/api/v1/admin/profiles", adminMw(authMw(http.HandlerFunc(h.List)))).Methods("GET")
    r.Handle("/api/v1/admin/profiles/{id}", adminMw(authMw(http.HandlerFunc(h.Get)))).Methods("GET")
    r.Handle("/api/v1/admin/profiles", adminMw(authMw(http.HandlerFunc(h.Create)))).Methods("POST")
    r.Handle("/api/v1/admin/profiles/{id}", adminMw(authMw(http.HandlerFunc(h.Update)))).Methods("PUT")
    r.Handle("/api/v1/admin/profiles/{id}", adminMw(authMw(http.HandlerFunc(h.Delete)))).Methods("DELETE")
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
    // Parse query params (limit, offset, filters)
    // Call service.List
    // Return JSON response
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
    // Parse request body
    // Call service.Create
    // Return 201 Created
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
    // Parse request body (partial updates)
    // Call service.Update
    // Return updated profile with updated_fields
}
```

**Test Cases:**
- [ ] List returns profiles
- [ ] List paginates correctly
- [ ] Get returns single profile
- [ ] Create creates profile
- [ ] Update updates only specified fields
- [ ] Delete removes profile
- [ ] Auth required for all endpoints
- [ ] Admin role required

**Verification Commands:**
```bash
# Run integration tests
go test ./internal/adapter/rest/ -v -run TestProfileHandler

# Test manually with curl
curl -X GET http://localhost:8080/api/v1/admin/profiles \
  -H "Authorization: Bearer <token>"
```

---

### Task P4.4: BotConfig REST Handlers

**ID:** P4.4
**Dependencies:** P4.2, P3.4
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement REST API handlers for bot configuration CRUD operations.

**Files to Create:**
- `internal/adapter/rest/bot_handler.go` - HTTP handlers
- `internal/adapter/rest/bot_handler_test.go` - Integration tests

**Acceptance Criteria:**
- [ ] GET /api/v1/admin/bots/slack - List Slack bots
- [ ] GET /api/v1/admin/bots/slack/:id - Get Slack bot
- [ ] POST /api/v1/admin/bots/slack - Create Slack bot
- [ ] PUT /api/v1/admin/bots/slack/:id - Update Slack bot
- [ ] DELETE /api/v1/admin/bots/slack/:id - Delete Slack bot
- [ ] Similar endpoints for Telegram
- [ ] Tokens never returned in responses (masked)
- [ ] Integration tests cover all endpoints

**Verification Commands:**
```bash
# Run integration tests
go test ./internal/adapter/rest/ -v -run TestBotHandler

# Test manually
curl -X POST http://localhost:8080/api/v1/admin/bots/slack \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"botName":"Test","botType":"private","slackBotToken":"xoxb-..."}'
```

---

### Task P4.5: Config REST Handlers

**ID:** P4.5
**Dependencies:** P4.2, P1.3
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement REST API handlers for configuration management.

**Files to Create:**
- `internal/adapter/rest/config_handler.go` - HTTP handlers
- `internal/adapter/rest/config_handler_test.go` - Integration tests

**Acceptance Criteria:**
- [ ] GET /api/v1/admin/config - Get current config
- [ ] PUT /api/v1/admin/config - Update config
- [ ] POST /api/v1/admin/config/reload - Trigger reload
- [ ] POST /api/v1/admin/config/validate - Validate config
- [ ] Integration tests cover all endpoints

**Verification Commands:**
```bash
# Run tests
go test ./internal/adapter/rest/ -v -run TestConfigHandler

# Test reload
curl -X POST http://localhost:8080/api/v1/admin/config/reload \
  -H "Authorization: Bearer <token>"
```

---

### Task P4.6: Server Control REST Handlers

**ID:** P4.6
**Dependencies:** P4.2
**Duration:** 2-3 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement REST API handlers for server status and control.

**Files to Create:**
- `internal/adapter/rest/server_handler.go` - HTTP handlers
- `internal/adapter/rest/server_handler_test.go` - Integration tests

**Acceptance Criteria:**
- [ ] GET /api/v1/admin/status - Server status
- [ ] GET /api/v1/admin/metrics - Server metrics
- [ ] GET /api/v1/admin/logs - Recent logs
- [ ] Integration tests cover all endpoints

**Verification Commands:**
```bash
# Run tests
go test ./internal/adapter/rest/ -v -run TestServerHandler

# Test status
curl -X GET http://localhost:8080/api/v1/admin/status \
  -H "Authorization: Bearer <token>"
```

---

### Task P4.7: API Integration Testing

**ID:** P4.7
**Dependencies:** P4.3, P4.4, P4.5, P4.6
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Comprehensive integration tests for all REST API endpoints.

**Files to Create:**
- `internal/adapter/rest/api_integration_test.go` - E2E API tests

**Acceptance Criteria:**
- [ ] E2E test: Create user → Get user → Update user → Delete user
- [ ] E2E test: Create bot → Enable/disable → Delete bot
- [ ] E2E test: Update config → Reload → Verify changes
- [ ] E2E test: Authentication and authorization flows
- [ ] All tests pass

**Verification Commands:**
```bash
# Run integration tests
go test ./internal/adapter/rest/ -v -tags=integration

# Run with race detector
go test ./internal/adapter/rest/ -v -race -tags=integration
```

---

### Task P4.8: API Documentation (OpenAPI)

**ID:** P4.8
**Dependencies:** P4.3, P4.4, P4.5, P4.6
**Duration:** 2-3 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Generate OpenAPI specification for REST API.

**Files to Create:**
- `documentation/api-reference.md` - OpenAPI spec in markdown
- `api/openapi.yaml` - OpenAPI 3.0 spec

**Acceptance Criteria:**
- [ ] All endpoints documented
- [ ] Request/response schemas defined
- [ ] Authentication documented
- [ ] Example requests provided
- [ ] Swagger UI accessible

**Verification Commands:**
```bash
# Validate OpenAPI spec
npx @apidevtools/swagger-cli validate api/openapi.yaml

# View in Swagger UI
# (Start swagger-ui-watcher or similar)
```

---

## Phase 5: Web Admin Interface (30-40 hours)

### Task P5.1: HTML Template Base

**ID:** P5.1
**Dependencies:** P4.1, P4.2
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Create base HTML templates with navigation and layout.

**Files to Create:**
- `internal/adapter/web/templates/base.html` - Base layout
- `internal/adapter/web/templates/nav.html` - Navigation menu
- `internal/adapter/web/static/styles.css` - Tailwind CSS styles

**Acceptance Criteria:**
- [ ] Base template with header, nav, content, footer
- [ ] Responsive navigation menu
- [ ] Tailwind CSS integrated
- [ ] HTMX and Alpine.js included
- [ ] Authentication-aware navigation

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/
```

---

### Task P5.2: Dashboard Page

**ID:** P5.2
**Dependencies:** P5.1, P4.6
**Duration:** 4-5 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement dashboard with server status and metrics.

**Files to Create:**
- `internal/adapter/web/dashboard_handler.go` - Dashboard handler
- `internal/adapter/web/templates/dashboard.html` - Dashboard template

**Acceptance Criteria:**
- [ ] Server status displayed (uptime, version, memory)
- [ ] Active connections shown (Slack, Telegram, CLI)
- [ ] Recent activity log (last 50 entries)
- [ ] Quick stats cards
- [ ] Reload config button
- [ ] Auto-refresh every 30 seconds

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/dashboard
```

---

### Task P5.3: User Management Pages

**ID:** P5.3
**Dependencies:** P5.1, P4.3
**Duration:** 6-8 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement user management UI (list, create, edit, delete).

**Files to Create:**
- `internal/adapter/web/user_handler.go` - User management handlers
- `internal/adapter/web/templates/users.html` - User list
- `internal/adapter/web/templates/user_form.html` - Create/edit form

**Acceptance Criteria:**
- [ ] User list with search and filters
- [ ] Create user form
- [ ] Edit user form with tabs (Basic, Profile, Access)
- [ ] Delete user with confirmation
- [ ] Platform ID management
- [ ] Agent preferences editor
- [ ] HTMX for dynamic updates

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/users
```

---

### Task P5.4: Bot Management Pages

**ID:** P5.4
**Dependencies:** P5.1, P4.4
**Duration:** 6-8 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement bot management UI (list, create, edit, delete).

**Files to Create:**
- `internal/adapter/web/bot_handler.go` - Bot management handlers
- `internal/adapter/web/templates/bots.html` - Bot list (Slack/Telegram tabs)
- `internal/adapter/web/templates/bot_form.html` - Create/edit form

**Acceptance Criteria:**
- [ ] Bot list with Slack and Telegram tabs
- [ ] Create bot form
- [ ] Edit bot form
- [ ] Delete bot with confirmation
- [ ] Public bot user access management
- [ ] Enable/disable toggle
- [ ] Test connection button
- [ ] Bot status indicators

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/bots
```

---

### Task P5.5: LLM Configuration Page

**ID:** P5.5
**Dependencies:** P5.1, P4.5
**Duration:** 4-5 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement LLM configuration management UI.

**Files to Create:**
- `internal/adapter/web/llm_handler.go` - LLM config handlers
- `internal/adapter/web/templates/llm.html` - LLM config page

**Acceptance Criteria:**
- [ ] Default model selection dropdowns
- [ ] Provider configuration forms
- [ ] Model instance list (add, edit, delete)
- [ ] Enable/disable toggles
- [ ] Test connection buttons
- [ ] Save changes button

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/llm
```

---

### Task P5.6: Server Configuration Page

**ID:** P5.6
**Dependencies:** P5.1, P4.5
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement server configuration management UI.

**Files to Create:**
- `internal/adapter/web/config_handler.go` - Server config handlers
- `internal/adapter/web/templates/config.html` - Server config page

**Acceptance Criteria:**
- [ ] Paths configuration form
- [ ] Logging configuration form
- [ ] Gateway enable/disable toggles
- [ ] Admin port configuration
- [ ] Save changes button
- [ ] Reload trigger

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/config
```

---

### Task P5.7: Activity Log Viewer

**ID:** P5.7
**Dependencies:** P5.1, P4.6
**Duration:** 3-4 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Implement activity log viewer with filtering.

**Files to Create:**
- `internal/adapter/web/log_handler.go` - Log viewer handlers
- `internal/adapter/web/templates/logs.html` - Log viewer page

**Acceptance Criteria:**
- [ ] Log table with timestamp, user, action columns
- [ ] Filter by log level
- [ ] Filter by time range
- [ ] Search by text
- [ ] Pagination
- [ ] Export to CSV/JSON

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/logs
```

---

### Task P5.8: Session Management

**ID:** P5.8
**Dependencies:** P5.1
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Implement session-based authentication for web interface.

**Files to Create:**
- `internal/adapter/web/auth.go` - Session authentication
- `internal/adapter/web/templates/login.html` - Login page

**Acceptance Criteria:**
- [ ] Login page with username/password
- [ ] Session creation on successful login
- [ ] Session validation middleware
- [ ] Session timeout (24 hours)
- [ ] Logout functionality
- [ ] CSRF protection

**Verification Commands:**
```bash
# Build and test
go build -o bin/nuimanbotd ./cmd/nuimanbotd
./bin/nuimanbotd
# Open http://localhost:8080/admin/login
```

---

### Task P5.9: Web UI Integration Testing

**ID:** P5.9
**Dependencies:** P5.2, P5.3, P5.4, P5.5, P5.6, P5.7, P5.8
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
E2E tests for web admin interface.

**Files to Create:**
- `internal/adapter/web/web_integration_test.go` - E2E web tests

**Acceptance Criteria:**
- [ ] E2E test: Login → Dashboard → Logout
- [ ] E2E test: Create user via web form
- [ ] E2E test: Create bot via web form
- [ ] E2E test: Update config via web form
- [ ] E2E test: View logs
- [ ] All tests pass

**Verification Commands:**
```bash
# Run E2E tests
go test ./internal/adapter/web/ -v -tags=integration

# Manual testing checklist
# - Login works
# - All pages load
# - Forms submit correctly
# - HTMX updates work
# - CSRF protection works
```

---

### Task P5.10: Responsive Design & Accessibility

**ID:** P5.10
**Dependencies:** P5.2, P5.3, P5.4, P5.5, P5.6, P5.7
**Duration:** 3-4 hours
**Priority:** P2 (Medium)
**Status:** ⬜ Not Started

**Description:**
Ensure web interface is responsive and accessible.

**Files to Modify:**
- `internal/adapter/web/static/styles.css` - Responsive styles
- All template files - Accessibility improvements

**Acceptance Criteria:**
- [ ] Mobile responsive (320px width)
- [ ] Tablet responsive (768px width)
- [ ] Desktop responsive (1024px+ width)
- [ ] Keyboard navigation works
- [ ] Screen reader compatible
- [ ] WCAG 2.1 AA compliance (goal)

**Verification Commands:**
```bash
# Manual testing
# - Resize browser to various widths
# - Navigate with keyboard only
# - Test with screen reader
# - Run accessibility audit (Lighthouse)
```

---

## Phase 6: Documentation & Polish (15-20 hours)

### Task P6.1: Admin Guide

**ID:** P6.1
**Dependencies:** P5.9
**Duration:** 4-5 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Write comprehensive admin guide for web interface and CLI.

**Files to Create:**
- `documentation/admin-guide.md` - Admin guide

**Acceptance Criteria:**
- [ ] Installation and setup instructions
- [ ] Web interface user guide (all pages)
- [ ] CLI command reference (all commands)
- [ ] Common tasks documented
- [ ] Troubleshooting section
- [ ] Screenshots included

**Verification Commands:**
```bash
# Review guide
cat documentation/admin-guide.md
```

---

### Task P6.2: Migration Guide

**ID:** P6.2
**Dependencies:** P1.7, P2.3, P3.3
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Write migration guide for upgrading from old architecture.

**Files to Create:**
- `documentation/migration-guide.md` - Migration guide

**Acceptance Criteria:**
- [ ] Pre-migration checklist
- [ ] Step-by-step migration instructions
- [ ] Config migration documented
- [ ] User migration documented
- [ ] Bot migration documented
- [ ] Rollback procedures documented
- [ ] Common issues and solutions

**Verification Commands:**
```bash
# Review guide
cat documentation/migration-guide.md

# Test migration on sample data
# (Follow guide step-by-step)
```

---

### Task P6.3: Configuration Reference

**ID:** P6.3
**Dependencies:** P1.7
**Duration:** 2-3 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Document all configuration options in config.yaml.

**Files to Create:**
- `documentation/configuration-reference.md` - Config reference

**Acceptance Criteria:**
- [ ] All config sections documented
- [ ] All fields with types and defaults
- [ ] Environment variable overrides listed
- [ ] Examples for common scenarios
- [ ] Validation rules documented

**Verification Commands:**
```bash
# Review reference
cat documentation/configuration-reference.md
```

---

### Task P6.4: Update Product Documentation

**ID:** P6.4
**Dependencies:** P6.1, P6.2, P6.3
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Update product documentation (README, product-summary, technical-details).

**Files to Modify:**
- `README.md` - Project overview
- `documentation/product-summary.md` - Executive overview
- `documentation/product-details.md` - Detailed requirements
- `documentation/technical-details.md` - Architecture and design

**Acceptance Criteria:**
- [ ] README updated with new features
- [ ] Quick start guide updated
- [ ] Architecture diagrams updated
- [ ] API documentation linked
- [ ] All docs consistent and current

**Verification Commands:**
```bash
# Review updated docs
cat README.md
cat documentation/product-summary.md
cat documentation/technical-details.md
```

---

### Task P6.5: Quality Assurance Pass

**ID:** P6.5
**Dependencies:** All previous tasks
**Duration:** 3-4 hours
**Priority:** P0 (Critical)
**Status:** ⬜ Not Started

**Description:**
Full QA pass across all features.

**Acceptance Criteria:**
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All E2E tests pass
- [ ] Code coverage >80%
- [ ] golangci-lint passes
- [ ] Manual testing checklist complete
- [ ] Performance benchmarks met
- [ ] Security review complete

**Verification Commands:**
```bash
# Run all tests
go test ./... -v

# Check coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# Run linter
golangci-lint run

# Build both binaries
go build -o bin/nuimanbotd ./cmd/nuimanbotd
go build -o bin/nuimanbot ./cmd/nuimanbot

# Verify they run
./bin/nuimanbotd --help
./bin/nuimanbot --help
```

---

### Task P6.6: Create Demo Environment

**ID:** P6.6
**Dependencies:** P6.5
**Duration:** 2-3 hours
**Priority:** P2 (Medium)
**Status:** ⬜ Not Started

**Description:**
Set up demo environment with sample data.

**Files to Create:**
- `demo/` - Demo environment directory
- `demo/docker-compose.yml` - Docker compose for demo
- `demo/sample-data/` - Sample users and bots

**Acceptance Criteria:**
- [ ] Docker compose starts demo environment
- [ ] Sample users preloaded
- [ ] Sample bots configured
- [ ] Web interface accessible
- [ ] Demo guide written

**Verification Commands:**
```bash
# Start demo
cd demo
docker-compose up -d

# Test access
curl http://localhost:8080/admin/
```

---

### Task P6.7: Release Preparation

**ID:** P6.7
**Dependencies:** P6.5, P6.6
**Duration:** 1-2 hours
**Priority:** P1 (High)
**Status:** ⬜ Not Started

**Description:**
Prepare release artifacts and changelog.

**Files to Create:**
- `CHANGELOG.md` - Release changelog
- `RELEASE.md` - Release notes

**Acceptance Criteria:**
- [ ] CHANGELOG.md updated with all changes
- [ ] Release notes written
- [ ] Breaking changes documented
- [ ] Migration path documented
- [ ] Version numbers updated

**Verification Commands:**
```bash
# Review release files
cat CHANGELOG.md
cat RELEASE.md
```

---

## Task Dependencies Graph

**Visual representation:**

```
Phase 0 (Planning):
P0.1 ──┐
P0.2 ──┼─→ P0.3
P0.4 ──┘

Phase 1 (Core):
P0.4, P0.8 ──→ P1.1
P0.2, P0.3 ──→ P1.2 ──┬─→ P1.3
                      ├─→ P1.4 ──→ P1.5
                      └─→ P1.7
P1.1, P1.3 ──→ P1.6

Phase 2 (Users):
P0.2, P1.4 ──→ P2.1 ──→ P2.2 ──→ P2.3 ──┬─→ P2.4 ──┬─→ P2.5 ──→ P2.7
                                        └─→ P2.6 ──┘

Phase 3 (Bots):
P0.2, P1.4 ──→ P3.1 ──→ P3.2 ──→ P3.3 ──→ P3.4 ──┬─→ P3.5
                                                  └─→ P3.6

Phase 4 (API):
P2.4 ──→ P4.1 ──→ P4.2 ──┬─→ P4.3 ──┐
                        ├─→ P4.4 ──┼─→ P4.7 ──→ P4.8
P1.3 ──→ P4.5 ─────────┤          │
                        └─→ P4.6 ──┘

Phase 5 (Web):
P4.1, P4.2 ──→ P5.1 ──┬─→ P5.2 ──┐
P4.3 ────────────────┼─→ P5.3 ──┤
P4.4 ────────────────┼─→ P5.4 ──┼─→ P5.9 ──→ P5.10
P4.5 ────────────────┼─→ P5.5 ──┤
P4.5 ────────────────┼─→ P5.6 ──┤
P4.6 ────────────────┼─→ P5.7 ──┘
                     └─→ P5.8 ──┘

Phase 6 (Docs):
P5.9 ──→ P6.1 ──┐
P1.7, P2.3, P3.3 ──→ P6.2 ──┼─→ P6.4 ──→ P6.5 ──┬─→ P6.6
P1.7 ──→ P6.3 ──┘                              └─→ P6.7
```

**Critical Path:**
```
P0.2 → P0.3 → P1.2 → P1.4 → P2.1 → P2.2 → P2.3 → P2.4 → P4.1 → P4.2 → P4.3 → P4.7 → P5.1 → P5.3 → P5.9 → P6.1 → P6.4 → P6.5

Estimated Critical Path Duration: ~85-110 hours
```

---

## Quality Gate Checklist

**Before marking ANY task complete, ALL of these must pass:**

### Code Quality
- [ ] `go fmt ./...` - Code formatted
- [ ] `go mod tidy` - Dependencies clean
- [ ] `go vet ./...` - No suspicious constructs
- [ ] `golangci-lint run` - Linter passes
- [ ] `go test ./...` - All tests pass
- [ ] `go test ./... -race` - No race conditions
- [ ] Code coverage >90% for domain/usecase layers

### Build & Run
- [ ] `go build -o bin/nuimanbotd ./cmd/nuimanbotd` - Server builds
- [ ] `go build -o bin/nuimanbot ./cmd/nuimanbot` - CLI builds
- [ ] `./bin/nuimanbotd --help` - Server runs
- [ ] `./bin/nuimanbot --help` - CLI runs

### Documentation
- [ ] All exported types have doc comments
- [ ] README updated if needed
- [ ] Implementation notes updated
- [ ] Breaking changes documented

### Testing
- [ ] Unit tests written (TDD - Red-Green-Refactor)
- [ ] Integration tests written where applicable
- [ ] Test data cleanup in teardown
- [ ] Error cases covered

**Quick validation script:**
```bash
#!/bin/bash
set -e

echo "Running quality gates..."

echo "1. Formatting..."
go fmt ./...

echo "2. Tidying dependencies..."
go mod tidy

echo "3. Vetting..."
go vet ./...

echo "4. Linting..."
golangci-lint run

echo "5. Testing..."
go test ./... -v

echo "6. Race detection..."
go test ./... -race

echo "7. Coverage check..."
go test ./... -cover -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

echo "8. Building server..."
go build -o bin/nuimanbotd ./cmd/nuimanbotd

echo "9. Building CLI..."
go build -o bin/nuimanbot ./cmd/nuimanbot

echo "10. Verifying binaries..."
./bin/nuimanbotd --help
./bin/nuimanbot --help

echo "✅ All quality gates passed!"
```

---

## Common Issues & Solutions

### Issue 1: File Locking Errors

**Symptom:** `resource temporarily unavailable` or `file is locked`

**Solution:**
- Ensure all file operations use proper locking (flock)
- Add retry logic with exponential backoff
- Check for deadlocks in nested lock acquisition
- Use `defer unlock()` to ensure locks are released

**Code Example:**
```go
func (fs *FileStorage) LockAndRead(filename string, dest interface{}) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    // Acquire lock with timeout
    if err := flock(file, 5*time.Second); err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer funlock(file)

    // Read and unmarshal
    return json.NewDecoder(file).Decode(dest)
}
```

### Issue 2: Hot Reload Disrupts Connections

**Symptom:** Active bot connections drop during /refresh

**Solution:**
- Use graceful reload pattern
- Keep old config in memory until new config validated
- Only swap configs after successful validation
- Reconnect bots incrementally, not all at once

**Code Example:**
```go
func (s *Server) Reload(ctx context.Context) error {
    // Load new config
    newConfig, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }

    // Validate before applying
    if err := newConfig.Validate(); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    // Swap config atomically
    s.mu.Lock()
    oldConfig := s.config
    s.config = newConfig
    s.mu.Unlock()

    // Reload components (keep old connections until new ones ready)
    if err := s.reloadComponents(ctx, oldConfig, newConfig); err != nil {
        // Rollback
        s.mu.Lock()
        s.config = oldConfig
        s.mu.Unlock()
        return err
    }

    return nil
}
```

### Issue 3: JSON Index Corruption

**Symptom:** Index doesn't match actual data in users.json

**Solution:**
- Always update indexes in same transaction as data
- Rebuild indexes on load (don't trust stored indexes)
- Add validation check on load
- Use atomic writes (temp file + rename)

**Code Example:**
```go
func (r *FileUserProfileRepository) rebuildIndexes(registry *UserRegistry) {
    registry.Indexes = UserIndexes{
        ByUsername: make(map[string]string),
        ByEmail:    make(map[string]string),
        ByPlatform: make(map[string]map[string]string),
    }

    for _, user := range registry.Users {
        registry.Indexes.ByUsername[user.Username] = user.UserID
        registry.Indexes.ByEmail[user.PrimaryEmail] = user.UserID

        if user.PlatformIDs.Slack != "" {
            if registry.Indexes.ByPlatform["slack"] == nil {
                registry.Indexes.ByPlatform["slack"] = make(map[string]string)
            }
            registry.Indexes.ByPlatform["slack"][user.PlatformIDs.Slack] = user.UserID
        }
        // ... telegram, cli
    }
}
```

### Issue 4: Encrypted Data Lost After Restart

**Symptom:** Bot tokens can't be decrypted after server restart

**Solution:**
- Ensure encryption key stored in environment variable
- Document key management in admin guide
- Add key validation on startup
- Provide key rotation tool

**Code Example:**
```go
func loadEncryptionKey() ([]byte, error) {
    keyStr := os.Getenv("NUIMANBOT_ENCRYPTION_KEY")
    if keyStr == "" {
        return nil, errors.New("NUIMANBOT_ENCRYPTION_KEY not set")
    }

    key, err := base64.StdEncoding.DecodeString(keyStr)
    if err != nil {
        return nil, fmt.Errorf("invalid encryption key format: %w", err)
    }

    if len(key) != 32 {
        return nil, errors.New("encryption key must be 32 bytes (AES-256)")
    }

    return key, nil
}
```

### Issue 5: Tests Fail on CI But Pass Locally

**Symptom:** `go test` passes locally but fails in CI

**Common Causes:**
- File paths hardcoded (use `t.TempDir()`)
- Race conditions (run with `-race`)
- Test isolation issues (cleanup in teardown)
- Environment variables missing
- File permissions issues

**Solution:**
```go
func TestFileStorage(t *testing.T) {
    // Use temp directory
    tempDir := t.TempDir()

    storage := NewFileStorage(tempDir)

    // Test operations
    // ...

    // Cleanup automatic via t.TempDir()
}
```

---

## Notes

**Assumptions:**
- Go 1.21+ available
- golangci-lint installed
- SQLite available
- Development on macOS/Linux (file locking APIs)
- 8 hours/week commitment

**Risks:**
- File locking complexity on Windows (mitigation: extensive testing)
- Hot reload edge cases (mitigation: thorough integration tests)
- Encryption key management (mitigation: clear documentation)
- Large users.json performance (mitigation: benchmark tests, migration path to DB)

**Open Questions:**
- Should we support webhook notifications for config changes?
- Should we add audit log export to external systems (Splunk, ELK)?
- Should we support multi-tenancy in future?
- Should we add CLI REPL for admin commands?

---

**Last Updated:** 2026-02-08
**Next Review:** After Phase 1 completion
