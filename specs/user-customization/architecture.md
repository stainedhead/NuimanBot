# User Customization - System Architecture

**Created:** 2026-02-15
**Version:** 1.0
**Status:** Draft
**Last Updated:** 2026-02-15

---

## Architecture Overview

**High-Level Summary:**
The User Customization feature adds per-user persona files (SOUL.md, USER.md, RULES.md) that inject customized instructions into the system prompt. Memory writes are explicit actions requiring user confirmation. All file operations are confined to user data directories with strict security validation.

**Architectural Style:** Clean Architecture + Explicit Memory Writes

**Key Principles:**
- Dependency Inversion: Outer layers depend on inner layers
- Explicit over implicit: Memory writes are visible tool calls, not "magic"
- Security first: Path traversal prevention, RBAC, audit logging
- File-based: Markdown files for human readability and version control

**Architecture Diagram:**
```
┌─────────────────────────────────────────────────────┐
│            Infrastructure Layer                      │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────┐   │
│  │FileRepository│  │RulesParser   │  │AuditLog │   │
│  │(filesystem)  │  │(YAML)        │  │         │   │
│  └──────────────┘  └──────────────┘  └─────────┘   │
└────────────────┬────────────────────────────────────┘
                 │ implements
┌────────────────▼────────────────────────────────────┐
│            Use Case Layer                            │
│  ┌───────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │PromptComposer │  │RulesEnforcer │  │MemoryWritr││
│  │               │  │              │  │           │ │
│  │OnboardingWizard│                                  │
│  └───────────────┘  └──────────────┘  └──────────┘ │
└────────────────┬────────────────────────────────────┘
                 │ uses
┌────────────────▼────────────────────────────────────┐
│              Domain Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │PersonaFile│  │RulesConfig│  │MemoryAction│        │
│  │Repository │  │           │  │          │        │
│  │Interface  │  │           │  │          │        │
│  └──────────┘  └──────────┘  └──────────┘          │
└─────────────────────────────────────────────────────┘
```

---

## Component Architecture

### PromptComposer Service

**Responsibility:** Build SystemPrompt from persona files and global policy

**Inputs:**
- UserID
- Platform (Slack/Telegram/CLI)
- AgentPreferences

**Process:**
1. Load persona files (SOUL.md, USER.md, RULES.md) from repository
2. Apply token budget and truncate if necessary
3. Compose SystemPrompt in order: Global Policy → RULES → SOUL → USER → Operational Notes

**Outputs:**
- SystemPrompt (string)
- TokensUsed (int)
- Truncated (bool)

**For detailed architecture, see sections below**

---

## Data Flow

### Prompt Composition Flow

```
User Request → ChatService
                  ↓
           PromptComposer.Compose()
                  ↓
           PersonaFileRepository.List()
                  ↓
           [Load SOUL.md, USER.md, RULES.md]
                  ↓
           [Apply Token Budget & Truncate]
                  ↓
           [Compose: Policy + RULES + SOUL + USER]
                  ↓
           SystemPrompt → LLM API
```

### Memory Write Flow

```
User: "Remember I prefer concise answers"
                  ↓
           MemoryWriter.ProposeWrite()
                  ↓
           RulesEnforcer.CheckConfirmation()
                  ↓
           [Ask user: "Confirm write to USER.md?"]
                  ↓
User: "Yes"       ↓
           MemoryWriter.ExecuteWrite()
                  ↓
           PersonaFileRepository.Save()
                  ↓
           AuditLogger.Log()
                  ↓
           [Success: USER.md updated]
```

---

## Sequence Diagrams

### Sequence 1: New User Onboarding

```
User     Adapter   OnboardingWizard   FileRepo   Files
 |          |            |               |        |
 |─request─>|            |               |        |
 |          |─start────>|               |        |
 |          |            |─check files─>|        |
 |          |            |<─not found──|        |
 |          |            |─scaffold───────────>|
 |          |            |<─SOUL.md created────|
 |          |            |<─USER.md created────|
 |          |            |<─RULES.md created───|
 |          |<─complete─|               |        |
 |<─success─|            |               |        |
```

### Sequence 2: Rule Enforcement (Blocked Action)

