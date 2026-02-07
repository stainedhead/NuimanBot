# Developer Productivity Skills - Task Breakdown

**Feature Suite:** Developer Productivity Skills (Phase 5)
**Version:** 1.0
**Created:** 2026-02-07
**Status:** Planning

---

## Task Organization

Tasks are organized by phase and assigned to specific subagents for parallel execution. Each task includes:
- **ID:** Unique task identifier
- **Phase:** Implementation phase (1-4)
- **Agent:** Assigned subagent
- **Dependencies:** Prerequisites that must complete first
- **Estimated Duration:** Time estimate in days
- **Acceptance Criteria:** Definition of done

---

## Phase 1: Foundation Infrastructure

### Task F1.1: ExecutorService Interface and Implementation

**ID:** F1.1
**Agent:** Agent 1 (Foundation)
**Dependencies:** None
**Duration:** 2 days
**Priority:** P0 (Critical - blocks all skills)

**Description:**
Create ExecutorService interface and implementation for command execution with timeout, PTY mode, and background session support.

**Files to Create:**
- `internal/usecase/skill/executor/service.go`
- `internal/usecase/skill/executor/service_test.go`
- `internal/usecase/skill/executor/types.go`
- `internal/usecase/skill/executor/pty.go` (PTY mode support)

**Acceptance Criteria:**
- [ ] ExecutorService interface defined with Execute() and ExecuteBackground() methods
- [ ] Implementation supports timeout enforcement (context.WithTimeout)
- [ ] Implementation supports PTY mode for interactive CLIs (using github.com/creack/pty)
- [ ] Background sessions stored in-memory with unique session IDs
- [ ] Error handling covers: timeout, command not found, non-zero exit codes
- [ ] Unit tests cover: successful execution, timeout, PTY mode, background sessions
- [ ] Test coverage ≥90%

**Test Cases:**
```go
TestExecutorService_Execute_Success
TestExecutorService_Execute_Timeout
TestExecutorService_Execute_CommandNotFound
TestExecutorService_Execute_NonZeroExit
TestExecutorService_Execute_PTYMode
TestExecutorService_ExecuteBackground_CreateSession
TestExecutorService_ExecuteBackground_GetOutput
```

---

### Task F1.2: RateLimiter Implementation

**ID:** F1.2
**Agent:** Agent 1 (Foundation)
**Dependencies:** None (can run in parallel with F1.1)
**Duration:** 1 day
**Priority:** P0 (Critical)

**Description:**
Create per-skill, per-user rate limiter using golang.org/x/time/rate.

**Files to Create:**
- `internal/usecase/skill/common/ratelimiter.go`
- `internal/usecase/skill/common/ratelimiter_test.go`

**Acceptance Criteria:**
- [ ] RateLimiter supports per-skill, per-user limits
- [ ] Rate limits configurable (e.g., "30/minute", "20/hour")
- [ ] Thread-safe implementation (sync.Mutex)
- [ ] Allow() method returns bool for rate limit check
- [ ] Cleanup of stale limiters (last used >1 hour ago)
- [ ] Unit tests cover: allowed, denied, concurrent requests
- [ ] Test coverage ≥95%

**Test Cases:**
```go
TestRateLimiter_Allow_UnderLimit
TestRateLimiter_Allow_OverLimit
TestRateLimiter_Allow_ConcurrentRequests
TestRateLimiter_Cleanup_StaleLimiters
```

---

### Task F1.3: OutputSanitizer Implementation

**ID:** F1.3
**Agent:** Agent 1 (Foundation)
**Dependencies:** None (can run in parallel with F1.1, F1.2)
**Duration:** 1 day
**Priority:** P0 (Critical)

**Description:**
Create output sanitizer for secret pattern detection and redaction.

**Files to Create:**
- `internal/usecase/skill/common/sanitizer.go`
- `internal/usecase/skill/common/sanitizer_test.go`

**Acceptance Criteria:**
- [ ] Sanitizer detects patterns: GitHub tokens (ghp_*), OpenAI keys (sk-*), Google API keys (AIza*)
- [ ] Redacts matches with "[REDACTED]"
- [ ] Configurable pattern list (regex-based)
- [ ] SanitizeOutput() method returns sanitized string
- [ ] Unit tests cover: each secret pattern, no false positives
- [ ] Test coverage ≥95%

**Test Cases:**
```go
TestSanitizer_RedactGitHubToken
TestSanitizer_RedactOpenAIKey
TestSanitizer_RedactGoogleAPIKey
TestSanitizer_NoFalsePositives
```

---

