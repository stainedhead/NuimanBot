# User Customization - Research

**Created:** 2026-02-15
**Source:** `Feature-Self-Organizing-Memory-PRD.md`
**Status:** In Progress

---

## Overview

Research for implementing per-user persona files (SOUL.md, USER.md, RULES.md) and explicit memory write mechanisms, inspired by OpenClaw's workspace memory design.

**Research Questions:**
1. How does OpenClaw handle system prompt bootstrap files (SOUL.md, USER.md)?
2. How does OpenClaw implement explicit memory writes (tool calls vs. implicit)?
3. What YAML frontmatter parsing libraries are available in Go?
4. What are best practices for path traversal prevention in file operations?
5. How should we structure the token budget for multi-file prompt injection?

**For full details, see the source PRD:** `Feature-Self-Organizing-Memory-PRD.md`

---

## Table of Contents

1. [OpenClaw Memory Design](#openclaw-memory-design)
2. [YAML Frontmatter Parsing](#yaml-frontmatter-parsing)
3. [Security Considerations](#security-considerations)
4. [Token Budget Management](#token-budget-management)
5. [File-Based Memory Patterns](#file-based-memory-patterns)
6. [Best Practices](#best-practices)

---

## OpenClaw Memory Design

### System Prompt Bootstrap Files

**Source:** `/opt/homebrew/lib/node_modules/openclaw/docs/concepts/system-prompt.md`

**Key Findings:**
- OpenClaw injects stable persona/context files into the system prompt ("bootstrap files")
- Files are Markdown-first: human-readable, editable, version-controllable
- Bootstrap files include: SOUL.md, USER.md, MEMORY.md
- Files are loaded and concatenated into the system prompt on every LLM request

**Pattern:**
```
System Prompt = [Global Policy] + [SOUL.md] + [USER.md] + [MEMORY.md] + [Operational Notes]
```

**Benefits:**
- Markdown is familiar to developers and non-developers
- Easy to version control (git)
- Auditable: see exactly what instructions the agent received
- No "magic": all context is explicit

**Applicability to Nuimanbot:**
- Adopt same file-based approach: SOUL.md, USER.md, RULES.md
- Use Markdown for readability
- Inject into SystemPrompt via PromptComposer service

---

### Explicit Memory Writes

**Source:** `/opt/homebrew/lib/node_modules/openclaw/docs/experiments/research/memory.md`

**Key Concept:**
> "Memory writes are explicit tool calls (append/replace/insert), persisted, then re-injected next turn"

**Approach:**
1. **Not Implicit**: Don't rely on "the model will remember" via context
2. **Tool Calls**: Memory updates are explicit actions (append, replace, insert)
3. **Persisted**: Writes are saved to files immediately
4. **Re-injected**: Updated content is loaded into next prompt

**Operations:**
- `memory.append(file, content)` - Append to end of file
- `memory.replace(file, old, new)` - Replace section
- `memory.insert(file, position, content)` - Insert at position

**Retain/Recall/Reflect Loop:**
1. **Retain**: Agent decides what to remember (explicit action)
2. **Recall**: Agent reads memory files on next request
3. **Reflect**: Agent can summarize/compress memory over time

**Applicability to Nuimanbot:**
- Implement internal actions: `memory.write_file`, `persona.update`
- Agent proposes memory writes, user confirms (default)
- All writes audited (timestamp, user, action, file)
- Allow user to disable confirmation via RULES.md

---

## YAML Frontmatter Parsing

### Library: gopkg.in/yaml.v3

**Documentation:** https://github.com/go-yaml/yaml
**Installation:** `go get gopkg.in/yaml.v3`

**What We Need:**
- Parse YAML frontmatter from Markdown files
- Frontmatter format: content between `---` delimiters at file start

**Example Parsing:**
```go
package main

import (
    "gopkg.in/yaml.v3"
    "strings"
)

type FrontMatter struct {
    RequiresConfirmation []string          `yaml:"requires_confirmation"`
    BlockedTools         []string          `yaml:"blocked_tools"`
    Privacy              map[string]interface{} `yaml:"privacy"`
}

func ParseMarkdownWithFrontmatter(content string) (FrontMatter, string, error) {
    var fm FrontMatter

    // Split on --- delimiters
    parts := strings.SplitN(content, "---", 3)
    if len(parts) < 3 {
        // No frontmatter, return empty struct and full content
        return fm, content, nil
    }

    yamlContent := parts[1]
    markdownContent := parts[2]

    err := yaml.Unmarshal([]byte(yamlContent), &fm)
    if err != nil {
        return fm, content, err
    }

    return fm, strings.TrimSpace(markdownContent), nil
}
```

**Error Handling:**
- If YAML parse fails, log error and use previous valid version
- Graceful degradation: soft rules only if frontmatter invalid
- Validate frontmatter on write (before saving)

**Applicability to Nuimanbot:**
- Use yaml.v3 for RULES.md frontmatter parsing
- Define RulesConfig struct for frontmatter schema
- Implement validation and graceful fallback

---

## Security Considerations

### Path Traversal Prevention

**Threat:** User-supplied file paths could escape user directory (e.g., `../../../etc/passwd`)

**Mitigation Strategies:**

#### 1. Path Allowlist (Recommended)
```go
package security

import (
    "path/filepath"
    "strings"
)

// ValidateUserPath ensures path is within user's data directory
func ValidateUserPath(userDataDir, requestedPath string) (string, error) {
    // Resolve to absolute paths
    absDataDir, err := filepath.Abs(userDataDir)
    if err != nil {
        return "", err
    }

    absRequestedPath, err := filepath.Abs(filepath.Join(userDataDir, requestedPath))
    if err != nil {
        return "", err
    }

    // Ensure requested path is within data directory
    if !strings.HasPrefix(absRequestedPath, absDataDir+string(filepath.Separator)) {
        return "", fmt.Errorf("path traversal detected: %s", requestedPath)
    }

    return absRequestedPath, nil
}
```

#### 2. Path Sanitization
```go
// SanitizePath removes dangerous characters and patterns
func SanitizePath(path string) string {
    // Remove ../ and ..\
    path = strings.ReplaceAll(path, "../", "")
    path = strings.ReplaceAll(path, "..\\", "")

    // Remove absolute path indicators
    path = strings.TrimPrefix(path, "/")
    path = strings.TrimPrefix(path, "\\")

    return path
}
```

#### 3. File Extension Allowlist
```go
var AllowedExtensions = []string{".md", ".txt", ".json"}

func ValidateExtension(path string) error {
    ext := filepath.Ext(path)
    for _, allowed := range AllowedExtensions {
        if ext == allowed {
            return nil
        }
    }
    return fmt.Errorf("file extension not allowed: %s", ext)
}
```

**Best Practices:**
- Always use absolute paths internally
- Never trust user-supplied paths
- Validate before every file operation (read/write)
- Log all path validation failures (security audit)
- Test with malicious inputs (e.g., `../../../etc/passwd`)

**Applicability to Nuimanbot:**
- Implement ValidateUserPath in infrastructure layer
- Call validation before every FileRepository operation
- Add security tests for path traversal attempts

---

## Token Budget Management

### Challenge
With multiple persona files (SOUL.md, USER.md, RULES.md), we could exceed token limits:
- SOUL.md: ~500-1000 tokens
- USER.md: ~500-1000 tokens
- RULES.md: ~500-1000 tokens
- **Total:** 1500-3000 tokens (before global policy, operational notes)

**LLM Context Limits:**
- GPT-4: 8k-32k tokens (depending on variant)
- Claude: 100k-200k tokens
- Gemini: 32k-1M tokens

**System Prompt Budget:**
- Target: <4000 tokens for all persona content
- Leaves room for: global policy, operational notes, conversation history

### Strategies

#### 1. Per-File Token Limits
```go
type TokenBudget struct {
    MaxTotal   int  // 4000 total
    MaxPerFile map[string]int
}

var DefaultBudget = TokenBudget{
    MaxTotal: 4000,
    MaxPerFile: map[string]int{
        "SOUL.md":  1500,
        "USER.md":  1500,
        "RULES.md": 1000,
    },
}
```

#### 2. Truncation with Markers
```go
func TruncateWithMarker(content string, maxTokens int) string {
    tokens := CountTokens(content)
    if tokens <= maxTokens {
        return content
    }

    // Truncate and add marker
    truncated := TruncateToTokens(content, maxTokens-10)
    return truncated + "\n\n[... truncated, see full file ...]"
}
```

#### 3. Priority-Based Truncation
Order of importance:
1. RULES.md (highest priority - safety/compliance)
2. SOUL.md (medium priority - persona)
3. USER.md (lowest priority - context)

If over budget, truncate in reverse order:
```go
func ApplyTokenBudget(files map[string]string, budget TokenBudget) map[string]string {
    priorityOrder := []string{"USER.md", "SOUL.md", "RULES.md"}

    totalTokens := 0
    for _, content := range files {
        totalTokens += CountTokens(content)
    }

    if totalTokens <= budget.MaxTotal {
        return files  // No truncation needed
    }

    // Truncate lower-priority files first
    for _, filename := range priorityOrder {
        if totalTokens <= budget.MaxTotal {
            break
        }

        content := files[filename]
        maxForFile := budget.MaxPerFile[filename]
        files[filename] = TruncateWithMarker(content, maxForFile)
        totalTokens = RecalculateTotal(files)
    }

    return files
}
```

#### 4. Caching
- Cache file content for 15 minutes (reduce disk I/O)
- Cache token counts (expensive to compute)
- Invalidate cache on file write

**Applicability to Nuimanbot:**
- Implement TokenBudget configuration
- Add truncation logic to PromptComposer
- Cache file content and token counts
- Log truncation events (monitoring)

---

## File-Based Memory Patterns

### Daily Memory Logs (Future Enhancement)

**Pattern from OpenClaw:**
```
memory/
├── MEMORY.md              # Summary/index
├── 2026-02-15.md          # Daily log
├── 2026-02-14.md
└── 2026-02-13.md
```

**Daily Log Format:**
```markdown
# 2026-02-15

## Summary
[1-2 sentence summary of the day]

## Key Events
- [Event 1]
- [Event 2]

## Decisions Made
- [Decision 1]: [Rationale]

## Lessons Learned
- [Lesson 1]
```

**Benefits:**
- Chronological record of interactions
- Easy to search/reference specific dates
- Can compress/summarize older logs

**Not in V1 scope**, but worth considering for future:
- memory.log_daily(bullet_point) internal action
- Automatic daily file creation
- Summarization/compression of old logs

---

## Best Practices

### Best Practice 1: Markdown-First Design

**Source:** OpenClaw design philosophy

**Description:**
Use Markdown for all persona/memory files:
- Human-readable
- Version-controllable (git)
- Familiar to developers and non-developers
- Easy to edit in any text editor

**Example:**
```markdown
# SOUL.md

You are a helpful, friendly assistant with expertise in software development.

## Voice & Tone
- Casual but professional
- Use emojis sparingly
- Explain concepts clearly

## Communication Style
- Be concise by default
- Expand when asked for details
- Use code examples when helpful
```

**Applicability:**
All persona files (SOUL.md, USER.md, RULES.md) use Markdown

---

### Best Practice 2: YAML Frontmatter for Machine-Enforceable Rules

**Source:** Jekyll, Hugo, other static site generators

**Description:**
Use YAML frontmatter for structured data that needs to be parsed and enforced by code:

**Example:**
```markdown
---
requires_confirmation:
  - external_message
  - credential_use
blocked_tools:
  - shell.exec
privacy:
  never_store:
    - api_keys
    - passwords
---

# Rules

Plain English rules follow frontmatter...
```

**Rationale:**
- Clear separation: machine-readable (YAML) vs. human-readable (Markdown)
- YAML is widely supported
- Frontmatter is familiar to developers

**Applicability:**
RULES.md uses frontmatter for hard enforcement

---

### Best Practice 3: Explicit Over Implicit

**Source:** OpenClaw memory research

**Description:**
Memory writes should be explicit actions, not implicit "magic":
- User says "remember this" → agent proposes memory write → user confirms
- Agent can suggest memory updates but doesn't write without permission
- All writes are audited

**Rationale:**
- Transparency: user sees exactly what's being remembered
- Control: user can review/approve before writing
- Auditability: clear record of all memory changes

**Example Workflow:**
```
User: "Remember that I prefer concise answers"
Agent: "I'll update your USER.md with this preference. Confirm?"
User: "Yes"
Agent: [Executes memory.write_file action]
Agent: "Updated USER.md. You can review it anytime."
```

**Applicability:**
MemoryWriter service requires confirmation by default (unless user disables via RULES.md)

---

## Open Questions

### Question 1: File Storage Location

**Context:** Where should per-user persona files live?

**Options:**
- Option A: `UserProfile.DataDirectory` (recommended)
  - Pros: Already modeled, per-user isolation, clear ownership
  - Cons: Need to ensure DataDirectory is created for all users
- Option B: Global directory keyed by user ID (e.g., `/data/persona/{userId}/`)
  - Pros: Centralized storage, easier backup
  - Cons: Less flexible, doesn't leverage existing UserProfile model

**Research Needed:**
- Verify UserProfile.DataDirectory is populated for all users
- Check if DataDirectory is created on user creation

**Decision:** Use UserProfile.DataDirectory (Option A)

**Rationale:** Leverage existing model, clear per-user isolation, follows Clean Architecture principles

---

### Question 2: Cross-Platform User Mapping

**Context:** Should Slack/Telegram users share a single profile?

**Options:**
- Option A: Shared profile (map Slack/Telegram to internal user ID)
  - Pros: Consistent persona across platforms
  - Cons: Need to implement user mapping logic
- Option B: Separate profiles per platform
  - Pros: Simpler implementation
  - Cons: Fragmented user experience (repeat onboarding per platform)

**Research Needed:**
- Current user model: does it support multi-platform mapping?
- User expectations: do users want shared or separate personas?

**Decision:** Shared profile (Option A) - recommended

**Rationale:** Better user experience, consistent persona across platforms

---

### Question 3: LLM Autonomous Persona Edits

**Context:** Should the LLM be able to edit persona files autonomously?

**Options:**
- Option A: Require confirmation for all persona edits (recommended)
  - Pros: User control, transparency, prevents unwanted changes
  - Cons: More friction, requires user interaction
- Option B: Allow autonomous edits for "safe" changes
  - Pros: Less friction, more convenient
  - Cons: Risk of unwanted changes, less transparency

**Research Needed:**
- User preferences: how much control do they want?
- Safety: what could go wrong with autonomous edits?

**Decision:** Require confirmation by default (Option A)

**Rationale:** Safety and transparency are priorities. Users can opt-in to autonomous edits via RULES.md if desired.

---

## References

### Documentation
- OpenClaw System Prompt: `/opt/homebrew/lib/node_modules/openclaw/docs/concepts/system-prompt.md`
- OpenClaw Memory Research: `/opt/homebrew/lib/node_modules/openclaw/docs/experiments/research/memory.md`
- YAML v3 Go Library: https://github.com/go-yaml/yaml
- Jekyll Frontmatter: https://jekyllrb.com/docs/front-matter/

### Articles/Blog Posts
- Clean Architecture: https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- OWASP Path Traversal: https://owasp.org/www-community/attacks/Path_Traversal

### GitHub Repositories
- gopkg.in/yaml.v3: https://github.com/go-yaml/yaml

---

## Research Summary

**Key Findings:**
1. OpenClaw's file-based memory design is proven and user-friendly
2. Explicit memory writes (tool calls) are more transparent than implicit
3. YAML frontmatter is ideal for machine-enforceable rules
4. Path traversal prevention requires strict allowlist validation
5. Token budget management needs priority-based truncation

**Decisions Made:**
1. Adopt file-based memory (SOUL.md, USER.md, RULES.md)
2. Use YAML frontmatter for RULES.md hard enforcement
3. Implement explicit memory writes (internal actions)
4. Use UserProfile.DataDirectory for file storage
5. Require confirmation for memory writes by default

**Next Steps:**
1. Create data-dictionary.md (define all entities and types)
2. Create architecture.md (sequence diagrams, component design)
3. Create plan.md (implementation strategy)
4. Create tasks.md (detailed task breakdown)

**Open Items:**
- Verify UserProfile.DataDirectory is created for all users
- Determine cross-platform user mapping strategy
- Define token counting implementation (library or API)
