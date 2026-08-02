# Tasks: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Status:** Planning

## Progress Summary

0/17 tasks complete (0%)

(Corrected during spec review: the original count of 21 did not match the 15 task headers actually present. Two tasks were added during review — P0.2 and P1.6b — bringing the accurate total to 17.)

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

### P0.2 — Resolve Buzz protocol conventions (event kind, channel tag, agent-identity tag)

- **Dependencies:** none (can run in parallel with P0.1)
- **Duration:** ~0.5 day
- **Description:** Spike/read Buzz's open-source protocol spec/source (PRD §2) to determine: (a) the NIP-01 event `Kind` used for channel messages, (b) the tag name/format used to associate an event with a Buzz channel ID, (c) the tag name/format Buzz uses to mark an event as agent-authored (needed for `sender_is_agent` on the read side and outgoing agent-tagging in P2.2). Not a TDD task (research, not implementation) — resolve research.md Q5 and record the decision + rationale in implementation-notes.md before starting P1.1/P1.3.
- **Acceptance criteria:** research.md Q5 marked resolved; implementation-notes.md records the chosen event kind and tag conventions with a source reference (Buzz repo file/commit or protocol doc section); P1.1 and P1.3 can proceed without guessing kind/tag values.

---

## Phase 1 — Read-only participation (FR-001–FR-007)

### P1.1 — NIP-01 event construction, ID computation, signing (`internal/infrastructure/nostr/event.go`)

- **Dependencies:** P0.1, P0.2 (event `Kind` value needed for realistic fixtures, even though ID-computation itself is kind-agnostic per NIP-01)
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

- **Dependencies:** P1.1, P0.2 (channel-tag convention needed to build correct filters, not just a placeholder)
- **Duration:** ~0.5 day
- **Red:** Tests asserting a `Filter` built from configured channel IDs serializes to the expected NIP-01 `REQ` shape.
- **Green:** Implement `Filter`/`Subscription` construction from `[]string` channel IDs (FR-002), using the tag convention resolved in P0.2.
- **Refactor:** Confirm naming matches the Buzz-specific channel-tag convention recorded in implementation-notes.md from P0.2.
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

### P1.6b — Generate-if-absent secp256k1 keypair helper (FR-007)

- **Dependencies:** P0.1 (library choice determines key-generation API)
- **Duration:** ~0.5 day
- **Red:** Tests: (a) when `BuzzConfig.PrivateKey` is empty, a new secp256k1 keypair is generated and persisted through the existing `VersionedVault` (AES-256-GCM, same mechanism as other vault-stored secrets); (b) when `BuzzConfig.PrivateKey` is already set, no new key is generated and the existing key is used unchanged; (c) the generated public key is derivable/consistent with the stored private key.
- **Green:** Implement a generate-if-absent helper in `internal/infrastructure/crypto/` (e.g. `keygen.go`, alongside the existing `keygen.go`/`vault.go` in that package) that NuimanBot's Buzz gateway `Start()` calls before initializing `nostr.Client`.
- **Refactor:** Confirm the helper doesn't duplicate logic already in `internal/infrastructure/crypto/keygen.go`; ensure the generated key is written back through `VersionedVault` so it persists across restarts, not just held in memory.
- **Acceptance criteria:** No private key configured → key generated once and persisted; restart reuses the persisted key without regenerating; matches spec.md's Phase 1 exit criterion for FR-007.

### P1.7 — `buzz.Gateway`: Start, dedupe, verify, map to `domain.IncomingMessage` (`internal/adapter/gateway/buzz/gateway.go`)

