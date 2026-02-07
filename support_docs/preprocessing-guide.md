# Preprocessing Guide

## Overview

**Preprocessing** allows skills to embed shell commands that execute during skill rendering, providing dynamic, real-time information in skill prompts. This enables skills to fetch current project status, git information, file listings, and other live data.

## What is Preprocessing?

Preprocessing commands (marked with `!command`) run **before** the skill is executed by the LLM, inserting their output directly into the skill prompt. Think of it as "macro expansion" for skills.

### Example

**Skill with preprocessing:**
```markdown
# Current Status

Git status:

!command
git status --short

Recent commits:

!command
git log --oneline --max-count=5
```

**After preprocessing (what the LLM sees):**
```markdown
# Current Status

Git status:

```
M internal/domain/skill.go
?? new_file.go
```

Recent commits:

```
a1b2c3d Add preprocessing support
d4e5f6g Update documentation
g7h8i9j Fix bug in parser
```
```

The LLM receives **actual current data**, not static text.

## Creating Preprocessing Skills

### Basic Syntax

Use the `!command` marker followed by a shell command:

```markdown
!command
<your-command-here>
```

**Rules:**
- `!command` must be on its own line
- Command starts on the next line
- Empty line ends the command block
- Output is inserted as a code block

### Multi-Line Commands

Use backslash for line continuation:

```markdown
!command
git log \
  --oneline \
  --max-count=10 \
  --since="1 week ago"
```

### Multiple Commands

Include as many command blocks as needed:

```markdown
# Project Overview

Branch:

!command
git branch --show-current

Status:

!command
git status --short

Pull Requests:

!command
gh pr list --limit 5
```

## Allowed Commands

For security, only whitelisted commands are permitted:

| Command | Purpose | Example |
|---------|---------|---------|
| **git** | Git operations | `git status`, `git log`, `git diff` |
| **gh** | GitHub CLI | `gh pr list`, `gh issue list` |
| **ls** | List files | `ls -la`, `ls src/` |
| **cat** | Read files | `cat README.md`, `cat package.json` |
| **grep** | Search text | `grep TODO *.go`, `grep -r "pattern" src/` |

