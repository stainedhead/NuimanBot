# Research: NuimanBot Support for Buzz — Auto-Review Remediation

**Created:** 2026-08-02
**Source PRD:** `nuimanbot-support-buzz-auto-review-PRD.md` (this spec directory)

## Note: Design Questions Are Pre-Resolved

Unlike a typical spec, this one derives from a *code review* rather than a fresh feature PRD. The review PRD's own "Open Questions" section (see source PRD, "Open Questions") already resolved every design decision with the product owner on 2026-08-02, before this spec was created:

1. **FR-004** — Gauge set from the `buzz` adapter layer via `ConnectedRelayCount()`, not from `internal/infrastructure/nostr` directly.
2. **FR-005** — `NIP05` marked reserved now (matching `DMPolicy` precedent), not wired up.
3. **FR-006** — Deferred out of this remediation pass entirely (pre-existing, not Buzz-specific).
4. **FR-009** — Left to the implementer: pick a reasonable TTL/capacity bound for `agentCache` eviction, document rationale in `implementation-notes.md`, cover with a test. No existing cache-eviction pattern in this codebase to match against.
5. **FR-010** — Implement reconnect backfill via `Filter.Since`; reclassified P2→P1 as a real reliability gap (not a confirmed prior scope cut).

No new research questions are needed to resolve *design intent* — that work is done. The items below are the genuinely open *implementation-detail* questions surfaced while breaking the review PRD into tasks.

## Research Questions (implementation-detail only)

1. **FR-010 backfill mechanism:** What is the correct source for the "timestamp of the last successfully processed event" per relay? Candidates: track a single gateway-wide high-water mark, or track per-relay high-water marks (relays may deliver events out of order relative to each other). Needs inspection of `internal/infrastructure/nostr/client.go`'s reconnect/subscription-restart path to see where `Since` would need to be threaded through (`New*Filter` constructors, `subscription.go:57-61`).
2. **FR-010 backfill duplicate-safety:** Does the existing `seen`/`seenMu` dedupe in `gateway.go` have sufficient capacity/TTL to safely absorb a backfill window without either (a) missing dedup on borderline-timestamp events, or (b) growing unbounded itself? (Relates to FR-009's separate unbounded-cache concern — verify these two are not the same map before assuming FR-009's fix also covers this.)
3. **FR-004 wiring mechanism:** Ticker-based polling of `ConnectedRelayCount()` vs. reacting to connect/disconnect callbacks from `internal/infrastructure/nostr/client.go`'s `registerConn`/`unregisterConn` (`client.go:195-210`) — the latter requires either a callback hook or the adapter observing connection-count deltas. Needs to confirm whether `client.go` currently exposes any hook for this, or whether one needs to be added (adapter-layer only, per the resolved decision) without giving `nostr/` Prometheus awareness.
4. **FR-011 interface shape:** Confirm the exact subset of `*user.Service` methods `buzz.Gateway` calls (review PRD names `GetUserByPlatformUID`/`CreateUser` as likely candidates) by reading `gateway.go:48,64` and its call sites, to size the new consumer-side interface correctly — avoid over- or under-scoping relative to `chat.UserService`'s existing pattern (`usecase/chat/service.go:44`).
5. **FR-016 removal vs. documentation:** Determine whether `ChatService.resolveUser` (added by this branch's cross-platform RBAC fix) fully subsumes Buzz's own `resolveUser`/`CreateUser` calls (`gateway.go:271-275,356-368`), or whether Buzz's early resolution is load-bearing for something `ChatService`'s later resolution doesn't cover (e.g., needed before `ChatService.ProcessMessage` is invoked). This determines whether FR-016's fix is a deletion or a documentation-only change.

## Industry Standards

Not applicable — this is an internal reliability/observability remediation pass, not new protocol work. Nostr protocol conventions (NIP-01 `Since` filter semantics) were already researched in the original feature spec (`specs/260802-nuimanbot-support-buzz/research.md`).

## Existing Implementations

- `internal/adapter/gateway/slack/gateway.go:84-86` — reference pattern for nil-client guarding (FR-001).
- `usecase/chat/service.go:44` — reference pattern for consumer-side interface segregation (FR-011).
- `internal/config/gateway_config.go` `DMPolicy` field — reference pattern for "reserved, not yet wired" config field documentation (FR-005).

## API Documentation

Not applicable — no external API changes in this pass.

## Best Practices

- TDD Red-Green-Refactor per AGENTS.md, applied per-FR (see spec.md Non-Functional Requirements).
- `-race`-clean concurrency testing, consistent with the existing Buzz test suite's established pattern (real in-process WebSocket relays, real signed events) per the original review's positive findings.

## Open Questions

None outstanding at the design level (see note above). Implementation-detail questions are listed above and should be resolved during Cluster A/B/D task breakdown, with answers recorded in `implementation-notes.md` as they're settled.

## References

- Source review PRD: `nuimanbot-support-buzz-auto-review-PRD.md` (this directory), "Open Questions" section
- Original feature spec: `specs/260802-nuimanbot-support-buzz/spec.md`, `specs/260802-nuimanbot-support-buzz/research.md`
