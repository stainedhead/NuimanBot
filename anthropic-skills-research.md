# Anthropic‑style “Skills” (saved prompts) for a Go agent bot — research + implementation pointers

## Executive summary
Anthropic’s “Skills” (as used in Claude Code and Claude.ai) are **file-based, discoverable instruction packages** that an agent can load on-demand. They behave like a **prompt registry + policy + packaging format**, not a new model capability.

The most reusable core is the **Agent Skills open standard**: a skill is a folder that contains a `SKILL.md` with **YAML frontmatter** (metadata like `name`, `description`, optional constraints) and a Markdown body (instructions). Supporting files (references, templates, scripts) can live alongside it to enable *progressive disclosure*.

For your Go bot, you can implement this as:
- a **skill discovery + indexer** (scan directories, parse frontmatter, build an in-memory registry)
- a **skill selection policy** (model-chooses vs user-invokes)
- a **skill activation renderer** (inject the chosen SKILL.md body + referenced context into the prompt)
- optional **subagent execution** / sandboxing (run skill tasks in an isolated context)
- optional **preprocessing / dynamic injection** (render placeholders, run safe commands to materialize context)

If you align with the Agent Skills format now, you can later share skills across tools (Claude Code, other agent products) with minimal changes.

## What Anthropic “Skills” are (capability model)

### 1) Packaging + discovery
From Claude Code docs (“Extend Claude with skills”): skills live at multiple scopes and are **discovered from the filesystem**:
- Project: `.claude/skills/<skill-name>/SKILL.md`
- Personal: `~/.claude/skills/<skill-name>/SKILL.md`
- Enterprise-managed settings (org-wide)
- Plugins: `<plugin>/skills/<skill-name>/SKILL.md` (namespaced)

Claude Code also supports discovery from **nested directories** (monorepo-friendly): if you’re working in a subdirectory, it looks for `.claude/skills/` in that subtree as well.

**Implementation takeaway (Go):** treat skills as a set of **roots** (project root + user home + configured extra dirs). Each root has priority ordering and optional namespacing.

### 2) Metadata-driven routing
Both Agent Skills spec and Claude Code behavior emphasize:
- `description` is crucial: it’s what the model uses to decide relevance.
- Only a small “index” (name + description) needs to be loaded globally; full content loads only when activated.

**Implementation takeaway:** build a **two-tier context strategy**:
- Always include a compact “skills catalog” (name + description + invocation hints) in the model context.
- Load full SKILL.md content only when selected.

### 3) Invocation control / permissions
Claude Code adds fields beyond the base spec:
- `disable-model-invocation: true` — only user can run it (workflows with side-effects)
- `user-invocable: false` — hidden from user menus, model can still use it as background knowledge
- `allowed-tools:` — allowlisted tools while skill is active
- `context: fork` + `agent: Explore/Plan/...` — run in a subagent/forked context

**Implementation takeaway:** implement a policy layer:
- **Who may activate** a skill: user, model, or both.
- **What tools/actions** become available while the skill is active.

### 4) Rendering / placeholders / dynamic context
Claude Code supports simple argument substitution:
- `$ARGUMENTS`, `$ARGUMENTS[N]`, `$0`, `$1`, etc.
- `${CLAUDE_SESSION_ID}`

It also supports a preprocessing pattern where `!` command blocks are executed **before** the prompt is sent to the model, and the output is embedded (Claude sees only the rendered prompt).

**Implementation takeaway:** build a deterministic **skill renderer**:
- expand arguments safely
- optionally evaluate preprocessor directives (only if you can sandbox + audit)

## The Agent Skills open standard (portable core)
A compliant skill is:

```
skill-name/
└── SKILL.md
```

`SKILL.md` is:
- **YAML frontmatter** between `---` lines
- followed by **Markdown instructions**

Spec-required fields:
- `name` (lowercase, hyphens, <= 64 chars, matches directory)
- `description` (<= 1024 chars)

Optional fields (spec):
- `license`, `compatibility`, `metadata`, (experimental) `allowed-tools`

It recommends *progressive disclosure*:
- Keep SKILL.md relatively short
- Put heavy docs in referenced files in the skill folder

**Go takeaway:** you can implement the spec without any Anthropic dependencies.

