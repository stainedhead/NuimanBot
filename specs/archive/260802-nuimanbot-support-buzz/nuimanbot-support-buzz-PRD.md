# RFC: NuimanBot Support for Buzz

**Status:** Draft
**Author:** stainedhead (via AI agent research session)
**Date:** 2026-08-02
**Related:** [Clean Architecture layering](AGENTS.md), existing gateways (`internal/adapter/gateway/{slack,telegram,cli}`)

---

## 1. Summary

Add a new `Buzz` gateway to NuimanBot so a NuimanBot-hosted agent can participate in [Buzz](https://github.com/block) — Block's open-source, Nostr-based group-chat platform for mixed human/agent teams — as a first-class networked participant: joining channels, reading and posting messages, and (later) invoking approved automations, alongside human users and other agents.

## 2. Background: What Buzz Is

Buzz launched July 21, 2026 from Block (Jack Dorsey). It is functionally similar to Slack/Discord — channels, threads, DMs, voice, media sharing, code repositories — but differs in two structural ways that matter for this RFC:

1. **Decentralized transport.** Buzz is built on [Nostr](https://nostr.com/), a relay-based protocol. There is no single API endpoint to authenticate against; clients (human or agent) publish signed events to one or more relays over WebSocket and subscribe to event streams filtered by kind/author/tag.
2. **Agents as first-class identities.** Every agent gets its own Nostr keypair (secp256k1), cryptographically bound to its human owner. Agents are not bots that only respond to `@mentions` — they can post, review code, run approved automations, and join conversations autonomously, alongside other agents in the same channel.

Buzz is free and Apache-2.0 licensed, with a desktop app (macOS/Windows/Linux) and an open GitHub repo.

**Sources:**
- [Jack Dorsey is taking on Slack with Buzz — TechCrunch](https://techcrunch.com/2026/07/21/jack-dorsey-is-taking-on-slack-with-buzz-a-group-chat-platform-for-teams-and-their-ai-agents/)
- [Block built a Slack for AI agents — The New Stack](https://thenewstack.io/block-buzz-agent-workspace/)
- [Block Launches Buzz: Open-Source Workspace Where AI Agents Sign Their Own Work — TechTimes](https://www.techtimes.com/articles/321242/20260722/block-launches-buzz-open-source-workspace-where-ai-agents-sign-their-own-work.htm)

## 3. Motivation

NuimanBot already supports multiple messaging platforms (CLI, Telegram, Slack) through a uniform `domain.Gateway` interface. Buzz represents a new category of surface for NuimanBot: a channel where NuimanBot's agent cooperates with other autonomous agents, not just relays messages to/from a single human. Supporting Buzz lets a NuimanBot instance:

- Join shared human+agent workspaces without a bespoke integration per team.
- Participate in multi-agent workflows (e.g., code review handoffs) using NuimanBot's existing `GitHub`, `RepoSearch`, and `CodingAgent` tools.
- Establish a portable, cryptographically verifiable agent identity independent of any single vendor's bot-token system.

## 4. Goals

- Implement a `domain.Gateway`-conformant Buzz gateway following the existing Slack/Telegram pattern.
- Support connecting to one or more Nostr relays, publishing signed events, and subscribing to channel/DM event streams.
- Generate, store, and use a per-agent Nostr keypair via the existing credential vault.
- Distinguish human- vs agent-authored messages to prevent agent-to-agent reply loops.
- Apply NuimanBot's existing security pipeline (input validation, injection pattern matching, audit logging, RBAC) to all Buzz-originated content, treating other agents as a potentially adversarial input source.
- Map a Buzz identity (Nostr pubkey) to an internal `domain.User`/`Role` for RBAC purposes.

## 5. Non-Goals (initial phase)

- Voice channel support.
- Media/file sharing beyond text messages.
- Running Buzz's "approved automations" execution model directly — initial phase covers chat participation only; automation execution can reuse existing tools (`GitHub`, `CodingAgent`) but is not scoped here.
- Multi-relay conflict resolution / eventual-consistency guarantees beyond basic redundancy (best-effort, not exhaustively specified).

## 6. Proposed Design

### 6.1 New Components

```
internal/infrastructure/nostr/
├── client.go        # Relay WebSocket client (connect, reconnect, multi-relay fanout)
├── event.go         # NIP-01 event construction, ID computation, signing
├── verify.go         # Signature verification for inbound events
└── subscription.go   # Filter-based subscription management

internal/adapter/gateway/buzz/
└── gateway.go        # domain.Gateway implementation for Buzz
```

This mirrors the existing `slack`/`telegram` gateway packages and keeps the Nostr protocol concerns in infrastructure (outermost layer), per Clean Architecture rules in `AGENTS.md`.

**Transport note:** `github.com/gorilla/websocket` is already an indirect dependency (currently pulled in transitively). This RFC proposes promoting it to a direct dependency for the Nostr relay client — no new WebSocket library is required. A secp256k1 signing library (e.g. `github.com/btcsuite/btcd/btcec` or `github.com/nbd-wtf/go-nostr`) will need to be added; evaluating a small, audited Nostr client library instead of hand-rolling the protocol is preferred and should be settled during implementation planning, not in this RFC.

### 6.2 Domain Changes

```go
// internal/domain/message.go
const PlatformBuzz Platform = "buzz"
```

`IncomingMessage.Metadata` / `OutgoingMessage.Metadata` carry Buzz-specific fields, consistent with how Slack stores `channel`/`thread_ts`:

| Key | Meaning |
|---|---|
| `event_id` | Nostr event ID (hex) |
| `relay_url` | Relay the event was received from |
| `sender_pubkey` | Author's Nostr public key |
| `sender_is_agent` | Bool — derived from Buzz agent-identity metadata, used for loop prevention |
| `channel_id` | Buzz channel identifier |
| `signature` | Event signature (for audit trail) |

No changes to `Gateway`, `MessageHandler`, or the core `Send`/`OnMessage` contract are needed — Buzz fits the existing interface.

### 6.3 Configuration

```go
// internal/config/gateway_config.go
type BuzzConfig struct {
    Enabled       bool                `yaml:"enabled"`
    PrivateKey    domain.SecureString `yaml:"private_key"`     // secp256k1, vault-stored
    Relays        []string            `yaml:"relays"`          // wss:// relay URLs
    NIP05         string              `yaml:"nip05"`           // optional verified identifier
    ChannelIDs    []string            `yaml:"channel_ids"`     // channels to join
    DMPolicy      DMPolicy            `yaml:"dm_policy"`       // reuses existing enum
}
```

Added to `GatewaysConfig` alongside `Telegram`, `Slack`, `CLI`, `WebUI`. Wired in `cmd/nuimanbot/main.go` following the identical `if app.Config.Gateways.Buzz.Enabled { ... }` pattern used for Telegram/Slack.

### 6.4 Credential Vault

The vault (`internal/infrastructure/crypto/file_credential_vault.go`) currently stores opaque secret strings (bot tokens). This RFC proposes:

- A key-generation path: if no private key is configured, generate a new secp256k1 keypair on first run and persist it through the existing `VersionedVault` mechanism (same AES-256-GCM encryption at rest).
- No change to the vault's storage format is required — a Nostr private key is just another `domain.SecureString` value. The addition is a **generate-if-absent** helper, not a new vault subsystem.

### 6.5 Identity & RBAC Bridging

Buzz ties agent identity to a human owner cryptographically; NuimanBot's RBAC is role-based per platform user ID (`RoleGuest`/`RoleUser`/`RoleAdmin`). Proposed mapping:

- On first message from a given `sender_pubkey`, resolve/create a `domain.User` keyed by `buzz:<pubkey>` (consistent with existing `PlatformUID` handling in other gateways).
- Default role: `RoleGuest` for unrecognized pubkeys; promotion to `RoleUser`/`RoleAdmin` follows the existing admin-command flow already used for Telegram/Slack.
- NIP-05 verification (if configured) can be used as an optional signal for auto-trust, but is not required for MVP.

### 6.6 Loop Prevention

Buzz explicitly supports multiple cooperating agents in one channel. The gateway must:

- Tag outgoing events with an agent marker (Buzz's own agent-identity mechanism, per its protocol spec) so other Buzz-aware clients can distinguish agent from human authorship.
- On the receive side, set `sender_is_agent` in metadata and let the Chat Service apply a simple guard (e.g., don't auto-reply to messages that are themselves agent-authored replies to this agent, within a short time window) to avoid runaway agent-to-agent reply chains. Exact heuristic to be finalized during implementation — this is a known failure mode in multi-agent chat integrations and needs test coverage.

### 6.7 Security Implications

The existing Security Service (`internal/usecase/security/service.go`) already runs prompt-injection and command-injection pattern matching (30+ / 50+ patterns) on all input. This becomes **more load-bearing** for Buzz: content now regularly originates from other autonomous agents, some of which may be malicious or compromised, not just human users. No new validation logic is proposed here, but:

- Buzz-originated messages must go through `ValidateInput()` identically to other platforms (no bypass).
- Signature verification (Nostr event signatures) must be checked before an event is treated as trusted input from the claimed pubkey — a forged/unsigned event should be dropped, not just logged.
- Tool execution triggered by Buzz-channel content still goes through the existing RBAC + rate-limiting + audit pipeline in `internal/usecase/tool/service.go` unchanged.

### 6.8 Observability

New Prometheus metrics, consistent with existing conventions (`internal/infrastructure/metrics`):

```
buzz_relay_connections{relay_url, status}
buzz_events_received_total{channel_id, sender_is_agent}
buzz_events_published_total{status}
buzz_signature_verification_failures_total
```

### 6.9 Functional Requirements

**Phase 1**
- FR-001: Connect to configured Nostr relays over WebSocket, with reconnect-on-drop.
- FR-002: Subscribe to configured channel IDs via NIP-01 filters.
- FR-003: Verify event signatures before processing; drop unsigned/forged events.
- FR-004: De-duplicate events by event ID across relays.
- FR-005: Map verified events to `domain.IncomingMessage` (`PlatformBuzz`) including metadata.
- FR-006: Resolve/create `domain.User` (`RoleGuest`) keyed by `buzz:<pubkey>` on first message.
- FR-007: Generate-if-absent secp256k1 keypair via existing vault.

**Phase 2**
- FR-008: Publish signed, agent-tagged events via `Send()`.
- FR-009: Apply loop-prevention guard to agent-authored reply chains.

**Phase 3**
- FR-010: Trigger `GitHub`/`CodingAgent`/`RepoSearch` tools from Buzz messages under existing RBAC.

### 6.10 Non-Functional Requirements

**Reliability**
- Gateway MUST continue operating if some (not all) configured relays are unreachable — partial connectivity is not a startup failure.
- Reconnect attempts MUST use bounded exponential backoff (no tight reconnect loop).
- Duplicate events across relays MUST NOT be delivered to the message handler more than once (ties to FR-004).

**Performance**
- No hard latency SLA (chat gateway, not a real-time system) — but per-relay goroutines/buffers MUST be bounded so N configured relays don't cause unbounded resource growth.
- Signature verification MUST NOT block the relay read loop for other events (async/queued verification acceptable).

## 7. Alternatives Considered

- **MCP bridge instead of a native gateway.** Buzz could theoretically be exposed as an MCP server that NuimanBot calls as a tool. Rejected as primary approach: Buzz is a bidirectional, event-driven surface where NuimanBot itself is a channel participant (like Slack/Telegram), not a one-shot tool call — the `domain.Gateway` pattern is the correct fit, matching how Slack/Telegram were integrated.
- **Reuse Slack gateway's token model for auth.** Not viable — Buzz has no bot-token concept; identity is cryptographic by design.

## 8. Rollout Plan

1. **Phase 1 — Read-only participation:** Connect to relays, subscribe to configured channels, log/store incoming events, no publishing. Validates event parsing, signature verification, and RBAC mapping without any risk of the agent posting.

   **Exit criteria:**
   - Gateway connects to all configured relay URLs, with reconnect-on-drop.
   - Event signatures are verified before processing; forged/unsigned events are dropped and counted (`buzz_signature_verification_failures_total`).
   - Duplicate events (same event ID delivered by multiple relays) are forwarded to the message handler exactly once.
   - First message from a new `sender_pubkey` creates a `domain.User` with `RoleGuest`.
   - All Buzz-originated messages pass through `ValidateInput()` identically to other platforms.

2. **Phase 2 — Full gateway:** Enable `Send()` (publish signed events), wire into `connectGateway()` in `main.go`, add loop-prevention guard.

   **Exit criteria:**
   - `Send()` publishes correctly-signed, correctly-tagged (agent-marked) events to configured relays.
   - Loop-prevention guard demonstrably terminates a runaway agent-to-agent reply chain within the defined time window (test: simulated N-message exchange terminates rather than running indefinitely).

3. **Phase 3 — Tool integration:** Allow Buzz channel messages to trigger existing tools (`GitHub`, `CodingAgent`, `RepoSearch`) under normal RBAC, enabling the "review code / run automations" use case Buzz is designed for.

   **Exit criteria:**
   - Buzz channel messages can trigger `GitHub`/`CodingAgent`/`RepoSearch` tool execution under the existing RBAC + rate-limiting pipeline, with no bypass for Buzz-originated requests.
   - Tool execution triggered from Buzz is audit-logged identically to tool execution triggered from other platforms.

Each phase should follow the standard TDD + quality-gate workflow in `AGENTS.md`. All three phases are tracked within a single spec directory (not one directory per phase), with phase structure captured in that spec's `tasks.md`/`plan.md`.

### Overall Acceptance

Buzz support is complete when a NuimanBot-hosted agent can join a Buzz channel, participate in multi-agent conversation without triggering reply loops, and safely execute approved tools triggered from that channel — all under the existing security, RBAC, and audit pipeline, with no reduction in the guarantees NuimanBot already provides on other platforms.

## 9. Open Questions

- Which Nostr client/signing library to depend on (hand-rolled minimal NIP-01 client vs. an existing Go library) — needs a short spike before Phase 1 implementation.
- Exact loop-prevention heuristic (time window, event-tag based vs. content-based detection).
- Whether Buzz's "approved automations" concept requires a new permission tier beyond `RoleGuest`/`RoleUser`/`RoleAdmin`, or maps cleanly onto existing tool RBAC.
- Multi-relay consistency: is best-effort fanout sufficient, or does NuimanBot need to de-duplicate the same event arriving from multiple relays (likely yes — dedupe by event ID).

## 10. References

- [Jack Dorsey is taking on Slack with Buzz — TechCrunch](https://techcrunch.com/2026/07/21/jack-dorsey-is-taking-on-slack-with-buzz-a-group-chat-platform-for-teams-and-their-ai-agents/)
- [Block built a Slack for AI agents — The New Stack](https://thenewstack.io/block-buzz-agent-workspace/)
- [Block Launches Buzz — TechTimes](https://www.techtimes.com/articles/321242/20260722/block-launches-buzz-open-source-workspace-where-ai-agents-sign-their-own-work.htm)
- Nostr protocol: [nostr.com](https://nostr.com/)
- Existing gateway pattern: `internal/domain/gateway.go`, `internal/adapter/gateway/slack/gateway.go`, `internal/adapter/gateway/telegram/gateway.go`
