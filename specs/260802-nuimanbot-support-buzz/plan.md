# Plan: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Status:** Planning

## Development Approach

Strict TDD (Red-Green-Refactor) per AGENTS.md, task by task, within a single spec directory covering all three PRD-defined phases. Each phase is gated on the previous phase's exit criteria (spec.md) being met and its quality gates passing (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run`, `go test ./...`, build, `./bin/nuimanbot --help`).

Sequencing:
1. Resolve the Nostr library spike (research.md Q1) before any Phase 1 coding task.
2. Build infrastructure layer (`internal/infrastructure/nostr/`) bottom-up: event construction/signing → verification → subscription filters → relay client (connect/reconnect/fanout).
3. Build adapter layer (`internal/adapter/gateway/buzz/gateway.go`) on top, wiring dedupe + verify + map-to-domain + RBAC resolution.
4. Wire config (`BuzzConfig`) and `main.go` — enables end-to-end manual testing against a real or local Nostr relay.
5. Phase 2 adds the publish path (`Send()`) and loop-prevention guard.
6. Phase 3 wires tool-triggering from Buzz messages, reusing existing RBAC/tool service unchanged.

## Phase Breakdown

### Phase 1 — Read-only participation (FR-001–FR-007)

Goal: NuimanBot can connect, subscribe, verify, dedupe, and log/store incoming Buzz messages with correct RBAC user resolution — no publishing. See tasks.md P1.\* for task-level breakdown.

### Phase 2 — Full gateway (FR-008, FR-009)

Goal: `Send()` publishes signed, agent-tagged events; loop-prevention guard is demonstrably effective. Depends on Phase 1's `nostr.event.go` (signing) and `nostr.client.go` (publish path) being complete. See tasks.md P2.\*.

### Phase 3 — Tool integration (FR-010)

Goal: Buzz messages can trigger `GitHub`/`CodingAgent`/`RepoSearch` tools under existing RBAC with full audit logging. Depends on Phase 2's loop-prevention guard (to avoid tool-triggering loops) and the standard `ChatService`/`ToolService` wiring already used by other gateways. See tasks.md P3.\*.

## Critical Path

```
research.md Q1 spike (library choice)
   → P1.1 event.go (construct/ID/sign)
   → P1.2 verify.go (signature verification)
   → P1.3 subscription.go (filters)
   → P1.4 client.go (relay connect/reconnect/fanout)
   → P1.5 domain.PlatformBuzz constant
   → P1.6 config.BuzzConfig
   → P1.7 adapter/gateway/buzz/gateway.go (Start, dedupe, verify, map, handleEvents)
   → P1.8 RBAC user resolution (buzz:<pubkey> → RoleGuest)
   → P1.9 main.go wiring (Buzz enabled block)
   → P1.10 Prometheus metrics
   → [Phase 1 exit criteria verified]
   → P2.1 Send() publish path (event.go signing reused)
   → P2.2 agent-tagging on outgoing events
   → P2.3 loop-prevention guard
   → [Phase 2 exit criteria verified]
   → P3.1 tool-triggering wiring from Buzz messages (reuses existing ToolService)
   → [Phase 3 exit criteria verified, Overall Acceptance met]
```

## Testing Strategy

- **Unit tests** for every file in `internal/infrastructure/nostr/`: event ID computation against known NIP-01 test vectors, signature sign/verify round-trip, filter construction, dedupe logic.
- **Unit tests** for `adapter/gateway/buzz/gateway.go`: table-driven tests mapping raw `nostr.Event` → `domain.IncomingMessage`, verification-failure drop path (+ metric increment), dedupe-across-relays path, RBAC user creation on first message.
- **Fake/mock relay** (in-process WebSocket test server, similar to how Slack/Telegram tests likely mock their SDK clients — confirm pattern by reading `internal/adapter/gateway/slack/*_test.go` before writing) for `client.go` reconnect-on-drop and bounded-backoff behavior.
- **Loop-prevention test** (Phase 2): simulated N-message agent-to-agent exchange must terminate within the defined time window — this is an explicit PRD exit criterion, not optional.
- **Security pipeline regression test**: confirm a Buzz-originated message still passes through `ValidateInput()` identically to a Slack/Telegram message (no bypass) — likely an existing-test-pattern extension in `internal/usecase/security`.
- **Tool RBAC regression test** (Phase 3): confirm a Buzz-triggered tool call goes through identical RBAC + rate-limit + audit path as other platforms — extend existing `internal/usecase/tool` test patterns.
- Follow existing coverage conventions in this repo (see recent commits: "gateway, domain, tools coverage improvements", "adapter/web, api, mcp coverage improvements").

## Rollout Strategy

Matches PRD §8 exactly:
1. Phase 1 ships with `Send()` unimplemented/no-op-safe (read-only) — validates parsing, verification, RBAC mapping with zero risk of the agent posting.
2. Phase 2 enables `Send()` and wires loop prevention — first point at which the agent can post to Buzz.
3. Phase 3 enables tool-triggering from Buzz content.

`BuzzConfig.Enabled` defaults to `false`; rollout in any given deployment is opt-in via config regardless of which code phases have shipped.

## Success Metrics

Directly the Phase exit criteria and Overall Acceptance statement in spec.md — no additional business metrics defined by the PRD. Operational health is observed via the four new Prometheus metrics (relay connection status, events received/published, signature verification failures).