## Design options for a Go bot
Below are three increasing-sophistication designs.

### Option A — “Prompt library” (manual invocation only)
- You store skill folders in a repo.
- User types `/<skill> args...`.
- Bot loads `SKILL.md`, renders args, and prepends it to the request.

Pros: simple, safe. Cons: no automatic routing.

### Option B — “Model-routed skills” (Anthropic-like)
- At startup (or on file watch), scan skills and create a catalog (name + description).
- For each user message, ask the model to choose skills (or none).
- Activate chosen skill by loading full instructions.

Pros: closest to Claude Code. Cons: requires careful prompt design to prevent overuse.

### Option C — “Skill tool” (structured routing)
- Add a first stage “router” LLM call that outputs structured JSON:
  - chosen skill name
  - arguments
  - confidence
  - whether to ask user confirmation
- Then a second stage does the actual task with that skill injected.

Pros: auditable and controllable. Cons: extra latency/cost.

## Concrete Go implementation pointers

### 1) Directory layout (recommended)
Mirror Claude Code / Agent Skills:

```
.claude/skills/
  deploy/
    SKILL.md
    references/...
    scripts/...
  api-conventions/
    SKILL.md
```

Also support:
- `~/.claude/skills/...` for user-wide skills
- `--skills-dir` or config to add more roots

### 2) Parsing SKILL.md frontmatter
Use a YAML frontmatter parser:
- Split on the first two `---` markers.
- Parse frontmatter YAML with `gopkg.in/yaml.v3`.

Define a struct:

```go
type SkillFrontmatter struct {
  Name        string            `yaml:"name"`
  Description string            `yaml:"description"`
  License     string            `yaml:"license,omitempty"`
  Compatibility string          `yaml:"compatibility,omitempty"`
  Metadata    map[string]string `yaml:"metadata,omitempty"`

  // Claude Code extensions you may choose to support:
  DisableModelInvocation bool   `yaml:"disable-model-invocation,omitempty"`
  UserInvocable          *bool  `yaml:"user-invocable,omitempty"`
  AllowedTools           string `yaml:"allowed-tools,omitempty"` // parse into []ToolRule
  Context                string `yaml:"context,omitempty"`       // e.g., "fork"
  Agent                  string `yaml:"agent,omitempty"`         // e.g., "Explore"
}
```

Validate:
- `name` matches directory basename
- naming rules (lowercase + hyphens)
- description length

### 3) Registry + priority
Model a registry entry:

```go
type Skill struct {
  ID          string // scope:name or plugin:skill
  Name        string
  Description string
  Root        string // directory root
  Dir         string // absolute path to skill folder
  BodyMD      string // markdown body
  Frontmatter SkillFrontmatter
  Scope       string // enterprise|user|project|plugin
  Priority    int
}
```

Resolve conflicts:
- enterprise > user > project
- plugin skills are namespaced; don’t conflict

### 4) Skill selection / routing prompt
To replicate “Anthropic-style” behavior, include a catalog snippet:

- Provide the model a list:
  - name
  - description
  - whether user-invocable
  - whether model-invocable

Ask it to output a structured choice:

```json
{ "skills": [{"name": "api-conventions", "args": "", "reason": "..."}], "ask_user_confirmation": false }
```

Then, for chosen skills:
- load SKILL.md bodies
- prepend or place in a dedicated system/developer “skills” block

**Important:** don’t just “dump all skills” into every context; it wastes tokens and increases accidental activation.

### 5) Rendering arguments + substitutions
Support:
- `$ARGUMENTS` (raw trailing text)
- `$ARGUMENTS[N]` / `$N`

Implementation:
- tokenize args by shell-like parsing (or keep raw + simple split)
- do deterministic string replacement

### 6) Supporting files + progressive disclosure
Anthropic suggests using supporting files so SKILL.md stays short.
In your own bot, you can implement:
- “reference links” in Markdown that your runtime can follow, e.g. `[reference.md](reference.md)`
- a “skill loader” that only loads referenced files when the model requests them.

Simpler approach:
- expose a tool/function: `read_skill_file(skill, path)` that is restricted to within the skill directory.

