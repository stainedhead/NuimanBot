# Developer Productivity Skills - Research

**Feature Suite:** Developer Productivity Skills (Phase 5)
**Version:** 1.0
**Created:** 2026-02-07
**Status:** Research Phase

---

## Table of Contents

1. [External Tool APIs](#external-tool-apis)
2. [Reference Implementations](#reference-implementations)
3. [Go Libraries](#go-libraries)
4. [Best Practices](#best-practices)
5. [Risks and Mitigations](#risks-and-mitigations)

---

## External Tool APIs

### 1. GitHub CLI (`gh`)

**Installation:**
```bash
# macOS
brew install gh

# Linux (Debian/Ubuntu)
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install gh

# Go binary download
# Download from https://github.com/cli/cli/releases
```

**Authentication:**
```bash
# Interactive login
gh auth login

# Check status
gh auth status

# Token-based (for automation)
export GH_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxx"
gh auth status
```

**Core Commands:**

**Issues:**
```bash
# List issues
gh issue list --repo owner/repo --state open --json number,title,labels

# View issue
gh issue view 123 --repo owner/repo --json title,body,comments

# Create issue
gh issue create --repo owner/repo --title "Bug: Something broke" --body "Description here"

# Comment on issue
gh issue comment 123 --repo owner/repo --body "Investigating this"

# Close issue
gh issue close 123 --repo owner/repo
```

**Pull Requests:**
```bash
# List PRs
gh pr list --repo owner/repo --state open --json number,title,author

# View PR
gh pr view 456 --repo owner/repo --json title,body,commits,reviews

# Create PR
gh pr create --repo owner/repo --title "feat: Add feature" --body "Description" --base main --head feature-branch

# Review PR
gh pr review 456 --repo owner/repo --approve
gh pr review 456 --repo owner/repo --request-changes --body "Please fix X"

# Merge PR
gh pr merge 456 --repo owner/repo --squash --delete-branch
```

**Repositories:**
```bash
# View repo
gh repo view owner/repo --json name,description,stargazerCount

# Clone repo
gh repo clone owner/repo

# Fork repo
gh repo fork owner/repo

# Create repo
gh repo create my-new-repo --public --description "My new repo"
```

**Workflows (GitHub Actions):**
```bash
# List workflow runs
gh run list --repo owner/repo --workflow ci.yml --json databaseId,status,conclusion

# View workflow run
gh run view 123456 --repo owner/repo --json jobs

# Trigger workflow
gh workflow run ci.yml --repo owner/repo --ref main
```

**Output Format:**
- Text (default): Human-readable table format
- JSON (`--json field1,field2`): Machine-parseable JSON
- Template (`--template`): Go template formatting

**Example JSON Output:**
```json
{
  "number": 123,
  "title": "Bug: Login fails for new users",
  "state": "open",
  "labels": [
    {"name": "bug", "color": "d73a4a"},
    {"name": "priority:high", "color": "ff0000"}
  ],
  "author": {"login": "johndoe"},
  "createdAt": "2026-02-01T10:30:00Z"
}
```

---

### 2. Ripgrep (`rg`)

**Installation:**
```bash
# macOS
brew install ripgrep

# Linux (Debian/Ubuntu)
sudo apt install ripgrep

# Go binary download
# Download from https://github.com/BurntSushi/ripgrep/releases
```

**Core Commands:**

**Content Search:**
```bash
# Basic search
rg "pattern" /path/to/search

# Case-insensitive
rg -i "pattern" /path/to/search

# Regex search
rg "func\s+\w+\(" /path/to/search

# File type filtering
rg "pattern" --type go /path/to/search

# Exclude patterns
rg "pattern" --glob "!*.test.go" /path/to/search

# Context lines
rg "pattern" -C 2 /path/to/search  # 2 lines before and after

# JSON output
rg "pattern" --json /path/to/search
```

**Example JSON Output:**
```json
{"type":"begin","data":{"path":{"text":"internal/domain/user.go"}}}
{"type":"match","data":{"path":{"text":"internal/domain/user.go"},"lines":{"text":"type User struct {\n"},"line_number":42,"absolute_offset":1024,"submatches":[{"match":{"text":"User"},"start":5,"end":9}]}}
{"type":"context","data":{"path":{"text":"internal/domain/user.go"},"lines":{"text":"\tID string\n"},"line_number":43,"absolute_offset":1047,"submatches":[]}}
{"type":"end","data":{"path":{"text":"internal/domain/user.go"},"stats":{"elapsed":{"secs":0,"nanos":1245678},"searches":1,"searches_with_match":1}}}
```

**Performance:**
- Typical repo (<100k LOC): <500ms
- Large repo (500k LOC): 1-2s
- Very large repo (1M+ LOC): 3-5s

**File Type Support:**
- Auto-detection based on extension
- Custom types: `--type-add 'customgo:*.{go,mod}' --type customgo`

---

### 3. yt-dlp (YouTube Downloader)

**Installation:**
```bash
# Python pip
pip install yt-dlp

# macOS
brew install yt-dlp

# Binary download
# Download from https://github.com/yt-dlp/yt-dlp/releases
```

**Core Commands:**

**Get Video Info:**
```bash
# Video metadata
yt-dlp --dump-json https://www.youtube.com/watch?v=VIDEO_ID

# Video title
yt-dlp --get-title https://www.youtube.com/watch?v=VIDEO_ID

# Duration
yt-dlp --get-duration https://www.youtube.com/watch?v=VIDEO_ID
```

**Extract Subtitles/Transcript:**
```bash
# Download auto-generated subtitles
yt-dlp --write-auto-sub --sub-lang en --skip-download --sub-format vtt https://www.youtube.com/watch?v=VIDEO_ID

# Convert subtitles to plain text
yt-dlp --write-auto-sub --sub-lang en --skip-download --sub-format txt https://www.youtube.com/watch?v=VIDEO_ID
```

**Example Metadata JSON:**
```json
{
  "id": "VIDEO_ID",
  "title": "Introduction to Go Programming",
  "description": "Learn the basics of Go...",
  "uploader": "TechChannel",
  "duration": 1234,
  "upload_date": "20260201",
  "view_count": 50000,
  "like_count": 1500,
  "categories": ["Education"],
  "tags": ["golang", "programming", "tutorial"]
}
```

**Subtitle Extraction:**
- Format: VTT (WebVTT) or plain text
- Languages: Auto-detect or specify (`--sub-lang en`)
- Quality: Auto-generated (ASR) or manual (if available)

---

### 4. Claude Code CLI (Anthropic)

**Installation:**
```bash
# Binary download from Anthropic
# https://claude.com/claude-code

# Verify installation
claude-code --version
```

**Authentication:**
```bash
# Login (opens browser)
claude-code auth login

# Check status
claude-code auth status

# Logout
claude-code auth logout
```

**Core Commands:**

**Run Task:**
```bash
# Interactive mode (default)
claude-code "Add input validation to User entity"

# Auto-edit mode (auto-approve edits in workspace)
claude-code --approval-mode auto_edit "Add input validation to User entity"

# Plan mode (create plan, no edits)
claude-code --approval-mode plan "Add input validation to User entity"

# YOLO mode (no approvals, high risk)
claude-code --yolo "Add input validation to User entity"

# Specify directory
claude-code --add-dir ./internal/domain "Add input validation to User entity"
```

**Operational Notes (from OpenClaw):**
- **PTY Mode Required:** Some CLIs hang without PTY (pseudo-terminal)
- **Background Sessions:** Long tasks should run in background with log tailing
- **Input Submission:** Send input via write/submit for approval prompts
- **Git Requirement:** Claude Code requires a git repository (initialize temp repo if needed)
- **Safety Modes:**
  - `auto_edit`: Auto-approve edits within workspace (safe for isolated projects)
  - `plan`: Create plan only, no code changes (safest)
  - `yolo`: No approvals, high risk (admin only)

**Example Interaction:**
```
$ claude-code "Add input validation to User entity"
I'll help you add input validation to the User entity.

Let me first read the User entity definition...

[Reads internal/domain/user.go]

I recommend adding a Validate() method with the following checks:
- Username: non-empty, 3-50 characters, alphanumeric
- Email: valid email format
- Role: must be "admin" or "user"

Should I proceed with these changes? [y/N]: y

Creating internal/domain/user.go:Validate()...
Running tests...

✓ All tests pass
✓ Task completed successfully

Files modified:
- internal/domain/user.go (+15 lines)
- internal/domain/user_test.go (+30 lines)
```

---

### 5. Codex CLI (Codex AI)

**Installation:**
```bash
# npm global install
npm install -g @codexai/cli

# Verify installation
codex --version
```

**Authentication:**
```bash
# Login with API key
codex auth login --api-key YOUR_API_KEY

# Check status
codex auth status
```

**Core Commands:**

**Run Task:**
```bash
# Interactive mode
codex "Implement user authentication"

# Specify files to edit
codex --files user.go,auth.go "Add JWT token generation"

# Full auto mode (workspace only)
codex --full-auto "Implement user authentication"

# Specify working directory
codex --cwd ./internal/domain "Add validation methods"
```

**Operational Notes:**
- **Git Requirement:** Codex requires a git repository (won't work in non-git directories)
- **Workspace Safety:** `--full-auto` only auto-approves changes within workspace
- **File Scope:** Can specify files to limit scope of changes
- **PTY Mode:** Similar to Claude Code, benefits from PTY for interactive prompts

---

## Reference Implementations

### OpenClaw Skills

**Location:** `examples/HomeBots/openclaw/skills/`

**Key Implementations:**

**1. `github` skill** (`openclaw/skills/github/SKILL.md`)
- Uses `gh` CLI with `--json` output
- Supports issues, PRs, repos, workflows
- Implements retry logic for API rate limits
- Sanitizes GitHub URLs in output

**2. `summarize` skill** (`openclaw/skills/summarize/SKILL.md`)
- Fetches URLs with requests library
- Extracts main content with BeautifulSoup (Python equivalent: use goquery in Go)
- YouTube support with yt-dlp for transcript extraction
- LLM summarization with provider abstraction
- Caches summaries (URL hash as key)

**3. `coding-agent` skill** (`openclaw/skills/coding-agent/SKILL.md`)
- PTY mode for interactive CLIs
- Background session management for long tasks
- Input/output handling via read/write operations
- Approval workflow for code changes
- Safety modes: interactive, auto, yolo

**Key Learnings:**
- PTY mode critical for interactive CLIs (prevents hangs)
- Background sessions essential for tasks >30s
- Approval prompts must be captured and forwarded to user
- Git repo requirement should be enforced (create temp repo if needed)

---

### NanoBot Skills

**Location:** `examples/HomeBots/nanobot/nanobot/skills/`

**Key Implementations:**

**1. `github` skill** (`nanobot/skills/github/SKILL.md`)
- Similar to OpenClaw but with different parameter structure
- Uses JSON schema for strict input validation
- Implements GitHub webhook listener (out of scope for NuimanBot MVP)

**2. `cron` skill** (`nanobot/skills/cron/SKILL.md`)
- Scheduled task execution (future Phase 6)
- Uses cron syntax for scheduling
- Persists scheduled tasks to database

---

### Internal Proposals

**Location:** `BOT_TOOL_IDEAS.md` (root)

**Key Ideas:**

**1. `repo_search` skill**
- Fast codebase search with ripgrep
- File type filtering
- Context lines for better understanding
- Workspace restriction for security

**2. `doc_summarize` skill**
- Internal documentation summarization
- Supports Markdown, text, HTML, PDF
- LLM-based summarization
- Metadata extraction (topics, key points)

---

## Go Libraries

### 1. Command Execution

**Standard Library: `os/exec`**

```go
import "os/exec"

// Execute command with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

cmd := exec.CommandContext(ctx, "gh", "issue", "list", "--repo", "owner/repo", "--json", "number,title")
output, err := cmd.CombinedOutput()
if err != nil {
    return fmt.Errorf("command failed: %w", err)
}

// Parse JSON output
var issues []Issue
if err := json.Unmarshal(output, &issues); err != nil {
    return fmt.Errorf("failed to parse output: %w", err)
}
```

**Error Handling:**
```go
// Check for exit code
if exitErr, ok := err.(*exec.ExitError); ok {
    stderr := string(exitErr.Stderr)
    return fmt.Errorf("command failed with exit code %d: %s", exitErr.ExitCode(), stderr)
}
```

---

### 2. JSON Parsing

**Standard Library: `encoding/json`**

```go
import "encoding/json"

type GitHubIssue struct {
    Number int    `json:"number"`
    Title  string `json:"title"`
    State  string `json:"state"`
    Labels []struct {
        Name  string `json:"name"`
        Color string `json:"color"`
    } `json:"labels"`
}

func parseIssues(data []byte) ([]GitHubIssue, error) {
    var issues []GitHubIssue
    if err := json.Unmarshal(data, &issues); err != nil {
        return nil, err
    }
    return issues, nil
}
```

---

### 3. Regular Expressions

**Standard Library: `regexp`**

```go
import "regexp"

// Validate GitHub repo format (owner/repo)
var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)

func validateRepo(repo string) bool {
    return repoPattern.MatchString(repo)
}

// Extract URL from text
var urlPattern = regexp.MustCompile(`https?://[^\s]+`)

func extractURLs(text string) []string {
    return urlPattern.FindAllString(text, -1)
}
```

---

### 4. HTTP Client (for Web Fetching)

**Standard Library: `net/http`**

```go
import (
    "io"
    "net/http"
    "time"
)

func fetchURL(url string, timeout time.Duration) ([]byte, error) {
    client := &http.Client{
        Timeout: timeout,
    }

    resp, err := client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch URL: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    // Limit response size (10MB)
    limitReader := io.LimitReader(resp.Body, 10*1024*1024)
    body, err := io.ReadAll(limitReader)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    return body, nil
}
```

---

### 5. HTML Parsing (for Web Scraping)

**Third-Party: `github.com/PuerkitoBio/goquery`**

```go
import (
    "github.com/PuerkitoBio/goquery"
    "strings"
)

func extractMainContent(htmlBody string) (string, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
    if err != nil {
        return "", err
    }

    // Remove scripts, styles, nav, footer
    doc.Find("script, style, nav, footer, aside").Remove()

    // Extract main content
    mainContent := doc.Find("article, main, .content").First().Text()
    if mainContent == "" {
        // Fallback to body
        mainContent = doc.Find("body").Text()
    }

    // Clean up whitespace
    mainContent = strings.TrimSpace(mainContent)
    return mainContent, nil
}
```

---

## Best Practices

### 1. Command Execution Safety

**Always use context with timeout:**
```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

cmd := exec.CommandContext(ctx, "gh", "issue", "list", ...)
```

**Validate command arguments:**
```go
// BAD: Vulnerable to command injection
cmd := exec.Command("sh", "-c", "gh issue list --repo " + userRepo)

// GOOD: Pass arguments separately
cmd := exec.Command("gh", "issue", "list", "--repo", userRepo)
```

**Sanitize error messages:**
```go
// Remove potential secrets from stderr
stderr := sanitizeOutput(string(exitErr.Stderr))
```

---

### 2. Input Validation

**Schema Validation:**
```go
func validateGitHubSkillInput(params map[string]any) error {
    action, ok := params["action"].(string)
    if !ok {
        return fmt.Errorf("missing or invalid 'action' parameter")
    }

    validActions := map[string]bool{
        "issue_list": true,
        "issue_create": true,
        "pr_list": true,
        // ...
    }

    if !validActions[action] {
        return fmt.Errorf("invalid action: %s", action)
    }

    // Validate repo format if present
    if repo, ok := params["repo"].(string); ok {
        if !validateRepo(repo) {
            return fmt.Errorf("invalid repo format: %s (expected owner/repo)", repo)
        }
    }

    return nil
}
```

---

### 3. Output Sanitization

**Redact Secrets:**
```go
var secretPatterns = []*regexp.Regexp{
    regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),  // GitHub token
    regexp.MustCompile(`sk-[a-zA-Z0-9]{48}`),   // OpenAI key
    regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35}`), // Google API key
}

func sanitizeOutput(output string) string {
    for _, pattern := range secretPatterns {
        output = pattern.ReplaceAllString(output, "[REDACTED]")
    }
    return output
}
```

---

### 4. Rate Limiting

**Per-Skill Rate Limiter:**
```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
}

func (r *RateLimiter) Allow(skillName, userID string) bool {
    r.mu.Lock()
    defer r.mu.Unlock()

    key := skillName + ":" + userID
    limiter, exists := r.limiters[key]
    if !exists {
        // 30 requests per minute
        limiter = rate.NewLimiter(rate.Every(2*time.Second), 1)
        r.limiters[key] = limiter
    }

    return limiter.Allow()
}
```

---

## Risks and Mitigations

### Risk 1: Command Injection

**Scenario:** Malicious user crafts input to execute arbitrary commands

**Example:**
```
User input: repo = "owner/repo; rm -rf /"
Command: gh issue list --repo owner/repo; rm -rf /
```

**Mitigation:**
- Use `exec.Command()` with separate arguments (never `sh -c`)
- Validate all inputs against strict patterns
- Reject inputs containing shell metacharacters (`;`, `|`, `&`, etc.)

**Implementation:**
```go
// Validate repo format strictly
if !regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`).MatchString(repo) {
    return fmt.Errorf("invalid repo format")
}

// Use separate arguments
cmd := exec.Command("gh", "issue", "list", "--repo", repo)
```

---

### Risk 2: Path Traversal

**Scenario:** Malicious user accesses files outside workspace

**Example:**
```
User input: path = "../../etc/passwd"
Search: rg "pattern" ../../etc/passwd
```

**Mitigation:**
- Validate paths against allowed workspace directories
- Reject paths containing `../` or absolute paths outside workspace
- Use `filepath.Clean()` and `filepath.Abs()` for canonicalization

**Implementation:**
```go
func validatePath(path string, allowedDirs []string) error {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("invalid path: %w", err)
    }

    // Check if path is within allowed directories
    for _, allowedDir := range allowedDirs {
        absAllowedDir, _ := filepath.Abs(allowedDir)
        if strings.HasPrefix(absPath, absAllowedDir) {
            return nil
        }
    }

    return fmt.Errorf("path outside allowed workspace: %s", path)
}
```

---

### Risk 3: Secrets Exposure

**Scenario:** Search results or error messages contain API keys, tokens

**Example:**
```
Search result: export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
Error message: Authentication failed for token sk_xxxxxxxxx
```

**Mitigation:**
- Scan all outputs for secret patterns
- Redact matching patterns with `[REDACTED]`
- Log original (unredacted) output to secure audit log only

**Implementation:**
```go
func sanitizeForUser(output string) string {
    return sanitizeOutput(output)  // Redact secrets
}

func logToAudit(output string) {
    // Log full output (unredacted) to secure audit log
    auditLogger.Log(output)
}
```

---

### Risk 4: Resource Exhaustion

**Scenario:** Malicious user triggers expensive operations repeatedly

**Example:**
```
User spams: "Summarize https://huge-pdf.com/file.pdf" (100MB PDF)
Result: Memory exhaustion, service degradation
```

**Mitigation:**
- Rate limiting: Max N operations per user per hour
- Size limits: Max file size, max response size
- Timeouts: All operations must complete within configured timeout
- Queueing: Serialize expensive operations per user

**Implementation:**
```go
// Check rate limit
if !rateLimiter.Allow(skillName, userID) {
    return fmt.Errorf("rate limit exceeded, try again later")
}

// Enforce size limit
if fileSize > maxFileSize {
    return fmt.Errorf("file too large: %d bytes (max %d)", fileSize, maxFileSize)
}

// Set timeout
ctx, cancel := context.WithTimeout(ctx, skillTimeout)
defer cancel()
```

---

### Risk 5: Coding Agent RCE

**Scenario:** Coding agent executes arbitrary code via crafted task description

**Example:**
```
User input (admin): task = "Delete all files; $(rm -rf /)"
Coding agent: Executes malicious commands
```

**Mitigation:**
- Admin-only permission by default
- Workspace restriction: Jail coding agent to workspace directory
- Approval workflow: Require explicit user approval for each file change
- Mode restrictions: YOLO mode requires explicit admin configuration
- Audit logging: Log all coding agent operations

**Implementation:**
```go
// Check admin permission
if user.Role != domain.RoleAdmin {
    return fmt.Errorf("coding_agent skill requires admin role")
}

// Workspace jail
if !isWithinWorkspace(workingDir, allowedWorkspace) {
    return fmt.Errorf("working directory outside allowed workspace")
}

// Approval workflow (interactive mode)
if mode == "interactive" {
    approval := promptUserApproval(fileChanges)
    if !approval {
        return fmt.Errorf("user denied file changes")
    }
}
```

---

**Research Complete:** Ready to proceed to `data-dictionary.md` for entity definitions.
