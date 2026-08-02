# Architecture: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Status:** Draft

## Architecture Overview

Buzz support follows NuimanBot's existing Clean Architecture layering exactly as Slack/Telegram do. The only new layer concern is the Nostr transport protocol itself, which is isolated entirely in `internal/infrastructure/nostr/` (outermost layer). No changes are required to `domain.Gateway`, `domain.MessageHandler`, or the core `Send`/`OnMessage` contract — Buzz fits the existing interface without modification.

```
domain/            <- +1 constant (PlatformBuzz), no interface changes
  message.go

usecase/            <- unchanged; Security Service, RBAC, tool service already
                       apply identically once Buzz messages reach domain.IncomingMessage

adapter/gateway/buzz/  <- NEW: domain.Gateway implementation
  gateway.go            (mirrors adapter/gateway/slack/gateway.go)

infrastructure/nostr/  <- NEW, outermost layer: Nostr protocol concerns only
  client.go              (relay WebSocket connect/reconnect/fanout)
  event.go               (NIP-01 event construction, ID, signing)
  verify.go              (signature verification)
  subscription.go        (filter-based subscription mgmt)

infrastructure/crypto/  <- MODIFIED: +generate-if-absent secp256k1 keypair helper
config/                 <- MODIFIED: +BuzzConfig
cmd/nuimanbot/main.go   <- MODIFIED: +wiring block, mirrors Telegram/Slack
```

## Component Architecture

- **`internal/infrastructure/nostr` (infrastructure layer)**
  - `client.go` — owns WebSocket connections to one or more relays. Responsible for connect, bounded-exponential-backoff reconnect-on-drop, and multi-relay fanout (both for publishing and for merging inbound event streams). Exposes a channel-based API (`chan Event`, `Publish(ctx, Event) error`) to the adapter layer — no `domain` or `config` imports beyond what's needed for raw types.
  - `event.go` — pure functions for NIP-01 event construction: canonical serialization, ID computation (SHA-256 of serialized event), and signing (secp256k1/Schnorr, library TBD per research.md Q1).
  - `verify.go` — signature verification for inbound events (FR-003). Must be safe to call off the relay read loop (NFR: verification must not block the read loop — async/queued acceptable).
  - `subscription.go` — builds and manages NIP-01 `REQ` filters for configured channel IDs (FR-002); tracks subscription state per relay connection.

- **`internal/adapter/gateway/buzz` (adapter layer)**
  - `gateway.go` — implements `domain.Gateway` (`Platform() domain.Platform`, `Start(ctx) error`, `Stop(ctx) error`, `Send(ctx, domain.OutgoingMessage) error`, `OnMessage(handler)`), matching the structure of `internal/adapter/gateway/slack/gateway.go`:
    - `New(cfg *config.BuzzConfig) (*Gateway, error)` — constructor, no I/O.
    - `Start(ctx)` — validates config, initializes `nostr.Client` from `internal/infrastructure/nostr`, subscribes to configured channels, launches the event-handling goroutine loop, blocks (mirrors `socketClient.Run()` pattern in Slack).
    - `handleEvents(ctx)` — reads from the `nostr.Client` event channel; for each event: dedupe by event ID (FR-004), verify signature (FR-003, drop on failure + increment `buzz_signature_verification_failures_total`), map to `domain.IncomingMessage` (FR-005), resolve/create `domain.User` keyed `buzz:<pubkey>` (FR-006), invoke `g.messageHandler`.
    - `Send(ctx, msg)` (Phase 2) — constructs and signs an outgoing NIP-01 event via `nostr.event.go`, agent-tags it, publishes via `nostr.Client.Publish`.
    - `Stop(ctx)` — cancels context, closes relay connections gracefully.

- **`internal/config`**
  - `BuzzConfig` added per data-dictionary.md, `Buzz BuzzConfig` field added to `GatewaysConfig`.

- **`cmd/nuimanbot/main.go`**
  - New block after the existing Slack block (~line 950), identical shape:
    ```go
    if app.Config.Gateways.Buzz.Enabled {
        buzzGateway, err := buzz.New(&app.Config.Gateways.Buzz)
        if err != nil {
            slog.Warn("Failed to create Buzz gateway", "error", err)
        } else {
            app.connectGateway(buzzGateway)
            gateways = append(gateways, buzzGateway)
            go func() {
                slog.Info("Starting gateway", "platform", "buzz")
                if err := buzzGateway.Start(ctx); err != nil {
                    slog.Error("Buzz gateway error", "error", err)
                }
            }()
        }
    }
    ```
    (Phase 2 wiring — `Send()` publish path — is exercised the same way `connectGateway` already wires `Send` for other gateways; no additional main.go change needed beyond the block above, which is added once in Phase 1 with `Send()` initially returning `not yet supported` until Phase 2 implements it, OR the block is added in Phase 2 once `Send()` is implemented — see plan.md for sequencing decision.)

## Layer Responsibilities

