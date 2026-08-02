# Tasks: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Status:** Planning

## Progress Summary

0/21 tasks complete (0%)

Every task below follows AGENTS.md's mandatory Red-Green-Refactor cycle: write a failing test first (Red), implement the minimal code to pass it (Green), then refactor for clarity/duplication/naming while keeping tests green (Refactor — not optional). A task is not "done" until all three sub-steps are complete and the quality-gate command passes:

```bash
go fmt ./... && go mod tidy && go vet ./... && golangci-lint run && go test ./... && go build -o bin/nuimanbot ./cmd/nuimanbot && ./bin/nuimanbot --help
```

---

## Phase 0 — Pre-implementation spike

### P0.1 — Resolve Nostr client/signing library choice

- **Dependencies:** none
- **Duration:** ~0.5 day
- **Description:** Spike/evaluate hand-rolled NIP-01 vs. `github.com/nbd-wtf/go-nostr` vs. `github.com/btcsuite/btcd/btcec` (signing-only). Not a TDD task (research, not implementation) — resolve research.md Q1 and record the decision + rationale in implementation-notes.md before starting P1.1.
- **Acceptance criteria:** research.md Q1 marked resolved; implementation-notes.md records the chosen library and why; go.mod dependency direction decided (promote gorilla/websocket to direct + add chosen signing/client lib).

---

## Phase 1 — Read-only participation (FR-001–FR-007)

### P1.1 — NIP-01 event construction, ID computation, signing (`internal/infrastructure/nostr/event.go`)

- **Dependencies:** P0.1
- **Duration:** ~1 day
- **Red:** Write tests asserting event ID computation matches known NIP-01 test vectors (canonical serialization → SHA-256), and that sign/verify round-trips correctly for a generated keypair.
- **Green:** Implement `Event` struct, canonical serialization, ID computation, and signing using the library chosen in P0.1.
- **Refactor:** Extract serialization/hashing helpers if duplicated with verify.go (P1.2); ensure no `domain`/`config` imports leak in.
- **Acceptance criteria:** Event IDs match NIP-01 reference test vectors; sign/verify round-trip passes; `go test ./internal/infrastructure/nostr/...` green.

### P1.2 — Signature verification for inbound events (`internal/infrastructure/nostr/verify.go`)

- **Dependencies:** P1.1
- **Duration:** ~0.5 day
- **Red:** Tests for: valid signature accepted; tampered content rejected; wrong pubkey rejected; malformed event rejected without panic.
- **Green:** Implement `Verify(event Event) (bool, error)`.
- **Refactor:** Ensure verification is a pure function safe to call concurrently/async (NFR: must not block relay read loop).
- **Acceptance criteria:** All forged/malformed cases rejected; valid cases accepted; no panics on malformed input.

### P1.3 — Filter-based subscription management (`internal/infrastructure/nostr/subscription.go`)

