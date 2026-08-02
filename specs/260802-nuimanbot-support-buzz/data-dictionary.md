# Data Dictionary: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02

## Purpose

Defines the data structures, types, and schemas introduced or extended by Buzz gateway support: configuration shape, domain metadata carried on messages, and the NIP-01 Nostr event structure the infrastructure layer must construct/parse.

## Entities

### `config.BuzzConfig`

New config struct in `internal/config/gateway_config.go`, mirroring the shape of `TelegramConfig`/`SlackConfig`.

| Field | Type | YAML key | Meaning |
|---|---|---|---|
| `Enabled` | `bool` | `enabled` | Whether the Buzz gateway is active. Default `false` (opt-in). |
| `PrivateKey` | `domain.SecureString` | `private_key` | secp256k1 private key, vault-stored. Generated on first run if absent (FR-007). |
| `Relays` | `[]string` | `relays` | List of `wss://` relay URLs to connect to. |
| `NIP05` | `string` | `nip05` | Optional verified identifier (NIP-05); used only as an optional auto-trust signal, not required for MVP. |
| `ChannelIDs` | `[]string` | `channel_ids` | Channels to subscribe to on connect. |
| `DMPolicy` | `config.DMPolicy` | `dm_policy` | Reuses existing `DMPolicy` enum (`pairing`/`allowlist`/`open`) already defined for Telegram/Slack. |

Added to `GatewaysConfig` as `Buzz BuzzConfig \`yaml:"buzz"\``, alongside `Telegram`, `Slack`, `CLI`, `WebUI`.

### `domain.Platform` (extended)

`internal/domain/message.go` — one new constant added to the existing enum:

```go
const PlatformBuzz Platform = "buzz"
```

No other domain type changes. `IncomingMessage`/`OutgoingMessage` struct shapes are unchanged; Buzz-specific data rides in `Metadata map[string]any`.

## Value Objects — IncomingMessage.Metadata keys (Buzz)

Populated by `internal/adapter/gateway/buzz/gateway.go` when mapping a verified Nostr event to `domain.IncomingMessage`, consistent with how the Slack gateway stores `channel`/`thread_ts` (see `internal/adapter/gateway/slack/gateway.go:183-188`).

| Key | Go type | Meaning |
|---|---|---|
| `event_id` | `string` | Nostr event ID (hex-encoded, SHA-256 of the serialized event per NIP-01). |
| `relay_url` | `string` | The relay URL the event was received from (for audit/debugging; a given event may arrive from multiple relays — see dedup, FR-004). |
| `sender_pubkey` | `string` | Author's Nostr public key (hex-encoded secp256k1 x-only pubkey per NIP-01). Used to derive `buzz:<pubkey>` as the `domain.User` key. |
| `sender_is_agent` | `bool` | Derived from Buzz's agent-identity metadata/tags. Used by the loop-prevention guard (FR-009) to avoid agent-to-agent runaway reply chains. |
| `channel_id` | `string` | Buzz channel identifier the event belongs to. |
| `signature` | `string` | Event signature (hex-encoded Schnorr signature per NIP-01), retained for audit trail after verification. |

`OutgoingMessage.Metadata` (Phase 2, for `Send()`) is expected to accept at minimum `channel_id` (target channel) and will need an outgoing equivalent of the agent-marker tag (exact tag name TBD — see research.md Q5) so other Buzz clients can distinguish this agent's authorship.

## NIP-01 Event Structure (infrastructure-internal)

Modeled in `internal/infrastructure/nostr/event.go`. Not exposed outside the infrastructure layer — the adapter layer maps this to `domain.IncomingMessage`/`domain.OutgoingMessage`.

| Field | Type | Meaning |
|---|---|---|
| `ID` | `string` (hex) | SHA-256 hash of the serialized event (per NIP-01 canonical serialization). |
| `PubKey` | `string` (hex) | Author's public key. |
| `CreatedAt` | `int64` (unix seconds) | Event timestamp. |
| `Kind` | `int` | NIP-01 event kind (exact kind(s) used by Buzz for channel messages TBD — research.md Q5). |
| `Tags` | `[][]string` | NIP-01 tag array — expected to carry channel ID and Buzz's agent-identity marker (exact tag names TBD). |
| `Content` | `string` | Message text/content. |
| `Sig` | `string` (hex) | Schnorr signature over `ID`, using `PubKey`'s corresponding private key. |

### Filter (subscription.go)

NIP-01 `REQ` filter shape used to subscribe to configured channels — fields TBD pending research.md Q5 (exact tag-based filtering Buzz requires), but at minimum expected to include `kinds`, `#channel-tag` (or equivalent), and `since` (for reconnect/backfill behavior).

## Enumerations

- `config.DMPolicy` — reused, no changes (`pairing`, `allowlist`, `open`).
- `domain.Role` — reused, no changes (`RoleGuest`, `RoleUser`, `RoleAdmin`); Buzz users default to `RoleGuest` per FR-006.
- `domain.Platform` — extended with `PlatformBuzz`.

## API Request/Response Types

Not applicable in the traditional sense — Buzz has no REST API; all interaction is WebSocket-based Nostr relay messages (`EVENT`, `REQ`, `CLOSE`, `OK`, `EOSE`, `NOTICE` per NIP-01 relay-client message types). These are internal to `internal/infrastructure/nostr/client.go` and not part of the public data dictionary surface consumed by the domain/adapter layers.
