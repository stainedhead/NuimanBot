# Web Workspace Guide

**Version:** 1.0
**Last Updated:** 2026-08-05
**Target Audience:** NuimanBot Users

This guide covers NuimanBot's web workspace: **Chats**, **Projects**, **Jobs**, **Chores**, **History**, and **Memories**, plus the **Settings** page. These are new environments in the web admin interface (`/admin/...`), separate from chatting with NuimanBot through Telegram, Slack, or the CLI.

---

## Table of Contents

1. [What to Expect Today](#what-to-expect-today)
2. [Signing In](#signing-in)
3. [Chats](#chats)
4. [Projects](#projects)
5. [Jobs](#jobs)
6. [Chores](#chores)
7. [History](#history)
8. [Memories](#memories)
9. [Settings](#settings)
10. [Troubleshooting](#troubleshooting)

---

## What to Expect Today

This part of NuimanBot is new, and it is honest to say up front: **the organizing and scheduling pieces work today, but the agent itself is not yet plugged into them.** Please read this section before you start, so nothing here surprises you.

**Works today:**
- Creating, naming, listing, and deleting Chats, Projects, Jobs, and Chores
- Jobs and Chores queue correctly and run through their full lifecycle (Queued → Running → Completed)
- Job/Chore run history, with filtering and a notification badge for runs you haven't looked at yet
- Browsing and searching NuimanBot's long-term memory (read-only)
- Choosing between localhost-only and remote network access, with an optional allowlist

**Not yet working — please don't rely on these:**
- **Jobs and Chores do not do real agent work yet.** When a Job or Chore "runs," it goes through the full queue → execute → complete pipeline, but the result is a placeholder file that says no agent work was performed. Think of this as the scaffolding being built and tested before the agent is connected to it.
- **Typing a message in a web Chat does not get you a reply.** The message is saved, but nothing responds. If you want to actually talk with NuimanBot today, use Telegram, Slack, or the CLI — those work exactly as before and are unaffected by any of this.
- There's no "chat about this specific Job/Chore/run/memory" conversation yet — those panels are planned but not built.
- Setting a retention period (e.g., "delete Chats after 90 days") is recorded, but nothing currently goes and deletes anything automatically yet.
- Run status and notification badges don't update live on screen — refresh the page to see the latest status.
- If the server restarts while a Job/Chore run is actively executing (not just waiting in the queue), that run can get stuck showing its last status rather than resuming or being marked failed automatically.
- Deleting a Chore while its run is active removes the Chore's record right away; deleting a Job in the same situation marks it for removal but doesn't yet automatically finish cleaning it up once the run ends.

If you're looking to actually get work done with NuimanBot today, use the existing chat gateways (Telegram/Slack/CLI) — see the [User Onboarding Guide](user-onboarding.md). Use this web workspace to get familiar with Projects, Jobs, and Chores organization ahead of the agent being connected to them.

---

## Signing In

1. Open your browser to the NuimanBot admin address (e.g. `https://localhost:8443/admin` — ask your administrator for the exact URL, especially if the server is configured for remote access)
2. Sign in with your username and password
3. If this is your first time signing in with a default password, you'll be asked to set a new one before continuing
4. Once signed in, you'll see a left-hand navigation sidebar with **Chats**, **Projects**, **Jobs**, **Chores**, **History**, and **Memories**. (A few older pages — Dashboard, Bots, Users, Confirmations — don't have this sidebar yet; use your browser's back button or re-enter a workspace URL to get back.)

---

## Chats

A Chat is a lightweight, ad-hoc conversation with no project or folder attached to it.

### Creating a Chat

1. Click **Chats** in the sidebar
2. Type your first message and submit it
3. NuimanBot names the Chat automatically from that first message (if you leave the message blank, it names the Chat with a timestamp instead, so you'll never see an unnamed Chat in your list)

### Using a Chat

- Open a Chat from the list to see its full message history
- You can keep typing and sending messages — they're saved to the Chat's history
- **Remember:** as of this writing, no reply comes back in the web UI. Your message is recorded, not answered here.

### Exporting a Chat

Open a Chat and use the export/download control to save its transcript as JSON or Markdown — useful for keeping a record or moving a conversation elsewhere.

### Deleting a Chat

Open a Chat and use the delete control. Deletion is immediate and cannot be undone.

### Retention

Chats can be set to auto-delete after a period of inactivity (see [Settings](#settings)), or kept forever ("Never"). As noted above, this setting is currently recorded but not yet acted on automatically — nothing will be deleted on your behalf yet, even if you set a retention window.

---

## Projects

A Project is a durable workspace tied to a real folder on disk — unlike a Chat, it isn't ephemeral, and you can work with its files outside NuimanBot too.

### Creating a Project

1. Click **Projects** in the sidebar
2. Give it a name and an output directory (a folder path on the server's filesystem)
3. NuimanBot creates that folder (if it doesn't already exist) along with a hidden folder alongside it for the agent's own notes — you won't see the hidden folder in the Project's file view, and you don't need to manage it

### AGENTS.md

`AGENTS.md`, if present in a Project's output directory, is meant to steer how the agent works in that Project (similar in spirit to this repository's own `AGENTS.md`).

- The intended way to create or edit it is by asking the agent to do so in a conversation scoped to the Project — this path isn't available yet (see [What to Expect Today](#what-to-expect-today))
- In the meantime, open the Project's detail page and use the subdued **"Add AGENTS.md"** control to create a starter file
- You can also open and edit the Project's output folder directly with any text editor or file manager on the server — NuimanBot never locks you out of your own files. If you and the agent ever edit `AGENTS.md` at the same time, whichever save happens last wins, the same as editing any file open in two places at once.

### Deleting a Project

Deleting a Project removes the Project record, but **does not** touch the files in its output directory, and does not delete any Job or Chore that references it. Today, since Jobs/Chores don't yet do real agent work (see [What to Expect Today](#what-to-expect-today)), a Job/Chore pointed at a deleted Project won't actually notice — it will still "complete" with a placeholder result rather than reporting an error. Once real agent execution is connected, a Job/Chore referencing a deleted Project is intended to fail clearly instead.

### Retention

Same model as Chats: configurable, including "Never," but not yet automatically enforced.

---

## Jobs

A Job is a one-time task you hand to the agent, run once, and check the results of later.

### Creating a Job

1. Click **Jobs** in the sidebar
2. Give it a Title and a Description of the task
3. Choose whether it runs in the context of a Chat or a Project. If you pick a Project, that Project's output folder becomes the Job's working directory by default.
4. Submit — your Job joins the queue

### What happens next

Jobs run in the order they were created (first in, first out), through a shared pool of workers your administrator sizes in Settings. Right now, "running" a Job exercises the full queue-and-execute pipeline, but produces a placeholder result rather than real agent work — see [What to Expect Today](#what-to-expect-today).

### Checking on a Job

Open the Job's detail page to see its current status, or check [History](#history) for its full run record once it finishes.

### Deleting a Job

If a Job's run is currently in progress, deleting it marks it for removal rather than deleting it outright — nothing running is killed mid-run. As of this writing, that mark is not yet automatically cleared once the run finishes; a Job in this state may need a follow-up delete or admin cleanup rather than disappearing on its own.

---

## Chores

A Chore is a Job that repeats on a schedule.

### Creating a Chore

1. Click **Chores** in the sidebar
2. Give it a Title, Description, and (optionally) a working directory
3. Set a schedule — either a common preset (hourly, daily, weekly, monthly) or your own cron expression for finer control
4. Submit

### Agent-proposed schedules

If a schedule was suggested by the agent rather than set by you directly, it stays in a **"pending confirmation"** state and will never fire until you explicitly confirm it. It won't quietly expire either — it just waits for your decision.

### Overlapping runs

If a Chore's next scheduled time arrives while its previous run is still going, NuimanBot skips that firing (and records it as "skipped" in History) rather than running two copies at once. The Chore picks back up at its next scheduled time as normal.

### Deleting a Chore

Unlike Jobs, deleting a Chore removes it immediately, even if a run is currently in progress — the run itself keeps executing to completion, but the Chore's record disappears from your list right away rather than waiting. Bringing this in line with Jobs' behavior is planned but not done yet.

---

## History

History is your list of every Job and Chore run you own, in one place.

### Browsing runs

- Open **History** to see every run, most recent first
- Filter by which Job/Chore it belongs to, a date range, or status (Queued, Running, Completed, Failed, Skipped)
- Open any run to see its full log and results

### Notification badge

A badge on the **History** nav item shows how many completed runs you haven't looked at yet. Opening a run clears its badge. As with the rest of this workspace, the badge doesn't update live on screen yet — refresh the page to see the current count.

### Retention

Configurable independently from Chat and Project retention, including "Never" — again, not yet automatically enforced.

---

## Memories

Memories is a read-only window into what NuimanBot has learned and remembered about your conversations over time (the same long-term memory system described in the [Self-Organizing Memory Guide](self-organizing-memory-guide.md)).

- Browse or search memory entries from here
- You cannot create, edit, or delete a memory entry directly in this view — the agent is the only writer, based on what comes up in conversation
- A chat panel for asking about or requesting changes to specific memories is planned but not built yet

---

## Settings

Settings has two kinds of controls: **system-wide** (visible to everyone, changeable by admins only) and things that reflect your own account.

### What admins can change here

- **Worker pool size** — how many Jobs/Chores can run at the same time, across all users. Changing this takes effect immediately and never interrupts a run already in progress.
- **Network access mode** — localhost-only (the safe default) or remote. Note: switching this here changes who's *allowed* to connect, but if you're moving to remote access for the first time, your administrator also needs to have set a bind address in the configuration file and restarted the server — this page alone won't move the server onto a new network address.
- **Retention defaults** — the default number of days Chats, Projects, and History runs are kept before (eventually) auto-deleting, shown here for reference.

### What's config-file-only for now

The remote-access allowlist (which specific IPs/hostnames are trusted) and the bind address itself aren't editable from this page yet — your administrator sets those in `config.yaml`. See the [Configuration Reference](configuration-reference.md#network-access--workspace-configuration) for details.

### Skills, Plugins, Gateways, Users

These are existing systems, simply linked from Settings rather than rebuilt — see the [Admin Guide](admin-guide.md) and [Agent Skills Guide](skills-guide.md) for how to manage them.

---

## Troubleshooting

**I sent a message in a Chat and nothing answered me.**
Expected for now — see [What to Expect Today](#what-to-expect-today). Use Telegram, Slack, or the CLI for an actual conversation with the agent today.

**My Job/Chore says "Completed" but there's no real output.**
Expected for now. The queue and execution pipeline is real; the agent isn't connected to it yet. Check back after this is announced as ready.

**A run's status didn't change even though I know it finished.**
Refresh the page. Live updates aren't wired up in the browser yet, even though the server is already sending them.

**I set a retention period, but nothing has been deleted.**
Expected — retention windows are recorded but not yet enforced by an automatic cleanup process.

**I can't reach the workspace from another computer.**
Your administrator needs to enable remote access in `config.yaml` (and possibly add you to an allowlist) — see the [Configuration Reference](configuration-reference.md#network-access--workspace-configuration). By default, NuimanBot only accepts connections from the same machine it's running on.

**A page I try to open says "not found."**
If you're trying to open a Chat/Project/Job/Chore/run you don't own — including by guessing or reusing a link — NuimanBot always shows "not found" rather than any error that would confirm it exists. This is intentional: your resources are private to you, even from administrators.