### Task F1.4: PathValidator Implementation

**ID:** F1.4
**Agent:** Agent 1 (Foundation)
**Dependencies:** None (can run in parallel with F1.1-F1.3)
**Duration:** 1 day
**Priority:** P0 (Critical)

**Description:**
Create path validator for workspace restriction and path traversal prevention.

**Files to Create:**
- `internal/usecase/skill/common/validator.go`
- `internal/usecase/skill/common/validator_test.go`

**Acceptance Criteria:**
- [ ] ValidatePath() checks path against allowed directories
- [ ] Rejects paths with "../" or absolute paths outside workspace
- [ ] Uses filepath.Clean() and filepath.Abs() for canonicalization
- [ ] Returns clear error messages for invalid paths
- [ ] Unit tests cover: valid paths, traversal attempts, absolute paths
- [ ] Test coverage ≥95%

**Test Cases:**
```go
TestPathValidator_ValidPath
TestPathValidator_RejectTraversal
TestPathValidator_RejectAbsoluteOutsideWorkspace
TestPathValidator_CanonicalizePath
```

---

### Task F1.5: Test Utilities for Skills

**ID:** F1.5
**Agent:** Agent 1 (Foundation)
**Dependencies:** F1.1 (needs ExecutorService interface)
**Duration:** 0.5 days
**Priority:** P1 (High)

**Description:**
Create shared test utilities for skill testing (mock executor, test helpers).

**Files to Create:**
- `internal/usecase/skill/testutil/mock_executor.go`
- `internal/usecase/skill/testutil/helpers.go`

**Acceptance Criteria:**
- [ ] MockExecutor implements ExecutorService interface
- [ ] MockExecutor allows configuring responses and errors
- [ ] Test helpers for common assertions (validateSkillResult, etc.)
- [ ] Documented examples of usage

**Test Cases:**
- No tests needed (this is test infrastructure)

---

## Phase 2: Core Skills Development (Parallel)

### Task S2.1: GitHubSkill Implementation

**ID:** S2.1
**Agent:** Agent 2 (GitHub)
**Dependencies:** F1.1, F1.2, F1.3, F1.4, F1.5
**Duration:** 5 days
**Priority:** P1 (High)

**Description:**
Implement GitHubSkill with all 12 GitHub operations.

**Files to Create:**
- `internal/usecase/skill/github/skill.go`
- `internal/usecase/skill/github/skill_test.go`
- `internal/usecase/skill/github/types.go`
- `internal/usecase/skill/github/parser.go`

**Acceptance Criteria:**
- [ ] Implements Skill interface (Name, Description, InputSchema, Execute, RequiredPermissions, Config)
- [ ] Supports 12 actions: issue_list, issue_view, issue_create, issue_comment, issue_close, pr_list, pr_view, pr_create, pr_review, pr_merge, repo_view, workflow_run
- [ ] Validates repo format (owner/repo pattern)
- [ ] Uses ExecutorService to run `gh` CLI commands
- [ ] Parses JSON output from `gh --json` flag
- [ ] Rate limits enforced (30 ops/min per user)
- [ ] Error handling for: command failure, auth failure, repo not found
- [ ] Unit tests with mock executor
- [ ] Test coverage ≥90%

**Test Cases:**
```go
TestGitHubSkill_Execute_IssueList
TestGitHubSkill_Execute_IssueCreate
TestGitHubSkill_Execute_PRList
TestGitHubSkill_Execute_PRMerge
TestGitHubSkill_Execute_InvalidRepo
TestGitHubSkill_Execute_CommandFailure
TestGitHubSkill_Execute_RateLimitExceeded
```

---

### Task S2.2: RepoSearchSkill Implementation

**ID:** S2.2
**Agent:** Agent 3 (RepoSearch)
**Dependencies:** F1.1, F1.2, F1.3, F1.4, F1.5
**Duration:** 3 days
**Priority:** P1 (High)

**Description:**
Implement RepoSearchSkill for codebase search via ripgrep.

**Files to Create:**
- `internal/usecase/skill/reposearch/skill.go`
- `internal/usecase/skill/reposearch/skill_test.go`
- `internal/usecase/skill/reposearch/parser.go`

**Acceptance Criteria:**
- [ ] Implements Skill interface
- [ ] Supports parameters: query, path, file_type, max_results, context_lines
- [ ] Validates path against allowed directories (PathValidator)
- [ ] Uses ExecutorService to run `rg --json` command
- [ ] Parses JSON output from ripgrep
- [ ] Returns SearchResponse with results, total_matches, truncated flag
- [ ] Performance: <2s for typical repos (<100k LOC)
- [ ] Unit tests with mock executor
- [ ] Test coverage ≥90%

