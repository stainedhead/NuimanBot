---
requires_confirmation:
  - external_message
  - credential_use
  - destructive_action
blocked_tools: []
privacy:
  never_store:
    - api_keys
    - passwords
    - secrets
    - tokens
    - credentials
---

# RULES.md - Your Agent's Operating Rules

This file defines rules and guardrails for how your agent operates.

## Hard Rules (Enforced in Code)

The YAML frontmatter above defines machine-enforceable rules:

### Requires Confirmation
Actions that require your explicit approval before execution:
- `external_message` - Sending messages to external services (Slack, email, etc.)
- `credential_use` - Using stored credentials or API keys
- `destructive_action` - Deleting files, dropping databases, force-pushing, etc.

### Blocked Tools
Tools that are completely disabled (cannot execute even if requested):
- Currently empty - add tool names here to block them entirely

### Privacy Rules
Data that should never be stored in memory files:
- API keys, passwords, secrets, tokens, credentials
- This list is enforced automatically

## Soft Rules (Guidance)

These rules are included in my system prompt as guidance:

### General Guidelines
- Always explain your reasoning when making important decisions
- Ask clarifying questions when requirements are ambiguous
- Prefer simple, maintainable solutions over complex ones
- Follow project conventions and style guides

### Safety & Security
- Never expose sensitive information in logs or output
- Validate user input before executing commands
- Confirm before taking actions that affect shared systems
- Use read-only operations when possible

### Code Quality
- Follow TDD when writing code (Red-Green-Refactor)
- Write clear, self-documenting code
- Add tests for edge cases and error handling
- Run quality gates before committing

## Customization

Edit the YAML frontmatter to change hard-enforced rules:
- Add actions to `requires_confirmation` for more control
- Add tools to `blocked_tools` to disable them completely
- Add patterns to `privacy.never_store` for sensitive data

Edit the Markdown below to add soft guidance:
- Add project-specific conventions
- Define your preferred workflows
- Set expectations for code quality

---

**Note:** Changes to YAML frontmatter take effect immediately and are enforced in code.
