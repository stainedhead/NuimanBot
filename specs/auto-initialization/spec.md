# Auto-Initialization Feature Specification

**Feature**: Application Self-Initialization
**Priority**: P1 (High)
**Status**: Planned
**Target Version**: Next Release

---

## Executive Summary

NuimanBot should automatically initialize its storage infrastructure on startup, eliminating the need for manual shell scripts. The application should be truly self-sufficient, creating directories, default users, and exposing comprehensive health metrics without external tooling.

## Problem Statement

**Current State:**
- Manual shell scripts required for initial setup (`init-file-storage.sh`)
- External monitoring script needed to check storage health (`monitor-file-storage.sh`)
- Deployment requires pre-initialization steps before app can start
- Not container-friendly (requires external init scripts)

**Impact:**
- Poor developer experience (extra setup steps)
- Deployment friction (manual pre-deployment tasks)
- Not cloud-native (doesn't follow 12-factor app principles)
- Requires documentation of "pre-flight" steps

**User Pain Points:**
1. "I just want to run the binary and have it work"
2. "Why do I need shell scripts for a Go application?"
3. "How do I deploy this in Kubernetes without external init containers?"

## Goals

### Must Have (P0)
- ✅ Application auto-creates required directories on startup
- ✅ Default admin user created automatically on first run
- ✅ Enhanced `/health` endpoint with storage metrics
- ✅ Zero-config startup experience (just run the binary)

### Should Have (P1)
- ✅ Graceful handling of permission errors
- ✅ Idempotent initialization (safe to run multiple times)
- ✅ Structured logging of initialization steps
- ✅ Health endpoint includes disk usage and storage metrics

### Nice to Have (P2)
- ⚪ CLI command for manual re-initialization: `nuimanbot init`
- ⚪ Initialization configuration in `config.yaml`
- ⚪ Pre-flight check command: `nuimanbot preflight`

## Non-Goals

- ❌ Database migrations (SQLite already handles schema)
- ❌ Backup/restore functionality (remain as operational scripts)
- ❌ Scheduled tasks or cron-like functionality
- ❌ External monitoring integrations (use `/metrics` endpoint)

## User Stories

### Story 1: First-Time User
**As a** new user downloading NuimanBot
**I want to** run the binary and start using it immediately
**So that** I don't need to read complex setup documentation

**Acceptance Criteria:**
- Download binary → Run `./nuimanbot` → Application starts successfully
- No "directory not found" errors
- Default admin user available for initial login
- Health check passes immediately

### Story 2: DevOps Engineer
**As a** DevOps engineer deploying to Kubernetes
**I want** NuimanBot to self-initialize on startup
**So that** I don't need init containers or pre-deployment jobs

**Acceptance Criteria:**
- Kubernetes Pod starts without init containers
- Volume mount empty → app creates directories
- Liveness/readiness probes work immediately
- Logs clearly show initialization steps

### Story 3: Monitoring Integration
**As an** SRE monitoring production systems
**I want** storage metrics exposed via `/health` endpoint
**So that** I can query them programmatically without shell scripts

**Acceptance Criteria:**
- `/health` returns JSON with storage metrics
- Includes: disk usage, user count, file counts
- Compatible with standard monitoring tools (Prometheus, Datadog)
- No need for external `monitor-file-storage.sh` script

## Technical Design

### Architecture

**New Component: Storage Initializer**

```
internal/infrastructure/storage/
├── initializer.go          # Main initialization logic
├── initializer_test.go     # Unit tests
├── default_data.go         # Default admin user data
└── health_metrics.go       # Storage metrics collector
```

**Integration Points:**
- `cmd/nuimanbot/main.go` - Call initializer before starting services
- `internal/infrastructure/health/checks.go` - Add storage metrics
- `internal/adapter/web/server.go` - Enhanced `/health` endpoint

### Implementation Plan

#### Phase 1: Auto-Initialization (3-5 hours)

**Files to Create:**
- `internal/infrastructure/storage/initializer.go`
- `internal/infrastructure/storage/initializer_test.go`
- `internal/infrastructure/storage/default_data.go`

**Key Functions:**

```go
// Initialize creates required directory structure and default data
func Initialize(baseDir string) error {
    // 1. Create directory structure
    // 2. Set permissions
    // 3. Create default admin user if none exists
    // 4. Create audit log file
    // 5. Return error or nil
}

// EnsureDirectories creates required directories
func EnsureDirectories(baseDir string) error {
    dirs := []string{"users", "system"}
    for _, dir := range dirs {
        path := filepath.Join(baseDir, dir)
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", path, err)
        }
    }
    return nil
}

// CreateDefaultAdmin creates admin user if none exists
func CreateDefaultAdmin(baseDir string) error {
    adminPath := filepath.Join(baseDir, "users", "admin", "profile.json")
    if _, err := os.Stat(adminPath); err == nil {
        return nil // Admin already exists
    }
    // Create admin user
    return createUserProfile("admin", adminData)
}
```

**Main.go Integration:**

```go
func main() {
    // Load config
    cfg := loadConfig()

    // Initialize storage (NEW)
    if err := storage.Initialize(cfg.DataDir); err != nil {
        log.Fatal("Failed to initialize storage", "error", err)
    }

    // Continue with existing startup...
}
```

#### Phase 2: Enhanced Health Endpoint (2-3 hours)

**Files to Modify:**
- `internal/infrastructure/health/checks.go`
- `internal/infrastructure/health/server.go`

**Enhanced Health Response:**

```json
{
  "status": "healthy",
  "timestamp": "2026-02-16T12:00:00Z",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "storage": {
    "status": "healthy",
    "data_directory": "/data",
    "total_size_mb": 150,
    "disk_available_gb": 50,
    "disk_used_percent": 15,
    "writable": true
  },
  "users": {
    "total_count": 25,
    "active_count": 10
  },
  "data": {
    "conversations": 500,
    "notes": 150,
    "memory_cells": 1200
  },
  "checks": [
    {"name": "database", "status": "ok"},
    {"name": "storage", "status": "ok"},
    {"name": "llm_provider", "status": "ok"}
  ]
}
```

**New Functions:**

```go
// GetStorageMetrics collects storage health metrics
func GetStorageMetrics(dataDir string) (*StorageMetrics, error) {
    return &StorageMetrics{
        TotalSizeMB:      calculateDirSize(dataDir),
        DiskAvailableGB:  getDiskAvailable(dataDir),
        DiskUsedPercent:  getDiskUsedPercent(dataDir),
        Writable:         isWritable(dataDir),
        UserCount:        countUsers(dataDir),
        ConversationCount: countConversations(dataDir),
        NoteCount:        countNotes(dataDir),
        MemoryCellCount:  countMemoryCells(dataDir),
    }, nil
}
```

### Testing Strategy

**Unit Tests:**
```go
func TestInitialize(t *testing.T) {
    // Test: Creates directories
    // Test: Creates default admin user
    // Test: Idempotent (safe to run twice)
    // Test: Handles permission errors gracefully
    // Test: Creates .gitignore
}

func TestGetStorageMetrics(t *testing.T) {
    // Test: Counts users correctly
    // Test: Calculates disk usage
    // Test: Handles missing directories
    // Test: Returns error on unreadable directory
}
```

**Integration Tests:**
```go
func TestFullInitialization(t *testing.T) {
    // Test: Empty directory → fully initialized
    // Test: Partial initialization → completes missing pieces
    // Test: Health endpoint returns storage metrics
}
```

**Manual Testing:**
```bash
# Test 1: Fresh install
rm -rf data/ && ./bin/nuimanbot
# Expected: App starts, creates directories, admin user exists

# Test 2: Restart with existing data
./bin/nuimanbot
# Expected: App starts, no errors, doesn't recreate admin

# Test 3: Health endpoint
curl http://localhost:8080/health | jq
# Expected: JSON with storage metrics
```

## Configuration

**New Config Options (config.yaml):**

```yaml
storage:
  data_dir: "./data"              # Base data directory
  auto_init: true                 # Enable auto-initialization (default: true)

  # Default admin user (created on first run)
  default_admin:
    user_id: "admin"
    email: "admin@localhost"
    create_on_startup: true       # Create if missing (default: true)
```

**Environment Variables:**

```bash
NUIMANBOT_STORAGE_DATA_DIR=./data
NUIMANBOT_STORAGE_AUTO_INIT=true
NUIMANBOT_STORAGE_DEFAULT_ADMIN_EMAIL=admin@example.com
```

## Success Metrics

**Developer Experience:**
- ⚡ Time to first run: < 30 seconds (download, run, works)
- 📉 Support questions about setup: Reduce by 80%

**Operational:**
- ✅ Container startup: 0 init containers required
- ✅ Health check response time: < 100ms
- ✅ Zero manual pre-deployment steps

**Code Quality:**
- ✅ Test coverage: >90% for initialization code
- ✅ No regressions in existing health checks
- ✅ Backward compatible (existing deployments unaffected)

## Migration & Backward Compatibility

**Existing Installations:**
- If `data/` directory exists → initialization is no-op
- Existing admin user preserved
- No breaking changes to existing functionality

**Migration Steps:**
1. Users with existing shell script setup: No action required, app initializes idempotently
2. New users: Just run the binary, no setup needed
3. Shell scripts remain available in `support_docs/operations/` as optional helpers

## Documentation Updates

**Files to Update:**
1. `README.md` - Update quick start (remove manual init steps)
2. `support_docs/install-and-setup.md` - Simplify setup instructions
3. `support_docs/admin-guide.md` - Document enhanced health endpoint
4. `support_docs/operations/README.md` - Mark scripts as optional
5. `config.yaml` - Clarify multi-bot architecture and gateway configuration

**New Documentation:**
- Health endpoint schema documentation
- Storage metrics reference
- Multi-bot support explanation in config.yaml

**Multi-Bot Configuration Clarification:**
Add comprehensive documentation to `config.yaml` gateways section explaining:
- System-level bots (config.yaml) vs user-owned bots (bots.json)
- Bot selection priority (user private bot → public bot → system bot)
- BYOB (Bring Your Own Bot) capability via Admin API
- Credential storage locations (config.yaml vs encrypted bots.json)
- How users can create their own Slack/Telegram bots

## Open Questions

1. **Q**: Should we create a CLI command for manual re-initialization?
   **A**: Not in MVP, can add later if needed

2. **Q**: What if user runs app without write permissions to data directory?
   **A**: Log clear error message, fail fast with actionable guidance

3. **Q**: Should default admin password be configurable?
   **A**: Yes, via config or environment variable

4. **Q**: Should we validate disk space before initialization?
   **A**: Yes, fail if < 100MB available

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Permission errors on startup | High - app won't start | Clear error messages with resolution steps |
| Existing deployments break | High - production impact | Thoroughly test idempotent behavior |
| Performance impact of health checks | Medium - slow monitoring | Cache metrics for 5 seconds |
| Default admin security concerns | High - unauthorized access | Force password change on first login |

## Implementation Checklist

### Phase 1: Auto-Initialization
- [ ] Create `internal/infrastructure/storage/initializer.go`
- [ ] Implement `Initialize()` function
- [ ] Implement `EnsureDirectories()`
- [ ] Implement `CreateDefaultAdmin()`
- [ ] Write unit tests (>90% coverage)
- [ ] Integrate into `cmd/nuimanbot/main.go`
- [ ] Test: Fresh install scenario
- [ ] Test: Restart with existing data
- [ ] Test: Permission error handling

### Phase 2: Enhanced Health Endpoint
- [ ] Create `internal/infrastructure/storage/metrics.go`
- [ ] Implement `GetStorageMetrics()`
- [ ] Enhance `/health` endpoint response
- [ ] Add storage metrics to response
- [ ] Write unit tests for metrics collection
- [ ] Integration test: Health endpoint E2E
- [ ] Performance test: Health endpoint < 100ms

### Phase 3: Documentation & Cleanup
- [ ] Update README.md quick start
- [ ] Update install-and-setup.md
- [ ] Create health endpoint documentation
- [ ] Update `config.yaml` with multi-bot architecture clarification
  - [ ] Add comprehensive header to `gateways:` section
  - [ ] Document system bot vs user-owned bot distinction
  - [ ] Explain bot selection priority (user → system fallback)
  - [ ] Document BYOB capability via Admin API
  - [ ] Clarify credential storage locations
- [ ] Update support_docs/operations/README.md (already done)
- [ ] Update AGENTS.md if needed

### Phase 4: Quality Gates
- [ ] All tests pass (`go test ./...`)
- [ ] Test coverage >90%
- [ ] Linter passes (`golangci-lint run`)
- [ ] Build succeeds (`go build -o bin/nuimanbot ./cmd/nuimanbot`)
- [ ] Manual smoke test: Fresh install
- [ ] Manual smoke test: Kubernetes deployment

## Timeline Estimate

| Phase | Effort | Duration |
|-------|--------|----------|
| Phase 1: Auto-Init | 3-5 hours | 0.5 day |
| Phase 2: Health Metrics | 2-3 hours | 0.5 day |
| Phase 3: Documentation | 1-2 hours | 0.25 day |
| Phase 4: Testing & QA | 2-3 hours | 0.5 day |
| **Total** | **8-13 hours** | **~2 days** |

## References

- [Clean Architecture Guidelines](../../AGENTS.md)
- [12-Factor App Principles](https://12factor.net/)
- [Health Check Best Practices](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Operational Scripts (Optional)](../../support_docs/operations/README.md)

---

**Status**: Ready for implementation
**Complexity**: Medium
**Impact**: High (significantly improves UX and deployment experience)
