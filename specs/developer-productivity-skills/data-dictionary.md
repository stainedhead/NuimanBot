# Developer Productivity Skills - Data Dictionary

**Feature Suite:** Developer Productivity Skills (Phase 5)
**Version:** 1.0
**Created:** 2026-02-07
**Status:** Planning

---

## Table of Contents

1. [Domain Entities](#domain-entities)
2. [Skill Definitions](#skill-definitions)
3. [Configuration Structures](#configuration-structures)
4. [Request/Response Types](#requestresponse-types)
5. [Error Types](#error-types)

---

## Domain Entities

### No New Domain Entities Required

This feature suite reuses existing domain entities from the MVP:
- `User` (from `internal/domain/user.go`)
- `Message` (from `internal/domain/message.go`)
- `Skill` interface (from `internal/domain/skill.go`)
- `Permission` (from `internal/domain/permission.go`)

**Rationale:** All five new skills implement the existing `Skill` interface and operate within the existing RBAC and conversation management systems. No new domain entities are needed.

---

## Skill Definitions

### Base Skill Interface (Existing)

```go
package domain

// Skill defines the interface for all skills in NuimanBot
type Skill interface {
    // Name returns the unique skill identifier
    Name() string

    // Description returns a human-readable description
    Description() string

    // InputSchema returns the JSON schema for parameters
    InputSchema() map[string]any

    // Execute runs the skill with given parameters
    Execute(ctx context.Context, params map[string]any) (*SkillResult, error)

    // RequiredPermissions returns permissions needed to use this skill
    RequiredPermissions() []Permission

    // Config returns the skill's specific configuration
    Config() SkillConfig
}

// SkillResult represents the output of a skill execution
type SkillResult struct {
    Output   string
    Metadata map[string]any
    Error    string  // Empty if successful
}

// SkillConfig defines configuration for a skill
type SkillConfig struct {
    Enabled bool
    APIKey  SecureString
    Env     map[string]string
    Params  map[string]interface{}
}
```

---

### Skill 1: GitHubSkill

**Package:** `internal/usecase/skill/github`

**Type Definition:**
```go
package github

import (
    "context"
    "nuimanbot/internal/domain"
)

// GitHubSkill implements GitHub operations via gh CLI
type GitHubSkill struct {
    config     domain.SkillConfig
    execSvc    ExecutorService
    rateLimiter *RateLimiter
}

// GitHubAction represents a GitHub operation
type GitHubAction string

const (
    ActionIssueList   GitHubAction = "issue_list"
    ActionIssueView   GitHubAction = "issue_view"
    ActionIssueCreate GitHubAction = "issue_create"
    ActionIssueComment GitHubAction = "issue_comment"
    ActionIssueClose  GitHubAction = "issue_close"
    ActionPRList      GitHubAction = "pr_list"
    ActionPRView      GitHubAction = "pr_view"
    ActionPRCreate    GitHubAction = "pr_create"
    ActionPRReview    GitHubAction = "pr_review"
    ActionPRMerge     GitHubAction = "pr_merge"
    ActionRepoView    GitHubAction = "repo_view"
    ActionWorkflowRun GitHubAction = "workflow_run"
)

// GitHubParams represents parameters for GitHub operations
type GitHubParams struct {
    Action GitHubAction       `json:"action"`
    Repo   string             `json:"repo,omitempty"`
    Number int                `json:"number,omitempty"`    // Issue/PR number
    Title  string             `json:"title,omitempty"`
    Body   string             `json:"body,omitempty"`
    State  string             `json:"state,omitempty"`     // "open", "closed", "all"
    Extra  map[string]any     `json:"extra,omitempty"`
}

// GitHubIssue represents a GitHub issue
type GitHubIssue struct {
    Number    int               `json:"number"`
    Title     string            `json:"title"`
    State     string            `json:"state"`
    Body      string            `json:"body"`
    Author    GitHubUser        `json:"author"`
    Labels    []GitHubLabel     `json:"labels"`
    CreatedAt string            `json:"createdAt"`
    UpdatedAt string            `json:"updatedAt"`
}

// GitHubPullRequest represents a GitHub pull request
type GitHubPullRequest struct {
    Number      int            `json:"number"`
    Title       string         `json:"title"`
    State       string         `json:"state"`
    Body        string         `json:"body"`
    Author      GitHubUser     `json:"author"`
    BaseBranch  string         `json:"baseRefName"`
    HeadBranch  string         `json:"headRefName"`
    IsDraft     bool           `json:"isDraft"`
    Mergeable   string         `json:"mergeable"`
    Reviews     []GitHubReview `json:"reviews"`
    CreatedAt   string         `json:"createdAt"`
}

// GitHubUser represents a GitHub user
type GitHubUser struct {
    Login string `json:"login"`
    Name  string `json:"name,omitempty"`
}

// GitHubLabel represents a GitHub label
type GitHubLabel struct {
    Name  string `json:"name"`
    Color string `json:"color"`
}

// GitHubReview represents a pull request review
type GitHubReview struct {
    Author GitHubUser `json:"author"`
    State  string     `json:"state"`  // "APPROVED", "CHANGES_REQUESTED", "COMMENTED"
    Body   string     `json:"body"`
}
```

**Input Schema:**
```go
func (s *GitHubSkill) InputSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "action": map[string]any{
                "type": "string",
                "enum": []string{
                    "issue_list", "issue_view", "issue_create", "issue_comment", "issue_close",
                    "pr_list", "pr_view", "pr_create", "pr_review", "pr_merge",
                    "repo_view", "workflow_run",
                },
                "description": "GitHub operation to perform",
            },
            "repo": map[string]any{
                "type":        "string",
                "description": "Repository in format 'owner/repo'",
                "pattern":     "^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$",
            },
            "number": map[string]any{
                "type":        "integer",
                "description": "Issue or PR number",
            },
            "title": map[string]any{
                "type":        "string",
                "description": "Title for new issue/PR",
            },
            "body": map[string]any{
                "type":        "string",
                "description": "Body text for issue/PR/comment",
            },
        },
        "required": []string{"action"},
    }
}
```

---

### Skill 2: RepoSearchSkill

**Package:** `internal/usecase/skill/reposearch`

**Type Definition:**
```go
package reposearch

// RepoSearchSkill implements codebase search via ripgrep
type RepoSearchSkill struct {
    config          domain.SkillConfig
    execSvc         ExecutorService
    allowedDirs     []string
    excludePatterns []string
}

// SearchParams represents search parameters
type SearchParams struct {
    Query        string `json:"query"`
    Path         string `json:"path,omitempty"`
    FileType     string `json:"file_type,omitempty"`
    MaxResults   int    `json:"max_results,omitempty"`
    ContextLines int    `json:"context_lines,omitempty"`
    CaseSensitive bool  `json:"case_sensitive,omitempty"`
}

// SearchResult represents a single search match
type SearchResult struct {
    File          string   `json:"file"`
    Line          int      `json:"line"`
    Column        int      `json:"column,omitempty"`
    Match         string   `json:"match"`
    ContextBefore []string `json:"context_before,omitempty"`
    ContextAfter  []string `json:"context_after,omitempty"`
}

// SearchResponse represents the complete search results
type SearchResponse struct {
    Results      []SearchResult `json:"results"`
    TotalMatches int            `json:"total_matches"`
    Truncated    bool           `json:"truncated"`
    SearchTime   float64        `json:"search_time_ms"`
}
```

---

### Skill 3: DocSummarizeSkill

**Package:** `internal/usecase/skill/docsummarize`

**Type Definition:**
```go
package docsummarize

// DocSummarizeSkill implements document summarization
type DocSummarizeSkill struct {
    config     domain.SkillConfig
    llmSvc     LLMService
    httpClient *http.Client
}

// DocSummarizeParams represents summarization parameters
type DocSummarizeParams struct {
    Source   string `json:"source"`    // File path, Git URL, or HTTP URL
    MaxWords int    `json:"max_words,omitempty"`
    Focus    string `json:"focus,omitempty"`
}

// DocumentSource represents the source of a document
type DocumentSource struct {
    Type      SourceType `json:"type"`       // "file", "git", "http"
    Location  string     `json:"location"`
    Content   string     `json:"content"`
    Metadata  SourceMeta `json:"metadata"`
}

// SourceType represents the type of document source
type SourceType string

const (
    SourceTypeFile SourceType = "file"
    SourceTypeGit  SourceType = "git"
    SourceTypeHTTP SourceType = "http"
)

// SourceMeta represents metadata about a document source
type SourceMeta struct {
    Title       string    `json:"title,omitempty"`
    Author      string    `json:"author,omitempty"`
    WordCount   int       `json:"word_count"`
    CreatedAt   time.Time `json:"created_at,omitempty"`
    ModifiedAt  time.Time `json:"modified_at,omitempty"`
}

// SummaryResult represents the summarization output
type SummaryResult struct {
    Summary    string   `json:"summary"`
    Source     string   `json:"source"`
    WordCount  int      `json:"word_count"`
    KeyTopics  []string `json:"key_topics,omitempty"`
    Timestamp  string   `json:"timestamp"`
}
```

---

### Skill 4: SummarizeSkill

**Package:** `internal/usecase/skill/summarize`

**Type Definition:**
```go
package summarize

// SummarizeSkill implements external URL/video summarization
type SummarizeSkill struct {
    config     domain.SkillConfig
    llmSvc     LLMService
    httpClient *http.Client
    ytExtractor YouTubeExtractor
}

// SummarizeParams represents summarization parameters
type SummarizeParams struct {
    URL           string       `json:"url"`
    Format        SummaryFormat `json:"format,omitempty"`
    IncludeQuotes bool         `json:"include_quotes,omitempty"`
}

// SummaryFormat represents the output format
type SummaryFormat string

const (
    FormatBrief        SummaryFormat = "brief"
    FormatDetailed     SummaryFormat = "detailed"
    FormatBulletPoints SummaryFormat = "bullet_points"
)

// ContentType represents the type of content
type ContentType string

const (
    ContentTypeArticle ContentType = "article"
    ContentTypeVideo   ContentType = "video"
    ContentTypePDF     ContentType = "pdf"
    ContentTypeUnknown ContentType = "unknown"
)

// SummarizeResult represents the summarization output
type SummarizeResult struct {
    Summary       string      `json:"summary"`
    Title         string      `json:"title"`
    Author        string      `json:"author,omitempty"`
    PublishedDate string      `json:"published_date,omitempty"`
    SourceType    ContentType `json:"source_type"`
    ReadingTime   string      `json:"reading_time,omitempty"`
    KeyQuotes     []string    `json:"key_quotes,omitempty"`
    URL           string      `json:"url"`
}

// YouTubeVideo represents YouTube video metadata
type YouTubeVideo struct {
    ID          string        `json:"id"`
    Title       string        `json:"title"`
    Description string        `json:"description"`
    Duration    int           `json:"duration"`  // seconds
    Uploader    string        `json:"uploader"`
    Transcript  string        `json:"transcript"`
}
```

---

### Skill 5: CodingAgentSkill

**Package:** `internal/usecase/skill/codingagent`

**Type Definition:**
```go
package codingagent

// CodingAgentSkill implements coding agent orchestration
type CodingAgentSkill struct {
    config     domain.SkillConfig
    execSvc    ExecutorService
    sessionMgr SessionManager
}

// CodingAgentParams represents parameters for coding agent
type CodingAgentParams struct {
    Tool      CodingTool `json:"tool"`
    Task      string     `json:"task"`
    Mode      AgentMode  `json:"mode,omitempty"`
    Workspace string     `json:"workspace,omitempty"`
    Timeout   int        `json:"timeout,omitempty"`  // seconds
}

// CodingTool represents the coding agent CLI tool
type CodingTool string

const (
    ToolCodex      CodingTool = "codex"
    ToolClaudeCode CodingTool = "claude_code"
    ToolOpenCode   CodingTool = "opencode"
    ToolGemini     CodingTool = "gemini"
    ToolCopilot    CodingTool = "copilot"
)

// AgentMode represents the approval mode
type AgentMode string

const (
    ModeInteractive AgentMode = "interactive"  // Prompt user for each approval
    ModeAuto        AgentMode = "auto"         // Auto-approve workspace edits
    ModeYOLO        AgentMode = "yolo"         // No approvals (high risk)
)

// CodingAgentSession represents a background session
type CodingAgentSession struct {
    ID          string     `json:"id"`
    Tool        CodingTool `json:"tool"`
    Task        string     `json:"task"`
    Status      SessionStatus `json:"status"`
    StartedAt   time.Time  `json:"started_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    Output      string     `json:"output"`
    Error       string     `json:"error,omitempty"`
}

// SessionStatus represents the status of a coding agent session
type SessionStatus string

const (
    StatusRunning   SessionStatus = "running"
    StatusCompleted SessionStatus = "completed"
    StatusFailed    SessionStatus = "failed"
    StatusCancelled SessionStatus = "cancelled"
)

// CodingAgentResult represents the result of a coding agent task
type CodingAgentResult struct {
    Status            SessionStatus `json:"status"`
    Output            string        `json:"output"`
    FilesModified     []string      `json:"files_modified"`
    SessionID         string        `json:"session_id"`
    Duration          float64       `json:"duration"`
    ApprovalsRequested int          `json:"approvals_requested,omitempty"`
    ApprovalsGranted  int          `json:"approvals_granted,omitempty"`
}

// ApprovalRequest represents a request for user approval
type ApprovalRequest struct {
    Type         string `json:"type"`  // "file_edit", "command_execute", etc.
    Description  string `json:"description"`
    FilePath     string `json:"file_path,omitempty"`
    Changes      string `json:"changes,omitempty"`
    Command      string `json:"command,omitempty"`
}
```

---

## Configuration Structures

### Skills System Configuration

```go
package domain

// SkillsSystemConfig defines global settings for the skill system (from MVP)
type SkillsSystemConfig struct {
    Entries map[string]SkillConfig // Individual skill configurations by ID
    Load    struct {
        ExtraDirs []string // Additional directories to scan for skills
        Watch     bool     // Watch skill folders for changes
    }
}
```

### Developer Productivity Skills Configuration

```yaml
skills:
  entries:
    github:
      enabled: true
      params:
        timeout: 30
        default_repo: ""  # Optional default repo

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
        max_file_size: 1048576  # 1MB

    doc_summarize:
      enabled: true
      params:
        timeout: 60
        max_document_size: 5242880  # 5MB
        allowed_domains:
          - github.com
          - docs.google.com
          - notion.so

    summarize:
      enabled: true
      params:
        timeout: 90
        youtube:
          api_key: ${YOUTUBE_API_KEY}
          max_duration: 3600  # 1 hour
        user_agent: "NuimanBot/1.0"

    coding_agent:
      enabled: false  # Admin must explicitly enable
      params:
        allowed_tools:
          - codex
          - claude_code
        default_mode: interactive
        pty_mode: true
        require_git_repo: true
        auto_mode_whitelist:
          - ./internal
          - ./cmd
```

---

## Request/Response Types

### Skill Execution Request (Reuses Existing)

```go
package usecase

// SkillExecutionRequest represents a request to execute a skill
type SkillExecutionRequest struct {
    UserID    string
    SkillName string
    Params    map[string]any
    Context   ExecutionContext
}

// ExecutionContext provides context for skill execution
type ExecutionContext struct {
    ConversationID string
    Platform       domain.Platform
    Workspace      string
}
```

### Skill Execution Response (Reuses Existing)

```go
// SkillExecutionResponse represents the response from skill execution
type SkillExecutionResponse struct {
    Result    *domain.SkillResult
    Metadata  map[string]any
    Duration  time.Duration
}
```

---

## Error Types

### Skill-Specific Errors

```go
package skill

import "errors"

var (
    // Input validation errors
    ErrInvalidAction     = errors.New("invalid action")
    ErrInvalidRepo       = errors.New("invalid repository format")
    ErrInvalidURL        = errors.New("invalid URL")
    ErrInvalidPath       = errors.New("invalid path")

    // Permission errors
    ErrPermissionDenied  = errors.New("permission denied")
    ErrRateLimitExceeded = errors.New("rate limit exceeded")

    // Execution errors
    ErrCommandFailed     = errors.New("command execution failed")
    ErrCommandTimeout    = errors.New("command execution timeout")
    ErrToolNotFound      = errors.New("external tool not found")
    ErrToolNotInstalled  = errors.New("external tool not installed")

    // Resource errors
    ErrFileTooLarge      = errors.New("file too large")
    ErrDocumentTooLarge  = errors.New("document too large")
    ErrVideoTooLong      = errors.New("video duration exceeds limit")

    // Security errors
    ErrPathTraversal     = errors.New("path traversal detected")
    ErrCommandInjection  = errors.New("command injection detected")
    ErrDomainNotAllowed  = errors.New("domain not in allowlist")
)

// SkillError wraps errors with skill context
type SkillError struct {
    SkillName string
    Operation string
    Err       error
}

func (e *SkillError) Error() string {
    return fmt.Sprintf("%s skill (%s): %v", e.SkillName, e.Operation, e.Err)
}

func (e *SkillError) Unwrap() error {
    return e.Err
}
```

---

## Service Interfaces

### ExecutorService

```go
package skill

// ExecutorService handles external command execution
type ExecutorService interface {
    // Execute runs a command with timeout and returns output
    Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error)

    // ExecuteBackground runs a command in the background
    ExecuteBackground(ctx context.Context, req ExecutionRequest) (*BackgroundSession, error)

    // GetSessionStatus returns the status of a background session
    GetSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error)
}

// ExecutionRequest represents a command execution request
type ExecutionRequest struct {
    Command      string
    Args         []string
    WorkingDir   string
    Env          map[string]string
    Timeout      time.Duration
    PTYMode      bool
}

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Duration time.Duration
}

// BackgroundSession represents a long-running background session
type BackgroundSession struct {
    ID        string
    StartedAt time.Time
}
```

### SessionManager

```go
package skill

// SessionManager manages background coding agent sessions
type SessionManager interface {
    // CreateSession creates a new background session
    CreateSession(ctx context.Context, params CodingAgentParams) (*CodingAgentSession, error)

    // GetSession retrieves a session by ID
    GetSession(ctx context.Context, sessionID string) (*CodingAgentSession, error)

    // ListSessions lists all active sessions for a user
    ListSessions(ctx context.Context, userID string) ([]*CodingAgentSession, error)

    // CancelSession cancels a running session
    CancelSession(ctx context.Context, sessionID string) error

    // GetOutput retrieves the current output of a session
    GetOutput(ctx context.Context, sessionID string) (string, error)
}
```

### YouTubeExtractor

```go
package skill

// YouTubeExtractor handles YouTube video metadata and transcript extraction
type YouTubeExtractor interface {
    // GetVideoInfo retrieves metadata for a YouTube video
    GetVideoInfo(ctx context.Context, videoID string) (*YouTubeVideo, error)

    // GetTranscript retrieves the transcript for a YouTube video
    GetTranscript(ctx context.Context, videoID string, lang string) (string, error)
}
```

---

## Database Schema Extensions

### No New Tables Required

This feature suite does not require new database tables. All data is:
- Stored in existing `messages` table (conversation history)
- Logged in existing `audit_events` table (security audit)
- Cached in memory (LLM response cache, rate limiters)

**Rationale:** Skills are stateless operations that produce outputs stored as messages. No persistent skill-specific state needed.

---

**Data Dictionary Complete:** Ready to proceed to `plan.md` for implementation approach and architecture decisions.
