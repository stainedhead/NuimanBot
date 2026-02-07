# User Onboarding Guide

Welcome to NuimanBot! This guide will help you get started using NuimanBot, understand its features, and customize your experience.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Your First Conversation](#your-first-conversation)
3. [Using Tools](#using-tools)
4. [Using Skills](#using-skills)
5. [Advanced Features](#advanced-features)
6. [Customizing Your Experience](#customizing-your-experience)
7. [Common Use Cases](#common-use-cases)
8. [Tips & Best Practices](#tips--best-practices)
9. [Troubleshooting](#troubleshooting)

---

## Getting Started

### Prerequisites

Before you can use NuimanBot, an administrator must:
1. Create your user account
2. Assign you a role (guest, user, or admin)
3. Configure your tool permissions (if user role)
4. Provide you with access credentials (if using Telegram/Slack)

**Need an account?** Contact your NuimanBot administrator to get set up.

### Access Methods

You can interact with NuimanBot through three platforms:

#### CLI (Command-Line Interface)
- Direct access on the server running NuimanBot
- Interactive REPL (Read-Eval-Print Loop)
- Best for: Development, testing, admin tasks

```bash
# Start NuimanBot
./bin/nuimanbot

# You'll see:
>
```

#### Telegram
- Access via Telegram messenger app
- Private messages with your bot
- Best for: Mobile access, quick queries

**Setup:**
1. Open Telegram
2. Search for your bot's username (provided by admin)
3. Send `/start` to begin

#### Slack
- Access via your Slack workspace
- Direct messages or mention in channels
- Best for: Team collaboration, work context

**Setup:**
1. Open Slack workspace
2. Find the NuimanBot app in your workspace
3. Send a direct message or mention `@NuimanBot`

---

## Your First Conversation

### Basic Chat

Simply type a message and press Enter. NuimanBot will respond using its configured LLM (Claude, GPT, or Ollama).

**Example:**

```
> Hello! What can you help me with?
Bot: Hi! I'm NuimanBot, an AI assistant. I can help you with:
- General questions and conversation
- Using various tools (calculator, web search, etc.)
- Running specialized skills for tasks like code review
- And much more! What would you like to do today?
```

### Understanding Responses

NuimanBot may:
- Answer directly from its knowledge base
- Use tools to get real-time information
- Invoke skills for specialized tasks
- Ask clarifying questions

**Tool Usage Example:**

```
> What's the weather in London?
Bot: [Using weather tool...]
Bot: The current weather in London is:
- Temperature: 12°C
- Conditions: Partly cloudy
- Humidity: 65%
- Wind: 15 km/h NW
```

### Exiting (CLI Only)

```
> exit
# or
> quit
```

---

## Using Tools

### What Are Tools?

Tools are specialized functions that NuimanBot can use to perform specific tasks:
- Get real-time information (weather, web search)
- Perform calculations
- Manage notes
- Interact with GitHub
- Search codebases
- And more

### Available Tools

Your administrator controls which tools you can use. To see what's available, ask:

```
> What tools do I have access to?
```

**Common Tools:**

| Tool | What It Does | Example |
|------|--------------|---------|
| **calculator** | Math operations | "Calculate 25 * 4" |
| **datetime** | Current time, date formatting | "What time is it in Tokyo?" |
| **weather** | Current weather and forecasts | "Weather in San Francisco?" |
| **websearch** | Search the web | "Search for golang tutorials" |
| **notes** | Create and manage notes | "Create a note: Meeting at 3pm" |
| **github** | GitHub operations | "List issues in user/repo" |
| **repo_search** | Search code | "Find all TODO comments" |
| **doc_summarize** | Summarize documents | "Summarize https://..." |
| **summarize** | Summarize web pages/videos | "Summarize this YouTube video" |

### How Tools Are Used

**Automatic Tool Selection:**
NuimanBot automatically chooses the right tool based on your request.

```
> What's 123 * 456?
Bot: [Using calculator...]
Bot: The result is 56,088.

> Search for "AI safety research"
Bot: [Using websearch...]
Bot: Here are the top results:
1. AI Safety Research - OpenAI
2. ...
```

**Tool Permissions:**
If you lack permission for a tool, you'll see:

```
> Search the web for golang tips
Bot: I don't have permission to use the websearch tool.
Please contact your administrator to request access.
```

---

## Using Skills

### What Are Skills?

Skills are reusable prompt templates designed for specific tasks:
- Code reviews
- Debugging assistance
- API documentation generation
- Refactoring suggestions
- Test writing help

Skills provide structured, expert-level responses tailored to their domain.

### Listing Available Skills

```
> /help
```

**Example Output:**

```
Available skills:
  /code-review - Comprehensive code review with quality analysis
  /debugging - Systematic debugging assistance
  /api-docs - Generate API documentation
  /refactoring - Code refactoring with patterns
  /testing - Help write comprehensive tests
```

### Describing a Skill

Get details about what a skill does:

```
> /describe code-review
```

**Example Output:**

```
Skill: code-review
Description: Perform comprehensive code review with quality analysis

This skill provides expert code review covering:
- Code quality analysis (readability, maintainability)
- Security checks and SOLID principles
- Bug detection and edge cases
- Performance considerations
- Best practices and recommendations

Usage: /code-review <file-path-or-code>
Allowed Tools: repo_search, github
```

### Invoking a Skill

Use the skill name with `/` prefix, followed by arguments:

```
> /code-review src/auth/login.go
Bot: [Skill activated: code-review]
Bot: # Code Review Summary

**File:** src/auth/login.go

## Strengths
- Clear error handling with proper error wrapping
- Good use of constants for magic strings
...

## Issues Found
1. **Medium Severity**: Password validation lacks complexity requirements
   - Line 45: Password check only validates length
   - Recommendation: Add regex for uppercase, lowercase, numbers
...

## Recommendations
- Consider adding rate limiting for login attempts
- Implement password strength meter
- Add logging for failed login attempts
```

### Skills with Arguments

Skills can accept multiple arguments:

```
> /refactoring apply factory pattern to user creation
Bot: [Analyzing user creation code...]
Bot: Here's a refactored version using the Factory pattern:
...
```

---

## Advanced Features

NuimanBot includes advanced features for power users:

### Subagent Execution

Some skills can run autonomously in the background, performing multi-step investigations:

```
> /debug-issue The login button doesn't work on mobile
Bot: Subagent started: debug-issue-abc123
Bot: I've started an autonomous investigation. This may take a few minutes.

[Subagent works in background, using tools, analyzing code, checking logs]

Bot: Investigation complete!

# Findings
1. Mobile viewport CSS issue detected
2. JavaScript event listener not mobile-compatible
3. Touch events not properly handled

# Root Cause
The click handler only listens for 'click' events, which don't fire
reliably on mobile browsers. Need to add 'touchstart' event listener.

# Recommended Fix
[Code snippet provided...]
```

**Features:**
- Runs in background
- Multi-step autonomous execution
- Uses tools independently
- Provides comprehensive analysis

### Dynamic Content with Preprocessing

Skills can include real-time data from git, filesystem, and other sources:

```
> /project-status
Bot: [Executing preprocessing commands...]
Bot: # Project Status Report

## Git Status
Modified files:
- README.md
- internal/domain/skill.go
- internal/usecase/chat/service.go

## Recent Commits
- abc123 feat: add preprocessing support
- def456 fix: resolve race condition in cache
- ghi789 docs: update API documentation

## Summary
3 files modified, last commit 2 hours ago
```

**How It Works:**
- Skills embed shell commands (git, ls, cat, etc.)
- Commands execute safely in sandbox
- Output is inserted into skill content
- Fresh data on every invocation

### Persistent Memory

Skills can remember information between invocations:

```
> /project-tracker Remember I'm working on auth system
Bot: Noted! I'll remember you're working on the auth system.

[Later...]

> /project-tracker What am I working on?
Bot: You're currently working on the auth system.
```

**Memory Scopes:**
- **Skill-specific**: Only that skill remembers
- **User-specific**: Follows you across platforms (future)
- **Global**: Shared across all users
- **Session**: Temporary, clears on restart

---

## Customizing Your Experience

### Understanding Your Role

Your role determines what you can do:

**Guest Role:**
- Chat with the LLM only
- No tool access
- Limited functionality

**User Role (most common):**
- Access to configured tools
- Can invoke skills
- Custom tool allowlist per user

**Admin Role:**
- All tools available
- Admin commands
- User management
- System configuration

**Check Your Role:**
```
> What is my role?
Bot: Your role is: user
```

### Requesting Tool Access

If you need additional tools:

1. Identify which tool you need
2. Contact your administrator
3. Provide business justification
4. Administrator updates your allowlist

**Example Request:**

> Hi Admin, I'd like access to the `github` tool so I can check issue status
> and create PRs as part of my development workflow. Thanks!

### Multi-Platform Access

You can use the same account across multiple platforms:

**Example:**
- Use CLI for development work
- Use Telegram for quick mobile queries
- Use Slack for team collaboration

**Note:** Currently, conversation history is platform-specific. Cross-platform conversation merging is planned for a future release.

---

## Common Use Cases

### 1. Research & Information Gathering

**Web Research:**
```
> Search for "serverless architecture best practices" and summarize the top 3 results
Bot: [Using websearch and summarize tools...]
Bot: Top 3 insights on serverless best practices:

1. Cold Start Optimization
   - Keep functions small and focused
   - Use connection pooling
   ...

2. Cost Management
   - Set up billing alerts
   - Monitor function duration
   ...

3. Security Considerations
   - Implement least privilege IAM
   - Validate all inputs
   ...
```

**Document Summarization:**
```
> Summarize this article: https://example.com/long-article
Bot: [Using doc_summarize tool...]
Bot: Summary (250 words):

The article discusses the evolution of cloud computing...
[Concise summary provided]
```

### 2. Development Workflow

**Code Review:**
```
> /code-review src/api/handlers/user.go
Bot: [Analyzing code...]
Bot: # Code Review

## Strengths
- Good error handling
- Clear function names

## Issues
1. SQL injection vulnerability (High)
   - Line 42: User input directly in query
   - Use parameterized queries

2. Missing input validation (Medium)
   - Line 38: Email not validated
   ...
```

**Debugging Assistance:**
```
> /debugging I'm getting "connection refused" on port 5432
Bot: Let's debug this systematically.

## Phase 1: Understand
Q: Is this PostgreSQL? Is it running locally or remote?
A: [You answer]

## Phase 2: Verify
Let me check if the service is running...
[Guides you through debugging steps]
```

**Repository Search:**
```
> Find all functions that handle authentication
Bot: [Using repo_search...]
Bot: Found 8 functions handling authentication:

1. internal/auth/service.go:45
   func (s *Service) Authenticate(ctx context.Context, token string) error

2. internal/auth/jwt.go:23
   func ValidateToken(token string) (*Claims, error)

...
```

### 3. Note-Taking & Organization

**Creating Notes:**
```
> Create a note titled "Meeting Notes" with content: Discussed Q1 roadmap, prioritize auth feature
Bot: [Using notes tool...]
Bot: Note created successfully!
ID: note-abc123
Title: Meeting Notes
```

**Listing Notes:**
```
> List my notes
Bot: [Using notes tool...]
Bot: You have 5 notes:

1. Meeting Notes (2026-02-07)
2. API Design Ideas (2026-02-06)
3. Bug Investigation (2026-02-05)
...
```

**Reading Notes:**
```
> Read note "Meeting Notes"
Bot: [Using notes tool...]
Bot: Title: Meeting Notes
Created: 2026-02-07
Content: Discussed Q1 roadmap, prioritize auth feature
```

### 4. GitHub Workflow

**Check Issues:**
```
> List open issues in myorg/myrepo
Bot: [Using github tool...]
Bot: Open issues in myorg/myrepo (5):

1. #42 - Add user authentication
   Labels: enhancement, priority-high

2. #38 - Fix mobile responsive layout
   Labels: bug, ui
...
```

**Create Pull Request:**
```
> Create a PR for branch "feature/auth" with title "Add JWT authentication"
Bot: [Using github tool...]
Bot: Pull request created!
PR #43: Add JWT authentication
URL: https://github.com/myorg/myrepo/pull/43
```

### 5. Learning & Tutorials

**Explain Concepts:**
```
> Explain dependency injection with a Go example
Bot: Dependency Injection (DI) is a design pattern where dependencies
are provided to a component rather than created by it.

Example in Go:

```go
// Without DI (tightly coupled)
type UserService struct {
    db *sql.DB
}

func NewUserService() *UserService {
    db, _ := sql.Open("postgres", "...") // Hard-coded dependency
    return &UserService{db: db}
}

// With DI (loosely coupled)
type UserService struct {
    db Database // Interface, not concrete type
}

func NewUserService(db Database) *UserService {
    return &UserService{db: db} // Dependency injected
}
```

Benefits:
- Easier testing (inject mocks)
- Better modularity
- Flexibility to change implementations
...
```

**Code Examples:**
```
> Show me how to handle HTTP middleware in Go
Bot: Here's how to implement HTTP middleware in Go:

[Detailed code examples and explanation]
```

---

## Tips & Best Practices

### Effective Communication

**1. Be Specific**
```
❌ "Fix my code"
✅ "Review this authentication function for security issues"

❌ "Search the web"
✅ "Search for React performance optimization techniques in 2026"
```

**2. Provide Context**
```
> I'm working on a REST API in Go using Gin framework.
  How should I structure error handling middleware?
```

**3. Ask Follow-up Questions**
```
> Can you explain that in simpler terms?
> What about edge cases?
> Show me an example
```

### Using Skills Effectively

**1. Explore Available Skills**
```
> /help
> /describe <skill-name>
```

**2. Provide Necessary Context**
```
❌ /code-review
✅ /code-review src/auth/handler.go

❌ /debugging error
✅ /debugging "connection refused on port 5432" when starting PostgreSQL
```

**3. Combine Skills with Tools**

Skills can use tools automatically:
```
> /code-review src/api/
[Skill uses repo_search to find files, analyzes them, provides comprehensive review]
```

### Managing Conversations

**1. Long Conversations**

NuimanBot automatically summarizes old messages to stay within token limits. Your recent context is always preserved.

**2. Starting Fresh**

For a new topic, you can:
- Exit and restart (CLI)
- Start a new chat (Telegram/Slack)
- Simply change topics (context is preserved but old info may be summarized)

**3. Multi-Platform Continuity**

If switching platforms:
```
[CLI]
> Create a note: Research GraphQL vs REST

[Later, on Telegram]
> Read my notes
Bot: You have 1 note:
1. Research GraphQL vs REST
```

### Security & Privacy

**1. Don't Share Sensitive Information**
```
❌ "My password is abc123"
❌ "Here's our API key: sk-..."
✅ "How should I securely store API keys?"
```

**2. Be Aware of Tool Permissions**

Some tools have access to:
- File system (repo_search)
- GitHub repositories (github)
- Web content (websearch, summarize)

Always verify before asking NuimanBot to access sensitive data.

**3. Audit Trail**

All tool executions are logged. Administrators can review:
- What tools you used
- When you used them
- What parameters you provided

---

## Troubleshooting

### "Permission denied" or "You don't have access to this tool"

**Problem:** Your role doesn't include the tool you're trying to use.

**Solution:**
1. Check your available tools: "What tools do I have?"
2. Contact your administrator to request access
3. Provide justification for why you need the tool

---

### Skill not working or returns error

**Problem:** Skill may require tools you don't have access to.

**Solutions:**

1. **Check skill requirements:**
```
> /describe <skill-name>
# Look for "Allowed Tools" section
```

2. **Request necessary tool access:**

If skill requires `repo_search` but you don't have it, ask admin for access.

3. **Try a different skill:**

Some skills overlap in functionality but use different tools.

---

### Bot response is slow

**Possible Causes:**

1. **LLM provider latency** - Normal for complex queries (2-5 seconds)
2. **Tool execution** - Weather/web search APIs can be slow
3. **Subagent execution** - Autonomous tasks may take minutes

**Tips:**
- Be patient with complex requests
- For autonomous tasks, the bot will notify when complete
- Simple questions are usually instant

---

### "No response" or connection issues

**CLI:**
```
# Check if service is running
ps aux | grep nuimanbot

# Check logs
tail -f logs/nuimanbot.log
```

**Telegram/Slack:**
1. Check your internet connection
2. Verify bot is online (ask admin)
3. Try restarting the app

---

### Lost conversation history

**Problem:** Conversation seems to "forget" earlier context.

**Explanation:**
NuimanBot automatically summarizes old messages when approaching token limits. This is normal and preserves key information while maintaining performance.

**Solution:**
- Important information is preserved in summaries
- For critical data, use the notes tool to persist it
- Start a new conversation for unrelated topics

---

### Incorrect tool selection

**Problem:** Bot uses wrong tool or doesn't use a tool when it should.

**Example:**
```
> Calculate 25 * 4
Bot: The answer is approximately 100.
[Should have used calculator tool for exact answer]
```

**Solution:**
Be more explicit:
```
> Use the calculator tool to calculate 25 * 4
Bot: [Using calculator...]
Bot: The result is 100.
```

---

### Platform-specific issues

**Telegram:**
- **Bot doesn't respond:** Check if you're on the allowlist (admin setting)
- **Rate limited:** Too many messages too quickly (wait 1 minute)

**Slack:**
- **Bot doesn't respond:** Make sure you mention `@NuimanBot` in channels
- **DM not working:** Verify bot is added to your workspace

**CLI:**
- **Can't type:** Check if process is running (`ps aux | grep nuimanbot`)
- **Echo disabled:** This is normal (like password input), text is still captured

---

## Getting More Help

### Documentation

- **[Installation & Setup Guide](install-and-setup.md)** - System installation (admin)
- **[CLI Administration Guide](cli-admin-guide.md)** - User management (admin)
- **[Agent Skills User Guide](skills-guide.md)** - Creating custom skills

### Phase 3 Advanced Features

- **[Subagents Guide](subagents-guide.md)** - Autonomous multi-step workflows
- **[Preprocessing Guide](preprocessing-guide.md)** - Dynamic content with shell commands
- **[Plugins Guide](plugins-guide.md)** - Third-party skill packages
- **[Versioning Guide](versioning-guide.md)** - Skill version management
- **[Memory Guide](memory-guide.md)** - Persistent skill state

### Support

- **Issues**: https://github.com/stainedhead/NuimanBot/issues
- **Ask Your Admin**: For access, permissions, configuration
- **Community**: Check if your organization has a NuimanBot user community

---

## Quick Reference

### Essential Commands

```bash
# CLI Exit
exit
quit

# Help
/help                    # List all skills
/describe <skill-name>  # Describe a skill

# Skills
/code-review <file>
/debugging <issue>
/api-docs <file>
/refactoring <code>
/testing <file>

# Common Queries
What tools do I have?
What is my role?
List my notes
Search for "..."
Calculate ...
What's the weather in ...?
```

### Tool Reference

| Tool | Permission Needed | Usage |
|------|-------------------|-------|
| calculator | User | Math operations |
| datetime | User | Time and date info |
| weather | User | Weather data |
| websearch | User | Web search |
| notes | User | Note management |
| github | User/Admin | GitHub operations |
| repo_search | User | Code search |
| doc_summarize | User | Document summaries |
| summarize | User | Web/video summaries |
| coding_agent | Admin only | External coding tools |

---

**Welcome to NuimanBot!** 🤖

We hope this guide helps you get the most out of your AI assistant. If you have questions, feedback, or suggestions, please reach out to your administrator or open an issue on GitHub.

Happy chatting! 🚀

---

**Last Updated:** 2026-02-07
**Version:** 1.0
