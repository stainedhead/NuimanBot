# Spec: NuimanBot Support for Buzz

**Created:** 2026-08-02
**Source PRD:** [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md)
**Status:** Draft

## Executive Summary

Add a new `Buzz` gateway to NuimanBot so a NuimanBot-hosted agent can participate in Buzz — Block's open-source, Nostr-based group-chat platform for mixed human/agent teams — as a first-class networked participant: joining channels, reading and posting messages, and (later) invoking approved automations, alongside human users and other agents.

Buzz differs structurally from existing gateways (Telegram, Slack) in two ways: it has a decentralized, relay-based transport (Nostr, over WebSocket, no single API endpoint) and it treats agents as first-class cryptographically-identified participants rather than bots that only respond to mentions.

## Problem Statement

NuimanBot already supports multiple messaging platforms (CLI, Telegram, Slack) through a uniform `domain.Gateway` interface. Buzz represents a new category of surface: a channel where NuimanBot's agent cooperates with other autonomous agents, not just relays messages to/from a single human. Without Buzz support, NuimanBot cannot join these shared human+agent workspaces or participate in multi-agent workflows (e.g., code review handoffs) using its existing `GitHub`, `RepoSearch`, and `CodingAgent` tools.

## Goals

- Implement a `domain.Gateway`-conformant Buzz gateway following the existing Slack/Telegram pattern.
- Support connecting to one or more Nostr relays, publishing signed events, and subscribing to channel/DM event streams.
- Generate, store, and use a per-agent Nostr keypair via the existing credential vault.
- Distinguish human- vs agent-authored messages to prevent agent-to-agent reply loops.
- Apply NuimanBot's existing security pipeline (input validation, injection pattern matching, audit logging, RBAC) to all Buzz-originated content, treating other agents as a potentially adversarial input source.
- Map a Buzz identity (Nostr pubkey) to an internal `domain.User`/`Role` for RBAC purposes.

## Non-Goals (initial phase)

- Voice channel support.
- Media/file sharing beyond text messages.
- Running Buzz's "approved automations" execution model directly — initial phase covers chat participation only; automation execution can reuse existing tools (`GitHub`, `CodingAgent`) but is not scoped here.
- Multi-relay conflict resolution / eventual-consistency guarantees beyond basic redundancy (best-effort, not exhaustively specified).
- **Buzz direct messages (DMs).** No FR, exit criterion, or NIP-01 filter/tag design in this spec addresses DM subscription or DM `Send()` addressing (e.g. NIP-04/NIP-17 encrypted DMs). `BuzzConfig.DMPolicy` (data-dictionary.md) is carried over from the Telegram/Slack config shape for structural consistency and reserved for a future phase — it has no effect until DM support is explicitly specced. All three phases (FR-001–FR-010) scope to **channel** participation only.

## User Requirements — Functional Requirements

Requirement numbering is carried over verbatim from PRD §6.9, grouped by rollout phase (PRD §8). All three phases are tracked within this single spec directory; phase structure is represented in `tasks.md` and `plan.md`, not as separate spec directories.

### Phase 1 — Read-only participation

- **FR-001:** Connect to configured Nostr relays over WebSocket, with reconnect-on-drop.
- **FR-002:** Subscribe to configured channel IDs via NIP-01 filters.
- **FR-003:** Verify event signatures before processing; drop unsigned/forged events.
- **FR-004:** De-duplicate events by event ID across relays.
- **FR-005:** Map verified events to `domain.IncomingMessage` (`PlatformBuzz`) including metadata.
- **FR-006:** Resolve/create `domain.User` (`RoleGuest`) for `Platform: PlatformBuzz` / `PlatformUID: <pubkey>` on first message, via the existing `usecase/user.Service` (`GetUserByPlatformUID` / `CreateUser`) — see data-dictionary.md for the corrected key convention.
- **FR-007:** Generate-if-absent secp256k1 keypair via existing vault.

> **Note on FR-006:** no existing gateway (Telegram/Slack) currently auto-creates a user on first message — `CreateUser` is presently only invoked from CLI admin commands (`internal/adapter/gateway/cli/admin_commands.go`). Buzz will be the first gateway to call the existing, already-exported `usecase/user.Service` methods from a gateway's message-handling path. This is new *usage* of existing usecase methods, not new usecase-layer code — see architecture.md.

### Phase 2 — Full gateway

- **FR-008:** Publish signed `kind:9` channel messages via `Send()`, plus a `kind:10100` agent-profile event declaring this agent's identity (P0.2 finding: Buzz has no per-message agent tag — agent identity is a separate profile event, not a message tag; corrected 2026-08-02).
- **FR-009:** Apply loop-prevention guard to agent-authored reply chains, sourcing `sender_is_agent` from a `kind:9000`/`kind:10100` membership+profile cache (research.md Q2, corrected 2026-08-02) rather than a per-message field.

### Phase 3 — Tool integration

