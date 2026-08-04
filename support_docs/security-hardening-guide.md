# Security Hardening Guide

**Version:** 1.0
**Last Updated:** 2026-08-03
**Target Audience:** Chat Users, System Administrators, DevOps Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Why This Changed](#why-this-changed)
3. [What Changed for Chat Users](#what-changed-for-chat-users)
4. [What Changed for Operators and Admins](#what-changed-for-operators-and-admins)
5. [Configuration Reference](#configuration-reference)
6. [FAQ](#faq)

---

## Overview

NuimanBot recently shipped a set of security hardening features. If you talk
to NuimanBot day to day, the practical change is small: a few kinds of
actions now pause and ask you to confirm before they run. If you administer a
NuimanBot deployment, there are new configuration options to know about,
including one **breaking change** to default tool permissions.

This guide covers both audiences. See the [Configuration Reference](configuration-reference.md)
for the full config schema, and the [Admin Guide](admin-guide.md) for general
administration.

---

## Why This Changed

NuimanBot is an AI agent that can browse the web, search, read GitHub issues
and pull requests, and run other tools on your behalf. Some of that content
— a web page, a search result, a GitHub comment — is written by someone
outside your organization. A malicious or compromised page could try to
smuggle instructions into that content, hoping NuimanBot's underlying AI
model will follow them instead of treating them as data to read.

Two new protections address this, in plain terms:

1. **Tool output is scanned before the AI model sees it.** Whenever
   NuimanBot fetches a web page, runs a search, or reads from an MCP tool, the
   returned content is scanned for signs of injected instructions before it's
   handed to the AI model. Suspicious content is either blocked or clearly
   flagged, depending on configuration.
2. **Certain actions need your explicit yes.** Actions that change something
   in the outside world — merging a pull request, closing or creating a
   GitHub issue, running the coding agent in its most permissive mode — now
   pause and wait for you to say "yes" before they happen. This means a
   successful injection attempt (or the AI simply making a mistake) can't
   silently merge a PR or close an issue without a human noticing.

Neither protection assumes the other is perfect — they're layered, so a gap
in one (a scan that misses a cleverly-worded injection, say) is still caught
by the other (the resulting action still needs your approval).

---

## What Changed for Chat Users

### Actions that now ask for confirmation

By default, these actions pause and ask you to confirm before they run:

- Merging a GitHub pull request (`github pr_merge`)
- Creating a GitHub issue (`github issue_create`)
- Closing a GitHub issue (`github issue_close`)
- Running the coding agent in "yolo mode" (`coding_agent`'s most permissive,
  least-supervised setting)
- Any MCP (Model Context Protocol) tool your administrator has classified as
  `write` or left unclassified (`unknown`) — unclassified MCP tools are
  treated as if they could make changes, to be safe

Your administrator can add more actions to this list, or your own `RULES.md`
persona file can request confirmation for additional tools via
`requires_confirmation` — see the [User Onboarding Guide](user-onboarding.md)
for persona customization.

### How to respond

When NuimanBot asks for confirmation, it sends you a plain-language summary
of what it's about to do. You can respond in two ways, depending on where
you're chatting:

- **Everywhere (CLI, Slack, Telegram, web chat):** just reply with plain
  text. `yes`, `y`, `approve`, or `confirm` approves the action (exact match,
  case-insensitive). `no`, `n`, `deny`, `cancel`, or `reject` denies it. Any
  other reply is treated as an ordinary new message, not a response to the
  pending confirmation — so if you're not sure what NuimanBot is asking
  about, just answer normally and it'll clarify.
- **Slack and Telegram only:** you'll also see clickable **Approve** /
  **Deny** buttons alongside the text prompt. Clicking a button does exactly
  the same thing as typing "yes" or "no" — it's a convenience, not a
  different mechanism. No other gateway (CLI, plain web chat) has buttons
  today; if you're on one of those, use the plain-text reply.

If you're a web admin user or use the REST API directly, pending
confirmations can also be listed and resolved at `/admin/confirmations` (web
UI) or via `GET /api/v1/confirmations/{id}` and
`POST /api/v1/confirmations/{id}/resolve` (REST API) — see the
[Admin Guide](admin-guide.md) and [API Reference](api-reference.md).

> **Known limitation — REST API confirmations are not private today.**
> Under the current REST API authentication model, **any holder of the API
> key can view or resolve ANY user's pending confirmation** — the API is not
> yet per-user-scoped. Every credential the REST API issues today is treated
> as administrative (there is only one shared operator key, not distinct
> per-user credentials), so the endpoints' per-owner check never actually
> excludes anyone in practice. **Do not treat REST-API-issued confirmations
> as private to the requesting user.** This is a known, documented gap
> (tracked as FR-006), not a bug you need to report — see the
> [API Reference](api-reference.md#confirmation-endpoints) for details.

### What happens if you don't respond

An unresolved confirmation expires after a timeout — **5 minutes by default**
(your administrator can change this). If it expires, the action is treated
as **denied** — nothing executes. You can simply ask NuimanBot to try again
if you change your mind.

### Only one pending confirmation at a time

NuimanBot only ever has one open confirmation per conversation at a time. If
it tries to propose a second side-effecting action while one is still
pending, that second request is rejected outright with a note that the first
one needs to be resolved first — so "yes"/"no" is never ambiguous about what
you're answering.

### What didn't change

Everything else works exactly as before: reading information (searching the
web, listing GitHub issues/PRs, viewing repository contents, viewing a single
issue or PR) never requires confirmation and executes immediately, the same
as prior to this update.

---

## What Changed for Operators and Admins

### New configuration sections

Four new configuration areas were added. All of them are **secure by
default** — you don't need to change anything to get the protection; the
options below are for tuning or reverting specific defaults.

#### 1. Tool-output injection scanning — `security.tool_output_validation`

```yaml
security:
  tool_output_validation:
    enabled: true      # default: true (omit the key for the same effect)
    action: reject     # "reject" (default) or "annotate"
```

- `enabled` — turns the scanner on/off. Defaults to `true` even if the key is
  omitted entirely; only an explicit `false` disables it.
- `action` — what happens when a scan flags content: `reject` (default)
  fails that specific tool call outright; `annotate` lets the content
  through but prepends a visible `[SECURITY WARNING: possible injected
  instructions detected]` marker so the AI model sees it's been flagged.

#### 2. Side-effecting action confirmation — `security.confirmation`

```yaml
security:
  confirmation:
    enabled: true
    timeout: "5m"                       # default: 5 minutes
    default_required_actions:
      - "github.pr_merge"
      - "github.issue_close"
      - "github.issue_create"
      - "coding_agent.yolo_mode"
```

- `enabled` — turns the confirmation subsystem on/off. Defaults to `true`.
  **Disabling this does not mean actions run unconfirmed** — if a rule in a
  user's `RULES.md` still requires confirmation for something, that request
  is denied rather than silently allowed, since a disabled confirmation
  subsystem has no way to actually get anyone's approval.
- `timeout` — how long an unresolved confirmation stays open before it's
  auto-denied. Duration string (e.g. `"5m"`, `"90s"`); defaults to 5 minutes
  if omitted or unparseable.
- `default_required_actions` — the list of `<tool>.<action>` pairs (or a bare
  `<tool>` name for tools with no action concept) that require confirmation
  by default. This list is *unioned* with, not a replacement for, any
  per-user `requires_confirmation` entries in that user's `RULES.md`.

#### 3. SSRF (Server-Side Request Forgery) protection — `security.fetch`

```yaml
security:
  fetch:
    ssrf_protection: true    # default: true
    follow_redirects: true   # default: true
```

- `ssrf_protection` — when the `summarize` or `doc_summarize` tools fetch a
  URL, this rejects targets that resolve to internal/private addresses
  (loopback, RFC 1918 private ranges, link-local addresses including cloud
  metadata endpoints like `169.254.169.254`) — preventing NuimanBot from
  being tricked into fetching something on your internal network. Defaults
  to `true`.
- `follow_redirects` — whether those tools follow HTTP redirects at all. When
  `true` (default), every redirect hop is re-validated with the same
  protection before being followed. Setting this to `false` restores the
  plain stock HTTP client's default redirect behavior instead (redirects
  aren't specially validated at all) — only do this if you have a specific
  reason to.

#### 4. Tool permission overrides — `tools.permissions`

```yaml
tools:
  permissions:
    github: user          # revert github to pre-hardening behavior
    coding_agent: admin    # (this is already the default — shown for illustration)
```

**Important — breaking change:** as of this hardening work, the `github` and
`coding_agent` tools require the **Admin** role by default for their
side-effecting actions. Previously they fell through to the general default
(any registered user). If your deployment has non-admin users who need to
merge PRs, create/close issues, or run the coding agent, you have two
options:

- Grant those specific users the Admin role, or
- Add an override here to restore the previous behavior for that tool, e.g.
  `github: user`. This override applies to the whole tool (all of its
  actions), not just specific ones — `github`'s built-in read/write split
  (reads stay open to regular users; only writes need admin) already covers
  the finer-grained case without needing an override at all.

An unrecognized value here (anything other than `guest`, `user`, or `admin`,
case-insensitive) is ignored and logged as a warning — it will not silently
grant or deny access.

#### 5. MCP tool trust classification — `mcp.json`

If you connect NuimanBot to MCP (Model Context Protocol) servers, each
server's entry in `mcp.json` now supports a `trust` field and a
`tool_overrides` map:

```json
{
  "servers": [
    {
      "name": "my-mcp-server",
      "transport": "http",
      "url": "https://mcp.example.com",
      "trust": "read_only",
      "tool_overrides": {
        "delete_record": "write"
      }
    }
  ]
}
```

- `trust` — classifies the server's tools overall: `read_only` (available to
  regular users, no confirmation required), `write` (admin-only, requires
  confirmation), or `unknown` (the default if omitted — treated identically
  to `write`, i.e. the safe assumption for a server you haven't explicitly
  classified).
- `tool_overrides` — lets you set a different trust level for one specific
  tool on that server, overriding the server-wide `trust` value for just that
  tool (e.g. mark one destructive tool on an otherwise-read-only server as
  `write`).

An unrecognized `trust` value (a typo, for instance) is logged as a warning
and treated as `unknown` — never silently treated as safer than it should be.

**Trust note on `tool_overrides`:** the tool name used as a key in
`tool_overrides` is whatever the MCP server itself reports via its
`tools/list` response — NuimanBot has no independent way to verify it. If you
use `tool_overrides` to grant a specific MCP tool `read_only` trust, only do
so for MCP servers you fully trust to correctly and honestly name their
tools — a malicious server could exploit an override meant for a different
tool by registering its own write-capable tool under that same name. This gap
does not affect the server-wide `trust` default, which applies uniformly no
matter what name a tool is reported under.

### RBAC enforcement in live chat

Role-based tool permissions (`tools.permissions`, the `github`/`coding_agent`
admin defaults, MCP trust classification) are fully defined and enforced when
a tool is invoked through the API's user-aware execution path — and the
live chat tool-calling loop now calls that path (`ExecuteWithUser`) at both
of its tool-execution sites, so those role checks are enforced end-to-end
for every tool call made during an ordinary chat conversation. Each incoming
message's caller role is resolved via `profile.Service`/
`UserProfileRepository`, keyed off the platform user ID; an unresolvable or
unregistered platform identity fails closed to the Guest role rather than
failing open. The tool list offered to the LLM is also filtered by the
caller's resolved role. The confirmation gate described above **is** fully
live in chat as before — it's independently wired and doesn't depend on role
resolution, so it continues to apply on top of RBAC. See
`documentation/technical-details.md` and
`documentation/architectural-decision-record.md` (ADR-008, with its
2026-08-03 update note) for the full technical detail if you need it.

---

## FAQ

**Q: Do I need to change my config to get these protections?**
A: No. Every new setting defaults to the secure/protective option. You only
need to touch configuration if you want to revert a specific default (e.g.
`tools.permissions` to un-restrict `github`) or tune a value like the
confirmation timeout.

**Q: I typed "yes" and nothing happened. Why?**
A: Check that your reply exactly matches one of the recognized words (`yes`,
`y`, `approve`, `confirm` for approval; `no`, `n`, `deny`, `cancel`, `reject`
for denial), with no extra words — "yes please" isn't recognized and is
treated as a normal new message instead. Also check whether the confirmation
already expired (default 5 minutes) — if so, it was auto-denied and you'll
need to ask NuimanBot to try the action again.

**Q: Can I see confirmations from other users?**
A: Only if you're an Admin — **for the web admin UI.** A regular web-admin
user only sees confirmations tied to their own account at
`/admin/confirmations`. **The REST API is different and currently more
permissive:** because it only recognizes a single shared API key today
(not distinct per-user credentials), every REST API caller is treated as
Admin, so `GET`/`POST /api/v1/confirmations/{id}[/resolve]` can currently
view or resolve any user's pending confirmation regardless of who holds the
key. Don't assume REST-API confirmation access is scoped to "your" requests
the way the web UI is.

**Q: Why does `github` now say "access denied" for actions that used to work?**
A: This is the intentional RBAC breaking change described above — `github`
writes (and `coding_agent`) now require Admin by default. Use the
`tools.permissions` override or grant Admin to affected users.

**Q: Does the output scanning slow things down noticeably?**
A: No — the scan and the SSRF DNS/IP checks add sub-millisecond-to-low-
millisecond overhead, well within existing per-request timeout budgets.