### 7) Tools policy / sandboxing
The dangerous part is *skills that execute scripts*.
If you support scripts (like Claude Code examples), add guardrails:
- explicit `allowed-tools`/allowlist per skill
- require user confirmation for side-effects
- run scripts in a sandbox (container, restricted user, no secrets)
- log every tool call with skill name + args

### 8) Versioning + distribution
Treat skills as **code**:
- keep in git
- semantic version via `metadata.version`
- optionally a `skills.lock` in projects pinning versions

Also consider “plugin/marketplace” style:
- skills in a repo, installed into a local skill root

## Anthropic‑style “Agents” / Subagents (saved prompts + isolated context)

Anthropic’s “subagents” (as implemented in Claude Code) are **specialized agent profiles** that run in their **own context window** with:
- a dedicated **system prompt** (the subagent’s markdown body)
- constrained **tool access** (allowlist/denylist)
- optional different **model**
- independent **permission mode**
- optional **preloaded skills** (full skill contents injected at startup)
- optional **persistent memory directory**

Conceptually:
- **Skills** = instruction packs you *load into an agent’s context* to shape behavior on a task.
- **Subagents** = separate “worker processes” with their own prompt + policies, used to keep noisy work out of the main thread and enforce constraints.

### Key behaviors worth copying
From Claude Code docs (“Create custom subagents”):
- **Delegation is description-driven**: the main agent decides to delegate based on each subagent’s `description`.
- Subagents can run **foreground** (interactive) or **background** (concurrent), with different limitations.
- Subagents can be **resumed** later; transcripts persist separately from the main conversation.
- There are built-in subagents (e.g., Explore, Plan, general-purpose) that demonstrate useful defaults:
  - *Explore*: fast, read-only (denies Write/Edit) for codebase discovery.

### File format / configuration
Subagents are defined similarly to skills: **Markdown + YAML frontmatter**.
Example shape:

```md
---
name: code-reviewer
description: Expert code review specialist. Use proactively after code changes.
tools: Read, Glob, Grep, Bash
model: inherit
maxTurns: 12
skills:
  - api-conventions
memory: project
---

You are a senior code reviewer...
```

Supported frontmatter fields (Claude Code):
- `name`, `description` (required)
- `tools`, `disallowedTools`
- `model` (sonnet/opus/haiku/inherit in Claude Code)
- `permissionMode` (default/acceptEdits/dontAsk/delegate/bypassPermissions/plan)
- `maxTurns`
- `skills` (preload full content; subagents **don’t inherit** parent skills)
- `hooks` (lifecycle hooks)
- `memory` (persistent memory scope: user/project/local)

### What “saved prompts” means here
Subagents are essentially **saved system prompts** with additional operational policy.
So if your goal is “saved prompts,” subagents are a natural home for:
- “reviewer”, “researcher”, “refactorer”, “incident triage”, etc.

Skills and subagents combine two ways:
1) **Skill → subagent execution** (skills with `context: fork`): the skill content becomes the task prompt for a chosen subagent type.
2) **Subagent → preloaded skills**: a subagent can preload a curated set of skills to specialize it.

### Go implementation pointers (subagents)

#### 1) Define an AgentProfile type

```go
type AgentProfile struct {
  Name        string   `yaml:"name"`
  Description string   `yaml:"description"`
  Model       string   `yaml:"model,omitempty"`           // or your provider model id
  Tools       []string `yaml:"tools,omitempty"`
  Disallowed  []string `yaml:"disallowedTools,omitempty"`
  PermissionMode string `yaml:"permissionMode,omitempty"`
  MaxTurns    int      `yaml:"maxTurns,omitempty"`
  Skills      []string `yaml:"skills,omitempty"`          // names to preload
  MemoryScope string   `yaml:"memory,omitempty"`          // user|project|local
  Hooks       any      `yaml:"hooks,omitempty"`           // optional
  PromptMD    string   // markdown body
}
```

Parse similarly to SKILL.md: frontmatter + body.

#### 2) Agent selection: router then run
Use a two-step orchestration:
- Router step decides: **use main agent** vs **delegate to subagent X**.
- Execution step runs chosen profile with its own context window.

You can implement subagents without multiple models by just:
- launching a fresh conversation state (new message history)
- seeding it with the subagent system prompt
- optionally injecting preloaded skills (full text)