**Test Cases:**
```go
TestRepoSearchSkill_Execute_Success
TestRepoSearchSkill_Execute_CaseInsensitive
TestRepoSearchSkill_Execute_FileTypeFilter
TestRepoSearchSkill_Execute_MaxResults
TestRepoSearchSkill_Execute_InvalidPath
TestRepoSearchSkill_Execute_PathTraversal
```

---

### Task S2.3: DocSummarizeSkill Implementation

**ID:** S2.3
**Agent:** Agent 4 (DocSummarize)
**Dependencies:** F1.1, F1.2, F1.3, F1.4, F1.5
**Duration:** 5 days
**Priority:** P1 (High)

**Description:**
Implement DocSummarizeSkill for internal documentation summarization.

**Files to Create:**
- `internal/usecase/skill/docsummarize/skill.go`
- `internal/usecase/skill/docsummarize/skill_test.go`
- `internal/usecase/skill/docsummarize/fetcher.go`
- `internal/usecase/skill/docsummarize/summarizer.go`

**Acceptance Criteria:**
- [ ] Implements Skill interface
- [ ] Supports source types: file, git, http
- [ ] Validates domain allowlist for http sources
- [ ] Enforces file size limits (5MB max)
- [ ] Uses LLMService for summarization
- [ ] Extracts key topics from summary (via LLM)
- [ ] Returns SummaryResult with summary, source, word_count, key_topics
- [ ] Rate limits enforced (10 summaries/hour per user)
- [ ] Unit tests with mock LLMService
- [ ] Test coverage ≥85%

**Test Cases:**
```go
TestDocSummarizeSkill_Execute_FileSource
TestDocSummarizeSkill_Execute_HTTPSource
TestDocSummarizeSkill_Execute_InvalidDomain
TestDocSummarizeSkill_Execute_FileTooLarge
TestDocSummarizeSkill_Execute_LLMFailure
TestDocSummarizeSkill_Execute_RateLimitExceeded
```

---

### Task S2.4: SummarizeSkill Implementation

**ID:** S2.4
**Agent:** Agent 5 (Summarize)
**Dependencies:** F1.1, F1.2, F1.3, F1.4, F1.5
**Duration:** 7 days
**Priority:** P1 (High)

**Description:**
Implement SummarizeSkill for external URL and YouTube video summarization.

**Files to Create:**
- `internal/usecase/skill/summarize/skill.go`
- `internal/usecase/skill/summarize/skill_test.go`
- `internal/usecase/skill/summarize/extractor.go`
- `internal/usecase/skill/summarize/youtube.go`
- `internal/usecase/skill/summarize/summarizer.go`

**Acceptance Criteria:**
- [ ] Implements Skill interface
- [ ] Supports HTTP/HTTPS URLs and YouTube videos
- [ ] Extracts main content from HTML (removes nav, ads, scripts) using goquery
- [ ] Extracts YouTube transcripts via yt-dlp
- [ ] Generates summaries in brief/detailed/bullet_points formats
- [ ] Optionally includes key quotes from source
- [ ] Validates URL format and content-type
- [ ] Rate limits enforced (20 summaries/hour per user)
- [ ] Unit tests with mock HTTP responses and mock yt-dlp
- [ ] Test coverage ≥85%

**Test Cases:**
```go
TestSummarizeSkill_Execute_HTTPArticle
TestSummarizeSkill_Execute_YouTubeVideo
TestSummarizeSkill_Execute_BulletPointsFormat
TestSummarizeSkill_Execute_IncludeQuotes
TestSummarizeSkill_Execute_InvalidURL
TestSummarizeSkill_Execute_VideoTooLong
TestSummarizeSkill_Execute_RateLimitExceeded
```

---

### Task S2.5: CodingAgentSkill Implementation

**ID:** S2.5
**Agent:** Agent 6 (CodingAgent)
**Dependencies:** F1.1, F1.2, F1.3, F1.4, F1.5
**Duration:** 7 days
**Priority:** P1 (High)

**Description:**
Implement CodingAgentSkill for orchestrating external coding CLI tools.

**Files to Create:**
- `internal/usecase/skill/codingagent/skill.go`
- `internal/usecase/skill/codingagent/skill_test.go`
- `internal/usecase/skill/codingagent/session.go`
- `internal/usecase/skill/codingagent/executor.go`