- **FR-010:** Trigger `GitHub`/`CodingAgent`/`RepoSearch` tools from Buzz messages under existing RBAC.
- **FR-011** *(added 2026-08-02, research.md Q6):* `ChatService`'s tool-invocation path (`usecase/chat/tool_conversion.go`) MUST call `tool.Service.ExecuteWithUser()` instead of the unchecked `Execute()`, for all platforms — Telegram, Slack, CLI, and Buzz — so tool execution is actually RBAC- and rate-limit-checked, not just for Buzz.
- **FR-012** *(added 2026-08-02, research.md Q6):* `tool.Service.ListTools()` MUST apply role-based filtering (currently a `TODO` at `internal/usecase/tool/service.go:261`), for all platforms.

> **Scope note:** FR-011/FR-012 fix a pre-existing gap discovered during spec review — tool execution today is unenforced for every platform, not just Buzz. Fixing it only for Buzz would be inconsistent (and impossible to test meaningfully, since the shared `ChatService` path doesn't branch by platform). Decision recorded in research.md Q6: fix for all platforms as part of Phase 3, accepting that existing Telegram/Slack/CLI users may see tool availability change if their role lacks permission for a tool they could previously call unchecked.

## Non-Functional Requirements

Carried over verbatim from PRD §6.10.

**Reliability**
- Gateway MUST continue operating if some (not all) configured relays are unreachable — partial connectivity is not a startup failure.
- Reconnect attempts MUST use bounded exponential backoff (no tight reconnect loop).
- Duplicate events across relays MUST NOT be delivered to the message handler more than once (ties to FR-004).

**Performance**
- No hard latency SLA (chat gateway, not a real-time system) — but per-relay goroutines/buffers MUST be bounded so N configured relays don't cause unbounded resource growth.
- Signature verification MUST NOT block the relay read loop for other events (async/queued verification acceptable).

## System Architecture

See [architecture.md](./architecture.md) for full detail. Summary:

- **New infrastructure layer:** `internal/infrastructure/nostr/` (client.go, event.go, verify.go, subscription.go) — Nostr protocol concerns, outermost layer per Clean Architecture rules.
- **New adapter layer:** `internal/adapter/gateway/buzz/gateway.go` — `domain.Gateway` implementation, mirrors `internal/adapter/gateway/slack/gateway.go`.
- **Domain change:** one new constant, `domain.PlatformBuzz`, in `internal/domain/message.go`. No changes to `Gateway`, `MessageHandler`, or the `Send`/`OnMessage` contract.
- **Config change:** new `BuzzConfig` in `internal/config/gateway_config.go`, added to `GatewaysConfig` alongside `Telegram`/`Slack`/`CLI`/`WebUI`.
- **Wiring:** `cmd/nuimanbot/main.go`, following the identical `if app.Config.Gateways.Buzz.Enabled { ... }` pattern used for Telegram/Slack (~line 924-950).
- **Vault:** generate-if-absent helper for secp256k1 keypairs in `internal/infrastructure/crypto/`, reusing the existing `VersionedVault` AES-256-GCM storage — no new vault subsystem.
- **Observability:** new Prometheus metrics in `internal/infrastructure/metrics`: `buzz_relay_connections`, `buzz_events_received_total`, `buzz_events_published_total`, `buzz_signature_verification_failures_total`.

## Scope of Changes

**New files:**
- `internal/infrastructure/nostr/client.go` — relay WebSocket client (connect, reconnect, multi-relay fanout)
- `internal/infrastructure/nostr/event.go` — NIP-01 event construction, ID computation, signing
- `internal/infrastructure/nostr/verify.go` — signature verification for inbound events
- `internal/infrastructure/nostr/subscription.go` — filter-based subscription management
- `internal/adapter/gateway/buzz/gateway.go` — `domain.Gateway` implementation for Buzz

**Modified files:**
- `internal/domain/message.go` — add `PlatformBuzz Platform = "buzz"` constant
- `internal/config/gateway_config.go` — add `BuzzConfig` struct, add `Buzz BuzzConfig` field to `GatewaysConfig`
- `cmd/nuimanbot/main.go` — wire Buzz gateway construction/start alongside Telegram/Slack blocks
- `internal/infrastructure/crypto/` — generate-if-absent secp256k1 keypair helper
- `internal/infrastructure/metrics/prometheus.go` — new Buzz metrics

**Dependencies:**
- Promote `github.com/gorilla/websocket` from indirect to direct dependency (already in `go.mod` as indirect, v1.5.3).
- Add a secp256k1 signing library — exact choice (hand-rolled NIP-01 vs. `github.com/nbd-wtf/go-nostr` vs. `github.com/btcsuite/btcd/btcec` for signing only) is an open research question, see [research.md](./research.md); resolved via spike before Phase 1 coding begins.

## Breaking Changes

None. `PlatformBuzz` is additive to the `Platform` enum. `BuzzConfig` is additive to `GatewaysConfig` with `Enabled: false` default (opt-in). No changes to existing `domain.Gateway` interface, `IncomingMessage`/`OutgoingMessage` shapes, or other gateways' behavior.

## Success Criteria and Acceptance Criteria

Carried over verbatim from PRD §8 (Rollout Plan) exit criteria and Overall Acceptance.

### Phase 1 exit criteria — Read-only participation

- Gateway connects to all configured relay URLs, with reconnect-on-drop.
- Event signatures are verified before processing; forged/unsigned events are dropped and counted (`buzz_signature_verification_failures_total`).
- Duplicate events (same event ID delivered by multiple relays) are forwarded to the message handler exactly once.
- First message from a new `sender_pubkey` creates a `domain.User` with `RoleGuest`.
- All Buzz-originated messages pass through `ValidateInput()` identically to other platforms.
- If no private key is configured, a secp256k1 keypair is generated on first run and persisted via the existing vault (`VersionedVault`, AES-256-GCM); subsequent runs reuse the persisted key without regenerating it (FR-007).

### Phase 2 exit criteria — Full gateway

- `Send()` publishes correctly-signed `kind:9` channel messages to configured relays.
- Agent identity is published as a `kind:10100` profile event (not a per-message tag — corrected 2026-08-02 per P0.2 finding), verifiable/decodable by another Buzz-aware client.
- Gateway subscribes to `kind:9000` (channel membership) and `kind:10100` (profile) events per joined channel and maintains a pubkey→is_agent cache; `sender_is_agent` in `IncomingMessage.Metadata` reflects this cache, not a per-message field.
- Loop-prevention guard, consulting the is_agent cache, demonstrably terminates a runaway agent-to-agent reply chain within the defined time window (test: simulated N-message exchange terminates rather than running indefinitely).

### Phase 3 exit criteria — Tool integration

- Buzz channel messages can trigger `GitHub`/`CodingAgent`/`RepoSearch` tool execution under the existing RBAC + rate-limiting pipeline, with no bypass for Buzz-originated requests.
- Tool execution triggered from Buzz is audit-logged identically to tool execution triggered from other platforms.
- `ChatService`'s tool-invocation path calls `ExecuteWithUser()` (not the unchecked `Execute()`), verified by a regression test asserting RBAC/rate-limiting is enforced for tool calls originating from **each** platform (Telegram, Slack, CLI, Buzz), not just Buzz.
- `ListTools()` returns a role-filtered tool list per caller, verified for at least two distinct roles (`RoleGuest` vs `RoleAdmin`) returning different tool sets where the fixture data warrants it.

**Resolved 2026-08-02 (research.md Q6):** tool execution is unenforced for every platform today, not just Buzz — `tool_conversion.go` calls the unchecked `Execute()`, and `ListTools` ignores role entirely. Decision: fix this for all platforms as part of Phase 3 (FR-011, FR-012), rather than scoping Buzz down to match the existing gap. This is a deliberate behavior change for existing Telegram/Slack/CLI users, not a Buzz-only fix.

### Overall Acceptance

Buzz support is complete when a NuimanBot-hosted agent can join a Buzz channel, participate in multi-agent conversation without triggering reply loops, and safely execute approved tools triggered from that channel — all under the existing security, RBAC, and audit pipeline, with no reduction in the guarantees NuimanBot already provides on other platforms.

### Quality Gates

Every task across all three phases must pass the standard AGENTS.md quality gates before being marked complete:

```bash
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

## Risks and Mitigation

| Risk | Mitigation |
|---|---|
| Wrong choice of Nostr/signing library stalls implementation | Spike research question resolved before Phase 1 coding (see research.md) |
| Agent-to-agent reply loops cause runaway tool/API usage | FR-009 loop-prevention guard; NFR requires demonstrable termination within a time window; test coverage required per PRD §6.6 |
| Malicious/compromised agents inject adversarial content | Existing Security Service `ValidateInput()` pipeline applied unchanged (PRD §6.7); no bypass for Buzz |
| Forged events accepted as trusted input | Signature verification (FR-003) required before any event is treated as trusted; unsigned/forged events dropped, not just logged |
| Relay unavailability causes gateway-wide failure | NFR: partial connectivity is not a startup failure; bounded exponential backoff reconnect |
| Unbounded resource growth from N relays | NFR: per-relay goroutines/buffers must be bounded |

## Timeline and Milestones

Phased per PRD §8 Rollout Plan — see [plan.md](./plan.md) and [tasks.md](./tasks.md) for task-level breakdown and dependencies. No fixed calendar dates specified in the PRD; sequencing is Phase 1 → Phase 2 → Phase 3, each gated on its own exit criteria before proceeding.

## References

- Source PRD (now co-located): [nuimanbot-support-buzz-PRD.md](./nuimanbot-support-buzz-PRD.md)
- Clean Architecture layering: `AGENTS.md`
- Existing gateway pattern: `internal/domain/gateway.go`, `internal/adapter/gateway/slack/gateway.go`, `internal/adapter/gateway/telegram/gateway.go`
- Nostr protocol: [nostr.com](https://nostr.com/)
