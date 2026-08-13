# CLI Environments Guide

**Version:** 1.1
**Last Updated:** 2026-08-11
**Target Audience:** NuimanBot Users

This guide covers the CLI's slash-commands for **Chats**, **Projects**, **Jobs**, **Chores**, **History**, and **Memories**, plus the **Settings** command — the same six environments and Settings page documented for the web admin UI in the [Web Workspace Guide](web-workspace-guide.md), now available directly from the terminal REPL. It also covers the CLI's login requirement, introduced alongside these commands — see [Login Required](#login-required-breaking-change) if you're upgrading from an earlier version.

---

## Table of Contents

1. [Login Required (Breaking Change)](#login-required-breaking-change)
2. [Chats](#chats)
3. [Projects](#projects)
4. [Jobs](#jobs)
5. [Chores](#chores)
6. [History](#history)
7. [Memories](#memories)
8. [Settings](#settings)
9. [What's Not Implemented Yet](#whats-not-implemented-yet)

---

## Login Required (Breaking Change)

**As of this release, `./bin/nuimanbot` no longer grants immediate admin access.** Previously, starting the CLI automatically logged you in as a trusted local administrator (`cli_admin`) with no prompt. This has been removed.

Starting the CLI now prompts for a username and password before accepting any command or chat message:

```
$ ./bin/nuimanbot
Starting CLI Gateway...
Username: admin
Password:
Welcome back, admin.
>
```

**What this means for you:**

- You need a real account (the same accounts the web admin UI uses — see [Admin Guide](admin-guide.md) for how the default account is provisioned, and the `admin`/`admin` default credentials if this is a fresh install).
- Your session is saved to a local file (with restrictive OS permissions) so you don't have to log in again every time you restart the CLI within the session's expiry window (24 hours by default).
- Type `/logout` at any time to end your session; the CLI will prompt you to log in again immediately.
- Non-admin accounts can chat and use the six environments below, but cannot run admin-only commands (`/admin`, `/profile`, `/bot`, `/config`, `/memory`, `/settings show --system`, `/settings set worker-pool-size`) — these now correctly check your real role, instead of the old behavior where every CLI session was implicitly an admin.

**If you have scripts or automation that pipe input into `./bin/nuimanbot` non-interactively:** they now need to supply a username and password as the first two lines of input, or the CLI will exit with an authentication error rather than proceeding. Password entry is unmasked in this piped/non-interactive mode (there's no terminal to mask input against) — keep this in mind for anything that might log the input stream.

**Exit-code change for empty/closed stdin, including `./bin/nuimanbot --help`:** `--help` has never been parsed as a real flag by this CLI — running `./bin/nuimanbot --help` (or `./bin/nuimanbot < /dev/null`, or any invocation with immediately-closed stdin) has always just started the app, ignoring the argument entirely. In earlier versions, the old no-login REPL treated an immediate end-of-input gracefully and exited with status `0`. Now that login gates all input by design, empty or closed stdin fails at the username prompt (it can't read a username) and the process exits with status `1` instead. This is an intentional consequence of adding real login, not a regression — but if you have automation that runs `./bin/nuimanbot --help` (or similar) and checks for exit code `0`, it will now see `1` and needs updating to supply real credentials via stdin, per the piped-input note above.

---

## Chats

- `/chat list` — list your chats
- `/chat show <id>` — show a chat's message history
- `/chat new <message>` — create a new chat and send the first message
- `/chat send <id> <message>` — send a message to an existing chat
- `/chat delete <id>` — delete a chat immediately
- `/chat export <id> [json|markdown] [path]` — export a chat's transcript; without a path, prints it; with a path, writes it to that local file

A Chat created via the CLI shows up in the web admin UI's Chats page, and vice versa — both are scoped to your logged-in username, so other users never see your chats.

---

## Projects

- `/project list` — list your projects
- `/project create <name> <output-directory>` — create a project
- `/project show <id>` — show a project's output directory, `AGENTS.md` presence, and retention setting
- `/project add-agents-file <id>` — create/refresh the project's `AGENTS.md`
- `/project delete <id>` — delete a project

**Not available:** `/project chat` — the web UI doesn't have a per-project chat interface yet either, so there's nothing for the CLI to mirror. This will be added once the web side supports it.

---

## Jobs

- `/job list [--project <id>] [--chat <id>]` — list your jobs, optionally filtered by context
- `/job create <title> <description> [--project <id>] [--chat <id>]` — create a job
- `/job show <id>` — show a job's details
- `/job delete <id>` — delete a job

**Not available:** `/job chat` — same reasoning as Projects above.

---

## Chores

- `/chore list` — list your chores
- `/chore create <title> <description> [--dir <path>] --schedule <preset-or-cron-expr>` — create a chore (schedule starts unconfirmed; `--dir` is optional)
- `/chore confirm-schedule <id>` — confirm the schedule before it becomes active
- `/chore show <id>` — show a chore's details
- `/chore delete <id>` — delete a chore; if a run is currently active, it's soft-deleted (marked pending) and removed automatically once that run finishes, same as the web UI

**Not available:** `/chore chat` — same reasoning as Projects above.

---

## History

- `/history list [--job <id>] [--chore <id>] [--status <status>] [--since <value>]` — list your run history, optionally filtered
- `/history show <run-id>` — show a run's details

Large result sets are truncated rather than dumped unbounded to the terminal — narrow your filters if you don't see the run you're looking for.

**Not available:** `/history chat` — same reasoning as Projects above.

---

## Memories

- `/memories browse [query]` — read-only search/browse over your long-term memory (shows up to 20 cells at a time; refine your query if you don't see the entry you're looking for — large result sets are truncated rather than dumped unbounded to the terminal, same as `/history list`)
- `/memories chat <cell-id> <message>` — ask the agent a question about a specific memory entry

Note the trailing "s" — `/memories` (plural) is this environment; `/memory` (singular, no trailing "s") is the separate, pre-existing admin command for memory system stats/export/import/rebuild. The two don't collide, but it's easy to mistype one for the other.

Unlike the other five environments, `/memories chat` **is** implemented — the web UI has a real backing capability (`AskAboutCell`) for it. The agent remains the sole writer of memory content; asking a question through this command never edits the entry itself.

---

## Settings

- `/settings show` — show the system-wide Chat/Project/History retention defaults (read-only)
- `/settings show --system` — (admin only) show worker pool size, registered skills, and network mode
- `/settings set worker-pool-size <n>` — (admin only) change the live worker pool concurrency

**Not available:**
- `/settings set retention <chat|project|history> <value|never>` — per-user retention overrides don't exist anywhere in the system yet (the web UI's Settings page shows the same system-wide default you see here, not a per-user value). This isn't CLI-specific; it needs a separate piece of work to add per-user retention storage in the first place.
- `/settings set network-mode <localhost|remote>` — even the web UI's own network-mode control has a known limitation (it doesn't rebind the currently-running listener). Wiring this up for the CLI too wouldn't make it actually work, so it's deferred until that underlying limitation is fixed.

Both unavailable commands, if you try them, tell you clearly that they're not implemented rather than silently doing nothing.

---

## What's Not Implemented Yet

- Per-item "chat with the agent" commands for Projects, Jobs, Chores, and History (see each section above) — Memories is the one exception, since it already has a real backing capability.
- Per-user retention overrides (`/settings set retention`) — no such capability exists anywhere in the system yet, web or CLI.
- CLI-side network-mode control (`/settings set network-mode`) — blocked on a pre-existing web UI limitation.

If you try `/project chat`, `/job chat`, `/chore chat`, or `/history chat`, the CLI returns a specific "'/X chat' is not yet implemented" message naming that exact command, not a generic unrecognized-command error — so it's always clear the command was understood but is deliberately deferred, not mistyped. A genuine typo (e.g. `/job chta`) still gets the ordinary "Unknown command" response. `/settings set retention` and `/settings set network-mode` similarly return a clear deferred-capability message rather than a generic error or silent no-op (see the Settings section above) — `/settings set network-mode` additionally requires an admin role, so a non-admin sees a permission error there instead of the deferral message.

See the [Web Workspace Guide](web-workspace-guide.md) for the equivalent web UI experience, which the CLI commands above are designed to match exactly — data created in one is visible in the other.