```
User    Adapter   RulesEnforcer   FileRepo   RULES.md
 |         |           |             |          |
 |─execute─>|          |             |          |
 shell.exec |          |             |          |
 |         |─check───>|             |          |
 |         |          |─load rules─>|          |
 |         |          |             |─read───>|
 |         |          |             |<─content─|
 |         |          |<─rules─────|          |
 |         |          |[parse YAML]           |
 |         |          |[blocked: shell.exec]  |
 |         |<─blocked─|             |          |
 |<─error──|          |             |          |
 "Action blocked by rules"
```

---

## Architectural Decisions

### ADR-001: File-Based Memory (Markdown)

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
Need a way to store persona/rules/context that is:
- Human-readable
- Editable
- Version-controllable
- Auditable

**Decision:**
Use Markdown files (SOUL.md, USER.md, RULES.md) stored in user's data directory.

**Rationale:**
- Markdown is familiar to developers and non-developers
- Easy to edit in any text editor
- Version control with git
- Inspired by OpenClaw's proven approach

**Consequences:**
**Positive:**
- Easy for users to review and edit
- Clear audit trail (file modification timestamps)
- No vendor lock-in (plain text files)

**Negative:**
- File I/O overhead (mitigated with caching)
- No built-in querying (vs. database)

---

### ADR-002: YAML Frontmatter for Hard Rules

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
RULES.md needs both human-readable rules (Markdown) and machine-enforceable rules (parsed and enforced in code).

**Decision:**
Use YAML frontmatter (content between `---` delimiters) for hard rules.

**Rationale:**
- Clear separation: machine-readable (YAML) vs. human-readable (Markdown)
- Familiar pattern from static site generators (Jekyll, Hugo)
- Easy to parse with gopkg.in/yaml.v3

**Consequences:**
**Positive:**
- Hard rules are enforced consistently
- YAML is structured and validated
- Users can see exactly what rules are enforced

**Negative:**
- Users must learn YAML syntax
- Parse errors could break rule enforcement (mitigated with graceful fallback)

---

### ADR-003: Explicit Memory Writes (Confirmation Required)

**Date:** 2026-02-15
**Status:** Accepted

**Context:**
Need a mechanism for agent to update persona/memory files. Options:
- Implicit: Agent writes without user knowledge
- Explicit: Agent proposes write, user confirms

**Decision:**
Explicit memory writes with user confirmation by default.

**Rationale:**
- Transparency: User sees exactly what's being written
- Control: User can reject unwanted changes
- Auditability: Clear record of all changes
- Inspired by OpenClaw's explicit action philosophy

**Consequences:**
**Positive:**
- User trust (no "magic" changes)
- Clear audit trail
- User can review before approval

**Negative:**
- More friction (requires user interaction)
- Slower workflow

**Mitigation:**
- Users can disable confirmation via RULES.md if desired
- Batch multiple writes into single confirmation

---

## Security Architecture

**Security Layers:**
1. **Path Validation (Infrastructure):** Strict allowlist of user data directory
2. **RBAC (Use Case):** Only user and admins can access their files
3. **Audit Logging (Infrastructure):** All file writes logged with timestamp, user, action

**Threat Model:**
- **Path Traversal:** Mitigate with absolute path validation and allowlist
- **Unauthorized Access:** Mitigate with RBAC checks before every file operation
- **Data Leakage:** Mitigate with privacy rules (never_store list)

**Security Controls:**
- Path sanitization and validation before every file operation
- RBAC checks in Use Case layer
- Audit logging in Infrastructure layer
- Encryption at rest (filesystem-level, not application-level)

---

## References

- [spec.md](spec.md) - Feature specification
- [data-dictionary.md](data-dictionary.md) - Data structures
- [plan.md](plan.md) - Implementation plan
- OpenClaw System Prompt: `/opt/homebrew/lib/node_modules/openclaw/docs/concepts/system-prompt.md`
- OpenClaw Memory Research: `/opt/homebrew/lib/node_modules/openclaw/docs/experiments/research/memory.md`

---

**Next:** Complete detailed sequence diagrams, integration points, component diagrams.