**Prohibited:**
- `rm`, `mv`, `cp` - File modification
- `curl`, `wget` - Network requests
- `bash`, `sh` - Shell execution
- `npm`, `make` - Build commands
- Pipes (`|`), redirects (`>`, `<`), command substitution (`$()`, `` ` ``)

## Security Constraints

Preprocessing runs in a **sandboxed environment** with strict security controls:

### Command Validation

All commands are validated before execution:

```go
// ✅ Allowed
git status
ls -la /tmp
grep TODO *.go

// ❌ Blocked - not whitelisted
rm -rf /
curl https://api.example.com
npm install

// ❌ Blocked - shell metacharacters
git log | grep "pattern"
ls $(whoami)
cat file > output.txt
```

### Resource Limits

| Limit | Value | Purpose |
|-------|-------|---------|
| **Timeout** | 5 seconds | Prevents hanging commands |
| **Output Size** | 10 KB | Prevents memory issues |
| **Filesystem** | Read-only | No file modifications |

Commands exceeding these limits are terminated automatically.

### Error Handling

Failed commands don't crash skills - errors are captured and displayed:

```markdown
!command
cat /nonexistent/file.txt
```

**Result:**
```
ERROR: Command failed with exit code 1
cat: /nonexistent/file.txt: No such file or directory
```

## Use Cases

### 1. Project Status Skills

Show current project state:

```markdown
---
name: project-status
description: Current project overview
user-invocable: true
---

# Project Status

## Git Status

!command
git status --short

## Recent Activity

!command
git log --oneline --max-count=10

## Open Issues

!command
gh issue list --limit 5
```

### 2. Code Analysis Skills

Analyze codebase:

```markdown
---
name: analyze-todos
description: Find all TODO comments
user-invocable: true
---

# TODO Analysis

All TODO comments:

!command
grep -r "TODO" --include="*.go" .

Count by file:

!command
grep -r "TODO" --include="*.go" . | cut -d: -f1 | sort | uniq -c
```

### 3. Environment Check Skills

Verify environment:

```markdown
---
name: env-check
description: Check development environment
user-invocable: true
---

# Environment Check

## Go Version

!command
git describe --tags --abbrev=0

## Dependencies

!command
cat go.mod
```

## Combining with Arguments

Preprocessing works seamlessly with argument substitution:

```markdown
---
name: file-status
description: Check status of specific file
user-invocable: true
---

# Status for: $0

## Git Log

!command
git log --oneline --max-count=5 -- $0

## Current Content

!command
cat $0
```

**Invocation:**
```bash
./bin/nuimanbot skill execute file-status README.md
```

**Processing Order:**
1. Argument substitution: `$0` → `README.md`
2. Preprocessing: Execute `git log ... -- README.md` and `cat README.md`
3. LLM receives final rendered prompt

## Best Practices

### 1. Keep Commands Fast

Commands timeout after 5 seconds - use fast operations:

```markdown
✅ Fast
!command
git status --short

!command
ls src/

❌ Slow (may timeout)
!command
git log --all --since="1 year ago"

!command
grep -r "pattern" /
```

### 2. Handle Large Output

Output is truncated at 10 KB - limit results:

```markdown
✅ Limited output
!command
git log --oneline --max-count=10

!command
ls -1 | head -20

❌ Potentially huge
!command
git log --all

!command
cat /dev/urandom
```

### 3. Provide Context

Explain what commands are fetching:

```markdown
✅ Clear context
## Recent Commits

!command
git log --oneline --max-count=5

❌ No context
!command
git log --oneline --max-count=5
```

### 4. Use Specific Paths

Avoid glob expansions that might match too much:

```markdown
✅ Specific
!command
grep TODO src/main.go

!command
ls internal/domain/

❌ Too broad
!command
grep TODO *

!command
ls
```

## Troubleshooting

### Command Blocked

**Problem**: "command not in whitelist"

**Solution**: Only use allowed commands (git, gh, ls, cat, grep)

```markdown
❌ Blocked
!command
npm list

✅ Alternative
!command
cat package.json
```

### Command Times Out

**Problem**: "command timed out"

**Solution**: Limit scope or use faster alternatives

```markdown
❌ Too slow
!command
git log --all

✅ Fast
!command
git log --max-count=10
```

### Shell Metacharacters Rejected

**Problem**: "command contains shell metacharacters"

**Solution**: Avoid pipes, redirects, substitutions

```markdown
❌ Rejected
!command
git log | grep "fix"

✅ Use grep directly
!command
grep "fix" .git/logs/HEAD
```

### Output Truncated

**Problem**: "(output truncated)" message

**Solution**: Limit results with command flags

```markdown
❌ Truncated
!command
git log

✅ Limited
!command
git log --max-count=20
```

## Examples

### Complete Project Status Skill

```markdown
---
name: project-status
description: Comprehensive project status report
user-invocable: true
---

# Project Status Report

## Current Branch

!command
git branch --show-current

## Uncommitted Changes

!command
git status --short

## Recent Commits (Last 10)

!command
git log --oneline --max-count=10

## Open Pull Requests

!command
gh pr list --limit 5

## Recent File Changes

!command
git diff --name-only HEAD~5..HEAD

---

Based on this information:
1. Summarize current work
2. Identify any blockers
3. Suggest next steps
```

### File Analysis Skill

```markdown
---
name: analyze-file
description: Analyze a specific file
user-invocable: true
---

# File Analysis: $0

## Git History

!command
git log --oneline --max-count=5 -- $0

## Current Content

!command
cat $0

## TODO Items

!command
grep -n "TODO" $0

---

Analyze this file and suggest improvements.
```

## FAQ

**Q: Can I use npm/make/other build commands?**
A: No. Only git, gh, ls, cat, and grep are whitelisted for security.

**Q: Can I pipe commands together?**
A: No. Shell metacharacters (|, $, >, etc.) are blocked to prevent injection attacks.

**Q: How do I run commands longer than 5 seconds?**
A: You can't. Limit command scope to stay within timeout.

**Q: Can I modify files with preprocessing?**
A: No. The sandbox is read-only. Use regular tools via the main agent instead.

**Q: Can I fetch data from APIs?**
A: No. Network commands (curl, wget) are blocked. Use web tools instead.

---

**Learn More**:
- [Skills System Overview](./skills-guide.md)
- [Creating Custom Skills](./custom-skills.md)
- [Security Model](./security-guide.md)