- **Dependencies:** P1.2, P1.4, P1.5, P1.6
- **Duration:** ~1.5 days
- **Red:** Table-driven tests: (a) valid signed event → correct `domain.IncomingMessage` with all six metadata keys populated (FR-005); (b) unsigned/forged event → dropped, not forwarded to handler, `buzz_signature_verification_failures_total` incremented (FR-003); (c) same event ID from two relays → handler invoked exactly once (FR-004); (d) `Platform()` returns `domain.PlatformBuzz`.
- **Green:** Implement `New`, `Platform`, `Start`, `Stop`, `OnMessage`, and `handleEvents` per architecture.md's Phase 1 data flow (dedupe → verify → map → RBAC resolve → handler).
- **Refactor:** Compare against `slack.Gateway`'s structure for consistency (naming, error wrapping with `%w`, doc comments on exported symbols per AGENTS.md).
- **Acceptance criteria:** All Phase 1 exit criteria in spec.md related to connect/verify/dedupe/mapping are demonstrated by tests; mirrors slack/gateway.go's shape.

### P1.8 — RBAC user resolution: `(PlatformBuzz, pubkey)` → `RoleGuest` on first message (FR-006)

- **Dependencies:** P1.7
- **Duration:** ~0.5 day
- **Red:** Test that a message from a new `sender_pubkey` creates a `domain.User` (UUID-assigned `ID`, per `usecase/user.Service.CreateUser`) with `PlatformIDs[domain.PlatformBuzz] == pubkey` and `Role: RoleGuest`; a second message from the same pubkey does not create a duplicate user (verified via `GetUserByPlatformUID` returning the existing user first).
- **Green:** Call `usecase/user.Service.GetUserByPlatformUID(ctx, domain.PlatformBuzz, pubkey)`; if it returns `ErrUserNotFound`, call `CreateUser(ctx, domain.PlatformBuzz, pubkey, domain.RoleGuest)`. **Note:** confirmed during spec review that no existing gateway currently calls this on first message — `CreateUser` today is only invoked from CLI admin commands (`internal/adapter/gateway/cli/admin_commands.go`). Buzz's `gateway.go` will be the first caller from a message-handling path; both methods are already exported on `usecase/user.Service`, so this is new *usage*, not new usecase-layer code.
- **Refactor:** Ensure lookup/creation happens exactly once per event (guard against a race if two events from a brand-new pubkey arrive concurrently across relays before dedupe — see P1.7's dedupe-by-event-ID, which happens first in the pipeline and reduces but doesn't eliminate this window if the same pubkey posts two different events near-simultaneously).
- **Acceptance criteria:** New pubkey → new `RoleGuest` user via `(Platform, PlatformUID)` tuple; repeat pubkey → no duplicate; existing Telegram/Slack user-resolution tests unaffected; no `"buzz:"`-prefixed string ID is introduced anywhere (matches data-dictionary.md's corrected convention).

### P1.9 — Wire Buzz gateway into `cmd/nuimanbot/main.go`

- **Dependencies:** P1.7, P1.8, P1.6b
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

**Design correction (2026-08-02, research.md Q2/Q5):** the original P2.1-P2.3 breakdown assumed Buzz has a per-message agent-identity tag. P0.2's protocol spike found this is false — agent identity comes from a `kind:10100` profile event (agent-authored, replaceable) and `kind:9000` channel-membership role events, not a tag on each `kind:9` message. Revised task breakdown below reflects this.

### P2.1 — `Send()`: publish signed `kind:9` channel messages

- **Dependencies:** P1.1, P1.4, P1.7 (Phase 1 checkpoint passed)
- **Duration:** ~1 day
- **Red:** Test that `Send(ctx, domain.OutgoingMessage{...})` produces a correctly-signed `kind:9` NIP-01 event (verifiable via P1.2's `Verify`), carries the required `#h` channel-ID tag (per P0.2), and calls `Client.Publish` for each configured relay.
- **Green:** Implement `Send` on `buzz.Gateway` using `nostr/event.go`'s construction+signing and `nostr/client.go`'s publish path.
- **Refactor:** Ensure error handling wraps relay publish failures with `%w` context; confirm partial-publish-failure (one relay down) doesn't fail the whole `Send()` if at least one relay accepts it (consistent with Phase 1's partial-connectivity NFR).
- **Acceptance criteria:** Signed `kind:9` events verifiable, carry `#h` tag; `buzz_events_published_total{status}` incremented; matches Phase 2 exit criterion "Send() publishes correctly-signed kind:9 channel messages."

### P2.2 — Publish agent-profile event (`kind:10100`)

- **Dependencies:** P2.1
- **Duration:** ~0.5 day
- **Pre-requisite:** confirm the exact `kind:10100` event schema (content/tag fields for "agent metadata + owner reference") against `github.com/block/buzz`'s source (`crates/buzz-core/src/kind.rs` and wherever `KIND_AGENT_PROFILE` fields are defined/consumed) — P0.2 confirmed the kind number and its purpose but not the full field schema. Treat this as a short in-task spike, not a guess; document findings in implementation-notes.md.
- **Red:** Test that the gateway publishes (or republishes on config change, since `kind:10100` is replaceable) a correctly-signed `kind:10100` event containing this agent's identity/owner-reference per the confirmed schema, once at `Start()` and not repeated per-message.
- **Green:** Implement profile-event construction and publication, reusing `event.go`/`client.go`'s signing/publish path from P2.1.
- **Refactor:** Keep profile-event construction separate from `Send()`'s per-message path — it's a distinct, low-frequency publication, not part of the message-send hot path.
- **Acceptance criteria:** A `kind:10100` profile event is published and independently verifiable/decodable by another Buzz-aware client as declaring this agent's identity; matches Phase 2 exit criterion on agent-profile publication.

### P2.3 — Subscribe to `kind:9000`/`kind:10100`, maintain is_agent cache

- **Dependencies:** P1.3 (subscription.go exists), P2.2 (so the cache logic can be tested against this agent's own published profile too)
- **Duration:** ~1 day
- **Red:** Test that receiving a `kind:9000` event with `MemberRole: Bot` for a pubkey, or a `kind:10100` profile event from a pubkey, updates an in-memory pubkey→is_agent cache; test cache lookup returns correct is_agent value for known/unknown pubkeys; test `sender_is_agent` in `IncomingMessage.Metadata` reflects a cache lookup at message-receive time, not a hardcoded value.
- **Green:** Extend `subscription.go`/`gateway.go` to subscribe to `kind:9000` and `kind:10100` per joined channel, and add the pubkey→is_agent cache (simple in-memory map with a mutex or `sync.Map`; no persistence required for Phase 2).
- **Refactor:** Keep the cache as an independently testable component, not buried in the event-read loop; confirm it's safe under concurrent access (relay read loop + membership/profile update loop).
- **Acceptance criteria:** `sender_is_agent` correctly reflects live membership/profile data, not a placeholder; matches Phase 2 exit criterion on the is_agent cache.

### P2.4 — Loop-prevention guard

- **Dependencies:** P2.3
- **Duration:** ~1 day
- **Red:** Simulated-exchange test: construct a chain of N agent-authored messages (cache-marked as agent via P2.3) addressed to this agent in rapid succession; assert the guard stops auto-reply after the defined threshold/time window rather than replying indefinitely (explicit Phase 2 exit criterion in spec.md).
- **Green:** Implement the guard per the heuristic finalized in research.md Q2 (time-window and/or reply-chain based), consulting the P2.3 is_agent cache rather than a per-message field.
- **Refactor:** Extract the guard into a small, independently testable component (not buried inline in `handleEvents`) so the heuristic can be tuned without touching the read loop.
- **Acceptance criteria:** Simulated runaway N-message exchange demonstrably terminates; guard does not suppress legitimate human-to-agent or single agent-to-agent reply.

**[Phase 2 exit criteria checkpoint — verify all four spec.md Phase 2 exit criteria before starting Phase 3]**

---

## Phase 3 — Tool integration (FR-010, FR-011, FR-012)

### P3.1 — Wire `ChatService`'s tool-invocation path to `ExecuteWithUser()` for all platforms

- **Dependencies:** P2.3 (Phase 2 checkpoint passed)
- **Duration:** ~1 day
- **Resolved 2026-08-02 (research.md Q6):** `usecase/chat/tool_conversion.go:31` currently calls the unchecked `Execute()` for **all** platforms today (Telegram, Slack, CLI) — not a Buzz-specific gap. Decision: fix it for everyone as part of this phase (FR-011), accepting that existing users may see tool availability change if their role lacks permission for a tool they could previously call unchecked.
- **Red:** Regression tests asserting that a message from each existing platform (Telegram, Slack, CLI) routed through `ChatService` → tool conversion triggers `tool.Service.ExecuteWithUser()` (not `Execute()`), with `checkPermission`, rate limiting, and audit logging all exercised. Include a case where a `RoleGuest` user is denied a tool their role doesn't permit — today this would silently succeed via `Execute()`; after the fix it must be rejected.
- **Green:** Change `usecase/chat/tool_conversion.go:31` to call `tool.Service.ExecuteWithUser()`, threading the resolved `domain.User` through the existing `ChatService.ProcessMessage` call chain.
- **Refactor:** Confirm no duplicate permission-checking logic is introduced in `usecase/chat` — `ExecuteWithUser` already owns `checkPermission`/rate-limiting/audit; `ChatService` should not re-implement any of it.
- **Acceptance criteria:** FR-011 met; all pre-existing platform tests still pass except where they asserted the old unchecked behavior (those are the ones the Red step updates); no Clean Architecture violation (usecase layer still owns the RBAC decision, adapter/gateway layers unchanged).

### P3.2 — Role-filtered `ListTools()`

- **Dependencies:** P3.1
- **Duration:** ~0.5 day
- **Red:** Test that `ListTools(ctx, userID)` returns different tool sets for `RoleGuest` vs `RoleAdmin` against fixture data where at least one tool is role-restricted.
- **Green:** Implement the `TODO: Implement user-specific tool filtering` at `internal/usecase/tool/service.go:261`.
- **Refactor:** Ensure the filtering logic is shared with (not duplicated from) whatever role-check helper `ExecuteWithUser`/`checkPermission` already uses.
- **Acceptance criteria:** FR-012 met; `ListTools` output is role-accurate for every existing platform, not just Buzz.

### P3.3 — Trigger `GitHub`/`CodingAgent`/`RepoSearch` tools from Buzz messages under (now-enforced) RBAC

- **Dependencies:** P3.1, P3.2
- **Duration:** ~0.5 day
- **Red:** Test that a Buzz-originated message routed through `ChatService`/`ToolService` triggers tool execution identically to an equivalent Slack/Telegram message post-P3.1/P3.2 — same `checkPermission`/rate-limit/audit path, same `RoleGuest` denial behavior for an under-permissioned Buzz pubkey.
- **Green:** Confirm the existing `messageHandler` → `ChatService.ProcessMessage` → tool-invocation path (already used by CLI per `cmd/nuimanbot/main.go`'s `skillHandler.SetMessageHandler` pattern) works unchanged for `PlatformBuzz` — this should require no new code if P3.1/P3.2 are platform-agnostic as intended.
- **Refactor:** Remove any Buzz-specific special-casing discovered to be unnecessary; keep the tool-triggering path platform-agnostic.
- **Acceptance criteria:** FR-010 met; matches both Phase 3 exit criteria in spec.md, including the RBAC-enforcement and role-filtered-`ListTools` criteria added alongside FR-011/FR-012.

**[Phase 3 exit criteria checkpoint — verify Overall Acceptance in spec.md]**

---

## Task Dependency Summary

```
P0.1 ─┬─ P1.1 ─┬─ P1.2 ─┐
P0.2 ─┘        └─ P1.3 ─┴─ P1.4 ─┐
P0.1 ─ P1.6b (parallel) ─────────┤
P1.5 (parallel) ─────────────────┼─ P1.7 ─┬─ P1.8 ─┬─ P1.9 (also needs P1.6b)
P1.6 (parallel) ─────────────────┘        └─ P1.10 ┘
                                              │
                              [Phase 1 checkpoint — incl. FR-007 keypair criterion]
                                              │
                                     P2.1 ─ P2.2 ─ P2.3 ─ P2.4
                                              │
                              [Phase 2 checkpoint]
                                              │
                                     P3.1 ─ P3.2 ─ P3.3
                                              │
                              [Phase 3 checkpoint / Overall Acceptance]
```
