# PRD: Per-user SOUL.md / USER.md / RULES.md + explicit memory-write hooks (Nuimanbot)
_Date: 2026-02-14_

## Summary
Add **per-user configuration files** that customize the agent experience and behavior:
- `SOUL.md` — persona/voice/identity instructions
- `USER.md` — user profile/context the agent should know
- `RULES.md` — operating rules/guardrails for how the agent functions (like OpenClaw’s `AGENTS.md` behavior, but per user)

Also add a **memory write mechanism** inspired by OpenClaw’s approach: memory updates are **explicit actions** (tool calls / internal actions), not “magic.”

This is a new feature request PRD intended to be implemented **separately** from the self-organizing memory work.

---

## Background / Research: how OpenClaw handles persona + memory writes
OpenClaw’s “workspace memory” design is Markdown-first:
- A set of stable persona/context files (`SOUL.md`, `USER.md`, etc.) are **injected into the system prompt** (“bootstrap files”).
- Long-term “memory” is maintained as Markdown files (`MEMORY.md`, `memory/YYYY-MM-DD.md`).
- **Writes are explicit**: the agent updates memory by writing to files (append/edit) using file tools.

Two useful OpenClaw references in the local docs:
- System prompt bootstrap files: `/opt/homebrew/lib/node_modules/openclaw/docs/concepts/system-prompt.md`
- Memory research notes (explicit tool-call writes; retain/recall/reflect loop):
  `/opt/homebrew/lib/node_modules/openclaw/docs/experiments/research/memory.md`
  - Key line of intent: “memory writes are explicit tool calls (append/replace/insert), persisted, then re-injected next turn”

**Implication for Nuimanbot:**
- Don’t rely on implicit “the model will remember.”
- Provide a clean, auditable mechanism to store persona/rules/user context and to update them intentionally.

---

## Problem
Nuimanbot currently has:
- `SystemPrompt` support in LLM requests, but it’s effectively hardcoded in `ChatService.ProcessMessage()`.
- A user profile model + agent preferences types, but no file-based persona/rules injection.

As a result:
- “Personality” customization is limited.
- Rules/guardrails are not user-owned and not easily reviewed in a single place.
- There is no consistent workflow for onboarding and maintaining these instructions.

---

## Goals
1) Provide **per-user** persona + context + rules that shape the agent consistently.
2) Make configuration **human-readable, editable, and auditable** (Markdown files).
3) Establish a safe, explicit **memory write pattern** (agent can propose or perform updates via internal actions, under policy and RBAC).
4) Keep the design compatible with Nuimanbot’s Clean Architecture and security posture.

## Non-goals (v1)
- Full long-horizon “self-organizing memory cells/scenes” system (separate PRD).
- Vector DB / embeddings.
- Multi-user collaborative shared persona (can come later).

---

## User stories
- As a user, I want to define my agent’s voice/persona so it feels consistent.
- As a user, I want the agent to know my background/preferences without repeating them every chat.
- As a user, I want to define rules like “don’t send external messages without confirmation” or “always summarize before acting.”
- As an admin, I want to enforce minimum security rules and prevent users from disabling safety controls.
- As a developer, I want a clean injection mechanism that works across Slack/Telegram/CLI.

---

## Proposed design

### 1) Per-user files and storage layout
Each user gets a “home” directory already modeled as `UserProfile.DataDirectory`.

Add three Markdown files per user:
- `<dataDir>/SOUL.md`
- `<dataDir>/USER.md`
- `<dataDir>/RULES.md`

Optional (nice-to-have):
- `<dataDir>/MEMORY.md` and `<dataDir>/memory/YYYY-MM-DD.md` (if we later adopt OpenClaw-style Markdown journaling per user)

### 2) Prompt injection model (Nuimanbot equivalent of OpenClaw bootstrap)
Create a `PromptComposer` (usecase layer) that builds:
- `SystemPrompt` (string)
- optional prelude messages (system/user) if provider supports

