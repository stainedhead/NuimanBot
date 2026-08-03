# Architecture: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Status:** Draft

## Architecture Overview

No new components or layers are introduced. This is a remediation pass entirely within the existing Buzz gateway architecture established by the original feature spec (`specs/260802-nuimanbot-support-buzz/`). Clean Architecture boundaries are preserved: all fixes stay within `internal/adapter/gateway/buzz/` (adapter layer), `internal/infrastructure/nostr/` (infrastructure layer), `internal/config/` (config), and documentation. No domain or usecase layer changes are required — FR-011's new interface is adapter-layer, consumer-side, matching the existing `chat.UserService` pattern rather than introducing a new dependency direction.

## Component Architecture

```
internal/adapter/gateway/buzz/
├── gateway.go            # Cluster A: lifecycle guards (FR-001/002/003), sync (FR-008),
│                          #   interface segregation (FR-011), test coverage (FR-012),
│                          #   metric scope (FR-013), redundant resolution (FR-016)
│                          # Cluster B: FR-004 gauge wiring (touches same file — coordinate)
├── gateway_test.go        # New/expanded tests for FR-001/002/003/004/008/012
├── agent_cache.go          # Cluster D: FR-009 TTL/capacity eviction
├── agent_cache_test.go     # New eviction test
└── loop_guard.go           # Referenced by FR-009 (bounded in practice, no fix required)

internal/infrastructure/nostr/
├── subscription.go        # Cluster D: FR-010 Since population on reconnect
├── subscription_test.go   # New Since-population test
└── client.go               # Possible: connect/disconnect hook for FR-004 (Cluster B),
                             #   reconnect high-water-mark plumbing for FR-010 (Cluster D)

internal/config/
└── gateway_config.go       # Cluster C: FR-005 NIP05 doc comment (reserved)

support_docs/
└── buzz-guide.md            # Cluster C: FR-005 (NIP05 reserved), FR-007 (env var doc fix)

documentation/
└── technical-details.md    # Cluster C: FR-014 (stale SkillPermissions/ToolPermissions example)
```

## Layer Responsibilities (unchanged by this pass)

- **`internal/infrastructure/nostr/`** — protocol-only, no Prometheus/metrics awareness (per FR-004's resolved decision). Owns `Filter`/`Since` marshaling (FR-010) and low-level connection bookkeeping (`registerConn`/`unregisterConn`).
- **`internal/adapter/gateway/buzz/`** — owns all metric emission (FR-004, FR-013), lifecycle guards (FR-001/002/003), concurrency safety (FR-008), and its own consumer-side interfaces (FR-011). This is where Prometheus awareness lives for Buzz, consistent with how other gateways are structured.
- **`internal/config/`** — schema only; `NIP05` remains a declared-but-reserved field (FR-005), not removed, to preserve config-file compatibility for users who've already set it.

## Data Flow

### FR-004: Relay connection count → gauge (Cluster B)

```
nostr.Client (registerConn/unregisterConn, client.go:195-210)
    │
    │  ConnectedRelayCount() — existing method, currently test-only caller
    ▼
buzz.Gateway (adapter layer — new: ticker or callback-driven poll)
    │
    ▼
metrics.BuzzRelayConnections.WithLabelValues(relayURL, status).Set(...)
```

Exact trigger mechanism (ticker vs. callback) is an open implementation question — see research.md question 3.

### FR-010: Reconnect backfill (Cluster D)

```
Relay disconnects
    │
nostr.Client detects disconnect, attempts reconnect
    │
    ▼
Gateway/Client computes Since = last-successfully-processed-event timestamp
    │
    ▼
New REQ frame built with Filter.Since populated (subscription.go:57-61)
    │
    ▼
Relay replays events since Since → gateway.processEvent (existing dedupe via seen/seenMu
    prevents double-delivery of events already processed before the disconnect)
```

Per-relay vs. gateway-wide high-water mark is an open implementation question — see research.md question 1.

## Sequence Diagrams

Not needed for this pass — no new multi-actor flows are introduced; existing sequence diagrams in the original feature spec's architecture.md (if present) remain accurate for the steady-state message flow. This pass modifies error/edge-case handling within already-diagrammed flows (lifecycle start/stop, reconnect).

## Integration Points

- Prometheus (`internal/infrastructure/metrics/prometheus.go`) — no new metrics declared; FR-004 wires up an existing declaration, FR-013 possibly adjusts help text/labels on an existing declaration.
- Nostr relay protocol (NIP-01 `Since` filter) — FR-010 is the first use of an already-declared-but-dormant protocol field; no new protocol surface.

## Architectural Decisions

1. **Metrics stay in the adapter layer, not infrastructure** (FR-004, reaffirmed for FR-013). Rationale: `internal/infrastructure/nostr/` explicitly states a goal of remaining a swappable, metrics-agnostic protocol layer (`event.go:1-4`). Decided by product owner, 2026-08-02.
2. **Config fields are marked reserved, not removed, when unwired** (FR-005). Rationale: matches existing `DMPolicy` precedent; avoids breaking user configs that already set the field; keeps a clear signal for the field's future intended use (PRD §6.5).
3. **No new cache-eviction abstraction is introduced project-wide for FR-009** — the implementer picks a bound/TTL locally for `agentCache`, documented in `implementation-notes.md`, rather than building a shared eviction utility. Rationale: no existing precedent to generalize from; premature abstraction avoided per AGENTS.md's "don't design for hypothetical future requirements."
4. **FR-011's interface lives with the consumer (`buzz` package), not with `user.Service`.** Rationale: matches the project's stated interface-segregation convention (AGENTS.md: "Define interfaces where they are used, not where implemented") and the existing `chat.UserService` precedent.
