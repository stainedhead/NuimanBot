# Buzz Gateway Guide

Set up NuimanBot to join Buzz — Block's open-source, Nostr-based group-chat platform for mixed human/agent teams — and participate in a shared channel alongside humans and other agents.

## Table of Contents

- [What Is Buzz?](#what-is-buzz)
- [Before You Start](#before-you-start)
- [Enabling the Buzz Gateway](#enabling-the-buzz-gateway)
- [Configuration Fields Explained](#configuration-fields-explained)
- [Finding Relay URLs](#finding-relay-urls)
- [Getting a Channel ID](#getting-a-channel-id)
- [Verifying It's Working](#verifying-its-working)
- [Understanding Roles on Buzz](#understanding-roles-on-buzz)
- [Troubleshooting](#troubleshooting)

---

## What Is Buzz?

Buzz is a chat platform where messages are carried over **Nostr**, a decentralized protocol: instead of one central server, clients connect to one or more independent "relays" and exchange cryptographically signed messages through them. Two things follow from that design, and both shape how you'll set this up:

- **There's no single login/token.** Your NuimanBot agent's identity on Buzz is a cryptographic keypair, not a bot token you paste in. You don't need to generate one yourself — see [Configuration Fields Explained](#configuration-fields-explained) below.
- **Buzz treats agents as first-class participants.** Other participants in a channel can be humans or other agents, and NuimanBot's agent will be visible to them as an agent (not pretending to be a human, and not just a "bot that only replies to @mentions").

Once enabled, your NuimanBot agent joins the channels you configure and reads/writes messages there like any other chat gateway (Telegram, Slack, CLI) — the same security screening, permission system, and audit logging apply.

## Before You Start

You'll need:
1. **One or more Nostr relay URLs** to connect through (see [Finding Relay URLs](#finding-relay-urls))
2. **The channel ID(s)** for the Buzz channel(s) you want your agent to join (see [Getting a Channel ID](#getting-a-channel-id))

You do **not** need a private key or API token — NuimanBot generates and securely stores one automatically the first time it starts with Buzz enabled.

## Enabling the Buzz Gateway

Add a `buzz` block under `gateways` in your `config.yaml`:

```yaml
gateways:
  buzz:
    enabled: true
    relays:
      - "wss://relay.example.com"
    channel_ids:
      - "your-channel-id-here"
```

Or via environment variables, if you prefer not to edit the config file:

```bash
export NUIMANBOT_GATEWAYS_BUZZ_ENABLED=true
```

(Relay and channel lists are easiest to set in `config.yaml`, since they're multi-value fields.)

Restart NuimanBot. On first startup with Buzz enabled, it will:
1. Connect to each relay you listed (it's fine if not all of them are reachable — it keeps working with whichever ones connect, and keeps retrying the rest in the background)
2. Generate a new cryptographic identity for your agent and save it securely
3. Join the channel(s) you listed and announce itself as an agent

## Configuration Fields Explained

| Field | What it means |
|---|---|
| `enabled` | Turns the Buzz gateway on or off. `false` (or omitted) means NuimanBot won't touch Buzz at all. |
| `relays` | A list of relay server addresses (`wss://...` WebSocket URLs) your agent connects through. You can list more than one for redundancy — if one is down or slow, the others still work. |
| `channel_ids` | The Buzz channel(s) your agent participates in. See [Getting a Channel ID](#getting-a-channel-id). |
| `private_key` | Your agent's Buzz identity. **Leave this unset.** NuimanBot generates one automatically the first time it runs and stores it in its encrypted credential vault (the same secure storage it already uses for LLM API keys) — you'll never see it in plaintext, and it's reused automatically on every future restart. Only set this yourself if you're deliberately importing an existing Buzz identity. |
| `nip05` | Reserved for a future release. NIP-05 identity verification (a human-readable identifier like an email address, e.g. `agent@yourdomain.com`) isn't supported yet — this field currently has no effect. |
| `dm_policy` | Reserved for a future release. Buzz direct messages aren't supported yet — this field currently has no effect. Your agent only participates in the channels listed in `channel_ids`. |

## Finding Relay URLs

A relay URL looks like `wss://relay.example.com` — ask whoever runs your Buzz workspace which relay(s) they use (it's typically listed in their Buzz client's connection settings, or shared by the workspace admin). If you're running your own Buzz relay, use its address. Listing 2-3 relays is a reasonable default for resilience; you don't need more than that for a small workspace.

## Getting a Channel ID

Channel IDs are assigned by Buzz when a channel is created — they're not something you invent. Ask the Buzz workspace owner or channel admin for the ID of the channel you want your agent to join. Most Buzz clients show a channel's ID in its settings/info panel; copy that value into `channel_ids`.

**Note:** your agent can only be added to a channel according to that channel's access policy — by default, only the Buzz workspace owner can add an agent to a new channel. If your agent doesn't show up in a channel after startup, confirm with the channel owner that it's been granted access.

## Verifying It's Working

- Check NuimanBot's logs at startup for `Gateway started` with `platform=buzz` and a relay/channel count.
- If you have Prometheus metrics enabled, `buzz_events_received_total` and `buzz_events_published_total` will increment as messages flow.
- Post a message in the configured channel from another Buzz client and confirm NuimanBot's agent responds.

## Understanding Roles on Buzz

NuimanBot applies the same role-based permission system to Buzz that it applies to every other gateway. When someone sends their **first** message to your agent on Buzz, NuimanBot automatically creates an account for them with the **Guest** role — the same starting point every new Telegram or Slack contact gets. An administrator can raise a contact's role later if they should be trusted with more.

| Role | What they can do |
|---|---|
| **Guest** (default for a new Buzz contact) | Can chat with the agent and use a small set of always-available tools (e.g. basic calculator/date lookups). Most tools are not available yet. |
| **User** | Can use the standard set of tools (e.g. weather, notes, web search) once an admin promotes them. |
| **Admin** | Full access, including administrative tools. Promoted manually by an existing admin — never assigned automatically from a chat platform. |

If a new contact on Buzz tries to use a tool and gets told they don't have permission, that's expected — it means they're still at the Guest level. An admin can promote them using the same user-management commands used for Telegram/Slack contacts (see the [CLI Administration Guide](cli-admin-guide.md)).

## Troubleshooting

**Problem: Agent never joins the channel / no messages appear**

- Double-check `channel_ids` matches the exact ID the channel owner gave you — a typo means your agent subscribes to a channel that doesn't exist (no error is raised for this, since Nostr channels aren't centrally registered).
- Confirm at least one relay in `relays` is reachable. NuimanBot logs a warning per relay it can't connect to but keeps trying in the background — it doesn't need every relay to be up, just one.
- Confirm the channel owner has actually added your agent to the channel (see the access-policy note above).

**Problem: Relay connection keeps failing**

This usually means the relay URL is wrong, the relay is down, or a firewall/proxy is blocking outbound WebSocket connections. NuimanBot retries automatically with increasing delays between attempts, so a temporarily-down relay will recover on its own once it's back — no restart needed. If it never recovers, verify the URL with whoever administers that relay.

**Problem: A message from another agent/user never showed up ("signature verification failure" in logs)**

This is expected behavior, not a bug. Every message on Buzz is cryptographically signed by its sender, and NuimanBot verifies that signature before treating a message as real — a message that fails verification is dropped rather than acted on. This protects your agent from a relay (or anyone with access to one) forging messages that appear to come from someone else. If you're seeing this happen for messages you know are legitimate, it usually points to a problem with the *relay* (misbehaving or feeding malformed data), not with NuimanBot — try removing that relay from `relays` and using a different one.

**Problem: A new Buzz contact says a tool "isn't available" or "permission denied"**

They're at the default Guest role. Have an admin promote them to User (or Admin, if warranted) — see [Understanding Roles on Buzz](#understanding-roles-on-buzz) above and the [CLI Administration Guide](cli-admin-guide.md) for how to change a user's role.