- **Dependencies:** P1.1
- **Duration:** ~0.5 day
- **Red:** Tests asserting a `Filter` built from configured channel IDs serializes to the expected NIP-01 `REQ` shape.
- **Green:** Implement `Filter`/`Subscription` construction from `[]string` channel IDs (FR-002).
- **Refactor:** Clarify naming vs. Buzz-specific channel-tag conventions once research.md Q5 is answered (may require a follow-up note if Buzz tag conventions differ from assumed defaults — document in implementation-notes.md, don't silently guess).
- **Acceptance criteria:** Filter construction covers all configured channel IDs; unit tests green.

### P1.4 — Relay WebSocket client: connect, reconnect, multi-relay fanout (`internal/infrastructure/nostr/client.go`)

- **Dependencies:** P1.3
- **Duration:** ~1.5 days
- **Red:** Tests against an in-process fake WebSocket relay server: (a) client connects and receives events; (b) on connection drop, client reconnects with bounded exponential backoff (no tight loop); (c) with N relays configured and one unreachable, the client still delivers events from reachable relays (partial connectivity is not a startup failure — NFR); (d) per-relay goroutines/buffers are bounded (no unbounded growth as relay count grows).
- **Green:** Implement `Client` with per-relay goroutine, reconnect/backoff logic, and a merged output channel.
- **Refactor:** Extract backoff policy into a small reusable helper if it starts duplicating logic; ensure clean shutdown (`Stop`) closes all relay connections without goroutine leaks.
- **Acceptance criteria:** Reconnect-on-drop verified in test; partial-relay-failure does not block other relays; goroutine/buffer bounds verified (e.g., via `go test -race` and a leak check).

### P1.5 — Domain constant: `PlatformBuzz`

- **Dependencies:** none (can run in parallel with P1.1–P1.4)
- **Duration:** ~0.1 day
- **Red:** Existing `domain` tests should still pass; add/extend a table-driven test (if one exists for `Platform` values) to include `PlatformBuzz`.
- **Green:** Add `const PlatformBuzz Platform = "buzz"` to `internal/domain/message.go`.
- **Refactor:** N/A (single constant) — confirm no other switch statements over `Platform` need a new case (grep for `PlatformSlack`/`PlatformTelegram` usage to check for exhaustive switches that might need updating).
- **Acceptance criteria:** `go vet`/`golangci-lint` clean on any switch-over-Platform sites; existing tests green.

### P1.6 — `config.BuzzConfig`

- **Dependencies:** none (can run in parallel with P1.1–P1.4)
- **Duration:** ~0.3 day
- **Red:** Test that a YAML fixture with a `buzz:` block unmarshals into `GatewaysConfig.Buzz` correctly, defaulting `Enabled: false` when absent.
- **Green:** Add `BuzzConfig` struct (per data-dictionary.md) to `internal/config/gateway_config.go`; add `Buzz BuzzConfig \`yaml:"buzz"\`` field to `GatewaysConfig`.
- **Refactor:** Confirm consistent field ordering/style with `TelegramConfig`/`SlackConfig`.
- **Acceptance criteria:** Config unmarshal test green; `Enabled` defaults false; existing config tests unaffected.

### P1.7 — `buzz.Gateway`: Start, dedupe, verify, map to `domain.IncomingMessage` (`internal/adapter/gateway/buzz/gateway.go`)

- **Dependencies:** P1.2, P1.4, P1.5, P1.6
- **Duration:** ~1.5 days
- **Red:** Table-driven tests: (a) valid signed event → correct `domain.IncomingMessage` with all six metadata keys populated (FR-005); (b) unsigned/forged event → dropped, not forwarded to handler, `buzz_signature_verification_failures_total` incremented (FR-003); (c) same event ID from two relays → handler invoked exactly once (FR-004); (d) `Platform()` returns `domain.PlatformBuzz`.
- **Green:** Implement `New`, `Platform`, `Start`, `Stop`, `OnMessage`, and `handleEvents` per architecture.md's Phase 1 data flow (dedupe → verify → map → RBAC resolve → handler).
- **Refactor:** Compare against `slack.Gateway`'s structure for consistency (naming, error wrapping with `%w`, doc comments on exported symbols per AGENTS.md).
- **Acceptance criteria:** All Phase 1 exit criteria in spec.md related to connect/verify/dedupe/mapping are demonstrated by tests; mirrors slack/gateway.go's shape.

### P1.8 — RBAC user resolution: `buzz:<pubkey>` → `RoleGuest` on first message (FR-006)

- **Dependencies:** P1.7
- **Duration:** ~0.5 day
- **Red:** Test that a message from a new `sender_pubkey` creates a `domain.User` with ID `buzz:<pubkey>` and `Role: RoleGuest`; a second message from the same pubkey does not create a duplicate user.
- **Green:** Wire into whatever existing user-resolution mechanism Telegram/Slack use on first message (locate and reuse, don't duplicate — check how Slack/Telegram currently resolve/create users, since neither gateway file shown resolves users directly, it may happen in usecase/chat or a UserRepository).
- **Refactor:** Ensure the `buzz:` prefix convention is applied consistently and doesn't collide with other platforms' PlatformUID-derived keys.
- **Acceptance criteria:** New pubkey → new `RoleGuest` user; repeat pubkey → no duplicate; existing Telegram/Slack user-resolution tests unaffected.

### P1.9 — Wire Buzz gateway into `cmd/nuimanbot/main.go`

- **Dependencies:** P1.7, P1.8
- **Duration:** ~0.3 day
- **Red:** N/A for main.go wiring (not typically unit-tested directly) — verify via manual `./bin/nuimanbot --help` run and, if the project has any main-package smoke tests, extend them.
- **Green:** Add the `if app.Config.Gateways.Buzz.Enabled { ... }` block per architecture.md, following the identical Telegram/Slack pattern at ~line 924-950.
- **Refactor:** Confirm `gateways = append(gateways, buzzGateway)` and logging match existing conventions exactly.
- **Acceptance criteria:** `go build -o bin/nuimanbot ./cmd/nuimanbot` succeeds; `./bin/nuimanbot --help` runs without panic with `Buzz.Enabled: false` (default, no relay connection attempted).

### P1.10 — Prometheus metrics

- **Dependencies:** P1.7
- **Duration:** ~0.3 day
- **Red:** Test that a verification failure increments `buzz_signature_verification_failures_total`; a successful event increments `buzz_events_received_total{channel_id,sender_is_agent}`.
- **Green:** Register the four metrics in `internal/infrastructure/metrics/prometheus.go`, call from `gateway.go`/`client.go` at the appropriate points.
- **Refactor:** Match existing metric-naming/labeling conventions in the same file.
- **Acceptance criteria:** Metrics visible via existing `/metrics` endpoint (if one exists) or metrics registry test; labels match data-dictionary.md.

**[Phase 1 exit criteria checkpoint — verify all five spec.md Phase 1 exit criteria before starting Phase 2]**

---

## Phase 2 — Full gateway (FR-008, FR-009)

### P2.1 — `Send()`: publish signed, agent-tagged events

- **Dependencies:** P1.1, P1.4, P1.7 (Phase 1 checkpoint passed)
- **Duration:** ~1 day
- **Red:** Test that `Send(ctx, domain.OutgoingMessage{...})` produces a correctly-signed NIP-01 event (verifiable via P1.2's `Verify`) and calls `Client.Publish` for each configured relay.
- **Green:** Implement `Send` on `buzz.Gateway` using `nostr/event.go`'s construction+signing and `nostr/client.go`'s publish path.
- **Refactor:** Ensure error handling wraps relay publish failures with `%w` context; confirm partial-publish-failure (one relay down) doesn't fail the whole `Send()` if at least one relay accepts it (consistent with Phase 1's partial-connectivity NFR).
- **Acceptance criteria:** Signed events verifiable; `buzz_events_published_total{status}` incremented; matches Phase 2 exit criterion "Send() publishes correctly-signed, correctly-tagged events."

### P2.2 — Agent-tagging on outgoing events

- **Dependencies:** P2.1
- **Duration:** ~0.3 day
- **Red:** Test that outgoing events carry the agent-identity tag/marker (exact tag format resolved via research.md Q5 — if still unresolved at this point, this task blocks on a quick follow-up spike, documented in implementation-notes.md).
- **Green:** Add agent-marker tag construction in `event.go` or `gateway.go`'s `Send` path.
- **Refactor:** Keep tag-construction logic co-located with other NIP-01 tag handling (subscription.go's channel tags) for consistency.
- **Acceptance criteria:** Outgoing events are distinguishable as agent-authored by other Buzz-aware clients per the resolved tag convention.

### P2.3 — Loop-prevention guard

- **Dependencies:** P2.2
- **Duration:** ~1 day
- **Red:** Simulated-exchange test: construct a chain of N agent-authored messages addressed to this agent in rapid succession; assert the guard stops auto-reply after the defined threshold/time window rather than replying indefinitely (explicit Phase 2 exit criterion in spec.md).
- **Green:** Implement the guard per the heuristic finalized in research.md Q2 (time-window and/or tag-based).
- **Refactor:** Extract the guard into a small, independently testable component (not buried inline in `handleEvents`) so the heuristic can be tuned without touching the read loop.
- **Acceptance criteria:** Simulated runaway N-message exchange demonstrably terminates; guard does not suppress legitimate human-to-agent or single agent-to-agent reply.

**[Phase 2 exit criteria checkpoint — verify both spec.md Phase 2 exit criteria before starting Phase 3]**

---

## Phase 3 — Tool integration (FR-010)

### P3.1 — Trigger `GitHub`/`CodingAgent`/`RepoSearch` tools from Buzz messages under existing RBAC

- **Dependencies:** P2.3 (Phase 2 checkpoint passed)
- **Duration:** ~1 day
- **Red:** Test that a Buzz-originated message routed through `ChatService`/`ToolService` triggers tool execution identically to an equivalent Slack/Telegram message — same RBAC checks, same rate-limiting, same audit-log entries (no Buzz-specific bypass).
- **Green:** Confirm/wire the existing `messageHandler` → `ChatService.ProcessMessage` → tool-invocation path (already used by CLI per `cmd/nuimanbot/main.go`'s `skillHandler.SetMessageHandler` pattern) works unchanged for `PlatformBuzz`; add any Buzz-specific plumbing only if a gap is found (expected to be none, per architecture.md's "usecase layer unchanged" decision).
- **Refactor:** Remove any Buzz-specific special-casing discovered to be unnecessary; keep the tool-triggering path platform-agnostic.
- **Acceptance criteria:** Tool execution triggered from Buzz is RBAC-checked and audit-logged identically to other platforms; matches both Phase 3 exit criteria in spec.md.

**[Phase 3 exit criteria checkpoint — verify Overall Acceptance in spec.md]**

---

## Task Dependency Summary

```
P0.1
 └─ P1.1 ─┬─ P1.2 ─┐
          └─ P1.3 ─┴─ P1.4 ─┐
P1.5 (parallel) ────────────┼─ P1.7 ─┬─ P1.8 ─┬─ P1.9
P1.6 (parallel) ────────────┘        └─ P1.10 ┘
                                              │
                              [Phase 1 checkpoint]
                                              │
                                     P2.1 ─ P2.2 ─ P2.3
                                              │
                              [Phase 2 checkpoint]
                                              │
                                           P3.1
                                              │
                              [Phase 3 checkpoint / Overall Acceptance]
```