Inputs:
- platform (Slack/Telegram/CLI)
- user profile (structured fields)
- agent preferences (communication style/verbosity/format)
- SOUL.md, USER.md, RULES.md contents
- global admin policy (non-overridable)

Output structure (recommended):
- System prompt sections:
  1. Global policy (non-overridable)
  2. RULES.md (user rules)
  3. SOUL.md (persona)
  4. USER.md (user context)
  5. Operational notes (how to use tools, confirmations, etc.)

Token budget:
- Put hard limits (truncate per section) and include “(truncated)” markers.

### 3) RULES.md semantics
RULES.md should be plain English first. But to make it enforceable, support an optional **frontmatter** block (YAML) for machine-enforceable rules.

Example:
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

Enforcement model:
- **Hard rules**: enforced by code (e.g., blocked tools).
- **Soft rules**: injected into prompts and validated post-hoc.

### 4) Memory writes: explicit internal actions (recommended)
Instead of “LLM magically remembers,” implement an internal action/tool pattern:

#### 4.1 Internal actions
Add internal actions that the model can request (similar to tool calls):
- `memory.write_file` (append/replace)
- `persona.update` (edit SOUL/USER/RULES)
- `memory.log_daily` (append bullet to daily log)

Each action:
- is validated (path allowlist confined to user dataDir)
- is audited
- is permissioned via RBAC
- may require confirmation depending on RULES/admin policy

#### 4.2 How the agent decides to write
Two complementary triggers:
1) **User-initiated**: user says “remember this”, “update my preferences”, etc.
2) **Agent-initiated suggestion**: agent proposes a memory update (“Want me to save this preference to USER.md?”) and executes only on confirmation.

This aligns with OpenClaw’s philosophy described in the memory research doc: memory writes as explicit operations.

### 5) Onboarding mode (yes, likely needed)
Add a guided onboarding workflow to populate these files.

Reasons:
- Most users won’t author good persona/rules docs from scratch.
- Consistency matters for safety + user experience.

Design:
- `onboarding.enabled` per user
- A short wizard via chat (Slack/Telegram/CLI) that asks:
  - name/pronouns/timezone
  - preferred tone (friendly/technical)
  - boundaries (external comms, safety)
  - what to remember (role, projects, preferences)
- Generates initial SOUL/USER/RULES files.
- Allows later “/onboard” rerun.

---

## Functional requirements

### FR1 — Create/maintain per-user persona files
- On first user interaction, ensure files exist; if not, scaffold defaults.
- Provide admin/user APIs to view/edit.

### FR2 — Inject persona/rules into prompts
- Every LLM request uses `PromptComposer` output for `SystemPrompt`.
- Must respect token budgets + truncation.

### FR3 — Enforce RULES hard controls
- Blocked tools/actions cannot run even if the LLM requests them.
- Confirmation requirements are enforced at runtime.

### FR4 — Memory write actions
- Implement `memory.write_file` and `persona.update` internal actions.
- Confine writes to user directory.
- Audit all writes.

### FR5 — Onboarding wizard
- Trigger on first interaction if enabled.
- Allow manual invocation.

---

## Security & safety
- Path traversal prevention; strict allowlist.
- RBAC: only the user (and admins) can read/write their files.
- Admin policy is injected and cannot be overridden by user RULES.
- Audit events for every file write/edit.

---

## Open questions / decisions needed
1) Where should per-user files live exactly?
   - Use `UserProfile.DataDirectory` (recommended) vs a global directory keyed by user ID.
2) Should Slack/Telegram users share a single profile? (Probably yes; map by internal user id.)
3) Do we allow the LLM to edit these files autonomously?
   - Recommendation: **default require confirmation** for persona/rules updates.

---

## Milestones
1) PromptComposer + file scaffolding + injection
2) RULES hard enforcement + confirmation gates
3) Memory write internal actions + audit
4) Onboarding wizard

---

## Success metrics
- Users can reliably change persona and see behavior change within 1–2 turns.
- Reduced repetition (“I already told you that”) via USER.md.
- Fewer unsafe actions due to RULES enforcement.
- Auditability: every persona/rules change is attributable.