# Data Dictionary: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02

## Purpose

This document defines the data structures touched or introduced by the review-remediation fixes. Most FRs in this pass are behavioral (add a guard, add a test, wire an existing metric) rather than data-structure-introducing; this dictionary covers the handful that do add or change shape.

## Entities / Structs

### `buzz.Gateway` (existing, modified — FR-008, FR-011)

Relevant fields (per `internal/adapter/gateway/buzz/gateway.go:46-59`):

| Field | Type (current) | Change |
|---|---|---|
| `client` | `*nostr.Client` | FR-008: add mutex protection (read from `Send()`/`Stop()`/`handleEvents()`, written in `Start()`) |
| `cancel` | `context.CancelFunc` | FR-008: add mutex protection |
| `messageHandler` | handler type (existing) | FR-008: add mutex protection (written in `OnMessage()`) |
| `userService` | `*user.Service` (concrete) | FR-011: replace with new consumer-side interface, e.g. `buzzUserService` |
| `seen` / `seenMu` | existing dedupe map + mutex | Unchanged — reference pattern for FR-008's fix |

### New interface: `buzz.UserService` (or similar name — FR-011)

Consumer-side interface, adapter-layer (in `internal/adapter/gateway/buzz/`), modeled on `usecase/chat/service.go:44`'s `UserService` pattern. Exact method set TBD pending research question 4 (research.md) — expected to cover `GetUserByPlatformUID` and `CreateUser` at minimum.

```go
// buzzUserService is the minimal user-lookup surface Gateway depends on.
type buzzUserService interface {
    GetUserByPlatformUID(ctx context.Context, platform domain.Platform, uid string) (*domain.User, error)
    CreateUser(ctx context.Context, /* ... */) (*domain.User, error)
}
```

### `nostr.Filter` (existing, modified — FR-010)

`internal/infrastructure/nostr/subscription.go:57-61`. `Since *int64` (or equivalent) field already exists and is marshaled; this pass populates it at reconnect time rather than adding a new field. New: a per-gateway (or per-relay, pending research question 1) high-water-mark tracker for "timestamp of last successfully processed event," used to compute the `Since` value on reconnect.

### `agentCache` (existing, modified — FR-009)

`internal/adapter/gateway/buzz/agent_cache.go`. `isAgent map[string]bool` (or similar) gains either:
- a TTL wrapper (timestamp per entry, periodic sweep), or
- an LRU/bounded-capacity structure,

per the implementer's choice (product owner deferred this design call — see research.md / spec.md FR-009).

## Value Objects

No new value objects introduced. `BuzzConfig.NIP05` (FR-005) is an existing `string` field — no shape change, only doc-comment and documentation-text changes marking it reserved.

## Interfaces

- New: adapter-layer consumer-side `buzzUserService`-style interface (FR-011, see above).
- No changes to `nostr.Client`'s public interface required by FR-004 if the ticker/polling approach is chosen (research question 3); a small change (new callback registration method) may be needed if the callback approach is chosen instead. To be settled during Cluster B implementation and recorded here or in `implementation-notes.md`.

## Enumerations

None introduced.

## API Request/Response Types

Not applicable — no external API surface in this pass. `BuzzRelayConnections` (Prometheus `GaugeVec`, already declared in `internal/infrastructure/metrics/prometheus.go:244-250`) gains its first `.WithLabelValues(relayURL, status).Set(...)` call sites (FR-004) — label values `relayURL` (string) and `status` (string, e.g. `"connected"`/`"disconnected"`) already match the existing declaration; no schema change.
