# User Customization - Specification

**Version:** 1.0
**Created:** 2026-02-15
**Status:** Planning
**Priority:** P0 (Critical)
**Effort:** Large (40-60h)
**PRD Source:** `Feature-Self-Organizing-Memory-PRD.md`

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Goals and Non-Goals](#goals-and-non-goals)
4. [User Requirements](#user-requirements)
5. [System Architecture](#system-architecture)
6. [Scope of Changes](#scope-of-changes)
7. [Breaking Changes](#breaking-changes)
8. [Success Criteria](#success-criteria)
9. [Risks and Mitigation](#risks-and-mitigation)
10. [Timeline](#timeline)
11. [References](#references)

---

## Executive Summary

**What is this feature?**
Per-user configuration files that customize the agent experience and behavior through:
- `SOUL.md` — persona/voice/identity instructions
- `USER.md` — user profile/context the agent should know
- `RULES.md` — operating rules/guardrails for how the agent functions

Additionally, this feature implements an **explicit memory write mechanism** inspired by OpenClaw's approach: memory updates are explicit actions (tool calls / internal actions), not "magic."

**Why now?**
Currently, Nuimanbot has limited persona customization capabilities. The system prompt is effectively hardcoded, and there's no file-based persona/rules injection. Users cannot:
- Customize agent personality consistently across interactions
- Maintain persistent user context without repetition
- Define and enforce operational rules/guardrails
- Audit or version-control their agent's behavior instructions

**Core components to build:**
1. Per-user file storage (SOUL.md, USER.md, RULES.md)
2. PromptComposer (Use Case layer) for prompt injection
3. RULES.md enforcement engine (frontmatter-based hard rules)
4. Memory write internal actions (explicit memory updates)
5. Onboarding wizard for initial file generation

**Impact:**
- Files affected: 15-20 new files, 5-8 modified files
- Database changes: May need user metadata table updates
- Configuration changes: New feature flags, file paths, token limits
- Documentation updates: User guide, admin guide, API docs

**Compatibility:**
- Backward compatible: Existing users get default scaffolded files
- No breaking changes to existing APIs
- Migration: Auto-scaffold files on first interaction after deployment

---

## Problem Statement

### Current State

**Limitations:**
1. SystemPrompt is hardcoded in ChatService.ProcessMessage() - no per-user customization
2. User profile model exists but no file-based persona/rules injection
3. No auditable, version-controlled mechanism for agent behavior instructions
4. No persistent user context - users repeat information across sessions
5. No user-defined rules or safety guardrails

**Use Cases We Can't Support:**
- User wants agent to adopt a specific persona (friendly vs. technical)
- User wants agent to remember their role, projects, preferences
- User wants to enforce rules like "always confirm before external actions"
- Admin wants to enforce minimum security rules across all users
- User wants to audit/track changes to agent behavior over time

**Example Desired Workflow:**
```bash
# User onboarding - scaffold persona files
> /onboard
Agent: I'll help you set up your profile...
[Guided wizard generates SOUL.md, USER.md, RULES.md]

# Agent now uses custom persona consistently
> Help me with this task
Agent: [Responds using SOUL.md voice/persona]

# User updates their preferences
> Remember that I prefer concise answers
Agent: I'll update your USER.md with that preference.
[Explicit memory write to USER.md]

# Next session - agent remembers
> Another task
Agent: [Responds concisely per USER.md preference]
```

**Pain Points:**
- Users must repeat context/preferences every session
- No way to customize agent personality across platforms (Slack/Telegram/CLI)
- No user-controlled guardrails or safety rules
- Behavior changes are implicit and non-auditable

### Why This Matters

**User Impact:**
Users waste time re-explaining preferences and context. Without consistent persona, the agent feels impersonal and unpredictable. No way to enforce safety rules leads to potential mistakes (unwanted external messages, etc.).

**Business Impact:**
Poor user experience reduces retention and engagement. Lack of customization limits enterprise adoption. No audit trail for agent behavior creates compliance risks.

**Technical Debt:**
Hardcoded system prompts are inflexible. Current approach doesn't scale to multi-platform (Slack/Telegram/CLI). No foundation for advanced memory features.

---

## Goals and Non-Goals

### Goals

**Primary Goals:**
1. Enable per-user persona/voice/identity customization via SOUL.md
2. Maintain persistent user context/preferences via USER.md
3. Define and enforce user-controlled rules/guardrails via RULES.md
4. Provide explicit, auditable memory write mechanism
5. Auto-scaffold persona files for new users (onboarding wizard)

**Secondary Goals:**
- Make all persona/rules human-readable, editable, and version-controllable
- Support both soft rules (prompt-based) and hard rules (enforced in code)
- Work consistently across Slack, Telegram, and CLI platforms
- Respect token budgets with smart truncation

### Non-Goals

**Explicitly Out of Scope:**
- Full long-horizon "self-organizing memory cells/scenes" system - Rationale: Separate PRD, distinct feature
- Vector DB / embeddings for memory - Rationale: V1 focuses on file-based memory
- Multi-user collaborative shared persona - Rationale: Can come later as enhancement
- Automatic memory summarization/compression - Rationale: V1 uses explicit writes only

**Future Considerations:**
- Daily/weekly memory logs (MEMORY.md, memory/YYYY-MM-DD.md) per OpenClaw pattern
- Memory search/retrieval (vector embeddings)
- Collaborative persona templates (team sharing)
- Memory compression and archival strategies

---

## User Requirements

### Functional Requirements

#### FR-001: Per-User Persona Files
**Priority:** P0
**Description:** Create and maintain per-user persona files (SOUL.md, USER.md, RULES.md) in user's data directory.

**Acceptance Criteria:**
- [ ] Files auto-scaffold on first user interaction if missing
- [ ] Files stored in UserProfile.DataDirectory
- [ ] Files are valid Markdown format
- [ ] Admin/user can view and edit files via API
- [ ] Files are loaded and injected into system prompt on every LLM request

**User Story:**
```
As a user,
I want my agent to have a consistent persona and know my preferences,
So that I don't have to repeat myself every session.
```

#### FR-002: Prompt Injection via PromptComposer
**Priority:** P0
**Description:** Create PromptComposer (Use Case layer) that builds SystemPrompt from multiple sources.

**Acceptance Criteria:**
- [ ] PromptComposer accepts user profile, agent preferences, persona files
- [ ] Output includes Global Admin Policy (non-overridable)
- [ ] Output includes RULES.md, SOUL.md, USER.md content
- [ ] Respects token budget limits with truncation markers
- [ ] Works across Slack/Telegram/CLI platforms
- [ ] SystemPrompt structure: Global Policy → RULES → SOUL → USER → Operational Notes

#### FR-003: RULES.md Hard Enforcement
**Priority:** P0
**Description:** Parse RULES.md frontmatter (YAML) and enforce hard rules in code.

**Acceptance Criteria:**
- [ ] Support requires_confirmation list (e.g., external_message, credential_use)
- [ ] Support blocked_tools list (prevent execution even if LLM requests)
- [ ] Support privacy.never_store list (e.g., api_keys, passwords)
- [ ] Admin policy overrides user rules (non-negotiable security rules)
- [ ] Violations are logged and audited
- [ ] User-facing error messages when rules block actions

**Example RULES.md:**
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
- Never send messages to third parties without asking me first.
- Prefer concise answers unless I ask for deep detail.
```

#### FR-004: Memory Write Internal Actions
**Priority:** P1
**Description:** Implement internal actions for explicit memory writes.

**Acceptance Criteria:**
- [ ] Internal action: memory.write_file (append/replace content to user file)
- [ ] Internal action: persona.update (edit SOUL/USER/RULES)
- [ ] All writes confined to user's DataDirectory (path allowlist)
- [ ] All writes are audited with timestamp, user, action
- [ ] RBAC: Only user (and admins) can write to their files
- [ ] Confirmation required by default (unless user disables in RULES.md)

**User Story:**
```
As a user,
I want to tell the agent "remember this preference",
So that it updates my USER.md file and remembers it next time.
```

#### FR-005: Onboarding Wizard
**Priority:** P1
**Description:** Guided onboarding workflow to populate initial persona files.

**Acceptance Criteria:**
- [ ] Trigger on first user interaction if files missing
- [ ] Wizard asks: name, pronouns, timezone, preferred tone, boundaries, preferences
- [ ] Generates initial SOUL.md, USER.md, RULES.md from wizard responses
- [ ] Allow manual invocation via /onboard command
- [ ] Support Slack, Telegram, CLI platforms
- [ ] Wizard can be re-run to update files

### Non-Functional Requirements

#### NFR-001: Performance
- Prompt composition must complete in <100ms (excluding LLM call)
- File reads cached (15-minute TTL) to avoid disk I/O on every request
- Token budget enforced: max 4000 tokens for persona/rules content

#### NFR-002: Security
- Path traversal prevention: strict allowlist of user DataDirectory
- RBAC: Only user and admins can read/write their persona files
- Admin global policy cannot be overridden by user RULES.md
- All file writes logged to audit trail

#### NFR-003: Usability
- Files are human-readable Markdown
- Clear documentation/examples for each file type
- Error messages guide users when rules block actions

#### NFR-004: Reliability
- Graceful degradation if files are missing/corrupt (use defaults)
- File parse errors logged but don't crash system
- Backward compatible: existing users get auto-scaffolded files

---

## System Architecture

### High-Level Design

**Architecture Diagram:**
```
┌─────────────────────────────────────────────────────┐
│            Infrastructure Layer                      │
│  ┌──────────────┐  ┌──────────────┐                │
│  │FileRepository│  │AuditLogger   │                │
│  └──────────────┘  └──────────────┘                │
└────────────────┬────────────────────────────────────┘
                 │ implements interfaces
┌────────────────▼────────────────────────────────────┐
│              Adapter Layer                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │CLIHandler│  │SlackAdapter│  │Telegram │          │
│  └──────────┘  └──────────┘  └──────────┘          │
└────────────────┬────────────────────────────────────┘
                 │ uses
┌────────────────▼────────────────────────────────────┐
│            Use Case Layer                            │
│  ┌────────────────────────────────────┐             │
│  │   PromptComposer Service           │             │
│  │   RulesEnforcer Service            │             │
│  │   MemoryWriter Service             │             │
│  │   OnboardingWizard Service         │             │
│  └────────────────────────────────────┘             │
└────────────────┬────────────────────────────────────┘
                 │ uses
┌────────────────▼────────────────────────────────────┐
│              Domain Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │UserProfile│  │PersonaFiles│  │RulesConfig│        │
│  │PersonaFile│  │MemoryAction│  │Interfaces│         │
│  └──────────┘  └──────────┘  └──────────┘          │
└─────────────────────────────────────────────────────┘
```

### Clean Architecture Layers

**Domain Layer:**
- PersonaFile entity (path, content, type: SOUL/USER/RULES)
- RulesConfig value object (parsed YAML frontmatter)
- UserProfile entity (updated with persona file paths)
- PersonaFileRepository interface
- RulesEnforcer interface
- MemoryWriter interface

**Use Case Layer:**
- PromptComposer service (builds SystemPrompt from sources)
- RulesEnforcer service (parses and enforces RULES.md)
- MemoryWriter service (handles memory.write_file, persona.update actions)
- OnboardingWizard service (guided file generation)

**Infrastructure Layer:**
- PersonaFileRepository implementation (filesystem-based)
- AuditLogger implementation (logs all memory writes)
- RulesParser implementation (YAML frontmatter parser)

**Adapter Layer:**
- CLI adapter: integrates with existing CLI handlers
- Slack adapter: integrates with Slack message processing
- Telegram adapter: integrates with Telegram message processing

**For detailed architecture, see:** `architecture.md`

---

## Scope of Changes

### New Files/Packages

**Domain Layer:**
- `internal/domain/personafile.go` - PersonaFile entity
- `internal/domain/personafile_test.go` - Tests
- `internal/domain/rulesconfig.go` - RulesConfig value object
- `internal/domain/rulesconfig_test.go` - Tests
- `internal/domain/memoryaction.go` - MemoryAction entity
- `internal/domain/memoryaction_test.go` - Tests

**Use Case Layer:**
- `internal/usecase/persona/promptcomposer.go` - PromptComposer service
- `internal/usecase/persona/promptcomposer_test.go` - Tests
- `internal/usecase/persona/rulesenforcer.go` - RulesEnforcer service
- `internal/usecase/persona/rulesenforcer_test.go` - Tests
- `internal/usecase/persona/memorywriter.go` - MemoryWriter service
- `internal/usecase/persona/memorywriter_test.go` - Tests
- `internal/usecase/persona/onboardingwizard.go` - OnboardingWizard service
- `internal/usecase/persona/onboardingwizard_test.go` - Tests

**Infrastructure Layer:**
- `internal/infrastructure/persona/filerepository.go` - Filesystem-based repository
- `internal/infrastructure/persona/filerepository_test.go` - Tests
- `internal/infrastructure/persona/rulesparser.go` - YAML frontmatter parser
- `internal/infrastructure/persona/rulesparser_test.go` - Tests
- `internal/infrastructure/audit/logger.go` - Audit logger
- `internal/infrastructure/audit/logger_test.go` - Tests

**Adapter Layer:**
- `internal/adapter/cli/onboarding.go` - CLI onboarding command handler
- `internal/adapter/slack/persona.go` - Slack persona integration
- `internal/adapter/telegram/persona.go` - Telegram persona integration

### Modified Files

- `internal/domain/userprofile.go` - Add persona file paths
- `internal/usecase/chat/service.go` - Integrate PromptComposer, RulesEnforcer
- `internal/config/config.go` - Add persona feature config
- `cmd/nuimanbot/main.go` - Wire up new services
- `README.md` - Document new feature
- `documentation/product-details.md` - Update product requirements

### Database Changes

**Schema Updates:**
```sql
-- Add columns to user_profiles table (if using SQL)
ALTER TABLE user_profiles ADD COLUMN persona_directory TEXT;
ALTER TABLE user_profiles ADD COLUMN onboarding_completed BOOLEAN DEFAULT FALSE;

-- Create audit log table
CREATE TABLE persona_audit_log (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,  -- 'write', 'update', 'delete'
    file_path TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    details TEXT  -- JSON details
);
```

### Configuration Changes

**New Config Fields:**
```yaml
persona:
  enabled: true
  default_directory: "{{user_data_dir}}/persona"
  files:
    - name: "SOUL.md"
      template: "templates/SOUL.md"
    - name: "USER.md"
      template: "templates/USER.md"
    - name: "RULES.md"
      template: "templates/RULES.md"
  token_limits:
    max_total: 4000
    max_per_file: 1500
  cache_ttl: 900  # 15 minutes
  onboarding:
    enabled: true
    auto_trigger: true
```

**Environment Variables:**
- `NUIMANBOT_PERSONA_ENABLED` - Enable/disable persona feature
- `NUIMANBOT_PERSONA_TOKEN_LIMIT` - Max tokens for persona content
- `NUIMANBOT_PERSONA_CACHE_TTL` - Cache TTL in seconds

---

## Breaking Changes

### None Expected

This feature is designed to be fully backward compatible:
- Existing users get auto-scaffolded persona files on first interaction
- Default templates provide sensible defaults (minimal persona)
- Existing system prompt behavior preserved if files are empty
- No changes to existing APIs or command structures

**Migration Path:**
- On deployment: run migration to scaffold persona files for existing users
- Or scaffold on-demand: first interaction after deployment triggers file creation
- Users can opt-out via config (persona.enabled: false)

---

## Success Criteria

### Metrics

**Primary Metrics:**
- User adoption: >80% of active users have customized persona files within 2 weeks
- Reduced repetition: Users report not repeating context (survey or support tickets)
- Rule enforcement: 0 bypassed blocked_tools violations

**Secondary Metrics:**
- Onboarding completion rate: >90% of new users complete onboarding wizard
- File update frequency: Average 2-3 persona updates per user per month

### Acceptance Tests

**Test Scenario 1: New User Onboarding**
```
Given a new user without persona files
When the user sends their first message
Then the onboarding wizard is triggered
And the user completes the wizard
And SOUL.md, USER.md, RULES.md are generated
And subsequent messages use the custom persona
```

**Test Scenario 2: Rule Enforcement**
```
Given a user with RULES.md blocking 'shell.exec'
When the agent attempts to execute shell.exec
Then the action is blocked
And the user receives a clear error message
And the violation is logged to audit trail
```

**Test Scenario 3: Explicit Memory Write**
```
Given a user says "Remember that I prefer concise answers"
When the agent proposes updating USER.md
And the user confirms
Then USER.md is updated with the preference
And subsequent responses are concise
```

### Quality Gates

- [ ] All unit tests passing (>90% coverage)
- [ ] Integration tests passing (all platforms: CLI, Slack, Telegram)
- [ ] E2E tests passing (onboarding, memory writes, rule enforcement)
- [ ] Security review complete (path traversal, RBAC, audit logging)
- [ ] Documentation complete (user guide, admin guide)
- [ ] Performance benchmarks met (prompt composition <100ms)

---

## Risks and Mitigation

### Technical Risks

**Risk 1: Token Budget Overruns**
- **Likelihood:** Medium
- **Impact:** High (increased LLM costs, slower responses)
- **Mitigation:** Enforce hard token limits, truncate with clear markers, cache file content
- **Contingency:** Dynamic truncation algorithm prioritizes RULES > SOUL > USER

**Risk 2: File Parse Errors (Corrupt YAML)**
- **Likelihood:** Medium
- **Impact:** Medium (degraded experience, no hard enforcement)
- **Mitigation:** Validate YAML on write, graceful fallback to soft rules if parse fails
- **Contingency:** Log parse errors, alert user, use previous valid version

**Risk 3: Path Traversal Security Vulnerability**
- **Likelihood:** Low
- **Impact:** High (data breach, unauthorized file access)
- **Mitigation:** Strict allowlist of user DataDirectory, path sanitization, security tests
- **Contingency:** Security review before deployment, penetration testing

### Operational Risks

**Risk 1: User Confusion (Onboarding Complexity)**
- **Likelihood:** Medium
- **Impact:** Medium (poor adoption, support burden)
- **Mitigation:** Clear UX, guided wizard, examples, documentation
- **Contingency:** Provide video tutorials, support templates, FAQ

**Risk 2: Admin Policy Conflicts with User Rules**
- **Likelihood:** Low
- **Impact:** Medium (user frustration if rules are overridden)
- **Mitigation:** Clear precedence: Admin > User, document in onboarding
- **Contingency:** Alert users when admin policy overrides their rules

### Dependencies

**External Dependencies:**
- YAML parsing library (gopkg.in/yaml.v3) - Status: Available
- File I/O (stdlib os/io) - Status: Available

**Internal Dependencies:**
- UserProfile model - Status: Exists, needs extension
- ChatService integration - Status: Needs modification
- Audit logging - Status: Needs implementation

---

## Timeline

### Estimated Duration
50-60 hours (6-8 weeks part-time)

### Phases

**Phase 0: Planning** (Estimated: 6-8 hours)
- Research and spec creation
- Data dictionary and architecture design
- Implementation plan and task breakdown

**Phase 1: Domain Layer** (Estimated: 8-10 hours)
- PersonaFile, RulesConfig, MemoryAction entities
- Repository interfaces
- Unit tests

**Phase 2: Infrastructure Layer** (Estimated: 10-12 hours)
- FileRepository implementation
- RulesParser implementation
- AuditLogger implementation
- Unit and integration tests

**Phase 3: Use Case Layer** (Estimated: 12-15 hours)
- PromptComposer service
- RulesEnforcer service
- MemoryWriter service
- OnboardingWizard service
- Unit tests

**Phase 4: Adapter Layer** (Estimated: 8-10 hours)
- CLI onboarding handler
- Slack/Telegram integration
- ChatService modification
- Integration tests

**Phase 5: Testing & Documentation** (Estimated: 6-8 hours)
- E2E tests
- Security testing
- User guide, admin guide
- API documentation

**Phase 6: Deployment & Monitoring** (Estimated: 4-5 hours)
- Migration scripts
- Monitoring setup
- Rollout to staging/production

### Critical Path
[Identify which tasks must complete in sequence]

**For detailed timeline, see:** `plan.md` and `tasks.md`

---

## References

### Internal Documents
- PRD: `Feature-Self-Organizing-Memory-PRD.md`
- Research: `specs/user-customization/research.md`
- Architecture: `specs/user-customization/architecture.md`
- Implementation Plan: `specs/user-customization/plan.md`
- Tasks: `specs/user-customization/tasks.md`

### External Resources
- OpenClaw System Prompt Bootstrap: `/opt/homebrew/lib/node_modules/openclaw/docs/concepts/system-prompt.md`
- OpenClaw Memory Research: `/opt/homebrew/lib/node_modules/openclaw/docs/experiments/research/memory.md`
- YAML Frontmatter Spec: https://jekyllrb.com/docs/front-matter/
- Clean Architecture: https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html

### Related Features
- Self-Organizing Memory (separate PRD, future work)
- Multi-platform Support (Slack, Telegram, CLI)

---

**Next Steps:**
1. Review and approve this specification
2. Create research.md (OpenClaw integration, YAML parsing)
3. Create data-dictionary.md (PersonaFile, RulesConfig, etc.)
4. Create architecture.md (detailed design, sequence diagrams)
5. Create plan.md (implementation approach, phases)
6. Create tasks.md (breakdown into tasks)
7. **Update status.md** - Mark Phase 0 complete, begin implementation