**Acceptance Criteria:**
- [ ] Implements Skill interface
- [ ] Supports tools: codex, claude_code, opencode, gemini, copilot
- [ ] Executes in PTY mode for interactive CLIs
- [ ] Manages background sessions (in-memory SessionManager)
- [ ] Implements approval workflow for interactive mode
- [ ] Validates workspace restrictions
- [ ] Requires admin permission by default
- [ ] Returns CodingAgentResult with status, files_modified, approvals
- [ ] Unit tests with mock executor and mock sessions
- [ ] Test coverage ≥85%

**Test Cases:**
```go
TestCodingAgentSkill_Execute_InteractiveMode
TestCodingAgentSkill_Execute_AutoMode
TestCodingAgentSkill_Execute_BackgroundSession
TestCodingAgentSkill_Execute_WorkspaceRestriction
TestCodingAgentSkill_Execute_PermissionDenied
TestCodingAgentSkill_Execute_ToolNotFound
```

---

## Phase 3: Integration and Configuration

### Task I3.1: Skill Registry Integration

**ID:** I3.1
**Agent:** Agent 7 (Integration)
**Dependencies:** S2.1, S2.2, S2.3, S2.4, S2.5
**Duration:** 1 day
**Priority:** P0 (Critical)

**Description:**
Register all 5 skills in NuimanBot skill registry and update main.go.

**Files to Modify:**
- `cmd/nuimanbot/main.go`
- `internal/usecase/skill/registry.go` (if needed)

**Acceptance Criteria:**
- [ ] All 5 skills instantiated with config
- [ ] All 5 skills registered in SkillRegistry
- [ ] Skills load successfully on application startup
- [ ] Skills accessible via chat service and LLM tool calling
- [ ] Error handling for skill initialization failures
- [ ] Integration tests for skill loading

**Test Cases:**
```go
TestSkillRegistry_LoadAllSkills
TestSkillRegistry_ExecuteGitHubSkill
TestSkillRegistry_ExecuteRepoSearchSkill
TestSkillRegistry_ExecuteDocSummarizeSkill
TestSkillRegistry_ExecuteSummarizeSkill
TestSkillRegistry_ExecuteCodingAgentSkill
```

---

### Task I3.2: Configuration Schema Updates

**ID:** I3.2
**Agent:** Agent 7 (Integration)
**Dependencies:** S2.1, S2.2, S2.3, S2.4, S2.5
**Duration:** 0.5 days
**Priority:** P0 (Critical)

**Description:**
Add configuration entries for all 5 skills to config.yaml.

**Files to Modify:**
- `config.yaml` (example config)
- `internal/infrastructure/config/config.go` (if schema changes needed)

**Acceptance Criteria:**
- [ ] Configuration entries for all 5 skills
- [ ] All parameters documented with comments
- [ ] Default values provided where appropriate
- [ ] Configuration validates successfully
- [ ] Example config.yaml updated

**Configuration Sections:**
```yaml
skills:
  entries:
    github:
      enabled: true
      params: {...}
    repo_search:
      enabled: true
      params: {...}
    doc_summarize:
      enabled: true
      params: {...}
    summarize:
      enabled: true
      params: {...}
    coding_agent:
      enabled: false
      params: {...}
```

---

### Task I3.3: Documentation Updates

**ID:** I3.3
**Agent:** Agent 7 (Integration)
**Dependencies:** S2.1, S2.2, S2.3, S2.4, S2.5
**Duration:** 1 day
**Priority:** P1 (High)

**Description:**
Update README.md and technical-details.md with new skill documentation.

**Files to Modify:**
- `README.md`
- `documentation/technical-details.md`
- `documentation/product-summary.md` (update feature list)

**Acceptance Criteria:**
- [ ] README.md includes:
  - Description of each skill
  - Usage examples
  - Configuration instructions
  - Required external tools
- [ ] technical-details.md includes:
  - Architecture details for skills
  - API documentation for skill parameters
  - Security controls
- [ ] product-summary.md updated with new skills

---

## Phase 4: End-to-End Testing (Parallel)

### Task E4.1: CLI Gateway E2E Tests

**ID:** E4.1
**Agent:** Agent 8 (CLI Testing)
**Dependencies:** I3.1, I3.2
**Duration:** 1.5 days
**Priority:** P1 (High)

**Description:**
Create and run E2E tests for all skills via CLI gateway.

**Files to Create:**
- `e2e/cli_gateway_skills_test.go`

**Test Scenarios:**
- [ ] User lists GitHub issues via CLI
- [ ] User searches codebase via CLI
- [ ] User summarizes doc via CLI
- [ ] User summarizes URL via CLI
- [ ] Admin runs coding agent task via CLI (manual verification)