| Layer | Responsibility | Buzz-specific additions |
|---|---|---|
| domain | Entities, business rules | `PlatformBuzz` constant only |
| usecase | Security Service (`ValidateInput`), RBAC, tool service, Chat Service | None — applies unchanged to Buzz-originated `domain.IncomingMessage` (per PRD §6.7) |
| adapter | Gateway implementation, converts external → domain types | `adapter/gateway/buzz/gateway.go` |
| infrastructure | Nostr protocol, WebSocket transport, crypto | `infrastructure/nostr/*`, +helper in `infrastructure/crypto/` |

## Data Flow (Phase 1 — read path)

```
Relay (wss://)
   │  raw NIP-01 EVENT message
   ▼
infrastructure/nostr/client.go   — WebSocket read loop, per-relay goroutine
   │  nostr.Event{ID, PubKey, Kind, Tags, Content, Sig, relay origin}
   ▼
adapter/gateway/buzz/gateway.go  — handleEvents loop
   │  1. dedupe by event.ID (in-memory set/LRU) — FR-004
   │  2. infrastructure/nostr/verify.go: verify Sig against PubKey — FR-003
   │     (drop + count buzz_signature_verification_failures_total if invalid)
   │  3. map to domain.IncomingMessage{Platform: PlatformBuzz, PlatformUID: pubkey,
   │     Metadata: {event_id, relay_url, sender_pubkey, sender_is_agent, channel_id, signature}} — FR-005
   ▼
usecase (via app-level user resolution, not shown as a separate file here)
   │  4. resolve/create domain.User keyed "buzz:<pubkey>", default RoleGuest — FR-006
   ▼
g.messageHandler(ctx, incomingMsg)  — same MessageHandler used by all gateways
   ▼
usecase/security.Service.ValidateInput()  — unchanged, applies identically (PRD §6.7)
   ▼
usecase/chat (ChatService.ProcessMessage) — unchanged
```

## Sequence Diagram (Phase 1 connect + receive)

```
main.go            buzz.Gateway         nostr.Client          Relay
  │  Start(ctx)         │                    │                  │
  ├────────────────────>│                    │                  │
  │                      │  New(relays)       │                  │
  │                      ├───────────────────>│                  │
  │                      │                    │  Dial wss://…    │
  │                      │                    ├─────────────────>│
  │                      │                    │<─────────────────┤ (connected)
  │                      │  Subscribe(filters)│                  │
  │                      ├───────────────────>│  REQ …           │
  │                      │                    ├─────────────────>│
  │                      │                    │<─────────────────┤ EVENT …
  │                      │<───────────────────┤ (event chan)     │
  │                      │  verify + dedupe   │                  │
  │                      │  map to Incoming   │                  │
  │                      │  Msg               │                  │
  │                      │  messageHandler()  │                  │
  │                      │  (async, per event)│                  │
```

## Integration Points

- **Security Service** (`internal/usecase/security/service.go`) — `ValidateInput()` applied to all Buzz `domain.IncomingMessage.Text` identically to other platforms, no bypass.
- **Tool Service** (`internal/usecase/tool/service.go`) — Phase 3 only; RBAC + rate-limiting + audit pipeline applied unchanged to tool calls triggered from Buzz messages.
- **Credential Vault** (`internal/infrastructure/crypto/`) — generate-if-absent secp256k1 keypair, stored via existing `VersionedVault` (AES-256-GCM), no new storage subsystem.
- **Metrics** (`internal/infrastructure/metrics/prometheus.go`) — new counters/gauges: `buzz_relay_connections{relay_url,status}`, `buzz_events_received_total{channel_id,sender_is_agent}`, `buzz_events_published_total{status}`, `buzz_signature_verification_failures_total`.
- **User/RBAC resolution** — reuses whatever existing mechanism resolves/creates `domain.User` on first message for Telegram/Slack (`buzz:<pubkey>` follows the same `PlatformUID` convention).

## Architectural Decisions

- **No `domain.Gateway` interface changes.** Buzz's event-driven, multi-relay nature is fully absorbed by the infrastructure + adapter layers; the domain-facing contract (`Start`/`Stop`/`Send`/`OnMessage`) is unchanged, matching PRD §7's rejection of an MCP-bridge alternative in favor of the native gateway pattern.
- **Nostr protocol isolated to infrastructure layer.** No Nostr types (events, filters, relay messages) leak into `domain` or `usecase`; the adapter layer is the sole translation point, consistent with AGENTS.md's dependency rules (inner layers define interfaces, outer layers implement them — here, `domain.Gateway` is the interface, `buzz.Gateway` the implementation).
- **Vault storage format unchanged.** A Nostr private key is treated as an opaque `domain.SecureString`, per PRD §6.4 — avoids a new vault subsystem for a single new secret type.
- **Signing/client library choice deferred to a pre-Phase-1 spike**, not decided in this document — see research.md Q1. Whichever is chosen, it must stay confined to `internal/infrastructure/nostr/event.go` and not leak into the adapter or higher layers, so the eventual choice is swappable without touching `gateway.go`'s public behavior.
