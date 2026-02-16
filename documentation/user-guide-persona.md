# User Guide: Persona Customization

**Version:** 1.0
**Last Updated:** 2026-02-15
**Audience:** End Users

---

## Table of Contents

1. [Introduction](#introduction)
2. [Getting Started](#getting-started)
3. [SOUL.md - AI Personality](#soulmd---ai-personality)
4. [USER.md - Your Context](#usermd---your-context)
5. [RULES.md - Hard Rules](#rulesmd---hard-rules)
6. [Best Practices](#best-practices)
7. [Examples](#examples)
8. [Troubleshooting](#troubleshooting)
9. [FAQ](#faq)

---

## Introduction

### What is Persona Customization?

Persona customization allows you to personalize your NuimanBot experience by customizing three aspects:

- **SOUL.md** - The AI's personality, voice, and expertise
- **USER.md** - Your profile, preferences, and context
- **RULES.md** - Hard rules that control what the AI can and cannot do

These files are stored in your personal directory (`~/.nuimanbot/personas/<your-user-id>/`) and are automatically loaded whenever you interact with NuimanBot.

### Why Customize Your Persona?

**Better Responses:**
- AI understands your preferences and communication style
- Responses are tailored to your role and expertise level
- Context is preserved across conversations

**Safety and Control:**
- Block tools you don't want the AI to use
- Require confirmation for sensitive operations
- Enforce your own rules and guidelines

**Efficiency:**
- AI remembers your preferences (timezone, language, frameworks)
- No need to repeat context in every conversation
- Consistent behavior across all platforms (Telegram, Slack, CLI)

---

## Getting Started

### Initialize Your Persona Files

Your administrator will initialize your persona files with default templates:

```bash
# Administrator command (you don't run this)
./bin/nuimanbot persona init <your-user-id>
```

This creates three files in `~/.nuimanbot/personas/<your-user-id>/`:
- `SOUL.md` - AI personality template
- `USER.md` - User profile template
- `RULES.md` - Rules template

### Locating Your Files

**Linux/Mac:**
```bash
cd ~/.nuimanbot/personas/<your-user-id>/
ls -la
# You should see: SOUL.md, USER.md, RULES.md
```

**Windows:**
```cmd
cd %USERPROFILE%\.nuimanbot\personas\<your-user-id>
dir
```

### Editing Your Files

Use any text editor:
- **CLI:** `vim`, `nano`, `emacs`
- **GUI:** VS Code, Sublime Text, Notepad++
- **Online:** GitHub web editor (if files are in version control)

**Important:** Files are plain Markdown - no special software needed!

---

## SOUL.md - AI Personality

### Purpose

SOUL.md defines **how the AI behaves and communicates** with you.

### What to Include

**Communication Style:**
- Tone: formal, casual, friendly, technical
- Verbosity: concise vs detailed explanations
- Response format: bullet points, paragraphs, code-first

**Expertise and Knowledge:**
- What areas should the AI specialize in?
- What level of detail do you expect?
- What jargon or terminology should it use?

**Personality Traits:**
- Proactive vs reactive
- Patient vs efficient
- Teach-and-explain vs quick-answers

### Example: Software Developer

```markdown
# AI Personality

You are a senior software engineer specializing in Go and Clean Architecture.

## Communication Style

- **Tone:** Professional and technical
- **Verbosity:** Concise - get to the point quickly
- **Format:** Code examples first, explanations second

## Expertise

- Deep knowledge of Go (stdlib, concurrency, generics)
- Clean Architecture and SOLID principles
- Test-Driven Development (TDD)
- Microservices and distributed systems

## Behavior

- Always provide working code examples
- Reference official documentation when applicable
- Point out potential bugs or edge cases
- Suggest performance optimizations when relevant

## Avoid

- Don't explain basic programming concepts unless asked
- Don't suggest Python or Node.js - I only use Go
- Don't provide untested code snippets
```

### Example: Data Analyst

```markdown
# AI Personality

You are a data analysis expert specializing in Python and SQL.

## Communication Style

- **Tone:** Friendly and educational
- **Verbosity:** Detailed explanations with examples
- **Format:** Step-by-step walkthroughs

## Expertise

- Python data analysis (pandas, numpy, matplotlib)
- SQL query optimization
- Statistical analysis and visualization
- Business intelligence and reporting

## Behavior

- Explain the "why" behind recommendations
- Provide multiple approaches to problems
- Include visualization suggestions
- Validate assumptions with data checks

## Teaching Approach

- Break down complex concepts into simple steps
- Use analogies and real-world examples
- Encourage best practices (reproducibility, documentation)
```

---

## USER.md - Your Context

### Purpose

USER.md stores **your profile, preferences, and ongoing context**.

### What to Include

**Personal Profile:**
- Name and role
- Timezone and location
- Preferred language

**Technical Preferences:**
- Programming languages and frameworks
- Tools and editors
- Coding style and conventions

**Current Projects:**
- What are you working on?
- Tech stack and architecture
- Challenges and goals

**Preferences:**
- How do you like to work?
- What are your priorities?
- What should the AI remember about you?

### Example: Backend Engineer

```markdown
# User Profile

**Name:** Alice Chen
**Role:** Senior Backend Engineer
**Company:** TechCorp
**Timezone:** America/New_York (EST/EDT)

## Technical Preferences

**Languages:** Go (primary), Python (scripts), SQL
**Frameworks:** Standard library preferred, gRPC, Chi router
**Tools:** VS Code, Docker, PostgreSQL, Redis
**Version Control:** Git with GitHub

## Coding Style

- Follow Google Go Style Guide
- Use table-driven tests
- Verbose error wrapping with context
- Prefer composition over inheritance
- Maximum function complexity: 15 cyclomatic complexity

## Current Projects

### Project: Event-Driven Microservices

**Goal:** Migrate monolith to microservices
**Architecture:**
- gRPC for inter-service communication
- PostgreSQL for primary storage
- Redis for caching and pub/sub
- Docker + Kubernetes for deployment

**Challenges:**
- Managing distributed transactions
- Service discovery and health checks
- Observability (metrics, tracing, logging)

**Status:** Phase 2 - Breaking down authentication service

## Working Style

- Prefer TDD (test-first development)
- Code review every PR before merge
- Daily standup at 9 AM EST
- Deep work blocks: 10 AM - 12 PM, 2 PM - 4 PM
```

### Example: Product Manager

```markdown
# User Profile

**Name:** Bob Smith
**Role:** Senior Product Manager
**Company:** StartupCo
**Timezone:** Europe/London (GMT/BST)

## Work Context

**Products:** SaaS analytics platform
**Team Size:** 15 (3 engineers, 2 designers, 1 data analyst)
**Methodology:** Agile (2-week sprints)

## Priorities

1. User retention and engagement
2. Feature adoption metrics
3. Revenue growth (MRR, ARR)
4. Customer satisfaction (NPS)

## Communication Preferences

- Data-driven decision making
- Focus on user impact and business value
- Concise summaries with action items
- Visual aids (charts, mockups, diagrams)

## Ongoing Initiatives

### Q1 Goals

- **OKR 1:** Increase user retention from 75% to 85%
- **OKR 2:** Launch mobile app (iOS, Android)
- **OKR 3:** Reduce churn by 20%

### Current Blockers

- API performance issues affecting user experience
- Mobile app delayed (waiting on design approval)
- Need A/B testing framework for feature experiments
```

---

## RULES.md - Hard Rules

### Purpose

RULES.md defines **hard constraints** that the AI **must follow**. These rules are enforced by the system and cannot be bypassed.

### YAML Frontmatter

RULES.md uses YAML frontmatter for machine-enforceable rules:

```yaml
---
blocked_tools:
  - tool_name_1
  - tool_name_2
requires_confirmation:
  - sensitive_tool_1
  - sensitive_tool_2
---
```

### Rule Types

**1. blocked_tools** - Tools the AI is **completely forbidden** from using

**2. requires_confirmation** - Tools that require **user approval** before execution (UI integration pending)

### Available Tools

Ask your administrator for a list of available tools. Common examples:
- `calculator` - Math calculations
- `datetime` - Date and time operations
- `weather` - Weather information
- `websearch` - Web search
- `github` - GitHub operations
- `repo_search` - Code search
- `filesystem_delete` - File deletion (dangerous!)
- `external_api` - External API calls

### Example: Conservative Rules

```yaml
---
blocked_tools:
  - filesystem_delete
  - credential_use
  - external_api
requires_confirmation:
  - github
  - repo_search
---

# Custom Rules

## Code Quality Standards

- **Never** commit code without tests
- **Always** run linter before committing
- **Required:** Minimum 85% test coverage per package

## Development Guidelines

- Use Go standard library over third-party packages when possible
- All public functions must have doc comments
- Error handling is mandatory - never ignore errors
- Use `context.Context` for cancellation and timeouts

## Security

- Never log sensitive data (credentials, tokens, PII)
- Validate all user input
- Use parameterized queries (prevent SQL injection)
- Encrypt secrets at rest (AES-256-GCM)

## Communication

- Be direct and concise
- Focus on solutions, not problems
- Provide code examples over explanations
- Link to official documentation
```

### Example: Permissive Rules

```yaml
---
blocked_tools: []
requires_confirmation:
  - filesystem_delete
  - credential_use
---

# Custom Rules

## Preferences

- Prefer detailed explanations with examples
- Show multiple approaches to problems
- Teach concepts, don't just solve problems
- Encourage experimentation and learning

## Code Style

- Readability over cleverness
- Comments for non-obvious logic
- Consistent naming conventions
- Test edge cases

## Responses

- Start with a summary (TL;DR)
- Include step-by-step instructions
- Provide links to further reading
- Ask clarifying questions when requirements are unclear
```

---

## Best Practices

### SOUL.md Best Practices

✅ **Do:**
- Be specific about tone and style
- Define your expertise areas clearly
- Include examples of desired behavior
- Update as your needs change

❌ **Don't:**
- Make it too long (aim for 1-2 pages)
- Include personal information (use USER.md instead)
- Contradict your RULES.md constraints
- Use conflicting instructions

### USER.md Best Practices

✅ **Do:**
- Keep it current (update projects, priorities)
- Include timezone and availability
- Document your tech stack
- Note ongoing challenges

❌ **Don't:**
- Include passwords or secrets
- Over-share personal details
- Duplicate SOUL.md content
- Let it become outdated

### RULES.md Best Practices

✅ **Do:**
- Start conservative, relax later
- Document *why* tools are blocked
- Review periodically (remove unnecessary blocks)
- Use `requires_confirmation` for learning

❌ **Don't:**
- Block tools you actually need
- Forget to update when policies change
- Use empty lists (use `[]` for empty arrays)
- Add rules that can't be enforced

---

## Examples

### Example 1: DevOps Engineer

**SOUL.md:**
```markdown
# AI Personality

You are a DevOps/SRE expert specializing in Kubernetes and AWS.

## Style
- Imperative commands and examples
- Focus on automation and reliability
- Production-ready solutions only

## Expertise
- Kubernetes, Docker, Helm
- AWS (EC2, EKS, RDS, S3)
- Terraform, Ansible
- Prometheus, Grafana
```

**USER.md:**
```markdown
# User Profile

**Name:** Carlos Rodriguez
**Role:** DevOps Engineer
**Timezone:** America/Los_Angeles

## Current Project
Migrating monolith to Kubernetes on AWS EKS

## Tech Stack
- Kubernetes 1.28
- AWS EKS with Fargate
- Terraform for IaC
- Prometheus + Grafana monitoring
```

**RULES.md:**
```yaml
---
blocked_tools:
  - production_deploy
  - database_migration
requires_confirmation:
  - kubectl_apply
  - terraform_apply
---

# Rules

- Always include rollback procedures
- Test in staging before production
- Include monitoring and alerting
```

### Example 2: Frontend Developer

**SOUL.md:**
```markdown
# AI Personality

You are a React/TypeScript expert focused on modern frontend development.

## Style
- Show working code first
- Explain component design patterns
- Focus on TypeScript type safety

## Expertise
- React 18+ with hooks
- TypeScript (strict mode)
- CSS-in-JS (styled-components)
- State management (Redux, Zustand)
```

**USER.md:**
```markdown
# User Profile

**Name:** Sarah Johnson
**Role:** Frontend Developer
**Timezone:** Europe/Paris

## Preferences
- Functional components only (no classes)
- TypeScript strict mode
- CSS modules over styled-components
- Prefer native browser APIs over libraries

## Current Project
Building customer dashboard with real-time updates

## Tech Stack
- React 18 + TypeScript
- Vite for bundling
- React Query for data fetching
- WebSockets for real-time
```

**RULES.md:**
```yaml
---
blocked_tools: []
requires_confirmation:
  - npm_install
  - git_push
---

# Rules

- All components must be typed
- Write tests for user interactions
- Accessibility (WCAG AA minimum)
- Mobile-first responsive design
```

---

## Troubleshooting

### AI Doesn't Seem to Use My Persona Files

**Check file locations:**
```bash
ls -la ~/.nuimanbot/personas/<your-user-id>/
```

**Verify file names:**
- Must be exactly `SOUL.md`, `USER.md`, `RULES.md` (case-sensitive)
- Must be in your user-specific directory

**Check file permissions:**
```bash
chmod 644 ~/.nuimanbot/personas/<your-user-id>/*.md
```

**Ask administrator to verify:**
- Persona system is enabled in config
- Your user ID is correct
- File cache isn't stale (15-minute TTL)

### RULES.md Not Blocking Tools

**Check YAML syntax:**
```yaml
---
blocked_tools:
  - tool_name  # Correct: list item with hyphen
requires_confirmation:
  - another_tool
---
```

**Common errors:**
```yaml
# ❌ Wrong: missing hyphen
blocked_tools:
  tool_name

# ❌ Wrong: missing colon
blocked_tools
  - tool_name

# ✅ Correct
blocked_tools:
  - tool_name
```

**Verify tool names:**
- Ask administrator for exact tool names
- Tool names are case-sensitive
- Check for typos

### Changes Not Taking Effect

**Wait for cache expiry:**
- File cache TTL: 15 minutes
- Changes may take up to 15 minutes to apply

**Force cache refresh:**
- Ask administrator to restart NuimanBot
- Or wait for cache TTL to expire

### File Too Large Error

**Limit:** 100KB per file

**Solution:**
- Keep SOUL.md concise (1-2 pages)
- Move detailed examples to USER.md
- Remove unnecessary content
- Split into sections with headers

---

## FAQ

### Q: Can I use multiple SOUL.md files?

**A:** No, only one SOUL.md per user. Use headers and sections to organize different aspects.

### Q: Can I share my persona files with teammates?

**A:** Yes! Persona files are plain Markdown. You can:
- Copy files to other users' directories
- Store in version control (Git)
- Create team templates

**Note:** Each user needs files in their own directory.

### Q: What happens if I delete a persona file?

**A:** NuimanBot gracefully degrades:
- Missing SOUL.md → Uses default AI behavior
- Missing USER.md → No user-specific context
- Missing RULES.md → No custom rules enforced

No errors or crashes - the system continues working.

### Q: Can I use HTML or other formats?

**A:** Persona files must be Markdown (`.md`). HTML within Markdown may work but is not officially supported.

### Q: How do I reset to defaults?

**A:** Ask your administrator to re-run initialization:
```bash
./bin/nuimanbot persona init <your-user-id> --force
```

Or manually delete and recreate from templates.

### Q: Can I use emojis and Unicode?

**A:** Yes! Full Unicode support including emojis, accented characters, and non-Latin scripts.

### Q: Are my persona files private?

**A:** Files are stored locally on the server. Security depends on:
- File system permissions (administrator-configured)
- Who has access to the server
- Whether files are backed up or version-controlled

Ask your administrator about your organization's privacy policies.

### Q: Can admin override my rules?

**A:** Yes. Administrators can set global policies that take precedence over user rules. This ensures organization-wide security compliance.

### Q: How often should I update my persona files?

**A:** Update when:
- Starting a new project
- Your role or responsibilities change
- You discover new preferences
- Your tech stack changes

No need for daily updates - weekly or monthly reviews are sufficient.

---

## Getting Help

**Administrator:** Contact your NuimanBot administrator for:
- File initialization issues
- Permission errors
- Tool availability questions
- System configuration

**Documentation:** See also:
- [README.md](../README.md) - Quick start guide
- [Product Details](product-details.md) - Technical specifications
- [Admin Guide](admin-guide-persona.md) - Administrator documentation

**Support:** Report issues at: https://github.com/anthropics/nuimanbot/issues

---

**Document Version:** 1.0
**Last Updated:** 2026-02-15
**License:** MIT
