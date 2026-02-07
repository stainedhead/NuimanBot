# Agent Skills User Guide

## Overview

Agent Skills are reusable prompt templates that extend NuimanBot's capabilities. Skills enable specialized behaviors for common development tasks like code review, debugging, testing, and more.

Skills follow the [Anthropic Agent Skills](https://github.com/anthropics/anthropic-skills) open standard, making them portable and interoperable across Claude-based tools.

## Using Skills

### List Available Skills

To see all available skills:

```
/help
```

Example output:
```
Available skills:
  /api-docs - Generate comprehensive API documentation from code or specifications
  /code-review - Perform comprehensive code review with quality analysis
  /debugging - Systematic debugging assistance to identify and fix bugs
  /refactoring - Suggest code refactoring improvements while maintaining functionality
  /testing - Help write comprehensive tests with strategies and edge cases
```

### Invoke a Skill

To use a skill, type `/skill-name` followed by optional arguments:

```
/code-review src/auth/login.go
```

The skill will render its prompt template with your arguments and process it through the LLM.

### Describe a Skill

To see detailed information about a skill:

```
/describe code-review
```

This displays:
- Skill name and description
- Allowed tools
- Full skill prompt template
- Metadata (scope, invocability)

## Example Skills

NuimanBot includes 5 production-ready example skills:

### `/code-review` - Code Review

Performs comprehensive code review with quality analysis.

**Usage:**
```
/code-review src/api/handlers.go
/code-review "authentication module"
```

**Features:**
- Code quality analysis (readability, maintainability, DRY)
- Correctness and logic verification
- Performance bottleneck identification
- Security vulnerability detection (SQL injection, XSS, etc.)
- Best practices validation (SOLID principles, design patterns)
- Documentation quality check

### `/debugging` - Systematic Debugging

5-phase systematic debugging approach.

**Usage:**
```
/debugging null pointer error in user service
/debugging intermittent timeout on API calls
```

**Features:**
- Problem understanding and reproduction steps
- Hypothesis formation (ranked by likelihood)
- Investigation plan with verification steps
- Root cause identification
- Fix recommendation with prevention strategies

### `/api-docs` - API Documentation

Generates comprehensive API documentation.

**Usage:**
```
/api-docs GET /api/users endpoint
/api-docs UserService REST API
```

**Features:**
- Endpoint documentation (method, path, description)
- Request/response schemas with field descriptions
- Authentication and error handling
- Code examples in multiple languages (cURL, JavaScript, Python, Go)
- Data model documentation

### `/refactoring` - Code Refactoring

Suggests refactoring improvements using proven patterns.

**Usage:**
```
/refactoring OrderProcessor class
/refactoring "payment validation logic"
```

**Features:**
- Code smell detection (bloaters, couplers, etc.)
- SOLID principle violations
- Refactoring pattern recommendations (Extract Method, Replace Conditional, etc.)
- Prioritized opportunities (high impact, low risk first)
- Before/after code examples
- Risk assessment and mitigation strategies

### `/testing` - Test Writing

Helps write comprehensive tests with edge case coverage.

**Usage:**
```
/testing UserService.CreateUser function
/testing "order calculation logic"
```

**Features:**
- Test plan creation
- AAA pattern (Arrange-Act-Assert)
- Table-driven test examples
- Edge case identification (boundaries, null handling, time zones)
- Mocking strategies
- Coverage analysis recommendations

## Creating Custom Skills

### Skill Directory Structure

Skills are stored in NuimanBot's data directory:

```
data/skills/
├── shared/              # Available to all users
│   ├── code-review/
│   │   └── SKILL.md
│   └── debugging/
│       └── SKILL.md
└── users/               # User-specific skills
    ├── cli_user/
    │   └── my-skill/
    │       └── SKILL.md
    └── telegram_123456/
        └── custom-skill/
            └── SKILL.md
```

**Shared Skills** (`data/skills/shared/`):
- Available to all users across all platforms (CLI, Telegram, Slack)
- Useful for organization-wide development standards
- Lower priority (100) than user-specific skills

**User-Specific Skills** (`data/skills/users/{platform}_{uid}/`):
- Private to individual users
- Higher priority (200) than shared skills
- Format: `cli_user`, `telegram_123456`, `slack_U01ABC123`

### Skill File Format

Each skill is a `SKILL.md` file with YAML frontmatter:

```markdown
---
name: my-skill
description: Brief description of what this skill does
user-invocable: true
model-invocable: true
allowed-tools:
  - repo_search
  - github
---

# My Skill

You are an expert in [domain].

## Task

[Instruction]: $ARGUMENTS

## Guidelines

- Guideline 1
- Guideline 2

## Output Format

[Expected output structure]
```

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill identifier (kebab-case, used for `/skill-name`) |
| `description` | Yes | Brief description shown in skill list |
| `user-invocable` | No | Allow users to invoke directly (default: true) |
| `model-invocable` | No | Allow LLM to invoke autonomously (default: true) |
| `allowed-tools` | No | Restrict tool access (empty = all tools allowed) |

### Argument Substitution

Use placeholder variables in your skill body:

- `$ARGUMENTS` - All arguments joined with spaces
- `$0`, `$1`, `$2`, ... - Individual arguments by index

**Example:**

Skill file:
```markdown
---
name: greet
description: Generate a personalized greeting
---
Hello, $0! Welcome to $1.
```

Invocation:
```
/greet Alice "our platform"
```

Rendered:
```
Hello, Alice! Welcome to our platform.
```

### Tool Restrictions

Limit which tools the LLM can use when executing the skill:

```yaml
allowed-tools:
  - repo_search
  - github
  - websearch
```

If `allowed-tools` is empty or omitted, all tools are available.

**Common tools:**
- `repo_search` - Search codebase
- `github` - GitHub API operations
- `websearch` - Web search
- `notes` - Note-taking
- `doc_summarize` - Document summarization

## Configuration

### Skill Roots

Configure skill directories in `config.yaml`:

```yaml
skills:
  enabled: true
  roots:
    - path: "./data/skills/shared"
      scope: 2  # ScopeProject (priority: 100)

    - path: "./data/skills/users/cli_user"
      scope: 1  # ScopeUser (priority: 200)
```

### Skill Scopes and Priority

When multiple skills have the same name, priority determines which is used:

| Scope | Value | Priority | Use Case |
|-------|-------|----------|----------|
| Enterprise | 0 | 300 | Organization-wide mandatory skills |
| User | 1 | 200 | Personal user customizations |
| Project | 2 | 100 | Project/repository-specific skills |
| Plugin | 3 | 50 | Third-party plugin skills |

**Higher priority wins.** If a user creates a skill with the same name as a shared skill, the user's version is used.

### Path Expansion

Skill paths support:
- **Relative paths**: `./data/skills/shared`
- **Home directory**: `~/my-skills`
- **Absolute paths**: `/opt/nuimanbot/skills`

Tilde (`~`) expands to the user's home directory automatically.

## Best Practices

### Skill Design

1. **Clear Purpose**: Each skill should solve one specific problem
2. **Descriptive Names**: Use kebab-case names that describe the task
3. **Concise Descriptions**: Keep descriptions under 80 characters
4. **Professional Tone**: Write skills for technical audiences
5. **Structured Output**: Define clear output formats

### Skill Content

1. **Context Setting**: Begin with role/expertise definition
2. **Task Clarity**: Clearly state what the skill does
3. **Guidelines**: Provide specific, actionable guidelines
4. **Examples**: Include examples in the skill body
5. **Output Format**: Specify expected structure

### Tool Usage

1. **Minimize Tools**: Only allow tools that are necessary
2. **Security**: Be cautious with file system and network tools
3. **Performance**: Fewer tools = faster execution

### Testing

Test your skills before deploying:

1. **Describe**: Use `/describe skill-name` to review rendered content
2. **Invoke**: Test with various arguments
3. **Edge Cases**: Try with no arguments, many arguments, special characters
4. **Validation**: Verify tool restrictions work as expected

## Troubleshooting

### Skill Not Found

**Problem:** `/my-skill` returns "skill not found"

**Solutions:**
- Verify skill directory is in configured skill roots
- Check `SKILL.md` file exists in `my-skill/` directory
- Ensure frontmatter has correct `name: my-skill`
- Restart NuimanBot to reload skills
- Check logs for parsing errors

### Skill Not Listed

**Problem:** Skill doesn't appear in `/help`

**Solutions:**
- Verify `user-invocable: true` in frontmatter
- Check skill passed validation (valid YAML, required fields)
- Ensure higher-priority skill doesn't shadow it
- Look for parsing errors in logs

### Argument Substitution Not Working

**Problem:** `$ARGUMENTS` appears literally in output

**Solutions:**
- Verify placeholder is in skill body, not frontmatter
- Check for typos: `$ARGUMENTS` (case-sensitive)
- Ensure skill was invoked with arguments if using `$0`, `$1`, etc.

### Tool Access Denied

**Problem:** Skill can't access expected tools

**Solutions:**
- Check `allowed-tools` list in frontmatter
- Verify tool names match exactly (case-sensitive)
- Remove `allowed-tools` to allow all tools
- Check if tool is registered in NuimanBot

## Advanced Topics

### Skill Chaining

Skills can reference other skills in their output:

```markdown
For code review, use `/code-review` first, then `/refactoring` for improvements.
```

Users must invoke each skill separately (no automatic chaining in Phase 8).

### Dynamic Skill Loading

Skills are loaded at startup. To reload skills without restarting:

- Modify config to add/remove roots
- Restart NuimanBot (reload command not yet implemented)

### Version Control

Store custom skills in version control:

```bash
# Create git repository for skills
cd data/skills/users/cli_user/
git init
git add .
git commit -m "Initial skill collection"
```

Benefits:
- Track skill evolution over time
- Share skills across team members
- Rollback problematic changes

### Skill Templates

Create skill templates for common patterns:

```bash
# Template structure
data/skills/templates/
└── basic-skill/
    └── SKILL.md
```

Copy and customize for new skills:

```bash
cp -r data/skills/templates/basic-skill data/skills/users/cli_user/my-new-skill
# Edit SKILL.md with your content
```

## FAQ

**Q: Can skills execute code or commands?**
A: No. Skills are prompt templates that guide the LLM. They cannot execute code directly. The LLM may use tools if allowed.

**Q: How many skills can I create?**
A: No hard limit. Practical limit is around 100-200 skills before /help output becomes unwieldy.

**Q: Can I disable built-in tools within a skill?**
A: Yes, use `allowed-tools: []` to disable all tools, or list only specific allowed tools.

**Q: Can skills call other skills?**
A: Not automatically. Skills can suggest other skills in their output, but users must invoke them separately.

**Q: Are skills shared across platforms?**
A: Shared skills (in `data/skills/shared/`) are available on all platforms. User-specific skills are isolated by platform and user ID.

**Q: How do I update a skill?**
A: Edit the SKILL.md file and restart NuimanBot to reload.

**Q: Can I use markdown in skill bodies?**
A: Yes! Skill bodies are markdown and can include formatting, code blocks, lists, etc.

**Q: What's the maximum skill body size?**
A: No hard limit, but keep skills under 10KB for performance. Large skills increase LLM processing time.

## Additional Resources

- [Anthropic Agent Skills Standard](https://github.com/anthropics/anthropic-skills)
- [Technical Documentation](technical-details.md)
- [Product Details](product-details.md)
- [Example Skills](../data/skills/shared/)

## Support

For issues or questions:
- Check logs: NuimanBot outputs skill parsing errors
- Review configuration: `config.yaml` skills section
- Validate YAML: Use online YAML validator for frontmatter
- Report bugs: GitHub issues
