# Status: NuimanBot Support for Buzz

**Feature:** nuimanbot-support-buzz
**Created:** 2026-08-02
**Last Updated:** 2026-08-02

## Overall Progress

| Phase | Description | Status | Progress |
|---|---|---|---|
| Phase 0 | Spec creation + review | Complete | 100% |
| Phase 0 | Pre-implementation spikes (P0.1, P0.2) | Complete | 100% |
| Phase 1 | Read-only participation (FR-001..FR-007) | Complete | 100% |
| Phase 2 | Full gateway — Send() + loop prevention (FR-008, FR-009) | In Progress | 25% |
| Phase 3 | Tool integration (FR-010) | Not Started (out of this agent's scope) | 0% |

## Phase 2 Task Checklist

- [x] P2.1 — `Send()`: publish signed `kind:9` channel messages (`internal/adapter/gateway/buzz/gateway.go`, `internal/infrastructure/nostr/client.go` `Publish`/`ConnectedRelayCount`)
- [ ] P2.2 — Publish agent-profile event (`kind:10100`)
- [ ] P2.3 — Subscribe to `kind:9000`/`kind:10100`, maintain is_agent cache
- [ ] P2.4 — Loop-prevention guard

## Phase 0 Task Checklist

- [x] Spec directory created (`specs/260802-nuimanbot-support-buzz/`)
- [x] spec.md populated from PRD (FRs, NFRs, acceptance criteria carried over from PRD §6.9/§6.10/§8)
- [x] Research questions identified (seeded in research.md, including Nostr library spike)
- [x] Phase files initialized (research, data-dictionary, architecture, plan, tasks, implementation-notes)
- [x] PRD moved into spec directory (`nuimanbot-support-buzz-PRD.md`)
- [x] Spec reviewed (`/review-spec`) — 2026-08-02, verdict: ready for implementation after fixes (see Recent Activity)
- [x] P0.1 — Nostr library spike resolved (research.md Q1): `btcec/v2` + `btcec/v2/schnorr`, hand-rolled NIP-01 layer
- [x] P0.2 — Buzz protocol conventions spike resolved (research.md Q5): kind:9, `h` channel tag, no per-message agent tag

## Phase 1 Task Checklist

- [x] P1.1 — `internal/infrastructure/nostr/event.go`: NIP-01 event construction, ID computation, signing (btcec/v2 + schnorr), verified against an independently-computed (Python) NIP-01 test vector
- [x] P1.2 — `internal/infrastructure/nostr/verify.go`: signature verification (valid/tampered/wrong-pubkey/malformed cases, concurrency-safe, race-detector clean)
- [x] P1.3 — `internal/infrastructure/nostr/subscription.go`: `Filter`/`NewChannelFilter`/`NewSubscriptionRequest` (kind:9, `#h` tag)
- [x] P1.4 — `internal/infrastructure/nostr/client.go`: relay WebSocket client — connect, reconnect w/ bounded exponential backoff, multi-relay fanout, bounded goroutines (tested against an in-process fake relay, race-detector clean)
- [x] P1.5 — `domain.PlatformBuzz` constant added to `internal/domain/message.go`
- [x] P1.6 — `config.BuzzConfig` added to `internal/config/gateway_config.go` + `internal/config/loader.go` (SecureString private_key handling)
- [x] P1.6b — `crypto.EnsureBuzzKeypair` generate-if-absent secp256k1 keypair helper (`internal/infrastructure/crypto/buzz_keygen.go`), persisted via `domain.CredentialVault`
- [x] P1.7 — `buzz.Gateway` (`internal/adapter/gateway/buzz/gateway.go`): Start/Stop/Send-stub/OnMessage, dedupe → verify → map → RBAC resolve → handler pipeline
- [x] P1.8 — RBAC user resolution: `(PlatformBuzz, pubkey)` → `RoleGuest` on first message, via `usecase/user.Service`
- [x] P1.9 — Wired into `cmd/nuimanbot/main.go` (mirrors Telegram/Slack block); manually smoke-tested with `Buzz.Enabled: true`/`false`, no panic either way
- [x] P1.10 — Prometheus metrics registered in `internal/infrastructure/metrics/prometheus.go`, exercised by gateway tests

**Deviation from tasks.md scope:** P1.8/P1.9 required a production `domain.UserRepository` implementation that did not previously exist anywhere in the codebase (only a test mock). Added `internal/infrastructure/storage/file_user_repository.go` (`FileUserRepository`, implementing `usecase/user.ExtendedUserRepository`) to make the wiring real rather than theoretical. See implementation-notes.md.

**Phase 1 exit criteria (spec.md) — all verified:**
- Gateway connects to all configured relay URLs, with reconnect-on-drop. ✓ (client_test.go)
- Event signatures verified before processing; forged/unsigned dropped + counted. ✓ (gateway_test.go)
- Duplicate events (same ID, multiple relays) forwarded exactly once. ✓ (gateway_test.go)
- First message from new `sender_pubkey` creates `RoleGuest` user. ✓ (gateway_test.go)
- Buzz messages map to `domain.IncomingMessage` (`ValidateInput()` applies unchanged downstream — no Buzz-specific bypass introduced). ✓ by construction — no code touches the Security Service.
- Keypair generate-if-absent + persisted via vault, reused on restart. ✓ (buzz_keygen_test.go)

## Blockers

None. Phase 0 and Phase 1 are complete; P2.1 complete. All AGENTS.md quality gates pass (`go fmt`, `go mod tidy`, `go vet`, `golangci-lint run` — 0 issues, `go test ./...` — all green, `go build` succeeds, `./bin/nuimanbot --help` runs without panic).

Phase 3 (P3.1) remains blocked on a design decision: research.md Q6 (tool-execution RBAC-enforcement gap, found during spec review — `usecase/chat/tool_conversion.go` currently bypasses `ExecuteWithUser`'s RBAC/rate-limit checks for all platforms, not just Buzz). Does not block Phase 1/2. Phase 3 is out of this agent's scope and has not been started.

## Recent Activity (Phase 2)

- 2026-08-02: P2.1 complete — `Client.Publish` (new: relay connection tracking via `relayConn`/`connsMu`, `ConnectedRelayCount`) added to `internal/infrastructure/nostr/client.go`; `Event` given NIP-01 wire JSON tags + a `Tag()` helper in `event.go`; `Gateway.Send()` implemented in `internal/adapter/gateway/buzz/gateway.go`, publishing correctly-signed, `#h`-tagged `kind:9` events and incrementing `buzz_events_published_total{status}`. All new code covered by tests (client_test.go, event_test.go, gateway_test.go), race-detector clean, full quality gate green.

## Recent Activity (Phase 0-1)

- 2026-08-02: Spec directory created from `nuimanbot-support-buzz-PRD.md`. All phase files initialized. PRD content (FRs, NFRs, rollout exit criteria) carried into spec.md without re-derivation, per explicit instruction — this PRD was already reviewed/hardened for this purpose.
- 2026-08-02: Spec review completed (`/review-spec`). Verified claims against the actual codebase and fixed several gaps directly: (1) added tasks.md P0.2 spike for research.md Q5 (was previously unscoped, silently deferred into P1.3's refactor step); (2) added tasks.md P1.6b — FR-007's generate-if-absent keypair helper had zero task coverage; added corresponding Phase 1 exit criterion to spec.md; (3) corrected the `"buzz:<pubkey>"` user-key convention across spec.md/data-dictionary.md/architecture.md/tasks.md — `domain.User.ID` is a UUID, lookup is by `(Platform, PlatformUID)` tuple via the existing `usecase/user.Service`, and no gateway currently auto-creates users on first message (that claim was inaccurate — CreateUser today is CLI-admin-only); (4) clarified `BuzzConfig.DMPolicy` is reserved/unused — no FR in this spec covers Buzz DMs; (5) flagged (not resolved) research.md Q6: Phase 3's "no bypass" RBAC claim can't be verified as-is because the chat-triggered tool path bypasses RBAC/rate-limit checks for all platforms today, not just Buzz — needs a design decision before P3.1 starts.
