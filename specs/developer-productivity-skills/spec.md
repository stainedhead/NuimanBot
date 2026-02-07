# Developer Productivity Skills - Feature Specification

**Feature Suite:** Developer Productivity Skills (Phase 5)
**Version:** 1.0
**Created:** 2026-02-07
**Status:** Planning
**Priority:** P2 (High - Post-MVP)

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Goals and Success Metrics](#goals-and-success-metrics)
3. [Skill Specifications](#skill-specifications)
4. [User Stories](#user-stories)
5. [Acceptance Criteria](#acceptance-criteria)
6. [Dependencies and Prerequisites](#dependencies-and-prerequisites)
7. [Security Considerations](#security-considerations)
8. [Testing Strategy](#testing-strategy)

---

## Executive Summary

This feature suite adds five developer-focused skills to NuimanBot, enabling engineering workflows for research, code exploration, documentation, and AI-assisted coding. These skills are inspired by successful HomeBots implementations (OpenClaw, NanoBot) and address common developer needs while maintaining NuimanBot's security-first approach.

### Skills in Scope

| Skill | Purpose | Complexity | Reference Implementation |
|-------|---------|------------|-------------------------|
| `github` | GitHub operations via `gh` CLI | Medium | OpenClaw/NanoBot `github` |
| `repo_search` | Fast codebase search via ripgrep | Low | Internal proposal (BOT_TOOL_IDEAS) |
| `doc_summarize` | Summaries of internal docs and links | Medium | Internal proposal (BOT_TOOL_IDEAS) |
| `summarize` | External URL/file/YouTube summarization | High | OpenClaw `summarize` |
| `coding_agent` | Orchestrate Codex/Claude Code/OpenCode CLI runs | High | OpenClaw `coding-agent` |

### Key Benefits

- **Research Efficiency**: Quick codebase exploration with `repo_search` and `doc_summarize`
- **GitHub Integration**: PR reviews, issue management, repo operations without leaving chat
- **Knowledge Synthesis**: Summarize documentation, articles, videos for quick understanding
- **AI-Assisted Coding**: Delegate complex coding tasks to specialized CLI tools (Codex, Claude Code)
- **Security Maintained**: All skills follow RBAC, audit logging, and permission gating

---

## Goals and Success Metrics

### Goals

| Goal | Metric | Target |
|------|--------|--------|
| Enable GitHub workflows | All CRUD operations for issues, PRs, repos functional | 100% |
| Fast code search | Repo search completes in <2s for typical repos (<100k LOC) | <2s |
| Accurate summarization | Summary quality rated 4+/5 by users | ≥80% |
| Coding agent reliability | Coding agent tasks complete successfully (non-interactive) | ≥90% |
| Security parity | No security regressions; all events audited | 100% |

### Non-Goals

- Building custom GitHub API client (use `gh` CLI)
- Full browser automation for web scraping (deferred to Phase 8)
- Real-time collaboration features (pair programming)
- Auto-commit/auto-push without explicit user approval

---

## Skill Specifications

### Skill 1: `github`

**Description:** GitHub operations via `gh` CLI wrapper

**Permissions Required:** `network`, `shell` (restricted to `gh` command only)

**Functional Requirements:**
- **Issues:** Create, list, view, comment, close, reopen
- **Pull Requests:** Create, list, view, review, comment, merge, close
- **Repositories:** Clone, fork, create, view, archive
- **Workflows:** List runs, view logs, trigger manually
- **Authentication:** Use `gh auth status` to verify, require manual `gh auth login` on first use

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "action": {
      "type": "string",
      "enum": ["issue_create", "issue_list", "pr_create", "pr_list", "pr_review", "repo_view", "workflow_run"]
    },
    "repo": {
      "type": "string",
      "description": "Repository in format 'owner/repo'"
    },
    "params": {
      "type": "object",
      "description": "Action-specific parameters"
    }
  },
  "required": ["action"]
}
```

**Output Format:**
- Success: JSON response from `gh` CLI with `--json` flag
- Error: Stderr from `gh` command with sanitized error message

**Configuration:**
```yaml
skills:
  entries:
    github:
      enabled: true
      params:
        timeout: 30  # Command timeout in seconds
        default_repo: ""  # Optional default repo for current project
```

**Security Controls:**
- Command allowlist: Only `gh` command allowed (no `git` or other shell commands)
- Output sanitization: Remove potential secrets from error messages
- Audit logging: Log all GitHub operations with user, repo, action
- Rate limiting: Max 30 operations per user per minute (align with GitHub API limits)

---

### Skill 2: `repo_search`

**Description:** Fast codebase search using ripgrep (`rg`)

**Permissions Required:** `read` (filesystem access to allowed directories)

**Functional Requirements:**
- **Search Modes:** Content search, filename search, pattern search (regex)
- **Filters:** File type, directory scope, exclude patterns
- **Output:** File paths, line numbers, matched content (with context)
- **Workspace Restriction:** Search only within allowed workspace directories

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search query (regex or literal)"
    },
    "path": {
      "type": "string",
      "description": "Directory to search (default: current workspace)"
    },
    "file_type": {
      "type": "string",
      "description": "Filter by file extension (e.g., 'go', 'js')"
    },
    "max_results": {
      "type": "integer",
      "default": 50,
      "description": "Maximum number of results to return"
    },
    "context_lines": {
      "type": "integer",
      "default": 2,
      "description": "Number of context lines before/after match"
    }
  },
  "required": ["query"]
}
```

**Output Format:**
```json
{
  "results": [
    {
      "file": "internal/domain/user.go",
      "line": 42,
      "match": "type User struct {",
      "context_before": ["package domain", ""],
      "context_after": ["\tID string", "\tUsername string"]
    }
  ],
  "total_matches": 15,
  "truncated": false
}
```

**Configuration:**
```yaml
skills:
  entries:
    repo_search:
      enabled: true
      params:
        allowed_directories:
          - ./internal
          - ./cmd
          - ./pkg
        excluded_patterns:
          - "*.log"
          - "*.tmp"
          - ".git/*"
        max_file_size: 1048576  # 1MB max per file
```

**Security Controls:**
- Workspace restriction: Only search within configured allowed directories
- Path traversal prevention: Reject queries with `../` or absolute paths outside workspace
- File size limits: Skip files larger than configured max
- Output sanitization: Remove sensitive patterns (API keys, tokens) from results

---

### Skill 3: `doc_summarize`

**Description:** Summarize internal documentation files and links

**Permissions Required:** `read`, `network` (for external links)

**Functional Requirements:**
- **Input Types:** Local file paths, Git URLs, HTTP/HTTPS URLs
- **Supported Formats:** Markdown, plain text, HTML (basic), PDF (via external tool)
- **Summarization:** Use LLM provider to generate concise summary (target: 200-500 words)
- **Metadata:** Include source, word count, key topics, timestamp

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "source": {
      "type": "string",
      "description": "File path, Git URL, or HTTP URL"
    },
    "max_words": {
      "type": "integer",
      "default": 300,
      "description": "Target summary length in words"
    },
    "focus": {
      "type": "string",
      "description": "Optional focus area (e.g., 'API changes', 'security')"
    }
  },
  "required": ["source"]
}
```

**Output Format:**
```json
{
  "summary": "NuimanBot is a security-hardened AI agent...",
  "source": "https://github.com/user/repo/blob/main/README.md",
  "word_count": 287,
  "key_topics": ["security", "AI agent", "multi-platform"],
  "timestamp": "2026-02-07T10:30:00Z"
}
```

**Configuration:**
```yaml
skills:
  entries:
    doc_summarize:
      enabled: true
      params:
        timeout: 60  # Longer timeout for large documents
        max_document_size: 5242880  # 5MB max
        allowed_domains:
          - github.com
          - docs.google.com
          - notion.so
```

**Security Controls:**
- Domain allowlist: Only fetch from configured allowed domains
- File size limits: Reject documents larger than configured max
- Content sanitization: Remove scripts and executable content before summarization
- Rate limiting: Max 10 summarizations per user per hour (LLM cost control)

---

### Skill 4: `summarize`

**Description:** Summarize external URLs, files, and YouTube videos

**Permissions Required:** `network`

**Functional Requirements:**
- **URL Summarization:** Fetch and summarize web pages, articles, blog posts
- **YouTube Summarization:** Extract transcript (via yt-dlp or YouTube API) and summarize
- **File Summarization:** Support PDF, DOCX, TXT via file upload or URL
- **Smart Extraction:** Extract main content, ignore ads/navigation/boilerplate

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "description": "URL to summarize (HTTP, HTTPS, or YouTube)"
    },
    "format": {
      "type": "string",
      "enum": ["brief", "detailed", "bullet_points"],
      "default": "brief"
    },
    "include_quotes": {
      "type": "boolean",
      "default": false,
      "description": "Include key quotes from source"
    }
  },
  "required": ["url"]
}
```

**Output Format:**
```json
{
  "summary": "This article discusses the benefits of...",
  "title": "Why Go is Great for Backend Development",
  "author": "John Doe",
  "published_date": "2026-01-15",
  "source_type": "article",
  "reading_time": "8 minutes",
  "key_quotes": ["Go's simplicity is its strength", "..."],
  "url": "https://example.com/article"
}
```

**Configuration:**
```yaml
skills:
  entries:
    summarize:
      enabled: true
      params:
        timeout: 90  # Longer timeout for video transcripts
        youtube:
          api_key: ${YOUTUBE_API_KEY}  # Optional, falls back to yt-dlp
          max_duration: 3600  # Max 1 hour videos
        user_agent: "NuimanBot/1.0 (+https://github.com/user/repo)"
```

**Security Controls:**
- URL validation: Reject non-HTTP(S) schemes, localhost, private IPs
- Content-Type validation: Only process text/html, application/pdf, etc.
- Download size limits: Max 10MB for web pages, 50MB for videos (transcript only)
- Rate limiting: Max 20 summarizations per user per hour

---

### Skill 5: `coding_agent`

**Description:** Orchestrate external coding CLI tools (Codex, Claude Code, OpenCode, Gemini CLI, Copilot CLI)

**Permissions Required:** `shell` (admin only by default)

**Functional Requirements:**
- **CLI Support:** Codex, Claude Code (`claude-code`), OpenCode, Gemini CLI, GitHub Copilot CLI
- **Modes:** Interactive (with approval prompts), Auto (workspace-only auto-approve), YOLO (no approvals, high risk)
- **Session Management:** Background sessions for long tasks, log/poll operations
- **PTY Mode:** Interactive CLI support (required for some tools)
- **Input/Output:** Send input via write/submit, capture output via log tailing

**Input Schema:**
```json
{
  "type": "object",
  "properties": {
    "tool": {
      "type": "string",
      "enum": ["codex", "claude_code", "opencode", "gemini", "copilot"]
    },
    "task": {
      "type": "string",
      "description": "Task description for the coding agent"
    },
    "mode": {
      "type": "string",
      "enum": ["interactive", "auto", "yolo"],
      "default": "interactive"
    },
    "workspace": {
      "type": "string",
      "description": "Working directory (default: current workspace)"
    },
    "timeout": {
      "type": "integer",
      "default": 300,
      "description": "Task timeout in seconds"
    }
  },
  "required": ["tool", "task"]
}
```

**Output Format:**
```json
{
  "status": "completed",
  "output": "Created function calculateTotal() in utils.go...",
  "files_modified": ["internal/domain/utils.go"],
  "session_id": "coding_agent_123456",
  "duration": 45.3,
  "approvals_requested": 2,
  "approvals_granted": 2
}
```

**Configuration:**
```yaml
skills:
  entries:
    coding_agent:
      enabled: false  # Admin must explicitly enable
      params:
        allowed_tools:
          - codex
          - claude_code
        default_mode: interactive
        pty_mode: true
        require_git_repo: true  # Codex requirement
        auto_mode_whitelist:
          - ./internal
          - ./cmd
```

**Security Controls:**
- Admin-only permission by default (configurable per-user)
- Workspace restriction: Tasks confined to configured workspace directories
- Git repository requirement: Prevent scratch tasks outside version control
- Approval logging: Log all approval requests and user responses
- Mode restrictions: YOLO mode requires explicit admin configuration
- Command injection prevention: Sanitize task descriptions before passing to CLI

---

## User Stories

### Story 1: GitHub Issue Triage (User Role)
**As a** developer
**I want to** list and view GitHub issues via chat
**So that** I can triage issues without switching to browser

**Acceptance Criteria:**
- User sends "List open issues in myorg/myrepo"
- NuimanBot uses `github` skill to run `gh issue list --repo myorg/myrepo --state open`
- NuimanBot responds with formatted list of issues (number, title, labels)
- User can click issue number to view details
- User can create new issue via "Create issue: Title here"

### Story 2: Code Search (User Role)
**As a** developer
**I want to** search for function definitions across the codebase
**So that** I can quickly understand code structure without manual grepping

**Acceptance Criteria:**
- User sends "Search for 'func Authenticate' in internal/"
- NuimanBot uses `repo_search` skill with ripgrep
- NuimanBot responds with file paths, line numbers, and code snippets
- Results include 2 lines of context before/after match
- User can request more context or navigate to file

### Story 3: Documentation Summary (User Role)
**As a** developer
**I want to** summarize README files from GitHub repos
**So that** I can quickly understand a library without reading full documentation

**Acceptance Criteria:**
- User sends "Summarize https://github.com/anthropics/anthropic-sdk-go/blob/main/README.md"
- NuimanBot uses `doc_summarize` skill to fetch and summarize
- NuimanBot responds with 200-300 word summary highlighting key features
- Summary includes installation instructions, usage examples, key concepts
- User can request "detailed" or "brief" versions

### Story 4: Article Summarization (User Role)
**As a** developer
**I want to** summarize technical articles and blog posts
**So that** I can stay informed without reading full articles

**Acceptance Criteria:**
- User sends "Summarize https://blog.example.com/go-best-practices"
- NuimanBot uses `summarize` skill to extract and summarize content
- NuimanBot responds with article summary, author, reading time
- Summary includes key takeaways in bullet points
- User can optionally include key quotes from article

### Story 5: AI-Assisted Coding (Admin Role)
**As a** developer with admin privileges
**I want to** delegate coding tasks to Claude Code CLI
**So that** I can accelerate implementation of well-defined features

**Acceptance Criteria:**
- Admin sends "Use Claude Code to add input validation to User entity"
- NuimanBot uses `coding_agent` skill to spawn `claude-code` CLI in background
- Claude Code analyzes codebase, writes validation logic, runs tests
- NuimanBot prompts admin for approval at each file modification
- Admin approves/rejects each change
- NuimanBot reports final status with files modified

---

## Acceptance Criteria

### Functional Acceptance

- [ ] All 5 skills implement the Skill interface correctly
- [ ] Input schemas validate parameters and reject invalid inputs
- [ ] Output formats are consistent and machine-parseable (JSON)
- [ ] Error handling includes clear user-facing messages
- [ ] Timeouts are enforced for all external command executions
- [ ] Streaming output supported where applicable (e.g., coding_agent)

### Security Acceptance

- [ ] All skills enforce permission checks before execution
- [ ] RBAC rules applied: `shell` permission required for github/coding_agent
- [ ] Audit logging captures all skill invocations with parameters
- [ ] Command injection vectors blocked (input sanitization)
- [ ] Path traversal attacks prevented (workspace restriction)
- [ ] Rate limiting applied per skill per user
- [ ] Sensitive data redacted from logs and outputs

### Performance Acceptance

- [ ] `repo_search` completes in <2s for repos with <100k LOC
- [ ] `github` skill operations complete in <5s (excluding GitHub API latency)
- [ ] `doc_summarize` handles 5MB documents within 60s timeout
- [ ] `summarize` processes web pages within 30s timeout
- [ ] `coding_agent` supports background sessions for tasks >5 minutes

### Testing Acceptance

- [ ] 85%+ test coverage for all skill implementations
- [ ] Unit tests for input validation, output formatting, error handling
- [ ] Integration tests with mock CLI commands (github, rg, gh)
- [ ] E2E tests with real tools in controlled environment
- [ ] Security tests for injection attacks, path traversal, privilege escalation

---

## Dependencies and Prerequisites

### External Tools

| Tool | Required For | Installation | Version |
|------|-------------|--------------|---------|
| `gh` CLI | github skill | `brew install gh` or binary download | ≥2.0 |
| `ripgrep` | repo_search skill | `brew install ripgrep` or binary download | ≥13.0 |
| `yt-dlp` | summarize skill (YouTube) | `pip install yt-dlp` or binary download | ≥2023.01 |
| `codex` CLI | coding_agent skill | `npm install -g @codexai/cli` | ≥1.0 |
| `claude-code` CLI | coding_agent skill | Binary from Anthropic | ≥1.0 |

### Go Libraries

| Library | Purpose | Version |
|---------|---------|---------|
| `os/exec` | Command execution (stdlib) | - |
| `regexp` | Pattern validation (stdlib) | - |
| `encoding/json` | JSON parsing (stdlib) | - |

### Configuration Prerequisites

- User workspace directories configured in config.yaml
- GitHub CLI authenticated (`gh auth login` completed manually)
- Coding agent tools installed and in PATH
- LLM provider configured for summarization tasks

---

## Security Considerations

### Threat Model

| Threat | Impact | Mitigation |
|--------|--------|------------|
| Command injection via skill parameters | RCE, data exfiltration | Input sanitization, command allowlists |
| Path traversal in file operations | Unauthorized file access | Workspace restriction, path validation |
| Secrets exposure in search results | API key leakage | Output sanitization, secret pattern detection |
| Arbitrary code execution via coding_agent | System compromise | Admin-only permission, approval workflow, workspace jail |
| GitHub token theft | Repository access | Use `gh` CLI credential store, never pass tokens directly |

### Defense-in-Depth

1. **Input Layer:** Validate all parameters against schema, sanitize strings
2. **Execution Layer:** Command allowlists, workspace restrictions, timeouts
3. **Output Layer:** Sanitize results, redact secrets, limit response size
4. **Audit Layer:** Log all operations with full context
5. **Permission Layer:** RBAC enforcement, per-user allowlists

---

## Testing Strategy

### Unit Tests

**Coverage Target:** 90%+

**Focus Areas:**
- Input schema validation (valid, invalid, edge cases)
- Output formatting (JSON serialization, error messages)
- Error handling (command failures, timeouts, invalid responses)
- Permission checks (allowed, denied, role-based)

**Example Test:**
```go
func TestGitHubSkill_ValidateInput(t *testing.T) {
    skill := NewGitHubSkill(cfg)

    tests := []struct {
        name    string
        input   map[string]any
        wantErr bool
    }{
        {"valid issue list", map[string]any{"action": "issue_list", "repo": "owner/repo"}, false},
        {"missing action", map[string]any{"repo": "owner/repo"}, true},
        {"invalid action", map[string]any{"action": "invalid"}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := skill.ValidateInput(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateInput() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

**Coverage Target:** Key workflows

**Focus Areas:**
- Command execution with mock CLIs
- File system operations with temporary directories
- LLM provider integration for summarization
- Error propagation from CLI tools

**Example Test:**
```go
func TestRepoSearchSkill_Execute_Integration(t *testing.T) {
    // Setup temporary test repository
    tmpDir := t.TempDir()
    createTestFiles(tmpDir)

    skill := NewRepoSearchSkill(cfg)
    ctx := context.Background()

    result, err := skill.Execute(ctx, map[string]any{
        "query": "func.*User",
        "path": tmpDir,
    })

    require.NoError(t, err)
    assert.Contains(t, result.Output, "user.go")
    assert.Contains(t, result.Output, "type User struct")
}
```

### E2E Tests

**Coverage Target:** User stories

**Focus Areas:**
- Full message flow: User input → Skill execution → LLM integration → User response
- Multi-step workflows (e.g., search → summarize → create issue)
- Permission enforcement across gateways
- Error recovery and graceful degradation

### Security Tests

**Coverage Target:** Threat model

**Focus Areas:**
- Command injection attempts (malicious parameters)
- Path traversal attempts (../../../etc/passwd)
- Secrets exposure in outputs
- Privilege escalation attempts (user accessing admin-only features)

---

## Open Questions

1. **GitHub CLI Authentication:** Should NuimanBot manage `gh` authentication automatically or require manual setup?
   - **Recommendation:** Require manual setup for security (avoid storing GitHub tokens)

2. **Coding Agent Safety:** Should we sandbox coding agent execution in Docker containers?
   - **Recommendation:** Phase 1: workspace restrictions only; Phase 2: optional Docker sandboxing

3. **Summarization Quality:** Which LLM provider should be default for summarization?
   - **Recommendation:** Use user's configured primary model (typically Claude Sonnet)

4. **Rate Limiting:** Should rate limits be global or per-skill?
   - **Recommendation:** Per-skill to allow different limits for expensive operations

5. **YouTube Summarization:** yt-dlp vs YouTube API?
   - **Recommendation:** Start with yt-dlp (no API key required), add YouTube API as fallback

---

**Next Steps:**
1. Review and approve this specification
2. Proceed to `research.md` for API documentation and examples
3. Create `data-dictionary.md` for entity definitions
4. Develop `plan.md` for implementation approach
5. Break down into tasks in `tasks.md`