#### 3) Context isolation and return contract
Define a strict return contract for subagents:
- subagent produces a **summary + artifacts** (file diffs, commands to run, links)
- main agent decides how to apply it

This mirrors Claude Code’s motivation: keep verbose logs/tests/research out of the main context.

#### 4) Tools + permissions enforcement
Treat `tools`/`disallowedTools` as an allow/deny list in your tool dispatcher.
Add a “permissionMode” concept:
- `default`: ask user before risky tools
- `dontAsk`: auto-deny anything not explicitly allowed
- `acceptEdits`: auto-approve file edits (dangerous; use with caution)

#### 5) Memory
Implement `memory` as a directory your subagent can read/write:
- `user`: `~/.yourbot/agent-memory/<agent-name>/`
- `project`: `<repo>/.yourbot/agent-memory/<agent-name>/`
- `local`: `<repo>/.yourbot/agent-memory-local/<agent-name>/` (gitignored)

Then:
- seed the subagent prompt with a short excerpt of `MEMORY.md`
- enforce size limits and require it to curate

#### 6) Background execution / concurrency
Claude Code distinguishes foreground vs background.
In Go, you can implement:
- foreground: synchronous call with interactive tool approvals
- background: run in a goroutine/worker pool with **pre-approved tool set**; deny any “ask user” tool.

#### 7) Transcripts and resumability
Persist subagent transcripts separately (e.g., JSONL):
- enables resuming and debugging
- enables offline evaluation

### When to use subagents vs skills
- Use **skills** when you want reusable guidance applied inline (or a user-invoked workflow).
- Use **subagents** when you want isolation, specialized policy/tool limits, or to keep high-volume output out of the main thread.

## Sample repos / references (regardless of language)

### Directly relevant (Anthropic / Agent Skills / Claude Code)
- Claude Code Docs — Subagents: https://code.claude.com/docs/en/sub-agents
- Claude Code Docs — Skills: https://code.claude.com/docs/en/skills
- Claude Code Docs — Plugins: https://code.claude.com/docs/en/plugins
- Agent Skills spec + reference implementation: https://github.com/agentskills/agentskills
- Spec site: https://agentskills.io/specification
- Anthropic public skills examples: https://github.com/anthropics/skills

### Similar “agent profile / multi-agent” patterns (not Anthropic)
- Microsoft Semantic Kernel (agent framework + plugins/prompt templates): https://github.com/microsoft/semantic-kernel
- Microsoft AutoGen (multi-agent framework): https://github.com/microsoft/autogen
- CrewAI (role-based multi-agent orchestration): https://github.com/crewAIInc/crewAI
- LangGraph (stateful agent/workflow graphs): https://github.com/langchain-ai/langgraph

### Similar “prompt/skill registry” patterns (not Anthropic)
(Conceptually similar, even if format differs.)
- Microsoft Semantic Kernel “prompt skills” / plugins (prompt templates as files)
- LangChain Hub / prompt template registries
- Continue.dev slash commands / prompt library (IDE-based)

Note: I couldn’t use Brave web_search in this environment (missing API key), so I leaned on direct source URLs and web_fetch/browser access for the primary Anthropic/AgentSkills materials above.

## Recommended path for your Go bot (pragmatic)
1) Implement **Agent Skills spec compatibility** (scan + parse + validate + load).
2) Add **manual invocation** (`/skill-name ...`) first.
3) Add **model-routed selection** second, using only descriptions in the global context.
4) Add **allowed-tools** + confirmations before you allow any execution.
5) Add “plugins” (namespacing + versioned installs) once you need sharing.

## Next steps I can do for you
- If you paste (or point me at) your Go bot repo structure, I can propose:
  - the exact package layout (`skills/` package, interfaces)
  - a routing prompt template
  - example YAML frontmatter + 2–3 sample skills
  - a minimal “skills validator” CLI (`go run ./cmd/skills validate`)

---

## Sources
- Claude Code Docs — Skills: https://code.claude.com/docs/en/skills
- Claude Code Docs — Subagents: https://code.claude.com/docs/en/sub-agents
- Claude Code Docs — Plugins: https://code.claude.com/docs/en/plugins
- Agent Skills spec: https://agentskills.io/specification
- Agent Skills repo: https://github.com/agentskills/agentskills
- Anthropic Skills examples: https://github.com/anthropics/skills
