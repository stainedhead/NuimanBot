# Buzz ACP Harness Guide

Register NuimanBot as a custom Buzz "harness" — a subprocess Buzz spawns per conversation and drives directly over the Agent Client Protocol (ACP), the same mechanism Buzz uses to run Goose, Codex, and Claude Code as agents inside it.

This is a different way to connect NuimanBot to Buzz than the [Buzz Gateway](buzz-guide.md), which instead runs one long-lived NuimanBot process that connects outward to a Nostr relay. You do not need both — pick whichever fits how you want to run NuimanBot. This guide covers the ACP harness path.

## Table of Contents

- [How This Differs From the Buzz Gateway](#how-this-differs-from-the-buzz-gateway)
- [Before You Start](#before-you-start)
- [Registering the Harness](#registering-the-harness)
- [Verifying It's Working](#verifying-its-working)
- [Understanding Roles](#understanding-roles)
- [Known Limitations](#known-limitations)
- [Troubleshooting](#troubleshooting)

---

## How This Differs From the Buzz Gateway

| | Buzz Gateway | ACP Harness (this guide) |
|---|---|---|
| How it runs | One NuimanBot process you start and keep running, connecting outward to a relay | Buzz starts and stops a NuimanBot process itself, one per conversation |
| Setup location | `config.yaml`'s `gateways.buzz` block | Buzz's own agent-registration settings |
| Identity | A Nostr keypair NuimanBot generates and manages | Implicit — Buzz trusts whichever process it spawned |
| Multiple channels | Yes, via `channel_ids` | No — one process per conversation |

If you're not sure which you want: the Buzz Gateway is the right choice if you want NuimanBot to be a standing presence across specific channels you configure ahead of time. The ACP harness is the right choice if you want Buzz itself to manage when and where NuimanBot runs, the same way it manages its other built-in agents.

## Before You Start

You'll need:
1. The NuimanBot binary built (`go build -o bin/nuimanbot ./cmd/nuimanbot`, per the project's build instructions)
2. `config.yaml` set up with at least an LLM provider — everything the ACP harness needs (config, storage, security, tools) is the same as any other NuimanBot setup; there's nothing ACP-specific to configure in `config.yaml` itself
3. Access to Buzz's agent-registration settings (its managed-agents config, wherever your Buzz installation exposes it)

You do **not** need to configure a relay, a private key, or channel IDs — none of that applies to this integration path.

## Registering the Harness

Point Buzz's agent-command configuration at the NuimanBot binary with `acp` as its argument:

- **Command:** `/path/to/bin/nuimanbot` (use the absolute path to your built binary)
- **Args:** `acp`

This mirrors how Buzz registers its other custom harnesses (e.g. `goose acp`, or a separate `codex-acp`/`claude-agent-acp` binary) — an agent-command plus arguments Buzz spawns as a subprocess per conversation, communicating over that subprocess's stdin/stdout using JSON-RPC.

Once registered, every new conversation with this agent in Buzz starts a fresh `nuimanbot acp` process, sends it an `initialize` call, opens a session, and forwards prompts to it — ending the process when the conversation session ends.

## Verifying It's Working

NuimanBot's ACP mode writes nothing but the ACP JSON-RPC protocol to its own stdout — everything else (startup diagnostics, request logs, errors) goes to stderr, which Buzz typically captures separately for its own logs/debugging. If you have access to that log stream, look for:

- `"ACP mode starting"` — confirms the process launched and reached the ACP entrypoint
- `"ACP server ready, reading stdio"` — confirms bootstrap (config, storage, tools) completed and it's waiting for Buzz's `initialize` call

If a conversation with the registered agent never gets a reply, check that log stream first — a bootstrap failure (e.g. an invalid LLM API key, a missing encryption key) surfaces there as an error before the process would have been able to respond to anything.

## Understanding Roles

NuimanBot applies the same role-based permission system to ACP-originated contacts as every other platform (Telegram, Slack, Buzz Gateway, CLI). The first message from a new ACP session automatically creates an account with the **Guest** role — the same starting point every new contact on any platform gets, except the CLI (which is inherently trusted, since running the binary already implies machine access). An administrator can promote a contact's role later using the same commands used for any other platform — see the [CLI Administration Guide](cli-admin-guide.md).

## Known Limitations

This is a first working version, not a full-parity implementation of everything the ACP spec or Buzz's own built-in agents support:

- **Replies arrive as a single message, not streamed token-by-token.** NuimanBot's chat processing is synchronous internally (same as every other gateway), so a reply is delivered as one complete update, not incremental chunks.
- **No per-conversation MCP tools from Buzz.** Buzz can hand each ACP session its own MCP server; NuimanBot doesn't yet consume that — it only uses whatever MCP tools are configured centrally in its own `mcp.json`, same as every other platform.
- **Session state doesn't survive a process restart.** If Buzz respawns the subprocess mid-conversation (e.g. after a crash), NuimanBot starts a fresh session rather than resuming the prior one.
- **Concurrent conversations mean concurrent NuimanBot processes sharing the same file-based storage, with no cross-process locking.** Buzz spawns one subprocess per conversation (observed running several at once). Each is a separate OS process, but all of them read and write the same `./data` directory and `.env` file — NuimanBot's file-storage locking is in-process only, so it doesn't coordinate across separate processes. In practice this means: a burst of first-ever messages from different new contacts, arriving in different subprocesses at nearly the same moment, could race on writes to shared files like `domain_users.json`. It also means that if `.env` doesn't yet have an encryption key when several subprocesses start close together, more than one could try to generate and write a key at once, leaving `.env` with a key that doesn't match what was used to encrypt existing data. **Avoid this by running NuimanBot's normal `./bin/nuimanbot` once first** (letting it generate `.env` and initialize `./data` on its own, outside of Buzz), before registering the ACP harness — every ACP subprocess will then find an encryption key already in place and only race on ordinary data writes, which is a narrower and less severe risk.
- **Protocol details may need adjustment.** This was built against the publicly documented ACP spec and field names observed in Buzz's own tooling, but hasn't yet been exercised against a live Buzz-initiated session. If something doesn't work as expected, that's the most likely place to look first — not a sign the whole approach is broken.

None of these block basic use — a conversation, a reply, and role-based tool access all work — but they're worth knowing about before relying on this for something that needs streaming replies or session continuity across restarts.

## Troubleshooting

**Problem: Buzz never gets a response from the agent**

Check the process's stderr log (however Buzz surfaces it) for a bootstrap error — most commonly a missing/invalid LLM provider API key in `config.yaml`, or a missing encryption key. NuimanBot's ACP mode fails the same way any other NuimanBot startup would fail on bad config; the difference is just that the error goes to stderr instead of an interactive terminal.

**Problem: A response comes back garbled or Buzz reports a protocol error**

This most likely means a field name or shape in NuimanBot's ACP messages doesn't match what your Buzz version expects — see [Known Limitations](#known-limitations) above. This is the area most likely to need a small adjustment once exercised against a real Buzz session.

**Problem: A contact says a tool "isn't available" or "permission denied"**

They're at the default Guest role — see [Understanding Roles](#understanding-roles) above.