**Acceptance Criteria:**
- [ ] All scenarios complete successfully
- [ ] Error messages are clear and actionable
- [ ] Rate limiting works as expected
- [ ] Audit events logged correctly
- [ ] Conversation history updated correctly

---

### Task E4.2: Telegram Gateway E2E Tests

**ID:** E4.2
**Agent:** Agent 9 (Telegram Testing)
**Dependencies:** I3.1, I3.2
**Duration:** 1.5 days
**Priority:** P1 (High)

**Description:**
Create and run E2E tests for skills via Telegram gateway.

**Files to Create:**
- `e2e/telegram_gateway_skills_test.go`

**Test Scenarios:**
- [ ] User creates GitHub PR via Telegram
- [ ] User searches docs via Telegram
- [ ] User summarizes YouTube video via Telegram

**Acceptance Criteria:**
- [ ] All scenarios complete successfully
- [ ] Responses formatted correctly for Telegram
- [ ] Markdown rendering works
- [ ] Permission denials handled gracefully

---

### Task E4.3: Security Testing

**ID:** E4.3
**Agent:** Agent 10 (Security Testing)
**Dependencies:** I3.1, I3.2
**Duration:** 2 days
**Priority:** P0 (Critical)

**Description:**
Create and run security tests for all threat model scenarios.

**Files to Create:**
- `internal/usecase/skill/security_test.go`
- `e2e/security_test.go`

**Test Scenarios:**
- [ ] Command injection attempts (github, coding_agent skills)
- [ ] Path traversal attempts (repo_search skill)
- [ ] Domain allowlist bypass (summarize, doc_summarize skills)
- [ ] Rate limit evasion attempts
- [ ] Privilege escalation attempts (user accessing admin-only skills)
- [ ] Secret exposure in outputs

**Acceptance Criteria:**
- [ ] All injection attempts blocked
- [ ] Path traversal prevented
- [ ] Domain allowlist enforced
- [ ] Rate limits cannot be bypassed
- [ ] RBAC enforced correctly
- [ ] Secrets redacted from outputs
- [ ] All attempts logged to audit log

---

## Task Dependencies Graph

```
Phase 1 (Foundation):
F1.1 (ExecutorService) ─┐
                        ├─→ F1.5 (Test Utilities) ─┐
F1.2 (RateLimiter) ─────┤                           │
F1.3 (Sanitizer) ───────┤                           │
F1.4 (PathValidator) ───┘                           │
                                                    ↓
Phase 2 (Skills - All parallel, depend on Phase 1):
                                    ┌─→ S2.1 (GitHub) ─────┐
                                    ├─→ S2.2 (RepoSearch) ─┤
F1.1, F1.2, F1.3, F1.4, F1.5 ──────┼─→ S2.3 (DocSummarize)─┤
                                    ├─→ S2.4 (Summarize) ───┤
                                    └─→ S2.5 (CodingAgent) ─┤
                                                            ↓
Phase 3 (Integration - Sequential):
S2.1, S2.2, S2.3, S2.4, S2.5 ─→ I3.1 (Registry) ─┐
                                                  ├─→ I3.3 (Docs)
                              → I3.2 (Config) ────┘
                                                    ↓
Phase 4 (Testing - Parallel):
I3.1, I3.2 ─┬─→ E4.1 (CLI Tests) ────┐
            ├─→ E4.2 (Telegram Tests) ┤
            └─→ E4.3 (Security Tests) ┘
```

---

## Task Summary

| Phase | Total Tasks | Agent-Days | Parallelization |
|-------|-------------|------------|-----------------|
| Phase 1: Foundation | 5 tasks | 5.5 days | Limited (1 agent) |
| Phase 2: Skills | 5 tasks | 27 days | Maximum (5 agents in parallel) |
| Phase 3: Integration | 3 tasks | 2.5 days | Low (1 agent) |
| Phase 4: Testing | 3 tasks | 5 days | Medium (3 agents in parallel) |
| **Total** | **16 tasks** | **40 days** | **~12-15 calendar days** |

---

## Next Steps

1. **Review and Approve:** Get stakeholder approval for task breakdown
2. **Agent Assignment:** Confirm subagent assignments and availability
3. **Create STATUS.md:** Set up progress tracking file
4. **Begin Phase 1:** Start with foundation infrastructure
5. **Monitor Progress:** Daily standup to track blockers and adjust timeline

**Task Breakdown Complete:** Ready to proceed to implementation and STATUS.md creation.
